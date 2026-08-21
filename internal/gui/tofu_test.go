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

	for _, page := range []string{pageTofuWorkspaces, pageTofuResources, pageTofuPlan} {
		if got := moduleForPage(page); got != moduleTofu {
			t.Fatalf("moduleForPage(%q) = %q", page, got)
		}
	}
}

func TestFilteredTofuPlanChangesMatchesAddressActionAndType(t *testing.T) {
	t.Parallel()

	plan := &tofu.PlanResult{Changes: []tofu.ResourceChange{
		{Address: "aws_s3_bucket.assets", Type: "aws_s3_bucket", Action: "create"},
		{Address: "aws_ecs_service.api", Type: "aws_ecs_service", Action: "update"},
	}}
	for query, want := range map[string]string{"assets": "aws_s3_bucket.assets", "UPDATE": "aws_ecs_service.api", "s3_bucket": "aws_s3_bucket.assets"} {
		changes := filteredTofuPlanChanges(plan, query)
		if len(changes) != 1 || changes[0].Address != want {
			t.Fatalf("filteredTofuPlanChanges(%q) = %#v", query, changes)
		}
	}
}

func TestFormatTofuPlanChangeIncludesFullDiff(t *testing.T) {
	t.Parallel()

	change := tofu.ResourceChange{
		Address: "aws_ecs_service.api",
		Type:    "aws_ecs_service",
		Name:    "api",
		Action:  "update",
		Diffs:   []tofu.AttrDiff{{Path: "desired_count", Before: "2", After: "4", Action: "change"}},
	}
	text := formatTofuPlanChange(change)
	for _, expected := range []string{"aws_ecs_service.api", "desired_count", "before: 2", "after:  4"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("formatTofuPlanChange() missing %q in %q", expected, text)
		}
	}
}

func TestFormatTofuOperationOutputKeepsCommandOutput(t *testing.T) {
	t.Parallel()

	text := formatTofuOperationOutput("INIT COMPLETED", config.TofuDirEntry{Name: "prod", Dir: "/infra/prod"}, "Provider installation complete")
	for _, expected := range []string{"INIT COMPLETED", "prod", "/infra/prod", "Provider installation complete"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("formatTofuOperationOutput() missing %q in %q", expected, text)
		}
	}
}
