package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/dostrow/e9s/internal/model"
)

// ListSecrets fetches all secret metadata without secret values.
func (c *Client) ListSecrets(ctx context.Context) ([]model.Secret, error) {
	input := &secretsmanager.ListSecretsInput{}

	var secrets []model.Secret
	paginator := secretsmanager.NewListSecretsPaginator(c.SM, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.SecretList {
			name := derefStrAws(s.Name)
			sec := model.Secret{
				Name: name,
				ARN:  derefStrAws(s.ARN),
				Tags: make(map[string]string),
			}
			if s.Description != nil {
				sec.Description = *s.Description
			}
			if s.LastAccessedDate != nil {
				sec.LastAccessed = *s.LastAccessedDate
			}
			if s.LastChangedDate != nil {
				sec.LastChanged = *s.LastChangedDate
			}
			for _, t := range s.Tags {
				if t.Key != nil && t.Value != nil {
					sec.Tags[*t.Key] = *t.Value
				}
			}
			secrets = append(secrets, sec)
		}
	}
	return secrets, nil
}

// GetSecretValueByName fetches the current value of a secret.
func (c *Client) GetSecretValueByName(ctx context.Context, secretName string) (*model.SecretValue, error) {
	out, err := c.SM.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return nil, err
	}
	value := "(binary secret)"
	binary := true
	if out.SecretString != nil {
		value = *out.SecretString
		binary = false
	}
	return &model.SecretValue{
		Name:   derefStrAws(out.Name),
		Value:  value,
		Binary: binary,
	}, nil
}

// CreateSecret creates a new secret with the given name and value.
func (c *Client) CreateSecret(ctx context.Context, name, value, description string) error {
	input := &secretsmanager.CreateSecretInput{
		Name:         &name,
		SecretString: &value,
	}
	if description != "" {
		input.Description = &description
	}
	_, err := c.SM.CreateSecret(ctx, input)
	return err
}

// PutSecretValue updates a secret's value.
func (c *Client) PutSecretValue(ctx context.Context, secretName, value string) error {
	_, err := c.SM.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     &secretName,
		SecretString: &value,
	})
	return err
}
