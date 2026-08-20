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
	"unsafe"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type vteTerminal struct {
	widget *gtk.Widget
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
	return &vteTerminal{widget: widget}
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

func (terminal *vteTerminal) Stop() {
	C.e9s_vte_terminal_stop((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func (terminal *vteTerminal) Running() bool {
	return C.e9s_vte_terminal_running((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native()))) != 0
}

func (terminal *vteTerminal) ExitStatus() int {
	return int(C.e9s_vte_terminal_exit_status((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native()))))
}

func (terminal *vteTerminal) SetFontScale(scale float64) {
	C.e9s_vte_terminal_set_font_scale(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		C.double(scale),
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
