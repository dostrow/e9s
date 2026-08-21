//go:build gui && vte

package gui

/*
#cgo pkg-config: vte-2.91-gtk4
#include <stdlib.h>
#include "vte_bridge.h"
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type vteTerminal struct {
	widget      *gtk.Widget
	contextMenu *gtk.Popover
	contextCopy *gtk.Button
}

func newVTETerminal() *vteTerminal {
	native := C.e9s_vte_terminal_new()
	object := coreglib.Take(unsafe.Pointer(native))
	cast := object.WalkCast(func(candidate coreglib.Objector) bool {
		_, ok := candidate.(*gtk.Widget)
		return ok
	})
	widget, ok := cast.(*gtk.Widget)
	if !ok {
		panic("VTE terminal did not inherit GtkWidget")
	}
	widget.SetHExpand(true)
	widget.SetVExpand(true)
	terminal := &vteTerminal{widget: widget}
	terminal.installContextMenu()
	return terminal
}

func (terminal *vteTerminal) Widget() gtk.Widgetter {
	return terminal.widget
}

func (terminal *vteTerminal) Spawn(executable string, args []string) error {
	return terminal.SpawnInDirectory(executable, args, "")
}

func (terminal *vteTerminal) SpawnInDirectory(executable string, args []string, workingDirectory string) error {
	if executable == "" {
		return fmt.Errorf("terminal command is empty")
	}
	argv := append([]string{executable}, args...)
	native := C.malloc(C.size_t(len(argv)+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
	if native == nil {
		return fmt.Errorf("allocate terminal argument vector")
	}
	defer C.free(native)
	items := unsafe.Slice((**C.char)(native), len(argv)+1)
	for i, argument := range argv {
		items[i] = C.CString(argument)
		defer C.free(unsafe.Pointer(items[i]))
	}
	items[len(argv)] = nil
	var directory *C.char
	if workingDirectory != "" {
		directory = C.CString(workingDirectory)
		defer C.free(unsafe.Pointer(directory))
	}
	C.e9s_vte_terminal_reset((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
	C.e9s_vte_terminal_spawn(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		(**C.char)(native),
		directory,
	)
	return nil
}

func (terminal *vteTerminal) GrabFocus() { terminal.widget.GrabFocus() }

func (terminal *vteTerminal) HasFocus() bool { return terminal.widget.HasFocus() }

func (terminal *vteTerminal) CopyClipboard() {
	C.e9s_vte_terminal_copy_clipboard((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func (terminal *vteTerminal) PasteClipboard() {
	C.e9s_vte_terminal_paste_clipboard((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func (terminal *vteTerminal) HasSelection() bool {
	return C.e9s_vte_terminal_has_selection((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native()))) != 0
}

func (terminal *vteTerminal) SelectAll() {
	C.e9s_vte_terminal_select_all((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func (terminal *vteTerminal) installContextMenu() {
	menu := gtk.NewBox(gtk.OrientationVertical, 2)
	menu.SetMarginTop(6)
	menu.SetMarginBottom(6)
	menu.SetMarginStart(6)
	menu.SetMarginEnd(6)

	copyButton := gtk.NewButtonWithLabel("Copy")
	copyButton.AddCSSClass("flat")
	copyButton.SetHAlign(gtk.AlignFill)
	copyButton.ConnectClicked(func() {
		terminal.CopyClipboard()
		terminal.contextMenu.Popdown()
	})
	menu.Append(copyButton)

	pasteButton := gtk.NewButtonWithLabel("Paste")
	pasteButton.AddCSSClass("flat")
	pasteButton.SetHAlign(gtk.AlignFill)
	pasteButton.ConnectClicked(func() {
		terminal.PasteClipboard()
		terminal.contextMenu.Popdown()
	})
	menu.Append(pasteButton)

	selectAllButton := gtk.NewButtonWithLabel("Select All")
	selectAllButton.AddCSSClass("flat")
	selectAllButton.SetHAlign(gtk.AlignFill)
	selectAllButton.ConnectClicked(func() {
		terminal.SelectAll()
		terminal.contextMenu.Popdown()
	})
	menu.Append(selectAllButton)

	popover := gtk.NewPopover()
	popover.AddCSSClass("menu")
	popover.SetChild(menu)
	popover.SetParent(terminal.widget)
	terminal.contextMenu = popover
	terminal.contextCopy = copyButton

	click := gtk.NewGestureClick()
	click.SetButton(3)
	click.SetPropagationPhase(gtk.PhaseCapture)
	click.ConnectPressed(func(_ int, x, y float64) {
		terminal.GrabFocus()
		terminal.contextCopy.SetSensitive(terminal.HasSelection())
		rectangle := gdk.NewRectangle(int(x), int(y), 1, 1)
		terminal.contextMenu.SetPointingTo(&rectangle)
		terminal.contextMenu.Popup()
	})
	terminal.widget.AddController(click)
}

func (terminal *vteTerminal) Stop() {
	C.e9s_vte_terminal_stop((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func (terminal *vteTerminal) Running() bool {
	return C.e9s_vte_terminal_running((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native()))) != 0
}

func (terminal *vteTerminal) ExitStatus() int {
	return int(C.e9s_vte_terminal_exit_status((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native()))))
}

func (terminal *vteTerminal) WindowTitle() string {
	title := C.e9s_vte_terminal_window_title((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
	if title == nil {
		return ""
	}
	return strings.TrimSpace(C.GoString(title))
}

func (terminal *vteTerminal) CurrentDirectory() string {
	uri := C.e9s_vte_terminal_current_directory_uri((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
	if uri == nil {
		return ""
	}
	return terminalDirectoryFromURI(C.GoString(uri))
}

func (terminal *vteTerminal) ConnectWindowTitleChanged(f func()) {
	terminal.widget.NotifyProperty("window-title", f)
}

func (terminal *vteTerminal) SetFontScale(scale float64) {
	C.e9s_vte_terminal_set_font_scale(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		C.double(scale),
	)
}

func (terminal *vteTerminal) SetFont(description string) {
	font := C.CString(description)
	defer C.free(unsafe.Pointer(font))
	C.e9s_vte_terminal_set_font(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		font,
	)
}

func (terminal *vteTerminal) SetScrollbackLines(lines int) {
	C.e9s_vte_terminal_set_scrollback_lines(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		C.long(lines),
	)
}

func (terminal *vteTerminal) SetPalette(palette semanticPalette) {
	colors := []string{
		rgbaHex(palette.foreground), rgbaHex(palette.surface), rgbaHex(palette.accent),
		rgbaHex(palette.success), rgbaHex(palette.warning), rgbaHex(palette.error),
		rgbaHex(palette.info), rgbaHex(palette.muted),
	}
	cColors := make([]*C.char, len(colors))
	for i, color := range colors {
		cColors[i] = C.CString(color)
		defer C.free(unsafe.Pointer(cColors[i]))
	}
	C.e9s_vte_terminal_set_palette(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		cColors[0], cColors[1], cColors[2], cColors[3], cColors[4], cColors[5], cColors[6], cColors[7],
	)
}

func vteAvailable() bool { return true }
