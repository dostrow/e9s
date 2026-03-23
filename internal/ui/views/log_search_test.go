package views

import (
	"strings"
	"testing"
)

func TestSplitGroupStream_WithPipe(t *testing.T) {
	group, stream := splitGroupStream("/aws/ecs/cluster|stream-name/abc123", "default")
	if group != "/aws/ecs/cluster" {
		t.Errorf("group = %q, want %q", group, "/aws/ecs/cluster")
	}
	if stream != "stream-name/abc123" {
		t.Errorf("stream = %q, want %q", stream, "stream-name/abc123")
	}
}

func TestSplitGroupStream_NoPipe_LogGroup(t *testing.T) {
	group, stream := splitGroupStream("/aws/lambda/my-func", "default")
	if group != "/aws/lambda/my-func" {
		t.Errorf("group = %q, want %q", group, "/aws/lambda/my-func")
	}
	if stream != "" {
		t.Errorf("stream = %q, want empty", stream)
	}
}

func TestSplitGroupStream_NoPipe_PlainStream(t *testing.T) {
	group, stream := splitGroupStream("some-stream", "default-group")
	if group != "default-group" {
		t.Errorf("group = %q, want %q", group, "default-group")
	}
	if stream != "some-stream" {
		t.Errorf("stream = %q, want %q", stream, "some-stream")
	}
}

func TestHighlightPattern(t *testing.T) {
	result := highlightPattern("Error occurred in module", "error")
	// In test environment, lipgloss may not emit ANSI codes, so just check
	// the function doesn't panic and returns something containing the original text
	if !strings.Contains(result, "occurred in module") {
		t.Errorf("Expected result to contain surrounding text, got: %q", result)
	}
}

func TestHighlightPattern_NoMatch(t *testing.T) {
	input := "No match here"
	result := highlightPattern(input, "xyz")
	if result != input {
		t.Errorf("Expected unchanged output for no match")
	}
}

func TestHighlightPattern_EmptyPattern(t *testing.T) {
	input := "Some text"
	result := highlightPattern(input, "")
	if result != input {
		t.Errorf("Expected unchanged output for empty pattern")
	}
}
