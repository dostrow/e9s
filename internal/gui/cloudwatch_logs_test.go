//go:build gui

package gui

import (
	"reflect"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestFilterLogGroups(t *testing.T) {
	groups := []model.LogGroup{
		{Name: "/aws/ecs/api", StoredBytes: 1},
		{Name: "/aws/lambda/worker", StoredBytes: 2},
	}

	filtered := filterLogGroups(groups, "LAMBDA")
	if len(filtered) != 1 || filtered[0].Name != "/aws/lambda/worker" {
		t.Fatalf("filtered groups = %#v", filtered)
	}
	if unfiltered := filterLogGroups(groups, ""); len(unfiltered) != 2 {
		t.Fatalf("unfiltered groups = %#v", unfiltered)
	}
}

func TestBuildCloudWatchSearch(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	spec, err := buildCloudWatchSearch(
		" /aws/ecs/api, /aws/ecs/worker, /aws/ecs/api ", "", "request failed", 1, "", "", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Groups, []string{"/aws/ecs/api", "/aws/ecs/worker"}) {
		t.Fatalf("groups = %#v", spec.Groups)
	}
	if spec.Filter != `"request failed"` {
		t.Fatalf("filter = %q", spec.Filter)
	}
	if got := time.UnixMilli(spec.EndTime).Sub(time.UnixMilli(spec.StartTime)); got != time.Hour {
		t.Fatalf("range = %s, want 1h", got)
	}
}

func TestBuildCloudWatchSearchCustomUTC(t *testing.T) {
	spec, err := buildCloudWatchSearch(
		"/aws/ecs/api", "ecs/api/one, ecs/api/two", `{ $.level = "error" }`,
		len(cloudWatchTimePresets)-1, "2026-08-18 10:00", "2026-08-18 11:30", time.Time{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Filter != `{ $.level = "error" }` {
		t.Fatalf("filter = %q", spec.Filter)
	}
	if got := time.UnixMilli(spec.EndTime).Sub(time.UnixMilli(spec.StartTime)); got != 90*time.Minute {
		t.Fatalf("range = %s, want 90m", got)
	}
}

func TestBuildCloudWatchSearchRejectsStreamsAcrossGroups(t *testing.T) {
	_, err := buildCloudWatchSearch("one,two", "stream", "", 0, "", "", time.Now())
	if err == nil {
		t.Fatal("buildCloudWatchSearch() error = nil")
	}
}

func TestCorrelatedCloudWatchSearchAddsTemporaryRule(t *testing.T) {
	existing := model.LogHighlightRule{
		Pattern: "timeout", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightWarning,
	}
	original := cloudWatchSearch{
		Groups: []string{"/aws/ecs/api"}, Streams: []string{"api/one"},
		Filter: `"request failed"`, Lookback: time.Hour, HighlightRules: []model.LogHighlightRule{existing},
	}
	entry := model.LogEntry{Timestamp: 120_000, Message: "Request Failed for job 42"}

	got := correlatedCloudWatchSearch(original, entry, 30*time.Second)
	if got.Filter != "" || got.Lookback != 0 || got.StartTime != 90_000 || got.EndTime != 150_000 {
		t.Fatalf("correlated search = %#v", got)
	}
	if len(got.HighlightRules) != 2 || got.HighlightRules[0].Pattern != "request failed" || got.HighlightRules[1] != existing {
		t.Fatalf("highlight rules = %#v", got.HighlightRules)
	}
	if got.Anchor == nil || got.Anchor.Key() != entry.Key() {
		t.Fatalf("anchor = %#v, want %#v", got.Anchor, entry)
	}
	if len(original.HighlightRules) != 1 || original.HighlightRules[0] != existing {
		t.Fatalf("original rules mutated: %#v", original.HighlightRules)
	}
}

func TestQuoteCloudWatchFilter(t *testing.T) {
	tests := map[string]string{
		"error":                 `"error"`,
		`"already quoted"`:      `"already quoted"`,
		`message "with quotes"`: `"message with quotes"`,
		`{ $.level = "error" }`: `{ $.level = "error" }`,
		`[ip, user]`:            `[ip, user]`,
		"":                      "",
	}
	for input, want := range tests {
		if got := quoteCloudWatchFilter(input); got != want {
			t.Errorf("quoteCloudWatchFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchFromSavedLogUsesRelativeLookback(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	rules := []model.LogHighlightRule{{
		Pattern: "error", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightError,
	}}
	path := config.LogPathEntry{
		Name: "API errors", LogGroup: "/aws/ecs/api", LogGroups: []string{"/aws/ecs/api"},
		Streams: []string{"api/one"}, Filter: `"error"`, Lookback: "6h",
		HighlightRules: rules, HiddenStreams: []string{"api/noisy"},
	}
	spec, err := searchFromSavedLog(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title != path.Name || spec.Lookback != 6*time.Hour {
		t.Fatalf("spec = %#v", spec)
	}
	if got := spec.EndTime - spec.StartTime; got != (6 * time.Hour).Milliseconds() {
		t.Fatalf("range = %dms", got)
	}
	if !reflect.DeepEqual(spec.HighlightRules, rules) {
		t.Fatalf("highlight rules = %#v", spec.HighlightRules)
	}
	if !reflect.DeepEqual(spec.HiddenStreams, path.HiddenStreams) {
		t.Fatalf("hidden streams = %#v", spec.HiddenStreams)
	}
	path.HighlightRules[0].Pattern = "mutated"
	path.HiddenStreams[0] = "mutated"
	if spec.HighlightRules[0].Pattern != "error" {
		t.Fatal("search spec retained saved-search highlight slice")
	}
	if spec.HiddenStreams[0] != "api/noisy" {
		t.Fatal("search spec retained saved-search hidden-stream slice")
	}
}

func TestBuildSavedLogDefinitionRelative(t *testing.T) {
	rules := []model.LogHighlightRule{{
		Pattern: "error", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightError,
	}}
	path, err := buildSavedLogDefinition(
		"API errors", "/aws/ecs/api", "api/one", savedLogModeRelative,
		"request failed", "90m", "", "", "health, debug", rules,
	)
	if err != nil {
		t.Fatal(err)
	}
	if path.Filter != `"request failed"` || path.Lookback != "1h30m0s" {
		t.Fatalf("relative query = %#v", path)
	}
	if !reflect.DeepEqual(path.HiddenStreams, []string{"health", "debug"}) || !reflect.DeepEqual(path.HighlightRules, rules) {
		t.Fatalf("presentation defaults = %#v", path)
	}
}

func TestBuildSavedLogDefinitionRejectsInvalidScopesAndRanges(t *testing.T) {
	if _, err := buildSavedLogDefinition(
		"bad scope", "one,two", "stream", savedLogModeDestination, "", "", "", "", "", nil,
	); err == nil {
		t.Fatal("multi-group stream scope was accepted")
	}
	if _, err := buildSavedLogDefinition(
		"bad range", "one", "", savedLogModeFixed, "", "", "2026-08-18 11:00", "2026-08-18 10:00", "", nil,
	); err == nil {
		t.Fatal("reversed fixed range was accepted")
	}
}

func TestWorkspaceSavedLogDefinitionUsesCurrentSearchAndPresentation(t *testing.T) {
	rules := []model.LogHighlightRule{{
		Pattern: "error", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightError,
	}}
	w := mainWindow{
		showingLogs: true,
		logSearchSpec: &cloudWatchSearch{
			Groups: []string{"/aws/ecs/api"}, Streams: []string{"api/one"},
			Filter: `"error"`, Lookback: 90 * time.Minute,
			StartTime: 1, EndTime: 2,
		},
		logHighlightRules: rules,
		logHiddenStreams:  map[string]struct{}{"api/noisy": {}},
	}
	path, ok := w.workspaceSavedLogDefinition("API errors")
	if !ok {
		t.Fatal("workspace definition was unavailable")
	}
	if path.Name != "API errors" || path.LogGroup != "/aws/ecs/api" || path.Stream != "api/one" {
		t.Fatalf("scope = %#v", path)
	}
	if path.Lookback != "1h30m0s" || path.StartTime != 0 || path.EndTime != 0 {
		t.Fatalf("relative range = %#v", path)
	}
	if !reflect.DeepEqual(path.HighlightRules, rules) || !reflect.DeepEqual(path.HiddenStreams, []string{"api/noisy"}) {
		t.Fatalf("presentation = %#v", path)
	}
}

func TestSavedLogWorkspaceDirtyNormalizesLegacyScope(t *testing.T) {
	saved := config.LogPathEntry{Name: "api", LogGroup: "/aws/ecs/api", Stream: "api/one"}
	w := mainWindow{
		options:        Options{Config: &config.Config{LogPaths: []config.LogPathEntry{saved}}},
		activeSavedLog: "api",
		showingLogs:    true,
		logSource:      model.LogSource{Group: "/aws/ecs/api", Streams: []string{"api/one"}},
	}
	if w.savedLogWorkspaceDirty() {
		t.Fatal("equivalent canonical and legacy scopes were considered different")
	}
	w.logHiddenStreams = map[string]struct{}{"api/noisy": {}}
	if !w.savedLogWorkspaceDirty() {
		t.Fatal("presentation change did not mark the workspace modified")
	}
}

func TestLogHighlightOptionIndices(t *testing.T) {
	if got := highlightMatchOptionIndex(model.LogHighlightRegex); got != 2 {
		t.Fatalf("regex option index = %d", got)
	}
	if got := highlightStyleOptionIndex(model.LogHighlightWarning); got != 3 {
		t.Fatalf("warning option index = %d", got)
	}
	if got := highlightMatchOptionIndex("unknown"); got != 0 {
		t.Fatalf("unknown match option index = %d", got)
	}
}

func TestLegacySavedLogScope(t *testing.T) {
	path := config.LogPathEntry{Name: "api", LogGroup: "/aws/ecs/api", Stream: "api/one"}
	if savedLogHasQuery(path) {
		t.Fatal("legacy destination unexpectedly has a query")
	}
	if got := savedLogGroups(path); !reflect.DeepEqual(got, []string{"/aws/ecs/api"}) {
		t.Fatalf("groups = %#v", got)
	}
	if got := savedLogStreams(path); !reflect.DeepEqual(got, []string{"api/one"}) {
		t.Fatalf("streams = %#v", got)
	}
}

func TestFormatByteSize(t *testing.T) {
	tests := map[int64]string{
		0:           "0 B",
		1024:        "1.0 KiB",
		1536:        "1.5 KiB",
		1024 * 1024: "1.0 MiB",
	}
	for input, want := range tests {
		if got := formatByteSize(input); got != want {
			t.Fatalf("formatByteSize(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestFilterLogStreams(t *testing.T) {
	streams := []model.LogStream{
		{Name: "ecs/api/one"},
		{Name: "ecs/worker/two"},
	}
	filtered := filterLogStreams(streams, "WORKER")
	if len(filtered) != 1 || filtered[0].Name != "ecs/worker/two" {
		t.Fatalf("filtered streams = %#v", filtered)
	}
}
