# GTK GUI expansion roadmap

This roadmap records the agreed order for expanding the GTK frontend after the
ECS proof of concept. The TUI remains first-class, and both frontends must use
the same UI-neutral connection, query, and mutation services.

## Module order

Implement and evaluate modules in this order:

1. **CloudWatch Logs** — reuse the existing GTK log workspace and validate that
   the shared-service boundary generalizes beyond ECS.
2. **CloudWatch Alarms** — build on the service metrics and alarm presentation
   already used by ECS.
3. **SSM Parameter Store** — establish the common browser, detail, editing, and
   saved-filter patterns for configuration data.
4. **Secrets Manager** — reuse the SSM patterns while preserving explicit
   confirmation and careful handling of sensitive values.
5. **Lambda** — reuse CloudWatch Logs, environment-variable inspection, secret
   resolution, and the embedded editor.
6. **CodeBuild** — reuse the complete CloudWatch Logs workspace, environment
   presentation, and pending-operation controls for build history and actions.
7. **EC2** — reuse the embedded terminal for Session Manager while adding rich
   infrastructure details and guarded lifecycle operations.
8. **ECR** — add repository/image drill-down, vulnerability findings, scanning,
   clipboard integration, and guarded deletion.
9. **RDS** — add cluster/instance drill-down and build on the shared CloudWatch
   time-series chart foundation.
10. **S3** — add bucket searches, hierarchical object browsing, metadata/tags,
    and native cancellable downloads.
11. **DynamoDB** — add table and item browsing, paged scans, saved tables and
    PartiQL queries, and guarded item mutations.
12. **SQS** — add queue/message drill-down, DLQ navigation, and guarded message
    send/delete workflows.
13. **Route53** — add hosted-zone and record-set browsing with carefully scoped
    record mutations.
14. **OpenTofu/Terraform** — adapt workspace, state, plan, and apply workflows
    to the native editor, terminal, and long-running-operation surfaces.

The first reassessment was completed after Lambda. CodeBuild, EC2, and ECR form
the next **compute and delivery** batch: together they exercise log-heavy jobs,
interactive host sessions, nested resource browsers, and destructive actions.
The full ordered module expansion is complete, including the final
OpenTofu/Terraform data-and-workflow module. Further work can now be prioritized
as cross-module refinement rather than basic frontend coverage.

## Infrastructure and data expansion

The next completed batch adds ElastiCache, API Gateway, and SQL Workbench to
both frontends. ElastiCache covers replication groups, clusters, serverless
caches, topology, configuration, and metrics. API Gateway covers REST, HTTP,
WebSocket, and custom-domain resources with their stages, routes,
integrations, mappings, and metrics.

SQL Workbench uses a UI-neutral PostgreSQL execution layer. GTK presents
connection-scoped tabs and a vertically split editor/results workspace; the
TUI presents the same saved connections, restored tabs, inline editor, query
execution, reconnection, and CSV export in terminal-native form. Authentication
may come from ordered global and per-connection `.pgpass` files, an ephemeral
prompt, RDS IAM, Secrets Manager, the RDS Data API, or an optional SSM tunnel.
Only query text and tab metadata are persisted. Results, passwords, and tokens
remain in memory, and write authorization is always relocked at process start.

## Cost Explorer

Cost Explorer is a shared GUI/TUI module with lazy Overview, Breakdown,
Anomalies, and opt-in Resources views. Configured `cost_views` appear as saved
sub-items. Query results live only in process memory, expire after 24 hours,
and are discarded when e9s exits. Cache keys include the effective AWS
session, billing view, date range, metric, grouping, filters, and resource/
forecast options.

The GUI renders those cached points as a theme-aware Cost and Usage chart in
stacked-bar, grouped-bar, or line mode. It keeps the highest-cost groups
visible, combines the remainder as `Others`, and aggregates display buckets
from daily to weekly or monthly for longer ranges. Chart-mode changes, hover
inspection, and maximization are local presentation operations and do not make
additional paid API requests.

The module never auto-refreshes. Ordinary navigation and Refresh reuse a valid
cache entry. **Force paid refresh** is the only cache-bypass path and always
requires confirmation that Cost Explorer charges $0.01 per paginated API
request, including the minimum request count for the active view. The UI shows
cache time, data-through time, and actual page count. Resource-level results
are explicitly opt-in, limited to 14 days, and may lag aggregate data.

## Cost-conscious refresh policy

Both frontends share conservative refresh classes instead of applying the
configured base interval indiscriminately:

- operational ECS and CodeBuild views may use the base interval, with a
  five-second floor;
- ordinary inventory views use at least 30 seconds;
- request-metered S3, SQS, Secrets Manager, and Parameter Store views use at
  least 60 seconds;
- CloudWatch metric dashboards use at least 60 seconds; and
- DynamoDB item scans, saved log searches, OpenTofu workspaces, editors, and
  terminals refresh only in response to an explicit user action.

Automatic requests are coalesced so a slow response cannot be overlapped by
the next tick. The GUI pauses polling while inactive, while Settings is open,
or after the configured inactivity timeout; explicit log following observes
the same inactivity and window-visibility gate. Terminal and Settings input do
not count as AWS-view activity. A visible GUI control can pause automatic
polling without disabling manual Refresh.

