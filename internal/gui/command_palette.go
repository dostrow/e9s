//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/command"
)

// paletteAccel opens the command palette. "?" (a printable shortcut) opens
// it too, replacing the old static help text.
const paletteAccel = "<Control><Shift>p"

// actionSpec describes a GIO action for the palette. Command IDs match the
// TUI's where the behavior is the same, so recency and docs line up.
type actionSpec struct {
	action   string
	id       string
	title    string
	keywords []string
	danger   bool
	// button, when set, is the toolbar button that runs the same handler;
	// its visibility and sensitivity (maintained per page and selection by
	// updateActionSensitivity) decide availability.
	button func(*mainWindow) *gtk.Button
	// where names the page that shows button, for "Available in" hints.
	where string
	// available returns "" when the action can run, else a reason. It is
	// consulted when there is no button, or the button is not on screen.
	available func(*mainWindow) string
}

func always(*mainWindow) string { return "" }

func requireLogs(w *mainWindow) string {
	if !w.showingLogs {
		return "Open a log view first"
	}
	return ""
}

func requireTaskDefinition(w *mainWindow) string {
	if w.currentPage != pageTaskDefinitions || w.selectedTaskDefinition == nil {
		return "Select a task definition"
	}
	return ""
}

func requireTerminalDock(w *mainWindow) string {
	if !w.terminalDockVisible {
		return "Open the terminal dock first"
	}
	return ""
}

