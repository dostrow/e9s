// Package tofu provides OpenTofu/Terraform command execution and output parsing.
package tofu

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes tofu/terraform commands in a working directory.
type Runner struct {
	Dir    string // working directory
	Binary string // "tofu" or "terraform"
}

// Workspace describes a validated OpenTofu/Terraform working directory.
type Workspace struct {
	Dir         string
	Binary      string
	Initialized bool
}

// Resource is a parsed resource address from state.
type Resource struct {
	Address string
	Type    string
	Name    string
	Module  string
}

// Command describes a command that a frontend may run interactively.
type Command struct {
	Executable string
	Args       []string
}

// NewRunner creates a runner for the given directory.
// Auto-detects tofu vs terraform binary.
func NewRunner(dir string) (*Runner, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open workspace %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace %q is not a directory", abs)
	}
	binary, err := detectBinary()
	if err != nil {
		return nil, err
	}
	return &Runner{Dir: abs, Binary: binary}, nil
}

// IsInitialized checks if the directory has been initialized (.terraform dir exists).
func (r *Runner) IsInitialized() bool {
	return r.IsInitializedContext(context.Background())
}

func (r *Runner) IsInitializedContext(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, r.Binary, "providers")
	cmd.Dir = r.Dir
	return cmd.Run() == nil
}

func (r *Runner) Workspace(ctx context.Context) Workspace {
	return Workspace{Dir: r.Dir, Binary: r.Binary, Initialized: r.IsInitializedContext(ctx)}
}

// Init runs tofu init.
func (r *Runner) Init() (string, error) {
	return r.InitContext(context.Background())
}

func (r *Runner) InitContext(ctx context.Context) (string, error) {
	return r.runContext(ctx, "init", "-no-color", "-input=false")
}

// StateList returns the list of resources in the state.
func (r *Runner) StateList() ([]string, error) {
	return r.StateListContext(context.Background())
}

func (r *Runner) StateListContext(ctx context.Context) ([]string, error) {
	out, err := r.runContext(ctx, "state", "list")
	if err != nil {
		return nil, err
	}
	var resources []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			resources = append(resources, line)
		}
	}
	return resources, nil
}

func (r *Runner) ResourcesContext(ctx context.Context) ([]Resource, error) {
	addresses, err := r.StateListContext(ctx)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(addresses))
	for _, address := range addresses {
		resources = append(resources, ParseResourceAddress(address))
	}
	return resources, nil
}

// StateShow returns the detail for a single resource.
func (r *Runner) StateShow(address string) (string, error) {
	return r.StateShowContext(context.Background(), address)

}

func (r *Runner) StateShowContext(ctx context.Context, address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("resource address is required")
	}
	return r.runContext(ctx, "state", "show", "-no-color", address)
}

// PlanJSON runs tofu plan and returns the JSON output for parsing.
// The generated plan file is removed after the JSON is rendered.
func (r *Runner) PlanJSON() (string, error) {
	jsonOut, planFile, err := r.PlanJSONSavedContext(context.Background())
	if err != nil {
		return "", err
	}
	_ = os.Remove(planFile)
	return jsonOut, nil
}

// PlanJSONSaved runs tofu plan, returns the JSON output for parsing,
// and preserves the generated plan file for a later apply.
func (r *Runner) PlanJSONSaved() (string, string, error) {
	return r.PlanJSONSavedContext(context.Background())
}

func (r *Runner) PlanJSONSavedContext(ctx context.Context) (string, string, error) {
	planFile, err := os.CreateTemp("", "e9s-tofu-plan-*.tfplan")
	if err != nil {
		return "", "", fmt.Errorf("create plan file: %w", err)
	}
	planPath := planFile.Name()
	if err := planFile.Close(); err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("close plan file: %w", err)
	}

	_, err = r.runContext(ctx, "plan", "-no-color", "-input=false", "-out="+planPath)
	if err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("plan failed: %w", err)
	}
	jsonOut, err := r.runContext(ctx, "show", "-json", planPath)
	if err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("show plan failed: %w", err)
	}
	return jsonOut, planPath, nil
}

// Validate runs tofu validate and returns the output.
func (r *Runner) Validate() (string, error) {
	return r.runContext(context.Background(), "validate", "-no-color")
}

// Output returns the outputs from the state.
func (r *Runner) Output() (string, error) {
	return r.runContext(context.Background(), "output", "-no-color")
}

// ApplyCommand returns an interactive command. A non-empty plan file applies
// exactly the reviewed plan and therefore does not require another prompt.
func (r *Runner) ApplyCommand(planFile string) Command {
	args := []string{"-chdir=" + r.Dir, "apply", "-no-color"}
	if planFile != "" {
		args = append(args, planFile)
	}
	return Command{Executable: r.Binary, Args: args}
}

func (r *Runner) InitCommand() Command {
	return Command{Executable: r.Binary, Args: []string{"-chdir=" + r.Dir, "init", "-no-color"}}
}

func (r *Runner) ApplyContext(ctx context.Context, planFile string) (string, error) {
	if strings.TrimSpace(planFile) == "" {
		return "", fmt.Errorf("a reviewed plan file is required for non-interactive apply")
	}
	return r.runContext(ctx, "apply", "-no-color", "-input=false", planFile)
}

func (r *Runner) run(args ...string) (string, error) {
	return r.runContext(context.Background(), args...)
}

func (r *Runner) runContext(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("%s %s: %w", r.Binary, strings.Join(args, " "), ctxErr)
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = stdout.String()
		}
		return "", fmt.Errorf("%s %s: %s", r.Binary, strings.Join(args, " "), strings.TrimSpace(errMsg))
	}
	return stdout.String(), nil
}

// ParseResourceAddress separates an address into its module, type, and name.
func ParseResourceAddress(address string) Resource {
	resource := Resource{Address: address}
	parts := strings.Split(address, ".")
	index := 0
	var modules []string
	for index+1 < len(parts) && parts[index] == "module" {
		modules = append(modules, "module."+parts[index+1])
		index += 2
	}
	resource.Module = strings.Join(modules, ".")
	if index < len(parts) {
		resource.Type = parts[index]
	}
	if index+1 < len(parts) {
		resource.Name = strings.Join(parts[index+1:], ".")
	}
	if resource.Type == "" {
		resource.Type = address
	}
	return resource
}

func detectBinary() (string, error) {
	if path, err := exec.LookPath("tofu"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("terraform"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("neither 'tofu' nor 'terraform' found in PATH")
}
