package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/runbook"
	"github.com/dostrow/e9s/internal/ui/theme"
	"github.com/dostrow/e9s/internal/ui/views"
)

type RunbookSubmitMsg struct{ Invocation runbook.Invocation }
type RunbookCancelMsg struct{}
type runbookDoneMsg struct{ err error }

type runbookFormField struct {
	spec        runbook.Input
	text        textinput.Model
	boolean     bool
	selectIndex int
}

type RunbookFormModel struct {
	Active  bool
	action  runbook.ConfiguredAction
	context runbook.Context
	fields  []runbookFormField
	focus   int
	err     string
	height  int
}

func NewRunbookForm(action runbook.ConfiguredAction, context runbook.Context) RunbookFormModel {
	initial := runbook.InitialValues(action, context)
	form := RunbookFormModel{Active: true, action: action, context: context}
	for _, input := range action.Action.Inputs {
		field := runbookFormField{spec: input}
		switch input.Type {
		case "boolean":
			field.boolean, _ = strconv.ParseBool(initial[input.ID])
		case "select":
			for index, option := range input.Options {
				if option.Value == initial[input.ID] {
					field.selectIndex = index
				}
			}
		default:
			field.text = textinput.New()
			field.text.Width = 30
			field.text.CharLimit = 1000
			field.text.Placeholder = input.Placeholder
			field.text.SetValue(initial[input.ID])
		}
		form.fields = append(form.fields, field)
	}
	form.focusField(0)
	return form
}

func (m RunbookFormModel) Update(msg tea.Msg) (RunbookFormModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || !m.Active {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.Active = false
		return m, func() tea.Msg { return RunbookCancelMsg{} }
	case "ctrl+s":
		invocation, err := runbook.BuildInvocation(m.action, m.values(), m.context)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.Active = false
		return m, func() tea.Msg { return RunbookSubmitMsg{Invocation: invocation} }
	case "tab", "down":
		if len(m.fields) > 0 {
			m.focusField((m.focus + 1) % len(m.fields))
		}
		return m, nil
	case "shift+tab", "up":
		if len(m.fields) > 0 {
			m.focusField((m.focus + len(m.fields) - 1) % len(m.fields))
		}
		return m, nil
	case "left":
		m.changeChoice(-1)
		return m, nil
	case "right", " ":
		m.changeChoice(1)
		return m, nil
	}
	if m.focus >= 0 && m.focus < len(m.fields) && m.fields[m.focus].spec.Type == "text" {
		var cmd tea.Cmd
		m.fields[m.focus].text, cmd = m.fields[m.focus].text.Update(msg)
		m.err = ""
		return m, cmd
	}
	return m, nil
}

func (m RunbookFormModel) View() string {
	if !m.Active {
		return ""
	}
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render(m.action.PluginDisplayName + " — " + m.action.Action.Title))
	if m.action.Action.Description != "" {
		b.WriteString("\n" + theme.HelpStyle.Render(m.action.Action.Description))
	}
	b.WriteString("\n\n")
	if len(m.fields) == 0 {
		b.WriteString(theme.HelpStyle.Render("This action has no launch parameters.") + "\n")
	}
	start, end := m.visibleFieldRange()
	if start > 0 || end < len(m.fields) {
		fmt.Fprintf(&b, "%s\n", theme.HelpStyle.Render(fmt.Sprintf("Fields %d–%d of %d", start+1, end, len(m.fields))))
	}
	for index := start; index < end; index++ {
		field := m.fields[index]
		labelStyle := theme.HelpStyle
		if index == m.focus {
			labelStyle = theme.TitleStyle
		}
		label := field.spec.Label
		if field.spec.Required {
			label += " *"
		}
		b.WriteString(labelStyle.Render(label+":") + "  ")
		switch field.spec.Type {
		case "boolean":
			if field.boolean {
				b.WriteString("[x]")
			} else {
				b.WriteString("[ ]")
			}
		case "select":
			if len(field.spec.Options) > 0 {
				b.WriteString("<" + field.spec.Options[field.selectIndex].Label + ">")
			}
		default:
			b.WriteString(field.text.View())
		}
		b.WriteString("\n")
		if index == m.focus && field.spec.Description != "" {
			b.WriteString("  " + theme.HelpStyle.Render(field.spec.Description) + "\n")
		}
	}
	if m.err != "" {
		b.WriteString("\n" + theme.ErrorStyle.Render(m.err) + "\n")
	}
	b.WriteString("\n" + theme.HelpStyle.Render("Tab/Shift+Tab move  Space/←/→ change options  Ctrl+S review  Esc cancel"))
	width := 72
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorMauve).Padding(1, 3).Width(width).Render(b.String())
}