var paletteActionSpecs = []actionSpec{
	{action: "refresh", id: "app.refresh", title: "Refresh", keywords: []string{"reload"}, available: always,
		button: func(w *mainWindow) *gtk.Button { return w.refreshButton }},
	{action: "search", id: "app.filter", title: "Focus filter", keywords: []string{"find", "search"}, available: always},
	{action: "back", id: "app.back", title: "Go back", keywords: []string{"close", "up"}, available: always},
	{action: "modes", id: "app.switch-mode", title: "Choose module…", keywords: []string{"module", "mode", "picker"}, available: always},
	{action: "settings", id: "app.settings", title: "Open settings", keywords: []string{"preferences", "config", "theme", "font"}, available: func(w *mainWindow) string {
		if w.settingsOpen {
			return "Settings are already open"
		}
		return ""
	}},

	{action: "logs", where: "ECS › Tasks", id: "ecs.service-logs", title: "Follow service logs", keywords: []string{"tail", "logs"},
		button: func(w *mainWindow) *gtk.Button { return w.logsButton }},
	{action: "task-logs", where: "ECS › Tasks", id: "ecs.task-logs", title: "Follow selected task logs", keywords: []string{"tail", "logs"},
		button: func(w *mainWindow) *gtk.Button { return w.taskLogsButton }},
	{action: "standalone-tasks", where: "ECS › Services and Tasks", id: "ecs.standalone-tasks", title: "Toggle standalone/service tasks", keywords: []string{"standalone"},
		button: func(w *mainWindow) *gtk.Button { return w.standaloneButton }},
	{action: "run-task", where: "ECS › Standalone tasks", id: "ecs.run-task", title: "Run standalone task", keywords: []string{"launch", "start"},
		button: func(w *mainWindow) *gtk.Button { return w.runTaskButton }},
	{action: "metrics", where: "ECS › Tasks", id: "ecs.metrics", title: "Metrics", keywords: []string{"cloudwatch", "cpu", "memory", "utilization"},
		button: func(w *mainWindow) *gtk.Button { return w.metricsButton },
		available: func(w *mainWindow) string {
			if w.currentPage == pageRDSInstances && w.selectedRDSInstance != "" {
				return ""
			}
			return "Select a service, task, or RDS instance"
		}},
	{action: "ecs-exec", where: "ECS › Tasks", id: "ecs.exec", title: "ECS Exec in embedded terminal", keywords: []string{"ssh", "shell", "exec", "connect"},
		button: func(w *mainWindow) *gtk.Button { return w.execButton }},
	{action: "scale-service", where: "ECS › Tasks", id: "ecs.scale", title: "Scale service", keywords: []string{"desired count", "replicas"},
		button: func(w *mainWindow) *gtk.Button { return w.scaleButton }},
	{action: "stop-task", where: "ECS › Tasks", id: "ecs.stop-task", title: "Stop selected task", danger: true, keywords: []string{"kill"},
		button: func(w *mainWindow) *gtk.Button { return w.stopTaskButton }},
	{action: "force-deploy", where: "ECS › Tasks", id: "ecs.force-redeploy", title: "Force new deployment", danger: true, keywords: []string{"redeploy", "restart", "rollout"},
		button: func(w *mainWindow) *gtk.Button { return w.deployButton }},
	{action: "toggle-scale-in", id: "ecs.toggle-scale-in", title: "Toggle scale-in suspension", keywords: []string{"autoscaling"}, available: func(w *mainWindow) string {
		if !w.showingMetrics || w.metricsTaskID != "" || !w.scaleInKnown || w.selectedService == "" {
			return "Open a service's metrics first"
		}
		return ""
	}},
	{action: "task-definitions", id: "ecs.task-definitions", title: "Browse task definitions", keywords: []string{"taskdef"}, available: func(w *mainWindow) string {
		if w.currentPage == pageTaskDefinitions {
			return "Already browsing task definitions"
		}
		return ""
	}},
	{action: "task-definition-env", id: "ecs.env-vars", title: "Environment variables", keywords: []string{"env", "environment", "secrets"}, available: func(w *mainWindow) string {
		taskPage := w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks || w.currentPage == pageStoppedTasks
		if (taskPage && w.selectedTask != "") || (w.currentPage == pageTaskDefinitions && w.selectedTaskDefinition != nil) {
			return ""
		}
		return "Select a task or task definition"
	}},
	{action: "task-definition-diff", id: "ecs.taskdef-diff", title: "Diff against previous revision", keywords: []string{"compare", "revision"}, available: requireTaskDefinition},
	{action: "task-definition-edit", id: "ecs.taskdef-edit", title: "Edit task-definition JSON", keywords: []string{"register", "revision"}, available: func(w *mainWindow) string {
		if w.showingEditor {
			return "An editor is already open"
		}
		return requireTaskDefinition(w)
	}},
	{action: "task-definition-register", id: "gui.save-editor", title: "Save/register active editor", keywords: []string{"save", "register", "deploy"}, available: func(w *mainWindow) string {
		if !w.showingEditor {
			return "No editor is open"
		}
		return ""
	}},

	{action: "toggle-logs", id: "logs.follow", title: "Pause/resume log follow", keywords: []string{"tail", "pause", "resume"}, available: func(w *mainWindow) string {
		if !w.showingLogs || w.logSource.Group == "" {
			return "Open a log view first"
		}
		return ""
	}},
	{action: "log-timestamps", id: "logs.timestamps", title: "Cycle log timestamps", keywords: []string{"time", "utc"}, available: requireLogs},
	{action: "copy-logs", id: "logs.copy", title: "Copy log buffer", keywords: []string{"clipboard", "yank"}, available: func(w *mainWindow) string {
		if !w.showingLogs || w.logStore == nil || w.logStore.len() == 0 {
			return "No log lines to copy"
		}
		return ""
	}},
	{action: "clear-logs", id: "logs.clear", title: "Clear log buffer", available: requireLogs},

	{action: "terminal-dock", id: "gui.terminal-dock", title: "Toggle terminal dock", keywords: []string{"shell", "console", "terminal"}, available: always},
	{action: "terminal-new-tab", id: "gui.terminal-new-tab", title: "New terminal tab", keywords: []string{"shell"}, available: func(*mainWindow) string {
		if !vteAvailable() {
			return "This build has no embedded terminal"
		}
		return ""
	}},
	{action: "terminal-close-pane", id: "gui.terminal-close-pane", title: "Close focused terminal pane", available: requireTerminalDock},
	{action: "terminal-previous-tab", id: "gui.terminal-previous-tab", title: "Previous terminal tab", available: requireTerminalTabs},
	{action: "terminal-next-tab", id: "gui.terminal-next-tab", title: "Next terminal tab", available: requireTerminalTabs},
	{action: "terminal-split-right", id: "gui.terminal-split-right", title: "Split terminal right", keywords: []string{"pane", "vertical"}, available: requireTerminalDock},
	{action: "terminal-split-down", id: "gui.terminal-split-down", title: "Split terminal down", keywords: []string{"pane", "horizontal"}, available: requireTerminalDock},
	{action: "terminal-previous-pane", id: "gui.terminal-previous-pane", title: "Focus previous terminal pane", available: requireTerminalDock},
	{action: "terminal-next-pane", id: "gui.terminal-next-pane", title: "Focus next terminal pane", available: requireTerminalDock},

	{action: "zoom-in", id: "gui.zoom-in", title: "Zoom active pane in", keywords: []string{"font", "bigger"}, available: always},
	{action: "zoom-out", id: "gui.zoom-out", title: "Zoom active pane out", keywords: []string{"font", "smaller"}, available: always},
	{action: "zoom-reset", id: "gui.zoom-reset", title: "Reset active pane zoom", keywords: []string{"font"}, available: always},
}

