//go:build gui

package gui

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func (o Options) ConfigSecretFilters() []config.SMFilter {
	if o.Config == nil {
		return nil
	}
	return o.Config.SMFilters
}

func (w *mainWindow) openSecretsModule() {
	if w.currentPage == pageSecrets && w.activeSavedSecretFilter == "" && w.secretNameFilter == "" {
		return
	}
	if w.guardEditorNavigation(w.openSecretsModule) {
		return
	}
	w.loadSecrets("", "")
}

func (w *mainWindow) loadSecrets(nameFilter, savedName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearSecretBrowser()
	w.currentPage = pageSecrets
	w.secretNameFilter = strings.TrimSpace(nameFilter)
	w.activeSavedSecretFilter = savedName
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb(secretBreadcrumb(savedName, w.secretNameFilter, ""))
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter loaded secrets…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSecrets)
	w.setDetail("Loading Secrets Manager metadata…", detailIntro)

	if w.options.Secrets == nil {
		w.setDetail("Secrets Manager is unavailable because no Secrets Manager service was configured.", detailError)
		w.setStatus("Secrets Manager service unavailable", true)
		return
	}

	label := "Loading Secrets Manager secrets…"
	if w.secretNameFilter != "" {
		label = "Loading Secrets Manager secrets matching " + w.secretNameFilter + "…"
	}
	filter := w.secretNameFilter
	ctx, generation := w.startRequest(label)
	go func() {
		secrets, err := w.options.Secrets.List(ctx, filter)
		w.finishRequest(ctx, generation, err, func() {
			w.allSecrets = secrets
			w.applySecretFilter()
			w.setDetail(secretListSummary(filter, len(secrets)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshSecrets(foreground bool) {
	if w.options.Secrets == nil {
		return
	}
	filter, selected := w.secretNameFilter, w.selectedSecret
	previous, previousFound := findSecret(w.allSecrets, selected)
	ctx, generation := w.startRefreshRequest("Refreshing Secrets Manager metadata…", foreground)
	go func() {
		secrets, err := w.options.Secrets.List(ctx, filter)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allSecrets = secrets
			w.applySecretFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(secretListSummary(filter, len(secrets)), detailIntro)
				}
				return
			}
			secret, found := findSecret(secrets, selected)
			if !found {
				w.selectedSecret = ""
				w.setBreadcrumb(secretBreadcrumb(w.activeSavedSecretFilter, filter, ""))
				w.setDetail("The selected secret is no longer available.\n\n"+secretListSummary(filter, len(secrets)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			if w.detailContent == detailSecret {
				if previousFound && previous.LastChanged.Equal(secret.LastChanged) && w.secretDetail != nil && w.secretDetail.Name == secret.Name {
					w.setDetail(formatSecretValueDetail(secret, *w.secretDetail), detailSecret)
				} else {
					w.secretDetail = nil
					w.setDetail(formatSecretSummary(secret), detailSecret)
				}
			}
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearSecretBrowser() {
	w.allSecrets = nil
	w.filteredSecrets = nil
	w.selectedSecret = ""
	if w.secretTable != nil {
		w.secretTable.clear()
	}
}

func (w *mainWindow) applySecretFilter() {
	w.filteredSecrets = filterSecrets(w.allSecrets, w.search.Text())
	rows := make([]string, len(w.filteredSecrets))
	for i, secret := range w.filteredSecrets {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s", secret.Name, compactStatusMessage(secret.Description),
			formatTime(secret.LastChanged), formatTime(secret.LastAccessed))
	}
	w.secretTable.replace(rows)
}

func (w *mainWindow) selectSecretRow() {
	if w.currentPage != pageSecrets {
		return
	}
	position := w.secretTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredSecrets) {
		if w.selectedSecret != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedSecret = ""
			w.setBreadcrumb(secretBreadcrumb(w.activeSavedSecretFilter, w.secretNameFilter, ""))
			w.setDetail(secretListSummary(w.secretNameFilter, len(w.allSecrets)), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	secret := w.filteredSecrets[position]
	if w.selectedSecret != secret.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedSecret = secret.Name
	w.setBreadcrumb(secretBreadcrumb(w.activeSavedSecretFilter, w.secretNameFilter, secret.Name))
	w.setDetail(formatSecretSummary(secret), detailSecret)
	w.updateActionSensitivity()
}

func (w *mainWindow) openSecretAt(position uint) {
	if int(position) < len(w.filteredSecrets) {
		w.secretTable.selection.SetSelected(position)
		w.promptRevealSecret()
	}
}

func filterSecrets(secrets []model.Secret, query string) []model.Secret {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Secret(nil), secrets...)
	}
	filtered := make([]model.Secret, 0, len(secrets))
	for _, secret := range secrets {
		haystack := strings.ToLower(secret.Name + " " + secret.Description + " " + secret.ARN)
		for key, value := range secret.Tags {
			haystack += " " + strings.ToLower(key+" "+value)
		}
		if strings.Contains(haystack, query) {
			filtered = append(filtered, secret)
		}
	}
	return filtered
}

func findSecret(secrets []model.Secret, name string) (model.Secret, bool) {
	for _, secret := range secrets {
		if secret.Name == name {
			return secret, true
		}
	}
	return model.Secret{}, false
}

func secretBreadcrumb(savedName, filter, secret string) string {
	root := "Secrets"
	if filter != "" {
		root += " (" + filter + ")"
	}
	if savedName != "" {
		root = savedName + " (" + filter + ")"
	}
	crumb := "Secrets Manager / " + root
	if secret != "" {
		crumb += " / " + secret
	}
	return crumb
}

func secretListSummary(filter string, count int) string {
	scope := "All secrets"
	if filter != "" {
		scope = "Name contains " + filter
	}
	if count == 0 {
		return "No Secrets Manager secrets found.\n\nScope: " + scope
	}
	return fmt.Sprintf("SECRETS MANAGER\n\nScope   %s\nSecrets %d\n\nSelect a secret to inspect its metadata. Values are loaded only through an explicit reveal action.", scope, count)
}

func formatSecretSummary(secret model.Secret) string {
	return fmt.Sprintf("SECRET METADATA\n\nName          %s\nARN           %s\nDescription   %s\nLast changed  %s\nLast accessed %s\n\nTags\n%s\n\nValue\n••••••••",
		secret.Name, valueOrDash(secret.ARN), valueOrDash(secret.Description), formatTime(secret.LastChanged),
		formatTime(secret.LastAccessed), formatSecretTags(secret.Tags))
}

func formatSecretTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, key := range keys {
		lines[i] = key + " = " + tags[key]
	}
	return strings.Join(lines, "\n")
}

func (w *mainWindow) rebuildSecretFilterRail() {
	if w.secretsModuleItems == nil {
		return
	}
	if w.savedSecretFiltersLabel != nil {
		w.secretsModuleItems.Remove(w.savedSecretFiltersLabel)
	}
	for _, button := range w.savedSecretFilterButtons {
		w.secretsModuleItems.Remove(button)
	}
	w.savedSecretFiltersLabel = nil
	w.savedSecretFilterButtons = nil
	filters := w.options.ConfigSecretFilters()
	if len(filters) == 0 {
		return
	}
	w.savedSecretFiltersLabel = gtk.NewLabel("SAVED FILTERS")
	w.savedSecretFiltersLabel.SetXAlign(0)
	w.savedSecretFiltersLabel.AddCSSClass("section-title")
	w.secretsModuleItems.Append(w.savedSecretFiltersLabel)
	for _, filter := range filters {
		filter := filter
		button := newModuleRailButton(filter.Name, func() {
			if !w.guardEditorNavigation(func() { w.loadSecrets(filter.Filter, filter.Name) }) {
				w.loadSecrets(filter.Filter, filter.Name)
			}
		})
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(filter.Filter)
		w.secretsModuleItems.Append(button)
		w.savedSecretFilterButtons = append(w.savedSecretFilterButtons, button)
	}
}

func (w *mainWindow) reloadSecretFilterConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.SMFilters = append([]config.SMFilter(nil), fresh.SMFilters...)
	w.rebuildSecretFilterRail()
	if w.activeSavedSecretFilter == "" {
		w.updateActionSensitivity()
		return true
	}
	for _, filter := range w.options.Config.SMFilters {
		if filter.Name == w.activeSavedSecretFilter {
			w.secretNameFilter = filter.Filter
			w.updateActionSensitivity()
			return true
		}
	}
	w.loadSecrets("", "")
	w.setStatus("The active saved Secrets Manager filter was removed from the configuration", false)
	return false
}

func (w *mainWindow) promptSecretFilter() {
	dialog, entry := w.newSavedLogNameDialog("Filter Secrets Manager", "Secret name contains", w.secretNameFilter)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.loadSecrets(strings.TrimSpace(entry.Text()), "")
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptSaveSecretFilter() {
	if w.options.Config == nil {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save Secrets Manager filter", "Filter name", "")
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
		for _, filter := range w.options.Config.SMFilters {
			if filter.Name == name {
				errorLabel.SetLabel("A saved Secrets Manager filter already uses that name")
				return
			}
		}
		if !w.mutateSecretFilters(func(cfg *config.Config) { cfg.AddSMFilter(name, w.secretNameFilter) }) {
			return
		}
		dialog.Destroy()
		w.activeSavedSecretFilter = name
		w.rebuildSecretFilterRail()
		w.updateActionSensitivity()
		w.setBreadcrumb(secretBreadcrumb(name, w.secretNameFilter, w.selectedSecret))
		w.setStatus("Saved Secrets Manager filter "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageSecretFilters() {
	filters := w.options.ConfigSecretFilters()
	if len(filters) == 0 {
		return
	}
	names := make([]string, len(filters))
	selected := 0
	for i, filter := range filters {
		names[i] = filter.Name
		if filter.Name == w.activeSavedSecretFilter {
			selected = i
		}
	}
	dialog := gtk.NewDialogWithFlags("Saved Secrets Manager filters", &w.window.Window, gtk.DialogModal)
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
		if index >= 0 && index < len(filters) {
			detail.SetLabel("Name contains: " + valueOrDash(filters[index].Filter))
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
		if index < 0 || index >= len(filters) {
			return
		}
		switch response {
		case 101:
			w.loadSecrets(filters[index].Filter, filters[index].Name)
		case 102:
			w.confirmDeleteSecretFilter(filters[index].Name)
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteSecretFilter(name string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved Secrets Manager filter <b>" + html.EscapeString(name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateSecretFilters(func(cfg *config.Config) { cfg.RemoveSMFilter(name) }) {
			return
		}
		wasActive := w.activeSavedSecretFilter == name
		w.rebuildSecretFilterRail()
		if wasActive {
			w.loadSecrets("", "")
		} else {
			w.updateActionSensitivity()
		}
	})
	dialog.Present()
}

func (w *mainWindow) mutateSecretFilters(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.SMFilter(nil), w.options.Config.SMFilters...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.SMFilters = before
		w.setStatus("Save Secrets Manager filter: "+err.Error(), true)
		return false
	}
	return true
}

func (w *mainWindow) promptRevealSecret() {
	secret, found := findSecret(w.allSecrets, w.selectedSecret)
	if !found || w.secretActionPending || w.options.Secrets == nil {
		return
	}
	if w.secretDetail != nil && w.secretDetail.Name == secret.Name {
		w.setDetail(formatSecretValueDetail(secret, *w.secretDetail), detailSecret)
		return
	}
	w.confirmSecretValueRead(secret, "Reveal secret value",
		"Reveal the current value of <b>"+html.EscapeString(secret.Name)+"</b>?",
		"The plaintext value will remain visible in the Workspace Pane until you change context or select another secret.",
		func(value *model.SecretValue) {
			w.setDetail(formatSecretValueDetail(secret, *value), detailSecret)
		})
}

func (w *mainWindow) editSelectedSecret() {
	secret, found := findSecret(w.allSecrets, w.selectedSecret)
	if !found || w.secretActionPending || w.options.Secrets == nil {
		return
	}
	if w.secretDetail != nil && w.secretDetail.Name == secret.Name {
		w.showSecretValueEditor(secret, w.secretDetail)
		return
	}
	w.confirmSecretValueRead(secret, "Edit secret",
		"Read the current value of <b>"+html.EscapeString(secret.Name)+"</b> for editing?",
		"The plaintext value will be placed in an embedded editor. Saving requires a second confirmation.",
		func(value *model.SecretValue) { w.showSecretValueEditor(secret, value) })
}

func (w *mainWindow) cloneSelectedSecret() {
	secret, found := findSecret(w.allSecrets, w.selectedSecret)
	if !found || w.secretActionPending || w.options.Secrets == nil {
		return
	}
	if w.secretDetail != nil && w.secretDetail.Name == secret.Name {
		w.showCloneSecretEditor(secret, w.secretDetail)
		return
	}
	w.confirmSecretValueRead(secret, "Clone secret",
		"Read the current value of <b>"+html.EscapeString(secret.Name)+"</b> for cloning?",
		"The plaintext value will be placed in an embedded editor. Creating the clone requires a second confirmation.",
		func(value *model.SecretValue) { w.showCloneSecretEditor(secret, value) })
}

func (w *mainWindow) confirmSecretValueRead(secret model.Secret, title, markup, secondary string, apply func(*model.SecretValue)) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle(title)
	dialog.SetMarkup(markup)
	dialog.SetObjectProperty("secondary-text", secondary)
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.loadSecretValue(secret.Name, apply)
		}
	})
	dialog.Present()
}

func (w *mainWindow) loadSecretValue(name string, apply func(*model.SecretValue)) {
	if name == "" || w.options.Secrets == nil || w.secretActionPending {
		return
	}
	w.secretActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Reading Secrets Manager secret " + name + "…")
	go func() {
		value, err := w.options.Secrets.Detail(ctx, name)
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.secretActionPending = false
			if err != nil {
				w.updateActionSensitivity()
				w.setStatus(err.Error(), true)
				return
			}
			if w.currentPage != pageSecrets || w.selectedSecret != name {
				return
			}
			w.secretDetail = value
			apply(value)
			w.updateActionSensitivity()
			w.lastSuccessfulLoad = time.Now()
			w.setStatus("Loaded Secrets Manager secret "+name, false)
		})
	}()
}

func formatSecretValueDetail(secret model.Secret, value model.SecretValue) string {
	display := prettySecretValue(value)
	return fmt.Sprintf("SECRET\n\nName          %s\nARN           %s\nDescription   %s\nLast changed  %s\nLast accessed %s\n\nTags\n%s\n\nValue\n%s",
		secret.Name, valueOrDash(secret.ARN), valueOrDash(secret.Description), formatTime(secret.LastChanged),
		formatTime(secret.LastAccessed), formatSecretTags(secret.Tags), display)
}

func prettySecretValue(value model.SecretValue) string {
	if value.Binary {
		return "(binary secret; binary values are not displayed)"
	}
	var parsed any
	if json.Unmarshal([]byte(value.Value), &parsed) == nil {
		if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
			return string(pretty)
		}
	}
	return value.Value
}

