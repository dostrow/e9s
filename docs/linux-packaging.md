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

## Debian packages

`make package-deb` produces separate TUI and GUI packages under `dist/`. The
GUI runtime dependency list is generated from the linked binary by
`dpkg-shlibdeps`; it is intentionally not a hand-maintained list. Release
packages are built inside the Ubuntu 24.04 job so their symbol requirements do
not accidentally inherit a newer developer workstation.

The `debian/` directory also provides conventional debhelper metadata for
distribution rebuilds. Required build packages on Ubuntu 24.04 are Go 1.24 or
newer, `pkg-config`, `libfontconfig-dev`, `libgtk-4-dev`,
`libgtksourceview-5-dev`, and `libvte-2.91-gtk4-dev`.

## AppImage

`make package-appimage` stages the same desktop assets and builds a portable
GUI under `dist/`. The builder pins linuxdeploy
`1-alpha-20251107-1` by SHA-256 for x86-64 and AArch64. It deploys the linked
GTK 4, GtkSourceView, VTE, Fontconfig, and Pango dependency closure directly;
the legacy linuxdeploy GTK plugin is deliberately not used because it only
supports GTK 2 and GTK 3.

The custom `AppRun` points e9s at its private fonts and any bundled Gio or GDK
Pixbuf module caches. Release AppImages, like Debian packages, must be built in
the Ubuntu 24.04 baseline environment so their glibc requirement remains
portable to all supported distributions.

## Releases and Arch Linux

Tag pushes retain the existing static TUI matrix and add native Ubuntu 24.04
desktop jobs on `ubuntu-24.04` and `ubuntu-24.04-arm`. The jobs test the GUI,
build both Debian packages and the AppImage, upload every artifact, and publish
one sorted SHA-256 manifest with the GitHub release.

`packaging/arch/PKGBUILD` is an AUR-ready split `-git` recipe. It produces
`e9s-git` and `e9s-gui-git`; only the GUI package depends on `fontconfig`,
`gtk4`, `gtksourceview5`, and `vte4`. A versioned AUR recipe can replace it once
the first release containing the stable Linux artifact layout is tagged.

## Staged installation

`make install-gui` installs the GUI and desktop assets. Packaging builds should
stage the result instead of writing to the host:

```sh
make install-gui DESTDIR="$package_root" PREFIX=/usr
```

The staged tree includes the executable, desktop entry, AppStream metadata,
full-color and symbolic icons, raster fallback, and project license. Debian and
AppImage builders consume this same layout so their desktop integration cannot
silently diverge. It also installs the app-private font set under
`$PREFIX/share/e9s/fonts`; e9s registers that directory only inside its own
process and never modifies the system or user font configuration.
