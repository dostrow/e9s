//go:build gui

package gui

import "testing"

func TestMetricFractionClampsToProgressRange(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  float64
	}{
		{value: -4, want: 0},
		{value: 0, want: 0},
		{value: 42.5, want: 0.425},
		{value: 100, want: 1},
		{value: 180, want: 1},
	} {
		if got := metricFraction(test.value); got != test.want {
			t.Errorf("metricFraction(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}
