package aws

import (
	"context"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
)

// CBProject represents a CodeBuild project summary.
type CBProject struct {
	Name         string
	Description  string
	Source       string // source type (CODECOMMIT, GITHUB, etc.)
	LastModified time.Time
}

// CBBuild represents a CodeBuild build summary.
type CBBuild struct {
	ID            string
	BuildNumber   int64
	Status        string // SUCCEEDED, FAILED, IN_PROGRESS, STOPPED, TIMED_OUT, FAULT
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	Initiator     string
	SourceVersion string
	CurrentPhase  string
}

// CBBuildDetail holds extended build information.
type CBBuildDetail struct {
	CBBuild
	ProjectName   string
	Arn           string
	Source        CBSource
	Phases        []CBBuildPhase
	LogGroupName  string
	LogStreamName string
	Environment   []CBEnvVar
}

// CBSource describes the build source.
type CBSource struct {
	Type     string
	Location string
	Version  string
}

// CBBuildPhase represents a build phase.
type CBBuildPhase struct {
	Name     string
	Status   string
	Duration time.Duration
	Contexts []string // error messages
}

// CBEnvVar represents a build environment variable.
type CBEnvVar struct {
	Name  string
	Value string
	Type  string // PLAINTEXT, PARAMETER_STORE, SECRETS_MANAGER
}

// ListCBProjects returns all CodeBuild projects with summary info.
func (c *Client) ListCBProjects(ctx context.Context) ([]CBProject, error) {
	var projectNames []string
	paginator := codebuild.NewListProjectsPaginator(c.CodeBuild, &codebuild.ListProjectsInput{
		SortBy:    cbtypes.ProjectSortByTypeLastModifiedTime,
		SortOrder: cbtypes.SortOrderTypeDescending,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		projectNames = append(projectNames, page.Projects...)
	}

	if len(projectNames) == 0 {
		return nil, nil
	}

	// Fetch project details in batches of 100
	var projects []CBProject
	for i := 0; i < len(projectNames); i += 100 {
		end := min(i+100, len(projectNames))
		batch, err := c.CodeBuild.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
			Names: projectNames[i:end],
		})
		if err != nil {
			return nil, err
		}
		for _, p := range batch.Projects {
			proj := CBProject{
				Name:        derefStrAws(p.Name),
				Description: derefStrAws(p.Description),
			}
			if p.Source != nil {
				proj.Source = string(p.Source.Type)
			}
			if p.LastModified != nil {
				proj.LastModified = *p.LastModified
			}
			projects = append(projects, proj)
		}
	}

	return projects, nil
}

// ListCBBuilds returns recent builds for a project.
func (c *Client) ListCBBuilds(ctx context.Context, projectName string, maxBuilds int) ([]CBBuild, error) {
	var buildIDs []string
	input := &codebuild.ListBuildsForProjectInput{
		ProjectName: &projectName,
		SortOrder:   cbtypes.SortOrderTypeDescending,
	}
	// Collect up to maxBuilds IDs
	paginator := codebuild.NewListBuildsForProjectPaginator(c.CodeBuild, input)
	for paginator.HasMorePages() && len(buildIDs) < maxBuilds {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		buildIDs = append(buildIDs, page.Ids...)
	}
	if len(buildIDs) > maxBuilds {
		buildIDs = buildIDs[:maxBuilds]
	}
	if len(buildIDs) == 0 {
		return nil, nil
	}

	return c.batchGetBuilds(ctx, buildIDs)
}

// GetCBBuildDetail fetches full detail for a single build.
func (c *Client) GetCBBuildDetail(ctx context.Context, buildID string) (*CBBuildDetail, error) {
	out, err := c.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: []string{buildID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Builds) == 0 {
		return nil, nil
	}

	b := out.Builds[0]
	detail := &CBBuildDetail{
		CBBuild:     buildFromSDK(b),
		ProjectName: derefStrAws(b.ProjectName),
		Arn:         derefStrAws(b.Arn),
	}

	if b.Source != nil {
		detail.Source = CBSource{
			Type:     string(b.Source.Type),
			Location: derefStrAws(b.Source.Location),
		}
	}
	if b.SourceVersion != nil {
		detail.Source.Version = *b.SourceVersion
	}

	if b.Logs != nil {
		detail.LogGroupName = derefStrAws(b.Logs.GroupName)
		detail.LogStreamName = derefStrAws(b.Logs.StreamName)
	}

	for _, p := range b.Phases {
		phase := CBBuildPhase{
			Name:   string(p.PhaseType),
			Status: string(p.PhaseStatus),
		}
		if p.DurationInSeconds != nil {
			phase.Duration = time.Duration(*p.DurationInSeconds) * time.Second
		}
		for _, ctx := range p.Contexts {
			if ctx.Message != nil {
				phase.Contexts = append(phase.Contexts, *ctx.Message)
			}
		}
		detail.Phases = append(detail.Phases, phase)
	}

	if b.Environment != nil {
		for _, ev := range b.Environment.EnvironmentVariables {
			detail.Environment = append(detail.Environment, CBEnvVar{
				Name:  derefStrAws(ev.Name),
				Value: derefStrAws(ev.Value),
				Type:  string(ev.Type),
			})
		}
	}

	return detail, nil
}

// StartCBBuild triggers a new build for the given project.
func (c *Client) StartCBBuild(ctx context.Context, projectName string, sourceVersion string) (*CBBuild, error) {
	input := &codebuild.StartBuildInput{
		ProjectName: &projectName,
	}
	if sourceVersion != "" {
		input.SourceVersion = &sourceVersion
	}
	out, err := c.CodeBuild.StartBuild(ctx, input)
	if err != nil {
		return nil, err
	}
	if out.Build == nil {
		return nil, nil
	}
	build := buildFromSDK(*out.Build)
	return &build, nil
}

// StopCBBuild stops an in-progress build.
func (c *Client) StopCBBuild(ctx context.Context, buildID string) error {
	_, err := c.CodeBuild.StopBuild(ctx, &codebuild.StopBuildInput{
		Id: &buildID,
	})
	return err
}

func (c *Client) batchGetBuilds(ctx context.Context, ids []string) ([]CBBuild, error) {
	var builds []CBBuild
	for i := 0; i < len(ids); i += 100 {
		end := min(i+100, len(ids))
		out, err := c.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
			Ids: ids[i:end],
		})
		if err != nil {
			return nil, err
		}
		for _, b := range out.Builds {
			builds = append(builds, buildFromSDK(b))
		}
	}
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].StartTime.After(builds[j].StartTime)
	})
	return builds, nil
}

func buildFromSDK(b cbtypes.Build) CBBuild {
	build := CBBuild{
		ID:            derefStrAws(b.Id),
		Status:        string(b.BuildStatus),
		Initiator:     derefStrAws(b.Initiator),
		SourceVersion: derefStrAws(b.SourceVersion),
		CurrentPhase:  derefStrAws(b.CurrentPhase),
	}
	if b.BuildNumber != nil {
		build.BuildNumber = *b.BuildNumber
	}
	if b.StartTime != nil {
		build.StartTime = *b.StartTime
	}
	if b.EndTime != nil {
		build.EndTime = *b.EndTime
		build.Duration = build.EndTime.Sub(build.StartTime)
	}
	return build
}
