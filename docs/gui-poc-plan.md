# GTK 4 GUI proof of concept

## Goal

Determine whether e9s can support a polished, keyboard-friendly GTK 4 interface on
Wayland/Hyprland while keeping AWS connection, query, mutation, polling, and domain
logic shared with the existing TUI.

The proof of concept is successful when the TUI and GUI are two presentation layers
over the same Go application services. The TUI remains fully functional throughout
the experiment.

## Proposed technology

- GTK 4 for the native Wayland UI.
- `github.com/diamondburned/gotk4` for Go bindings.
- Plain GTK initially; do not require libadwaita for the proof of concept.
- Go contexts and ordinary Go values at the shared-service boundary.
- A separate `e9s-gui` executable so the experiment does not change the existing
  `e9s` startup path or add GTK as a runtime dependency of the TUI binary.

The first technical spike must pin compatible GTK/gotk4 versions. The dependency
should remain isolated to the GUI command and GUI package.

## Vertical slice

Implement one narrow but representative workflow:

1. Start with the same profile, region, configuration, and credentials as the TUI.
2. List ECS clusters.
3. Select a cluster and list its services.
4. Select a service and show service details, deployments, tasks, and recent events.
5. Refresh data manually and on the configured interval.
6. Filter cluster and service lists.
7. Open a service log view, fetch recent CloudWatch events, and follow new events.
8. Cancel in-flight requests and log polling when navigating away or closing the
   window.
9. Trigger one mutation, preferably force deployment, behind a confirmation dialog.
10. Surface loading, empty, success, and error states without blocking the UI.

This slice exercises data tables, master/detail navigation, background AWS calls,
polling, streaming-style updates, configuration, cancellation, and safe mutations.

## Non-goals

- Porting every e9s module.
- Replacing or redesigning the TUI.
- Sharing widget state, selection state, or navigation state between frontends.
- Introducing a generic framework for every possible AWS operation before the
  vertical slice needs it.
- Supporting Windows or macOS in this experiment.
- Using layer-shell protocols; e9s should be a normal desktop window.
- Matching every terminal keybinding exactly.
- Committing to libadwaita, Flatpak, or a final packaging format.

## Target architecture

```text
AWS SDK
   |
internal/aws        AWS API adapters and SDK transformations
   |
internal/service    UI-neutral workflows, polling, and cancellation
   |
internal/model      UI-neutral domain and result types
   |
   +-- internal/ui  Bubble Tea presentation adapter
   |
   +-- internal/gui GTK presentation adapter
```

### Shared responsibilities

The following behavior must be common to both frontends:

- AWS configuration, credential/profile loading, and region selection.
- AWS queries and mutations.
- Pagination tokens and fetch semantics.
- Polling intervals, retry decisions, and cancellation.
- CloudWatch log stream resolution and event retrieval.
- Transformations from AWS SDK types into e9s domain models.
- Configuration and bookmark persistence.
- Consistent operation errors and action results.

### Frontend responsibilities

Each frontend independently owns:

- Navigation history and selected rows.
- Keyboard, mouse, focus, and shortcut behavior.
- Rendering, layout, tables, dialogs, and notifications.
- Toolkit event-loop integration.
- Ephemeral presentation state such as an open filter box or inspector tab.

Shared packages must not import Bubble Tea, Lip Gloss, GTK, or libadwaita.

## Initial shared API

Add small capability-oriented services rather than a single application controller.
Interfaces should live with their consumers or in the shared service package when
both frontends use them.

```go
type ECSService interface {
    ListClusters(context.Context) ([]model.Cluster, error)
    ListServices(context.Context, string) ([]model.Service, error)
    ListTasks(context.Context, string, string) ([]model.Task, error)
    ForceNewDeployment(context.Context, string, string) error
}

type LogService interface {
    ResolveServiceStreams(
        context.Context,
        string,
        string,
    ) (model.LogSource, error)

    Fetch(
        context.Context,
        model.LogQuery,
    ) (model.LogPage, error)
}
```

Exact names can change during implementation. The important constraints are:

