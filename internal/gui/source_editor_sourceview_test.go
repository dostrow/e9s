//go:build gui && sourceview

package gui

import (
	"testing"
)

func TestSourceViewLanguageDefinitionsAreAvailable(t *testing.T) {
	initializeSourceEditor()
	for path, want := range map[string]string{
		"task-definition.json": "json",
		"terraform.tfvars":     "terraform",
		"handler.py":           "python3",
		"index.js":             "js",
	} {
		got := sourceViewLanguageID(sourceDocument{Path: path})
		if got == "" {
			t.Errorf("no GtkSourceView language detected for %q", path)
			continue
		}
		if got != want {
			t.Errorf("GtkSourceView language for %q = %q, want %q", path, got, want)
		}
	}
}
