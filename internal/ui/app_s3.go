package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- S3 Browser ---

func (a App) promptS3Browser() (App, tea.Cmd) {
	saved := a.cfg.S3Searches
	if len(saved) == 0 {
		a.input = NewInput(InputS3Search, "Search buckets (substring match, or empty for all)", "")
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, s := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", s.Name, s.Filter))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom search]")
	a.picker = NewPickerWithDelete(PickerS3Search, "Select S3 search", items, savedCount)
	return a, nil
}

func (a App) openS3Buckets(filter string) (App, tea.Cmd) {
	a.mode = modeS3
	a.state = viewS3Buckets
	a.s3BucketsView = views.NewS3Buckets(filter)
	a.s3BucketsView = a.s3BucketsView.SetSize(a.width, a.height-3)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		buckets, err := client.ListBuckets(context.Background(), filter)
		if err != nil {
			return errMsg{err}
		}
		return s3BucketsLoadedMsg{buckets}
	}
}

func (a App) openS3Objects(bucket, prefix string) (App, tea.Cmd) {
	a.mode = modeS3
	a.state = viewS3Objects
	a.s3ObjectsView = views.NewS3Objects(bucket, prefix)
	a.s3ObjectsView = a.s3ObjectsView.SetSize(a.width, a.height-3)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		objects, err := client.ListObjects(context.Background(), bucket, prefix)
		if err != nil {
			return errMsg{err}
		}
		return s3ObjectsLoadedMsg{objects}
	}
}

func (a App) promptS3KeySearch() (App, tea.Cmd) {
	bucket := a.s3ObjectsView.Bucket()
	currentPrefix := a.s3ObjectsView.Prefix()
	a.input = NewInput(InputS3KeySearch,
		fmt.Sprintf("Search by key prefix in %s", bucket), currentPrefix)
	return a, nil
}

func (a App) searchS3Keys(prefix string) (App, tea.Cmd) {
	bucket := a.s3ObjectsView.Bucket()
	return a.openS3Objects(bucket, prefix)
}

func (a App) loadS3Detail(bucket, key string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		detail, err := client.GetObjectDetail(context.Background(), bucket, key)
		if err != nil {
			return errMsg{err}
		}
		return s3DetailLoadedMsg{bucket: bucket, detail: detail}
	}
}

func (a App) showS3ObjectDetail() (App, tea.Cmd) {
	obj := a.s3ObjectsView.SelectedObject()
	if obj == nil || obj.IsPrefix {
		return a, nil
	}
	return a, a.loadS3Detail(a.s3ObjectsView.Bucket(), obj.Key)
}

func (a App) promptS3Download() (App, tea.Cmd) {
	obj := a.s3ObjectsView.SelectedObject()
	if obj == nil {
		return a, nil
	}
	a.s3DownloadBucket = a.s3ObjectsView.Bucket()
	a.s3DownloadKey = obj.Key
	a.s3DownloadIsPrefix = obj.IsPrefix
	if obj.IsPrefix {
		a.input = NewInput(InputS3Download, fmt.Sprintf("Download s3://%s/%s to directory", a.s3DownloadBucket, obj.Key), a.cfg.SaveDir())
	} else {
		name := obj.Key
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		a.input = NewInput(InputS3Download, fmt.Sprintf("Download %s to", name), filepath.Join(a.cfg.SaveDir(), name))
	}
	return a, nil
}

func (a App) doS3Download(destPath string) tea.Cmd {
	destPath = strings.TrimSpace(destPath)
	if destPath == "" {
		return func() tea.Msg {
			return s3DownloadDoneMsg{err: fmt.Errorf("no path specified")}
		}
	}
	client := a.client
	bucket := a.s3DownloadBucket
	key := a.s3DownloadKey
	isPrefix := a.s3DownloadIsPrefix

	return func() tea.Msg {
		// Ensure parent directory exists
		dir := destPath
		if !isPrefix {
			dir = filepath.Dir(destPath)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return s3DownloadDoneMsg{err: fmt.Errorf("cannot create directory %s: %w", dir, err)}
		}

		if isPrefix {
			count, err := client.DownloadPrefix(context.Background(), bucket, key, destPath)
			if err != nil {
				return s3DownloadDoneMsg{err: err}
			}
			return s3DownloadDoneMsg{message: fmt.Sprintf("Downloaded %d files to %s", count, destPath)}
		}
		err := client.DownloadObject(context.Background(), bucket, key, destPath)
		if err != nil {
			return s3DownloadDoneMsg{err: err}
		}
		return s3DownloadDoneMsg{message: fmt.Sprintf("Downloaded to %s", destPath)}
	}
}

func (a App) promptS3DownloadFromDetail() (App, tea.Cmd) {
	detail := a.s3DetailView.Detail()
	if detail == nil {
		return a, nil
	}
	a.s3DownloadBucket = a.s3DetailView.Bucket()
	a.s3DownloadKey = detail.Key
	a.s3DownloadIsPrefix = false
	name := detail.Key
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	a.input = NewInput(InputS3Download, fmt.Sprintf("Download %s to", name), filepath.Join(a.cfg.SaveDir(), name))
	return a, nil
}

func (a App) saveS3Search() (App, tea.Cmd) {
	filter := a.s3BucketsView.SearchTerm()
	a.input = NewInput(InputS3SaveName,
		fmt.Sprintf("Save S3 search %q — enter a name", filter), "")
	return a, nil
}

func (a App) doSaveS3Search(name string) (App, tea.Cmd) {
	filter := a.s3BucketsView.SearchTerm()
	a.cfg.AddS3Search(name, filter)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved S3 search %q as %q", filter, name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}
