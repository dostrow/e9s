package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	tea "github.com/charmbracelet/bubbletea"
	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- Service Operations ---

func (a App) promptForceDeploy() (App, tea.Cmd) {
	s := a.serviceView.SelectedService()
	if s == nil {
		return a, nil
	}
	a.selectedService = s
	a.confirm = NewConfirm(ConfirmForceDeploy,
		fmt.Sprintf("Force new deployment for service '%s'?", s.Name))
	return a, nil
}

func (a App) doForceDeploy() tea.Cmd {
	cluster := ""
	service := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	if a.selectedService != nil {
		service = a.selectedService.Name
	}
	return func() tea.Msg {
		err := a.client.ForceNewDeployment(context.Background(), cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{fmt.Sprintf("Force deploy initiated for '%s'", service)}
	}
}

func (a App) promptScale() (App, tea.Cmd) {
	s := a.serviceView.SelectedService()
	if s == nil {
		return a, nil
	}
	a.selectedService = s
	a.input = NewInput(InputScale,
		fmt.Sprintf("Scale '%s' (current: %d)", s.Name, s.DesiredCount),
		fmt.Sprintf("%d", s.DesiredCount))
	return a, nil
}

func (a App) doScale(count int) tea.Cmd {
	cluster := ""
	service := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	if a.selectedService != nil {
		service = a.selectedService.Name
	}
	return func() tea.Msg {
		err := a.client.ScaleService(context.Background(), cluster, service, count)
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{fmt.Sprintf("Scaled '%s' to %d", service, count)}
	}
}

func (a App) promptStopTask() (App, tea.Cmd) {
	t := a.taskView.SelectedTask()
	if t == nil {
		return a, nil
	}
	a.selectedTask = t
	id := t.TaskID
	if len(id) > 8 {
		id = id[:8]
	}
	a.confirm = NewConfirm(ConfirmStopTask,
		fmt.Sprintf("Stop task '%s'?", id))
	return a, nil
}

func (a App) doStopTask() tea.Cmd {
	cluster := ""
	taskARN := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	if a.selectedTask != nil {
		taskARN = a.selectedTask.TaskARN
	}
	return func() tea.Msg {
		err := a.client.StopTask(context.Background(), cluster, taskARN, "Stopped by e9s")
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{"Task stopped"}
	}
}

func (a App) showServiceDetail() (App, tea.Cmd) {
	if s := a.serviceView.SelectedService(); s != nil {
		a.selectedService = s
		a.state = viewServiceDetail
		a.serviceDetailView = views.NewServiceDetail(s)
		return a, a.loadServices()
	}
	return a, nil
}

// --- Standalone Tasks ---

func (a App) showStandaloneTasks() (App, tea.Cmd) {
	a.state = viewStandaloneTasks
	a.prevState = viewServices
	a.standaloneView = views.NewStandaloneTasks()
	a.loading = true
	return a, a.loadStandaloneTasks()
}

func (a App) loadStandaloneTasks() tea.Cmd {
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	return func() tea.Msg {
		tasks, err := a.client.ListTasks(context.Background(), clusterName, "")
		if err != nil {
			return errMsg{err}
		}
		return standaloneTasksLoadedMsg{tasks}
	}
}

func (a App) openStandaloneTaskLogs() (App, tea.Cmd) {
	t := a.standaloneView.SelectedTask()
	if t == nil {
		return a, nil
	}
	a.selectedTask = t
	a.prevState = viewStandaloneTasks

	if len(t.Containers) == 0 {
		a.err = fmt.Errorf("task has no containers")
		return a, nil
	}
	if len(t.Containers) == 1 {
		return a, a.doLogForContainer(t.Containers[0].Name)
	}
	names := make([]string, len(t.Containers))
	for i, c := range t.Containers {
		names[i] = c.Name
	}
	a.picker = NewPicker(PickerLogContainer, "Select container for logs", names)
	return a, nil
}

func (a App) promptStopStandaloneTask() (App, tea.Cmd) {
	t := a.standaloneView.SelectedTask()
	if t == nil {
		return a, nil
	}
	a.selectedTask = t
	id := t.TaskID
	if len(id) > 8 {
		id = id[:8]
	}
	a.confirm = NewConfirm(ConfirmStopTask,
		fmt.Sprintf("Stop task '%s'?", id))
	return a, nil
}

// --- Task Definition Diff ---

func (a App) showTaskDefDiff() (App, tea.Cmd) {
	if a.selectedService == nil || len(a.selectedService.Deployments) < 2 {
		a.err = fmt.Errorf("need at least 2 deployments to diff")
		return a, nil
	}

	client := a.client
	deps := a.selectedService.Deployments
	oldTD := deps[1].TaskDefinition
	newTD := deps[0].TaskDefinition

	return a, func() tea.Msg {
		oldDef, err := client.GetTaskDefinition(context.Background(), oldTD)
		if err != nil {
			return errMsg{fmt.Errorf("fetching old task def: %w", err)}
		}
		newDef, err := client.GetTaskDefinition(context.Background(), newTD)
		if err != nil {
			return errMsg{fmt.Errorf("fetching new task def: %w", err)}
		}
		diff := e9saws.DiffTaskDefinitions(oldDef, newDef)
		return taskDefDiffReadyMsg{
			title: fmt.Sprintf("%s → %s", oldTD, newTD),
			diff:  diff,
		}
	}
}

// --- Metrics & Alarms ---

func (a App) showMetrics() (App, tea.Cmd) {
	s := a.serviceView.SelectedService()
	if s == nil {
		return a, nil
	}
	a.selectedService = s
	a.state = viewMetrics
	a.metricsView = views.NewMetrics(s.Name)
	a.metricsView = a.metricsView.SetSize(a.width, a.height-3)
	a.loading = true
	return a, a.loadMetrics()
}

func (a App) loadMetrics() tea.Cmd {
	client := a.client
	cluster := ""
	service := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	if a.selectedService != nil {
		service = a.selectedService.Name
	}
	return func() tea.Msg {
		metrics, err := client.GetServiceMetrics(context.Background(), cluster, service, 15*time.Minute)
		if err != nil {
			return errMsg{err}
		}
		alarms, err := client.ListAlarms(context.Background(), cluster, service)
		if err != nil {
			return metricsLoadedMsg{metrics: metrics}
		}
		return metricsLoadedMsg{metrics: metrics, alarms: alarms}
	}
}

// --- ECS Exec ---

func (a App) execIntoTask() (App, tea.Cmd) {
	t := a.taskView.SelectedTask()
	if t == nil {
		return a, nil
	}
	if t.Status != "RUNNING" {
		a.err = fmt.Errorf("can only exec into RUNNING tasks (task is %s)", t.Status)
		return a, nil
	}

	if a.selectedService != nil && !a.selectedService.EnableExecuteCommand {
		a.err = fmt.Errorf("ECS Exec is not enabled on service %q — set enableExecuteCommand: true on the service", a.selectedService.Name)
		return a, nil
	}
	if !t.ExecAgentRunning {
		a.err = fmt.Errorf("ExecuteCommandAgent is not running on this task — ensure the service has enableExecuteCommand: true and the task role has SSM permissions, then redeploy")
		return a, nil
	}

	a.selectedTask = t

	if len(t.Containers) == 0 {
		a.err = fmt.Errorf("task has no containers")
		return a, nil
	}
	if len(t.Containers) == 1 {
		return a.doExec(t.Containers[0].Name)
	}
	names := make([]string, len(t.Containers))
	for i, c := range t.Containers {
		names[i] = c.Name
	}
	a.picker = NewPicker(PickerExecContainer, "Select container to exec into", names)
	return a, nil
}

func (a App) doExec(containerName string) (App, tea.Cmd) {
	a.execContainerName = containerName
	a.input = NewInput(InputExecCommand, fmt.Sprintf("Command to run in %s", containerName), "/bin/sh")
	return a, nil
}

func (a App) doExecWithCommand(command string) tea.Cmd {
	t := a.selectedTask
	client := a.client
	cluster := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	taskARN := t.TaskARN
	containerName := a.execContainerName

	return func() tea.Msg {
		pluginPath, err := e9saws.SessionManagerPluginPath()
		if err != nil {
			return errMsg{err}
		}

		session, err := client.ExecuteCommand(context.Background(), cluster, taskARN, containerName, command)
		if err != nil {
			return errMsg{err}
		}

		args, err := session.BuildPluginArgs()
		if err != nil {
			return errMsg{err}
		}

		return execSessionReadyMsg{pluginPath: pluginPath, args: args}
	}
}

// --- Environment Variables ---

func (a App) showEnvVars() (App, tea.Cmd) {
	t := a.selectedTask
	if t == nil {
		return a, nil
	}

	if len(t.Containers) == 0 {
		a.err = fmt.Errorf("task has no containers")
		return a, nil
	}
	if len(t.Containers) == 1 {
		return a, a.doShowEnvVars(t.Containers[0].Name)
	}
	names := make([]string, len(t.Containers))
	for i, c := range t.Containers {
		names[i] = c.Name
	}
	a.picker = NewPicker(PickerEnvContainer, "Select container to view env vars", names)
	return a, nil
}

func (a App) doShowEnvVars(containerName string) tea.Cmd {
	t := a.selectedTask
	client := a.client
	return func() tea.Msg {
		td, err := client.GetTaskDefinition(context.Background(), t.TaskDefinition)
		if err != nil {
			return errMsg{err}
		}
		for _, c := range td.Containers {
			if c.Name == containerName {
				resolved := client.ResolveEnvVars(context.Background(), c.EnvVars)
				return envVarsReadyMsg{
					title:   fmt.Sprintf("%s/%s", t.TaskID[:min(8, len(t.TaskID))], containerName),
					envVars: resolved,
				}
			}
		}
		return errMsg{fmt.Errorf("container %q not found in task definition", containerName)}
	}
}

// --- Log Viewing ---

func (a App) openTaskLogs() (App, tea.Cmd) {
	t := a.taskView.SelectedTask()
	if t == nil {
		return a, nil
	}
	a.selectedTask = t
	a.prevState = viewTasks

	if len(t.Containers) == 0 {
		a.err = fmt.Errorf("task has no containers")
		return a, nil
	}
	if len(t.Containers) == 1 {
		return a, a.doLogForContainer(t.Containers[0].Name)
	}
	names := make([]string, len(t.Containers))
	for i, c := range t.Containers {
		names[i] = c.Name
	}
	a.picker = NewPicker(PickerLogContainer, "Select container for logs", names)
	return a, nil
}

func (a App) doLogForContainer(containerName string) tea.Cmd {
	t := a.selectedTask
	client := a.client
	return func() tea.Msg {
		logGroup, streamPrefix, err := client.GetLogConfig(
			context.Background(), t.TaskDefinition, containerName)
		if err != nil {
			return errMsg{err}
		}
		stream := e9saws.BuildLogStreamName(streamPrefix, containerName, t.TaskID)
		return logReadyMsg{
			title:    fmt.Sprintf("%s/%s", t.TaskID[:min(8, len(t.TaskID))], containerName),
			logGroup: logGroup,
			streams:  []string{stream},
		}
	}
}

func (a App) openServiceLogs() (App, tea.Cmd) {
	s := a.serviceView.SelectedService()
	if s == nil {
		return a, nil
	}
	a.selectedService = s
	client := a.client
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	serviceName := s.Name

	return a, func() tea.Msg {
		tasks, err := client.ListTasks(context.Background(), clusterName, serviceName)
		if err != nil {
			return errMsg{err}
		}
		logGroup, streams, err := client.ResolveTaskLogStreams(context.Background(), tasks)
		if err != nil {
			return errMsg{err}
		}
		if logGroup == "" || len(streams) == 0 {
			return errMsg{fmt.Errorf("no log streams found for service '%s'", serviceName)}
		}
		return logReadyMsg{
			title:    serviceName + " (all tasks)",
			logGroup: logGroup,
			streams:  streams,
		}
	}
}

// --- Log Buffer Save ---

func (a App) promptSaveLogBuffer() (App, tea.Cmd) {
	defaultName := filepath.Join(a.cfg.SaveDir(), "e9s-logs.txt")
	a.input = NewInput(InputLogSaveFile, fmt.Sprintf("Save %d lines to file", len(a.logView.ExportLines())), defaultName)
	return a, nil
}

func (a App) copyLogBufferToClipboard() (App, tea.Cmd) {
	lines := a.logView.ExportLines()
	if len(lines) == 0 {
		a.err = fmt.Errorf("no log lines to copy")
		return a, nil
	}
	content := strings.Join(lines, "\n") + "\n"

	if err := clipboard.WriteAll(content); err != nil {
		a.err = fmt.Errorf("clipboard: %w", err)
		return a, nil
	}

	a.flashMessage = fmt.Sprintf("Copied %d lines to clipboard", len(lines))
	a.flashExpiry = time.Now().Add(3 * time.Second)
	return a, nil
}

func (a App) openLogBufferInEditor() (App, tea.Cmd) {
	lines := a.logView.ExportLines()
	if len(lines) == 0 {
		a.err = fmt.Errorf("no log lines to open")
		return a, nil
	}
	content := strings.Join(lines, "\n") + "\n"

	tmpFile, err := os.CreateTemp("", "e9s-logs-*.txt")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	tmpFile.Close()

	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		// Don't remove the temp file — user might want to save-as from the editor
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{"Editor closed"}
	})
}

