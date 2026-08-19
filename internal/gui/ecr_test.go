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
