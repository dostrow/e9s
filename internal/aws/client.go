package aws

import (
	"context"

	awscfg "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Client struct {
	ECS    *ecs.Client
	Logs   *cloudwatchlogs.Client
	CW     *cloudwatch.Client
	SSM    *ssm.Client
	SM     *secretsmanager.Client
	S3     *s3.Client
	Lambda *lambda.Client
	cfg    awscfg.Config
	region string
}

func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		ECS:    ecs.NewFromConfig(cfg),
		Logs:   cloudwatchlogs.NewFromConfig(cfg),
		CW:     cloudwatch.NewFromConfig(cfg),
		SSM:    ssm.NewFromConfig(cfg),
		SM:     secretsmanager.NewFromConfig(cfg),
		S3:     s3.NewFromConfig(cfg),
		Lambda: lambda.NewFromConfig(cfg),
		cfg:    cfg,
		region: cfg.Region,
	}, nil
}

func (c *Client) Region() string {
	return c.region
}

// SwitchRegion creates new service clients for a different region.
func (c *Client) SwitchRegion(ctx context.Context, region string) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return err
	}

	c.ECS = ecs.NewFromConfig(cfg)
	c.Logs = cloudwatchlogs.NewFromConfig(cfg)
	c.CW = cloudwatch.NewFromConfig(cfg)
	c.SSM = ssm.NewFromConfig(cfg)
	c.SM = secretsmanager.NewFromConfig(cfg)
	c.S3 = s3.NewFromConfig(cfg)
	c.Lambda = lambda.NewFromConfig(cfg)
	c.cfg = cfg
	c.region = region
	return nil
}
