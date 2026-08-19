//go:build gui

package gui

import (
	"fmt"
	"html"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func (w *mainWindow) buildTerminalPane() gtk.Widgetter {
	back := gtk.NewButtonWithLabel("Disconnect")
	back.AddCSSClass("destructive-action")
	back.ConnectClicked(w.closeTerminal)
	w.terminalTitle = gtk.NewLabel("Terminal session")
	w.terminalTitle.SetXAlign(0)
	w.terminalTitle.SetHExpand(true)
	w.terminalTitle.AddCSSClass("breadcrumb")
	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(back)
	toolbar.Append(w.terminalTitle)

	w.terminal = newVTETerminal()
	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(w.terminal.Widget())
	return pane
}

func (w *mainWindow) openExec() {
	if !vteAvailable() || w.selectedTask == "" {
		return
	}
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found {
		w.setStatus("The selected task is no longer available", true)
		return
	}
	if w.currentPage == pageTasks {
		if service, found := findService(w.allServices, w.selectedService); found && !service.EnableExecuteCommand {
			w.setStatus("ECS Exec is not enabled on service "+service.Name, true)
			return
		}
	}
	names := taskContainerNames(task)
	if len(names) == 0 {
		w.setStatus("The selected task has no containers", true)
		return
	}
	if len(names) == 1 {
		w.promptExecCommand(task, names[0])
		return
	}

	selector := gtk.NewDropDownFromStrings(names)
	dialog := gtk.NewDialogWithFlags("Choose a container", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(12)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	content.Append(gtk.NewLabel("Open ECS Exec in container:"))
	content.Append(selector)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Continue", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		selected := -1
		if response == int(gtk.ResponseOK) {
			selected = int(selector.Selected())
		}
		dialog.Destroy()
		if selected >= 0 && selected < len(names) {
			w.promptExecCommand(task, names[selected])
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptExecCommand(task model.Task, container string) {
	dialog := gtk.NewDialogWithFlags("ECS Exec command", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(12)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Interactive command for " + container)
	label.SetXAlign(0)
	command := gtk.NewEntry()
	command.SetText("/bin/sh")
	command.SetActivatesDefault(true)
	command.SetHExpand(true)
	content.Append(label)
	content.Append(command)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Connect", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		value := command.Text()
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.startExec(task, container, value)
		}
	})
	dialog.Present()
}

func (w *mainWindow) startExec(task model.Task, container, command string) {
	cluster := w.selectedCluster
	ctx, generation := w.startRequest("Starting ECS Exec session…")
	go func() {
		launch, err := w.options.ECS.PrepareExecSession(ctx, cluster, task, container, command)
		w.finishRequestWithStatus(ctx, generation, err, "ECS Exec connected to "+shortID(task.TaskID)+" / "+container, func() {
			if err := w.terminal.Spawn(launch.Executable, launch.Args); err != nil {
				w.setStatus(err.Error(), true)
				return
			}
			if w.logCancel != nil && w.showingLogs {
				w.logCancel()
				w.logGeneration++
			}
			w.showingLogs = false
			w.showingMetrics = false
			w.showingTerminal = true
			w.terminalTask = task
			w.terminalContainer = container
			w.terminalCommand = command
			w.terminalDescription = "ECS Exec for " + shortID(task.TaskID) + " / " + container
			w.terminalTitle.SetLabel(fmt.Sprintf("ECS Exec — %s / %s — %s", shortID(task.TaskID), container, command))
			w.detailStack.SetVisibleChildName("terminal")
		})
	}()
}

func (w *mainWindow) closeTerminal() {
	if !w.showingTerminal {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle("Disconnect terminal session")
	description := valueOrDash(w.terminalDescription)
	dialog.SetMarkup("Disconnect <b>" + html.EscapeString(description) + "</b>?")
	dialog.SetDefaultResponse(int(gtk.ResponseNo))
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.closeTerminalNow(true)
		}
	})
	dialog.Present()
}

func (w *mainWindow) closeTerminalNow(updateStatus bool) {
	if w.terminal != nil {
		w.terminal.Stop()
	}
	w.showingTerminal = false
	description := w.terminalDescription
	w.terminalDescription = ""
	w.detailStack.SetVisibleChildName("detail")
	if updateStatus {
		if description == "" {
			description = "Terminal session"
		}
		w.setStatus(description+" disconnected", false)
	}
}
