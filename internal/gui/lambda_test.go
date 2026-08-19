//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterLambdaFunctionsSearchesLoadedMetadata(t *testing.T) {
	functions := []model.LambdaFunction{
		{Name: "prod-api", Description: "HTTP handler", ARN: "arn:api", Runtime: "provided.al2023", State: "Active"},
		{Name: "queue-worker", Description: "Processes jobs", ARN: "arn:worker", Runtime: "go1.x", State: "Pending"},
	}
	for query, want := range map[string]string{
		"API":        "prod-api",
		"jobs":       "queue-worker",
		"PROVIDED":   "prod-api",
		"pending":    "queue-worker",
		"arn:worker": "queue-worker",
	} {
		got := filterLambdaFunctions(functions, query)
		if len(got) != 1 || got[0].Name != want {
			t.Fatalf("filterLambdaFunctions(%q) = %#v, want %q", query, got, want)
		}
	}
}

func TestLambdaSummaryIncludesOperationalConfiguration(t *testing.T) {
	function := model.LambdaFunction{
		Name: "worker", Runtime: "go1.x", Handler: "bootstrap", PackageType: "Zip",
		MemoryMB: 512, TimeoutSec: 30, CodeSize: 1536, LogGroup: "/aws/lambda/worker",
		LastModified: time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
	}
	got := formatLambdaSummary(function)
	for _, want := range []string{"worker", "go1.x", "bootstrap", "Zip", "512 MB", "30s", "1.5 KB", "/aws/lambda/worker"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func TestLambdaBreadcrumbUsesSavedSearch(t *testing.T) {
	got := lambdaBreadcrumb("Production", "prod-", "prod-worker")
	for _, want := range []string{"Lambda", "Production", "prod-", "prod-worker"} {
		if !strings.Contains(got, want) {
			t.Fatalf("breadcrumb %q missing %q", got, want)
		}
	}
}

func TestFormatLambdaBytes(t *testing.T) {
	for size, want := range map[int64]string{512: "512 B", 1536: "1.5 KB", 3 << 20: "3.0 MB"} {
		if got := formatLambdaBytes(size); got != want {
			t.Errorf("formatLambdaBytes(%d) = %q, want %q", size, got, want)
		}
	}
}
