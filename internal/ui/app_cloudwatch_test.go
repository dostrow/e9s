package ui

import (
	"reflect"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestSavedLogPathScopesSupportNewAndLegacyFields(t *testing.T) {
	path := config.LogPathEntry{
		LogGroup: "/aws/ecs/api", LogGroups: []string{"/aws/ecs/api", "/aws/ecs/worker"},
		Stream: "legacy", Streams: []string{"api/one", "api/two"},
	}
	if got := savedLogPathGroups(path); !reflect.DeepEqual(got, path.LogGroups) {
		t.Fatalf("groups = %#v", got)
	}
	if got := savedLogPathStreams(path); !reflect.DeepEqual(got, path.Streams) {
		t.Fatalf("streams = %#v", got)
	}

	legacy := config.LogPathEntry{LogGroup: "/aws/ecs/api", Stream: "api/one"}
	if got := savedLogPathGroups(legacy); !reflect.DeepEqual(got, []string{"/aws/ecs/api"}) {
		t.Fatalf("legacy groups = %#v", got)
	}
	if got := savedLogPathStreams(legacy); !reflect.DeepEqual(got, []string{"api/one"}) {
		t.Fatalf("legacy streams = %#v", got)
	}
}

func TestSavedLogPathCarriesHighlightsToSearchAndTail(t *testing.T) {
	rules := []model.LogHighlightRule{{
		Pattern: "ERROR", Match: model.LogHighlightLiteral, Style: model.LogHighlightError,
	}}
	a := App{cfg: &config.Config{}}
	search := config.LogPathEntry{
		Name: "errors", LogGroup: "/aws/ecs/api", Filter: `"ERROR"`, Lookback: time.Hour.String(),
		HighlightRules: rules,
	}
	got, _ := a.openSavedLogDestination(search)
	if got.logSearchSavedPath != "errors" || !reflect.DeepEqual(got.logSearchHighlightRules, rules) {
		t.Fatalf("saved search state = %q %#v", got.logSearchSavedPath, got.logSearchHighlightRules)
	}

	tail := config.LogPathEntry{
		Name: "api", LogGroup: "/aws/ecs/api", Stream: "ecs/api/one", HighlightRules: rules,
	}
	got, cmd := a.openSavedLogDestination(tail)
	if cmd == nil {
		t.Fatal("saved tail did not return a command")
	}
	msg, ok := cmd().(logReadyMsg)
	if !ok || msg.savedLogPath != "api" || !reflect.DeepEqual(msg.highlightRules, rules) {
		t.Fatalf("saved tail message = %#v", msg)
	}
	if got.logBrowseSavedPath != "api" {
		t.Fatalf("saved browser path = %q", got.logBrowseSavedPath)
	}
}
