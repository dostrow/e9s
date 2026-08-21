package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// SecretsAPI is the low-level Secrets Manager behavior used by shared workflows.
type SecretsAPI interface {
	ListSecrets(context.Context) ([]model.Secret, error)
	GetSecretValueByName(context.Context, string) (*model.SecretValue, error)
	CreateSecret(context.Context, string, string, string) error
	PutSecretValue(context.Context, string, string) error
}

// Secrets exposes Secrets Manager workflows without frontend dependencies.
type Secrets struct {
	api SecretsAPI
}

func NewSecrets(api SecretsAPI) *Secrets {
	return &Secrets{api: api}
}

// List returns secret metadata in deterministic name order. Filtering is kept
// client-side because the AWS API's name filter is prefix-based rather than the
// substring behavior exposed by e9s.
func (s *Secrets) List(ctx context.Context, nameFilter string) ([]model.Secret, error) {
	secrets, err := s.api.ListSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Secrets Manager secrets: %w", err)
	}
	filter := strings.ToLower(strings.TrimSpace(nameFilter))
	if filter != "" {
		filtered := make([]model.Secret, 0, len(secrets))
		for _, secret := range secrets {
			if strings.Contains(strings.ToLower(secret.Name), filter) {
				filtered = append(filtered, secret)
			}
		}
		secrets = filtered
	}
	sort.SliceStable(secrets, func(i, j int) bool {
		return strings.ToLower(secrets[i].Name) < strings.ToLower(secrets[j].Name)
	})
	return secrets, nil
}

func (s *Secrets) Detail(ctx context.Context, name string) (*model.SecretValue, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("read Secrets Manager secret: name is required")
	}
	value, err := s.api.GetSecretValueByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("read Secrets Manager secret %q: %w", name, err)
	}
	if value == nil {
		return nil, fmt.Errorf("read Secrets Manager secret %q: secret was not found", name)
	}
	return value, nil
}

func (s *Secrets) Create(ctx context.Context, name, value, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("create Secrets Manager secret: name is required")
	}
	if err := s.api.CreateSecret(ctx, name, value, description); err != nil {
		return fmt.Errorf("create Secrets Manager secret %q: %w", name, err)
	}
	return nil
}

func (s *Secrets) Update(ctx context.Context, name, value string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("update Secrets Manager secret: name is required")
	}
	if err := s.api.PutSecretValue(ctx, name, value); err != nil {
		return fmt.Errorf("update Secrets Manager secret %q: %w", name, err)
	}
	return nil
}