- Every operation accepts a caller-owned `context.Context`.
- Results contain no toolkit-specific values.
- AWS SDK types do not leak into either frontend unless they are truly part of the
  low-level adapter contract.
- Polling is driven by a reusable service/session or by the frontend calling shared
  one-shot operations; it is not hidden in an immortal goroutine.
- Tests can provide fakes without constructing real AWS SDK clients.

For log following, begin with a cancellable polling session rather than a complex
multi-channel abstraction. A callback or event channel is acceptable if ownership,
shutdown, backpressure, and error behavior are explicit.

## Repository layout

The likely end state for the proof of concept is:

```text
cmd/
  e9s/             TUI entry point
  e9s-gui/         GTK entry point
internal/
  aws/             Existing AWS adapters
  config/          Shared configuration
  model/           Shared domain types
  service/
    ecs.go
    logs.go
    poll.go         Only if a shared polling primitive proves useful
  ui/               Existing Bubble Tea frontend
  gui/
    app.go
    window.go
    clusters.go
    services.go
    service_detail.go
    logs.go
    style.css
```

Moving the current root `main.go` to `cmd/e9s` is optional during the first spike.
It should happen only if both commands cannot otherwise be built cleanly.

## Work plan

### Phase 0: Binding and Wayland spike

- Pin gotk4 packages compatible with the GTK version available on the development
  system.
- Create `cmd/e9s-gui` with a normal application window and CSS resource.
- Verify native Wayland operation under Hyprland, fractional scaling, dark mode,
  clipboard access, keyboard shortcuts, and clean shutdown.
- Build a `GtkColumnView` with synthetic rows and update it from background work by
  scheduling mutations on the GTK main thread.
- Append several thousand synthetic log lines and confirm acceptable scrolling and
  memory behavior.

Exit criterion: the binding supports the widgets and asynchronous update pattern
needed for the vertical slice without crashes or obvious leaks.

### Phase 1: Establish a shared seam

- Inventory ECS and CloudWatch logic currently invoked from `internal/ui`.
- Separate UI-neutral orchestration from Bubble Tea commands and messages.
- Introduce the smallest ECS and log services required by the vertical slice.
- Give services narrow SDK-facing interfaces so unit tests can use fakes.
- Replace `context.Background()` inside relevant TUI commands with caller-owned or
  lifecycle-owned cancellable contexts where practical.
- Keep presentation strings in the frontend; wrap operational errors with useful
  shared context.

Exit criterion: the existing TUI uses the extracted services for the selected ECS
and log workflows, with no observable behavior regression.

### Phase 2: Read-only GTK workflow

- Initialize configuration and the shared AWS client from the GUI command.
- Implement the cluster and service `GtkColumnView` models.
- Implement master/detail navigation with a sidebar or split-pane layout.
- Add filtering, selection preservation, loading states, empty states, and errors.
- Add manual refresh and configured periodic refresh.
- Cancel stale requests and ignore results from superseded selections.
- Keep every GTK mutation on the GTK main thread.

Exit criterion: cluster -> service -> detail browsing is usable with mouse or
keyboard and remains responsive during slow or failed AWS calls.

### Phase 3: Logs and one safe mutation

- Resolve log groups and streams through the shared log service.
- Load recent events and follow new events with explicit cancellation.
- Bound retained log history so long sessions do not grow indefinitely.
- Add pause/follow, search, copy, and clear behavior if time permits.
- Implement force deployment with confirmation and visible success/error feedback.
- Refresh affected service data after a successful mutation.

Exit criterion: the GUI demonstrates both a continuous read workflow and a guarded
write workflow using exactly the same backend operations as the TUI.

### Phase 4: Evaluate and document

- Measure startup time, idle memory, log-follow memory growth, and refresh behavior.
- Run under native Wayland and verify the surface is not using XWayland.
- Test at common Hyprland tile sizes and at 1.0 and fractional display scales.
- Record distro build/runtime dependencies and produce a repeatable development
  build command.
- Document binding problems, workarounds, and any shared-core design friction.
- Decide whether to proceed, change toolkit, or stop after the proof of concept.

## UI outline

The first layout should favor the dense, keyboard-first character of the TUI:

