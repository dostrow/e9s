package aws

import (
	"testing"
	"time"

	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/dostrow/e9s/internal/model"
)

func TestEnhancedECRFindingFromSDK(t *testing.T) {
	title, severity := "CVE fallback title", "CRITICAL"
	vulnerability, source, description := "CVE-2026-1234", "https://example.test/CVE-2026-1234", "important vulnerability"
	packageName, version := "openssl", "3.1.0"
	finding := enhancedECRFindingFromSDK(ecrtypes.EnhancedImageScanFinding{
		Title:       &title,
		Severity:    &severity,
		Description: &description,
		PackageVulnerabilityDetails: &ecrtypes.PackageVulnerabilityDetails{
			VulnerabilityId: &vulnerability,
			SourceUrl:       &source,
			VulnerablePackages: []ecrtypes.VulnerablePackage{{
				Name: &packageName, Version: &version,
			}},
		},
	})
	if finding.Name != vulnerability || finding.Severity != severity || finding.Package != packageName ||
		finding.Version != version || finding.URI != source || finding.Description != description {
		t.Fatalf("enhanced finding = %#v", finding)
	}
}

func TestEnhancedECRFindingFallsBackToInspectorMetadata(t *testing.T) {
	title, arn := "Inspector finding", "arn:aws:inspector2:us-east-2:123456789012:finding/one"
	finding := enhancedECRFindingFromSDK(ecrtypes.EnhancedImageScanFinding{Title: &title, FindingArn: &arn})
	if finding.Name != title || finding.URI != arn {
		t.Fatalf("enhanced finding fallback = %#v", finding)
	}
}

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
	findings := []model.ECRFinding{
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
	var findings []model.ECRFinding
	sortFindingsBySeverity(findings) // should not panic
}

func TestSortFindingsBySeverity_SingleElement(t *testing.T) {
	findings := []model.ECRFinding{{Name: "only", Severity: "HIGH"}}
	sortFindingsBySeverity(findings)
	if findings[0].Severity != "HIGH" {
		t.Errorf("unexpected severity: %q", findings[0].Severity)
	}
}

func TestSortImagesByPushDate(t *testing.T) {
	now := time.Now()
	images := []model.ECRImage{
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
	var images []model.ECRImage
	sortImagesByPushDate(images) // should not panic
}
