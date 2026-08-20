//go:build gui

package gui

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

const (
	terminalDockPreferredHeight = 280
	terminalTabMinimumChars     = 8
	terminalTabMaximumChars     = 28
)

type terminalDockSession struct {
	id          int
	terminal    *vteTerminal
	cwd         string
	autoTitle   string
	customTitle string
	tab         *terminalDockTab
	node        *terminalDockNode
}

type terminalDockTab struct {
	id          int
	container   *gtk.Box
	root        *terminalDockNode
	tabLabel    *gtk.Label
	customTitle string
	active      *terminalDockSession
}

type terminalDockNode struct {
	parent  *terminalDockNode
	widget  gtk.Widgetter
	session *terminalDockSession
}

func (w *mainWindow) buildTerminalDock() *gtk.Box {
	hideButton := gtk.NewButtonWithLabel("Hide")
	hideButton.ConnectClicked(func() { w.terminalDockButton.SetActive(false) })
	newButton := gtk.NewButtonWithLabel("New tab")
	newButton.SetTooltipText("Open another local terminal (Ctrl+Shift+T)")
	newButton.ConnectClicked(func() { w.newTerminalDockTab() })
	renameButton := gtk.NewButtonWithLabel("Rename tab…")
	renameButton.SetTooltipText("Rename the active terminal tab")
	renameButton.ConnectClicked(w.promptRenameActiveTerminalDock)
	closeButton := gtk.NewButtonWithLabel("Close tab…")
	closeButton.SetTooltipText("Close the active terminal (Ctrl+Shift+W)")
	closeButton.ConnectClicked(w.confirmCloseActiveTerminalDock)
	restartButton := gtk.NewButtonWithLabel("Restart shell…")
	restartButton.ConnectClicked(w.confirmRestartTerminalDock)
	w.terminalDockTitle = gtk.NewLabel("Local terminals")
	w.terminalDockTitle.SetXAlign(0)
	w.terminalDockTitle.SetHExpand(true)
	w.terminalDockTitle.SetEllipsize(pango.EllipsizeMiddle)
	w.terminalDockTitle.AddCSSClass("breadcrumb")

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(hideButton)
	toolbar.Append(w.terminalDockTitle)
	toolbar.Append(newButton)
	toolbar.Append(renameButton)
	toolbar.Append(closeButton)
	toolbar.Append(restartButton)

	w.terminalDockNotebook = gtk.NewNotebook()
	w.terminalDockNotebook.AddCSSClass("e9s-terminal-notebook")
	w.terminalDockNotebook.SetHExpand(true)
	w.terminalDockNotebook.SetVExpand(true)
	w.terminalDockNotebook.SetScrollable(true)
	w.terminalDockNotebook.ConnectSwitchPage(func(_ gtk.Widgetter, _ uint) {
		w.updateTerminalDockTitle()
	})

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.AddCSSClass("terminal-dock")
	pane.SetSizeRequest(-1, 160)
	pane.Append(toolbar)
	pane.Append(w.terminalDockNotebook)
	return pane
}

func (w *mainWindow) toggleTerminalDock() {
	if w.terminalDockButton == nil {
		return
	}
	w.terminalDockButton.SetActive(!w.terminalDockButton.Active())
}

func (w *mainWindow) setTerminalDockVisible(visible bool) {
	if visible && !vteAvailable() {
		w.terminalDockButton.SetActive(false)
		w.setStatus("Local terminal unavailable: rebuild e9s-gui with the gui and vte tags", true)
		return
	}
	if w.terminalDockContainer == nil {
		return
	}
	w.terminalDockVisible = visible
	w.terminalDockContainer.SetVisible(visible)
	if !visible {
		w.setStatus("Local terminal dock hidden; its shells remain running", false)
		return
	}
	if len(w.terminalDockSessions) == 0 {
		if _, err := w.spawnTerminalDock(); err != nil {
			w.setStatus(err.Error(), true)
			return
		}
	}
	glib.IdleAdd(func() {
		if !w.terminalDockPositioned && w.terminalDockSplit != nil {
			height := w.terminalDockSplit.Height()
			position := height - terminalDockPreferredHeight
			if position < terminalDockPreferredHeight {
				position = terminalDockPreferredHeight
			}
			w.terminalDockSplit.SetPosition(position)
			w.terminalDockPositioned = true
		}
		if session := w.activeTerminalDockSession(); session != nil {
			session.terminal.GrabFocus()
		}
	})
}

