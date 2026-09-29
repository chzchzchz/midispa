package main

import (
	"reflect"
	"strings"
	"testing"
)

// setupMap builds the lookups from each map's target, which is one of
// "control", "control/channel" or "Arm:a,b". The target is split on the first
// slash, so a name containing a later slash keeps everything after it in the
// channel, which Sscanf reads up to the first non-digit.
func TestSetupMapParsesChannelAndArm(t *testing.T) {
	a := Assignments{Maps: [][2]string{
		{"Knob1", "Filter1Cutoff"},
		{"Knob2", "Filter1Cutoff/3"},
		{"Knob3", armPrefix + "Knob1,Knob2"},
		{"Knob4", armPrefix + "*"},
	}}
	if err := a.setupMap(); err != nil {
		t.Fatalf("setupMap: %v", err)
	}

	for _, tc := range []struct {
		in      string
		control string
		channel int
	}{
		{"Knob1", "Filter1Cutoff", 0},
		{"Knob2", "Filter1Cutoff", 3},
		{"Knob3", "", 0}, // arms rather than drives, so it has no target
		{"Knob4", "", 0},
	} {
		control, channel := a.InToOut(tc.in)
		if control != tc.control || channel != tc.channel {
			t.Errorf("InToOut(%q) = (%q, %d), want (%q, %d)",
				tc.in, control, channel, tc.control, tc.channel)
		}
	}

	if got, want := a.in2out["Knob3"].arms, []string{"Knob1", "Knob2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Knob3 arms are %v, want %v", got, want)
	}
	if got, want := a.in2out["Knob4"].arms, []string{"*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Knob4 arms are %v, want %v", got, want)
	}
	if a.in2out["Knob3"].MayArm {
		t.Error("Knob3 arms inputs, so it should not itself be armable")
	}
}

// "Arm:" must arm the inputs named after the prefix. The parser used to keep
// the text before the colon instead, so every "Arm:" entry in
// assignments.drums.json armed the literal name "Arm" and did nothing.
func TestSetupMapArmActivatesNamedInputs(t *testing.T) {
	a := Assignments{Maps: [][2]string{
		{"Knob1", "Filter1Cutoff"},
		{"Knob2", "Filter1Cutoff/3"},
		{"Knob3", armPrefix + "Knob1,Knob2"},
	}}
	if err := a.setupMap(); err != nil {
		t.Fatalf("setupMap: %v", err)
	}
	for _, in := range []string{"Knob1", "Knob2"} {
		if !a.in2out[in].MayArm {
			t.Fatalf("%s is named by an Arm: entry, so it should be armable", in)
		}
	}

	// Selecting a program disarms everything armable, and pressing the Arm
	// input brings the named ones back.
	a.Enable()
	if control, _ := a.InToOut("Knob1"); control != "" {
		t.Fatalf("Knob1 is still routed after Enable, got %q", control)
	}
	if got := a.Arm("Knob3"); !reflect.DeepEqual(got, []string{"Knob1", "Knob2"}) {
		t.Errorf("Arm(Knob3) = %v, want [Knob1 Knob2]", got)
	}
	if control, channel := a.InToOut("Knob1"); control != "Filter1Cutoff" || channel != 0 {
		t.Errorf("Knob1 after Arm is (%q, %d), want (Filter1Cutoff, 0)", control, channel)
	}
}

// "Arm:*" makes every direct mapping armable, which is the wildcard branch
// that a literal "*" argument is meant to reach.
func TestSetupMapArmWildcard(t *testing.T) {
	a := Assignments{Maps: [][2]string{
		{"Knob1", "Filter1Cutoff"},
		{"Knob2", "Filter1Cutoff/3"},
		{"Knob3", armPrefix + "*"},
	}}
	if err := a.setupMap(); err != nil {
		t.Fatalf("setupMap: %v", err)
	}
	for _, in := range []string{"Knob1", "Knob2"} {
		if !a.in2out[in].MayArm {
			t.Errorf("%s should be armable under Arm:*", in)
		}
	}
	if a.in2out["Knob3"].MayArm {
		t.Error("Knob3 arms inputs, so it should not itself be armable")
	}
}

// A target with no slash, an empty target, and one with several slashes all
// have to keep working, since a zero or missing channel is meaningful. A
// trailing slash is reported as an error, see the next test.
func TestSetupMapTargetEdges(t *testing.T) {
	a := Assignments{Maps: [][2]string{
		{"Empty", ""},
		{"Extra", "Control/4/extra"},
		{"Leading", "/7"},
	}}
	if err := a.setupMap(); err != nil {
		t.Fatalf("setupMap: %v", err)
	}
	for _, tc := range []struct {
		in      string
		control string
		channel int
	}{
		{"Empty", "", 0},
		{"Extra", "Control", 4},
		{"Leading", "", 7},
	} {
		control, channel := a.InToOut(tc.in)
		if control != tc.control || channel != tc.channel {
			t.Errorf("InToOut(%q) = (%q, %d), want (%q, %d)",
				tc.in, control, channel, tc.control, tc.channel)
		}
	}
}

// A target ending in "/" leaves an empty channel, and scanning an empty
// string as a number fails. setupMap used to panic there with a bare EOF; it
// now names the offending map so the config file can be found.
func TestSetupMapRejectsEmptyChannel(t *testing.T) {
	a := Assignments{Maps: [][2]string{{"Knob1", "Control/"}}}
	err := a.setupMap()
	if err == nil {
		t.Fatal("a trailing slash was accepted")
	}
	if msg := err.Error(); !strings.Contains(msg, `"Knob1"`) || !strings.Contains(msg, "Control/") {
		t.Errorf("error %q does not identify the offending map", msg)
	}
}
