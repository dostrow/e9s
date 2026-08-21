//go:build gui && vte

#pragma once

#include <gtk/gtk.h>

GtkWidget *e9s_vte_terminal_new(void);
void e9s_vte_terminal_copy_clipboard(GtkWidget *widget);
void e9s_vte_terminal_paste_clipboard(GtkWidget *widget);
gboolean e9s_vte_terminal_has_selection(GtkWidget *widget);
void e9s_vte_terminal_select_all(GtkWidget *widget);
void e9s_vte_terminal_spawn(GtkWidget *widget, char **argv, const char *working_directory);
void e9s_vte_terminal_stop(GtkWidget *widget);
gboolean e9s_vte_terminal_running(GtkWidget *widget);
int e9s_vte_terminal_exit_status(GtkWidget *widget);
const char *e9s_vte_terminal_window_title(GtkWidget *widget);
const char *e9s_vte_terminal_current_directory_uri(GtkWidget *widget);
void e9s_vte_terminal_reset(GtkWidget *widget);
void e9s_vte_terminal_set_font_scale(GtkWidget *widget, double scale);
void e9s_vte_terminal_set_font(GtkWidget *widget, const char *description);
void e9s_vte_terminal_set_scrollback_lines(GtkWidget *widget, long lines);
void e9s_vte_terminal_set_palette(
    GtkWidget *widget,
    const char *foreground,
    const char *background,
    const char *accent,
    const char *success,
    const char *warning,
    const char *error,
    const char *info,
    const char *muted
);
