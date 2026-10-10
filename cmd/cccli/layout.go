package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/chzchzchz/midispa/cc"
)

const (
	// inputWidth is how many characters a MIDI value needs
	// at most, and so how wide each cell's input is.
	inputWidth = 3
	// nameGap separates a field's name from its input.
	nameGap = 1
	// cellPad separates one cell from the next.
	cellPad = 2
)

// The editor's palette. Each color is a true-color value;
// the renderer downgrades it to whatever the terminal
// supports, so a monochrome terminal sees plain text.
const (
	focusedColor = "#f1fa8c"
	dimColor     = "#6272a4"
	headerColor  = "#8be9fd"
	goodColor    = "#50fa7b"
	badColor     = "#ff5555"
)

var (
	// focusedStyle marks the cell being edited: bold and
	// bright, so it stands out without the terminal's
	// own cursor.
	focusedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(focusedColor))
	// nameStyle dims a field's name, so the values are
	// what read across a grid of cells.
	nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(dimColor))
)

// nameWidth is the width every cell pads its name to,
// which is the widest name in the model, so the value
// fields all start at the same column.
func nameWidth(fields []cc.ControlField) int {
	widest := 0
	for _, field := range fields {
		if len(field.Name) > widest {
			widest = len(field.Name)
		}
	}
	return widest
}

// cellWidth is the uniform width every cell renders in: the
// widest field name, the gap, the input, and the padding, so
// columns line up no matter which field a cell shows.
func cellWidth(fields []cc.ControlField) int {
	return nameWidth(fields) + nameGap + inputWidth + cellPad
}

// columnCount fits as many uniform cells as the width allows,
// always at least one, which is what fits the most fields on
// screen.
func columnCount(width, widthPerCell int) int {
	if widthPerCell <= 0 {
		return 1
	}
	return max(1, width/widthPerCell)
}

// columnSlices splits the indices 0..count into column-major
// chunks. Column c holds the indices c*rows onward, where
// rows is ceil(count/columns), so column chunks never hold
// more than the row count and a chunk that would start at or
// past count is not built at all: columns clamp themselves to
// the count rather than leaving an empty column behind.
func columnSlices(count, columns int) [][2]int {
	if columns > count {
		columns = count
	}
	if columns <= 0 {
		return nil
	}
	rows := (count + columns - 1) / columns
	chunks := make([][2]int, 0, columns)
	for start := 0; start < count; start += rows {
		end := start + rows
		if end > count {
			end = count
		}
		chunks = append(chunks, [2]int{start, end})
	}
	return chunks
}

// renderCell renders one field's name and value, aligned
// as a column of numbers: the name is padded to the widest
// name and the value is right-aligned in its field, so
// every value in the grid starts and ends at the same
// column. The focused cell is bold and bright, so the cell
// being edited stands out without the terminal's own
// cursor.
func renderCell(name, value string, nameWidth int, focused bool) string {
	pad := strings.Repeat(" ", nameWidth-len(name))
	gap := strings.Repeat(" ", nameGap)
	right := strings.Repeat(" ", max(0, inputWidth-len(value)))
	if focused {
		return focusedStyle.Render(name + pad + gap + right + value)
	}
	return nameStyle.Render(name) + pad + gap + right + value
}

// joinGrid joins rendered cells column-major: each column is a
// vertical block and the blocks sit side by side. The cells
// arrive already padded to a uniform width, which the horizontal
// join preserves, so the columns line up.
func joinGrid(cells []string, columns int) string {
	if len(cells) == 0 {
		return ""
	}
	chunks := columnSlices(len(cells), columns)
	blocks := make([]string, len(chunks))
	for index, chunk := range chunks {
		blocks[index] = lipgloss.JoinVertical(lipgloss.Top, cells[chunk[0]:chunk[1]]...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}

// renderView stacks the header, the scrollable content, and
// the footer, one line each, so the editor reads top to bottom.
func renderView(content, header, footer string) string {
	return lipgloss.JoinVertical(lipgloss.Top, header, content, footer)
}
