package main

import (
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputNewTrack
	inputEditName
	inputSaveConfirm
	inputTime
)

type focusArea int

const (
	focusTracks focusArea = iota
	focusAudio
	focusPosition
	focusSegments
	focusWav
)

var focusBorderColor = lipgloss.Color("#FFFFFF")
var unfocusBorderColor = lipgloss.Color("#000040")

type trackUIModel struct {
	state          *State
	trackList      *trackListModel
	nameInput      textinput.Model
	timeInput      textinput.Model
	inputMode      inputMode
	editIndex      int
	ticking        bool
	focused        focusArea
	currentRecPath string
	width          int
	height         int
	trackAudio     *trackAudioModel
	segmentList    *SegmentListModel
	wavUI          *wavUIModel
}

const tickUpdateRate = 250 * time.Millisecond

type tickMsg struct{}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 32
	ti.Placeholder = placeholder
	return ti
}

func initTrackUIModel(s *State) *trackUIModel {
	return &trackUIModel{
		state:      s,
		trackList:  newTrackListModel(s),
		nameInput:  newTextInput("Enter track name"),
		focused:    focusTracks,
		trackAudio: NewTrackAudioModel(s),
	}
}

func (m *trackUIModel) borderColorFor(focus focusArea) color.Color {
	if m.focused == focus && m.inputMode == inputNone {
		return focusBorderColor
	}
	if m.inputMode != inputNone {
		return focusBorderColor
	}
	return unfocusBorderColor
}

func (m *trackUIModel) Init() tea.Cmd {
	return nil
}

func (m *trackUIModel) toggleRecord() {
	m.stopRecording()
	m.trackList.toggleRecord()
	if m.state.RecordTrack != nil && m.ticking {
		m.startRecording()
	}
}

func (m *trackUIModel) startRecording() {
	if m.state.RecordTrack == nil || m.state.Recording() {
		return
	}
	path := m.state.getRecordingPath()
	if path == "" {
		return
	}
	if err := m.state.rec.Start(path); err != nil {
		log.Printf("error starting recording: %v\n", err)
		return
	}
	m.currentRecPath = path
}

func (m *trackUIModel) stopRecording() {
	if !m.state.Recording() {
		return
	}
	if err := m.state.rec.Stop(); err != nil {
		log.Printf("error stopping recording: %v\n", err)
	}
	if m.currentRecPath != "" && m.state.RecordTrack != nil {
		m.state.RecordTrack.AddSegment(&Segment{Path: m.currentRecPath})
		m.currentRecPath = ""
	}
}

func (m *trackUIModel) startInput(mode inputMode) {
	m.inputMode = mode
	switch mode {
	case inputNewTrack, inputEditName:
		m.nameInput = newTextInput("Enter track name")
		if mode == inputEditName {
			m.nameInput.Placeholder = "Edit track name"
			selectedTrack := m.trackList.getSelectedTrack()
			if selectedTrack != nil {
				m.nameInput.SetValue(selectedTrack.Name)
			}
		}
		m.nameInput.Focus()
	case inputTime:
		m.timeInput = newTextInput("MM:SS.mmmm")
		m.timeInput.SetValue(m.formatPosition())
		m.timeInput.Focus()
	}
}

func (m *trackUIModel) confirmInput() {
	name := m.nameInput.Value()
	if name == "" {
		m.cancelInput()
		return
	}
	switch m.inputMode {
	case inputNewTrack:
		m.state.tracks.add(name)
	case inputEditName:
		m.trackList.renameTrackIfValid(name)
	}
	m.inputMode = inputNone
	m.nameInput.Reset()
	m.trackList.refresh()
}

func (m *trackUIModel) confirmTimeInput() bool {
	input := m.timeInput.Value()
	newPos := parseTimeInputWithBase(input, m.state.clock.Position())
	m.state.clock.SetPosition(newPos)
	return true
}

func (m *trackUIModel) cancelInput() {
	m.inputMode = inputNone
	m.nameInput.Reset()
	m.timeInput.Reset()
}

func (m *trackUIModel) openSegmentList() {
	selectedTrack := m.trackList.getSelectedTrack()
	if selectedTrack == nil {
		return
	}
	m.segmentList = NewSegmentListModel(m.state, selectedTrack, m.width-16, 8)
	m.focused = focusSegments
}

