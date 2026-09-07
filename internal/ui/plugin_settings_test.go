package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginSettingsValidatesNewRegistration(t *testing.T) {
	directory := t.TempDir()
	manifest := `api_version: e9s/v1alpha1
name: test-operations
display_name: Test Operations
actions:
  - id: inspect
    title: Inspect
    run:
      command: /bin/true
`
	path := filepath.Join(directory, "plugin.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	model := NewPluginSettings(nil).SetSize(100, 30).BeginAdd()
	model.name.SetValue("Friendly Operations")
	model.manifest.SetValue(path)
	entries, err := model.editedEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "Friendly Operations" || entries[0].Manifest != path {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestPluginSettingsRejectsMissingManifest(t *testing.T) {
	model := NewPluginSettings(nil).BeginAdd()
	if _, err := model.editedEntries(); err == nil {
		t.Fatal("expected missing manifest validation error")
	}
}
