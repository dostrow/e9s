package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/dostrow/e9s/internal/model"
)

func (c *Client) BuildRDSAuthToken(ctx context.Context, endpoint, user string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	user = strings.TrimSpace(user)
	if endpoint == "" || user == "" {
		return "", fmt.Errorf("RDS IAM authentication requires an endpoint and user")
	}
	token, err := auth.BuildAuthToken(ctx, endpoint, c.region, user, c.cfg.Credentials)
	if err != nil {
		return "", fmt.Errorf("build RDS IAM authentication token: %w", err)
	}
	return token, nil
}

func (c *Client) ResolveSQLSecret(ctx context.Context, secretARN string) (model.SQLCredentials, error) {
	if c.SM == nil {
		return model.SQLCredentials{}, fmt.Errorf("secrets manager client is unavailable")
	}
	output, err := c.SM.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: awssdk.String(strings.TrimSpace(secretARN))})
	if err != nil {
		return model.SQLCredentials{}, err
	}
	value := awssdk.ToString(output.SecretString)
	if value == "" && len(output.SecretBinary) > 0 {
		value = string(output.SecretBinary)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return model.SQLCredentials{}, fmt.Errorf("database secret must be a JSON object: %w", err)
	}
	credentials := model.SQLCredentials{
		Username: stringField(fields, "username", "user"), Password: stringField(fields, "password"),
		Database: stringField(fields, "dbname", "database"), Host: stringField(fields, "host"), Port: intField(fields, "port"),
	}
	if credentials.Username == "" || credentials.Password == "" {
		return model.SQLCredentials{}, fmt.Errorf("database secret %q does not contain username and password fields", secretARN)
	}
	return credentials, nil
}

func stringField(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	return ""
}

func intField(fields map[string]any, key string) int {
	value, ok := fields[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}
