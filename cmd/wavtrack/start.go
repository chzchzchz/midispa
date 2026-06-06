package main

import (
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chzchzchz/midispa/jack"
)

type startModel struct {
	ProjectName   string
	RecordPort    string
	PlaybackPorts []string

	width  int
	height int

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
	projectStyle  = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue).Bold(true)
	playbackStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightGreen).Bold(true)
	recordStyle   = lipgloss.NewStyle().Foreground(lipgloss.BrightRed).Bold(true)
	createStyle   = lipgloss.NewStyle().Foreground(lipgloss.BrightMagenta).Bold(true)
)

func initStartModel() *startModel {
	// Get JACK ports
	ports, err := jack.Ports()
	if err != nil {
		ports = []string{"Error getting ports"}
	}

	m := &startModel{
		jackPorts: ports,
		width:     80,
		height:    24,
	}

	// Initialize project name input
	m.projectNameInput = textinput.New()
	m.projectNameInput.Placeholder = "Enter project name..."
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

func (m *startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	enter := false
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focused++
			if m.focused > focusedCreate {
				m.focused = focusedProjectName
			}
			return m, nil
		case "shift+tab":
			m.focused--
			if m.focused < focusedProjectName {
				m.focused = focusedCreate
			}
			return m, nil
		case "enter":
			enter = true
			if m.focused == focusedCreate {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd

	switch m.focused {
	case focusedProjectName:
		m.projectNameInput, cmd = m.projectNameInput.Update(msg)
		return m, cmd
	case focusedPlayback:
		m.playbackList, cmd = m.playbackList.Update(msg)
		if enter && m.playbackList.GlobalIndex() >= 0 {
			port := m.jackPorts[m.playbackList.GlobalIndex()]
			if !sliceContains(m.PlaybackPorts, port) && len(m.PlaybackPorts) < 2 {
				m.PlaybackPorts = append(m.PlaybackPorts, port)
			}
		}
		return m, cmd
	case focusedRecord:
		m.recordList, cmd = m.recordList.Update(msg)
		if enter && m.recordList.GlobalIndex() >= 0 {
			m.RecordPort = m.jackPorts[m.recordList.GlobalIndex()]
		}
		return m, cmd
	}

	return m, nil
}

func renderFocusStyle(v string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(v)
}

func renderNoFocusStyle(v string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.BrightBlack).
		Render(v)
}

func (m *startModel) View() tea.View {
	var view strings.Builder

	view.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightWhite).Render("Project Setup") + "\n")

	projectInput := m.projectNameInput.View()
	if m.focused == focusedProjectName {
		projectInput = renderFocusStyle(projectInput)
	} else {
		projectInput = renderNoFocusStyle(projectInput)
	}
	view.WriteString(projectStyle.Render("Project Name:") + "\n" + projectInput + "\n")

	playbackList := m.playbackList.View()
	if m.focused == focusedPlayback {
		playbackList = renderFocusStyle(playbackList)
	} else {
		playbackList = renderNoFocusStyle(playbackList)
	}
	view.WriteString(playbackStyle.Render("Playback Ports:") + "\n")
	view.WriteString(playbackList + "\n")

	// Display selected playback ports
	if len(m.PlaybackPorts) > 0 {
		view.WriteString(playbackStyle.Render("Selected:"))
		for i, port := range m.PlaybackPorts {
			if i > 0 {
				view.WriteString(", ")
			}
			view.WriteString(lipgloss.NewStyle().Foreground(lipgloss.BrightGreen).Render(port))
		}
		view.WriteString("\n")
	}

	// Record list
	recordList := m.recordList.View()
	if m.focused == focusedRecord {
		recordList = renderFocusStyle(recordList)
	} else {
		recordList = renderNoFocusStyle(recordList)
	}
	view.WriteString(recordStyle.Render("Record Port: ") + "\n")
	view.WriteString(recordList + "\n")

	// Display selected record port
	if m.RecordPort != "" {
		view.WriteString(recordStyle.Render("Selected: "))
		view.WriteString(lipgloss.NewStyle().Foreground(lipgloss.BrightRed).Render(m.RecordPort) + "\n\n")
	}

	// Create button
	createButton := "  Create  "
	if m.focused == focusedCreate {
		createButton = lipgloss.NewStyle().Background(lipgloss.BrightBlue).Render(createButton)
	} else {
		createButton = lipgloss.NewStyle().Foreground(lipgloss.BrightMagenta).Render(createButton)
	}
	view.WriteString(createStyle.Render("Create: ") + createButton + "\n\n")

	// Instructions
	instructions := "Tab to navigate, Enter to select, Esc to cancel"
	view.WriteString(lipgloss.NewStyle().Faint(true).Render(instructions))

	v := tea.NewView(view.String())
	v.AltScreen = true
	return v
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func Start() *startModel {
	model := initStartModel()

	p := tea.NewProgram(model /*, tea.WithAltScreen(), tea.WithMouseCellMotion()*/)
	if _, err := p.Run(); err != nil {
		panic(err)
	}

	return model
}
