package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// LambdaAPI is the low-level Lambda behavior used by shared workflows.
type LambdaAPI interface {
	ListLambdaFunctions(context.Context) ([]model.LambdaFunction, error)
	GetLambdaFunction(context.Context, string) (*model.LambdaFunction, error)
	ResolveEnvVars(context.Context, []model.EnvVar) []model.EnvVar
	LambdaPackageType(context.Context, string) (string, error)
	DownloadLambdaCode(context.Context, string) (string, error)
	UpdateLambdaCode(context.Context, string, []byte) error
}

// Lambda exposes Lambda workflows without frontend dependencies.
type Lambda struct {
	api LambdaAPI
}

func NewLambda(api LambdaAPI) *Lambda {
	return &Lambda{api: api}
}

func (s *Lambda) List(ctx context.Context, filter string) ([]model.LambdaFunction, error) {
	functions, err := s.api.ListLambdaFunctions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Lambda functions: %w", err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" {
		filtered := make([]model.LambdaFunction, 0, len(functions))
		for _, function := range functions {
			if strings.Contains(strings.ToLower(function.Name), filter) ||
				strings.Contains(strings.ToLower(function.Description), filter) {
				filtered = append(filtered, function)
			}
		}
		functions = filtered
	}
	sort.SliceStable(functions, func(i, j int) bool {
		return strings.ToLower(functions[i].Name) < strings.ToLower(functions[j].Name)
	})
	return functions, nil
}

func (s *Lambda) Detail(ctx context.Context, name string) (*model.LambdaFunction, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("read Lambda function: name is required")
	}
	function, err := s.api.GetLambdaFunction(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("read Lambda function %q: %w", name, err)
	}
	if function == nil {
		return nil, fmt.Errorf("read Lambda function %q: function was not found", name)
	}
	return function, nil
}

func (s *Lambda) Environment(ctx context.Context, name string, resolveSecrets bool) ([]model.EnvVar, error) {
	function, err := s.Detail(ctx, name)
	if err != nil {
		return nil, err
	}
	environment := append([]model.EnvVar(nil), function.EnvVars...)
	if resolveSecrets {
		environment = s.api.ResolveEnvVars(ctx, environment)
	}
	sort.SliceStable(environment, func(i, j int) bool { return environment[i].Name < environment[j].Name })
	return environment, nil
}

func (s *Lambda) PackageType(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("read Lambda package type: name is required")
	}
	packageType, err := s.api.LambdaPackageType(ctx, name)
	if err != nil {
		return "", fmt.Errorf("read Lambda package type for %q: %w", name, err)
	}
	return packageType, nil
}

func (s *Lambda) DownloadCode(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("download Lambda code: name is required")
	}
	directory, err := s.api.DownloadLambdaCode(ctx, name)
	if err != nil {
		return "", fmt.Errorf("download Lambda code for %q: %w", name, err)
	}
	return directory, nil
}

func (s *Lambda) UpdateCode(ctx context.Context, name string, archive []byte) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("update Lambda code: name is required")
	}
	if len(archive) == 0 {
		return fmt.Errorf("update Lambda code %q: deployment archive is empty", name)
	}
	if err := s.api.UpdateLambdaCode(ctx, name, archive); err != nil {
		return fmt.Errorf("update Lambda code for %q: %w", name, err)
	}
	return nil
}

// ZipDirectory creates a deployment archive while preserving relative paths.
func ZipDirectory(dir string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relative
		header.Method = zip.Deflate
		destination, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(destination, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
