package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/command"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// paletteRunMsg asks the App to run a palette item after the palette closes.
type paletteRunMsg struct{ item paletteItem }

// PaletteModel is the fuzzy command palette overlay. It holds a snapshot of
// the commands available when it opened; the App rebuilds it on each open.
type PaletteModel struct {
	Active  bool
	input   textinput.Model
	items   map[string]paletteItem
	entries []command.Entry
	legend  []paletteItem
	recency *command.Recency
	results []command.Result
	cursor  int
	offset  int
	height  int
	prompt  string
	// flat hides section headers, for nested steps with a single kind of
	// item (a region list).
	flat bool
}

func newPalette(items []paletteItem, legend []paletteItem, recency *command.Recency, prompt string, height int) PaletteModel {
	ti := textinput.New()
	ti.Prompt = prompt + " "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)
	ti.Placeholder = "Type to search commands · @ for loaded resources"
	ti.CharLimit = 120
	ti.Focus()

	p := PaletteModel{
		Active:  true,
		input:   ti,
		items:   make(map[string]paletteItem, len(items)),
		legend:  legend,
		recency: recency,
		prompt:  prompt,
		height:  height,
	}
	for _, item := range items {
		if item.legend || item.ID == "" {
			continue
		}
		if _, dup := p.items[item.ID]; dup {
			continue
		}
		p.items[item.ID] = item
		p.entries = append(p.entries, item.Entry)
	}
	p.refilter()
	return p
}

func (p *PaletteModel) refilter() {
	query, resourcesOnly := resourceQuery(p.input.Value())
	entries, recency := p.entries, p.recency
	if resourcesOnly {
		entries = nil
		for _, e := range p.entries {
			if e.Section == command.SectionResources {
				entries = append(entries, e)
			}
		}
	}
	if resourcesOnly && strings.TrimSpace(query) == "" {
		p.results = make([]command.Result, 0, len(entries))
		for _, e := range entries {
			p.results = append(p.results, command.Result{Entry: e})
		}
	} else {
		p.results = command.Filter(entries, query, recency)
	}
	p.cursor, p.offset = 0, 0
	p.skipUnavailable(1)
	p.scrollToCursor()
}

// skipUnavailable moves the cursor off rows that cannot run, in direction
// dir, so enter always targets something runnable. When no row in that
// direction can run, the cursor stays where it was.
func (p *PaletteModel) skipUnavailable(dir int) {
	for i := p.cursor; i >= 0 && i < len(p.results); i += dir {
		if p.results[i].Available() {
			p.cursor = i
			return
		}
	}
}

// hasRunnable reports whether any result can run.
func (p PaletteModel) hasRunnable() bool {
	for _, r := range p.results {
		if r.Available() {
			return true
		}
	}
	return false
}

func (p PaletteModel) Update(msg tea.KeyMsg) (PaletteModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		p.Active = false
		return p, nil
	case "enter":
		if p.cursor >= len(p.results) {
			return p, nil
		}
		r := p.results[p.cursor]
		if !r.Available() {
			return p, nil
		}
		item := p.items[r.ID]
		p.Active = false
		return p, func() tea.Msg { return paletteRunMsg{item: item} }
	case "up", "ctrl+p", "ctrl+k", "shift+tab":
		p.move(-1)
		return p, nil
	case "down", "ctrl+n", "ctrl+j", "tab":
		p.move(1)
		return p, nil
	case "pgup":
		p.move(-paletteMaxRows)
		return p, nil
	case "pgdown":
		p.move(paletteMaxRows)
		return p, nil
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.refilter()
	}
	return p, cmd
}

func (p *PaletteModel) move(delta int) {
	if len(p.results) == 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	if !p.hasRunnable() {
		return
	}
	from := p.cursor
	p.cursor = max(0, min(p.cursor+delta, len(p.results)-1))
	p.skipUnavailable(dir)
	if !p.results[p.cursor].Available() {
		p.skipUnavailable(-dir)
	}
	if !p.results[p.cursor].Available() {
		p.cursor = from
	}
	p.scrollToCursor()
}

