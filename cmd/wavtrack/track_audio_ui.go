package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	trackAudioHeaderForeground = lipgloss.Color("#FFFFFF")
	trackAudioHeaderBackground = lipgloss.Color("#0000FF")
	trackAudioSelectedSegment  = lipgloss.Color("#FF0000")
	trackAudioSegment          = lipgloss.Color("#00FF00")
	trackAudioStatusForeground = lipgloss.Color("#FFFFFF")
)

type trackAudioModel struct {
	state                *State
	width                int
	timeSpan             time.Duration // Total time range displayed
	startTime            time.Duration // Start of the time range
	selectedTrackIndex   int
	selectedSegmentIndex int
	zoomRatio            float64
	timeInput            textinput.Model
	inputActive          bool
	segmentList          list.Model
	segmentListActive    bool
	fixedView            bool
}

func NewTrackAudioModel(s *State) *trackAudioModel {
	return &trackAudioModel{
		state:                s,
		width:                80, // Default width
		timeSpan:             10 * time.Second,
		startTime:            0,
		selectedTrackIndex:   0,
		selectedSegmentIndex: 0,
		zoomRatio:            1.0,
		timeInput:            newTextInput("MM:SS.mmmm"),
		segmentList:          list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
	}
}

type segmentItem struct {
	segment *Segment
	index   int
}

func (i segmentItem) Title() string {
	return filepath.Base(i.segment.Path)
}

func (i segmentItem) Description() string {
	return fmt.Sprintf("%v", i.segment.Duration)
}

func (i segmentItem) FilterValue() string {
	return filepath.Base(i.segment.Path)
}

func (m *trackAudioModel) buildSegmentListItems() []list.Item {
	if !m.hasSelectedTrack() {
		return []list.Item{}
	}
	track := &m.state.tracks.Tracks[m.selectedTrackIndex]
	items := make([]list.Item, len(track.segmentStore))
	for i, seg := range track.segmentStore {
		items[i] = segmentItem{segment: seg, index: i}
	}
	return items
}

func (m *trackAudioModel) initSegmentList() {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	l := list.New(m.buildSegmentListItems(), delegate, 0, 0)
	l.Title = "Segments"
	l.SetSize(m.width-4, 8)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	m.segmentList = l
}

func (m *trackAudioModel) addSegmentAtStartTime(seg *Segment) {
	if !m.hasSelectedTrack() {
		return
	}
	track := &m.state.tracks.Tracks[m.selectedTrackIndex]
	sampleRate := seg.SampleRate
	start := SampleTick(m.startTime.Seconds() * float64(sampleRate))
	length := seg.Samples
	track.AddTrackSegment(TrackSegment{
		Segment: seg,
		SegmentWindow: SegmentWindow{
			Start:  start,
			Length: length,
		},
	})
}

func (m *trackAudioModel) Init() tea.Cmd {
	return nil
}

func (m *trackAudioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.inputActive {
		return m.updateTimeInput(msg)
	}
	if m.segmentListActive {
		return m.updateSegmentList(msg)
	}
	return m.updateMain(msg)
}

func (m *trackAudioModel) hasSelectedTrack() bool {
	return len(m.state.tracks.Tracks) > 0 && m.selectedTrackIndex >= 0 && m.selectedTrackIndex < len(m.state.tracks.Tracks)
}

func (m *trackAudioModel) updateTimeInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		kpStr := msg.String()
		switch kpStr {
		case "enter":
			m.startTime = parseTimeInputWithBase(m.timeInput.Value(), m.startTime)
			m.inputActive = false
			m.timeInput.Reset()
			return m, nil
		case "esc":
			m.inputActive = false
			m.timeInput.Reset()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.timeInput, cmd = m.timeInput.Update(msg)
	return m, cmd
}

