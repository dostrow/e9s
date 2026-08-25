//go:build gui

package gui

import (
	"strings"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type columnSpec struct {
	title  string
	field  int
	expand bool
}

type stringTable struct {
	model          *gtk.StringList
	selection      *gtk.SingleSelection
	multiSelection *gtk.MultiSelection
	view           *gtk.ColumnView
	columns        []*gtk.ColumnViewColumn
	specs          []columnSpec
	count          uint
	rows           []string
	cellActivated  func(position uint, field int)
	dragText       func(position uint) string
}

func newStringTable(columns []columnSpec) *stringTable {
	model := gtk.NewStringList(nil)
	selection := gtk.NewSingleSelection(model)
	selection.SetAutoselect(false)
	selection.SetCanUnselect(true)
	view := gtk.NewColumnView(selection)
	view.AddCSSClass("e9s-table")
	view.SetShowColumnSeparators(true)
	view.SetShowRowSeparators(true)
	view.SetSingleClickActivate(false)

	table := &stringTable{model: model, selection: selection, view: view}
	table.setColumns(columns)
	return table
}

func newMultiStringTable(columns []columnSpec) *stringTable {
	model := gtk.NewStringList(nil)
	selection := gtk.NewMultiSelection(model)
	view := gtk.NewColumnView(selection)
	view.AddCSSClass("e9s-table")
	view.SetShowColumnSeparators(true)
	view.SetShowRowSeparators(true)
	view.SetSingleClickActivate(false)

	table := &stringTable{model: model, multiSelection: selection, view: view}
	table.setColumns(columns)
	return table
}

// setColumns replaces the presentation schema without replacing the row model.
// This is used by schemaless browsers such as DynamoDB, where columns are
// discovered from the current result set and may expand when another page loads.
func (t *stringTable) setColumns(specs []columnSpec) {
	if columnSpecsEqual(t.specs, specs) {
		t.view.SetVisible(len(specs) > 0)
		return
	}
	// GtkColumnView's horizontal adjustment requires the content width to be
	// at least the allocated page width. Avoid allocating the transient
	// zero-column state while dynamic schemas are replaced.
	t.view.SetVisible(false)
	for _, column := range t.columns {
		t.view.RemoveColumn(column)
	}
	t.columns = t.columns[:0]
	for _, spec := range specs {
		column := t.textColumn(spec)
		t.view.AppendColumn(column)
		t.columns = append(t.columns, column)
	}
	t.specs = append(t.specs[:0], specs...)
	t.view.SetVisible(len(specs) > 0)
	t.view.QueueDraw()
}

func columnSpecsEqual(left, right []columnSpec) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (t *stringTable) replace(rows []string) {
	if stringRowsEqual(t.rows, rows) {
		return
	}
	selected, preserveSelection := uint(gtk.InvalidListPosition), false
	if t.selection != nil {
		selected, preserveSelection = preservedRowPosition(t.rows, rows, t.selection.Selected())
	}
	if t.multiSelection != nil {
		t.multiSelection.UnselectAll()
	}
	t.model.Splice(0, t.count, rows)
	t.count = uint(len(rows))
	t.rows = append(t.rows[:0], rows...)
	if preserveSelection && t.selection != nil {
		t.selection.SetSelected(selected)
	}
}

// clear synchronously removes every row and selection from the GTK model.
// Context changes use this instead of replace(nil) so stale list items cannot
// remain rendered while the next asynchronous request is in flight.
func (t *stringTable) clear() {
	if t.selection != nil {
		t.selection.SetSelected(gtk.InvalidListPosition)
	}
	if t.multiSelection != nil {
		t.multiSelection.UnselectAll()
	}
	if t.count > 0 {
		t.model.Splice(0, t.count, nil)
	}
	t.count = 0
	t.rows = nil
	t.view.QueueDraw()
}

func (t *stringTable) selectedPositions() []int {
	if t.multiSelection != nil {
		positions := make([]int, 0)
		for position := range t.rows {
			if t.multiSelection.IsSelected(uint(position)) {
				positions = append(positions, position)
			}
		}
		return positions
	}
	if t.selection == nil {
		return nil
	}
	position := t.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(t.rows) {
		return nil
	}
	return []int{int(position)}
}

func (t *stringTable) textColumn(spec columnSpec) *gtk.ColumnViewColumn {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(object *coreglib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		label := gtk.NewLabel("")
		label.SetXAlign(0)
		label.SetEllipsize(3)
		label.AddCSSClass("table-cell")
		if t.cellActivated != nil {
			click := gtk.NewGestureClick()
			click.SetButton(gdk.BUTTON_PRIMARY)
			click.ConnectPressed(func(nPress int, _, _ float64) {
				if nPress != 2 {
					return
				}
				position := cell.Position()
				if position != gtk.InvalidListPosition {
					t.cellActivated(position, spec.field)
				}
			})
			label.AddController(click)
		}
		if t.dragText != nil {
			drag := gtk.NewDragSource()
			drag.SetActions(gdk.ActionCopy)
			drag.ConnectPrepare(func(_, _ float64) *gdk.ContentProvider {
				position := cell.Position()
				if position == gtk.InvalidListPosition {
					return nil
				}
				text := t.dragText(position)
				if text == "" {
					return nil
				}
				return gdk.NewContentProviderForValue(coreglib.NewValue(text))
			})
			label.AddController(drag)
		}
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
