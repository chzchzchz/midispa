package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/chzchzchz/midispa/wav"
)

type wavUIModel struct {
	state         *State
	track         *Track
	segment       *Segment
	segmentWindow SegmentWindow
	width         int
	height        int
	wavReader     *wav.WavReader
	timeSpan      time.Duration
	// Max absolute value for each column (for loudness)
	columnMaxValues []int
	// Min and max sample values for each column (for waveform range)
	columnMinValues  []int
	columnMaxSamples []int
	// Track if we need to reload data
	dirty bool
	// WavPlayer for playback
	wavPlayer *WavPlayer
	// Playback state
	playStartTime     time.Time
	playColumn        int
	offsetInput       textinput.Model
	lengthInput       textinput.Model
	offsetInputActive bool
	lengthInputActive bool
}

func NewWavUIModelFromTrackSegment(s *State, track *Track, seg *TrackSegment) *wavUIModel {
	var segment *Segment
	var sw SegmentWindow
	if seg != nil && seg.Segment != nil {
		segment = seg.Segment
		sw = seg.SegmentWindow
	}
	wavPlayer, err := NewWavPlayer(segment.Path, s.Play)
	if err != nil {
		log.Printf("failed to create wav player: %v", err)
		return nil
	}
	return &wavUIModel{
		state:         s,
		track:         track,
		segment:       segment,
		segmentWindow: sw,
		width:         80,
		height:        24,
		timeSpan:      1 * time.Second,
		dirty:         true,
		wavPlayer:     wavPlayer,
	}
}

func NewWavUIModelFromSegment(s *State, track *Track, seg *Segment) *wavUIModel {
	wavPlayer, err := NewWavPlayer(seg.Path, s.Play)
	if err != nil {
		log.Printf("failed to create wav player: %v", err)
		return nil
	}
	return &wavUIModel{
		state:   s,
		track:   track,
		segment: seg,
		segmentWindow: SegmentWindow{
			Offset: 0,
			Length: seg.Samples,
		},
		width:     80,
		height:    24,
		timeSpan:  1 * time.Second,
		dirty:     true,
		wavPlayer: wavPlayer,
	}
}

func (m *wavUIModel) Init() tea.Cmd {
	if m.segment == nil {
		return nil
	}
	var err error
	m.wavReader, err = wav.OpenReader(m.segment.Path)
	if err != nil {
		return nil
	}
	return nil
}

func (m *wavUIModel) Close() {
	m.wavPlayer.Close()
	m.state.Play.Stop()
	m.wavReader.Close()
	m.columnMinValues = nil
	m.columnMaxValues = nil
	m.columnMaxSamples = nil
}

func (m *wavUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.offsetInputActive {
		return m.updateOffsetInput(msg)
	}
	if m.lengthInputActive {
		return m.updateLengthInput(msg)
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.dirty = m.width != msg.Width
		m.width = msg.Width
		m.height = min(msg.Height, 24)
	case tickMsg:
		return m.Tick()
	}
	return m.viewDirty()
}

func (m *wavUIModel) Tick() (tea.Model, tea.Cmd) {
	// Update elapsed time while playing
	if m.wavPlayer.Running() {
		playElapsed := time.Since(m.playStartTime)
		// Track current playing column
		elapsedSamples := int64(playElapsed.Seconds() * float64(m.segment.SampleRate))
		newCol := int(float64(elapsedSamples) / float64(m.segmentWindow.Length) * float64(m.width))
		newCol = max(newCol, 0)
		newCol = min(newCol, m.width-1)
		m.dirty = m.playColumn != newCol
		m.playColumn = newCol
		if playElapsed >= m.timeSpan {
			m.stopPlayback()
			return m, nil
		}
	} else {
		m.dirty = m.playColumn != -1
		m.playColumn = -1
	}

	// Continue ticking if still playing
	if m.wavPlayer.Running() {
		return m, tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
			return tickMsg{}
		})
	}
	return m.viewDirty()
}

func (m *wavUIModel) viewDirty() (tea.Model, tea.Cmd) {
	// Ensure a reread happens even without a tick to clear the highlight on stop
	if m.dirty {
		m.loadColumnData()
	}
	return m, nil
}

