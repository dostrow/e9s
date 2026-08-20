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

func TestTerminalDockSessionTitlePrefersRenameThenTerminalContext(t *testing.T) {
	session := &terminalDockSession{id: 4, autoTitle: "nvim main.go"}
	if got := terminalDockSessionTitle(session); got != "nvim main.go" {
		t.Fatalf("automatic title = %q", got)
	}
	session.customTitle = "API logs"
	if got := terminalDockSessionTitle(session); got != "API logs" {
		t.Fatalf("custom title = %q", got)
	}
	session.customTitle = ""
	session.autoTitle = ""
	if got := terminalDockSessionTitle(session); got != "Terminal 4" {
		t.Fatalf("fallback title = %q", got)
	}
}

func TestTerminalDockTabTitlePrefersTabRenameThenActiveSession(t *testing.T) {
	session := &terminalDockSession{id: 7, autoTitle: "psql reporting"}
	tab := &terminalDockTab{active: session}
	session.tab = tab
	if got := terminalDockTabTitle(tab); got != "psql reporting" {
		t.Fatalf("automatic tab title = %q", got)
	}
	tab.customTitle = "Database"
	if got := terminalDockTabTitle(tab); got != "Database" {
		t.Fatalf("custom tab title = %q", got)
	}
}

func TestTerminalDockTabWidthTracksTitleWithinBounds(t *testing.T) {
	for _, test := range []struct {
		title string
		want  int
	}{
		{title: "sh", want: terminalTabMinimumChars},
		{title: "API logs", want: 8},
		{title: "database migration", want: 18},
		{title: strings.Repeat("x", terminalTabMaximumChars+10), want: terminalTabMaximumChars},
	} {
		if got := terminalDockTabWidth(test.title); got != test.want {
			t.Errorf("terminalDockTabWidth(%q) = %d, want %d", test.title, got, test.want)
		}
	}
}
