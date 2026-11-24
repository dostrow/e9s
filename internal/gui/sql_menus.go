//go:build gui

package gui

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/graphene"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

// Opening a query tab and browsing a database's objects are separate
// actions: double-clicking a connection only opens a tab, and the object
// browser is reached from a connection's or a tab's right-click menu, or
// from the toolbar.

// sqlMenu is a popover of flat buttons, parented once to a long-lived
// widget so it outlives the rows and tab labels that open it.
type sqlMenu struct {
	popover *gtk.Popover
	box     *gtk.Box
	items   map[string]*gtk.Button
}

func newSQLMenu(parent gtk.Widgetter) *sqlMenu {
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	box.SetMarginTop(6)
	box.SetMarginBottom(6)
	box.SetMarginStart(6)
	box.SetMarginEnd(6)
	popover := gtk.NewPopover()
	popover.AddCSSClass("menu")
	popover.SetHasArrow(false)
	popover.SetChild(box)
	popover.SetParent(parent)
	return &sqlMenu{popover: popover, box: box, items: map[string]*gtk.Button{}}
}

func (m *sqlMenu) add(label, tooltip string, action func()) *gtk.Button {
	text := gtk.NewLabel(label)
	text.SetXAlign(0)
	button := gtk.NewButton()
	button.SetChild(text)
	button.AddCSSClass("flat")
	button.SetHAlign(gtk.AlignFill)
	button.SetTooltipText(tooltip)
	button.ConnectClicked(func() {
		m.popover.Popdown()
		action()
	})
	m.box.Append(button)
	m.items[label] = button
	return button
}

func (m *sqlMenu) popupAt(x, y float64) {
	rectangle := gdk.NewRectangle(int(x), int(y), 1, 1)
	m.popover.SetPointingTo(&rectangle)
	m.popover.Popup()
}

// translatePoint converts a point in from's coordinates to to's, or returns
// it unchanged when the widgets share no common ancestor.
func translatePoint(from, to gtk.Widgetter, x, y float64) (float64, float64) {
	point, ok := gtk.BaseWidget(from).ComputePoint(to, graphene.NewPointAlloc().Init(float32(x), float32(y)))
	if !ok || point == nil {
		return x, y
	}
	return float64(point.X()), float64(point.Y())
}

// isContextMenuKey reports the keys that open a context menu from the
// keyboard: the Menu key and Shift+F10.
func isContextMenuKey(keyval uint, state gdk.ModifierType) bool {
	return keyval == gdk.KEY_Menu || (keyval == gdk.KEY_F10 && state&gdk.ShiftMask != 0)
}

// installSQLProfileContextMenu gives each saved connection a right-click
// menu for opening a query tab or browsing its objects.
func (w *mainWindow) installSQLProfileContextMenu() {
	table := w.sqlProfileTable
	if table == nil {
		return
	}
	menu := newSQLMenu(table.view)
	withTarget := func(action func(config.SQLConnection)) func() {
		return func() {
			if profile, found := w.sqlProfileNamed(w.sqlProfileMenuTarget); found {
				action(profile)
			}
		}
	}
	menu.add("Open query tab", "Open a new query tab on this connection", withTarget(func(profile config.SQLConnection) {
		w.openSQLTab(profile, sqlworkbench.TabState{})
	}))
	menu.add("Browse database objects", "Show this connection's schemas, tables and functions in the Browser Pane", withTarget(w.browseSQLObjectsFor))
	menu.add("Manage connections…", "Add, edit or remove saved connections", w.manageSQLConnections)
	open := func(position uint, x, y float64) {
		if int(position) >= len(w.filteredSQLProfiles) {
			return
		}
		w.sqlProfileMenuTarget = w.filteredSQLProfiles[position].Name
		menu.popupAt(x, y)
	}
	table.contextMenu = open
	w.sqlProfileMenu = menu

	keys := gtk.NewEventControllerKey()
	keys.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if !isContextMenuKey(keyval, state) {
			return false
		}
		position := table.selection.Selected()
		if position == gtk.InvalidListPosition {
			return false
		}
		open(position, 24, 24)
		return true
	})
	table.view.AddController(keys)
}

