package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/aws"
)

func TestFormatDetailValue_String(t *testing.T) {
	got := formatDetailValue("hello world")
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestFormatDetailValue_MultilineString(t *testing.T) {
	got := formatDetailValue("line1\nline2\nline3")
	if !strings.Contains(got, "\n") {
		t.Error("Multi-line strings should preserve newlines")
	}
}

func TestFormatDetailValue_Integer(t *testing.T) {
	if got := formatDetailValue(float64(42)); got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestFormatDetailValue_Float(t *testing.T) {
	if got := formatDetailValue(float64(3.14)); got != "3.14" {
		t.Errorf("got %q, want %q", got, "3.14")
	}
}

func TestFormatDetailValue_Bool(t *testing.T) {
	if got := formatDetailValue(true); got != "true" {
		t.Errorf("got %q", got)
	}
	if got := formatDetailValue(false); got != "false" {
		t.Errorf("got %q", got)
	}
}

func TestFormatDetailValue_Nil(t *testing.T) {
	if got := formatDetailValue(nil); got != "(null)" {
		t.Errorf("got %q", got)
	}
}

func TestFormatDetailValue_Map(t *testing.T) {
	got := formatDetailValue(map[string]interface{}{"key": "val"})
	if !strings.Contains(got, "key") {
		t.Errorf("Should contain key: %q", got)
	}
}

func TestFormatDetailValue_Slice(t *testing.T) {
	got := formatDetailValue([]interface{}{"a", "b"})
	if !strings.Contains(got, "[") {
		t.Errorf("Should start with '[': %q", got)
	}
}

func TestBuildFieldEntries_ShortValues(t *testing.T) {
	item := aws.DynamoItem{"id": "123", "name": "Alice"}
	fields := buildFieldEntries(item, nil)

	if len(fields) < 2 {
		t.Fatalf("Expected at least 2 fields, got %d", len(fields))
	}
	// Short values should have 1 line each (inline)
	for _, f := range fields {
		if len(f.lines) != 1 {
			t.Errorf("Field %q should have 1 line, got %d", f.key, len(f.lines))
		}
	}
}

func TestBuildFieldEntries_MultilineValue(t *testing.T) {
	item := aws.DynamoItem{"config": "line1\nline2\nline3"}
	fields := buildFieldEntries(item, nil)

	if len(fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(fields))
	}
	// Multi-line value: key line + 3 value lines = 4
	if len(fields[0].lines) < 4 {
		t.Errorf("Expected at least 4 lines for multiline value, got %d", len(fields[0].lines))
	}
}

func TestBuildFieldEntries_KeysFirst(t *testing.T) {
	item := aws.DynamoItem{"data": "x", "PK": "user1", "SK": "profile", "name": "Alice"}
	fields := buildFieldEntries(item, []string{"PK", "SK"})

	if len(fields) < 4 {
		t.Fatalf("Expected 4 fields, got %d", len(fields))
	}
	if fields[0].key != "PK" {
		t.Errorf("First field should be PK, got %q", fields[0].key)
	}
	if fields[1].key != "SK" {
		t.Errorf("Second field should be SK, got %q", fields[1].key)
	}
	if !fields[0].isKey {
		t.Error("PK should be marked as key")
	}
	if !fields[1].isKey {
		t.Error("SK should be marked as key")
	}
	if fields[2].isKey {
		t.Error("Third field should not be a key")
	}
}

func TestBuildFieldEntries_LongValue(t *testing.T) {
	item := aws.DynamoItem{"data": strings.Repeat("x", 100)}
	fields := buildFieldEntries(item, nil)

	// Long value should have key on own line + value on next line = 2
	if len(fields[0].lines) < 2 {
		t.Errorf("Expected at least 2 lines for long value, got %d", len(fields[0].lines))
	}
}

func TestFormatMapIndented(t *testing.T) {
	got := formatMapIndented(map[string]interface{}{"a": "1"}, "  ")
	if !strings.HasPrefix(got, "{") {
		t.Errorf("Should start with '{': %q", got)
	}
}

func TestFormatMapIndented_Empty(t *testing.T) {
	if got := formatMapIndented(map[string]interface{}{}, ""); got != "{}" {
		t.Errorf("got %q, want %q", got, "{}")
	}
}

func TestFormatSliceIndented(t *testing.T) {
	got := formatSliceIndented([]interface{}{"x"}, "  ")
	if !strings.HasPrefix(got, "[") {
		t.Errorf("Should start with '[': %q", got)
	}
}

func TestFormatSliceIndented_Empty(t *testing.T) {
	if got := formatSliceIndented([]interface{}{}, ""); got != "[]" {
		t.Errorf("got %q, want %q", got, "[]")
	}
}
