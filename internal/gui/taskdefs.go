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

const (
	taskDefinitionSummaryMode = "summary"
	taskDefinitionEnvMode     = "environment"
	taskDefinitionDiffMode    = "diff"
)

func (w *mainWindow) buildTaskDefinitionPane() gtk.Widgetter {
	w.taskDefinitionSummaryButton = gtk.NewButtonWithLabel("Summary")
	w.taskDefinitionSummaryButton.ConnectClicked(w.showTaskDefinitionSummary)
	w.taskDefinitionEnvButton = gtk.NewButtonWithLabel("Environment")
	w.taskDefinitionEnvButton.ConnectClicked(w.openTaskDefinitionEnvironment)
	w.taskDefinitionRevealButton = gtk.NewButtonWithLabel("Reveal secret values")
	w.taskDefinitionRevealButton.SetVisible(false)
	w.taskDefinitionRevealButton.ConnectClicked(w.confirmRevealTaskDefinitionSecrets)
	w.taskDefinitionDiffButton = gtk.NewButtonWithLabel("Diff previous")
	w.taskDefinitionDiffButton.ConnectClicked(w.openTaskDefinitionDiff)
	w.taskDefinitionEditButton = gtk.NewButtonWithLabel("Edit JSON")
	w.taskDefinitionEditButton.ConnectClicked(w.openTaskDefinitionEditor)

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(w.taskDefinitionSummaryButton)
	toolbar.Append(w.taskDefinitionEnvButton)
	toolbar.Append(w.taskDefinitionRevealButton)
	toolbar.Append(w.taskDefinitionDiffButton)
	toolbar.Append(w.taskDefinitionEditButton)

	w.taskDefinitionBuffer = gtk.NewTextBuffer(nil)
	view := gtk.NewTextViewWithBuffer(w.taskDefinitionBuffer)
	view.SetEditable(false)
	view.SetCursorVisible(false)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapWordChar)
	view.AddCSSClass("inspector")
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetChild(view)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(scroll)
	return pane
}

func (w *mainWindow) buildTaskDefinitionEditor() gtk.Widgetter {
	closeButton := gtk.NewButtonWithLabel("Cancel")
	closeButton.ConnectClicked(w.closeTaskDefinitionEditor)
	validateButton := gtk.NewButtonWithLabel("Validate & format")
	validateButton.ConnectClicked(w.validateTaskDefinitionEditor)
	registerButton := gtk.NewButtonWithLabel("Register new revision")
	registerButton.AddCSSClass("suggested-action")
	registerButton.ConnectClicked(w.confirmRegisterTaskDefinition)
	label := gtk.NewLabel("Task-definition JSON editor")
	label.SetXAlign(0)
	label.SetHExpand(true)
	label.AddCSSClass("breadcrumb")

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(closeButton)
	toolbar.Append(label)
	toolbar.Append(validateButton)
	toolbar.Append(registerButton)

	w.editorBuffer = gtk.NewTextBuffer(nil)
	w.editorBuffer.ConnectChanged(func() {
		if !w.editorLoading {
			w.editorDirty = true
		}
	})
	editor := gtk.NewTextViewWithBuffer(w.editorBuffer)
	editor.SetEditable(true)
	editor.SetCursorVisible(true)
	editor.SetMonospace(true)
	editor.SetWrapMode(gtk.WrapNone)
	editor.AddCSSClass("inspector")
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(scroll)
	return pane
}

func (w *mainWindow) openTaskDefinitions() {
	if w.currentPage == pageTaskDefinitions {
		return
	}
	if w.guardEditorNavigation(w.openTaskDefinitions) {
		return
	}
	w.loadTaskDefinitions()
}

