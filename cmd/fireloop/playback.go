package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// sequencerWriter is what playback needs from the ALSA client. Playback writes two
// different things: musical events on the default port, and transport on the sync port it
// created for the purpose. Those are the two calls below, written out rather than composed
// from the halves alsa publishes, so that the two places this type is used read the same
// way as everything else that talks to the sequencer.
type sequencerWriter interface {
	Write(alsa.SeqEvent) error
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
	writer     alsa.EventWriter
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

func (p *Playback) playBeat(aseq alsa.EventWriter, pat *Pattern) (float32, error) {
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
		midi.MakeNoteOn(protocolChannel(channel)),
		byte(clampMidiDataValue(note)),
		byte(clampMidiDataValue(velocity)),
	}
}

func chromaticMidiOff(channel, note int) []byte {
	return []byte{
		midi.MakeNoteOff(protocolChannel(channel)),
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
func (p *Playback) playChromaticEvent(aseq alsa.EventWriter, event Event) error {
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
	writerActive := !isNilMidiWriter(aseq)
	var previousNote *activeChromaticNote
	var messages []chromaticOutbound
	p.activeMu.Lock()
	if previous, hasPrevious := p.active[event.Voice]; hasPrevious {
		previousNote = &previous
	}
	messages = chromaticOutboundMessages(previousNote, current, event.Velocity)
	// The sounding note is recorded before anything is written. A write that fails part
	// way through would otherwise leave a note on with nothing tracking it to release.
	if p.active == nil {
		p.active = make(map[*Voice]activeChromaticNote)
	}
	p.active[event.Voice] = current
	p.activeMu.Unlock()
	logger.Debug("chromatic transition",
		"voice", voiceLabel(event.Voice),
		"note", current.note,
		"velocity", event.Velocity,
		"tie", current.tie,
		"previous", previousNoteLabel(previousNote),
		"messages", len(messages),
	)
	// The lock is released before the writes, because an ALSA write can block and a
	// release arriving from the button handler must not queue behind a note still going
	// out. That is the same order releaseExpired and releaseAll use.
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
func (p *Playback) releaseExpired(aseq alsa.EventWriter, step int) error {
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
func (p *Playback) releaseAll(aseq alsa.EventWriter) error {
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

// writeSequencerPort sends a transport message from the sync port, which is why it needs
// the port rather than the default one: a Fire has to see Start and Stop on the port it
// subscribed to, separate from the notes.
func writeSequencerPort(aseq alsa.PortWriter, data []byte) error {
	if isNilMidiWriter(aseq) {
		return nil
	}
	return aseq.WritePort(alsa.SeqEvent{SeqAddr: alsa.SubsSeqAddr, Data: data}, syncPort.Port)
}

func (p *Playback) run(ctx context.Context, aseq sequencerWriter, clock stepClock) (runErr error) {
	started := false
	// The run's swing is a property of the run rather than an event inside it, so it is a
	// field here rather than a line of its own.
	logger.Info("playback start", "bpm", currentBPM(), "swing", currentSwingPct())
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
	curPattern := p.patternForBeat(0)
	defer clock.Stop()
	// The settle wait belongs here rather than in the button handler, so a press while
	// an instrument is loading is still read, and the anchor is taken after it so
	// the first measure keeps its full length.
	if p.settle > 0 {
		logger.Debug("settling after a patch", "duration", p.settle)
		if !clock.Wait(ctx, p.settle) {
			return nil
		}
	}
	// Compute measures w/r/t this anchor + the clock's now() to avoid drift.
	anchor := clock.Now()
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
		nextBeat = nextEventBeat(patBeat, nextBeat, curPattern.Beats())

		var waitUntil time.Duration
		if nextBeat != 0 {
			// The swing read is in the call rather than hoisted into a variable beside
			// curBpm, so a turn of the knob takes effect from the next step. The tempo is
			// sampled once and re-read only at a pattern boundary, which is right for a tap
			// and wrong for a groove: a boundary is a coarse place to put between a knob and
			// the music, so the two deliberately do not agree about this.
			waitUntil = swungSpan(patBeat, nextBeat, beatDuration(curBpm), currentSwingPct())
			songBeat, _, _ = p.position()
			p.setPosition(songBeat+nextBeat-patBeat, nextBeat)
		} else {
			advance := p.crossMeasure(aseq, curPattern, curBpm, patBeat, anchor)
			curPattern, curBpm, anchor = advance.pattern, advance.bpm, advance.anchor
			waitUntil = anchor.Sub(clock.Now())
		}
		if !clock.Wait(ctx, waitUntil) {
			return nil
		}
	}
}

// nextEventBeat is where the playhead should land next inside a pattern. The pattern's own
// next event is used when it falls sooner than the step grid, so a step written off the grid
// still sounds on time. Zero means the end of the pattern, where the boundary takes over.
func nextEventBeat(patBeat, nextBeat, patternBeats float32) float32 {
	// TODO: this should be PPQ for midi clock mastering.
	next16th := float32(math.Floor(float64(patBeat*patternStepsPerBeat))+1.0) * patternBeatsPerStep
	if (nextBeat == 0 || nextBeat > next16th) && patBeat < patternBeats {
		nextBeat = next16th
	}
	if nextBeat >= patternBeats {
		// Past measure; the boundary settles it instead.
		return 0
	}
	return nextBeat
}

// measureAdvance is what settling the end of one pattern produces: the pattern that plays
// next, the tempo to play it at, and the moment the next one is due. The wait until that
// moment is the caller's to take, because the clock is the caller's.
type measureAdvance struct {
	pattern *Pattern
	bpm     int
	anchor  time.Time
}

// crossMeasure carries the playhead past the end of one pattern. The wait is measured
// against the running anchor rather than from now, so the drift picked up inside each step
// is taken back here instead of accumulating across a set. A seek asked for while playing
// takes effect at this boundary, which is the only place it is applied.
func (p *Playback) crossMeasure(aseq sequencerWriter, cur *Pattern, curBpm int, patBeat float32, anchor time.Time) measureAdvance {
	nextAnchor := anchor.Add(patternDuration(cur, curBpm))
	bpm := currentBPM()
	var songBeat float32
	if requested, ok := p.takeNextSongBeat(); ok {
		songBeat = requested
	} else {
		songBeat, _, _ = p.position()
		songBeat += cur.Beats() - patBeat
	}
	logger.Debug("pattern boundary", "songBeat", songBeat, "beats", cur.Beats(), "bpm", bpm)
	_ = p.releaseAll(aseq)
	nextPattern := p.patternForBeat(songBeat)
	if nextPattern == nil {
		// Nothing plays here, so the loop starts again rather than leaving a gap.
		songBeat = 0
		nextPattern = p.patternForBeat(0)
	}
	p.setPosition(songBeat, 0)
	return measureAdvance{pattern: nextPattern, bpm: bpm, anchor: nextAnchor}
}

// patternForBeat asks the playback's own source which pattern sits at a song position. It
// is nil for a playback with no source and for a song with no measure there, which are the
// same thing to the loop: there is nothing to play until it starts again.
func (p *Playback) patternForBeat(songBeat float32) *Pattern {
	if p.nextPattern == nil {
		return nil
	}
	return p.nextPattern(songBeat)
}

// newPlayback builds the playback this bank would run: which pattern answers each beat,
// and how the playhead is drawn as it moves. It starts nothing and claims nothing, because
// a bank says what it would play and the controller decides that something is. The writer
// is an interface so a test can drive the real sequencer loop with a stub instead of a
// port.
func (pb *PatternBank) newPlayback() *Playback {
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
		if pb.noteEditActive() {
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
	return &Playback{updatePads: update, nextPattern: next, vb: pb.vb}
}

// newPlayback is the song bank's counterpart. It measures against the song rather than
// against a pattern, and it is where the running measure readout comes from.
func (sb *SongBank) newPlayback() *Playback {
	p := &Playback{vb: sb.kit()}
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
	return p
}
