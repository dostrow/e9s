//go:build gui

package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type paneZoomTarget uint8

const (
	zoomBrowser paneZoomTarget = iota
	zoomWorkspace
	zoomTerminal
)

var (
	zoomInAccelerators    = []string{"<Control>plus", "<Control>equal", "<Control>KP_Add"}
	zoomOutAccelerators   = []string{"<Control>minus", "<Control>KP_Subtract"}
	zoomResetAccelerators = []string{"<Control>0", "<Control>KP_0"}
)

func (w *mainWindow) installPaneZoom(browser, workspace *gtk.Widget) {
	w.browserPane = browser
	w.workspacePane = workspace
	w.browserZoom = zoomDefault
	w.workspaceZoom = zoomDefault
	w.terminalZoom = zoomDefault
	w.zoomTarget = zoomBrowser
	w.applyZoomClass(browser, 0, zoomDefault)
	w.applyZoomClass(workspace, 0, zoomDefault)
	w.installZoomControllers(browser, zoomBrowser)
	w.installZoomControllers(workspace, zoomWorkspace)
}

func (w *mainWindow) installZoomControllers(widget *gtk.Widget, target paneZoomTarget) {
	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(_, _ float64) { w.zoomTarget = target })
	widget.AddController(motion)

	focus := gtk.NewEventControllerFocus()
	focus.ConnectEnter(func() { w.zoomTarget = target })
	widget.AddController(focus)

	scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical | gtk.EventControllerScrollDiscrete)
	scroll.SetPropagationPhase(gtk.PhaseCapture)
	scroll.ConnectScroll(func(_, dy float64) bool {
		if !scroll.CurrentEventState().Has(gdk.ControlMask) || dy == 0 {
			return false
		}
		w.zoomTarget = target
		if dy < 0 {
			w.adjustPaneZoom(target, 1)
		} else {
			w.adjustPaneZoom(target, -1)
		}
		return true
	})
	widget.AddController(scroll)
}

func (w *mainWindow) adjustActivePaneZoom(direction int) {
	w.adjustPaneZoom(w.zoomTarget, direction)
}

func (w *mainWindow) resetActivePaneZoom() {
	w.setPaneZoom(w.zoomTarget, zoomDefault)
}

func (w *mainWindow) adjustPaneZoom(target paneZoomTarget, direction int) {
	current := w.browserZoom
	switch target {
	case zoomWorkspace:
		current = w.workspaceZoom
	case zoomTerminal:
		current = w.terminalZoom
	}
	w.setPaneZoom(target, steppedZoom(current, direction))
}

func (w *mainWindow) setPaneZoom(target paneZoomTarget, level int) {
	name := "Browser"
	widget := w.browserPane
	current := w.browserZoom
	switch target {
	case zoomWorkspace:
		name = "Workspace"
		widget = w.workspacePane
		current = w.workspaceZoom
	case zoomTerminal:
		if len(w.terminalDockSessions) == 0 {
			return
		}
		w.terminalZoom = level
		for _, session := range w.terminalDockSessions {
			session.terminal.SetFontScale(float64(level) / 100)
		}
		w.setStatus(fmt.Sprintf("Terminal dock zoom: %d%%", level), false)
		return
	}
	if widget == nil {
		return
	}
	w.applyZoomClass(widget, current, level)
	if target == zoomWorkspace {
		w.workspaceZoom = level
		if w.terminal != nil {
			w.terminal.SetFontScale(float64(level) / 100)
		}
	} else {
		w.browserZoom = level
	}
	w.setStatus(fmt.Sprintf("%s pane zoom: %d%%", name, level), false)
}

func (w *mainWindow) applyZoomClass(widget *gtk.Widget, oldLevel, newLevel int) {
	if oldLevel != 0 {
		widget.RemoveCSSClass(zoomClass(oldLevel))
	}
	widget.AddCSSClass(zoomClass(newLevel))
}
