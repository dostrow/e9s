// Package aws provides AWS SDK client wrappers for ECS, CloudWatch, SSM, Secrets Manager, S3, and Lambda.
package aws

import (
	"context"

	awscfg "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go/middleware"
)

type Client struct {
	ECS            *ecs.Client
	AppAutoScaling *applicationautoscaling.Client
	APIGateway     *apigateway.Client
	APIGatewayV2   *apigatewayv2.Client
	Logs           *cloudwatchlogs.Client
	CW             *cloudwatch.Client
	SSM            *ssm.Client
	SM             *secretsmanager.Client
	S3             *s3.Client
	Lambda         *lambda.Client
	DynamoDB       *dynamodb.Client
	SQS            *sqs.Client
	CodeBuild      *codebuild.Client
	CostExplorer   *costexplorer.Client
	EC2            *ec2.Client
	ELBV2          *elasticloadbalancingv2.Client
	ECR            *ecr.Client
	ElastiCache    *elasticache.Client
	RDS            *rds.Client
	RDSData        *rdsdata.Client
	Route53        *route53.Client
	cfg            awscfg.Config
	region         string
	requests       *requestObserver
}

func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	requests := newRequestObserver()
	opts = append(opts, config.WithAPIOptions([]func(*middleware.Stack) error{requests.middleware}))
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	ceConfig := cfg
	ceConfig.Region = "us-east-1"
	return &Client{
		ECS:            ecs.NewFromConfig(cfg),
		AppAutoScaling: applicationautoscaling.NewFromConfig(cfg),
		APIGateway:     apigateway.NewFromConfig(cfg),
		APIGatewayV2:   apigatewayv2.NewFromConfig(cfg),
		Logs:           cloudwatchlogs.NewFromConfig(cfg),
		CW:             cloudwatch.NewFromConfig(cfg),
		SSM:            ssm.NewFromConfig(cfg),
		SM:             secretsmanager.NewFromConfig(cfg),
		S3:             s3.NewFromConfig(cfg),
		Lambda:         lambda.NewFromConfig(cfg),
		DynamoDB:       dynamodb.NewFromConfig(cfg),
		SQS:            sqs.NewFromConfig(cfg),
		CodeBuild:      codebuild.NewFromConfig(cfg),
		CostExplorer:   costexplorer.NewFromConfig(ceConfig),
		EC2:            ec2.NewFromConfig(cfg),
		ELBV2:          elasticloadbalancingv2.NewFromConfig(cfg),
		ECR:            ecr.NewFromConfig(cfg),
		ElastiCache:    elasticache.NewFromConfig(cfg),
		RDS:            rds.NewFromConfig(cfg),
		RDSData:        rdsdata.NewFromConfig(cfg),
		Route53:        route53.NewFromConfig(cfg),
		cfg:            cfg,
		region:         cfg.Region,
		requests:       requests,
	}, nil
}

func (c *Client) Region() string {
	return c.region
}

func (c *Client) RequestSnapshot() RequestSnapshot {
	if c == nil || c.requests == nil {
		return RequestSnapshot{}
	}
	return c.requests.snapshot()
}

// SwitchRegion creates new service clients for a different region.
func (c *Client) SwitchRegion(ctx context.Context, region string) error {
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if c.requests != nil {
		opts = append(opts, config.WithAPIOptions([]func(*middleware.Stack) error{c.requests.middleware}))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return err
	}

	c.ECS = ecs.NewFromConfig(cfg)
	c.AppAutoScaling = applicationautoscaling.NewFromConfig(cfg)
	c.APIGateway = apigateway.NewFromConfig(cfg)
	c.APIGatewayV2 = apigatewayv2.NewFromConfig(cfg)
	c.Logs = cloudwatchlogs.NewFromConfig(cfg)
	c.CW = cloudwatch.NewFromConfig(cfg)
	c.SSM = ssm.NewFromConfig(cfg)
	c.SM = secretsmanager.NewFromConfig(cfg)
	c.S3 = s3.NewFromConfig(cfg)
	c.Lambda = lambda.NewFromConfig(cfg)
	c.DynamoDB = dynamodb.NewFromConfig(cfg)
	c.SQS = sqs.NewFromConfig(cfg)
	c.CodeBuild = codebuild.NewFromConfig(cfg)
	ceConfig := cfg
	ceConfig.Region = "us-east-1"
	c.CostExplorer = costexplorer.NewFromConfig(ceConfig)
	c.EC2 = ec2.NewFromConfig(cfg)
	c.ELBV2 = elasticloadbalancingv2.NewFromConfig(cfg)
	c.ECR = ecr.NewFromConfig(cfg)
	c.ElastiCache = elasticache.NewFromConfig(cfg)
	c.RDS = rds.NewFromConfig(cfg)
	c.RDSData = rdsdata.NewFromConfig(cfg)
	c.Route53 = route53.NewFromConfig(cfg)
	c.cfg = cfg
	c.region = region
	return nil
}
