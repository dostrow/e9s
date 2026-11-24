package sqlworkbench

import (
	"strings"
	"testing"
)

func TestParseScriptTurnsEchoIntoHeadingsAndIgnoresPsqlSettings(t *testing.T) {
	source := `-- written for psql
\set ON_ERROR_STOP on
\pset pager off

BEGIN;
\echo '== Study links to restore'
SELECT 1;
\echo '== It''s' "two" words
\qecho second line
SELECT 2;
ROLLBACK;
\echo done`
	script, err := ParseScript(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Statements) != 4 || script.Statements[0].SQL != "BEGIN" {
		t.Fatalf("statements = %#v", script.Statements)
	}
	if got := script.Statements[1].Heading; got != "== Study links to restore" {
		t.Fatalf("heading = %q", got)
	}
	if got := script.Statements[2].Heading; got != "== It's \"two\" words\nsecond line" {
		t.Fatalf("heading = %q", got)
	}
	if script.Statements[0].Heading != "" || script.TrailingHeading != "done" {
		t.Fatalf("headings misplaced: %#v trailing=%q", script.Statements, script.TrailingHeading)
	}
	if event := script.IgnoredEvent(); !strings.Contains(event, `\set ON_ERROR_STOP on; \pset pager off`) {
		t.Fatalf("ignored event = %q", event)
	}
	if StatementLine(source, script.Statements[1].Start) != 7 {
		t.Fatalf("line of SELECT 1 = %d", StatementLine(source, script.Statements[1].Start))
	}
}

func TestParseScriptRefusesWhatItCannotHonour(t *testing.T) {
	for source, want := range map[string]string{
		"SELECT 1;\n\\i other.sql":             `line 2: e9s does not support the psql command \i`,
		"SELECT 1 \\gset":                      "follows a statement that has no semicolon",
		"\\set ON_ERROR_STOP off\nSELECT 1;":   "cannot be turned off",
		"\\set customer 42\nSELECT :customer;": "psql variables are not supported",
		"\\echo `date`\nSELECT 1;":             "backquoted shell commands",
		"\\copy t from 'x.csv'":                `\copy`,
	} {
		_, err := ParseScript(source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: error = %v, want %q", source, err, want)
		}
	}
}

func TestSplitStatementsKeepsBackslashesInsideSQL(t *testing.T) {
	source := "SELECT E'a\\\\b', '\\echo';\n-- \\echo in a comment\nSELECT $$\\set x$$;"
	statements := SplitStatements(source)
	if len(statements) != 2 || !strings.Contains(statements[1].SQL, "$$\\set x$$") {
		t.Fatalf("statements = %#v", statements)
	}
	if _, err := ParseScript(source); err != nil {
		t.Fatalf("backslashes in strings, comments and dollar quotes are not psql commands: %v", err)
	}
}

func TestSplitStatementsDropsCommentOnlyChunks(t *testing.T) {
	statements := SplitStatements("SELECT 1; -- done\n/* trailing */")
	if len(statements) != 1 {
		t.Fatalf("statements = %#v", statements)
	}
	if err := ValidateReadOnly("\\echo heading\nSELECT 1;"); err != nil {
		t.Fatalf("an \\echo line made a read-only script look like a write: %v", err)
	}
}
