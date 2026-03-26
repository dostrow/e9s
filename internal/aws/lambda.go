package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// DownloadLambdaCode downloads the function's deployment package and extracts
// it to a temporary directory. Returns the directory path. Caller must clean up.
func (c *Client) DownloadLambdaCode(ctx context.Context, functionName string) (string, error) {
	out, err := c.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return "", fmt.Errorf("get function: %w", err)
	}
	if out.Code == nil || out.Code.Location == nil {
		return "", fmt.Errorf("no downloadable code (may be a container image)")
	}

	// Download the ZIP
	resp, err := http.Get(*out.Code.Location)
	if err != nil {
		return "", fmt.Errorf("download code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read zip: %w", err)
	}

	// Extract to temp dir
	dir, err := os.MkdirTemp("", "e9s-lambda-*")
	if err != nil {
		return "", err
	}

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("invalid zip: %w", err)
	}

	for _, f := range zr.File {
		target := filepath.Join(dir, f.Name)

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}

	return dir, nil
}

// ZipDirectory creates a ZIP archive from all files in a directory.
func ZipDirectory(dir string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UpdateLambdaCode uploads new code to a Lambda function.
func (c *Client) UpdateLambdaCode(ctx context.Context, functionName string, zipData []byte) error {
	_, err := c.Lambda.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: &functionName,
		ZipFile:      zipData,
	})
	return err
}

// LambdaPackageType returns "Zip" or "Image" for the function's deployment type.
func (c *Client) LambdaPackageType(ctx context.Context, functionName string) (string, error) {
	out, err := c.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return "", err
	}
	if out.Configuration != nil {
		return string(out.Configuration.PackageType), nil
	}
	return "Zip", nil
}

func containsLower(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
