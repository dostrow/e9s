package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/runbook"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type PluginSettingsSaveMsg struct{ Entries []config.PluginEntry }
type PluginSettingsDeleteMsg struct{ Index int }

type PluginSettingsModel struct {
	entries   []config.PluginEntry
	cursor    int
	editing   bool
	editIndex int
	focus     int
	name      textinput.Model
	manifest  textinput.Model
	err       string
	width     int
	height    int
}

func NewPluginSettings(entries []config.PluginEntry) PluginSettingsModel {
	return PluginSettingsModel{entries: append([]config.PluginEntry(nil), entries...), editIndex: -1}
}

func (m PluginSettingsModel) SetSize(width, height int) PluginSettingsModel {
	m.width, m.height = width, height
	inputWidth := max(24, min(72, width-24))
	m.name.Width = inputWidth
	m.manifest.Width = inputWidth
	return m
}

func (m PluginSettingsModel) Editing() bool { return m.editing }

func (m PluginSettingsModel) BeginAdd() PluginSettingsModel {
	return m.beginEdit(-1, config.PluginEntry{})
}

func (m PluginSettingsModel) BeginEditSelected() PluginSettingsModel {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return m.BeginAdd()
	}
	return m.beginEdit(m.cursor, m.entries[m.cursor])
}

func (m PluginSettingsModel) beginEdit(index int, entry config.PluginEntry) PluginSettingsModel {
	m.editing = true
	m.editIndex = index
	m.focus = 0
	m.err = ""
	m.name = textinput.New()
	m.name.Placeholder = "Optional friendly name"
	m.name.CharLimit = 200
	m.name.SetValue(entry.Name)
	m.manifest = textinput.New()
	m.manifest.Placeholder = "/path/to/repository/.e9s/plugin.yaml"
	m.manifest.CharLimit = 2000
	m.manifest.SetValue(entry.Manifest)
	m = m.SetSize(m.width, m.height)
	m.focusInput(0)
	return m
}

func (m PluginSettingsModel) Update(msg tea.Msg) (PluginSettingsModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.editing {
		switch keyMsg.String() {
		case "esc":
			m.editing = false
			m.err = ""
			return m, nil
		case "tab", "down":
			m.focusInput((m.focus + 1) % 2)
			return m, nil
		case "shift+tab", "up":
			m.focusInput((m.focus + 1) % 2)
			return m, nil
		case "ctrl+s":
			entries, err := m.editedEntries()
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.editing = false
			return m, func() tea.Msg { return PluginSettingsSaveMsg{Entries: entries} }
		}
		var cmd tea.Cmd
		if m.focus == 0 {
			m.name, cmd = m.name.Update(msg)
		} else {
			m.manifest, cmd = m.manifest.Update(msg)
		}
		m.err = ""
		return m, cmd
	}

	switch {
	case key.Matches(keyMsg, theme.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(keyMsg, theme.Keys.Down):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case keyMsg.String() == "a":
		m = m.BeginAdd()
	case keyMsg.String() == "e":
		m = m.BeginEditSelected()
	case keyMsg.String() == "d":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			index := m.cursor
			return m, func() tea.Msg { return PluginSettingsDeleteMsg{Index: index} }
		}
	}
	return m, nil
}

func (m PluginSettingsModel) View() string {
	if m.editing {
		return m.editView()
	}
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("  Settings / Plugins"))
	b.WriteString("\n\n")
	b.WriteString(theme.HelpStyle.Render("  Plugin manifests are trusted local code and are loaded only when explicitly registered here."))
	b.WriteString("\n\n")
	if len(m.entries) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No plugins registered. Press a to add one."))
		return b.String()
	}
	table := components.NewTable([]components.Column{{Title: "FRIENDLY NAME"}, {Title: "MANIFEST"}})
	for _, entry := range m.entries {
		name := entry.Name
		if strings.TrimSpace(name) == "" {
			name = "(from manifest)"
		}
		table.AddRow(components.Plain(name), components.Plain(entry.Manifest))
	}
	b.WriteString(table.Render(m.cursor, "", max(0, m.height-10)))
	return b.String()
}

