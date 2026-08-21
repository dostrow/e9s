//go:build gui && sourceview

#include "sourceview_bridge.h"

#include <gtksourceview/gtksource.h>
#include <glib/gstdio.h>

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

void e9s_source_editor_set_palette(
    uintptr_t widget_pointer,
    const char *foreground,
    const char *background,
    const char *accent,
    const char *success,
    const char *warning,
    const char *error_color,
    const char *info,
    const char *muted,
    const char *selection,
    const char *current_line,
    const char *gutter
) {
    static char *scheme_directory = NULL;
    static char *previous_path = NULL;
    static guint64 generation = 0;
    static gboolean search_path_installed = FALSE;
    GtkWidget *widget = (GtkWidget *)widget_pointer;
    if (scheme_directory == NULL) {
        scheme_directory = g_dir_make_tmp("e9s-sourceview-XXXXXX", NULL);
    }
    if (scheme_directory == NULL) {
        return;
    }

    generation++;
    char *scheme_id = g_strdup_printf("e9s-gtk-theme-%" G_GUINT64_FORMAT, generation);
    char *contents = g_strdup_printf(
        "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
        "<style-scheme id=\"%s\" name=\"e9s GTK Theme\" version=\"1.0\">\n"
        "  <style name=\"text\" foreground=\"%s\" background=\"%s\"/>\n"
        "  <style name=\"selection\" foreground=\"%s\" background=\"%s\"/>\n"
        "  <style name=\"cursor\" foreground=\"%s\"/>\n"
        "  <style name=\"secondary-cursor\" foreground=\"%s\"/>\n"
        "  <style name=\"current-line\" background=\"%s\"/>\n"
        "  <style name=\"line-numbers\" foreground=\"%s\" background=\"%s\"/>\n"
        "  <style name=\"current-line-number\" foreground=\"%s\" background=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"bracket-match\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"bracket-mismatch\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"search-match\" foreground=\"%s\" background=\"%s\"/>\n"
        "  <style name=\"def:comment\" foreground=\"%s\" italic=\"true\"/>\n"
        "  <style name=\"def:shebang\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:constant\" foreground=\"%s\"/>\n"
        "  <style name=\"def:string\" foreground=\"%s\"/>\n"
        "  <style name=\"def:special-char\" foreground=\"%s\"/>\n"
        "  <style name=\"def:identifier\" foreground=\"%s\"/>\n"
        "  <style name=\"def:statement\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:type\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:preprocessor\" foreground=\"%s\"/>\n"
        "  <style name=\"def:error\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:warning\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:note\" foreground=\"%s\"/>\n"
        "  <style name=\"def:heading\" foreground=\"%s\" bold=\"true\"/>\n"
        "  <style name=\"def:link-text\" foreground=\"%s\" underline=\"single\"/>\n"
        "  <style name=\"diff:added-line\" foreground=\"%s\"/>\n"
        "  <style name=\"diff:removed-line\" foreground=\"%s\"/>\n"
        "  <style name=\"diff:changed-line\" foreground=\"%s\"/>\n"
        "</style-scheme>\n",
        scheme_id,
        foreground, background,
        foreground, selection,
        accent, muted,
        current_line,
        muted, gutter,
        accent, current_line,
        accent,
        error_color,
        foreground, warning,
        muted,
        muted,
        warning,
        success,
        warning,
        info,
        accent,
        success,
        accent,
        error_color,
        warning,
        info,
        success,
        info,
        success,
        error_color,
        warning
    );
    char *scheme_path = g_build_filename(scheme_directory, scheme_id, NULL);
    char *scheme_file = g_strconcat(scheme_path, ".xml", NULL);
    g_free(scheme_path);
    if (!g_file_set_contents(scheme_file, contents, -1, NULL)) {
        g_free(scheme_file);
        g_free(contents);
        g_free(scheme_id);
        return;
    }

    GtkSourceStyleSchemeManager *manager = gtk_source_style_scheme_manager_get_default();
    if (!search_path_installed) {
        gtk_source_style_scheme_manager_append_search_path(manager, scheme_directory);
        search_path_installed = TRUE;
    }
    gtk_source_style_scheme_manager_force_rescan(manager);
    GtkSourceStyleScheme *scheme = gtk_source_style_scheme_manager_get_scheme(manager, scheme_id);
    if (scheme != NULL) {
        GtkTextBuffer *text_buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(widget));
        gtk_source_buffer_set_style_scheme(GTK_SOURCE_BUFFER(text_buffer), scheme);
    }
    if (previous_path != NULL) {
        g_remove(previous_path);
        g_free(previous_path);
    }
    previous_path = scheme_file;
    g_free(contents);
    g_free(scheme_id);
}

const char *e9s_source_language_id(const char *path, const char *language_id) {
    GtkSourceLanguage *language = e9s_source_language(path, language_id);
    return language == NULL ? NULL : gtk_source_language_get_id(language);
}
