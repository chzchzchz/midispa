package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/chzchzchz/midispa/sysex"
	dx7 "github.com/chzchzchz/midispa/sysex/yamaha/dx7"
)

const dx7SingleFormatName = "dx7-single"

// sysexFormat is the vendor boundary for one SysEx message kind. It owns the
// root type, the wire codec, and the policy that says which decoded fields are
// metadata rather than sound parameters. The engine only ever sees reflected
// gene values, so a new vendor is a new implementation of this interface
// rather than a new patch type.
type sysexFormat interface {
	ID() string
	NewRoot() any
	Decode(message []byte) (any, error)
	Encode(root any) ([][]byte, error)
	Immutable(path string) bool
	// OutputExtension is the suffix a raw dump of this message kind uses.
	OutputExtension() string
}

// dx7Format speaks the 155-byte one-voice edit-buffer message. A bulk dump
// would overwrite all 32 internal voices, so this format cannot emit one.
type dx7Format struct {
	channel int // zero-based, applied at decode and at encode
}

func (dx7Format) ID() string { return dx7SingleFormatName }

func (dx7Format) OutputExtension() string { return ".syx" }

func (dx7Format) NewRoot() any { return &dx7.SingleVoice{} }

// Decode validates one complete vendor message. The exact-length check in
// UnmarshalBinary is also what rejects a file holding trailing bytes or a
// second message.
func (format dx7Format) Decode(message []byte) (any, error) {
	var voice dx7.SingleVoice
	if err := voice.UnmarshalBinary(message); err != nil {
		return nil, err
	}
	// The channel is routing, not a sound parameter, so the configured
	// channel wins over whatever the dump was captured on.
	voice.Channel = format.channel
	return &voice, nil
}

// Encode returns exactly one message. A bulk dump would erase the other 31
// voices in the instrument, so the single-message result is a safety property
// and not only an API shape.
func (format dx7Format) Encode(root any) ([][]byte, error) {
	voice, ok := root.(*dx7.SingleVoice)
	if !ok {
		return nil, fmt.Errorf("format %s expects a one-voice program, got %T", format.ID(), root)
	}
	clone := *voice
	clone.Channel = format.channel
	message, err := clone.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return [][]byte{message}, nil
}

// Immutable lists the decoded fields that are not sound parameters. They keep
// their seeded value and never enter the mutation budget, while remaining
// addressable so a fixed rule can still pin one deliberately. The channel is
// routing metadata, a name is text, and the operator-on field belongs to no
// DX7 wire layout, so changing it would report a difference the instrument
// never hears.
func (dx7Format) Immutable(path string) bool {
	switch {
	case path == "Channel", path == "OperatorOn":
		return true
	case strings.HasPrefix(path, "VoiceName["):
		return true
	}
	return false
}

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
		genes = append(genes, newGene(spec, value, format.Immutable(spec.Name), semantics[spec.Name]))
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
// only source a SysEx gene can be seeded from. A field the format calls
// metadata is pinned to its seeded value instead, which is a different reason
// to be unchangeable and is kept distinct for that reason.
func newGene(spec sysex.FieldSpec, value int, immutable bool, rule geneSemantic) gene {
	current := gene{
		name:   spec.Name,
		value:  value,
		policy: genePolicyMutable,
		domain: newValueDomain(spec.Domain),
	}
	switch {
	case rule.Policy == "fixed":
		current.policy = genePolicyFixed
		if rule.Value != nil {
			current.value = *rule.Value
		}
	case immutable:
		current.policy = genePolicyImmutable
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
// SysEx seed is a whole patch rather than a set of controller updates.
func (candidate *SysexPatch) loadMessage(message []byte) error {
	root, err := candidate.format.Decode(message)
	if err != nil {
		return err
	}
	loaded, err := newSysexPatch(candidate.format, root, candidate.semantics)
	if err != nil {
		return err
	}
	*candidate = *loaded
	return nil
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
		return sysexPatchFactory{format: dx7Format{channel: midiChannel - 1}}, nil
	default:
		return nil, fmt.Errorf("unsupported --format %q, expected %s or %s", name, ccFormatName, dx7SingleFormatName)
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