func (m *wavUIModel) updateOffsetInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		kpStr := msg.String()
		switch kpStr {
		case "enter":
			newOffset := parseTimeInputToSampleTick(m.offsetInput.Value(), m.segmentWindow.Offset, m.segment.SampleRate)
			m.segmentWindow.Offset = newOffset
			m.ensureVisible()
			m.offsetInputActive = false
			m.offsetInput.Reset()
			m.dirty = true
			return m, nil
		case "esc":
			m.offsetInputActive = false
			m.offsetInput.Reset()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.offsetInput, cmd = m.offsetInput.Update(msg)
	return m, cmd
}

func (m *wavUIModel) updateLengthInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		kpStr := msg.String()
		switch kpStr {
		case "enter":
			newLength := parseTimeInputToSampleTick(m.lengthInput.Value(), m.segmentWindow.Length, m.segment.SampleRate)
			m.segmentWindow.Length = newLength
			m.ensureVisible()
			m.lengthInputActive = false
			m.lengthInput.Reset()
			m.dirty = true
			return m, nil
		case "esc":
			m.lengthInputActive = false
			m.lengthInput.Reset()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.lengthInput, cmd = m.lengthInput.Update(msg)
	return m, cmd
}

func (m *wavUIModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kpStr := msg.String()

	switch kpStr {
	case "ctrl+c":
		return m, tea.Quit
	case "p":
		return m.handlePlayPress()
	case "o":
		m.offsetInputActive = true
		m.offsetInput = newTextInput("MM:SS.mmmm")
		m.offsetInput.SetValue(FormatSampleTick(m.segmentWindow.Offset, m.segment.SampleRate))
		m.offsetInput.Focus()
		m.dirty = true
		return m, nil
	case "l":
		m.lengthInputActive = true
		m.lengthInput = newTextInput("MM:SS.mmmm")
		m.lengthInput.SetValue(FormatSampleTick(m.segmentWindow.Length, m.segment.SampleRate))
		m.lengthInput.Focus()
		m.dirty = true
		return m, nil
	case "[":
		// Subtract from offset based on current zoom
		m.segmentWindow.Offset -= SampleTick(m.samplesPerColumn())
		m.segmentWindow.Offset = max(m.segmentWindow.Offset, 0)
		m.ensureVisible()
		m.dirty = true
	case "]":
		// Add to offset based on current zoom
		m.segmentWindow.Offset += SampleTick(m.samplesPerColumn())
		maxOffset := m.segment.Samples - (SampleTick(m.samplesPerColumn()) * SampleTick(m.width))
		m.segmentWindow.Offset = min(m.segmentWindow.Offset, maxOffset)
		m.segmentWindow.Offset = max(m.segmentWindow.Offset, 0)
		m.ensureVisible()
		m.dirty = true
	case "{":
		// Subtract from length based on current zoom
		m.segmentWindow.Length -= SampleTick(m.samplesPerColumn())
		m.segmentWindow.Length = max(m.segmentWindow.Length, 0)
		m.ensureVisible()
		m.dirty = true
	case "}":
		// Add to length based on current zoom
		m.segmentWindow.Length += SampleTick(m.samplesPerColumn())
		m.segmentWindow.Length = min(m.segmentWindow.Length, m.segment.Samples)
		m.ensureVisible()
		m.dirty = true
	}
	return m, nil
}

func (m *wavUIModel) handlePlayPress() (tea.Model, tea.Cmd) {
	if m.wavPlayer.Running() {
		m.stopPlayback()
		return m, nil
	}

	// Seek to the beginning of the wavui window
	startSample := int(m.segmentWindow.Offset)
	if err := m.wavPlayer.reader.Seek(startSample, io.SeekStart); err != nil {
		log.Printf("failed to seek wav reader: %v", err)
		return m, nil
	}

	// Calculate timeout based on window length
	windowDuration := time.Duration(float64(m.segmentWindow.Length) / float64(m.segment.SampleRate) * float64(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), windowDuration)

	// Start playing
	m.state.Play.Start()
	m.wavPlayer.Play(ctx, m.segmentWindow.Offset)
	m.playStartTime = time.Now()

	// Store cancel function to stop playback later
	m.wavPlayer.cancel = cancel

	return m, tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *wavUIModel) stopPlayback() {
	m.wavPlayer.Stop()
	m.state.Play.Stop()
	m.playColumn = -1
}