The AWS client records service/operation request counts for the process
lifetime. Settings reports the observed count and only estimates charges whose
request price is sufficiently direct; regional S3 pricing, SQS free-tier
effects, DynamoDB capacity, and transfer charges are deliberately not guessed.
`defaults.cost_guard_usd` pauses automatic polling once this conservative known
session estimate reaches its configured threshold. It defaults to one dollar;
zero disables it. `defaults.idle_timeout` defaults to 300 seconds, while an
explicit zero continues to mean never pause for inactivity.

## Local terminal tabs

The GUI-only terminal dock owns independent VTE sessions behind a scrollable
tab strip. Each tab owns a nested split tree, so terminals can be split right
or down repeatedly. A new tab inherits the active OpenTofu workspace directory
when one is selected and otherwise starts in the e9s process directory. A split
inherits the focused terminal's current directory when VTE knows it, falling
back to that session's starting directory. Hiding the dock does not stop its
shells; closing a pane or tab does and therefore requires confirmation while
its shell is running. Font, palette, and zoom changes apply to every session.
`Ctrl+Shift+T` opens a tab, `Ctrl+Shift+W` closes the focused pane,
`Ctrl+PageUp`/`Ctrl+PageDown` move between tabs, `Ctrl+Alt+R` and `Ctrl+Alt+D`
split right and down, and `Ctrl+Alt+Arrow` moves through panes in visual order.
When a shell exits, its pane closes and the split tree collapses automatically;
exiting the final shell also hides the empty drawer. Sessions and layouts are
not restored after the application exits.

## Shared metrics foundation

- Preserve complete timestamped CloudWatch series instead of retaining only a
  latest scalar sample.
- Keep query construction, pagination, de-duplication, ordering, scaling, and
  adaptive resolution UI-neutral.
- Render the same data as theme-derived Cairo charts in the GUI and compact
  terminal sparklines in the TUI.
- Support 15-minute, 1-hour, 6-hour, 24-hour, 7-day, 2-week, and 30-day GUI ranges without
  replacing or flashing unrelated Workspace Pane content.
- ECS service/task, EC2 instance, and RDS instance dashboards share this path.
  Enhanced RDS monitoring and Database Insights remain capability-dependent
  follow-ups and are never enabled implicitly.

Each module should be delivered in independently committed phases. A phase is
complete only when its shared service behavior, GUI state transitions, stale
response protection, tests, and relevant Hyprland smoke checks pass.

After CloudWatch Logs, run the Windows and macOS spike defined in the
[`GUI portability notes`](gui-portability.md). This checkpoint informs eventual
distribution plans without blocking Linux module development.

## CloudWatch Logs navigation decision

CloudWatch Logs is a collapsible top-level Module Rail entry. Its default fixed
sub-item is **Log groups**. Individual saved searches are dynamic sub-items in
the same module, because they are durable destinations rather than actions on
the current page.

The intended hierarchy is:

```text
▾ CloudWatch Logs
    Log groups
    SAVED SEARCHES
    API errors
    Deployment failures
    Request correlation
```

`SAVED SEARCHES` is a non-selectable section label and is omitted when no saved
searches exist. Each saved search is directly selectable; it is not hidden
behind a separate saved-search browser. The Module Rail must scroll
independently so a growing saved-search list cannot displace or hide other
modules.

Selecting a saved search must:

1. mark that exact rail item as active;
2. cancel requests and polling owned by the previous context;
3. clear context-dependent Browser and Workspace content immediately;
4. restore the saved group/stream scope, filter expression, and time range; and
5. execute the search as the explicit result of that selection.

Selecting **Log groups**, changing to an unsaved scope, or switching modules
removes the saved-search highlight. Saving, renaming, reordering, or deleting a
search updates the rail without restarting the application. Management actions
belong in contextual menus or the applicable Header Bar state, not as a row of
permanent buttons beside every rail item.

Existing `log_paths` entries remain backward compatible. Entries that contain
only a group, stream, or multi-group scope appear as saved destinations: their
rail item opens that scope but does not invent a filter or time range. New
fully specified saved searches persist enough information to reproduce and run
the query.

## CloudWatch Logs delivery phases

All five phases were implemented on the GTK proof-of-concept branch. The
module now shares group, stream, range, filter, and follow routing with the TUI;
the frontend owns only GTK navigation, presentation, and dialogs.

### Phase 1: shared browsing service and module shell

- Add UI-neutral log-group and log-stream listing services used by both TUI and
  GUI.
- Add the collapsible CloudWatch Logs Module Rail entry and fixed **Log groups**
  sub-item.
- Browse and filter log groups in the Browser Pane.
- Clear both panes on context changes and reject stale list responses.

### Phase 2: streams and existing log workspace

- Drill from a group into its streams.
- Support stream peek, stream follow, and whole-group follow.
- Reuse the existing bounded log buffer, wrapping, timestamps, pause/follow,
  copy, clear, and export behavior in the Workspace Pane.
- Preserve Browser context while the Workspace switches between details and
  logs.

### Phase 3: search workflow

- Support single- and multi-group or stream selection.
- Add filter expressions, relative time presets, and custom UTC ranges.
- Render bounded search results in the existing log workspace.
- Add older/newer retrieval and correlation navigation where applicable.

### Phase 4: saved searches in the Module Rail

- Render existing saved log destinations as dynamic sub-items.
- Persist complete saved searches without breaking existing `log_paths`
  configuration.
