//go:build !windows

package tofu

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
