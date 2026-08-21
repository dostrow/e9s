package ui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/tofu"
	"github.com/dostrow/e9s/internal/ui/views"
)

func TestConfirmTofuApplyUsesReviewedPlanSummary(t *testing.T) {
	t.Parallel()

	app := App{
		state:        viewTofuPlan,
		tofuDir:      "/infra/prod",
		tofuPlanFile: "/tmp/reviewed.tfplan",
		tofuPlanView: views.NewTofuPlan("/infra/prod").SetPlan(&tofu.PlanResult{CreateCount: 2, DeleteCount: 1}),
	}
	updated, _ := app.confirmTofuApply()
	if !updated.confirm.Active || updated.confirm.Action != ConfirmTofuApply {
		t.Fatalf("confirm = %+v", updated.confirm)
	}
	if !strings.Contains(updated.confirm.Message, "2 to create, 1 to delete") {
		t.Fatalf("confirmation missing plan summary: %q", updated.confirm.Message)
	}
}

func TestConfirmTofuInitDescribesWorkspaceEffects(t *testing.T) {
	t.Parallel()

	updated, _ := (App{tofuDir: "/infra/prod"}).confirmTofuInit()
	if updated.confirm.Action != ConfirmTofuInit || !strings.Contains(updated.confirm.Message, "dependency lock file") {
		t.Fatalf("confirm = %+v", updated.confirm)
	}
}
