//go:build gui && vte

#include "vte_bridge.h"

#include <math.h>
#include <signal.h>
#define VTE_DISABLE_DEPRECATION_WARNINGS
#include <vte/vte.h>

typedef struct {
    GPid child_pid;
    guint64 generation;
    gboolean spawning;
    gboolean exited;
    int exit_status;
    GtkEventController *motion_controller;
    gboolean motion_suppressed;
    guint motion_handler_blocks;
    gboolean has_pointer_position;
    double pointer_x;
    double pointer_y;
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
        gchar *message = g_strdup_printf("\r\nUnable to start terminal command: %s\r\n", error->message);
        e9s_vte_feed_status(terminal, message);
        g_free(message);
        state->child_pid = 0;
        state->spawning = FALSE;
        state->exited = TRUE;
        state->exit_status = -1;
        g_free(spawn);
        return;
    }
    state->child_pid = pid;
    state->spawning = FALSE;
    g_free(spawn);
}

static void e9s_vte_child_exited(VteTerminal *terminal, int status, gpointer user_data) {
    E9sVteState *state = user_data;
    state->child_pid = 0;
    state->spawning = FALSE;
    state->exited = TRUE;
    state->exit_status = status;
    gchar *message = g_strdup_printf("\r\nCommand ended (status %d).\r\n", status);
    e9s_vte_feed_status(terminal, message);
    g_free(message);
}

static GtkEventController *e9s_vte_motion_controller(GtkWidget *widget) {
    GListModel *controllers = gtk_widget_observe_controllers(widget);
    GtkEventController *fallback = NULL;
    GtkEventController *match = NULL;
    guint count = g_list_model_get_n_items(controllers);
    for (guint i = 0; i < count; i++) {
        GtkEventController *controller = g_list_model_get_item(controllers, i);
        if (!GTK_IS_EVENT_CONTROLLER_MOTION(controller)) {
            g_object_unref(controller);
            continue;
        }
        if (fallback == NULL) {
            fallback = controller;
        }
        const char *name = gtk_event_controller_get_name(controller);
        if (g_strcmp0(name, "vte-motion-controller") == 0) {
            match = controller;
            break;
        }
        if (controller != fallback) {
            g_object_unref(controller);
        }
    }
    if (match != NULL && fallback != NULL && match != fallback) {
        g_object_unref(fallback);
    }
    if (match == NULL) {
        match = fallback;
    }
    g_object_unref(controllers);

    /* The widget owns the controller. Drop the temporary list-model reference
     * while retaining a borrowed pointer for the widget's lifetime. */
    if (match != NULL) {
        g_object_unref(match);
    }
    return match;
}

static guint e9s_vte_block_motion_signal(GtkEventController *controller, const char *name, gboolean block) {
    guint signal_id = g_signal_lookup(name, GTK_TYPE_EVENT_CONTROLLER_MOTION);
    if (signal_id == 0) {
        return 0;
    }
    if (block) {
        return g_signal_handlers_block_matched(
            controller,
            G_SIGNAL_MATCH_ID,
            signal_id,
            0,
            NULL,
            NULL,
            NULL
        );
    }
    return g_signal_handlers_unblock_matched(
        controller,
        G_SIGNAL_MATCH_ID,
        signal_id,
        0,
        NULL,
        NULL,
        NULL
    );
}

static void e9s_vte_set_motion_suppressed(E9sVteState *state, gboolean suppressed) {
    if (state == NULL || state->motion_controller == NULL || state->motion_suppressed == suppressed) {
        return;
    }

    if (suppressed) {
        state->motion_handler_blocks =
            e9s_vte_block_motion_signal(state->motion_controller, "enter", TRUE) +
            e9s_vte_block_motion_signal(state->motion_controller, "motion", TRUE);
        state->motion_suppressed = state->motion_handler_blocks > 0;
        return;
    }

    e9s_vte_block_motion_signal(state->motion_controller, "enter", FALSE);
    e9s_vte_block_motion_signal(state->motion_controller, "motion", FALSE);
    state->motion_handler_blocks = 0;
    state->motion_suppressed = FALSE;
}

