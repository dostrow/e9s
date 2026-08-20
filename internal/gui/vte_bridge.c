//go:build gui && vte

#include "vte_bridge.h"

#include <signal.h>
#include <vte/vte.h>

typedef struct {
    GPid child_pid;
    guint64 generation;
} E9sVteState;

typedef struct {
    E9sVteState *state;
    guint64 generation;
} E9sVteSpawn;

static E9sVteState *e9s_vte_state(GtkWidget *widget) {
    return g_object_get_data(G_OBJECT(widget), "e9s-vte-state");
}

static void e9s_vte_feed_status(VteTerminal *terminal, const char *message) {
    vte_terminal_feed(terminal, message, -1);
}

static void e9s_vte_spawned(VteTerminal *terminal, GPid pid, GError *error, gpointer user_data) {
    E9sVteSpawn *spawn = user_data;
    E9sVteState *state = spawn->state;
    if (spawn->generation != state->generation) {
        if (pid != -1) {
            kill(pid, SIGHUP);
        }
        g_free(spawn);
        return;
    }
    if (error != NULL) {
        gchar *message = g_strdup_printf("\r\nUnable to start Session Manager plugin: %s\r\n", error->message);
        e9s_vte_feed_status(terminal, message);
        g_free(message);
        state->child_pid = 0;
        g_free(spawn);
        return;
    }
    state->child_pid = pid;
    g_free(spawn);
}

static void e9s_vte_child_exited(VteTerminal *terminal, int status, gpointer user_data) {
    E9sVteState *state = user_data;
    state->child_pid = 0;
    gchar *message = g_strdup_printf("\r\nECS Exec session ended (status %d).\r\n", status);
    e9s_vte_feed_status(terminal, message);
    g_free(message);
}

GtkWidget *e9s_vte_terminal_new(void) {
    GtkWidget *widget = vte_terminal_new();
    E9sVteState *state = g_new0(E9sVteState, 1);
    /* Keep the tiny state allocation alive for any late async spawn callback. */
    g_object_set_data(G_OBJECT(widget), "e9s-vte-state", state);
    g_signal_connect(widget, "child-exited", G_CALLBACK(e9s_vte_child_exited), state);
    vte_terminal_set_scrollback_lines(VTE_TERMINAL(widget), 10000);
    vte_terminal_set_scroll_on_keystroke(VTE_TERMINAL(widget), TRUE);
    vte_terminal_set_scroll_on_output(VTE_TERMINAL(widget), FALSE);
    return widget;
}

void e9s_vte_terminal_spawn(GtkWidget *widget, char **argv) {
    E9sVteState *state = e9s_vte_state(widget);
    if (state->child_pid != 0) {
        kill(state->child_pid, SIGHUP);
        state->child_pid = 0;
    }
    state->generation++;
    E9sVteSpawn *spawn = g_new0(E9sVteSpawn, 1);
    spawn->state = state;
    spawn->generation = state->generation;
    vte_terminal_spawn_async(
        VTE_TERMINAL(widget),
        VTE_PTY_DEFAULT,
        NULL,
        argv,
        NULL,
        G_SPAWN_DEFAULT,
        NULL,
        NULL,
        NULL,
        -1,
        NULL,
        e9s_vte_spawned,
        spawn
    );
}

void e9s_vte_terminal_stop(GtkWidget *widget) {
    E9sVteState *state = e9s_vte_state(widget);
    if (state != NULL) {
        state->generation++;
        if (state->child_pid != 0) {
            kill(state->child_pid, SIGHUP);
            state->child_pid = 0;
        }
    }
}

void e9s_vte_terminal_reset(GtkWidget *widget) {
    vte_terminal_reset(VTE_TERMINAL(widget), TRUE, TRUE);
}

void e9s_vte_terminal_set_font_scale(GtkWidget *widget, double scale) {
    vte_terminal_set_font_scale(VTE_TERMINAL(widget), scale);
}

static GdkRGBA e9s_vte_color(const char *value) {
    GdkRGBA color = {0};
    gdk_rgba_parse(&color, value);
    return color;
}

static GdkRGBA e9s_vte_blend(GdkRGBA from, GdkRGBA to, double amount) {
    GdkRGBA color = {
        .red = from.red + (to.red - from.red) * amount,
        .green = from.green + (to.green - from.green) * amount,
        .blue = from.blue + (to.blue - from.blue) * amount,
        .alpha = 1.0,
    };
    return color;
}

void e9s_vte_terminal_set_palette(
    GtkWidget *widget,
    const char *foreground_value,
    const char *background_value,
    const char *accent_value,
    const char *success_value,
    const char *warning_value,
    const char *error_value,
    const char *info_value,
    const char *muted_value
) {
    GdkRGBA foreground = e9s_vte_color(foreground_value);
    GdkRGBA background = e9s_vte_color(background_value);
    GdkRGBA accent = e9s_vte_color(accent_value);
    GdkRGBA success = e9s_vte_color(success_value);
    GdkRGBA warning = e9s_vte_color(warning_value);
    GdkRGBA error_color = e9s_vte_color(error_value);
    GdkRGBA info = e9s_vte_color(info_value);
    GdkRGBA muted = e9s_vte_color(muted_value);
    GdkRGBA cyan = e9s_vte_blend(info, success, 0.5);
    GdkRGBA palette[16] = {
        e9s_vte_blend(background, foreground, 0.14),
        error_color,
        success,
        warning,
        info,
        accent,
        cyan,
        foreground,
        muted,
        e9s_vte_blend(error_color, foreground, 0.22),
        e9s_vte_blend(success, foreground, 0.22),
        e9s_vte_blend(warning, foreground, 0.22),
        e9s_vte_blend(info, foreground, 0.22),
        e9s_vte_blend(accent, foreground, 0.22),
        e9s_vte_blend(cyan, foreground, 0.22),
        foreground,
    };
    VteTerminal *terminal = VTE_TERMINAL(widget);
    vte_terminal_set_colors(terminal, &foreground, &background, palette, 16);
    vte_terminal_set_color_cursor(terminal, &accent);
    vte_terminal_set_color_cursor_foreground(terminal, &background);
    vte_terminal_set_color_highlight(terminal, &accent);
    vte_terminal_set_color_highlight_foreground(terminal, &background);
    vte_terminal_set_color_bold(terminal, &foreground);
}
