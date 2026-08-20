# Experimental GTK GUI development

The GUI is an experimental GTK 4 frontend for e9s. It is
built as a separate `e9s-gui` executable; the normal `e9s` TUI does not compile,
link, or require GTK.

## Prerequisites

Install Go 1.24 or newer, a C compiler, `pkg-config`, the GTK 4 development
files, the GtkSourceView 5 development files, the GTK 4 VTE development files,
and AWS's `session-manager-plugin`.
The following distro package sets cover the compile-time dependencies:

### Ubuntu and Debian

```bash
sudo apt install build-essential pkg-config libgtk-4-dev libgtksourceview-5-dev libvte-2.91-gtk4-dev
```

The PoC was verified on Ubuntu 25.10 with GTK 4.20.1, GLib 2.86.0, gcc 15.2,
and `pkg-config` 1.8.1.

### Arch Linux

```bash
sudo pacman -S --needed base-devel pkgconf gtk4 gtksourceview5 vte4
```

Arch publishes the development headers, shared library, and pkg-config metadata
in its [`gtk4` package](https://archlinux.org/packages/extra/x86_64/gtk4/); its
[`pkgconf` package](https://archlinux.org/packages/core/x86_64/pkgconf/) provides
the pkg-config implementation.

### Fedora

```bash
sudo dnf install gcc pkgconf-pkg-config gtk4-devel gtksourceview5-devel vte291-gtk4-devel
```

Fedora publishes the headers and build metadata in
[`gtk4-devel`](https://packages.fedoraproject.org/pkgs/gtk4/gtk4-devel/).

## Build and run

```bash
make build-gui
./e9s-gui
```

`make build-gui` enables the `gui`, `vte`, and `sourceview` tags. It includes the
embedded ECS Exec terminal plus syntax-aware task-definition and Lambda source
editors. A GTK-only diagnostic build remains available as `make build-gui-basic`;
it uses the plain text editor fallback, disables Exec, and explains the missing
terminal feature in the GUI.

The highlighted editor uses the GtkSourceView 5 C API through its
`gtksourceview-5` pkg-config module. If detection fails, verify the development
package—not only the runtime library—is installed:

```bash
pkg-config --modversion gtk4 gtksourceview-5 vte-2.91-gtk4
```

`gtksourceview-4` is the GTK 3 generation and cannot be used by this GTK 4 GUI.

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
Parameter Store, **Secrets** for Secrets Manager, **Functions** for Lambda,
**Projects** for CodeBuild, **Instances** for EC2, **Repositories** for ECR,
**Clusters** for RDS, **Buckets** for S3, **Tables** for DynamoDB, **Queues**
for SQS, **Hosted zones** for Route53, and **Workspaces** for OpenTofu. Use
`Ctrl+P` to reopen the picker
from anywhere.

The GUI honors the same implemented-module names and aliases as the TUI, including
`ECS`, `CWL`/`CW`/`cloudwatch`, `CWA`, `SSM`, `SM`/`secrets`, and `Lambda`/`λ`.
`CB` and `CodeBuild` select the CodeBuild project browser; `EC2` and `EC2i`
select the EC2 instance browser. `ECR`, `RDS`, `S3`, `DDB`/`DynamoDB`, `SQS`,
`R53`/`Route53`, and `TF`/`Tofu`/`Terraform` select their corresponding default
browsers.
For example:

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
go test -tags "gui sourceview" ./...
go test -race -tags "gui sourceview" -gcflags=all=-d=checkptr=0 \
  ./internal/gui ./internal/service
go vet ./...
go build -tags gui ./cmd/e9s-gui
go build -tags "gui vte sourceview" ./cmd/e9s-gui
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
saved filters, guarded JSON/plaintext reveal, editing, and cloning; Lambda
browsing, saved searches, configuration/environment inspection, CloudWatch log
workflows, and guarded ZIP editing; CodeBuild project/build browsing, phase and
failure inspection, environment references, log viewing/search, and confirmed
start/stop operations; EC2 instance browsing, networking/security/storage/tag
details, console output, Session Manager, and guarded lifecycle operations;
EC2 security-group, VPC, subnet, EBS-volume, ALB/NLB, and target-group browsing
with inline cross-resource navigation; ECR repository, image, scan, and finding
browsing; RDS cluster/instance browsing and shared metrics dashboards; S3 bucket,
folder, object-metadata, key-prefix search, saved-search, and cancellable download
workflows; DynamoDB table/item browsing, paged and filtered scans, PartiQL, saved
destinations, and guarded field editing and item cloning; SQS queue/message
browsing, saved queues, explicit cancellable polling, DLQ navigation, and guarded
send/clone/delete actions; ECS-task infrastructure links;
service-wide and selected-task metrics; scaling and guarded ECS mutations; and
ECS Exec in an embedded VTE terminal.
The GTK frontend and Bubble Tea frontend both call the same UI-neutral
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
39. Expand Lambda and open Functions. Search loaded metadata, create and delete a
    saved search, confirm exact Module Rail highlighting, and verify external
    `lambda_searches` changes are reloaded on explicit refresh.
40. Select a function and confirm the Workspace loads current configuration.
    View environment references, verify no secret value is fetched initially,
    then confirm the warning gate before resolving a disposable SSM/Secrets
    Manager reference.
41. Follow and search the function log group in the shared log Workspace, then
    use Browse logs to open its streams in CloudWatch Logs. Confirm the usual
    empty-range fallback, wrapping, scrollback, highlights, and stream controls.
42. Edit a disposable ZIP function containing text and binary files. Switch
    between text files, cancel once to verify the unsaved-change warning, then
    upload after the mutation confirmation. Confirm binary content remains
    usable and the function configuration refreshes. Repeat with an image-based
    function and confirm editing is unavailable with an explanation.
43. Expand CodeBuild and open Projects. Filter project metadata, drill into a
    project's newest-first 50-build history, then use Back to return to Projects.
    Confirm Browser and Workspace content clear immediately during rapid project
    and build changes and that only the final request is displayed.
44. Select successful, failed, and in-progress builds. Confirm source, initiator,
    duration/current phase, phase failures, environment references, and log
    destination render in the Workspace. Refresh and verify the selected build
    remains selected when it still exists.
45. Open a completed build's logs and page backward through history; open an
    in-progress build and verify follow begins at the build start, receives a
    final event sharing the previous event's timestamp, and continues after the
    build reaches a terminal state. Search logs and confirm the dialog defaults
    to the exact build group, stream, and padded build time range.
46. Start a disposable project build and stop an in-progress build. Confirm both
    operations require confirmation, all CodeBuild actions and refresh remain
    disabled while pending, errors use the shared dismissible error surface, and
    the affected build context reloads after success.
47. Expand EC2 and open Instances. Filter by name, ID, IP, type, and state;
    rapidly change selections and modules, and confirm Browser and Workspace
    content clear immediately and late responses never replace the final choice.
48. Select an instance and confirm metadata, networking, security-group rules,
    EBS volumes, and tags render deterministically. Refresh and verify the
    selected instance remains selected while it still exists.
49. View serial console output, return to Details, and open Session Manager on a
    disposable running SSM-managed instance. Confirm the embedded terminal
    accepts input, resizes, and uses the shared disconnect confirmation.
50. On disposable instances, exercise start, stop, and reboot, then inspect the
    terminate confirmation without accepting it. Confirm invalid actions are
    unavailable, termination defaults to No, pending operations disable all EC2
    actions and refresh, and successful mutations reload the selected context.
51. Expand ECR and open Repositories. Confirm it is alphabetically positioned,
    Repositories is its default item, repository filtering is case-insensitive,
    and rapid repository changes clear stale image and Workspace content before
    the final AWS response arrives.
52. Drill from a repository through images and completed scan findings. Confirm
    newest images appear first, vulnerability counts and findings use severity
    order, Back preserves the prior selection, and manual/automatic refresh does
    not replace an unrelated Workspace context.
53. Copy tagged and untagged image URIs and verify the tag/digest syntax. On a
    disposable image, start a scan and confirm the action and refresh cannot be
    spammed while pending. Inspect the deletion dialog, confirm it names the
    digest and defaults to Cancel, then delete the disposable image and verify
    the refreshed Images browser is restored.
54. Expand S3 and confirm Buckets is its default item. Filter the bucket list,
    save multiple bucket searches, edit and rename one, and verify saved searches
    appear as directly selectable Module Rail sub-items with the active item
    highlighted. Delete the active search and confirm the view returns to Buckets.
55. Double-click a bucket and traverse at least three folder levels. Confirm
    folders sort before objects, Back visits each parent prefix before returning
    to the selected bucket, local filtering uses relative names, and Key prefix
    search returns flat matching keys. Rapidly change folders and selections and
    confirm stale Browser and Workspace content never reappears.
56. Select an object and verify size, content type, ETag, storage class, modified
    time, and sorted tags. Download an object with the native save dialog and a
    folder with the native directory dialog; confirm cumulative byte/file progress
    is visible and conflicting S3 actions are disabled. Cancel a large transfer
    and verify no partial destination file remains. Include an object whose key
    contains path traversal components and confirm recursive download rejects it.
57. Expand DynamoDB and confirm Tables is its default item. Filter the table
    browser, select tables rapidly, and verify only the final configuration
    appears in the Workspace. Double-click a table and confirm the old item rows
    clear before its first scan returns.
58. Apply comparison, `begins_with`, and `contains` scan filters. Load multiple
    pages and confirm Loaded and cumulative Scanned counts advance while Load
    more disappears with the final page. Run PartiQL and locally filter its
    loaded result buffer.
59. Save a table and a PartiQL query, open both from their direct Module Rail
    entries, rename/edit them through Manage saved, and delete the active entry.
    Confirm exact rail highlighting and external config reload remain correct.
60. On a disposable table, edit a non-key string and structured field and verify
    the refreshed item remains selected. Clone an item after changing its full
    key. Confirm same-key clones are rejected, confirmation precedes each write,
    actions and refresh remain disabled while pending, and a concurrent field
    change or existing clone key produces a visible conflict instead of an
    overwrite. Swap GTK themes while an editor is open and verify its palette is
    theme-derived.
61. Expand SQS and confirm Queues is its default item. Filter by queue name and
    URL, rapidly change selections, and verify only the final queue counters and
    configuration appear in the Workspace.
62. Save several queues, open them from direct Module Rail sub-items, rename and
    edit one through Manage saved, and delete the active destination. Confirm
    active highlighting and external config reload remain correct.
63. Double-click a queue and confirm no receive occurs until Poll messages is
    pressed. During a long poll, verify the Workspace shows progress, automatic
    refresh pauses, conflicting actions are disabled, and cancellation returns
    the page to an operable state. Poll twice and verify renewed receipt handles
    do not duplicate buffered messages; filter by body and message attribute.
64. Open a configured DLQ and use Back to restore the originating queue. On a
    disposable standard and FIFO queue, send and clone text, JSON, and binary-
    attribute messages, confirming invalid FIFO group IDs and base64 are rejected.
    Delete a received message and verify its receipt handle is never rendered.
65. Expand Route53 and confirm Hosted zones is its default item. Filter public
    and private zones by name, comment, and ID; rapidly change selection during
    refresh and verify stale zone details never replace the current Workspace.
66. Double-click a zone, filter record sets by name, value, alias, routing policy,
    and set identifier, and verify Back restores the selected zone. Select simple,
    alias, and routed records and confirm all values and routing metadata display.
    Run Test DNS and verify pending state, disabled controls, and the inline answer.
67. In a disposable zone, create and edit records through the JSON review flow.
    Confirm invalid TTL/value/alias combinations and identity changes are rejected,
    refresh pauses during mutations, and authoritative records reload afterward.
    Delete a disposable record and confirm NS/SOA deletion is unavailable.
68. Expand OpenTofu and confirm Workspaces is its default item. Add valid and
    invalid workspace paths, edit and remove saved entries, and confirm saved
    workspaces appear as direct highlighted Module Rail sub-items without any
    workspace files being changed.
69. Open a workspace, filter its state by address, type, and module, and
    double-click a resource to load complete state while the Browser remains in
    place. Verify Back restores the selected workspace and automatic refresh does
    not repeatedly execute local OpenTofu commands.
70. Run Plan and verify the Workspace pending indicator, disabled controls,
    native change table, full attribute diffs, no-change state, and plan snapshot
    refresh behavior. Navigate away and confirm the temporary plan is removed.
71. Confirm Init describes provider/module/backend and lock-file effects. Apply a
    disposable reviewed plan and verify only that saved plan is used, output
    streams in the embedded terminal, refresh pauses, cancellation works, and a
    successful close returns to refreshed state. Repeat without the `vte` tag and
    verify the captured-output fallback.

See [`gui-poc-results.md`](gui-poc-results.md) for the measurements, limitations,
and recommendation from the initial experiment.

The agreed module order, CloudWatch Logs phases, and Module Rail behavior for
saved searches are recorded in the [`GUI expansion roadmap`](gui-roadmap.md).
Cross-platform constraints and the future Windows/macOS packaging spike are
recorded in the [`GUI portability notes`](gui-portability.md).
