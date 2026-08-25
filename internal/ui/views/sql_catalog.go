package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/sqlworkbench"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SQLCatalogRequestKind int

const (
	SQLCatalogRequestNone SQLCatalogRequestKind = iota
	SQLCatalogRequestObjects
	SQLCatalogRequestColumns
)

type SQLCatalogRequest struct {
	Kind       SQLCatalogRequestKind
	Schema     string
	ObjectKind sqlworkbench.ObjectKind
	Object     sqlworkbench.DatabaseObject
}

type sqlCatalogRowKind int

const (
	sqlCatalogMessage sqlCatalogRowKind = iota
	sqlCatalogSchema
	sqlCatalogCategory
	sqlCatalogObject
	sqlCatalogColumn
)

type sqlCatalogRow struct {
	key        string
	kind       sqlCatalogRowKind
	schema     string
	objectKind sqlworkbench.ObjectKind
	object     sqlworkbench.DatabaseObject
	column     sqlworkbench.ObjectColumn
	label      string
}

type SQLCatalogModel struct {
	Open           bool
	Schemas        []string
	Objects        map[string][]sqlworkbench.DatabaseObject
	Columns        map[string][]sqlworkbench.ObjectColumn
	Loaded         map[string]bool
	ColumnsLoaded  map[string]bool
	Loading        map[string]bool
	ColumnsLoading map[string]bool
	Expanded       map[string]bool
	Rows           []sqlCatalogRow
	Cursor         int
	LoadingSchemas bool
	SchemasLoaded  bool
	Status         string
}

func NewSQLCatalogModel() SQLCatalogModel {
	return SQLCatalogModel{
		Objects: make(map[string][]sqlworkbench.DatabaseObject), Columns: make(map[string][]sqlworkbench.ObjectColumn),
		Loaded: make(map[string]bool), ColumnsLoaded: make(map[string]bool), Loading: make(map[string]bool),
		ColumnsLoading: make(map[string]bool), Expanded: make(map[string]bool),
	}
}

func sqlCatalogSchemaKey(schema string) string { return "schema:" + schema }

func sqlCatalogCategoryKey(schema string, kind sqlworkbench.ObjectKind) string {
	return "category:" + schema + ":" + string(kind)
}

func sqlCatalogObjectKey(object sqlworkbench.DatabaseObject) string {
	return "object:" + object.OID + ":" + string(object.Kind)
}

func (m *SQLCatalogModel) Toggle() (opened, needsSchemas bool) {
	m.Open = !m.Open
	if m.Open && !m.SchemasLoaded && !m.LoadingSchemas {
		m.LoadingSchemas = true
		m.Status = "Loading schemas…"
		return true, true
	}
	m.rebuild()
	return m.Open, false
}

func (m *SQLCatalogModel) SetSchemas(schemas []string, err error) {
	m.LoadingSchemas = false
	if err != nil {
		m.Status = "Load schemas: " + err.Error()
		m.rebuild()
		return
	}
	m.Schemas = append(m.Schemas[:0], schemas...)
	m.SchemasLoaded = true
	m.Status = fmt.Sprintf("Loaded %d schema(s)", len(schemas))
	m.rebuild()
}

func (m *SQLCatalogModel) SetObjects(schema string, kind sqlworkbench.ObjectKind, objects []sqlworkbench.DatabaseObject, err error) {
	key := sqlCatalogCategoryKey(schema, kind)
	delete(m.Loading, key)
	if err != nil {
		m.Status = "Load " + sqlworkbench.ObjectKindLabel(kind) + ": " + err.Error()
		m.rebuild()
		return
	}
	m.Objects[key] = objects
	m.Loaded[key] = true
	m.Status = fmt.Sprintf("Loaded %d %s", len(objects), strings.ToLower(sqlworkbench.ObjectKindLabel(kind)))
	m.rebuild()
}

func (m *SQLCatalogModel) SetColumns(object sqlworkbench.DatabaseObject, columns []sqlworkbench.ObjectColumn, err error) {
	key := sqlCatalogObjectKey(object)
	delete(m.ColumnsLoading, key)
	if err != nil {
		m.Status = "Load columns for " + object.QualifiedName() + ": " + err.Error()
		m.rebuild()
		return
	}
	m.Columns[key] = columns
	m.ColumnsLoaded[key] = true
	m.Status = fmt.Sprintf("Loaded %d column(s) for %s", len(columns), object.QualifiedName())
	m.rebuild()
}

func (m *SQLCatalogModel) Move(delta int) {
	m.Cursor = max(0, min(len(m.Rows)-1, m.Cursor+delta))
}

func (m *SQLCatalogModel) Top()    { m.Cursor = 0 }
func (m *SQLCatalogModel) Bottom() { m.Cursor = max(0, len(m.Rows)-1) }

func (m *SQLCatalogModel) Activate() SQLCatalogRequest {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return SQLCatalogRequest{}
	}
	row := m.Rows[m.Cursor]
	switch row.kind {
	case sqlCatalogSchema:
		m.Expanded[row.key] = !m.Expanded[row.key]
	case sqlCatalogCategory:
		m.Expanded[row.key] = !m.Expanded[row.key]
		if m.Expanded[row.key] && !m.Loaded[row.key] && !m.Loading[row.key] {
			m.Loading[row.key] = true
			m.rebuild()
			return SQLCatalogRequest{Kind: SQLCatalogRequestObjects, Schema: row.schema, ObjectKind: row.objectKind}
		}
	case sqlCatalogObject:
		if sqlworkbench.ObjectKindHasColumns(row.object.Kind) {
			m.Expanded[row.key] = !m.Expanded[row.key]
			if m.Expanded[row.key] && !m.ColumnsLoaded[row.key] && !m.ColumnsLoading[row.key] {
				m.ColumnsLoading[row.key] = true
				m.rebuild()
				return SQLCatalogRequest{Kind: SQLCatalogRequestColumns, Object: row.object}
			}
		}
	}
	m.rebuild()
	return SQLCatalogRequest{}
}

