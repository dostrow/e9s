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
	path := config.LogPathEntry{
		Name: "API errors", LogGroup: "/aws/ecs/api", LogGroups: []string{"/aws/ecs/api"},
		Streams: []string{"api/one"}, Filter: `"error"`, Lookback: "6h",
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
