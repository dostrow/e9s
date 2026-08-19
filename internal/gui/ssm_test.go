//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterSSMParametersDoesNotSearchSecureValues(t *testing.T) {
	parameters := []model.Parameter{
		{Name: "/prod/api/url", Type: "String", Value: "internal.example"},
		{Name: "/prod/api/token", Type: "SecureString", Value: "super-secret"},
	}
	if got := filterSSMParameters(parameters, "EXAMPLE"); len(got) != 1 || got[0].Name != "/prod/api/url" {
		t.Fatalf("ordinary value filter = %#v", got)
	}
	if got := filterSSMParameters(parameters, "super-secret"); len(got) != 0 {
		t.Fatalf("secure value unexpectedly searchable: %#v", got)
	}
	if got := filterSSMParameters(parameters, "TOKEN"); len(got) != 1 || got[0].Name != "/prod/api/token" {
		t.Fatalf("name filter = %#v", got)
	}
}

func TestSSMSecureStringIsMaskedInBrowserAndSummary(t *testing.T) {
	parameter := model.Parameter{Name: "/prod/token", Type: "SecureString", Value: "plaintext-secret", Version: 3}
	if got := ssmListValue(parameter); strings.Contains(got, parameter.Value) {
		t.Fatalf("browser value exposed SecureString: %q", got)
	}
	if got := formatSSMParameterSummary(parameter); strings.Contains(got, parameter.Value) {
		t.Fatalf("summary exposed SecureString: %q", got)
	}
}

func TestSSMBreadcrumbUsesSavedPrefixName(t *testing.T) {
	got := ssmBreadcrumb("Production", "/prod", "/prod/api")
	for _, want := range []string{"Production", "/prod", "/prod/api"} {
		if !strings.Contains(got, want) {
			t.Fatalf("breadcrumb %q missing %q", got, want)
		}
	}
}
