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
		`.e9s-root .inspector`,
		`font-family: "JetBrainsMono Nerd Font"`,
		"background-color:",
	} {
		if !strings.Contains(css, fragment) {
			t.Errorf("appearance CSS is missing %q:\n%s", fragment, css)
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
