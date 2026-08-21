#!/usr/bin/env bash
set -euo pipefail
umask 022

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
input_version=${1:-$(git -C "$repo_dir" describe --tags --always)}
output_dir=${2:-$repo_dir/dist}
runtime_prefix=${MINGW_PREFIX:-}

if [[ ${MSYSTEM:-} != UCRT64 || -z $runtime_prefix ]]; then
  echo "build-portable.sh must run in an MSYS2 UCRT64 shell" >&2
  exit 1
fi
if [[ ! -f $runtime_prefix/bin/libgtk-4-1.dll || ! -f $runtime_prefix/bin/libgtksourceview-5-0.dll ]]; then
  echo "GTK 4 and GtkSourceView 5 are required beneath $runtime_prefix" >&2
  exit 1
fi
for tool in ldd pacman zip; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required packaging tool is unavailable: $tool" >&2
    exit 1
  fi
done

version=$(printf '%s' "$input_version" | sed -e 's/^v//' -e 's/[^0-9A-Za-z.+~-]/-/g')
archive_name="e9s-gui-$version-windows-amd64"
mkdir -p "$output_dir"

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/e9s-windows.XXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT HUP INT TERM
bundle_dir=$work_dir/$archive_name
package_list=$work_dir/native-packages.txt
dependency_list=$work_dir/native-dependencies.txt
mkdir -p "$bundle_dir" "$bundle_dir/licenses/e9s/fonts" "$bundle_dir/share/e9s/fonts"
: >"$package_list"
: >"$dependency_list"

if [[ -n ${E9S_GUI_BINARY:-} ]]; then
  install -m 0755 "$E9S_GUI_BINARY" "$bundle_dir/e9s-gui.exe"
else
  (cd "$repo_dir" && make build-gui-windows-amd64 VERSION="$input_version")
  install -m 0755 "$repo_dir/e9s-gui-windows-amd64.exe" "$bundle_dir/e9s-gui.exe"
fi

install -m 0644 "$repo_dir/LICENSE" "$bundle_dir/licenses/e9s/LICENSE"
install -m 0644 "$repo_dir/assets/fonts/README.md" "$bundle_dir/share/e9s/fonts/README.md"
install -m 0644 "$repo_dir/assets/fonts/"*.ttf "$bundle_dir/share/e9s/fonts/"
install -m 0644 "$repo_dir/assets/fonts/licenses/"* "$bundle_dir/licenses/e9s/fonts/"
install -m 0644 "$script_dir/README.txt" "$bundle_dir/README.txt"

# Preserve the runtime prefix layout. GLib, GTK, Fontconfig, GdkPixbuf, and
# GtkSourceView all derive data paths from the DLL installation prefix on
# Windows, so flattening these trees would break schemas, icons, or language
# definitions after the archive is moved to another machine.
copy_runtime_tree() {
  local relative=$1
  local source=$runtime_prefix/$relative
  local destination=$bundle_dir/$(dirname "$relative")
  if [[ -e $source ]]; then
    mkdir -p "$destination"
    cp -a "$source" "$destination/"
  fi
}

for relative in \
  etc/fonts \
  etc/gtk-4.0 \
  lib/gdk-pixbuf-2.0 \
  lib/gio/modules \
  lib/gtk-4.0 \
  lib/pango \
  share/fontconfig \
  share/glib-2.0/schemas \
  share/gtk-4.0 \
  share/gtksourceview-5 \
  share/icons/Adwaita \
  share/icons/hicolor \
  share/mime \
  share/themes; do
  copy_runtime_tree "$relative"
done

# Install the e9s icons in the standard hicolor locations as well as carrying
# the icon themes needed by GTK widgets and dialogs.
install -Dm644 "$repo_dir/assets/icons/io.github.dostrow.e9s.svg" \
  "$bundle_dir/share/icons/hicolor/scalable/apps/io.github.dostrow.e9s.svg"
install -Dm644 "$repo_dir/assets/icons/io.github.dostrow.e9s-symbolic.svg" \
  "$bundle_dir/share/icons/hicolor/symbolic/apps/io.github.dostrow.e9s-symbolic.svg"
install -Dm644 "$repo_dir/assets/icons/io.github.dostrow.e9s-48.png" \
  "$bundle_dir/share/icons/hicolor/48x48/apps/io.github.dostrow.e9s.png"

# GLib uses the spawn helpers at runtime. The cache utilities are retained so
# the next portability phase can refresh relocatable module caches during a
# clean-machine launch without requiring MSYS2.
for helper in \
  gdk-pixbuf-query-loaders.exe \
  gio-querymodules.exe \
  glib-compile-schemas.exe \
  gspawn-win64-helper.exe \
  gspawn-win64-helper-console.exe; do
  if [[ -f $runtime_prefix/bin/$helper ]]; then
    install -m 0755 "$runtime_prefix/bin/$helper" "$bundle_dir/$helper"
  fi
done

declare -A visited=()
declare -a queue=("$bundle_dir/e9s-gui.exe")
while IFS= read -r helper; do
  queue+=("$helper")