func requireTerminalTabs(w *mainWindow) string {
	if w.terminalDockNotebook == nil || w.terminalDockNotebook.NPages() < 2 {
		return "Open at least two terminal tabs"
	}
	return ""
}

// printableShortcut binds an unmodified (or Shift-only) key to an action
// when no text input has focus. accel is the display form for the palette.
type printableShortcut struct {
	keyval uint
	mods   gdk.ModifierType
	action string
	accel  string
}

var printableShortcuts = []printableShortcut{
	{gdk.KEY_slash, 0, "search", "slash"},
	{gdk.KEY_m, 0, "metrics", "m"},
	{gdk.KEY_e, 0, "task-definition-env", "e"},
	{gdk.KEY_d, 0, "task-definition-diff", "d"},
	{gdk.KEY_t, 0, "log-timestamps", "t"},
	{gdk.KEY_question, gdk.ShiftMask, "command-palette", "question"},
	{gdk.KEY_L, gdk.ShiftMask, "logs", "<Shift>l"},
	{gdk.KEY_S, gdk.ShiftMask, "standalone-tasks", "<Shift>s"},
	{gdk.KEY_T, gdk.ShiftMask, "task-definitions", "<Shift>t"},
}

// keyControllerAccels are handled by the window key controller rather than
// GIO accelerators (so a focused terminal can receive them first).
var keyControllerAccels = map[string]string{
	"back":      "Escape",
	"copy-logs": "<Control><Shift>c",
}

// paletteCommand is one palette row's command plus how to run it.
type paletteCommand struct {
	command.Entry
	accel string
	run   func()
}

