//go:build gui && !sourceview

package gui

func initializeSourceEditor() {}

func sourceEditorAvailable() bool { return false }
