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
		pageSSM:             moduleSSM,
		pageSecrets:         moduleSecrets,
		pageLambda:          moduleLambda,
		pageModulePicker:    "",
		"unknown":           "",
	}
	for page, want := range tests {
		if got := moduleForPage(page); got != want {
			t.Errorf("moduleForPage(%q) = %q, want %q", page, got, want)
		}
	}
}

func TestModuleSectionIndexAcceptsTUIDefaultModeAliases(t *testing.T) {
	sections := []moduleRailSection{
		{key: moduleECS, name: "ECS", aliases: []string{"ecs"}},
		{key: moduleCloudWatchLogs, name: "CloudWatch Logs", aliases: []string{"cwl", "cw", "cloudwatch-logs", "cloudwatch"}},
		{key: moduleCloudWatchAlarms, name: "CloudWatch Alarms", aliases: []string{"cwa", "cloudwatch-alarms"}},
		{key: moduleSSM, name: "SSM Parameter Store", aliases: []string{"ssm"}},
		{key: moduleSecrets, name: "Secrets Manager", aliases: []string{"sm", "secrets"}},
		{key: moduleLambda, name: "Lambda", aliases: []string{"lambda", "λ"}},
	}
	tests := map[string]string{
		"ECS":               moduleECS,
		" CW ":              moduleCloudWatchLogs,
		"CloudWatch Logs":   moduleCloudWatchLogs,
		"CWA":               moduleCloudWatchAlarms,
		"cloudwatch-alarms": moduleCloudWatchAlarms,
		"SSM":               moduleSSM,
		"SM":                moduleSecrets,
		"Lambda":            moduleLambda,
		"λ":                 moduleLambda,
	}
	for value, want := range tests {
		index, found := moduleSectionIndex(sections, value)
		if !found {
			t.Fatalf("moduleSectionIndex(%q) did not resolve", value)
		}
		if got := sections[index].key; got != want {
			t.Fatalf("moduleSectionIndex(%q) = %q, want %q", value, got, want)
		}
	}
	if _, found := moduleSectionIndex(sections, "S3"); found {
		t.Fatal("unimplemented GUI module unexpectedly resolved")
	}
}

func TestModulePickerLabelsDescribeDefaultSubItems(t *testing.T) {
	sections := []moduleRailSection{
		{name: "ECS", defaultItem: "Clusters"},
		{name: "CloudWatch Logs", defaultItem: "Log groups"},
	}
	got := modulePickerLabels(sections)
	want := []string{"ECS — Clusters", "CloudWatch Logs — Log groups"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("label %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestModuleRailSectionsStayAlphabetical(t *testing.T) {
	sections := []moduleRailSection{
		{name: "ECS"},
		{name: "CloudWatch Logs"},
		{name: "CloudWatch Alarms"},
		{name: "SSM Parameter Store"},
	}
	sortModuleRailSections(sections)
	want := []string{"CloudWatch Alarms", "CloudWatch Logs", "ECS", "SSM Parameter Store"}
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
