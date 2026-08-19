# Experimental GTK GUI development

The GUI is an experimental GTK 4 frontend for e9s. It is
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

## Startup and module selection

The Module Rail starts collapsed. With no `defaults.default_mode`, the GUI opens
a module picker; choosing a module expands only that module and opens its default
sub-item. The current defaults are **Clusters** for ECS, **Log groups** for
CloudWatch Logs, **All alarms** for CloudWatch Alarms, **Parameters** for SSM
Parameter Store, and **Secrets** for Secrets Manager. Use `Ctrl+P` to reopen the
picker from anywhere.

The GUI honors the same implemented-module names and aliases as the TUI, including
`ECS`, `CWL`/`CW`/`cloudwatch`, `CWA`, `SSM`, and `SM`/`secrets`. For example:

```yaml
defaults:
  default_mode: CWL
```

A configured or command-line cluster takes precedence over `default_mode` and
opens ECS directly, matching the TUI startup contract. An unknown or not-yet
implemented GUI default leaves the application at the picker and reports the
configuration problem in the status bar.

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
go test -tags gui ./...
go test -race -tags gui -gcflags=all=-d=checkptr=0 \
  ./internal/gui ./internal/service
go vet ./...
go build -tags gui ./cmd/e9s-gui
go build -tags "gui vte" ./cmd/e9s-gui
```

## Current scope

The GUI provides cluster, service, service-task, and standalone-task browsing,
including recent stopped-task diagnostics for both service and standalone
tasks loaded in 50-task pages;
task-definition inspection, diffing, environment lookup, and revision editing;
service and per-container logs; CloudWatch Logs browsing, searches, saved
destinations, and follow workflows; CloudWatch Alarm browsing and guarded
operations; SSM Parameter Store browsing, saved prefixes, decrypted value
inspection, and guarded multiline editing; Secrets Manager metadata browsing,
saved filters, guarded JSON/plaintext reveal, editing, and cloning; service-wide and selected-task
metrics; scaling and guarded ECS mutations; and ECS Exec in an embedded VTE
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
19. Launch without `defaults.default_mode`; confirm every Module Rail section is
    collapsed and the module picker appears. Open each module in turn and confirm
    only it expands and its documented default sub-item is selected.
20. Set each implemented `defaults.default_mode` alias, then relaunch and confirm
    the picker is skipped. Repeat with `--cluster` and confirm ECS wins. Use
    `Ctrl+P` from a nested view and confirm choosing a module returns to its
    default sub-item.
21. Move through cluster, service, task, and standalone contexts and confirm the
    header shows only applicable action groups; unavailable capabilities remain
    visible but disabled within an applicable context.
22. Open one service's task browser, navigate away, and open another service.
    Confirm the previous task rows and selection disappear before the new AWS
    response arrives. Repeat while switching from tasks to Task Defs.
23. Expand CloudWatch Logs, browse groups and streams, then peek and follow a
    stream and a whole group. Confirm Browser context remains stable while the
    Workspace switches between details and logs.
24. Search one group, comma-separated groups, and comma-separated streams with
    relative presets and a custom UTC range. Exercise Older, Newer, and
    Correlate at cursor, including while the buffered filter is active.
25. Cycle Local, UTC, and Relative timestamps, copy and save the buffer, and
    confirm wrapped continuations remain aligned with the message text.
    Repeat with a stream that has been quiet for more than 15 minutes: its last
    10 lines should appear, Older should continue before their first timestamp,
    and Resume should follow from their newest timestamp without duplicates.
26. Save a group/stream destination and a complete search. Rename, reorder, and
    delete it through Manage saved, verify exact Module Rail highlighting, then
    edit `log_paths` externally and press Refresh to verify the rail reloads.
27. Rapidly switch between ECS, Log groups, streams, and saved searches while
    requests or follow polling are active. Confirm old rows clear immediately
    and no stale result replaces the new Browser or Workspace context.
28. Expand CloudWatch Alarms and switch among All alarms, In alarm, OK, and
    Insufficient data. Confirm each exact rail sub-item highlights, old rows
    clear before the new request returns, and filtering matches alarm name,
    metric, and namespace.
29. Select alarms rapidly and confirm only the final selection's configuration,
    dimensions, actions, and recent history appear in the Workspace Pane. Toggle
    Local/UTC time and verify both the Browser and Workspace update together.
30. Enable or disable an alarm's actions and exercise a manual state override in
    a safe test account. Confirm dialogs clearly describe the operation, buttons
    remain disabled while pending, and an alarm leaves a state-filtered scope
    when its new state no longer belongs there.
31. Expand SSM Parameter Store, open `/`, then browse a custom nested path.
    Confirm old Browser and Workspace content clears before each request and the
    Parameters rail item remains highlighted for unsaved paths.
32. Save the current path, open its dynamic SAVED PREFIXES rail item, and delete
    it through Saved prefixes. Confirm its tooltip shows the path, exact active
    highlighting follows navigation, and Refresh imports external
    `ssm_prefixes` configuration changes.
33. Filter String, StringList, and SecureString parameters. Confirm ordinary
    values are searchable while SecureString plaintext is neither displayed nor
    searched in the Browser or metadata Workspace.
34. View and edit a disposable SecureString. Confirm reveal requires an explicit
    warning, the multiline editor shows decrypted content only after that read,
    update requires a second confirmation, action buttons remain disabled and
    auto-refresh pauses while pending, and the new version appears afterward.
35. Expand Secrets Manager and open Secrets. Confirm selection shows metadata
    and a masked value without making a value request. Filter by name,
    description, ARN, and tags.
36. Open a custom name-substring filter, save it, and select its dynamic rail
    item. Confirm exact highlighting, deletion, and external `sm_filters` reload.
37. Reveal a disposable JSON secret and confirm the warning appears before the
    plaintext read, JSON is formatted, and changing selection clears the value.
    Repeat with a binary secret and confirm no bytes are displayed.
38. Edit and clone a disposable text secret. Confirm each initial read and final
    mutation requires confirmation, editors wrap multiline content, pending
    operations disable contextual actions and pause refresh, Clone states that
    tags/policies are not copied, and Copy ARN never reads the value.

See [`gui-poc-results.md`](gui-poc-results.md) for the measurements, limitations,
and recommendation from the initial experiment.

The agreed module order, CloudWatch Logs phases, and Module Rail behavior for
saved searches are recorded in the [`GUI expansion roadmap`](gui-roadmap.md).
Cross-platform constraints and the future Windows/macOS packaging spike are
recorded in the [`GUI portability notes`](gui-portability.md).
