# Windows GUI development

The Windows frontend is built natively with the MSYS2 UCRT64 toolchain. It is
not a normal `GOOS=windows` cross-compile: gotk4 and GTK use CGO and therefore
need a Windows C compiler, headers, import libraries, and `pkg-config` metadata.

The first portability milestone builds the GTK frontend with GtkSourceView. It
deliberately omits only the `vte` tag:

- the editor retains syntax highlighting through the native GtkSourceView 5
  package; and
- embedded terminals, ECS Exec, and Session Manager terminal views report that
  the capability is unavailable instead of preventing the application from
  starting.

This gives the GTK/Win32 port an independent acceptance boundary before a
ConPTY terminal implementation is added.

## Native prerequisites

Install MSYS2 and open a UCRT64 shell. Install the native GTK development stack:

```sh
pacman -S --needed \
  mingw-w64-ucrt-x86_64-gcc \
  mingw-w64-ucrt-x86_64-gtk4 \
  mingw-w64-ucrt-x86_64-gtksourceview5 \
  mingw-w64-ucrt-x86_64-pkgconf \
  zip
```

Install Go 1.24.4 or newer for Windows and ensure `go.exe` is visible inside
the UCRT64 shell. Verify that all tools resolve to the intended environment:

```sh
go version
gcc --version
pkg-config --modversion gtk4
pkg-config --modversion gtksourceview-5
go env GOOS GOARCH CGO_ENABLED CC
```

`pkg-config` and GCC should resolve beneath the UCRT64 prefix. Build the GUI:

```sh
make build-gui-windows-amd64
```

The result is `e9s-gui-windows-amd64.exe`. This raw executable still requires
the MSYS2 runtime. Build a distributable archive with:

```sh
make package-windows-zip
```

The resulting `dist/e9s-gui-<version>-windows-amd64.zip` carries the native DLL
closure, GTK and GLib schemas and data, GtkSourceView language definitions and
styles, icon and MIME data, Fontconfig configuration, bundled application fonts,
and the applicable license texts.

At startup, the portable build derives every data path from `e9s-gui.exe`,
regenerates the GdkPixbuf loader cache beneath the current user's cache
directory, and registers the bundled fonts privately with Pango. It does not
install fonts or write GTK configuration globally. The internal `--self-test`
flag initializes GTK, registers the fonts, loads GtkSourceView's SQL language
definition, and exercises an editor buffer without loading AWS configuration.
It is hidden because it is a packaging diagnostic rather than a user workflow.

## CI boundary

`.github/workflows/windows-gui.yml` performs the same build on a native Windows
runner using MSYS2 UCRT64. It runs the shared test suite, the GTK-tagged GUI
tests, validates the executable's dynamic dependencies, assembles the portable
ZIP, extracts it, removes MSYS2 from `PATH`, runs the packaged runtime self-test,
and uploads both the archive and the raw compile artifact for inspection.

VTE must not be added to the Windows build tags. GtkSourceView language
definitions and style schemes are included in the portable runtime.
