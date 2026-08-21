#!/bin/sh
set -eu
umask 022

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
input_version=${1:-$(git -C "$repo_dir" describe --tags --always)}
output_dir=${2:-$repo_dir/dist}
architecture=${DEB_ARCH:-$(dpkg --print-architecture)}

# Git describe output is made Debian-version-safe while preserving ordering.
version=$(printf '%s' "$input_version" | sed -e 's/^v//' -e 's/-/+/g' -e 's/[^0-9A-Za-z.+:~]/./g')
case "$version" in
  [0-9]*) ;;
  *) version="0+$version" ;;
esac

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/e9s-deb.XXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT HUP INT TERM
tui_root=$work_dir/e9s
gui_root=$work_dir/e9s-gui
mkdir -p "$output_dir" "$tui_root/DEBIAN" "$tui_root/usr/bin" "$tui_root/usr/share/doc/e9s" "$gui_root/DEBIAN" "$gui_root/usr/bin"

ldflags="-s -w -X main.version=$input_version"
if [ -n "${E9S_TUI_BINARY:-}" ]; then
  install -m 0755 "$E9S_TUI_BINARY" "$tui_root/usr/bin/e9s"
else
  (cd "$repo_dir" && CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$tui_root/usr/bin/e9s" .)
fi
if [ -n "${E9S_GUI_BINARY:-}" ]; then
  install -m 0755 "$E9S_GUI_BINARY" "$gui_root/usr/bin/e9s-gui"
else
  (cd "$repo_dir" && go build -trimpath -tags "gui vte sourceview" -ldflags "$ldflags" -o "$gui_root/usr/bin/e9s-gui" ./cmd/e9s-gui)
fi
(cd "$repo_dir" && make install-gui-assets DESTDIR="$gui_root" PREFIX=/usr)
install -m 0644 "$repo_dir/LICENSE" "$tui_root/usr/share/doc/e9s/copyright"

# Debian packages must not share ordinary files unless they coordinate through
# dpkg. Keep each package's licensing material under its own policy-standard
# /usr/share/doc directory. The generic asset installer uses the cross-distro
# /usr/share/licenses layout, so normalize its staged output here.
mkdir -p "$gui_root/usr/share/doc"
mv "$gui_root/usr/share/licenses/e9s" "$gui_root/usr/share/doc/e9s-gui"
mv "$gui_root/usr/share/doc/e9s-gui/LICENSE" "$gui_root/usr/share/doc/e9s-gui/copyright"
rmdir "$gui_root/usr/share/licenses"

dependencies=$(cd "$repo_dir" && dpkg-shlibdeps -O -e"$gui_root/usr/bin/e9s-gui" | sed -n 's/^shlibs:Depends=//p')
if [ -z "$dependencies" ]; then
  echo "dpkg-shlibdeps did not produce GUI runtime dependencies" >&2
  exit 1
fi

tui_size=$(du -sk "$tui_root" | cut -f1)
gui_size=$(du -sk "$gui_root" | cut -f1)
cat >"$tui_root/DEBIAN/control" <<EOF
Package: e9s
Version: $version
Section: admin
Priority: optional
Architecture: $architecture
Maintainer: Daniel Ostrow <dostrow@users.noreply.github.com>
Homepage: https://github.com/dostrow/e9s
Installed-Size: $tui_size
Description: terminal workspace for operating AWS resources
 A keyboard-oriented terminal interface for AWS operational workflows.
EOF
cat >"$gui_root/DEBIAN/control" <<EOF
Package: e9s-gui
Version: $version
Section: admin
Priority: optional
Architecture: $architecture
Maintainer: Daniel Ostrow <dostrow@users.noreply.github.com>
Homepage: https://github.com/dostrow/e9s
Installed-Size: $gui_size
Depends: $dependencies
Recommends: e9s
Description: native GTK workspace for operating AWS resources
 The GTK 4 frontend for e9s with GtkSourceView and VTE integration.
EOF

dpkg-deb --root-owner-group --build "$tui_root" "$output_dir/e9s_${version}_${architecture}.deb"
dpkg-deb --root-owner-group --build "$gui_root" "$output_dir/e9s-gui_${version}_${architecture}.deb"
printf 'Created:\n  %s\n  %s\n' "$output_dir/e9s_${version}_${architecture}.deb" "$output_dir/e9s-gui_${version}_${architecture}.deb"
