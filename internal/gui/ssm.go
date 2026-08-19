//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (o Options) ConfigSSMPrefixes() []config.SSMPrefix {
	if o.Config == nil {
		return nil
	}
	return o.Config.SSMPrefixes
}

func (w *mainWindow) openSSMModule() {
	if w.currentPage == pageSSM && w.activeSSMPrefix == "" && w.ssmPath == "/" {
		return
	}
	w.loadSSMPath("/", "")
}

func (w *mainWindow) loadSSMPath(path, savedName string) {
	normalized, err := service.NormalizeParameterPath(path)
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearSSMBrowser()
	w.currentPage = pageSSM
	w.ssmPath = normalized
	w.activeSSMPrefix = savedName
	w.selectedSSMParameter = ""
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb(ssmBreadcrumb(savedName, normalized, ""))
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter parameters…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSSM)
	w.setDetail("Loading SSM parameters…", detailIntro)

	if w.options.SSM == nil {
		w.setDetail("SSM Parameter Store is unavailable because no SSM service was configured.", detailError)
		w.setStatus("SSM Parameter Store service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading SSM parameters under " + normalized + "…")
	go func() {
		parameters, listErr := w.options.SSM.List(ctx, normalized)
		w.finishRequest(ctx, generation, listErr, func() {
			w.allSSMParameters = parameters
			w.applySSMFilter()
			w.setDetail(ssmListSummary(normalized, len(parameters)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshSSM(foreground bool) {
	if w.options.SSM == nil || w.ssmPath == "" {
		return
	}
	path, selected := w.ssmPath, w.selectedSSMParameter
	ctx, generation := w.startRefreshRequest("Refreshing SSM parameters under "+path+"…", foreground)
	go func() {
		parameters, err := w.options.SSM.List(ctx, path)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allSSMParameters = parameters
			w.applySSMFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(ssmListSummary(path, len(parameters)), detailIntro)
				}
				return
			}
			parameter, found := findSSMParameter(parameters, selected)
			if !found {
				w.selectedSSMParameter = ""
				w.setBreadcrumb(ssmBreadcrumb(w.activeSSMPrefix, path, ""))
				w.setDetail("The selected parameter is no longer available.\n\n"+ssmListSummary(path, len(parameters)), detailIntro)
				return
			}
			if w.detailContent == detailSSM {
				w.setDetail(formatSSMParameterSummary(parameter), detailSSM)
			}
		})
	}()
}

func (w *mainWindow) clearSSMBrowser() {
	w.allSSMParameters = nil
	w.filteredSSMParameters = nil
	w.selectedSSMParameter = ""
	if w.ssmTable != nil {
		w.ssmTable.clear()
	}
}

func (w *mainWindow) applySSMFilter() {
	w.filteredSSMParameters = filterSSMParameters(w.allSSMParameters, w.search.Text())
	rows := make([]string, len(w.filteredSSMParameters))
	for i, parameter := range w.filteredSSMParameters {
		rows[i] = fmt.Sprintf("%s\t%s\t%d\t%s\t%s", parameter.Name, parameter.Type,
			parameter.Version, ssmListValue(parameter), formatTime(parameter.LastModified))
	}
	w.ssmTable.replace(rows)
}

func (w *mainWindow) selectSSMParameterRow() {
	if w.currentPage != pageSSM {
		return
	}
	position := w.ssmTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredSSMParameters) {
		return
	}
	parameter := w.filteredSSMParameters[position]
	w.selectedSSMParameter = parameter.Name
	w.setBreadcrumb(ssmBreadcrumb(w.activeSSMPrefix, w.ssmPath, parameter.Name))
	w.setDetail(formatSSMParameterSummary(parameter), detailSSM)
	w.updateActionSensitivity()
}

func (w *mainWindow) openSSMParameterAt(position uint) {
	if int(position) >= len(w.filteredSSMParameters) {
		return
	}
	w.ssmTable.selection.SetSelected(position)
}

func filterSSMParameters(parameters []model.Parameter, query string) []model.Parameter {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Parameter(nil), parameters...)
	}
	filtered := make([]model.Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		haystack := strings.ToLower(parameter.Name + " " + parameter.Type)
		if parameter.Type != "SecureString" {
			haystack += " " + strings.ToLower(parameter.Value)
		}
		if strings.Contains(haystack, query) {
			filtered = append(filtered, parameter)
		}
	}
	return filtered
}

func findSSMParameter(parameters []model.Parameter, name string) (model.Parameter, bool) {
	for _, parameter := range parameters {
		if parameter.Name == name {
			return parameter, true
		}
	}
	return model.Parameter{}, false
}

func ssmListValue(parameter model.Parameter) string {
	if parameter.Type == "SecureString" {
		return "••••••••"
	}
	return compactStatusMessage(parameter.Value)
}

func ssmBreadcrumb(savedName, path, parameter string) string {
	root := path
	if savedName != "" {
		root = savedName + " (" + path + ")"
	}
	crumb := "SSM Parameter Store / " + root
	if parameter != "" {
		crumb += " / " + parameter
	}
	return crumb
}

