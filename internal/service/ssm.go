package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// SSMAPI is the low-level Parameter Store behavior used by shared workflows.
type SSMAPI interface {
	ListParameters(context.Context, string) ([]model.Parameter, error)
	GetParameter(context.Context, string) (*model.Parameter, error)
	PutParameter(context.Context, string, string) error
}

// SSM exposes Parameter Store workflows without frontend dependencies.
type SSM struct {
	api SSMAPI
}

func NewSSM(api SSMAPI) *SSM {
	return &SSM{api: api}
}

func NormalizeParameterPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/", nil
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("SSM parameter path must begin with /")
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path, nil
}

func (s *SSM) List(ctx context.Context, path string) ([]model.Parameter, error) {
	normalized, err := NormalizeParameterPath(path)
	if err != nil {
		return nil, fmt.Errorf("list SSM parameters: %w", err)
	}
	params, err := s.api.ListParameters(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("list SSM parameters under %q: %w", normalized, err)
	}
	sort.SliceStable(params, func(i, j int) bool {
		return strings.ToLower(params[i].Name) < strings.ToLower(params[j].Name)
	})
	return params, nil
}

func (s *SSM) Detail(ctx context.Context, name string) (*model.Parameter, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("read SSM parameter: name is required")
	}
	parameter, err := s.api.GetParameter(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("read SSM parameter %q: %w", name, err)
	}
	if parameter == nil {
		return nil, fmt.Errorf("read SSM parameter %q: parameter was not found", name)
	}
	return parameter, nil
}

func (s *SSM) Update(ctx context.Context, name, value string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("update SSM parameter: name is required")
	}
	if err := s.api.PutParameter(ctx, name, value); err != nil {
		return fmt.Errorf("update SSM parameter %q: %w", name, err)
	}
	return nil
}
