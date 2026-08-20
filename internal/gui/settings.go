//go:build gui

package gui

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"gopkg.in/yaml.v3"
)

type settingsChoice struct {
	label string
	value string
}

var settingsModuleChoices = []settingsChoice{
	{label: "Ask when e9s starts", value: ""},
	{label: "CloudWatch Alarms", value: "CloudWatch Alarms"},
	{label: "CloudWatch Logs", value: "CloudWatch Logs"},
	{label: "CodeBuild", value: "CodeBuild"},
	{label: "DynamoDB", value: "DynamoDB"},
	{label: "EC2", value: "EC2"},
	{label: "ECR", value: "ECR"},
	{label: "ECS", value: "ECS"},
	{label: "Lambda", value: "Lambda"},
	{label: "OpenTofu", value: "Tofu"},
	{label: "RDS", value: "RDS"},
	{label: "Route 53", value: "Route53"},
	{label: "S3", value: "S3"},
	{label: "Secrets Manager", value: "SM"},
	{label: "SQS", value: "SQS"},
	{label: "SSM Parameter Store", value: "SSM"},
}

func settingsChoiceIndex(choices []settingsChoice, value string) int {
	value = strings.TrimSpace(value)
	for index, choice := range choices {
		if strings.EqualFold(choice.value, value) {
			return index
		}
	}
	return 0
}

func settingsChoiceLabels(choices []settingsChoice) []string {
	labels := make([]string, len(choices))
	for index, choice := range choices {
		labels[index] = choice.label
	}
	return labels
}

func settingsPage() *gtk.Box {
	page := gtk.NewBox(gtk.OrientationVertical, 10)
	page.SetMarginTop(16)
	page.SetMarginBottom(16)
	page.SetMarginStart(16)
	page.SetMarginEnd(16)
	return page
}

func settingsRow(labelText string, control gtk.Widgetter) *gtk.Box {
	label := gtk.NewLabel(labelText)
	label.SetXAlign(0)
	label.SetHExpand(true)
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.Append(label)
	row.Append(control)
	return row
}

func settingsNote(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.SetXAlign(0)
	label.SetWrap(true)
	label.AddCSSClass("muted")
	return label
}

