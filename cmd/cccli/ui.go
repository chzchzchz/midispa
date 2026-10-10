package main

import (
	"fmt"
	"io"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chzchzchz/midispa/cc"
)

const (
	// stepSmall and stepLarge are how much the value
	// keys move the focused field, by one and by ten;
	// tab also moves focus by stepSmall, one field.
	stepSmall = 1
	stepLarge = 10

	// boxColumns is the grid box's left and right borders,
	// which the fields live inside of.
	boxColumns = 2

	// viewportMargin is the lines the viewport leaves room
	// for: the header, its rule, the grid box's top and
	// bottom, the footer's rule, and the footer.
	viewportMargin = 6

	// inputLimit is how many characters a MIDI value needs at
	// most, so a cell never holds a longer one.
	inputLimit = 3
)

// valueSteps maps the value keys to how far they move the
// focused field: up and pgup increase it, down and pgdn
// decrease it.
var valueSteps = map[tea.KeyType]int{
	tea.KeyUp:     stepSmall,
	tea.KeyDown:   -stepSmall,
	tea.KeyPgUp:   stepLarge,
	tea.KeyPgDown: -stepLarge,
}

// statusKind says how the status line is colored: a
// value that went out and a model that saved are
// good, a send or save that failed and a buffer that
// does not parse are bad, and anything else, such as
// the nrpn notice, is plain news.
type statusKind int

const (
	statusPlain statusKind = iota
	statusGood
	statusBad
)

var (
	// headerStyle titles the editor with the model,
	// channel and port, under a rule of the same
	// color. Only the rule's side is turned on at
	// render time, so the title sits above a line
	// rather than inside a box.
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(headerColor)).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(headerColor))

	// gridStyle boxes the fields, in a dim border
	// that frames the values without competing with
	// them.
	gridStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(dimColor))

	// footerStyle lists the keys and the status, over
	// a rule of the same color.
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(dimColor)).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(dimColor))

	// statusStyles colors the status line by how the
	// last action went.
	statusStyles = [...]lipgloss.Style{
		statusPlain: lipgloss.NewStyle(),
		statusGood:  lipgloss.NewStyle().Foreground(lipgloss.Color(goodColor)),
		statusBad:   lipgloss.NewStyle().Foreground(lipgloss.Color(badColor)),
	}
)

// cliModel is the editor: one textinput per field, a viewport
// that scrolls them, and the writer control changes go out on.
type cliModel struct {
	cfg        configuration
	fields     []cc.ControlField
	inputs     []textinput.Model
	focusIndex int
	columns    int
	out        io.Writer
	channel    int
	status     string
	statusKind statusKind
	vp         viewport.Model
	width      int
	// sentCount is how many control changes have gone
	// out, which the header shows next to the port.
	sentCount int
}

// newCLI builds a model around an existing writer. Production
// passes the ALSA seq writer; tests pass a bytes.Buffer, so no
// test opens a sequencer. cfg supplies the model and port names
// for the header, the MIDI channel, and the --output path.
func newCLI(cfg configuration, fields []cc.ControlField, out io.Writer) *cliModel {
	model := &cliModel{
		cfg:     cfg,
		fields:  fields,
		out:     out,
		channel: cfg.midiChannel - 1,
		columns: 1,
		vp:      viewport.New(0, 0),
	}
	model.inputs = make([]textinput.Model, len(fields))
	for index, field := range fields {
		input := textinput.New()
		// The prompt is dropped: a cell is a bare value next
		// to its name, not a line to fill in.
		input.Prompt = ""
		input.CharLimit = inputLimit
		input.Width = inputWidth
		input.Validate = validateValue
		input.SetValue(strconv.Itoa(*field.Value))
		model.inputs[index] = input
	}
	if len(model.inputs) > 0 {
		model.inputs[0].Focus()
	}
	if params, err := cc.NewModelParams(cfg.modelName); err == nil {
		if count := nrpnFieldCount(params); count > 0 {
			model.setStatus(fmt.Sprintf("%d nrpn fields are not shown", count), statusPlain)
		}
	}
	return model
}

func (m *cliModel) Init() tea.Cmd {
	return nil
}

func (m *cliModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.relayout(msg.Width, msg.Height)
		return m, nil
	case tea.MouseMsg:
		// The viewport consumes every message it is given,
		// key messages included, so only mouse messages are
		// routed to it: the keys are the editor's own.
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		return m, m.updateKey(msg)
	}
	return m, nil
}

