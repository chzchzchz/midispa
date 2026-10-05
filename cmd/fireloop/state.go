package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// A session is everything the user has made: the patterns, the songs that arrange them,
// the kit's voices assigned to the tracks, and the tempo. It lives in one JSON file so a
// set survives the process.
//
// Two rules shape everything below. A save never damages the set already on disk, so the
// bytes go to a temporary file in the same directory and are renamed into place. A load
// never silently plays the wrong thing, so every voice reference is resolved against the
// kit this run actually loaded, and anything that cannot be resolved is counted and
// reported rather than dropped in silence.
//
// Where the user was looking is not part of a session. The selected pattern, the scroll
// windows, the step cursor and the palette octave are the display's business, and a set
// that came back somewhere else is still the same set.

const (
	// stateApp marks a file as ours, so a file of some other kind is refused with a clear
	// message instead of a confusing decode failure.
	stateApp = "fireloop"
	// stateVersion changes whenever the layout changes in a way an older build cannot
	// read. A version this build does not know is refused rather than half-read.
	stateVersion = 1
	// stateIndent keeps the file readable and diffable, which matters because it is the
	// one artefact of a session a user can inspect after a show.
	stateIndent = "  "
	// stateTempSuffix marks the half-written file an interrupted save leaves behind. The
	// rename publishes the name, so the file on disk is always a whole file.
	stateTempSuffix = ".tmp"
	// stateTempoMin and stateTempoMax are the bounds the tempo entry gesture accepts, so
	// a restored tempo is one the sequencer would have accepted in the first place.
	stateTempoMin = 21
	stateTempoMax = 299
	// emptyMeasure is how a song records a measure holding no pattern.
	emptyMeasure = -1
)

// stateFile is the whole on-disk session. It records what it was written against, because
// a voice index only means something against the kit that produced it.
type stateFile struct {
	App     string    `json:"app"`
	Version int       `json:"version"`
	SavedAt time.Time `json:"savedAt"`
	Kit     []string  `json:"kit"`
	// VoiceCount is the number of voices that kit had, since a voice index on its own
	// says nothing about whether that voice still exists.
	VoiceCount int `json:"voiceCount"`
	BPM        int `json:"bpm"`
	// TrackVoices is the voice sitting on each track. It belongs to the pattern bank
	// rather than to any one pattern, and it is not recoverable from the patterns: a
	// pattern records which voice sounds on a step, not which track row it sits on, and
	// two rows are allowed to hold the same voice. Its length is the track count.
	TrackVoices []int          `json:"trackVoices"`
	Patterns    []statePattern `json:"patterns"`
	Songs       []stateSong    `json:"songs"`
}

type statePattern struct {
	Index       int          `json:"index"`
	LengthSteps int          `json:"lengthSteps"`
	Events      []stateEvent `json:"events"`
}

// stateEvent is a grid position and a voice index, never a beat. The beat in a live event
// is derived from the step, so writing the float would put a float32 in the file for no
// gain and cost precision on the way back in.
type stateEvent struct {
	Voice    int  `json:"voice"`
	Step     int  `json:"step"`
	Note     int  `json:"note"`
	Velocity int  `json:"velocity"`
	Tie      bool `json:"tie,omitempty"`
}

// stateSong lists one entry per measure holding the pattern in that measure. Measures
// holding nothing are emptyMeasure, and the trailing ones are trimmed the way
// Song.SetPattern trims them, so a song reads as the list of measures it actually has.
type stateSong struct {
	Index    int   `json:"index"`
	Measures []int `json:"measures"`
}

// stateReport is what a save or a load reports. Dropped counts everything the file asked
// for that this kit or this set cannot supply: the one outcome that must never happen
// quietly is a pattern losing notes on the way back in.
type stateReport struct {
	Bytes    int
	Patterns int
	Songs    int
	Dropped  int
}