const (
	paletteMaxRows  = 14
	paletteMaxWidth = 96
)

// SetHeight records the terminal height so scrolling can keep the cursor on
// screen.
func (p PaletteModel) SetHeight(height int) PaletteModel {
	p.height = height
	p.scrollToCursor()
	return p
}

func (p PaletteModel) maxRows() int {
	return max(3, min(paletteMaxRows, p.height-12))
}

func (p PaletteModel) browsing() bool {
	query, resourcesOnly := resourceQuery(p.input.Value())
	return !resourcesOnly && strings.TrimSpace(query) == ""
}

// scrollToCursor adjusts the scroll offset so the cursor row is visible,
// keeping its section header in view when the cursor sits at the top.
func (p *PaletteModel) scrollToCursor() {
	browsing := p.browsing()
	line := p.cursorLine(browsing)
	rows := p.maxRows()
	if line < p.offset {
		p.offset = line
		for p.offset > 0 && p.isHeaderLine(p.offset-1, browsing) {
			p.offset--
		}
	}
	if line >= p.offset+rows {
		p.offset = line - rows + 1
	}
	p.offset = max(0, p.offset)
}

func (p PaletteModel) isHeaderLine(index int, browsing bool) bool {
	line := 0
	section := command.Section(-1)
	for i, r := range p.results {
		if !p.flat && (browsing || r.Section == command.SectionUnavailable) && r.Section != section {
			section = r.Section
			if browsing && i > 0 {
				if line == index {
					return true
				}
				line++
			}
			if line == index {
				return true
			}
			line++
		}
		if line == index {
			return false
		}
		line++
	}
	return false
}

// View renders the palette sized to the terminal.
func (p PaletteModel) View(width int) string {
	boxWidth := min(paletteMaxWidth, width-4)
	inner := max(20, boxWidth-4) // border + horizontal padding
	p.input.Width = inner - lipgloss.Width(p.input.Prompt) - 1

	browsing := p.browsing()
	lines := p.renderRows(inner, browsing)
	maxRows := p.maxRows()
	p.offset = max(0, min(p.offset, max(0, len(lines)-maxRows)))

	var b strings.Builder
	b.WriteString(p.input.View())
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(strings.Repeat("─", inner)))
	b.WriteString("\n")
	if len(p.results) == 0 {
		b.WriteString(theme.HelpStyle.Render("No matching commands"))
		b.WriteString("\n")
	}
	end := min(len(lines), p.offset+maxRows)
	for _, line := range lines[p.offset:end] {
		b.WriteString(line.text)
		b.WriteString("\n")
	}
	if more := len(lines) - end; more > 0 {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  ↓ %d more", more)))
		b.WriteString("\n")
	}
	if browsing && len(p.legend) > 0 {
		b.WriteString(theme.HelpStyle.Render(strings.Repeat("─", inner)))
		b.WriteString("\n")
		b.WriteString(wrapLegend(p.legend, inner))
		b.WriteString("\n")
	}
	b.WriteString(theme.HelpStyle.Render("[enter] run  [↑/↓] move  [esc] close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(0, 1).
		Width(boxWidth - 2).
		Render(b.String())
}

type paletteLine struct {
	text   string
	header bool
}

func (p PaletteModel) renderRows(inner int, browsing bool) []paletteLine {
	var lines []paletteLine
	section := command.Section(-1)
	for i, r := range p.results {
		if !p.flat && (browsing || r.Section == command.SectionUnavailable) && r.Section != section {
			section = r.Section
			if browsing && i > 0 {
				lines = append(lines, paletteLine{text: "", header: true})
			}
			title := lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true).Render(strings.ToUpper(section.Title()))
			lines = append(lines, paletteLine{text: title, header: true})
		}
		lines = append(lines, paletteLine{text: renderPaletteRow(r, i == p.cursor && r.Available(), inner)})
	}
	return lines
}

