package views

import (
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
		{"zero", 0, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input time.Time
			if tt.age == 0 {
				input = time.Time{}
			} else {
				input = time.Now().Add(-tt.age)
			}
			got := formatAge(input)
			if got != tt.want {
				t.Errorf("formatAge(%v ago) = %q, want %q", tt.age, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSetServicesSortsAlphabeticallyByName(t *testing.T) {
	m := NewServiceList("cluster").SetServices([]model.Service{
		{Name: "worker"},
		{Name: "api"},
		{Name: "batch"},
	})

	filtered := m.filteredServices()
	if len(filtered) != 3 {
		t.Fatalf("expected 3 services, got %d", len(filtered))
	}
	if filtered[0].Name != "api" || filtered[1].Name != "batch" || filtered[2].Name != "worker" {
		t.Fatalf("services not sorted alphabetically: %#v", filtered)
	}
}

func TestSetServicesDoesNotMutateInputSlice(t *testing.T) {
	input := []model.Service{
		{Name: "worker"},
		{Name: "api"},
	}

	_ = NewServiceList("cluster").SetServices(input)

	if input[0].Name != "worker" || input[1].Name != "api" {
		t.Fatalf("input slice was mutated: %#v", input)
	}
}