- Add save, rename, reorder, and delete workflows.
- Keep active-item highlighting synchronized with navigation and configuration
  reloads.

### Phase 5: parity and evaluation

- Compare the GUI workflow with the complete TUI CloudWatch Logs feature set.
- Exercise rapid navigation, cancellation, polling, large result sets, zoom,
  keyboard focus, theme behavior, and long-running follow sessions.
- Update the development guide and record any intentional frontend differences.

The parity pass retained two intentional frontend differences. Multi-scope
selection is entered as a comma-separated scope in the GUI search dialog rather
than using terminal-style marked rows, and buffered filtering hides non-matches
instead of providing `n`/`N` match navigation. GTK selection, cursor placement,
and the Correlate action provide the corresponding graphical navigation.

## CloudWatch Alarms delivery phases

CloudWatch Alarms is implemented in the GTK frontend and shares its domain
values, validation, ordering, error context, and mutations with the TUI through
`service.Alarms`.

### Phase 1: shared service

- Move alarm summaries, details, and history into the UI-neutral model package.
- Centralize state validation, newest-first ordering, detail lookup, action
  enablement, and manual state overrides.
- Migrate the TUI alarm workflows to the shared service.

### Phase 2: module shell and browser

- Add a collapsible **CloudWatch Alarms** Module Rail entry with **All alarms**,
  **In alarm**, **OK**, and **Insufficient data** sub-items.
- Browse and filter alarms by name, metric, or namespace.
- Clear stale Browser and Workspace state immediately when scopes change and
  reject late responses using the common request-generation guard.

### Phase 3: detail and operations

- Load configuration, dimensions, action destinations, and recent history into
  the Workspace Pane when an alarm is selected.
- Support local and UTC timestamp presentation.
- Provide contextual action enable/disable and manual state override controls.
- Require explicit dialogs, disable long-running actions while pending, and
  refresh the selected alarm after successful mutations.

## SSM Parameter Store delivery phases

SSM Parameter Store is implemented in the GTK frontend and shares path
normalization, deterministic ordering, decrypted detail lookup, error context,
and updates with the TUI through `service.SSM`.

### Phase 1: shared service

- Move parameter values into the UI-neutral model package.
- Centralize path validation and normalization, name ordering, decrypted detail
  reads, mutation validation, and contextual errors.
- Migrate TUI listing, value inspection, and editing away from direct AWS client
  calls.

### Phase 2: module shell, browser, and saved prefixes

- Add an alphabetically positioned, collapsible **SSM Parameter Store** Module
  Rail entry with a fixed **Parameters** item.
- Render configured `ssm_prefixes` as dynamic **SAVED PREFIXES** sub-items with
  active highlighting, tooltips, direct opening, save, deletion, and explicit
  configuration reload.
- Browse custom normalized paths, filter by name/type/non-sensitive value, and
  mask SecureString content in the Browser and metadata Workspace.
- Clear old rows immediately and reject late responses through the common
  cancellation generation guard.

### Phase 3: values and editing

- Keep ordinary selection metadata-only and perform value reads only through an
  explicit View action or row activation.
- Require a warning confirmation before revealing decrypted SecureString
  content and clear revealed content when selection or module context changes.
- Edit potentially multiline values in a wrapped GTK text buffer, require a
  second review confirmation, and disable actions while the mutation is pending.
- Pause automatic refresh while reads or writes are pending, then refresh list
  metadata and selected detail after a successful update.

The GUI intentionally adds explicit SecureString reveal confirmation and a
multiline editor. The TUI retains its compact Enter/value and input-dialog
metaphors, but now obtains decrypted values and performs writes through the same
shared service.

## Secrets Manager delivery phases

Secrets Manager is implemented in the GTK frontend and shares substring
filtering, deterministic ordering, explicit value reads, creation, updates, and
contextual errors with the TUI through `service.Secrets`.

### Phase 1: shared service

- Move secret metadata and current values into the UI-neutral model package.
- Centralize true case-insensitive substring filtering, name ordering, input
  validation, detail reads, creation, updates, and error context.
- Migrate TUI list, reveal, edit, and clone workflows away from direct AWS client
  calls and detached background contexts.

### Phase 2: module shell, browser, and saved filters

- Add an alphabetically positioned, collapsible **Secrets Manager** Module Rail
  entry with a fixed **Secrets** item.
- Render configured `sm_filters` as dynamic **SAVED FILTERS** sub-items with
  active highlighting, tooltips, direct opening, save, deletion, and explicit
  configuration reload.
- Browse and locally filter metadata by name, description, ARN, or tags without
  retrieving or searching secret values.
- Clear old rows immediately and reject late responses through the common
  cancellation generation guard.

### Phase 3: guarded values, editing, and cloning

- Keep row selection metadata-only; require an explicit warning before each
  first plaintext read for Reveal, Edit, or Clone.
- Pretty-print JSON values, identify binary values without displaying their
  bytes, and clear revealed plaintext on context changes.
- Edit and clone through wrapped embedded text editors followed by a second
  confirmation. Do not imply that clone copies tags or resource policies.
- Copy ARNs without reading values, disable applicable actions while requests
  are pending, pause automatic refresh, and reload metadata/detail following a
  successful mutation.

The GUI intentionally uses embedded editors and confirmation gates instead of
the TUI's external `$EDITOR` workflow. Both presentation layers execute the same
shared value, update, and create operations.

## Lambda delivery phases

