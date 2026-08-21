// Package tofu provides OpenTofu/Terraform command execution and output parsing.
package tofu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	// VariablesFilename is the conventional workspace variable file edited by e9s.
	VariablesFilename = "terraform.tfvars"
	maxVariablesBytes = 4 << 20
	missingRevision   = "missing"
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

// VariablesDocument is a snapshot of a workspace's terraform.tfvars file.
// Revision is used to prevent overwriting changes made after the file was read.
type VariablesDocument struct {
	Path     string
	Content  string
	Revision string
	Exists   bool
}

// ReadVariables reads terraform.tfvars from a validated workspace. A missing
// file is represented as an empty document so the editor can create it.
func ReadVariables(ctx context.Context, dir string) (VariablesDocument, error) {
	workspace, err := resolveWorkspaceDir(dir)
	if err != nil {
		return VariablesDocument{}, err
	}
	if err := ctx.Err(); err != nil {
		return VariablesDocument{}, err
	}
	path := filepath.Join(workspace, VariablesFilename)
	data, exists, _, err := readVariablesFile(path)
	if err != nil {
		return VariablesDocument{}, err
	}
	if err := ctx.Err(); err != nil {
		return VariablesDocument{}, err
	}
	return variablesDocument(path, data, exists), nil
}

// SaveVariables atomically writes terraform.tfvars when the on-disk revision
// still matches the snapshot that was opened in the editor.
func SaveVariables(ctx context.Context, original VariablesDocument, content string) (VariablesDocument, error) {
	if strings.TrimSpace(original.Path) == "" || filepath.Base(original.Path) != VariablesFilename {
		return VariablesDocument{}, fmt.Errorf("save %s: invalid document path", VariablesFilename)
	}
	workspace, err := resolveWorkspaceDir(filepath.Dir(original.Path))
	if err != nil {
		return VariablesDocument{}, err
	}
	path := filepath.Join(workspace, VariablesFilename)
	if filepath.Clean(path) != filepath.Clean(original.Path) {
		return VariablesDocument{}, fmt.Errorf("save %s: workspace path changed since it was opened", VariablesFilename)
	}
	if len(content) > maxVariablesBytes {
		return VariablesDocument{}, fmt.Errorf("save %s: file exceeds the %d MiB editor limit", VariablesFilename, maxVariablesBytes>>20)
	}
	if !utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
		return VariablesDocument{}, fmt.Errorf("save %s: content must be UTF-8 text without NUL bytes", VariablesFilename)
	}
	if err := ctx.Err(); err != nil {
		return VariablesDocument{}, err
	}
	current, exists, mode, err := readVariablesFile(path)
	if err != nil {
		return VariablesDocument{}, err
	}
	if variablesRevision(current, exists) != original.Revision {
		return VariablesDocument{}, fmt.Errorf("save %s: file changed on disk since it was opened; reopen it before saving", VariablesFilename)
	}
	if !exists {
		mode = 0o600
	}
	if err := atomicWriteVariables(ctx, path, []byte(content), mode); err != nil {
		return VariablesDocument{}, err
	}
	return variablesDocument(path, []byte(content), true), nil
}

func resolveWorkspaceDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("workspace path is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("open workspace %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", abs)
	}
	return abs, nil
}

func readVariablesFile(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, 0, nil
	}
	if err != nil {
		return nil, false, 0, fmt.Errorf("inspect %s: %w", VariablesFilename, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, 0, fmt.Errorf("open %s: symbolic links are not supported", VariablesFilename)
	}
	if !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("open %s: path is not a regular file", VariablesFilename)
	}
	if info.Size() > maxVariablesBytes {
		return nil, false, 0, fmt.Errorf("open %s: file exceeds the %d MiB editor limit", VariablesFilename, maxVariablesBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", VariablesFilename, err)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, false, 0, fmt.Errorf("open %s: file must be UTF-8 text without NUL bytes", VariablesFilename)
	}
	return data, true, info.Mode().Perm(), nil
}

func variablesDocument(path string, content []byte, exists bool) VariablesDocument {
	return VariablesDocument{
		Path:     path,
		Content:  string(content),
		Revision: variablesRevision(content, exists),
		Exists:   exists,
	}
}

func variablesRevision(content []byte, exists bool) string {
	if !exists {
		return missingRevision
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}

func atomicWriteVariables(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".terraform.tfvars.e9s-*")
	if err != nil {
		return fmt.Errorf("create temporary %s: %w", VariablesFilename, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary %s permissions: %w", VariablesFilename, err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary %s: %w", VariablesFilename, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary %s: %w", VariablesFilename, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary %s: %w", VariablesFilename, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", VariablesFilename, err)
	}
	return nil
}

// NewRunner creates a runner for the given directory.
// Auto-detects tofu vs terraform binary.
func NewRunner(dir string) (*Runner, error) {
	abs, err := resolveWorkspaceDir(dir)
	if err != nil {
		return nil, err
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
