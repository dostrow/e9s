// Package tofu provides OpenTofu/Terraform command execution and output parsing.
package tofu

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes tofu/terraform commands in a working directory.
type Runner struct {
	Dir    string // working directory
	Binary string // "tofu" or "terraform"
}

// NewRunner creates a runner for the given directory.
// Auto-detects tofu vs terraform binary.
func NewRunner(dir string) (*Runner, error) {
	binary, err := detectBinary()
	if err != nil {
		return nil, err
	}
	return &Runner{Dir: dir, Binary: binary}, nil
}

// IsInitialized checks if the directory has been initialized (.terraform dir exists).
func (r *Runner) IsInitialized() bool {
	cmd := exec.Command(r.Binary, "providers")
	cmd.Dir = r.Dir
	return cmd.Run() == nil
}

// Init runs tofu init.
func (r *Runner) Init() (string, error) {
	return r.run("init", "-no-color")
}

// StateList returns the list of resources in the state.
func (r *Runner) StateList() ([]string, error) {
	out, err := r.run("state", "list")
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

// StateShow returns the detail for a single resource.
func (r *Runner) StateShow(address string) (string, error) {
	return r.run("state", "show", "-no-color", address)
}

// PlanJSON runs tofu plan and returns the JSON output for parsing.
// The generated plan file is removed after the JSON is rendered.
func (r *Runner) PlanJSON() (string, error) {
	jsonOut, planFile, err := r.PlanJSONSaved()
	if err != nil {
		return "", err
	}
	_ = os.Remove(planFile)
	return jsonOut, nil
}

// PlanJSONSaved runs tofu plan, returns the JSON output for parsing,
// and preserves the generated plan file for a later apply.
func (r *Runner) PlanJSONSaved() (string, string, error) {
	planFile, err := os.CreateTemp("", "e9s-tofu-plan-*.tfplan")
	if err != nil {
		return "", "", fmt.Errorf("create plan file: %w", err)
	}
	planPath := planFile.Name()
	if err := planFile.Close(); err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("close plan file: %w", err)
	}

	_, err = r.run("plan", "-no-color", "-out="+planPath)
	if err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("plan failed: %w", err)
	}
	jsonOut, err := r.run("show", "-json", planPath)
	if err != nil {
		_ = os.Remove(planPath)
		return "", "", fmt.Errorf("show plan failed: %w", err)
	}
	return jsonOut, planPath, nil
}

// Validate runs tofu validate and returns the output.
func (r *Runner) Validate() (string, error) {
	return r.run("validate", "-no-color")
}

// Output returns the outputs from the state.
func (r *Runner) Output() (string, error) {
	return r.run("output", "-no-color")
}

func (r *Runner) run(args ...string) (string, error) {
	cmd := exec.Command(r.Binary, args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = stdout.String()
		}
		return "", fmt.Errorf("%s %s: %s", r.Binary, strings.Join(args, " "), strings.TrimSpace(errMsg))
	}
	return stdout.String(), nil
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