Lambda is implemented in the GTK frontend and shares function discovery,
configuration reads, environment resolution, deployment download, and code
updates with the TUI through `service.Lambda`.

### Phase 1: shared service

- Move function configuration into the UI-neutral model package.
- Centralize substring filtering, deterministic ordering, detail lookup,
  environment resolution, package-type checks, deployment downloads, archive
  creation, updates, and contextual errors.
- Migrate the TUI Lambda workflows away from direct AWS client calls.

### Phase 2: module shell, browser, and saved searches

- Add an alphabetically positioned, collapsible **Lambda** Module Rail entry
  with a fixed **Functions** default item.
- Render configured `lambda_searches` as dynamic **SAVED SEARCHES** items with
  active highlighting, direct opening, save, deletion, and config reload.
- Browse and filter loaded function metadata while immediately clearing stale
  Browser and Workspace state and rejecting late responses.

### Phase 3: configuration, environment, and logs

- Fetch current configuration when a function is selected and refresh it while
  preserving the active Workspace presentation.
- Display environment references first; require an explicit warning before
  resolving and rendering SSM or Secrets Manager values.
- Reuse the shared CloudWatch workspace for function follow and search, and
  provide direct navigation to the function's log-group streams.
- Disable contextual operations and pause automatic refresh while a Lambda
  request is pending.

### Phase 4: embedded ZIP editor

- Download ZIP deployments into an application-owned temporary directory and
  expose UTF-8 text files through a Workspace-pane file selector and editor.
- Preserve binary and oversized files unchanged in the deployment archive.
- Protect unsaved changes, require explicit upload confirmation, reject path
  traversal, clean temporary data on every exit path, and refresh configuration
  after a successful `UpdateFunctionCode` request.
- Treat container-image functions as read-only and explain why the ZIP editor
  is unavailable.

The GUI editor deliberately supports text files up to 2 MiB each; other package
content is preserved but not rendered. The TUI continues to launch `$EDITOR`
against the extracted directory, while both frontends use the same download,
archive, and update service operations.

Task-definition JSON and editable Lambda text files use the shared GtkSourceView
editor when built with the `sourceview` tag. It detects Lambda languages by file
name and provides theme-aware syntax highlighting, line numbers, current-line
and bracket highlighting, auto-indent, and undo/redo behavior. Basic GUI builds
retain the plain `GtkTextView` fallback. LSP process management remains deferred;
the shared document abstraction provides the path and language boundary needed
for a later opt-in client without coupling it to Lambda or ECS workflows.

## CodeBuild delivery phases

CodeBuild is implemented in the GTK frontend and shares project/build
discovery, detail retrieval, start/stop validation and mutations, and
CloudWatch log access with the TUI through `service.CodeBuild` and
`service.Logs`.

### Phase 1: shared service

- Move project, build summary, build detail, phase, environment, and log-source
  values into the UI-neutral model package.
- Centralize project filtering and ordering, newest-first build history, detail
  lookup, start/stop validation, mutations, and contextual errors.
- Migrate the TUI CodeBuild workflows away from direct AWS client calls and
  detached background contexts.

### Phase 2: module shell and browser

- Add an alphabetically positioned, collapsible **CodeBuild** Module Rail entry
  with **Projects** as its default sub-item.
- Browse and filter projects, drill into a project's build history, and retain a
  clear route back to the project browser.
- Clear Browser and Workspace content immediately on every context change,
  reject stale responses, and pause refresh while requests are pending.

### Phase 3: details, logs, and guarded actions

- Render build state, source, phases and failure contexts, environment, duration,
  initiator, and log destination in the Workspace Pane.
- Open completed builds at full available log history and follow in-progress
  builds through the shared CloudWatch Logs workspace; support server-side log
  search without duplicating the log viewer.
- Start builds from project context and stop only in-progress builds. Require
  explicit confirmation, disable pending actions, and refresh the affected
  project/build context after a successful mutation.

### Phase 4: parity and evaluation

- Compare GUI behavior with the complete TUI CodeBuild workflow, including final
  log delivery when a build changes from in-progress to terminal state.
- Exercise rapid navigation, cancellation, build refresh, empty histories,
  long-running actions, theme behavior, and Module Rail error/status handling.
- Record intentional frontend differences and update the development guide and
  PoC results.

## EC2 delivery phases

EC2 is implemented in the GTK frontend and shares instance discovery, detail
composition, console output, Session Manager preparation, and validated
lifecycle operations with the TUI through `service.EC2`.

The expansion now also includes security groups, VPCs, subnets, EBS volumes,
application/network load balancers, target groups, listeners, and target
health. Resource IDs in GUI details are inline links; a single compact linked-
resource menu preserves keyboard and accessibility navigation without allowing
the Workspace toolbar to grow with the number of relationships. The TUI uses
`o` to open the equivalent linked-resource picker.

### Phase 1: shared service

- Centralize instance discovery, filtering and ordering, detail composition,
  console-output retrieval, Session Manager command construction, lifecycle
  validation and mutations, and contextual errors.
- Migrate the TUI EC2 workflows to the shared service.

### Phase 2: module shell, browser, and details

- Add a collapsible **EC2** Module Rail entry with **Instances** as its default.
- Browse instances and render metadata, networking, security groups, EBS
  volumes, and tags in the Workspace Pane.
- Apply the common immediate-clear, cancellation-generation, and refresh rules.