// paletteCommands assembles every candidate for the current window state,
// highest priority first (the first registration of an ID wins).
func (w *mainWindow) paletteCommands() []paletteCommand {
	var out []paletteCommand
	represented := map[uintptr]bool{}
	category := w.paletteCategory()

	for _, spec := range paletteActionSpecs {
		spec := spec
		entry := command.Entry{
			Command: command.Command{ID: spec.id, Title: spec.title, Keywords: spec.keywords, Danger: spec.danger},
			Section: command.SectionGlobal,
		}
		reason := ""
		var button *gtk.Button
		if spec.button != nil {
			button = spec.button(w)
		}
		if button != nil {
			represented[nativeID(button)] = true
		}
		switch {
		case button != nil && button.IsDrawable():
			entry.Section = command.SectionView
			entry.Category = category
			if !button.IsSensitive() {
				reason = "Not available for the current selection"
			}
		case spec.available != nil:
			reason = spec.available(w)
			if reason == "" && spec.button != nil {
				entry.Section = command.SectionView
				entry.Category = category
			}
		case spec.where != "":
			reason = "Available in " + spec.where
		default:
			reason = "Not available on this page"
		}
		if entry.Section == command.SectionGlobal {
			entry.Category = "Global"
		}
		entry.Disabled = reason
		accel := w.actionAccel(spec.action)
		entry.Key = accelLabel(accel)
		out = append(out, paletteCommand{Entry: entry, accel: accel, run: func() { w.activateAppAction(spec.action) }})
	}
	if w.refreshPauseButton != nil {
		represented[nativeID(w.refreshPauseButton)] = true
		pause := w.refreshPauseButton
		out = append(out, paletteCommand{
			Entry: command.Entry{
				Command: command.Command{ID: "app.pause", Title: "Pause/resume automatic refresh", Category: "Global", Keywords: []string{"polling", "stop polling"}},
				Section: command.SectionGlobal,
			},
			run: func() { pause.SetActive(!pause.Active()) },
		})
	}
	if w.backButton != nil {
		represented[nativeID(w.backButton)] = true
	}

	out = append(out, w.harvestToolbarCommands(category, represented)...)
	out = append(out, w.railCommands()...)
	return out
}

// harvestToolbarCommands turns every labeled button on screen outside the
// module rail into a "This view" command. Module toolbars already show and
// enable their buttons per page and selection, so this covers each module
// without duplicating its availability rules.
func (w *mainWindow) harvestToolbarCommands(category string, represented map[uintptr]bool) []paletteCommand {
	root := w.window.Child()
	if root == nil {
		return nil
	}
	var out []paletteCommand
	seen := map[string]bool{}
	module := moduleForPage(w.currentPage)
	walkWidgets(root, func(widget gtk.Widgetter) bool {
		base := gtk.BaseWidget(widget)
		if w.moduleRail != nil && nativeID(base) == nativeID(w.moduleRail) {
			return false // the rail is harvested separately
		}
		if !base.IsDrawable() {
			return false
		}
		button := asButton(widget)
		if button == nil {
			return true
		}
		if represented[nativeID(base)] || base.HasCSSClass("palette-skip") {
			return false
		}
		label := buttonText(widget)
		if label == "" {
			return false
		}
		title := strings.TrimSpace(strings.TrimSuffix(label, "…"))
		id := "gui." + slug(module) + "." + slug(title)
		if seen[id] {
			return false
		}
		seen[id] = true
		entry := command.Entry{
			Command: command.Command{
				ID:       id,
				Title:    title,
				Category: category,
				Danger:   base.HasCSSClass("destructive-action") || dangerousLabel(title),
			},
			Section: command.SectionView,
		}
		if tip := strings.TrimSpace(base.TooltipText()); tip != "" {
			entry.Keywords = []string{tip}
		}
		if !base.IsSensitive() {
			entry.Disabled = "Not available for the current selection"
		}
		out = append(out, paletteCommand{Entry: entry, run: func() {
			if base.IsSensitive() && base.IsDrawable() {
				base.Activate()
			}
		}})
		return false
	})
	return out
}

