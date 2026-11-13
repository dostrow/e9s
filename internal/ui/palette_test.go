package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/runbook"
	"github.com/dostrow/e9s/internal/ui/views"
)

func TestKeyMsgFromStringRoundTripsEveryBinding(t *testing.T) {
	keys := []string{"enter", "esc", "tab", "shift+tab", "ctrl+e", "ctrl+s", "ctrl+r", "ctrl+p", "alt+x", "pgdown", "[", "]", "/", "?", ":", "`", "!", "X"}
	kb := reflect.ValueOf(NewKeyBindings())
	for i := range kb.NumField() {
		keys = append(keys, kb.Field(i).String())
	}
	for _, k := range keys {
		if got := keyMsgFromString(k).String(); got != k {
			t.Errorf("keyMsgFromString(%q).String() = %q", k, got)
		}
	}
	if got := keyMsgFromString("space"); got.Type != tea.KeySpace {
		t.Errorf("space replays as %v", got.Type)
	}
}

func paletteTestApp(state viewState, modes ...topMode) App {
	a := App{kb: NewKeyBindings(), state: state, width: 120, height: 40, recency: command.NewRecency()}
	for _, m := range modes {
		a.modeTabs = append(a.modeTabs, ModeTab{Mode: m, Label: modeShortName(m)})
	}
	return a
}

func itemByID(items []paletteItem, id string) (paletteItem, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return paletteItem{}, false
}

func TestPaletteItemsResolveContext(t *testing.T) {
	a := paletteTestApp(viewServices, modeECS, modeSQS)
	items, legend := a.paletteItems()

	redeploy, ok := itemByID(items, "ecs.force-redeploy")
	if !ok || redeploy.Section != command.SectionView || redeploy.Key != "r" || !redeploy.Danger || redeploy.Category != "ECS › Services" {
		t.Fatalf("force redeploy = %+v", redeploy)
	}
	if quit, ok := itemByID(items, "app.quit"); !ok || quit.Section != command.SectionGlobal {
		t.Fatalf("global quit = %+v", quit)
	}
	if errItem, _ := itemByID(items, "app.error-details"); errItem.Available() {
		t.Fatal("error details should be unavailable without an error")
	}
	if goSQS, ok := itemByID(items, "mode.sqs"); !ok || goSQS.Section != command.SectionGoTo || goSQS.run == nil {
		t.Fatalf("mode item = %+v", goSQS)
	}
	if del, ok := itemByID(items, "sqs.delete"); !ok || del.Available() || !strings.Contains(del.Disabled, "SQS › Messages") {
		t.Fatalf("sqs delete from another view = %+v", del)
	}
	if _, ok := itemByID(items, "ec2.terminate"); ok {
		t.Fatal("commands from disabled modes must not be suggested")
	}
	if len(legend) != 0 {
		t.Fatalf("services view has no legend keys, got %v", legend)
	}
}

func TestPaletteViewKeyShadowsGlobalMeaning(t *testing.T) {
	items, _ := paletteTestApp(viewCostExplorer, modeCostExplorer).paletteItems()
	if _, ok := itemByID(items, "app.refresh"); ok {
		t.Fatal("R means a paid refresh in Cost Explorer; the generic refresh must be hidden")
	}
	if item, ok := itemByID(items, "cost.force-refresh"); !ok || item.Key != "R" {
		t.Fatalf("cost refresh = %+v", item)
	}
}

func TestPaletteLegendCarriesMovementKeys(t *testing.T) {
	_, legend := paletteTestApp(viewTaskDetail, modeECS).paletteItems()
	var keys []string
	for _, item := range legend {
		keys = append(keys, item.displayKey())
	}
	if !reflect.DeepEqual(keys, []string{"j/k", "g/G"}) {
		t.Fatalf("legend = %v", keys)
	}
}