### Phase 3: sessions, console output, and lifecycle actions

- Reuse the embedded VTE workspace for SSM sessions on eligible running
  instances and expose serial console output as a non-live text workspace.
- Add contextual start, stop, reboot, and terminate actions with explicit
  confirmations; make termination visually distinct and never the default.
- Disable actions while pending and refresh selected instance state after
  successful mutations.

### Phase 4: parity and evaluation

- Compare the GUI with the complete TUI EC2 workflow, including filtering,
  instance details, console output, Session Manager, and lifecycle validation.
- Exercise rapid navigation, cancellation, terminal disconnect, long-running
  actions, confirmation defaults, and Module Rail error/status handling.
- Record intentional frontend differences and update the development guide and
  PoC results.

### Cross-resource follow-ups

- ECS task details resolve and link their actual ENI/subnet, VPC, security
  groups, EC2 container instance, ECS-managed EBS volumes, and the parent
  service's target groups. EC2 enrichment is best-effort so a missing optional
  `ec2:Describe*` permission does not make the ECS task browser unusable.
- RDS details link VPC and security-group identifiers in the TUI. Apply the same
  links when RDS reaches the GUI.
- When shared Lambda and CodeBuild models retain their VPC configuration, expose
  their subnet and security-group identifiers through this same shared
  `ResourceRef` navigation path. Do not add frontend-only AWS lookups.

## IAM context and policy simulation

Do not label a resource detail section **Computed permissions**. AWS
authorization is evaluated for a principal, action, resource, and request
context; identity policies, resource policies, permissions boundaries,
session policies, Organizations controls, conditions, and service-specific
mechanisms can all affect the result. A resource alone therefore has no single
complete permission set.

Use this staged design when an IAM module is implemented:

1. Add an **IAM context** section to applicable resource details. Show the
   attached or acting principals already present in service metadata: EC2
   instance-profile role, ECS task and execution roles, Lambda execution role,
   CodeBuild service role, and ECS infrastructure role. Make role identifiers
   linked IAM resources.
2. In the IAM module, show role trust policy, attached managed policies, inline
   policies, and permissions boundary as inspectable source documents. Show
   resource policies separately on resource types that support them.
3. Offer an explicit **Simulate access…** workflow. Require or infer a principal,
   select a curated or user-entered action set, target the current resource ARN,
   and request any missing condition context. Render allowed, implicit-deny, and
   explicit-deny results with matched statements and missing context values.
