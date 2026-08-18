//go:build gui

package gui

import (
	"html"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func (w *mainWindow) confirmForceDeployment() {
	if w.selectedCluster == "" || w.selectedService == "" {
		return
	}

	dialog := gtk.NewMessageDialog(
		&w.window.Window,
		gtk.DialogModal,
		gtk.MessageQuestion,
		gtk.ButtonsYesNo,
	)
	dialog.SetMarkup("Force a new deployment for <b>" + html.EscapeString(w.selectedService) + "</b>?")
	dialog.SetTitle("Confirm force deployment")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.forceDeployment()
		}
	})
	dialog.Present()
}

func (w *mainWindow) forceDeployment() {
	cluster, service := w.selectedCluster, w.selectedService
	ctx, generation := w.startRequest("Requesting a new deployment for " + service + "…")
	go func() {
		err := w.options.ECS.ForceDeployment(ctx, cluster, service)
		w.finishRequestWithStatus(ctx, generation, err, "Force deployment started for "+service, func() {
			time.AfterFunc(time.Second, func() {
				if w.ctx.Err() == nil {
					// refresh schedules all widget work on the GTK thread
					// through the existing periodic/main-loop path.
					w.scheduleRefresh()
				}
			})
		})
	}()
}
