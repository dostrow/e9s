#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
version=${1:-$(git -C "$repo_dir" describe --tags --always)}
output=${2:-$repo_dir/e9s-gui-windows-amd64.exe}
resource_object=$repo_dir/cmd/e9s-gui/e9s_windows_amd64.syso

if [[ ${MSYSTEM:-} != UCRT64 ]]; then
  echo "build-gui.sh must run in an MSYS2 UCRT64 shell" >&2
  exit 1
fi
if ! command -v windres >/dev/null 2>&1; then
  echo "windres is required to embed the Windows application icon" >&2
  exit 1
fi

cleanup() {
  rm -f -- "$resource_object"
}
trap cleanup EXIT HUP INT TERM

(cd "$repo_dir" && windres -O coff -i packaging/windows/e9s.rc -o "$resource_object")
(cd "$repo_dir" && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
  go build -trimpath -tags "gui sourceview" \
  -ldflags "-s -w -H=windowsgui -X main.version=$version" \
  -o "$output" ./cmd/e9s-gui)

test -s "$output"