static void e9s_vte_pointer_motion(
    GtkEventControllerMotion *controller,
    double x,
    double y,
    gpointer user_data
) {
    E9sVteState *state = user_data;
    gboolean moved = state->has_pointer_position &&
        (fabs(x - state->pointer_x) >= 0.5 || fabs(y - state->pointer_y) >= 0.5);
    state->pointer_x = x;
    state->pointer_y = y;
    state->has_pointer_position = TRUE;
    if (moved) {
        e9s_vte_set_motion_suppressed(state, FALSE);
    }
}

static void e9s_vte_pointer_pressed(
    GtkGestureClick *gesture,
    int press_count,
    double x,
    double y,
    gpointer user_data
) {
    E9sVteState *state = user_data;
    state->pointer_x = x;
    state->pointer_y = y;
    state->has_pointer_position = TRUE;
    e9s_vte_set_motion_suppressed(state, FALSE);
}

static gboolean e9s_vte_pointer_scrolled(
    GtkEventControllerScroll *controller,
    double dx,
    double dy,
    gpointer user_data
) {
    e9s_vte_set_motion_suppressed(user_data, FALSE);
    return FALSE;
}

static void e9s_vte_install_pointer_observers(GtkWidget *widget, E9sVteState *state) {
    GtkEventController *motion = gtk_event_controller_motion_new();
    gtk_event_controller_set_propagation_phase(motion, GTK_PHASE_CAPTURE);
    g_signal_connect(motion, "motion", G_CALLBACK(e9s_vte_pointer_motion), state);
    gtk_widget_add_controller(widget, motion);

    GtkGesture *click = gtk_gesture_click_new();
    gtk_event_controller_set_propagation_phase(GTK_EVENT_CONTROLLER(click), GTK_PHASE_CAPTURE);
    gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(click), 0);
    g_signal_connect(click, "pressed", G_CALLBACK(e9s_vte_pointer_pressed), state);
    gtk_widget_add_controller(widget, GTK_EVENT_CONTROLLER(click));

    GtkEventController *scroll = gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_BOTH_AXES);
    gtk_event_controller_set_propagation_phase(scroll, GTK_PHASE_CAPTURE);
    g_signal_connect(scroll, "scroll", G_CALLBACK(e9s_vte_pointer_scrolled), state);
    gtk_widget_add_controller(widget, scroll);
}

GtkWidget *e9s_vte_terminal_new(void) {
    GtkWidget *widget = vte_terminal_new();
#if VTE_CHECK_VERSION(0, 76, 0)
    /* GtkPaned allocates fractional character rows while its divider moves.
     * Filling that remainder stretches every row until another complete row
     * fits, then snaps the grid back to its normal cell height. Leave the
     * sub-row remainder unused and anchor the grid to the drawer's fixed
     * bottom edge so resizing reveals whole rows above the existing text. */
    vte_terminal_set_yalign(VTE_TERMINAL(widget), VTE_ALIGN_END);
    vte_terminal_set_yfill(VTE_TERMINAL(widget), FALSE);
#endif
    E9sVteState *state = g_new0(E9sVteState, 1);
    state->motion_controller = e9s_vte_motion_controller(widget);
    e9s_vte_install_pointer_observers(widget, state);
    /* Keep the tiny state allocation alive for any late async spawn callback. */
    g_object_set_data(G_OBJECT(widget), "e9s-vte-state", state);
    g_signal_connect(widget, "child-exited", G_CALLBACK(e9s_vte_child_exited), state);
    vte_terminal_set_scrollback_lines(VTE_TERMINAL(widget), 10000);
    vte_terminal_set_scroll_on_keystroke(VTE_TERMINAL(widget), TRUE);
    vte_terminal_set_scroll_on_output(VTE_TERMINAL(widget), FALSE);
    return widget;
}

void e9s_vte_terminal_copy_clipboard(GtkWidget *widget) {
    vte_terminal_copy_clipboard_format(VTE_TERMINAL(widget), VTE_FORMAT_TEXT);
}

