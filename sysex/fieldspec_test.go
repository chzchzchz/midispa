package sysex_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/sysex"
	"github.com/chzchzchz/midispa/sysex/behringer/pro800"
	dx7 "github.com/chzchzchz/midispa/sysex/yamaha/dx7"
)

type EmbeddedSpec struct {
	Level int `range:"0..3"`
}

type nestedSpec struct {
	Channel  int     `range:"0..15"`
	Step     [2]int  `range:"0..99"`
	Mode     int     `oneof:"0,2,3"`
	Switch   [3]int  `oneof:"0,1"`
	Name     [4]byte `range:"0..127" ascii:"32..126"`
	Untagged int
	Oscs     [2]EmbeddedSpec
	EmbeddedSpec
}

type conflictingSpec struct {
	Value int `range:"0..7" oneof:"0,1"`
}

type pointerSpec struct {
	Value *int `range:"0..7"`
}

type mapSpec struct {
	Value map[string]int `range:"0..7"`
}

type textSpec struct {
	Name string `range:"0..7"`
}

// mixedSpec is the shape a vendor record takes when it holds more than
// parameters: a name, a presence flag, a reserved float, and an untagged
// integer that is a table the vendor has not declared bounds for.
type mixedSpec struct {
	Volume  int     `range:"0..99"`
	Name    string  `range:"0..15"`
	Present bool    `oneof:"0,1"`
	Ratio   float64 `range:"0..1"`
	Notes   []string
	Table   [4]int32
	Nested  [2]EmbeddedSpec
	// Address is routing rather than a sound, and declares that itself.
	Address int `range:"0..399" mutate:"skip"`
	// Reserved is a byte the record marks unused.
	Reserved [2]int `range:"0..255" mutate:"skip"`
	// Pinned is a struct whose whole subtree is skipped.
	Pinned EmbeddedSpec `mutate:"skip"`
}

type malformedMutateSpec struct {
	Value int `range:"0..7" mutate:"skipmost"`
}

type unexportedSpec struct {
	Value int `range:"0..7"`
	inner int
}

func specByName(t *testing.T, specs []sysex.FieldSpec, name string) sysex.FieldSpec {
	t.Helper()
	for _, spec := range specs {
		if spec.Name == name {
			return spec
		}
	}
	t.Fatalf("no field spec named %q in %v", name, specNames(specs))
	return sysex.FieldSpec{}
}

func specByNameOrNil(specs []sysex.FieldSpec, name string) *sysex.FieldSpec {
	for index := range specs {
		if specs[index].Name == name {
			return &specs[index]
		}
	}
	return nil
}

func specNames(specs []sysex.FieldSpec) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}

