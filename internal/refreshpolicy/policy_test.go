package refreshpolicy

import (
	"testing"
	"time"
)

func TestConservativeMinimumIntervals(t *testing.T) {
	for class, want := range map[Class]time.Duration{
		Operational: 5 * time.Second,
		Inventory:   30 * time.Second,
		Metered:     time.Minute,
		Metrics:     time.Minute,
		Manual:      0,
	} {
		if got := Interval(time.Second, class); got != want {
			t.Errorf("Interval(%d) = %s, want %s", class, got, want)
		}
	}
}

func TestDue(t *testing.T) {
	now := time.Unix(1000, 0)
	if Due(now.Add(-29*time.Second), now, time.Second, Inventory) {
		t.Fatal("inventory refresh became due before its minimum interval")
	}
	if !Due(now.Add(-30*time.Second), now, time.Second, Inventory) {
		t.Fatal("inventory refresh was not due at its minimum interval")
	}
	if Due(time.Time{}, now, time.Second, Manual) {
		t.Fatal("manual views must never auto-refresh")
	}
}
