//go:build gui

package gui

import "testing"

func TestCanonicalApplicationID(t *testing.T) {
	const want = "io.github.dostrow.e9s"
	if applicationID != want {
		t.Fatalf("applicationID = %q, want %q", applicationID, want)
	}
}
