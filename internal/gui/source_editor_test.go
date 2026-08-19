//go:build gui

package gui

import "testing"

func TestEditorColorDarkness(t *testing.T) {
	for _, test := range []struct {
		name             string
		red, green, blue float64
		want             bool
	}{
		{name: "dark", red: 0.06, green: 0.06, blue: 0.06, want: true},
		{name: "light", red: 0.9, green: 0.9, blue: 0.9, want: false},
	} {
		if got := editorColorIsDark(test.red, test.green, test.blue); got != test.want {
			t.Errorf("%s color classified dark = %t, want %t", test.name, got, test.want)
		}
	}
}