// loadText reports what came back on the readout row. The dropped count leads when it is
// there, because it is the figure that must not be missed, and the totals only come along
// while both numbers fit: a sentence cut at the row width ends in a different number than
// it started with, and on stage that is worse than saying less.
func (r stateReport) loadText() string {
	switch {
	case r.Dropped > 0:
		if text := fmt.Sprintf("Loaded %d, dropped %d", r.Patterns, r.Dropped); len(text) <= oledTextWidth {
			return text
		}
		return fmt.Sprintf("dropped %d", r.Dropped)
	case r.Patterns == 0 && r.Songs == 0:
		return "Loaded empty session"
	default:
		return fmt.Sprintf("Loaded %d patterns", r.Patterns)
	}
}

// saveText reports the size on the readout row, which is the figure that tells the user
// the file was written rather than left behind.
func (r stateReport) saveText() string {
	return fmt.Sprintf("Saved %.1f KB", float64(r.Bytes)/1024)
}

func (r stateReport) logAttrs() []any {
	return []any{"patterns", r.Patterns, "songs", r.Songs, "dropped", r.Dropped, "bytes", r.Bytes}
}

// statePath is the file the session is saved to and loaded from, and stateKitPaths is the
// kit it is measured against. Both stay empty until main wires the flag, so the panel
// gestures report that there is nowhere to save rather than writing somewhere unasked.
var (
	statePath     string
	stateKitPaths []string
)

// The two ways a file can be refused are named so the panel can say which one it hit in
// the handful of characters the readout row has, instead of the path and the version
// numbers that the log is for.
var (
	// errStateNotSession is a file written by something other than fireloop.
	errStateNotSession = errors.New("not a fireloop session")
	// errStateVersion is a session in a layout this build does not know how to read.
	errStateVersion = errors.New("unreadable session version")
)

// stateFrom takes the snapshot the banks own and turns it into the file layout. It reads
// only, so it is safe to call while a pattern is playing: the playback worker is handed the
// pattern to sound rather than reaching into the bank, so the only writer of the pattern map
// is the goroutine that calls this, which is also the goroutine a load installs from.
func stateFrom(pb *PatternBank, sb *SongBank, kit []string) stateFile {
	state := stateFile{
		App:     stateApp,
		Version: stateVersion,
		SavedAt: time.Now(),
		Kit:     append([]string(nil), kit...),
		BPM:     currentBPM(),
	}
	if pb == nil {
		return state
	}
	state.VoiceCount = len(pb.vb.voices)
	trackVoices, patterns := pb.snapshotSession()
	state.TrackVoices = trackVoices
	voices := pb.vb.voiceIndex()
	// The banks are maps and Go randomises map iteration, so the keys are walked in index
	// order: without that two saves of one session would differ byte for byte and the file
	// could not be diffed.
	for _, index := range slices.Sorted(maps.Keys(patterns)) {
		saved := statePatternFrom(index, patterns[index], voices)
		// An untouched pattern is not worth a file entry: every pattern the user ever
		// landed on has one, and hundreds of empty placeholders make the file harder to
		// read without making it more complete. A shortened empty pattern still goes in,
		// because its length is not the default.
		if len(saved.Events) == 0 && saved.LengthSteps == defaultPatternSteps {
			continue
		}
		state.Patterns = append(state.Patterns, saved)
	}
	if sb != nil {
		state.Songs = stateSongsFrom(sb, patternIndexMap(patterns))
	}
	return state
}

// validPatternIndex reports whether a pattern or song index names a slot the banks have.
// A file is free to carry anything, so an index outside the range is refused rather than
// clamped: a clamped song index would land the arrangement on a measure the user never
// wrote.
func validPatternIndex(index int) bool {
	return index >= 1 && index <= maxPatternIndex
}

func statePatternFrom(index int, pattern *Pattern, voices map[*Voice]int) statePattern {
	saved := statePattern{Index: index}
	if pattern == nil {
		saved.LengthSteps = defaultPatternSteps
		return saved
	}
	events, lengthSteps := pattern.snapshot()
	saved.LengthSteps = lengthSteps
	for _, event := range events {
		voice, ok := voices[event.Voice]
		if !ok {
			// A voice outside the bank cannot be named, and guessing an index would put
			// the note on whatever voice happens to sit there.
			logger.Warn("state event has no voice", "pattern", index, "step", eventStep(event), "voice", voiceLabel(event.Voice))
			continue
		}
		saved.Events = append(saved.Events, stateEvent{
			Voice:    voice,
			Step:     eventStep(event),
			Note:     event.ChromaticNote,
			Velocity: event.Velocity,
			Tie:      event.Tie,
		})
	}
	return saved
}