func (w *mainWindow) showSecretValueEditor(secret model.Secret, value *model.SecretValue) {
	if value == nil || value.Name != w.selectedSecret {
		return
	}
	if value.Binary {
		w.setStatus("Binary Secrets Manager values cannot be edited as text", true)
		return
	}
	dialog := gtk.NewDialogWithFlags("Edit Secrets Manager secret", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(680, 420)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel(secret.Name)
	label.SetXAlign(0)
	label.SetWrap(true)
	content.Append(label)
	warning := gtk.NewLabel("This editor contains the decrypted secret value.")
	warning.SetXAlign(0)
	warning.SetWrap(true)
	warning.AddCSSClass("error")
	content.Append(warning)
	buffer, view := newSecretValueEditor(value.Value)
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(view)
	content.Append(scroll)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review update…", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		start, end := buffer.Bounds()
		edited := buffer.Text(start, end, true)
		dialog.Destroy()
		if edited == value.Value {
			w.setStatus("Secrets Manager secret was not changed", false)
			return
		}
		w.confirmSecretUpdate(secret.Name, edited)
	})
	dialog.Present()
}

func newSecretValueEditor(value string) (*gtk.TextBuffer, *gtk.TextView) {
	buffer := gtk.NewTextBuffer(nil)
	buffer.SetText(value)
	view := gtk.NewTextViewWithBuffer(buffer)
	view.SetEditable(true)
	view.SetCursorVisible(true)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapWordChar)
	return buffer, view
}

