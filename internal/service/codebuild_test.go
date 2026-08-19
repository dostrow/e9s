package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeCodeBuildAPI struct {
	projects      []model.CodeBuildProject
	builds        []model.CodeBuildBuild
	detail        *model.CodeBuildDetail
	started       *model.CodeBuildBuild
	err           error
	projectName   string
	limit         int
	buildID       string
	sourceVersion string
	stoppedID     string
}

func (f *fakeCodeBuildAPI) ListCBProjects(context.Context) ([]model.CodeBuildProject, error) {
	return append([]model.CodeBuildProject(nil), f.projects...), f.err
}

func (f *fakeCodeBuildAPI) ListCBBuilds(_ context.Context, project string, limit int) ([]model.CodeBuildBuild, error) {
	f.projectName, f.limit = project, limit
	return append([]model.CodeBuildBuild(nil), f.builds...), f.err
}

func (f *fakeCodeBuildAPI) GetCBBuildDetail(_ context.Context, id string) (*model.CodeBuildDetail, error) {
	f.buildID = id
	return f.detail, f.err
}

func (f *fakeCodeBuildAPI) StartCBBuild(_ context.Context, project, sourceVersion string) (*model.CodeBuildBuild, error) {
	f.projectName, f.sourceVersion = project, sourceVersion
	return f.started, f.err
}

func (f *fakeCodeBuildAPI) StopCBBuild(_ context.Context, id string) error {
	f.stoppedID = id
	return f.err
}

func TestCodeBuildProjectsFilterAndSort(t *testing.T) {
	now := time.Now()
	api := &fakeCodeBuildAPI{projects: []model.CodeBuildProject{
		{Name: "worker", Description: "background", Source: "GITHUB", LastModified: now.Add(-time.Hour)},
		{Name: "api", Description: "production service", Source: "CODECOMMIT", LastModified: now},
		{Name: "API-canary", Description: "smoke test", Source: "GITHUB", LastModified: now},
	}}
	got, err := NewCodeBuild(api).ListProjects(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{got[0].Name, got[1].Name}
	if !reflect.DeepEqual(names, []string{"api", "API-canary"}) {
		t.Fatalf("ListProjects() names = %#v", names)
	}
}

func TestCodeBuildBuildsValidateAndSort(t *testing.T) {
	now := time.Now()
	api := &fakeCodeBuildAPI{builds: []model.CodeBuildBuild{
		{ID: "older", StartTime: now.Add(-time.Hour)},
		{ID: "newer", StartTime: now},
	}}
	svc := NewCodeBuild(api)
	got, err := svc.ListBuilds(context.Background(), " api ", CodeBuildHistoryLimit)
	if err != nil {
		t.Fatal(err)
	}
	if api.projectName != "api" || api.limit != CodeBuildHistoryLimit || got[0].ID != "newer" {
		t.Fatalf("ListBuilds() = %#v, project %q, limit %d", got, api.projectName, api.limit)
	}
	if _, err := svc.ListBuilds(context.Background(), "", 50); err == nil {
		t.Fatal("empty project name was accepted")
	}
	if _, err := svc.ListBuilds(context.Background(), "api", 0); err == nil {
		t.Fatal("non-positive limit was accepted")
	}
}

func TestCodeBuildDetailAndMutations(t *testing.T) {
	api := &fakeCodeBuildAPI{
		detail:  &model.CodeBuildDetail{CodeBuildBuild: model.CodeBuildBuild{ID: "api:1"}},
		started: &model.CodeBuildBuild{ID: "api:2", BuildNumber: 2},
	}
	svc := NewCodeBuild(api)
	if _, err := svc.Detail(context.Background(), " api:1 "); err != nil || api.buildID != "api:1" {
		t.Fatalf("Detail() error = %v, ID %q", err, api.buildID)
	}
	if _, err := svc.Start(context.Background(), " api ", " main "); err != nil || api.projectName != "api" || api.sourceVersion != "main" {
		t.Fatalf("Start() error = %v, project %q, version %q", err, api.projectName, api.sourceVersion)
	}
	if err := svc.Stop(context.Background(), " api:1 ", "IN_PROGRESS"); err != nil || api.stoppedID != "api:1" {
		t.Fatalf("Stop() error = %v, ID %q", err, api.stoppedID)
	}
	if err := svc.Stop(context.Background(), "api:1", "SUCCEEDED"); err == nil {
		t.Fatal("terminal build was accepted for stop")
	}
}

func TestCodeBuildReportsMissingResultsAndWrapsErrors(t *testing.T) {
	svc := NewCodeBuild(&fakeCodeBuildAPI{})
	if _, err := svc.Detail(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Detail() error = %v", err)
	}
	if _, err := svc.Start(context.Background(), "api", ""); err == nil || !strings.Contains(err.Error(), "returned no build") {
		t.Fatalf("Start() error = %v", err)
	}

	api := &fakeCodeBuildAPI{err: errors.New("denied")}
	if _, err := NewCodeBuild(api).ListProjects(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list CodeBuild projects") {
		t.Fatalf("ListProjects() error = %v", err)
	}
}
