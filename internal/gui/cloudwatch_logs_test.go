//go:build gui

package gui

import (
	"testing"

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
