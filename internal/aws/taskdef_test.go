package aws

import (
	"strings"
	"testing"
)

func TestDiffTaskDefinitions_NoDifferences(t *testing.T) {
	old := &TaskDefSummary{Family: "app", Revision: 1, CPU: "256", Memory: "512"}
	new := &TaskDefSummary{Family: "app", Revision: 2, CPU: "256", Memory: "512"}

	diff := DiffTaskDefinitions(old, new)
	if diff != "No differences found." {
		t.Errorf("Expected no differences, got: %s", diff)
	}
}

func TestDiffTaskDefinitions_CPUMemoryChange(t *testing.T) {
	old := &TaskDefSummary{Family: "app", Revision: 1, CPU: "256", Memory: "512"}
	new := &TaskDefSummary{Family: "app", Revision: 2, CPU: "512", Memory: "1024"}

	diff := DiffTaskDefinitions(old, new)
	if !strings.Contains(diff, "CPU: 256 → 512") {
		t.Errorf("Expected CPU change in diff: %s", diff)
	}
	if !strings.Contains(diff, "Memory: 512 → 1024") {
		t.Errorf("Expected Memory change in diff: %s", diff)
	}
}

func TestDiffTaskDefinitions_ImageChange(t *testing.T) {
	old := &TaskDefSummary{
		Family: "app", Revision: 1,
		Containers: []TaskDefContainer{
			{Name: "web", Image: "repo/app:v1"},
		},
	}
	new := &TaskDefSummary{
		Family: "app", Revision: 2,
		Containers: []TaskDefContainer{
			{Name: "web", Image: "repo/app:v2"},
		},
	}

	diff := DiffTaskDefinitions(old, new)
	if !strings.Contains(diff, "repo/app:v1 → repo/app:v2") {
		t.Errorf("Expected image change in diff: %s", diff)
	}
}

func TestDiffTaskDefinitions_ContainerAdded(t *testing.T) {
	old := &TaskDefSummary{Family: "app", Revision: 1}
	new := &TaskDefSummary{
		Family: "app", Revision: 2,
		Containers: []TaskDefContainer{
			{Name: "sidecar", Image: "repo/sidecar:v1"},
		},
	}

	diff := DiffTaskDefinitions(old, new)
	if !strings.Contains(diff, "Container added: sidecar") {
		t.Errorf("Expected container added in diff: %s", diff)
	}
}

func TestDiffTaskDefinitions_ContainerRemoved(t *testing.T) {
	old := &TaskDefSummary{
		Family: "app", Revision: 1,
		Containers: []TaskDefContainer{
			{Name: "sidecar", Image: "repo/sidecar:v1"},
		},
	}
	new := &TaskDefSummary{Family: "app", Revision: 2}

	diff := DiffTaskDefinitions(old, new)
	if !strings.Contains(diff, "Container removed: sidecar") {
		t.Errorf("Expected container removed in diff: %s", diff)
	}
}

func TestDiffTaskDefinitions_EnvVarChange(t *testing.T) {
	old := &TaskDefSummary{
		Family: "app", Revision: 1,
		Containers: []TaskDefContainer{
			{Name: "web", Image: "img", EnvVarKeys: []string{"DB_HOST", "OLD_VAR"}},
		},
	}
	new := &TaskDefSummary{
		Family: "app", Revision: 2,
		Containers: []TaskDefContainer{
			{Name: "web", Image: "img", EnvVarKeys: []string{"DB_HOST", "NEW_VAR"}},
		},
	}

	diff := DiffTaskDefinitions(old, new)
	if !strings.Contains(diff, "+ Env: NEW_VAR") {
		t.Errorf("Expected new env var in diff: %s", diff)
	}
	if !strings.Contains(diff, "- Env: OLD_VAR") {
		t.Errorf("Expected removed env var in diff: %s", diff)
	}
}

func TestExtractSSMParamName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"arn:aws:ssm:us-east-1:123456:parameter/my/param", "/my/param"},
		{"/my/param", "/my/param"},
		{"plain-name", "plain-name"},
	}
	for _, tt := range tests {
		got := extractSSMParamName(tt.input)
		if got != tt.want {
			t.Errorf("extractSSMParamName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSecretARN(t *testing.T) {
	tests := []struct {
		input     string
		wantARN   string
		wantKey   string
	}{
		{
			"arn:aws:secretsmanager:us-east-1:123456:secret:my-secret",
			"arn:aws:secretsmanager:us-east-1:123456:secret:my-secret",
			"",
		},
		{
			"arn:aws:secretsmanager:us-east-1:123456:secret:my-secret:username::",
			"arn:aws:secretsmanager:us-east-1:123456:secret:my-secret",
			"username",
		},
	}
	for _, tt := range tests {
		gotARN, gotKey := parseSecretARN(tt.input)
		if gotARN != tt.wantARN {
			t.Errorf("parseSecretARN(%q) arn = %q, want %q", tt.input, gotARN, tt.wantARN)
		}
		if gotKey != tt.wantKey {
			t.Errorf("parseSecretARN(%q) key = %q, want %q", tt.input, gotKey, tt.wantKey)
		}
	}
}

func TestExtractJSONKey(t *testing.T) {
	json := `{"username":"admin","password":"secret123"}`

	val, err := extractJSONKey(json, "username")
	if err != nil {
		t.Fatalf("extractJSONKey error: %v", err)
	}
	if val != "admin" {
		t.Errorf("extractJSONKey(username) = %q, want %q", val, "admin")
	}

	_, err = extractJSONKey(json, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}

	_, err = extractJSONKey("not json", "key")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestBuildLogStreamName(t *testing.T) {
	got := BuildLogStreamName("prefix", "container", "taskid123")
	want := "prefix/container/taskid123"
	if got != want {
		t.Errorf("BuildLogStreamName = %q, want %q", got, want)
	}
}

func TestDynamoItemToJSON(t *testing.T) {
	item := DynamoItem{
		"name": "test",
		"age":  float64(30),
	}
	json := DynamoItemToJSON(item)
	if !strings.Contains(json, `"name"`) {
		t.Errorf("Expected name in JSON: %s", json)
	}
	if !strings.Contains(json, `"age"`) {
		t.Errorf("Expected age in JSON: %s", json)
	}
}
