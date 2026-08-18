//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"
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

func (w *mainWindow) promptRunTask() {
	if w.currentPage != pageStandaloneTasks || w.selectedCluster == "" {
		return
	}
	seedTaskDefinition := ""
	if task, found := findTask(w.allTasks, w.selectedTask); found {
		seedTaskDefinition = task.TaskDefinition
	}

	dialog := gtk.NewDialogWithFlags("Run standalone task", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(560, -1)
	content := dialog.ContentArea()
	content.SetSpacing(10)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	taskDefinition := appendDialogEntry(content, "Task definition", "family:revision or ARN", seedTaskDefinition)
	launchLabel := gtk.NewLabel("Launch type")
	launchLabel.SetXAlign(0)
	launchType := gtk.NewDropDownFromStrings([]string{"Cluster default", "FARGATE", "EC2", "EXTERNAL", "MANAGED_INSTANCES"})
	launchType.SetHExpand(true)
	content.Append(launchLabel)
	content.Append(launchType)
	countLabel := gtk.NewLabel("Count")
	countLabel.SetXAlign(0)
	count := gtk.NewSpinButtonWithRange(1, 100, 1)
	count.SetValue(1)
	content.Append(countLabel)
	content.Append(count)
	subnets := appendDialogEntry(content, "Subnets", "subnet-a, subnet-b (required for awsvpc)", "")
	securityGroups := appendDialogEntry(content, "Security groups", "sg-a, sg-b", "")
	group := appendDialogEntry(content, "Task group", "optional", "")
	publicIP := gtk.NewCheckButtonWithLabel("Assign public IP")
	exec := gtk.NewCheckButtonWithLabel("Enable ECS Exec")
	content.Append(publicIP)
	content.Append(exec)
	note := gtk.NewLabel("For awsvpc/Fargate tasks, provide at least one subnet. Leaving launch type blank uses the cluster's default capacity-provider strategy.")
	note.SetXAlign(0)
	note.SetWrap(true)
	note.AddCSSClass("muted")
	content.Append(note)

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Run task", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		request := model.RunTaskRequest{}
		if response == int(gtk.ResponseOK) {
			launchTypes := []string{"", "FARGATE", "EC2", "EXTERNAL", "MANAGED_INSTANCES"}
			selectedLaunchType := int(launchType.Selected())
			request = model.RunTaskRequest{
				Cluster:              w.selectedCluster,
				TaskDefinition:       taskDefinition.Text(),
				Count:                count.ValueAsInt(),
				Subnets:              splitCommaSeparated(subnets.Text()),
				SecurityGroups:       splitCommaSeparated(securityGroups.Text()),
				AssignPublicIP:       publicIP.Active(),
				EnableExecuteCommand: exec.Active(),
				Group:                group.Text(),
			}
			if selectedLaunchType >= 0 && selectedLaunchType < len(launchTypes) {
				request.LaunchType = launchTypes[selectedLaunchType]
			}
		}
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runTask(request)
		}
	})
	dialog.Present()
}

func appendDialogEntry(content *gtk.Box, labelText, placeholder, value string) *gtk.Entry {
	label := gtk.NewLabel(labelText)
	label.SetXAlign(0)
	entry := gtk.NewEntry()
	entry.SetHExpand(true)
	entry.SetPlaceholderText(placeholder)
	entry.SetText(value)
	content.Append(label)
	content.Append(entry)
	return entry
}

func (w *mainWindow) runTask(request model.RunTaskRequest) {
	ctx, generation := w.startRequest("Starting " + valueOrDash(strings.TrimSpace(request.TaskDefinition)) + "…")
	go func() {
		tasks, err := w.options.ECS.RunTask(ctx, request)
		message := fmt.Sprintf("Started %d task(s) from %s", len(tasks), strings.TrimSpace(request.TaskDefinition))
		w.finishRequestWithStatus(ctx, generation, err, message, w.refreshAfterAction)
	}()
}