func (m *trackAudioModel) updateSegmentList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		kpStr := msg.String()
		switch kpStr {
		case "enter":
			m.segmentListActive = false
			if item := m.segmentList.SelectedItem(); item != nil {
				if segItem, ok := item.(segmentItem); ok {
					m.addSegmentAtStartTime(segItem.segment)
				}
			}
			return m, nil
		case "esc":
			m.segmentListActive = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.segmentList, cmd = m.segmentList.Update(msg)
	return m, cmd
}

func (m *trackAudioModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m *trackAudioModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kpStr := msg.String()

	switch kpStr {
	case "ctrl+c":
		return m, tea.Quit
	case "-":
		m.zoomRatio *= 2
		m.timeSpan *= 2
		if m.startTime < 0 {
			m.startTime = 0
		}
	case "+":
		m.zoomRatio /= 2
		m.timeSpan /= 2
		if m.timeSpan < 100*time.Millisecond {
			m.timeSpan = 100 * time.Millisecond
			m.zoomRatio = 0.01
		}
		if m.startTime < 0 {
			m.startTime = 0
		}
	case "left":
		m.startTime -= m.timeSpan / time.Duration(m.width)
		if m.startTime < 0 {
			m.startTime = 0
		}
	case "right":
		m.startTime += m.timeSpan / time.Duration(m.width)
	case "up", "k":
		if len(m.state.tracks.Tracks) == 0 {
			break
		}
		m.selectedTrackIndex--
		if m.selectedTrackIndex < 0 {
			m.selectedTrackIndex = len(m.state.tracks.Tracks) - 1
		}
		m.selectedSegmentIndex = 0
	case "down", "j":
		if len(m.state.tracks.Tracks) == 0 {
			break
		}
		m.selectedTrackIndex++
		if m.selectedTrackIndex >= len(m.state.tracks.Tracks) {
			m.selectedTrackIndex = 0
		}
		m.selectedSegmentIndex = 0
	case "pgdown":
		m.selectNextSegment()
	case "pgup":
		m.selectPrevSegment()
	case "t":
		m.inputActive = true
		m.timeInput = newTextInput("MM:SS.mmmm")
		m.timeInput.Focus()
	case "f":
		m.fixedView = !m.fixedView
	case "i":
		if m.hasSelectedTrack() {
			m.segmentListActive = true
			m.initSegmentList()
		}
	case "delete", "backspace":
		m.deleteSelectedSegment()
	}
	return m, nil
}

func (m *trackAudioModel) selectNextSegment() {
	if !m.hasSelectedTrack() {
		return
	}
	selectedTrack := &m.state.tracks.Tracks[m.selectedTrackIndex]
	if len(selectedTrack.Segments) == 0 {
		return
	}
	m.selectedSegmentIndex++
	if m.selectedSegmentIndex >= len(selectedTrack.Segments) {
		m.selectedSegmentIndex = 0
	}
}

func (m *trackAudioModel) selectPrevSegment() {
	if !m.hasSelectedTrack() {
		return
	}
	selectedTrack := &m.state.tracks.Tracks[m.selectedTrackIndex]
	if len(selectedTrack.Segments) == 0 {
		return
	}
	m.selectedSegmentIndex--
	if m.selectedSegmentIndex < 0 {
		m.selectedSegmentIndex = len(selectedTrack.Segments) - 1
	}
}

func (m *trackAudioModel) hasSelectedSegment(track *Track) bool {
	return len(track.Segments) > 0 && m.selectedSegmentIndex >= 0 && m.selectedSegmentIndex < len(track.Segments)
}

func (m *trackAudioModel) deleteSelectedSegment() {
	if !m.hasSelectedTrack() {
		return
	}
	track := &m.state.tracks.Tracks[m.selectedTrackIndex]
	if !m.hasSelectedSegment(track) {
		return
	}
	// Get the Start time of the segment to remove
	start := track.Segments[m.selectedSegmentIndex].Start
	// Remove the segment using RemoveTrackSegment
	track.RemoveTrackSegment(start)
	// Adjust selectedSegmentIndex if needed
	if m.selectedSegmentIndex >= len(track.Segments) {
		m.selectedSegmentIndex = len(track.Segments) - 1
	}
}

