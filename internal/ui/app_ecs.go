package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

const taskHistoryBatchSize = 50

func (a App) selectedClusterName() string {
	if a.selectedCluster == nil {
		return ""
	}
	return a.selectedCluster.Name
}

func (a App) selectedServiceName() string {
	if a.selectedService == nil {
		return ""
	}
	return a.selectedService.Name
}

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
		err := a.ecs.ForceDeployment(a.ctx, cluster, service)
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
		err := a.ecs.ScaleService(a.ctx, cluster, service, count)
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
	if t.Status == "STOPPED" {
		a.err = fmt.Errorf("task %q is already stopped", t.TaskID)
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
		err := a.ecs.StopTask(a.ctx, cluster, taskARN, "Stopped by e9s")
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{"Task stopped"}
	}
}

func (a App) showServiceDetail() (App, tea.Cmd) {
	var service *model.Service
	if a.state == viewServices {
		service = a.serviceView.SelectedService()
	} else {
		service = a.selectedService
	}
	if service == nil {
		return a, nil
	}
	a.serviceDetailReturnState = a.state
	a.selectedService = service
	a.state = viewServiceDetail
	a.serviceDetailView = views.NewServiceDetail(service)
	a.loading = true
	return a, a.loadServices()
}

func (a App) openTaskDefinitions() (App, tea.Cmd) {
	a.taskDefsReturnState = a.state
	a.state = viewTaskDefs
	a.selectedTaskDef = ""
	a.taskDefsView = views.NewTaskDefs()
	a.taskDefsView = a.taskDefsView.SetSize(a.width, a.height-3)
	a.loading = true
	return a, a.loadTaskDefinitions()
}