func TestFieldSpecsExpandsNestedCollections(t *testing.T) {
	specs, err := sysex.FieldSpecs(&nestedSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	want := []string{
		"Channel",
		"Step[0]",
		"Step[1]",
		"Mode",
		"Switch[0]",
		"Switch[1]",
		"Switch[2]",
		"Name[0]",
		"Name[1]",
		"Name[2]",
		"Name[3]",
		"Oscs[0].Level",
		"Oscs[1].Level",
		"Level",
	}
	if got := specNames(specs); !reflect.DeepEqual(got, want) {
		t.Fatalf("field names are %v, want %v", got, want)
	}
}

func TestFieldSpecsSkipsUntaggedFields(t *testing.T) {
	specs, err := sysex.FieldSpecs(&nestedSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	for _, spec := range specs {
		if spec.Name == "Untagged" {
			t.Fatal("an untagged field is not a sound parameter")
		}
	}
}

func TestFieldSpecsReadsConstraintKinds(t *testing.T) {
	specs, err := sysex.FieldSpecs(&nestedSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	step := specByName(t, specs, "Step[1]").Domain
	if step.Minimum != 0 || step.Maximum != 99 || len(step.Allowed) != 0 {
		t.Fatalf("range domain is %v, want 0..99", step)
	}
	mode := specByName(t, specs, "Mode").Domain
	if !reflect.DeepEqual(mode.Allowed, []int{0, 2, 3}) {
		t.Fatalf("oneof domain is %v, want 0,2,3", mode)
	}
	if !mode.Contains(2) || mode.Contains(1) {
		t.Fatalf("oneof domain %v admits 1", mode)
	}
	// A oneof on a collection constrains each element, not the collection.
	if got, want := specByName(t, specs, "Switch[2]").Domain.Allowed, []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("element oneof domain is %v, want 0,1", got)
	}
	name := specByName(t, specs, "Name[0]").Domain
	if name.Contains(0) || name.Contains(31) || !name.Contains(126) {
		t.Fatalf("ascii domain %v is not printable", name)
	}
}

func TestFieldSpecsEmbedsFieldsWithoutASegment(t *testing.T) {
	specs, err := sysex.FieldSpecs(&nestedSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	if got := specByName(t, specs, "Level"); got.Domain.Maximum != 3 {
		t.Fatalf("embedded field domain is %v, want 0..3", got.Domain)
	}
}

func TestFieldSpecsPathAddressesTheValue(t *testing.T) {
	program := &nestedSpec{}
	program.Step[1] = 42
	specs, err := sysex.FieldSpecs(program)
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	step := specByName(t, specs, "Step[1]")
	value := reflect.ValueOf(program)
	for _, index := range step.Path {
		value = resolvePath(value, []int{index})
	}
	if got := value.Int(); got != 42 {
		t.Fatalf("path %v addresses %d, want 42", step.Path, got)
	}
}

func TestFieldSpecsRejectsUnusableFields(t *testing.T) {
	tests := []struct {
		name    string
		root    any
		message string
	}{
		{name: "range and oneof", root: &conflictingSpec{}, message: "carries both"},
		{name: "pointer", root: &pointerSpec{}, message: "unsupported field kind"},
		{name: "map", root: &mapSpec{}, message: "unsupported field kind"},
		{name: "malformed mutate tag", root: &malformedMutateSpec{}, message: "malformed mutate tag"},
		{name: "not a struct", root: 7, message: "not a struct"},
		{name: "nil pointer", root: (*nestedSpec)(nil), message: "nil program pointer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := sysex.FieldSpecs(test.root)
			if err == nil {
				t.Fatal("accepted an unusable field")
			}
			if !errors.Is(err, sysex.ErrBadRange) && !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error is %v, want it to mention %q", err, test.message)
			}
		})
	}
}

// A vendor record holds text, flags and reserved bytes. The walker must walk
// past all of them, and must not offer one as a gene even when it carries a
// domain tag, because a gene is a bounded integer by definition.
func TestFieldSpecsSkipsFieldsThatAreNotParameters(t *testing.T) {
	specs, err := sysex.FieldSpecs(&mixedSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	if got, want := specNames(specs), []string{"Volume", "Nested[0].Level", "Nested[1].Level"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("field names are %v, want %v", got, want)
	}
	for _, name := range []string{"Name", "Present", "Ratio", "Notes", "Table", "Address", "Reserved[0]", "Reserved[1]", "Pinned.Level"} {
		if specByNameOrNil(specs, name) != nil {
			t.Fatalf("%s is not a bounded integer and must not be a gene", name)
		}
	}
}

func TestFieldSpecsSkipsATaggedName(t *testing.T) {
	// textSpec tags a string, which is a struct-tag mistake rather than a
	// reason to fail the whole walk: the rest of the record is still usable.
	specs, err := sysex.FieldSpecs(&textSpec{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("a tagged string became %v", specNames(specs))
	}
}

func TestFieldSpecsIgnoresUnexportedFields(t *testing.T) {
	specs, err := sysex.FieldSpecs(&unexportedSpec{inner: 3})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	if got, want := specNames(specs), []string{"Value"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("field names are %v, want %v", got, want)
	}
}

// TestFieldSpecsCoversVendorRecords checks the catalog against real vendor
// records without naming a field here: an independent type-level walk counts
// the constrained integer leaves, and every spec must resolve to a real integer
// field with a usable domain. A record that also holds a name or a presence
// flag has to survive the walk, since a generator cannot use either.
func TestFieldSpecsCoversVendorRecords(t *testing.T) {
	records := []struct {
		name  string
		root  any
		count int
	}{
		{name: "dx7 single voice", root: &dx7.SingleVoice{}},
		{name: "pro800 patch", root: &pro800.Patch{}},
	}
	for _, record := range records {
		t.Run(record.name, func(t *testing.T) {
			specs, err := sysex.FieldSpecs(record.root)
			if err != nil {
				t.Fatalf("FieldSpecs: %v", err)
			}
			if want := countConstrainedLeaves(reflect.TypeOf(record.root).Elem()); len(specs) != want {
				t.Fatalf("catalog has %d fields, want %d", len(specs), want)
			}
			names := make(map[string]bool, len(specs))
			for _, spec := range specs {
				if names[spec.Name] {
					t.Fatalf("duplicate field name %q", spec.Name)
				}
				names[spec.Name] = true
				value := resolvePath(reflect.ValueOf(record.root), spec.Path)
				if value.Kind() == reflect.Invalid || !isIntegerType(value.Kind()) {
					t.Fatalf("path %v for %q does not address an integer field", spec.Path, spec.Name)
				}
				assertUsableDomain(t, spec)
			}
		})
	}
}

// countConstrainedLeaves walks a type independently of the walker, so a
// missed or invented field shows up as a count mismatch.
func countConstrainedLeaves(structType reflect.Type) int {
	count := 0
	for index := 0; index < structType.NumField(); index++ {
		info := structType.Field(index)
		if !info.IsExported() {
			continue
		}
		if tag, ok := info.Tag.Lookup(sysex.MutateTag); ok && tag == sysex.SkipField {
			continue
		}
		constrained := false
		for _, tag := range []string{sysex.RangeTag, sysex.OneofTag, sysex.AsciiTag} {
			if _, ok := info.Tag.Lookup(tag); ok {
				constrained = true
			}
		}
		switch info.Type.Kind() {
		case reflect.Struct:
			count += countConstrainedLeaves(info.Type)
		case reflect.Array, reflect.Slice:
			if info.Type.Elem().Kind() == reflect.Struct {
				count += countConstrainedLeaves(info.Type.Elem()) * info.Type.Len()
			} else if constrained {
				count += info.Type.Len()
			}
		default:
			if constrained && isIntegerType(info.Type.Kind()) {
				count++
			}
		}
	}
	return count
}

func isIntegerType(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	default:
		return false
	}
}

func resolvePath(value reflect.Value, path []int) reflect.Value {
	for _, index := range path {
		for value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			value = value.Field(index)
		case reflect.Array, reflect.Slice:
			value = value.Index(index)
		default:
			return reflect.Value{}
		}
	}
	return value
}

func assertUsableDomain(t *testing.T, spec sysex.FieldSpec) {
	t.Helper()
	domain := spec.Domain
	if len(domain.Allowed) > 0 {
		if !domain.Contains(domain.Allowed[0]) {
			t.Fatalf("%s rejects its own first allowed value", spec.Name)
		}
		return
	}
	if domain.Minimum > domain.Maximum {
		t.Fatalf("%s has an inverted domain %v", spec.Name, domain)
	}
	if !domain.Contains(domain.Minimum) || !domain.Contains(domain.Maximum) {
		t.Fatalf("%s rejects its own bounds %v", spec.Name, domain)
	}
}

func TestFieldSpecsOrderIsDeterministic(t *testing.T) {
	first, err := sysex.FieldSpecs(&dx7.SingleVoice{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	second, err := sysex.FieldSpecs(&dx7.SingleVoice{})
	if err != nil {
		t.Fatalf("FieldSpecs: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two walks of the same type disagree")
	}
}
