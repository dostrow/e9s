//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
)

func TestAppearancePresetsHaveCompletePalettes(t *testing.T) {
	for _, preset := range appearancePresets {
		palette, custom := configuredPreset(preset.name)
		if preset.name == appearanceSystem {
			if custom {
				t.Fatal("system appearance unexpectedly overrides the GTK palette")
			}
			continue
		}
		if !custom {
			t.Fatalf("preset %q was not found", preset.name)
		}
		if palette.foreground.Alpha() == 0 || palette.background.Alpha() == 0 || palette.accent.Alpha() == 0 {
			t.Fatalf("preset %q has an incomplete palette", preset.name)
		}
	}
}

func TestConfiguredAppearanceCSSUsesSeparateFontRoles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GUI.Appearance.InterfaceFont = "Inter 11"
	cfg.GUI.Appearance.MonospaceFont = "JetBrainsMono Nerd Font 10"
	palette, _ := configuredPreset("gruvbox-material-dark")
	css := configuredAppearanceCSS(&cfg, palette, true)
	for _, fragment := range []string{
		`.e9s-root { font-family: "Inter"`,
		`window, popover, tooltip { font-family: "Inter"`,
		`.e9s-root .inspector`,
		`font-family: "JetBrainsMono Nerd Font"`,
		`window checkbutton > check`,
		`-gtk-icon-source: -gtk-icontheme("object-select-symbolic")`,
		`window switch > slider`,
		`window searchentry > text`,
		`window columnview > header > button`,
		`.e9s-settings .e9s-dialog-surface`,
		`window button:not(.flat)`,
		`.e9s-root button.module-subitem`,
		`.e9s-root .e9s-settings-page`,
		`.e9s-root checkbutton check`,
		`.e9s-root entry.resource-search image`,
		`.e9s-root scrollbar slider`,
		`notebook.e9s-settings-notebook > header > tabs`,
		`window dropdown > button`,
		`window spinbutton > button`,
		`popover.background > contents, tooltip`,
		`popover.background button:disabled`,
		"background-color:",
	} {
		if !strings.Contains(css, fragment) {
			t.Errorf("appearance CSS is missing %q:\n%s", fragment, css)
		}
	}
	if strings.Contains(css, "window button {") {
		t.Fatal("built-in appearance must not restore borders on flat navigation buttons")
	}
	if strings.Contains(css, "%!") {
		t.Fatalf("appearance CSS contains a formatting error:\n%s", css)
	}
}

func TestSystemAppearanceDoesNotRestyleGTKWindows(t *testing.T) {
	cfg := config.DefaultConfig()
	palette, _ := configuredPreset("gruvbox-material-dark")
	css := configuredAppearanceCSS(&cfg, palette, false)
	for _, fragment := range []string{"window { color:", "window button", "window checkbutton", "background-color:"} {
		if strings.Contains(css, fragment) {
			t.Errorf("system appearance unexpectedly contains GTK override %q:\n%s", fragment, css)
		}
	}
}

func TestUnknownAppearanceFallsBackToSystem(t *testing.T) {
	if _, custom := configuredPreset("some-future-theme"); custom {
		t.Fatal("unknown appearance should defer to the GTK theme")
	}
	if got := appearancePresetIndex("some-future-theme"); got != 0 {
		t.Fatalf("unknown appearance index = %d, want system index", got)
	}
}