// railCommands lists module entries, their sub-views, saved items, and
// plugin actions from the module rail.
func (w *mainWindow) railCommands() []paletteCommand {
	var out []paletteCommand
	for _, section := range w.moduleSections {
		section := section
		plugin := strings.HasPrefix(section.key, "plugin:")
		if !plugin {
			out = append(out, paletteCommand{
				Entry: command.Entry{
					Command: command.Command{
						ID:       "mode." + slug(section.name),
						Title:    "Go to " + section.name,
						Category: "Modules",
						Keywords: append([]string{"module", "mode", "switch"}, section.aliases...),
					},
					Section: command.SectionGoTo,
					Target:  w.currentModuleMarker(section.key),
				},
				run: func() { w.activateModule(section.key) },
			})
		}
		if section.expander == nil || section.expander.Child() == nil {
			continue
		}
		savedGroup := ""
		walkWidgets(section.expander.Child(), func(widget gtk.Widgetter) bool {
			base := gtk.BaseWidget(widget)
			if label, ok := widget.(*gtk.Label); ok {
				if base.HasCSSClass("module-saved-section") {
					savedGroup = sentenceCase(label.Label())
				}
				return false
			}
			if asButton(widget) == nil {
				return true
			}
			if !base.Visible() {
				return false
			}
			title := buttonText(widget)
			if title == "" || (!plugin && savedGroup == "" && title == section.defaultItem) {
				return false
			}
			entry := command.Entry{Command: command.Command{Title: title}}
			switch {
			case plugin:
				entry.ID = "plugin." + slug(section.key) + "." + slug(title)
				entry.Category = section.name
				entry.Section = command.SectionPlugins
				entry.Keywords = append([]string{"plugin", "runbook"}, section.aliases...)
				if tip := base.TooltipText(); tip != "" {
					entry.Keywords = append(entry.Keywords, tip)
				}
				if !base.IsSensitive() {
					entry.Disabled = "A plugin action is already running"
				}
			case savedGroup != "":
				entry.ID = "saved." + slug(section.key) + "." + slug(savedGroup) + "." + slug(title)
				entry.Category = section.name + " · " + savedGroup
				entry.Section = command.SectionSaved
				entry.Keywords = append([]string{"saved", "bookmark"}, section.aliases...)
			default:
				entry.ID = "goto." + slug(section.key) + "." + slug(title)
				entry.Title = section.name + " › " + title
				entry.Category = "Modules"
				entry.Section = command.SectionGoTo
				entry.Keywords = section.aliases
			}
			out = append(out, paletteCommand{Entry: entry, run: func() { base.Activate() }})
			return false
		})
	}
	return out
}

func (w *mainWindow) currentModuleMarker(key string) string {
	if moduleForPage(w.currentPage) == key {
		return "current"
	}
	return ""
}

func (w *mainWindow) paletteCategory() string {
	module := moduleForPage(w.currentPage)
	for _, section := range w.moduleSections {
		if section.key == module {
			return section.name
		}
	}
	if module == "" {
		return ""
	}
	return sentenceCase(module)
}

// walkWidgets visits widget and its descendants depth-first. visit returns
// whether to descend into the widget's children.
func walkWidgets(widget gtk.Widgetter, visit func(gtk.Widgetter) bool) {
	if widget == nil {
		return
	}
	if !visit(widget) {
		return
	}
	for child := gtk.BaseWidget(widget).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		walkWidgets(child, visit)
	}
}

// nativeID identifies a widget's underlying GObject, since gotk4 may hand out
// distinct Go wrappers for the same object.
func nativeID(widget glib.Objector) uintptr {
	return glib.BaseObject(widget).Native()
}

func asButton(widget gtk.Widgetter) *gtk.Button {
	switch b := widget.(type) {
	case *gtk.Button:
		return b
	case *gtk.ToggleButton:
		return &b.Button
	}
	return nil
}

// buttonText returns a button's label, or the text of its Label child.
func buttonText(widget gtk.Widgetter) string {
	button := asButton(widget)
	if button == nil {
		return ""
	}
	if text := strings.TrimSpace(button.Label()); text != "" {
		return text
	}
	if label, ok := button.Child().(*gtk.Label); ok {
		return strings.TrimSpace(label.Label())
	}
	return ""
}

