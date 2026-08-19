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

## Phase 4: Evaluation and documentation

Status: passed with constraints on 2026-08-17.

### Measurements

Measurements were taken from cached builds on the Ubuntu/Hyprland development
system. They are comparative observations, not release benchmarks.

| Check | Result |
| --- | ---: |
| GTK process start to mapped Wayland surface | 353 ms |
| GTK process RSS immediately after startup | 142,388 KiB |
| Cached GUI build | 2.02 s, 829,884 KiB maximum RSS |
| Cached TUI build | 1.11 s, 680,832 KiB maximum RSS |
| GUI binary | 31,928,648 bytes |
| TUI binary | 32,134,907 bytes |

The log buffer was exercised with 100,000 synthetic events in 1,000 batches. It
retained exactly the newest 2,000 entries with a capacity of 2,000. This verifies
that retained log storage does not grow with session duration; it is not a
substitute for profiling a long authenticated CloudWatch session.

Refresh and log results use independent cancellation contexts and generation
checks. The race-enabled GUI and service tests passed, and repeated mapping,
resizing, and shutdown did not leave a GUI process running.

### Wayland, tiling, and scaling

Hyprland reported the real PoC window as mapped with class
`com.github.dostrow.e9s.gui` and `xwayland: false`. The surface remained mapped at
960x720 and 1280x800 floating sizes. It also mapped as a 1520x963 tiled client on
a 1920x1280 display temporarily set to scale 1.25; the monitor was restored to
scale 1.0 after the check.

A faithful screenshot could not be captured during final evaluation because the
active compositor surface was locked. Native capture contained only the lock
screen, while the Broadway automation fallback produced a blank canvas. No image
is included rather than presenting an inaccurate screenshot. Visual interaction,
dark-theme details, and clipboard paste should be repeated with an unlocked
session using the checklist in [`gui-development.md`](gui-development.md).

### Build and dependency findings

The verified system used Ubuntu 25.10, GTK 4.20.1, GLib 2.86.0, gcc 15.2,
`libgtk-4-dev`, `libglib2.0-dev`, and `pkg-config`. Repeatable Ubuntu/Debian, Arch,
and Fedora setup commands are recorded in
[`gui-development.md`](gui-development.md).

gotk4 remains the principal technical constraint. Version `v0.3.1` supplies the
required widgets and runs reliably in this slice, but later binding releases did
not compile against the available GLib headers. The pinned version produces
non-fatal generated-C warnings, and its first clean build compiles a large wrapper.
Pinning and testing the complete native dependency matrix in CI would be required
before treating the GUI as a supported release artifact.

Build tags successfully contain that cost. The normal TUI build passes without
GTK installed in its package graph, and `CGO_ENABLED=0 go build .` still succeeds.

### Shared-core evaluation

The service seam held up well for the vertical slice. Both frontends share AWS
configuration, ECS queries and mutation, log-source resolution, log query routing,
error context, and caller-owned cancellation. Toolkit models, selections,
navigation, formatting, and event-loop integration remain frontend-owned.

The main friction was not the service boundary; it was adapting GTK's main-loop
and object-model conventions to Go. Generation checks and immutable result values
kept stale goroutine results out of newer views without forcing GTK concepts into
the shared packages.

### Go/no-go decision

**Go, with constraints.** GTK 4 is the better next experiment than a Qt rewrite
for this repository. It provides native Wayland operation, dense desktop widgets,
keyboard and clipboard integration, and acceptable runtime behavior. More
importantly, the PoC proves the TUI and GUI can share connection, query, mutation,
and log logic without compromising the TUI build.

Proceed incrementally rather than declaring the GUI production-ready:

1. keep the TUI first-class and GTK behind a separate command/build tag;
2. pin gotk4 and the tested GTK/GLib matrix in GUI CI;
3. repeat unlocked visual, clipboard, and long authenticated log-follow profiling;
4. port one additional module to validate that the shared services generalize;
5. decide on packaging only after the second module and native dependency CI pass.

Qt does not currently offer a compelling benefit large enough to offset restarting
the binding, event-loop, and widget work. Reconsider it only if gotk4 compatibility
or release cadence prevents supporting the target distributions.

### CloudWatch Logs module follow-up

The second-module experiment is complete. The GTK frontend now browses groups
and streams; peeks and follows streams or whole groups; searches single or
comma-separated multi-group/stream scopes; supports relative and custom UTC
ranges, adjacent windows, and cursor-based correlation; and renders results in
the same bounded, wrapping Workspace log view used by ECS.

Saved `log_paths` are dynamic CloudWatch Logs Module Rail destinations. Existing
group, stream, and multi-group entries remain valid, while new entries may also
store multiple streams, a normalized filter, a relative lookback, or fixed UTC
bounds. Save and Manage saved are contextual Header Bar actions; rename,
reorder, delete, active highlighting, and explicit config reload are supported.

The parity pass moved TUI group listing, stream listing, filter normalization,
and search dispatch through `service.Logs`, eliminating the duplicate frontend
query path. Both frontends now share the same AWS routing and saved-search
schema. The GUI additionally gained local/UTC/relative timestamps, clipboard
copy, and portal-backed buffer export. Multi-scope entry and local match
navigation intentionally follow native GUI metaphors as documented in the
roadmap.