func (m SQLCatalogModel) Insertion(qualified bool) (string, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return "", false
	}
	row := m.Rows[m.Cursor]
	switch row.kind {
	case sqlCatalogObject:
		if qualified {
			return row.object.QualifiedName(), true
		}
		return sqlworkbench.QuoteIdentifier(row.object.Name), true
	case sqlCatalogColumn:
		if qualified {
			return row.column.QualifiedName(row.object), true
		}
		return row.column.SQLName(), true
	default:
		return "", false
	}
}

func (m *SQLCatalogModel) rebuild() {
	selectedKey := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		selectedKey = m.Rows[m.Cursor].key
	}
	m.Rows = m.Rows[:0]
	for _, schema := range m.Schemas {
		schemaKey := sqlCatalogSchemaKey(schema)
		m.Rows = append(m.Rows, sqlCatalogRow{key: schemaKey, kind: sqlCatalogSchema, schema: schema,
			label: sqlCatalogTreeLabel(0, m.Expanded[schemaKey], schema)})
		if !m.Expanded[schemaKey] {
			continue
		}
		for _, kind := range sqlworkbench.CatalogKinds() {
			categoryKey := sqlCatalogCategoryKey(schema, kind)
			label := sqlworkbench.ObjectKindLabel(kind)
			if m.Loaded[categoryKey] {
				label += fmt.Sprintf(" (%d)", len(m.Objects[categoryKey]))
			}
			m.Rows = append(m.Rows, sqlCatalogRow{key: categoryKey, kind: sqlCatalogCategory, schema: schema, objectKind: kind,
				label: sqlCatalogTreeLabel(1, m.Expanded[categoryKey], label)})
			if !m.Expanded[categoryKey] {
				continue
			}
			if m.Loading[categoryKey] {
				m.Rows = append(m.Rows, sqlCatalogRow{key: categoryKey + ":loading", kind: sqlCatalogMessage, label: "    Loading…"})
				continue
			}
			for _, object := range m.Objects[categoryKey] {
				objectKey := sqlCatalogObjectKey(object)
				label := "    " + object.DisplayName()
				if sqlworkbench.ObjectKindHasColumns(object.Kind) {
					label = sqlCatalogTreeLabel(2, m.Expanded[objectKey], object.DisplayName())
				}
				m.Rows = append(m.Rows, sqlCatalogRow{key: objectKey, kind: sqlCatalogObject, schema: schema, objectKind: kind, object: object, label: label})
				if !m.Expanded[objectKey] || !sqlworkbench.ObjectKindHasColumns(object.Kind) {
					continue
				}
				if m.ColumnsLoading[objectKey] {
					m.Rows = append(m.Rows, sqlCatalogRow{key: objectKey + ":loading", kind: sqlCatalogMessage, label: "            Loading columns…"})
					continue
				}
				for _, column := range m.Columns[objectKey] {
					m.Rows = append(m.Rows, sqlCatalogRow{key: objectKey + ":column:" + column.Name, kind: sqlCatalogColumn,
						schema: schema, objectKind: kind, object: object, column: column, label: "            " + column.DisplayName()})
				}
			}
		}
	}
	if selectedKey != "" {
		for index, row := range m.Rows {
			if row.key == selectedKey {
				m.Cursor = index
				return
			}
		}
	}
	m.Cursor = min(m.Cursor, max(0, len(m.Rows)-1))
}

func sqlCatalogTreeLabel(depth int, expanded bool, label string) string {
	chevron := "▸"
	if expanded {
		chevron = "▾"
	}
	return strings.Repeat("  ", depth) + chevron + " " + label
}

func (m SQLCatalogModel) View(width, height int) string {
	width = max(24, width)
	height = max(4, height)
	innerHeight := max(2, height-2)
	lines := make([]string, 0, innerHeight)
	lines = append(lines, theme.TitleStyle.Render("DATABASE OBJECTS"))
	visible := max(1, innerHeight-2)
	start := max(0, m.Cursor-visible/2)
	start = min(start, max(0, len(m.Rows)-visible))
	end := min(len(m.Rows), start+visible)
	for index := start; index < end; index++ {
		label := truncateSQLCell(m.Rows[index].label, max(8, width-4))
		if index == m.Cursor {
			label = theme.SelectedRowStyle.Width(max(1, width-4)).Render(label)
		}
		lines = append(lines, label)
	}
	if len(m.Rows) == 0 {
		empty := "No user schemas found"
		if m.LoadingSchemas {
			empty = "Loading schemas…"
		}
		lines = append(lines, theme.HelpStyle.Render(empty))
	}
	for len(lines) < innerHeight-1 {
		lines = append(lines, "")
	}
	if m.Status != "" {
		lines = append(lines, theme.HelpStyle.Render(truncateSQLCell(m.Status, max(8, width-4))))
	} else {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorCyan).
		Width(width-2).Height(innerHeight).Padding(0, 1).Render(strings.Join(lines, "\n"))
}
