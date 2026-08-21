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

func TestVariablesDocumentCreatesAndUpdatesAtomically(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	document, err := ReadVariables(context.Background(), workspace)
	if err != nil {
		t.Fatalf("ReadVariables missing file: %v", err)
	}
	if document.Exists || document.Content != "" || document.Revision != missingRevision {
		t.Fatalf("missing VariablesDocument = %+v", document)
	}

	saved, err := SaveVariables(context.Background(), document, "region = \"us-east-2\"\n")
	if err != nil {
		t.Fatalf("SaveVariables create: %v", err)
	}
	if !saved.Exists || saved.Revision == missingRevision {
		t.Fatalf("saved VariablesDocument = %+v", saved)
	}
	info, err := os.Stat(filepath.Join(workspace, VariablesFilename))
	if err != nil {
		t.Fatalf("stat created variables file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("created variables mode = %o, want 600", got)
	}

	updated, err := SaveVariables(context.Background(), saved, "region = \"us-west-2\"\n")
	if err != nil {
		t.Fatalf("SaveVariables update: %v", err)
	}
	if updated.Content != "region = \"us-west-2\"\n" || updated.Revision == saved.Revision {
		t.Fatalf("updated VariablesDocument = %+v", updated)
	}
	if matches, err := filepath.Glob(filepath.Join(workspace, ".terraform.tfvars.e9s-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files after save = %v, %v", matches, err)
	}
}

func TestSaveVariablesRejectsStaleDocument(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, VariablesFilename)
	if err := os.WriteFile(path, []byte("value = 1\n"), 0o640); err != nil {
		t.Fatalf("write variables: %v", err)
	}
	document, err := ReadVariables(context.Background(), workspace)
	if err != nil {
		t.Fatalf("ReadVariables: %v", err)
	}
	if err := os.WriteFile(path, []byte("value = 2\n"), 0o640); err != nil {
		t.Fatalf("modify variables: %v", err)
	}
	if _, err := SaveVariables(context.Background(), document, "value = 3\n"); err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("SaveVariables stale error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "value = 2\n" {
		t.Fatalf("stale save changed file to %q, %v", data, err)
	}
}

func TestSaveVariablesPreservesExistingPermissions(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, VariablesFilename)
	if err := os.WriteFile(path, []byte("value = 1\n"), 0o640); err != nil {
		t.Fatalf("write variables: %v", err)
	}
	document, err := ReadVariables(context.Background(), workspace)
	if err != nil {
		t.Fatalf("ReadVariables: %v", err)
	}
	if _, err := SaveVariables(context.Background(), document, "value = 2\n"); err != nil {
		t.Fatalf("SaveVariables: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat variables: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("updated variables mode = %o, want 640", got)
	}
}

func TestVariablesRejectsSymlink(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.tfvars")
	if err := os.WriteFile(target, []byte("secret = true\n"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(workspace, VariablesFilename)); err != nil {
		t.Fatalf("create variables symlink: %v", err)
	}
	if _, err := ReadVariables(context.Background(), workspace); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("ReadVariables symlink error = %v", err)
	}
}

func TestVariablesHonorCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadVariables(ctx, t.TempDir()); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("ReadVariables canceled error = %v", err)
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
