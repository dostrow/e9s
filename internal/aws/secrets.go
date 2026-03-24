package aws

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type Secret struct {
	Name         string
	ARN          string
	Description  string
	Tags         map[string]string
	LastAccessed time.Time
	LastChanged  time.Time
}

type SecretValue struct {
	Name  string
	Value string // the secret string (or "(binary)" for binary secrets)
}

// ListSecrets fetches secrets, optionally filtered by name substring.
// Fetches all secrets and filters client-side for true substring matching
// (the AWS API only supports prefix matching).
func (c *Client) ListSecrets(ctx context.Context, nameFilter string) ([]Secret, error) {
	input := &secretsmanager.ListSecretsInput{}

	var secrets []Secret
	lf := strings.ToLower(nameFilter)
	paginator := secretsmanager.NewListSecretsPaginator(c.SM, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.SecretList {
			name := derefStrAws(s.Name)
			if lf != "" && !strings.Contains(strings.ToLower(name), lf) {
				continue
			}
			sec := Secret{
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
func (c *Client) GetSecretValueByName(ctx context.Context, secretName string) (*SecretValue, error) {
	out, err := c.SM.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return nil, err
	}
	value := "(binary secret)"
	if out.SecretString != nil {
		value = *out.SecretString
	}
	return &SecretValue{
		Name:  derefStrAws(out.Name),
		Value: value,
	}, nil
}

// PutSecretValue updates a secret's value.
func (c *Client) PutSecretValue(ctx context.Context, secretName, value string) error {
	_, err := c.SM.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     &secretName,
		SecretString: &value,
	})
	return err
}