func (m *trackUIModel) closeSegmentList() {
	m.segmentList = nil
	m.focused = focusTracks
}

func (m *trackUIModel) openWavUI() tea.Cmd {
	if m.segmentList == nil {
		return nil
	}
	seg := m.segmentList.SelectedSegment()
	if seg == nil {
		return nil
	}
	selectedTrack := m.trackList.getSelectedTrack()
	if selectedTrack == nil {
		return nil
	}
	m.wavUI = NewWavUIModelFromSegment(m.state, selectedTrack, seg)
	m.focused = focusWav
	return m.wavUI.Init()
}

func (m *trackUIModel) closeWavUI() {
	if m.wavUI != nil {
		m.wavUI.Close()
		m.wavUI = nil
	}
	m.focused = focusTracks
}

func (m *trackUIModel) confirmSave() {
	m.state.Save()
	m.inputMode = inputNone
}

func (m *trackUIModel) togglePlayback() {
	m.ticking = !m.ticking
	if m.ticking {
		m.state.clock.Start()
		m.startRecording()
	} else {
		m.state.clock.Stop()
		m.stopRecording()
	}
}

func (m *trackUIModel) positionLeft() {
	if !m.state.Recording() {
		m.state.clock.Seek(m.state.clock.Ticks(-5 * time.Second))
	}
}

func (m *trackUIModel) positionRight() {
	if !m.state.Recording() {
		m.state.clock.Seek(m.state.clock.Ticks(5 * time.Second))
	}
}

func (m *trackUIModel) positionHome() {
	if !m.state.Recording() {
		m.state.clock.Reset()
	}
}

func (m *trackUIModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if cmd, done := m.handleInputMsg(msg); done {
		return m, cmd
	}
	if m.trackAudio.inputActive || m.trackAudio.segmentListActive {
		_, cmd := m.trackAudio.Update(msg)
		return m, cmd
	}

	kpStr := msg.String()
	if kpStr != "esc" && m.focused == focusWav {
		_, cmd := m.wavUI.Update(msg)
		return m, cmd
	}

	switch kpStr {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+s":
		m.inputMode = inputSaveConfirm
		return m, nil
	case "esc":
		switch m.focused {
		case focusSegments:
			m.closeSegmentList()
		case focusWav:
			m.closeWavUI()
		default:
			return m, tea.Quit
		}
		return m, nil
	case "p":
		m.togglePlayback()
		if m.ticking {
			return m, m.tick()
		}
		return m, nil
	case "tab":
		m.cycleFocus()
		return m, nil
	}

	switch m.focused {
	case focusSegments:
		if kpStr == "enter" {
			return m, m.openWavUI()
		} else {
			_, cmd := m.segmentList.Update(msg)
			return m, cmd
		}
	case focusTracks:
		m.handleTracksKey(msg)
	case focusAudio:
		_, cmd := m.handleAudioKey(msg)
		return m, cmd
	case focusPosition:
		m.handlePositionKey(kpStr)
	case focusWav:
		_, cmd := m.handleWavKey(msg)
		return m, cmd
	}
	return m, nil
}

func (m *trackUIModel) handleInputMsg(msg tea.KeyPressMsg) (cmd tea.Cmd, done bool) {
	if m.inputMode == inputNone {
		return nil, false
	}
	if msg.String() == "esc" {
		m.cancelInput()
		return nil, true
	}
	isEnter := msg.String() == "enter"
	switch m.inputMode {
	case inputTime:
		m.timeInput, cmd = m.timeInput.Update(msg)
		if isEnter {
			if m.confirmTimeInput() {
				m.inputMode = inputNone
				m.timeInput.Reset()
			}
		}
	case inputNewTrack, inputEditName:
		m.nameInput, cmd = m.nameInput.Update(msg)
		if isEnter {
			m.confirmInput()
		}
	case inputSaveConfirm:
		switch msg.String() {
		case "y":
			m.confirmSave()
		case "n":
			m.inputMode = inputNone
		}
	default:
		return nil, false
	}
	return cmd, true
}

