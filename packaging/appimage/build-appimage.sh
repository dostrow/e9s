#!/bin/sh
set -eu
umask 022

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
input_version=${1:-$(git -C "$repo_dir" describe --tags --always)}
output_dir=${2:-$repo_dir/dist}
machine=${APPIMAGE_ARCH:-$(uname -m)}

case "$machine" in
  x86_64|amd64)
    tool_arch=x86_64
    artifact_arch=x86_64
    tool_sha256=c20cd71e3a4e3b80c3483cef793cda3f4e990aca14014d23c544ca3ce1270b4d
    ;;
  aarch64|arm64)
    tool_arch=aarch64
    artifact_arch=aarch64
    tool_sha256=620095110d693282b8ebeb244a95b5e911cf8f65f76c88b4b47d16ae6346fcff
    ;;
  *)
    echo "unsupported AppImage architecture: $machine" >&2
    exit 1
    ;;
esac

version=$(printf '%s' "$input_version" | sed -e 's/^v//' -e 's/[^0-9A-Za-z.+~-]/-/g')
linuxdeploy_release=1-alpha-20251107-1
cache_root=${XDG_CACHE_HOME:-${HOME}/.cache}/e9s/packaging
linuxdeploy=${LINUXDEPLOY:-$cache_root/linuxdeploy-$tool_arch.AppImage}
mkdir -p "$cache_root" "$output_dir"

if [ ! -x "$linuxdeploy" ]; then
  temporary_tool=$linuxdeploy.download
  curl -fL --retry 3 -o "$temporary_tool" \
    "https://github.com/linuxdeploy/linuxdeploy/releases/download/$linuxdeploy_release/linuxdeploy-$tool_arch.AppImage"
  printf '%s  %s\n' "$tool_sha256" "$temporary_tool" | sha256sum -c -
  chmod 0755 "$temporary_tool"
  mv "$temporary_tool" "$linuxdeploy"
fi
printf '%s  %s\n' "$tool_sha256" "$linuxdeploy" | sha256sum -c -

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/e9s-appimage.XXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT HUP INT TERM
app_dir=$work_dir/e9s.AppDir
mkdir -p "$app_dir/usr/bin"

if [ -n "${E9S_GUI_BINARY:-}" ]; then
  install -m 0755 "$E9S_GUI_BINARY" "$app_dir/usr/bin/e9s-gui"
else
  ldflags="-s -w -X main.version=$input_version"
  (cd "$repo_dir" && go build -trimpath -tags "gui vte sourceview" -ldflags "$ldflags" -o "$app_dir/usr/bin/e9s-gui" ./cmd/e9s-gui)
fi
(cd "$repo_dir" && make install-gui-assets DESTDIR="$app_dir" PREFIX=/usr)
# appimagetool still recognizes the historical appdata.xml suffix when
# discovering metadata, while distributions consume the canonical metainfo
# file installed above.
install -m 0644 "$repo_dir/packaging/linux/io.github.dostrow.e9s.metainfo.xml" \
  "$app_dir/usr/share/metainfo/io.github.dostrow.e9s.appdata.xml"

output=$output_dir/e9s-gui-$version-$artifact_arch.AppImage
rm -f -- "$output"
APPIMAGE_EXTRACT_AND_RUN=1 NO_STRIP=1 LDAI_OUTPUT="$output" "$linuxdeploy" \
  --appdir "$app_dir" \
  --deploy-deps-only "$app_dir/usr/bin/e9s-gui" \
  --desktop-file "$repo_dir/packaging/linux/io.github.dostrow.e9s.desktop" \
  --icon-file "$repo_dir/assets/icons/io.github.dostrow.e9s.svg" \
  --custom-apprun "$script_dir/AppRun" \
  --output appimage

test -x "$output"
printf 'Created %s\n' "$output"
