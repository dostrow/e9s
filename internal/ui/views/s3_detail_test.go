package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/theme"
)

func TestS3DetailHighlightsArchiveAccessTier(t *testing.T) {
	detail := &model.S3ObjectDetail{
		StorageClass:  "INTELLIGENT_TIERING",
		ArchiveStatus: model.S3ArchiveAccessStatus,
	}
	got := NewS3Detail("archive", detail).View()
	want := theme.ErrorStyle.Render(model.S3ArchiveAccessStatus)
	if !strings.Contains(got, want) {
		t.Fatalf("S3 detail does not contain styled archive status: %q", got)
	}
}

func TestS3DetailExplainsUnknownActiveTier(t *testing.T) {
	detail := &model.S3ObjectDetail{StorageClass: "INTELLIGENT_TIERING"}
	got := NewS3Detail("archive", detail).View()
	if !strings.Contains(got, "exact tier requires S3 Inventory") {
		t.Fatalf("S3 detail does not explain active tier limitation: %q", got)
	}
}
