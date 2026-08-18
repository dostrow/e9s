# GTK GUI proof-of-concept results

This document records evidence and decisions as each phase of the proof of
concept is completed. The implementation plan remains in
[`gui-poc-plan.md`](gui-poc-plan.md).

## Phase 0: GTK, gotk4, and Wayland spike

Status: passed on 2026-08-17.

### Environment

- Hyprland running a Wayland session (`WAYLAND_DISPLAY=wayland-1`).
- GTK 4.20.1.
- GLib/GIO/GObject 2.86.0.
- Go 1.24.4.
- AMD Radeon 890M using GTK's Vulkan renderer.

### Binding decision

gotk4 is pinned to `v0.3.1`. Both `v0.4.0` and `v0.4.1` generate references to
GLib APIs newer than the installed GLib 2.86 headers, including
`g_get_monotonic_time_ns`. The older release builds against the current system
and supplies the GTK widgets required by the PoC.

gotk4 `v0.3.1` emits harmless gcc warnings about its generated declaration of
`free`. These warnings are noisy but do not prevent compilation or execution.

### Exercised GTK surface

- A normal `GtkApplicationWindow` with application CSS.
- A virtualized `GtkColumnView` containing 1,200 synthetic services.
- Resizable split-pane layout.
- A monospace `GtkTextView` containing 5,000 synthetic log lines.
- A `Ctrl+R` application action.
- Background goroutine updates marshaled through `glib.IdleAdd`.
- Context cancellation during application shutdown.

GTK 4.20 emits `GtkColumnViewCell` from the column factory even though older
examples often cast the object to `GtkListItem`; the PoC uses the runtime-correct
cell type.

### Runtime evidence

Hyprland reported the window with:

```text
class:    com.github.dostrow.e9s.gui
title:    e9s GTK 4 spike
mapped:   true
xwayland: false
```

The running spike used approximately 128 MiB RSS after creating the synthetic
table and log buffer. It stayed responsive while receiving one main-loop update
per second. The first clean gotk4 build is expensive because gcc compiles a large
generated GTK wrapper; a cached GUI rebuild completed in about 0.1 seconds.

The linked executable resolves GTK 4, GLib, GDK Pixbuf, Pango, and the Wayland
client libraries dynamically. GTK development packages are required only for the
GUI build; the build-tagged GUI command is excluded from ordinary TUI builds and
tests.

### Verification

```text
go build -tags gui -o /tmp/e9s-gui-spike ./cmd/e9s-gui  PASS
GDK_BACKEND=wayland /tmp/e9s-gui-spike                  PASS
go build -o /tmp/e9s-tui .                              PASS
go test ./...                                            PASS
```

### Phase decision

Proceed to the shared-service extraction. GTK, gotk4, the required widgets, the
main-loop handoff pattern, and native Wayland operation all passed the Phase 0
exit criterion. Clipboard behavior and production log-history bounds remain part
of the real workflow in later phases rather than the synthetic spike.
