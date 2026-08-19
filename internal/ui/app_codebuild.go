package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- CodeBuild ---

func (a App) openCBProjects() (App, tea.Cmd) {
	a.mode = modeCodeBuild
	a.state = viewCBProjects
	a.cbProjectsView = views.NewCBProjects()
	a.cbProjectsView = a.cbProjectsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	codeBuild := a.codeBuild
	ctx := a.ctx
	return a, func() tea.Msg {
		projects, err := codeBuild.ListProjects(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return cbProjectsLoadedMsg{projects}
	}
}

func (a App) openCBBuilds(projectName string) (App, tea.Cmd) {
	a.state = viewCBBuilds
	a.cbBuildsView = views.NewCBBuilds(projectName)
	a.cbBuildsView = a.cbBuildsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	codeBuild := a.codeBuild
	ctx := a.ctx
	return a, func() tea.Msg {
		builds, err := codeBuild.ListBuilds(ctx, projectName, service.CodeBuildHistoryLimit)
		if err != nil {
			return errMsg{err}
		}
		return cbBuildsLoadedMsg{builds}
	}
}

func (a App) openCBBuildDetail() (App, tea.Cmd) {
	build := a.cbBuildsView.SelectedBuild()
	if build == nil {
		return a, nil
	}
	a.state = viewCBBuildDetail
	a.loading = true
	codeBuild := a.codeBuild
	ctx := a.ctx
	buildID := build.ID
	return a, func() tea.Msg {
		detail, err := codeBuild.Detail(ctx, buildID)
		if err != nil {
			return errMsg{err}
		}
		return cbBuildDetailLoadedMsg{detail}
	}
}

func (a App) triggerCBBuild() (App, tea.Cmd) {
	var projectName string
	switch a.state {
	case viewCBBuilds:
		projectName = a.cbBuildsView.ProjectName()
	case viewCBBuildDetail:
		projectName = a.cbBuildDetailView.ProjectName()
	case viewCBProjects:
		if p := a.cbProjectsView.SelectedProject(); p != nil {
			projectName = p.Name
		}
	}
	if projectName == "" {
		return a, nil
	}
	a.confirm = NewConfirm(ConfirmCBStartBuild,
		fmt.Sprintf("Start a new build for %q?", projectName))
	a.cbTriggerProject = projectName
	return a, nil
}

func (a App) doStartCBBuild() tea.Cmd {
	codeBuild := a.codeBuild
	ctx := a.ctx
	projectName := a.cbTriggerProject
	return func() tea.Msg {
		build, err := codeBuild.Start(ctx, projectName, "")
		if err != nil {
			return errMsg{err}
		}
		msg := fmt.Sprintf("Build #%d started for %s", build.BuildNumber, projectName)
		return cbBuildStartedMsg{message: msg}
	}
}

func (a App) stopCBBuild() (App, tea.Cmd) {
	detail := a.cbBuildDetailView.Detail()
	if detail == nil || detail.Status != "IN_PROGRESS" {
		return a, nil
	}
	a.confirm = NewConfirm(ConfirmCBStopBuild,
		fmt.Sprintf("Stop build #%d?", detail.BuildNumber))
	return a, nil
}

func (a App) doStopCBBuild() tea.Cmd {
	codeBuild := a.codeBuild
	ctx := a.ctx
	buildID := a.cbBuildDetailView.BuildID()
	status := ""
	if detail := a.cbBuildDetailView.Detail(); detail != nil {
		status = detail.Status
	}
	return func() tea.Msg {
		err := codeBuild.Stop(ctx, buildID, status)
		if err != nil {
			return errMsg{err}
		}
		return cbBuildStoppedMsg{message: "Build stopped"}
	}
}

func (a App) viewCBBuildLogs() (App, tea.Cmd) {
	detail := a.cbBuildDetailView.Detail()
	if detail == nil {
		return a, nil
	}
	logGroup := detail.LogGroupName
	logStream := detail.LogStreamName
	if logGroup == "" || logStream == "" {
		a.err = fmt.Errorf("no log information available for this build")
		return a, nil
	}
	a.prevState = viewCBBuildDetail

	// For completed builds, fetch logs from the build start time.
	// For in-progress builds, use follow mode.
	inProgress := detail.Status == "IN_PROGRESS"
	follow := inProgress
	lookback := time.Since(detail.StartTime) + time.Minute // pad 1 min
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("Build #%d logs", detail.BuildNumber),
			logGroup: logGroup,
			streams:  []string{logStream},
			follow:   &follow,
			lookback: lookback,
		}
	}
}

func (a App) searchCBBuildLogs() (App, tea.Cmd) {
	detail := a.cbBuildDetailView.Detail()
	if detail == nil {
		return a, nil
	}
	logGroup := detail.LogGroupName
	logStream := detail.LogStreamName
	if logGroup == "" {
		a.err = fmt.Errorf("no log information available for this build")
		return a, nil
	}
	a.prevState = viewCBBuildDetail
	a.logSearchGroup = logGroup
	a.logSearchGroups = []string{logGroup}
	a.logSearchStreams = []string{logStream}
	a.logSearchStartMs = detail.StartTime.Add(-time.Minute).UnixMilli()
	if detail.EndTime.IsZero() {
		a.logSearchEndMs = time.Now().UnixMilli()
	} else {
		a.logSearchEndMs = detail.EndTime.Add(time.Minute).UnixMilli()
	}
	a.input = NewInput(InputLogSearchPattern, "Search build logs (CloudWatch filter syntax)", "")
	return a, nil
}

func (a App) refreshCBBuilds() tea.Cmd {
	projectName := a.cbBuildsView.ProjectName()
	codeBuild := a.codeBuild
	ctx := a.ctx
	return func() tea.Msg {
		builds, err := codeBuild.ListBuilds(ctx, projectName, service.CodeBuildHistoryLimit)
		if err != nil {
			return errMsg{err}
		}
		return cbBuildsLoadedMsg{builds}
	}
}

func (a App) refreshCBBuildDetail() tea.Cmd {
	buildID := a.cbBuildDetailView.BuildID()
	if buildID == "" {
		return nil
	}
	codeBuild := a.codeBuild
	ctx := a.ctx
	return func() tea.Msg {
		detail, err := codeBuild.Detail(ctx, buildID)
		if err != nil {
			return errMsg{err}
		}
		return cbBuildDetailLoadedMsg{detail}
	}
}

func (a App) refreshCBProjects() tea.Cmd {
	codeBuild := a.codeBuild
	ctx := a.ctx
	return func() tea.Msg {
		projects, err := codeBuild.ListProjects(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return cbProjectsLoadedMsg{projects}
	}
}

// handleCBBuildStarted processes the result of triggering a build.
func (a App) handleCBBuildStarted(msg cbBuildStartedMsg) (App, tea.Cmd) {
	a.flashMessage = msg.message
	a.flashExpiry = time.Now().Add(5 * time.Second)
	// Refresh the builds list if we're on it
	if a.state == viewCBBuilds {
		a.loading = true
		return a, a.refreshCBBuilds()
	}
	return a, nil
}
