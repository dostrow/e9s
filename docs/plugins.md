# External-action plugins

External-action plugins let private repositories expose operator runbooks in
both e9s frontends. e9s owns the launch form, review, and terminal; the external
program continues to own workflow state, runtime prompts, and business logic.

Plugins are never discovered from the current repository automatically. Add,
edit, validate, or remove registrations in **Settings → Plugins** in the GUI or
the **Settings (`SET`) → Plugins** page in the TUI. Both update the shared
`~/.config/e9s/config.yaml`; it can also be edited directly:

```yaml
plugins:
  - name: My Operations       # optional local friendly-name override
    manifest: /path/to/private-repository/.e9s/plugin.yaml
```

The friendly name is shown as an expander beneath the GUI's `PLUGINS` heading
and in the TUI action list. Without it, `display_name` from the manifest is used.
Both frontends validate the complete registration set before a structured
settings save and update their plugin navigation immediately.

## Manifest

```yaml
api_version: e9s/v1alpha1
name: example-operations
display_name: Example Operations
description: Private operational runbooks

actions:
  - id: release
    title: Production release
    description: Promote candidate builds to production
    risk: production
    run:
      command: ./bin/release
      launch_directory: ..
      working_directory: ./infrastructure
      terminal: true
      login_shell: true
      arguments: [--interactive]
      environment:
        AWS_PROFILE: ${e9s.profile}
        AWS_REGION: ${e9s.region}
    inputs:
      - id: candidate
        label: Candidate tag
        type: text
        argument: --candidate
        required: true
        pattern: '^[A-Za-z0-9._-]+$'
      - id: strategy
        label: Strategy
        type: select
        argument: --strategy
        default: rolling
        options:
          - {label: Rolling, value: rolling}
          - {label: Blue/green, value: blue-green}
      - id: dry_run
        label: Validate only
        type: boolean
        argument: --dry-run
        conflicts_with: [apply]
      - id: apply
        label: Apply
        type: boolean
        argument: --apply
        conflicts_with: [dry_run]
```

The API version is intentionally strict. Unknown fields and invalid input types
are rejected instead of being silently ignored.

## Directories

Three locations are kept distinct:

- The **manifest directory** is the directory containing `plugin.yaml`.
- `launch_directory` resolves relative to the manifest directory and is the
  base for a relative `command`. It defaults to the manifest directory.
- `working_directory` becomes the child process's current working directory.
  It resolves relative to `launch_directory` when one is specified, otherwise
  relative to the manifest directory. It defaults to the launch directory.

Both resolved directories and the exact argv are shown before execution. Input
values are passed as individual arguments, never interpolated into a shell
command. A shell can still be selected explicitly as `command`, but then the
plugin author owns the resulting quoting and injection risk.

`login_shell: true` runs an action through the user's configured terminal shell
with login and interactive initialization before replacing the shell with the
plugin command. It is opt-in; omitted or false actions continue to execute the
configured command directly. e9s uses a fixed `exec "$@"` trampoline and passes
the executable and arguments as separate values, so operator input is not
interpolated into shell source. The shell must support conventional `-l`, `-i`,
and `-c` options and `$@` argument expansion, such as Bash or Zsh. In the GUI,
`gui.terminal_shell` selects the shell when configured; otherwise both
frontends use `$SHELL`, falling back to `sh`.

## Inputs and context

Supported input types are `text`, `boolean`, and `select`. Empty optional text
values are omitted. A set boolean contributes only its `argument`; text and
select inputs contribute `argument` followed by their value. Inputs without an
`argument` are positional.

`${e9s.region}` and `${e9s.profile}` can be used in defaults, base arguments,
directories, commands, and environment values. Empty environment values are
omitted. Environment overrides are merged with the process environment.

Valid risk values are `normal`, `sensitive`, `production`, and `destructive`.
Every action receives a review confirmation regardless of risk.

## Terminal behavior

The TUI temporarily yields its terminal to the plugin process. The GTK frontend
runs it in the operation terminal and retains its output and exit status. GTK
builds without embedded-terminal support reject interactive plugin launches
with a capability error. In `v1alpha1`, plugin actions always run in a terminal;
`terminal: true` records that requirement explicitly for future runner types.
