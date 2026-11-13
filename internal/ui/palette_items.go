package ui

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
)

// openPalette snapshots the commands for the current context and shows the
// palette.
func (a App) openPalette() App {
	items, legend := a.paletteItems()
	a.palette = newPalette(items, legend, a.recency, ":", a.height)
	return a
}

// paletteItems assembles every palette candidate for the current context, in
// priority order (the first registration of an ID wins), plus the movement
// legend for the browse view.
func (a App) paletteItems() ([]paletteItem, []paletteItem) {
	var items, legend []paletteItem
	category := viewCategory(a.state)
	target := a.paletteTarget()

	viewKeys := map[string]bool{}
	for _, item := range a.viewItems(a.state) {
		if item.legend {
			legend = append(legend, item)
			continue
		}
		item.Section = command.SectionView
		item.Category = category
		item.Target = target
		item.Key = item.displayKey()
		viewKeys[item.runKey()] = true
		items = append(items, item)
	}

	for _, item := range a.globalItems() {
		// A view command that reuses a global key (R in Cost Explorer means
		// a paid refresh) is what that key does here; listing the global
		// meaning too would replay the view command under the wrong title.
		if viewKeys[item.runKey()] {
			continue
		}
		item.Section = command.SectionGlobal
		item.Category = "Global"
		item.Key = item.displayKey()
		if item.ID == "app.error-details" && a.err == nil {
			item.Disabled = "No error to show"
		}
		items = append(items, item)
	}

	items = append(items, a.modeItems()...)
	items = append(items, a.unavailableItems(items)...)
	return items, legend
}

// modeItems lists a "Go to" command for each enabled mode.
func (a App) modeItems() []paletteItem {
	tabs := append([]ModeTab(nil), a.modeTabs...)
	sort.SliceStable(tabs, func(i, j int) bool {
		return modeDisplayName(tabs[i].Mode) < modeDisplayName(tabs[j].Mode)
	})
	items := make([]paletteItem, 0, len(tabs))
	for _, tab := range tabs {
		mode := tab.Mode
		name := modeDisplayName(mode)
		short := modeShortName(mode)
		item := paletteItem{
			Entry: command.Entry{
				Command: command.Command{
					ID:       "mode." + strings.ToLower(strings.ReplaceAll(name, " ", "-")),
					Title:    "Go to " + name,
					Category: "Modes",
					Keywords: []string{short, tab.Label, "mode", "module", "switch"},
				},
				Section: command.SectionGoTo,
			},
			run: func(a App) (App, tea.Cmd) { return a.switchMode(mode) },
		}
		if mode == a.mode {
			item.Target = "current"
		}
		items = append(items, item)
	}
	return items
}

// unavailableItems lists commands from other views of enabled modes so a
// precise search can say where a command lives instead of finding nothing.
func (a App) unavailableItems(available []paletteItem) []paletteItem {
	have := make(map[string]bool, len(available))
	for _, item := range available {
		have[item.ID] = true
	}
	enabled := map[topMode]bool{}
	for _, tab := range a.modeTabs {
		enabled[tab.Mode] = true
	}
	var out []paletteItem
	for state := viewClusters; state <= viewPluginSettings; state++ {
		if state == a.state {
			continue
		}
		if mode, ok := stateMode[state]; !ok || !enabled[mode] {
			continue
		}
		for _, item := range a.viewItems(state) {
			if item.legend || have[item.ID] {
				continue
			}
			have[item.ID] = true
			item.Section = command.SectionUnavailable
			item.Category = viewCategory(state)
			item.Key = item.displayKey()
			item.Disabled = "Available in " + viewCategory(state)
			out = append(out, item)
		}
	}
	return out
}

