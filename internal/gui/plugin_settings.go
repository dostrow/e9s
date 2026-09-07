//go:build gui

package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/runbook"
)

type pluginSettingsRow struct {
	container *gtk.Box
	name      *gtk.Entry
	manifest  *gtk.Entry
	status    *gtk.Label
}

type pluginSettingsEditor struct {
	window *mainWindow
	page   *gtk.Box
	rows   *gtk.Box
	items  []*pluginSettingsRow
}

func newPluginSettingsEditor(window *mainWindow, entries []config.PluginEntry) *pluginSettingsEditor {
	editor := &pluginSettingsEditor{window: window, page: settingsPage()}
	editor.page.Append(settingsNote("Plugin manifests are trusted local code. e9s validates their declarations, shows the exact command before launch, and only loads manifests registered here."))
	editor.rows = gtk.NewBox(gtk.OrientationVertical, 12)
	editor.rows.SetHExpand(true)
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(editor.rows)
	editor.page.Append(scroll)

	controls := gtk.NewBox(gtk.OrientationHorizontal, 8)
	add := gtk.NewButtonWithLabel("Add plugin")
	add.ConnectClicked(func() {
		row := editor.append(config.PluginEntry{})
		row.manifest.GrabFocus()
	})
	controls.Append(add)
	controls.Append(settingsNote("Choose plugin.yaml, or enter a directory containing plugin.yaml."))
	editor.page.Append(controls)

	for _, entry := range entries {
		row := editor.append(entry)
		editor.validateRow(row)
	}
	return editor
}

func (editor *pluginSettingsEditor) append(entry config.PluginEntry) *pluginSettingsRow {
	row := &pluginSettingsRow{container: gtk.NewBox(gtk.OrientationVertical, 6)}
	row.container.AddCSSClass("card")
	row.container.SetMarginTop(4)
	row.container.SetMarginBottom(4)
	row.container.SetMarginStart(4)
	row.container.SetMarginEnd(4)
	row.name = gtk.NewEntry()
	row.name.SetText(entry.Name)
	row.name.SetPlaceholderText("Optional friendly name")
	row.name.SetHExpand(true)
	row.manifest = gtk.NewEntry()
	row.manifest.SetText(entry.Manifest)
	row.manifest.SetPlaceholderText("/path/to/repository/.e9s/plugin.yaml")
	row.manifest.SetHExpand(true)
	row.status = settingsNote("Not validated")

	row.container.Append(settingsRow("Friendly name", row.name))
	manifestControls := gtk.NewBox(gtk.OrientationHorizontal, 6)
	manifestControls.SetHExpand(true)
	manifestControls.Append(row.manifest)
	browse := gtk.NewButtonWithLabel("Browse…")
	manifestControls.Append(browse)
	row.container.Append(settingsRow("Manifest", manifestControls))

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	validate := gtk.NewButtonWithLabel("Validate")
	remove := gtk.NewButtonWithLabel("Remove")
	remove.AddCSSClass("destructive-action")
	actions.Append(validate)
	actions.Append(remove)
	actions.Append(row.status)
	row.container.Append(actions)
	editor.rows.Append(row.container)
	editor.items = append(editor.items, row)

	markUnvalidated := func() {
		row.status.RemoveCSSClass("error")
		row.status.SetLabel("Not validated")
	}
	row.name.ConnectChanged(markUnvalidated)
	row.manifest.ConnectChanged(markUnvalidated)
	validate.ConnectClicked(func() { editor.validateRow(row) })
	remove.ConnectClicked(func() { editor.remove(row) })
	browse.ConnectClicked(func() { editor.browse(row) })
	return row
}

func (editor *pluginSettingsEditor) remove(target *pluginSettingsRow) {
	for index, row := range editor.items {
		if row != target {
			continue
		}
		editor.rows.Remove(row.container)
		editor.items = append(editor.items[:index], editor.items[index+1:]...)
		return
	}
}

func (editor *pluginSettingsEditor) browse(row *pluginSettingsRow) {
	chooser := gtk.NewFileChooserNative("Choose plugin manifest", &editor.window.window.Window, gtk.FileChooserActionOpen, "Choose", "Cancel")
	chooser.SetModal(true)
	if path := strings.TrimSpace(row.manifest.Text()); path != "" {
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(config.Path()), resolved)
		}
		if info, err := filepath.Abs(resolved); err == nil {
			_ = chooser.SetFile(gio.NewFileForPath(info))
		}
	}
	chooser.ConnectResponse(func(response int) {
		defer chooser.Destroy()
		if response != int(gtk.ResponseAccept) || chooser.File() == nil || chooser.File().Path() == "" {
			return
		}
		row.manifest.SetText(chooser.File().Path())
		editor.validateRow(row)
	})
	chooser.Show()
}

func (editor *pluginSettingsEditor) validateRow(row *pluginSettingsRow) bool {
	entry := config.PluginEntry{Name: strings.TrimSpace(row.name.Text()), Manifest: strings.TrimSpace(row.manifest.Text())}
	if entry.Manifest == "" {
		row.status.SetLabel("Manifest path is required")
		row.status.AddCSSClass("error")
		return false
	}
	actions, err := loadPluginEntries([]config.PluginEntry{entry})
	if err != nil {
		row.status.SetLabel(err.Error())
		row.status.AddCSSClass("error")
		return false
	}
	row.status.RemoveCSSClass("error")
	row.status.SetLabel(fmt.Sprintf("Valid: %s — %d action(s)", actions[0].PluginDisplayName, len(actions)))
	return true
}

func (editor *pluginSettingsEditor) entries() ([]config.PluginEntry, error) {
	entries := make([]config.PluginEntry, 0, len(editor.items))
	for index, row := range editor.items {
		entry := config.PluginEntry{Name: strings.TrimSpace(row.name.Text()), Manifest: strings.TrimSpace(row.manifest.Text())}
		if entry.Manifest == "" {
			return nil, fmt.Errorf("plugin %d manifest path is required", index+1)
		}
		entries = append(entries, entry)
	}
	if _, err := loadPluginEntries(entries); err != nil {
		return nil, err
	}
	for _, row := range editor.items {
		editor.validateRow(row)
	}
	return entries, nil
}

func loadPluginEntries(entries []config.PluginEntry) ([]runbook.ConfiguredAction, error) {
	sources := make([]runbook.Source, len(entries))
	for index, entry := range entries {
		sources[index] = runbook.Source{Name: entry.Name, Path: entry.Manifest}
	}
	return runbook.LoadSources(sources, filepath.Dir(config.Path()))
}
