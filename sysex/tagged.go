package sysex

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const (
	// RangeTag constrains a field to the inclusive lo..hi interval it holds,
	// and applies elementwise to arrays and slices.
	RangeTag = "range"
	// OneofTag constrains a scalar field to the exact permitted values it lists.
	OneofTag = "oneof"
)

// CheckTaggedFields validates every RangeTag and OneofTag on the struct pointed
// to by value, including the structs it nests by value. Vendor packages use it
// to reject a program whose parameters are outside the ranges the instrument
// itself would accept, so that a dump cannot be re-encoded into something the
// hardware silently clamps.
func CheckTaggedFields(value any) error {
	valueOf := reflect.ValueOf(value)
	for valueOf.Kind() == reflect.Pointer {
		if valueOf.IsNil() {
			return ErrBadRange
		}
		valueOf = valueOf.Elem()
	}
	if valueOf.Kind() != reflect.Struct {
		return ErrBadRange
	}

	valueType := valueOf.Type()
	for i := 0; i < valueOf.NumField(); i++ {
		field := valueOf.Field(i)
		fieldInfo := valueType.Field(i)
		if tag, ok := fieldInfo.Tag.Lookup(RangeTag); ok {
			lo, hi, err := parseRangeTag(tag)
			if err != nil {
				return fmt.Errorf("%w: %s has a malformed %s tag %q", ErrBadRange, fieldInfo.Name, RangeTag, tag)
			}
			if !valueInRange(field, lo, hi) {
				return fmt.Errorf("%w: %s is outside %s", ErrBadRange, fieldInfo.Name, tag)
			}
		}
		if tag, ok := fieldInfo.Tag.Lookup(OneofTag); ok {
			if !valueInSet(field, tag) {
				return fmt.Errorf("%w: %s is outside %s", ErrBadRange, fieldInfo.Name, tag)
			}
		}
		if err := checkNestedFields(field, fieldInfo.IsExported()); err != nil {
			return err
		}
	}
	return nil
}

// checkNestedFields validates a nested struct, and structs held in an array or
// a slice, on their own terms. A program is mostly a voice, and a tag on the
// program says nothing about whether the voice inside it is in range, so
// stopping at the top level would validate the wrapper and none of the data.
func checkNestedFields(value reflect.Value, exported bool) error {
	if !exported || !value.CanInterface() {
		return nil
	}
	switch value.Kind() {
	case reflect.Struct:
		if !value.CanAddr() {
			return nil
		}
		return CheckTaggedFields(value.Addr().Interface())
	case reflect.Array, reflect.Slice:
		if value.Type().Elem().Kind() != reflect.Struct {
			return nil
		}
		for i := 0; i < value.Len(); i++ {
			if err := checkNestedFields(value.Index(i), exported); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseRangeTag(tag string) (int, int, error) {
	parts := strings.Split(tag, "..")
	if len(parts) != 2 {
		return 0, 0, ErrBadRange
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, ErrBadRange
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil || lo > hi {
		return 0, 0, ErrBadRange
	}
	return lo, hi, nil
}

func valueInRange(value reflect.Value, lo, hi int) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if !valueInRange(value.Index(i), lo, hi) {
				return false
			}
		}
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		current := value.Int()
		return current >= int64(lo) && current <= int64(hi)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		current := value.Uint()
		return lo >= 0 && current >= uint64(lo) && current <= uint64(hi)
	default:
		return false
	}
}

func valueInSet(value reflect.Value, tag string) bool {
	allowed, err := parseOneofTag(tag)
	if err != nil {
		return false
	}
	return valueInAllowed(value, allowed)
}

// valueInAllowed applies the elementwise rule valueInRange already uses: a
// oneof tag on an array or a slice constrains each element, not the collection
// as a whole.
func valueInAllowed(value reflect.Value, allowed []int) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if !valueInAllowed(value.Index(i), allowed) {
				return false
			}
		}
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		current := value.Int()
		for _, candidate := range allowed {
			if current == int64(candidate) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// parseOneofTag returns the permitted values in ascending order so a caller can
// treat the set as a domain and report it deterministically.
func parseOneofTag(tag string) ([]int, error) {
	parts := strings.Split(tag, ",")
	allowed := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		candidate, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, ErrBadRange
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		allowed = append(allowed, candidate)
	}
	if len(allowed) == 0 {
		return nil, ErrBadRange
	}
	sort.Ints(allowed)
	return allowed, nil
}
