package tofu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunnerRejectsEmptyWorkspace(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := NewRunner("  "); err == nil || !strings.Contains(err.Error(), "workspace path is required") {
		t.Fatalf("NewRunner error = %v", err)
	}
}

func TestParseResourceAddress(t *testing.T) {
	t.Parallel()

	resource := ParseResourceAddress("module.network.module.private.aws_subnet.this[0]")
	if resource.Module != "module.network.module.private" || resource.Type != "aws_subnet" || resource.Name != "this[0]" {
		t.Fatalf("ParseResourceAddress = %+v", resource)
	}
}

func TestRunnerHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	binary := writeFakeTofuBinary(t, `{"format_version":"1.2"}`)
	runner := &Runner{Dir: workdir, Binary: binary}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runner.StateListContext(ctx); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("StateListContext error = %v", err)
	}
}

func TestApplyCommandUsesReviewedPlan(t *testing.T) {
	t.Parallel()

	runner := &Runner{Dir: "/work/tofu", Binary: "/usr/bin/tofu"}
	command := runner.ApplyCommand("/tmp/reviewed.tfplan")
	want := []string{"-chdir=/work/tofu", "apply", "-no-color", "/tmp/reviewed.tfplan"}
	if strings.Join(command.Args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ApplyCommand args = %#v, want %#v", command.Args, want)
	}
}

func TestPlanJSONSavedPreservesPlanFile(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	binary := writeFakeTofuBinary(t, `{"format_version":"1.2"}`)
	runner := &Runner{Dir: workdir, Binary: binary}

	jsonOut, planFile, err := runner.PlanJSONSaved()
	if err != nil {
		t.Fatalf("PlanJSONSaved: %v", err)
	}
	if strings.TrimSpace(jsonOut) != `{"format_version":"1.2"}` {
		t.Fatalf("jsonOut = %q", jsonOut)
	}
	if planFile == "" {
		t.Fatal("planFile should not be empty")
	}
	if _, err := os.Stat(planFile); err != nil {
		t.Fatalf("saved plan file missing: %v", err)
	}
	if filepath.Dir(planFile) == workdir {
		t.Fatalf("plan file should be created outside the workspace, got %q", planFile)
	}
}

func writeFakeTofuBinary(t *testing.T, jsonOut string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "fake-tofu")
	script := `#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "plan" ]]; then
  for arg in "$@"; do
    if [[ "$arg" == -out=* ]]; then
      plan_file="${arg#-out=}"
      printf 'planned\n' > "$plan_file"
      exit 0
    fi
  done
  echo "missing -out flag" >&2
  exit 1
fi

if [[ "$1" == "show" && "$2" == "-json" ]]; then
  printf '%s\n' '` + jsonOut + `'
  exit 0
fi

echo "unexpected args: $*" >&2
exit 1
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tofu binary: %v", err)
	}
	return path
}
