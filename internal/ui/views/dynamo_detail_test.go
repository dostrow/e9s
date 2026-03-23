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
	if !strings.Contains(got, "line2") {
		t.Errorf("Should contain line2: %q", got)
	}
}

func TestFormatDetailValue_Integer(t *testing.T) {
	got := formatDetailValue(float64(42))
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestFormatDetailValue_Float(t *testing.T) {
	got := formatDetailValue(float64(3.14))
	if got != "3.14" {
		t.Errorf("got %q, want %q", got, "3.14")
	}
}

func TestFormatDetailValue_Bool(t *testing.T) {
	if got := formatDetailValue(true); got != "true" {
		t.Errorf("got %q, want %q", got, "true")
	}
	if got := formatDetailValue(false); got != "false" {
		t.Errorf("got %q, want %q", got, "false")
	}
}

func TestFormatDetailValue_Nil(t *testing.T) {
	got := formatDetailValue(nil)
	if got != "(null)" {
		t.Errorf("got %q, want %q", got, "(null)")
	}
}

func TestFormatDetailValue_Map(t *testing.T) {
	got := formatDetailValue(map[string]interface{}{"key": "val"})
	if !strings.Contains(got, "key") || !strings.Contains(got, "val") {
		t.Errorf("Should contain key and val: %q", got)
	}
	if !strings.Contains(got, "{") {
		t.Errorf("Should be formatted as JSON-like: %q", got)
	}
}

func TestFormatDetailValue_Slice(t *testing.T) {
	got := formatDetailValue([]interface{}{"a", "b"})
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("Should contain elements: %q", got)
	}
	if !strings.Contains(got, "[") {
		t.Errorf("Should be formatted as array: %q", got)
	}
}

func TestFormatItemForDetail_ShortValues(t *testing.T) {
	item := aws.DynamoItem{
		"id":   "123",
		"name": "Alice",
	}
	lines := formatItemForDetail(item)

	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d", len(lines))
	}
	// Short values should be inline with key
	foundInline := false
	for _, line := range lines {
		if strings.Contains(line, "123") || strings.Contains(line, "Alice") {
			foundInline = true
		}
	}
	if !foundInline {
		t.Error("Short values should appear inline with their keys")
	}
}

func TestFormatItemForDetail_MultilineValue(t *testing.T) {
	item := aws.DynamoItem{
		"config": "line1\nline2\nline3",
	}
	lines := formatItemForDetail(item)

	// Multi-line value should have key on its own line
	foundKey := false
	foundValue := false
	for _, line := range lines {
		if strings.Contains(line, "config") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			foundKey = true
		}
		if strings.Contains(line, "line2") {
			foundValue = true
		}
	}
	// Key should be on its own line (the styled version ends with ":")
	if !foundKey {
		t.Error("Multi-line value should have key on its own line ending with ':'")
	}
	if !foundValue {
		t.Error("Multi-line value lines should appear in output")
	}
}

func TestFormatItemForDetail_LongValue(t *testing.T) {
	item := aws.DynamoItem{
		"data": strings.Repeat("x", 100),
	}
	lines := formatItemForDetail(item)

	// Long value should be on its own line below the key
	if len(lines) < 2 {
		t.Error("Long value should produce at least 2 lines (key + value)")
	}
}

func TestFormatMapIndented(t *testing.T) {
	m := map[string]interface{}{
		"a": "1",
		"b": "2",
	}
	got := formatMapIndented(m, "  ")
	if !strings.HasPrefix(got, "{") {
		t.Errorf("Should start with '{': %q", got)
	}
	if !strings.Contains(got, `"a"`) {
		t.Errorf("Should contain key 'a': %q", got)
	}
}

func TestFormatMapIndented_Empty(t *testing.T) {
	got := formatMapIndented(map[string]interface{}{}, "")
	if got != "{}" {
		t.Errorf("Empty map should be '{}', got %q", got)
	}
}

func TestFormatSliceIndented(t *testing.T) {
	s := []interface{}{"x", "y"}
	got := formatSliceIndented(s, "  ")
	if !strings.HasPrefix(got, "[") {
		t.Errorf("Should start with '[': %q", got)
	}
}

func TestFormatSliceIndented_Empty(t *testing.T) {
	got := formatSliceIndented([]interface{}{}, "")
	if got != "[]" {
		t.Errorf("Empty slice should be '[]', got %q", got)
	}
}