// runPaletteItem executes an item the palette returned, records it for
// recency ranking, and, when it has a shortcut, teaches that shortcut.
func (a App) runPaletteItem(item paletteItem) (tea.Model, tea.Cmd) {
	a.recency.Record(item.ID)
	save := a.saveRecencyCmd()
	if item.run != nil {
		next, cmd := item.run(a)
		return next, tea.Batch(cmd, save)
	}
	key := item.runKey()
	if key == "" {
		return a, save
	}
	before := a.flashMessage
	model, cmd := a.Update(keyMsgFromString(key))
	next, ok := model.(App)
	if !ok {
		return model, tea.Batch(cmd, save)
	}
	// Only teach when the command did not post its own status message.
	if teachShortcut(key) && next.flashMessage == before && !next.palette.Active {
		next.flashMessage = "Tip: press " + key + " to run “" + item.Title + "” directly"
		next.flashExpiry = time.Now().Add(4 * time.Second)
	}
	return next, tea.Batch(cmd, save)
}

// teachShortcut reports whether a key is worth a "press X next time" tip.
// Navigation keys everyone already uses would only add noise.
func teachShortcut(key string) bool {
	switch key {
	case "enter", "esc", "q", "/", "?", "tab":
		return false
	}
	return true
}

func (a App) saveRecencyCmd() tea.Cmd {
	recency, path := a.recency, a.recencyPath
	if recency == nil || path == "" {
		return nil
	}
	return func() tea.Msg {
		_ = recency.Save(path)
		return nil
	}
}

// paletteTarget names the object the current view's commands act on.
func (a App) paletteTarget() string {
	switch a.state {
	case viewClusters:
		if c := a.clusterView.SelectedCluster(); c != nil {
			return c.Name
		}
	case viewServices:
		if s := a.serviceView.SelectedService(); s != nil {
			return s.Name
		}
	case viewTasks:
		if t := a.taskView.SelectedTask(); t != nil {
			return t.TaskID
		}
	case viewStandaloneTasks:
		if t := a.standaloneView.SelectedTask(); t != nil {
			return t.TaskID
		}
	case viewTaskDetail:
		if a.selectedTask != nil {
			return a.selectedTask.TaskID
		}
	case viewServiceDetail, viewMetrics:
		if a.selectedService != nil {
			return a.selectedService.Name
		}
	case viewTaskDefs:
		if td := a.taskDefsView.SelectedTaskDef(); td != nil {
			return td.Family
		}
	case viewSSM:
		if p := a.ssmView.SelectedParam(); p != nil {
			return p.Name
		}
	case viewSecrets:
		if s := a.secretsView.SelectedSecret(); s != nil {
			return s.Name
		}
	case viewLambdaList:
		if f := a.lambdaListView.SelectedFunction(); f != nil {
			return f.Name
		}
	case viewDynamoTables:
		return a.dynamoTablesView.SelectedTable()
	case viewSQSQueues:
		if q := a.sqsQueuesView.SelectedQueue(); q != nil {
			return q.Name
		}
	case viewLogGroups:
		if g := a.logGroupsView.SelectedGroup(); g != nil {
			return g.Name
		}
	case viewLogStreams:
		if s := a.logStreamsView.SelectedStream(); s != nil {
			return s.Name
		}
	case viewAlarms:
		if al := a.alarmsView.SelectedAlarm(); al != nil {
			return al.Name
		}
	case viewCBProjects:
		if p := a.cbProjectsView.SelectedProject(); p != nil {
			return p.Name
		}
	case viewCBBuilds:
		if b := a.cbBuildsView.SelectedBuild(); b != nil {
			return b.ID
		}
	case viewEC2Instances:
		if i := a.ec2InstancesView.SelectedInstance(); i != nil {
			if i.Name != "" {
				return i.Name
			}
			return i.InstanceID
		}
	case viewECRRepos:
		if r := a.ecrReposView.SelectedRepo(); r != nil {
			return r.Name
		}
	case viewECRImages:
		if img := a.ecrImagesView.SelectedImage(); img != nil {
			if len(img.Tags) > 0 {
				return img.Tags[0]
			}
			return shortDigest(img.Digest)
		}
	case viewR53Zones:
		if z := a.r53ZonesView.SelectedZone(); z != nil {
			return z.Name
		}
	case viewR53RecordDetail:
		if r := a.r53DetailView.Record(); r != nil {
			return r.Name
		}
	case viewRDSInstances:
		if i := a.rdsInstancesView.SelectedInstance(); i != nil {
			return i.Identifier
		}
	case viewRDSClusters:
		if c := a.rdsClustersView.SelectedCluster(); c != nil {
			return c.Identifier
		}
	case viewTofuResources:
		if r := a.tofuResourcesView.SelectedResource(); r != nil {
			return r.Address
		}
	case viewPlugins:
		if action := a.runbooksView.SelectedAction(); action != nil {
			return action.Action.Title
		}
	}
	return ""
}

func shortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

// keyMsgFromString builds the KeyMsg whose String() is s, so a palette item
// can replay its shortcut through the normal key handler.
func keyMsgFromString(s string) tea.KeyMsg {
	alt := false
	if rest, ok := strings.CutPrefix(s, "alt+"); ok && rest != "" {
		alt, s = true, rest
	}
	if s == "space" || s == " " {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}, Alt: alt}
	}
	if t, ok := namedKeyTypes[s]; ok {
		return tea.KeyMsg{Type: t, Alt: alt}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Alt: alt}
}

// namedKeyTypes maps bubbletea's key names ("enter", "ctrl+s") back to their
// key types.
var namedKeyTypes = func() map[string]tea.KeyType {
	names := map[string]tea.KeyType{}
	for t := tea.KeyType(-256); t <= 256; t++ {
		if t == tea.KeyRunes || t == tea.KeySpace {
			continue
		}
		if name := (tea.Key{Type: t}).String(); name != "" {
			if _, exists := names[name]; !exists {
				names[name] = t
			}
		}
	}
	return names
}()

// stateMode maps each view to the mode that owns it, so commands from
// disabled modes are never suggested.
var stateMode = map[viewState]topMode{
	viewClusters: modeECS, viewServices: modeECS, viewTasks: modeECS, viewTaskDetail: modeECS,
	viewTaskDefs: modeECS, viewTaskDefDetail: modeECS, viewServiceDetail: modeECS,
	viewStandaloneTasks: modeECS, viewTaskDefDiff: modeECS, viewMetrics: modeECS, viewEnvVars: modeECS,
	viewLogs: modeCWLogs, viewLogGroups: modeCWLogs, viewLogStreams: modeCWLogs, viewLogSearch: modeCWLogs,
	viewSSM:     modeSSM,
	viewSecrets: modeSM, viewSecretValue: modeSM,
	viewS3Buckets: modeS3, viewS3Objects: modeS3, viewS3Detail: modeS3,
	viewLambdaList: modeLambda, viewLambdaDetail: modeLambda,
	viewDynamoTables: modeDynamoDB, viewDynamoItems: modeDynamoDB, viewDynamoItemDetail: modeDynamoDB,
	viewSQSQueues: modeSQS, viewSQSDetail: modeSQS, viewSQSMessages: modeSQS, viewSQSMessageDetail: modeSQS,
	viewAlarms: modeCWAlarms, viewAlarmDetail: modeCWAlarms,
	viewCBProjects: modeCodeBuild, viewCBBuilds: modeCodeBuild, viewCBBuildDetail: modeCodeBuild,
	viewEC2Instances: modeEC2, viewEC2Detail: modeEC2, viewEC2Console: modeEC2,
	viewEC2SecurityGroups: modeEC2, viewEC2SecurityGroupDetail: modeEC2,
	viewEC2VPCs: modeEC2, viewEC2VPCDetail: modeEC2, viewEC2Subnets: modeEC2, viewEC2SubnetDetail: modeEC2,
	viewEC2Volumes: modeEC2, viewEC2VolumeDetail: modeEC2, viewEC2LoadBalancers: modeEC2,
	viewEC2LoadBalancerDetail: modeEC2, viewEC2TargetGroups: modeEC2, viewEC2TargetGroupDetail: modeEC2,
	viewECRRepos: modeECR, viewECRImages: modeECR, viewECRFindings: modeECR,
	viewTofuResources: modeTofu, viewTofuStateDetail: modeTofu, viewTofuPlan: modeTofu, viewTofuPlanDetail: modeTofu,
	viewR53Zones: modeRoute53, viewR53Records: modeRoute53, viewR53RecordDetail: modeRoute53,
	viewRDSClusters: modeRDS, viewRDSInstances: modeRDS, viewRDSDetail: modeRDS,
	viewCostExplorer: modeCostExplorer,
	viewElastiCache:  modeElastiCache, viewElastiCacheDetail: modeElastiCache,
	viewAPIGateway: modeAPIGateway, viewAPIGatewayDetail: modeAPIGateway,
	viewSQLConnections: modeSQLWorkbench, viewSQLWorkbench: modeSQLWorkbench,
	viewPlugins:        modePlugins,
	viewPluginSettings: modeSettings,
}

