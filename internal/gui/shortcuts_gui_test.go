//go:build gui

package gui

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

func TestPrintableShortcutAction(t *testing.T) {
	tests := []struct {
		name   string
		keyval uint
		state  gdk.ModifierType
		want   string
	}{
		{name: "search", keyval: gdk.KEY_slash, want: "search"},
		{name: "metrics", keyval: gdk.KEY_m, want: "metrics"},
		{name: "environment", keyval: gdk.KEY_e, want: "task-definition-env"},
		{name: "diff", keyval: gdk.KEY_d, want: "task-definition-diff"},
		{name: "timestamps", keyval: gdk.KEY_t, want: "log-timestamps"},
		{name: "help", keyval: gdk.KEY_question, state: gdk.ShiftMask, want: "help"},
		{name: "logs", keyval: gdk.KEY_L, state: gdk.ShiftMask, want: "logs"},
		{name: "standalone", keyval: gdk.KEY_S, state: gdk.ShiftMask, want: "standalone-tasks"},
		{name: "task definitions", keyval: gdk.KEY_T, state: gdk.ShiftMask, want: "task-definitions"},
		{name: "modified printable", keyval: gdk.KEY_e, state: gdk.ControlMask},
		{name: "ordinary input", keyval: gdk.KEY_a},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := printableShortcutAction(test.keyval, test.state); got != test.want {
				t.Fatalf("printableShortcutAction() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDynamoLoadMoreShortcut(t *testing.T) {
	tests := []struct {
		name     string
		keyval   uint
		state    gdk.ModifierType
		activate bool
		consume  bool
	}{
		{name: "closing bracket", keyval: gdk.KEY_bracketright, activate: true, consume: true},
		{name: "page down", keyval: gdk.KEY_Page_Down, activate: true, consume: false},
		{name: "modified page down", keyval: gdk.KEY_Page_Down, state: gdk.ControlMask},
		{name: "unrelated key", keyval: gdk.KEY_n},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			activate, consume := dynamoLoadMoreShortcut(test.keyval, test.state)
			if activate != test.activate || consume != test.consume {
				t.Fatalf("dynamoLoadMoreShortcut() = (%t, %t), want (%t, %t)",
					activate, consume, test.activate, test.consume)
			}
		})
	}
}
