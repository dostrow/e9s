//go:build gui && sourceview

package gui

import gtksource "libdb.so/gotk4-sourceview/pkg/gtksource/v4"

func initializeSourceEditor() {
	gtksource.Init()
}

func sourceEditorAvailable() bool { return true }