var viewNames = map[viewState]string{
	viewClusters: "Clusters", viewServices: "Services", viewTasks: "Tasks", viewTaskDetail: "Task detail",
	viewTaskDefs: "Task definitions", viewTaskDefDetail: "Task definition", viewServiceDetail: "Service detail",
	viewStandaloneTasks: "Standalone tasks", viewTaskDefDiff: "Task definition diff", viewMetrics: "Metrics",
	viewEnvVars: "Environment", viewLogs: "Log viewer", viewLogGroups: "Log groups", viewLogStreams: "Log streams",
	viewLogSearch: "Search results", viewSSM: "Parameters", viewSecrets: "Secrets", viewSecretValue: "Secret value",
	viewS3Buckets: "Buckets", viewS3Objects: "Objects", viewS3Detail: "Object detail",
	viewLambdaList: "Functions", viewLambdaDetail: "Function detail",
	viewDynamoTables: "Tables", viewDynamoItems: "Items", viewDynamoItemDetail: "Item detail",
	viewSQSQueues: "Queues", viewSQSDetail: "Queue detail", viewSQSMessages: "Messages", viewSQSMessageDetail: "Message detail",
	viewAlarms: "Alarms", viewAlarmDetail: "Alarm detail",
	viewCBProjects: "Projects", viewCBBuilds: "Builds", viewCBBuildDetail: "Build detail",
	viewEC2Instances: "Instances", viewEC2Detail: "Instance detail", viewEC2Console: "Console output",
	viewEC2SecurityGroups: "Security groups", viewEC2SecurityGroupDetail: "Security group",
	viewEC2VPCs: "VPCs", viewEC2VPCDetail: "VPC", viewEC2Subnets: "Subnets", viewEC2SubnetDetail: "Subnet",
	viewEC2Volumes: "Volumes", viewEC2VolumeDetail: "Volume", viewEC2LoadBalancers: "Load balancers",
	viewEC2LoadBalancerDetail: "Load balancer", viewEC2TargetGroups: "Target groups", viewEC2TargetGroupDetail: "Target group",
	viewECRRepos: "Repositories", viewECRImages: "Images", viewECRFindings: "Scan findings",
	viewTofuResources: "Resources", viewTofuStateDetail: "Resource state", viewTofuPlan: "Plan", viewTofuPlanDetail: "Planned change",
	viewR53Zones: "Hosted zones", viewR53Records: "Records", viewR53RecordDetail: "Record detail",
	viewRDSClusters: "Clusters", viewRDSInstances: "Instances", viewRDSDetail: "Instance detail",
	viewCostExplorer: "Costs", viewElastiCache: "Caches", viewElastiCacheDetail: "Cache detail",
	viewAPIGateway: "APIs", viewAPIGatewayDetail: "API detail",
	viewSQLConnections: "Connections", viewSQLWorkbench: "Workbench",
	viewPlugins: "Actions", viewPluginSettings: "Plugin settings",
}

// viewCategory labels a view as "Mode › View" for palette rows.
func viewCategory(state viewState) string {
	name := viewNames[state]
	mode, ok := stateMode[state]
	if !ok {
		return name
	}
	return modeShortName(mode) + " › " + name
}
