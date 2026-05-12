package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/aws"
)

func TestNewEnvVarsSortsAlphabeticallyByName(t *testing.T) {
	m := NewEnvVars("test", []aws.EnvVar{
		{Name: "Z_VAR", Value: "z"},
		{Name: "A_VAR", Value: "a"},
		{Name: "M_VAR", Value: "m"},
	})

	filtered := m.filteredVars()
	if len(filtered) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(filtered))
	}
	if filtered[0].Name != "A_VAR" || filtered[1].Name != "M_VAR" || filtered[2].Name != "Z_VAR" {
		t.Fatalf("env vars not sorted alphabetically: %#v", filtered)
	}
}

func TestNewEnvVarsDoesNotMutateInputSlice(t *testing.T) {
	input := []aws.EnvVar{
		{Name: "B_VAR", Value: "b"},
		{Name: "A_VAR", Value: "a"},
	}

	_ = NewEnvVars("test", input)

	if input[0].Name != "B_VAR" || input[1].Name != "A_VAR" {
		t.Fatalf("input slice was mutated: %#v", input)
	}
}