4. Mark every result as a simulation, not an authoritative statement of live
   access. The IAM API evaluates supplied action/resource combinations and does
   not automatically retrieve resource policies; resource-policy simulation is
   limited, and Organizations or service-specific controls can make simulated
   and live results differ. See the AWS
   [`SimulatePrincipalPolicy` API](https://docs.aws.amazon.com/IAM/latest/APIReference/API_SimulatePrincipalPolicy.html)
   and [policy simulator limitations](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_testing-policies.html).

This keeps ordinary resource details useful without granting broad IAM policy-
read permissions, while making deeper authorization diagnostics an explicit,
auditable action.

## ECR delivery phases

ECR is implemented in the GTK frontend and shares repository/image discovery,
finding retrieval, scan validation, URI construction, and digest deletion with
the TUI through `service.ECR`. The Module Rail exposes Repositories as its
default destination; repository, image, and finding depth remains in the
Browser Pane so the rail does not grow with transient registry content.
Both frontends accept basic-scan `findings` and Amazon Inspector-backed enhanced
`enhancedFindings`; an `ACTIVE` continuous scan is readable but cannot be
manually restarted. Because `DescribeImages` omits enhanced scan status and
severity counts, enhanced findings are retrieved on demand when an image is
selected in the GUI or opened in the TUI rather than by issuing one scan request
per image in the repository. Unknown table values remain visibly unknown until
that request completes, and cached summaries survive ordinary list refresh.

### Phase 1: shared service

- Centralize repository and image discovery, deterministic ordering, scan
  summaries/findings, scan and delete validation, mutations, and contextual
  errors, then migrate the TUI to that service.

### Phase 2: repositories, images, and findings

- Add a collapsible **ECR** Module Rail entry with **Repositories** as its
  default, then drill through repositories, images, and vulnerability findings.
- Preserve severity ordering and clear stale nested content immediately while
  navigation or refresh requests are pending.

### Phase 3: image operations

- Copy complete image URIs without additional AWS calls.
- Start scans only when image state permits and refresh scan status while it is
  active without replacing the user's current Workspace content.
- Require explicit confirmation before digest deletion, disable pending actions,
  and return to the refreshed image browser after success.

All three phases are complete on the GTK proof-of-concept branch. Repository,
image, and finding context clears before asynchronous loads; Back and refresh
preserve valid selections; scan findings retain severity ordering; and scan and
delete operations display Workspace progress while automatic refresh is paused.

## S3 delivery phases

S3 is implemented in the GTK frontend and shares bucket filtering, deterministic
folder-first ordering, object metadata/tag lookup, key-prefix search, validation,
and download routing with the TUI through `service.S3`. The Module Rail exposes
**Buckets** as the default destination and configured `s3_searches` as dynamic
**SAVED SEARCHES** sub-items.

### Phase 1: shared service and TUI migration

- Move bucket, object, metadata, download request/result, and progress values into
  the UI-neutral model package.
- Centralize bucket filtering, folder-first ordering, validation, contextual
  errors, and object-versus-prefix download routing.
- Migrate the TUI S3 workflow to the shared service and caller-owned context.

### Phase 2: module shell, buckets, and saved searches

- Add an alphabetically positioned, collapsible **S3** Module Rail entry with
  **Buckets** as its default item.
- Render configured searches as dynamic sub-items with active highlighting,
  direct opening, save, rename/filter editing, deletion, and config reload.
- Browse and locally filter buckets while clearing stale Browser and Workspace
  content and rejecting late responses through the request-generation guard.

### Phase 3: object navigation and detail

- Drill into a folder-first object browser and walk Back through parent prefixes
  before returning to the selected bucket.
- Support local relative-name filtering and flat server-side key-prefix search.
- Load object metadata and sorted tags on selection while prefixes remain
  navigable rather than triggering object metadata calls.

### Phase 4: native downloads

- Use native save and directory choosers for object and recursive-prefix
  downloads.
- Report cumulative bytes, completed files, and current key in the Workspace;
  disable conflicting S3 actions and provide explicit cancellation while pending.
- Write individual files atomically, remove partial temporary files after failure
  or cancellation, and reject keys that would escape the selected directory.

### Phase 5: parity and evaluation

- Verify both frontends compile against and exercise the same S3 service while
  retaining their native navigation and destination-selection metaphors.
- Cover filtering, ordering, breadcrumb/parent-prefix behavior, metadata/tag
  presentation, progress routing, cancellation-safe writes, and path safety.
- Record the Hyprland smoke workflow in the GUI development guide.

The intentional frontend differences are presentational. The TUI uses its picker,
key bindings, and configured save-directory input; GTK renders saved searches in
the Module Rail and uses platform-native file/folder choosers plus a cancellable
Workspace progress bar. Both invoke the same service methods and AWS adapter.

## DynamoDB delivery phases

DynamoDB is implemented in the GTK frontend and shares table discovery,
configuration, scan/filter routing, opaque pagination, PartiQL, item reads, and
guarded mutations with the TUI through `service.DynamoDB`. The Module Rail
exposes **Tables** as the default destination plus configured saved tables and
queries as dynamic sub-items.

### Phase 1: shared service and TUI migration

- Move table, schema, item, page, filter, and mutation values into the UI-neutral
  model package.
- Centralize validation, sorting, filter-operator routing, key construction, and
  contextual errors in the shared service.
- Preserve high-precision pagination keys through opaque tokens and migrate the
  TUI to caller-owned context.

### Phase 2: module shell and tables

- Add an alphabetically positioned, collapsible **DynamoDB** Module Rail entry
  with **Tables** as its default item.
- Render configured tables and PartiQL queries as directly selectable saved
  destinations with active highlighting and config reload.
- Browse/filter tables and show status, size, billing mode, key schema, and
  global secondary indexes while rejecting stale responses.

### Phase 3: items, queries, and saved destinations

- Scan table items in pages, retain cumulative scanned counts, and expose Load
  more only while an opaque next token exists.
- Apply comparison and function-style server filters, execute PartiQL, and
  locally filter the currently loaded buffer.
- Save, open, rename, edit, and delete saved tables and queries without allowing
  accidental name collisions.

### Phase 4: guarded mutations

- Edit non-key fields in a theme-aware embedded editor and re-read the item after
  a successful update.
- Clone full items in a JSON-aware editor, require a changed and complete key,
  and require explicit confirmation before writing.
- Use conditional updates and creates so concurrent field changes or existing
  item keys fail visibly instead of being silently overwritten. Apply the same
  clone guard to the TUI.

### Phase 5: parity and evaluation

- Verify plain GTK and GtkSourceView builds plus the full TUI/service suite.
- Exercise stale-response cancellation, filtering, pagination, saved-config
  reload, theme changes, and guarded writes under Hyprland.
- Keep the frontend differences presentational: GTK uses Module Rail saved
  destinations and embedded editors, while the TUI uses pickers and `$EDITOR`.

## SQS delivery phases

SQS is implemented in the GTK frontend and shares queue discovery,
configuration, receive, send, delete, and dead-letter resolution workflows with
the TUI through `service.SQS`. The Module Rail exposes **Queues** as the default
destination and configured queues as dynamic saved sub-items.

### Phase 1: shared service and TUI migration

- Move queue, counter, message, attribute, receive, and send values into the
  UI-neutral model package.
- Centralize sorting, contextual errors, receive limits, FIFO validation, and
  binary-attribute encoding in the shared service.
- Migrate the TUI to caller-owned context and the same portable JSON send
  document used by the GUI.

### Phase 2: module shell and queues

- Add an alphabetically positioned, collapsible **SQS** Module Rail entry with
  **Queues** as its default item.
- Browse and filter queues, show approximate counters and configuration, and
  reject stale list/detail responses.
- Save, open, rename, edit, and delete queue destinations as direct Module Rail
  sub-items with exact active highlighting.

### Phase 3: messages and dead-letter navigation

- Enter a queue without receiving messages automatically; make the visibility-
  changing receive operation an explicit, cancellable **Poll messages** action.
- Buffer and locally filter received messages, replace renewed receipt handles
  without duplicating messages, and render JSON bodies plus sorted attributes.
- Resolve and open configured dead-letter queues while preserving Back
  navigation to the originating queue.

### Phase 4: guarded message actions

- Send and clone messages through a theme-aware JSON editor that preserves
  binary attributes as base64 and validates FIFO group IDs and delays.
- Require a review summary and confirmation before sends; require destructive
  confirmation before deleting a received message.
- Disable conflicting controls, pause automatic refresh, and show Workspace
  progress for every pending receive or mutation. Never expose receipt handles.

### Phase 5: parity and evaluation

- Verify plain GTK, GtkSourceView, and full VTE builds plus the TUI/service suite.
- Exercise cancellation, stale-response rejection, saved-config reload, local
  filtering, DLQ navigation, FIFO sends, binary attributes, and guarded deletes
  under Hyprland.
- Keep frontend differences presentational: GTK uses contextual Header Bar
  actions and embedded editors, while the TUI retains its picker/key workflow.

## Route53 delivery phases

Route53 is implemented in the GTK frontend and shares hosted-zone discovery,
record-set listing, DNS tests, portable JSON editing, validation, and mutations
with the TUI through `service.Route53`. The Module Rail exposes **Hosted zones**
as the default destination.

### Phase 1: shared service and TUI migration

- Move hosted-zone, record-set, DNS-answer, and edit-document values into the
  UI-neutral model package.
- Centralize sorting, contextual errors, validation, routing-policy round trips,
  and apex NS/SOA protection in the shared service.
- Migrate the TUI to caller-owned context and the same portable JSON document
  used by the GUI.

### Phase 2: hosted-zone browser

- Add an alphabetically positioned, collapsible **Route53** Module Rail entry
  with **Hosted zones** as its default item.
- Browse and locally filter public and private zones by name, comment, or ID.
- Preserve selection across refreshes and reject stale responses.

### Phase 3: record sets and DNS tests

- Drill into a zone's complete record-set list, with filtering across names,
  types, values, aliases, routing policies, and set identifiers.
- Render TTL, alias, health-check, and routing metadata without flattening the
  record representation required by Route53 mutations.
- Run `TestDNSAnswer` as a pending Workspace operation and render its response
  inline with the selected record.

### Phase 4: guarded mutations

- Create and edit records in the theme-aware embedded JSON editor with a review
  confirmation before submission.
- Keep name, type, and set identifier immutable during an edit because Route53
  UPSERT would otherwise create a second record rather than rename the first.
- Require destructive confirmation for deletion and submit the exact loaded
  record set. Hide deletion for protected NS and SOA records.
- Pause refresh and disable conflicting controls for every pending action.

### Phase 5: parity and evaluation

- Verify the TUI/service suite plus plain GTK, GtkSourceView, and full VTE GUI
  builds.
- Exercise filtering, navigation, refresh preservation, DNS testing, editor
  validation, confirmations, mutation reloads, errors, and theme switching
  under Hyprland.

## OpenTofu/Terraform delivery phases

OpenTofu/Terraform is implemented in the GTK frontend and shares its validated,
context-aware command workflow with the TUI. It automatically selects `tofu`
when available and falls back to `terraform`.

### Phase 1: shared workflow and TUI migration

- Validate and normalize workspace paths, centralize binary discovery, resource
  address parsing, state reads, init, saved-plan generation, and apply commands.
- Make non-interactive operations caller-cancellable and preserve the exact
  reviewed plan artifact until apply or navigation cleanup.
- Migrate the TUI to the shared service and add explicit confirmations for init
  and apply.

### Phase 2: workspace and state browser

- Add an alphabetically positioned, collapsible **OpenTofu** Module Rail entry
  with **Workspaces** as its default item and saved workspaces as direct sub-items.
- Add, edit, open, and remove saved workspace paths without touching their files.
- Browse and locally filter parsed state resources by address, type, name, or
  module, with stable Back navigation to the workspace list.

### Phase 3: state and native plan presentation

- Load complete `state show` output into the Workspace without replacing the
  Browser's state-resource context.
- Render a native, filterable planned-change table and attribute-level add,
  remove, and before/after details without truncating the underlying values.
- Treat plans as snapshots rather than rerunning them in the automatic refresh
  loop.

### Phase 4: guarded operations

- Open or create the active workspace's `terraform.tfvars` in the theme-aware
  source editor. Save atomically, preserve existing permissions, reject stale
  on-disk changes, and discard plans whose inputs changed.
- Explain init's provider, module, backend, and lock-file effects before launch.
- Apply only the exact reviewed saved plan from the plan view, after a destructive
  confirmation that includes the plan summary.
- Stream full-build init/apply output in the theme-derived embedded VTE terminal;
  pause refresh, disable conflicting actions, expose cancellation, and refresh
  state after successful completion. Preserve a captured-output fallback for
  GUI builds without VTE.

### Phase 5: parity and evaluation

- Verify the TUI/service suite plus plain GTK, GtkSourceView, and full VTE GUI
  builds, including cancellation and plan-artifact cleanup.
- Exercise workspace management, filtering, state inspection, plan/no-change
  output, failed init/apply, reviewed apply, Back navigation, and theme switching
  under Hyprland.

## Deferred cross-cutting architecture

The module expansion is complete, but three cross-cutting capabilities are
intentionally documented before implementation so their safety boundaries do
not emerge accidentally from frontend-specific code.

### OpenTofu source freshness and plan provenance

The objective is to warn or stop a plan or apply when its workspace is behind
its Git upstream or differs from the inputs that produced the reviewed plan.
e9s must not automatically run `git pull`: pull mutates the working tree, may
merge or rebase, can prompt for credentials, and would invalidate an existing
plan without producing a newly reviewed replacement.

Implement this as a shared service used by both frontends:

- Detect whether the canonical workspace is inside a Git worktree. Workspaces
  outside Git skip this policy without error.
- Inspect tracked changes and untracked Terraform source files, run an explicit
  cancellable `git fetch`, and compare `HEAD` with the configured upstream as
  current, ahead, behind, or diverged. Detached heads and branches without an
  upstream are **unverifiable**, not current.
- Support per-workspace policy levels `off`, `advisory`, and `required`, starting
  with `advisory` as the default. Cache successful fetch results only for a
  bounded freshness interval and always show when the last check occurred.
- Attach a provenance manifest to every saved plan: canonical workspace path,
  executable, Git root and `HEAD`, fetched upstream revision, tracked-worktree
  state, and content digests for Terraform source, `.terraform.lock.hcl`,
  `terraform.tfvars`, `*.auto.tfvars`, and their JSON variants. Hash relevant
  `TF_VAR_*` and `TF_CLI_ARGS*` inputs without retaining their secret values.
- Before Apply, compare the active workspace with that manifest. The normal
  recovery from any mismatch is to discard the plan and run Plan again. Apply
  must continue to submit only the exact saved plan artifact that was reviewed.
- A required-policy failure may expose **Break glass**, but only after showing
  every failed check, requiring an explicit confirmation and a short reason,
  and marking the resulting plan/session as bypassed. It must never silently
  fetch, merge, reset, or modify workspace files.

The GUI should present this in the Workspace pending surface and plan summary.
The TUI should use the same preflight result in a confirmation modal. Network,
authentication, cancellation, and upstream-configuration failures remain
distinguishable so users do not mistake an unavailable check for a clean repo.

### IaC ownership index

The supported claim is “this AWS resource is represented in current OpenTofu
state,” not “some unapplied source file may eventually manage this resource.”
Source-code inference would require evaluating modules, variables, provider
aliases, conditionals, and `for_each`; it is both expensive and less authoritative
than state for detecting likely drift.

Build an opt-in shared ownership catalog with these constraints:

- Pull state for saved workspaces in a bounded background queue or by explicit
  refresh. Never run a state command on every Browser selection. Cache each
  result with its workspace, state lineage/serial when available, refresh time,
  and an expiry.
- Treat complete state as sensitive. Parse it transiently, retain only provider
  type, resource address, and normalized resource identities, and never place
  raw state or attribute values in logs or the persistent config.
- Prefer exact ARNs. Where AWS does not expose one, use resource-specific
  account/region/ID compound keys. Name-only or otherwise ambiguous matches are
  **possible matches**, never confirmed ownership.
- Represent four outcomes: confirmed managed, possible match, not found in a
  fresh complete index, and unknown because the index is stale, partial, or
  unavailable. Unknown must never be rendered or enforced as unmanaged.
- Add a theme-derived, clickable ownership annotation to resource details. It
  should name the workspace and resource address and use existing cross-module
  navigation/history to open that OpenTofu state entry. The TUI should expose
  the same information and a keyboard navigation action where practical.
- Begin mutation integration as advisory. A later optional strict policy may
  require a fresh exact match plus Break glass before e9s mutates a managed
  resource. Ambiguous or stale data may warn but must not block.

Pilot exact identity adapters with EC2 instances, security groups, subnets, EBS
volumes, load balancers and target groups, then ECS services, RDS resources, and
S3 buckets. Measure remote-state latency, credential failures, cache memory,
and false-match rates before expanding across every module.

### Configurable GUI editor modes

Editor modes apply only to the GTK built-in editor. The TUI already delegates
editing to the user's terminal editor and should not emulate another modal
editor internally.

Deliver the GTK work in this order:

1. Add a shared Plain-mode search bar using `GtkSourceSearchContext`: forward
   and backward navigation, case and whole-word controls, regex validation,
   replace current, replace all, highlighted matches, and occurrence counts.
2. Add `plain` and `vim` configuration modes. Vim mode should use GtkSourceView
   5's
   [`GtkSourceVimIMContext`](https://gnome.pages.gitlab.gnome.org/gtksourceview/gtksourceview5/class.VimIMContext.html)
   rather than an e9s keybinding reimplementation. Display its command bar and
   command preview, route write/quit signals through the active module's guarded
   Save and unsaved-change workflows, and test conflicts with application
   shortcuts such as Escape, Ctrl+R, Ctrl+P, and Ctrl+S.
3. Add an `external` mode backed by the GUI-only Terminal Dock/VTE. Launch a
   configurable command such as `hx` or `nvim`; edit local files directly and
   use private `0600` temporary files for remote/generated buffers. Reload the
   buffer only after a successful editor exit, then retain the module's normal
   Validate/Register/Upload/Save boundary.

Do not promise a native Helix mode at this stage. Faithful Helix behavior is
selection-first and depends on multiple simultaneous selections, which would
require a custom cursor model, rendering, edit transactions, undo integration,
registers, motions, and command system on top of GtkTextBuffer. External mode
provides actual Helix semantics without turning e9s into an editor project. LSP
integration remains an orthogonal future evaluation after these modes are
stable.