// updateKey routes one key. The editor owns the keys that mean
// something to it; anything else is typing in the focused cell.
func (m *cliModel) updateKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit
	case tea.KeyTab:
		return m.moveFocus(stepSmall)
	case tea.KeyShiftTab:
		return m.moveFocus(-stepSmall)
	case tea.KeyLeft:
		return m.moveFocusRow(-m.rowCount())
	case tea.KeyRight:
		return m.moveFocusRow(m.rowCount())
	case tea.KeyCtrlS:
		m.save()
		return nil
	}
	if delta, ok := valueSteps[msg.Type]; ok {
		m.stepFocused(delta)
		return nil
	}
	return m.editFocused(msg)
}

func (m *cliModel) View() string {
	// The cells are refreshed here rather than on every
	// edit: SetContent keeps the scroll position, so a
	// value key's sweep stays in view while it happens.
	m.vp.SetContent(joinGrid(m.renderCells(), m.columns))
	header := headerStyle.Width(m.width).BorderBottom(true).Render(
		fmt.Sprintf("%s channel %d port %s (%d sent)",
			m.cfg.modelName, m.cfg.midiChannel, m.cfg.portName, m.sentCount))
	grid := gridStyle.Width(max(0, m.width-boxColumns)).Render(m.vp.View())
	footer := footerStyle.Width(m.width).BorderTop(true).Render(m.footerText())
	return renderView(grid, header, footer)
}

// footerText is the key list plus the last action's
// status, colored by how that action went.
func (m *cliModel) footerText() string {
	keys := "↑/↓ ±1, pgup/pgdn ±10, ⇥/shift-⇥/←/→ move, ctrl+s save"
	if m.status == "" {
		return keys
	}
	return keys + " | " + statusStyles[m.statusKind].Render(m.status)
}

// setStatus records the status line and how it is
// colored.
func (m *cliModel) setStatus(text string, kind statusKind) {
	m.status = text
	m.statusKind = kind
}

// rowCount is how many fields a column holds, which is how far
// apart the fields of one visual row sit in the field list.
func (m *cliModel) rowCount() int {
	count := len(m.fields)
	if count == 0 {
		return 0
	}
	return (count + m.columns - 1) / m.columns
}

// moveFocus moves focus by delta fields, wrapping at the ends.
func (m *cliModel) moveFocus(delta int) tea.Cmd {
	count := len(m.fields)
	if count == 0 {
		return nil
	}
	index := (m.focusIndex + delta) % count
	if index < 0 {
		index += count
	}
	return m.setFocus(index)
}

// moveFocusRow moves focus by delta visual rows, clamped at the
// first and last field rather than wrapping.
func (m *cliModel) moveFocusRow(delta int) tea.Cmd {
	count := len(m.fields)
	if count == 0 {
		return nil
	}
	index := min(max(m.focusIndex+delta, 0), count-1)
	return m.setFocus(index)
}

// setFocus restores the cell being left to the model's value,
// so a cell that was left half-edited shows what the model
// holds, and focuses the cell being entered.
func (m *cliModel) setFocus(index int) tea.Cmd {
	if index < 0 || index >= len(m.inputs) {
		return nil
	}
	m.inputs[m.focusIndex].SetValue(strconv.Itoa(*m.fields[m.focusIndex].Value))
	m.inputs[m.focusIndex].Blur()
	m.focusIndex = index
	m.ensureFocusedVisible()
	return m.inputs[m.focusIndex].Focus()
}

// stepFocused moves the focused field's value by delta, clamped
// to the MIDI range, and sends the change at once: each press
// and each repeat a held key sends is one step of the sweep,
// the same way a knob turns. At a limit there is nothing to
// send, so nothing is.
func (m *cliModel) stepFocused(delta int) {
	field := &m.fields[m.focusIndex]
	m.commitValue(field, stepValue(*field.Value, delta))
}

// editFocused passes a key to the focused cell and applies what
// it did: every keystroke that parses as a value in range
// writes the model and sends the control change immediately,
// so the instrument tracks typing the way it tracks a knob.
func (m *cliModel) editFocused(msg tea.KeyMsg) tea.Cmd {
	input, cmd := m.inputs[m.focusIndex].Update(msg)
	m.inputs[m.focusIndex] = input
	m.applyFocused()
	return cmd
}

