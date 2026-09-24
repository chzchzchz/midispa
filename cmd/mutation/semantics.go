package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

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
			case "exclude":
				if rule.Value != nil {
					return nil, fmt.Errorf("gene %q cannot be excluded with a value", name)
				}
			case "fixed":
				if rule.Value != nil && (*rule.Value < 0 || *rule.Value > maxMIDIValue) {
					return nil, fmt.Errorf("gene %q fixed value is outside 0-127", name)
				}
			default:
				return nil, fmt.Errorf("gene %q has unsupported policy %q", name, rule.Policy)
			}
			rule.Policy = policy
			semantics[name] = rule
		}
	}
	return semantics, nil
}
