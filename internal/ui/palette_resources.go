package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
	"github.com/dostrow/e9s/internal/model"
)

// maxResourcesPerSource bounds how many rows one list view contributes, so a
// session that loaded thousands of log groups keeps the palette responsive.
const maxResourcesPerSource = 2000

// resourceSource is a top-level list view whose loaded rows the palette can
// jump to. Jumping selects the row in its list rather than opening it, so a
// "go to" never fetches data or reveals values on its own.
type resourceSource struct {
	mode  topMode
	state viewState
	names func(App) []string
	focus func(App, int) App
	// ready reports whether the view's rows can be shown now (services need
	// their cluster to still be loaded).
	ready func(App) bool
}

var resourceSources = []resourceSource{
	{mode: modeECS, state: viewClusters,
		names: func(a App) []string { return a.clusterView.RowNames() },
		focus: func(a App, i int) App { a.clusterView = a.clusterView.WithCursor(i); return a }},
	{mode: modeECS, state: viewServices,
		names: func(a App) []string { return a.serviceView.RowNames() },
		ready: func(a App) bool { _, ok := a.serviceCluster(); return ok },
		focus: func(a App, i int) App {
			if c, ok := a.serviceCluster(); ok {
				a.selectedCluster = c
			}
			a.serviceView = a.serviceView.WithCursor(i)
			return a
		}},
	{mode: modeSQS, state: viewSQSQueues,
		names: func(a App) []string { return a.sqsQueuesView.RowNames() },
		focus: func(a App, i int) App { a.sqsQueuesView = a.sqsQueuesView.WithCursor(i); return a }},
	{mode: modeLambda, state: viewLambdaList,
		names: func(a App) []string { return a.lambdaListView.RowNames() },
		focus: func(a App, i int) App { a.lambdaListView = a.lambdaListView.WithCursor(i); return a }},
	{mode: modeS3, state: viewS3Buckets,
		names: func(a App) []string { return a.s3BucketsView.RowNames() },
		focus: func(a App, i int) App { a.s3BucketsView = a.s3BucketsView.WithCursor(i); return a }},
	{mode: modeDynamoDB, state: viewDynamoTables,
		names: func(a App) []string { return a.dynamoTablesView.RowNames() },
		focus: func(a App, i int) App { a.dynamoTablesView = a.dynamoTablesView.WithCursor(i); return a }},
	{mode: modeCWLogs, state: viewLogGroups,
		names: func(a App) []string { return a.logGroupsView.RowNames() },
		focus: func(a App, i int) App { a.logGroupsView = a.logGroupsView.WithCursor(i); return a }},
	{mode: modeCWAlarms, state: viewAlarms,
		names: func(a App) []string { return a.alarmsView.RowNames() },
		focus: func(a App, i int) App { a.alarmsView = a.alarmsView.WithCursor(i); return a }},
	{mode: modeCodeBuild, state: viewCBProjects,
		names: func(a App) []string { return a.cbProjectsView.RowNames() },
		focus: func(a App, i int) App { a.cbProjectsView = a.cbProjectsView.WithCursor(i); return a }},
	{mode: modeEC2, state: viewEC2Instances,
		names: func(a App) []string { return a.ec2InstancesView.RowNames() },
		focus: func(a App, i int) App { a.ec2InstancesView = a.ec2InstancesView.WithCursor(i); return a }},
	{mode: modeECR, state: viewECRRepos,
		names: func(a App) []string { return a.ecrReposView.RowNames() },
		focus: func(a App, i int) App { a.ecrReposView = a.ecrReposView.WithCursor(i); return a }},
	{mode: modeRoute53, state: viewR53Zones,
		names: func(a App) []string { return a.r53ZonesView.RowNames() },
		focus: func(a App, i int) App { a.r53ZonesView = a.r53ZonesView.WithCursor(i); return a }},
	{mode: modeRDS, state: viewRDSClusters,
		names: func(a App) []string { return a.rdsClustersView.RowNames() },
		focus: func(a App, i int) App { a.rdsClustersView = a.rdsClustersView.WithCursor(i); return a }},
	{mode: modeRDS, state: viewRDSInstances,
		names: func(a App) []string { return a.rdsInstancesView.RowNames() },
		focus: func(a App, i int) App { a.rdsInstancesView = a.rdsInstancesView.WithCursor(i); return a }},
	{mode: modeSM, state: viewSecrets,
		names: func(a App) []string { return a.secretsView.RowNames() },
		focus: func(a App, i int) App { a.secretsView = a.secretsView.WithCursor(i); return a }},
	{mode: modeSSM, state: viewSSM,
		names: func(a App) []string { return a.ssmView.RowNames() },
		focus: func(a App, i int) App { a.ssmView = a.ssmView.WithCursor(i); return a }},
}

