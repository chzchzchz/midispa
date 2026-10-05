package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/stretchr/testify/require"
)

// This file captures what the sequencer logs so a scripted sequence can be read back the
// same way a -log-level debug run reads on the terminal. It also carries the seams a
// tracer needs to wait for something to happen, or to make a timeout fire quickly.

// waitTimeout bounds every tracer wait, so a stuck sequencer fails the test instead of
// hanging it.
const waitTimeout = 2 * time.Second

// waitFor polls until cond holds or waitTimeout passes. It replaces a fixed sleep, which
// is either flaky when too short or slow when too long.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s", waitTimeout, what)
		}
		time.Sleep(time.Millisecond)
	}
}

// captureLog keeps every record so a test can assert on what a sequence logged.
type captureLog struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *captureLog) Enabled(context.Context, slog.Level) bool { return true }

func (l *captureLog) Handle(_ context.Context, record slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, record.Clone())
	return nil
}

func (l *captureLog) WithAttrs([]slog.Attr) slog.Handler { return l }

func (l *captureLog) WithGroup(string) slog.Handler { return l }

// useCaptureLog installs a capturing logger for the duration of a test.
func useCaptureLog(t *testing.T) *captureLog {
	t.Helper()
	capture := &captureLog{}
	previous := logger
	setLogger(slog.New(capture))
	t.Cleanup(func() { setLogger(previous) })
	return capture
}

// attr reads one attribute from a record, or an empty string when it is absent.
func attr(record slog.Record, key string) string {
	found := ""
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			found = a.Value.String()
			return false
		}
		return true
	})
	return found
}

// outbound renders the "midi out" records as "note on 60 100" lines, in the order they
// were written. That is the sequence a synth actually sees.
func (l *captureLog) outbound() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	for _, record := range l.records {
		if record.Message != "midi out" {
			continue
		}
		lines = append(lines, strings.TrimSpace(
			attr(record, "kind")+" "+attr(record, "note")+" "+attr(record, "velocity")))
	}
	return lines
}

// messages returns the log messages written so far, which is how a test finds a step.
func (l *captureLog) messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, record := range l.records {
		out = append(out, record.Message)
	}
	return out
}