func dangerousLabel(title string) bool {
	lower := strings.ToLower(title)
	for _, prefix := range []string{"delete", "terminate", "stop", "reboot", "force", "apply", "acknowledge", "remove"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func sentenceCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// actionAccel returns the accelerator string that triggers action, from GIO
// accelerators, the window key controller, or the printable shortcuts.
func (w *mainWindow) actionAccel(action string) string {
	if w.application != nil {
		if accels := w.application.AccelsForAction("app." + action); len(accels) > 0 {
			return accels[0]
		}
	}
	if accel, ok := keyControllerAccels[action]; ok {
		return accel
	}
	for _, shortcut := range printableShortcuts {
		if shortcut.action == action {
			return shortcut.accel
		}
	}
	return ""
}

// accelLabel renders an accelerator for text ("Ctrl+Shift+R").
func accelLabel(accel string) string {
	if accel == "" {
		return ""
	}
	key, mods, ok := gtk.AcceleratorParse(accel)
	if !ok {
		return accel
	}
	return gtk.AcceleratorGetLabel(key, mods)
}

func (w *mainWindow) activateAppAction(action string) {
	if w.application != nil {
		w.application.ActivateAction(action, nil)
	}
}

// --- Palette window ---

type commandPalette struct {
	w        *mainWindow
	window   *gtk.Window
	entry    *gtk.SearchEntry
	list     *gtk.ListBox
	scroll   *gtk.ScrolledWindow
	footer   *gtk.Label
	commands map[string]paletteCommand
	entries  []command.Entry
	rows     []*gtk.ListBoxRow
	results  []command.Result
}

func (w *mainWindow) ensurePaletteRecency() {
	if w.paletteRecency == nil {
		w.paletteRecencyPath = command.DefaultRecencyPath("gui")
		w.paletteRecency = command.LoadRecency(w.paletteRecencyPath)
	}
}

// showCommandPalette opens the palette over the main window.
func (w *mainWindow) showCommandPalette() {
	if w.palette != nil {
		w.palette.window.Present()
		return
	}
	w.ensurePaletteRecency()
	p := &commandPalette{w: w, commands: map[string]paletteCommand{}}
	for _, c := range w.paletteCommands() {
		if c.ID == "" {
			continue
		}
		if _, dup := p.commands[c.ID]; dup {
			continue
		}
		p.commands[c.ID] = c
		p.entries = append(p.entries, c.Entry)
	}

	p.window = gtk.NewWindow()
	p.window.SetTitle("Command palette")
	p.window.SetTransientFor(&w.window.Window)
	p.window.SetModal(true)
	p.window.SetDestroyWithParent(true)
	p.window.SetDecorated(false)
	p.window.SetResizable(false)
	p.window.SetDefaultSize(760, 520)
	p.window.AddCSSClass("command-palette")

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(12)
	box.SetMarginBottom(12)
	box.SetMarginStart(12)
	box.SetMarginEnd(12)

	p.entry = gtk.NewSearchEntry()
	p.entry.SetPlaceholderText("Type to search commands, modules, and saved items…")
	p.entry.SetHExpand(true)
	p.entry.ConnectSearchChanged(p.refilter)
	p.entry.ConnectStopSearch(p.close)
	p.entry.ConnectActivate(p.runSelected)

	p.list = gtk.NewListBox()
	p.list.SetSelectionMode(gtk.SelectionBrowse)
	p.list.SetActivateOnSingleClick(true)
	p.list.AddCSSClass("command-palette-list")
	p.list.ConnectRowActivated(func(row *gtk.ListBoxRow) { p.runRow(row) })

	p.scroll = gtk.NewScrolledWindow()
	p.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	p.scroll.SetVExpand(true)
	p.scroll.SetChild(p.list)

	p.footer = gtk.NewLabel("")
	p.footer.SetXAlign(0)
	p.footer.SetWrap(true)
	p.footer.AddCSSClass("muted")

	box.Append(p.entry)
	box.Append(p.scroll)
	box.Append(p.footer)
	p.window.SetChild(box)

	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		ctrl := state&gdk.ControlMask != 0
		switch {
		case keyval == gdk.KEY_Escape:
			p.close()
		case keyval == gdk.KEY_Down || keyval == gdk.KEY_Tab || (ctrl && (keyval == gdk.KEY_n || keyval == gdk.KEY_j)):
			p.move(1)
		case keyval == gdk.KEY_Up || keyval == gdk.KEY_ISO_Left_Tab || (ctrl && (keyval == gdk.KEY_p || keyval == gdk.KEY_k)):
			p.move(-1)
		case keyval == gdk.KEY_Page_Down:
			p.move(8)
		case keyval == gdk.KEY_Page_Up:
			p.move(-8)
		case keyval == gdk.KEY_Return || keyval == gdk.KEY_KP_Enter:
			p.runSelected()
		default:
			return false
		}
		return true
	})
	p.window.AddController(keys)
	p.window.ConnectCloseRequest(func() bool {
		w.palette = nil
		return false
	})

	w.palette = p
	p.refilter()
	p.window.Present()
	p.entry.GrabFocus()
}

