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

func TestTerminalDockNodeSessionsFollowVisualOrder(t *testing.T) {
	first := &terminalDockSession{id: 1}
	second := &terminalDockSession{id: 2}
	third := &terminalDockSession{id: 3}
	root := &terminalDockNode{
		first: &terminalDockNode{session: first},
		second: &terminalDockNode{
			first:  &terminalDockNode{session: second},
			second: &terminalDockNode{session: third},
		},
	}
	sessions := terminalDockNodeSessions(root)
	if len(sessions) != 3 {
		t.Fatalf("session count = %d, want 3", len(sessions))
	}
	for index, want := range []*terminalDockSession{first, second, third} {
		if sessions[index] != want {
			t.Errorf("session %d = %v, want %v", index, sessions[index], want)
		}
	}
	if got := firstTerminalDockSession(root); got != first {
		t.Fatalf("first session = %v, want %v", got, first)
	}
}

func TestTerminalDirectoryFromURI(t *testing.T) {
	for _, test := range []struct {
		uri  string
		want string
	}{
		{uri: "file:///home/example/My%20Project", want: "/home/example/My Project"},
		{uri: "file://localhost/tmp/e9s", want: "/tmp/e9s"},
		{uri: "file://remote.example/tmp/e9s"},
		{uri: "https://example.com/tmp/e9s"},
	} {
		if got := terminalDirectoryFromURI(test.uri); got != test.want {
			t.Errorf("terminalDirectoryFromURI(%q) = %q, want %q", test.uri, got, test.want)
		}
	}
}

func TestTerminalDockPageAfterClose(t *testing.T) {
	for _, test := range []struct {
		name      string
		current   int
		closed    int
		remaining int
		want      int
	}{
		{name: "active middle selects prior", current: 2, closed: 2, remaining: 3, want: 1},
		{name: "active first selects new first", current: 0, closed: 0, remaining: 2, want: 0},
		{name: "background before active preserves active tab", current: 3, closed: 1, remaining: 3, want: 2},
		{name: "background after active preserves active tab", current: 1, closed: 3, remaining: 3, want: 1},
		{name: "final tab has no target", current: 0, closed: 0, remaining: 0, want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := terminalDockPageAfterClose(test.current, test.closed, test.remaining); got != test.want {
				t.Fatalf("terminalDockPageAfterClose(%d, %d, %d) = %d, want %d", test.current, test.closed, test.remaining, got, test.want)
			}
		})
	}
}
