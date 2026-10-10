package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// forceTrueColor makes the renderer emit ANSI styling, which
// the default renderer strips: it detects the environment,
// which is a pipe during a test, and downgrades to plain
// text. SetColorProfile pins it instead of detecting.
func forceTrueColor(t *testing.T) {
	t.Helper()
	renderer := lipgloss.DefaultRenderer()
	previous := renderer.ColorProfile()
	renderer.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		renderer.SetColorProfile(previous)
	})
}

func TestCellWidth(t *testing.T) {
	fields := testFields()
	// "FilterResonance" would be the widest name in a real
	// model; here the widest is "Alpha" at 5, plus the gap,
	// the input, and the padding.
	if width := cellWidth(fields); width != 5+nameGap+inputWidth+cellPad {
		t.Errorf("cellWidth = %d, want %d", width, 5+nameGap+inputWidth+cellPad)
	}
}

func TestColumnCount(t *testing.T) {
	tests := []struct {
		width, cellWidth, want int
	}{
		{80, 20, 4},
		{80, 11, 7},
		{20, 20, 1},
		{19, 20, 1},
		{0, 20, 1},
		{80, 0, 1},
	}
	for _, test := range tests {
		if got := columnCount(test.width, test.cellWidth); got != test.want {
			t.Errorf("columnCount(%d, %d) = %d, want %d", test.width, test.cellWidth, got, test.want)
		}
	}
}

func TestColumnSlices(t *testing.T) {
	tests := []struct {
		count, columns int
		want           [][2]int
	}{
		{5, 2, [][2]int{{0, 3}, {3, 5}}},
		{4, 2, [][2]int{{0, 2}, {2, 4}}},
		{4, 3, [][2]int{{0, 2}, {2, 4}}},
		{3, 5, [][2]int{{0, 1}, {1, 2}, {2, 3}}},
		{0, 2, nil},
		{5, 0, nil},
	}
	for _, test := range tests {
		if got := columnSlices(test.count, test.columns); !slices.Equal(got, test.want) {
			t.Errorf("columnSlices(%d, %d) = %v, want %v", test.count, test.columns, got, test.want)
		}
	}
}

func TestRenderCell(t *testing.T) {
	forceTrueColor(t)

	plain := renderCell("Alpha", "42", 5, false)
	if !strings.Contains(plain, "Alpha") || !strings.Contains(plain, "42") {
		t.Errorf("unfocused cell %q lost the name or the value", plain)
	}
	// The name is dimmed; the value after it is not,
	// so the values are what read across a grid.
	if !strings.Contains(plain[:len(plain)-3], "\x1b[") {
		t.Error("unfocused name is not dimmed")
	}

	focused := renderCell("Alpha", "42", 5, true)
	if !strings.Contains(focused, "Alpha") || !strings.Contains(focused, "42") {
		t.Errorf("focused cell %q lost the name or the value", focused)
	}
	// Bold and the foreground share one escape
	// sequence, so the bold parameter leads it.
	if !strings.Contains(focused, "\x1b[1;") {
		t.Errorf("focused cell %q is not bold", focused)
	}
}

// plainText removes ANSI styling, so a test can
// check a rendered cell's alignment by eye.
func plainText(styled string) string {
	return ansiPattern.ReplaceAllString(styled, "")
}

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestRenderCellAlignsValues(t *testing.T) {
	// However long a name is, its value occupies the
	// same columns, so a column of cells reads like
	// a column of numbers.
	const width = 15
	tests := []struct{ name, value string }{
		{"A", "7"},
		{"Medium", "42"},
		{"LongFieldName", "127"},
	}
	for _, test := range tests {
		plain := plainText(renderCell(test.name, test.value, width, false))
		start := strings.LastIndex(plain, test.value)
		if want := width + nameGap + inputWidth - len(test.value); start != want {
			t.Errorf("value %q starts at column %d, want %d", test.value, start, want)
		}
		if end := start + len(test.value); end != width+nameGap+inputWidth {
			t.Errorf("value %q ends at column %d, want %d", test.value, end, width+nameGap+inputWidth)
		}
	}
}

func TestJoinGrid(t *testing.T) {
	cells := []string{"a", "b", "c", "d"}
	grid := joinGrid(cells, 2)
	lines := strings.Split(grid, "\n")
	if len(lines) != 2 {
		t.Fatalf("grid has %d lines, want 2:\n%s", len(lines), grid)
	}
	// Column-major join: the first line holds the top of each
	// column, so a and c, then b and d.
	if !strings.Contains(lines[0], "a") || !strings.Contains(lines[0], "c") {
		t.Errorf("first line %q misses a or c", lines[0])
	}
	if !strings.Contains(lines[1], "b") || !strings.Contains(lines[1], "d") {
		t.Errorf("second line %q misses b or d", lines[1])
	}
}

func TestJoinGridEmpty(t *testing.T) {
	if grid := joinGrid(nil, 2); grid != "" {
		t.Errorf("joinGrid of no cells = %q, want empty", grid)
	}
}

func TestRenderView(t *testing.T) {
	view := renderView("content", "header", "footer")
	lines := strings.Split(view, "\n")
	// The join pads every line to the widest block, so
	// the columns line up.
	want := []string{"header ", "content", "footer "}
	if len(lines) != len(want) {
		t.Fatalf("view has %d lines, want %d:\n%s", len(lines), len(want), view)
	}
	for index := range want {
		if lines[index] != want[index] {
			t.Errorf("view line %d = %q, want %q", index, lines[index], want[index])
		}
	}
}

func TestRenderViewKeepsUniformCellWidths(t *testing.T) {
	forceTrueColor(t)
	fields := testFields()
	width := cellWidth(fields)
	cells := make([]string, len(fields))
	for index, field := range fields {
		cells[index] = lipgloss.NewStyle().Width(width).Render(
			renderCell(field.Name, "0", nameWidth(fields), index == 0))
	}
	grid := joinGrid(cells, 2)
	for _, line := range strings.Split(grid, "\n") {
		// Every column block is padded to the same width, so
		// a line is as wide as the two columns together. The
		// styled cell carries ANSI escapes, which Width
		// counts out rather than in.
		if got := lipgloss.Width(line); got != 2*width {
			t.Errorf("line %q is %d cells wide, want %d", line, got, 2*width)
		}
	}
}
