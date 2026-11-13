package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
)

// paletteItem is a TUI command: shared command metadata plus how this
// frontend runs it. Items with keys run by replaying keys[0] through Update,
// so the palette and the keyboard share one dispatch path (including every
// confirmation dialog). Items with run set are invoked directly instead.
// Legend items document movement keys and are never runnable.
type paletteItem struct {
	command.Entry
	keys   []string
	legend bool
	run    func(App) (App, tea.Cmd)
}

func act(id, key, title string) paletteItem {
	return paletteItem{Entry: command.Entry{Command: command.Command{ID: id, Title: title}}, keys: []string{key}}
}

func legendItem(key, title string) paletteItem {
	return paletteItem{Entry: command.Entry{Command: command.Command{Title: title}}, keys: []string{key}, legend: true}
}

func (p paletteItem) danger() paletteItem { p.Danger = true; return p }

func (p paletteItem) kw(words ...string) paletteItem {
	p.Keywords = append(append([]string(nil), p.Keywords...), words...)
	return p
}

// alt adds keys that trigger the same command; they appear in the display key
// ("enter/e") but only the first key is replayed.
func (p paletteItem) alt(keys ...string) paletteItem {
	p.keys = append(append([]string(nil), p.keys...), keys...)
	return p
}

func (p paletteItem) displayKey() string { return strings.Join(p.keys, "/") }

// runKey is the key replayed to run the item, or "" when it has none.
func (p paletteItem) runKey() string {
	if p.legend || len(p.keys) == 0 {
		return ""
	}
	return p.keys[0]
}

var (
	scrollLegend = legendItem("j/k", "Scroll")
	topLegend    = legendItem("g/G", "Top/bottom")
)

// globalItems are available in every view.
func (a App) globalItems() []paletteItem {
	kb := a.kb
	return []paletteItem{
		act("app.error-details", kb.ErrorDetails, "Show the full error (when present)").kw("error", "details"),
		act("app.switch-mode", kb.SwitchMode, "Switch mode").kw("module"),
		act("app.reopen-picker", kb.ReopenPicker, "Reopen mode picker").kw("entry", "prefix"),
		act("app.pause", kb.PauseResume, "Pause/resume polling").kw("refresh", "stop polling"),
		act("app.edit-config", kb.EditConfig, "Edit config").kw("settings", "yaml", "editor"),
		act("app.switch-region", kb.SwitchRegion, "Switch AWS region").kw("region"),
		act("app.refresh", "R", "Refresh data").kw("reload"),
		act("app.filter", "/", "Filter/search").kw("find"),
		act("app.back", "esc", "Go back"),
		act("app.quit", "q", "Quit").kw("exit"),
		act("app.help", "?", "Toggle this help").kw("keys", "shortcuts"),
	}
}

