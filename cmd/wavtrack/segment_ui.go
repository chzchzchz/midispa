package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type segmentListItem struct {
	segment      *Segment
	segmentIndex int
	trackIndex   int
}

func (i segmentListItem) Title() string {
	return filepath.Base(i.segment.Path)
}

func (i segmentListItem) Description() string {
	return fmt.Sprintf("%v", i.segment.Duration)
}

func (i segmentListItem) FilterValue() string {
	return filepath.Base(i.segment.Path)
}

type SegmentListModel struct {
	state         *State
	track         *Track
	width         int
	height        int
	list          list.Model
	nameInput     textinput.Model
	inputActive   bool
	selectedIndex int
	focussed      bool
}

func NewSegmentListModel(s *State, track *Track, width, height int) *SegmentListModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	items := buildSegmentListItems(s, track)
	l := list.New(items, delegate, 0, 0)
	l.Title = "Segments"
	l.SetSize(width, height)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	l.DisableQuitKeybindings()

	return &SegmentListModel{
		state:       s,
		track:       track,
		width:       width,
		height:      height,
		list:        l,
		nameInput:   newTextInput("New segment name"),
		inputActive: false,
		focussed:    true,
	}
}

func buildSegmentListItems(_ *State, track *Track) []list.Item {
	if track == nil {
		return []list.Item{}
	}
	items := make([]list.Item, len(track.segmentStore))
	for i, seg := range track.segmentStore {
		items[i] = segmentListItem{
			segment:      seg,
			segmentIndex: i,
			trackIndex:   0, // Will need to be set properly
		}
	}
	return items
}

func (m *SegmentListModel) Init() tea.Cmd {
	return nil
}

func (m *SegmentListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(m.width, m.height)
	}
	return m, nil
}

func (m *SegmentListModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kpStr := msg.String()

	if m.inputActive {
		switch kpStr {
		case "enter":
			m.commitRename()
			m.inputActive = false
			m.nameInput.Reset()
			return m, nil
		case "esc":
			m.inputActive = false
			m.nameInput.Reset()
			return m, nil
		}
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}

	switch kpStr {
	case "r":
		m.inputActive = true
		m.nameInput.Focus()
		if item := m.list.SelectedItem(); item != nil {
			if segItem, ok := item.(segmentListItem); ok {
				m.nameInput.SetValue(filepath.Base(segItem.segment.Path))
			}
		}
		return m, nil
	case "enter":
		// Return the selected segment
		return m, nil
	case "esc":
		// Exit segment list
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *SegmentListModel) commitRename() {
	if m.track == nil {
		return
	}
	item := m.list.SelectedItem()
	if item == nil {
		return
	}
	segItem, ok := item.(segmentListItem)
	if !ok {
		return
	}

	newName := m.nameInput.Value()
	if newName == "" {
		return
	}

	oldPath := segItem.segment.Path
	oldDir := filepath.Dir(oldPath)
	newPath := filepath.Join(oldDir, newName)

	// Rename the file
	if err := os.Rename(oldPath, newPath); err != nil {
		return
	}

	// Update the segment path
	segItem.segment.Path = newPath

	// Refresh the list
	m.list.SetItems(buildSegmentListItems(m.state, m.track))
}

func (m *SegmentListModel) SelectedSegment() *Segment {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	segItem, ok := item.(segmentListItem)
	if !ok {
		return nil
	}
	return segItem.segment
}

func (m *SegmentListModel) SelectedTrackSegment() *TrackSegment {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	segItem, ok := item.(segmentListItem)
	if !ok {
		return nil
	}
	// Find the corresponding TrackSegment in the track
	for i := range m.track.Segments {
		if m.track.Segments[i].Segment == nil {
			continue
		}
		if m.track.Segments[i].Segment.Path == segItem.segment.Path {
			return &m.track.Segments[i]
		}
	}
	return nil
}

func (m *SegmentListModel) View() tea.View {
	var sb strings.Builder

	borderColor := unfocusBorderColor
	if m.focussed {
		borderColor = focusBorderColor
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	var inner strings.Builder
	inner.WriteString(m.list.View())
	if m.inputActive {
		inner.WriteString("\n")
		inner.WriteString(m.nameInput.View())
	}
	sb.WriteString(border.Render(inner.String()))

	return tea.NewView(sb.String())
}

func (m *SegmentListModel) Focus() {
	m.focussed = true
}

func (m *SegmentListModel) Blur() {
	m.focussed = false
	m.inputActive = false
	m.nameInput.Reset()
}
