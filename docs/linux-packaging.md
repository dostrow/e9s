# Linux desktop packaging

## Stable application identity

The GUI uses `io.github.dostrow.e9s` as its canonical desktop identity. The
same value is used for the GTK application ID, desktop-entry basename,
application icon, AppStream component, and future portable-package metadata.
The executable remains `e9s-gui`, while the TUI remains `e9s`.

Changing the desktop identity does not change configuration storage. Both
frontends continue to share `$XDG_CONFIG_HOME/e9s/config.yaml`, with the
existing legacy migration from `~/.e9s.yaml`.

## Initial compatibility baseline

The reference full GUI package targets Ubuntu 24.04 LTS or newer on amd64 and
arm64. Ubuntu 24.04 is the first Ubuntu LTS that supplies the GTK 4 VTE runtime
used by the embedded operation terminal and Terminal Dock. Release builds must
be compiled and tested in the oldest supported build environment rather than
copied from a newer development system.

The release set is intentionally split:

- `e9s`: a static, TUI-only archive built without CGO.
- `e9s-gui`: a native Debian package with GTK 4, GtkSourceView 5, and GTK 4
  VTE runtime dependencies.
- `e9s.AppImage`: a portable GUI for modern non-Ubuntu Linux distributions.

Flatpak, RPM, Nix, and Snap packaging remain follow-up targets. An Arch package
recipe is included after release artifact names and URLs are stable.

## Staged installation

`make install-gui` installs the GUI and desktop assets. Packaging builds should
stage the result instead of writing to the host:

```sh
make install-gui DESTDIR="$package_root" PREFIX=/usr
```

The staged tree includes the executable, desktop entry, AppStream metadata,
full-color and symbolic icons, raster fallback, and project license. Debian and
AppImage builders consume this same layout so their desktop integration cannot
silently diverge.
