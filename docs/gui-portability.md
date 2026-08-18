# GTK GUI portability and packaging notes

This document records the constraints and decision points for shipping the GTK
frontend on Linux, Windows, and macOS. It is guidance for a future packaging
effort, not a claim that the current GUI is supported outside Linux.

## Current decision

GTK remains the right toolkit for the Linux- and Hyprland-first GUI experiment.
Choosing it does not prevent Windows or macOS releases: GTK 4 has maintained
[Win32](https://docs.gtk.org/gtk4/windows.html) and
[macOS/Quartz](https://docs.gtk.org/gtk4/osx.html) backends.

It does mean that cross-platform releases will require native build and packaging
work. They will not be ordinary `GOOS`/`GOARCH` cross-compiles. The GUI uses CGO,
gotk4, GTK, GLib, and optional VTE, so each target needs a compatible C toolchain
and target libraries. Prefer native Windows and macOS CI runners for release
artifacts.

The application architecture protects this choice. AWS configuration, models,
queries, polling, and mutations remain UI-neutral Go code. GTK types must stay in
`internal/gui`, and platform-specific terminal or desktop integration must stay
behind narrow interfaces and build tags. A future presentation-layer change must
not require rewriting the shared core or TUI.

## Risk summary

### GTK distribution

GTK applications normally bundle the toolkit on Windows. A distributable build
must include the required GTK and GLib DLLs, themes, icons, settings schemas, and
other runtime assets; the GTK project documents MSYS2 and gvsbuild-based build
paths and installer bundling in its
[Windows installation guide](https://www.gtk.org/docs/installations/windows).

On macOS, GTK uses the Quartz backend. GTK does not itself create an application
bundle, although the project documents `gtk-mac-bundler` for collecting the
executable and dependent libraries into a `.app`. See the
[GTK macOS guide](https://www.gtk.org/docs/installations/macos). Public releases
outside the App Store should be Developer ID signed and notarized according to
[Apple's distribution guidance](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution).

### gotk4

gotk4 is a greater long-term risk than GTK's platform backends. The project
currently warns that generated APIs can be incomplete and that some portions may
leak or crash. See the [gotk4 project notes](https://github.com/diamondburned/gotk4).
e9s is pinned to gotk4 `v0.3.1` because newer generated bindings referenced GLib
APIs unavailable in the verified development environment.

Every supported platform therefore needs a pinned and tested GTK, GLib, and
gotk4 matrix. Updating any member of that matrix is a deliberate compatibility
change rather than an incidental dependency upgrade.

### Embedded terminal

The current embedded terminal uses GTK VTE. VTE's PTY interface is Unix-oriented
and exposes file descriptors, process spawning, and Unix terminal behavior. Do
not assume that the VTE implementation will carry over to Windows.

Keep terminal support capability-based:

- Linux may use the existing VTE implementation.
- macOS should validate VTE availability and bundling separately; an external
  terminal fallback is acceptable for the initial port.
- Windows requires a separate ConPTY-backed implementation or an external
  terminal fallback.
- Builds without an embedded terminal must remain functional and explain the
  unavailable capability rather than failing to start.

The AWS Session Manager Plugin remains a separate runtime prerequisite wherever
ECS Exec or SSM sessions are supported.

### Platform conventions

A functional GTK build is not automatically a polished platform-native build.
Budget explicit work for:

- Command-key shortcuts, application menus, native controls, and bundle metadata
  on macOS;
- Windows fonts, cursor/theme assets, installer behavior, and code signing;
- native file dialogs, clipboard behavior, accessibility, scaling, and dark/light
  theme changes on both platforms;
- testing x86-64 and ARM64 where releases claim support.

GTK does not automatically map the primary shortcut modifier to Command on
macOS, so shortcuts must be represented as actions with platform-specific
accelerators. See GTK's
[input handling notes](https://docs.gtk.org/gtk4/input-handling.html).

## Portability spike

Run a bounded portability spike after the CloudWatch Logs GUI module and before
committing to the remaining cross-platform release work.

### Windows artifact

1. Build the GTK-only GUI natively on a Windows CI runner without VTE.
2. Bundle all required GTK runtime files and produce an installable or portable
   artifact that does not require the user to install MSYS2.
3. Launch the application, load AWS configuration, browse ECS and CloudWatch
   Logs, and exercise filtering, zoom, clipboard, dialogs, and theme behavior.
4. Verify the terminal-unavailable fallback, then separately estimate a ConPTY
   implementation.
5. Record artifact size, startup time, dependency inventory, signing path, and
   update strategy.

### macOS artifact

1. Build the GTK-only GUI natively on macOS for Intel and Apple Silicon, or
   produce and validate a universal build.
2. Create a self-contained `.app` with its GTK libraries and resources.
3. Exercise ECS and CloudWatch Logs with macOS menus, Command shortcuts, Retina
   scaling, clipboard, dialogs, themes, and native window controls.
4. Sign the complete nested bundle and verify that notarization accepts it.
5. Validate VTE independently; retain the external-terminal fallback if VTE
   materially complicates bundling or signing.

### Common acceptance criteria

Proceed with GTK cross-platform releases only if:

- end users do not need a compiler, package manager, or separate GTK install;
- the packaged application starts reliably and locates all bundled resources;
- AWS credential/profile behavior matches the TUI on the same platform;
- tables, text, log following, zoom, clipboard, shortcuts, and dialogs behave
  acceptably;
- rapid navigation and shutdown cancel work without stale updates or lingering
  processes;
- the native build matrix and release signing can be automated in CI;
- gotk4 does not require a growing set of unsafe platform-specific patches; and
- the maintenance and artifact-size cost is acceptable.

## Fallbacks if GTK packaging fails

The preferred fallback is a presentation-layer spike with
[Wails](https://wails.io/docs/introduction/). Wails keeps application logic in Go
and uses the operating system's existing webview rather than bundling an Electron
runtime. It supports Windows, macOS, and Linux packaging, but the interface would
be implemented with HTML/CSS and JavaScript or TypeScript. The shared services
and TUI would remain unchanged.

[Fyne](https://docs.fyne.io/started/cross-compiling/) is the all-Go-oriented
alternative to evaluate if avoiding web UI code is more important than preserving
the GTK look and widget behavior. It would still require a complete GUI rewrite
and its native graphics dependencies make cross-compilation more involved than a
plain Go binary.

Do not migrate preemptively. First run the native GTK packaging spike and make
the decision from measured build, runtime, integration, and maintenance results.

