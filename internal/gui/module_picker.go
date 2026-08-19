//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func sortModuleRailSections(sections []moduleRailSection) {
	sort.SliceStable(sections, func(i, j int) bool {
		return strings.ToLower(sections[i].name) < strings.ToLower(sections[j].name)
	})
}

func normalizeModuleSelection(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func moduleSectionIndex(sections []moduleRailSection, value string) (int, bool) {
	selection := normalizeModuleSelection(value)
	if selection == "" {
		return 0, false
	}
	for i, section := range sections {
		if selection == normalizeModuleSelection(section.key) || selection == normalizeModuleSelection(section.name) {
			return i, true
		}
		for _, alias := range section.aliases {
			if selection == normalizeModuleSelection(alias) {
				return i, true
			}
		}
	}
	return 0, false
}

func modulePickerLabels(sections []moduleRailSection) []string {
	labels := make([]string, len(sections))
	for i, section := range sections {
		labels[i] = section.name
		if section.defaultItem != "" {
			labels[i] += " — " + section.defaultItem
		}
	}
	return labels
}

func (w *mainWindow) start() {
	if w.options.DefaultCluster != "" {
		w.activateModule(moduleECS)
		return
	}
	if w.options.Config != nil && strings.TrimSpace(w.options.Config.Defaults.DefaultMode) != "" {
		configured := w.options.Config.Defaults.DefaultMode
		if w.activateModule(configured) {
			return
		}
		w.setStatus(fmt.Sprintf("Configured default module %q is not available in the GUI", configured), true)
	}
	w.showModulePicker()
}

func (w *mainWindow) activateModule(value string) bool {
	index, found := moduleSectionIndex(w.moduleSections, value)
	if !found {
		return false
	}
	if w.showingEditor {
		w.closeEditorThen(func() { w.activateModule(value) })
		return true
	}
	for i := range w.moduleSections {
		w.moduleSections[i].expander.SetExpanded(i == index)
	}
	w.search.SetSensitive(true)
	w.moduleSections[index].activate()
	return true
}

func (w *mainWindow) showModulePicker() {
	if w.modulePickerOpen || len(w.moduleSections) == 0 {
		return
	}
	w.modulePickerOpen = true

	dialog := gtk.NewDialogWithFlags("Choose a module", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	prompt := gtk.NewLabel("Choose a module to open its default view.")
	prompt.SetXAlign(0)
	selector := gtk.NewDropDownFromStrings(modulePickerLabels(w.moduleSections))
	selector.SetHExpand(true)
	if current, found := moduleSectionIndex(w.moduleSections, moduleForPage(w.currentPage)); found {
		selector.SetSelected(uint(current))
	}
	detail := gtk.NewLabel("")
	detail.SetXAlign(0)
	detail.AddCSSClass("muted")
	updateDetail := func() {
		index := int(selector.Selected())
		if index >= 0 && index < len(w.moduleSections) {
			detail.SetLabel("Default view: " + w.moduleSections[index].defaultItem)
		}
	}
	selector.NotifyProperty("selected", updateDetail)
	updateDetail()
	content.Append(prompt)
	content.Append(selector)
	content.Append(detail)

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectDestroy(func() {
		w.modulePickerOpen = false
	})
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		dialog.Destroy()
		if response == int(gtk.ResponseOK) && index >= 0 && index < len(w.moduleSections) {
			w.activateModule(w.moduleSections[index].key)
		}
	})
	dialog.Present()
}