void e9s_vte_terminal_paste_clipboard(GtkWidget *widget) {
    vte_terminal_paste_clipboard(VTE_TERMINAL(widget));
}

gboolean e9s_vte_terminal_has_selection(GtkWidget *widget) {
    return vte_terminal_get_has_selection(VTE_TERMINAL(widget));
}

void e9s_vte_terminal_select_all(GtkWidget *widget) {
    vte_terminal_select_all(VTE_TERMINAL(widget));
}

gboolean e9s_vte_terminal_motion_suppression_available(GtkWidget *widget) {
    E9sVteState *state = e9s_vte_state(widget);
    return state != NULL && state->motion_controller != NULL;
}

gboolean e9s_vte_terminal_motion_suppression_active(GtkWidget *widget) {
    E9sVteState *state = e9s_vte_state(widget);
    return state != NULL && state->motion_suppressed && state->motion_handler_blocks >= 2;
}

void e9s_vte_terminal_set_motion_suppressed(GtkWidget *widget, gboolean suppressed) {
    E9sVteState *state = e9s_vte_state(widget);
    /* VTE's GTK4 motion controller turns both its "enter" and "motion"
     * signals into terminal mouse reports. The enter signal has no underlying
     * GdkEvent, so it must be blocked at its source. Capture-phase pointer
     * controllers on the VTE widget restore the handlers on genuine activity
     * without passing borrowed GDK event objects through cgo. */
    e9s_vte_set_motion_suppressed(state, suppressed);
}

void e9s_vte_terminal_spawn(GtkWidget *widget, char **argv, const char *working_directory) {
    E9sVteState *state = e9s_vte_state(widget);
    if (state->child_pid != 0) {
        kill(state->child_pid, SIGHUP);
        state->child_pid = 0;
    }
    state->generation++;
    state->spawning = TRUE;
    state->exited = FALSE;
    state->exit_status = 0;
    E9sVteSpawn *spawn = g_new0(E9sVteSpawn, 1);
    spawn->state = state;
    spawn->generation = state->generation;
    vte_terminal_spawn_async(
        VTE_TERMINAL(widget),
        VTE_PTY_DEFAULT,
        working_directory,
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
        state->spawning = FALSE;
        if (state->child_pid != 0) {
            kill(state->child_pid, SIGHUP);
            state->child_pid = 0;
        }
    }
}

gboolean e9s_vte_terminal_running(GtkWidget *widget) {
    E9sVteState *state = e9s_vte_state(widget);
    return state != NULL && (state->spawning || state->child_pid != 0);
}

int e9s_vte_terminal_exit_status(GtkWidget *widget) {
    E9sVteState *state = e9s_vte_state(widget);
    if (state == NULL || !state->exited) {
        return 0;
    }
    return state->exit_status;
}

const char *e9s_vte_terminal_window_title(GtkWidget *widget) {
    /* This compatibility accessor remains available across the GTK4 VTE
     * versions supported by e9s; newer releases implement it via xterm.title. */
    return vte_terminal_get_window_title(VTE_TERMINAL(widget));
}

const char *e9s_vte_terminal_current_directory_uri(GtkWidget *widget) {
    return vte_terminal_get_current_directory_uri(VTE_TERMINAL(widget));
}

void e9s_vte_terminal_reset(GtkWidget *widget) {
    vte_terminal_reset(VTE_TERMINAL(widget), TRUE, TRUE);
}

void e9s_vte_terminal_set_font_scale(GtkWidget *widget, double scale) {
    vte_terminal_set_font_scale(VTE_TERMINAL(widget), scale);
}

void e9s_vte_terminal_set_font(GtkWidget *widget, const char *description) {
    PangoFontDescription *font = NULL;
    if (description != NULL && description[0] != '\0') {
        font = pango_font_description_from_string(description);
    }
    vte_terminal_set_font(VTE_TERMINAL(widget), font);
    if (font != NULL) {
        pango_font_description_free(font);
    }
}

void e9s_vte_terminal_set_scrollback_lines(GtkWidget *widget, long lines) {
    vte_terminal_set_scrollback_lines(VTE_TERMINAL(widget), lines);
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