func (w *mainWindow) confirmSecretUpdate(name, value string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Update Secrets Manager secret")
	dialog.SetMarkup("Create a new version of <b>" + html.EscapeString(name) + "</b> with the edited value?")
	dialog.SetObjectProperty("secondary-text", "The current AWSCURRENT version will be replaced. Existing versions remain governed by Secrets Manager staging labels and retention behavior.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.runSecretUpdate(name, value)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runSecretUpdate(name, value string) {
	if name == "" || w.options.Secrets == nil || w.secretActionPending {
		return
	}
	w.secretActionPending = true
	w.updateActionSensitivity()
	filter := w.secretNameFilter
	ctx, generation := w.startRequest("Updating Secrets Manager secret " + name + "…")
	go func() {
		err := w.options.Secrets.Update(ctx, name, value)
		var secrets []model.Secret
		var detail *model.SecretValue
		var refreshErr error
		if err == nil {
			secrets, refreshErr = w.options.Secrets.List(ctx, filter)
			if refreshErr == nil {
				detail, refreshErr = w.options.Secrets.Detail(ctx, name)
			}
		}
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.secretActionPending = false
			if err != nil {
				w.updateActionSensitivity()
				w.setStatus(err.Error(), true)
				return
			}
			if refreshErr == nil && w.currentPage == pageSecrets && w.selectedSecret == name {
				w.allSecrets = secrets
				w.applySecretFilter()
				w.secretDetail = detail
				if secret, found := findSecret(secrets, name); found {
					w.setDetail(formatSecretValueDetail(secret, *detail), detailSecret)
				}
			}
			w.updateActionSensitivity()
			w.lastSuccessfulLoad = time.Now()
			status := "Updated Secrets Manager secret " + name
			if refreshErr != nil {
				status += " • refresh failed: " + refreshErr.Error()
			}
			w.setStatus(status, false)
		})
	}()
}