```text
+ Modes -------+ Resource list ----------------+ Inspector -----------+
| ECS          | service-api       healthy     | Overview             |
| CloudWatch   | service-worker    degraded    | Deployments          |
| ...          | ...                           | Tasks / Events       |
+--------------+-------------------------------+----------------------+
| profile | region | refresh state | status / errors                |
+-------------------------------------------------------------------+
```

For the proof of concept, the mode sidebar may contain only ECS. Important shortcuts:

- `Ctrl+P`: focus a command/mode switcher placeholder.
- `/`: focus filtering for the active list.
- `Enter`: open the selected item.
- `Escape`: close transient UI or return to the previous level.
- `Ctrl+R`: refresh.
- `?`: show discoverable shortcuts.

Mouse controls and context menus should supplement rather than replace shortcuts.

## Testing strategy

### Shared services

- Unit-test queries, transformations, pagination, mutation arguments, cancellation,
  and error wrapping against fake SDK interfaces.
- Use deterministic clocks for polling tests.
- Run the existing suite with `go test ./...` after every extraction.
- Do not require AWS credentials for automated tests.

### TUI

- Preserve existing view and application tests.
- Add regression tests where extracting orchestration changes message flow or
  cancellation behavior.

### GUI

- Keep filtering and row-to-model mapping in pure Go where it can be unit-tested.
- Smoke-test widget construction and lifecycle when a display is available.
- Maintain a short manual Hyprland checklist for focus, shortcuts, scaling, window
  closing, and rapid navigation during active requests.

## Build and packaging experiment

Add separate targets such as:

```text
make build       # existing TUI; no GTK dependency introduced
make build-gui   # GTK development build
make test
```

The GUI build may require a C compiler, `pkg-config`, GTK 4 development headers,
and compatible runtime libraries. Record exact Arch package names during the spike,
but avoid embedding distribution assumptions into Go code.

The TUI should retain its current simple build and cross-compilation story. GUI
cross-compilation and binary bundling are evaluation items, not PoC prerequisites.

## Risks and mitigations

### gotk4 coverage and stability

Exercise `GtkColumnView`, models, factories, shortcuts, dialogs, text/log rendering,
and main-loop handoff in Phase 0. Stop early if required APIs are missing or unstable.

### Accidental duplicate business logic

Require the TUI to consume each extracted service before the GUI does. A GUI package
must not call AWS SDK clients directly for operations already represented by a
shared service.

### Over-generalizing the shared layer

Extract only the ECS/log vertical slice. Prefer concrete small services over a
framework, event bus, or universal resource abstraction.

### UI thread and cancellation bugs

Give each page/selection a cancellable context. Marshal GTK changes onto its main
thread, discard stale results, and test rapid navigation and shutdown.

### Unbounded log memory

Use a configurable ring or line limit and batch UI inserts. Measure behavior during
an extended follow session.

### GTK dependency leaking into the TUI

Keep the GUI in a separate command/package and verify `go build` of the TUI on a
system or container without GTK development packages.

## Go/no-go criteria

Proceed with a broader GUI only if all of the following are true:

- The GUI runs as a native Wayland application under Hyprland.
- GTK tables, text/log views, shortcuts, dialogs, and background updates are stable.
- The TUI and GUI use the same ECS and log service implementations.
- The TUI still builds without GTK system dependencies.
- Slow AWS operations never freeze the GUI.
- Navigation and shutdown cancel work cleanly without stale updates or goroutine
  leaks.
- The log view remains responsive and memory-bounded during an extended session.
- The resulting UI retains e9s' dense, keyboard-driven identity.
- Development and runtime dependency costs are acceptable and documented.

Reconsider the toolkit if gotk4 itself is the principal blocker after the Phase 0
spike. In that case, compare a small MIQT/Qt 6 spike using the same shared services;
do not change the backend architecture merely to accommodate the frontend toolkit.

## Deliverables

- A buildable `e9s-gui` proof-of-concept executable.
- Shared ECS and log application services used by both frontends.
- Passing automated tests for the extracted logic and existing TUI.
- A repeatable GUI development/build setup.
- Notes with measurements, binding limitations, screenshots, and the final go/no-go
  recommendation.
