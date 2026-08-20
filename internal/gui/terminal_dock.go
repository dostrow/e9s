//go:build gui

package gui

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

const terminalDockPreferredHeight = 280

type terminalDockSession struct {
	id       int
	terminal *vteTerminal
	cwd      string
	page     gtk.Widgetter
	tabLabel *gtk.Label
}

func (w *mainWindow) buildTerminalDock() *gtk.Box {
	hideButton := gtk.NewButtonWithLabel("Hide")
	hideButton.ConnectClicked(func() { w.terminalDockButton.SetActive(false) })
	newButton := gtk.NewButtonWithLabel("New tab")
	newButton.SetTooltipText("Open another local terminal (Ctrl+Shift+T)")
	newButton.ConnectClicked(func() { w.newTerminalDockTab() })
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
	toolbar.Append(closeButton)
	toolbar.Append(restartButton)

	w.terminalDockNotebook = gtk.NewNotebook()
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
	configuredShell := os.Getenv("SHELL")
	if w.options.Config != nil && strings.TrimSpace(w.options.Config.GUI.TerminalShell) != "" {
		configuredShell = w.options.Config.GUI.TerminalShell
	}
	shell, err := resolveTerminalShell(configuredShell)
	if err != nil {
		return nil, err
	}
	workingDirectory := w.terminalDockWorkingDirectory()
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
		id:       w.terminalDockNextID,
		terminal: terminal,
		cwd:      workingDirectory,
		page:     terminal.Widget(),
		tabLabel: gtk.NewLabel(fmt.Sprintf("Terminal %d", w.terminalDockNextID)),
	}
	session.tabLabel.SetTooltipText(workingDirectory)
	tab := w.terminalDockTabLabel(session)
	w.terminalDockSessions = append(w.terminalDockSessions, session)
	page := w.terminalDockNotebook.AppendPage(session.page, tab)
	w.terminalDockNotebook.SetCurrentPage(page)
	w.updateTerminalDockTitle()
	w.setStatus("Started local terminal in "+workingDirectory, false)
	return session, nil
}

func (w *mainWindow) terminalDockTabLabel(session *terminalDockSession) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.Append(session.tabLabel)
	closeButton := gtk.NewButtonFromIconName("window-close-symbolic")
	closeButton.AddCSSClass("flat")
	closeButton.AddCSSClass("terminal-tab-close")
	closeButton.SetTooltipText("Close this terminal")
	closeButton.ConnectClicked(func() { w.confirmCloseTerminalDock(session) })
	box.Append(closeButton)
	return box
}

func (w *mainWindow) activeTerminalDockSession() *terminalDockSession {
	if w.terminalDockNotebook == nil {
		return nil
	}
	page := w.terminalDockNotebook.CurrentPage()
	if page < 0 || page >= len(w.terminalDockSessions) {
		return nil
	}
	return w.terminalDockSessions[page]
}

func (w *mainWindow) indexOfTerminalDockSession(session *terminalDockSession) int {
	for index, candidate := range w.terminalDockSessions {
		if candidate == session {
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
	w.terminalDockTitle.SetLabel(fmt.Sprintf("Local terminal %d — %s", session.id, session.cwd))
	w.terminalDockTitle.SetTooltipText(session.cwd)
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
	index := w.indexOfTerminalDockSession(session)
	if index < 0 {
		return
	}
	session.terminal.Stop()
	w.terminalDockNotebook.RemovePage(index)
	w.terminalDockSessions = append(w.terminalDockSessions[:index], w.terminalDockSessions[index+1:]...)
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
