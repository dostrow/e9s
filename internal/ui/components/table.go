package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// Column defines a table column.
type Column struct {
	Title      string
	MinWidth   int
	RightAlign bool
}

// Cell holds rendered content for a single table cell.
type Cell struct {
	Content string // may contain ANSI style codes
}

// Table renders aligned columns with nushell-style grid borders.
type Table struct {
	columns  []Column
	rows     [][]Cell
	colWidth []int
	pad      int // padding inside each cell (left+right of content)
}

func NewTable(columns []Column) *Table {
	t := &Table{
		columns:  columns,
		colWidth: make([]int, len(columns)),
		pad:      1,
	}
	for i, c := range columns {
		w := lipgloss.Width(c.Title)
		if c.MinWidth > w {
			w = c.MinWidth
		}
		t.colWidth[i] = w
	}
	return t
}

// AddRow adds a row. The number of cells should match the number of columns.
func (t *Table) AddRow(cells ...Cell) {
	t.rows = append(t.rows, cells)
	for i, c := range cells {
		if i >= len(t.colWidth) {
			break
		}
		w := lipgloss.Width(c.Content)
		if w > t.colWidth[i] {
			t.colWidth[i] = w
		}
	}
}

// Render returns the full table with nushell-style grid borders.
// cursorIdx is the selected row index (-1 for no selection).
// indent is prepended to each line.
// maxRows limits the number of visible data rows (0 = no limit).
// When maxRows is set, the viewport scrolls to keep cursorIdx visible.
func (t *Table) Render(cursorIdx int, indent string, maxRows int) string {
	if len(t.rows) == 0 && len(t.columns) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorDim)
	sep := borderStyle.Render("│")
	p := strings.Repeat(" ", t.pad)

	// Determine visible row window
	startRow := 0
	endRow := len(t.rows)
	if maxRows > 0 && len(t.rows) > maxRows {
		// Scroll so cursor is visible, preferring cursor near the middle
		startRow = cursorIdx - maxRows/2
		if startRow < 0 {
			startRow = 0
		}
		endRow = startRow + maxRows
		if endRow > len(t.rows) {
			endRow = len(t.rows)
			startRow = endRow - maxRows
			if startRow < 0 {
				startRow = 0
			}
		}
	}

	var b strings.Builder

	// ╭─── top border ───╮
	b.WriteString(indent)
	b.WriteString("  ") // cursor gutter
	b.WriteString(borderStyle.Render(t.horizontalRule("╭", "┬", "╮")))
	b.WriteString("\n")

	// Header row
	b.WriteString(indent)
	b.WriteString("  ") // cursor gutter
	b.WriteString(sep)
	for i, col := range t.columns {
		styled := theme.HeaderStyle.Render(col.Title)
		visual := lipgloss.Width(col.Title)
		target := t.colWidth[i]
		trailing := ""
		if visual < target {
			trailing = strings.Repeat(" ", target-visual)
		}
		b.WriteString(p + styled + trailing + p + sep)
	}
	b.WriteString("\n")

	// ├─── header separator ───┤
	b.WriteString(indent)
	b.WriteString("  ")
	b.WriteString(borderStyle.Render(t.horizontalRule("├", "┼", "┤")))
	b.WriteString("\n")

	// Data rows (only the visible window)
	for ri := startRow; ri < endRow; ri++ {
		row := t.rows[ri]
		cursor := "  "
		rowStyle := lipgloss.NewStyle()
		if ri == cursorIdx {
			cursor = "► "
			rowStyle = theme.SelectedRowStyle
		}

		var line strings.Builder
		line.WriteString(indent)
		line.WriteString(cursor)
		line.WriteString(sep)
		for i, cell := range row {
			if i >= len(t.columns) {
				break
			}
			content := t.padContent(cell.Content, i, true)
			line.WriteString(p + content + p + sep)
		}
		for i := len(row); i < len(t.columns); i++ {
			line.WriteString(p + t.padContent("", i, true) + p + sep)
		}

		if ri == cursorIdx {
			b.WriteString(rowStyle.Render(line.String()))
		} else {
			b.WriteString(line.String())
		}
		b.WriteString("\n")
	}

	// ╰─── bottom border ───╯
	b.WriteString(indent)
	b.WriteString("  ")
	b.WriteString(borderStyle.Render(t.horizontalRule("╰", "┴", "╯")))
	b.WriteString("\n")

	// Scroll indicator
	if maxRows > 0 && len(t.rows) > maxRows {
		info := fmt.Sprintf("  %d–%d of %d", startRow+1, endRow, len(t.rows))
		if startRow > 0 {
			info += " ↑"
		}
		if endRow < len(t.rows) {
			info += " ↓"
		}
		b.WriteString(indent + theme.HelpStyle.Render(info) + "\n")
	}

	return b.String()
}

// Width returns the total visual width of the table (including borders and cursor gutter).
func (t *Table) Width() int {
	// cursor gutter (2) + border chars (len(columns)+1) + each column (width + 2*pad)
	w := 2 + len(t.colWidth) + 1 // gutter + separators
	for _, cw := range t.colWidth {
		w += cw + t.pad*2
	}
	return w
}

// horizontalRule builds a horizontal border line like ╭───┬───┬───╮
func (t *Table) horizontalRule(left, mid, right string) string {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range t.colWidth {
		b.WriteString(strings.Repeat("─", w+t.pad*2))
		if i < len(t.colWidth)-1 {
			b.WriteString(mid)
		}
	}
	b.WriteString(right)
	return b.String()
}

// padContent pads cell content to the column width, respecting visual width and alignment.
func (t *Table) padContent(content string, colIdx int, isData bool) string {
	if colIdx >= len(t.colWidth) {
		return content
	}
	target := t.colWidth[colIdx]
	visual := lipgloss.Width(content)
	if visual >= target {
		return content
	}
	pad := target - visual
	if t.columns[colIdx].RightAlign && isData {
		return strings.Repeat(" ", pad) + content
	}
	return content + strings.Repeat(" ", pad)
}

// Plain creates a Cell with no styling.
func Plain(s string) Cell {
	return Cell{Content: s}
}

// Styled creates a Cell with a lipgloss style applied.
func Styled(s string, style lipgloss.Style) Cell {
	return Cell{Content: style.Render(s)}
}
