package highlight

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestMatcherModesAndUnicodeRuneOffsets(t *testing.T) {
	rules := []model.LogHighlightRule{
		{Pattern: "ERROR", Match: model.LogHighlightLiteral, Style: model.LogHighlightError},
		{Pattern: "timeout", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightWarning},
		{Pattern: `request_id=[a-f0-9-]+`, Match: model.LogHighlightRegex, Style: model.LogHighlightInfo},
	}
	matcher, err := Compile(rules)
	if err != nil {
		t.Fatal(err)
	}
	text := "🔥 ERROR Timeout request_id=abc-123 timeout"
	want := []model.LogHighlightSpan{
		{Start: 2, End: 7, Style: model.LogHighlightError},
		{Start: 8, End: 15, Style: model.LogHighlightWarning},
		{Start: 16, End: 34, Style: model.LogHighlightInfo},
		{Start: 35, End: 42, Style: model.LogHighlightWarning},
	}
	if got := matcher.Spans(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("Spans() = %#v, want %#v", got, want)
	}
}

func TestEarlierRulesWinOverlaps(t *testing.T) {
	matcher, err := Compile([]model.LogHighlightRule{
		{Pattern: "ERROR timeout", Match: model.LogHighlightLiteral, Style: model.LogHighlightError},
		{Pattern: "timeout", Match: model.LogHighlightLiteral, Style: model.LogHighlightWarning},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []model.LogHighlightSpan{{Start: 0, End: 13, Style: model.LogHighlightError}}
	if got := matcher.Spans("ERROR timeout"); !reflect.DeepEqual(got, want) {
		t.Fatalf("Spans() = %#v, want %#v", got, want)
	}
}

func TestCompileRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		rule model.LogHighlightRule
		want string
	}{
		{name: "empty", rule: model.LogHighlightRule{Match: model.LogHighlightLiteral, Style: model.LogHighlightDefault}, want: "pattern cannot be empty"},
		{name: "match", rule: model.LogHighlightRule{Pattern: "x", Match: "glob", Style: model.LogHighlightDefault}, want: "unknown match type"},
		{name: "style", rule: model.LogHighlightRule{Pattern: "x", Match: model.LogHighlightLiteral, Style: "purple"}, want: "unknown style"},
		{name: "regex", rule: model.LogHighlightRule{Pattern: "[", Match: model.LogHighlightRegex, Style: model.LogHighlightDefault}, want: "invalid regular expression"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile([]model.LogHighlightRule{tt.rule})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Compile() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestZeroWidthRegexMatchesAreIgnored(t *testing.T) {
	matcher, err := Compile([]model.LogHighlightRule{{
		Pattern: "^", Match: model.LogHighlightRegex, Style: model.LogHighlightDefault,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := matcher.Spans("line"); len(got) != 0 {
		t.Fatalf("Spans() = %#v, want no spans", got)
	}
}
