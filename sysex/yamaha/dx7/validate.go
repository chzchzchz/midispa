package yamaha

import (
	"reflect"
	"strconv"
	"strings"
)

const (
	rangeTag = "range"
	oneofTag = "oneof"
)

// Range tags use inclusive lo..hi intervals and apply elementwise to arrays and slices.
// Oneof tags list the exact permitted scalar values.
func checkTaggedFields(value any) error {
	valueOf := reflect.ValueOf(value)
	for valueOf.Kind() == reflect.Pointer {
		if valueOf.IsNil() {
			return errBadRange
		}
		valueOf = valueOf.Elem()
	}
	if valueOf.Kind() != reflect.Struct {
		return errBadRange
	}

	valueType := valueOf.Type()
	for i := 0; i < valueOf.NumField(); i++ {
		field := valueOf.Field(i)
		fieldInfo := valueType.Field(i)
		if tag, ok := fieldInfo.Tag.Lookup(rangeTag); ok {
			lo, hi, err := parseRangeTag(tag)
			if err != nil || !valueInRange(field, lo, hi) {
				return errBadRange
			}
		}
		if tag, ok := fieldInfo.Tag.Lookup(oneofTag); ok {
			if !valueInSet(field, tag) {
				return errBadRange
			}
		}
	}
	return nil
}

func parseRangeTag(tag string) (int, int, error) {
	parts := strings.Split(tag, "..")
	if len(parts) != 2 {
		return 0, 0, errBadRange
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errBadRange
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil || lo > hi {
		return 0, 0, errBadRange
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
	var current int64
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		current = value.Int()
	default:
		return false
	}
	for _, part := range strings.Split(tag, ",") {
		candidate, err := strconv.Atoi(part)
		if err == nil && current == int64(candidate) {
			return true
		}
	}
	return false
}
