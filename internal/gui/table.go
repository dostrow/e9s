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

// clear synchronously removes every row and selection from the GTK model.
// Context changes use this instead of replace(nil) so stale list items cannot
// remain rendered while the next asynchronous request is in flight.
func (t *stringTable) clear() {
	t.selection.SetSelected(gtk.InvalidListPosition)
	if t.count > 0 {
		t.model.Splice(0, t.count, nil)
	}
	t.count = 0
	t.rows = nil
	t.view.QueueDraw()
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
		label := cell.Child().(*gtk.Label)
		label.SetLabel(text)
		for _, class := range semanticTableClasses {
			label.RemoveCSSClass(class)
		}
		if class := semanticClassForValue(text); class != "" {
			label.AddCSSClass(class)
		}
	})

	column := gtk.NewColumnViewColumn(spec.title, &factory.ListItemFactory)
	column.SetExpand(spec.expand)
	column.SetResizable(true)
	return column
}

var semanticTableClasses = []string{
	"semantic-success", "semantic-warning", "semantic-error", "semantic-info", "semantic-muted",
}

func semanticClassForValue(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.NewReplacer(" ", "_", "-", "_").Replace(normalized)
	switch normalized {
	case "ACTIVE", "AVAILABLE", "HEALTHY", "RUNNING", "OK", "ENABLED",
		"SUCCEEDED", "SUCCESS", "COMPLETE", "COMPLETED", "PASS", "PASSED":
		return "semantic-success"
	case "PENDING", "PROVISIONING", "DRAINING", "IN_PROGRESS", "INSUFFICIENT_DATA",
		"STARTING", "STOPPING", "MODIFYING", "UPGRADING", "REBOOTING", "BACKING_UP",
		"MAINTENANCE", "HIGH", "MEDIUM":
		return "semantic-warning"
	case "FAILED", "FAILURE", "UNHEALTHY", "ALARM", "ERROR", "CRITICAL", "DELETED":
		return "semantic-error"
	case "WRITER", "READER", "PRIMARY", "REPLICA", "INACTIVE", "INFORMATIONAL", "INFO":
		return "semantic-info"
	case "UNKNOWN", "UNDEFINED", "DISABLED":
		return "semantic-muted"
	default:
		return ""
	}
}
