package sqlworkbench

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePGPassUsesFirstMatchingEntry(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".pgpass")
	data := "db.example:5432:app:reader:first\\:secret\n*:5432:*:reader:second\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	password, usedPath, err := ResolvePGPass([]string{path}, PGPassMatch{Host: "db.example", Port: "5432", Database: "app", User: "reader"})
	if err != nil {
		t.Fatal(err)
	}
	if password != "first:secret" || usedPath != path {
		t.Fatalf("got password %q from %q", password, usedPath)
	}
}

func TestResolvePGPassNoMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pgpass")
	if err := os.WriteFile(path, []byte("other:5432:app:user:secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolvePGPass([]string{path}, PGPassMatch{Host: "db", Port: "5432", Database: "app", User: "user"})
	if !errors.Is(err, ErrPGPassNoMatch) {
		t.Fatalf("expected ErrPGPassNoMatch, got %v", err)
	}
}

func TestLoadPGPassRejectsInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows pgpass permissions use ACLs")
	}
	path := filepath.Join(t.TempDir(), ".pgpass")
	if err := os.WriteFile(path, []byte("*:5432:*:*:secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPGPass(path); err == nil {
		t.Fatal("expected insecure permissions to fail")
	}
}
