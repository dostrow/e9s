package views

import (
	"strings"
	"testing"
)

func TestFormatImageSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero returns dash", 0, "-"},
		{"small MB", 1024 * 1024, "1.0 MB"},
		{"large MB", 500 * 1024 * 1024, "500.0 MB"},
		{"exactly 1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"fractional GB", int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatImageSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatImageSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestVulnSummaryCell_Empty(t *testing.T) {
	cell := vulnSummaryCell(nil)
	if cell.Content != "-" {
		t.Errorf("empty counts: Content = %q, want %q", cell.Content, "-")
	}
}

func TestVulnSummaryCell_EmptyMap(t *testing.T) {
	cell := vulnSummaryCell(map[string]int32{})
	if cell.Content != "-" {
		t.Errorf("empty map: Content = %q, want %q", cell.Content, "-")
	}
}

func TestVulnSummaryCell_Clean(t *testing.T) {
	// All zero counts — should show "clean"
	counts := map[string]int32{
		"INFORMATIONAL": 2,
	}
	cell := vulnSummaryCell(counts)
	if !strings.Contains(cell.Content, "clean") {
		t.Errorf("all-zero counts: Content = %q, want it to contain %q", cell.Content, "clean")
	}
}

func TestVulnSummaryCell_WithCritical(t *testing.T) {
	counts := map[string]int32{
		"CRITICAL": 3,
		"HIGH":     5,
	}
	cell := vulnSummaryCell(counts)
	if !strings.Contains(cell.Content, "C:3") {
		t.Errorf("Content = %q, want it to contain %q", cell.Content, "C:3")
	}
	if !strings.Contains(cell.Content, "H:5") {
		t.Errorf("Content = %q, want it to contain %q", cell.Content, "H:5")
	}
}

func TestVulnSummaryCell_WithMediumAndLow(t *testing.T) {
	counts := map[string]int32{
		"MEDIUM": 2,
		"LOW":    10,
	}
	cell := vulnSummaryCell(counts)
	if !strings.Contains(cell.Content, "M:2") {
		t.Errorf("Content = %q, want it to contain %q", cell.Content, "M:2")
	}
	if !strings.Contains(cell.Content, "L:10") {
		t.Errorf("Content = %q, want it to contain %q", cell.Content, "L:10")
	}
}

func TestVulnSummaryCell_ZeroCountsExcluded(t *testing.T) {
	// Zero counts should not appear in the summary
	counts := map[string]int32{
		"CRITICAL": 0,
		"HIGH":     1,
	}
	cell := vulnSummaryCell(counts)
	if strings.Contains(cell.Content, "C:0") {
		t.Errorf("zero critical count should not appear: Content = %q", cell.Content)
	}
	if !strings.Contains(cell.Content, "H:1") {
		t.Errorf("Content = %q, want it to contain %q", cell.Content, "H:1")
	}
}
