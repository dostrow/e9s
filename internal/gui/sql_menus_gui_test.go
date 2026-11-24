//go:build gui

package gui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
)

// TestSQLObjectBrowserOpensOnlyWhenAsked builds the main window with two
// saved connections and checks that opening a query tab leaves the
// connection list in place, and that the right-click menus, the toolbar
// button and the command palette reach the object browser. Like the other
// window tests it only runs when E9S_GUI_WINDOW_TESTS is set.
func TestSQLObjectBrowserOpensOnlyWhenAsked(t *testing.T) {
	if os.Getenv("E9S_GUI_WINDOW_TESTS") == "" {
		t.Skip("set E9S_GUI_WINDOW_TESTS=1 to run tests that map windows")
	}
	if !gtk.InitCheck() {
		t.Skip("GTK display unavailable")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	app := gtk.NewApplication("io.github.dostrow.e9s.sqlmenutest", gio.ApplicationNonUnique)
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, strings.TrimSpace(fmt.Sprintf(format, args...)))
	}
	app.ConnectActivate(func() {
		installStyles()
		// Port 1 refuses connections at once, so catalog loads fail fast.
		cfg := &config.Config{SQL: config.SQLConfig{Connections: []config.SQLConnection{
			{Name: "prod", Host: "127.0.0.1", Port: 1, Database: "app", User: "reader", Auth: "password"},
			{Name: "staging", Host: "127.0.0.1", Port: 1, Database: "app", User: "reader", Auth: "password"},
		}}}
		w := newMainWindow(context.Background(), app, Options{Config: cfg})
		installSemanticStyles(w)
		w.window.SetDefaultSize(1280, 800)
		w.window.Present()

		glib.TimeoutAdd(300, func() bool {
			defer app.Quit()
			w.openSQLWorkbenchModule()
			if len(w.filteredSQLProfiles) != 2 {
				fail("expected two connections, got %d", len(w.filteredSQLProfiles))
				return false
			}

			// Double-click: a tab, and the connection list stays.
			w.openSQLProfileAt(0)
			if len(w.sqlTabs) != 1 || w.sqlTabs[0].state.ProfileName != "prod" {
				fail("double-click did not open one prod tab: %d tab(s)", len(w.sqlTabs))
				return false
			}
			if w.currentPage != pageSQLConnections {
				fail("double-click switched the Browser Pane to %q", w.currentPage)
			}
			if !w.sqlBrowseButton.Sensitive() {
				fail("Browse objects should be enabled with a tab open")
			}
			browseCommand := false
			for _, c := range w.paletteCommands() {
				if c.Title == "Browse objects" {
					browseCommand = true
				}
			}
			if !browseCommand {
				fail("the command palette lacks Browse objects")
			}

			// Right-click the prod row and browse: the existing tab is used.
			w.sqlProfileTable.contextMenu(0, 10, 10)
			if w.sqlProfileMenuTarget != "prod" || !w.sqlProfileMenu.popover.Visible() {
				fail("right-click did not open the menu for prod (target %q)", w.sqlProfileMenuTarget)
			}
			w.sqlProfileMenu.items["Browse database objects"].Emit("clicked")
			if w.sqlProfileMenu.popover.Visible() {
				fail("the menu stayed open after choosing an item")
			}
			if w.currentPage != pageSQLObjects || w.sqlBrowserTab != w.sqlTabs[0] || len(w.sqlTabs) != 1 {
				fail("browsing prod: page %q, %d tab(s), browser on the prod tab %v", w.currentPage, len(w.sqlTabs), w.sqlBrowserTab == w.sqlTabs[0])
			}

			// Back, then browse staging, which has no tab: one is opened.
			w.openSQLWorkbenchModule()
			w.sqlProfileTable.contextMenu(1, 10, 10)
			w.sqlProfileMenu.items["Browse database objects"].Emit("clicked")
			if len(w.sqlTabs) != 2 || w.activeSQLTab().state.ProfileName != "staging" || w.currentPage != pageSQLObjects || w.sqlBrowserTab != w.activeSQLTab() {
				fail("browsing staging without a tab: %d tab(s), page %q, menu target %q", len(w.sqlTabs), w.currentPage, w.sqlProfileMenuTarget)
				return false
			}

			// The tab menu browses its own tab, even when it is not active.
			w.openSQLWorkbenchModule()
			w.sqlTabMenuTarget = w.sqlTabs[0]
			w.sqlTabMenu.items["Browse database objects"].Emit("clicked")
			if w.activeSQLTab() != w.sqlTabs[0] || w.sqlBrowserTab != w.sqlTabs[0] || w.currentPage != pageSQLObjects {
				fail("the tab menu did not browse the prod tab")
			}

			// The toolbar button browses the active tab.
			w.openSQLWorkbenchModule()
			w.sqlBrowseButton.Emit("clicked")
			if w.currentPage != pageSQLObjects || w.sqlBrowserTab != w.activeSQLTab() {
				fail("Browse objects did not show the active tab's objects")
			}

			// The tab menu's Close closes that tab.
			w.sqlTabMenuTarget = w.sqlTabs[1]
			w.sqlTabMenu.items["Close tab"].Emit("clicked")
			if len(w.sqlTabs) != 1 || w.sqlTabs[0].state.ProfileName != "prod" {
				fail("Close tab closed the wrong tab: %d left", len(w.sqlTabs))
			}
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
