//go:build gui

package gui

import (
	"os"
	"path/filepath"
	"runtime"
)

func bundledFontDirectory() string {
	for _, candidate := range bundledFontCandidates() {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func bundledFontCandidates() []string {
	var candidates []string
	if dataDirectory := os.Getenv("E9S_DATA_DIR"); dataDirectory != "" {
		candidates = append(candidates, filepath.Join(dataDirectory, "fonts"))
	}
	if executable, err := os.Executable(); err == nil {
		directory := filepath.Dir(executable)
		// Portable Windows bundles keep share beside the executable. Linux
		// installs conventionally place the executable one directory deeper in
		// bin, so retain both layouts in priority order.
		candidates = append(candidates,
			filepath.Join(directory, "share", "e9s", "fonts"),
			filepath.Clean(filepath.Join(directory, "..", "share", "e9s", "fonts")),
		)
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(workingDirectory, "assets", "fonts"))
	}
	if _, source, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "assets", "fonts")))
	}
	return append(candidates, "/usr/local/share/e9s/fonts", "/usr/share/e9s/fonts")
}
