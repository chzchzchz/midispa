package fieldrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRulesFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600), "write rules")
	return path
}

func TestLoadAcceptsExcludeOnlyFile(t *testing.T) {
	// A file of only exclude rules is what a dump writes before it
	// is edited, and what cccli passes as --field-rules, so the
	// parser has to accept it even though it carries no value.
	path := writeRulesFile(t, `[
		{"SoundController1":{"policy":"exclude"}},
		{"SoundController2":{"policy":"exclude"}}
	]`)
	rules, err := Load(path)
	require.NoError(t, err, "Load")
	require.Len(t, rules, 2, "loaded rules")
	assert.Equal(t, PolicyExclude, rules["SoundController1"].Policy, "unexpected exclude rule")
	assert.Equal(t, PolicyExclude, rules["SoundController2"].Policy, "unexpected exclude rule")
	assert.Nil(t, rules["SoundController1"].Value, "excluded rule carries a value")
}

func TestLoadAcceptsMixedPolicies(t *testing.T) {
	fixedValue := 40
	path := writeRulesFile(t, `[
		{"SoundController1":{"policy":"exclude"}},
		{"SoundController2":{"policy":"fixed","value":40}}
	]`)
	rules, err := Load(path)
	require.NoError(t, err, "Load")
	require.Len(t, rules, 2, "loaded rules")
	assert.Equal(t, PolicyExclude, rules["SoundController1"].Policy, "unexpected exclude rule")
	if assert.NotNil(t, rules["SoundController2"].Value, "unexpected fixed value") {
		assert.Equal(t, fixedValue, *rules["SoundController2"].Value, "unexpected fixed value")
	}
}

func TestLoadEmptyPathLoadsNoRules(t *testing.T) {
	rules, err := Load("")
	require.NoError(t, err, "Load")
	assert.Empty(t, rules, "an empty path loaded rules")
}

func TestLoadRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{name: "unknown policy", contents: `[{"SoundController1":{"policy":"unknown"}}]`},
		{name: "exclude with a value", contents: `[{"SoundController1":{"policy":"exclude","value":1}}]`},
		{name: "repeated field", contents: `[{"SoundController1":{"policy":"exclude"}},{"SoundController1":{"policy":"fixed"}}]`},
		{name: "several rules at once", contents: `[{"SoundController1":{"policy":"exclude"},"SoundController2":{"policy":"fixed"}}]`},
		{name: "empty field name", contents: `[{"":{"policy":"exclude"}}]`},
		{name: "not json", contents: `not-json`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(writeRulesFile(t, test.contents))
			assert.Error(t, err, "accepted invalid rules: %s", test.contents)
		})
	}
}
