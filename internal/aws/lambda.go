package aws

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type LambdaFunction struct {
	Name         string
	ARN          string
	Runtime      string
	Handler      string
	Description  string
	MemoryMB     int
	TimeoutSec   int
	CodeSize     int64
	State        string // Active, Inactive, Pending, Failed
	LastModified time.Time
	LogGroup     string
	EnvVars      []EnvVar
	RawEnvVars   map[string]string // original key-value pairs
}

// ListLambdaFunctions returns Lambda functions, optionally filtered by name substring.
func (c *Client) ListLambdaFunctions(ctx context.Context, filter string) ([]LambdaFunction, error) {
	var functions []LambdaFunction
	paginator := lambda.NewListFunctionsPaginator(c.Lambda, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, f := range page.Functions {
			fn := transformLambdaFunction(f)
			if filter != "" && !containsLower(fn.Name, filter) && !containsLower(fn.Description, filter) {
				continue
			}
			functions = append(functions, fn)
		}
	}
	return functions, nil
}

// GetLambdaFunction returns detailed info for a single Lambda function.
func (c *Client) GetLambdaFunction(ctx context.Context, name string) (*LambdaFunction, error) {
	out, err := c.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	})
	if err != nil {
		return nil, err
	}
	fn := transformLambdaConfig(*out.Configuration)
	return &fn, nil
}

func transformLambdaFunction(f lambdaTypes.FunctionConfiguration) LambdaFunction {
	return transformLambdaConfig(f)
}

func transformLambdaConfig(f lambdaTypes.FunctionConfiguration) LambdaFunction {
	fn := LambdaFunction{
		Name:     derefStrAws(f.FunctionName),
		ARN:      derefStrAws(f.FunctionArn),
		Runtime:  string(f.Runtime),
		Handler:  derefStrAws(f.Handler),
		Description: derefStrAws(f.Description),
		CodeSize: f.CodeSize,
		State:    string(f.State),
	}
	if f.MemorySize != nil {
		fn.MemoryMB = int(*f.MemorySize)
	}
	if f.Timeout != nil {
		fn.TimeoutSec = int(*f.Timeout)
	}
	if f.LastModified != nil {
		t, err := time.Parse("2006-01-02T15:04:05.000+0000", *f.LastModified)
		if err == nil {
			fn.LastModified = t
		}
	}
	if f.LoggingConfig != nil && f.LoggingConfig.LogGroup != nil {
		fn.LogGroup = *f.LoggingConfig.LogGroup
	} else {
		fn.LogGroup = "/aws/lambda/" + fn.Name
	}
	if f.Environment != nil && f.Environment.Variables != nil {
		fn.RawEnvVars = f.Environment.Variables
		for k, v := range f.Environment.Variables {
			ev := EnvVar{Name: k, Value: v}
			if strings.Contains(v, "arn:aws:secretsmanager:") {
				ev.Source = "secrets-manager"
			} else if strings.Contains(v, "arn:aws:ssm:") || strings.HasPrefix(v, "/") && strings.Count(v, "/") >= 2 {
				// Heuristic: SSM parameters often look like /path/to/param
				// but only flag ARNs definitively
				if strings.Contains(v, "arn:aws:ssm:") {
					ev.Source = "ssm"
				}
			}
			fn.EnvVars = append(fn.EnvVars, ev)
		}
		// Sort by name for consistent display
		sortEnvVars(fn.EnvVars)
	}
	return fn
}

func sortEnvVars(vars []EnvVar) {
	slices.SortFunc(vars, func(a, b EnvVar) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
}

func containsLower(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
