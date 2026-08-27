//go:build gui && windows

package gui

/*
#cgo pkg-config: pangocairo
#include <stdlib.h>
#include <glib.h>
#include <pango/pangocairo.h>

static char *e9s_pango_add_font_file(const char *path) {
    GError *error = NULL;
    PangoFontMap *font_map = pango_cairo_font_map_get_default();
    if (pango_font_map_add_font_file(font_map, path, &error)) {
        return NULL;
    }
    char *message = g_strdup(error == NULL ? "unknown Pango font error" : error->message);
    g_clear_error(&error);
    return message;
}

static void e9s_pango_free_error(char *message) {
    g_free(message);
}
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"unsafe"
)

func registerBundledFonts() (string, error) {
	directory := bundledFontDirectory()
	if directory == "" {
		return "", nil
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.ttf"))
	if err != nil {
		return "", fmt.Errorf("find bundled fonts in %s: %w", directory, err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no bundled TrueType fonts found in %s", directory)
	}
	for _, file := range files {
		path := C.CString(file)
		message := C.e9s_pango_add_font_file(path)
		C.free(unsafe.Pointer(path))
		if message != nil {
			detail := C.GoString(message)
			C.e9s_pango_free_error(message)
			return "", fmt.Errorf("register bundled font %s: %s", filepath.Base(file), detail)
		}
	}
	return directory, nil
}
