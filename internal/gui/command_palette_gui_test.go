//go:build gui

package gui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/command"
	"github.com/dostrow/e9s/internal/config"
)

func TestPrintableShortcutsHaveUniqueKeysAndKnownActions(t *testing.T) {
	known := map[string]bool{"command-palette": true}
	for _, spec := range paletteActionSpecs {
		known[spec.action] = true
	}
	seen := map[[2]uint]string{}
	for _, shortcut := range printableShortcuts {
		if !known[shortcut.action] {
			t.Errorf("printable shortcut %q targets unknown action %q", shortcut.accel, shortcut.action)
		}
		key := [2]uint{shortcut.keyval, uint(shortcut.mods)}
		if other, dup := seen[key]; dup {
			t.Errorf("%q and %q share a key", other, shortcut.action)
		}
		seen[key] = shortcut.action
	}
}

func TestPaletteActionSpecsAreWellFormed(t *testing.T) {
	ids := map[string]bool{}
	for _, spec := range paletteActionSpecs {
		if spec.action == "" || spec.id == "" || spec.title == "" {
			t.Errorf("incomplete spec %+v", spec)
		}
		if spec.button == nil && spec.available == nil {
			t.Errorf("%s has no availability rule", spec.id)
		}
		if ids[spec.id] {
			t.Errorf("duplicate id %s", spec.id)
		}
		ids[spec.id] = true
	}
}

func TestHighlightMarkupEscapesAndMarks(t *testing.T) {
	got := highlightMarkup("Clone & send", []int{0, 8})
	want := `<span weight="bold" underline="single">C</span>lone &amp; <span weight="bold" underline="single">s</span>end`
	if got != want {
		t.Fatalf("highlightMarkup = %q\nwant %q", got, want)
	}
	if slug("Acknowledge / delete…") != "acknowledge-delete" || slug("plugin:ops-tools") != "plugin-ops-tools" {
		t.Fatal("slug")
	}
	if !dangerousLabel("Terminate…") || dangerousLabel("Start…") {
		t.Fatal("dangerousLabel")
	}
}

