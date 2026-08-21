package sqlworkbench

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

type AuthProvider interface {
	BuildRDSAuthToken(context.Context, string, string) (string, error)
	ResolveSQLSecret(context.Context, string) (model.SQLCredentials, error)
}

type PasswordPrompt func(context.Context, config.SQLConnection) (string, error)

type ResolvedConnection struct {
	Profile      config.SQLConnection
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	AuthMode     string
	PGPassSource string
	DataAPI      bool
}

func ResolveConnection(ctx context.Context, profile config.SQLConnection, globalPGPassFiles []string, provider AuthProvider, prompt PasswordPrompt) (ResolvedConnection, error) {
	resolved := ResolvedConnection{Profile: profile, Host: strings.TrimSpace(profile.Host), Port: profile.Port,
		Database: strings.TrimSpace(profile.Database), User: strings.TrimSpace(profile.User), SSLMode: strings.TrimSpace(profile.SSLMode),
		AuthMode: strings.ToLower(strings.TrimSpace(profile.Auth))}
	if resolved.Port == 0 {
		resolved.Port = 5432
	}
	if resolved.SSLMode == "" {
		resolved.SSLMode = "verify-full"
	}
	if resolved.AuthMode == "" {
		resolved.AuthMode = "pgpass"
	}
	if resolved.Database == "" {
		return ResolvedConnection{}, fmt.Errorf("database is required")
	}
	switch resolved.AuthMode {
	case "data-api":
		resolved.DataAPI = true
		return resolved, nil
	case "secrets-manager":
		if provider == nil {
			return ResolvedConnection{}, fmt.Errorf("Secrets Manager authentication is unavailable")
		}
		credentials, err := provider.ResolveSQLSecret(ctx, profile.SecretARN)
		if err != nil {
			return ResolvedConnection{}, err
		}
		applySecretCredentials(&resolved, credentials)
	case "iam":
		if provider == nil {
			return ResolvedConnection{}, fmt.Errorf("RDS IAM authentication is unavailable")
		}
		if resolved.Host == "" || resolved.User == "" {
			return ResolvedConnection{}, fmt.Errorf("RDS IAM authentication requires host and user")
		}
		token, err := provider.BuildRDSAuthToken(ctx, endpoint(resolved.Host, resolved.Port), resolved.User)
		if err != nil {
			return ResolvedConnection{}, err
		}
		resolved.Password = token
	case "password":
		if prompt == nil {
			return ResolvedConnection{}, fmt.Errorf("this connection requires an ephemeral password prompt")
		}
		password, err := prompt(ctx, profile)
		if err != nil {
			return ResolvedConnection{}, err
		}
		resolved.Password = password
	case "pgpass":
		if resolved.Host == "" || resolved.User == "" {
			return ResolvedConnection{}, fmt.Errorf("pgpass authentication requires host and user")
		}
		paths := append([]string(nil), globalPGPassFiles...)
		if strings.TrimSpace(profile.PGPassFile) != "" {
			paths = append([]string{profile.PGPassFile}, paths...)
		}
		password, source, err := ResolvePGPass(paths, PGPassMatch{Host: resolved.Host, Port: strconv.Itoa(resolved.Port), Database: resolved.Database, User: resolved.User})
		if err != nil {
			return ResolvedConnection{}, err
		}
		resolved.Password, resolved.PGPassSource = password, source
	default:
		return ResolvedConnection{}, fmt.Errorf("unsupported SQL authentication mode %q", profile.Auth)
	}
	if resolved.Host == "" || resolved.User == "" || resolved.Password == "" {
		return ResolvedConnection{}, fmt.Errorf("SQL connection %q did not resolve a host, user, and password", profile.Name)
	}
	return resolved, nil
}

func applySecretCredentials(connection *ResolvedConnection, credentials model.SQLCredentials) {
	connection.Password = credentials.Password
	if connection.User == "" {
		connection.User = credentials.Username
	}
	if connection.Host == "" {
		connection.Host = credentials.Host
	}
	if connection.Profile.Port == 0 && credentials.Port > 0 {
		connection.Port = credentials.Port
	}
	if connection.Database == "" {
		connection.Database = credentials.Database
	}
}

func endpoint(host string, port int) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port)
}