func (w *mainWindow) loadTaskDefinitions() {
	w.resetWorkspaceForBrowserChange()
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		w.clearTaskBrowser()
	}
	w.currentPage = pageTaskDefinitions
	w.selectedTaskDefinition = nil
	w.allTaskDefinitions = nil
	w.filteredTaskDefinitions = nil
	w.taskDefinitionTable.clear()
	w.updateActionSensitivity()
	w.setBreadcrumb("ECS / Task definitions")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter task definitions…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTaskDefinitions)
	w.detailStack.SetVisibleChildName("task-definition")
	w.taskDefinitionBuffer.SetText("Select a task definition and press Enter to inspect it.")
	w.setTaskDefinitionControls(false)

	ctx, generation := w.startRequest("Loading ECS task definitions…")
	go func() {
		definitions, err := w.options.ECS.ListTaskDefinitions(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allTaskDefinitions = definitions
			w.applyTaskDefinitionFilter()
			if len(definitions) == 0 {
				w.taskDefinitionBuffer.SetText("No active task definitions found in this region.")
			}
		})
	}()
}

func (w *mainWindow) applyTaskDefinitionFilter() {
	w.filteredTaskDefinitions = filterTaskDefinitions(w.allTaskDefinitions, w.search.Text())
	rows := make([]string, len(w.filteredTaskDefinitions))
	for i, definition := range w.filteredTaskDefinitions {
		rows[i] = fmt.Sprintf("%s\t%d\t%s", definition.Family, definition.Revision, definition.ARN)
	}
	w.taskDefinitionTable.replace(rows)
}

func filterTaskDefinitions(definitions []model.TaskDefRef, query string) []model.TaskDefRef {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.TaskDefRef(nil), definitions...)
	}
	filtered := make([]model.TaskDefRef, 0, len(definitions))
	for _, definition := range definitions {
		if strings.Contains(strings.ToLower(definition.Family), query) ||
			strings.Contains(strings.ToLower(definition.ARN), query) ||
			strings.Contains(fmt.Sprintf("%d", definition.Revision), query) {
			filtered = append(filtered, definition)
		}
	}
	return filtered
}

func (w *mainWindow) openTaskDefinitionAt(position uint) {
	if int(position) >= len(w.filteredTaskDefinitions) {
		return
	}
	ref := w.filteredTaskDefinitions[position]
	if w.selectedTaskDefinition != nil && w.selectedTaskDefinition.ARN == ref.ARN {
		return
	}
	if w.showingEditor {
		w.closeEditorThen(func() { w.openTaskDefinition(ref) })
		return
	}
	w.openTaskDefinition(ref)
}

func (w *mainWindow) openTaskDefinition(ref model.TaskDefRef) {
	w.resetWorkspaceForBrowserChange()
	w.selectedTaskDefinition = nil
	w.setTaskDefinitionControls(false)
	w.taskDefinitionBuffer.SetText("Loading " + ref.Family + ":" + fmt.Sprint(ref.Revision) + "…")
	w.detailStack.SetVisibleChildName("task-definition")
	ctx, generation := w.startRequest("Loading " + ref.Family + ":" + fmt.Sprint(ref.Revision) + "…")
	go func() {
		definition, err := w.options.ECS.GetTaskDefinition(ctx, ref.ARN)
		w.finishRequest(ctx, generation, err, func() {
			w.selectedTaskDefinition = definition
			w.setBreadcrumb(fmt.Sprintf("ECS / Task definitions / %s:%d", definition.Family, definition.Revision))
			w.showTaskDefinitionSummary()
			w.setTaskDefinitionControls(true)
		})
	}()
}

func (w *mainWindow) setTaskDefinitionControls(enabled bool) {
	w.taskDefinitionSummaryButton.SetSensitive(enabled)
	w.taskDefinitionEnvButton.SetSensitive(enabled)
	w.taskDefinitionDiffButton.SetSensitive(enabled)
	w.taskDefinitionEditButton.SetSensitive(enabled)
	if !enabled {
		w.taskDefinitionRevealButton.SetVisible(false)
	}
}

