package gui

import (
	"regexp"
	"strings"
	"testing"
)

func TestApplicationCSSDoesNotOverrideGTKPalette(t *testing.T) {
	hexColor := regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b`)
	if hexColor.MatchString(styleCSS) {
		t.Fatal("application CSS contains a hard-coded color")
	}

	for lineNumber, line := range strings.Split(styleCSS, "\n") {
		declaration := strings.TrimSpace(line)
		if strings.HasPrefix(declaration, "color:") ||
			strings.HasPrefix(declaration, "background:") ||
			strings.HasPrefix(declaration, "background-color:") {
			t.Fatalf("application CSS overrides the GTK palette on line %d: %s", lineNumber+1, declaration)
		}
	}
}

func TestApplicationCSSDefinesEveryZoomLevel(t *testing.T) {
	for level := zoomMinimum; level <= zoomMaximum; level += zoomStep {
		selector := "." + zoomClass(level)
		if !strings.Contains(styleCSS, selector) {
			t.Errorf("application CSS is missing %s", selector)
		}
	}
}

func TestApplicationCSSDefinesSoftPaneSurfaces(t *testing.T) {
	for _, fragment := range []string{
		".pane-card",
		"border-radius: 10px",
		".pane-split > separator",
		".pane-split > separator:hover",
	} {
		if !strings.Contains(styleCSS, fragment) {
			t.Errorf("application CSS is missing %q", fragment)
		}
	}
}
