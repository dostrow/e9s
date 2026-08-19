package model

import "time"

// Secret is the UI-neutral Secrets Manager list representation. List operations
// never include the secret value; callers must explicitly request SecretValue.
type Secret struct {
	Name         string
	ARN          string
	Description  string
	Tags         map[string]string
	LastAccessed time.Time
	LastChanged  time.Time
}

// SecretValue is the current value of a Secrets Manager secret.
type SecretValue struct {
	Name   string
	Value  string
	Binary bool
}
