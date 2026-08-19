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

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