func (a App) loadTaskDefinitions() tea.Cmd {
	return func() tea.Msg {
		defs, err := a.ecs.ListTaskDefinitions(a.ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return taskDefsLoadedMsg{defs}
	}
}

func (a App) openSelectedTaskDefinition() (App, tea.Cmd) {
	td := a.taskDefsView.SelectedTaskDef()
	if td == nil {
		return a, nil
	}
	a.loading = true
	a.selectedTaskDef = fmt.Sprintf("%s:%d", td.Family, td.Revision)
	arn := td.ARN
	return a, a.loadTaskDefinitionDetail(arn)
}

func (a App) loadTaskDefinitionDetail(taskDef string) tea.Cmd {
	return func() tea.Msg {
		def, err := a.ecs.GetTaskDefinition(a.ctx, taskDef)
		if err != nil {
			return errMsg{err}
		}
		return taskDefLoadedMsg{taskDefinition: taskDef, def: def}
	}
}

func (a App) refreshSelectedTaskDefinition() tea.Cmd {
	td := a.taskDefDetailView.TaskDef()
	if td == nil || td.ARN == "" {
		return a.loadTaskDefinitions()
	}
	return a.loadTaskDefinitionDetail(td.ARN)
}

// --- Standalone Tasks ---

func (a App) showStandaloneTasks() (App, tea.Cmd) {
	if a.state == viewStandaloneTasks {
		return a.returnFromStandaloneTasks()
	}
	if a.state == viewTaskDetail && a.taskDetailReturnState == viewStandaloneTasks {
		a.state = viewStandaloneTasks
		return a.returnFromStandaloneTasks()
	}
	a.standaloneReturnState = a.state
	a.standaloneReturnService = a.selectedService
	a.standaloneReturnTask = a.selectedTask
	a.state = viewStandaloneTasks
	a.selectedService = nil
	a.selectedTask = nil
	a.taskScopeStopped = false
	a.taskNextToken = ""
	a.standaloneView = views.NewStandaloneTasks().SetScope(false)
	a.loading = true
	return a, a.loadStandaloneTasks()
}

func (a App) returnFromStandaloneTasks() (App, tea.Cmd) {
	returnState := a.standaloneReturnState
	if returnState != viewServices && returnState != viewTasks && returnState != viewTaskDetail && returnState != viewServiceDetail {
		returnState = viewServices
	}
	a.state = returnState
	a.selectedService = a.standaloneReturnService
	a.selectedTask = a.standaloneReturnTask
	a.standaloneReturnService = nil
	a.standaloneReturnTask = nil
	a.loading = false
	return a, nil
}

func (a App) loadStandaloneTasks() tea.Cmd {
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	stopped := a.taskScopeStopped
	return func() tea.Msg {
		if stopped {
			page, err := a.ecs.ListStoppedStandaloneTasks(a.ctx, clusterName, "", taskHistoryBatchSize)
			if err != nil {
				return errMsg{err}
			}
			sortStoppedTasks(page.Tasks)
			return standaloneTasksLoadedMsg{cluster: clusterName, tasks: page.Tasks, stopped: true, nextToken: page.NextToken}
		}
		tasks, err := a.ecs.ListStandaloneTasks(a.ctx, clusterName)
		if err != nil {
			return errMsg{err}
		}
		return standaloneTasksLoadedMsg{cluster: clusterName, tasks: tasks}
	}
}

func (a App) toggleTaskScope() (App, tea.Cmd) {
	if a.state != viewTasks && a.state != viewStandaloneTasks {
		return a, nil
	}
	a.taskScopeStopped = !a.taskScopeStopped
	a.taskNextToken = ""
	a.selectedTask = nil
	a.loading = true
	if a.state == viewStandaloneTasks {
		a.standaloneView = a.standaloneView.SetScope(a.taskScopeStopped)
		return a, a.loadStandaloneTasks()
	}
	a.taskView = a.taskView.SetScope(a.taskScopeStopped)
	return a, a.loadTasks()
}

func (a App) loadMoreStoppedTasks() (App, tea.Cmd) {
	if !a.taskScopeStopped || a.taskNextToken == "" || (a.state != viewTasks && a.state != viewStandaloneTasks) {
		return a, nil
	}
	cluster := ""
	serviceName := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	if a.selectedService != nil {
		serviceName = a.selectedService.Name
	}
	nextToken := a.taskNextToken
	standalone := a.state == viewStandaloneTasks
	a.loading = true
	return a, func() tea.Msg {
		if standalone {
			page, err := a.ecs.ListStoppedStandaloneTasks(a.ctx, cluster, nextToken, taskHistoryBatchSize)
			if err != nil {
				return errMsg{err}
			}
			sortStoppedTasks(page.Tasks)
			return standaloneTasksLoadedMsg{cluster: cluster, tasks: page.Tasks, stopped: true, append: true, nextToken: page.NextToken}
		}
		page, err := a.ecs.ListStoppedServiceTasks(a.ctx, cluster, serviceName, nextToken, taskHistoryBatchSize)
		if err != nil {
			return errMsg{err}
		}
		sortStoppedTasks(page.Tasks)
		return tasksLoadedMsg{cluster: cluster, service: serviceName, tasks: page.Tasks, stopped: true, append: true, nextToken: page.NextToken}
	}
}

func sortStoppedTasks(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].StoppedAt.After(tasks[j].StoppedAt)
	})
}

func (a App) openStandaloneTaskLogs() (App, tea.Cmd) {
	return a.openTaskLogs()
}