func (m RunbookFormModel) SetHeight(height int) RunbookFormModel {
	m.height = height
	return m
}

func (m RunbookFormModel) visibleFieldRange() (int, int) {
	count := len(m.fields)
	maximum := m.height - 10
	if maximum < 4 {
		maximum = 4
	}
	if count <= maximum {
		return 0, count
	}
	start := m.focus - maximum/2
	if start < 0 {
		start = 0
	}
	if start+maximum > count {
		start = count - maximum
	}
	return start, start + maximum
}

func (m *RunbookFormModel) focusField(index int) {
	if len(m.fields) == 0 {
		m.focus = -1
		return
	}
	m.focus = index
	for fieldIndex := range m.fields {
		if m.fields[fieldIndex].spec.Type == "text" {
			m.fields[fieldIndex].text.Blur()
		}
	}
	if m.fields[m.focus].spec.Type == "text" {
		m.fields[m.focus].text.Focus()
	}
}

func (m *RunbookFormModel) changeChoice(delta int) {
	if m.focus < 0 || m.focus >= len(m.fields) {
		return
	}
	field := &m.fields[m.focus]
	switch field.spec.Type {
	case "boolean":
		field.boolean = !field.boolean
	case "select":
		if len(field.spec.Options) > 0 {
			field.selectIndex = (field.selectIndex + delta + len(field.spec.Options)) % len(field.spec.Options)
		}
	}
}

func (m RunbookFormModel) values() map[string]string {
	values := make(map[string]string, len(m.fields))
	for _, field := range m.fields {
		switch field.spec.Type {
		case "boolean":
			values[field.spec.ID] = strconv.FormatBool(field.boolean)
		case "select":
			if len(field.spec.Options) > 0 {
				values[field.spec.ID] = field.spec.Options[field.selectIndex].Value
			}
		default:
			values[field.spec.ID] = field.text.Value()
		}
	}
	return values
}

func runbookConfirmation(invocation runbook.Invocation) string {
	return fmt.Sprintf("Run %s?\n\nRisk: %s\nExecution: %s\nCommand: %s\nLaunch directory: %s\nWorking directory: %s",
		invocation.Title, invocation.Risk, invocation.ExecutionMode(), invocation.DisplayCommand(), invocation.LaunchDirectory, invocation.WorkingDirectory)
}

func loadConfiguredRunbooks(cfg *config.Config) ([]runbook.ConfiguredAction, error) {
	sources := make([]runbook.Source, len(cfg.Plugins))
	for index, plugin := range cfg.Plugins {
		sources[index] = runbook.Source{Name: plugin.Name, Path: plugin.Manifest}
	}
	return runbook.LoadSources(sources, filepath.Dir(config.Path()))
}

func (a App) reloadConfiguredRunbooks(cfg *config.Config) App {
	actions, err := loadConfiguredRunbooks(cfg)
	if err != nil {
		a.err = err
		return a
	}
	a.runbookActions = actions
	a.runbooksView = views.NewRunbooks(actions).SetSize(a.width-3, a.height-6)
	if !a.pluginSettingsView.Editing() {
		a.pluginSettingsView = NewPluginSettings(cfg.Plugins).SetSize(a.width-3, a.height-6)
	}
	pluginTab := -1
	for index, tab := range a.modeTabs {
		if tab.Mode == modePlugins {
			pluginTab = index
			break
		}
	}
	if len(actions) > 0 && pluginTab < 0 {
		a.modeTabs = append(a.modeTabs, ModeTab{Mode: modePlugins, Label: "PLUG", Key: strconv.Itoa(len(a.modeTabs) + 1)})
	}
	if len(actions) == 0 && pluginTab >= 0 {
		a.modeTabs = append(a.modeTabs[:pluginTab], a.modeTabs[pluginTab+1:]...)
	}
	for index := range a.modeTabs {
		a.modeTabs[index].Key = strconv.Itoa(index + 1)
	}
	return a
}

func (a App) runPendingRunbook() (App, tea.Cmd) {
	if a.pendingRunbook == nil {
		a.err = fmt.Errorf("plugin invocation is no longer available")
		return a, nil
	}
	invocation := *a.pendingRunbook
	a.pendingRunbook = nil
	executable, args, err := invocation.ExecutionCommand(a.cfg.GUI.TerminalShell)
	if err != nil {
		a.err = err
		return a, nil
	}
	wrap := NewExecWrapInDirectory(executable, args, invocation.WorkingDirectory, invocation.Environment)
	return a, tea.Exec(wrap, func(err error) tea.Msg { return runbookDoneMsg{err: err} })
}