// viewItems returns the commands for state, in help-display order.
// Conditionals mirror what the view's key handler accepts.
func (a App) viewItems(state viewState) []paletteItem {
	kb := a.kb
	switch state {
	case viewClusters:
		return []paletteItem{
			act("ecs.open-cluster", "enter", "Browse services"),
			legendItem("1-9", "Quick select"),
			act("app.filter", "/", "Filter"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
			act("app.refresh", "R", "Refresh"),
		}
	case viewServices:
		return []paletteItem{
			act("ecs.open-service", "enter", "Browse tasks"),
			act("ecs.service-detail", kb.ServiceDetail, "Service detail (deployments + events)").kw("deployments", "events"),
			act("ecs.service-logs", kb.ServiceLogs, "Tail logs (all tasks)").kw("logs", "tail"),
			act("ecs.force-redeploy", kb.ForceRedeploy, "Force new deployment").danger().kw("redeploy", "restart", "rollout"),
			act("ecs.scale", kb.Scale, "Scale service").kw("desired count", "replicas"),
			act("ecs.metrics", kb.Metrics, "CPU/memory metrics + alarms").kw("cloudwatch", "utilization"),
			act("ecs.toggle-scale-in", kb.ToggleScaleIn, "Toggle scale-in suspension").kw("autoscaling"),
			act("ecs.standalone-tasks", kb.StandaloneTasks, "Standalone tasks (non-service)"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
		}
	case viewTasks:
		items := []paletteItem{
			act("ecs.task-detail", "enter", "Task detail"),
			act("ecs.service-detail", kb.ServiceDetail, "Service detail").kw("deployments", "events"),
			act("ecs.standalone-tasks", kb.StandaloneTasks, "Toggle standalone tasks"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
			act("ecs.task-logs", kb.TaskLogs, "Tail logs").kw("logs"),
			act("ecs.task-scope", kb.TaskScope, "Switch Active/Recently stopped").kw("stopped tasks"),
		}
		return append(items, a.taskScopeItems()...)
	case viewTaskDetail:
		return []paletteItem{
			act("ecs.service-detail", kb.ServiceDetail, "Service detail").kw("deployments", "events"),
			act("ecs.standalone-tasks", kb.StandaloneTasks, "Toggle standalone tasks"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
			act("ecs.env-vars", kb.EnvVars, "View environment variables").kw("env", "environment", "secrets"),
			act("ecs.task-logs", kb.TaskLogs, "Tail logs").kw("logs"),
			act("ecs.metrics", kb.Metrics, "Task CPU/memory metrics").kw("cloudwatch", "utilization"),
			act("app.open-resource", kb.OpenResource, "Open linked resource").kw("related", "jump"),
			scrollLegend,
			topLegend,
		}
	case viewTaskDefs:
		return []paletteItem{
			act("ecs.taskdef-detail", "enter", "Task definition detail"),
			act("app.filter", "/", "Filter"),
		}
	case viewTaskDefDetail:
		return []paletteItem{
			act("app.switch-tab", "tab", "Switch Summary/JSON"),
			act("ecs.env-vars", kb.EnvVars, "View environment variables").kw("env", "environment", "secrets"),
			act("ecs.taskdef-diff", kb.TaskDefDiff, "Diff against previous active revision").kw("compare", "revision"),
			act("ecs.taskdef-edit", kb.TaskDefEdit, "Edit in $EDITOR and register revision").kw("register", "revision"),
			scrollLegend,
			topLegend,
		}
	case viewServiceDetail:
		return []paletteItem{
			act("app.switch-tab", "tab", "Switch between Deployments and Events"),
			act("app.open-resource", kb.OpenResource, "Open linked resource").kw("related", "jump"),
			act("ecs.standalone-tasks", kb.StandaloneTasks, "Toggle standalone tasks"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
			act("ecs.taskdef-diff", kb.TaskDefDiff, "Task definition deployment diff").kw("compare", "revision"),
			scrollLegend,
		}
	case viewLogs:
		return []paletteItem{
			act("logs.follow", kb.LogFollow, "Toggle follow mode").kw("tail", "pause", "resume"),
			act("logs.timestamps", kb.LogTimestamp, "Cycle timestamps (relative/local/UTC)").kw("time", "utc"),
			act("app.filter", "/", "Search — jump with n/N").kw("find"),
			act("logs.next-match", "n", "Next match"),
			act("logs.previous-match", "N", "Previous match"),
			act("logs.older", kb.LogOlder, "Load older logs").kw("history", "backfill"),
			act("logs.newer", kb.LogNewer, "Load newer logs"),
			act("logs.save", kb.LogSave, "Save buffer to file").kw("export", "write"),
			act("logs.copy", kb.LogCopy, "Copy buffer to clipboard").kw("yank"),
			act("logs.open-editor", kb.LogOpenEditor, "Open buffer in $EDITOR"),
			act("logs.highlights", kb.LogHighlights, "Manage highlight rules").kw("color", "rules"),
			act("logs.streams", kb.LogStreams, "Show/hide buffered streams"),
			legendItem("g/G", "Jump to top/bottom"),
			legendItem("PgUp/PgDn", "Scroll by page"),
		}
	case viewStandaloneTasks:
		items := []paletteItem{
			act("ecs.task-detail", "enter", "Task detail"),
			act("ecs.standalone-tasks", kb.StandaloneTasks, "Return to services/tasks"),
			act("ecs.task-definitions", kb.TaskDefinitions, "Task definitions explorer").kw("taskdef"),
			act("ecs.task-logs", kb.TaskLogs, "Tail logs").kw("logs"),
			act("ecs.task-scope", kb.TaskScope, "Switch Active/Recently stopped").kw("stopped tasks"),
			act("ecs.run-task", kb.RunTask, "Run standalone task").kw("launch", "start"),
		}
		return append(items, a.taskScopeItems()...)
	case viewTaskDefDiff:
		return []paletteItem{scrollLegend, topLegend}
	case viewMetrics:
		items := []paletteItem{act("app.refresh", "R", "Refresh metrics")}
		if !a.metricsTaskScope {
			items = append(items, act("ecs.toggle-scale-in", kb.ToggleScaleIn, "Toggle scale-in suspension").kw("autoscaling"))
		}
		return items
	case viewSSM:
		return []paletteItem{
			act("ssm.view", "enter", "View parameter value"),
			act("ssm.edit", kb.EditValue, "Edit parameter value").kw("update", "put"),
			act("app.save", kb.Save, "Save prefix for quick access").kw("bookmark"),
		}
	case viewSecrets:
		return []paletteItem{
			act("secrets.view", "enter", "View secret value"),
			act("secrets.edit", kb.EditValue, "Edit secret value").kw("update", "put"),
			act("secrets.clone", kb.CloneSecret, "Clone secret to new name").kw("copy", "duplicate"),
			act("secrets.copy-arn", kb.CopyARN, "Copy ARN to clipboard").kw("yank"),
			act("app.save", kb.Save, "Save filter for quick access").kw("bookmark"),
		}
	case viewSecretValue:
		return []paletteItem{scrollLegend, topLegend}
	case viewS3Buckets:
		return []paletteItem{
			act("s3.open-bucket", "enter", "Browse bucket"),
			act("app.save", kb.Save, "Save search").kw("bookmark"),
		}
	case viewS3Objects:
		return []paletteItem{
			act("s3.open-object", "enter", "Open folder / view detail"),
			act("s3.object-detail", "i", "View detail + tags").kw("metadata", "info"),
			act("s3.search-prefix", kb.Search, "Search by key prefix").kw("find"),
			act("s3.download", kb.Download, "Download object or folder").kw("save", "get"),
		}
	case viewS3Detail:
		return []paletteItem{act("s3.download", kb.Download, "Download").kw("save", "get")}
	case viewLambdaList:
		return []paletteItem{
			act("lambda.detail", "enter", "Function detail"),
			act("lambda.tail-logs", kb.TailStream, "Tail function logs (live)").kw("logs"),
			act("lambda.browse-streams", kb.BrowseStreams, "Browse log streams (historical)").kw("logs"),
			act("lambda.search-logs", kb.Search, "Search function logs").kw("find"),
			act("app.save", kb.Save, "Save search").kw("bookmark"),
		}
	case viewLambdaDetail:
		return []paletteItem{
			act("lambda.edit-code", kb.EditCode, "Edit code (download, $EDITOR, deploy)").kw("deploy", "upload"),
			act("lambda.env-vars", kb.EnvVars, "View environment variables").kw("env", "environment", "secrets"),
			act("lambda.tail-logs", kb.TailStream, "Tail logs (live)").kw("logs"),
			act("lambda.browse-streams", kb.BrowseStreams, "Browse log streams (historical)").kw("logs"),
			act("lambda.search-logs", kb.Search, "Search logs").kw("find"),
		}
	case viewDynamoTables:
		return []paletteItem{
			act("dynamo.open-table", "enter", "Browse table"),
			act("app.save", kb.Save, "Save table").kw("bookmark"),
		}
	case viewDynamoItems:
		return []paletteItem{
			act("dynamo.item-detail", "enter", "View item detail"),
			act("dynamo.filter-scan", kb.FilterScan, "Filter scan (attribute + operator + value)").kw("query", "where"),
			act("dynamo.partiql", kb.PartiQL, "PartiQL query").kw("sql", "select"),
			act("dynamo.next-page", kb.NextPage, "Load next page").kw("more"),
			act("app.save", kb.Save, "Save PartiQL query").kw("bookmark"),
		}
	case viewDynamoItemDetail:
		return []paletteItem{
			act("dynamo.edit-field", kb.EditField, "Edit field value").kw("update"),
			act("dynamo.clone-item", kb.CloneItem, "Clone item").kw("copy", "duplicate"),
			scrollLegend,
			topLegend,
		}
	case viewSQSQueues:
		return []paletteItem{
			act("sqs.queue-detail", "enter", "Queue detail"),
			act("app.save", kb.Save, "Save queue").kw("bookmark"),
		}
	case viewSQSDetail:
		return []paletteItem{
			act("sqs.messages", kb.Messages, "Browse messages"),
			act("sqs.send", kb.SendMessage, "Send new message").kw("publish", "enqueue"),
			act("sqs.dlq", kb.NavigateDLQ, "Navigate to dead letter queue").kw("dlq", "redrive"),
			act("app.refresh", "R", "Refresh stats"),
		}
	case viewSQSMessages:
		return []paletteItem{
			act("sqs.message-detail", "enter", "Message detail"),
			act("sqs.poll", kb.PollMessages, "Configure and poll for messages").kw("receive"),
			act("sqs.send", kb.SendMessage, "Send new message").kw("publish", "enqueue"),
			act("sqs.release", kb.ReleaseMsg, "Release message now").kw("visibility"),
			act("sqs.delete", kb.DeleteMsg, "Acknowledge / delete message").danger().kw("ack", "remove"),
			act("sqs.clear-local", "X", "Clear local list (does not release)"),
		}
	case viewSQSMessageDetail:
		return []paletteItem{
			act("sqs.clone-send", kb.CloneSend, "Clone & send").kw("resend", "copy"),
			act("sqs.release", kb.ReleaseMsg, "Release message now").kw("visibility"),
			act("sqs.delete", kb.DeleteMsg, "Acknowledge / delete message").danger().kw("ack", "remove"),
			scrollLegend,
			topLegend,
		}
	case viewEnvVars:
		if a.envVarsView.HasSecrets() {
			if a.envVarsView.SecretsResolved() {
				return []paletteItem{act("env.toggle-secrets", "a", "Toggle secret references/values")}
			}
			return []paletteItem{act("env.reveal-secrets", kb.RevealSecrets, "Resolve and reveal secret values").kw("show", "decrypt")}
		}
		return nil
	case viewLogGroups:
		items := []paletteItem{
			act("logs.open-group", "enter", "Browse streams"),
			act("logs.multi-select", "space", "Multi-select groups").kw("mark"),
			act("logs.tail-group", kb.TailStream, "Tail entire log group").kw("live"),
			act("logs.search", kb.Search, "Search selected groups").kw("find", "insights"),
			act("app.save", kb.Save, "Save log path / group selection").kw("bookmark"),
		}
		if a.logCorrelationActive {
			items = append(items, act("logs.correlate", kb.LogCorrelate, "Correlate selected groups"))
		}
		return items
	case viewLogStreams:
		items := []paletteItem{
			act("logs.peek", "enter", "Peek (last 1 min, paused)"),
			act("logs.multi-select", "space", "Multi-select streams").kw("mark"),
			act("logs.tail-stream", kb.TailStream, "Tail stream").kw("live"),
			act("logs.tail-group", kb.TailGroup, "Tail entire log group").kw("live"),
			act("logs.search", kb.Search, "Search selected streams").kw("find"),
			act("app.save", kb.Save, "Save log path").kw("bookmark"),
		}
		if a.logCorrelationActive {
			items = append(items, act("logs.correlate", kb.LogCorrelate, "Correlate selected streams"))
		}
		return items
	case viewLogSearch:
		return []paletteItem{
			act("logs.jump", "enter", "Jump to log at timestamp"),
			act("logs.correlate", kb.LogCorrelate, "Correlate selected result"),
			act("app.timestamps", kb.Timestamp, "Toggle timestamps").kw("utc", "time"),
			topLegend,
		}
	case viewAlarms:
		return []paletteItem{
			act("alarms.detail", "enter", "View alarm detail"),
			act("app.timestamps", kb.Timestamp, "Toggle local/UTC timestamps").kw("utc", "time"),
			act("app.filter", "/", "Filter alarms"),
		}
	case viewAlarmDetail:
		return []paletteItem{
			act("alarms.toggle-actions", kb.ToggleActions, "Enable/disable alarm actions").kw("mute", "silence"),
			act("alarms.set-state", kb.SetAlarmState, "Set alarm state (testing)").kw("trigger"),
			scrollLegend,
			topLegend,
		}
	case viewCBProjects:
		return []paletteItem{
			act("codebuild.builds", "enter", "View builds"),
			act("codebuild.start", kb.StartBuild, "Start new build").kw("run", "trigger"),
			act("app.filter", "/", "Filter projects"),
		}
	case viewCBBuilds:
		return []paletteItem{
			act("codebuild.build-detail", "enter", "View build detail"),
			act("codebuild.start", kb.StartBuild, "Start new build").kw("run", "trigger"),
			act("app.timestamps", kb.Timestamp, "Toggle local/UTC timestamps").kw("utc", "time"),
		}
	case viewCBBuildDetail:
		return []paletteItem{
			act("codebuild.logs", kb.ViewLogs, "View build logs (buffer)"),
			act("codebuild.search-logs", kb.Search, "Search build logs (full)").kw("find"),
			act("codebuild.start", kb.StartBuild, "Start new build").kw("run", "trigger"),
			act("codebuild.stop", kb.StopBuild, "Stop build (if in progress)").danger().kw("cancel", "abort"),
			scrollLegend,
			topLegend,
		}
	case viewTofuResources:
		return []paletteItem{
			act("tofu.state-detail", "enter", "View resource state"),
			act("tofu.plan", kb.RunPlan, "Run plan (parsed view)").kw("terraform", "diff"),
			act("tofu.apply", kb.RunApply, "Run apply (interactive)").danger().kw("terraform", "deploy"),
			act("tofu.init", kb.RunInit, "Run init").kw("terraform"),
			act("app.save", kb.Save, "Save workspace").kw("bookmark"),
			act("app.filter", "/", "Filter resources"),
		}
	case viewTofuPlan:
		return []paletteItem{
			act("tofu.change-detail", "enter", "View change detail"),
			act("tofu.apply", kb.RunApply, "Run apply (interactive)").danger().kw("terraform", "deploy"),
			act("app.filter", "/", "Filter changes"),
		}
	case viewTofuPlanDetail, viewTofuStateDetail:
		return []paletteItem{scrollLegend, topLegend}
	case viewECRRepos:
		return []paletteItem{
			act("ecr.images", "enter", "View images"),
			act("app.filter", "/", "Filter repositories"),
		}
	case viewECRImages:
		return []paletteItem{
			act("ecr.findings", "enter", "View scan findings").kw("vulnerabilities", "cve"),
			act("ecr.scan", kb.StartScan, "Start scan").kw("vulnerabilities"),
			act("ecr.copy-uri", kb.CopyURI, "Copy image URI to clipboard").kw("yank"),
			act("ecr.delete-image", kb.DeleteImage, "Delete image").danger().kw("remove"),
			act("app.filter", "/", "Filter by tag or digest"),
		}
	case viewECRFindings:
		return []paletteItem{act("app.filter", "/", "Filter findings")}
	case viewR53Zones:
		return []paletteItem{
			act("route53.records", "enter", "View records"),
			act("app.filter", "/", "Filter zones"),
		}
	case viewR53Records:
		return []paletteItem{
			act("route53.record-detail", "enter", "View record detail"),
			act("route53.new-record", kb.NewRecord, "Create new record").kw("add", "dns"),
			act("app.filter", "/", "Filter records"),
		}
	case viewR53RecordDetail:
		return []paletteItem{
			act("route53.test-dns", kb.TestDNS, "Test DNS resolution").kw("resolve", "dig"),
			act("route53.edit-record", kb.EditRecord, "Edit record").kw("update", "dns"),
			act("route53.delete-record", kb.DeleteRecord, "Delete record").danger().kw("remove", "dns"),
			scrollLegend,
			topLegend,
		}
	case viewRDSInstances:
		return []paletteItem{
			act("rds.instance-detail", "enter", "View instance detail + metrics"),
			act("app.filter", "/", "Filter instances"),
			act("rds.timestamps", "T", "Toggle relative/absolute timestamps").kw("time"),
		}
	case viewRDSClusters:
		return []paletteItem{
			act("rds.cluster-members", "enter", "Browse member instances"),
			act("rds.all-instances", "tab", "Switch to all instances"),
			act("app.filter", "/", "Filter clusters"),
		}
	case viewRDSDetail:
		return []paletteItem{
			act("app.open-resource", kb.OpenResource, "Open linked EC2 resource").kw("related", "jump", "security group"),
			scrollLegend,
			topLegend,
		}
	case viewCostExplorer:
		return []paletteItem{
			act("cost.views", "v", "Select overview, breakdown, anomalies, resources, or a saved view").kw("billing"),
			act("cost.force-refresh", "R", "Force paid refresh (confirmation required)").danger().kw("reload", "billing"),
			act("app.filter", "/", "Filter current rows"),
		}
	case viewElastiCache:
		return []paletteItem{
			act("elasticache.detail", "enter", "View cache details and current metrics"),
			act("elasticache.replication-groups", "1", "Replication groups").kw("redis", "valkey"),
			act("elasticache.cache-clusters", "2", "Cache clusters").kw("memcached", "redis"),
			act("elasticache.serverless", "3", "Serverless caches"),
			act("app.filter", "/", "Filter resources"),
		}
	case viewElastiCacheDetail, viewAPIGatewayDetail:
		return []paletteItem{scrollLegend, topLegend}
	case viewAPIGateway:
		return []paletteItem{
			act("apigateway.detail", "enter", "View stages, routes, integrations, mappings, and metrics"),
			act("apigateway.rest", "1", "REST APIs"),
			act("apigateway.http", "2", "HTTP APIs"),
			act("apigateway.websocket", "3", "WebSocket APIs"),
			act("apigateway.domains", "4", "Custom domains").kw("dns"),
			act("app.filter", "/", "Filter resources"),
		}
	case viewSQLConnections:
		return []paletteItem{
			act("sql.new-tab", "enter", "Open a new query tab for selected connection").kw("connect"),
			act("sql.open-tabs", "t", "Return to open tabs"),
			act("app.filter", "/", "Filter connections"),
		}
	case viewSQLWorkbench:
		return []paletteItem{
			act("sql.edit", "e", "Edit query (Esc returns to commands)"),
			act("sql.objects", "o", "Toggle database object browser").kw("catalog", "schema", "tables"),
			act("sql.run-all", "r", "Run all statements").kw("execute"),
			act("sql.run-current", "c", "Run statement at cursor").kw("execute"),
			act("sql.reconnect", "R", "Reconnect"),
			act("sql.previous-tab", "[", "Previous tab"),
			act("sql.next-tab", "]", "Next tab"),
			act("sql.connections", "n", "Connection browser / new tab"),
			act("sql.rename-tab", "t", "Rename tab"),
			act("sql.close-tab", "d", "Close tab"),
			act("sql.export-csv", "x", "Export latest result as CSV").kw("save"),
			act("sql.toggle-writes", "w", "Toggle per-tab write break-glass").danger().kw("write", "unsafe"),
			act("sql.catalog-expand", "enter", "Expand selected schema, category, table, or view in object browser"),
			act("sql.insert-name", "i", "Insert selected object name"),
			act("sql.insert-qualified", "I", "Insert selected qualified name"),
			legendItem("j/k", "Scroll result rows or object browser"),
		}
	case viewPlugins:
		return []paletteItem{
			act("plugins.run", "enter", "Configure and run selected plugin action"),
			act("app.filter", "/", "Filter plugin actions"),
		}
	case viewPluginSettings:
		return []paletteItem{
			act("plugins.add", "a", "Add and validate a plugin"),
			act("plugins.edit", "enter", "Edit selected plugin").alt("e"),
			act("plugins.remove", "d", "Remove selected plugin registration").danger(),
			act("plugins.edit-raw", "ctrl+e", "Edit raw configuration"),
		}
	case viewEC2Instances:
		return []paletteItem{
			act("ec2.instance-detail", "enter", "View instance detail"),
			act("app.filter", "/", "Filter instances"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
		}
	case viewEC2Detail:
		return []paletteItem{
			act("ec2.ssm-session", kb.SSMSession, "SSM session (shell into instance)").kw("ssh", "shell", "connect"),
			act("ec2.console-output", kb.ConsoleOutput, "View console output").kw("boot", "serial"),
			act("ec2.start", kb.StartInstance, "Start instance").kw("power on"),
			act("ec2.stop", kb.StopInstance, "Stop instance").danger().kw("shutdown", "power off"),
			act("ec2.reboot", kb.RebootInstance, "Reboot instance").danger().kw("restart"),
			act("ec2.terminate", kb.TermInstance, "Terminate instance").danger().kw("destroy", "delete"),
			act("app.open-resource", kb.OpenResource, "Open linked resource").kw("related", "jump"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
			scrollLegend,
			topLegend,
		}
	case viewEC2Console:
		return []paletteItem{scrollLegend, topLegend}
	case viewEC2SecurityGroups:
		return []paletteItem{
			act("ec2.security-group-detail", "enter", "View security group detail").kw("rules", "firewall"),
			act("app.filter", "/", "Filter security groups"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
		}
	case viewEC2SecurityGroupDetail:
		return []paletteItem{
			act("app.open-resource", kb.OpenResource, "Open linked resource").kw("related", "jump"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
			scrollLegend,
			topLegend,
		}
	case viewEC2VPCs, viewEC2Subnets, viewEC2Volumes, viewEC2LoadBalancers, viewEC2TargetGroups:
		return []paletteItem{
			act("ec2.resource-detail", "enter", "View detail"),
			act("app.filter", "/", "Filter resources"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
		}
	case viewEC2VPCDetail, viewEC2SubnetDetail, viewEC2VolumeDetail, viewEC2LoadBalancerDetail, viewEC2TargetGroupDetail:
		return []paletteItem{
			act("app.open-resource", kb.OpenResource, "Open linked resource").kw("related", "jump"),
			act("ec2.switch-resource", kb.EC2Resource, "Switch EC2 resource").kw("vpc", "subnet", "volume", "security group", "load balancer"),
			scrollLegend,
		}
	}
	return nil
}

// taskScopeItems are the task-list commands that depend on whether the list
// shows active or recently stopped tasks.
func (a App) taskScopeItems() []paletteItem {
	kb := a.kb
	if a.taskScopeStopped {
		if a.taskNextToken != "" {
			return []paletteItem{act("ecs.load-more", kb.LoadMore, "Load 50 more stopped tasks").kw("more", "page")}
		}
		return nil
	}
	return []paletteItem{
		act("ecs.stop-task", kb.StopTask, "Stop task").danger().kw("kill"),
		act("ecs.exec", kb.ECSExec, "ECS Exec (shell into container)").kw("ssh", "shell", "exec", "connect"),
		act("ecs.metrics", kb.Metrics, "Task CPU/memory metrics").kw("cloudwatch", "utilization"),
	}
}

// contextHelpLines returns the full keybinding list for the current view:
// view commands, a blank separator, then global commands.
func (a App) contextHelpLines() []struct{ key, desc string } {
	type kv = struct{ key, desc string }
	var all []kv
	if context := a.viewItems(a.state); len(context) > 0 {
		for _, item := range context {
			all = append(all, kv{item.displayKey(), item.Title})
		}
		all = append(all, kv{"", ""})
	}
	for _, item := range a.globalItems() {
		all = append(all, kv{item.displayKey(), item.Title})
	}
	return all
}