// installSQLTabContextMenu gives a query tab's label a right-click menu.
// One menu, parented to the notebook, serves every tab.
func (w *mainWindow) installSQLTabContextMenu() {
	if w.sqlNotebook == nil {
		return
	}
	menu := newSQLMenu(w.sqlNotebook)
	act := func(action func(*sqlWorkbenchTab)) func() {
		return func() {
			if tab := w.sqlTabMenuTarget; tab != nil && w.sqlTabIndex(tab) >= 0 {
				action(tab)
			}
		}
	}
	menu.add("Browse database objects", "Show this tab's database objects in the Browser Pane", act(func(tab *sqlWorkbenchTab) {
		w.activateSQLTab(tab)
		w.showSQLObjectBrowser(tab)
	}))
	menu.add("Rename tab…", "Rename this query tab", act(func(tab *sqlWorkbenchTab) {
		w.activateSQLTab(tab)
		w.promptRenameSQLTab()
	}))
	menu.add("Close tab", "Close this query tab and its connection", act(w.closeSQLTab))
	w.sqlTabMenu = menu
}

// attachSQLTabMenu opens the shared tab menu when label is right-clicked.
func (w *mainWindow) attachSQLTabMenu(tab *sqlWorkbenchTab, label gtk.Widgetter) {
	click := gtk.NewGestureClick()
	click.SetButton(gdk.BUTTON_SECONDARY)
	click.ConnectPressed(func(_ int, x, y float64) {
		if w.sqlTabMenu == nil {
			return
		}
		w.sqlTabMenuTarget = tab
		x, y = translatePoint(label, w.sqlNotebook, x, y)
		w.sqlTabMenu.popupAt(x, y)
	})
	gtk.BaseWidget(label).AddController(click)
}

func (w *mainWindow) sqlTabIndex(tab *sqlWorkbenchTab) int {
	for index, existing := range w.sqlTabs {
		if existing == tab {
			return index
		}
	}
	return -1
}

func (w *mainWindow) activateSQLTab(tab *sqlWorkbenchTab) {
	if index := w.sqlTabIndex(tab); index >= 0 && w.sqlNotebook.CurrentPage() != index {
		w.sqlNotebook.SetCurrentPage(index)
	}
}

// browseActiveSQLObjects shows the active tab's object browser.
func (w *mainWindow) browseActiveSQLObjects() {
	tab := w.activeSQLTab()
	if tab == nil {
		w.setStatus("Open a query tab first; right-click a connection to browse its objects", true)
		return
	}
	w.showSQLObjectBrowser(tab)
}

// browseSQLObjectsFor shows the object browser for a connection. The
// browser belongs to a query tab, because inserting names and previewing
// rows act on that tab, so it uses the tab chosen by chooseSQLBrowserTab
// and opens one only when the connection has none.
func (w *mainWindow) browseSQLObjectsFor(profile config.SQLConnection) {
	candidates := make([]sqlBrowserCandidate, len(w.sqlTabs))
	for index, tab := range w.sqlTabs {
		candidates[index] = sqlBrowserCandidate{profile: tab.state.ProfileName, lastUsed: tab.lastActive}
	}
	active := -1
	if w.sqlNotebook != nil {
		active = w.sqlNotebook.CurrentPage()
	}
	if index := chooseSQLBrowserTab(candidates, active, profile.Name); index >= 0 {
		tab := w.sqlTabs[index]
		w.activateSQLTab(tab)
		w.showSQLObjectBrowser(tab)
		return
	}
	w.openSQLTab(profile, sqlworkbench.TabState{})
	w.showSQLObjectBrowser(w.activeSQLTab())
}

type sqlBrowserCandidate struct {
	profile  string
	lastUsed time.Time
}

// chooseSQLBrowserTab picks the tab whose object browser to show for
// profile: the active tab when it is on that connection, otherwise the
// connection's most recently used tab. It returns -1 when the connection
// has no tab.
func chooseSQLBrowserTab(tabs []sqlBrowserCandidate, active int, profile string) int {
	if active >= 0 && active < len(tabs) && tabs[active].profile == profile {
		return active
	}
	chosen := -1
	for index, tab := range tabs {
		if tab.profile != profile {
			continue
		}
		if chosen < 0 || tab.lastUsed.After(tabs[chosen].lastUsed) {
			chosen = index
		}
	}
	return chosen
}
