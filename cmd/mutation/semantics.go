package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// The two policies the format defines, named once because a rule is written
// out as well as read back in, and a file that dumps one policy must not be
// able to drift from the parser that will read it.
const (
	excludePolicy = "exclude"
	fixedPolicy   = "fixed"
)

// geneSemanticsFileMode matches the mode every other file this command writes
// uses, so a dumped template is not more readable than a generated patch.
const geneSemanticsFileMode = 0o600

type geneSemantic struct {
	Policy string `json:"policy"`
	Value  *int   `json:"value,omitempty"`
}

// loadGeneSemantics reads one rule per array entry so duplicate gene names can
// be rejected instead of silently choosing one policy.
func loadGeneSemantics(path string) (map[string]geneSemantic, error) {
	semantics := make(map[string]geneSemantic)
	if path == "" {
		return semantics, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read gene semantics %q: %w", path, err)
	}
	var entries []map[string]geneSemantic
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse gene semantics %q: %w", path, err)
	}
	for entryIndex, entry := range entries {
		if len(entry) != 1 {
			return nil, fmt.Errorf("gene semantics entry %d must contain one gene name", entryIndex)
		}
		for name, rule := range entry {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("gene semantics entry %d has an empty gene name", entryIndex)
			}
			if _, exists := semantics[name]; exists {
				return nil, fmt.Errorf("gene %q has more than one semantic rule", name)
			}
			policy := strings.ToLower(strings.TrimSpace(rule.Policy))
			switch policy {
			case excludePolicy:
				if rule.Value != nil {
					return nil, fmt.Errorf("gene %q cannot be excluded with a value", name)
				}
			case fixedPolicy:
				// The numeric bound is checked against the gene's own domain when
				// the patch is built, because a SysEx field is not a 0-127 CC.
			default:
				return nil, fmt.Errorf("gene %q has unsupported policy %q", name, rule.Policy)
			}
			rule.Policy = policy
			semantics[name] = rule
		}
	}
	return semantics, nil
}

// dumpExcludedGenes writes every parameter the selected model exposes as an
// exclude rule, and does nothing else.
//
// The result is the starting point for a --gene-semantics file rather than a
// finished one: exclude everything, then delete the entries worth evolving. The
// alternative is discovering a parameter's spelling one rejected rule at a time,
// and a SysEx name like Osc[0].EgRate[1] is not something to guess.
//
// Nothing a run needs is required here, because nothing a run does is started.
// No seed, since a SysEx catalog comes from the program type rather than from a
// particular dump, and no port or output, since no instrument is addressed.
func dumpExcludedGenes(config configuration, output io.Writer) error {
	if config.geneSemantics != "" {
		return fmt.Errorf("--gene-semantics does not apply to --dump-excludes, which writes a fresh file of every parameter")
	}
	format, err := selectPatchFormat(config)
	if err != nil {
		return err
	}
	if err := validateModelName(format, config); err != nil {
		return err
	}
	// The catalog is built with no rules applied, so a parameter the caller
	// already excludes is still listed. A dump of what a run would already
	// reach would be useless as a starting point.
	candidate, err := format.newPatch(nil)
	if err != nil {
		return err
	}
	entries := excludeEveryGene(candidate.geneStore().genes)
	if err := writeGeneSemantics(config.dumpExcludes, entries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "wrote %d excluded parameters for %s to %s\n", len(entries), dumpLabel(format, config), config.dumpExcludes); err != nil {
		return err
	}
	// A file that excludes everything evolves nothing, so the file is only
	// half of what the reader is about to do. Saying the other half now
	// avoids a run that silently breeds identical candidates.
	_, err = fmt.Fprintln(output, "delete the entries worth evolving, then pass the file to --gene-semantics")
	return err
}

// dumpLabel names what was dumped. A CC model is named by --model and a SysEx
// program by its format, so the label is whichever the format expects rather
// than a model name that is empty for every SysEx run. A file holds one model's
// parameters, so the reader needs to know which.
func dumpLabel(format patchFactory, config configuration) string {
	if format.acceptsModelName() {
		return config.modelName
	}
	return config.format
}

// excludeEveryGene renders one rule per gene in the order the model declares
// them, so two dumps of the same model are the same file and a diff between
// them shows a change in the model rather than in the order of a map.
func excludeEveryGene(genes []gene) []map[string]geneSemantic {
	entries := make([]map[string]geneSemantic, 0, len(genes))
	for _, current := range genes {
		entries = append(entries, map[string]geneSemantic{current.name: {Policy: excludePolicy}})
	}
	return entries
}

// writeGeneSemantics writes the shape loadGeneSemantics reads. The result is
// indented and newline-terminated because its purpose is to be edited: one gene
// per line is what makes deleting the ones worth keeping a readable edit.
func writeGeneSemantics(path string, entries []map[string]geneSemantic) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode gene semantics %q: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), geneSemanticsFileMode); err != nil {
		return fmt.Errorf("write gene semantics %q: %w", path, err)
	}
	return nil
}
