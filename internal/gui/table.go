//go:build gui

package gui

import (
	"strings"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type columnSpec struct {
	title  string
	field  int
	expand bool
}

type stringTable struct {
	model     *gtk.StringList
	selection *gtk.SingleSelection
	view      *gtk.ColumnView
	count     uint
	rows      []string
}

func newStringTable(columns []columnSpec) *stringTable {
	model := gtk.NewStringList(nil)
	selection := gtk.NewSingleSelection(model)
	selection.SetAutoselect(false)
	selection.SetCanUnselect(true)
	view := gtk.NewColumnView(selection)
	view.SetShowColumnSeparators(true)
	view.SetShowRowSeparators(true)
	view.SetSingleClickActivate(false)

	for _, spec := range columns {
		view.AppendColumn(textColumn(spec))
	}
	return &stringTable{model: model, selection: selection, view: view}
}

func (t *stringTable) replace(rows []string) {
	if stringRowsEqual(t.rows, rows) {
		return
	}
	selected, preserveSelection := preservedRowPosition(t.rows, rows, t.selection.Selected())
	t.model.Splice(0, t.count, rows)
	t.count = uint(len(rows))
	t.rows = append(t.rows[:0], rows...)
	if preserveSelection {
		t.selection.SetSelected(selected)
	}
}

func textColumn(spec columnSpec) *gtk.ColumnViewColumn {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(object *coreglib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		label := gtk.NewLabel("")
		label.SetXAlign(0)
		label.SetEllipsize(3)
		label.AddCSSClass("table-cell")
		cell.SetChild(label)
	})
	factory.ConnectBind(func(object *coreglib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		value := cell.Item().Cast().(*gtk.StringObject).String()
		fields := strings.Split(value, "\t")
		text := ""
		if spec.field < len(fields) {
			text = fields[spec.field]
		}
		cell.Child().(*gtk.Label).SetLabel(text)
	})

	column := gtk.NewColumnViewColumn(spec.title, &factory.ListItemFactory)
	column.SetExpand(spec.expand)
	column.SetResizable(true)
	return column
}
