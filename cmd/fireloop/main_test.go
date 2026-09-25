package main

import (
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