func (m *wavUIModel) ensureVisible() {
	if m.segment == nil {
		return
	}
	// Ensure the visible range (offset to offset+length) is within the segment
	if m.segmentWindow.Offset+m.segmentWindow.Length > m.segment.Samples {
		if m.segmentWindow.Length > m.segment.Samples {
			m.segmentWindow.Length = m.segment.Samples
			m.segmentWindow.Offset = 0
		} else {
			m.segmentWindow.Offset = m.segment.Samples - m.segmentWindow.Length
		}
	}
	if m.segmentWindow.Offset < 0 {
		m.segmentWindow.Offset = 0
	}
}

func (m *wavUIModel) samplesPerColumn() float64 {
	samplesPerColumn := float64(m.segmentWindow.Length) / float64(m.width)
	if samplesPerColumn < 1 {
		samplesPerColumn = 1
	}
	return samplesPerColumn
}

// loadColumnData reads only the samples needed for the current view and computes max values per column
func (m *wavUIModel) loadColumnData() {
	if m.wavReader == nil || m.segment == nil || m.width <= 0 || m.segmentWindow.Length <= 0 {
		m.columnMaxValues = nil
		m.columnMinValues = nil
		m.columnMaxSamples = nil
		log.Printf("failed to load column data")
		return
	}

	samplesPerColumn := m.samplesPerColumn()

	// Buffer for reading samples - read in chunks
	// We need to read samples from offset to offset+length
	startSample := int(m.segmentWindow.Offset)
	endSample := int(m.segmentWindow.Offset + m.segmentWindow.Length)
	endSample = min(endSample, int(m.segment.Samples))
	numSamples := endSample - startSample
	if numSamples <= 0 {
		return
	}

	// Seek to the start position
	if err := m.wavReader.Seek(startSample, io.SeekStart); err != nil {
		log.Printf("failed to seek on wav %v", err)
		return
	}

	// Allocate summary storage
	m.columnMaxValues = make([]int, m.width)
	m.columnMinValues = make([]int, m.width)
	m.columnMaxSamples = make([]int, m.width)

	// Initialize min/max values
	for col := 0; col < m.width; col++ {
		m.columnMinValues[col] = math.MaxInt32
		m.columnMaxSamples[col] = math.MinInt32
	}

	// Read samples column by column
	colSamples := make([]int, int(samplesPerColumn))
	for col := 0; col < m.width; col++ {
		// Calculate how many samples this column should have
		colStartIdx := int(float64(col) * samplesPerColumn)
		colEndIdx := int(float64(col+1) * samplesPerColumn)
		colEndIdx = min(colEndIdx, numSamples)
		colNumSamples := colEndIdx - colStartIdx
		if colNumSamples <= 0 {
			continue
		}

		if _, err := m.wavReader.Read(colSamples); err != nil && err != io.EOF {
			log.Printf("failed to read column %d samples: %v", col, err)
			break
		}

		// Find max absolute value and min/max for this column's samples
		maxVal := 0
		for _, sample := range colSamples {
			absVal := int(math.Abs(float64(sample)))
			maxVal = max(absVal, maxVal)
			m.columnMinValues[col] = min(sample, m.columnMinValues[col])
			m.columnMaxSamples[col] = max(sample, m.columnMaxSamples[col])
		}
		m.columnMaxValues[col] = maxVal
	}

	m.dirty = false
}

func (m *wavUIModel) View() tea.View {
	if m.segment == nil {
		return tea.NewView("No segment selected\n")
	}

	// Load data if dirty
	if m.dirty {
		m.loadColumnData()
	}

	var sb strings.Builder

	// Calculate time range for display
	m.timeSpan = time.Duration(float64(m.segmentWindow.Length) / float64(m.segment.SampleRate) * float64(time.Second))

	// Render active inputs
	m.renderInputs(&sb)

	m.renderWavData(&sb)
	m.renderIndicators(&sb)

	return tea.NewView(sb.String())
}

