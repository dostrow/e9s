package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
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
