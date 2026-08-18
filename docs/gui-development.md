# Experimental GTK GUI development

The GUI is an experimental GTK 4 frontend for the ECS proof of concept. It is
built as a separate `e9s-gui` executable; the normal `e9s` TUI does not compile,
link, or require GTK.

## Prerequisites

Install Go 1.24 or newer, a C compiler, `pkg-config`, and the GTK 4 development
files. The following package sets are intentionally small:

### Ubuntu and Debian

```bash
sudo apt install build-essential pkg-config libgtk-4-dev
```

The PoC was verified on Ubuntu 25.10 with GTK 4.20.1, GLib 2.86.0, gcc 15.2,
and `pkg-config` 1.8.1.

### Arch Linux

```bash
sudo pacman -S --needed base-devel pkgconf gtk4
```

Arch publishes the development headers, shared library, and pkg-config metadata
in its [`gtk4` package](https://archlinux.org/packages/extra/x86_64/gtk4/); its
[`pkgconf` package](https://archlinux.org/packages/core/x86_64/pkgconf/) provides
the pkg-config implementation.

### Fedora

```bash
sudo dnf install gcc pkgconf-pkg-config gtk4-devel
```

Fedora publishes the headers and build metadata in
[`gtk4-devel`](https://packages.fedoraproject.org/pkgs/gtk4/gtk4-devel/).

## Build and run

```bash
make build-gui
./e9s-gui
```

The GUI accepts the ECS-relevant TUI settings and YAML defaults:

```bash
./e9s-gui --profile production --region us-east-2 \
  --cluster my-cluster --refresh 5
```

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
go test -race ./internal/gui ./internal/service
go vet ./...
go build -tags gui ./cmd/e9s-gui
```

## Current scope

The PoC provides cluster and service browsing, service inspection, filtering,
refresh, bounded live service logs, and a confirmed force-deployment action. The
GTK frontend and Bubble Tea frontend both call the same UI-neutral services in
`internal/service`; GTK code does not call AWS SDK adapters directly.

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

See [`gui-poc-results.md`](gui-poc-results.md) for the measurements, limitations,
and recommendation from the initial experiment.
