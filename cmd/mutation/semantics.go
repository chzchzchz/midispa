package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/chzchzchz/midispa/internal/fieldrules"
)

// geneSemanticsFileMode matches the mode every other file this command writes
// uses, so a dumped template is not more readable than a generated patch.
const geneSemanticsFileMode = 0o600

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
func excludeEveryGene(genes []gene) []map[string]fieldrules.Rule {
	entries := make([]map[string]fieldrules.Rule, 0, len(genes))
	for _, current := range genes {
		entries = append(entries, map[string]fieldrules.Rule{current.name: {Policy: fieldrules.PolicyExclude}})
	}
	return entries
}

// writeGeneSemantics writes the shape fieldrules.Load reads. The result is
// indented and newline-terminated because its purpose is to be edited: one gene
// per line is what makes deleting the ones worth keeping a readable edit.
func writeGeneSemantics(path string, entries []map[string]fieldrules.Rule) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode gene semantics %q: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), geneSemanticsFileMode); err != nil {
		return fmt.Errorf("write gene semantics %q: %w", path, err)
	}
	return nil
}
