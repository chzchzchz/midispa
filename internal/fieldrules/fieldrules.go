// Package fieldrules parses the per-field rule file that cccli and
// mutation both read. A rule is keyed by a field name, which is a
// cc.ControlField name for a CC model and a gene name for a SysEx
// format, and says either that the field is excluded or that its value
// is fixed. The name describes what the rules are keyed by rather
// than either command's vocabulary, because one parser serves one
// file format for both.
package fieldrules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// The two policies the format defines, named once because a rule is
// written out as well as read back in, and a file that dumps one
// policy must not be able to drift from the parser that will read it.
const (
	PolicyExclude = "exclude"
	PolicyFixed   = "fixed"
)

// Rule is one field's rule: an exclusion, or a fixed value to write
// instead of the field's own.
type Rule struct {
	Policy string `json:"policy"`
	Value  *int   `json:"value,omitempty"`
}

// Load reads one rule per array entry so duplicate field names can be
// rejected instead of silently choosing one policy. An empty path
// loads no rules, so a caller that makes the file optional passes
// its flag through unchanged.
func Load(path string) (map[string]Rule, error) {
	rules := make(map[string]Rule)
	if path == "" {
		return rules, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read field rules %q: %w", path, err)
	}
	var entries []map[string]Rule
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse field rules %q: %w", path, err)
	}
	for entryIndex, entry := range entries {
		if len(entry) != 1 {
			return nil, fmt.Errorf("field rules entry %d must contain one field name", entryIndex)
		}
		for name, rule := range entry {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("field rules entry %d has an empty field name", entryIndex)
			}
			if _, exists := rules[name]; exists {
				return nil, fmt.Errorf("field %q has more than one rule", name)
			}
			policy := strings.ToLower(strings.TrimSpace(rule.Policy))
			switch policy {
			case PolicyExclude:
				if rule.Value != nil {
					return nil, fmt.Errorf("field %q cannot be excluded with a value", name)
				}
			case PolicyFixed:
				// The numeric bound is checked against the field's own
				// domain when the patch is built, because a SysEx field
				// is not a 0-127 CC.
			default:
				return nil, fmt.Errorf("field %q has unsupported policy %q", name, rule.Policy)
			}
			rule.Policy = policy
			rules[name] = rule
		}
	}
	return rules, nil
}
