//go:build gui

package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/highlight"
	"github.com/dostrow/e9s/internal/model"
)

var logHighlightMatchOptions = []struct {
	label string
	value model.LogHighlightMatch
}{
	{label: "Exact", value: model.LogHighlightLiteral},
	{label: "Case-insensitive", value: model.LogHighlightLiteralCI},
	{label: "Regular expression", value: model.LogHighlightRegex},
}

var logHighlightStyleOptions = []struct {
	label string
	value model.LogHighlightStyle
}{
	{label: "Highlight", value: model.LogHighlightDefault},
	{label: "Info", value: model.LogHighlightInfo},
	{label: "Success", value: model.LogHighlightSuccess},
	{label: "Warning", value: model.LogHighlightWarning},
	{label: "Error", value: model.LogHighlightError},
}

func (w *mainWindow) promptLogHighlights() {
	if !w.showingLogs {
		return
	}
	saveLabel := ""
	if w.options.Config != nil && (w.activeSavedLog != "" || w.logSearchSpec != nil) {
		saveLabel = "Apply & save search…"
		if w.activeSavedLog != "" {
			saveLabel = "Apply & update saved"
		}
	}
	w.promptHighlightRuleEditor(&w.window.Window, "Log highlight rules", w.logHighlightRules, saveLabel, func(rules []model.LogHighlightRule, save bool) {
		if err := w.setLogHighlightRules(rules); err != nil {
			return
		}
		w.renderLogs()
		w.updateActionSensitivity()
		w.setStatus(fmt.Sprintf("Applied %d log highlight rules", len(rules)), false)
		if !save {
			return
		}
		if w.activeSavedLog != "" {
			w.updateActiveSavedLogFromWorkspace()
			return
		}
		w.promptSaveLogSearch()
	})
}

