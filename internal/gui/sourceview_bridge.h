//go:build gui && sourceview

#pragma once

#include <gtk/gtk.h>
#include <stdint.h>

void e9s_source_init(void);
GtkWidget *e9s_source_editor_new(void);
void e9s_source_editor_set_document(uintptr_t widget_pointer, const char *path, const char *language_id);
void e9s_source_editor_set_dark(uintptr_t widget_pointer, gboolean dark);
const char *e9s_source_language_id(const char *path, const char *language_id);
