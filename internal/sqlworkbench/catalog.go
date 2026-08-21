package sqlworkbench

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

// ObjectKind is a PostgreSQL catalog object family exposed by the workbench.
type ObjectKind string

const (
	ObjectTable            ObjectKind = "table"
	ObjectView             ObjectKind = "view"
	ObjectMaterializedView ObjectKind = "materialized-view"
	ObjectSequence         ObjectKind = "sequence"
	ObjectFunction         ObjectKind = "function"
)

type DatabaseObject struct {
	OID               string
	Schema            string
	Name              string
	Kind              ObjectKind
	IdentityArguments string
}

func (object DatabaseObject) DisplayName() string {
	if object.Kind == ObjectFunction && object.IdentityArguments != "" {
		return object.Name + "(" + object.IdentityArguments + ")"
	}
	return object.Name
}

func (object DatabaseObject) QualifiedName() string {
	return QuoteIdentifier(object.Schema) + "." + QuoteIdentifier(object.Name)
}

type ObjectColumn struct {
	Name       string
	DataType   string
	Nullable   bool
	Default    string
	Identity   string
	Generation string
}

type NamedDefinition struct {
	Name       string
	Definition string
}

type ObjectDetail struct {
	Object      DatabaseObject
	Owner       string
	Columns     []ObjectColumn
	Constraints []NamedDefinition
	Indexes     []NamedDefinition
	Definition  string
}

func ObjectKindLabel(kind ObjectKind) string {
	switch kind {
	case ObjectTable:
		return "Tables"
	case ObjectView:
		return "Views"
	case ObjectMaterializedView:
		return "Materialized views"
	case ObjectSequence:
		return "Sequences"
	case ObjectFunction:
		return "Functions"
	default:
		return string(kind)
	}
}

func CatalogKinds() []ObjectKind {
	return []ObjectKind{ObjectTable, ObjectView, ObjectMaterializedView, ObjectSequence, ObjectFunction}
}

func (e *Executor) ListSchemas(ctx context.Context, profile config.SQLConnection) ([]string, error) {
	result, err := e.catalogQuery(ctx, profile, `
SELECT n.nspname
FROM pg_catalog.pg_namespace n
WHERE n.nspname <> 'information_schema'
  AND n.nspname NOT LIKE 'pg_%'
ORDER BY n.nspname`)
	if err != nil {
		return nil, err
	}
	schemas := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 && row[0] != "" {
			schemas = append(schemas, row[0])
		}
	}
	return schemas, nil
}

