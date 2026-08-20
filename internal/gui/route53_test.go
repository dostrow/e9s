//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterRoute53ZonesMatchesNameCommentAndID(t *testing.T) {
	zones := []model.Route53Zone{
		{ID: "ZPUBLIC", Name: "example.com.", Comment: "production"},
		{ID: "ZPRIVATE", Name: "internal.example.", Comment: "private services"},
	}
	for query, want := range map[string]string{"EXAMPLE.COM": "ZPUBLIC", "private services": "ZPRIVATE", "zpublic": "ZPUBLIC"} {
		filtered := filterRoute53Zones(zones, query)
		if len(filtered) != 1 || filtered[0].ID != want {
			t.Fatalf("filterRoute53Zones(%q) = %#v, want %s", query, filtered, want)
		}
	}
}

func TestFormatRoute53ZoneIncludesVisibilityAndCounts(t *testing.T) {
	detail := formatRoute53Zone(model.Route53Zone{ID: "Z123", Name: "internal.example.", Private: true, RecordCount: 12})
	for _, want := range []string{"internal.example.", "Z123", "Private", "12"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("formatRoute53Zone() missing %q in %q", want, detail)
		}
	}
}
