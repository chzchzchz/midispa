package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/chzchzchz/midispa/sysex"
)

const (
	dx7SingleFormatName = "dx7-single"
	pro800FormatName    = "pro800"
)

// SysexPatch is one decoded vendor message. The gene values are the source of
// truth and the decoded program is scratch space, written only while
// encoding, so a parent and the candidates mutated from it cannot alias.
type SysexPatch struct {
	format    sysexFormat
	semantics map[string]geneSemantic
	root      any
	specs     []sysex.FieldSpec
	patchGenes
}

// newSysexPatch reads every constrained field of a decoded program into a gene
// list. Names and bounds come from reflection over the program's own struct
// tags, so no DX7 field name appears in the mutation code.
func newSysexPatch(format sysexFormat, root any, semantics map[string]geneSemantic) (*SysexPatch, error) {
	specs, err := sysex.FieldSpecs(root)
	if err != nil {
		return nil, err
	}
	genes := make([]gene, 0, len(specs))
	tracked := make([]sysex.FieldSpec, 0, len(specs))
	known := make(map[string]bool, len(specs))
	for _, spec := range specs {
		known[spec.Name] = true
		if semantics[spec.Name].Policy == "exclude" {
			continue
		}
		value, err := geneValue(root, spec)
		if err != nil {
			return nil, err
		}
		genes = append(genes, newGene(spec, value, semantics[spec.Name]))
		tracked = append(tracked, spec)
	}
	for name := range semantics {
		if !known[name] {
			return nil, fmt.Errorf("gene %q is not a field of format %q", name, format.ID())
		}
	}
	return &SysexPatch{
		format:    format,
		semantics: semantics,
		root:      root,
		specs:     tracked,
		patchGenes: patchGenes{
			format: format.ID(),
			genes:  genes,
		},
	}, nil
}

// newGene seeds one gene from a decoded value. A fixed rule with a value pins
// that value and a fixed rule without one keeps the seeded value, which is the
// only source a SysEx gene can be seeded from. A field the record marks as not
// a sound parameter never reaches here: the walker leaves it out of the
// catalog, so the vendor declares that on its own struct.
func newGene(spec sysex.FieldSpec, value int, rule geneSemantic) gene {
	current := gene{
		name:   spec.Name,
		value:  value,
		policy: genePolicyMutable,
		domain: newValueDomain(spec.Domain),
	}
	if rule.Policy == "fixed" {
		current.policy = genePolicyFixed
		if rule.Value != nil {
			current.value = *rule.Value
		}
	}
	return current
}

func (candidate *SysexPatch) clone() patch {
	return &SysexPatch{
		format:     candidate.format,
		semantics:  candidate.semantics,
		root:       candidate.root,
		specs:      candidate.specs,
		patchGenes: *candidate.cloneGenes(),
	}
}

// loadMessage replaces the decoded program and every gene value, because a
// SysEx seed is a whole patch rather than a set of controller updates. Only
// those two change: the catalog is a property of the program's type, and the
// semantics were applied when the patch was built.
func (candidate *SysexPatch) loadMessage(message []byte) error {
	root, err := candidate.format.Decode(message)
	if err != nil {
		return err
	}
	loaded, err := newSysexPatch(candidate.format, root, candidate.semantics)
	if err != nil {
		return err
	}
	candidate.root = loaded.root
	candidate.genes = loaded.genes
	return nil
}

// encodable asks the codec whether the current values have anywhere to go in
// its message, which is the only authority on that: a record may be wider
// than the dump it is written as.
func (candidate *SysexPatch) encodable() error {
	_, err := candidate.encode(0)
	return err
}