func (m *trackUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate window size to trackAudioModel
		// Account for track list width and borders
		msg2 := msg
		msg2.Width -= 16
		_, audioCmd := m.trackAudio.Update(msg2)
		if m.segmentList != nil {
			m.segmentList.list.SetSize(m.width-16, m.height-4)
		}
		if m.wavUI != nil {
			m.wavUI.Update(msg2)
		}
		return m, audioCmd
	case tickMsg:
		if m.ticking {
			m.state.clock.Update()
			return m, m.tick()
		}
		if m.focused == focusWav {
			_, cmd := m.wavUI.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

func (m *trackUIModel) formatPosition() string {
	return formatDuration(m.state.clock.Position())
}

func (m *trackUIModel) renderPositionArea(sb *strings.Builder) {
	// Position area - square border
	posBorderColor := m.borderColorFor(focusPosition)
	posBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(posBorderColor).
		Padding(0, 1)

	switch m.inputMode {
	case inputSaveConfirm:
		sb.WriteString(posBorder.Render("Save? (y/n)"))
	case inputTime:
		sb.WriteString(posBorder.Render(m.timeInput.View()))
	case inputEditName:
		sb.WriteString(posBorder.Render(m.nameInput.View()))
	default:
		playIcon := "[]"
		if m.ticking {
			playIcon = ">|"
		}
		posText := fmt.Sprintf("Position (%s): %s", playIcon, m.formatPosition())
		sb.WriteString(posBorder.Render(posText))
	}
}

func (m *trackUIModel) renderRightPane() string {
	// Right pane - segment list, wavUI, or track audio
	var rightPane string
	rightBorderColor := unfocusBorderColor
	if m.inputMode == inputNone && (m.focused == focusAudio || m.focused == focusSegments || m.focused == focusWav) {
		rightBorderColor = focusBorderColor
	}
	rightBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(rightBorderColor)

	switch m.focused {
	case focusSegments:
		rightPane = rightBorder.Render(m.segmentList.View().Content)
	case focusWav:
		rightPane = rightBorder.Render(m.wavUI.View().Content)
	default:
		rightPane = rightBorder.Render(m.trackAudio.View().Content)
	}
	return rightPane
}

func (m *trackUIModel) renderLeftPane() string {
	// Track list box - square corners
	borderColor := m.borderColorFor(focusTracks)
	trackBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)
	return trackBorder.Render(m.trackList.View())
}

func (m *trackUIModel) View() tea.View {
	var sb strings.Builder
	// Join track list and right pane side-by-side
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderLeftPane(),
		m.renderRightPane(),
	))
	sb.WriteString("\n")
	m.renderPositionArea(&sb)
	return tea.NewView(sb.String())
}

func (m *trackUIModel) tick() tea.Cmd {
	return tea.Tick(tickUpdateRate, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func TrackUI(s *State) {
	m := initTrackUIModel(s)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error running UI: %v\n", err)
	}
}

func (m *trackUIModel) cycleFocus() {
	switch m.focused {
	case focusTracks:
		m.focused = focusAudio
	case focusAudio:
		m.focused = focusPosition
	case focusPosition:
		m.focused = focusTracks
	}
}

func (m *trackUIModel) handleTracksKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "s":
		m.openSegmentList()
	case "e":
		m.startInput(inputEditName)
	case "r":
		m.toggleRecord()
	case "m":
		m.trackList.toggleMute()
	case "n":
		m.startInput(inputNewTrack)
	default:
		m.trackList.Update(msg)
	}
}

func (m *trackUIModel) handleAudioKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Route to trackAudio - use Update if input active or segment list active, otherwise handleKey
	if m.trackAudio.inputActive || m.trackAudio.segmentListActive {
		_, cmd := m.trackAudio.Update(msg)
		return m, cmd
	}
	_, cmd := m.trackAudio.handleKey(msg)
	return m, cmd
}

func (m *trackUIModel) handlePositionKey(kpStr string) {
	switch kpStr {
	case "left":
		m.positionLeft()
	case "right":
		m.positionRight()
	case "home":
		m.positionHome()
	case "t":
		m.startInput(inputTime)
	}
}

func (m *trackUIModel) handleWavKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.wavUI != nil {
		_, cmd := m.wavUI.Update(msg)
		return m, cmd
	}
	return m, nil
}
