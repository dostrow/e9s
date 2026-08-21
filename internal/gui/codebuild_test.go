//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterCodeBuildProjectsSearchesLoadedMetadata(t *testing.T) {
	projects := []model.CodeBuildProject{
		{Name: "prod-api", Description: "HTTP service", Source: "CODECOMMIT"},
		{Name: "queue-worker", Description: "background jobs", Source: "GITHUB"},
	}
	for query, want := range map[string]string{
		"API":        "prod-api",
		"jobs":       "queue-worker",
		"codecommit": "prod-api",
	} {
		got := filterCodeBuildProjects(projects, query)
		if len(got) != 1 || got[0].Name != want {
			t.Fatalf("filterCodeBuildProjects(%q) = %#v, want %q", query, got, want)
		}
	}
}

func TestFilterCodeBuildBuildsSearchesOperationalFields(t *testing.T) {
	builds := []model.CodeBuildBuild{
		{ID: "api:41", BuildNumber: 41, Status: "FAILED", Initiator: "dan", SourceVersion: "main"},
		{ID: "api:42", BuildNumber: 42, Status: "IN_PROGRESS", Initiator: "pipeline", CurrentPhase: "BUILD"},
	}
	for query, want := range map[string]int64{
		"failed":   41,
		"pipeline": 42,
		"build":    42,
		"main":     41,
	} {
		got := filterCodeBuildBuilds(builds, query)
		if len(got) != 1 || got[0].BuildNumber != want {
			t.Fatalf("filterCodeBuildBuilds(%q) = %#v, want %d", query, got, want)
		}
	}
}

func TestCodeBuildSummariesContainNavigationContext(t *testing.T) {
	project := model.CodeBuildProject{Name: "api", Source: "GITHUB", Description: "service"}
	if got := formatCodeBuildProject(project); !strings.Contains(got, "Double-click") || !strings.Contains(got, "GITHUB") {
		t.Fatalf("project summary = %q", got)
	}
	build := model.CodeBuildBuild{
		ID: "api:7", BuildNumber: 7, Status: "SUCCEEDED",
		StartTime: time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
		Duration:  90 * time.Second, Initiator: "pipeline", SourceVersion: "abc123",
	}
	got := formatCodeBuildBuildSummary("api", build)
	for _, want := range []string{"api", "#7", "SUCCEEDED", "1m30s", "pipeline", "abc123"} {
		if !strings.Contains(got, want) {
			t.Fatalf("build summary %q missing %q", got, want)
		}
	}
}

func TestFormatCodeBuildDetailIncludesFailuresLogsAndEnvironment(t *testing.T) {
	detail := model.CodeBuildDetail{
		CodeBuildBuild: model.CodeBuildBuild{
			ID: "api:7", BuildNumber: 7, Status: "FAILED", Initiator: "pipeline",
			StartTime: time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, time.August, 18, 12, 2, 0, 0, time.UTC), Duration: 2 * time.Minute,
		},
		ProjectName: "api", ARN: "arn:build", Source: model.CodeBuildSource{Type: "GITHUB", Version: "abc123"},
		Phases:       []model.CodeBuildPhase{{Name: "BUILD", Status: "FAILED", Duration: time.Minute, Contexts: []string{"tests failed"}}},
		LogGroupName: "/aws/codebuild/api", LogStreamName: "api:7",
		Environment: []model.CodeBuildEnvVar{{Name: "TOKEN", Value: "/prod/token", Type: "PARAMETER_STORE"}},
	}
	got := formatCodeBuildDetail(detail)
	for _, want := range []string{"FAILED", "tests failed", "/aws/codebuild/api", "api:7", "PARAMETER_STORE", "/prod/token"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail %q missing %q", got, want)
		}
	}
}
