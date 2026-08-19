//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterECRResources(t *testing.T) {
	repositories := []model.ECRRepo{
		{Name: "payments-api", URI: "123.dkr.ecr.us-east-2.amazonaws.com/payments-api"},
		{Name: "worker", URI: "123.dkr.ecr.us-east-2.amazonaws.com/worker"},
	}
	if got := filterECRRepositories(repositories, "PAYMENTS"); len(got) != 1 || got[0].Name != "payments-api" {
		t.Fatalf("filter repositories = %#v", got)
	}

	images := []model.ECRImage{{Digest: "sha256:one", Tags: []string{"stable"}}, {Digest: "sha256:two", Tags: []string{"canary"}}}
	if got := filterECRImages(images, "CANARY"); len(got) != 1 || got[0].Digest != "sha256:two" {
		t.Fatalf("filter images = %#v", got)
	}

	findings := []model.ECRFinding{{Name: "CVE-1", Severity: "HIGH", Package: "openssl"}, {Name: "CVE-2", Severity: "LOW", Package: "curl"}}
	if got := filterECRFindings(findings, "OPENSSL"); len(got) != 1 || got[0].Name != "CVE-1" {
		t.Fatalf("filter findings = %#v", got)
	}
}

func TestFormatECRImageIncludesScanSummary(t *testing.T) {
	text := formatECRImageWithFindingCount("payments-api", model.ECRImage{
		Digest:       "sha256:0123456789",
		Tags:         []string{"stable", "latest"},
		ScanStatus:   "COMPLETE",
		ScanSeverity: map[string]int32{"CRITICAL": 1, "HIGH": 4},
	}, 7)
	for _, want := range []string{"payments-api", "latest, stable", "COMPLETE", "Findings      7", "CRITICAL        1", "HIGH            4"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatted image missing %q:\n%s", want, text)
		}
	}
}

func TestECRImageLabelFallsBackToDigest(t *testing.T) {
	if got := ecrImageLabel(model.ECRImage{Digest: "sha256:01234567890123456789"}); got != "sha256:012345678901…" {
		t.Fatalf("image label = %q", got)
	}
}

func TestCanStartECRScan(t *testing.T) {
	for _, status := range []string{"ACTIVE", "PENDING", "IN_PROGRESS", "pending"} {
		if canStartECRScan(model.ECRImage{Digest: "sha256:one", ScanStatus: status}) {
			t.Errorf("scan with status %q should be disabled", status)
		}
	}
	if !canStartECRScan(model.ECRImage{Digest: "sha256:one", ScanStatus: "COMPLETE"}) {
		t.Error("completed image should permit another scan")
	}
	if canStartECRScan(model.ECRImage{}) {
		t.Error("image without a digest should not permit a scan")
	}
}

func TestMergeECRScanSummary(t *testing.T) {
	w := &mainWindow{allECRImages: []model.ECRImage{{Digest: "sha256:one"}}}
	image := w.mergeECRScanSummary(w.allECRImages[0], model.ECRScan{
		Status: "ACTIVE", Severity: map[string]int32{"CRITICAL": 2, "HIGH": 1},
	})
	if image.ScanSeverity["CRITICAL"] != 2 || image.ScanSeverity["HIGH"] != 1 {
		t.Fatalf("merged summary = %#v", image.ScanSeverity)
	}
	if image.ScanStatus != "ACTIVE" {
		t.Fatalf("merged status = %q", image.ScanStatus)
	}
	if w.allECRImages[0].ScanSeverity["CRITICAL"] != 2 {
		t.Fatalf("cached summary = %#v", w.allECRImages[0].ScanSeverity)
	}
}

func TestECRUnknownAndLoadedSeverityPresentation(t *testing.T) {
	unknown := model.ECRImage{Digest: "sha256:one"}
	if got := ecrCriticalHighSummary(unknown); got != "—" {
		t.Fatalf("unknown summary = %q", got)
	}
	if text := formatECRImage("repo", unknown); !strings.Contains(text, "Not loaded") || strings.Contains(text, "CRITICAL        0") {
		t.Fatalf("unknown detail is misleading:\n%s", text)
	}
	loaded := model.ECRImage{Digest: "sha256:one", ScanSeverity: map[string]int32{"CRITICAL": 2, "HIGH": 5}}
	if got := ecrCriticalHighSummary(loaded); got != "2 / 5" {
		t.Fatalf("loaded summary = %q", got)
	}
}