func (m *cliModel) applyFocused() {
	value, ok := parseValue(m.inputs[m.focusIndex].Value())
	if !ok {
		// An empty or invalid buffer is not a value: the model
		// keeps what it had, and only the screen differs, until
		// the cell loses focus and is restored.
		m.setStatus("invalid", statusBad)
		return
	}
	m.commitValue(&m.fields[m.focusIndex], value)
}

// commitValue writes a value that differs from the field's
// current one, so a step or a typed value that lands where
// the model already is sends nothing.
func (m *cliModel) commitValue(field *cc.ControlField, value int) {
	if value != *field.Value {
		m.setFocusedValue(field, value)
	}
}

// setFocusedValue commits one value to the model, the cell, and
// the wire together, so the three can never disagree.
func (m *cliModel) setFocusedValue(field *cc.ControlField, value int) {
	*field.Value = value
	m.inputs[m.focusIndex].SetValue(strconv.Itoa(value))
	m.sendValue(field, value)
}

// sendValue sends one control change and reports it in the
// status line, or reports why it did not go out.
func (m *cliModel) sendValue(field *cc.ControlField, value int) {
	if err := sendCC(m.out, m.channel, field.Controller, value); err != nil {
		m.setStatus(fmt.Sprintf("send failed: %v", err), statusBad)
		return
	}
	m.sentCount++
	m.setStatus(fmt.Sprintf("%s = %d", field.Name, value), statusGood)
}

// save writes the model to --output.
func (m *cliModel) save() {
	if m.cfg.output == "" {
		m.setStatus("--output is required to save", statusBad)
		return
	}
	if err := writeCCSMF(m.cfg.output, m.fields, m.cfg.modelName, m.cfg.midiChannel); err != nil {
		m.setStatus(fmt.Sprintf("save failed: %v", err), statusBad)
		return
	}
	m.setStatus(fmt.Sprintf("saved %d controls", len(m.fields)), statusGood)
}

// ensureFocusedVisible scrolls the focused field's row into
// the viewport. The arrows edit values, so scrolling itself is
// the mouse wheel, and a focus move only needs to keep the
// focused row on screen.
func (m *cliModel) ensureFocusedVisible() {
	if m.vp.Height <= 0 {
		return
	}
	row := m.focusIndex % m.rowCount()
	if row < m.vp.YOffset {
		m.vp.SetYOffset(row)
		return
	}
	if row >= m.vp.YOffset+m.vp.Height {
		m.vp.SetYOffset(row - m.vp.Height + 1)
	}
}

// relayout recomputes the grid from the terminal size. The
// viewport is a no-op when the content fits and scrollable when
// it does not; every field stays reachable either way.
func (m *cliModel) relayout(width, height int) {
	m.columns = columnCount(width-boxColumns, cellWidth(m.fields))
	m.vp.Width = max(0, width-boxColumns)
	m.vp.Height = max(1, height-viewportMargin)
	m.vp.SetContent(joinGrid(m.renderCells(), m.columns))
	// Reclamp the scroll position to the new content height.
	m.vp.SetYOffset(m.vp.YOffset)
}

// renderCells renders every cell, padded to the uniform cell
// width so the columns line up when they are joined.
func (m *cliModel) renderCells() []string {
	width := cellWidth(m.fields)
	names := nameWidth(m.fields)
	style := lipgloss.NewStyle().Width(width)
	cells := make([]string, len(m.fields))
	for index, field := range m.fields {
		cell := renderCell(field.Name, strconv.Itoa(*field.Value), names, index == m.focusIndex)
		cells[index] = style.Render(cell)
	}
	return cells
}

// stepValue moves a value by delta, clamped to the MIDI range
// without wrapping: a knob at its end stops instead of jumping
// to the other end.
func stepValue(value, delta int) int {
	value += delta
	if value < 0 {
		return 0
	}
	if value > maxMIDIValue {
		return maxMIDIValue
	}
	return value
}

// parseValue reads a value a cell holds. An empty buffer is not
// a value: the model keeps what it had while the cell is being
// edited, and the caller reports the buffer invalid.
func parseValue(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, false
	}
	if value < 0 || value > maxMIDIValue {
		return 0, false
	}
	return value, true
}

// validateValue is the cells' advisory validator: it accepts
// empty or an integer in range, so typing digits never rings
// the terminal's bell. What an invalid buffer means is decided
// in applyFocused, which owns the model.
func validateValue(text string) error {
	if text == "" {
		return nil
	}
	if _, ok := parseValue(text); ok {
		return nil
	}
	return fmt.Errorf("value must be between 0 and %d", maxMIDIValue)
}
