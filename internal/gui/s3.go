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

func (o Options) ConfigS3Searches() []config.S3Search {
	if o.Config == nil {
		return nil
	}
	return o.Config.S3Searches
}

func (w *mainWindow) openS3Module() {
	if w.currentPage == pageS3Buckets && w.activeSavedS3Search == "" {
		return
	}
	if w.guardEditorNavigation(w.openS3Module) {
		return
	}
	w.loadS3Buckets("", "")
}

func (w *mainWindow) loadS3Buckets(filter, savedName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearS3Buckets()
	w.currentPage = pageS3Buckets
	w.s3BucketFilter = strings.TrimSpace(filter)
	w.activeSavedS3Search = savedName
	w.selectedS3Bucket = ""
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb(s3Breadcrumb(savedName, ""))
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter buckets…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageS3Buckets)
	w.setDetail("Loading S3 buckets…", detailIntro)

	if w.options.S3 == nil {
		w.setDetail("S3 is unavailable because no S3 service was configured.", detailError)
		w.setStatus("S3 service unavailable", true)
		return
	}

	serverFilter := w.s3BucketFilter
	ctx, generation := w.startRequest(s3LoadingMessage(serverFilter))
	go func() {
		buckets, err := w.options.S3.Buckets(ctx, serverFilter)
		w.finishRequest(ctx, generation, err, func() {
			w.allS3Buckets = buckets
			w.applyS3BucketFilter()
			w.setDetail(s3BucketListSummary(serverFilter, len(buckets)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshS3Buckets(foreground bool) {
	if w.options.S3 == nil {
		return
	}
	filter, selected := w.s3BucketFilter, w.selectedS3Bucket
	ctx, generation := w.startRefreshRequest(s3RefreshingMessage(filter), foreground)
	go func() {
		buckets, err := w.options.S3.Buckets(ctx, filter)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allS3Buckets = buckets
			w.applyS3BucketFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(s3BucketListSummary(filter, len(buckets)), detailIntro)
				}
				return
			}
			bucket, found := findS3Bucket(buckets, selected)
			if !found {
				w.selectedS3Bucket = ""
				w.setBreadcrumb(s3Breadcrumb(w.activeSavedS3Search, ""))
				w.setDetail("The selected bucket is no longer available.\n\n"+s3BucketListSummary(filter, len(buckets)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			w.setDetail(formatS3Bucket(bucket), detailS3Bucket)
		})
	}()
}

func (w *mainWindow) clearS3Buckets() {
	w.allS3Buckets = nil
	w.filteredS3Buckets = nil
	w.selectedS3Bucket = ""
	if w.s3BucketTable != nil {
		w.s3BucketTable.clear()
	}
}

func (w *mainWindow) applyS3BucketFilter() {
	w.filteredS3Buckets = filterS3Buckets(w.allS3Buckets, w.search.Text())
	rows := make([]string, len(w.filteredS3Buckets))
	for i, bucket := range w.filteredS3Buckets {
		rows[i] = fmt.Sprintf("%s\t%s", bucket.Name, formatTime(bucket.CreatedAt))
	}
	w.s3BucketTable.replace(rows)
}

func (w *mainWindow) selectS3BucketRow() {
	if w.currentPage != pageS3Buckets {
		return
	}
	position := w.s3BucketTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredS3Buckets) {
		if w.selectedS3Bucket != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedS3Bucket = ""
			w.setBreadcrumb(s3Breadcrumb(w.activeSavedS3Search, ""))
			w.setDetail(s3BucketListSummary(w.s3BucketFilter, len(w.allS3Buckets)), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	bucket := w.filteredS3Buckets[position]
	if w.selectedS3Bucket != bucket.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedS3Bucket = bucket.Name
	w.setBreadcrumb(s3Breadcrumb(w.activeSavedS3Search, bucket.Name))
	w.setDetail(formatS3Bucket(bucket), detailS3Bucket)
	w.updateActionSensitivity()
}

func (w *mainWindow) openS3BucketAt(position uint) {
	if int(position) >= len(w.filteredS3Buckets) {
		return
	}
	w.s3BucketTable.selection.SetSelected(position)
}

func filterS3Buckets(buckets []model.S3Bucket, query string) []model.S3Bucket {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.S3Bucket(nil), buckets...)
	}
	filtered := make([]model.S3Bucket, 0, len(buckets))
	for _, bucket := range buckets {
		if strings.Contains(strings.ToLower(bucket.Name), query) {
			filtered = append(filtered, bucket)
		}
	}
	return filtered
}

func findS3Bucket(buckets []model.S3Bucket, name string) (model.S3Bucket, bool) {
	for _, bucket := range buckets {
		if bucket.Name == name {
			return bucket, true
		}
	}
	return model.S3Bucket{}, false
}

func s3Breadcrumb(savedName, bucket string) string {
	root := "Buckets"
	if savedName != "" {
		root = savedName
	}
	crumb := "S3 / " + root
	if bucket != "" {
		crumb += " / " + bucket
	}
	return crumb
}

func s3BucketListSummary(filter string, count int) string {
	filter = strings.TrimSpace(filter)
	if count == 0 {
		if filter == "" {
			return "No S3 buckets found in this account."
		}
		return fmt.Sprintf("No S3 buckets matched %q.", filter)
	}
	summary := fmt.Sprintf("S3 BUCKETS\n\nBuckets %d", count)
	if filter != "" {
		summary += "\nSaved filter " + filter
	}
	return summary + "\n\nSelect a bucket to inspect it; double-click will browse its contents in the next phase."
}

func formatS3Bucket(bucket model.S3Bucket) string {
	return fmt.Sprintf("S3 BUCKET\n\nName     %s\nCreated  %s\nURI      s3://%s/", bucket.Name, formatTime(bucket.CreatedAt), bucket.Name)
}

func s3LoadingMessage(filter string) string {
	if strings.TrimSpace(filter) == "" {
		return "Loading S3 buckets…"
	}
	return "Loading saved S3 bucket search…"
}

func s3RefreshingMessage(filter string) string {
	if strings.TrimSpace(filter) == "" {
		return "Refreshing S3 buckets…"
	}
	return "Refreshing saved S3 bucket search…"
}

func (w *mainWindow) rebuildS3SearchRail() {
	if w.s3ModuleItems == nil {
		return
	}
	if w.savedS3SearchesLabel != nil {
		w.s3ModuleItems.Remove(w.savedS3SearchesLabel)
	}
	for _, button := range w.savedS3SearchButtons {
		w.s3ModuleItems.Remove(button)
	}
	w.savedS3SearchesLabel = nil
	w.savedS3SearchButtons = nil
	searches := w.options.ConfigS3Searches()
	if len(searches) == 0 {
		return
	}
	w.savedS3SearchesLabel = gtk.NewLabel("SAVED SEARCHES")
	w.savedS3SearchesLabel.SetXAlign(0)
	w.savedS3SearchesLabel.AddCSSClass("section-title")
	w.s3ModuleItems.Append(w.savedS3SearchesLabel)
	for _, search := range searches {
		search := search
		button := newModuleRailButton(search.Name, func() {
			load := func() { w.loadS3Buckets(search.Filter, search.Name) }
			if !w.guardEditorNavigation(load) {
				load()
			}
		})
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(search.Filter)
		w.s3ModuleItems.Append(button)
		w.savedS3SearchButtons = append(w.savedS3SearchButtons, button)
	}
}

func (w *mainWindow) reloadS3SearchConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.S3Searches = append([]config.S3Search(nil), fresh.S3Searches...)
	w.rebuildS3SearchRail()
	if w.activeSavedS3Search == "" {
		w.updateActionSensitivity()
		return true
	}
	for _, search := range w.options.Config.S3Searches {
		if search.Name == w.activeSavedS3Search {
			w.s3BucketFilter = search.Filter
			w.updateActionSensitivity()
			return true
		}
	}
	w.loadS3Buckets("", "")
	w.setStatus("The active saved S3 search was removed from the configuration", false)
	return false
}

func (w *mainWindow) promptSaveS3Search() {
	if w.options.Config == nil || w.currentPage != pageS3Buckets {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save S3 bucket search", "Search name", "")
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
		if _, found := findS3Search(w.options.Config.S3Searches, name); found {
			errorLabel.SetLabel("A saved S3 search already uses that name")
			return
		}
		if !w.mutateS3Searches(func(cfg *config.Config) { cfg.AddS3Search(name, w.s3BucketFilter) }) {
			return
		}
		dialog.Destroy()
		w.activeSavedS3Search = name
		w.rebuildS3SearchRail()
		w.setBreadcrumb(s3Breadcrumb(name, w.selectedS3Bucket))
		w.updateActionSensitivity()
		w.setStatus("Saved S3 search "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageS3Searches() {
	searches := w.options.ConfigS3Searches()
	if len(searches) == 0 {
		return
	}
	names := make([]string, len(searches))
	selected := 0
	for i, search := range searches {
		names[i] = search.Name
		if search.Name == w.activeSavedS3Search {
			selected = i
		}
	}
	dialog := gtk.NewDialogWithFlags("Saved S3 searches", &w.window.Window, gtk.DialogModal)
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
	nameLabel := gtk.NewLabel("Name")
	nameLabel.SetXAlign(0)
	nameEntry := gtk.NewEntry()
	filterLabel := gtk.NewLabel("Bucket name contains")
	filterLabel.SetXAlign(0)
	filterEntry := gtk.NewEntry()
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	loadSelected := func() {
		index := int(selector.Selected())
		if index >= 0 && index < len(searches) {
			nameEntry.SetText(searches[index].Name)
			filterEntry.SetText(searches[index].Filter)
			errorLabel.SetLabel("")
		}
	}
	selector.NotifyProperty("selected", loadSelected)
	loadSelected()
	content.Append(selector)
	content.Append(nameLabel)
	content.Append(nameEntry)
	content.Append(filterLabel)
	content.Append(filterEntry)
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Delete…", 102)
	dialog.AddButton("Apply", 103)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		if index < 0 || index >= len(searches) {
			dialog.Destroy()
			return
		}
		current := searches[index]
		switch response {
		case 101:
			dialog.Destroy()
			w.loadS3Buckets(current.Filter, current.Name)
		case 102:
			dialog.Destroy()
			w.confirmDeleteS3Search(current.Name)
		case 103:
			name := strings.TrimSpace(nameEntry.Text())
			filter := strings.TrimSpace(filterEntry.Text())
			if name == "" {
				errorLabel.SetLabel("Enter a name")
				return
			}
			if existing, found := findS3Search(searches, name); found && existing.Name != current.Name {
				errorLabel.SetLabel("A saved S3 search already uses that name")
				return
			}
			if !w.mutateS3Searches(func(cfg *config.Config) {
				cfg.RemoveS3Search(current.Name)
				cfg.AddS3Search(name, filter)
			}) {
				return
			}
			dialog.Destroy()
			w.rebuildS3SearchRail()
			if w.activeSavedS3Search == current.Name {
				w.loadS3Buckets(filter, name)
			} else {
				w.updateActionSensitivity()
			}
			w.setStatus("Updated S3 search "+name, false)
		default:
			dialog.Destroy()
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteS3Search(name string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved S3 search <b>" + html.EscapeString(name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateS3Searches(func(cfg *config.Config) { cfg.RemoveS3Search(name) }) {
			return
		}
		wasActive := w.activeSavedS3Search == name
		w.rebuildS3SearchRail()
		if wasActive {
			w.loadS3Buckets("", "")
		} else {
			w.updateActionSensitivity()
		}
		w.setStatus("Deleted S3 search "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) mutateS3Searches(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.S3Search(nil), w.options.Config.S3Searches...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.S3Searches = before
		w.setStatus("Save S3 search: "+err.Error(), true)
		return false
	}
	return true
}

func findS3Search(searches []config.S3Search, name string) (config.S3Search, bool) {
	for _, search := range searches {
		if search.Name == name {
			return search, true
		}
	}
	return config.S3Search{}, false
}
