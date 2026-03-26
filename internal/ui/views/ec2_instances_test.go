package views

import (
	"testing"
	"time"
)

func TestShortDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 45 * time.Second, "45s"},
		{"exactly 1 minute", time.Minute, "1m"},
		{"minutes", 90 * time.Second, "1m"},
		{"59 minutes", 59 * time.Minute, "59m"},
		{"exactly 1 hour", time.Hour, "1h"},
		{"hours", 5*time.Hour + 30*time.Minute, "5h"},
		{"23 hours", 23*time.Hour + 59*time.Minute, "23h"},
		{"exactly 1 day", 24 * time.Hour, "1d"},
		{"several days", 10 * 24 * time.Hour, "10d"},
		{"364 days", 364 * 24 * time.Hour, "364d"},
		{"exactly 1 year", 365 * 24 * time.Hour, "1y0d"},
		{"1 year and some days", 400 * 24 * time.Hour, "1y35d"},
		{"2 years", 730 * 24 * time.Hour, "2y0d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortDuration(tt.d)
			if got != tt.want {
				t.Errorf("shortDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
