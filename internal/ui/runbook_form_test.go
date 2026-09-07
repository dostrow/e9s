package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/runbook"
)

func TestRunbookFormBuildsReviewedInvocation(t *testing.T) {
	dir := t.TempDir()
	action := runbook.ConfiguredAction{
		PluginName: "example", PluginDisplayName: "Example Operations",
		ManifestPath: filepath.Join(dir, "plugin.yaml"),
		Action: runbook.Action{
			ID: "release", Title: "Release", Risk: "production",
			Run: runbook.RunSpec{Command: "release", LaunchDirectory: ".", WorkingDirectory: ".", Terminal: true},
			Inputs: []runbook.Input{
				{ID: "tag", Label: "Tag", Type: "text", Argument: "--tag", Required: true},
				{ID: "dry_run", Label: "Dry run", Type: "boolean", Argument: "--dry-run"},
			},
		},
	}
	form := NewRunbookForm(action, runbook.Context{})
	form.fields[0].text.SetValue("candidate-42")
	form.fields[1].boolean = true
	updated, cmd := form.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil || updated.Active {
		t.Fatal("submit did not close the form and return a command")
	}
	message, ok := cmd().(RunbookSubmitMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", cmd())
	}
	if got := strings.Join(message.Invocation.Args, " "); got != "--tag candidate-42 --dry-run" {
		t.Fatalf("invocation args = %q", got)
	}
	review := runbookConfirmation(message.Invocation)
	for _, expected := range []string{"Risk: production", "Launch directory: " + dir, "Working directory: " + dir} {
		if !strings.Contains(review, expected) {
			t.Fatalf("review is missing %q:\n%s", expected, review)
		}
	}
}

func TestMergeEnvironmentReplacesInheritedValues(t *testing.T) {
	got := mergeEnvironment(
		[]string{"PATH=/usr/bin", "AWS_REGION=old", "HOME=/tmp/home"},
		map[string]string{"AWS_REGION": "us-east-2", "NEW_VALUE": "present"},
	)
	want := []string{"AWS_REGION=us-east-2", "HOME=/tmp/home", "NEW_VALUE=present", "PATH=/usr/bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}
