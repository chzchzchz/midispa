package main

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// trackListModel manages the track list UI and operations
type trackListModel struct {
	listModel list.Model
	state     *State
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

func buildTrackListItems(s *State) []list.Item {
	items := make([]list.Item, len(s.tracks.Tracks))
	for i := range s.tracks.Tracks {
		isRecord := s.RecordTrack != nil && &s.tracks.Tracks[i] == s.RecordTrack
		items[i] = trackItem{track: &s.tracks.Tracks[i], isRecord: isRecord}
	}
	return items
}

func newTrackList(items []list.Item) list.Model {
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

// newTrackListModel creates a new trackListModel
func newTrackListModel(s *State) *trackListModel {
	return &trackListModel{
		listModel: newTrackList(buildTrackListItems(s)),
		state:     s,
	}
}

// refresh updates the track list items
func (m *trackListModel) refresh() {
	m.listModel.SetItems(buildTrackListItems(m.state))
}

// toggleMute toggles the mute state of the currently selected track
func (m *trackListModel) toggleMute() {
	idx := m.listModel.Index()
	if idx >= 0 && idx < len(m.state.tracks.Tracks) {
		m.state.tracks.Tracks[idx].Mute = !m.state.tracks.Tracks[idx].Mute
		m.refresh()
	}
}

// toggleRecord toggles recording for the currently selected track
func (m *trackListModel) toggleRecord() {
	idx := m.listModel.Index()
	if idx < 0 || idx >= len(m.state.tracks.Tracks) {
		return
	}
	if m.state.RecordTrack == &m.state.tracks.Tracks[idx] {
		m.state.RecordTrack = nil
	} else {
		m.state.RecordTrack = &m.state.tracks.Tracks[idx]
	}
	m.refresh()
}

// getSelectedTrack returns the currently selected track or nil
func (m *trackListModel) getSelectedTrack() *Track {
	idx := m.listModel.Index()
	if idx < 0 || idx >= len(m.state.tracks.Tracks) {
		return nil
	}
	return &m.state.tracks.Tracks[idx]
}

// renameTrackIfValid renames the currently selected track if the name is valid
func (m *trackListModel) renameTrackIfValid(name string) {
	idx := m.listModel.Index()
	if idx < 0 || idx >= len(m.state.tracks.Tracks) {
		return
	}
	if !m.state.tracks.rename(m.state.tracks.Tracks[idx].Name, name) {
		return
	}
}

// Index returns the current index of the track list
func (m *trackListModel) Index() int {
	return m.listModel.Index()
}

// SetItems sets the items in the track list
func (m *trackListModel) SetItems(items []list.Item) {
	m.listModel.SetItems(items)
}

// Update updates the track list model
func (m *trackListModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	m.listModel, cmd = m.listModel.Update(msg)
	return cmd
}

// View returns the view of the track list
func (m *trackListModel) View() string {
	return m.listModel.View()
}
