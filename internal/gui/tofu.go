//go:build gui

package gui

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/tofu"
)

func (o Options) ConfigTofuDirs() []config.TofuDirEntry {
	if o.Config == nil {
		return nil
	}
	return o.Config.TofuDirs
}

func (w *mainWindow) openTofuModule() {
	if w.currentPage == pageTofuWorkspaces {
		return
	}
	w.loadTofuWorkspaces()
}

func (w *mainWindow) loadTofuWorkspaces() {
	w.resetWorkspaceForBrowserChange()
	w.clearTofuWorkspaces()
	w.clearTofuResources()
	w.currentPage = pageTofuWorkspaces
	w.activeSavedTofuWorkspace = ""
	w.search.SetPlaceholderText("Filter workspaces…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTofuWorkspaces)
	w.backButton.SetSensitive(false)
	w.setBreadcrumb("OpenTofu / Workspaces")
	w.allTofuWorkspaces = append([]config.TofuDirEntry(nil), w.options.ConfigTofuDirs()...)
	sort.SliceStable(w.allTofuWorkspaces, func(i, j int) bool {
		return strings.ToLower(w.allTofuWorkspaces[i].Name) < strings.ToLower(w.allTofuWorkspaces[j].Name)
	})
	w.applyTofuWorkspaceFilter()
	w.setDetail(tofuWorkspaceListSummary(len(w.allTofuWorkspaces)), detailIntro)
	w.updateActionSensitivity()
}

func (w *mainWindow) openSavedTofuWorkspace(saved config.TofuDirEntry) {
	w.loadTofuResources(saved)
}

func (w *mainWindow) clearTofuWorkspaces() {
	w.allTofuWorkspaces = nil
	w.filteredTofuWorkspaces = nil
	w.selectedTofuWorkspace = ""
	w.tofuWorkspaceInfo = nil
	if w.tofuWorkspaceTable != nil {
		w.tofuWorkspaceTable.clear()
	}
}

func (w *mainWindow) clearTofuResources() {
	w.allTofuResources = nil
	w.filteredTofuResources = nil
	w.selectedTofuResource = ""
	if w.tofuResourceTable != nil {
		w.tofuResourceTable.clear()
	}
}

func (w *mainWindow) applyTofuWorkspaceFilter() {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredTofuWorkspaces = w.filteredTofuWorkspaces[:0]
	rows := make([]string, 0, len(w.allTofuWorkspaces))
	for _, workspace := range w.allTofuWorkspaces {
		if query != "" && !strings.Contains(strings.ToLower(workspace.Name), query) && !strings.Contains(strings.ToLower(workspace.Dir), query) {
			continue
		}
		w.filteredTofuWorkspaces = append(w.filteredTofuWorkspaces, workspace)
		rows = append(rows, workspace.Name+"\t"+workspace.Dir)
	}
	w.tofuWorkspaceTable.replace(rows)
}

func (w *mainWindow) applyTofuResourceFilter() {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredTofuResources = w.filteredTofuResources[:0]
	rows := make([]string, 0, len(w.allTofuResources))
	for _, resource := range w.allTofuResources {
		if query != "" && !strings.Contains(strings.ToLower(resource.Address), query) && !strings.Contains(strings.ToLower(resource.Type), query) && !strings.Contains(strings.ToLower(resource.Module), query) {
			continue
		}
		w.filteredTofuResources = append(w.filteredTofuResources, resource)
		rows = append(rows, strings.Join([]string{resource.Type, resource.Name, valueOrDash(resource.Module)}, "\t"))
	}
	w.tofuResourceTable.replace(rows)
}

func (w *mainWindow) applyTofuPlanFilter() {
	if w.tofuPlan == nil {
		w.tofuPlanTable.clear()
		return
	}
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	rows := make([]string, 0, len(w.tofuPlan.Changes))
	for _, change := range w.tofuPlan.Changes {
		if query != "" && !strings.Contains(strings.ToLower(change.Address), query) && !strings.Contains(strings.ToLower(change.Action), query) && !strings.Contains(strings.ToLower(change.Type), query) {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%d", strings.ToUpper(change.Action), change.Address, len(change.Diffs)))
	}
	w.tofuPlanTable.replace(rows)
}

func (w *mainWindow) selectTofuWorkspaceRow() {
	if w.currentPage != pageTofuWorkspaces {
		return
	}
	position := w.tofuWorkspaceTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredTofuWorkspaces) {
		w.selectedTofuWorkspace = ""
		w.tofuWorkspaceInfo = nil
		w.setBreadcrumb("OpenTofu / Workspaces")
		w.setDetail(tofuWorkspaceListSummary(len(w.allTofuWorkspaces)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	workspace := w.filteredTofuWorkspaces[position]
	w.selectedTofuWorkspace = workspace.Dir
	w.tofuWorkspaceInfo = nil
	w.setBreadcrumb("OpenTofu / Workspaces / " + workspace.Name)
	w.setDetail("Inspecting OpenTofu workspace…", detailIntro)
	w.updateActionSensitivity()
	if w.options.Tofu == nil {
		w.setDetail("OpenTofu is unavailable because no workflow service was configured.", detailError)
		return
	}
	ctx, generation := w.startRequest("Inspecting OpenTofu workspace " + workspace.Name + "…")
	go func() {
		info, err := w.options.Tofu.Workspace(ctx, workspace.Dir)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageTofuWorkspaces || w.selectedTofuWorkspace != workspace.Dir {
				return
			}
			w.tofuWorkspaceInfo = &info
			w.setDetail(formatTofuWorkspace(workspace, info), detailTofuWorkspace)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) openTofuWorkspaceAt(position uint) {
	if int(position) >= len(w.filteredTofuWorkspaces) {
		return
	}
	w.loadTofuResources(w.filteredTofuWorkspaces[position])
}

func (w *mainWindow) loadTofuResources(workspace config.TofuDirEntry) {
	if w.options.Tofu == nil || workspace.Dir == "" {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearTofuResources()
	w.currentPage = pageTofuResources
	w.selectedTofuWorkspace = workspace.Dir
	w.activeSavedTofuWorkspace = workspace.Name
	w.search.SetPlaceholderText("Filter state resources…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTofuResources)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(tofuResourceBreadcrumb(workspace.Name, ""))
	w.setDetail("Loading OpenTofu state resources…", detailIntro)
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Loading state for " + workspace.Name + "…")
	go func() {
		resources, err := w.options.Tofu.Resources(ctx, workspace.Dir)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageTofuResources || w.selectedTofuWorkspace != workspace.Dir {
				return
			}
			w.allTofuResources = resources
			w.applyTofuResourceFilter()
			w.setDetail(tofuResourceListSummary(workspace, len(resources)), detailTofuWorkspace)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) selectTofuResourceRow() {
	if w.currentPage != pageTofuResources {
		return
	}
	position := w.tofuResourceTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredTofuResources) {
		w.selectedTofuResource = ""
		workspace, _ := w.currentTofuWorkspace()
		w.setBreadcrumb(tofuResourceBreadcrumb(w.activeSavedTofuWorkspace, ""))
		w.setDetail(tofuResourceListSummary(workspace, len(w.allTofuResources)), detailTofuWorkspace)
		w.updateActionSensitivity()
		return
	}
	resource := w.filteredTofuResources[position]
	w.selectedTofuResource = resource.Address
	w.setBreadcrumb(tofuResourceBreadcrumb(w.activeSavedTofuWorkspace, resource.Address))
	w.setDetail(formatTofuResourceSummary(resource), detailTofuResource)
	w.updateActionSensitivity()
}

func (w *mainWindow) openTofuResourceAt(position uint) {
	if int(position) >= len(w.filteredTofuResources) || w.options.Tofu == nil || w.tofuActionPending {
		return
	}
	resource := w.filteredTofuResources[position]
	w.tofuResourceTable.selection.SetSelected(position)
	w.tofuActionPending = true
	w.updateActionSensitivity()
	w.setDetail("Loading complete state for "+resource.Address+"…", detailIntro)
	ctx, generation := w.startRequest("Loading state for " + resource.Address + "…")
	go func() {
		output, err := w.options.Tofu.State(ctx, w.selectedTofuWorkspace, resource.Address)
		w.finishTofuAction(ctx, generation, err, "Loaded state for "+resource.Address, func() {
			if w.currentPage != pageTofuResources || w.selectedTofuResource != resource.Address {
				return
			}
			w.setDetail(formatTofuState(resource, output), detailTofuResource)
		})
	}()
}

func (w *mainWindow) runTofuPlan() {
	workspace, found := w.currentTofuWorkspace()
	if !found || w.options.Tofu == nil || w.tofuActionPending {
		return
	}
	w.discardTofuPlan()
	w.tofuActionPending = true
	w.updateActionSensitivity()
	w.setDetail("Running OpenTofu plan…", detailIntro)
	ctx, generation := w.startRequest("Planning " + workspace.Name + "…")
	go func() {
		plan, planFile, err := w.options.Tofu.Plan(ctx, workspace.Dir)
		success := "Plan completed"
		if plan != nil {
			success += ": " + tofu.FormatPlanSummary(plan)
		}
		w.finishTofuAction(ctx, generation, err, success, func() {
			w.tofuPlan = plan
			w.tofuPlanFile = planFile
			w.selectedTofuPlanChange = ""
			w.currentPage = pageTofuPlan
			w.search.SetPlaceholderText("Filter planned changes…")
			w.search.SetText("")
			w.resourceStack.SetVisibleChildName(pageTofuPlan)
			w.backButton.SetSensitive(true)
			w.setBreadcrumb("OpenTofu / " + workspace.Name + " / Plan")
			w.applyTofuPlanFilter()
			w.setDetail(formatTofuPlanSummary(workspace, plan), detailTofuPlan)
		})
	}()
}

func (w *mainWindow) confirmTofuInit() {
	workspace, found := w.currentTofuWorkspace()
	if !found || w.options.Tofu == nil || w.tofuActionPending {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsNone)
	dialog.SetTitle("Initialize OpenTofu workspace")
	dialog.SetMarkup("Run <b>init</b> in <b>" + html.EscapeString(workspace.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "This may download providers and modules, initialize the backend, and update the dependency lock file.")
	dialog.SetDestroyWithParent(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Run init", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runTofuInit(workspace)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runTofuInit(workspace config.TofuDirEntry) {
	if vteAvailable() {
		command, err := w.options.Tofu.InitCommand(workspace.Dir)
		if err != nil {
			w.setStatus(err.Error(), true)
			return
		}
		w.startTofuTerminalCommand(workspace, "OpenTofu init", command, false)
		return
	}
	w.tofuActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Initializing " + workspace.Name + "…")
	go func() {
		output, err := w.options.Tofu.Init(ctx, workspace.Dir)
		w.finishTofuAction(ctx, generation, err, "Initialized "+workspace.Name, func() {
			w.setDetail(formatTofuOperationOutput("INIT COMPLETED", workspace, output), detailTofuWorkspace)
		})
	}()
}

func (w *mainWindow) confirmTofuApply() {
	workspace, found := w.currentTofuWorkspace()
	if !found || w.options.Tofu == nil || w.tofuActionPending || w.tofuPlan == nil || w.tofuPlanFile == "" || len(w.tofuPlan.Changes) == 0 {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Apply reviewed OpenTofu plan")
	dialog.SetMarkup("Apply the reviewed plan to <b>" + html.EscapeString(workspace.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", tofu.FormatPlanSummary(w.tofuPlan)+". The exact saved plan shown in this window will be applied; no new plan will be generated.")
	dialog.SetDestroyWithParent(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Apply plan", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runTofuApply(workspace)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runTofuApply(workspace config.TofuDirEntry) {
	planFile := w.tofuPlanFile
	if vteAvailable() {
		command, err := w.options.Tofu.ApplyCommand(workspace.Dir, planFile)
		if err != nil {
			w.setStatus(err.Error(), true)
			return
		}
		w.startTofuTerminalCommand(workspace, "OpenTofu apply", command, true)
		return
	}
	w.tofuActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Applying reviewed plan for " + workspace.Name + "…")
	go func() {
		output, err := w.options.Tofu.Apply(ctx, workspace.Dir, planFile)
		w.finishTofuAction(ctx, generation, err, "Applied reviewed plan to "+workspace.Name, func() {
			w.discardTofuPlan()
			w.currentPage = pageTofuResources
			w.search.SetPlaceholderText("Filter state resources…")
			w.search.SetText("")
			w.resourceStack.SetVisibleChildName(pageTofuResources)
			w.setBreadcrumb(tofuResourceBreadcrumb(workspace.Name, ""))
			w.setDetail(formatTofuOperationOutput("APPLY COMPLETED", workspace, output), detailTofuWorkspace)
		})
	}()
}

func (w *mainWindow) startTofuTerminalCommand(workspace config.TofuDirEntry, operation string, command tofu.Command, apply bool) {
	if w.terminal == nil || w.tofuActionPending {
		return
	}
	if err := w.terminal.Spawn(command.Executable, command.Args); err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.tofuTerminalGeneration++
	generation := w.tofuTerminalGeneration
	w.tofuActionPending = true
	w.spinner.Start()
	w.setWorkspaceBusy(operation+" is running…", true)
	w.setStatus(operation+" is running for "+workspace.Name+"…", false)
	w.updateActionSensitivity()
	w.showingLogs = false
	w.showingMetrics = false
	w.showingTerminal = true
	w.terminalDescription = operation + " for " + workspace.Name
	w.terminalTitle.SetLabel(operation + " — " + workspace.Name)
	succeeded := false
	w.terminalOnClose = func() {
		if succeeded {
			w.loadTofuResources(workspace)
			return
		}
		w.updateActionSensitivity()
	}
	w.detailStack.SetVisibleChildName("terminal")
	glib.TimeoutAdd(200, func() bool {
		if generation != w.tofuTerminalGeneration || !w.showingTerminal {
			return false
		}
		if w.terminal.Running() {
			return true
		}
		status := w.terminal.ExitStatus()
		w.tofuActionPending = false
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		succeeded = status == 0
		if succeeded {
			if apply {
				w.discardTofuPlan()
			}
			w.terminalTitle.SetLabel(operation + " completed — " + workspace.Name)
			w.setStatus(operation+" completed for "+workspace.Name+"; close the terminal to refresh state", false)
		} else {
			w.terminalTitle.SetLabel(operation + " failed — " + workspace.Name)
			w.setStatus(fmt.Sprintf("%s failed for %s (status %d)", operation, workspace.Name, status), true)
		}
		w.updateActionSensitivity()
		return false
	})
}

func (w *mainWindow) selectTofuPlanChangeRow() {
	if w.currentPage != pageTofuPlan || w.tofuPlan == nil {
		return
	}
	changes := filteredTofuPlanChanges(w.tofuPlan, w.search.Text())
	position := w.tofuPlanTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(changes) {
		w.selectedTofuPlanChange = ""
		workspace, _ := w.currentTofuWorkspace()
		w.setBreadcrumb("OpenTofu / " + workspace.Name + " / Plan")
		w.setDetail(formatTofuPlanSummary(workspace, w.tofuPlan), detailTofuPlan)
		w.updateActionSensitivity()
		return
	}
	change := changes[position]
	w.selectedTofuPlanChange = change.Address
	workspace, _ := w.currentTofuWorkspace()
	w.setBreadcrumb("OpenTofu / " + workspace.Name + " / Plan / " + change.Address)
	w.setDetail(formatTofuPlanChange(change), detailTofuPlan)
	w.updateActionSensitivity()
}

func (w *mainWindow) openTofuPlanChangeAt(position uint) {
	if int(position) < len(filteredTofuPlanChanges(w.tofuPlan, w.search.Text())) {
		w.tofuPlanTable.selection.SetSelected(position)
	}
}

func filteredTofuPlanChanges(plan *tofu.PlanResult, query string) []tofu.ResourceChange {
	if plan == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	changes := make([]tofu.ResourceChange, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		if query == "" || strings.Contains(strings.ToLower(change.Address), query) || strings.Contains(strings.ToLower(change.Action), query) || strings.Contains(strings.ToLower(change.Type), query) {
			changes = append(changes, change)
		}
	}
	return changes
}

func (w *mainWindow) restoreTofuResourcesFromPlan() {
	workspace, found := w.currentTofuWorkspace()
	if !found {
		w.loadTofuWorkspaces()
		return
	}
	w.discardTofuPlan()
	w.currentPage = pageTofuResources
	w.search.SetPlaceholderText("Filter state resources…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTofuResources)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(tofuResourceBreadcrumb(workspace.Name, ""))
	w.applyTofuResourceFilter()
	w.setDetail(tofuResourceListSummary(workspace, len(w.allTofuResources)), detailTofuWorkspace)
	w.updateActionSensitivity()
	w.setStatus("Ready", false)
}

func (w *mainWindow) discardTofuPlan() {
	if w.options.Tofu != nil {
		w.options.Tofu.CleanupPlan(w.tofuPlanFile)
	}
	w.tofuPlanFile = ""
	w.tofuPlan = nil
	w.selectedTofuPlanChange = ""
	if w.tofuPlanTable != nil {
		w.tofuPlanTable.clear()
	}
}

func (w *mainWindow) finishTofuAction(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.requestCancel = nil
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.tofuActionPending = false
		if err != nil {
			w.updateActionSensitivity()
			w.setDetail("ERROR\n\n"+err.Error(), detailError)
			w.setStatus(err.Error(), true)
			return
		}
		if apply != nil {
			apply()
		}
		w.updateActionSensitivity()
		w.setStatus(success, false)
	})
}

func (w *mainWindow) restoreTofuWorkspaceBrowser() {
	name := w.activeSavedTofuWorkspace
	w.loadTofuWorkspaces()
	for index, workspace := range w.filteredTofuWorkspaces {
		if workspace.Name == name {
			w.tofuWorkspaceTable.selection.SetSelected(uint(index))
			break
		}
	}
}

func (w *mainWindow) currentTofuWorkspace() (config.TofuDirEntry, bool) {
	for _, workspace := range w.options.ConfigTofuDirs() {
		if workspace.Dir == w.selectedTofuWorkspace || (w.activeSavedTofuWorkspace != "" && workspace.Name == w.activeSavedTofuWorkspace) {
			return workspace, true
		}
	}
	return config.TofuDirEntry{Name: w.activeSavedTofuWorkspace, Dir: w.selectedTofuWorkspace}, w.selectedTofuWorkspace != ""
}

func (w *mainWindow) refreshTofu(foreground bool) {
	if !foreground {
		return
	}
	if w.tofuActionPending {
		return
	}
	if w.currentPage == pageTofuPlan {
		w.setStatus("Plans are snapshots; run Plan again to refresh", false)
		return
	}
	if w.currentPage == pageTofuResources {
		workspace, found := w.currentTofuWorkspace()
		if found {
			w.loadTofuResources(workspace)
		}
		return
	}
	if foreground && w.options.ReloadConfig != nil && w.options.Config != nil {
		fresh := w.options.ReloadConfig()
		w.options.Config.TofuDirs = append([]config.TofuDirEntry(nil), fresh.TofuDirs...)
		w.rebuildTofuWorkspaceRail()
	}
	w.loadTofuWorkspaces()
}

func (w *mainWindow) rebuildTofuWorkspaceRail() {
	if w.tofuModuleItems == nil {
		return
	}
	if w.savedTofuWorkspaceLabel != nil {
		w.tofuModuleItems.Remove(w.savedTofuWorkspaceLabel)
	}
	for _, button := range w.savedTofuWorkspaceButtons {
		w.tofuModuleItems.Remove(button)
	}
	w.savedTofuWorkspaceLabel = nil
	w.savedTofuWorkspaceButtons = nil
	saved := sortedTofuWorkspaces(w.options.ConfigTofuDirs())
	if len(saved) == 0 {
		return
	}
	w.savedTofuWorkspaceLabel = newDynamoRailLabel("SAVED WORKSPACES")
	w.tofuModuleItems.Append(w.savedTofuWorkspaceLabel)
	for _, workspace := range saved {
		workspace := workspace
		button := newModuleRailButton(workspace.Name, func() { w.openSavedTofuWorkspace(workspace) })
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(workspace.Dir)
		w.tofuModuleItems.Append(button)
		w.savedTofuWorkspaceButtons = append(w.savedTofuWorkspaceButtons, button)
	}
}

func sortedTofuWorkspaces(workspaces []config.TofuDirEntry) []config.TofuDirEntry {
	sorted := append([]config.TofuDirEntry(nil), workspaces...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

func (w *mainWindow) promptAddTofuWorkspace() {
	if w.options.Config == nil {
		return
	}
	dialog, nameEntry := w.newSavedLogNameDialog("Add OpenTofu workspace", "Workspace name", "")
	content := dialog.ContentArea()
	pathLabel := gtk.NewLabel("Directory")
	pathLabel.SetXAlign(0)
	pathEntry := gtk.NewEntry()
	pathEntry.SetPlaceholderText("/path/to/workspace")
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(pathLabel)
	content.Append(pathEntry)
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Add", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name, path := strings.TrimSpace(nameEntry.Text()), strings.TrimSpace(pathEntry.Text())
		if name == "" || path == "" {
			errorLabel.SetLabel("Name and directory are required")
			return
		}
		if _, found := findTofuWorkspace(w.options.Config.TofuDirs, name); found {
			errorLabel.SetLabel("A saved workspace already uses that name")
			return
		}
		if !w.mutateTofuConfig(func(cfg *config.Config) { cfg.AddTofuDir(name, path) }) {
			return
		}
		dialog.Destroy()
		w.rebuildTofuWorkspaceRail()
		w.loadTofuResources(config.TofuDirEntry{Name: name, Dir: path})
	})
	dialog.Present()
}

func (w *mainWindow) promptManageTofuWorkspaces() {
	if w.options.Config == nil || len(w.options.Config.TofuDirs) == 0 {
		return
	}
	saved := append([]config.TofuDirEntry(nil), w.options.Config.TofuDirs...)
	sort.SliceStable(saved, func(i, j int) bool { return strings.ToLower(saved[i].Name) < strings.ToLower(saved[j].Name) })
	labels := make([]string, len(saved))
	for i := range saved {
		labels[i] = saved[i].Name
	}
	dialog := gtk.NewDialogWithFlags("Saved OpenTofu workspaces", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	selector := gtk.NewDropDownFromStrings(labels)
	nameEntry, pathEntry := gtk.NewEntry(), gtk.NewEntry()
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	loadSelected := func() {
		index := int(selector.Selected())
		if index >= 0 && index < len(saved) {
			nameEntry.SetText(saved[index].Name)
			pathEntry.SetText(saved[index].Dir)
		}
	}
	selector.NotifyProperty("selected", loadSelected)
	loadSelected()
	content.Append(selector)
	for _, row := range []struct {
		label string
		entry *gtk.Entry
	}{{"Name", nameEntry}, {"Directory", pathEntry}} {
		label := gtk.NewLabel(row.label)
		label.SetXAlign(0)
		content.Append(label)
		content.Append(row.entry)
	}
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Delete…", 102)
	dialog.AddButton("Apply", 103)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		if index < 0 || index >= len(saved) {
			dialog.Destroy()
			return
		}
		current := saved[index]
		switch response {
		case 101:
			dialog.Destroy()
			w.loadTofuResources(current)
		case 102:
			dialog.Destroy()
			w.confirmDeleteTofuWorkspace(current)
		case 103:
			name, path := strings.TrimSpace(nameEntry.Text()), strings.TrimSpace(pathEntry.Text())
			if name == "" || path == "" {
				errorLabel.SetLabel("Name and directory are required")
				return
			}
			if name != current.Name {
				if _, found := findTofuWorkspace(w.options.Config.TofuDirs, name); found {
					errorLabel.SetLabel("A saved workspace already uses that name")
					return
				}
			}
			if !w.mutateTofuConfig(func(cfg *config.Config) {
				cfg.RemoveTofuDir(current.Name)
				cfg.AddTofuDir(name, path)
			}) {
				return
			}
			dialog.Destroy()
			w.rebuildTofuWorkspaceRail()
			w.loadTofuResources(config.TofuDirEntry{Name: name, Dir: path})
		default:
			dialog.Destroy()
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteTofuWorkspace(workspace config.TofuDirEntry) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved OpenTofu workspace <b>" + html.EscapeString(workspace.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "The workspace directory and its files will not be changed.")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateTofuConfig(func(cfg *config.Config) { cfg.RemoveTofuDir(workspace.Name) }) {
			return
		}
		w.rebuildTofuWorkspaceRail()
		w.loadTofuWorkspaces()
		w.setStatus("Deleted saved OpenTofu workspace "+workspace.Name, false)
	})
	dialog.Present()
}

func (w *mainWindow) mutateTofuConfig(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.TofuDirEntry(nil), w.options.Config.TofuDirs...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.TofuDirs = before
		w.setStatus("Save OpenTofu workspace: "+err.Error(), true)
		return false
	}
	return true
}

func findTofuWorkspace(workspaces []config.TofuDirEntry, name string) (config.TofuDirEntry, bool) {
	for _, workspace := range workspaces {
		if workspace.Name == name {
			return workspace, true
		}
	}
	return config.TofuDirEntry{}, false
}

func tofuWorkspaceListSummary(count int) string {
	if count == 0 {
		return "OPENTOFU WORKSPACES\n\nNo workspaces are configured. Use Add workspace… to save a local OpenTofu or Terraform directory."
	}
	return fmt.Sprintf("OPENTOFU WORKSPACES\n\nWorkspaces  %d\n\nSelect a workspace to inspect it; double-click to browse its state resources.", count)
}

func formatTofuWorkspace(saved config.TofuDirEntry, workspace tofu.Workspace) string {
	initialized := "No — run Init before planning"
	if workspace.Initialized {
		initialized = "Yes"
	}
	return fmt.Sprintf("OPENTOFU WORKSPACE\n\nName         %s\nDirectory    %s\nExecutable   %s\nInitialized  %s", saved.Name, workspace.Dir, workspace.Binary, initialized)
}

func tofuResourceListSummary(workspace config.TofuDirEntry, count int) string {
	return fmt.Sprintf("OPENTOFU STATE\n\nWorkspace  %s\nDirectory  %s\nResources  %d\n\nSelect a resource for its address; double-click to load full state details.", workspace.Name, workspace.Dir, count)
}

func formatTofuResourceSummary(resource tofu.Resource) string {
	return fmt.Sprintf("OPENTOFU RESOURCE\n\nAddress  %s\nType     %s\nName     %s\nModule   %s\n\nDouble-click to load the complete state representation.", resource.Address, resource.Type, resource.Name, valueOrDash(resource.Module))
}

func formatTofuState(resource tofu.Resource, output string) string {
	return fmt.Sprintf("OPENTOFU RESOURCE STATE\n\nAddress  %s\nType     %s\nName     %s\nModule   %s\n\nSTATE\n%s", resource.Address, resource.Type, resource.Name, valueOrDash(resource.Module), strings.TrimSpace(output))
}

func formatTofuPlanSummary(workspace config.TofuDirEntry, plan *tofu.PlanResult) string {
	if plan == nil {
		return "No plan is loaded."
	}
	return fmt.Sprintf("OPENTOFU PLAN\n\nWorkspace  %s\nDirectory  %s\nSummary    %s\nCreate     %d\nUpdate     %d\nReplace    %d\nDelete     %d\nUnchanged  %d\n\nSelect a planned change for its attribute-level diff.", workspace.Name, workspace.Dir, tofu.FormatPlanSummary(plan), plan.CreateCount, plan.UpdateCount, plan.ReplaceCount, plan.DeleteCount, plan.NoOpCount)
}

func formatTofuPlanChange(change tofu.ResourceChange) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "OPENTOFU PLANNED CHANGE\n\nAction   %s\nAddress  %s\nType     %s\nName     %s\nModule   %s\n\nATTRIBUTE CHANGES", strings.ToUpper(change.Action), change.Address, change.Type, change.Name, valueOrDash(change.Module))
	if len(change.Diffs) == 0 {
		builder.WriteString("\nNo attribute-level changes were reported.")
		return builder.String()
	}
	for _, diff := range change.Diffs {
		switch diff.Action {
		case "add":
			fmt.Fprintf(&builder, "\n\n+ %s\n  %s", diff.Path, diff.After)
		case "remove":
			fmt.Fprintf(&builder, "\n\n- %s\n  %s", diff.Path, diff.Before)
		default:
			fmt.Fprintf(&builder, "\n\n~ %s\n  before: %s\n  after:  %s", diff.Path, diff.Before, diff.After)
		}
	}
	return builder.String()
}

func formatTofuOperationOutput(title string, workspace config.TofuDirEntry, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		output = "Command completed without output."
	}
	return fmt.Sprintf("%s\n\nWorkspace  %s\nDirectory  %s\n\nOUTPUT\n%s", title, workspace.Name, workspace.Dir, output)
}

func tofuResourceBreadcrumb(workspace, resource string) string {
	breadcrumb := "OpenTofu / " + workspace + " / State"
	if resource != "" {
		breadcrumb += " / " + resource
	}
	return breadcrumb
}
