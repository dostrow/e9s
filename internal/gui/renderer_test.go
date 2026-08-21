//go:build gui

package gui

import (
	"os"
	"testing"
)

func TestConfigureHyprlandRendererUsesGLByDefault(t *testing.T) {
	t.Setenv("GSK_RENDERER", "")
	t.Setenv("GDK_BACKEND", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("XDG_SESSION_DESKTOP", "")

	if !configureHyprlandRenderer() {
		t.Fatal("expected the Hyprland renderer safeguard to be applied")
	}
	if got := os.Getenv("GSK_RENDERER"); got != "gl" {
		t.Fatalf("GSK_RENDERER = %q, want gl", got)
	}
}

func TestConfigureHyprlandRendererPreservesExplicitChoice(t *testing.T) {
	t.Setenv("GSK_RENDERER", "vulkan")
	t.Setenv("GDK_BACKEND", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")

	if configureHyprlandRenderer() {
		t.Fatal("explicit renderer choice was unexpectedly replaced")
	}
	if got := os.Getenv("GSK_RENDERER"); got != "vulkan" {
		t.Fatalf("GSK_RENDERER = %q, want vulkan", got)
	}
}

func TestConfigureHyprlandRendererLeavesOtherSessionsAlone(t *testing.T) {
	t.Setenv("GSK_RENDERER", "")
	t.Setenv("GDK_BACKEND", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	t.Setenv("XDG_SESSION_DESKTOP", "gnome")

	if configureHyprlandRenderer() {
		t.Fatal("renderer safeguard unexpectedly applied outside Hyprland")
	}
	if got := os.Getenv("GSK_RENDERER"); got != "" {
		t.Fatalf("GSK_RENDERER = %q, want empty", got)
	}
}
