//go:build gui

package gui

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openCodeBuildModule() {
	if w.currentPage == pageCodeBuildProjects {
		return
	}
	if w.guardEditorNavigation(w.openCodeBuildModule) {
		return
	}
	w.loadCodeBuildProjects()
}

func (w *mainWindow) loadCodeBuildProjects() {
	w.resetWorkspaceForBrowserChange()
	w.clearCodeBuildProjects()
	w.clearCodeBuildBuilds()
	w.currentPage = pageCodeBuildProjects
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb("CodeBuild / Projects")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter projects…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageCodeBuildProjects)
	w.setDetail("Loading CodeBuild projects…", detailIntro)

	if w.options.CodeBuild == nil {
		w.setDetail("CodeBuild is unavailable because no CodeBuild service was configured.", detailError)
		w.setStatus("CodeBuild service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading CodeBuild projects…")
	go func() {
		projects, err := w.options.CodeBuild.ListProjects(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allCodeBuildProjects = projects
			w.applyCodeBuildProjectFilter()
			w.setDetail(codeBuildProjectListSummary(len(projects)), detailIntro)
		})
	}()
}

func (w *mainWindow) loadCodeBuildBuilds(projectName string) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" || w.options.CodeBuild == nil {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearCodeBuildBuilds()
	w.currentPage = pageCodeBuildBuilds
	w.selectedCodeBuildProject = projectName
	w.updateActionSensitivity()
	w.setBreadcrumb("CodeBuild / " + projectName + " / Builds")
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter builds…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageCodeBuildBuilds)
	w.setDetail("Loading builds for "+projectName+"…", detailIntro)

	ctx, generation := w.startRequest("Loading builds for " + projectName + "…")
	go func() {
		builds, err := w.options.CodeBuild.ListBuilds(ctx, projectName, service.CodeBuildHistoryLimit)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageCodeBuildBuilds || w.selectedCodeBuildProject != projectName {
				return
			}
			w.allCodeBuildBuilds = builds
			w.applyCodeBuildBuildFilter()
			w.setDetail(codeBuildBuildListSummary(projectName, len(builds)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshCodeBuild(foreground bool) {
	if w.options.CodeBuild == nil {
		return
	}
	if w.currentPage == pageCodeBuildProjects {
		selected := w.selectedCodeBuildProject
		ctx, generation := w.startRefreshRequest("Refreshing CodeBuild projects…", foreground)
		go func() {
			projects, err := w.options.CodeBuild.ListProjects(ctx, "")
			w.finishRefreshRequest(ctx, generation, err, foreground, func() {
				w.allCodeBuildProjects = projects
				w.applyCodeBuildProjectFilter()
				if project, found := findCodeBuildProject(projects, selected); found {
					w.setDetail(formatCodeBuildProject(project), detailIntro)
				} else {
					w.selectedCodeBuildProject = ""
					w.setBreadcrumb("CodeBuild / Projects")
					w.setDetail(codeBuildProjectListSummary(len(projects)), detailIntro)
				}
				w.updateActionSensitivity()
			})
		}()
		return
	}

	projectName, selected := w.selectedCodeBuildProject, w.selectedCodeBuild
	ctx, generation := w.startRefreshRequest("Refreshing builds for "+projectName+"…", foreground)
	go func() {
		builds, err := w.options.CodeBuild.ListBuilds(ctx, projectName, service.CodeBuildHistoryLimit)
		var detail *model.CodeBuildDetail
		if err == nil && selected != "" {
			if _, found := findCodeBuildBuild(builds, selected); found {
				detail, err = w.options.CodeBuild.Detail(ctx, selected)
			}
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allCodeBuildBuilds = builds
			w.applyCodeBuildBuildFilter()
			if build, found := findCodeBuildBuild(builds, selected); found {
				if detail != nil {
					w.codeBuildDetail = detail
					w.setDetail(formatCodeBuildDetail(*detail), detailCodeBuild)
				} else {
					w.setDetail(formatCodeBuildBuildSummary(projectName, build), detailIntro)
				}
			} else {
				w.selectedCodeBuild = ""
				w.codeBuildDetail = nil
				w.setBreadcrumb("CodeBuild / " + projectName + " / Builds")
				w.setDetail(codeBuildBuildListSummary(projectName, len(builds)), detailIntro)
			}
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearCodeBuildProjects() {
	w.allCodeBuildProjects = nil
	w.filteredCodeBuildProjects = nil
	w.selectedCodeBuildProject = ""
	if w.codeBuildProjectTable != nil {
		w.codeBuildProjectTable.clear()
	}
}

func (w *mainWindow) clearCodeBuildBuilds() {
	w.allCodeBuildBuilds = nil
	w.filteredCodeBuildBuilds = nil
	w.selectedCodeBuild = ""
	w.codeBuildDetail = nil
	if w.codeBuildBuildTable != nil {
		w.codeBuildBuildTable.clear()
	}
}

func (w *mainWindow) applyCodeBuildProjectFilter() {
	w.filteredCodeBuildProjects = filterCodeBuildProjects(w.allCodeBuildProjects, w.search.Text())
	rows := make([]string, len(w.filteredCodeBuildProjects))
	for i, project := range w.filteredCodeBuildProjects {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s", project.Name, valueOrDash(project.Source),
			valueOrDash(project.Description), formatTime(project.LastModified))
	}
	w.codeBuildProjectTable.replace(rows)
}

func (w *mainWindow) applyCodeBuildBuildFilter() {
	w.filteredCodeBuildBuilds = filterCodeBuildBuilds(w.allCodeBuildBuilds, w.search.Text())
	rows := make([]string, len(w.filteredCodeBuildBuilds))
	for i, build := range w.filteredCodeBuildBuilds {
		duration := valueOrDash(build.CurrentPhase)
		if build.Duration > 0 {
			duration = build.Duration.Truncate(time.Second).String()
		}
		rows[i] = fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s", build.BuildNumber,
			valueOrDash(build.Status), formatTime(build.StartTime), duration,
			valueOrDash(build.Initiator), valueOrDash(build.SourceVersion))
	}
	w.codeBuildBuildTable.replace(rows)
}

func (w *mainWindow) selectCodeBuildProjectRow() {
	if w.currentPage != pageCodeBuildProjects {
		return
	}
	position := w.codeBuildProjectTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredCodeBuildProjects) {
		w.selectedCodeBuildProject = ""
		w.setBreadcrumb("CodeBuild / Projects")
		w.setDetail(codeBuildProjectListSummary(len(w.allCodeBuildProjects)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	project := w.filteredCodeBuildProjects[position]
	if w.selectedCodeBuildProject != project.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedCodeBuildProject = project.Name
	w.setBreadcrumb("CodeBuild / Projects / " + project.Name)
	w.setDetail(formatCodeBuildProject(project), detailIntro)
	w.updateActionSensitivity()
}

func (w *mainWindow) openCodeBuildProjectAt(position uint) {
	if int(position) >= len(w.filteredCodeBuildProjects) {
		return
	}
	project := w.filteredCodeBuildProjects[position]
	w.codeBuildProjectTable.selection.SetSelected(position)
	w.loadCodeBuildBuilds(project.Name)
}

func (w *mainWindow) selectCodeBuildBuildRow() {
	if w.currentPage != pageCodeBuildBuilds {
		return
	}
	position := w.codeBuildBuildTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredCodeBuildBuilds) {
		w.selectedCodeBuild = ""
		w.codeBuildDetail = nil
		w.setBreadcrumb("CodeBuild / " + w.selectedCodeBuildProject + " / Builds")
		w.setDetail(codeBuildBuildListSummary(w.selectedCodeBuildProject, len(w.allCodeBuildBuilds)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	build := w.filteredCodeBuildBuilds[position]
	if w.selectedCodeBuild != build.ID {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedCodeBuild = build.ID
	w.codeBuildDetail = nil
	w.setBreadcrumb(fmt.Sprintf("CodeBuild / %s / Build #%d", w.selectedCodeBuildProject, build.BuildNumber))
	w.setDetail("Loading build details…\n\n"+formatCodeBuildBuildSummary(w.selectedCodeBuildProject, build), detailCodeBuild)
	w.updateActionSensitivity()
	w.loadCodeBuildDetail(build.ID)
}

func (w *mainWindow) openCodeBuildBuildAt(position uint) {
	if int(position) >= len(w.filteredCodeBuildBuilds) {
		return
	}
	if w.codeBuildBuildTable.selection.Selected() == position {
		w.loadCodeBuildDetail(w.filteredCodeBuildBuilds[position].ID)
	} else {
		w.codeBuildBuildTable.selection.SetSelected(position)
	}
}

func (w *mainWindow) loadCodeBuildDetail(buildID string) {
	if buildID == "" || w.options.CodeBuild == nil || w.codeBuildActionPending {
		return
	}
	w.codeBuildActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Loading CodeBuild build details…")
	go func() {
		detail, err := w.options.CodeBuild.Detail(ctx, buildID)
		w.finishCodeBuildRequest(ctx, generation, err, "", true, func() {
			if w.currentPage != pageCodeBuildBuilds || w.selectedCodeBuild != buildID {
				return
			}
			w.codeBuildDetail = detail
			w.setDetail(formatCodeBuildDetail(*detail), detailCodeBuild)
		})
	}()
}

func (w *mainWindow) finishCodeBuildRequest(ctx context.Context, generation uint64, err error, success string, showDetailError bool, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.codeBuildActionPending = false
		if err != nil {
			w.updateActionSensitivity()
			w.setStatus(err.Error(), true)
			if showDetailError {
				w.setDetail("ERROR\n\n"+err.Error(), detailError)
			}
			return
		}
		if success == "" {
			success = "Updated " + time.Now().Format("15:04:05")
		}
		w.lastSuccessfulLoad = time.Now()
		w.setStatus(success, false)
		if apply != nil {
			apply()
		}
		// Successful callbacks install the selected build detail and its log
		// destination. Recompute contextual actions afterward so View logs and
		// Search logs become visible as soon as that detail is ready.
		w.updateActionSensitivity()
	})
}

func (w *mainWindow) currentCodeBuildProject() string {
	if w.currentPage == pageCodeBuildProjects {
		return w.selectedCodeBuildProject
	}
	if w.currentPage == pageCodeBuildBuilds {
		return w.selectedCodeBuildProject
	}
	return ""
}

func (w *mainWindow) confirmStartCodeBuild() {
	projectName := w.currentCodeBuildProject()
	if projectName == "" || w.codeBuildActionPending || w.options.CodeBuild == nil {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle("Start CodeBuild build")
	dialog.SetMarkup("Start a new build for <b>" + html.EscapeString(projectName) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "The project will use its configured source version and environment.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.startCodeBuild(projectName)
		}
		w.updateActionSensitivity()
	})
	dialog.Present()
}

func (w *mainWindow) startCodeBuild(projectName string) {
	w.codeBuildActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Starting CodeBuild build for " + projectName + "…")
	go func() {
		build, err := w.options.CodeBuild.Start(ctx, projectName, "")
		success := ""
		if build != nil {
			success = fmt.Sprintf("Started build #%d for %s", build.BuildNumber, projectName)
		}
		w.finishCodeBuildRequest(ctx, generation, err, success, false, func() {
			w.loadCodeBuildBuilds(projectName)
		})
	}()
}

func (w *mainWindow) confirmStopCodeBuild() {
	detail := w.codeBuildDetail
	if detail == nil || detail.ID != w.selectedCodeBuild || detail.Status != "IN_PROGRESS" ||
		w.codeBuildActionPending || w.options.CodeBuild == nil {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Stop CodeBuild build")
	dialog.SetMarkup(fmt.Sprintf("Stop <b>%s build #%d</b>?", html.EscapeString(detail.ProjectName), detail.BuildNumber))
	dialog.SetObjectProperty("secondary-text", "CodeBuild will stop the running build and mark it stopped.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.stopCodeBuild(detail.ID, detail.Status)
		}
	})
	dialog.Present()
}

func (w *mainWindow) stopCodeBuild(buildID, status string) {
	w.codeBuildActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Stopping CodeBuild build…")
	go func() {
		err := w.options.CodeBuild.Stop(ctx, buildID, status)
		w.finishCodeBuildRequest(ctx, generation, err, "Build stop requested", false, func() {
			w.refreshCodeBuild(true)
		})
	}()
}

func (w *mainWindow) selectedCodeBuildLogSource() (model.LogSource, *model.CodeBuildDetail, bool) {
	detail := w.codeBuildDetail
	if detail == nil || detail.ID != w.selectedCodeBuild || detail.LogGroupName == "" || detail.LogStreamName == "" {
		return model.LogSource{}, nil, false
	}
	return model.LogSource{Group: detail.LogGroupName, Streams: []string{detail.LogStreamName}}, detail, true
}

func (w *mainWindow) viewCodeBuildLogs() {
	source, detail, ok := w.selectedCodeBuildLogSource()
	if !ok || w.options.Logs == nil || w.codeBuildActionPending {
		return
	}
	title := fmt.Sprintf("%s build #%d", detail.ProjectName, detail.BuildNumber)
	start := max(int64(0), detail.StartTime.Add(-time.Minute).UnixMilli())
	if detail.Status == "IN_PROGRESS" {
		w.showLogFollowFrom(source, title, start)
		return
	}
	w.codeBuildActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Loading CodeBuild logs…")
	pageSize := w.configuredLogPageSize()
	go func() {
		page, err := w.options.Logs.Fetch(ctx, source.Group, model.LogQuery{
			Streams: source.Streams, StartTime: start, Limit: pageSize, Tail: true, FallbackLimit: 10,
		})
		status := fmt.Sprintf("Loaded %d events for build #%d", len(page.Entries), detail.BuildNumber)
		w.finishCodeBuildRequest(ctx, generation, err, status, false, func() {
			w.showLogSnapshot(source, title, page)
		})
	}()
}

func (w *mainWindow) searchCodeBuildLogs() {
	source, detail, ok := w.selectedCodeBuildLogSource()
	if !ok || w.options.Logs == nil || w.codeBuildActionPending {
		return
	}
	end := time.Now().UnixMilli()
	if !detail.EndTime.IsZero() {
		end = detail.EndTime.Add(time.Minute).UnixMilli()
	}
	spec := cloudWatchSearch{
		Groups: []string{source.Group}, Streams: append([]string(nil), source.Streams...),
		StartTime: max(int64(0), detail.StartTime.Add(-time.Minute).UnixMilli()), EndTime: end,
		Title: fmt.Sprintf("%s build #%d", detail.ProjectName, detail.BuildNumber),
	}
	w.promptCloudWatchSearchWithDefaults(source.Group, &spec)
}

func filterCodeBuildProjects(projects []model.CodeBuildProject, query string) []model.CodeBuildProject {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return projects
	}
	filtered := make([]model.CodeBuildProject, 0, len(projects))
	for _, project := range projects {
		if strings.Contains(strings.ToLower(project.Name), query) ||
			strings.Contains(strings.ToLower(project.Description), query) ||
			strings.Contains(strings.ToLower(project.Source), query) {
			filtered = append(filtered, project)
		}
	}
	return filtered
}

func filterCodeBuildBuilds(builds []model.CodeBuildBuild, query string) []model.CodeBuildBuild {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return builds
	}
	filtered := make([]model.CodeBuildBuild, 0, len(builds))
	for _, build := range builds {
		searchable := fmt.Sprintf("%d %s %s %s %s", build.BuildNumber, build.Status,
			build.Initiator, build.SourceVersion, build.CurrentPhase)
		if strings.Contains(strings.ToLower(searchable), query) {
			filtered = append(filtered, build)
		}
	}
	return filtered
}

func findCodeBuildProject(projects []model.CodeBuildProject, name string) (model.CodeBuildProject, bool) {
	for _, project := range projects {
		if project.Name == name {
			return project, true
		}
	}
	return model.CodeBuildProject{}, false
}

func findCodeBuildBuild(builds []model.CodeBuildBuild, id string) (model.CodeBuildBuild, bool) {
	for _, build := range builds {
		if build.ID == id {
			return build, true
		}
	}
	return model.CodeBuildBuild{}, false
}

func codeBuildProjectListSummary(count int) string {
	return fmt.Sprintf("CodeBuild projects: %d\n\nSelect a project for its summary; double-click it to browse recent builds.", count)
}

func formatCodeBuildProject(project model.CodeBuildProject) string {
	return fmt.Sprintf("PROJECT\n\nName:          %s\nSource:        %s\nLast modified: %s\nDescription:   %s\n\nDouble-click this project to browse recent builds.",
		project.Name, valueOrDash(project.Source), formatTime(project.LastModified), valueOrDash(project.Description))
}

func codeBuildBuildListSummary(project string, count int) string {
	return fmt.Sprintf("PROJECT BUILDS\n\nProject: %s\nLoaded:  %d most recent builds\n\nSelect a build for its summary.", project, count)
}

func formatCodeBuildBuildSummary(project string, build model.CodeBuildBuild) string {
	duration := valueOrDash(build.CurrentPhase)
	if build.Duration > 0 {
		duration = build.Duration.Truncate(time.Second).String()
	}
	return fmt.Sprintf("BUILD SUMMARY\n\nProject:        %s\nBuild:          #%d\nStatus:         %s\nStarted:        %s\nDuration/phase: %s\nInitiator:      %s\nSource version: %s\nBuild ID:       %s",
		project, build.BuildNumber, valueOrDash(build.Status), formatTime(build.StartTime),
		duration, valueOrDash(build.Initiator), valueOrDash(build.SourceVersion), build.ID)
}

func formatCodeBuildDetail(detail model.CodeBuildDetail) string {
	var out strings.Builder
	fmt.Fprintf(&out, "CODEBUILD BUILD\n\nProject        %s\nBuild          #%d\nStatus         %s\nBuild ID       %s\nARN            %s\nStarted        %s\n",
		detail.ProjectName, detail.BuildNumber, valueOrDash(detail.Status), detail.ID,
		valueOrDash(detail.ARN), formatTime(detail.StartTime))
	if !detail.EndTime.IsZero() {
		fmt.Fprintf(&out, "Ended          %s\nDuration       %s\n", formatTime(detail.EndTime), detail.Duration.Truncate(time.Second))
	} else if detail.CurrentPhase != "" {
		fmt.Fprintf(&out, "Current phase  %s\n", detail.CurrentPhase)
	}
	fmt.Fprintf(&out, "Initiator      %s\n\nSOURCE\n\nType           %s\nLocation       %s\nVersion        %s\n",
		valueOrDash(detail.Initiator), valueOrDash(detail.Source.Type),
		valueOrDash(detail.Source.Location), valueOrDash(detail.Source.Version))

	if len(detail.Phases) > 0 {
		out.WriteString("\nPHASES\n")
		for _, phase := range detail.Phases {
			duration := "—"
			if phase.Duration > 0 {
				duration = phase.Duration.Truncate(time.Second).String()
			}
			fmt.Fprintf(&out, "\n%-18s %-14s %s\n", phase.Name, valueOrDash(phase.Status), duration)
			for _, message := range phase.Contexts {
				fmt.Fprintf(&out, "  %s\n", message)
			}
		}
	}

	fmt.Fprintf(&out, "\nLOGS\n\nGroup          %s\nStream         %s\n",
		valueOrDash(detail.LogGroupName), valueOrDash(detail.LogStreamName))
	if len(detail.Environment) > 0 {
		out.WriteString("\nENVIRONMENT\n")
		for _, variable := range detail.Environment {
			value := variable.Value
			if variable.Type != "" && variable.Type != "PLAINTEXT" {
				value = "[" + variable.Type + "] " + value
			}
			fmt.Fprintf(&out, "\n%-30s %s\n", variable.Name, valueOrDash(value))
		}
	}
	return strings.TrimRight(out.String(), "\n")
}
