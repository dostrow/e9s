# Windows support matrix

The Windows GUI is a native GTK 4 application bundled with its GTK,
GtkSourceView, GLib, Fontconfig, icon, MIME, schema, and language-definition
runtime. End users do not need MSYS2, Go, GTK, or administrator access.

## Supported releases

| Area | Supported boundary |
| --- | --- |
| Operating system | Windows 10 version 1809 or newer; Windows 11 |
| Architecture | x86-64 (`amd64`) |
| Distribution | Portable ZIP and per-user Inno Setup installer |
| Installation | `%LOCALAPPDATA%\Programs\e9s`; Start Menu shortcut; optional desktop shortcut |
| Configuration | `%APPDATA%\e9s\config.yaml` |
| Runtime cache | `%LOCALAPPDATA%\e9s` |
| Editor | GtkSourceView 5 with bundled language definitions and styles |
| Embedded terminal | Unavailable; a ConPTY backend is deferred |
| AWS behavior | Shared connection/query core and standard AWS SDK credential chain |

Windows 11 on Arm64 can run x86-64 applications through emulation and the
installer permits that configuration, but it is not yet part of the verified
CI or interactive support matrix. Windows Server and Wine are likewise not
currently release-tested. The separately published, console-only TUI remains a
pure-Go binary and includes native Windows `amd64` and `arm64` builds.

## Native dependency baseline

Release and pull-request builds use a native `windows-latest` runner with the
MSYS2 UCRT64 environment, GTK 4, GtkSourceView 5, GCC, and pkg-config. Installer
builds pin Inno Setup 6.7.1. MSYS2 is a rolling distribution, so every portable
archive includes `THIRD-PARTY-PACKAGES.txt` with the exact native package
versions used for that artifact; that inventory, rather than an assumed system
GTK version, is the runtime baseline for a release.

CI verifies the shared Go tests and GTK frontend tests, rejects unresolved or
MSYS-runtime DLL dependencies, starts the portable application with MSYS2
removed from `PATH`, exercises GTK and GtkSourceView through the packaging
self-test, installs and upgrades the per-user installer, starts the installed
application through the same self-test, and verifies uninstall cleanup.

## Remaining acceptance work

The automated boundary establishes packaging correctness, not complete desktop
qualification. Before calling the Windows GUI generally available, test the
signed build interactively on the oldest supported Windows 10 release and a
current Windows 11 release, including AWS profiles, high-DPI scaling, clipboard,
file dialogs, theme behavior, log following, editor workflows, rapid navigation,
sleep/resume, and shutdown cancellation. Record artifact size and cold-start
time for each release candidate.

