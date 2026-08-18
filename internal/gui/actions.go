//go:build gui

package gui

import (
	"fmt"
	"html"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

const stopReason = "Stopped by e9s"

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
		w.finishRequestWithStatus(ctx, generation, err, "Force deployment started for "+service, w.refreshAfterAction)
	}()
}

func (w *mainWindow) promptScaleService() {
	if w.selectedCluster == "" || w.selectedService == "" {
		return
	}
	service, found := findService(w.allServices, w.selectedService)
	if !found {
		w.setStatus("The selected service is no longer available", true)
		return
	}

	dialog := gtk.NewDialogWithFlags("Scale "+service.Name, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(12)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Desired task count")
	label.SetXAlign(0)
	count := gtk.NewSpinButtonWithRange(0, 10000, 1)
	count.SetValue(float64(service.DesiredCount))
	count.SetActivatesDefault(true)
	content.Append(label)
	content.Append(count)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Scale", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		desiredCount := count.ValueAsInt()
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.scaleService(desiredCount)
		}
	})
	dialog.Present()
}

func (w *mainWindow) scaleService(count int) {
	cluster, service := w.selectedCluster, w.selectedService
	ctx, generation := w.startRequest("Scaling " + service + "…")
	go func() {
		err := w.options.ECS.ScaleService(ctx, cluster, service, count)
		w.finishRequestWithStatus(ctx, generation, err,
			fmt.Sprintf("Scaled %s to %d tasks", service, count), w.refreshAfterAction)
	}()
}

func (w *mainWindow) confirmStopTask() {
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found {
		return
	}
	dialog := gtk.NewMessageDialog(
		&w.window.Window,
		gtk.DialogModal,
		gtk.MessageWarning,
		gtk.ButtonsYesNo,
	)
	dialog.SetMarkup("Stop task <b>" + html.EscapeString(shortID(task.TaskID)) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "ECS may start a replacement task to maintain the service's desired count.")
	dialog.SetTitle("Confirm stop task")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.stopTask(task)
		}
	})
	dialog.Present()
}

func (w *mainWindow) stopTask(task model.Task) {
	cluster := w.selectedCluster
	ctx, generation := w.startRequest("Stopping task " + shortID(task.TaskID) + "…")
	go func() {
		err := w.options.ECS.StopTask(ctx, cluster, task.TaskARN, stopReason)
		w.finishRequestWithStatus(ctx, generation, err, "Stop requested for task "+shortID(task.TaskID), w.refreshAfterAction)
	}()
}

func (w *mainWindow) refreshAfterAction() {
	time.AfterFunc(time.Second, func() {
		if w.ctx.Err() == nil {
			w.scheduleRefresh()
		}
	})
}
