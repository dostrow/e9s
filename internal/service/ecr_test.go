package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeECRAPI struct {
	repositories []model.ECRRepo
	images       []model.ECRImage
	findings     []model.ECRFinding
	scan         model.ECRScan
	err          error
	repository   string
	digest       string
	tags         []string
	deleted      string
}

func (f *fakeECRAPI) ListECRRepos(context.Context, string) ([]model.ECRRepo, error) {
	return append([]model.ECRRepo(nil), f.repositories...), f.err
}
func (f *fakeECRAPI) ListECRImages(_ context.Context, repository string) ([]model.ECRImage, error) {
	f.repository = repository
	return append([]model.ECRImage(nil), f.images...), f.err
}
func (f *fakeECRAPI) GetECRScanFindings(_ context.Context, repository, digest string) (model.ECRScan, error) {
	f.repository, f.digest = repository, digest
	scan := f.scan
	if scan.Findings == nil {
		scan.Findings = append([]model.ECRFinding(nil), f.findings...)
	}
	return scan, f.err
}
func (f *fakeECRAPI) StartECRScan(_ context.Context, repository, digest string, tags []string) error {
	f.repository, f.digest, f.tags = repository, digest, append([]string(nil), tags...)
	return f.err
}
func (f *fakeECRAPI) DeleteECRImage(_ context.Context, repository, digest string) error {
	f.repository, f.deleted = repository, digest
	return f.err
}

func TestECRRepositoriesFilterAndSort(t *testing.T) {
	api := &fakeECRAPI{repositories: []model.ECRRepo{{Name: "worker"}, {Name: "API"}, {Name: "api-canary"}}}
	got, err := NewECR(api).ListRepositories(context.Background(), " api ")
	if err != nil || len(got) != 2 || got[0].Name != "API" || got[1].Name != "api-canary" {
		t.Fatalf("ListRepositories() = %#v, %v", got, err)
	}
}

func TestECRImagesAndFindingsOrdering(t *testing.T) {
	now := time.Now()
	api := &fakeECRAPI{
		images: []model.ECRImage{{Digest: "old", PushedAt: now.Add(-time.Hour)}, {Digest: "new", PushedAt: now}},
		findings: []model.ECRFinding{
			{Name: "z-low", Severity: "LOW"}, {Name: "z-critical", Severity: "CRITICAL"}, {Name: "a-critical", Severity: "CRITICAL"},
		},
	}
	svc := NewECR(api)
	images, err := svc.ListImages(context.Background(), " repo ")
	if err != nil || api.repository != "repo" || images[0].Digest != "new" {
		t.Fatalf("ListImages() = %#v, repository %q, %v", images, api.repository, err)
	}
	findings, err := svc.Findings(context.Background(), "repo", "sha256:1")
	if err != nil || findings[0].Name != "a-critical" || findings[1].Name != "z-critical" || findings[2].Severity != "LOW" {
		t.Fatalf("Findings() = %#v, %v", findings, err)
	}
	scan, err := svc.ScanFindings(context.Background(), "repo", "sha256:1")
	if err != nil || scan.Severity["CRITICAL"] != 2 || scan.Findings[0].Name != "a-critical" {
		t.Fatalf("ScanFindings() = %#v, %v", scan, err)
	}
}

func TestECRMutationsValidationAndURI(t *testing.T) {
	api := &fakeECRAPI{}
	svc := NewECR(api)
	image := model.ECRImage{Digest: "sha256:1", Tags: []string{"latest"}}
	if err := svc.StartScan(context.Background(), " repo ", image); err != nil || api.repository != "repo" || api.digest != image.Digest {
		t.Fatalf("StartScan() repository %q digest %q error %v", api.repository, api.digest, err)
	}
	if err := svc.StartScan(context.Background(), "repo", model.ECRImage{Digest: "sha256:1", ScanStatus: "IN_PROGRESS"}); err == nil {
		t.Fatal("in-progress scan was accepted")
	}
	if err := svc.StartScan(context.Background(), "repo", model.ECRImage{Digest: "sha256:1", ScanStatus: "ACTIVE"}); err == nil || !strings.Contains(err.Error(), "continuous") {
		t.Fatalf("active continuous scan error = %v", err)
	}
	if err := svc.DeleteImage(context.Background(), " repo ", " sha256:1 "); err != nil || api.deleted != "sha256:1" {
		t.Fatalf("DeleteImage() deleted %q error %v", api.deleted, err)
	}
	if uri, err := svc.ImageURI("registry/repo", image); err != nil || uri != "registry/repo:latest" {
		t.Fatalf("ImageURI() = %q, %v", uri, err)
	}
	if uri, err := svc.ImageURI("registry/repo", model.ECRImage{Digest: "sha256:2"}); err != nil || uri != "registry/repo@sha256:2" {
		t.Fatalf("untagged ImageURI() = %q, %v", uri, err)
	}
}

func TestECRContextualErrors(t *testing.T) {
	svc := NewECR(&fakeECRAPI{err: errors.New("denied")})
	if _, err := svc.ListRepositories(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list ECR repositories") {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	if _, err := svc.ListImages(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "repository name is required") {
		t.Fatalf("ListImages() validation error = %v", err)
	}
	if _, err := svc.Findings(context.Background(), "repo", ""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("Findings() validation error = %v", err)
	}
}