func press(t *testing.T, a App, keys ...string) (App, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var m tea.Model
		m, cmd = a.Update(keyMsgFromString(k))
		a = m.(App)
	}
	return a, cmd
}

func typeText(t *testing.T, a App, text string) App {
	t.Helper()
	for _, r := range text {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		a = m.(App)
	}
	return a
}

func TestPaletteOpensSearchesAndRunsThroughKeyHandler(t *testing.T) {
	a := paletteTestApp(viewTaskDetail, modeECS)
	a, _ = press(t, a, ":")
	if !a.palette.Active {
		t.Fatal(": should open the palette")
	}
	if view := a.palette.View(a.width); !strings.Contains(view, "THIS VIEW") || !strings.Contains(view, "j/k scroll") {
		t.Fatalf("browse view missing sections or legend:\n%s", view)
	}

	a = typeText(t, a, "switch mode")
	if got := a.palette.results[a.palette.cursor].ID; got != "app.switch-mode" {
		t.Fatalf("top result = %q", got)
	}
	a, cmd := press(t, a, "enter")
	if a.palette.Active || cmd == nil {
		t.Fatal("enter should close the palette and emit the run message")
	}
	m, _ := a.Update(cmd())
	a = m.(App)
	if !a.modeSwitcher.Active {
		t.Fatal("running switch mode should replay ` and open the mode switcher")
	}
	if a.recency.Score("app.switch-mode") == 0 {
		t.Fatal("run should be recorded for recency")
	}
}

func TestQuestionMarkOpensPaletteAsHelp(t *testing.T) {
	a, _ := press(t, paletteTestApp(viewServices, modeECS), "?")
	if !a.palette.Active || a.palette.input.Value() != "" {
		t.Fatal("? should open the palette in browse mode")
	}
	if got := a.palette.results[0]; got.Section != command.SectionView || got.ID != "ecs.open-service" {
		t.Fatalf("browse view should start with this view's commands, got %+v", got.Entry)
	}
	a = typeText(t, a, "?")
	if a.palette.input.Value() != "?" || !a.palette.Active {
		t.Fatal("? typed inside the palette is query text")
	}
}

func TestPaletteTeachesShortcutAfterRun(t *testing.T) {
	a := paletteTestApp(viewTaskDetail, modeECS)
	m, _ := a.runPaletteItem(paletteItem{Entry: command.Entry{Command: command.Command{ID: "app.pause", Title: "Pause/resume polling"}}, keys: []string{"ctrl+s"}})
	a = m.(App)
	if !a.paused {
		t.Fatal("ctrl+s replay should pause polling")
	}
	// ctrl+s sets its own flash; the tip must not overwrite it.
	if a.flashMessage != "Polling paused" {
		t.Fatalf("flash = %q", a.flashMessage)
	}
	m, _ = a.runPaletteItem(paletteItem{Entry: command.Entry{Command: command.Command{ID: "app.switch-mode", Title: "Switch mode"}}, keys: []string{"`"}})
	a = m.(App)
	if !a.modeSwitcher.Active || !strings.Contains(a.flashMessage, "Tip: press `") {
		t.Fatalf("mode switcher=%v flash=%q", a.modeSwitcher.Active, a.flashMessage)
	}
}

func TestPaletteEscClosesAndCursorSkipsUnavailable(t *testing.T) {
	a := paletteTestApp(viewServices, modeECS, modeSQS)
	a, _ = press(t, a, ":")
	a = typeText(t, a, "delete message")
	results := a.palette.results
	if len(results) == 0 || results[0].Available() {
		t.Fatalf("expected only the unavailable SQS delete, got %+v", results)
	}
	a, cmd := press(t, a, "enter")
	if !a.palette.Active || cmd != nil {
		t.Fatal("enter on an unavailable row must do nothing")
	}
	if view := a.palette.View(a.width); !strings.Contains(view, "Available in SQS › Messages") {
		t.Fatalf("unavailable reason not rendered:\n%s", view)
	}
	a, _ = press(t, a, "esc")
	if a.palette.Active {
		t.Fatal("esc should close the palette")
	}
}