// encode writes the gene values into a copy of the decoded program and lets
// the codec validate the result, so a value outside what the instrument
// accepts fails here instead of being silently clamped by hardware. The
// channel argument is unused: the format owns routing and applies it while
// encoding, so a patch is not tied to the channel it happens to be auditioned
// on.
func (candidate *SysexPatch) encode(_ int) ([][]byte, error) {
	program, err := copyRoot(candidate.root)
	if err != nil {
		return nil, err
	}
	for index, spec := range candidate.specs {
		if err := setGeneValue(program, spec, candidate.genes[index].value); err != nil {
			return nil, err
		}
	}
	return candidate.format.Encode(program)
}

func copyRoot(root any) (any, error) {
	value := reflect.ValueOf(root)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return nil, fmt.Errorf("program root is not a settable pointer, got %T", root)
	}
	clone := reflect.New(value.Type().Elem())
	clone.Elem().Set(value.Elem())
	return clone.Interface(), nil
}

// fieldAt follows a spec path, so a field is reached by position rather than
// by a name lookup that would be repeated for every mutation.
func fieldAt(value reflect.Value, path []int) reflect.Value {
	for _, index := range path {
		for value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			value = value.Field(index)
		case reflect.Array, reflect.Slice:
			value = value.Index(index)
		}
	}
	return value
}

func geneValue(root any, spec sysex.FieldSpec) (int, error) {
	value := fieldAt(reflect.ValueOf(root), spec.Path)
	switch {
	case isSignedKind(value.Kind()):
		return int(value.Int()), nil
	case isUnsignedKind(value.Kind()):
		return int(value.Uint()), nil
	}
	return 0, fmt.Errorf("field %q is not an integer", spec.Name)
}

func setGeneValue(root any, spec sysex.FieldSpec, value int) error {
	target := fieldAt(reflect.ValueOf(root), spec.Path)
	if !target.CanSet() {
		return fmt.Errorf("field %q is not settable", spec.Name)
	}
	switch {
	case isSignedKind(target.Kind()):
		target.SetInt(int64(value))
	case isUnsignedKind(target.Kind()):
		target.SetUint(uint64(value))
	default:
		return fmt.Errorf("field %q is not an integer", spec.Name)
	}
	return nil
}

func isSignedKind(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

func isUnsignedKind(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}

// sysexPatchFactory builds patches for one SysEx format. A SysEx run has no
// meaningful random starting point, so it always requires a seed.
type sysexPatchFactory struct {
	format sysexFormat
}

func newSysexPatchFactory(name string, midiChannel int) (patchFactory, error) {
	switch name {
	case dx7SingleFormatName:
		return sysexPatchFactory{format: newDX7Format(midiChannel - 1)}, nil
	case pro800FormatName:
		return sysexPatchFactory{format: newPro800Format()}, nil
	default:
		return nil, fmt.Errorf("unsupported --format %q, expected %s, %s or %s", name, ccFormatName, dx7SingleFormatName, pro800FormatName)
	}
}

func (factory sysexPatchFactory) newPatch(semantics map[string]geneSemantic) (patch, error) {
	return newSysexPatch(factory.format, factory.format.NewRoot(), semantics)
}

func (factory sysexPatchFactory) loadSeed(path string, target patch) error {
	candidate, err := asSysexPatch(target)
	if err != nil {
		return err
	}
	message, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read seed %q: %w", path, err)
	}
	if err := candidate.loadMessage(message); err != nil {
		return fmt.Errorf("read seed %q: %w", path, err)
	}
	return nil
}

func (factory sysexPatchFactory) store(outputPath string) patchStore {
	return sysexPatchStore{outputPath: outputPath}
}

func (factory sysexPatchFactory) requiresSeed() bool { return true }

func (factory sysexPatchFactory) supportsJSONDump() bool { return false }

func (factory sysexPatchFactory) acceptsModelName() bool { return false }

func (factory sysexPatchFactory) outputExtension() string {
	return factory.format.OutputExtension()
}

func asSysexPatch(target patch) (*SysexPatch, error) {
	sysexPatch, ok := target.(*SysexPatch)
	if !ok {
		return nil, fmt.Errorf("SysEx handling requires a SysEx patch, got %s", target.formatID())
	}
	return sysexPatch, nil
}
