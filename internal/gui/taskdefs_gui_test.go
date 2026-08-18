//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterAndFindPreviousTaskDefinitions(t *testing.T) {
	definitions := []model.TaskDefRef{
		{ARN: "arn:api:4", Family: "api", Revision: 4},
		{ARN: "arn:worker:3", Family: "worker", Revision: 3},
		{ARN: "arn:api:2", Family: "api", Revision: 2},
		{ARN: "arn:api:1", Family: "api", Revision: 1},
	}
	filtered := filterTaskDefinitions(definitions, "WORKER")
	if len(filtered) != 1 || filtered[0].Family != "worker" {
		t.Fatalf("filterTaskDefinitions() = %#v", filtered)
	}
	previous, found := previousTaskDefinition(definitions, "api", 4)
	if !found || previous.Revision != 2 {
		t.Fatalf("previousTaskDefinition() = %#v, %v", previous, found)
	}
}

func TestFormatTaskDefinitionEnvironmentProtectsSecretsUntilResolved(t *testing.T) {
	definition := &model.TaskDefSummary{Family: "api", Revision: 7}
	environment := []model.EnvVar{
		{Name: "PLAIN", Value: "visible"},
		{Name: "TOKEN", Value: "arn:secret", ResolvedValue: "secret-value", Source: "secrets-manager"},
	}
	hidden := formatTaskDefinitionEnvironment(definition, "api", environment, false)
	if !strings.Contains(hidden, "arn:secret") || strings.Contains(hidden, "secret-value") {
		t.Fatalf("unresolved environment leaked or hid wrong value:\n%s", hidden)
	}
	revealed := formatTaskDefinitionEnvironment(definition, "api", environment, true)
	if !strings.Contains(revealed, "secret-value") {
		t.Fatalf("resolved environment missing value:\n%s", revealed)
	}
}

func TestFormatTaskDefinitionSummary(t *testing.T) {
	definition := &model.TaskDefSummary{
		ARN: "arn:api:7", Family: "api", Revision: 7, CPU: "512", Memory: "1024",
		Containers: []model.TaskDefContainer{{Name: "api", Image: "example/api:7", EnvVars: []model.EnvVar{{Name: "MODE"}}}},
	}
	got := formatTaskDefinitionSummary(definition)
	for _, want := range []string{"api:7", "512", "1024", "example/api:7", "1 variables"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatTaskDefinitionSummary() missing %q:\n%s", want, got)
		}
	}
}
