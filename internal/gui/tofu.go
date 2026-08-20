//go:build gui

package gui

import (
	"fmt"
	"html"
	"sort"
	"strings"

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
	if int(position) < len(w.filteredTofuResources) {
		w.tofuResourceTable.selection.SetSelected(position)
	}
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
	if w.tofuActionPending {
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

func tofuResourceBreadcrumb(workspace, resource string) string {
	breadcrumb := "OpenTofu / " + workspace + " / State"
	if resource != "" {
		breadcrumb += " / " + resource
	}
	return breadcrumb
}
