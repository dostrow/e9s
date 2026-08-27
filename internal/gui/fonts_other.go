//go:build gui && !linux && !windows

package gui

func registerBundledFonts() (string, error) { return "", nil }