### CloudWatch Alarms module follow-up

The next module now validates the same architecture for operational state and
mutations. `service.Alarms` is shared by the TUI and GUI and owns state
validation, deterministic ordering, error context, detail retrieval, action
enablement, and manual state overrides.

The GTK frontend exposes state scopes as sub-items in a collapsible CloudWatch
Alarms rail entry. Its Browser Pane supports alarm/metric/namespace filtering;
the Workspace Pane shows current state, configuration, dimensions, configured
actions, and recent history. Local/UTC timestamps, guarded contextual actions,
pending-state disabling, post-mutation refresh, and stale-response rejection
complete the TUI parity pass.

### SSM Parameter Store module follow-up

The configuration-data workflow now uses `service.SSM` in both frontends for
normalized paths, stable name ordering, decrypted detail reads, updates, and
contextual errors. Parameter Store values live in the UI-neutral model package;
the GTK layer does not call AWS adapters directly.

The GTK frontend adds an alphabetically sorted, collapsible SSM module with a
fixed Parameters destination and dynamic saved-prefix sub-items. Custom path
browsing, non-sensitive filtering, prefix save/delete/reload, exact rail
highlighting, immediate context clearing, and generation-guarded requests cover
the Browser workflow. SecureString list values remain masked and are excluded
from value filtering.

Workspace value inspection is explicit. SecureString decryption requires a
warning confirmation, and changing context clears revealed plaintext. Editing
uses a wrapped multiline buffer followed by a second confirmation. Reads and
writes disable applicable controls, hold automatic refresh through the shared
busy state, and refresh parameter metadata and detail after successful updates.
The TUI also now reads values through the shared decrypted-detail path.

### Secrets Manager module follow-up

Secrets Manager now uses `service.Secrets` in both frontends for true substring
filtering, stable name ordering, explicit current-value reads, creation, updates,
validation, cancellation, and contextual errors. Secret metadata and values are
UI-neutral model types; the GTK package does not call the AWS adapter directly.

The GTK frontend adds a collapsible Secrets Manager module with fixed Secrets
and dynamic saved-filter destinations. Its Browser Pane loads metadata only and
can search names, descriptions, ARNs, and tags without retrieving plaintext.
Saved filter creation, deletion, configuration reload, exact rail highlighting,
immediate clearing, and generation guards follow the SSM navigation pattern.

Selecting a secret remains metadata-only. Reveal, Edit, and Clone explicitly
warn before the first value read; editors expose multiline plaintext only after
that confirmation, and mutations require a second review. JSON is pretty-printed,
binary bytes are never displayed or converted to strings, context changes clear
loaded values, and pending operations disable contextual actions and pause
refresh. Copy ARN remains a metadata-only operation.

### Lambda module follow-up

Lambda now uses `service.Lambda` in both frontends for stable discovery,
configuration reads, environment resolution, package-type checks, deployment
downloads, ZIP creation, and code updates. Function configuration is a
UI-neutral model, and neither frontend duplicates AWS SDK query logic.

The GTK frontend adds a collapsible Lambda module with a fixed Functions item
and dynamic saved-search destinations. It provides local metadata filtering,
current configuration reads, guarded secret resolution, and direct follow,
search, and stream-browsing routes into the existing CloudWatch Logs workflow.
Pending operations disable contextual controls, pause automatic refresh, and
use generation guards to reject responses from abandoned contexts.

ZIP functions can be edited in the Workspace pane through a per-file selector.
Only UTF-8 text files up to 2 MiB are rendered; all other deployment content is
retained unchanged when the directory is repackaged. Unsaved changes and uploads
have separate confirmation gates, paths are validated before writes, temporary
downloads are removed on every exit path, and image-based functions remain
explicitly read-only. The TUI retains its external-editor presentation while
sharing the same service operations.

### Source editor follow-up

Task-definition JSON and Lambda text files now share a small document-oriented
editor abstraction. Full builds use GtkSourceView 5 for filename-based language
detection, theme-aware schemes, line numbers, current-line and bracket
highlighting, indentation, and native undo/redo. A narrow cgo bridge targets the
GtkSourceView 5 C API and exposes the editor through its `GtkTextView` base class,
avoiding a second generated binding with a mismatched GTK or gotk4 generation.
`gui`-only builds retain the former plain `GtkTextView` implementation as a
packaging and compatibility fallback.

This creates a suitable boundary for future diagnostics or completion providers
without introducing language-server lifecycle and workspace management now.

### Final verification

With Go 1.24, gotk4's weak-reference internals trip the race build's default
checkptr instrumentation while constructing GTK objects. The GUI race suite is
therefore run with checkptr disabled at the binding boundary; the ordinary GUI
tests and builds retain the default checks.

```text
go test ./...                                      PASS
go test -tags "gui sourceview" ./...               PASS
go test -race -tags "gui sourceview" -gcflags=all=-d=checkptr=0 \
  ./internal/gui ./internal/service                PASS
go vet ./...                                       PASS
go build .                                         PASS
go build -tags "gui vte sourceview" ./cmd/e9s-gui PASS
CGO_ENABLED=0 go build .                           PASS
```