// walkPattern plays a pattern one step at a time, the way the sequencer does, and then
// releases whatever is still sounding.
func walkPattern(t *testing.T, writer alsa.EventWriter, pattern *Pattern) *Playback {
	t.Helper()
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < pattern.LengthSteps(); step++ {
		playback.setPosition(0, stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
	}
	require.NoError(t, playback.releaseAll(writer))
	return playback
}

// The log has to show a note leaving as well as arriving. Each case pins the exact
// sequence a synth would receive.
func TestTraceChromaticNoteLifecycle(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	tests := []struct {
		name  string
		steps []int
		pitch []int
		tie   bool
		want  []string
	}{
		{
			name:  "one note is released",
			steps: []int{0}, pitch: []int{60},
			want: []string{"note on 60 100", "note off 60 0"},
		},
		{
			name:  "untied notes retrigger",
			steps: []int{0, 4}, pitch: []int{60, 62},
			want: []string{"note on 60 100", "note off 60 0", "note on 62 100", "note off 62 0"},
		},
		{
			name:  "untied same pitch retriggers",
			steps: []int{0, 4}, pitch: []int{60, 60},
			want: []string{"note on 60 100", "note off 60 0", "note on 60 100", "note off 60 0"},
		},
		{
			// A tie holds the note, so the second event does not speak again.
			name:  "tied same pitch holds",
			steps: []int{0, 4}, pitch: []int{60, 60}, tie: true,
			want: []string{"note on 60 100", "note off 60 0"},
		},
		{
			// A pitch change is legato: the new note starts before the old one ends.
			name:  "tied pitch change overlaps",
			steps: []int{0, 4}, pitch: []int{60, 62}, tie: true,
			want: []string{"note on 60 100", "note on 62 100", "note off 60 0", "note off 62 0"},
		},
		{
			// A note near the end must still be released rather than left sounding.
			name:  "late note is released",
			steps: []int{12}, pitch: []int{60},
			want: []string{"note on 60 100", "note off 60 0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &Pattern{}
			for i, step := range tt.steps {
				pattern.SetChromaticNote(step, voice, tt.pitch[i], 100)
			}
			if tt.tie {
				pattern.TieEventsAtSteps(tt.steps[0], tt.steps[1], voice)
			}
			log := useCaptureLog(t)
			walkPattern(t, &captureMidiWriter{}, pattern)
			got := log.outbound()
			t.Logf("outbound: %v", got)
			require.Len(t, got, len(tt.want))
			for i, line := range got {
				require.Equalf(t, tt.want[i], line, "outbound[%d]", i)
			}
		})
	}
}

// A note-on that cannot be paired with a note-off leaves the instrument hanging, so the
// sounding note is recorded before anything is written.
func TestTraceFailedWriteStillTracksTheNote(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(4, voice, 62, 100)
	pattern.TieEventsAtSteps(0, 4, voice)
	useCaptureLog(t)
	writer := &failingWriter{failAfter: 1}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	playback.setPosition(0, 0)
	_, err := playback.playBeat(writer, pattern)
	require.NoError(t, err)
	// The second write fails, which is the note-off half of the legato pair.
	playback.setPosition(0, stepBeat(4))
	_, err = playback.playBeat(writer, pattern)
	require.Error(t, err, "the failing write was not reported")
	require.Equal(t, 1, playback.activeNoteCount(),
		"the note that was written is not tracked, so nothing would release it")
}

// failingWriter refuses writes past a count, standing in for a full MIDI buffer.
type failingWriter struct {
	captureMidiWriter
	written   int
	failAfter int
}

func (w *failingWriter) Write(event alsa.SeqEvent) error {
	w.written++
	if w.written > w.failAfter {
		return errTestWrite
	}
	return w.captureMidiWriter.Write(event)
}

var errTestWrite = errors.New("test write failure")

// A tap outside the window starts a new tempo instead of joining the old one, and the test
// reaches that path in milliseconds rather than three seconds.
func TestTraceTapTempoWindowResets(t *testing.T) {
	_, patternBank, _ := useTestBanks(t)
	previousWindow, previousBPM := tapTempoWindow, currentBPM()
	t.Cleanup(func() {
		tapTempoWindow = previousWindow
		setBPM(previousBPM)
	})
	tapTempoWindow = 5 * time.Millisecond
	setBPM(120)

	pressButton(t, patternBank, NoteTap)
	waitFor(t, "a tap to be recorded", func() bool { return len(patternBank.controller.tapTimes) == 1 })
	time.Sleep(4 * tapTempoWindow)
	pressButton(t, patternBank, NoteTap)
	// The window passed, so the earlier tap was discarded rather than averaged in.
	require.Equal(t, 120, currentBPM(), "the discarded tap must leave the tempo alone")
	require.Len(t, patternBank.controller.tapTimes, 1, "the window should have reset the tap history to one")
	_ = patternBank
}

// A note is one step long unless a tie holds it, so it stops where it started instead of
// ringing until the pattern ends. This is the case a lone note used to get wrong.
func TestTraceNoteStopsAfterOneStep(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	log := useCaptureLog(t)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	// Only the first two steps are visited, so a note-off here cannot have come from the
	// pattern boundary or from a later event.
	for _, step := range []int{0, 1} {
		playback.setPosition(0, stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
	}
	got := log.outbound()
	t.Logf("outbound: %v", got)
	want := []string{"note on 60 100", "note off 60 0"}
	require.Len(t, got, len(want))
	for i, line := range got {
		require.Equalf(t, want[i], line, "outbound[%d]", i)
	}
	require.Zero(t, playback.activeNoteCount(), "the note is still sounding after its step")
}

// A tie is the way to hold a note past its step, so the gate must not cut it short.
func TestTraceTieHoldsPastItsStep(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(8, voice, 60, 100)
	pattern.TieEventsAtSteps(0, 8, voice)
	log := useCaptureLog(t)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step <= 4; step++ {
		playback.setPosition(0, stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
	}
	require.Equal(t, 1, playback.activeNoteCount(), "the tied note stopped at the end of its own step")
	require.Equal(t, []string{"note on 60 100"}, log.outbound(), "only the opening note is written while tied")
	// The tied event ends the hold.
	playback.setPosition(0, stepBeat(8))
	_, err := playback.playBeat(writer, pattern)
	require.NoError(t, err)
	playback.setPosition(0, stepBeat(9))
	_, err = playback.playBeat(writer, pattern)
	require.NoError(t, err)
	got := log.outbound()
	t.Logf("outbound: %v", got)
	want := []string{"note on 60 100", "note off 60 0"}
	require.Len(t, got, len(want))
	for i, line := range got {
		require.Equalf(t, want[i], line, "outbound[%d]", i)
	}
}

// The log format is a choice, not a hardcoded handler. JSON is selected by name and
// anything unrecognised falls back to text rather than leaving the sequencer silent.
func TestLogFormatChoosesTheHandler(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{format: "text", want: "level=INFO"},
		{format: "json", want: `{"time":`},
		{format: "JSON", want: `{"time":`},
		{format: "nonsense", want: "level=INFO"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var out bytes.Buffer
			logger := slog.New(newLogHandler(&out, slog.LevelInfo, tt.format))
			logger.Info("midi out", "kind", "note on", "note", 60, "velocity", 100)
			line := out.String()
			require.Containsf(t, line, tt.want, "format %q", tt.format)
			// Both shapes carry the same fields, so a reader sees the same facts.
			require.Containsf(t, line, "note on", "format %q dropped a field", tt.format)
			require.Containsf(t, line, "60", "format %q dropped a field", tt.format)
		})
	}
}
