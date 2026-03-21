package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Parameter struct {
	Name         string
	Value        string
	Type         string // String, StringList, SecureString
	Version      int64
	LastModified time.Time
}

// ListParameters fetches SSM parameters matching a path prefix.
func (c *Client) ListParameters(ctx context.Context, pathPrefix string) ([]Parameter, error) {
	var params []Parameter
	paginator := ssm.NewGetParametersByPathPaginator(c.SSM, &ssm.GetParametersByPathInput{
		Path:           &pathPrefix,
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(false),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.Parameters {
			params = append(params, Parameter{
				Name:         derefStrAws(p.Name),
				Value:        derefStrAws(p.Value),
				Type:         string(p.Type),
				Version:      p.Version,
				LastModified: derefTimeAws(p.LastModifiedDate),
			})
		}
	}
	return params, nil
}

// GetParameter fetches a single parameter with decryption.
func (c *Client) GetParameter(ctx context.Context, name string) (*Parameter, error) {
	out, err := c.SSM.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	p := out.Parameter
	return &Parameter{
		Name:         derefStrAws(p.Name),
		Value:        derefStrAws(p.Value),
		Type:         string(p.Type),
		Version:      p.Version,
		LastModified: derefTimeAws(p.LastModifiedDate),
	}, nil
}

// PutParameter updates an SSM parameter value.
func (c *Client) PutParameter(ctx context.Context, name, value string) error {
	_, err := c.SSM.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Overwrite: aws.Bool(true),
	})
	return err
}
