//go:build gui

package gui

import (
	"os"
	"path/filepath"
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

func TestFormatLambdaEnvironmentSortsAndControlsResolvedValues(t *testing.T) {
	environment := []model.EnvVar{
		{Name: "TOKEN", Value: "arn:secret", ResolvedValue: "plaintext", Source: "secrets-manager"},
		{Name: "MODE", Value: "production"},
	}
	masked := formatLambdaEnvironment("worker", environment, false)
	if strings.Contains(masked, "plaintext") || !strings.Contains(masked, "arn:secret") {
		t.Fatalf("unresolved environment exposed or omitted the wrong value: %q", masked)
	}
	if strings.Index(masked, "MODE") > strings.Index(masked, "TOKEN") {
		t.Fatalf("environment is not sorted by name: %q", masked)
	}
	resolved := formatLambdaEnvironment("worker", environment, true)
	if !strings.Contains(resolved, "plaintext") {
		t.Fatalf("resolved environment missing resolved value: %q", resolved)
	}
}

func TestScanLambdaEditableFilesSkipsBinaryAndSorts(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "z.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "nested", "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "binary"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := scanLambdaEditableFiles(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != filepath.Join("nested", "a.txt") || files[1].Path != "z.go" {
		t.Fatalf("scanLambdaEditableFiles() = %#v", files)
	}
}

func TestWriteLambdaEditableFilesRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "handler.go")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLambdaEditableFiles(directory, []lambdaEditableFile{{Path: "handler.go", Content: "new"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new" {
		t.Fatalf("updated file = %q, %v", got, err)
	}
	if err := writeLambdaEditableFiles(directory, []lambdaEditableFile{{Path: "../escape", Content: "bad"}}); err == nil {
		t.Fatal("writeLambdaEditableFiles accepted path traversal")
	}
}
