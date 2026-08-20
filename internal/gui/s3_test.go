//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestFilterS3BucketsIsCaseInsensitiveAndPreservesOrder(t *testing.T) {
	buckets := []model.S3Bucket{{Name: "Archive"}, {Name: "prod-assets"}, {Name: "Prod-logs"}}
	got := filterS3Buckets(buckets, "PROD")
	if len(got) != 2 || got[0].Name != "prod-assets" || got[1].Name != "Prod-logs" {
		t.Fatalf("filterS3Buckets() = %#v", got)
	}
	if len(filterS3Buckets(buckets, "missing")) != 0 {
		t.Fatal("filterS3Buckets returned a non-match")
	}
}

func TestS3PresentationHelpers(t *testing.T) {
	created := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	bucket := model.S3Bucket{Name: "prod-assets", CreatedAt: created}
	if got := s3Breadcrumb("Production", bucket.Name); got != "S3 / Production / prod-assets" {
		t.Fatalf("s3Breadcrumb() = %q", got)
	}
	if got := formatS3Bucket(bucket); got == "" || !containsAll(got, "prod-assets", "s3://prod-assets/") {
		t.Fatalf("formatS3Bucket() = %q", got)
	}
}

func TestFindS3Search(t *testing.T) {
	search, found := findS3Search([]config.S3Search{{Name: "Production", Filter: "prod-"}}, "Production")
	if !found || search.Filter != "prod-" {
		t.Fatalf("findS3Search() = %#v, %v", search, found)
	}
}

func TestS3ObjectNavigationHelpers(t *testing.T) {
	if got := parentS3Prefix("reports/2026/august/"); got != "reports/2026/" {
		t.Fatalf("parentS3Prefix() = %q", got)
	}
	if got := parentS3Prefix("reports/"); got != "" {
		t.Fatalf("root parentS3Prefix() = %q", got)
	}
	if got := s3ObjectDisplayName("reports/2026/", "reports/"); got != "2026" {
		t.Fatalf("s3ObjectDisplayName() = %q", got)
	}
	if got := s3ObjectBreadcrumb("", "archive", "reports/2026/", false); got != "S3 / Buckets / archive / reports/2026" {
		t.Fatalf("s3ObjectBreadcrumb() = %q", got)
	}
	if got := s3ObjectBreadcrumb("Production", "archive", "reports/", true); got != "S3 / Production / archive / Search: reports/" {
		t.Fatalf("search s3ObjectBreadcrumb() = %q", got)
	}
}

func TestFilterS3ObjectsUsesRelativeNames(t *testing.T) {
	objects := []model.S3Object{
		{Key: "reports/2025/", IsPrefix: true},
		{Key: "reports/summary.csv"},
	}
	got := filterS3Objects(objects, "SUMMARY", "reports/")
	if len(got) != 1 || got[0].Key != "reports/summary.csv" {
		t.Fatalf("filterS3Objects() = %#v", got)
	}
}

func TestFormatS3ObjectDetailSortsTags(t *testing.T) {
	detail := &model.S3ObjectDetail{
		Key: "reports/summary.csv", Size: 2048, ContentType: "text/csv",
		Tags: map[string]string{"zeta": "last", "alpha": "first"},
	}
	got := formatS3ObjectDetail("archive", detail)
	if !containsAll(got, "s3://archive/reports/summary.csv", "2.0 KiB", "text/csv") {
		t.Fatalf("formatS3ObjectDetail() = %q", got)
	}
	if strings.Index(got, "alpha = first") > strings.Index(got, "zeta = last") {
		t.Fatalf("tags are not sorted in %q", got)
	}
}

func TestFormatS3ObjectDetailReportsIntelligentTieringAccess(t *testing.T) {
	archived := &model.S3ObjectDetail{
		StorageClass: "INTELLIGENT_TIERING", ArchiveStatus: model.S3DeepArchiveAccessStatus,
	}
	if got := formatS3ObjectDetail("archive", archived); !containsAll(got, "Access tier", model.S3DeepArchiveAccessStatus) {
		t.Fatalf("archived Intelligent-Tiering detail = %q", got)
	}
	active := &model.S3ObjectDetail{StorageClass: "INTELLIGENT_TIERING"}
	if got := formatS3ObjectDetail("archive", active); !containsAll(got, "Access tier", "exact tier requires S3 Inventory") {
		t.Fatalf("active Intelligent-Tiering detail = %q", got)
	}
	standard := &model.S3ObjectDetail{StorageClass: "STANDARD"}
	if got := formatS3ObjectDetail("archive", standard); strings.Contains(got, "Access tier") {
		t.Fatalf("standard object unexpectedly has access tier: %q", got)
	}
}

func TestFormatS3DownloadProgress(t *testing.T) {
	request := model.S3DownloadRequest{Key: "reports/", IsPrefix: true}
	got := formatS3DownloadProgress(request, model.S3DownloadProgress{
		CurrentKey: "reports/2026/summary.csv", FilesCompleted: 3, BytesCompleted: 2 << 20,
	})
	if !containsAll(got, "reports", "2.0 MiB", "3 files", "summary.csv") {
		t.Fatalf("formatS3DownloadProgress() = %q", got)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
