//go:build gui

package gui

import (
	"strings"
	"testing"
)

func TestSettingsChoiceIndex(t *testing.T) {
	choices := []settingsChoice{{label: "Ask", value: ""}, {label: "ECS", value: "ECS"}}
	if got := settingsChoiceIndex(choices, "ecs"); got != 1 {
		t.Fatalf("settingsChoiceIndex() = %d, want 1", got)
	}
	if got := settingsChoiceIndex(choices, "unknown"); got != 0 {
		t.Fatalf("unknown choice index = %d, want fallback 0", got)
	}
}

func TestConfigurationDiff(t *testing.T) {
	diff := configurationDiff("one\ntwo\nthree\n", "one\nchanged\nthree\n")
	for _, want := range []string{"  one", "- two", "+ changed", "  three"} {
		if !strings.Contains(diff, want) {
			t.Fatalf("configurationDiff() missing %q:\n%s", want, diff)
		}
	}
	if got := configurationDiff("same\n", "same\n"); got != "" {
		t.Fatalf("unchanged diff = %q, want empty", got)
	}
}
