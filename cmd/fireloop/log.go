package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
)

// logger carries structured events from the sequencer: every MIDI message it writes, the
// playback lifecycle, and the UI transitions worth following. Tests replace it so a
// scripted sequence can be read back as a log.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// setLogger installs the handler for the rest of the process. A nil logger is ignored so
// a caller cannot silence the sequencer by accident.
func setLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// newLogger builds the command's handler at the named level and format.
func newLogger(level slog.Level, format string) *slog.Logger {
	return slog.New(newLogHandler(os.Stderr, level, format))
}

// newLogHandler picks the handler for a level and format. JSON is easier to read back with
// a tool, text is easier to read by eye. An unrecognised format falls back to text rather
// than leaving the sequencer silent.
func newLogHandler(w io.Writer, level slog.Level, format string) slog.Handler {
	options := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(format, logFormatJSON) {
		return slog.NewJSONHandler(w, options)
	}
	return slog.NewTextHandler(w, options)
}

const logFormatJSON = "json"

// voiceLabel names a voice for a log line, including the channel it plays on, so a note
// can be traced back to the kit entry that produced it.
func voiceLabel(v *Voice) string {
	if v == nil {
		return "none"
	}
	kind := "drum"
	if v.IsChromatic() {
		kind = "chromatic"
	}
	return v.Name + "/" + kind
}

func ledName(control int) string {
	switch control {
	case NoteAlt:
		return "Alt"
	case NoteShift:
		return "Shift"
	case NoteMode:
		return "Mode"
	case NoteOverview:
		return "Overview"
	case NoteRecord:
		return "Record"
	case NotePatternSong:
		return "PatternSong"
	case CCMuteLED1:
		return "Mute1"
	case CCMuteLED2:
		return "Mute2"
	case CCMuteLED3:
		return "Mute3"
	case CCMuteLED4:
		return "Mute4"
	}
	return fmt.Sprintf("cc%d", control)
}

// logIncoming records a control press that is not a grid pad. The Fire's buttons send
// plain note-ons, so pressing an unbound button and reading its number off the log is how
// a new control is identified without guessing.
func logIncoming(note, status, velocity int) {
	if _, _, onGrid := Note2Grid(note); onGrid {
		return
	}
	kind, _, _, _ := logMIDI([]byte{byte(status), byte(note), byte(velocity)})
	logger.Debug("in", "note", note, "kind", kind, "velocity", velocity)
}

// logOutbound records one message on its way to a device. Every write goes through here so
// a debug log always describes what actually left the process, in order.
func logOutbound(voice string, destination alsa.SeqAddr, data []byte) {
	kind, channel, note, velocity := logMIDI(data)
	logger.Debug("midi out",
		"kind", kind,
		"voice", voice,
		"channel", channel,
		"note", note,
		"velocity", velocity,
		"client", destination.Client,
		"port", destination.Port,
		"raw", data,
	)
}

// logMIDI decodes a message for logging. Unknown or short messages are still recorded so
// nothing disappears from the trace.
func logMIDI(msg []byte) (kind string, channel, note, velocity int) {
	if len(msg) == 0 {
		return "empty", 0, 0, 0
	}
	if len(msg) < 3 {
		return "short", 0, 0, 0
	}
	channel = int(msg[0]&0x0f) + 1
	note = int(msg[1])
	velocity = int(msg[2])
	switch msg[0] & 0xf0 {
	case 0x90:
		kind = "note on"
	case 0x80:
		kind = "note off"
	case 0xb0:
		kind = "control change"
	default:
		kind = "other"
	}
	return kind, channel, note, velocity
}
