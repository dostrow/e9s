package service

import (
	"context"
	"fmt"
	"os"

	"github.com/dostrow/e9s/internal/tofu"
)

// Tofu exposes the OpenTofu/Terraform workflow shared by both frontends.
type Tofu struct{}

func NewTofu() *Tofu { return &Tofu{} }

func (s *Tofu) Workspace(ctx context.Context, dir string) (tofu.Workspace, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return tofu.Workspace{}, err
	}
	return runner.Workspace(ctx), nil
}

func (s *Tofu) Variables(ctx context.Context, dir string) (tofu.VariablesDocument, error) {
	document, err := tofu.ReadVariables(ctx, dir)
	if err != nil {
		return tofu.VariablesDocument{}, fmt.Errorf("open workspace variables: %w", err)
	}
	return document, nil
}

func (s *Tofu) SaveVariables(ctx context.Context, document tofu.VariablesDocument, content string) (tofu.VariablesDocument, error) {
	saved, err := tofu.SaveVariables(ctx, document, content)
	if err != nil {
		return tofu.VariablesDocument{}, fmt.Errorf("save workspace variables: %w", err)
	}
	return saved, nil
}

func (s *Tofu) Resources(ctx context.Context, dir string) ([]tofu.Resource, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return nil, err
	}
	resources, err := runner.ResourcesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list state resources: %w", err)
	}
	return resources, nil
}

func (s *Tofu) State(ctx context.Context, dir, address string) (string, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return "", err
	}
	output, err := runner.StateShowContext(ctx, address)
	if err != nil {
		return "", fmt.Errorf("show state for %s: %w", address, err)
	}
	return output, nil
}

func (s *Tofu) Plan(ctx context.Context, dir string) (*tofu.PlanResult, string, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return nil, "", err
	}
	jsonOutput, planFile, err := runner.PlanJSONSavedContext(ctx)
	if err != nil {
		return nil, "", err
	}
	plan, err := tofu.ParsePlan(jsonOutput)
	if err != nil {
		_ = os.Remove(planFile)
		return nil, "", err
	}
	return plan, planFile, nil
}

func (s *Tofu) Init(ctx context.Context, dir string) (string, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return "", err
	}
	output, err := runner.InitContext(ctx)
	if err != nil {
		return "", fmt.Errorf("initialize workspace: %w", err)
	}
	return output, nil
}

func (s *Tofu) Apply(ctx context.Context, dir, planFile string) (string, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return "", err
	}
	output, err := runner.ApplyContext(ctx, planFile)
	if err != nil {
		return "", fmt.Errorf("apply reviewed plan: %w", err)
	}
	return output, nil
}

func (s *Tofu) ApplyCommand(dir, planFile string) (tofu.Command, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return tofu.Command{}, err
	}
	return runner.ApplyCommand(planFile), nil
}

func (s *Tofu) InitCommand(dir string) (tofu.Command, error) {
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		return tofu.Command{}, err
	}
	return runner.InitCommand(), nil
}

func (s *Tofu) CleanupPlan(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
