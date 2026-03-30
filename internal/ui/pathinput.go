package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

const pathInputMaxMatches = 8

// PathInput wraps a textinput with filesystem path completion.
// As the user types, it lists matching directories below the input
// and offers them as tab-completable suggestions.
type PathInput struct {
	Action  InputAction
	Prompt  string
	input   textinput.Model
	matches []string // current directory matches shown below input
	hasTF   bool     // true if current path contains .tf files
	errMsg  string   // validation error to show
	width   int
	height  int
}

// PathInputResultMsg is returned when the user submits a path.
type PathInputResultMsg struct {
	Action InputAction
	Value  string
}

// PathInputCancelMsg is returned when the user cancels.
type PathInputCancelMsg struct{}

func NewPathInput(action InputAction, prompt, defaultVal string) PathInput {
	ti := textinput.New()
	ti.Placeholder = "~/Projects/infra"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60
	ti.ShowSuggestions = true

	if defaultVal != "" {
		ti.SetValue(defaultVal)
		ti.CursorEnd()
	}

	p := PathInput{
		Action: action,
		Prompt: prompt,
		input:  ti,
		width:  80,
		height: 30,
	}
	p.updateSuggestions()
	return p
}

func (p PathInput) Update(msg tea.Msg) (PathInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := strings.TrimRight(p.input.Value(), "/")
			val = expandHome(val)
			// Validate: directory must contain .tf files
			if !dirHasTFFiles(val) {
				p.errMsg = fmt.Sprintf("No .tf files found in %s", collapseHome(val))
				return p, nil
			}
			p.errMsg = ""
			return p, func() tea.Msg {
				return PathInputResultMsg{Action: p.Action, Value: val}
			}
		case "esc":
			return p, func() tea.Msg { return PathInputCancelMsg{} }
		case "tab":
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			p.updateSuggestions()
			return p, cmd
		}
	}

	p.errMsg = ""
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.updateSuggestions()
	return p, cmd
}

func (p PathInput) View() string {
	var b strings.Builder

	// Fixed-height content area
	lines := make([]string, 0, pathInputMaxMatches+8)

	lines = append(lines, theme.TitleStyle.Render("  "+p.Prompt))
	lines = append(lines, "")
	lines = append(lines, "  "+p.input.View())

	// TF file indicator
	if p.hasTF {
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorGreen).Render(
			"  ✓ contains .tf files — press Enter to select"))
	} else {
		val := strings.TrimRight(p.input.Value(), "/")
		expanded := expandHome(val)
		if isDir(expanded) {
			lines = append(lines, theme.HelpStyle.Render("  no .tf files in this directory"))
		} else {
			lines = append(lines, "")
		}
	}

	// Error message
	if p.errMsg != "" {
		lines = append(lines, theme.ErrorStyle.Render("  "+p.errMsg))
	} else {
		lines = append(lines, "")
	}

	// Directory matches (fixed slot count)
	lines = append(lines, theme.HelpStyle.Render("  Directories:"))
	if len(p.matches) == 0 {
		lines = append(lines, theme.HelpStyle.Render("    (no matches)"))
		for i := 0; i < pathInputMaxMatches-1; i++ {
			lines = append(lines, "")
		}
	} else {
		shown := p.matches
		if len(shown) > pathInputMaxMatches {
			shown = shown[:pathInputMaxMatches]
		}
		for _, m := range shown {
			lines = append(lines, theme.HelpStyle.Render("    "+m))
		}
		// Pad to fixed height
		for i := len(shown); i < pathInputMaxMatches; i++ {
			lines = append(lines, "")
		}
		if len(p.matches) > pathInputMaxMatches {
			lines = append(lines, theme.HelpStyle.Render(
				fmt.Sprintf("    ... and %d more", len(p.matches)-pathInputMaxMatches)))
		} else {
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	lines = append(lines, theme.HelpStyle.Render("  [tab] complete  [enter] select  [esc] cancel"))

	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (p *PathInput) updateSuggestions() {
	val := p.input.Value()
	expanded := expandHome(val)

	// Check if current path has .tf files
	checkDir := strings.TrimRight(expanded, "/")
	p.hasTF = dirHasTFFiles(checkDir)

	// Get the directory to list and the prefix to match
	dir := expanded
	prefix := ""
	if !strings.HasSuffix(expanded, "/") {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		p.matches = nil
		p.input.SetSuggestions(nil)
		return
	}

	var suggestions []string
	var displayMatches []string
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

		// Mark directories that contain .tf files
		if dirHasTFFiles(fullPath) {
			display += " ✓"
		}
		displayMatches = append(displayMatches, display)

		suggestion := collapseHome(fullPath) + "/"
		if strings.HasPrefix(p.input.Value(), "/") {
			suggestion = fullPath + "/"
		}
		suggestions = append(suggestions, suggestion)
	}

	p.matches = displayMatches
	p.input.SetSuggestions(suggestions)
}

func (p PathInput) SetSize(w, h int) PathInput {
	p.width = w
	p.height = h
	if w > 10 {
		p.input.Width = w - 6
	}
	return p
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

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
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