func (w *mainWindow) newTerminalDockTab() {
	session, err := w.spawnTerminalDock()
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.terminalDockNotebook.SetCurrentPage(w.indexOfTerminalDockSession(session))
	glib.IdleAdd(session.terminal.GrabFocus)
}

func (w *mainWindow) openNewTerminalDockTab() {
	if !w.terminalDockVisible {
		hadSessions := len(w.terminalDockSessions) > 0
		w.terminalDockButton.SetActive(true)
		if !hadSessions {
			return
		}
	}
	w.newTerminalDockTab()
}

func (w *mainWindow) spawnTerminalDock() (*terminalDockSession, error) {
	workingDirectory := w.terminalDockWorkingDirectory()
	session, err := w.spawnTerminalDockSession(workingDirectory)
	if err != nil {
		return nil, err
	}
	node := &terminalDockNode{widget: session.terminal.Widget(), session: session}
	container := gtk.NewBox(gtk.OrientationVertical, 0)
	container.SetHExpand(true)
	container.SetVExpand(true)
	container.Append(node.widget)
	tab := &terminalDockTab{
		id:        session.id,
		container: container,
		root:      node,
		tabLabel:  gtk.NewLabel(""),
		active:    session,
	}
	session.tab = tab
	session.node = node
	tab.tabLabel.SetEllipsize(pango.EllipsizeMiddle)
	tab.tabLabel.SetMaxWidthChars(terminalTabMaximumChars)
	w.terminalDockTabs = append(w.terminalDockTabs, tab)
	w.terminalDockSessions = append(w.terminalDockSessions, session)
	w.updateTerminalDockSessionTitle(session)
	label := w.terminalDockTabLabel(tab)
	page := w.terminalDockNotebook.AppendPage(tab.container, label)
	w.terminalDockNotebook.SetCurrentPage(page)
	w.updateTerminalDockTitle()
	w.setStatus("Started local terminal in "+workingDirectory, false)
	return session, nil
}

func (w *mainWindow) spawnTerminalDockSession(workingDirectory string) (*terminalDockSession, error) {
	configuredShell := os.Getenv("SHELL")
	if w.options.Config != nil && strings.TrimSpace(w.options.Config.GUI.TerminalShell) != "" {
		configuredShell = w.options.Config.GUI.TerminalShell
	}
	shell, err := resolveTerminalShell(configuredShell)
	if err != nil {
		return nil, err
	}
	terminal := newVTETerminal()
	terminal.SetFontScale(float64(w.terminalZoom) / 100)
	if w.options.Config != nil {
		terminal.SetFont(strings.TrimSpace(w.options.Config.GUI.Appearance.MonospaceFont))
	}
	terminal.SetPalette(w.currentSemanticPalette(w.window.StyleContext()))
	if err := terminal.SpawnInDirectory(shell, nil, workingDirectory); err != nil {
		return nil, fmt.Errorf("start local terminal: %w", err)
	}
	w.terminalDockNextID++
	session := &terminalDockSession{
		id:        w.terminalDockNextID,
		terminal:  terminal,
		cwd:       workingDirectory,
		autoTitle: terminal.WindowTitle(),
	}
	terminal.ConnectWindowTitleChanged(func() {
		session.autoTitle = terminal.WindowTitle()
		w.updateTerminalDockSessionTitle(session)
	})
	return session, nil
}

func (w *mainWindow) terminalDockTabLabel(tab *terminalDockTab) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.Append(tab.tabLabel)
	closeButton := gtk.NewButtonFromIconName("window-close-symbolic")
	closeButton.AddCSSClass("flat")
	closeButton.AddCSSClass("terminal-tab-close")
	closeButton.SetTooltipText("Close this terminal")
	closeButton.ConnectClicked(func() { w.confirmCloseTerminalDock(tab.active) })
	box.Append(closeButton)
	return box
}

