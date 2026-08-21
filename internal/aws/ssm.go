package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/dostrow/e9s/internal/model"
)

// ListParameters fetches SSM parameters matching a path prefix.
func (c *Client) ListParameters(ctx context.Context, pathPrefix string) ([]model.Parameter, error) {
	var params []model.Parameter
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
			params = append(params, model.Parameter{
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
func (c *Client) GetParameter(ctx context.Context, name string) (*model.Parameter, error) {
	out, err := c.SSM.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	p := out.Parameter
	return &model.Parameter{
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
