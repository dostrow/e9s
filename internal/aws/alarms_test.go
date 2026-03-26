package aws

import (
	"testing"
)

func TestDerefFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input *float64
		want  float64
	}{
		{"nil pointer returns zero", nil, 0},
		{"non-nil pointer returns value", float64Ptr(3.14), 3.14},
		{"zero value pointer returns zero", float64Ptr(0), 0},
		{"negative value", float64Ptr(-42.5), -42.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derefFloat64(tt.input)
			if got != tt.want {
				t.Errorf("derefFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDerefInt32(t *testing.T) {
	tests := []struct {
		name  string
		input *int32
		want  int32
	}{
		{"nil pointer returns zero", nil, 0},
		{"non-nil pointer returns value", int32PtrAlarm(42), 42},
		{"zero value pointer returns zero", int32PtrAlarm(0), 0},
		{"negative value", int32PtrAlarm(-10), -10},
		{"large value", int32PtrAlarm(2147483647), 2147483647},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derefInt32(tt.input)
			if got != tt.want {
				t.Errorf("derefInt32(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func float64Ptr(v float64) *float64 { return &v }
func int32PtrAlarm(v int32) *int32  { return &v }
