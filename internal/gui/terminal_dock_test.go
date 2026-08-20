//go:build gui

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTerminalShell(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	shell := filepath.Join(directory, "e9s-test-shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write test shell: %v", err)
	}
	resolved, err := resolveTerminalShell(shell)
	if err != nil {
		t.Fatalf("resolveTerminalShell: %v", err)
	}
	if resolved != shell {
		t.Fatalf("resolved shell = %q, want %q", resolved, shell)
	}
	if _, err := resolveTerminalShell(filepath.Join(directory, "missing")); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("missing shell error = %v", err)
	}
}

func TestTerminalDockUsesActiveTofuWorkspaceAsDirectory(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	w := &mainWindow{currentPage: pageTofuResources, selectedTofuWorkspace: workspace}
	if got := w.terminalDockWorkingDirectory(); got != workspace {
		t.Fatalf("terminalDockWorkingDirectory = %q, want %q", got, workspace)
	}

	w.currentPage = pageClusters
	fallback, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if got := w.terminalDockWorkingDirectory(); got != fallback {
		t.Fatalf("non-Tofu terminal directory = %q, want %q", got, fallback)
	}
}
