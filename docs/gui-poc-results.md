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

## Phase 1: Shared ECS and log services

Status: passed on 2026-08-17.

### Shared model and services

The new `internal/service` package has no Bubble Tea or GTK dependencies. It
defines two capability-oriented services:

- `ECS` wraps cluster, service, and task listing; force deployment; and task- or
  service-level log-source resolution.
- `Logs` accepts a UI-neutral `model.LogQuery` and dispatches single-stream,
  multi-stream, whole-group, multi-group, tail, and fixed-range reads.

Shared log entries, sources, queries, and pages now live in `internal/model`.
`internal/aws.LogEntry` remains an alias so existing low-level callers and tests
continue to compile while frontends use the model type.

Every shared operation accepts a caller-owned `context.Context`. Operational
errors are wrapped with the affected cluster, service, or log group while
retaining their original cause.

### TUI migration

The ECS portion of the TUI now uses the shared ECS service for:

- cluster, service, task, and standalone-task queries;
- force deployment;
- task container log-source resolution; and
- service-wide log-source resolution.

The TUI log viewer now uses the shared log query dispatcher for all fetch, tail,
older/newer, and range operations. Its Bubble Tea messages, selection state,
keyboard handling, rendering, and 1,000-line display bound remain frontend-owned.

`ui.App` owns a cancellable lifecycle context. Relevant ECS and log commands use
that context, and the normal quit path cancels it before returning `tea.Quit`.

### Verification

Tests use high-level fake AWS interfaces and do not require credentials. They
cover query routing, default limits, log source construction, mutation arguments,
error wrapping, and cancellation propagation.

```text
go test ./...                         PASS
go test -race ./internal/service      PASS
go build -o /tmp/e9s-tui .           PASS
go build -tags gui ./cmd/e9s-gui     PASS
```

### Phase decision

Proceed to the read-only GTK workflow. Both frontends can now depend on the same
ECS and CloudWatch behavior without sharing toolkit state, and the existing TUI
suite passes without observable workflow changes.

## Phase 2: Read-only GTK ECS workflow

Status: passed on 2026-08-17.

### Application startup

`e9s-gui` now has its own Cobra entry point with the TUI's profile, region,
cluster, refresh-interval, and YAML-default precedence. It creates the same shared
AWS client and injects shared ECS and log services into the GTK frontend. GTK code
does not construct or call AWS SDK clients directly.

The GUI remains behind the `gui` build tag, so the normal TUI package graph does
not compile or link GTK.

### Read-only workflow

The synthetic Phase 0 content has been replaced by a real ECS workflow:

- load and filter clusters;
- open a cluster and load/filter services;
- open a service and load its tasks;
- inspect status, counts, task definition, launch type, deployments, tasks, and
  recent service events;
- refresh manually with `Ctrl+R` or on the configured interval; and
- preserve an open service across refresh when it still exists.

The layout uses a module sidebar, virtualized `GtkColumnView` resource tables,
master/detail panes, a monospace inspector, a breadcrumb, and a profile/region
status bar. `Enter`, `Escape`, `/`, `Ctrl+R`, `Ctrl+P`, and `?` provide the initial
keyboard-first navigation surface.

### Concurrency and lifecycle

All AWS operations run in goroutines. Every navigation or refresh starts a new
cancellable request and cancels the previous request. Results are scheduled onto
the GTK main loop and applied only when both their context and generation are
still current. Application shutdown cancels the root context, including the
periodic refresh loop and active AWS request.

Loading, empty, success, and error states appear in the inspector and status bar;
slow or failed AWS operations do not block GTK input or rendering.

### Testable presentation logic

Cluster filtering, service filtering, and service-detail formatting are pure Go
functions in a non-GTK source file. Their tests run as part of ordinary
`go test ./...`, without a display or GTK development headers.

### Runtime evidence

The real GUI executable mapped under Hyprland with `xwayland: false` and used
approximately 133 MiB RSS after startup. The window remained responsive while
the initial ECS request ran in the background.

### Verification

```text
go test ./...                                      PASS
go test -race ./internal/gui ./internal/service    PASS
go build -o /tmp/e9s-tui .                        PASS
go build -tags gui -o /tmp/e9s-gui ./cmd/e9s-gui PASS
GDK_BACKEND=wayland /tmp/e9s-gui --refresh 30     PASS
```

### Phase decision

Proceed to live logs and the guarded force-deployment action. The read-only
cluster -> service -> detail slice uses the shared backend, remains responsive,
and satisfies the Phase 2 exit criterion.

## Phase 3: Live logs and guarded deployment

Status: passed on 2026-08-17.

### Service log workflow

After selecting a service, the GUI resolves its active task log streams through
the shared ECS service and follows them through the shared log query service.
CloudWatch calls run outside the GTK thread on a two-second polling cadence, and
navigation, pause, or application shutdown cancels the polling context.

The log view includes:

- a 2,000-entry hard bound with old backing arrays released after trimming;
- automatic scrolling while following;
- pause and resume without discarding buffered entries;
- case-insensitive filtering across messages and stream names;
- copy through GTK's native Wayland clipboard;
- clear; and
- explicit exit back to the service inspector.

GTK only receives immutable log pages on its main loop. A separate log generation
prevents a late page or error from a cancelled session from updating a newer log
view.

### Force deployment workflow

The Force deploy action is enabled only after selecting a service. It opens a
modal Yes/No confirmation dialog naming the target service. Confirmation calls
the same shared `ECS.ForceDeployment` method used by the TUI, displays operation
progress and success/error status, and schedules a service refresh after success.

The GUI intentionally uses modified shortcuts for actions that could otherwise
fire while typing in a filter: `Ctrl+Shift+R` forces deployment,
`Ctrl+Space` pauses logs, and `Ctrl+Shift+C` copies logs.

### Verification

Pure Go tests exercise log trimming, capacity bounds, filtering, sanitization,
and clearing without GTK or AWS credentials. Shared service tests continue to
cover the actual log-query routing, cancellation, and deployment arguments.

```text
go test ./...                                      PASS
go test -race ./internal/gui ./internal/service    PASS
go vet ./...                                       PASS
go build -o /tmp/e9s-tui .                        PASS
go build -tags gui -o /tmp/e9s-gui ./cmd/e9s-gui PASS
GDK_BACKEND=wayland /tmp/e9s-gui --refresh 30     PASS
```

### Phase decision

Proceed to final evaluation. The PoC now demonstrates a continuous, bounded read
workflow and a confirmed write workflow through the same backend used by the TUI.
