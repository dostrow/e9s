package sqlworkbench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

type fakeAuthProvider struct{ credentials model.SQLCredentials }

func (f fakeAuthProvider) BuildRDSAuthToken(context.Context, string, string) (string, error) {
	return "iam-token", nil
}
func (f fakeAuthProvider) ResolveSQLSecret(context.Context, string) (model.SQLCredentials, error) {
	return f.credentials, nil
}

func TestResolveConnectionUsesProfilePGPassBeforeGlobal(t *testing.T) {
	directory := t.TempDir()
	profilePath, globalPath := filepath.Join(directory, "profile"), filepath.Join(directory, "global")
	for path, password := range map[string]string{profilePath: "profile-password", globalPath: "global-password"} {
		if err := os.WriteFile(path, []byte("db:5432:app:reader:"+password+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := ResolveConnection(context.Background(), config.SQLConnection{Name: "db", Host: "db", Database: "app", User: "reader", PGPassFile: profilePath}, []string{globalPath}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Password != "profile-password" || resolved.PGPassSource != profilePath {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
}

func TestResolveConnectionSecretFillsMissingMetadata(t *testing.T) {
	resolved, err := ResolveConnection(context.Background(), config.SQLConnection{Name: "db", Database: "app", Auth: "secrets-manager", SecretARN: "arn"}, nil,
		fakeAuthProvider{credentials: model.SQLCredentials{Username: "reader", Password: "secret", Host: "db", Port: 5432}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Host != "db" || resolved.User != "reader" || resolved.Password != "secret" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
}

func TestResolveConnectionUsesExplicitTLSRootCertificate(t *testing.T) {
	directory := t.TempDir()
	certificate := filepath.Join(directory, "rds-ca-bundle.pem")
	if err := os.WriteFile(certificate, []byte("test certificate bundle"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveConnection(context.Background(), config.SQLConnection{
		Name: "db", Host: "db", Database: "app", User: "reader", Auth: "password",
		SSLMode: "verify-full", SSLRootCert: certificate,
	}, nil, nil, func(context.Context, config.SQLConnection) (string, error) { return "password", nil })
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(certificate)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SSLRootCert != want {
		t.Fatalf("SSLRootCert = %q, want %q", resolved.SSLRootCert, want)
	}
}

func TestResolveConnectionRejectsMissingTLSRootCertificate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pem")
	_, err := ResolveConnection(context.Background(), config.SQLConnection{
		Name: "db", Host: "db", Database: "app", User: "reader", Auth: "password",
		SSLMode: "verify-full", SSLRootCert: missing,
	}, nil, nil, func(context.Context, config.SQLConnection) (string, error) { return "password", nil })
	if err == nil || !strings.Contains(err.Error(), "TLS root certificate") {
		t.Fatalf("ResolveConnection error = %v, want a TLS root certificate error", err)
	}
}

func TestResolveConnectionIgnoresTLSRootCertificateOutsideVerificationModes(t *testing.T) {
	resolved, err := ResolveConnection(context.Background(), config.SQLConnection{
		Name: "db", Host: "db", Database: "app", User: "reader", Auth: "password",
		SSLMode: "require", SSLRootCert: filepath.Join(t.TempDir(), "missing.pem"),
	}, nil, nil, func(context.Context, config.SQLConnection) (string, error) { return "password", nil })
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SSLRootCert != "" {
		t.Fatalf("SSLRootCert = %q in require mode, want empty", resolved.SSLRootCert)
	}
}
