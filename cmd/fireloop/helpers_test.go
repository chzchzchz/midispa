package main

import "testing"

// The setup every test starts from: a kit, a bank on it with the package globals pointed at
// it, and pattern 1 selected. Almost every test needs exactly that, so the three variants
// below differ only in where the display goes. A test that reads pads, lights or text asks
// for a recording; one that only inspects state discards them.

// chromaBank is the bank a test about pitch starts from: one chromatic voice, pattern 1, and
// the globals pointed at it. Track and percussive tests bring their own kit through quietBank.
func chromaBank(t *testing.T) (*PatternBank, *Voice) {
	t.Helper()
	return chromaBankOn(t, func([]byte) error { return nil })
}

// chromaBankOn is chromaBank with a display the test supplies, for the few tests that count
// what Fireloop writes to the unit.
func chromaBankOn(t *testing.T, write writeFunc) (*PatternBank, *Voice) {
	t.Helper()
	kit := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead", Channel: 1}}}})
	bank := newTestBank(t, NewFire(write), kit)
	return bank, kit.voices[0]
}

// quietBank discards display writes. It returns the kit as well as the bank, because a test
// that names a voice by index reads it from here rather than rebuilding the kit.
func quietBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank) {
	t.Helper()
	bank := quietBankOn(t, kit, func([]byte) error { return nil })
	return bank, kit
}

// quietBankOn is quietBank with a display the test supplies, for the tests that watch which
// buttons light rather than what the pads show.
func quietBankOn(t *testing.T, kit *VoiceBank, write writeFunc) *PatternBank {
	t.Helper()
	return newTestBank(t, NewFire(write), kit)
}

// recordedBank keeps the whole display: pad colours, button lights and a note of how many
// times the screen was wiped, which is how a blackout is told from a readout.
func recordedBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank, *fireSim) {
	t.Helper()
	sim := newFireSim()
	bank := newTestBank(t, NewFire(sim.write), kit)
	return bank, kit, sim
}

// screenBank records only the OLED text, for the tests that read what a row says and have no
// reason to look at a pad.
func screenBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank, *screenRecorder) {
	t.Helper()
	bank, kit := quietBank(t, kit)
	return bank, kit, useScreenRecorder(t, bank)
}

// newTestBank is the part every bank in a test shares.
func newTestBank(t *testing.T, fire *Fire, kit *VoiceBank) *PatternBank {
	t.Helper()
	bank := NewPatternBank(fire, kit)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	return bank
}
