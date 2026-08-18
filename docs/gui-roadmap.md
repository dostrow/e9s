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

Reassess the remaining modules after Lambda rather than fixing their order now.
At that point the GUI will have exercised streaming data, operational state,
configuration, secrets, and an executable compute resource.

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
