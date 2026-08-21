//go:build gui && linux

package gui

import (
	"path/filepath"
	"testing"
)

func TestBundledFontCandidatesHonorDataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("E9S_DATA_DIR", root)
	candidates := bundledFontCandidates()
	if len(candidates) == 0 || candidates[0] != filepath.Join(root, "fonts") {
		t.Fatalf("first bundled font candidate = %v, want E9S_DATA_DIR/fonts", candidates)
	}
}

func TestBundledFontDirectoryFindsDevelopmentAssets(t *testing.T) {
	if directory := bundledFontDirectory(); directory == "" {
		t.Fatal("bundledFontDirectory did not find repository font assets")
	}
}
