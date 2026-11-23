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
