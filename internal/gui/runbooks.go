//go:build gui

package gui

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/runbook"
)

func loadGUIRunbooks(cfg *config.Config) ([]runbook.ConfiguredAction, error) {
	return loadPluginEntries(cfg.Plugins)
}

func (w *mainWindow) populatePluginRail(sections []moduleRailSection) {
	if w.pluginRailContainer == nil {
		return
	}
	for child := w.pluginRailContainer.FirstChild(); child != nil; child = w.pluginRailContainer.FirstChild() {
		w.pluginRailContainer.Remove(child)
	}
	w.pluginRailContainer.SetVisible(len(sections) > 0)
	if len(sections) == 0 {
		return
	}
	label := gtk.NewLabel("PLUGINS")
	label.SetXAlign(0)
	label.SetMarginTop(8)
	label.AddCSSClass("section-title")
	label.AddCSSClass("module-group-title")
	w.pluginRailContainer.Append(label)
	for _, section := range sections {
		w.pluginRailContainer.Append(section.expander)
	}
}

func (w *mainWindow) reloadPluginConfiguration(cfg *config.Config) error {
	actions, err := loadGUIRunbooks(cfg)
	if err != nil {
		return err
	}
	base := make([]moduleRailSection, 0, len(w.moduleSections))
	for _, section := range w.moduleSections {
		if !strings.HasPrefix(section.key, "plugin:") {
			base = append(base, section)
		}
	}
	w.runbookActions = actions
	w.pluginLoadError = nil
	w.pluginActionButtons = nil
	pluginSections := w.buildPluginModuleSections()
	w.moduleSections = append(base, pluginSections...)
	sortModuleRailSections(w.moduleSections)
	w.populatePluginRail(pluginSections)
	w.updateActionSensitivity()
	return nil
}

func (w *mainWindow) buildPluginModuleSections() []moduleRailSection {
	var sections []moduleRailSection
	for start := 0; start < len(w.runbookActions); {
		end := start + 1
		for end < len(w.runbookActions) && w.runbookActions[end].PluginName == w.runbookActions[start].PluginName {
			end++
		}
		actions := append([]runbook.ConfiguredAction(nil), w.runbookActions[start:end]...)
		items := gtk.NewBox(gtk.OrientationVertical, 2)
		items.AddCSSClass("module-subitems")
		for _, configuredAction := range actions {
			action := configuredAction
			button := newPluginRailButton(action.Action.Title, func() { w.promptRunbook(action) })
			if action.Action.Description != "" {
				button.SetTooltipText(action.Action.Description)
			}
			items.Append(button)
			w.pluginActionButtons = append(w.pluginActionButtons, button)
		}
		pluginName := actions[0].PluginDisplayName
		key := "plugin:" + actions[0].PluginName
		expander := w.newModuleExpander(pluginName, key, items)
		sections = append(sections, moduleRailSection{
			key: key, name: pluginName, defaultItem: "Choose action",
			aliases: []string{actions[0].PluginName}, expander: expander,
			activate: func() { w.showRunbookPicker(pluginName, actions) },
		})
		start = end
	}
	return sections
}

func newPluginRailButton(label string, activate func()) *gtk.Button {
	text := gtk.NewLabel(label)
	text.SetXAlign(0)
	text.SetHAlign(gtk.AlignFill)
	text.SetHExpand(true)
	text.SetEllipsize(pango.EllipsizeEnd)
	button := gtk.NewButton()
	button.SetChild(text)
	button.SetHAlign(gtk.AlignFill)
	button.AddCSSClass("flat")
	button.AddCSSClass("module-subitem")
	button.ConnectClicked(activate)
	return button
}

