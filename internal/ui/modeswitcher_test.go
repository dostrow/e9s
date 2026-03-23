package ui

import (
	"testing"
)

func TestNewModeSwitcher_SortsAlphabetically(t *testing.T) {
	tabs := []ModeTab{
		{Mode: modeS3, Label: "S3"},
		{Mode: modeECS, Label: "ECS"},
		{Mode: modeCWLogs, Label: "CWL"},
		{Mode: modeDynamoDB, Label: "DDB"},
	}

	ms := NewModeSwitcher(tabs, modeECS)

	// Should be sorted: CloudWatch Logs, DynamoDB, ECS, S3
	expected := []topMode{modeCWLogs, modeDynamoDB, modeECS, modeS3}
	for i, want := range expected {
		if ms.tabs[i].Mode != want {
			t.Errorf("tabs[%d].Mode = %v, want %v (%s)",
				i, ms.tabs[i].Mode, want, modeDisplayName(want))
		}
	}
}

func TestNewModeSwitcher_CursorOnCurrent(t *testing.T) {
	tabs := []ModeTab{
		{Mode: modeECS, Label: "ECS"},
		{Mode: modeSSM, Label: "SSM"},
		{Mode: modeS3, Label: "S3"},
	}

	ms := NewModeSwitcher(tabs, modeSSM)

	// After sorting: ECS, S3, SSM — SSM is at index 2
	selectedMode := ms.tabs[ms.cursor].Mode
	if selectedMode != modeSSM {
		t.Errorf("Cursor should be on SSM, got %v (%s)",
			selectedMode, modeDisplayName(selectedMode))
	}
}