func (w *mainWindow) showTaskDefinitionSummary() {
	if w.selectedTaskDefinition == nil {
		return
	}
	w.taskDefinitionViewMode = taskDefinitionSummaryMode
	w.taskDefinitionRevealButton.SetVisible(false)
	w.taskDefinitionBuffer.SetText(formatTaskDefinitionSummary(w.selectedTaskDefinition))
	w.detailStack.SetVisibleChildName("task-definition")
}

func formatTaskDefinitionSummary(definition *model.TaskDefSummary) string {
	var out strings.Builder
	fmt.Fprintf(&out, "TASK DEFINITION\n\n%s:%d\n\n", definition.Family, definition.Revision)
	fmt.Fprintf(&out, "Status            %s\n", valueOrDash(definition.Status))
	fmt.Fprintf(&out, "CPU               %s\n", valueOrDash(definition.CPU))
	fmt.Fprintf(&out, "Memory            %s\n", valueOrDash(definition.Memory))
	fmt.Fprintf(&out, "Network mode      %s\n", valueOrDash(definition.NetworkMode))
	fmt.Fprintf(&out, "Compatibilities   %s\n", valueOrDash(strings.Join(definition.RequiresCompatibilities, ", ")))
	fmt.Fprintf(&out, "Task role         %s\n", valueOrDash(definition.TaskRoleArn))
	fmt.Fprintf(&out, "Execution role    %s\n", valueOrDash(definition.ExecutionRoleArn))
	fmt.Fprintf(&out, "Registered        %s\n", formatTime(definition.RegisteredAt))
	fmt.Fprintf(&out, "ARN               %s\n", valueOrDash(definition.ARN))
	out.WriteString("\nCONTAINERS\n")
	for _, container := range definition.Containers {
		fmt.Fprintf(&out, "\n  %s\n", valueOrDash(container.Name))
		fmt.Fprintf(&out, "    Image       %s\n", valueOrDash(container.Image))
		fmt.Fprintf(&out, "    CPU         %d\n", container.CPU)
		fmt.Fprintf(&out, "    Memory      %d\n", container.Memory)
		fmt.Fprintf(&out, "    Essential   %t\n", container.Essential)
		fmt.Fprintf(&out, "    Environment %d variables\n", len(container.EnvVars))
	}
	return strings.TrimRight(out.String(), "\n")
}

func (w *mainWindow) openTaskDefinitionEnvironment() {
	definition := w.selectedTaskDefinition
	if w.currentPage != pageTaskDefinitions || definition == nil {
		return
	}
	if len(definition.Containers) == 0 {
		w.setStatus("This task definition has no containers", true)
		return
	}
	if len(definition.Containers) == 1 {
		w.loadTaskDefinitionEnvironment(definition.Containers[0].Name, false)
		return
	}
	names := make([]string, 0, len(definition.Containers))
	for _, container := range definition.Containers {
		if container.Name != "" {
			names = append(names, container.Name)
		}
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
		if selected >= 0 && selected < len(names) {
			w.loadTaskDefinitionEnvironment(names[selected], false)
		}
	})
	dialog.Present()
}

func (w *mainWindow) loadTaskDefinitionEnvironment(container string, resolveSecrets bool) {
	definition := w.selectedTaskDefinition
	if definition == nil {
		return
	}
	ctx, generation := w.startRequest("Loading environment for " + container + "…")
	go func() {
		environment, err := w.options.ECS.TaskDefinitionEnvironment(ctx, definition.ARN, container, resolveSecrets)
		w.finishRequestWithStatus(ctx, generation, err, "Environment loaded for "+container, func() {
			w.taskDefinitionViewMode = taskDefinitionEnvMode
			w.taskDefinitionEnvContainer = container
			w.taskDefinitionBuffer.SetText(formatTaskDefinitionEnvironment(definition, container, environment, resolveSecrets))
			w.taskDefinitionRevealButton.SetVisible(!resolveSecrets && environmentHasSecrets(environment))
			w.detailStack.SetVisibleChildName("task-definition")
		})
	}()
}