func TestPaletteRendersAtNarrowSizes(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {60, 20}, {200, 60}} {
		a := paletteTestApp(viewSQLWorkbench, modeSQLWorkbench, modeECS)
		a.width, a.height = size[0], size[1]
		a, _ = press(t, a, ":")
		for range 30 {
			a, _ = press(t, a, "down")
		}
		if view := a.palette.View(a.width); view == "" {
			t.Fatalf("empty palette at %v", size)
		}
	}
}

func TestHighlightRunesMarksPositions(t *testing.T) {
	got := highlightRunes("Stop task", []int{0, 5}, paletteDimStyle, paletteMatchStyle)
	if !strings.Contains(got, "top") || !strings.Contains(got, "ask") {
		t.Fatalf("unexpected render %q", got)
	}
	if truncateRunes("Force new deployment", 6) != "Force…" {
		t.Fatal("truncate")
	}
}

func savedTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.SSMPrefixes = []config.SSMPrefix{{Name: "prod db", Prefix: "/prod/db"}}
	cfg.DynamoQueries = []config.DynamoQuery{{Name: "recent orders", Statement: `SELECT * FROM "orders"`}}
	cfg.SQSQueues = []config.SQSQueueEntry{{Name: "jobs", URL: "https://sqs.example/jobs"}}
	return cfg
}

func TestPaletteListsSavedItemsFromEnabledModes(t *testing.T) {
	a := paletteTestApp(viewClusters, modeECS, modeSSM, modeDynamoDB)
	a.cfg = savedTestConfig()
	items, _ := a.paletteItems()

	ssm, ok := itemByID(items, "saved.prefixes.prod-db")
	if !ok || ssm.Section != command.SectionSaved || ssm.Target != "/prod/db" || ssm.run == nil {
		t.Fatalf("saved SSM prefix = %+v", ssm)
	}
	if query, ok := itemByID(items, "saved.queries.recent-orders"); !ok || query.Available() {
		t.Fatalf("saved PartiQL outside a table should be unavailable: %+v", query)
	}
	if _, ok := itemByID(items, "saved.queues.jobs"); ok {
		t.Fatal("SQS is disabled; its saved queues must not be listed")
	}

	a.state = viewDynamoItems
	items, _ = a.paletteItems()
	if query, _ := itemByID(items, "saved.queries.recent-orders"); !query.Available() {
		t.Fatal("saved PartiQL should run from a table's item list")
	}
}

func TestPaletteSavedItemOpensThroughPickerPath(t *testing.T) {
	a := paletteTestApp(viewClusters, modeECS, modeSSM)
	a.cfg = savedTestConfig()
	items, _ := a.paletteItems()
	ssm, _ := itemByID(items, "saved.prefixes.prod-db")
	m, cmd := a.runPaletteItem(ssm)
	a = m.(App)
	if a.mode != modeSSM || a.state != viewSSM || cmd == nil {
		t.Fatalf("saved prefix should open SSM: mode=%v state=%v cmd=%v", a.mode, a.state, cmd != nil)
	}
	if got := a.ssmView.PathPrefix(); got != "/prod/db" {
		t.Fatalf("path prefix = %q", got)
	}
}

func TestPalettePluginItemsOpenRunbookForm(t *testing.T) {
	a := paletteTestApp(viewClusters, modeECS, modePlugins)
	a.runbookActions = []runbook.ConfiguredAction{{
		PluginName: "ops", PluginDisplayName: "Ops tools",
		Action: runbook.Action{ID: "rotate", Title: "Rotate credentials", Description: "Rotate service keys", Risk: "destructive"},
	}}
	items, _ := a.paletteItems()
	item, ok := itemByID(items, "plugin.ops-rotate")
	if !ok || item.Section != command.SectionPlugins || !item.Danger {
		t.Fatalf("plugin item = %+v", item)
	}
	m, _ := a.runPaletteItem(item)
	a = m.(App)
	if !a.runbookForm.Active || a.mode != modePlugins {
		t.Fatal("plugin item should open its runbook form")
	}
}

