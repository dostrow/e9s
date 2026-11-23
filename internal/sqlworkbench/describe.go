package sqlworkbench

import (
	"fmt"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

// Run presentation shared by the terminal and GTK frontends.

// EntryCount is the number of entries a run shows: one per completed
// statement, plus the failed statement when there is one.
func (o RunOutcome) EntryCount() int {
	count := len(o.Results)
	if o.Failure != nil {
		count++
	}
	return count
}

// Summary is a one-line account of the run.
func (o RunOutcome) Summary() string {
	rows := 0
	for _, result := range o.Results {
		rows += len(result.Rows)
	}
	var summary string
	if o.Failure != nil {
		summary = fmt.Sprintf("Failed at statement %d of %d", o.Failure.Index+1, o.Statements)
		if o.Failure.Line > 0 {
			summary += fmt.Sprintf(" (line %d)", o.Failure.Line)
		}
		summary += fmt.Sprintf("; %d statement(s) completed", len(o.Results))
	} else {
		summary = fmt.Sprintf("Executed %d statement(s); %d result row(s)", len(o.Results), rows)
	}
	if o.Transaction == TxOpen {
		summary += "; transaction open"
	}
	return summary
}

// EntryLabel names entry index of the run, for a result selector.
func (o RunOutcome) EntryLabel(index int) string {
	total := o.Statements
	if total < o.EntryCount() {
		total = o.EntryCount()
	}
	if index < len(o.Results) {
		result := o.Results[index]
		label := fmt.Sprintf("%d/%d  %s", index+1, total, ResultTag(result))
		if first := FirstLine(result.SQL); first != "" {
			label += "  " + first
		}
		return label
	}
	if o.Failure != nil && index == len(o.Results) {
		label := fmt.Sprintf("%d/%d  ERROR", o.Failure.Index+1, total)
		if first := FirstLine(o.Failure.SQL); first != "" {
			label += "  " + first
		}
		return label
	}
	return ""
}

// ResultTag is the command tag with a row count for SELECTs.
func ResultTag(result model.SQLQueryResult) string {
	tag := strings.TrimSpace(result.CommandTag)
	if tag == "" {
		tag = "OK"
	}
	if result.Truncated {
		tag += fmt.Sprintf(" (first %d rows shown)", len(result.Rows))
	}
	return tag
}

// FirstLine returns the first line of SQL that is not blank or a comment.
func FirstLine(sql string) string {
	for _, line := range strings.Split(sql, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		return line
	}
	return ""
}

// Describe says what the tab's session is doing, for a status indicator.
func (s SessionStatus) Describe(now time.Time) string {
	if s.State != TxOpen {
		return ""
	}
	text := "TRANSACTION OPEN"
	if !s.Since.IsZero() {
		text += " for " + now.Sub(s.Since).Truncate(time.Second).String()
	}
	if s.IdleTimeout > 0 {
		text += fmt.Sprintf(" (PostgreSQL ends it after %s idle)", s.IdleTimeout)
	}
	return text
}
