package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPadToWidth_Pad(t *testing.T) {
	got := padToWidth("hi", 10)
	if len(got) != 10 {
		t.Errorf("padToWidth(\"hi\", 10) len = %d, want 10", len(got))
	}
	if got != "hi        " {
		t.Errorf("padToWidth(\"hi\", 10) = %q", got)
	}
}

func TestPadToWidth_Exact(t *testing.T) {
	got := padToWidth("hello", 5)
	if got != "hello" {
		t.Errorf("padToWidth(\"hello\", 5) = %q", got)
	}
}

func TestPadToWidth_Truncate(t *testing.T) {
	got := padToWidth("hello world", 5)
	if len(got) > 5 {
		t.Errorf("padToWidth truncate len = %d, want <= 5", len(got))
	}
}

func TestModeDisplayName(t *testing.T) {
	tests := []struct {
		mode topMode
		want string
	}{
		{modeECS, "ECS"},
		{modeCWLogs, "CloudWatch Logs"},
		{modeCWAlarms, "CloudWatch Alarms"},
		{modeSSM, "SSM"},
		{modeSM, "Secrets Manager"},
		{modeS3, "S3"},
		{modeLambda, "Lambda"},
		{modeDynamoDB, "DynamoDB"},
	}
	for _, tt := range tests {
		got := modeDisplayName(tt.mode)
		if got != tt.want {
			t.Errorf("modeDisplayName(%d) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestModeShortName(t *testing.T) {
	if got := modeShortName(modeECS); got != "ECS" {
		t.Errorf("modeShortName(ECS) = %q", got)
	}
	if got := modeShortName(modeLambda); got != "λ" {
		t.Errorf("modeShortName(Lambda) = %q", got)
	}
}

func TestBuildInfoBar(t *testing.T) {
	bar := buildInfoBar([]string{"cluster", "service"}, "us-east-1",
		time.Time{}, false, "", time.Time{}, nil)
	if !strings.Contains(bar, "cluster") {
		t.Error("Should contain breadcrumbs")
	}
	if !strings.Contains(bar, "us-east-1") {
		t.Error("Should contain region")
	}
}

func TestBuildInfoBar_WithError(t *testing.T) {
	bar := buildInfoBar(nil, "us-east-1", time.Time{}, false, "", time.Time{},
		fmt.Errorf("test error"))
	if !strings.Contains(bar, "test error") {
		t.Error("Should contain error message")
	}
}

func TestResolveDefaultMode(t *testing.T) {
	tests := []struct {
		input string
		want  *topMode
	}{
		{"ECS", ptr(modeECS)},
		{"ecs", ptr(modeECS)},
		{"CloudWatch", ptr(modeCWLogs)},
		{"CW", ptr(modeCWLogs)},
		{"cw", ptr(modeCWLogs)},
		{"CWA", ptr(modeCWAlarms)},
		{"cwa", ptr(modeCWAlarms)},
		{"SSM", ptr(modeSSM)},
		{"DDB", ptr(modeDynamoDB)},
		{"dynamodb", ptr(modeDynamoDB)},
		{"Lambda", ptr(modeLambda)},
		{"", nil},
		{"nonexistent", nil},
	}
	for _, tt := range tests {
		got := resolveDefaultMode(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("resolveDefaultMode(%q) = %v, want nil", tt.input, *got)
			}
		} else {
			if got == nil {
				t.Errorf("resolveDefaultMode(%q) = nil, want %v", tt.input, *tt.want)
			} else if *got != *tt.want {
				t.Errorf("resolveDefaultMode(%q) = %v, want %v", tt.input, *got, *tt.want)
			}
		}
	}
}

func ptr(m topMode) *topMode { return &m }