func (w *mainWindow) activeTerminalDockSession() *terminalDockSession {
	tab := w.activeTerminalDockTab()
	if tab == nil {
		return nil
	}
	return tab.active
}

func (w *mainWindow) activeTerminalDockTab() *terminalDockTab {
	if w.terminalDockNotebook == nil {
		return nil
	}
	page := w.terminalDockNotebook.CurrentPage()
	if page < 0 || page >= len(w.terminalDockTabs) {
		return nil
	}
	return w.terminalDockTabs[page]
}

func (w *mainWindow) indexOfTerminalDockSession(session *terminalDockSession) int {
	for index, candidate := range w.terminalDockSessions {
		if candidate == session {
			return index
		}
	}
	return -1
}

func (w *mainWindow) indexOfTerminalDockTab(tab *terminalDockTab) int {
	for index, candidate := range w.terminalDockTabs {
		if candidate == tab {
			return index
		}
	}
	return -1
}

func (w *mainWindow) updateTerminalDockTitle() {
	if w.terminalDockTitle == nil {
		return
	}
	session := w.activeTerminalDockSession()
	if session == nil {
		w.terminalDockTitle.SetLabel("Local terminals")
		w.terminalDockTitle.SetTooltipText("")
		return
	}
	w.terminalDockTitle.SetLabel(fmt.Sprintf("%s — %s", terminalDockTabTitle(session.tab), session.cwd))
	w.terminalDockTitle.SetTooltipText(session.cwd)
}

func terminalDockSessionTitle(session *terminalDockSession) string {
	if session == nil {
		return "Local terminal"
	}
	if title := strings.TrimSpace(session.customTitle); title != "" {
		return title
	}
	if title := strings.TrimSpace(session.autoTitle); title != "" {
		return title
	}
	return fmt.Sprintf("Terminal %d", session.id)
}

func terminalDockTabTitle(tab *terminalDockTab) string {
	if tab == nil {
		return "Local terminal"
	}
	if title := strings.TrimSpace(tab.customTitle); title != "" {
		return title
	}
	return terminalDockSessionTitle(tab.active)
}

func (w *mainWindow) updateTerminalDockSessionTitle(session *terminalDockSession) {
	if session == nil || session.tab == nil || session.tab.tabLabel == nil || session.tab.active != session {
		return
	}
	title := terminalDockTabTitle(session.tab)
	session.tab.tabLabel.SetLabel(title)
	session.tab.tabLabel.SetWidthChars(terminalDockTabWidth(title))
	tooltip := title
	if session.cwd != "" {
		tooltip += "\n" + session.cwd
	}
	session.tab.tabLabel.SetTooltipText(tooltip)
	if w.activeTerminalDockSession() == session {
		w.updateTerminalDockTitle()
	}
}

func terminalDockTabWidth(title string) int {
	width := utf8.RuneCountInString(title)
	if width < terminalTabMinimumChars {
		return terminalTabMinimumChars
	}
	if width > terminalTabMaximumChars {
		return terminalTabMaximumChars
	}
	return width
}

func (w *mainWindow) promptRenameActiveTerminalDock() {
	session := w.activeTerminalDockSession()
	if session == nil {
		return
	}
	tab := session.tab
	dialog := gtk.NewDialogWithFlags("Rename terminal tab", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Tab title (leave blank to follow the terminal title)")
	label.SetXAlign(0)
	entry := gtk.NewEntry()
	entry.SetText(tab.customTitle)
	entry.SetPlaceholderText(terminalDockSessionTitle(&terminalDockSession{id: session.id, autoTitle: session.autoTitle}))
	entry.SetActivatesDefault(true)
	content.Append(label)
	content.Append(entry)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Rename", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response == int(gtk.ResponseOK) {
			tab.customTitle = strings.TrimSpace(entry.Text())
			w.updateTerminalDockSessionTitle(session)
		}
		dialog.Destroy()
	})
	dialog.Present()
}

func (w *mainWindow) confirmCloseActiveTerminalDock() {
	w.confirmCloseTerminalDock(w.activeTerminalDockSession())
}