// stateSongsFrom records each measure as the pattern index it holds, from the pointer to
// index map the bank builds for exactly this question.
//
// The arrangement is rebuilt through SetPattern rather than scanned for its own last
// measure, so the trimming rule lives in one place. A second copy of that rule here would
// drift the moment SetPattern changed, and the drift would only show up as a file that does
// not match the set it was written from.
func stateSongsFrom(sb *SongBank, indices map[*Pattern]int) []stateSong {
	var songs []stateSong
	for _, index := range slices.Sorted(maps.Keys(sb.Songs)) {
		rebuilt := &Song{}
		for measure, pattern := range sb.Songs[index].measurePatterns() {
			if pattern == nil {
				continue
			}
			if _, ok := indices[pattern]; !ok {
				logger.Warn("state measure holds a pattern the bank does not have", "song", index, "measure", measure)
				continue
			}
			rebuilt.SetPattern(pattern, measure)
		}
		measures := rebuilt.length()
		if measures == 0 {
			// A song with no measures carries nothing a restore could not invent.
			continue
		}
		saved := stateSong{Index: index, Measures: make([]int, measures)}
		for i := range saved.Measures {
			saved.Measures[i] = emptyMeasure
		}
		for measure, pattern := range rebuilt.measurePatterns() {
			if patternIndex, ok := indices[pattern]; ok {
				saved.Measures[measure] = patternIndex
			}
		}
		songs = append(songs, saved)
	}
	return songs
}

// saveState writes the session to path. It reads only, so it is safe to call while a
// pattern is playing, which is exactly when a user reaches for it.
func saveState(path string, pb *PatternBank, sb *SongBank, kit []string) (stateReport, error) {
	state := stateFrom(pb, sb, kit)
	written, err := writeStateFile(path, state)
	if err != nil {
		return stateReport{}, err
	}
	return stateReport{
		Bytes:    written,
		Patterns: len(state.Patterns),
		Songs:    len(state.Songs),
	}, nil
}

// writeStateFile encodes into memory and then publishes the result with a rename. A
// rename within one directory is atomic, so the file on disk is either the previous set or
// the new one and never a half-written mixture, and the failure mode of a save is always
// "the old set is still there".
func writeStateFile(path string, state stateFile) (int, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", stateIndent)
	if err := encoder.Encode(state); err != nil {
		return 0, fmt.Errorf("encode state: %w", err)
	}
	data := buf.Bytes()
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*"+stateTempSuffix)
	if err != nil {
		return 0, fmt.Errorf("save state %q: %w", path, err)
	}
	name := temp.Name()
	// Harmless once the rename has succeeded, and the reason an encode or write failure
	// does not leave litter next to the state file.
	defer func() { _ = os.Remove(name) }()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return 0, fmt.Errorf("save state %q: %w", path, err)
	}
	// The rename publishes the name, so the bytes have to be on the disk before it.
	if err := temp.Sync(); err != nil {
		temp.Close()
		return 0, fmt.Errorf("save state %q: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return 0, fmt.Errorf("save state %q: %w", path, err)
	}
	if err := os.Rename(name, path); err != nil {
		return 0, fmt.Errorf("save state %q: %w", path, err)
	}
	return len(data), nil
}

// readStateFile decodes and checks a file. A file that is not ours, or is from a version
// this build does not know, is refused here so no part of a session is applied from it.
func readStateFile(path string) (stateFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return stateFile{}, err
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return stateFile{}, fmt.Errorf("state %q is not readable: %w", path, err)
	}
	if state.App != stateApp {
		return stateFile{}, fmt.Errorf("state %q is not a %s session: %w", path, stateApp, errStateNotSession)
	}
	if state.Version != stateVersion {
		return stateFile{}, fmt.Errorf("state %q is version %d, this build reads version %d: %w",
			path, state.Version, stateVersion, errStateVersion)
	}
	return state, nil
}

