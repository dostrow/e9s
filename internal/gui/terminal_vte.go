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
	C.e9s_vte_terminal_reset((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
	C.e9s_vte_terminal_spawn(
		(*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())),
		(**C.char)(native),
	)
	return nil
}

func (terminal *vteTerminal) Stop() {
	C.e9s_vte_terminal_stop((*C.GtkWidget)(unsafe.Pointer(coreglib.BaseObject(terminal.widget).Native())))
}

func vteAvailable() bool { return true }
