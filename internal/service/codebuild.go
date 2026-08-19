package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

const CodeBuildHistoryLimit = 50

// CodeBuildAPI is the low-level CodeBuild behavior used by shared workflows.
type CodeBuildAPI interface {
	ListCBProjects(context.Context) ([]model.CodeBuildProject, error)
	ListCBBuilds(context.Context, string, int) ([]model.CodeBuildBuild, error)
	GetCBBuildDetail(context.Context, string) (*model.CodeBuildDetail, error)
	StartCBBuild(context.Context, string, string) (*model.CodeBuildBuild, error)
	StopCBBuild(context.Context, string) error
}

// CodeBuild exposes CodeBuild workflows without frontend dependencies.
type CodeBuild struct {
	api CodeBuildAPI
}

func NewCodeBuild(api CodeBuildAPI) *CodeBuild {
	return &CodeBuild{api: api}
}

func (s *CodeBuild) ListProjects(ctx context.Context, filter string) ([]model.CodeBuildProject, error) {
	projects, err := s.api.ListCBProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list CodeBuild projects: %w", err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" {
		filtered := make([]model.CodeBuildProject, 0, len(projects))
		for _, project := range projects {
			if strings.Contains(strings.ToLower(project.Name), filter) ||
				strings.Contains(strings.ToLower(project.Description), filter) ||
				strings.Contains(strings.ToLower(project.Source), filter) {
				filtered = append(filtered, project)
			}
		}
		projects = filtered
	}
	sort.SliceStable(projects, func(i, j int) bool {
		if projects[i].LastModified.Equal(projects[j].LastModified) {
			return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
		}
		return projects[i].LastModified.After(projects[j].LastModified)
	})
	return projects, nil
}

func (s *CodeBuild) ListBuilds(ctx context.Context, projectName string, limit int) ([]model.CodeBuildBuild, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil, fmt.Errorf("list CodeBuild builds: project name is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("list CodeBuild builds for %q: limit must be positive", projectName)
	}
	builds, err := s.api.ListCBBuilds(ctx, projectName, limit)
	if err != nil {
		return nil, fmt.Errorf("list CodeBuild builds for %q: %w", projectName, err)
	}
	sort.SliceStable(builds, func(i, j int) bool {
		return builds[i].StartTime.After(builds[j].StartTime)
	})
	return builds, nil
}

func (s *CodeBuild) Detail(ctx context.Context, buildID string) (*model.CodeBuildDetail, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("read CodeBuild build: build ID is required")
	}
	detail, err := s.api.GetCBBuildDetail(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("read CodeBuild build %q: %w", buildID, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("read CodeBuild build %q: build was not found", buildID)
	}
	return detail, nil
}

func (s *CodeBuild) Start(ctx context.Context, projectName, sourceVersion string) (*model.CodeBuildBuild, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil, fmt.Errorf("start CodeBuild build: project name is required")
	}
	build, err := s.api.StartCBBuild(ctx, projectName, strings.TrimSpace(sourceVersion))
	if err != nil {
		return nil, fmt.Errorf("start CodeBuild build for %q: %w", projectName, err)
	}
	if build == nil {
		return nil, fmt.Errorf("start CodeBuild build for %q: API returned no build", projectName)
	}
	return build, nil
}

func (s *CodeBuild) Stop(ctx context.Context, buildID, status string) error {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return fmt.Errorf("stop CodeBuild build: build ID is required")
	}
	if status != "IN_PROGRESS" {
		return fmt.Errorf("stop CodeBuild build %q: build is not in progress", buildID)
	}
	if err := s.api.StopCBBuild(ctx, buildID); err != nil {
		return fmt.Errorf("stop CodeBuild build %q: %w", buildID, err)
	}
	return nil
}