func TestPaletteRegionIsANestedStep(t *testing.T) {
	a := paletteTestApp(viewClusters, modeECS)
	items, _ := a.paletteItems()
	region, _ := itemByID(items, "app.switch-region")
	m, cmd := a.runPaletteItem(region)
	a = m.(App)
	if !a.palette.Active || !a.palette.flat || cmd != nil {
		t.Fatal("switch region from the palette should open a nested region list")
	}
	a = typeText(t, a, "euw2")
	if got := a.palette.results[a.palette.cursor].ID; got != "region.eu-west-2" {
		t.Fatalf("top region = %q", got)
	}
	a, cmd = press(t, a, "enter")
	if a.palette.Active || cmd == nil {
		t.Fatal("choosing a region should close the palette")
	}
	if msg, ok := cmd().(paletteRunMsg); !ok || msg.item.run == nil {
		t.Fatalf("region selection should run its switch, got %#v", msg)
	}
}

func resourceTestApp() App {
	a := paletteTestApp(viewTaskDetail, modeECS, modeSQS, modeSSM)
	a.cfg = &config.Config{}
	a.sqsQueuesView = views.NewSQSQueues("").SetQueues([]model.SQSQueue{{Name: "billing-jobs"}, {Name: "orders"}})
	a.clusterView = views.NewClusterList().SetClusters([]model.Cluster{{Name: "prod"}, {Name: "staging"}})
	a.serviceView = views.NewServiceList("prod").SetServices([]model.Service{{Name: "api"}, {Name: "worker"}})
	return a
}

func TestPaletteResourcesAreSearchableButNotBrowsed(t *testing.T) {
	a, _ := press(t, resourceTestApp(), ":")
	for _, r := range a.palette.results {
		if r.Section == command.SectionResources {
			t.Fatalf("browse view must not list resources, found %s", r.ID)
		}
	}
	a = typeText(t, a, "bill")
	if r := a.palette.results[a.palette.cursor]; r.Section != command.SectionResources || r.Title != "billing-jobs" {
		t.Fatalf("top result = %+v", r.Entry)
	}
	a, _ = press(t, a, "esc", ":")
	a = typeText(t, a, "@")
	for _, r := range a.palette.results {
		if r.Section != command.SectionResources {
			t.Fatalf("@ lists only resources, found %s", r.ID)
		}
	}
	if len(a.palette.results) != 6 {
		t.Fatalf("@ should list all 6 loaded resources, got %d", len(a.palette.results))
	}
}

func TestPaletteGoToResourceSelectsRowWithoutOpeningIt(t *testing.T) {
	a := resourceTestApp()
	a, _ = press(t, a, ":")
	a = typeText(t, a, "@orders")
	a, cmd := press(t, a, "enter")
	m, _ := a.Update(cmd())
	a = m.(App)
	if a.mode != modeSQS || a.state != viewSQSQueues {
		t.Fatalf("mode=%v state=%v", a.mode, a.state)
	}
	if q := a.sqsQueuesView.SelectedQueue(); q == nil || q.Name != "orders" {
		t.Fatalf("selected queue = %+v", q)
	}
	if !strings.Contains(a.flashMessage, "Selected orders") {
		t.Fatalf("flash = %q", a.flashMessage)
	}
}

