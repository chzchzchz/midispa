package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
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
)

type focusArea int

const (
	focusTracks focusArea = iota
	focusPosition
)

var focusBorderColor = lipgloss.Color("#FFFFFF")
var unfocusBorderColor = lipgloss.Color("#000040")

type trackUIModel struct {
	state          *State
	list           list.Model
	nameInput      textinput.Model
	inputMode      inputMode
	editIndex      int
	ticking        bool
	focused        focusArea
	currentRecPath string
}

type trackItem struct {
	track    *Track
	isRecord bool
}

func (i trackItem) Title() string {
	m, r, s := " ", " ", false
	if i.track.Mute {
		m, s = "M", true
	}
	if i.isRecord {
		r, s = "R", true
	}
	suffix := "     "
	if s {
		suffix = fmt.Sprintf(" [%s%s]", m, r)
	}
	return i.track.Name + suffix
}

func (i trackItem) Description() string {
	return ""
}

func (i trackItem) FilterValue() string {
	return i.track.Name
}

func buildListItems(s *State) []list.Item {
	items := make([]list.Item, len(s.tracks.Tracks))
	for i := range s.tracks.Tracks {
		isRecord := s.RecordTrack != nil && &s.tracks.Tracks[i] == s.RecordTrack
		items[i] = trackItem{track: &s.tracks.Tracks[i], isRecord: isRecord}
	}
	return items
}

func newList(items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	l.Title = "Tracks"
	l.SetSize(64, 16)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	return l
}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 32
	ti.Placeholder = placeholder
	return ti
}

func initTrackUIModel(s *State) *trackUIModel {
	items := buildListItems(s)
	l := newList(items)
	return &trackUIModel{
		state:     s,
		list:      l,
		nameInput: newTextInput("Enter track name"),
		focused:   focusTracks,
	}
}

func (m *trackUIModel) Init() tea.Cmd {
	return nil
}

func (m *trackUIModel) refreshList() {
	m.list.SetItems(buildListItems(m.state))
}

func (m *trackUIModel) toggleMute() {
	idx := m.list.Index()
	if idx >= 0 && idx < len(m.state.tracks.Tracks) {
		m.state.tracks.Tracks[idx].Mute = !m.state.tracks.Tracks[idx].Mute
		m.refreshList()
	}
}

func (m *trackUIModel) toggleRecord() {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.state.tracks.Tracks) {
		return
	}
	m.stopRecording()

	if m.state.RecordTrack == &m.state.tracks.Tracks[idx] {
		m.state.RecordTrack = nil
	} else {
		// Turning on recording for a new track
		m.state.RecordTrack = &m.state.tracks.Tracks[idx]
		if m.ticking {
			m.startRecording()
		}
	}
	m.refreshList()
}

func (m *trackUIModel) startRecording() {
	if m.state.RecordTrack == nil || m.state.Recording() {
		return
	}
	path := m.state.getRecordingPath()
	if path == "" {
		return
	}
	m.currentRecPath = path
	if err := m.state.rec.Start(path); err != nil {
		fmt.Printf("error starting recording: %v\n", err)
		return
	}
}

func (m *trackUIModel) stopRecording() {
	if !m.state.Recording() {
		return
	}
	if err := m.state.rec.Stop(); err != nil {
		fmt.Printf("error stopping recording: %v\n", err)
	}
	if m.currentRecPath != "" && m.state.RecordTrack != nil {
		m.state.RecordTrack.AddSegment(&Segment{Path: m.currentRecPath})
		m.currentRecPath = ""
	}
}

func (m *trackUIModel) startInput(mode inputMode) {
	m.inputMode = mode
	switch mode {
	case inputNewTrack:
		m.nameInput = newTextInput("Enter track name")
	case inputEditName:
		m.nameInput = newTextInput("Edit track name")
		idx := m.list.Index()
		if idx >= 0 && idx < len(m.state.tracks.Tracks) {
			m.nameInput.SetValue(m.state.tracks.Tracks[idx].Name)
		}
	}
	m.nameInput.Focus()
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
		m.renameTrackIfValid(name)
	}
	m.inputMode = inputNone
	m.nameInput.Reset()
	m.refreshList()
}

func (m *trackUIModel) cancelInput() {
	m.inputMode = inputNone
	m.nameInput.Reset()
}

func (m *trackUIModel) confirmSave() {
	m.state.Save()
	m.inputMode = inputNone
}

func (m *trackUIModel) cancelSave() {
	m.inputMode = inputNone
}

func (m *trackUIModel) renameTrackIfValid(name string) {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.state.tracks.Tracks) {
		return
	}
	if !m.state.tracks.rename(m.state.tracks.Tracks[idx].Name, name) {
		return
	}
}

