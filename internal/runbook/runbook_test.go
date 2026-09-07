package runbook

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAndBuildInvocation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "release"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `api_version: e9s/v1alpha1
name: example
display_name: Example Operations
actions:
  - id: release
    title: Production release
    risk: production
    run:
      command: ./release
      launch_directory: .
      working_directory: .
      terminal: true
      login_shell: true
      environment:
        AWS_PROFILE: ${e9s.profile}
    inputs:
      - id: tag
        label: Candidate tag
        type: text
        argument: --tag
        required: true
      - id: dry_run
        label: Validate only
        type: boolean
        argument: --dry-run
      - id: region
        label: Region
        type: text
        argument: --region
        default: ${e9s.region}
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	actions, err := Load([]string{dir}, "/unused")
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Key() != "example/release" {
		t.Fatalf("unexpected actions: %#v", actions)
	}
	invocation, err := BuildInvocation(actions[0], map[string]string{"tag": "server 42", "dry_run": "true"}, Context{Region: "us-east-2", Profile: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--tag", "server 42", "--dry-run", "--region", "us-east-2"}
	if !reflect.DeepEqual(invocation.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", invocation.Args, wantArgs)
	}
	if invocation.Environment["AWS_PROFILE"] != "prod" || !invocation.Terminal || !invocation.LoginShell || invocation.Risk != "production" {
		t.Fatalf("unexpected invocation: %#v", invocation)
	}
	if invocation.LaunchDirectory != dir || invocation.WorkingDirectory != dir || invocation.Executable != filepath.Join(dir, "release") {
		t.Fatalf("unexpected directories: %#v", invocation)
	}
	if !strings.Contains(invocation.DisplayCommand(), "'server 42'") {
		t.Fatalf("display command did not quote argument: %s", invocation.DisplayCommand())
	}
}

func TestExecutionCommandUsesFixedLoginShellTrampoline(t *testing.T) {
	t.Parallel()
	invocation := Invocation{
		Executable: "/opt/plugin/release", Args: []string{"--tag", "value with spaces; touch /tmp/nope"}, LoginShell: true,
	}
	executable, args, err := invocation.ExecutionCommand("sh")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(executable) != "sh" {
		t.Fatalf("executable = %q, want sh", executable)
	}
	want := []string{"-lic", `exec "$@"`, "e9s-plugin", "/opt/plugin/release", "--tag", "value with spaces; touch /tmp/nope"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if invocation.ExecutionMode() != "login shell" {
		t.Fatalf("execution mode = %q", invocation.ExecutionMode())
	}
}

func TestExecutionCommandDirectModeIsUnchanged(t *testing.T) {
	t.Parallel()
	invocation := Invocation{Executable: "release", Args: []string{"--dry-run"}}
	executable, args, err := invocation.ExecutionCommand("definitely-not-a-shell")
	if err != nil {
		t.Fatal(err)
	}
	if executable != invocation.Executable || !reflect.DeepEqual(args, invocation.Args) {
		t.Fatalf("direct command = %q %#v", executable, args)
	}
}

func TestBuildInvocationRejectsConflicts(t *testing.T) {
	dir := t.TempDir()
	action := ConfiguredAction{ManifestPath: filepath.Join(dir, "plugin.yaml"), Action: Action{
		ID: "release", Title: "Release", Run: RunSpec{Command: "release"},
		Inputs: []Input{
			{ID: "dry_run", Label: "Dry run", Type: "boolean", Conflicts: []string{"apply"}},
			{ID: "apply", Label: "Apply", Type: "boolean"},
		},
	}}
	_, err := BuildInvocation(action, map[string]string{"dry_run": "true", "apply": "true"}, Context{})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestLoadRejectsUnknownManifestFields(t *testing.T) {
	dir := t.TempDir()
	data := "api_version: e9s/v1alpha1\nname: example\nactions: []\nunknown: true\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{dir}, dir); err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("expected strict YAML error, got %v", err)
	}
}

func TestLoadSourcesAppliesFriendlyName(t *testing.T) {
	dir := t.TempDir()
	data := "api_version: e9s/v1alpha1\nname: example\ndisplay_name: Shared name\nactions:\n  - id: run\n    title: Run\n    run:\n      command: true\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	actions, err := LoadSources([]Source{{Name: "My Operations", Path: dir}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := actions[0].PluginDisplayName; got != "My Operations" {
		t.Fatalf("friendly name = %q", got)
	}
}

func TestLaunchAndWorkingDirectoriesAreIndependent(t *testing.T) {
	root := t.TempDir()
	manifestDirectory := filepath.Join(root, "plugin")
	launchDirectory := filepath.Join(root, "launcher")
	workingDirectory := filepath.Join(root, "repository")
	for _, directory := range []string{manifestDirectory, launchDirectory, workingDirectory} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	action := ConfiguredAction{ManifestPath: filepath.Join(manifestDirectory, "plugin.yaml"), Action: Action{
		Title: "Run", Run: RunSpec{
			Command: "./bin/run", LaunchDirectory: "../launcher", WorkingDirectory: "../repository",
		},
	}}
	invocation, err := BuildInvocation(action, nil, Context{})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.LaunchDirectory != launchDirectory {
		t.Fatalf("launch directory = %q", invocation.LaunchDirectory)
	}
	// working_directory is intentionally relative to launch_directory.
	if invocation.WorkingDirectory != workingDirectory {
		t.Fatalf("working directory = %q", invocation.WorkingDirectory)
	}
	if invocation.Executable != filepath.Join(launchDirectory, "bin/run") {
		t.Fatalf("executable = %q", invocation.Executable)
	}
}
