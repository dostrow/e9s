//go:build gui

package gui

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func (w *mainWindow) openECSEnvironment() {
	if w.selectedTask != "" && (w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks || w.currentPage == pageStoppedTasks) {
		w.openTaskEnvironment()
		return
	}
	w.openTaskDefinitionEnvironment()
}

func (w *mainWindow) openTaskEnvironment() {
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found || w.options.ECS == nil {
		return
	}
	if len(task.Containers) == 0 {
		w.setStatus("This task has no containers", true)
		return
	}

	names := make([]string, 0, len(task.Containers))
	for _, container := range task.Containers {
		if container.Name != "" && !slices.Contains(names, container.Name) {
			names = append(names, container.Name)
		}
	}
	if len(names) == 0 {
		w.setStatus("This task has no named containers", true)
		return
	}
	if len(names) == 1 {
		w.loadTaskEnvironment(task, names[0], false)
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
	content.Append(gtk.NewLabel("View environment for container:"))
	content.Append(selector)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("View", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		selected := -1
		if response == int(gtk.ResponseOK) {
			selected = int(selector.Selected())
		}
		dialog.Destroy()
		if selected >= 0 && selected < len(names) && w.selectedTask == task.TaskARN {
			w.loadTaskEnvironment(task, names[selected], false)
		}
	})
	dialog.Present()
}

func (w *mainWindow) loadTaskEnvironment(task model.Task, container string, resolveSecrets bool) {
	if task.TaskDefinition == "" || container == "" || w.options.ECS == nil {
		return
	}
	taskARN := task.TaskARN
	ctx, generation := w.startRequest("Loading environment for " + container + "…")
	go func() {
		environment, err := w.options.ECS.TaskDefinitionEnvironment(ctx, task.TaskDefinition, container, resolveSecrets)
		w.finishRequestWithStatus(ctx, generation, err, "Environment loaded for "+container, func() {
			if w.selectedTask != taskARN || (resolveSecrets && w.detailContent != detailTaskEnvironment) {
				return
			}
			w.taskEnvironmentContainer = container
			w.taskEnvironmentTaskARN = taskARN
			w.taskEnvironmentResolved = resolveSecrets
			w.taskEnvironmentHasSecrets = environmentHasSecrets(environment)
			w.setDetail(formatTaskEnvironment(task, container, environment, resolveSecrets), detailTaskEnvironment)
			w.detailStack.SetVisibleChildName("detail")
			w.updateActionSensitivity()
		})
	}()
}

func formatTaskEnvironment(task model.Task, container string, environment []model.EnvVar, resolved bool) string {
	environment = append([]model.EnvVar(nil), environment...)
	slices.SortFunc(environment, func(a, b model.EnvVar) int { return strings.Compare(a.Name, b.Name) })

	var out strings.Builder
	fmt.Fprintf(&out, "TASK ENVIRONMENT\n\n%s / %s\n", shortID(task.TaskID), container)
	fmt.Fprintf(&out, "Task definition  %s\n", valueOrDash(task.TaskDefinition))
	if len(environment) == 0 {
		out.WriteString("\nNo environment variables.")
		return out.String()
	}
	for _, variable := range environment {
		value := variable.Value
		if variable.Source != "" && resolved {
			value = variable.ResolvedValue
		}
		fmt.Fprintf(&out, "\n%s", variable.Name)
		if variable.Source != "" {
			fmt.Fprintf(&out, "  [%s]", variable.Source)
		}
		fmt.Fprintf(&out, "\n  %s\n", valueOrDash(value))
	}
	return strings.TrimRight(out.String(), "\n")
}

func (w *mainWindow) showSelectedTaskDetails() {
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found {
		return
	}
	w.taskEnvironmentContainer = ""
	w.taskEnvironmentTaskARN = ""
	w.taskEnvironmentResolved = false
	w.taskEnvironmentHasSecrets = false
	w.renderTaskDetail(task)
	w.detailStack.SetVisibleChildName("detail")
	w.updateActionSensitivity()
}

func (w *mainWindow) confirmRevealTaskEnvironmentSecrets() {
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found || w.detailContent != detailTaskEnvironment || w.taskEnvironmentContainer == "" ||
		w.taskEnvironmentResolved || !w.taskEnvironmentHasSecrets {
		return
	}
	container := w.taskEnvironmentContainer
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Reveal secret values")
	dialog.SetMarkup("Resolve and display secret values for <b>" + html.EscapeString(container) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "Values fetched from SSM and Secrets Manager will be visible in the application window.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) && w.selectedTask == task.TaskARN {
			w.loadTaskEnvironment(task, container, true)
		}
	})
	dialog.Present()
}
