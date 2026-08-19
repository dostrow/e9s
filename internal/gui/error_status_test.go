//go:build gui

package gui

import "testing"

func TestCompactStatusMessageMakesMultilineErrorsReadable(t *testing.T) {
	got := compactStatusMessage("request failed:\n  AccessDenied:   missing permission")
	want := "request failed: AccessDenied: missing permission"
	if got != want {
		t.Fatalf("compactStatusMessage() = %q, want %q", got, want)
	}
}

func TestModuleForPage(t *testing.T) {
	tests := map[string]string{
		pageClusters:        moduleECS,
		pageStoppedTasks:    moduleECS,
		pageTaskDefinitions: moduleECS,
		pageLogGroups:       moduleCloudWatchLogs,
		pageLogStreams:      moduleCloudWatchLogs,
		pageSavedLogSearch:  moduleCloudWatchLogs,
		pageAlarms:          moduleCloudWatchAlarms,
		"unknown":           "",
	}
	for page, want := range tests {
		if got := moduleForPage(page); got != want {
			t.Errorf("moduleForPage(%q) = %q, want %q", page, got, want)
		}
	}
}
