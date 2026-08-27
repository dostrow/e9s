#!/usr/bin/env bash

set -uo pipefail

max_attempts="${GO_MODULE_DOWNLOAD_ATTEMPTS:-4}"
attempt=1

while (( attempt <= max_attempts )); do
	if (( attempt == 1 )); then
		go mod download && exit 0
	else
		# proxy.golang.org occasionally resets an HTTP/2 stream while serving a
		# module archive. Retrying over HTTP/1.1 avoids repeating that transport
		# failure while retaining checksum verification and the configured proxy.
		GODEBUG="${GODEBUG:+${GODEBUG},}http2client=0" go mod download && exit 0
	fi

	status=$?
	if (( attempt == max_attempts )); then
		echo "Go module download failed after ${max_attempts} attempts" >&2
		exit "$status"
	fi

	delay=$((attempt * 5))
	echo "Go module download attempt ${attempt} failed; retrying in ${delay}s" >&2
	sleep "$delay"
	attempt=$((attempt + 1))
done
