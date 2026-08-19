//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func (o Options) ConfigLambdaSearches() []config.LambdaSearch {
	if o.Config == nil {
		return nil
	}
	return o.Config.LambdaSearches
}

func (w *mainWindow) openLambdaModule() {
	if w.currentPage == pageLambda && w.activeSavedLambdaSearch == "" && w.lambdaSearchTerm == "" {
		return
	}
	w.loadLambdaFunctions("", "")
}

func (w *mainWindow) loadLambdaFunctions(searchTerm, savedName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearLambdaBrowser()
	w.currentPage = pageLambda
	w.lambdaSearchTerm = strings.TrimSpace(searchTerm)
	w.activeSavedLambdaSearch = savedName
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb(lambdaBreadcrumb(savedName, w.lambdaSearchTerm, ""))
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter loaded functions…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageLambda)
	w.setDetail("Loading Lambda functions…", detailIntro)

	if w.options.Lambda == nil {
		w.setDetail("Lambda is unavailable because no Lambda service was configured.", detailError)
		w.setStatus("Lambda service unavailable", true)
		return
	}

	label := "Loading Lambda functions…"
	if w.lambdaSearchTerm != "" {
		label = "Loading Lambda functions matching " + w.lambdaSearchTerm + "…"
	}
	search := w.lambdaSearchTerm
	ctx, generation := w.startRequest(label)
	go func() {
		functions, err := w.options.Lambda.List(ctx, search)
		w.finishRequest(ctx, generation, err, func() {
			w.allLambdaFunctions = functions
			w.applyLambdaFilter()
			w.setDetail(lambdaListSummary(search, len(functions)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshLambda(foreground bool) {
	if w.options.Lambda == nil {
		return
	}
	search, selected := w.lambdaSearchTerm, w.selectedLambdaFunction
	ctx, generation := w.startRefreshRequest("Refreshing Lambda functions…", foreground)
	go func() {
		functions, err := w.options.Lambda.List(ctx, search)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allLambdaFunctions = functions
			w.applyLambdaFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(lambdaListSummary(search, len(functions)), detailIntro)
				}
				return
			}
			function, found := findLambdaFunction(functions, selected)
			if !found {
				w.selectedLambdaFunction = ""
				w.setBreadcrumb(lambdaBreadcrumb(w.activeSavedLambdaSearch, search, ""))
				w.setDetail("The selected Lambda function is no longer available.\n\n"+lambdaListSummary(search, len(functions)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			if w.detailContent == detailLambda {
				w.setDetail(formatLambdaSummary(function), detailLambda)
			}
		})
	}()
}

func (w *mainWindow) clearLambdaBrowser() {
	w.allLambdaFunctions = nil
	w.filteredLambdaFunctions = nil
	w.selectedLambdaFunction = ""
	if w.lambdaTable != nil {
		w.lambdaTable.clear()
	}
}

func (w *mainWindow) applyLambdaFilter() {
	w.filteredLambdaFunctions = filterLambdaFunctions(w.allLambdaFunctions, w.search.Text())
	rows := make([]string, len(w.filteredLambdaFunctions))
	for i, function := range w.filteredLambdaFunctions {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%d MB\t%ds\t%s", function.Name,
			valueOrDash(function.Runtime), valueOrDash(function.State), function.MemoryMB,
			function.TimeoutSec, formatTime(function.LastModified))
	}
	w.lambdaTable.replace(rows)
}

func (w *mainWindow) selectLambdaFunctionRow() {
	if w.currentPage != pageLambda {
		return
	}
	position := w.lambdaTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredLambdaFunctions) {
		if w.selectedLambdaFunction != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedLambdaFunction = ""
			w.setBreadcrumb(lambdaBreadcrumb(w.activeSavedLambdaSearch, w.lambdaSearchTerm, ""))
			w.setDetail(lambdaListSummary(w.lambdaSearchTerm, len(w.allLambdaFunctions)), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	function := w.filteredLambdaFunctions[position]
	if w.selectedLambdaFunction != function.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedLambdaFunction = function.Name
	w.setBreadcrumb(lambdaBreadcrumb(w.activeSavedLambdaSearch, w.lambdaSearchTerm, function.Name))
	w.setDetail(formatLambdaSummary(function), detailLambda)
	w.updateActionSensitivity()
}

func (w *mainWindow) openLambdaFunctionAt(position uint) {
	if int(position) < len(w.filteredLambdaFunctions) {
		w.lambdaTable.selection.SetSelected(position)
	}
}

func filterLambdaFunctions(functions []model.LambdaFunction, query string) []model.LambdaFunction {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.LambdaFunction(nil), functions...)
	}
	filtered := make([]model.LambdaFunction, 0, len(functions))
	for _, function := range functions {
		haystack := strings.ToLower(strings.Join([]string{
			function.Name, function.Description, function.ARN, function.Runtime,
			function.State, function.Handler, function.PackageType,
		}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, function)
		}
	}
	return filtered
}

func findLambdaFunction(functions []model.LambdaFunction, name string) (model.LambdaFunction, bool) {
	for _, function := range functions {
		if function.Name == name {
			return function, true
		}
	}
	return model.LambdaFunction{}, false
}

func lambdaBreadcrumb(savedName, search, function string) string {
	root := "Functions"
	if search != "" {
		root += " (" + search + ")"
	}
	if savedName != "" {
		root = savedName + " (" + search + ")"
	}
	crumb := "Lambda / " + root
	if function != "" {
		crumb += " / " + function
	}
	return crumb
}

func lambdaListSummary(search string, count int) string {
	scope := "All functions"
	if search != "" {
		scope = "Name or description contains " + search
	}
	if count == 0 {
		return "No Lambda functions found.\n\nScope: " + scope
	}
	return fmt.Sprintf("LAMBDA\n\nScope     %s\nFunctions %d\n\nSelect a function to inspect its configuration.", scope, count)
}

func formatLambdaSummary(function model.LambdaFunction) string {
	return fmt.Sprintf("LAMBDA FUNCTION\n\nName          %s\nState         %s\nRuntime       %s\nHandler       %s\nPackage       %s\nMemory        %d MB\nTimeout       %ds\nCode size     %s\nLast modified %s\nLog group     %s\nEnvironment   %d variables\nARN           %s\n\nDescription\n%s",
		function.Name, valueOrDash(function.State), valueOrDash(function.Runtime), valueOrDash(function.Handler),
		valueOrDash(function.PackageType), function.MemoryMB, function.TimeoutSec, formatLambdaBytes(function.CodeSize),
		formatTime(function.LastModified), valueOrDash(function.LogGroup), len(function.EnvVars), valueOrDash(function.ARN),
		valueOrDash(function.Description))
}

func formatLambdaBytes(size int64) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func (w *mainWindow) rebuildLambdaSearchRail() {
	if w.lambdaModuleItems == nil {
		return
	}
	if w.savedLambdaSearchesLabel != nil {
		w.lambdaModuleItems.Remove(w.savedLambdaSearchesLabel)
	}
	for _, button := range w.savedLambdaSearchButtons {
		w.lambdaModuleItems.Remove(button)
	}
	w.savedLambdaSearchesLabel = nil
	w.savedLambdaSearchButtons = nil
	searches := w.options.ConfigLambdaSearches()
	if len(searches) == 0 {
		return
	}
	w.savedLambdaSearchesLabel = gtk.NewLabel("SAVED SEARCHES")
	w.savedLambdaSearchesLabel.SetXAlign(0)
	w.savedLambdaSearchesLabel.AddCSSClass("section-title")
	w.lambdaModuleItems.Append(w.savedLambdaSearchesLabel)
	for _, search := range searches {
		search := search
		button := newModuleRailButton(search.Name, func() { w.loadLambdaFunctions(search.Filter, search.Name) })
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(search.Filter)
		w.lambdaModuleItems.Append(button)
		w.savedLambdaSearchButtons = append(w.savedLambdaSearchButtons, button)
	}
}

func (w *mainWindow) reloadLambdaSearchConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.LambdaSearches = append([]config.LambdaSearch(nil), fresh.LambdaSearches...)
	w.rebuildLambdaSearchRail()
	if w.activeSavedLambdaSearch == "" {
		w.updateActionSensitivity()
		return true
	}
	for _, search := range w.options.Config.LambdaSearches {
		if search.Name == w.activeSavedLambdaSearch {
			w.lambdaSearchTerm = search.Filter
			w.updateActionSensitivity()
			return true
		}
	}
	w.loadLambdaFunctions("", "")
	w.setStatus("The active saved Lambda search was removed from the configuration", false)
	return false
}

func (w *mainWindow) promptLambdaSearch() {
	dialog, entry := w.newSavedLogNameDialog("Search Lambda functions", "Name or description contains", w.lambdaSearchTerm)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.loadLambdaFunctions(strings.TrimSpace(entry.Text()), "")
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptSaveLambdaSearch() {
	if w.options.Config == nil {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save Lambda search", "Search name", "")
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	dialog.ContentArea().Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			errorLabel.SetLabel("Enter a name")
			return
		}
		for _, search := range w.options.Config.LambdaSearches {
			if search.Name == name {
				errorLabel.SetLabel("A saved Lambda search already uses that name")
				return
			}
		}
		if !w.mutateLambdaSearches(func(cfg *config.Config) { cfg.AddLambdaSearch(name, w.lambdaSearchTerm) }) {
			return
		}
		dialog.Destroy()
		w.activeSavedLambdaSearch = name
		w.rebuildLambdaSearchRail()
		w.updateActionSensitivity()
		w.setBreadcrumb(lambdaBreadcrumb(name, w.lambdaSearchTerm, w.selectedLambdaFunction))
		w.setStatus("Saved Lambda search "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageLambdaSearches() {
	searches := w.options.ConfigLambdaSearches()
	if len(searches) == 0 {
		return
	}
	names := make([]string, len(searches))
	selected := 0
	for i, search := range searches {
		names[i] = search.Name
		if search.Name == w.activeSavedLambdaSearch {
			selected = i
		}
	}
	dialog := gtk.NewDialogWithFlags("Saved Lambda searches", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	selector := gtk.NewDropDownFromStrings(names)
	selector.SetSelected(uint(selected))
	selector.SetHExpand(true)
	detail := gtk.NewLabel("")
	detail.SetXAlign(0)
	detail.AddCSSClass("muted")
	updateDetail := func() {
		index := int(selector.Selected())
		if index >= 0 && index < len(searches) {
			detail.SetLabel("Name or description contains: " + valueOrDash(searches[index].Filter))
		}
	}
	selector.NotifyProperty("selected", updateDetail)
	updateDetail()
	content.Append(selector)
	content.Append(detail)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Delete…", 102)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		dialog.Destroy()
		if index < 0 || index >= len(searches) {
			return
		}
		switch response {
		case 101:
			w.loadLambdaFunctions(searches[index].Filter, searches[index].Name)
		case 102:
			w.confirmDeleteLambdaSearch(searches[index].Name)
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteLambdaSearch(name string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved Lambda search <b>" + html.EscapeString(name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateLambdaSearches(func(cfg *config.Config) { cfg.RemoveLambdaSearch(name) }) {
			return
		}
		wasActive := w.activeSavedLambdaSearch == name
		w.rebuildLambdaSearchRail()
		if wasActive {
			w.loadLambdaFunctions("", "")
		} else {
			w.updateActionSensitivity()
		}
	})
	dialog.Present()
}

func (w *mainWindow) mutateLambdaSearches(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.LambdaSearch(nil), w.options.Config.LambdaSearches...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.LambdaSearches = before
		w.setStatus("Save Lambda search: "+err.Error(), true)
		return false
	}
	return true
}