done < <(find "$bundle_dir" -maxdepth 1 -type f -name '*.exe' ! -name 'e9s-gui.exe' -print | sort)
while IFS= read -r plugin; do
  queue+=("$plugin")
done < <(find "$bundle_dir/lib" -type f -iname '*.dll' -print 2>/dev/null | sort)

record_package_owner() {
  local path=$1
  local owner
  owner=$(pacman -Qqo "$path" 2>/dev/null || true)
  if [[ -n $owner ]]; then
    printf '%s\n' "$owner" >>"$package_list"
  fi
}

# ldd reports the complete PE dependency graph, but processing newly copied
# DLLs too makes the closure explicit and catches a missing transitive library
# before publishing the archive. Windows system DLLs are intentionally left to
# the OS; any dependency on the MSYS POSIX runtime is a packaging error.
while ((${#queue[@]})); do
  current=${queue[0]}
  queue=("${queue[@]:1}")
  current_key=$(basename "$current" | tr '[:upper:]' '[:lower:]')
  if [[ -n ${visited[$current_key]+x} ]]; then
    continue
  fi
  visited[$current_key]=1

  if ! report=$(ldd "$current" 2>&1); then
    printf '%s\n' "$report" >&2
    echo "unable to inspect native dependencies for $current" >&2
    exit 1
  fi
  if grep -qi 'not found' <<<"$report"; then
    printf '%s\n' "$report" >&2
    echo "unresolved native dependency for $current" >&2
    exit 1
  fi
  if grep -Eq '(^|[[:space:]])/usr/bin/.*\.dll([[:space:]]|$)' <<<"$report"; then
    printf '%s\n' "$report" >&2
    echo "native Windows artifact unexpectedly depends on the MSYS runtime" >&2
    exit 1
  fi

  while IFS= read -r dependency; do
    [[ -n $dependency ]] || continue
    case "$dependency" in
      "$runtime_prefix"/*)
        target=$bundle_dir/$(basename "$dependency")
        if [[ -e $target ]] && ! cmp -s "$dependency" "$target"; then
          echo "conflicting native DLL basenames: $dependency and $target" >&2
          exit 1
        fi
        if [[ ! -e $target ]]; then
          install -m 0755 "$dependency" "$target"
        fi
        printf '%s\n' "$dependency" >>"$dependency_list"
        record_package_owner "$dependency"
        queue+=("$target")
        ;;
    esac
  done < <(awk '$2 == "=>" { print $3 } $1 ~ /^\// && $2 != "=>" { print $1 }' <<<"$report" | sort -u)
done

for required_dll in libgtk-4-1.dll libgtksourceview-5-0.dll; do
  if [[ ! -f $bundle_dir/$required_dll ]]; then
    echo "native dependency collector did not bundle $required_dll" >&2
    exit 1
  fi
done

# Include the packages which own data-only runtime trees; they may not appear
# in the executable's DLL closure even though GTK loads their files at runtime.
for package in \
  mingw-w64-ucrt-x86_64-adwaita-icon-theme \
  mingw-w64-ucrt-x86_64-fontconfig \
  mingw-w64-ucrt-x86_64-gdk-pixbuf2 \
  mingw-w64-ucrt-x86_64-glib2 \
  mingw-w64-ucrt-x86_64-gtk4 \
  mingw-w64-ucrt-x86_64-gtksourceview5 \
  mingw-w64-ucrt-x86_64-hicolor-icon-theme \
  mingw-w64-ucrt-x86_64-shared-mime-info; do
  if pacman -Q "$package" >/dev/null 2>&1; then
    printf '%s\n' "$package" >>"$package_list"
  fi
done

sort -u "$package_list" -o "$package_list"
sort -u "$dependency_list" -o "$dependency_list"

while IFS= read -r package; do
  pacman -Q "$package"
done <"$package_list" >"$bundle_dir/THIRD-PARTY-PACKAGES.txt"

# Copy every license file declared by the packages represented in the bundle.
# The package inventory remains useful even for a package without an installed
# license directory.
while IFS= read -r package; do
  while IFS= read -r license_path; do
    [[ -f $license_path ]] || continue
    relative=${license_path#"$runtime_prefix/share/licenses/"}
    install -Dm644 "$license_path" "$bundle_dir/licenses/native/$relative"
  done < <(pacman -Ql "$package" | awk -v prefix="$runtime_prefix/share/licenses/" 'index($2, prefix) == 1 { print $2 }')
done <"$package_list"

sed "s/@VERSION@/$input_version/g" "$script_dir/README.txt" >"$bundle_dir/README.txt.tmp"
mv "$bundle_dir/README.txt.tmp" "$bundle_dir/README.txt"

archive=$output_dir/$archive_name.zip
rm -f -- "$archive"
(cd "$work_dir" && zip -q -r "$archive" "$archive_name")

test -s "$archive"
printf 'Created %s\n' "$archive"
printf 'Bundled %s native DLLs from %s packages\n' \
  "$(find "$bundle_dir" -type f -iname '*.dll' | wc -l | tr -d ' ')" \
  "$(wc -l <"$package_list" | tr -d ' ')"