func (w *mainWindow) showSettings() {
	if w.settingsDialog != nil {
		w.settingsDialog.Present()
		return
	}

	cfg := config.DefaultConfig()
	if w.options.Config != nil {
		cfg = *w.options.Config
	}
	raw, err := config.ReadRaw()
	if err != nil {
		raw, _ = yaml.Marshal(&cfg)
	}

	dialog := gtk.NewDialogWithFlags("e9s Settings", &w.window.Window, gtk.DialogModal)
	w.settingsDialog = dialog
	dialog.AddCSSClass("e9s-settings")
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(760, 600)
	content := dialog.ContentArea()
	content.AddCSSClass("e9s-dialog-surface")
	content.SetSpacing(8)
	notebook := gtk.NewNotebook()
	notebook.SetHExpand(true)
	notebook.SetVExpand(true)
	content.Append(notebook)

	general := settingsPage()
	moduleSelector := gtk.NewDropDownFromStrings(settingsChoiceLabels(settingsModuleChoices))
	moduleSelector.SetSelected(uint(settingsChoiceIndex(settingsModuleChoices, cfg.Defaults.DefaultMode)))
	refreshInterval := gtk.NewSpinButtonWithRange(1, 3600, 1)
	refreshInterval.SetValue(float64(cfg.Defaults.RefreshInterval))
	idleTimeout := gtk.NewSpinButtonWithRange(0, 86400, 1)
	idleTimeout.SetValue(float64(cfg.Defaults.IdleTimeout))
	timestampChoices := []settingsChoice{{label: "Relative", value: "relative"}, {label: "Absolute", value: "absolute"}}
	timestampSelector := gtk.NewDropDownFromStrings(settingsChoiceLabels(timestampChoices))
	timestampSelector.SetSelected(uint(settingsChoiceIndex(timestampChoices, cfg.Display.TimestampFormat)))
	general.Append(settingsRow("Default module", moduleSelector))
	general.Append(settingsRow("Refresh interval (seconds)", refreshInterval))
	general.Append(settingsRow("Pause refresh after inactivity (seconds; 0 disables)", idleTimeout))
	general.Append(settingsRow("Timestamps", timestampSelector))
	notebook.AppendPage(general, gtk.NewLabel("General"))

	appearancePage := settingsPage()
	presetSelector := gtk.NewDropDownFromStrings(appearancePresetLabels())
	presetSelector.SetSelected(uint(appearancePresetIndex(cfg.GUI.Appearance.Preset)))
	interfaceSystem := gtk.NewCheckButtonWithLabel("Use GTK interface font")
	interfaceSystem.SetActive(strings.TrimSpace(cfg.GUI.Appearance.InterfaceFont) == "")
	interfaceFontName := cfg.GUI.Appearance.InterfaceFont
	if strings.TrimSpace(interfaceFontName) == "" {
		interfaceFontName = "Sans 11"
	}
	interfaceFont := gtk.NewFontButtonWithFont(interfaceFontName)
	interfaceFont.SetTitle("Choose interface font")
	interfaceFont.SetUseFont(true)
	interfaceFont.SetUseSize(true)
	interfaceFont.SetSensitive(!interfaceSystem.Active())
	monospaceSystem := gtk.NewCheckButtonWithLabel("Use GTK monospace font")
	monospaceSystem.SetActive(strings.TrimSpace(cfg.GUI.Appearance.MonospaceFont) == "")
	monospaceFontName := cfg.GUI.Appearance.MonospaceFont
	if strings.TrimSpace(monospaceFontName) == "" {
		monospaceFontName = "Monospace 10"
	}
	monospaceFont := gtk.NewFontButtonWithFont(monospaceFontName)
	monospaceFont.SetTitle("Choose monospace font")
	monospaceFont.SetUseFont(true)
	monospaceFont.SetUseSize(true)
	monospaceFont.SetSensitive(!monospaceSystem.Active())
	appearancePage.Append(settingsRow("Color preset", presetSelector))
	appearancePage.Append(interfaceSystem)
	appearancePage.Append(settingsRow("Interface font", interfaceFont))
	appearancePage.Append(monospaceSystem)
	appearancePage.Append(settingsRow("Editor, logs, details, and terminal font", monospaceFont))
	appearancePage.Append(settingsNote("System follows the active GTK theme. Built-in presets provide a consistent fallback on desktops where installing GTK themes is less convenient."))
	notebook.AppendPage(appearancePage, gtk.NewLabel("Appearance"))

	appearanceDraft := func() config.Config {
		preview := cfg
		presetIndex := int(presetSelector.Selected())
		presetNames := appearancePresetNames()
		if presetIndex >= 0 && presetIndex < len(presetNames) {
			preview.GUI.Appearance.Preset = presetNames[presetIndex]
		}
		if interfaceSystem.Active() {
			preview.GUI.Appearance.InterfaceFont = ""
		} else {
			preview.GUI.Appearance.InterfaceFont = interfaceFont.Font()
		}
		if monospaceSystem.Active() {
			preview.GUI.Appearance.MonospaceFont = ""
		} else {
			preview.GUI.Appearance.MonospaceFont = monospaceFont.Font()
		}
		return preview
	}
	previewAppearance := func() {
		preview := appearanceDraft()
		w.applyAppearanceConfig(&preview)
	}
	presetSelector.NotifyProperty("selected", previewAppearance)
	interfaceSystem.ConnectToggled(func() {
		interfaceFont.SetSensitive(!interfaceSystem.Active())
		previewAppearance()
	})
	monospaceSystem.ConnectToggled(func() {
		monospaceFont.SetSensitive(!monospaceSystem.Active())
		previewAppearance()
	})
	interfaceFont.ConnectFontSet(func() {
		interfaceSystem.SetActive(false)
		previewAppearance()
	})
	monospaceFont.ConnectFontSet(func() {
		monospaceSystem.SetActive(false)
		previewAppearance()
	})

	awsPage := settingsPage()
	profile := gtk.NewEntry()
	profile.SetText(cfg.Defaults.Profile)
	region := gtk.NewEntry()
	region.SetText(cfg.Defaults.Region)
	cluster := gtk.NewEntry()
	cluster.SetText(cfg.Defaults.Cluster)
	saveDirectory := gtk.NewEntry()
	saveDirectory.SetText(cfg.Defaults.SaveDirectory)
	awsPage.Append(settingsRow("AWS profile", profile))
	awsPage.Append(settingsRow("AWS region", region))
	awsPage.Append(settingsRow("Default ECS cluster", cluster))
	awsPage.Append(settingsRow("Save directory", saveDirectory))
	awsPage.Append(settingsNote("Profile and region changes are saved immediately and take effect the next time e9s starts. Existing AWS clients are never replaced mid-request."))
	notebook.AppendPage(awsPage, gtk.NewLabel("AWS"))

	logsPage := settingsPage()
	maxEvents := gtk.NewSpinButtonWithRange(1, 100000, 50)
	maxEvents.SetValue(float64(cfg.Display.MaxEvents))
	maxLogLines := gtk.NewSpinButtonWithRange(100, 1000000, 100)
	maxLogLines.SetValue(float64(cfg.Display.MaxLogLines))
	logsPage.Append(settingsRow("Default event page size", maxEvents))
	logsPage.Append(settingsRow("Maximum buffered log lines", maxLogLines))
	logsPage.Append(settingsNote("Changes apply to newly opened log buffers. Existing buffers keep their current limits."))
	notebook.AppendPage(logsPage, gtk.NewLabel("Logs"))

	editorPage := settingsPage()
	terminalShell := gtk.NewEntry()
	terminalShell.SetText(cfg.GUI.TerminalShell)
	terminalShell.SetPlaceholderText("Use $SHELL")
	editorPage.Append(settingsRow("Terminal shell", terminalShell))
	editorPage.Append(settingsNote("The selected shell is used the next time the terminal dock starts. Editor keymaps remain in the standard GTK mode; Vim and Helix modes are documented future work."))
	notebook.AppendPage(editorPage, gtk.NewLabel("Editor & Terminal"))

	safetyPage := settingsPage()
	confirmActions := gtk.NewCheckButtonWithLabel("Require confirmation for destructive actions")
	confirmActions.SetActive(true)
	confirmActions.SetSensitive(false)
	safetyPage.Append(confirmActions)
	safetyPage.Append(settingsNote("Safety confirmations cannot currently be disabled. Break-glass controls for infrastructure-as-code-managed resources are planned separately."))
	notebook.AppendPage(safetyPage, gtk.NewLabel("Safety"))

	advancedPage := settingsPage()
	advancedToggle := gtk.NewSwitch()
	advancedPage.Append(settingsRow("Edit configuration as YAML", advancedToggle))
	advancedPage.Append(settingsNote("Advanced mode edits the complete configuration. Apply validates the YAML and shows a review before an atomic save. The prior file is retained as config.yaml.bak."))
	advancedPage.Append(settingsNote("Configuration: " + config.Path()))
	advancedEditor := newSourceEditor(sourceDocument{Path: "config.yaml", Language: "yaml"})
	advancedEditor.SetText(string(raw))
	advancedEditor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	advancedScroll := gtk.NewScrolledWindow()
	advancedScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	advancedScroll.SetMinContentHeight(360)
	advancedScroll.SetVExpand(true)
	advancedScroll.SetChild(advancedEditor.Widget())
	advancedScroll.SetVisible(false)
	advancedPage.Append(advancedScroll)
	advancedToggle.ConnectStateSet(func(state bool) bool {
		advancedScroll.SetVisible(state)
		return false
	})
	notebook.AppendPage(advancedPage, gtk.NewLabel("Advanced"))

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Apply", int(gtk.ResponseApply))
	dialog.AddButton("OK", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	closing := false
	restoreAndClose := func() {
		if closing {
			return
		}
		closing = true
		restored := cfg
		glib.IdleAdd(func() {
			dialog.Destroy()
			w.applyAppearanceConfig(&restored)
		})
	}
	dialog.ConnectDestroy(func() { w.settingsDialog = nil })
	dialog.ConnectCloseRequest(func() bool {
		restoreAndClose()
		return true
	})
	dialog.ConnectResponse(func(response int) {
		closeAfterSave := response == int(gtk.ResponseOK)
		if response != int(gtk.ResponseApply) && !closeAfterSave {
			restoreAndClose()
			return
		}
		if advancedToggle.Active() {
			w.reviewRawSettings(dialog, raw, []byte(advancedEditor.Text()), closeAfterSave, func(saved config.Config, savedRaw []byte) {
				cfg = saved
				raw = append(raw[:0], savedRaw...)
				advancedEditor.SetText(string(savedRaw))
			})
			return
		}

		updated := cfg
		moduleIndex := int(moduleSelector.Selected())
		if moduleIndex >= 0 && moduleIndex < len(settingsModuleChoices) {
			updated.Defaults.DefaultMode = settingsModuleChoices[moduleIndex].value
		}
		updated.Defaults.RefreshInterval = refreshInterval.ValueAsInt()
		updated.Defaults.IdleTimeout = idleTimeout.ValueAsInt()
		timestampIndex := int(timestampSelector.Selected())
		if timestampIndex >= 0 && timestampIndex < len(timestampChoices) {
			updated.Display.TimestampFormat = timestampChoices[timestampIndex].value
		}
		updated.Defaults.Profile = strings.TrimSpace(profile.Text())
		updated.Defaults.Region = strings.TrimSpace(region.Text())
		updated.Defaults.Cluster = strings.TrimSpace(cluster.Text())
		updated.Defaults.SaveDirectory = strings.TrimSpace(saveDirectory.Text())
		updated.Display.MaxEvents = maxEvents.ValueAsInt()
		updated.Display.MaxLogLines = maxLogLines.ValueAsInt()
		updated.GUI.TerminalShell = strings.TrimSpace(terminalShell.Text())
		appearance := appearanceDraft()
		updated.GUI.Appearance = appearance.GUI.Appearance
		if err := updated.Validate(); err != nil {
			w.setStatus("Invalid settings: "+err.Error(), true)
			return
		}
		if err := updated.Save(); err != nil {
			w.setStatus("Save settings: "+err.Error(), true)
			return
		}
		w.applyRuntimeSettings(updated)
		cfg = updated
		if savedRaw, readErr := config.ReadRaw(); readErr == nil {
			raw = savedRaw
			advancedEditor.SetText(string(savedRaw))
		}
		if closeAfterSave {
			closing = true
			dialog.Destroy()
		}
		w.setStatus("Settings saved", false)
	})
	dialog.Present()
}

func (w *mainWindow) applyRuntimeSettings(updated config.Config) {
	if w.options.Config == nil {
		w.options.Config = &updated
	} else {
		*w.options.Config = updated
	}
	w.options.RefreshInterval = updated.Defaults.RefreshInterval
	w.options.DefaultCluster = updated.Defaults.Cluster
	w.applyAppearanceConfig(&updated)
}

func (w *mainWindow) reviewRawSettings(parent *gtk.Dialog, before, after []byte, closeParent bool, onSaved func(config.Config, []byte)) {
	parsed, err := config.Parse(after)
	if err != nil {
		w.setStatus("Invalid configuration: "+err.Error(), true)
		return
	}
	diff := configurationDiff(string(before), string(after))
	if diff == "" {
		w.setStatus("Configuration is unchanged", false)
		if closeParent {
			parent.Destroy()
			glib.IdleAdd(func() { w.applyAppearanceConfig(&parsed) })
		} else {
			w.applyAppearanceConfig(&parsed)
		}
		return
	}

	review := gtk.NewDialogWithFlags("Review configuration changes", &w.window.Window, gtk.DialogModal)
	review.AddCSSClass("e9s-settings")
	review.SetDestroyWithParent(true)
	review.SetDefaultSize(760, 480)
	content := review.ContentArea()
	content.AddCSSClass("e9s-dialog-surface")
	content.SetSpacing(8)
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	content.Append(settingsNote("Review the complete set of changed lines before writing the configuration."))
	view := gtk.NewTextView()
	view.SetEditable(false)
	view.SetCursorVisible(false)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapNone)
	view.Buffer().SetText(diff)
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(view)
	content.Append(scroll)
	review.AddButton("Cancel", int(gtk.ResponseCancel))
	review.AddButton("Apply changes", int(gtk.ResponseApply))
	review.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseApply) {
			review.Destroy()
			return
		}
		saved, err := config.SaveRaw(after)
		if err != nil {
			w.setStatus("Save configuration: "+err.Error(), true)
			return
		}
		// Use the parsed value as an additional assertion that review and save
		// operated on the same document.
		if saved.Defaults.RefreshInterval != parsed.Defaults.RefreshInterval {
			w.setStatus("Configuration changed during review; reopen settings", true)
			return
		}
		w.applyRuntimeSettings(saved)
		if onSaved != nil {
			onSaved(saved, after)
		}
		review.Destroy()
		if closeParent {
			parent.Destroy()
		}
		w.setStatus("Configuration saved; previous version retained as config.yaml.bak", false)
	})
	review.Present()
}

func configurationDiff(before, after string) string {
	if before == after {
		return ""
	}
	oldLines := strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	newLines := strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix && oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	contextStart := prefix - 2
	if contextStart < 0 {
		contextStart = 0
	}
	var output strings.Builder
	fmt.Fprintf(&output, "@@ configuration lines %d… @@\n", prefix+1)
	for _, line := range oldLines[contextStart:prefix] {
		fmt.Fprintf(&output, "  %s\n", line)
	}
	for _, line := range oldLines[prefix : len(oldLines)-suffix] {
		fmt.Fprintf(&output, "- %s\n", line)
	}
	for _, line := range newLines[prefix : len(newLines)-suffix] {
		fmt.Fprintf(&output, "+ %s\n", line)
	}
	for _, line := range newLines[len(newLines)-suffix : min(len(newLines), len(newLines)-suffix+2)] {
		fmt.Fprintf(&output, "  %s\n", line)
	}
	return output.String()
}