func (a App) promptStopStandaloneTask() (App, tea.Cmd) {
	t := a.standaloneView.SelectedTask()
	if t == nil {
		return a, nil
	}
	if t.Status == "STOPPED" {
		a.err = fmt.Errorf("task %q is already stopped", t.TaskID)
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

func (a App) promptRunTask() (App, tea.Cmd) {
	if a.state != viewStandaloneTasks || a.selectedCluster == nil {
		return a, nil
	}
	seedTaskDefinition := ""
	if task := a.standaloneView.SelectedTask(); task != nil {
		seedTaskDefinition = task.TaskDefinition
	}
	a.runTaskForm = NewRunTaskForm(a.selectedCluster.Name, seedTaskDefinition)
	return a, nil
}

func (a App) runStandaloneTask(request model.RunTaskRequest) tea.Cmd {
	return func() tea.Msg {
		tasks, err := a.ecs.RunTask(a.ctx, request)
		if err != nil {
			return errMsg{err}
		}
		return runTaskStartedMsg{count: len(tasks), taskDefinition: request.TaskDefinition, cluster: request.Cluster}
	}
}

// --- Task Definition Diff ---

func (a App) showTaskDefDiff() (App, tea.Cmd) {
	if a.selectedService == nil || len(a.selectedService.Deployments) < 2 {
		a.err = fmt.Errorf("need at least 2 deployments to diff")
		return a, nil
	}

	deps := a.selectedService.Deployments
	oldTD := deps[1].TaskDefinition
	newTD := deps[0].TaskDefinition
	a.diffReturnState = viewServiceDetail

	return a, func() tea.Msg {
		diff, err := a.ecs.TaskDefinitionDiff(a.ctx, oldTD, newTD)
		if err != nil {
			return errMsg{err}
		}
		return taskDefDiffReadyMsg{
			title:       fmt.Sprintf("%s → %s", oldTD, newTD),
			diff:        diff,
			returnState: viewServiceDetail,
		}
	}
}

func (a App) showSelectedTaskDefDiff() (App, tea.Cmd) {
	definition := a.taskDefDetailView.TaskDef()
	if definition == nil {
		return a, nil
	}
	previous := a.taskDefsView.PreviousRevision(definition.Family, definition.Revision)
	if previous == nil {
		a.err = fmt.Errorf("no earlier active revision is available for %s:%d", definition.Family, definition.Revision)
		return a, nil
	}
	a.diffReturnState = viewTaskDefDetail
	oldRef, newRef := previous.ARN, definition.ARN
	return a, func() tea.Msg {
		diff, err := a.ecs.TaskDefinitionDiff(a.ctx, oldRef, newRef)
		if err != nil {
			return errMsg{err}
		}
		return taskDefDiffReadyMsg{
			title:       fmt.Sprintf("%s:%d → %s:%d", previous.Family, previous.Revision, definition.Family, definition.Revision),
			diff:        diff,
			returnState: viewTaskDefDetail,
		}
	}
}

func (a App) editSelectedTaskDefinition() (App, tea.Cmd) {
	definition := a.taskDefDetailView.TaskDef()
	if definition == nil {
		return a, nil
	}
	document, err := a.ecs.TaskDefinitionEditorDocument(definition.RawJSON)
	if err != nil {
		a.err = err
		return a, nil
	}
	file, err := os.CreateTemp("", "e9s-task-definition-*.json")
	if err != nil {
		a.err = err
		return a, nil
	}
	path := file.Name()
	if _, err = file.WriteString(document); err != nil {
		file.Close()
		os.Remove(path)
		a.err = err
		return a, nil
	}
	if err = file.Close(); err != nil {
		os.Remove(path)
		a.err = err
		return a, nil
	}

	editor := NewEditorCmd(path)
	ecs := a.ecs
	return a, tea.Exec(editor, func(editorErr error) tea.Msg {
		defer os.Remove(path)
		if editorErr != nil {
			return errMsg{editorErr}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return errMsg{err}
		}
		validated, err := ecs.TaskDefinitionEditorDocument(string(raw))
		if err != nil {
			return errMsg{err}
		}
		return taskDefinitionEditedMsg{document: validated}
	})
}

func (a App) registerEditedTaskDefinition() tea.Cmd {
	document := a.taskDefinitionDocument
	baseDefinition := a.selectedTaskDef
	return func() tea.Msg {
		definition, err := a.ecs.RegisterTaskDefinitionJSON(a.ctx, document)
		if err != nil {
			return errMsg{err}
		}
		return taskDefinitionRegisteredMsg{baseDefinition: baseDefinition, definition: definition}
	}
}

// --- Metrics & Alarms ---

func (a App) toggleScaleIn() (App, tea.Cmd) {
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	serviceName := a.metricsServiceName
	if a.state != viewMetrics {
		s := a.serviceView.SelectedService()
		if s == nil {
			return a, nil
		}
		serviceName = s.Name
	}
	if serviceName == "" || (a.state == viewMetrics && a.metricsTaskScope) {
		return a, nil
	}

	// Check current state, then confirm toggle
	return a, func() tea.Msg {
		suspended, err := a.ecs.ScaleInSuspended(a.ctx, clusterName, serviceName)
		if err != nil {
			return errMsg{fmt.Errorf("scale-in status: %w (auto-scaling may not be configured)", err)}
		}
		return scaleInStatusMsg{service: serviceName, cluster: clusterName, suspended: suspended}
	}
}

func (a App) doToggleScaleIn() tea.Cmd {
	cluster := a.scaleInCluster
	service := a.scaleInService
	newState := !a.scaleInCurrentState
	return func() tea.Msg {
		err := a.ecs.SetScaleInSuspended(a.ctx, cluster, service, newState)
		if err != nil {
			return errMsg{err}
		}
		action := "enabled"
		if newState {
			action = "suspended"
		}
		return actionSuccessMsg{fmt.Sprintf("Scale-in %s for %s", action, service)}
	}
}

func (a App) showMetrics() (App, tea.Cmd) {
	origin := a.state
	var task *model.Task
	serviceName := ""
	switch a.state {
	case viewServices:
		s := a.serviceView.SelectedService()
		if s == nil {
			return a, nil
		}
		a.selectedService = s
		a.selectedTask = nil
		serviceName = s.Name
	case viewTasks, viewStandaloneTasks, viewTaskDetail:
		task = a.taskForCurrentView()
		if task == nil {
			return a, nil
		}
		a.selectedTask = task
		if strings.HasPrefix(task.Group, "service:") && a.selectedService != nil {
			serviceName = a.selectedService.Name
		}
	default:
		return a, nil
	}

	scopeName := serviceName
	if task != nil {
		scopeName = task.TaskID
	}
	a.metricsReturnState = origin
	a.metricsTaskScope = task != nil
	a.metricsServiceName = serviceName
	a.state = viewMetrics
	a.metricsView = views.NewMetrics(scopeName, task != nil)
	a.metricsView = a.metricsView.SetSize(a.width, a.height-3)
	a.loading = true
	return a, a.loadMetrics()
}

func (a App) loadMetrics() tea.Cmd {
	cluster := ""
	service := a.metricsServiceName
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	taskScope := a.metricsTaskScope
	selectedTask := a.selectedTask
	return func() tea.Msg {
		var (
			metrics *model.ServiceMetrics
			err     error
		)
		if taskScope {
			if selectedTask == nil {
				return errMsg{fmt.Errorf("no task selected for metrics")}
			}
			metrics, err = a.ecs.GetTaskMetrics(a.ctx, cluster, service, *selectedTask, 15*time.Minute)
		} else {
			metrics, err = a.ecs.GetServiceMetrics(a.ctx, cluster, service, 15*time.Minute)
		}
		if err != nil {
			return errMsg{err}
		}
		taskARN := ""
		if selectedTask != nil {
			taskARN = selectedTask.TaskARN
		}
		if taskScope {
			return metricsLoadedMsg{cluster: cluster, service: service, taskARN: taskARN, metrics: metrics}
		}
		var warnings []string
		alarms, err := a.ecs.ListServiceAlarms(a.ctx, cluster, service)
		if err != nil {
			warnings = append(warnings, "Alarms unavailable: "+err.Error())
		}
		suspended, scaleErr := a.ecs.ScaleInSuspended(a.ctx, cluster, service)
		if scaleErr != nil {
			warnings = append(warnings, "Scale-in status unavailable: "+scaleErr.Error())
		}
		return metricsLoadedMsg{
			cluster: cluster, service: service,
			metrics: metrics, alarms: alarms,
			scaleKnown: scaleErr == nil, scaleSuspended: suspended,
			warnings: warnings,
		}
	}
}

func (a App) taskForCurrentView() *model.Task {
	switch a.state {
	case viewTasks:
		return a.taskView.SelectedTask()
	case viewStandaloneTasks:
		return a.standaloneView.SelectedTask()
	case viewTaskDetail:
		return a.selectedTask
	default:
		return nil
	}
}

// --- ECS Exec ---

func (a App) execIntoTask() (App, tea.Cmd) {
	t := a.taskForCurrentView()
	if t == nil {
		return a, nil
	}
	if t.Status != "RUNNING" {
		a.err = fmt.Errorf("can only exec into RUNNING tasks (task is %s)", t.Status)
		return a, nil
	}

	if strings.HasPrefix(t.Group, "service:") && a.selectedService != nil && !a.selectedService.EnableExecuteCommand {
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
	cluster := ""
	if a.selectedCluster != nil {
		cluster = a.selectedCluster.Name
	}
	containerName := a.execContainerName

	return func() tea.Msg {
		launch, err := a.ecs.PrepareExecSession(a.ctx, cluster, *t, containerName, command)
		if err != nil {
			return errMsg{err}
		}
		return execSessionReadyMsg{pluginPath: launch.Executable, args: launch.Args}
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
	returnState := a.state
	return func() tea.Msg {
		environment, err := a.ecs.TaskDefinitionEnvironment(a.ctx, t.TaskDefinition, containerName, false)
		if err != nil {
			return errMsg{err}
		}
		return envVarsReadyMsg{
			title:          fmt.Sprintf("%s/%s", t.TaskID[:min(8, len(t.TaskID))], containerName),
			envVars:        environment,
			taskDefinition: t.TaskDefinition,
			container:      containerName,
			returnState:    returnState,
		}
	}
}

func (a App) showTaskDefEnvVars() (App, tea.Cmd) {
	td := a.taskDefDetailView.TaskDef()
	if td == nil {
		return a, nil
	}

	a.prevState = viewTaskDefDetail
	if len(td.Containers) == 0 {
		a.err = fmt.Errorf("task definition has no containers")
		return a, nil
	}
	if len(td.Containers) == 1 {
		return a, a.doShowTaskDefEnvVars(td.Containers[0].Name)
	}
	names := make([]string, len(td.Containers))
	for i, c := range td.Containers {
		names[i] = c.Name
	}
	a.picker = NewPicker(PickerEnvContainer, "Select container to view env vars", names)
	return a, nil
}

func (a App) doShowTaskDefEnvVars(containerName string) tea.Cmd {
	td := a.taskDefDetailView.TaskDef()
	returnState := a.state
	return func() tea.Msg {
		if td == nil {
			return errMsg{fmt.Errorf("no task definition selected")}
		}
		environment, err := a.ecs.TaskDefinitionEnvironment(a.ctx, td.ARN, containerName, false)
		if err != nil {
			return errMsg{err}
		}
		return envVarsReadyMsg{
			title:          fmt.Sprintf("%s:%d/%s", td.Family, td.Revision, containerName),
			envVars:        environment,
			taskDefinition: td.ARN,
			container:      containerName,
			returnState:    returnState,
		}
	}
}

func (a App) confirmRevealEnvSecrets() (App, tea.Cmd) {
	if a.state != viewEnvVars || a.envTaskDefinition == "" || a.envContainer == "" ||
		a.envSecretsResolved || !a.envVarsView.HasSecrets() {
		return a, nil
	}
	a.confirm = NewConfirm(ConfirmRevealSecrets,
		fmt.Sprintf("Resolve and display secret values for container %q?", a.envContainer))
	return a, nil
}

func (a App) loadResolvedEnvSecrets() tea.Cmd {
	taskDefinition := a.envTaskDefinition
	container := a.envContainer
	title := a.envTitle
	returnState := a.prevState
	return func() tea.Msg {
		environment, err := a.ecs.TaskDefinitionEnvironment(a.ctx, taskDefinition, container, true)
		if err != nil {
			return errMsg{err}
		}
		return envVarsReadyMsg{
			title: title, envVars: environment,
			taskDefinition: taskDefinition, container: container,
			resolved: true, returnState: returnState,
		}
	}
}

// --- Log Viewing ---

func (a App) openTaskLogs() (App, tea.Cmd) {
	t := a.taskForCurrentView()
	if t == nil {
		return a, nil
	}
	a.selectedTask = t
	a.prevState = a.state

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
	returnState := a.prevState
	cluster := a.selectedClusterName()
	service := a.selectedServiceName()
	return func() tea.Msg {
		source, err := a.ecs.ContainerLogSource(a.ctx, *t, containerName)
		if err != nil {
			return errMsg{err}
		}
		return logReadyMsg{
			title:       fmt.Sprintf("%s/%s", t.TaskID[:min(8, len(t.TaskID))], containerName),
			logGroup:    source.Group,
			streams:     source.Streams,
			ecsGuard:    true,
			returnState: returnState,
			cluster:     cluster,
			service:     service,
			taskARN:     t.TaskARN,
		}
	}
}

func (a App) openServiceLogs() (App, tea.Cmd) {
	s := a.serviceView.SelectedService()
	if s == nil {
		return a, nil
	}
	a.selectedService = s
	a.prevState = a.state
	clusterName := ""
	if a.selectedCluster != nil {
		clusterName = a.selectedCluster.Name
	}
	serviceName := s.Name

	return a, func() tea.Msg {
		source, err := a.ecs.ServiceLogSource(a.ctx, clusterName, serviceName)
		if err != nil {
			return errMsg{err}
		}
		return logReadyMsg{
			title:       serviceName + " (all tasks)",
			logGroup:    source.Group,
			streams:     source.Streams,
			ecsGuard:    true,
			returnState: viewServices,
			cluster:     clusterName,
			service:     serviceName,
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
	case viewTaskDefs:
		return a.loadTaskDefinitions()
	case viewTaskDefDetail:
		return a.refreshSelectedTaskDefinition()
	case viewStandaloneTasks:
		return a.loadStandaloneTasks()
	case viewMetrics:
		return a.loadMetrics()
	case viewAlarms:
		return a.refreshAlarms()
	case viewAlarmDetail:
		return a.refreshAlarmDetail()
	case viewCBProjects:
		return a.refreshCBProjects()
	case viewCBBuilds:
		return a.refreshCBBuilds()
	case viewCBBuildDetail:
		return a.refreshCBBuildDetail()
	case viewTofuResources:
		return a.refreshTofuResources()
	case viewECRRepos:
		return a.refreshECRRepos()
	case viewECRImages:
		return a.refreshECRImages()
	case viewR53Zones:
		return a.refreshR53Zones()
	case viewR53Records:
		return a.refreshR53Records()
	case viewEC2Instances:
		return a.refreshEC2Instances()
	case viewEC2Detail:
		return a.refreshEC2Detail()
	case viewEC2SecurityGroups:
		return a.refreshEC2SecurityGroups()
	case viewEC2SecurityGroupDetail:
		return a.refreshEC2SecurityGroupDetail()
	case viewEC2VPCs:
		_, cmd := a.openEC2VPCs()
		return cmd
	case viewEC2VPCDetail:
		if a.ec2VPCDetail != nil {
			_, cmd := a.loadEC2VPCDetail(a.ec2VPCDetail.VpcID)
			return cmd
		}
	case viewEC2Subnets:
		_, cmd := a.openEC2Subnets(a.ec2SubnetVPCFilter)
		return cmd
	case viewEC2SubnetDetail:
		if a.ec2SubnetDetail != nil {
			_, cmd := a.loadEC2SubnetDetail(a.ec2SubnetDetail.SubnetID)
			return cmd
		}
	case viewEC2Volumes:
		_, cmd := a.openEC2Volumes()
		return cmd
	case viewEC2VolumeDetail:
		if a.ec2VolumeDetail != nil {
			_, cmd := a.loadEC2VolumeDetail(a.ec2VolumeDetail.VolumeID)
			return cmd
		}
	case viewEC2LoadBalancers:
		return a.refreshEC2LoadBalancers()
	case viewEC2LoadBalancerDetail:
		return a.refreshEC2LoadBalancerDetail()
	case viewEC2TargetGroups:
		return a.refreshEC2TargetGroups()
	case viewEC2TargetGroupDetail:
		return a.refreshEC2TargetGroupDetail()
	case viewRDSInstances:
		return a.refreshRDSInstances()
	case viewRDSClusters:
		return a.refreshRDSClusters()
	case viewRDSDetail:
		return a.refreshRDSDetail()
	default:
		return nil
	}
	return nil
}

func (a App) tick() tea.Cmd {
	return tea.Tick(time.Duration(a.refreshSec)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a App) loadClusters() tea.Cmd {
	return func() tea.Msg {
		clusters, err := a.ecs.ListClusters(a.ctx)
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
		services, err := a.ecs.ListServices(a.ctx, clusterName)
		if err != nil {
			return errMsg{err}
		}
		return servicesLoadedMsg{cluster: clusterName, services: services}
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
	stopped := a.taskScopeStopped
	return func() tea.Msg {
		if stopped {
			page, err := a.ecs.ListStoppedServiceTasks(a.ctx, clusterName, serviceName, "", taskHistoryBatchSize)
			if err != nil {
				return errMsg{err}
			}
			sortStoppedTasks(page.Tasks)
			return tasksLoadedMsg{cluster: clusterName, service: serviceName, tasks: page.Tasks, stopped: true, nextToken: page.NextToken}
		}
		tasks, err := a.ecs.ListTasks(a.ctx, clusterName, serviceName)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{cluster: clusterName, service: serviceName, tasks: tasks}
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
			return taskDetailRefreshedMsg{taskARN: taskARN, task: tasks}
		}
		return taskDetailRefreshedMsg{taskARN: taskARN, task: nil}
	}
}
