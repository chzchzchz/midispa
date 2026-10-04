package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

type sequencerWriter interface {
	midiWriter
	WritePort(alsa.SeqEvent, int) error
}

type playbackStopFunc func() error

// The active map is keyed by voice so different destinations can coexist in a mixed kit.
type activeChromaticNote struct {
	channel     int
	note        int
	destination alsa.SeqAddr
	tie         bool
	// step is where the note started, so a note can be stopped after one step.
	step int
}

type chromaticOutbound struct {
	destination alsa.SeqAddr
	data        []byte
}

type Playback struct {
	songBeat float32
	patBeat  float32

	// < 0 if no skip, >= 0 in case of a skip.
	nextSongBeat float32

	updatePads  func(curBeat float32) error
	nextPattern func(curBeat float32) *Pattern

	// vb is the kit whose patches are sent before the first note. It is nil when a
	// playback is started without one, which is what tests and the pattern painters
	// that run no worker do.
	vb *VoiceBank
	// settle is how long the worker waits after sending those patches before playing
	// anything, which is only a wait when a vendor dump was among them.
	settle time.Duration

	positionMu sync.Mutex
	activeMu   sync.Mutex
	active     map[*Voice]activeChromaticNote
	writer     midiWriter
}

func beatDuration(bpm int) time.Duration {
	if bpm <= 0 {
		return 0
	}
	// BPM is a count per minute, so calculate the quotient before converting it to a duration.
	return time.Duration(float64(time.Minute) / float64(bpm))
}

func patternDuration(pattern *Pattern, bpm int) time.Duration {
	if pattern == nil {
		return 0
	}
	return time.Duration(float64(pattern.Beats()) * float64(beatDuration(bpm)))
}

func (p *Playback) Start(aseq sequencerWriter) playbackStopFunc {
	if isNilMidiWriter(aseq) {
		p.reset()
		return func() error { return nil }
	}
	return p.start(aseq)
}

