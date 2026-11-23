package sqlworkbench

import "testing"

func TestSplitStatementsIgnoresQuotedSemicolons(t *testing.T) {
	source := "select ';' as value; -- ;\nselect $$a;b$$;"
	statements := SplitStatements(source)
	if len(statements) != 2 {
		t.Fatalf("got %d statements: %#v", len(statements), statements)
	}
}

func TestValidateReadOnlyRejectsWritesInCTE(t *testing.T) {
	err := ValidateReadOnly("with changed as (delete from jobs returning *) select * from changed")
	if err == nil {
		t.Fatal("expected data-modifying CTE to fail")
	}
}

func TestValidateReadOnlyIgnoresKeywordsInStringsAndComments(t *testing.T) {
	if err := ValidateReadOnly("select 'delete' as word -- update\n"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReadOnlyAllowsKeywordColumnNames(t *testing.T) {
	if err := ValidateReadOnly("select update, analyze from records"); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentStatement(t *testing.T) {
	source := "select 1;\nselect 2;"
	statement, ok := CurrentStatement(source, 15)
	if !ok || statement.SQL != "select 2" {
		t.Fatalf("unexpected statement: %#v, %t", statement, ok)
	}
}

func TestValidateReadOnlyAllowsEndingATransaction(t *testing.T) {
	for _, sql := range []string{"rollback", "ROLLBACK TO SAVEPOINT before_update", "abort"} {
		if err := ValidateReadOnly(sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	for _, sql := range []string{"commit", "begin", "rollback prepared 'tx1'"} {
		if err := ValidateReadOnly(sql); err == nil {
			t.Fatalf("%s passed the read-only check", sql)
		}
	}
}

func TestIsTransactionControl(t *testing.T) {
	for sql, want := range map[string]bool{
		"BEGIN": true, "start transaction read only": true, "-- note\ncommit": true, "END": true,
		"savepoint a": true, "release savepoint a": true, "prepare transaction 'x'": true,
		"select 'begin'": false, "prepare plan as select 1": false, "start": false, "update t set x = 1": false,
	} {
		if got := IsTransactionControl(sql); got != want {
			t.Fatalf("IsTransactionControl(%q) = %v, want %v", sql, got, want)
		}
	}
}

func TestValidateReadOnlyRefusesWaysAroundTheServerLock(t *testing.T) {
	for _, sql := range []string{
		"select set_config('default_transaction_read_only', 'off', false)",
		"SELECT pg_catalog.set_config('transaction_read_only', 'off', true)",
		"explain analyse delete from jobs",
		"with recent as (select 1) select * into copied from recent",
	} {
		if err := ValidateReadOnly(sql); err == nil {
			t.Fatalf("%s passed the read-only check", sql)
		}
	}
	if err := ValidateReadOnly("select current_setting('default_transaction_read_only')"); err != nil {
		t.Fatalf("reading a setting is read-only: %v", err)
	}
}

func TestSplitStatementsUnderstandsEscapeStringsAndDollarsInNames(t *testing.T) {
	for source, want := range map[string]int{
		`SELECT E'it\'s; x'; SELECT 2`:                      2,
		`SELECT E'\'' ; DELETE FROM t; SELECT 'a'`:          3,
		`SELECT 1 AS a$b$; DELETE FROM t; SELECT 1 AS c$b$`: 3,
		`SELECT 'a\'; SELECT 2`:                             2, // standard strings keep backslashes
		`SELECT $tag$ ; $tag$; SELECT 2`:                    2,
		`SELECT be'x' FROM t; SELECT 2`:                     2, // be'x' is not an escape string
	} {
		if got := len(SplitStatements(source)); got != want {
			t.Fatalf("%s: %d statements, want %d: %#v", source, got, want, SplitStatements(source))
		}
	}
	if err := ValidateReadOnly(`SELECT E'\'' ; DELETE FROM t; SELECT 'a'`); err == nil {
		t.Fatal("a DELETE after an escape string passed the read-only check")
	}
}
