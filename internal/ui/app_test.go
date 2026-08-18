package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func TestWrapText_NoWrap(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit"
	got := wrapText(input, 100)
	if got != input {
		t.Errorf("Should not wrap when within width:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestWrapText_Wraps(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit  [?] help"
	got := wrapText(input, 30)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("Expected multiple lines, got %d: %q", len(lines), got)
	}
}

func TestWrapText_BreaksBeforeBracket(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit"
	got := wrapText(input, 25)

	lines := strings.Split(got, "\n")
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
			t.Errorf("Wrapped line should start with '[': %q", trimmed)
		}
	}
}

func TestWrapText_EmptyInput(t *testing.T) {
	got := wrapText("", 50)
	if got != "" {
		t.Errorf("Expected empty, got %q", got)
	}
}

func TestWrapText_ZeroWidth(t *testing.T) {
	input := "  [a] test"
	got := wrapText(input, 0)
	if got != input {
		t.Errorf("Zero width should return input unchanged")
	}
}

func TestMaxLineWidth(t *testing.T) {
	input := "short\na longer line here\nmed"
	got := maxLineWidth(input)
	// "a longer line here" = 18 chars
	if got < 18 {
		t.Errorf("maxLineWidth = %d, want >= 18", got)
	}
}

func TestMaxLineWidth_Empty(t *testing.T) {
	got := maxLineWidth("")
	if got != 0 {
		t.Errorf("maxLineWidth(\"\") = %d, want 0", got)
	}
}

func TestStaleTaskResponseDoesNotReplaceCurrentServiceTasks(t *testing.T) {
	app := App{
		state:           viewTasks,
		selectedCluster: &model.Cluster{Name: "current-cluster"},
		selectedService: &model.Service{Name: "current-service"},
		taskView:        views.NewTaskList("current-service"),
		loading:         true,
	}

	updatedModel, _ := app.Update(tasksLoadedMsg{
		cluster: "old-cluster",
		service: "old-service",
		tasks:   []model.Task{{TaskID: "stale-task", TaskARN: "arn:stale"}},
	})
	updated := updatedModel.(App)
	if task := updated.taskView.SelectedTask(); task != nil {
		t.Fatalf("stale response populated the task browser: %#v", task)
	}

	updatedModel, _ = updated.Update(tasksLoadedMsg{
		cluster: "current-cluster",
		service: "current-service",
		tasks:   []model.Task{{TaskID: "current-task", TaskARN: "arn:current"}},
	})
	updated = updatedModel.(App)
	if task := updated.taskView.SelectedTask(); task == nil || task.TaskID != "current-task" {
		t.Fatalf("matching response did not populate the task browser: %#v", task)
	}
}

func TestStandaloneTasksToggleRestoresTaskContext(t *testing.T) {
	service := &model.Service{Name: "api"}
	task := &model.Task{TaskID: "task-1", TaskARN: "arn:task-1"}
	app := App{
		state:           viewTasks,
		selectedCluster: &model.Cluster{Name: "prod"},
		selectedService: service,
		selectedTask:    task,
	}

	standalone, _ := app.showStandaloneTasks()
	if standalone.state != viewStandaloneTasks {
		t.Fatalf("state = %v, want standalone tasks", standalone.state)
	}
	if standalone.selectedService != nil || standalone.selectedTask != nil {
		t.Fatal("standalone browser retained service task breadcrumbs")
	}

	restored, _ := standalone.showStandaloneTasks()
	if restored.state != viewTasks {
		t.Fatalf("state = %v, want task browser", restored.state)
	}
	if restored.selectedService != service || restored.selectedTask != task {
		t.Fatal("standalone toggle did not restore the prior task context")
	}
}

func TestServiceDetailReturnsToTaskDetail(t *testing.T) {
	service := &model.Service{Name: "api"}
	task := &model.Task{TaskID: "task-1", TaskARN: "arn:task-1"}
	app := App{
		state:           viewTaskDetail,
		selectedCluster: &model.Cluster{Name: "prod"},
		selectedService: service,
		selectedTask:    task,
	}

	detail, _ := app.showServiceDetail()
	if detail.state != viewServiceDetail {
		t.Fatalf("state = %v, want service detail", detail.state)
	}
	restored, _ := detail.goBack()
	if restored.state != viewTaskDetail {
		t.Fatalf("state = %v, want task detail", restored.state)
	}
	if restored.selectedService != service || restored.selectedTask != task {
		t.Fatal("service detail did not preserve the selected task context")
	}
}

func TestEnvVarsRevealKeyRequestsConfirmation(t *testing.T) {
	secret := e9saws.EnvVar{
		Name:   "API_TOKEN",
		Value:  "arn:aws:secretsmanager:us-east-1:123456789012:secret:api-token",
		Source: "secrets-manager",
	}
	app := App{
		state:             viewEnvVars,
		kb:                NewKeyBindings(),
		envTaskDefinition: "api:1",
		envContainer:      "api",
		envVarsView:       views.NewEnvVars("api", []e9saws.EnvVar{secret}),
	}

	updatedModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated := updatedModel.(App)
	if !updated.confirm.Active || updated.confirm.Action != ConfirmRevealSecrets {
		t.Fatalf("a did not open the reveal confirmation: %#v", updated.confirm)
	}
}

func TestEnvVarsRevealKeyTogglesAfterSecretsAreResolved(t *testing.T) {
	secret := e9saws.EnvVar{
		Name:          "API_TOKEN",
		Value:         "arn:aws:secretsmanager:us-east-1:123456789012:secret:api-token",
		ResolvedValue: "super-secret-value",
		Source:        "secrets-manager",
	}
	app := App{
		state:       viewEnvVars,
		kb:          NewKeyBindings(),
		envVarsView: views.NewEnvVars("api", []e9saws.EnvVar{secret}, true),
	}

	updatedModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated := updatedModel.(App)
	if updated.confirm.Active {
		t.Fatal("resolved secrets unexpectedly opened another confirmation")
	}
	view := updated.envVarsView.View()
	if !strings.Contains(view, secret.Value) || strings.Contains(view, secret.ResolvedValue) {
		t.Fatal("a did not toggle the resolved view back to secret references")
	}
}