func (w *mainWindow) showCloneSecretEditor(source model.Secret, value *model.SecretValue) {
	if value == nil || value.Name != w.selectedSecret {
		return
	}
	if value.Binary {
		w.setStatus("Binary Secrets Manager values cannot be cloned as text", true)
		return
	}
	dialog := gtk.NewDialogWithFlags("Clone Secrets Manager secret", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(680, 460)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	nameLabel := gtk.NewLabel("New secret name")
	nameLabel.SetXAlign(0)
	name := gtk.NewEntry()
	name.SetText(source.Name + "-copy")
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	warning := gtk.NewLabel("This editor contains the decrypted source value.")
	warning.SetXAlign(0)
	warning.AddCSSClass("error")
	buffer, view := newSecretValueEditor(value.Value)
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(view)
	content.Append(nameLabel)
	content.Append(name)
	content.Append(warning)
	content.Append(scroll)
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review clone…", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		newName := strings.TrimSpace(name.Text())
		if newName == "" {
			errorLabel.SetLabel("Enter a secret name")
			return
		}
		start, end := buffer.Bounds()
		cloneValue := buffer.Text(start, end, true)
		dialog.Destroy()
		w.confirmSecretClone(newName, cloneValue)
	})
	dialog.Present()
}

func (w *mainWindow) confirmSecretClone(name, value string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Create cloned secret")
	dialog.SetMarkup("Create a new Secrets Manager secret named <b>" + html.EscapeString(name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "The new secret receives the edited plaintext value. Tags and resource policies are not copied.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.runSecretClone(name, value)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runSecretClone(name, value string) {
	if name == "" || w.options.Secrets == nil || w.secretActionPending {
		return
	}
	w.secretActionPending = true
	w.updateActionSensitivity()
	filter := w.secretNameFilter
	ctx, generation := w.startRequest("Creating Secrets Manager secret " + name + "…")
	go func() {
		err := w.options.Secrets.Create(ctx, name, value, "")
		var secrets []model.Secret
		var refreshErr error
		if err == nil {
			secrets, refreshErr = w.options.Secrets.List(ctx, filter)
		}
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.secretActionPending = false
			if err != nil {
				w.updateActionSensitivity()
				w.setStatus(err.Error(), true)
				return
			}
			if refreshErr == nil && w.currentPage == pageSecrets {
				w.allSecrets = secrets
				w.applySecretFilter()
			}
			w.updateActionSensitivity()
			w.lastSuccessfulLoad = time.Now()
			status := "Created Secrets Manager secret " + name
			if refreshErr != nil {
				status += " • refresh failed: " + refreshErr.Error()
			}
			w.setStatus(status, false)
		})
	}()
}

func (w *mainWindow) copySelectedSecretARN() {
	secret, found := findSecret(w.allSecrets, w.selectedSecret)
	if !found || secret.ARN == "" {
		return
	}
	w.secretTable.view.Clipboard().SetText(secret.ARN)
	w.setStatus("Copied secret ARN", false)
}