// kitChanged reports whether this run loaded a different kit from the one the file was
// written against. The voice indices still line up when the two agree on how many voices
// there are, so the load goes ahead; the difference is logged so a wrong drum can be
// traced rather than guessed at.
func (s stateFile) kitChanged(kit []string, voices int) (changed bool, reason string) {
	if !slices.Equal(s.Kit, kit) {
		return true, "kit paths differ"
	}
	if s.VoiceCount != voices {
		return true, fmt.Sprintf("kit has %d voices, file was written with %d", voices, s.VoiceCount)
	}
	return false, ""
}

// loadState replaces the session the banks hold with the one in path. Everything is
// decoded and resolved into fresh objects before anything live is touched, so a file that
// turns out to be unusable leaves the running session exactly as it was. It does not
// repaint: only the caller knows which view is showing, and only the caller has the mode
// light to restore.
func loadState(path string, pb *PatternBank, sb *SongBank, kit []string) (stateReport, error) {
	state, err := readStateFile(path)
	if err != nil {
		return stateReport{}, err
	}
	var voiceBank *VoiceBank
	voiceCount := 0
	if pb != nil {
		voiceBank = pb.vb
		voiceCount = len(voiceBank.voices)
	}
	if changed, reason := state.kitChanged(kit, voiceCount); changed {
		logger.Warn("state written against a different kit",
			"path", path, "reason", reason, "fileKit", state.Kit, "fileVoices", state.VoiceCount)
	}
	patterns, droppedPatterns := resolvePatterns(state, voiceBank)
	// Playback is stopped only once there is something to install. The worker reads the
	// banks from its own goroutine, and stopping joins it, so from here on no reader is
	// left holding a pattern the load is about to replace.
	if err := stopPlayback(); err != nil {
		return stateReport{}, err
	}
	report := stateReport{Patterns: len(patterns), Dropped: droppedPatterns}
	if pb != nil {
		pb.restoreState(state, patterns)
	}
	songs, droppedMeasures := resolveSongs(state, patterns)
	report.Songs = len(songs)
	report.Dropped += droppedMeasures
	if sb != nil {
		sb.installSongs(songs)
	}
	setBPM(clampIndex(state.BPM, stateTempoMin, stateTempoMax))
	return report, nil
}

// resolvePatterns builds one pattern per index the file names. The same map goes to the
// bank and to the songs, which is what keeps a measure and the pattern pad pointing at
// one object the way they do while the session is running.
func resolvePatterns(state stateFile, vb *VoiceBank) (map[int]*Pattern, int) {
	patterns := make(map[int]*Pattern, len(state.Patterns))
	dropped := 0
	for _, saved := range state.Patterns {
		if !validPatternIndex(saved.Index) {
			logger.Warn("state pattern index out of range", "index", saved.Index)
			dropped++
			continue
		}
		pattern, lost := patternFromState(saved, vb)
		dropped += lost
		patterns[saved.Index] = pattern
	}
	return patterns, dropped
}

// patternFromState rebuilds one pattern. Every field is clamped rather than rejected,
// because a set with one bad number in it is still mostly a set, and the dropped count is
// what tells the user which parts did not survive.
func patternFromState(saved statePattern, vb *VoiceBank) (*Pattern, int) {
	pattern := &Pattern{}
	dropped := 0
	for _, event := range saved.Events {
		if event.Step < 0 || event.Step >= maxPatternSteps {
			logger.Warn("state event step out of range", "pattern", saved.Index, "step", event.Step)
			dropped++
			continue
		}
		voice, ok := vb.voiceAt(event.Voice)
		if !ok {
			logger.Warn("state event names a voice this kit does not have",
				"pattern", saved.Index, "step", event.Step, "voice", event.Voice)
			dropped++
			continue
		}
		pattern.Events = append(pattern.Events, Event{
			Voice:         voice,
			Beat:          stepBeat(event.Step),
			ChromaticNote: clampMidiDataValue(event.Note),
			Velocity:      clampMidiDataValue(event.Velocity),
			Tie:           event.Tie,
		})
	}
	// A length of zero in the file means the default rather than an empty pattern, which is
	// the same rule a live pattern uses for its stored zero, so both resolve it in one place.
	lengthSteps := storedLengthSteps(saved.LengthSteps)
	// Setting the length through the setter is what drops events past the restored end
	// and repairs the ties, so a hand-edited file cannot install a note outside its own
	// pattern or a tie pointing at nothing.
	pattern.SetLengthSteps(lengthSteps)
	return pattern, dropped
}