// cursorLine maps the cursor's result index to its rendered line, accounting
// for section headers and spacers.
func (p PaletteModel) cursorLine(browsing bool) int {
	line := 0
	section := command.Section(-1)
	for i, r := range p.results {
		if !p.flat && (browsing || r.Section == command.SectionUnavailable) && r.Section != section {
			section = r.Section
			if browsing && i > 0 {
				line++
			}
			line++
		}
		if i == p.cursor {
			return line
		}
		line++
	}
	return line
}

var (
	paletteKeyStyle    = lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)
	paletteMatchStyle  = lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true).Underline(true)
	paletteDimStyle    = lipgloss.NewStyle().Foreground(theme.ColorDim)
	paletteDangerStyle = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
)

func renderPaletteRow(r command.Result, selected bool, inner int) string {
	available := r.Available()

	right := r.Category
	if !available {
		right = r.Disabled
	}
	keyText := r.Key
	if !available {
		keyText = ""
	}
	marker := " "
	if r.Danger {
		marker = "⚠"
	}
	keyCol := 9
	rightMax := max(0, inner/2-keyCol-4)
	right = truncateRunes(right, rightMax)
	rightWidth := lipgloss.Width(right) + 1 + keyCol + 2

	titleWidth := max(8, inner-2-rightWidth-1)
	title, positions := r.Title, r.Positions
	target := ""
	if r.Target != "" {
		target = " · " + r.Target
	}
	if lipgloss.Width(title) > titleWidth {
		title = truncateRunes(title, titleWidth)
		target = ""
	} else if lipgloss.Width(title+target) > titleWidth {
		target = truncateRunes(target, titleWidth-lipgloss.Width(title))
	}

	titleStyle := lipgloss.NewStyle()
	if !available {
		titleStyle = paletteDimStyle
	} else if r.Danger {
		titleStyle = titleStyle.Foreground(theme.ColorRed)
	}
	if selected {
		titleStyle = titleStyle.Bold(true)
	}
	left := highlightRunes(title, positions, titleStyle, paletteMatchStyle) + paletteDimStyle.Render(target)

	cursor := "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true).Render("▸ ")
	}
	keyRendered := paletteKeyStyle.Render(keyText)
	markerRendered := " "
	if r.Danger {
		markerRendered = paletteDangerStyle.Render(marker)
	}
	gap := inner - 2 - lipgloss.Width(left) - lipgloss.Width(right) - 1 - keyCol - 2
	return cursor + left + strings.Repeat(" ", max(1, gap)) +
		paletteDimStyle.Render(right) + " " +
		strings.Repeat(" ", max(0, keyCol-lipgloss.Width(keyText))) + keyRendered + " " + markerRendered
}

// highlightRunes styles the runes of s at positions with match and the rest
// with base. Positions past the end of s (after truncation) are ignored.
func highlightRunes(s string, positions []int, base, match lipgloss.Style) string {
	if len(positions) == 0 {
		return base.Render(s)
	}
	marked := make(map[int]bool, len(positions))
	for _, p := range positions {
		marked[p] = true
	}
	var b strings.Builder
	var run []rune
	runMatched := false
	flush := func() {
		if len(run) == 0 {
			return
		}
		if runMatched {
			b.WriteString(match.Inherit(base).Render(string(run)))
		} else {
			b.WriteString(base.Render(string(run)))
		}
		run = run[:0]
	}
	for i, r := range []rune(s) {
		if marked[i] != runMatched {
			flush()
			runMatched = marked[i]
		}
		run = append(run, r)
	}
	flush()
	return b.String()
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}
	return string(r[:limit-1]) + "…"
}

func wrapLegend(items []paletteItem, width int) string {
	var lines []string
	line := ""
	for _, item := range items {
		chunk := item.displayKey() + " " + strings.ToLower(item.Title)
		if line != "" && lipgloss.Width(line)+3+lipgloss.Width(chunk) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " · "
		}
		line += chunk
	}
	if line != "" {
		lines = append(lines, line)
	}
	return paletteDimStyle.Render(strings.Join(lines, "\n"))
}
