package ui

import (
	"reflect"
	"testing"
)

func TestTofuApplyArgs_UsesSavedPlanFromPlanView(t *testing.T) {
	t.Parallel()

	app := App{
		state:        viewTofuPlan,
		tofuPlanFile: "/tmp/test-plan.tfplan",
	}

	got := app.tofuApplyArgs("/work/tofu")
	want := []string{
		"-chdir=/work/tofu",
		"apply",
		"-no-color",
		"/tmp/test-plan.tfplan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tofuApplyArgs() = %#v, want %#v", got, want)
	}
}

func TestTofuApplyArgs_DoesNotUseSavedPlanOutsidePlanView(t *testing.T) {
	t.Parallel()

	app := App{
		state:        viewTofuResources,
		tofuPlanFile: "/tmp/test-plan.tfplan",
	}

	got := app.tofuApplyArgs("/work/tofu")
	want := []string{
		"-chdir=/work/tofu",
		"apply",
		"-no-color",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tofuApplyArgs() = %#v, want %#v", got, want)
	}
}