type renderWindow struct {
	start time.Duration
	end   time.Duration
	width int
}

func (m *trackAudioModel) View() tea.View {
	if len(m.state.tracks.Tracks) == 0 {
		return tea.NewView("No tracks available\n")
	}

	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Foreground(trackAudioHeaderForeground).
		Background(trackAudioHeaderBackground)
	sb.WriteString(headerStyle.Render("Song"))
	sb.WriteString("\n\n")

	// Calculate time range
	startTime := m.startTime
	endTime := m.startTime + m.timeSpan

	// Auto-scroll: if clock is running and position is out of view, scroll to it
	// Skip if fixedView is enabled
	clockPos := m.state.clock.Position()
	if !m.fixedView && m.state.clock.Running() && (clockPos < startTime || clockPos > endTime) {
		startTime = clockPos
		endTime = clockPos + m.timeSpan
		m.startTime = startTime
	}

	// Render each track
	rw := renderWindow{
		start: startTime,
		end:   endTime,
		width: m.width - 2,
	}
	for i, track := range m.state.tracks.Tracks {
		// Track selection indicator
		indicator := "- "
		if i == m.selectedTrackIndex {
			indicator = "* "
		}
		sb.WriteString(indicator)
		m.renderTrackSegments(&sb, &track, i == m.selectedTrackIndex, &rw)
		sb.WriteString("\n")
	}

	// Render caret line
	m.renderCaretLine(&sb, startTime, endTime, &rw)

	// Separator line
	sb.WriteString("\n")

	// Time input box (just above the status line)
	if m.inputActive {
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)
		sb.WriteString(inputStyle.Render(m.timeInput.View()))
		sb.WriteString("\n")
	}

	// Segment list
	if m.segmentListActive {
		sb.WriteString(m.segmentList.View())
		sb.WriteString("\n")
	}

	// Bottom status line
	statusLine := m.renderStatusLine(startTime, endTime)
	sb.WriteString(statusLine)

	return tea.NewView(sb.String())
}

type blockRange struct {
	pos   int
	width int
}

func (m *trackAudioModel) blockRangeFromTrackSegment(seg *TrackSegment, rw *renderWindow) blockRange {
	totalRange := rw.Range()
	if totalRange <= 0 {
		return blockRange{}
	}

	// Convert segment offset and length to time.Duration
	sampleRate := float64(seg.Segment.SampleRate)
	segStartTime := time.Duration(float64(seg.Start) / sampleRate * float64(time.Second))
	segDuration := time.Duration(float64(seg.Length) / sampleRate * float64(time.Second))
	segEndTime := segStartTime + segDuration

	// Check if segment overlaps with visible time range
	if segEndTime < rw.start || segStartTime > rw.end {
		return blockRange{}
	}

	// Calculate segment position within the time range
	visibleStart, visibleEnd := segStartTime, segEndTime
	if segStartTime < rw.start {
		visibleStart = rw.start
	}
	if segEndTime > rw.end {
		visibleEnd = rw.end
	}

	// Calculate width of segment block
	segVisibleDuration := visibleEnd - visibleStart
	width := int(float64(segVisibleDuration) / float64(totalRange) * float64(rw.width))
	width = max(width, 1)

	// Calculate position of segment block
	segOffsetFromStart := visibleStart - rw.start
	pos := int(float64(segOffsetFromStart) / float64(totalRange) * float64(rw.width))

	return blockRange{pos: pos, width: width}
}

func (rw *renderWindow) Range() time.Duration { return rw.end - rw.start }

