//go:build gui

package gui

import "testing"

func TestCompactStatusMessageMakesMultilineErrorsReadable(t *testing.T) {
	got := compactStatusMessage("request failed:\n  AccessDenied:   missing permission")
	want := "request failed: AccessDenied: missing permission"
	if got != want {
		t.Fatalf("compactStatusMessage() = %q, want %q", got, want)
	}
}
