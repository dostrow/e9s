//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterSecretsSearchesMetadataOnly(t *testing.T) {
	secrets := []model.Secret{
		{Name: "prod/api", Description: "application credentials", ARN: "arn:prod", Tags: map[string]string{"team": "platform"}},
		{Name: "dev/worker", Description: "queue token", ARN: "arn:dev", Tags: map[string]string{"team": "jobs"}},
	}
	for query, want := range map[string]string{
		"API":      "prod/api",
		"queue":    "dev/worker",
		"PLATFORM": "prod/api",
		"arn:dev":  "dev/worker",
	} {
		got := filterSecrets(secrets, query)
		if len(got) != 1 || got[0].Name != want {
			t.Fatalf("filterSecrets(%q) = %#v, want %q", query, got, want)
		}
	}
}

func TestSecretSummaryMasksValueAndSortsTags(t *testing.T) {
	secret := model.Secret{
		Name: "prod/api",
		ARN:  "arn:aws:secretsmanager:region:account:secret:prod/api",
		Tags: map[string]string{"zeta": "last", "alpha": "first"},
	}
	got := formatSecretSummary(secret)
	if !strings.Contains(got, "••••••••") {
		t.Fatalf("summary did not render a masked value: %q", got)
	}
	if strings.Index(got, "alpha = first") > strings.Index(got, "zeta = last") {
		t.Fatalf("summary tags are not deterministic: %q", got)
	}
}

func TestSecretBreadcrumbUsesSavedFilter(t *testing.T) {
	got := secretBreadcrumb("Production", "prod/", "prod/api")
	for _, want := range []string{"Secrets Manager", "Production", "prod/", "prod/api"} {
		if !strings.Contains(got, want) {
			t.Fatalf("breadcrumb %q missing %q", got, want)
		}
	}
}

func TestPrettySecretValueFormatsJSON(t *testing.T) {
	got := prettySecretValue(model.SecretValue{Name: "api", Value: `{"token":"value","enabled":true}`})
	if !strings.Contains(got, "\n") || !strings.Contains(got, `"token": "value"`) {
		t.Fatalf("prettySecretValue() = %q", got)
	}
}

func TestPrettySecretValueDoesNotExposeBinaryData(t *testing.T) {
	got := prettySecretValue(model.SecretValue{Name: "binary", Value: "should-not-render", Binary: true})
	if strings.Contains(got, "should-not-render") || !strings.Contains(got, "binary") {
		t.Fatalf("binary prettySecretValue() = %q", got)
	}
}
