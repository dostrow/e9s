//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/tofu"
)

func TestSortedTofuWorkspacesDoesNotMutateConfigOrder(t *testing.T) {
	t.Parallel()

	workspaces := []config.TofuDirEntry{{Name: "Zulu", Dir: "/z"}, {Name: "alpha", Dir: "/a"}}
	sorted := sortedTofuWorkspaces(workspaces)
	if sorted[0].Name != "alpha" || workspaces[0].Name != "Zulu" {
		t.Fatalf("sorted = %#v, original = %#v", sorted, workspaces)
	}
}

func TestFormatTofuWorkspace(t *testing.T) {
	t.Parallel()

	text := formatTofuWorkspace(
		config.TofuDirEntry{Name: "production", Dir: "/infra/prod"},
		tofu.Workspace{Dir: "/infra/prod", Binary: "/usr/bin/tofu", Initialized: true},
	)
	for _, expected := range []string{"production", "/infra/prod", "/usr/bin/tofu", "Initialized  Yes"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("formatTofuWorkspace() missing %q in %q", expected, text)
		}
	}
}

func TestTofuPagesOwnModuleErrorGlyph(t *testing.T) {
	t.Parallel()

	for _, page := range []string{pageTofuWorkspaces, pageTofuResources} {
		if got := moduleForPage(page); got != moduleTofu {
			t.Fatalf("moduleForPage(%q) = %q", page, got)
		}
	}
}