func (e *Executor) ListObjects(ctx context.Context, profile config.SQLConnection, schema string, kind ObjectKind) ([]DatabaseObject, error) {
	var query string
	schemaLiteral := QuoteLiteral(schema)
	if kind == ObjectFunction {
		query = fmt.Sprintf(`
SELECT p.oid::text, p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid)
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = %s
  AND p.prokind IN ('f', 'p')
ORDER BY p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid)`, schemaLiteral)
	} else {
		relkind := map[ObjectKind]string{
			ObjectTable: "'r','p','f'", ObjectView: "'v'", ObjectMaterializedView: "'m'", ObjectSequence: "'S'",
		}[kind]
		if relkind == "" {
			return nil, fmt.Errorf("unsupported database object kind %q", kind)
		}
		query = fmt.Sprintf(`
SELECT c.oid::text, c.relname, ''
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = %s AND c.relkind IN (%s)
ORDER BY c.relname`, schemaLiteral, relkind)
	}
	result, err := e.catalogQuery(ctx, profile, query)
	if err != nil {
		return nil, err
	}
	objects := make([]DatabaseObject, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		object := DatabaseObject{OID: row[0], Schema: schema, Name: row[1], Kind: kind}
		if len(row) > 2 {
			object.IdentityArguments = row[2]
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func (e *Executor) InspectObject(ctx context.Context, profile config.SQLConnection, object DatabaseObject) (ObjectDetail, error) {
	if _, err := strconv.ParseUint(object.OID, 10, 64); err != nil {
		return ObjectDetail{}, fmt.Errorf("invalid catalog object identifier %q", object.OID)
	}
	detail := ObjectDetail{Object: object}
	ownerQuery := fmt.Sprintf(`SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c WHERE c.oid = %s::oid`, object.OID)
	if object.Kind == ObjectFunction {
		ownerQuery = fmt.Sprintf(`SELECT pg_catalog.pg_get_userbyid(p.proowner) FROM pg_catalog.pg_proc p WHERE p.oid = %s::oid`, object.OID)
	}
	ownerResult, err := e.catalogQuery(ctx, profile, ownerQuery)
	if err == nil && len(ownerResult.Rows) > 0 && len(ownerResult.Rows[0]) > 0 {
		detail.Owner = ownerResult.Rows[0][0]
	}
	if object.Kind != ObjectFunction {
		columns, queryErr := e.catalogQuery(ctx, profile, fmt.Sprintf(`
SELECT a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       CASE WHEN a.attnotnull THEN 'false' ELSE 'true' END,
       COALESCE(pg_catalog.pg_get_expr(d.adbin, d.adrelid), ''),
       CASE a.attidentity WHEN 'a' THEN 'ALWAYS' WHEN 'd' THEN 'BY DEFAULT' ELSE '' END,
       CASE a.attgenerated WHEN 's' THEN 'STORED' ELSE '' END
FROM pg_catalog.pg_attribute a
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE a.attrelid = %s::oid AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`, object.OID))
		if queryErr != nil {
			return ObjectDetail{}, queryErr
		}
		for _, row := range columns.Rows {
			if len(row) < 6 {
				continue
			}
			detail.Columns = append(detail.Columns, ObjectColumn{Name: row[0], DataType: row[1], Nullable: row[2] == "true",
				Default: row[3], Identity: row[4], Generation: row[5]})
		}
	}
	if object.Kind == ObjectTable || object.Kind == ObjectMaterializedView {
		constraints, queryErr := e.catalogQuery(ctx, profile, fmt.Sprintf(`
SELECT con.conname, pg_catalog.pg_get_constraintdef(con.oid, true)
FROM pg_catalog.pg_constraint con
WHERE con.conrelid = %s::oid
ORDER BY con.conname`, object.OID))
		if queryErr != nil {
			return ObjectDetail{}, queryErr
		}
		for _, row := range constraints.Rows {
			if len(row) >= 2 {
				detail.Constraints = append(detail.Constraints, NamedDefinition{Name: row[0], Definition: row[1]})
			}
		}
		indexes, queryErr := e.catalogQuery(ctx, profile, fmt.Sprintf(`
SELECT c.relname, pg_catalog.pg_get_indexdef(i.indexrelid)
FROM pg_catalog.pg_index i
JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid
WHERE i.indrelid = %s::oid
ORDER BY c.relname`, object.OID))
		if queryErr != nil {
			return ObjectDetail{}, queryErr
		}
		for _, row := range indexes.Rows {
			if len(row) >= 2 {
				detail.Indexes = append(detail.Indexes, NamedDefinition{Name: row[0], Definition: row[1]})
			}
		}
	}
	detail.Definition, err = e.objectDefinition(ctx, profile, detail)
	if err != nil {
		return ObjectDetail{}, err
	}
	return detail, nil
}

func (e *Executor) PreviewObject(ctx context.Context, profile config.SQLConnection, object DatabaseObject, limit int) (model.SQLQueryResult, error) {
	if object.Kind == ObjectFunction || object.Kind == ObjectSequence {
		return model.SQLQueryResult{}, fmt.Errorf("%s objects cannot be previewed as rows", ObjectKindLabel(object.Kind))
	}
	if limit <= 0 {
		limit = 100
	}
	return e.catalogQuery(ctx, profile, fmt.Sprintf("SELECT * FROM %s LIMIT %d", object.QualifiedName(), limit))
}

func (e *Executor) objectDefinition(ctx context.Context, profile config.SQLConnection, detail ObjectDetail) (string, error) {
	object := detail.Object
	switch object.Kind {
	case ObjectView, ObjectMaterializedView:
		result, err := e.catalogQuery(ctx, profile, fmt.Sprintf("SELECT pg_catalog.pg_get_viewdef(%s::oid, true)", object.OID))
		if err != nil {
			return "", err
		}
		body := firstCell(result)
		prefix := "CREATE OR REPLACE VIEW "
		if object.Kind == ObjectMaterializedView {
			prefix = "CREATE MATERIALIZED VIEW "
		}
		return prefix + object.QualifiedName() + " AS\n" + strings.TrimSpace(body) + ";", nil
	case ObjectFunction:
		result, err := e.catalogQuery(ctx, profile, fmt.Sprintf("SELECT pg_catalog.pg_get_functiondef(%s::oid)", object.OID))
		if err != nil {
			return "", err
		}
		return firstCell(result), nil
	case ObjectSequence:
		result, err := e.catalogQuery(ctx, profile, fmt.Sprintf(`
SELECT seqstart::text, seqincrement::text, seqmin::text, seqmax::text, seqcache::text, seqcycle::text
FROM pg_catalog.pg_sequence WHERE seqrelid = %s::oid`, object.OID))
		if err != nil {
			return "", err
		}
		if len(result.Rows) == 0 || len(result.Rows[0]) < 6 {
			return "CREATE SEQUENCE " + object.QualifiedName() + ";", nil
		}
		row := result.Rows[0]
		cycle := "NO CYCLE"
		if row[5] == "true" || row[5] == "t" {
			cycle = "CYCLE"
		}
		return fmt.Sprintf("CREATE SEQUENCE %s\n    START WITH %s\n    INCREMENT BY %s\n    MINVALUE %s\n    MAXVALUE %s\n    CACHE %s\n    %s;",
			object.QualifiedName(), row[0], row[1], row[2], row[3], row[4], cycle), nil
	default:
		return generatedTableDefinition(detail), nil
	}
}

func (e *Executor) catalogQuery(ctx context.Context, profile config.SQLConnection, query string) (model.SQLQueryResult, error) {
	results, err := e.Execute(ctx, profile, query, false)
	if err != nil {
		return model.SQLQueryResult{}, err
	}
	if len(results) == 0 {
		return model.SQLQueryResult{}, nil
	}
	return results[len(results)-1], nil
}

func generatedTableDefinition(detail ObjectDetail) string {
	lines := make([]string, 0, len(detail.Columns)+len(detail.Constraints))
	for _, column := range detail.Columns {
		line := "    " + QuoteIdentifier(column.Name) + " " + column.DataType
		if column.Generation != "" && column.Default != "" {
			line += " GENERATED ALWAYS AS (" + column.Default + ") " + column.Generation
		} else if column.Identity != "" {
			line += " GENERATED " + column.Identity + " AS IDENTITY"
		} else if column.Default != "" {
			line += " DEFAULT " + column.Default
		}
		if !column.Nullable {
			line += " NOT NULL"
		}
		lines = append(lines, line)
	}
	for _, constraint := range detail.Constraints {
		lines = append(lines, "    CONSTRAINT "+QuoteIdentifier(constraint.Name)+" "+constraint.Definition)
	}
	definition := "-- Generated from the PostgreSQL catalog; review before executing.\nCREATE TABLE " + detail.Object.QualifiedName() + " (\n" + strings.Join(lines, ",\n") + "\n);"
	if len(detail.Indexes) > 0 {
		definition += "\n\n"
		for index, item := range detail.Indexes {
			if index > 0 {
				definition += "\n"
			}
			definition += strings.TrimSuffix(item.Definition, ";") + ";"
		}
	}
	return definition
}

func ObjectStructure(detail ObjectDetail) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s  %s\nOwner  %s\n", strings.ToUpper(string(detail.Object.Kind)), detail.Object.QualifiedName(), valueOrDashText(detail.Owner))
	if len(detail.Columns) > 0 {
		output.WriteString("\nCOLUMNS\n")
		for _, column := range detail.Columns {
			nullable := "NOT NULL"
			if column.Nullable {
				nullable = "NULL"
			}
			fmt.Fprintf(&output, "  %-28s %-24s %s", column.Name, column.DataType, nullable)
			if column.Default != "" {
				fmt.Fprintf(&output, "  DEFAULT %s", column.Default)
			}
			output.WriteByte('\n')
		}
	}
	if len(detail.Constraints) > 0 {
		output.WriteString("\nCONSTRAINTS\n")
		for _, item := range detail.Constraints {
			fmt.Fprintf(&output, "  %s\n    %s\n", item.Name, item.Definition)
		}
	}
	if len(detail.Indexes) > 0 {
		output.WriteString("\nINDEXES\n")
		for _, item := range detail.Indexes {
			fmt.Fprintf(&output, "  %s\n    %s\n", item.Name, item.Definition)
		}
	}
	return strings.TrimSpace(output.String())
}

func QuoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func QuoteLiteral(value string) string { return `'` + strings.ReplaceAll(value, `'`, `''`) + `'` }

func firstCell(result model.SQLQueryResult) string {
	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return ""
	}
	return result.Rows[0][0]
}

func valueOrDashText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