func (p *Playback) start(aseq sequencerWriter) playbackStopFunc {
	p.reset()
	p.writer = aseq
	// An instrument has to be set up before its first note, so the kit's patches go out
	// here, ahead of the worker that plays the pattern. A patch that cannot be sent is
	// logged rather than returned: playback that starts with the wrong settings is
	// still playback, and an error escaping to the button handler would stop the
	// process outright. A dump is not a channel message, so the worker waits out the
	// settle the send asked for before it plays anything.
	settle, err := sendKitPatches(aseq, p.vb)
	p.settle = settle
	if err != nil {
		logger.Error("kit patches", "error", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = p.run(ctx, aseq)
		if runErr != nil && ctx.Err() == nil {
			log.Printf("fireloop playback stopped: %v", runErr)
		}
		close(done)
	}()
	return func() error {
		cancel()
		<-done
		return runErr
	}
}

func (p *Playback) reset() {
	_ = p.releaseAll(p.writer)
	p.positionMu.Lock()
	p.songBeat = 0
	p.patBeat = 0
	p.nextSongBeat = -1
	p.positionMu.Unlock()
	p.activeMu.Lock()
	p.active = make(map[*Voice]activeChromaticNote)
	p.activeMu.Unlock()
	p.writer = nil
}

func (p *Playback) position() (songBeat, patBeat, nextSongBeat float32) {
	p.positionMu.Lock()
	songBeat, patBeat, nextSongBeat = p.songBeat, p.patBeat, p.nextSongBeat
	p.positionMu.Unlock()
	return songBeat, patBeat, nextSongBeat
}

func (p *Playback) setPosition(songBeat, patBeat float32) {
	p.positionMu.Lock()
	p.songBeat = songBeat
	p.patBeat = patBeat
	p.positionMu.Unlock()
}

func (p *Playback) takeNextSongBeat() (float32, bool) {
	p.positionMu.Lock()
	defer p.positionMu.Unlock()
	if p.nextSongBeat < 0 {
		return 0, false
	}
	beat := p.nextSongBeat
	p.nextSongBeat = -1
	return beat, true
}

// JumpSongBeat schedules the seek to take effect at the next pattern boundary.
func (p *Playback) JumpSongBeat(beat float32) (oldSongBeat float32) {
	_ = p.releaseAll(p.writer)
	p.positionMu.Lock()
	oldSongBeat = p.songBeat
	p.nextSongBeat = beat
	p.positionMu.Unlock()
	return oldSongBeat
}

func (p *Playback) playBeat(aseq midiWriter, pat *Pattern) (float32, error) {
	if pat == nil {
		return 0, nil
	}
	_, patBeat, _ := p.position()
	if patBeat >= pat.Beats() {
		if err := p.releaseAll(aseq); err != nil {
			return 0, err
		}
	}
	currentStep := eventStep(Event{Beat: patBeat})
	// A note from an earlier step stops before this one starts, unless a tie held it.
	if err := p.releaseExpired(aseq, currentStep); err != nil {
		return 0, err
	}
	evs := pat.FindBeat(patBeat)
	nextBeat := float32(0)
	for _, ev := range evs {
		step := eventStep(ev)
		if step < currentStep {
			continue
		}
		if step > currentStep {
			// No more events to send.
			nextBeat = stepBeat(step)
			break
		}
		if ev.Voice == nil {
			continue
		}
		if ev.IsChromatic() {
			if err := p.playChromaticEvent(aseq, ev); err != nil {
				return 0, err
			}
			continue
		}
		if err := writeMidiMsgs(aseq, eventDestination(ev), ev.ToMidi()); err != nil {
			return 0, err
		}
	}
	if p.updatePads != nil {
		songBeat, _, _ := p.position()
		if err := p.updatePads(songBeat); err != nil {
			return 0, err
		}
	}
	return nextBeat, nil
}

func chromaticMidiOn(channel, note, velocity int) []byte {
	return []byte{
		midi.MakeNoteOn(channel - 1),
		byte(clampMidiDataValue(note)),
		byte(clampMidiDataValue(velocity)),
	}
}

func chromaticMidiOff(channel, note int) []byte {
	return []byte{
		midi.MakeNoteOff(channel - 1),
		byte(clampMidiDataValue(note)),
		0,
	}
}

// Tied same-pitch transitions emit no messages; tied pitch changes emit note-on before note-off.

func chromaticOutboundMessages(previous *activeChromaticNote, current activeChromaticNote, velocity int) []chromaticOutbound {
	if previous == nil {
		return []chromaticOutbound{{
			destination: current.destination,
			data:        chromaticMidiOn(current.channel, current.note, velocity),
		}}
	}
	if previous.tie && previous.note == current.note {
		return nil
	}
	if previous.tie {
		return []chromaticOutbound{
			{
				destination: current.destination,
				data:        chromaticMidiOn(current.channel, current.note, velocity),
			},
			{
				destination: previous.destination,
				data:        chromaticMidiOff(previous.channel, previous.note),
			},
		}
	}
	return []chromaticOutbound{
		{
			destination: previous.destination,
			data:        chromaticMidiOff(previous.channel, previous.note),
		},
		{
			destination: current.destination,
			data:        chromaticMidiOn(current.channel, current.note, velocity),
		},
	}
}

// playChromaticEvent performs the note transition before updating the active voice state.
func (p *Playback) playChromaticEvent(aseq midiWriter, event Event) error {
	if event.Voice == nil || !event.IsChromatic() {
		return nil
	}
	channel := event.Voice.EffectiveChannel()
	if channel == 0 {
		return nil
	}
	current := activeChromaticNote{
		channel:     channel,
		note:        event.NoteNumber(),
		destination: midiDestination(eventDestination(event)),
		tie:         event.Tie,
		step:        eventStep(event),
	}
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	previous, hasPrevious := p.active[event.Voice]
	var previousNote *activeChromaticNote
	if hasPrevious {
		previousNote = &previous
	}
	messages := chromaticOutboundMessages(previousNote, current, event.Velocity)
	writerActive := !isNilMidiWriter(aseq)
	// The sounding note is recorded before anything is written. A write that fails part
	// way through would otherwise leave a note on with nothing tracking it to release.
	if p.active == nil {
		p.active = make(map[*Voice]activeChromaticNote)
	}
	p.active[event.Voice] = current
	logger.Debug("chromatic transition",
		"voice", voiceLabel(event.Voice),
		"note", current.note,
		"velocity", event.Velocity,
		"tie", current.tie,
		"previous", previousNoteLabel(previousNote),
		"messages", len(messages),
	)
	for _, message := range messages {
		logOutbound(voiceLabel(event.Voice), message.destination, message.data)
		if !writerActive {
			continue
		}
		if err := aseq.Write(alsa.SeqEvent{SeqAddr: message.destination, Data: message.data}); err != nil {
			return err
		}
	}
	return nil
}

// previousNoteLabel names the note that was sounding, so a transition reads as a pair.
func previousNoteLabel(previous *activeChromaticNote) any {
	if previous == nil {
		return "none"
	}
	return previous.note
}

// releaseExpired stops notes whose step has passed. A chromatic note is one step long
// unless a tie holds it into the next event, so a lone note stops where it started
// instead of ringing until the pattern ends.
func (p *Playback) releaseExpired(aseq midiWriter, step int) error {
	type expiredNote struct {
		voice *Voice
		note  activeChromaticNote
	}
	var expired []expiredNote
	p.activeMu.Lock()
	for voice, note := range p.active {
		if note.tie || step <= note.step {
			continue
		}
		expired = append(expired, expiredNote{voice: voice, note: note})
		delete(p.active, voice)
	}
	p.activeMu.Unlock()
	var firstErr error
	for _, item := range expired {
		message := chromaticMidiOff(item.note.channel, item.note.note)
		logOutbound(voiceLabel(item.voice), item.note.destination, message)
		logger.Debug("release", "voice", voiceLabel(item.voice), "note", item.note.note, "reason", "step passed")
		if isNilMidiWriter(aseq) {
			continue
		}
		if err := aseq.Write(alsa.SeqEvent{SeqAddr: item.note.destination, Data: message}); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// releaseAll drains the map before writing, preventing duplicate releases during cancellation.
func (p *Playback) releaseAll(aseq midiWriter) error {
	p.activeMu.Lock()
	if len(p.active) == 0 {
		p.activeMu.Unlock()
		return nil
	}
	active := p.active
	p.active = make(map[*Voice]activeChromaticNote)
	p.activeMu.Unlock()
	var firstErr error
	for voice, note := range active {
		message := chromaticMidiOff(note.channel, note.note)
		logOutbound(voiceLabel(voice), note.destination, message)
		logger.Debug("release", "voice", voiceLabel(voice), "note", note.note, "reason", "release all")
		if isNilMidiWriter(aseq) {
			continue
		}
		if err := aseq.Write(alsa.SeqEvent{SeqAddr: note.destination, Data: message}); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *Playback) activeNoteCount() int {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	return len(p.active)
}

func writeSequencerPort(aseq sequencerWriter, data []byte) error {
	if aseq == nil {
		return nil
	}
	if seq, ok := aseq.(*alsa.Seq); ok && seq == nil {
		return nil
	}
	return aseq.WritePort(alsa.SeqEvent{SeqAddr: alsa.SubsSeqAddr, Data: data}, syncPort.Port)
}

func (p *Playback) run(ctx context.Context, aseq sequencerWriter) (runErr error) {
	started := false
	logger.Info("playback start", "bpm", currentBPM())
	defer func() {
		if err := p.releaseAll(aseq); runErr == nil {
			runErr = err
		}
		if started {
			if err := writeSequencerPort(aseq, []byte{midi.Stop}); runErr == nil {
				runErr = err
			}
		}
		logger.Info("playback end", "error", runErr)
	}()
	curBpm := currentBPM()
	var curPattern *Pattern
	if p.nextPattern != nil {
		curPattern = p.nextPattern(0)
	}
	// The settle wait belongs here rather than in the button handler, so a press while
	// an instrument is loading is still read, and the start time is taken after it so
	// the first measure keeps its full length.
	if p.settle > 0 {
		logger.Debug("settling after a patch", "duration", p.settle)
		select {
		case <-time.After(p.settle):
		case <-ctx.Done():
			return nil
		}
	}
	// Compute measures w/r/t this start time + now() to avoid drift.
	start := time.Now()
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		songBeat, patBeat, _ := p.position()
		if !started {
			if err := writeSequencerPort(aseq, []byte{midi.Start}); err != nil {
				return err
			}
			started = true
		}
		if curPattern == nil {
			curPattern = &emptyPattern
		}
		// A live length edit can move the playhead past the new end; rewind only the current pattern.
		if patBeat >= curPattern.Beats() {
			_ = p.releaseAll(aseq)
			p.setPosition(songBeat-patBeat, 0)
			_, patBeat, _ = p.position()
		}
		nextBeat, err := p.playBeat(aseq, curPattern)
		if err != nil {
			return err
		}
		_, patBeat, _ = p.position()
		// Find next event time, if any.
		next16th := float32(math.Floor(float64(patBeat*patternStepsPerBeat))+1.0) * patternBeatsPerStep
		patternBeats := curPattern.Beats()
		if (nextBeat == 0 || nextBeat > next16th) && patBeat < patternBeats {
			// TODO: this should be PPQ for midi clock mastering.
			nextBeat = next16th
		}
		if nextBeat >= patternBeats {
			// Past measure; reset.
			nextBeat = 0
		}
		var waitUntil time.Duration
		if nextBeat != 0 {
			waitTime := time.Duration(float64(nextBeat-patBeat) * float64(beatDuration(curBpm)))
			songBeat, _, _ = p.position()
			p.setPosition(songBeat+nextBeat-patBeat, nextBeat)
			waitUntil = waitTime
		} else {
			// Reset to next measure.
			measureLength := patternDuration(curPattern, curBpm)
			start = start.Add(measureLength)
			waitUntil = time.Until(start)
			curBpm = currentBPM()
			if requested, ok := p.takeNextSongBeat(); ok {
				songBeat = requested
			} else {
				songBeat, _, _ = p.position()
				songBeat += curPattern.Beats() - patBeat
			}
			logger.Debug("pattern boundary", "songBeat", songBeat, "beats", curPattern.Beats(), "bpm", curBpm)
			_ = p.releaseAll(aseq)
			if p.nextPattern != nil {
				curPattern = p.nextPattern(songBeat)
			} else {
				curPattern = nil
			}
			if curPattern == nil {
				// Loop.
				songBeat = 0
				if p.nextPattern != nil {
					curPattern = p.nextPattern(0)
				}
			}
			p.setPosition(songBeat, 0)
		}
		if timer == nil {
			timer = time.NewTimer(waitUntil)
		} else {
			timer.Reset(waitUntil)
		}
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil
		}
	}
}

// The writer is an interface so a test can drive the real sequencer loop with a stub
// instead of a port. A missing writer still means nothing to drive: the painters are built
// and published, but no worker starts, which is what Playback.Start's nil check did.
func (pb *PatternBank) startSequencer(aseq sequencerWriter) playbackStopFunc {
	var lastColumn int
	// Reset to start of pattern.
	next := func(beat float32) *Pattern {
		lastColumn = 15
		if beat == 0 {
			return pb.CurrentPattern()
		}
		return nil
	}
	// Light up column if new position.
	update := func(beat float32) error {
		if pb.NoteEditActive() {
			// Note editing owns the grid with the pitch palette, so the playhead moves
			// along the step strip rather than over the palette.
			return pb.drawStepPlayhead(eventStep(Event{Beat: beat}))
		}
		thisColumn := int(math.Floor(float64(beat*patternStepsPerBeat))) % 16
		if thisColumn == lastColumn {
			// No update.
			return nil
		}
		// Reset last column.
		if err := pb.drawPadColumn(lastColumn); err != nil {
			return err
		}
		// Set new column.
		lastColumn = thisColumn
		return pb.drawPadColumnInvert(thisColumn)
	}
	p := &Playback{updatePads: update, nextPattern: next, vb: pb.vb}
	pb.playback = p
	if isNilMidiWriter(aseq) {
		p.reset()
		return func() error { return nil }
	}
	return p.start(aseq)
}

func (sb *SongBank) startSequencer(aseq sequencerWriter) playbackStopFunc {
	p := &Playback{vb: sb.kit()}
	sb.playback = p
	// Move to next song pattern.
	p.nextPattern = func(beat float32) *Pattern {
		pat, _ := sb.CurrentSong().BeatToPattern(beat)
		return pat
	}
	// Light playing measure.
	lastSongBeat := float32(-99999)
	p.updatePads = func(beat float32) error {
		s := sb.CurrentSong()
		_, lastMeasure := s.BeatToPattern(lastSongBeat)
		pat, givenMeasure := s.BeatToPattern(beat)
		if givenMeasure < 0 {
			return nil
		}
		lastSongBeat = beat
		if lastMeasure == givenMeasure {
			return nil
		}
		if err := sb.ToggleMeasureBrightness(lastMeasure, givenMeasure); err != nil {
			return err
		}
		if err := sb.printRow(4, fmt.Sprintf("Measure %03d", givenMeasure+1)); err != nil {
			return err
		}
		pidx := sb.pb.PatternIdxMap()[pat]
		return sb.printRow(5, fmt.Sprintf("Pat-bar %03d", pidx))
	}
	return p.Start(aseq)
}