func (w *mainWindow) confirmCloseTerminalDock(session *terminalDockSession) {
	if session == nil {
		return
	}
	if !session.terminal.Running() {
		w.closeTerminalDockSession(session)
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Close local terminal")
	dialog.SetMarkup("Stop the shell running in <b>" + html.EscapeString(valueOrDash(session.cwd)) + "</b> and close its tab?")
	dialog.SetObjectProperty("secondary-text", "Any foreground command running in this terminal will also be stopped.")
	dialog.SetDefaultResponse(int(gtk.ResponseNo))
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.closeTerminalDockSession(session)
		}
	})
	dialog.Present()
}

func (w *mainWindow) closeTerminalDockSession(session *terminalDockSession) {
	if session == nil || session.tab == nil {
		return
	}
	tab := session.tab
	tabIndex := w.indexOfTerminalDockTab(tab)
	sessionIndex := w.indexOfTerminalDockSession(session)
	if tabIndex < 0 || sessionIndex < 0 {
		return
	}
	session.terminal.Stop()
	w.terminalDockNotebook.RemovePage(tabIndex)
	w.terminalDockTabs = append(w.terminalDockTabs[:tabIndex], w.terminalDockTabs[tabIndex+1:]...)
	w.terminalDockSessions = append(w.terminalDockSessions[:sessionIndex], w.terminalDockSessions[sessionIndex+1:]...)
	w.updateTerminalDockTitle()
	w.setStatus(fmt.Sprintf("Closed local terminal %d", session.id), false)
}

func (w *mainWindow) confirmRestartTerminalDock() {
	session := w.activeTerminalDockSession()
	if session == nil {
		w.newTerminalDockTab()
		return
	}
	if !session.terminal.Running() {
		w.restartTerminalDock(session)
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle("Restart local terminal")
	dialog.SetMarkup("Stop the shell running in <b>" + html.EscapeString(valueOrDash(session.cwd)) + "</b> and start a new one?")
	dialog.SetObjectProperty("secondary-text", "Any foreground command running in the terminal will also be stopped.")
	dialog.SetDefaultResponse(int(gtk.ResponseNo))
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.restartTerminalDock(session)
		}
	})
	dialog.Present()
}

func (w *mainWindow) restartTerminalDock(session *terminalDockSession) {
	configuredShell := os.Getenv("SHELL")
	if w.options.Config != nil && strings.TrimSpace(w.options.Config.GUI.TerminalShell) != "" {
		configuredShell = w.options.Config.GUI.TerminalShell
	}
	shell, err := resolveTerminalShell(configuredShell)
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	session.terminal.Stop()
	if err := session.terminal.SpawnInDirectory(shell, nil, session.cwd); err != nil {
		w.setStatus("Restart local terminal: "+err.Error(), true)
		return
	}
	w.setStatus(fmt.Sprintf("Restarted local terminal %d", session.id), false)
	glib.IdleAdd(session.terminal.GrabFocus)
}

func (w *mainWindow) selectAdjacentTerminalDock(direction int) {
	if w.terminalDockNotebook == nil || w.terminalDockNotebook.NPages() < 2 {
		return
	}
	page := w.terminalDockNotebook.CurrentPage() + direction
	if page < 0 {
		page = w.terminalDockNotebook.NPages() - 1
	} else if page >= w.terminalDockNotebook.NPages() {
		page = 0
	}
	w.terminalDockNotebook.SetCurrentPage(page)
	if session := w.activeTerminalDockSession(); session != nil {
		glib.IdleAdd(session.terminal.GrabFocus)
	}
}

func (w *mainWindow) terminalDockWorkingDirectory() string {
	fallback, err := os.Getwd()
	if err != nil {
		fallback = os.TempDir()
	}
	if w.selectedTofuWorkspace == "" || (w.currentPage != pageTofuWorkspaces && w.currentPage != pageTofuResources && w.currentPage != pageTofuPlan) {
		return fallback
	}
	directory, err := filepath.Abs(w.selectedTofuWorkspace)
	if err != nil {
		return fallback
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return fallback
	}
	return directory
}

func resolveTerminalShell(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		configured = "sh"
	}
	path, err := exec.LookPath(configured)
	if err != nil {
		return "", fmt.Errorf("start local terminal: shell %q was not found: %w", configured, err)
	}
	return path, nil
}