func (w *mainWindow) promptHighlightRuleEditor(parent *gtk.Window, title string, initial []model.LogHighlightRule, saveLabel string, apply func([]model.LogHighlightRule, bool)) {
	working := append([]model.LogHighlightRule(nil), initial...)
	dialog := gtk.NewDialogWithFlags(title, parent, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(840, 420)
	content := dialog.ContentArea()
	content.SetSpacing(10)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	help := gtk.NewLabel("Rules affect presentation only. Earlier rules win when matches overlap.")
	help.SetXAlign(0)
	help.SetWrap(true)
	help.AddCSSClass("muted")
	content.Append(help)

	headings := gtk.NewBox(gtk.OrientationHorizontal, 8)
	patternHeading := gtk.NewLabel("Pattern")
	patternHeading.SetXAlign(0)
	patternHeading.SetHExpand(true)
	matchHeading := gtk.NewLabel("Match")
	matchHeading.SetSizeRequest(160, -1)
	styleHeading := gtk.NewLabel("Style")
	styleHeading.SetSizeRequest(120, -1)
	actionsHeading := gtk.NewLabel("Actions")
	actionsHeading.SetSizeRequest(132, -1)
	headings.Append(patternHeading)
	headings.Append(matchHeading)
	headings.Append(styleHeading)
	headings.Append(actionsHeading)
	content.Append(headings)

	rowsBox := gtk.NewBox(gtk.OrientationVertical, 6)
	rowsScroll := gtk.NewScrolledWindow()
	rowsScroll.SetVExpand(true)
	rowsScroll.SetHExpand(true)
	rowsScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	rowsScroll.SetChild(rowsBox)
	content.Append(rowsScroll)

	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)

	var rows []*gtk.Box
	var rebuildRows func()
	rebuildRows = func() {
		for _, row := range rows {
			rowsBox.Remove(row)
		}
		rows = nil
		if len(working) == 0 {
			empty := gtk.NewBox(gtk.OrientationHorizontal, 0)
			label := gtk.NewLabel("No rules. Add one to highlight matching log text.")
			label.SetXAlign(0)
			label.AddCSSClass("muted")
			empty.Append(label)
			rowsBox.Append(empty)
			rows = append(rows, empty)
			return
		}
		for index := range working {
			index := index
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			pattern := gtk.NewEntry()
			pattern.SetText(working[index].Pattern)
			pattern.SetPlaceholderText("Text or Go regular expression")
			pattern.SetHExpand(true)
			pattern.ConnectChanged(func() {
				working[index].Pattern = pattern.Text()
				errorLabel.SetLabel("")
			})

			matchLabels := make([]string, len(logHighlightMatchOptions))
			for i, option := range logHighlightMatchOptions {
				matchLabels[i] = option.label
			}
			match := gtk.NewDropDownFromStrings(matchLabels)
			match.SetSizeRequest(160, -1)
			match.SetSelected(uint(highlightMatchOptionIndex(working[index].Match)))
			match.NotifyProperty("selected", func() {
				selected := int(match.Selected())
				if selected >= 0 && selected < len(logHighlightMatchOptions) {
					working[index].Match = logHighlightMatchOptions[selected].value
					errorLabel.SetLabel("")
				}
			})

			styleLabels := make([]string, len(logHighlightStyleOptions))
			for i, option := range logHighlightStyleOptions {
				styleLabels[i] = option.label
			}
			style := gtk.NewDropDownFromStrings(styleLabels)
			style.SetSizeRequest(120, -1)
			style.SetSelected(uint(highlightStyleOptionIndex(working[index].Style)))
			style.NotifyProperty("selected", func() {
				selected := int(style.Selected())
				if selected >= 0 && selected < len(logHighlightStyleOptions) {
					working[index].Style = logHighlightStyleOptions[selected].value
				}
			})

			up := gtk.NewButtonWithLabel("↑")
			up.SetTooltipText("Move rule earlier")
			up.SetSensitive(index > 0)
			up.ConnectClicked(func() {
				working[index-1], working[index] = working[index], working[index-1]
				rebuildRows()
			})
			down := gtk.NewButtonWithLabel("↓")
			down.SetTooltipText("Move rule later")
			down.SetSensitive(index < len(working)-1)
			down.ConnectClicked(func() {
				working[index], working[index+1] = working[index+1], working[index]
				rebuildRows()
			})
			remove := gtk.NewButtonWithLabel("Delete")
			remove.ConnectClicked(func() {
				working = append(working[:index], working[index+1:]...)
				rebuildRows()
			})
			actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
			actions.SetSizeRequest(132, -1)
			actions.Append(up)
			actions.Append(down)
			actions.Append(remove)

			row.Append(pattern)
			row.Append(match)
			row.Append(style)
			row.Append(actions)
			rowsBox.Append(row)
			rows = append(rows, row)
		}
	}
	rebuildRows()

	add := gtk.NewButtonWithLabel("Add rule")
	add.SetHAlign(gtk.AlignStart)
	add.ConnectClicked(func() {
		working = append(working, model.LogHighlightRule{
			Match: model.LogHighlightLiteral, Style: model.LogHighlightDefault,
		})
		rebuildRows()
	})
	content.Append(add)

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Apply", int(gtk.ResponseOK))
	if saveLabel != "" {
		dialog.AddButton(saveLabel, 101)
	}
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) && response != 101 {
			dialog.Destroy()
			return
		}
		if _, err := highlight.Compile(working); err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		rules := append([]model.LogHighlightRule(nil), working...)
		dialog.Destroy()
		if apply != nil {
			apply(rules, response == 101)
		}
	})
	dialog.Present()
}

func highlightMatchOptionIndex(value model.LogHighlightMatch) int {
	for i, option := range logHighlightMatchOptions {
		if option.value == value {
			return i
		}
	}
	return 0
}

func highlightStyleOptionIndex(value model.LogHighlightStyle) int {
	for i, option := range logHighlightStyleOptions {
		if option.value == value {
			return i
		}
	}
	return 0
}