// resolveSongs rebuilds each song against the resolved patterns and reports how many
// measures named a pattern the file does not carry. SetPattern is used rather than a
// direct slice so a song trims its empty tail exactly as it does while running. Making sure
// the selected slot exists is installSongs' business, not this one's, so that the
// guarantee lives in the place that owns the selection.
func resolveSongs(state stateFile, patterns map[int]*Pattern) (map[int]*Song, int) {
	songs := make(map[int]*Song, len(state.Songs))
	dropped := 0
	for _, saved := range state.Songs {
		if !validPatternIndex(saved.Index) {
			logger.Warn("state song index out of range", "index", saved.Index)
			dropped++
			continue
		}
		song := &Song{}
		for measure, patternIndex := range saved.Measures {
			if patternIndex < 1 {
				continue
			}
			if measure > maxMeasureIndex {
				logger.Warn("state measure index out of range", "song", saved.Index, "measure", measure)
				dropped++
				continue
			}
			pattern, ok := patterns[patternIndex]
			if !ok {
				logger.Warn("state measure names a pattern the file does not carry",
					"song", saved.Index, "measure", measure, "pattern", patternIndex)
				dropped++
				continue
			}
			song.SetPattern(pattern, measure)
		}
		songs[saved.Index] = song
	}
	return songs, dropped
}

// stateFailureText keeps a failure readable on the twenty-column readout row. The whole
// reason goes to the log, which both callers write before asking for this text; what fits
// here is a hint, not the story. Every hint is short enough to survive fitOLEDText, because
// a sentence cut at twenty characters says less than a blunt phrase that fits whole.
func stateFailureText(prefix string, err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return prefix + ": no file"
	case errors.Is(err, errStateNotSession):
		return prefix + ": foreign"
	case errors.Is(err, errStateVersion):
		return prefix + ": version"
	default:
		return prefix + ": error"
	}
}

// loadStateFile loads the session named by -state into the banks and repaints the pattern
// view, which is the view a run starts in. It is the startup path, so a missing file is
// reported rather than treated as an error: that is what a first run looks like.
func loadStateFile(path string) error {
	report, err := loadState(path, patbank, songbank, stateKitPaths)
	if err != nil {
		return err
	}
	if err := patbank.Jump(0); err != nil {
		return err
	}
	logger.Info("state loaded", append([]any{"path", path}, report.logAttrs()...)...)
	return nil
}

// exitSaveOnce keeps the exit save to one write. The signal path and a panic unwind can
// both reach it, and a second write would only add another timestamp to a file the user
// is about to read.
var exitSaveOnce sync.Once

// saveSessionOnExit writes the session as the process goes down. It is the save that stops
// a set being lost to a pulled cable, so it runs whether the program was asked to stop or
// the event handler fell over doing it. A panic on some other goroutine, the playback
// worker being the one that matters, still ends the process without it.
func saveSessionOnExit() {
	if statePath == "" || patbank == nil {
		return
	}
	exitSaveOnce.Do(func() {
		report, err := saveState(statePath, patbank, songbank, stateKitPaths)
		if err != nil {
			logger.Error("state save on exit failed", "path", statePath, "error", err)
			return
		}
		logger.Info("state saved on exit", append([]any{"path", statePath, "kit", stateKitPaths}, report.logAttrs()...)...)
	})
}
