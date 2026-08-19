//go:build gui && sourceview

#include "sourceview_bridge.h"

#include <gtksourceview/gtksource.h>

void e9s_source_init(void) {
    gtk_source_init();
}

static GtkSourceLanguage *e9s_source_language(const char *path, const char *language_id) {
    GtkSourceLanguageManager *manager = gtk_source_language_manager_get_default();
    GtkSourceLanguage *language = NULL;
    if (language_id != NULL && language_id[0] != '\0') {
        language = gtk_source_language_manager_get_language(manager, language_id);
    }
    if (language == NULL && path != NULL && path[0] != '\0') {
        language = gtk_source_language_manager_guess_language(manager, path, NULL);
    }
    return language;
}

GtkWidget *e9s_source_editor_new(void) {
    GtkSourceBuffer *buffer = gtk_source_buffer_new(NULL);
    gtk_source_buffer_set_highlight_syntax(buffer, TRUE);
    gtk_source_buffer_set_highlight_matching_brackets(buffer, TRUE);

    GtkWidget *widget = gtk_source_view_new_with_buffer(buffer);
    g_object_unref(buffer);

    GtkSourceView *view = GTK_SOURCE_VIEW(widget);
    gtk_source_view_set_show_line_numbers(view, TRUE);
    gtk_source_view_set_highlight_current_line(view, TRUE);
    gtk_source_view_set_auto_indent(view, TRUE);
    gtk_source_view_set_insert_spaces_instead_of_tabs(view, TRUE);
    gtk_source_view_set_tab_width(view, 4);
    return widget;
}

void e9s_source_editor_set_document(uintptr_t widget_pointer, const char *path, const char *language_id) {
    GtkWidget *widget = (GtkWidget *)widget_pointer;
    GtkTextBuffer *text_buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(widget));
    gtk_source_buffer_set_language(
        GTK_SOURCE_BUFFER(text_buffer),
        e9s_source_language(path, language_id)
    );
}

void e9s_source_editor_set_dark(uintptr_t widget_pointer, gboolean dark) {
    GtkWidget *widget = (GtkWidget *)widget_pointer;
    GtkSourceStyleSchemeManager *manager = gtk_source_style_scheme_manager_get_default();
    const char *scheme_id = dark ? "Adwaita-dark" : "Adwaita";
    GtkSourceStyleScheme *scheme = gtk_source_style_scheme_manager_get_scheme(manager, scheme_id);
    if (scheme != NULL) {
        GtkTextBuffer *text_buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(widget));
        gtk_source_buffer_set_style_scheme(GTK_SOURCE_BUFFER(text_buffer), scheme);
    }
}

const char *e9s_source_language_id(const char *path, const char *language_id) {
    GtkSourceLanguage *language = e9s_source_language(path, language_id);
    return language == NULL ? NULL : gtk_source_language_get_id(language);
}
