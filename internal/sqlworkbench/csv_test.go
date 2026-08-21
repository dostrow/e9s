package sqlworkbench

import (
	"bytes"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestWriteCSV(t *testing.T) {
	var output bytes.Buffer
	err := WriteCSV(&output, model.SQLQueryResult{Columns: []string{"id", "note"}, Rows: [][]string{{"1", "a,b"}}})
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != "id,note\n1,\"a,b\"\n" {
		t.Fatalf("CSV = %q", output.String())
	}
}
