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

func TestModuleRailSectionsStayAlphabetical(t *testing.T) {
	sections := []moduleRailSection{
		{name: "ECS"},
		{name: "CloudWatch Logs"},
		{name: "CloudWatch Alarms"},
	}
	sortModuleRailSections(sections)
	want := []string{"CloudWatch Alarms", "CloudWatch Logs", "ECS"}
	for i, name := range want {
		if sections[i].name != name {
			t.Fatalf("section %d = %q, want %q", i, sections[i].name, name)
		}
	}
}

func TestExpandedModuleHeadingAddsChevronSpacing(t *testing.T) {
	if got := moduleHeadingMargin(false); got != 0 {
		t.Fatalf("collapsed heading margin = %d, want 0", got)
	}
	if got := moduleHeadingMargin(true); got <= 0 {
		t.Fatalf("expanded heading margin = %d, want positive spacing", got)
	}
}
