package sysex

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	// AsciiTag constrains a text field to the inclusive lo..hi interval of
	// character codes a generator may produce. It is deliberately not a wire
	// constraint: a dump read from an instrument may legitimately carry
	// control bytes in a name field, so CheckTaggedFields ignores it and only
	// the valueDomain rules derived here apply it.
	AsciiTag = "ascii"
	// MutateTag marks a field that is not a sound parameter, so a generator
	// must leave it alone. SkipField is its only value, and a skipped field
	// is not enumerated at all rather than enumerated and pinned: it is
	// routing metadata, a label, or a byte the record marks unused, none of
	// which a search can hear and all of which a changed-gene report would
	// only have to explain. It is ignored by CheckTaggedFields for the same
	// reason ascii is, and a field may carry it whatever its type.
	MutateTag = "mutate"
	// SkipField is the MutateTag value that removes a field from the catalog.
	SkipField = "skip"
)

// Domain is the set of values one field may take. Allowed is nil for an
// interval and holds the permitted values for a oneof field, so a caller can
// branch on the constraint kind without re-reading the struct tag.
type Domain struct {
	Minimum int
	Maximum int
	Allowed []int
}

// Contains reports whether value satisfies the domain.
func (domain Domain) Contains(value int) bool {
	if len(domain.Allowed) > 0 {
		for _, allowed := range domain.Allowed {
			if value == allowed {
				return true
			}
		}
		return false
	}
	return value >= domain.Minimum && value <= domain.Maximum
}

// String renders the domain for a diagnostic so an error can quote the
// constraint the field actually carries.
func (domain Domain) String() string {
	if len(domain.Allowed) > 0 {
		parts := make([]string, 0, len(domain.Allowed))
		for _, allowed := range domain.Allowed {
			parts = append(parts, strconv.Itoa(allowed))
		}
		return strings.Join(parts, ",")
	}
	return fmt.Sprintf("%d..%d", domain.Minimum, domain.Maximum)
}

// FieldSpec is one addressable integer field of a decoded program. Path is a
// sequence of struct field indexes and collection indexes from the root, so
// traversal stays stable when two nested structs share a field name.
type FieldSpec struct {
	Path   []int
	Name   string
	Domain Domain
}

// FieldSpecs enumerates every constrained integer field reachable from root,
// which must be a pointer to a struct.
//
// A field is only a candidate if it carries a domain tag, holds an integer and
// is not tagged mutate:"skip". A vendor record also holds text, flags and
// reserved bytes, and a field that is routing metadata or a label rather than
// a sound parameter says so itself, so all three are declared on the struct
// rather than guessed here. A kind that could never be a message parameter at
// all is still a descriptive error.
//
// Embedded structs are flattened and contribute no name segment, matching how
// a one-voice message nests its voice fields inside the program wrapper.
func FieldSpecs(root any) ([]FieldSpec, error) {
	value, err := settableStruct(root)
	if err != nil {
		return nil, err
	}
	var specs []FieldSpec
	if err := appendFieldSpecs(&specs, value, nil, ""); err != nil {
		return nil, err
	}
	return specs, nil
}

func settableStruct(root any) (reflect.Value, error) {
	value := reflect.ValueOf(root)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("%w: nil program pointer", ErrBadRange)
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%w: program is a %s, not a struct", ErrBadRange, value.Kind())
	}
	if !value.CanSet() {
		return reflect.Value{}, fmt.Errorf("%w: program is not settable", ErrBadRange)
	}
	return value, nil
}

func appendFieldSpecs(specs *[]FieldSpec, value reflect.Value, path []int, prefix string) error {
	structType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		info := structType.Field(index)
		if !info.IsExported() {
			continue
		}
		field := value.Field(index)
		name := prefix + info.Name
		skip, err := skipMutation(info)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if skip {
			continue
		}
		domain, constrained, err := fieldDomain(info)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		switch field.Kind() {
		case reflect.Struct:
			// An embedded field is part of the enclosing value, so its fields
			// keep the shorter name a user would type.
			childPrefix := name + "."
			if info.Anonymous {
				childPrefix = prefix
			}
			if err := appendFieldSpecs(specs, field, extendPath(path, index), childPrefix); err != nil {
				return err
			}
		case reflect.Array, reflect.Slice:
			if err := appendCollectionSpecs(specs, field, extendPath(path, index), name, domain, constrained); err != nil {
				return err
			}
		default:
			if isReferenceKind(field.Kind()) {
				return fmt.Errorf("%s: unsupported field kind %s", name, field.Kind())
			}
			if !constrained || !isIntegerKind(field.Kind()) {
				continue
			}
			*specs = append(*specs, FieldSpec{Path: extendPath(path, index), Name: name, Domain: domain})
		}
	}
	return nil
}

