package main

import (
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type startModel struct {
	ProjectConfig

	projectNameInput textinput.Model
	playbackList     list.Model
	recordList       list.Model

	focused focusType

	jackPorts []string
}

type focusType int

const (
	focusedProjectName focusType = iota
	focusedPlayback
	focusedRecord
	focusedCreate
)

var (
	createButtonStyle = lipgloss.NewStyle().Background(lipgloss.BrightBlue)
	infoStyle         = lipgloss.NewStyle().Foreground(lipgloss.Green)
	projectStyle      = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue).Bold(true)
	playbackStyle     = lipgloss.NewStyle().Foreground(lipgloss.BrightGreen).Bold(true)
	recordStyle       = lipgloss.NewStyle().Foreground(lipgloss.BrightRed).Bold(true)
	createStyle       = lipgloss.NewStyle().Foreground(lipgloss.BrightMagenta).Bold(true)
	focusedBorder     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.BrightWhite)
)

func (m *startModel) ready() bool {
	return m.projectNameInput.Value() != "" &&
		m.LeftPlayback != "" &&
		m.RightPlayback != "" &&
		m.RecordPort != ""
}

func initStartModel(ports []string) *startModel {
	m := &startModel{
		jackPorts: ports,
	}

	// Initialize project name input
	m.projectNameInput = textinput.New()
	m.projectNameInput.Focus()
	m.projectNameInput.CharLimit = 32

	// Initialize playback list (select 2 items, green)
	playbackItems := make([]list.Item, len(ports))
	for i, port := range ports {
		playbackItems[i] = item{title: port}
	}
	del := list.NewDefaultDelegate()
	del.ShowDescription = false
	del.SetSpacing(0)
	m.playbackList = list.New(playbackItems, del, 0, 0)
	m.playbackList.Title = "Playback Ports (select 2)"
	m.playbackList.SetSize(32, 16)

	// Initialize record list (select 1 item, red)
	recordItems := make([]list.Item, len(ports))
	for i, port := range ports {
		recordItems[i] = item{title: port}
	}
	m.recordList = list.New(recordItems, del, 0, 0)
	m.recordList.Title = "Record Port (select 1)"
	m.recordList.SetSize(32, 16)

	m.focused = focusedProjectName

	return m
}

type item struct {
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.title }

func (m *startModel) Init() tea.Cmd {
	return nil
}

func (m *startModel) config() string {
	var s strings.Builder
	s.WriteString(infoStyle.Render("Project: "+m.projectNameInput.Value()) + "\n")

	l, r := "(none)", "(none)"
	if m.LeftPlayback != "" {
		l = m.LeftPlayback
	}
	if m.RightPlayback != "" {
		r = m.RightPlayback
	}
	s.WriteString(infoStyle.Render("Playback: L:"+l+", R:"+r) + "\n")
	s.WriteString(infoStyle.Render("Record: " + m.RecordPort))
	return focusedBorder.Render(s.String())
}

func (m *startModel) next() {
	m.focused++
	if !m.ready() && m.focused == focusedCreate {
		m.next()
	}
	if m.focused > focusedCreate {
		m.focused = focusedProjectName
	}
}

func (m *startModel) prev() {
	m.focused--
	if !m.ready() && m.focused == focusedCreate {
		m.prev()
		return
	}
	if m.focused < focusedProjectName {
		m.focused = focusedCreate
	}
}

func (m *startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	kpStr := ""
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		kpStr = msg.String()
		switch kpStr {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.next()
			return m, nil
		case "shift+tab":
			m.prev()
			return m, nil
		case "enter":
			if m.focused == focusedCreate && m.ready() {
				m.ProjectName = m.projectNameInput.Value()
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd

	switch m.focused {
	case focusedProjectName:
		m.projectNameInput, cmd = m.projectNameInput.Update(msg)
	case focusedPlayback:
		// Intercept l/r keys before list processes them
		if m.playbackList.GlobalIndex() < 0 {
			break
		}
		filtering := m.playbackList.FilterState() == list.Filtering
		if !filtering {
			port := m.jackPorts[m.playbackList.GlobalIndex()]
			if kpStr == "l" {
				if m.LeftPlayback == port {
					m.LeftPlayback = ""
				} else {
					m.LeftPlayback = port
				}
				break
			} else if kpStr == "r" {
				if m.RightPlayback == port {
					m.RightPlayback = ""
				} else {
					m.RightPlayback = port
				}
				break
			}
		}
		m.playbackList, cmd = m.playbackList.Update(msg)
		if filtering {
			return m, cmd
		}
	case focusedRecord:
		m.recordList, cmd = m.recordList.Update(msg)
		if m.recordList.GlobalIndex() < 0 {
			break
		}
		if kpStr == "space" || kpStr == "enter" {
			port := m.jackPorts[m.recordList.GlobalIndex()]
			if m.RecordPort == port {
				m.RecordPort = ""
			} else {
				m.RecordPort = port
			}
		}
	}

	if kpStr == "enter" {
		m.next()
	}
	return m, cmd
}

func (m *startModel) View() tea.View {
	var view strings.Builder

	view.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightWhite).Render("Project Setup") + "\n\n")

	switch m.focused {
	case focusedProjectName:
		projectInput := m.projectNameInput.View()
		view.WriteString(projectStyle.Render("Project Name:") + "\n")
		view.WriteString(focusedBorder.Render(projectInput) + "\n\n")
		view.WriteString(m.config() + "\n\n")

	case focusedPlayback:
		playbackList := m.playbackList.View()
		view.WriteString(playbackStyle.Render("Playback Ports:") + "\n")
		view.WriteString(focusedBorder.Render(playbackList) + "\n\n")
		view.WriteString(m.config() + "\n\n")

	case focusedRecord:
		recordList := m.recordList.View()
		view.WriteString(recordStyle.Render("Record Port:") + "\n")
		view.WriteString(focusedBorder.Render(recordList) + "\n\n")
		view.WriteString(m.config() + "\n\n")

	case focusedCreate:
		view.WriteString(m.config() + "\n\n")
		view.WriteString(createButtonStyle.Render("  Create  ") + "\n\n")
	}

	view.WriteString(lipgloss.NewStyle().Faint(true).Render("Tab: next  L/R: select left/right  Enter: confirm/next  Ctrl+C: quit"))

	return tea.NewView(view.String())
}

func Start(ports []string) *startModel {
	model := initStartModel(ports)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
	return model
}
