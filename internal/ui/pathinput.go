package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

const pathInputMaxVisible = 8

// PathInput provides a directory path input with filesystem completion.
// Type to filter, Tab/Down/Up to navigate suggestions, Enter to select.
type PathInput struct {
	Action    InputAction
	Prompt    string
	value     string // current input value
	cursor    int    // cursor in suggestions list (-1 = typing)
	matches   []pathMatch
	hasTF     bool
	errMsg    string
}

type pathMatch struct {
	display string
	path    string // full suggestion path
	hasTF   bool
}

// PathInputResultMsg is returned when the user submits a path.
type PathInputResultMsg struct {
	Action InputAction
	Value  string
}

// PathInputCancelMsg is returned when the user cancels.
type PathInputCancelMsg struct{}

func NewPathInput(action InputAction, prompt, defaultVal string) PathInput {
	p := PathInput{
		Action: action,
		Prompt: prompt,
		value:  defaultVal,
		cursor: -1,
	}
	p.refreshMatches()
	return p
}

func (p PathInput) Update(msg tea.Msg) (PathInput, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	switch km.Type {
	case tea.KeyEnter:
		// If cursor is on a suggestion, accept it
		if p.cursor >= 0 && p.cursor < len(p.matches) {
			selected := p.matches[p.cursor]
			if selected.hasTF {
				// Has .tf files — select it
				return p, func() tea.Msg {
					return PathInputResultMsg{Action: p.Action, Value: expandHome(strings.TrimRight(selected.path, "/"))}
				}
			}
			// No .tf files — navigate into it
			p.value = selected.path
			p.cursor = -1
			p.errMsg = ""
			p.refreshMatches()
			return p, nil
		}
		// Enter on the text input — try to select current value
		val := strings.TrimRight(p.value, "/")
		val = expandHome(val)
		if !dirHasTFFiles(val) {
			p.errMsg = fmt.Sprintf("No .tf files in %s", collapseHome(val))
			return p, nil
		}
		return p, func() tea.Msg {
			return PathInputResultMsg{Action: p.Action, Value: val}
		}

	case tea.KeyEsc:
		return p, func() tea.Msg { return PathInputCancelMsg{} }

	case tea.KeyTab, tea.KeyDown:
		// Move cursor into/through suggestions
		if len(p.matches) > 0 {
			if p.cursor < len(p.matches)-1 {
				p.cursor++
			}
		}
		return p, nil

	case tea.KeyShiftTab, tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		} else if p.cursor == 0 {
			p.cursor = -1 // back to typing
		}
		return p, nil

	case tea.KeyBackspace:
		if len(p.value) > 0 {
			p.value = p.value[:len(p.value)-1]
			p.cursor = -1
			p.errMsg = ""
			p.refreshMatches()
		}
		return p, nil

	case tea.KeyRunes:
		p.value += string(km.Runes)
		p.cursor = -1
		p.errMsg = ""
		p.refreshMatches()
		return p, nil

	case tea.KeySpace:
		// Ignore spaces in paths
		return p, nil
	}

	return p, nil
}

func (p PathInput) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(1, 2).
		Width(70)

	var lines []string

	lines = append(lines, theme.TitleStyle.Render(p.Prompt))
	lines = append(lines, "")

	// Input line with cursor
	inputStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Bold(true)
	cursorStr := inputStyle.Render(p.value) + lipgloss.NewStyle().
		Foreground(theme.ColorCyan).Blink(true).Render("█")
	lines = append(lines, cursorStr)

	// Status
	if p.errMsg != "" {
		lines = append(lines, theme.ErrorStyle.Render(p.errMsg))
	} else if p.hasTF {
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorGreen).
			Render("contains .tf files"))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, "")

	// Suggestions list (fixed height)
	if len(p.matches) == 0 {
		lines = append(lines, theme.HelpStyle.Render("(no matching directories)"))
		for i := 0; i < pathInputMaxVisible-1; i++ {
			lines = append(lines, "")
		}
	} else {
		// Window the visible range around cursor
		start := 0
		if p.cursor >= pathInputMaxVisible {
			start = p.cursor - pathInputMaxVisible + 1
		}
		end := start + pathInputMaxVisible
		if end > len(p.matches) {
			end = len(p.matches)
			start = max(0, end-pathInputMaxVisible)
		}

		for i := start; i < end; i++ {
			m := p.matches[i]
			prefix := "  "
			style := theme.HelpStyle
			if i == p.cursor {
				prefix = "► "
				style = theme.SelectedRowStyle
			}
			label := m.display
			if m.hasTF {
				label += " ✓"
			}
			lines = append(lines, style.Render(prefix+label))
		}
		// Pad to fixed height
		for i := end - start; i < pathInputMaxVisible; i++ {
			lines = append(lines, "")
		}
		if len(p.matches) > pathInputMaxVisible {
			lines = append(lines, theme.HelpStyle.Render(
				fmt.Sprintf("(%d total)", len(p.matches))))
		} else {
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	lines = append(lines, theme.HelpStyle.Render("[tab/↓↑] navigate  [enter] select  [esc] cancel"))

	content := strings.Join(lines, "\n")
	return boxStyle.Render(content)
}

func (p *PathInput) refreshMatches() {
	expanded := expandHome(p.value)

	// Check current path for .tf
	checkDir := strings.TrimRight(expanded, "/")
	p.hasTF = dirHasTFFiles(checkDir)

	dir := expanded
	prefix := ""
	if !strings.HasSuffix(expanded, "/") {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		p.matches = nil
		return
	}

	p.matches = nil
	lowerPrefix := strings.ToLower(prefix)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && prefix == "" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
			continue
		}

		fullPath := filepath.Join(dir, name)
		display := collapseHome(fullPath) + "/"

		suggestion := collapseHome(fullPath) + "/"
		if strings.HasPrefix(p.value, "/") {
			suggestion = fullPath + "/"
		}

		p.matches = append(p.matches, pathMatch{
			display: display,
			path:    suggestion,
			hasTF:   dirHasTFFiles(fullPath),
		})
	}
}

func dirHasTFFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
			return true
		}
	}
	return false
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	return path
}

func collapseHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
