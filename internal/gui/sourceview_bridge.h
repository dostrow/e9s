//go:build gui && sourceview

#pragma once

#include <gtk/gtk.h>
#include <stdint.h>

void e9s_source_init(void);
GtkWidget *e9s_source_editor_new(void);
void e9s_source_editor_set_document(uintptr_t widget_pointer, const char *path, const char *language_id);
void e9s_source_editor_set_palette(
    uintptr_t widget_pointer,
    const char *foreground,
    const char *background,
    const char *accent,
    const char *success,
    const char *warning,
    const char *error,
    const char *info,
    const char *muted,
    const char *selection,
    const char *current_line,
    const char *gutter
);
const char *e9s_source_language_id(const char *path, const char *language_id);
