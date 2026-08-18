# Experimental GTK GUI development

The GUI is an experimental GTK 4 frontend for the ECS proof of concept. It is
built as a separate `e9s-gui` executable; the normal `e9s` TUI does not compile,
link, or require GTK.

## Prerequisites

Install Go 1.24 or newer, a C compiler, `pkg-config`, the GTK 4 development
files, the GTK 4 VTE development files, and AWS's `session-manager-plugin`.
The following distro package sets cover the compile-time dependencies:

### Ubuntu and Debian

```bash
sudo apt install build-essential pkg-config libgtk-4-dev libvte-2.91-gtk4-dev
```

The PoC was verified on Ubuntu 25.10 with GTK 4.20.1, GLib 2.86.0, gcc 15.2,
and `pkg-config` 1.8.1.

### Arch Linux

```bash
sudo pacman -S --needed base-devel pkgconf gtk4 vte4
```

Arch publishes the development headers, shared library, and pkg-config metadata
in its [`gtk4` package](https://archlinux.org/packages/extra/x86_64/gtk4/); its
[`pkgconf` package](https://archlinux.org/packages/core/x86_64/pkgconf/) provides
the pkg-config implementation.

### Fedora

```bash
sudo dnf install gcc pkgconf-pkg-config gtk4-devel vte291-gtk4-devel
```

Fedora publishes the headers and build metadata in
[`gtk4-devel`](https://packages.fedoraproject.org/pkgs/gtk4/gtk4-devel/).

## Build and run

```bash
make build-gui
./e9s-gui
```

`make build-gui` enables the `gui` and `vte` tags and includes the embedded ECS
Exec terminal. A GTK-only diagnostic build remains available as
`make build-gui-basic`; it disables Exec and explains the missing build feature
in the GUI.

The GUI accepts the ECS-relevant TUI settings and YAML defaults:

```bash
./e9s-gui --profile production --region us-east-2 \
  --cluster my-cluster --refresh 5
```

To require native Wayland while developing under Hyprland:

```bash
GDK_BACKEND=wayland ./e9s-gui
```

The ordinary TUI remains independent:

```bash
make build
make test
CGO_ENABLED=0 go build .
```

GTK sources use the `gui` build tag. Run GUI package tests and static checks with:

```bash
go test ./...
go test -race ./internal/gui ./internal/service
go vet ./...
go build -tags gui ./cmd/e9s-gui
go build -tags "gui vte" ./cmd/e9s-gui
```

## Current scope

The GUI provides cluster, service, service-task, and standalone-task browsing,
including recent stopped-task diagnostics for both service and standalone
tasks loaded in 50-task pages;
task-definition inspection, diffing, environment lookup, and revision editing;
service and per-container logs; service-wide and selected-task metrics, service
alarms; scaling and guarded ECS mutations; and ECS Exec in an embedded VTE
terminal. The GTK frontend and Bubble Tea frontend both call the same UI-neutral
services in `internal/service`; GTK code does not call AWS SDK adapters directly.

Service metrics use the standard `AWS/ECS` service dimensions. Selected-task
metrics use `TaskCpuUtilization` and `TaskMemoryUtilization` from
`ECS/ContainerInsights`, so ECS Container Insights with enhanced observability
must be enabled for task-level datapoints to appear. The metrics workspace
reports unavailable datapoints as `No data` rather than treating them as zero.

gotk4 is pinned to `v0.3.1`. Releases `v0.4.0` and `v0.4.1` generated references
to GLib APIs newer than the GLib 2.86 headers available on the verified system.
The pinned version also emits non-fatal generated-C warnings about `free`. A
first clean GTK wrapper compilation can take substantially longer and use more
memory than a cached rebuild.

## Hyprland smoke checklist

1. Run with `GDK_BACKEND=wayland` and confirm `hyprctl clients` reports
   `xwayland: false` for `com.github.dostrow.e9s.gui`.
2. Exercise 960x720, 1280x800, and a normal tiled size.
3. Repeat on monitors at scale 1.0 and a fractional scale such as 1.25.
4. Check keyboard focus for `/`, `Enter`, `Escape`, `Ctrl+R`, `Ctrl+P`, and `?`.
5. Start a refresh or log follow, navigate away, and confirm no stale result
   replaces the current view.
6. Copy logs and paste into another native Wayland application.
7. Close the window during an active request and confirm the process exits.
8. Open ECS Exec, resize the tiled window, type in the remote shell, and confirm
   Disconnect terminates the local Session Manager plugin process.
9. Place the pointer over each pane and use Ctrl+mouse-wheel to confirm the
   browser and workspace zoom independently. Confirm Ctrl++/Ctrl+- affect the
   focused or most recently pointed-to pane and Ctrl+0 resets it.
10. Open Metrics with no task selected and confirm the service scope is shown.
    Select a task, reopen Metrics, and confirm the task scope is shown without
    service alarm or scale-in controls.
11. Switch between Clusters and Task Defs in the module rail and confirm the
    browser and workspace both change immediately.
12. Enter Standalone from both a service list and a service task list; confirm
    the header toggle returns to the corresponding service view.
13. Switch Standalone tasks between Active and Recently stopped. Confirm the
    latter is newest-first, exposes exit and stop details, and offers Load more
    only when ECS provides another page.
14. Repeat Active/Recently stopped switching for a service task browser. Select
    a stopped task, inspect its containers, open its logs, and return directly
    to the service details without leaving the task browser.
15. Open logs, then use the header Back button or select another browser item;
    confirm log polling stops and the workspace shows the new context.
16. Repeatedly switch between long and short breadcrumbs and confirm the header
    fully repaints without remnants of the previous text.
17. Collapse and expand ECS in the module rail and confirm its sub-items hide
    and return without changing the current browser context. Confirm the active
    sub-item remains highlighted using the current GTK theme.
18. Select a service task, then use Back to service details in the workspace;
    confirm the task browser remains visible and its row is unselected.
19. Move through cluster, service, task, and standalone contexts and confirm the
    header shows only applicable action groups; unavailable capabilities remain
    visible but disabled within an applicable context.
20. Open one service's task browser, navigate away, and open another service.
    Confirm the previous task rows and selection disappear before the new AWS
    response arrives. Repeat while switching from tasks to Task Defs.

See [`gui-poc-results.md`](gui-poc-results.md) for the measurements, limitations,
and recommendation from the initial experiment.

The agreed module order, CloudWatch Logs phases, and Module Rail behavior for
saved searches are recorded in the [`GUI expansion roadmap`](gui-roadmap.md).
