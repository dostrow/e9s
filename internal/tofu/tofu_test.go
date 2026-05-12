package tofu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