func (w *mainWindow) showRunbookPicker(pluginName string, actions []runbook.ConfiguredAction) {
	if len(actions) == 1 {
		w.promptRunbook(actions[0])
		return
	}
	labels := make([]string, len(actions))
	for index, action := range actions {
		labels[index] = action.Action.Title
	}
	dialog := gtk.NewDialogWithFlags(pluginName, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(10)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Choose a plugin action")
	label.SetXAlign(0)
	selector := gtk.NewDropDownFromStrings(labels)
	selector.SetHExpand(true)
	content.Append(label)
	content.Append(selector)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Continue", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		dialog.Destroy()
		if response == int(gtk.ResponseOK) && index >= 0 && index < len(actions) {
			w.promptRunbook(actions[index])
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptRunbook(action runbook.ConfiguredAction) {
	if w.pluginActionPending {
		w.setStatus("A plugin action is already running", false)
		return
	}
	dialog := gtk.NewDialogWithFlags(action.Action.Title, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(650, 700)
	content := dialog.ContentArea()
	content.SetSpacing(10)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	if action.Action.Description != "" {
		description := gtk.NewLabel(action.Action.Description)
		description.SetXAlign(0)
		description.SetWrap(true)
		description.AddCSSClass("muted")
		content.Append(description)
	}
	form := gtk.NewBox(gtk.OrientationVertical, 10)
	scroll := gtk.NewScrolledWindow()
	scroll.SetHExpand(true)
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(form)
	content.Append(scroll)
	context := runbook.Context{Region: w.options.Region, Profile: w.options.Profile}
	initial := runbook.InitialValues(action, context)
	readers := make(map[string]func() string, len(action.Action.Inputs))
	for _, input := range action.Action.Inputs {
		input := input
		labelText := input.Label
		if input.Required {
			labelText += " *"
		}
		label := gtk.NewLabel(labelText)
		label.SetXAlign(0)
		form.Append(label)
		switch input.Type {
		case "boolean":
			check := newApplicationCheckButton(input.Description)
			active, _ := strconv.ParseBool(initial[input.ID])
			check.SetActive(active)
			form.Append(check)
			readers[input.ID] = func() string { return strconv.FormatBool(check.Active()) }
		case "select":
			labels := make([]string, len(input.Options))
			selected := uint(0)
			for index, option := range input.Options {
				labels[index] = option.Label
				if option.Value == initial[input.ID] {
					selected = uint(index)
				}
			}
			selector := gtk.NewDropDownFromStrings(labels)
			selector.SetSelected(selected)
			selector.SetHExpand(true)
			form.Append(selector)
			readers[input.ID] = func() string {
				index := int(selector.Selected())
				if index < 0 || index >= len(input.Options) {
					return ""
				}
				return input.Options[index].Value
			}
		default:
			entry := gtk.NewEntry()
			entry.SetHExpand(true)
			entry.SetPlaceholderText(input.Placeholder)
			entry.SetText(initial[input.ID])
			form.Append(entry)
			readers[input.ID] = entry.Text
		}
		if input.Type != "boolean" && input.Description != "" {
			description := gtk.NewLabel(input.Description)
			description.SetXAlign(0)
			description.SetWrap(true)
			description.AddCSSClass("muted")
			form.Append(description)
		}
	}
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		values := make(map[string]string, len(readers))
		for id, read := range readers {
			values[id] = read()
		}
		invocation, err := runbook.BuildInvocation(action, values, context)
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		dialog.Destroy()
		w.confirmRunbook(invocation)
	})
	dialog.Present()
}

func (w *mainWindow) confirmRunbook(invocation runbook.Invocation) {
	dialog := gtk.NewDialogWithFlags("Review plugin action", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(720, -1)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	title := gtk.NewLabel("")
	title.SetMarkup("Run <b>" + html.EscapeString(invocation.Title) + "</b>?")
	title.SetXAlign(0)
	content.Append(title)
	for _, line := range []string{
		"Risk: " + invocation.Risk,
		"Execution: " + invocation.ExecutionMode(),
		"Command: " + invocation.DisplayCommand(),
		"Launch directory: " + invocation.LaunchDirectory,
		"Working directory: " + invocation.WorkingDirectory,
	} {
		label := gtk.NewLabel(line)
		label.SetXAlign(0)
		label.SetSelectable(true)
		label.SetWrap(true)
		content.Append(label)
	}
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Run", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.startRunbook(invocation)
		}
	})
	dialog.Present()
}

func (w *mainWindow) startRunbook(invocation runbook.Invocation) {
	if !vteAvailable() {
		w.setStatus("Plugin action requires embedded terminal support", true)
		return
	}
	if w.terminal == nil || w.pluginActionPending {
		return
	}
	if w.terminal.Running() {
		w.setStatus("The operation terminal is already in use", true)
		return
	}
	configuredShell := ""
	if w.options.Config != nil {
		configuredShell = w.options.Config.GUI.TerminalShell
	}
	executable, args, err := invocation.ExecutionCommand(configuredShell)
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	if err := w.terminal.SpawnWithEnvironment(executable, args, invocation.WorkingDirectory, invocation.Environment); err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.pluginTerminalGeneration++
	generation := w.pluginTerminalGeneration
	w.pluginActionPending = true
	w.updateActionSensitivity()
	w.showingLogs = false
	w.showingMetrics = false
	w.showingTerminal = true
	w.terminalDescription = "Plugin action " + invocation.Title
	w.terminalTitle.SetLabel(invocation.Title + " — " + invocation.WorkingDirectory)
	w.terminalOnClose = func() {
		w.pluginActionPending = false
		w.updateActionSensitivity()
	}
	w.detailStack.SetVisibleChildName("terminal")
	w.setStatus("Running plugin action "+invocation.Title+"…", false)
	glib.TimeoutAdd(200, func() bool {
		if generation != w.pluginTerminalGeneration || !w.showingTerminal {
			return false
		}
		if w.terminal.Running() {
			return true
		}
		status := w.terminal.ExitStatus()
		w.pluginActionPending = false
		w.updateActionSensitivity()
		if status == 0 {
			w.terminalTitle.SetLabel(invocation.Title + " completed")
			w.setStatus("Plugin action "+invocation.Title+" completed", false)
		} else {
			w.terminalTitle.SetLabel(invocation.Title + " failed")
			w.setStatus(fmt.Sprintf("Plugin action %s failed (status %d)", invocation.Title, status), true)
		}
		return false
	})
}
