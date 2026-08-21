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
  mingw-w64-ucrt-x86_64-pkgconf
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

The result is `e9s-gui-windows-amd64.exe`. At this phase it is a compile
artifact, not a distributable application: GTK DLLs and runtime data must still
be present in the MSYS2 environment. The portable bundle implemented in the
next phase will remove that end-user requirement.

## CI boundary

`.github/workflows/windows-gui.yml` performs the same build on a native Windows
runner using MSYS2 UCRT64. It runs the shared test suite, the GTK-tagged GUI
tests, records the executable's dynamic dependencies, and uploads the raw
compile artifact for inspection.

VTE must not be added to the Windows build tags. GtkSourceView language
definitions and style schemes must be included with the portable runtime in the
next phase.
