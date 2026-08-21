//go:build gui && sourceview

package gui

/*
#cgo pkg-config: gtksourceview-5
#include <stdlib.h>
#include "sourceview_bridge.h"
*/
import "C"

import (
	"unsafe"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newSourceEditor(document sourceDocument) *sourceEditor {
	native := C.e9s_source_editor_new()
	object := coreglib.Take(unsafe.Pointer(native))
	cast := object.WalkCast(func(candidate coreglib.Objector) bool {
		_, ok := candidate.(*gtk.TextView)
		return ok
	})
	view, ok := cast.(*gtk.TextView)
	if !ok {
		panic("GtkSourceView did not inherit GtkTextView")
	}
	configurePlainSourceView(view)
	buffer := view.Buffer()

	setDocument := func(document sourceDocument) {
		path := C.CString(document.Path)
		defer C.free(unsafe.Pointer(path))
		language := C.CString(document.Language)
		defer C.free(unsafe.Pointer(language))
		C.e9s_source_editor_set_document(
			C.uintptr_t(coreglib.BaseObject(view).Native()),
			path,
			language,
		)
	}
	setDocument(document)

	applyPalette := func(palette semanticPalette) {
		colors := sourceEditorPalette(palette)
		cColors := make([]*C.char, len(colors))
		for i, color := range colors {
			cColors[i] = C.CString(color)
			defer C.free(unsafe.Pointer(cColors[i]))
		}
		C.e9s_source_editor_set_palette(
			C.uintptr_t(coreglib.BaseObject(view).Native()),
			cColors[0], cColors[1], cColors[2], cColors[3], cColors[4], cColors[5],
			cColors[6], cColors[7], cColors[8], cColors[9], cColors[10],
		)
	}

	return &sourceEditor{
		buffer:       buffer,
		view:         view,
		setDocument:  setDocument,
		applyPalette: applyPalette,
	}
}

func sourceViewLanguageID(document sourceDocument) string {
	path := C.CString(document.Path)
	defer C.free(unsafe.Pointer(path))
	language := C.CString(document.Language)
	defer C.free(unsafe.Pointer(language))
	id := C.e9s_source_language_id(path, language)
	if id == nil {
		return ""
	}
	return C.GoString(id)
}