// serviceCluster finds the loaded cluster whose services the service list
// holds.
func (a App) serviceCluster() (*model.Cluster, bool) {
	name := a.serviceView.ClusterName()
	if name == "" {
		return nil, false
	}
	for i, n := range a.clusterView.RowNames() {
		if n == name {
			if c := a.clusterView.SelectIndex(i); c != nil {
				return c, true
			}
		}
	}
	return nil, false
}

// resourceItems lists rows already loaded into top-level list views of
// enabled modes. They are searchable but never shown in the browse view.
func (a App) resourceItems() []paletteItem {
	enabled := a.enabledModes()
	var items []paletteItem
	for _, source := range resourceSources {
		src := source
		if !enabled[src.mode] || (src.ready != nil && !src.ready(a)) {
			continue
		}
		names := src.names(a)
		category := viewCategory(src.state)
		for _, name := range names[:min(len(names), maxResourcesPerSource)] {
			resource := name
			items = append(items, paletteItem{
				Entry: command.Entry{
					Command: command.Command{
						ID:       "resource." + paletteSlug(category) + "." + resource,
						Title:    resource,
						Category: category,
						Keywords: []string{modeShortName(src.mode), modeDisplayName(src.mode), viewNames[src.state]},
					},
					Section: command.SectionResources,
				},
				run: func(a App) (App, tea.Cmd) { return a.goToResource(src, resource) },
			})
		}
	}
	return items
}

// goToResource shows the resource's list with its row selected. The row is
// found by name at run time because refreshes may reorder rows while the
// palette is open.
func (a App) goToResource(src resourceSource, name string) (App, tea.Cmd) {
	index := -1
	for i, n := range src.names(a) {
		if n == name {
			index = i
			break
		}
	}
	if index < 0 || (src.ready != nil && !src.ready(a)) {
		return a.flash(name + " is no longer loaded; refresh its list first"), nil
	}
	a.err = nil
	a.mode = src.mode
	a.state = src.state
	a = src.focus(a, index)
	return a.flash("Selected " + name + " — enter opens it"), nil
}

// openGoToPalette opens the palette in "go to" mode (ctrl+p): modules, saved
// bookmarks, and loaded resources, led by the current mode's entry picker so
// ctrl+p then enter still reopens it.
func (a App) openGoToPalette() App {
	items, _ := a.paletteItems()
	mode := modeDisplayName(a.mode)
	goTo := []paletteItem{{
		Entry: command.Entry{
			Command: command.Command{
				ID:       "app.reopen-entry-picker",
				Title:    "Reopen " + mode + " entry picker",
				Category: modeShortName(a.mode),
				Keywords: []string{"picker", "prefix", "search", "custom"},
			},
			Section: command.SectionView,
		},
		run: func(a App) (App, tea.Cmd) { return a.reopenModePicker() },
	}}
	for _, item := range items {
		switch item.Section {
		case command.SectionGoTo, command.SectionSaved, command.SectionPlugins, command.SectionResources:
			goTo = append(goTo, item)
		}
	}
	a.palette = newPalette(goTo, nil, a.recency, "go to ›", a.height)
	a.palette.stableBrowse = true
	a.palette.refilter()
	return a
}

func (a App) flash(message string) App {
	a.flashMessage = message
	a.flashExpiry = time.Now().Add(4 * time.Second)
	return a
}

// resourceQuery reports whether query asks for resources only ("@name") and
// returns it without the marker.
func resourceQuery(query string) (string, bool) {
	trimmed := strings.TrimLeft(query, " ")
	if rest, ok := strings.CutPrefix(trimmed, "@"); ok {
		return rest, true
	}
	return query, false
}