func (m *wavUIModel) renderWavData(sb *strings.Builder) {
	if len(m.columnMaxValues) == 0 {
		for i := 0; i < m.height-2; i++ {
			sb.WriteString(strings.Repeat(" ", m.width))
			sb.WriteByte('\n')
		}
		return
	}

	redStyle := lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	// Render each row
	for row := 0; row < m.height-2; row++ {
		var line strings.Builder
		for col := 0; col < m.width; col++ {
			if col < len(m.columnMaxValues) {
				char := m.getWaveformChar(col, row, m.height-2)
				if m.wavPlayer.Running() && col == m.playColumn {
					line.WriteString(redStyle.Render(char))
				} else {
					line.WriteString(char)
				}
			} else {
				line.WriteByte(' ')
			}
		}
		sb.WriteString(line.String())
		sb.WriteByte('\n')
	}
}

func (m *wavUIModel) getWaveformChar(col, row, totalRows int) string {
	// Get loudness character based on max absolute value
	if col >= len(m.columnMaxValues) {
		return " "
	}
	loudnessChar := m.getLoudnessChar(m.columnMaxValues[col])

	// If no waveform rendered but this is the center row, show a straight line
	// This handles the case where min/max values result in no rows being lit
	centerRow := totalRows / 2
	if row == centerRow && loudnessChar == " " {
		return "─"
	}

	// Get min and max for this column
	if col >= len(m.columnMinValues) || col >= len(m.columnMaxSamples) {
		return loudnessChar
	}

	minVal := float64(m.columnMinValues[col])
	maxVal := float64(m.columnMaxSamples[col])

	// Normalize sample values to [-1, 1] range
	// Assuming 16-bit samples: range is [-32768, 32767]
	const maxSample = 32768.0
	normalizedMin := minVal / maxSample
	normalizedMax := maxVal / maxSample

	// Calculate normalized position for this row
	// top row (row=0) -> 1.0, middle -> 0.0, bottom row (row=totalRows-1) -> -1.0
	if totalRows <= 1 {
		return loudnessChar
	}
	normalizedRow := 1.0 - 2.0*float64(row)/float64(totalRows-1)

	// Check if this row falls within the waveform range for this column
	if normalizedRow >= normalizedMin && normalizedRow <= normalizedMax {
		return loudnessChar
	}

	return " "
}

func (m *wavUIModel) getLoudnessChar(loudness int) string {
	// Normalize loudness to a range and pick a character
	// This assumes samples are in a reasonable range (e.g., -32768 to 32767 for 16-bit)
	const maxLoudness = 32768
	normalized := float64(loudness) / float64(maxLoudness)
	switch {
	case normalized >= 0.8:
		return "█"
	case normalized >= 0.6:
		return "▓"
	case normalized >= 0.4:
		return "▒"
	case normalized >= 0.2:
		return "░"
	default:
		return " "
	}
}

func (m *wavUIModel) renderInputs(sb *strings.Builder) {
	if !m.offsetInputActive && !m.lengthInputActive {
		return
	}
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)

	if m.offsetInputActive {
		sb.WriteString(inputStyle.Render("Offset: " + m.offsetInput.View()))
		sb.WriteString("\n")
	}
	if m.lengthInputActive {
		sb.WriteString(inputStyle.Render("Length: " + m.lengthInput.View()))
		sb.WriteString("\n")
	}
}

func (m *wavUIModel) renderIndicators(sb *strings.Builder) {
	// Bottom indicator line
	var indicatorLine strings.Builder

	fmt.Fprintf(&indicatorLine, "Segment: %s", filepath.Base(m.segment.Path))

	// Show playback status
	if m.wavPlayer.Running() {
		playElapsed := time.Since(m.playStartTime)
		elapsedStr := formatDuration(playElapsed)
		windowStr := formatDuration(m.timeSpan)
		fmt.Fprintf(&indicatorLine, " | [%s/%s]", elapsedStr, windowStr)
	}

	// Show current offset and length as time
	offsetTime := FormatSampleTick(m.segmentWindow.Offset, m.segment.SampleRate)
	lengthTime := FormatSampleTick(m.segmentWindow.Length, m.segment.SampleRate)
	fmt.Fprintf(&indicatorLine, "\nOff: %s | Len: %s |", offsetTime, lengthTime)

	// Show zoom level
	fmt.Fprintf(&indicatorLine, " (%d samples)", m.segmentWindow.Length)
	sb.WriteString(indicatorLine.String())
}
