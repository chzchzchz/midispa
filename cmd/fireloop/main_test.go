package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func testNote(note int) *int {
	return &note
}

func writeKitFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDevicesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `{"Name":"single","MidiPort":"port","Channel":1,"Voices":[{"Name":"voice","Note":60}]}`)

	devices, err := loadDevices(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Name != "single" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestLoadDevicesArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `[
		{"Name":"drums","MidiPort":"port-a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
		{"Name":"lead","MidiPort":"port-b","Channel":1,"Voices":[{"Name":"lead","Channel":3}]}
	]`)

	devices, err := loadDevices(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDevices(devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].MidiPort != "port-a" || devices[1].MidiPort != "port-b" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
	if devices[1].Voices[0].Channel != 3 {
		t.Fatalf("voice channel = %d, want 3", devices[1].Voices[0].Channel)
	}
}

func TestLoadDevicesDirectorySortsByName(t *testing.T) {
	dir := t.TempDir()
	writeKitFile(t, dir, "z.json", `{"Name":"zeta","MidiPort":"z","Channel":1,"Voices":[{"Name":"z","Note":60}]}`)
	writeKitFile(t, dir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":2,"Voices":[{"Name":"a","Note":61}]}`)
	writeKitFile(t, dir, "ignore.txt", "not a kit")

	devices, err := loadDevices(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].Name != "alpha" || devices[1].Name != "zeta" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestLoadKitMergesPathsInOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	writeKitFile(t, dir, "first.json", `[{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]}]`)
	writeKitFile(t, dir, "second.json", `[{"Name":"lead","MidiPort":"b","Channel":1,"Voices":[{"Name":"lead"}]}]`)

	devices, err := loadKit([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDevices(devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].Name != "drums" || devices[1].Name != "lead" {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	reversed, err := loadKit([]string{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(reversed) != 2 || reversed[0].Name != "lead" || reversed[1].Name != "drums" {
		t.Fatalf("kit order not taken from the command line: %+v", reversed)
	}
}

// A merged kit mixes files and directories, and the voices of a single device keep
// their order, because tracks are numbered by position in the merged list.
func TestLoadKitMergesFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	kitDir := filepath.Join(dir, "kit")
	if err := os.Mkdir(kitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKitFile(t, kitDir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":1,"Voices":[{"Name":"one","Note":60},{"Name":"two","Note":61}]}`)
	single := filepath.Join(dir, "single.json")
	writeKitFile(t, dir, "single.json", `{"Name":"solo","MidiPort":"s","Channel":2,"Voices":[{"Name":"solo","Note":62}]}`)

	devices, err := loadKit([]string{kitDir, single})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(devices))
	for _, dev := range devices {
		names = append(names, dev.Name)
	}
	if len(devices) != 2 || names[0] != "alpha" || names[1] != "solo" {
		t.Fatalf("unexpected devices: %v", names)
	}
	if len(devices[0].Voices) != 2 || devices[0].Voices[0].Name != "one" || devices[0].Voices[1].Name != "two" {
		t.Fatalf("voice order changed: %+v", devices[0].Voices)
	}
}

// One bad path fails the whole merge; a partial kit would silently drop voices.
func TestLoadKitRejectsBadPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	writeKitFile(t, dir, "good.json", `{"Name":"good","MidiPort":"g","Channel":1,"Voices":[{"Name":"v","Note":60}]}`)

	if _, err := loadKit([]string{good, filepath.Join(dir, "missing.json")}); err == nil {
		t.Fatal("expected an error for the missing kit")
	}
	if _, err := loadKit(nil); err == nil {
		t.Fatal("expected an error for an empty kit list")
	}
}

// The flag has to accumulate so the same kit can be given more than once on the
// command line, which is what lets several files be merged.
func TestKitPathsFlagAccumulates(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	if err := fs.Parse([]string{"-kit", "a.json", "-kit", "b.json", "-kit", "dir"}); err != nil {
		t.Fatal(err)
	}
	got := paths.all()
	if len(got) != 3 || got[0] != "a.json" || got[1] != "b.json" || got[2] != "dir" {
		t.Fatalf("kit paths = %v", got)
	}
	if paths.String() != "a.json,b.json,dir" {
		t.Fatalf("kit paths string = %q", paths.String())
	}
	if err := fs.Parse([]string{"-kit", ""}); err == nil {
		t.Fatal("expected an error for an empty kit path")
	}
}

// The default kit is only used when no -kit is given. It must not be merged with the
// kits on the command line, which would fail on any directory without a kit.json.
func TestKitPathsFlagReplacesDefault(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	if err := fs.Parse([]string{"-kit", "a.json", "-kit", "b.json"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths.all() {
		if path == defaultKitPath {
			t.Fatalf("default kit %q loaded alongside the given kits: %v", defaultKitPath, paths.all())
		}
	}

	empty := flag.NewFlagSet("empty", flag.ContinueOnError)
	var untouched kitPaths
	empty.Var(&untouched, "kit", "kit")
	if err := empty.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if got := untouched.all(); len(got) != 1 || got[0] != defaultKitPath {
		t.Fatalf("kit paths without any flag = %v, want the default", got)
	}
	if untouched.String() != defaultKitPath {
		t.Fatalf("kit paths string without any flag = %q", untouched.String())
	}
}

func TestValidateDevices(t *testing.T) {
	valid := []Device{{
		Name:     "valid",
		MidiPort: "port",
		Channel:  1,
		Voices:   []Voice{{Name: "voice", Note: testNote(60)}},
	}}
	if err := validateDevices(valid); err != nil {
		t.Fatalf("valid device rejected: %v", err)
	}

	tests := []struct {
		name   string
		device Device
	}{
		{name: "missing port", device: Device{Channel: 1, Voices: []Voice{{Note: testNote(60)}}}},
		{name: "invalid device channel", device: Device{MidiPort: "port", Channel: 17, Voices: []Voice{{Note: testNote(60)}}}},
		{name: "invalid voice channel", device: Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(60), Channel: 17}}}},
		{name: "missing voices", device: Device{MidiPort: "port", Channel: 1}},
		{name: "invalid note", device: Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(128)}}}},
		{name: "missing effective channel", device: Device{MidiPort: "port", Voices: []Voice{{Note: testNote(60)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDevices([]Device{tt.device}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadDevicesRejectsEmptyKit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	writeKitFile(t, dir, "empty.json", "")

	if _, err := loadDevices(path); err == nil {
		t.Fatal("expected an empty kit error")
	}
}