// appendCollectionSpecs expands an array or slice. A collection of structs is
// traversed per element; a collection of integers becomes one spec per element
// sharing the field's domain, because a oneof or range applies elementwise.
func appendCollectionSpecs(specs *[]FieldSpec, field reflect.Value, path []int, name string, domain Domain, constrained bool) error {
	for index := 0; index < field.Len(); index++ {
		element := field.Index(index)
		elementName := fmt.Sprintf("%s[%d]", name, index)
		elementPath := append(copyPath(path), index)
		if element.Kind() == reflect.Struct {
			if err := appendFieldSpecs(specs, element, elementPath, elementName+"."); err != nil {
				return err
			}
			continue
		}
		if isReferenceKind(element.Kind()) {
			return fmt.Errorf("%s: unsupported element kind %s", elementName, element.Kind())
		}
		if !isIntegerKind(element.Kind()) {
			continue
		}
		if !constrained {
			continue
		}
		*specs = append(*specs, FieldSpec{Path: elementPath, Name: elementName, Domain: domain})
	}
	return nil
}

func extendPath(path []int, index int) []int {
	return append(copyPath(path), index)
}

func copyPath(path []int) []int {
	copied := make([]int, len(path))
	copy(copied, path)
	return copied
}

// isReferenceKind reports the kinds that can never be a bounded program
// parameter: a pointer cannot even be traversed, and a map, channel, function
// or complex value has nothing a message could carry. A string, flag or float
// is deliberately not here, because a vendor record can legitimately hold one
// and the format is what decides whether it is a sound parameter.
func isReferenceKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

func isIntegerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	default:
		return false
	}
}

// skipMutation reads the one value of the mutation tag, so that a typo fails
// loudly instead of quietly leaving a field evolvable.
func skipMutation(info reflect.StructField) (bool, error) {
	tag, ok := info.Tag.Lookup(MutateTag)
	if !ok {
		return false, nil
	}
	if tag != SkipField {
		return false, fmt.Errorf("%w: malformed %s tag %q, expected %q", ErrBadRange, MutateTag, tag, SkipField)
	}
	return true, nil
}

// fieldDomain reads the constraint a field's tags declare. A oneof alongside
// any other domain tag is rejected because it describes a different kind of
// constraint and honoring one would hide the mistake in the struct tag. When
// both range and ascii are present the printable interval wins here, because
// this domain is what a generator may produce; range keeps its looser meaning
// for the wire, where a dump may legitimately carry control bytes.
func fieldDomain(info reflect.StructField) (Domain, bool, error) {
	rangeTag, hasRange := info.Tag.Lookup(RangeTag)
	oneofTag, hasOneof := info.Tag.Lookup(OneofTag)
	asciiTag, hasAscii := info.Tag.Lookup(AsciiTag)
	if hasOneof && (hasRange || hasAscii) {
		return Domain{}, false, fmt.Errorf("%w: field carries both %s and another domain tag", ErrBadRange, OneofTag)
	}
	switch {
	case hasOneof:
		allowed, err := parseOneofTag(oneofTag)
		if err != nil {
			return Domain{}, false, fmt.Errorf("%w: malformed %s tag %q", ErrBadRange, OneofTag, oneofTag)
		}
		return Domain{Minimum: allowed[0], Maximum: allowed[len(allowed)-1], Allowed: allowed}, true, nil
	case hasAscii:
		minimum, maximum, err := parseRangeTag(asciiTag)
		if err != nil {
			return Domain{}, false, fmt.Errorf("%w: malformed %s tag %q", ErrBadRange, AsciiTag, asciiTag)
		}
		return Domain{Minimum: minimum, Maximum: maximum}, true, nil
	case hasRange:
		minimum, maximum, err := parseRangeTag(rangeTag)
		if err != nil {
			return Domain{}, false, fmt.Errorf("%w: malformed %s tag %q", ErrBadRange, RangeTag, rangeTag)
		}
		return Domain{Minimum: minimum, Maximum: maximum}, true, nil
	}
	return Domain{}, false, nil
}
