//go:build gui

package gui

import (
	"testing"

	"github.com/dostrow/e9s/internal/config"
)

func TestSQLProfilePresentationDefaults(t *testing.T) {
	profile := config.SQLConnection{Name: "prod", Host: "db.example", Database: "app"}
	if got := sqlProfileResource(profile); got != "db.example:5432" {
		t.Fatalf("resource = %q", got)
	}
	if got := sqlAuthMode(profile); got != "pgpass" {
		t.Fatalf("auth = %q", got)
	}
}
