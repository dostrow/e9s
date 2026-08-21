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
