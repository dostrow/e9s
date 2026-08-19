//go:build gui && sourceview

package gui

import (
	"testing"

	gtksource "libdb.so/gotk4-sourceview/pkg/gtksource/v4"
)

func TestSourceViewLanguageDefinitionsAreAvailable(t *testing.T) {
	initializeSourceEditor()
	manager := gtksource.LanguageManagerGetDefault()
	for path, want := range map[string]string{
		"task-definition.json": "json",
		"handler.py":           "python3",
		"index.js":             "js",
	} {
		language := manager.GuessLanguage(path, "")
		if language == nil {
			t.Errorf("no GtkSourceView language detected for %q", path)
			continue
		}
		if got := language.ID(); got != want {
			t.Errorf("GtkSourceView language for %q = %q, want %q", path, got, want)
		}
	}
}