func formatTaskDefinitionEnvironment(definition *model.TaskDefSummary, container string, environment []model.EnvVar, resolved bool) string {
	environment = append([]model.EnvVar(nil), environment...)
	slices.SortFunc(environment, func(a, b model.EnvVar) int { return strings.Compare(a.Name, b.Name) })
	var out strings.Builder
	fmt.Fprintf(&out, "ENVIRONMENT\n\n%s:%d / %s\n", definition.Family, definition.Revision, container)
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

func environmentHasSecrets(environment []model.EnvVar) bool {
	for _, variable := range environment {
		if variable.Source != "" {
			return true
		}
	}
	return false
}

func (w *mainWindow) confirmRevealTaskDefinitionSecrets() {
	if w.taskDefinitionViewMode != taskDefinitionEnvMode || w.taskDefinitionEnvContainer == "" {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Reveal secret values")
	dialog.SetMarkup("Resolve and display secret values for <b>" + html.EscapeString(w.taskDefinitionEnvContainer) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "Values fetched from SSM and Secrets Manager will be visible in the application window.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.loadTaskDefinitionEnvironment(w.taskDefinitionEnvContainer, true)
		}
	})
	dialog.Present()
}

func (w *mainWindow) openTaskDefinitionDiff() {
	definition := w.selectedTaskDefinition
	if w.currentPage != pageTaskDefinitions || definition == nil {
		return
	}
	previous, found := previousTaskDefinition(w.allTaskDefinitions, definition.Family, definition.Revision)
	if !found {
		w.setStatus("No earlier active revision is available for this family", true)
		return
	}
	ctx, generation := w.startRequest(fmt.Sprintf("Diffing %s:%d and %s:%d…", previous.Family, previous.Revision, definition.Family, definition.Revision))
	go func() {
		diff, err := w.options.ECS.TaskDefinitionDiff(ctx, previous.ARN, definition.ARN)
		w.finishRequestWithStatus(ctx, generation, err, "Task-definition diff loaded", func() {
			w.taskDefinitionViewMode = taskDefinitionDiffMode
			w.taskDefinitionRevealButton.SetVisible(false)
			w.taskDefinitionBuffer.SetText(diff)
			w.detailStack.SetVisibleChildName("task-definition")
		})
	}()
}

func previousTaskDefinition(definitions []model.TaskDefRef, family string, revision int) (model.TaskDefRef, bool) {
	var previous model.TaskDefRef
	for _, candidate := range definitions {
		if candidate.Family == family && candidate.Revision < revision && candidate.Revision > previous.Revision {
			previous = candidate
		}
	}
	return previous, previous.ARN != ""
}

func (w *mainWindow) openTaskDefinitionEditor() {
	if w.currentPage != pageTaskDefinitions || w.selectedTaskDefinition == nil || w.showingEditor {
		return
	}
	document, err := w.options.ECS.TaskDefinitionEditorDocument(w.selectedTaskDefinition.RawJSON)
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.editorLoading = true
	w.editorBuffer.SetText(document)
	w.editorLoading = false
	w.editorDirty = false
	w.showingEditor = true
	w.editorKind = editorKindTaskDefinition
	w.detailStack.SetVisibleChildName("editor")
	w.setStatus("Editing creates a new task-definition revision", false)
}

func (w *mainWindow) editorText() string {
	start, end := w.editorBuffer.Bounds()
	return w.editorBuffer.Text(start, end, true)
}

func (w *mainWindow) validateTaskDefinitionEditor() {
	if !w.showingEditor {
		return
	}
	document, err := w.options.ECS.TaskDefinitionEditorDocument(w.editorText())
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.editorLoading = true
	w.editorBuffer.SetText(document)
	w.editorLoading = false
	w.editorDirty = true
	w.setStatus("Task-definition JSON is valid", false)
}