func ssmListSummary(path string, count int) string {
	if count == 0 {
		return "No SSM parameters found under " + path + "."
	}
	return fmt.Sprintf("SSM PARAMETER STORE\n\nPath       %s\nParameters %d\n\nSelect a parameter to inspect its metadata.", path, count)
}

func formatSSMParameterSummary(parameter model.Parameter) string {
	return fmt.Sprintf("SSM PARAMETER\n\nName       %s\nType       %s\nVersion    %d\nModified   %s\n\nValue\n%s",
		parameter.Name, parameter.Type, parameter.Version, formatTime(parameter.LastModified), ssmListValue(parameter))
}

func (w *mainWindow) rebuildSSMPrefixRail() {
	if w.ssmModuleItems == nil {
		return
	}
	if w.savedSSMPrefixLabel != nil {
		w.ssmModuleItems.Remove(w.savedSSMPrefixLabel)
	}
	for _, button := range w.savedSSMPrefixButtons {
		w.ssmModuleItems.Remove(button)
	}
	w.savedSSMPrefixLabel = nil
	w.savedSSMPrefixButtons = nil
	prefixes := w.options.ConfigSSMPrefixes()
	if len(prefixes) == 0 {
		return
	}
	w.savedSSMPrefixLabel = gtk.NewLabel("SAVED PREFIXES")
	w.savedSSMPrefixLabel.SetXAlign(0)
	w.savedSSMPrefixLabel.AddCSSClass("section-title")
	w.ssmModuleItems.Append(w.savedSSMPrefixLabel)
	for _, prefix := range prefixes {
		prefix := prefix
		button := newModuleRailButton(prefix.Name, func() { w.loadSSMPath(prefix.Prefix, prefix.Name) })
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(prefix.Prefix)
		w.ssmModuleItems.Append(button)
		w.savedSSMPrefixButtons = append(w.savedSSMPrefixButtons, button)
	}
}

func (w *mainWindow) reloadSSMPrefixConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.SSMPrefixes = append([]config.SSMPrefix(nil), fresh.SSMPrefixes...)
	w.rebuildSSMPrefixRail()
	if w.activeSSMPrefix == "" {
		w.updateActionSensitivity()
		return true
	}
	for _, prefix := range w.options.Config.SSMPrefixes {
		if prefix.Name == w.activeSSMPrefix {
			w.ssmPath = prefix.Prefix
			w.updateActionSensitivity()
			return true
		}
	}
	w.loadSSMPath("/", "")
	w.setStatus("The active saved SSM prefix was removed from the configuration", false)
	return false
}

func (w *mainWindow) promptSSMPath() {
	dialog, entry := w.newSavedLogNameDialog("Browse SSM path", "Parameter path", w.ssmPath)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	dialog.ContentArea().Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Browse", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		path, err := service.NormalizeParameterPath(entry.Text())
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		dialog.Destroy()
		w.loadSSMPath(path, "")
	})
	dialog.Present()
}

func (w *mainWindow) promptSaveSSMPrefix() {
	if w.options.Config == nil || w.ssmPath == "" {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save SSM prefix", "Prefix name", "")
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
		for _, prefix := range w.options.Config.SSMPrefixes {
			if prefix.Name == name {
				errorLabel.SetLabel("A saved SSM prefix already uses that name")
				return
			}
		}
		if !w.mutateSSMPrefixes(func(cfg *config.Config) { cfg.AddSSMPrefix(name, w.ssmPath) }) {
			return
		}
		dialog.Destroy()
		w.activeSSMPrefix = name
		w.rebuildSSMPrefixRail()
		w.updateActionSensitivity()
		w.setBreadcrumb(ssmBreadcrumb(name, w.ssmPath, w.selectedSSMParameter))
		w.setStatus("Saved SSM prefix "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageSSMPrefixes() {
	prefixes := w.options.ConfigSSMPrefixes()
	if len(prefixes) == 0 {
		return
	}
	names := make([]string, len(prefixes))
	selected := 0
	for i, prefix := range prefixes {
		names[i] = prefix.Name
		if prefix.Name == w.activeSSMPrefix {
			selected = i
		}
	}
	dialog := gtk.NewDialogWithFlags("Saved SSM prefixes", &w.window.Window, gtk.DialogModal)
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
		if index >= 0 && index < len(prefixes) {
			detail.SetLabel(prefixes[index].Prefix)
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
		if index < 0 || index >= len(prefixes) {
			return
		}
		switch response {
		case 101:
			w.loadSSMPath(prefixes[index].Prefix, prefixes[index].Name)
		case 102:
			w.confirmDeleteSSMPrefix(prefixes[index].Name)
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteSSMPrefix(name string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved SSM prefix <b>" + html.EscapeString(name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateSSMPrefixes(func(cfg *config.Config) { cfg.RemoveSSMPrefix(name) }) {
			return
		}
		wasActive := w.activeSSMPrefix == name
		w.rebuildSSMPrefixRail()
		if wasActive {
			w.loadSSMPath("/", "")
		} else {
			w.updateActionSensitivity()
		}
	})
	dialog.Present()
}

func (w *mainWindow) mutateSSMPrefixes(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.SSMPrefix(nil), w.options.Config.SSMPrefixes...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.SSMPrefixes = before
		w.setStatus("Save SSM prefix: "+err.Error(), true)
		return false
	}
	return true
}
