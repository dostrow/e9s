//go:build gui && vte

#pragma once

#include <gtk/gtk.h>

GtkWidget *e9s_vte_terminal_new(void);
void e9s_vte_terminal_spawn(GtkWidget *widget, char **argv);
void e9s_vte_terminal_stop(GtkWidget *widget);
void e9s_vte_terminal_reset(GtkWidget *widget);
void e9s_vte_terminal_set_font_scale(GtkWidget *widget, double scale);
