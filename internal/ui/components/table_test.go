package components

import (
	"strings"
	"testing"
)

func TestNewTable(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
		{Title: "VALUE", RightAlign: true},
	})

	if len(tbl.columns) != 2 {
		t.Fatalf("columns count = %d, want 2", len(tbl.columns))
	}
	if tbl.colWidth[0] < 4 { // "NAME" is 4 chars
		t.Errorf("colWidth[0] = %d, want >= 4", tbl.colWidth[0])
	}
}

func TestAddRow_ExpandsWidth(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "ID"},
	})

	initialWidth := tbl.colWidth[0]

	tbl.AddRow(Plain("a-very-long-identifier"))

	if tbl.colWidth[0] <= initialWidth {
		t.Error("Column width should expand for longer data")
	}
}

func TestAddRow_MinWidth(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "X", MinWidth: 20},
	})

	if tbl.colWidth[0] < 20 {
		t.Errorf("colWidth = %d, want >= 20", tbl.colWidth[0])
	}
}

func TestRender_HasBorders(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
	})
	tbl.AddRow(Plain("test"))

	output := tbl.Render(0, "", 0)

	if !strings.Contains(output, "╭") {
		t.Error("Expected top border")
	}
	if !strings.Contains(output, "╰") {
		t.Error("Expected bottom border")
	}
	if !strings.Contains(output, "│") {
		t.Error("Expected column separators")
	}
	if !strings.Contains(output, "├") {
		t.Error("Expected header separator")
	}
}

func TestRender_CursorMarker(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
	})
	tbl.AddRow(Plain("first"))
	tbl.AddRow(Plain("second"))

	output := tbl.Render(1, "", 0)

	// The cursor row should have ► marker
	lines := strings.Split(output, "\n")
	foundCursor := false
	for _, line := range lines {
		if strings.Contains(line, "►") && strings.Contains(line, "second") {
			foundCursor = true
		}
	}
	if !foundCursor {
		t.Error("Expected cursor ► on selected row")
	}
}

func TestRender_MaxRows(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
	})
	for i := 0; i < 20; i++ {
		tbl.AddRow(Plain("item"))
	}

	output := tbl.Render(0, "", 5)

	// Should show scroll indicator
	if !strings.Contains(output, "of 20") {
		t.Error("Expected scroll indicator showing total count")
	}
}

func TestRender_NoCursor(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
	})
	tbl.AddRow(Plain("test"))

	output := tbl.Render(-1, "", 0)

	if strings.Contains(output, "►") {
		t.Error("Should not have cursor marker when cursorIdx = -1")
	}
}

func TestWidth(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "NAME"},
		{Title: "VALUE"},
	})
	tbl.AddRow(Plain("test"), Plain("value"))

	w := tbl.Width()
	if w < 15 {
		t.Errorf("Width = %d, seems too small", w)
	}
}

func TestPlainAndStyled(t *testing.T) {
	p := Plain("hello")
	if p.Content != "hello" {
		t.Errorf("Plain content = %q", p.Content)
	}
}