func (m *trackUIModel) togglePlayback() {
	oldTicking := m.ticking
	m.ticking = !m.ticking

	if m.ticking {
		m.state.clock.Start()
	} else {
		m.state.clock.Stop()
	}

	// If starting playback and there's a record track, start recording
	if m.ticking && !oldTicking && m.state.RecordTrack != nil {
		m.startRecording()
	}
	// If stopping playback, stop recording
	if !m.ticking && oldTicking {
		m.stopRecording()
	}
}

func (m *trackUIModel) positionLeft() {
	if m.state.Recording() {
		return
	}
	m.state.clock.Seek(-5)
}

func (m *trackUIModel) positionRight() {
	if m.state.Recording() {
		return
	}
	m.state.clock.Seek(5)
}

func (m *trackUIModel) positionHome() {
	if m.state.Recording() {
		return
	}
	m.state.clock.Reset()
}

func (m *trackUIModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kpStr := msg.String()

	switch kpStr {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+s":
		m.inputMode = inputSaveConfirm
		return m, nil
	case "esc":
		if m.inputMode == inputSaveConfirm {
			m.cancelSave()
			return m, nil
		}
		m.focused = focusTracks
		return m, nil
	case "n":
		m.startInput(inputNewTrack)
		return m, nil
	case "p":
		m.togglePlayback()
		if m.ticking {
			return m, m.tick()
		}
		return m, nil
	case "tab":
		if m.focused == focusTracks {
			m.focused = focusPosition
		} else {
			m.focused = focusTracks
		}
		return m, nil
	}

	if m.focused == focusTracks {
		switch kpStr {
		case "e":
			m.startInput(inputEditName)
			m.editIndex = m.list.Index()
		case "r":
			m.toggleRecord()
		case "m":
			m.toggleMute()
		}
	} else if m.focused == focusPosition {
		switch kpStr {
		case "left":
			m.positionLeft()
		case "right":
			m.positionRight()
		case "home":
			m.positionHome()
		}
	}
	return m, nil
}

func (m *trackUIModel) handleInputMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "enter" {
		if m.inputMode == inputSaveConfirm {
			m.confirmSave()
		} else {
			m.confirmInput()
		}
	}
	return m, cmd
}

func (m *trackUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.inputMode != inputNone && m.inputMode != inputSaveConfirm {
			return m.handleInputMsg(msg)
		}
		if m.inputMode == inputSaveConfirm {
			switch msg.String() {
			case "y":
				m.confirmSave()
				return m, nil
			case "n", "esc":
				m.cancelSave()
				return m, nil
			}
		}
		result, cmd := m.handleKey(msg)
		if cmd != nil {
			return result, cmd
		}
		var listCmd tea.Cmd
		m.list, listCmd = m.list.Update(msg)
		return m, listCmd
	case tickMsg:
		if m.ticking {
			m.state.clock.Seek(1)
			return m, m.tick()
		}
	}

	if m.inputMode != inputNone && m.inputMode != inputSaveConfirm {
		return m.handleInputMsg(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *trackUIModel) formatPosition() string {
	dur := m.state.clock.Position()
	s := int(dur.Seconds())
	minutes := s / 60
	seconds := s % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (m *trackUIModel) View() tea.View {
	var sb strings.Builder

	// Track list box - square corners
	borderColor := unfocusBorderColor
	if m.focused == focusTracks && m.inputMode == inputNone {
		borderColor = focusBorderColor
	}
	trackBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	sb.WriteString(trackBorder.Render(m.list.View()) + "\n")

	// Position area - square border
	posBorderColor := unfocusBorderColor
	if m.focused == focusPosition && m.inputMode == inputNone {
		posBorderColor = focusBorderColor
	}
	if m.inputMode != inputNone {
		posBorderColor = lipgloss.Color("#FFFFFF")
	}
	posBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(posBorderColor).
		Padding(0, 1)

	if m.inputMode != inputNone {
		if m.inputMode == inputSaveConfirm {
			sb.WriteString(posBorder.Render("Save? (y/n)"))
		} else {
			sb.WriteString(posBorder.Render(m.nameInput.View()))
		}
	} else {
		playIcon := "[]"
		if m.ticking {
			playIcon = "|>"
		}
		posText := fmt.Sprintf("Position (%s): %s", playIcon, m.formatPosition())
		sb.WriteString(posBorder.Render(posText))
	}

	return tea.NewView(sb.String())
}

func (m *trackUIModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type tickMsg struct{}

func TrackUI(s *State) {
	m := initTrackUIModel(s)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error running UI: %v\n", err)
	}
}