func (m *trackAudioModel) renderTrackSegments(sb *strings.Builder, track *Track, isSelectedTrack bool, rw *renderWindow) {
	// Fill area before time 0 with half shade blocks
	if len(track.Segments) == 0 {
		sb.WriteString(strings.Repeat("░", rw.width))
		return
	}
	usedWidth := 0
	for i, seg := range track.Segments {
		// Clamp block width to ensure it doesn't exceed render window
		availableWidth := rw.width - usedWidth
		if availableWidth <= 0 {
			break
		}
		// Create the block
		br := m.blockRangeFromTrackSegment(&seg, rw)
		if br.width <= 0 {
			continue
		}
		if br.pos > usedWidth {
			// Pad to position relative to current position
			blankWidth := br.pos - usedWidth
			blankWidth = min(blankWidth, availableWidth)
			sb.WriteString(strings.Repeat("░", blankWidth))
			availableWidth -= blankWidth
			usedWidth += blankWidth
			if availableWidth <= 0 {
				break
			}
		}
		if br.pos < usedWidth {
			// Start of this segment overlaps with last segment's end.
			br.width -= usedWidth - br.pos
		}
		if br.width <= 0 {
			continue
		}
		if br.width > availableWidth {
			br.width = availableWidth
		}
		block := strings.Repeat("█", br.width)
		usedWidth += br.width

		// If this is the selected track and selected segment, highlight it
		if isSelectedTrack && i == m.selectedSegmentIndex {
			block = lipgloss.NewStyle().Foreground(trackAudioSelectedSegment).Render(block)
		} else {
			block = lipgloss.NewStyle().Foreground(trackAudioSegment).Render(block)
		}
		sb.WriteString(block)
	}
	// Pad to fill the entire width
	if usedWidth < rw.width {
		sb.WriteString(strings.Repeat("░", rw.width-usedWidth))
	}
}

func (m *trackAudioModel) renderCaretLine(sb *strings.Builder, startTime, endTime time.Duration, rw *renderWindow) {
	// Indent to align with track content (2 for indicator)
	sb.WriteString("  ")
	clockPos := m.state.clock.Position()
	if clockPos < startTime || clockPos > endTime {
		sb.WriteString(strings.Repeat(" ", rw.width))
		sb.WriteString("\n")
		return
	}
	caretPos := int(float64(clockPos-startTime) / float64(rw.Range()) * float64(rw.width))
	beforeCaret := caretPos
	afterCaret := rw.width - caretPos - 1
	sb.WriteString(strings.Repeat(" ", beforeCaret))
	sb.WriteString("^")
	sb.WriteString(strings.Repeat(" ", afterCaret))
	sb.WriteString("\n")
}

func (m *trackAudioModel) renderStatusLine(startTime, endTime time.Duration) string {
	// Format times
	formatDuration := func(d time.Duration) string {
		if d < 0 {
			return "-"
		}
		totalMs := int(d.Milliseconds())
		minutes := (totalMs / 60000) % 60
		seconds := (totalMs / 1000) % 60
		milliseconds := totalMs % 1000
		return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, milliseconds)
	}

	status := fmt.Sprintf("Range: %s to %s",
		formatDuration(startTime),
		formatDuration(endTime))

	// Add fixed view indicator
	if m.fixedView {
		status += " [F]"
	}

	// Add selected segment info
	if m.hasSelectedTrack() {
		track := &m.state.tracks.Tracks[m.selectedTrackIndex]
		status += " | Track: " + track.Name
		if m.hasSelectedSegment(track) {
			seg := track.Segments[m.selectedSegmentIndex]
			sampleRate := float64(seg.Segment.SampleRate)
			segStartTime := time.Duration(float64(seg.Start) / sampleRate * float64(time.Second))
			segEndTime := segStartTime + time.Duration(float64(seg.Length)/sampleRate*float64(time.Second))
			status += fmt.Sprintf(" | Seg[%d]: %s to %s",
				m.selectedSegmentIndex,
				formatDuration(segStartTime),
				formatDuration(segEndTime))
		}
	}

	return lipgloss.NewStyle().Foreground(trackAudioStatusForeground).Render(status)
}
