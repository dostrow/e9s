package sqlworkbench

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dostrow/e9s/internal/model"
)

// ReportContext identifies a run in its report.
type ReportContext struct {
	Connection string
	Database   string
	Tab        string
	Started    time.Time
}

// FormatReport renders a run as plain text in the style of psql output:
// each statement with its line, then its result table or command tag, then
// the failure and what e9s did on the session's behalf. It is meant to be
// read in full or saved next to the script it checked.
func FormatReport(outcome RunOutcome, context ReportContext) string {
	var b strings.Builder
	b.WriteString("-- e9s SQL Workbench report\n")
	if context.Connection != "" {
		fmt.Fprintf(&b, "-- Connection: %s", context.Connection)
		if context.Database != "" {
			fmt.Fprintf(&b, " (database %s)", context.Database)
		}
		b.WriteString("\n")
	}
	if context.Tab != "" {
		fmt.Fprintf(&b, "-- Tab: %s\n", context.Tab)
	}
	if !context.Started.IsZero() {
		fmt.Fprintf(&b, "-- Started: %s\n", context.Started.UTC().Format("2006-01-02 15:04:05 MST"))
	}
	fmt.Fprintf(&b, "-- Mode: %s\n", outcome.Mode)
	fmt.Fprintf(&b, "-- %s\n", outcome.Summary())
	if changes := outcome.ChangeSummary(); changes != "" {
		fmt.Fprintf(&b, "-- Changes: %s\n", changes)
	}
	total := max(outcome.Statements, outcome.EntryCount())
	for index, result := range outcome.Results {
		b.WriteString("\n")
		writeStatementHeading(&b, index, total, result.Line, result.SQL)
		writeResult(&b, result)
	}
	if failure := outcome.Failure; failure != nil {
		b.WriteString("\n")
		if failure.Index >= 0 {
			writeStatementHeading(&b, failure.Index, total, failure.Line, failure.SQL)
		}
		b.WriteString(failure.Message + "\n")
	}
	if len(outcome.Events) > 0 {
		b.WriteString("\n")
		for _, event := range outcome.Events {
			b.WriteString("-- e9s: " + event + "\n")
		}
	}
	return b.String()
}

func writeStatementHeading(b *strings.Builder, index, total, line int, sql string) {
	heading := fmt.Sprintf("-- [%d/%d]", index+1, total)
	if line > 0 {
		heading += fmt.Sprintf(" line %d", line)
	}
	b.WriteString(heading + "\n")
	if sql = strings.TrimSpace(sql); sql != "" {
		b.WriteString(sql + ";\n")
	}
}

func writeResult(b *strings.Builder, result model.SQLQueryResult) {
	if len(result.Columns) == 0 {
		b.WriteString(ResultTag(result) + "\n")
		return
	}
	widths := make([]int, len(result.Columns))
	for index, column := range result.Columns {
		widths[index] = utf8.RuneCountInString(column)
	}
	cells := make([][]string, len(result.Rows))
	for row, values := range result.Rows {
		cells[row] = make([]string, len(result.Columns))
		for index := range result.Columns {
			value := ""
			if index < len(values) {
				value = reportCell(values[index])
			}
			cells[row][index] = value
			widths[index] = max(widths[index], utf8.RuneCountInString(value))
		}
	}
	writeRow(b, result.Columns, widths)
	separators := make([]string, len(widths))
	for index, width := range widths {
		separators[index] = strings.Repeat("-", width+2)
	}
	b.WriteString(strings.Join(separators, "+") + "\n")
	for _, row := range cells {
		writeRow(b, row, widths)
	}
	rows := len(result.Rows)
	noun := "rows"
	if rows == 1 {
		noun = "row"
	}
	if result.Truncated {
		fmt.Fprintf(b, "(first %d %s shown; more were returned)\n", rows, noun)
	} else {
		fmt.Fprintf(b, "(%d %s)\n", rows, noun)
	}
}

func writeRow(b *strings.Builder, values []string, widths []int) {
	cells := make([]string, len(widths))
	for index, width := range widths {
		value := ""
		if index < len(values) {
			value = values[index]
		}
		cells[index] = " " + value + strings.Repeat(" ", width-utf8.RuneCountInString(value)) + " "
	}
	b.WriteString(strings.TrimRight(strings.Join(cells, "|"), " ") + "\n")
}

// reportCell keeps a value on one line so the table stays aligned.
func reportCell(value string) string {
	return strings.NewReplacer("\r\n", "↵", "\n", "↵", "\r", "", "\t", "    ").Replace(value)
}

// ChangeSummary lists the statements that reported changed rows, such as
// "line 12: UPDATE 6; line 20: DELETE 2", for confirming a commit.
func (o RunOutcome) ChangeSummary() string {
	var changes []string
	for _, result := range o.Results {
		words := strings.Fields(result.CommandTag)
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "COPY":
		default:
			// The Data API reports "N rows affected" instead of a tag.
			if len(result.Columns) > 0 || result.RowsAffected == 0 || !strings.HasSuffix(result.CommandTag, "rows affected") {
				continue
			}
		}
		change := result.CommandTag
		if result.Line > 0 {
			change = fmt.Sprintf("line %d: %s", result.Line, change)
		}
		changes = append(changes, change)
	}
	return strings.Join(changes, "; ")
}
