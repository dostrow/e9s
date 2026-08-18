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
