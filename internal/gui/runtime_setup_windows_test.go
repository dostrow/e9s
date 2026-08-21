//go:build gui && windows

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableWindowsRuntimeDetection(t *testing.T) {
	root := t.TempDir()
	if isPortableWindowsRuntime(root) {
		t.Fatal("empty directory detected as a portable Windows runtime")
	}
	paths := []string{
		filepath.Join(root, "libgtk-4-1.dll"),
		filepath.Join(root, "libgtksourceview-5-0.dll"),
		filepath.Join(root, "share", "glib-2.0", "schemas", "gschemas.compiled"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "share", "gtksourceview-5"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isPortableWindowsRuntime(root) {
		t.Fatal("complete portable Windows runtime was not detected")
	}
}

func TestWindowsRuntimeEnvironmentUsesBundleRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "e9s")
	environment := windowsRuntimeEnvironment(root)
	for name, value := range environment {
		if !strings.HasPrefix(filepath.Clean(value), filepath.Clean(root)) {
			t.Fatalf("%s = %q, want path beneath %q", name, value, root)
		}
	}
	if got := environment["E9S_DATA_DIR"]; got != filepath.Join(root, "share", "e9s") {
		t.Fatalf("E9S_DATA_DIR = %q", got)
	}
}

func TestPrependEnvironmentPathAvoidsDuplicates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("PATH", `C:\Windows\System32`)
	prependEnvironmentPath("PATH", root)
	prependEnvironmentPath("PATH", root)
	entries := filepath.SplitList(os.Getenv("PATH"))
	if len(entries) != 2 || !strings.EqualFold(entries[0], root) {
		t.Fatalf("PATH entries = %#v", entries)
	}
}