func (p *commandPalette) close() {
	if p.w.palette == p {
		p.w.palette = nil
	}
	p.window.Destroy()
}

func (p *commandPalette) refilter() {
	query := p.entry.Text()
	p.results = command.Filter(p.entries, query, p.w.paletteRecency)
	for child := p.list.FirstChild(); child != nil; child = p.list.FirstChild() {
		p.list.Remove(child)
	}
	p.rows = p.rows[:0]

	browsing := strings.TrimSpace(query) == ""
	section := command.Section(-1)
	var first *gtk.ListBoxRow
	for i, r := range p.results {
		if (browsing || r.Section == command.SectionUnavailable) && r.Section != section {
			section = r.Section
			p.list.Append(paletteHeaderRow(section.Title(), i > 0))
		}
		row := p.newRow(r)
		p.list.Append(row)
		p.rows = append(p.rows, row)
		if first == nil && r.Available() {
			first = row
		}
	}
	if len(p.results) == 0 {
		p.list.Append(paletteHeaderRow("No matching commands", false))
	}
	if first != nil {
		p.list.SelectRow(first)
	} else {
		p.list.UnselectAll()
	}
	p.scroll.VAdjustment().SetValue(0)
	if browsing {
		p.footer.SetLabel("Enter opens the selected row · Esc goes back · / focuses the filter · ] or PgDn loads more DynamoDB items\n↑/↓ move · Enter run · Esc close")
	} else {
		p.footer.SetLabel("↑/↓ move · Enter run · Esc close")
	}
}

func paletteHeaderRow(text string, spaced bool) *gtk.ListBoxRow {
	label := gtk.NewLabel(strings.ToUpper(text))
	label.SetXAlign(0)
	label.AddCSSClass("section-title")
	if spaced {
		label.SetMarginTop(8)
	}
	row := gtk.NewListBoxRow()
	row.SetChild(label)
	row.SetSelectable(false)
	row.SetActivatable(false)
	row.SetCanFocus(false)
	return row
}