func TestPaletteGoToServiceRestoresItsCluster(t *testing.T) {
	a := resourceTestApp()
	a.state = viewSQSQueues
	a.mode = modeSQS
	items, _ := a.paletteItems()
	worker, ok := itemByID(items, "resource.ecs-services.worker")
	if !ok {
		t.Fatal("loaded service should be a resource")
	}
	m, _ := a.runPaletteItem(worker)
	a = m.(App)
	if a.state != viewServices || a.selectedCluster == nil || a.selectedCluster.Name != "prod" {
		t.Fatalf("state=%v cluster=%+v", a.state, a.selectedCluster)
	}
	if s := a.serviceView.SelectedService(); s == nil || s.Name != "worker" {
		t.Fatalf("selected service = %+v", s)
	}

	a.clusterView = views.NewClusterList().SetClusters([]model.Cluster{{Name: "staging"}})
	items, _ = a.paletteItems()
	if _, ok := itemByID(items, "resource.ecs-services.worker"); ok {
		t.Fatal("services whose cluster is no longer loaded must not be offered")
	}
}

func TestCtrlPOpensGoToPaletteLedByEntryPicker(t *testing.T) {
	a := resourceTestApp()
	a.mode, a.state = modeSSM, viewSSM
	a.recency.Record("mode.sqs") // a Recent section must not displace the first row
	a, _ = press(t, a, "ctrl+p")
	if !a.palette.Active || a.palette.prompt != "go to ›" {
		t.Fatalf("ctrl+p should open the go-to palette, prompt=%q", a.palette.prompt)
	}
	if got := a.palette.results[0].ID; got != "app.reopen-entry-picker" {
		t.Fatalf("first go-to row = %q", got)
	}
	for _, r := range a.palette.results {
		if r.Section == command.SectionGlobal {
			t.Fatalf("go-to mode should not list global commands, found %s", r.ID)
		}
	}
	a, cmd := press(t, a, "enter")
	m, _ := a.Update(cmd())
	a = m.(App)
	if !a.input.Active {
		t.Fatal("ctrl+p then enter should still open the SSM path prompt")
	}
}

func TestPaletteSavedLogPathOpensLogStreams(t *testing.T) {
	a := paletteTestApp(viewClusters, modeECS, modeCWLogs)
	a.cfg = &config.Config{LogPaths: []config.LogPathEntry{{Name: "Production Connectors", LogGroup: "/prod/connectors"}}}
	items, _ := a.paletteItems()
	saved, ok := itemByID(items, "saved.log-paths.production-connectors")
	if !ok {
		t.Fatal("saved log path missing from palette")
	}
	m, cmd := a.runPaletteItem(saved)
	a = m.(App)
	if a.mode != modeCWLogs || a.state == viewClusters || cmd == nil {
		t.Fatalf("saved log path did not navigate: mode=%v state=%v cmd=%v", a.mode, a.state, cmd != nil)
	}
	t.Logf("saved log path opened state %s", viewCategory(a.state))
}

func TestPaletteRecentFollowsThisViewAndKeepsOriginalRow(t *testing.T) {
	a := paletteTestApp(viewServices, modeECS)
	a.recency.Record("app.edit-config")
	a, _ = press(t, a, ":")

	var sections []command.Section
	count := 0
	for _, r := range a.palette.results {
		if len(sections) == 0 || sections[len(sections)-1] != r.Section {
			sections = append(sections, r.Section)
		}
		if r.ID == "app.edit-config" {
			count++
		}
	}
	if len(sections) < 3 || sections[0] != command.SectionView || sections[1] != command.SectionRecent {
		t.Fatalf("section order = %v, want This view, Recent, ...", sections)
	}
	if count != 2 {
		t.Fatalf("a recent command should appear in Recent and in Global, found %d rows", count)
	}
	if got := a.palette.results[a.palette.cursor].Section; got != command.SectionView {
		t.Fatalf("cursor should start on this view's first command, got section %v", got)
	}
	view := a.palette.View(a.width)
	if strings.Index(view, "THIS VIEW") > strings.Index(view, "RECENT") {
		t.Fatalf("RECENT rendered above THIS VIEW:\n%s", view)
	}
}
