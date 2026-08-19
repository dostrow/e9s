//go:build gui && sourceview

package gui

/*
#cgo pkg-config: gtksourceview-5
#include "sourceview_bridge.h"
*/
import "C"

func initializeSourceEditor() {
	C.e9s_source_init()
}

func sourceEditorAvailable() bool { return true }
