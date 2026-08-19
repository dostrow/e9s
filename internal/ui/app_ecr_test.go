package ui

import (
	"testing"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func TestOpenECRFindingsAllowsEnhancedAndUnknownScanStatus(t *testing.T) {
	for _, status := range []string{"ACTIVE", ""} {
		t.Run(valueOrUnknown(status), func(t *testing.T) {
			app := App{width: 120, height: 40}
			app.ecrImagesView = views.NewECRImages("repo", "registry/repo").SetImages([]model.ECRImage{{
				Digest: "sha256:one", ScanStatus: status,
			}})
			next, command := app.openECRFindings()
			if next.state != viewECRFindings || command == nil {
				t.Fatalf("open findings with status %q: state=%v command=%v error=%v", status, next.state, command != nil, next.err)
			}
		})
	}
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
