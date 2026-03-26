package aws

import (
	"testing"
	"time"
)

func TestSeverityOrder(t *testing.T) {
	tests := []struct {
		severity string
		want     int
	}{
		{"CRITICAL", 0},
		{"HIGH", 1},
		{"MEDIUM", 2},
		{"LOW", 3},
		{"INFORMATIONAL", 4},
		{"UNDEFINED", 5},
		{"UNKNOWN", 9},
		{"", 9},
	}
	for _, tt := range tests {
		got := severityOrder(tt.severity)
		if got != tt.want {
			t.Errorf("severityOrder(%q) = %d, want %d", tt.severity, got, tt.want)
		}
	}
}

func TestSortFindingsBySeverity(t *testing.T) {
	findings := []ECRFinding{
		{Name: "low-vuln", Severity: "LOW"},
		{Name: "critical-vuln", Severity: "CRITICAL"},
		{Name: "medium-vuln", Severity: "MEDIUM"},
		{Name: "high-vuln", Severity: "HIGH"},
	}
	sortFindingsBySeverity(findings)

	wantOrder := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}
	for i, want := range wantOrder {
		if findings[i].Severity != want {
			t.Errorf("findings[%d].Severity = %q, want %q", i, findings[i].Severity, want)
		}
	}
}

func TestSortFindingsBySeverity_Empty(t *testing.T) {
	var findings []ECRFinding
	sortFindingsBySeverity(findings) // should not panic
}

func TestSortFindingsBySeverity_SingleElement(t *testing.T) {
	findings := []ECRFinding{{Name: "only", Severity: "HIGH"}}
	sortFindingsBySeverity(findings)
	if findings[0].Severity != "HIGH" {
		t.Errorf("unexpected severity: %q", findings[0].Severity)
	}
}

func TestSortImagesByPushDate(t *testing.T) {
	now := time.Now()
	images := []ECRImage{
		{Digest: "oldest", PushedAt: now.Add(-72 * time.Hour)},
		{Digest: "newest", PushedAt: now},
		{Digest: "middle", PushedAt: now.Add(-24 * time.Hour)},
	}
	sortImagesByPushDate(images)

	wantOrder := []string{"newest", "middle", "oldest"}
	for i, want := range wantOrder {
		if images[i].Digest != want {
			t.Errorf("images[%d].Digest = %q, want %q", i, images[i].Digest, want)
		}
	}
}

func TestSortImagesByPushDate_Empty(t *testing.T) {
	var images []ECRImage
	sortImagesByPushDate(images) // should not panic
}

func TestECRImageURI(t *testing.T) {
	tests := []struct {
		repoURI string
		tag     string
		want    string
	}{
		{"123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo", "latest", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest"},
		{"123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo", "v1.2.3", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:v1.2.3"},
		{"123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo", "", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo"},
	}
	for _, tt := range tests {
		got := ECRImageURI(tt.repoURI, tt.tag)
		if got != tt.want {
			t.Errorf("ECRImageURI(%q, %q) = %q, want %q", tt.repoURI, tt.tag, got, tt.want)
		}
	}
}
