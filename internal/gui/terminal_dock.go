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

func (w *mainWindow) buildTerminalDock() *gtk.Box {
	hideButton := gtk.NewButtonWithLabel("Hide")
	hideButton.ConnectClicked(func() { w.terminalDockButton.SetActive(false) })
	restartButton := gtk.NewButtonWithLabel("Restart shell…")
	restartButton.ConnectClicked(w.confirmRestartTerminalDock)
	w.terminalDockTitle = gtk.NewLabel("Local terminal")
	w.terminalDockTitle.SetXAlign(0)
	w.terminalDockTitle.SetHExpand(true)
	w.terminalDockTitle.SetEllipsize(pango.EllipsizeMiddle)
	w.terminalDockTitle.AddCSSClass("breadcrumb")

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(hideButton)
	toolbar.Append(w.terminalDockTitle)
	toolbar.Append(restartButton)

	w.terminalDock = newVTETerminal()
	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.AddCSSClass("terminal-dock")
	pane.SetSizeRequest(-1, 160)
	pane.Append(toolbar)
	pane.Append(w.terminalDock.Widget())
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
		w.setStatus("Local terminal dock hidden; its shell remains running", false)
		return
	}
	if !w.terminalDockStarted {
		if err := w.spawnTerminalDock(); err != nil {
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
		w.terminalDock.GrabFocus()
	})
}

func (w *mainWindow) spawnTerminalDock() error {
	configuredShell := os.Getenv("SHELL")
	if w.options.Config != nil && strings.TrimSpace(w.options.Config.GUI.TerminalShell) != "" {
		configuredShell = w.options.Config.GUI.TerminalShell
	}
	shell, err := resolveTerminalShell(configuredShell)
	if err != nil {
		return err
	}
	workingDirectory := w.terminalDockWorkingDirectory()
	if err := w.terminalDock.SpawnInDirectory(shell, nil, workingDirectory); err != nil {
		return fmt.Errorf("start local terminal: %w", err)
	}
	w.terminalDockStarted = true
	w.terminalDockCWD = workingDirectory
	w.terminalDockTitle.SetLabel("Local terminal — " + workingDirectory)
	w.terminalDockTitle.SetTooltipText(workingDirectory)
	w.setStatus("Started local terminal in "+workingDirectory, false)
	return nil
}

func (w *mainWindow) confirmRestartTerminalDock() {
	if w.terminalDock == nil {
		return
	}
	if !w.terminalDock.Running() {
		w.restartTerminalDock()
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle("Restart local terminal")
	dialog.SetMarkup("Stop the shell running in <b>" + html.EscapeString(valueOrDash(w.terminalDockCWD)) + "</b> and start a new one?")
	dialog.SetObjectProperty("secondary-text", "Any foreground command running in the terminal will also be stopped.")
	dialog.SetDefaultResponse(int(gtk.ResponseNo))
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.restartTerminalDock()
		}
	})
	dialog.Present()
}

func (w *mainWindow) restartTerminalDock() {
	w.terminalDock.Stop()
	w.terminalDockStarted = false
	if err := w.spawnTerminalDock(); err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	glib.IdleAdd(w.terminalDock.GrabFocus)
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