func (p *commandPalette) newRow(r command.Result) *gtk.ListBoxRow {
	box := gtk.NewBox(gtk.OrientationHorizontal, 10)
	box.SetMarginTop(3)
	box.SetMarginBottom(3)
	box.SetMarginStart(6)
	box.SetMarginEnd(6)

	title := gtk.NewLabel("")
	title.SetMarkup(highlightMarkup(r.Title, r.Positions))
	title.SetXAlign(0)
	title.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	if r.Danger && r.Available() {
		title.AddCSSClass("error")
	}
	box.Append(title)
	if r.Target != "" {
		target := gtk.NewLabel("· " + r.Target)
		target.AddCSSClass("muted")
		box.Append(target)
	}
	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	box.Append(spacer)

	detail := r.Category
	if !r.Available() {
		detail = r.Disabled
	}
	if detail != "" {
		label := gtk.NewLabel(detail)
		label.AddCSSClass("muted")
		label.SetEllipsize(3)
		label.SetMaxWidthChars(36)
		box.Append(label)
	}
	if c, ok := p.commands[r.ID]; ok && c.accel != "" && r.Available() {
		box.Append(gtk.NewShortcutLabel(c.accel))
	}
	if r.Danger {
		marker := gtk.NewLabel("⚠")
		marker.AddCSSClass("error")
		marker.SetTooltipText("Destructive: asks for confirmation before running")
		box.Append(marker)
	}

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	row.SetName(r.ID)
	if !r.Available() {
		row.SetSensitive(false)
		row.SetSelectable(false)
		row.SetActivatable(false)
	}
	return row
}

// highlightMarkup returns Pango markup for title with the runes at
// positions emphasized.
func highlightMarkup(title string, positions []int) string {
	marked := make(map[int]bool, len(positions))
	for _, p := range positions {
		marked[p] = true
	}
	var b strings.Builder
	open := false
	for i, r := range []rune(title) {
		if marked[i] && !open {
			b.WriteString(`<span weight="bold" underline="single">`)
			open = true
		} else if !marked[i] && open {
			b.WriteString("</span>")
			open = false
		}
		b.WriteString(html.EscapeString(string(r)))
	}
	if open {
		b.WriteString("</span>")
	}
	return b.String()
}

func (p *commandPalette) move(delta int) {
	if len(p.rows) == 0 {
		return
	}
	current := -1
	if selected := p.list.SelectedRow(); selected != nil {
		for i, row := range p.rows {
			if nativeID(row) == nativeID(selected) {
				current = i
				break
			}
		}
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	target := max(0, min(len(p.rows)-1, current+delta))
	for i := target; i >= 0 && i < len(p.rows); i += step {
		if p.rows[i].Selectable() {
			p.selectRow(p.rows[i])
			return
		}
	}
	for i := target; i >= 0 && i < len(p.rows); i -= step {
		if p.rows[i].Selectable() {
			p.selectRow(p.rows[i])
			return
		}
	}
}

func (p *commandPalette) selectRow(row *gtk.ListBoxRow) {
	p.list.SelectRow(row)
	bounds, ok := row.ComputeBounds(p.list)
	if !ok {
		return
	}
	adj := p.scroll.VAdjustment()
	top, bottom := float64(bounds.Y()), float64(bounds.Y()+bounds.Height())
	if top < adj.Value() {
		adj.SetValue(top)
	} else if bottom > adj.Value()+adj.PageSize() {
		adj.SetValue(bottom - adj.PageSize())
	}
}

func (p *commandPalette) runSelected() {
	if row := p.list.SelectedRow(); row != nil {
		p.runRow(row)
	}
}

func (p *commandPalette) runRow(row *gtk.ListBoxRow) {
	c, ok := p.commands[row.Name()]
	if !ok || !row.Selectable() {
		return
	}
	w := p.w
	p.close()
	// Run after the palette is gone so dialogs the command opens attach to
	// the main window and receive focus.
	glib.IdleAdd(func() { w.runPaletteCommand(c) })
}

// runPaletteCommand runs a command, records it for recency, and teaches its
// shortcut when the command did not post a status message of its own.
func (w *mainWindow) runPaletteCommand(c paletteCommand) {
	w.ensurePaletteRecency()
	w.paletteRecency.Record(c.ID)
	recency, path := w.paletteRecency, w.paletteRecencyPath
	go func() { _ = recency.Save(path) }()

	before := w.status.Label()
	c.run()
	if c.Key != "" && w.status.Label() == before {
		w.setStatus(fmt.Sprintf("Tip: press %s to run “%s” directly", c.Key, c.Title), false)
	}
}