func (w *mainWindow) confirmRegisterTaskDefinition() {
	if !w.showingEditor || w.selectedTaskDefinition == nil {
		return
	}
	document, err := w.options.ECS.TaskDefinitionEditorDocument(w.editorText())
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.editorLoading = true
	w.editorBuffer.SetText(document)
	w.editorLoading = false
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle("Register task definition")
	dialog.SetMarkup("Register a new revision of <b>" + html.EscapeString(w.selectedTaskDefinition.Family) + "</b> from this JSON?")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.registerTaskDefinition(document)
		}
	})
	dialog.Present()
}

func (w *mainWindow) registerTaskDefinition(document string) {
	ctx, generation := w.startRequest("Registering task-definition revision…")
	go func() {
		definition, err := w.options.ECS.RegisterTaskDefinitionJSON(ctx, document)
		w.finishRequestWithStatus(ctx, generation, err,
			fmt.Sprintf("Registered %s:%d", definitionFamily(definition), definitionRevision(definition)), func() {
				w.selectedTaskDefinition = definition
				w.showingEditor = false
				w.editorKind = ""
				w.editorDirty = false
				w.setBreadcrumb(fmt.Sprintf("ECS / Task definitions / %s:%d", definition.Family, definition.Revision))
				w.showTaskDefinitionSummary()
				w.refreshTaskDefinitions(false)
			})
	}()
}

func definitionFamily(definition *model.TaskDefSummary) string {
	if definition == nil {
		return "task definition"
	}
	return definition.Family
}

func definitionRevision(definition *model.TaskDefSummary) int {
	if definition == nil {
		return 0
	}
	return definition.Revision
}

func (w *mainWindow) closeTaskDefinitionEditor() {
	w.closeTaskDefinitionEditorThen(nil)
}

func (w *mainWindow) closeTaskDefinitionEditorThen(after func()) {
	if !w.showingEditor {
		if after != nil {
			after()
		}
		return
	}
	if !w.editorDirty {
		w.closeTaskDefinitionEditorNow()
		if after != nil {
			after()
		}
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Discard editor changes")
	dialog.SetMarkup("Discard the unsaved task-definition edits?")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.closeTaskDefinitionEditorNow()
			if after != nil {
				after()
			}
		}
	})
	dialog.Present()
}

func (w *mainWindow) closeTaskDefinitionEditorNow() {
	w.showingEditor = false
	w.editorKind = ""
	w.editorDirty = false
	w.detailStack.SetVisibleChildName("task-definition")
	w.setStatus("Editor closed", false)
}

func (w *mainWindow) refreshTaskDefinitions(foreground bool) {
	selectedARN := ""
	if w.selectedTaskDefinition != nil {
		selectedARN = w.selectedTaskDefinition.ARN
	}
	ctx, generation := w.startRefreshRequest("Refreshing task definitions…", foreground)
	go func() {
		definitions, err := w.options.ECS.ListTaskDefinitions(ctx, "")
		var selected *model.TaskDefSummary
		if err == nil && selectedARN != "" {
			if _, found := findTaskDefinition(definitions, selectedARN); found {
				selected, err = w.options.ECS.GetTaskDefinition(ctx, selectedARN)
			}
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allTaskDefinitions = definitions
			w.applyTaskDefinitionFilter()
			if selectedARN == "" {
				return
			}
			if selected == nil {
				w.selectedTaskDefinition = nil
				w.setTaskDefinitionControls(false)
				w.taskDefinitionBuffer.SetText("The selected task definition is no longer active.")
				return
			}
			w.selectedTaskDefinition = selected
			if w.taskDefinitionViewMode == taskDefinitionSummaryMode {
				w.showTaskDefinitionSummary()
			}
		})
	}()
}

func findTaskDefinition(definitions []model.TaskDefRef, arn string) (model.TaskDefRef, bool) {
	for _, definition := range definitions {
		if definition.ARN == arn {
			return definition, true
		}
	}
	return model.TaskDefRef{}, false
}
