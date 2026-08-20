//go:build gui

package gui

import (
	"fmt"
	"strings"

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
	{label: "API Gateway", value: "API Gateway"},
	{label: "CloudWatch Alarms", value: "CloudWatch Alarms"},
	{label: "CloudWatch Logs", value: "CloudWatch Logs"},
	{label: "CodeBuild", value: "CodeBuild"},
	{label: "Cost Explorer", value: "Cost Explorer"},
	{label: "ElastiCache", value: "ElastiCache"},
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
	{label: "SQL Workbench", value: "SQL Workbench"},
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
	if w.settingsOpen {
		return
	}
	if w.settingsNavButton != nil && !w.settingsNavButton.Active() {
		w.settingsNavButton.SetActive(true)
	}

	cfg := config.DefaultConfig()
	if w.options.Config != nil {
		cfg = *w.options.Config
	}
	raw, err := config.ReadRaw()
	if err != nil {
		raw, _ = yaml.Marshal(&cfg)
	}

	for child := w.settingsHost.FirstChild(); child != nil; child = w.settingsHost.FirstChild() {
		w.settingsHost.Remove(child)
	}
	w.settingsOpen = true
	w.settingsReturnBreadcrumb = w.breadcrumbText
	w.mainContentStack.SetVisibleChildName(pageSettings)
	w.setBreadcrumb("Settings")
	w.backButton.SetSensitive(true)

	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("e9s-settings-page")
	page.SetHExpand(true)
	page.SetVExpand(true)
	notebook := gtk.NewNotebook()
	notebook.AddCSSClass("e9s-settings-notebook")
	notebook.SetHExpand(true)
	notebook.SetVExpand(true)
	page.Append(notebook)

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
	interfaceSystem := newApplicationCheckButton("Use GTK interface font")
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
	monospaceSystem := newApplicationCheckButton("Use GTK monospace font")
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
	costGuard := gtk.NewSpinButtonWithRange(0, 1000, 0.10)
	costGuard.SetDigits(2)
	costGuard.SetValue(cfg.Defaults.CostGuardUSD)
	pgpassFiles := gtk.NewEntry()
	pgpassFiles.SetText(strings.Join(cfg.SQL.PGPassFiles, ", "))
	pgpassFiles.SetPlaceholderText("~/.pgpass, ~/.config/e9s/production.pgpass")
	awsPage.Append(settingsRow("AWS profile", profile))
	awsPage.Append(settingsRow("AWS region", region))
	awsPage.Append(settingsRow("Default ECS cluster", cluster))
	awsPage.Append(settingsRow("Save directory", saveDirectory))
	awsPage.Append(settingsRow("Pause at known session cost (USD; 0 disables)", costGuard))
	awsPage.Append(settingsRow("PostgreSQL password files", pgpassFiles))
	awsPage.Append(settingsNote("Password files are tried from left to right after any per-connection pgpass_file. PostgreSQL requires private file permissions (0600). Passwords are never copied into e9s configuration or session state."))
	awsPage.Append(settingsNote("Profile and region changes are saved immediately and take effect the next time e9s starts. Existing AWS clients are never replaced mid-request."))
	awsPage.Append(settingsNote("Cost Explorer results are cached in memory for 24 hours or until e9s exits. Only the explicitly confirmed Force paid refresh action bypasses that cache."))
	if w.options.RequestSnapshot != nil {
		snapshot := w.options.RequestSnapshot()
		awsPage.Append(settingsNote(fmt.Sprintf("This session: %d AWS API operations observed; at least $%.4f in directly attributable request charges. S3, SQS, DynamoDB capacity, data transfer, and free-tier effects are not estimated.", snapshot.Total, snapshot.EstimatedCostUSD)))
	}
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
	confirmActions := newApplicationCheckButton("Require confirmation for destructive actions")
	confirmActions.SetActive(true)
	confirmActions.SetSensitive(false)
	allowSQLWrites := newApplicationCheckButton("Allow SQL write break-glass controls")
	allowSQLWrites.SetActive(cfg.SQL.AllowWrites)
	safetyPage.Append(confirmActions)
	safetyPage.Append(settingsNote("Safety confirmations cannot currently be disabled. Break-glass controls for infrastructure-as-code-managed resources are planned separately."))
	safetyPage.Append(allowSQLWrites)
	safetyPage.Append(settingsNote("SQL tabs remain read-only unless this global setting and the tab's separately confirmed Writes control are both enabled. Write authorization is never restored when e9s restarts."))
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

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	actions.AddCSSClass("settings-actions")
	spacer := gtk.NewLabel("")
	spacer.SetHExpand(true)
	actions.Append(spacer)
	revertButton := gtk.NewButtonWithLabel("Revert")
	applyButton := gtk.NewButtonWithLabel("Apply")
	doneButton := gtk.NewButtonWithLabel("Done")
	actions.Append(revertButton)
	actions.Append(applyButton)
	actions.Append(doneButton)
	page.Append(actions)
	w.settingsHost.Append(page)

	w.settingsDiscard = func() {
		restored := cfg
		w.applyAppearanceConfig(&restored)
	}
	saveSettings := func(closeAfterSave bool) {
		if advancedToggle.Active() {
			w.reviewRawSettings(raw, []byte(advancedEditor.Text()), func(saved config.Config, savedRaw []byte) {
				cfg = saved
				raw = append(raw[:0], savedRaw...)
				advancedEditor.SetText(string(savedRaw))
				if closeAfterSave {
					w.closeSettings(false)
				}
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
		updated.Defaults.CostGuardUSD = costGuard.Value()
		updated.SQL.PGPassFiles = splitSettingsPaths(pgpassFiles.Text())
		updated.SQL.AllowWrites = allowSQLWrites.Active()
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
			w.closeSettings(false)
		}
		w.setStatus("Settings saved", false)
	}
	revertButton.ConnectClicked(func() {
		w.closeSettings(true)
		w.showSettings()
	})
	applyButton.ConnectClicked(func() { saveSettings(false) })
	doneButton.ConnectClicked(func() { saveSettings(true) })
}

func (w *mainWindow) closeSettings(restorePreview bool) {
	if !w.settingsOpen {
		return
	}
	if restorePreview && w.settingsDiscard != nil {
		w.settingsDiscard()
	}
	w.settingsDiscard = nil
	w.settingsOpen = false
	w.mainContentStack.SetVisibleChildName("content")
	w.setBreadcrumb(w.settingsReturnBreadcrumb)
	if w.settingsNavButton != nil && w.settingsNavButton.Active() {
		w.settingsNavButton.SetActive(false)
	}
	w.updateActionSensitivity()
}

func (w *mainWindow) applyRuntimeSettings(updated config.Config) {
	if w.options.Config == nil {
		w.options.Config = &updated
	} else {
		*w.options.Config = updated
	}
	w.options.RefreshInterval = updated.Defaults.RefreshInterval
	w.options.DefaultCluster = updated.Defaults.Cluster
	w.idleTimeoutSeconds.Store(int64(updated.Defaults.IdleTimeout))
	w.applyAppearanceConfig(&updated)
	if w.sqlExecutor != nil {
		w.sqlExecutor.SetPolicy(updated.SQL.AllowWrites, updated.SQL.PGPassFiles)
	}
}

func splitSettingsPaths(value string) []string {
	var paths []string
	for _, path := range strings.Split(value, ",") {
		if path = strings.TrimSpace(path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func (w *mainWindow) reviewRawSettings(before, after []byte, onSaved func(config.Config, []byte)) {
	parsed, err := config.Parse(after)
	if err != nil {
		w.setStatus("Invalid configuration: "+err.Error(), true)
		return
	}
	diff := configurationDiff(string(before), string(after))
	if diff == "" {
		w.setStatus("Configuration is unchanged", false)
		w.applyAppearanceConfig(&parsed)
		if onSaved != nil {
			onSaved(parsed, after)
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