func (a App) doSaveLogBuffer(filename string) (App, tea.Cmd) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		a.err = fmt.Errorf("no filename specified")
		return a, nil
	}

	// Ensure parent directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		a.err = fmt.Errorf("cannot create directory %s: %w", dir, err)
		return a, nil
	}

	lines := a.logView.ExportLines()
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		a.err = err
		return a, nil
	}

	a.flashMessage = fmt.Sprintf("Saved %d lines to %s", len(lines), filename)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

// --- Data Loading ---

func (a App) refreshCurrentView() tea.Cmd {
	switch a.state {
	case viewClusters:
		return a.loadClusters()
	case viewServices, viewServiceDetail:
		return a.loadServices()
	case viewTasks:
		return a.loadTasks()
	case viewTaskDetail:
		return a.reloadSelectedTask()
	case viewStandaloneTasks:
		return a.loadStandaloneTasks()
	case viewMetrics:
		return a.loadMetrics()
	case viewAlarms:
		return a.refreshAlarms()
	case viewAlarmDetail:
		return a.refreshAlarmDetail()
	default:
		return nil
	}
}

func (a App) tick() tea.Cmd {
	return tea.Tick(time.Duration(a.refreshSec)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a App) loadClusters() tea.Cmd {
	return func() tea.Msg {
		clusters, err := a.client.ListClusters(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return clustersLoadedMsg{clusters}
	}
}

func (a App) loadServices() tea.Cmd {
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	return func() tea.Msg {
		services, err := a.client.ListServices(context.Background(), clusterName)
		if err != nil {
			return errMsg{err}
		}
		return servicesLoadedMsg{services}
	}
}

func (a App) loadTasks() tea.Cmd {
	clusterName := ""
	serviceName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	if a.selectedService != nil {
		serviceName = a.selectedService.Name
	}
	return func() tea.Msg {
		tasks, err := a.client.ListTasks(context.Background(), clusterName, serviceName)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{tasks}
	}
}

func (a App) reloadSelectedTask() tea.Cmd {
	if a.selectedTask == nil || a.selectedCluster == nil {
		return nil
	}
	client := a.client
	cluster := a.selectedCluster.Name
	taskARN := a.selectedTask.TaskARN
	return func() tea.Msg {
		tasks, err := client.DescribeTask(context.Background(), cluster, taskARN)
		if err != nil {
			return errMsg{err}
		}
		if tasks != nil {
			return taskDetailRefreshedMsg{task: tasks}
		}
		return taskDetailRefreshedMsg{task: nil}
	}
}