func (m PluginSettingsModel) editView() string {
	title := "Add plugin"
	if m.editIndex >= 0 {
		title = "Edit plugin"
	}
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\nFriendly name (optional):\n")
	b.WriteString(m.name.View())
	b.WriteString("\n\nManifest file or directory:\n")
	b.WriteString(m.manifest.View())
	if m.err != "" {
		b.WriteString("\n\n" + theme.ErrorStyle.Render(m.err))
	}
	b.WriteString("\n\n" + theme.HelpStyle.Render("Tab/Shift+Tab move  Ctrl+S validate and save  Esc cancel"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorMauve).Padding(1, 3).Width(76).Render(b.String())
}

func (m *PluginSettingsModel) focusInput(index int) {
	m.focus = index
	m.name.Blur()
	m.manifest.Blur()
	if index == 0 {
		m.name.Focus()
	} else {
		m.manifest.Focus()
	}
}

func (m PluginSettingsModel) editedEntries() ([]config.PluginEntry, error) {
	entry := config.PluginEntry{Name: strings.TrimSpace(m.name.Value()), Manifest: strings.TrimSpace(m.manifest.Value())}
	if entry.Manifest == "" {
		return nil, fmt.Errorf("manifest path is required")
	}
	entries := append([]config.PluginEntry(nil), m.entries...)
	if m.editIndex < 0 {
		entries = append(entries, entry)
	} else if m.editIndex < len(entries) {
		entries[m.editIndex] = entry
	}
	if _, err := loadPluginActions(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func loadPluginActions(entries []config.PluginEntry) ([]runbook.ConfiguredAction, error) {
	sources := make([]runbook.Source, len(entries))
	for index, entry := range entries {
		sources[index] = runbook.Source{Name: entry.Name, Path: entry.Manifest}
	}
	return runbook.LoadSources(sources, filepath.Dir(config.Path()))
}

func pluginEntryLabel(entry config.PluginEntry) string {
	if name := strings.TrimSpace(entry.Name); name != "" {
		return name
	}
	if base := filepath.Base(strings.TrimSpace(entry.Manifest)); base != "." && base != "" {
		return base
	}
	return "plugin"
}

func (a App) savePluginEntries(entries []config.PluginEntry) (App, tea.Cmd) {
	updated := *a.cfg
	updated.Plugins = append([]config.PluginEntry(nil), entries...)
	if err := updated.Validate(); err != nil {
		a.err = fmt.Errorf("invalid plugin settings: %w", err)
		return a, nil
	}
	if _, err := loadPluginActions(updated.Plugins); err != nil {
		a.err = fmt.Errorf("invalid plugin settings: %w", err)
		return a, nil
	}
	if err := updated.Save(); err != nil {
		a.err = fmt.Errorf("save plugin settings: %w", err)
		return a, nil
	}
	*a.cfg = updated
	a.err = nil
	a = a.reloadConfiguredRunbooks(a.cfg)
	a.pluginSettingsView = NewPluginSettings(a.cfg.Plugins).SetSize(a.width-3, a.height-6)
	a.pendingPluginDelete = -1
	a.configModTime = config.ModTime()
	a.flashMessage = "Plugin settings saved"
	a.flashExpiry = time.Now().Add(3 * time.Second)
	return a, nil
}

func (a App) deletePendingPlugin() (App, tea.Cmd) {
	index := a.pendingPluginDelete
	a.pendingPluginDelete = -1
	if index < 0 || index >= len(a.cfg.Plugins) {
		a.err = fmt.Errorf("plugin registration is no longer available")
		return a, nil
	}
	entries := append([]config.PluginEntry(nil), a.cfg.Plugins...)
	entries = append(entries[:index], entries[index+1:]...)
	return a.savePluginEntries(entries)
}
