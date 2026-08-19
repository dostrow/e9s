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

	applyTheme := func() {
		dark := darkEditorBackground(&view.Widget)
		C.e9s_source_editor_set_dark(
			C.uintptr_t(coreglib.BaseObject(view).Native()),
			C.gboolean(boolToGBoolean(dark)),
		)
	}
	view.ConnectMap(applyTheme)
	if settings := gtk.SettingsGetDefault(); settings != nil {
		settings.NotifyProperty("gtk-theme-name", applyTheme)
		settings.NotifyProperty("gtk-application-prefer-dark-theme", applyTheme)
	}

	return &sourceEditor{
		buffer:      buffer,
		view:        view,
		setDocument: setDocument,
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

func boolToGBoolean(value bool) int {
	if value {
		return 1
	}
	return 0
}
