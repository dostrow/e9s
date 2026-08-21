e9s for Windows
===============

Version: @VERSION@

Run e9s-gui.exe directly from this directory. The archive is portable: keep
the DLLs and the share, lib, and etc directories beside the executable.

e9s reads the standard AWS configuration and credential files. Its own
configuration is stored below %APPDATA%\e9s unless overridden explicitly.

This initial Windows port includes the GtkSourceView editor but not the
embedded terminal. Terminal, ECS Exec, and Session Manager terminal actions
show an unavailable-capability message on Windows. They do not prevent the
rest of the application from running.

THIRD-PARTY-PACKAGES.txt records the exact MSYS2 runtime packages used to
assemble this archive. Their license texts are under licenses\native.

Project: https://github.com/dostrow/e9s
