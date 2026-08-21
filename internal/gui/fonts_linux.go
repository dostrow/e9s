//go:build gui && linux

package gui

/*
#cgo pkg-config: fontconfig pangocairo pangoft2
#include <stdlib.h>
#include <fontconfig/fontconfig.h>
#include <pango/pangocairo.h>
#include <pango/pangofc-fontmap.h>

static int e9s_register_font_directory(const char *directory) {
    if (!FcConfigAppFontAddDir(NULL, (const FcChar8 *)directory)) {
        return 0;
    }
    FcConfigBuildFonts(NULL);
    PangoFontMap *font_map = pango_cairo_font_map_get_default();
    if (PANGO_IS_FC_FONT_MAP(font_map)) {
        pango_fc_font_map_config_changed(PANGO_FC_FONT_MAP(font_map));
    }
    return 1;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func registerBundledFonts() (string, error) {
	directory := bundledFontDirectory()
	if directory == "" {
		return "", nil
	}
	native := C.CString(directory)
	defer C.free(unsafe.Pointer(native))
	if C.e9s_register_font_directory(native) == 0 {
		return "", fmt.Errorf("register bundled fonts from %s", directory)
	}
	return directory, nil
}