// TestCommandPaletteInRealWindow builds the main window, opens the palette,
// and checks its commands. It maps real windows, so it only runs when
// E9S_GUI_WINDOW_TESTS is set (CI or a headless broadway display), never by
// default on a desktop session.
func TestCommandPaletteInRealWindow(t *testing.T) {
	if os.Getenv("E9S_GUI_WINDOW_TESTS") == "" {
		t.Skip("set E9S_GUI_WINDOW_TESTS=1 to run tests that map windows")
	}
	if !gtk.InitCheck() {
		t.Skip("GTK display unavailable")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	app := gtk.NewApplication("io.github.dostrow.e9s.palettetest", gio.ApplicationNonUnique)
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, strings.TrimSpace(fmt.Sprintf(format, args...)))
	}
	app.ConnectActivate(func() {
		installStyles()
		cfg := &config.Config{}
		cfg.SSMPrefixes = []config.SSMPrefix{{Name: "prod-db", Prefix: "/prod/db"}}
		cfg.LogPaths = []config.LogPathEntry{{Name: "Production Connectors", LogGroup: "/prod/connectors"}}
		w := newMainWindow(context.Background(), app, Options{Config: cfg})
		installSemanticStyles(w)
		w.window.SetDefaultSize(1280, 800)
		w.window.Present()

		glib.TimeoutAdd(500, func() bool {
			// Saved items live in collapsed rail expanders, whose buttons
			// are unrealized; running one must still navigate.
			ran := false
			for _, c := range w.paletteCommands() {
				if c.Section == command.SectionSaved && c.Title == "Production Connectors" {
					w.runPaletteCommand(c)
					ran = true
				}
			}
			if !ran {
				fail("saved log path missing from palette commands")
			} else if w.activeSavedLog != "Production Connectors" {
				fail("running a saved log from the palette did not open it (activeSavedLog=%q)", w.activeSavedLog)
			}
			for _, section := range w.moduleSections {
				if section.name == "CloudWatch Logs" && !section.expander.Expanded() {
					fail("the saved log's rail section should expand to show where it went")
				}
			}

			app.ActivateAction("command-palette", nil)
			p := w.palette
			if p == nil {
				fail("Ctrl+Shift+P action did not open the palette")
				app.Quit()
				return false
			}
			// The saved log just ran: it is repeated in Recent, which follows
			// This view, and keeps its own row under Saved.
			var order []command.Section
			savedRows := map[command.Section]bool{}
			for _, r := range p.results {
				if len(order) == 0 || order[len(order)-1] != r.Section {
					order = append(order, r.Section)
				}
				if r.Title == "Production Connectors" {
					savedRows[r.Section] = true
				}
			}
			if len(order) < 2 || order[0] != command.SectionView || order[1] != command.SectionRecent {
				fail("browse section order = %v, want This view then Recent", order)
			}
			if !savedRows[command.SectionRecent] || !savedRows[command.SectionSaved] {
				fail("recent saved log should appear under Recent and Saved, got %v", savedRows)
			}
			byID := map[string]command.Result{}
			for _, r := range p.results {
				byID[r.ID] = r
			}
			for _, id := range []string{"app.refresh", "app.settings", "mode.ecs", "mode.sqs", "app.pause"} {
				if r, ok := byID[id]; !ok || !r.Available() {
					fail("browse view missing available %s (%+v)", id, r)
				}
			}
			if r, ok := byID["ecs.force-redeploy"]; ok {
				fail("force deploy should be hidden in browse view while unavailable: %+v", r)
			}
			if r := byID["app.refresh"]; r.Key == "" {
				fail("refresh should show its accelerator")
			}
			if r, ok := byID["saved.ssm.saved-prefixes.prod-db"]; !ok || r.Section != command.SectionSaved {
				fail("saved SSM prefix missing from Saved section: %+v (have %v)", r, keys(byID))
			}

			// Ctrl+P (the "modes" action) opens the go-to subset.
			p.close()
			app.ActivateAction("modes", nil)
			if w.palette == nil {
				fail("Ctrl+P did not open the go-to palette")
				app.Quit()
				return false
			}
			for _, r := range w.palette.results {
				switch r.Section {
				case command.SectionGoTo, command.SectionSaved, command.SectionPlugins, command.SectionRecent:
				default:
					fail("go-to palette listed %s from section %v", r.ID, r.Section)
				}
			}
			w.palette.close()
			app.ActivateAction("command-palette", nil)
			p = w.palette

			p.entry.SetText("force deploy")
			p.refilter()
			if len(p.results) == 0 || p.results[0].ID != "ecs.force-redeploy" || p.results[0].Available() {
				fail("precise search should surface the unavailable force deploy, got %+v", p.results)
			}
			if c := p.commands["ecs.force-redeploy"]; c.Key == "" || !c.Danger {
				fail("force deploy should carry its shortcut and danger flag: %+v", c.Entry)
			}

			p.entry.SetText("sqs")
			p.refilter()
			if len(p.results) == 0 || p.results[0].ID != "mode.sqs" {
				fail("sqs should rank Go to SQS first, got %+v", p.results)
			}
			// E9S_GUI_PALETTE_HOLD_MS keeps the palette on screen, first
			// browsing and then searching, so a broadway session can be
			// inspected or screenshotted.
			hold := uint(0)
			if ms, err := strconv.Atoi(os.Getenv("E9S_GUI_PALETTE_HOLD_MS")); err == nil && ms > 0 {
				hold = uint(ms)
			}
			p.entry.SetText("")
			p.refilter()
			glib.TimeoutAdd(hold+1, func() bool {
				p.entry.SetText("dep")
				p.refilter()
				glib.TimeoutAdd(hold+1, func() bool {
					defer app.Quit()
					p.entry.SetText("go to sqs")
					p.refilter()
					p.runSelected()
					if w.palette != nil {
						fail("running a command should close the palette")
					}
					return false
				})
				return false
			})
			return false
		})
	})
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			glib.IdleAdd(app.Quit)
		}
	}()
	app.Run(nil)
	close(done)
	for _, f := range failures {
		t.Error(f)
	}
}

func keys(m map[string]command.Result) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
