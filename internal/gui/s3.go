//go:build gui

package gui

import (
	"context"
	"errors"
	"fmt"
	"html"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
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
	w.loadS3Objects(w.filteredS3Buckets[position].Name, "")
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

func (w *mainWindow) loadS3Objects(bucket, prefix string) {
	w.resetWorkspaceForBrowserChange()
	w.clearS3Objects()
	w.currentPage = pageS3Objects
	w.selectedS3Bucket = bucket
	w.s3Prefix = prefix
	w.s3ObjectSearch = ""
	w.s3ObjectSearchActive = false
	w.updateActionSensitivity()
	w.setBreadcrumb(s3ObjectBreadcrumb(w.activeSavedS3Search, bucket, prefix, false))
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter objects…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageS3Objects)
	w.setDetail("Loading S3 objects…", detailIntro)

	if w.options.S3 == nil {
		w.setDetail("S3 is unavailable because no S3 service was configured.", detailError)
		w.setStatus("S3 service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading s3://" + bucket + "/" + prefix + "…")
	go func() {
		objects, err := w.options.S3.Objects(ctx, bucket, prefix)
		w.finishRequest(ctx, generation, err, func() {
			w.allS3Objects = objects
			w.applyS3ObjectFilter()
			w.setDetail(s3ObjectListSummary(bucket, prefix, false, len(objects)), detailIntro)
		})
	}()
}

func (w *mainWindow) searchS3Objects(bucket, keyPrefix string) {
	w.resetWorkspaceForBrowserChange()
	w.clearS3Objects()
	w.currentPage = pageS3Objects
	w.selectedS3Bucket = bucket
	w.s3Prefix = ""
	w.s3ObjectSearch = keyPrefix
	w.s3ObjectSearchActive = true
	w.updateActionSensitivity()
	w.setBreadcrumb(s3ObjectBreadcrumb(w.activeSavedS3Search, bucket, keyPrefix, true))
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter search results…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageS3Objects)
	w.setDetail("Searching S3 object keys…", detailIntro)

	if w.options.S3 == nil {
		w.setDetail("S3 is unavailable because no S3 service was configured.", detailError)
		w.setStatus("S3 service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Searching s3://" + bucket + "/" + keyPrefix + "…")
	go func() {
		objects, err := w.options.S3.Search(ctx, bucket, keyPrefix)
		w.finishRequest(ctx, generation, err, func() {
			w.allS3Objects = objects
			w.applyS3ObjectFilter()
			w.setDetail(s3ObjectListSummary(bucket, keyPrefix, true, len(objects)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshS3Objects(foreground bool) {
	if w.options.S3 == nil || w.selectedS3Bucket == "" {
		return
	}
	bucket, prefix := w.selectedS3Bucket, w.s3Prefix
	keyPrefix, searching := w.s3ObjectSearch, w.s3ObjectSearchActive
	selected := w.selectedS3Object
	ctx, generation := w.startRefreshRequest("Refreshing S3 objects…", foreground)
	go func() {
		var objects []model.S3Object
		var err error
		if searching {
			objects, err = w.options.S3.Search(ctx, bucket, keyPrefix)
		} else {
			objects, err = w.options.S3.Objects(ctx, bucket, prefix)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allS3Objects = objects
			w.applyS3ObjectFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(s3ObjectListSummary(bucket, valueIf(searching, keyPrefix, prefix), searching, len(objects)), detailIntro)
				}
				return
			}
			object, found := findS3Object(objects, selected)
			if !found {
				w.selectedS3Object = ""
				w.s3ObjectDetail = nil
				w.setBreadcrumb(s3ObjectBreadcrumb(w.activeSavedS3Search, bucket, valueIf(searching, keyPrefix, prefix), searching))
				w.setDetail("The selected object is no longer available.\n\n"+s3ObjectListSummary(bucket, valueIf(searching, keyPrefix, prefix), searching, len(objects)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			if object.IsPrefix {
				w.setDetail(formatS3Prefix(bucket, object.Key), detailS3Object)
			} else if w.s3ObjectDetail == nil || w.s3ObjectDetail.Key != object.Key {
				w.setDetail(formatS3ObjectSummary(bucket, object), detailS3Object)
			}
		})
	}()
}

func (w *mainWindow) clearS3Objects() {
	w.allS3Objects = nil
	w.filteredS3Objects = nil
	w.selectedS3Object = ""
	w.s3ObjectDetail = nil
	if w.s3ObjectTable != nil {
		w.s3ObjectTable.clear()
	}
}

func (w *mainWindow) applyS3ObjectFilter() {
	w.filteredS3Objects = filterS3Objects(w.allS3Objects, w.search.Text(), w.s3Prefix)
	rows := make([]string, len(w.filteredS3Objects))
	for i, object := range w.filteredS3Objects {
		kind, size, modified := "Object", formatS3Bytes(object.Size), formatTime(object.LastModified)
		if object.IsPrefix {
			kind, size, modified = "Folder", "—", "—"
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s", s3ObjectDisplayName(object.Key, w.s3Prefix), kind, size, modified)
	}
	w.s3ObjectTable.replace(rows)
}

func (w *mainWindow) selectS3ObjectRow() {
	if w.currentPage != pageS3Objects {
		return
	}
	position := w.s3ObjectTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredS3Objects) {
		if w.selectedS3Object != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedS3Object = ""
			w.s3ObjectDetail = nil
			w.setBreadcrumb(w.currentS3ObjectBreadcrumb())
			w.setDetail(w.currentS3ObjectSummary(), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	object := w.filteredS3Objects[position]
	if w.selectedS3Object != object.Key {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedS3Object = object.Key
	w.s3ObjectDetail = nil
	w.setBreadcrumb(w.currentS3ObjectBreadcrumb() + " / " + s3ObjectDisplayName(object.Key, w.s3Prefix))
	w.updateActionSensitivity()
	if object.IsPrefix {
		w.setDetail(formatS3Prefix(w.selectedS3Bucket, object.Key), detailS3Object)
		return
	}
	w.setDetail(formatS3ObjectSummary(w.selectedS3Bucket, object), detailS3Object)
	w.loadS3ObjectDetail(w.selectedS3Bucket, object.Key)
}

func (w *mainWindow) openS3ObjectAt(position uint) {
	if int(position) >= len(w.filteredS3Objects) {
		return
	}
	object := w.filteredS3Objects[position]
	w.s3ObjectTable.selection.SetSelected(position)
	if object.IsPrefix {
		w.loadS3Objects(w.selectedS3Bucket, object.Key)
	}
}

func (w *mainWindow) loadS3ObjectDetail(bucket, key string) {
	if w.options.S3 == nil || bucket == "" || key == "" {
		return
	}
	ctx, generation := w.startRequest("Loading S3 object metadata…")
	go func() {
		detail, err := w.options.S3.Detail(ctx, bucket, key)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageS3Objects || w.selectedS3Bucket != bucket || w.selectedS3Object != key {
				return
			}
			w.s3ObjectDetail = detail
			w.setDetail(formatS3ObjectDetail(bucket, detail), detailS3Object)
			w.applyS3ArchiveStatusStyle(detail)
		})
	}()
}

func (w *mainWindow) promptS3KeySearch() {
	if w.currentPage != pageS3Objects || w.selectedS3Bucket == "" {
		return
	}
	initial := w.s3Prefix
	if w.s3ObjectSearchActive {
		initial = w.s3ObjectSearch
	}
	dialog, entry := w.newSavedLogNameDialog("Search S3 object keys", "Key prefix", initial)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Search", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.searchS3Objects(w.selectedS3Bucket, strings.TrimSpace(entry.Text()))
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptS3Download() {
	object, found := findS3Object(w.allS3Objects, w.selectedS3Object)
	if !found || w.currentPage != pageS3Objects || w.s3DownloadPending || w.options.S3 == nil {
		return
	}
	action := gtk.FileChooserActionSave
	title := "Download S3 object"
	if object.IsPrefix {
		action = gtk.FileChooserActionSelectFolder
		title = "Download S3 folder into"
	}
	chooser := gtk.NewFileChooserNative(title, &w.window.Window, action, "Download", "Cancel")
	chooser.SetModal(true)
	if w.options.Config != nil {
		folder := gio.NewFileForPath(w.options.Config.SaveDir())
		_ = chooser.SetCurrentFolder(folder)
	}
	if !object.IsPrefix {
		chooser.SetCurrentName(path.Base(object.Key))
	}
	chooser.ConnectResponse(func(response int) {
		if response == int(gtk.ResponseAccept) {
			file := chooser.File()
			if file != nil && file.Path() != "" {
				w.runS3Download(model.S3DownloadRequest{
					Bucket: w.selectedS3Bucket, Key: object.Key, Destination: file.Path(), IsPrefix: object.IsPrefix,
				})
			}
		}
		chooser.Destroy()
	})
	chooser.Show()
}

func (w *mainWindow) runS3Download(request model.S3DownloadRequest) {
	if w.options.S3 == nil || w.s3DownloadPending {
		return
	}
	w.s3DownloadPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Starting S3 download…")
	cancel := w.requestCancel
	w.setWorkspaceCancellation(func() {
		if cancel != nil {
			cancel()
		}
		w.workspaceBusyLabel.SetLabel("Canceling S3 download…")
		w.workspaceCancelButton.SetSensitive(false)
	})
	go func() {
		result, err := w.options.S3.DownloadWithProgress(ctx, request, func(progress model.S3DownloadProgress) {
			label := formatS3DownloadProgress(request, progress)
			glib.IdleAdd(func() {
				if generation == w.generation && ctx.Err() == nil && w.s3DownloadPending {
					w.workspaceBusyLabel.SetLabel(label)
				}
			})
		})
		glib.IdleAdd(func() {
			if generation != w.generation {
				return
			}
			w.requestCancel = nil
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.s3DownloadPending = false
			w.updateActionSensitivity()
			switch {
			case errors.Is(err, context.Canceled) || ctx.Err() != nil:
				w.setStatus("S3 download canceled", false)
			case err != nil:
				w.setStatus(err.Error(), true)
			default:
				w.lastSuccessfulLoad = time.Now()
				if request.IsPrefix {
					w.setStatus(fmt.Sprintf("Downloaded %d files to %s", result.Files, result.Destination), false)
				} else {
					w.setStatus("Downloaded to "+result.Destination, false)
				}
			}
		})
	}()
}

func formatS3DownloadProgress(request model.S3DownloadRequest, progress model.S3DownloadProgress) string {
	label := "Downloading " + s3ObjectDisplayName(request.Key, "")
	if progress.BytesCompleted > 0 {
		label += " • " + formatS3Bytes(progress.BytesCompleted)
	}
	if request.IsPrefix && progress.FilesCompleted > 0 {
		label += fmt.Sprintf(" • %d files", progress.FilesCompleted)
	}
	if request.IsPrefix && progress.CurrentKey != "" {
		label += " • " + progress.CurrentKey
	}
	return label
}

func (w *mainWindow) navigateS3Back() bool {
	if w.currentPage != pageS3Objects {
		return false
	}
	if !w.s3ObjectSearchActive && w.s3Prefix != "" {
		w.loadS3Objects(w.selectedS3Bucket, parentS3Prefix(w.s3Prefix))
		return true
	}
	w.resetWorkspaceForBrowserChange()
	bucketName := w.selectedS3Bucket
	w.currentPage = pageS3Buckets
	w.clearS3Objects()
	w.search.SetPlaceholderText("Filter buckets…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageS3Buckets)
	w.backButton.SetSensitive(false)
	w.applyS3BucketFilter()
	if bucket, found := findS3Bucket(w.allS3Buckets, bucketName); found {
		w.selectedS3Bucket = bucketName
		w.setBreadcrumb(s3Breadcrumb(w.activeSavedS3Search, bucketName))
		w.setDetail(formatS3Bucket(bucket), detailS3Bucket)
		for index, candidate := range w.filteredS3Buckets {
			if candidate.Name == bucketName {
				w.s3BucketTable.selection.SetSelected(uint(index))
				break
			}
		}
	} else {
		w.selectedS3Bucket = ""
		w.setBreadcrumb(s3Breadcrumb(w.activeSavedS3Search, ""))
		w.setDetail(s3BucketListSummary(w.s3BucketFilter, len(w.allS3Buckets)), detailIntro)
	}
	w.updateActionSensitivity()
	w.setStatus("Ready", false)
	return true
}

func (w *mainWindow) currentS3ObjectBreadcrumb() string {
	value, searching := w.s3Prefix, false
	if w.s3ObjectSearchActive {
		value, searching = w.s3ObjectSearch, true
	}
	return s3ObjectBreadcrumb(w.activeSavedS3Search, w.selectedS3Bucket, value, searching)
}

func (w *mainWindow) currentS3ObjectSummary() string {
	value, searching := w.s3Prefix, false
	if w.s3ObjectSearchActive {
		value, searching = w.s3ObjectSearch, true
	}
	return s3ObjectListSummary(w.selectedS3Bucket, value, searching, len(w.allS3Objects))
}

func filterS3Objects(objects []model.S3Object, query, prefix string) []model.S3Object {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.S3Object(nil), objects...)
	}
	filtered := make([]model.S3Object, 0, len(objects))
	for _, object := range objects {
		if strings.Contains(strings.ToLower(s3ObjectDisplayName(object.Key, prefix)), query) {
			filtered = append(filtered, object)
		}
	}
	return filtered
}

func findS3Object(objects []model.S3Object, key string) (model.S3Object, bool) {
	for _, object := range objects {
		if object.Key == key {
			return object, true
		}
	}
	return model.S3Object{}, false
}

func s3ObjectDisplayName(key, prefix string) string {
	relative := strings.TrimPrefix(key, prefix)
	relative = strings.TrimSuffix(relative, "/")
	if relative == "" {
		return path.Base(strings.TrimSuffix(key, "/"))
	}
	return relative
}

func parentS3Prefix(prefix string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if index := strings.LastIndex(prefix, "/"); index >= 0 {
		return prefix[:index+1]
	}
	return ""
}

func s3ObjectBreadcrumb(savedName, bucket, prefix string, searching bool) string {
	root := "Buckets"
	if savedName != "" {
		root = savedName
	}
	crumb := "S3 / " + root + " / " + bucket
	if searching {
		if prefix == "" {
			return crumb + " / Search: all keys"
		}
		return crumb + " / Search: " + prefix
	}
	if prefix != "" {
		crumb += " / " + strings.TrimSuffix(prefix, "/")
	}
	return crumb
}

func s3ObjectListSummary(bucket, prefix string, searching bool, count int) string {
	location := "s3://" + bucket + "/" + prefix
	if searching {
		location = "key-prefix search in s3://" + bucket + "/ for " + fmt.Sprintf("%q", prefix)
	}
	if count == 0 {
		return "No S3 objects found for " + location + "."
	}
	return fmt.Sprintf("S3 OBJECTS\n\nLocation  %s\nResults   %d\n\nFolders are listed before objects. Double-click a folder to browse it; select an object to load metadata and tags.", location, count)
}

func formatS3Prefix(bucket, prefix string) string {
	return fmt.Sprintf("S3 PREFIX\n\nURI  s3://%s/%s\n\nDouble-click this folder to browse its contents.", bucket, prefix)
}

func formatS3ObjectSummary(bucket string, object model.S3Object) string {
	return fmt.Sprintf("S3 OBJECT\n\nURI            s3://%s/%s\nKey            %s\nSize           %s\nLast modified  %s\n\nLoading metadata and tags…",
		bucket, object.Key, object.Key, formatS3Bytes(object.Size), formatTime(object.LastModified))
}

func formatS3ObjectDetail(bucket string, detail *model.S3ObjectDetail) string {
	if detail == nil {
		return "S3 object metadata is unavailable."
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "S3 OBJECT\n\nURI            s3://%s/%s\nKey            %s\nSize           %s\nContent type   %s\nETag           %s\nStorage class  %s\nLast modified  %s",
		bucket, detail.Key, detail.Key, formatS3Bytes(detail.Size), valueOrDash(detail.ContentType), valueOrDash(detail.ETag), valueOrDash(detail.StorageClass), formatTime(detail.LastModified))
	if accessTier := s3IntelligentTieringAccessTier(detail); accessTier != "" {
		fmt.Fprintf(&builder, "\nAccess tier    %s", accessTier)
	}
	if len(detail.Tags) > 0 {
		keys := make([]string, 0, len(detail.Tags))
		for key := range detail.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteString("\n\nTAGS\n")
		for _, key := range keys {
			fmt.Fprintf(&builder, "\n%s = %s", key, detail.Tags[key])
		}
	}
	return builder.String()
}

func s3IntelligentTieringAccessTier(detail *model.S3ObjectDetail) string {
	if detail == nil || detail.StorageClass != "INTELLIGENT_TIERING" {
		return ""
	}
	if detail.ArchiveStatus != "" {
		return detail.ArchiveStatus
	}
	return "Active tier (exact tier requires S3 Inventory)"
}

func (w *mainWindow) applyS3ArchiveStatusStyle(detail *model.S3ObjectDetail) {
	if detail == nil || w.detailErrorTag == nil || !model.IsS3ArchiveAccessStatus(detail.ArchiveStatus) {
		return
	}
	byteOffset := strings.Index(w.detailText, detail.ArchiveStatus)
	if byteOffset < 0 {
		return
	}
	start := utf8.RuneCountInString(w.detailText[:byteOffset])
	end := start + utf8.RuneCountInString(detail.ArchiveStatus)
	w.detailBuffer.ApplyTag(w.detailErrorTag, w.detailBuffer.IterAtOffset(start), w.detailBuffer.IterAtOffset(end))
}

func formatS3Bytes(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(size)/float64(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func valueIf(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
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
