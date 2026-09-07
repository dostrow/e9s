// Package runbook loads explicitly trusted external-action plugins and builds
// process invocations without passing operator input through a shell.
package runbook

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const APIVersion = "e9s/v1alpha1"

type Manifest struct {
	APIVersion  string   `yaml:"api_version"`
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name"`
	Description string   `yaml:"description,omitempty"`
	Actions     []Action `yaml:"actions"`
}

type Action struct {
	ID          string  `yaml:"id"`
	Title       string  `yaml:"title"`
	Description string  `yaml:"description,omitempty"`
	Risk        string  `yaml:"risk,omitempty"`
	Run         RunSpec `yaml:"run"`
	Inputs      []Input `yaml:"inputs,omitempty"`
}

type RunSpec struct {
	Command          string            `yaml:"command"`
	Arguments        []string          `yaml:"arguments,omitempty"`
	LaunchDirectory  string            `yaml:"launch_directory,omitempty"`
	WorkingDirectory string            `yaml:"working_directory,omitempty"`
	Environment      map[string]string `yaml:"environment,omitempty"`
	Terminal         bool              `yaml:"terminal,omitempty"`
	LoginShell       bool              `yaml:"login_shell,omitempty"`
}

type Input struct {
	ID          string   `yaml:"id"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description,omitempty"`
	Type        string   `yaml:"type"`
	Argument    string   `yaml:"argument,omitempty"`
	Default     string   `yaml:"default,omitempty"`
	Placeholder string   `yaml:"placeholder,omitempty"`
	Required    bool     `yaml:"required,omitempty"`
	Pattern     string   `yaml:"pattern,omitempty"`
	Options     []Option `yaml:"options,omitempty"`
	Conflicts   []string `yaml:"conflicts_with,omitempty"`
}

type Option struct {
	Label string `yaml:"label"`
	Value string `yaml:"value"`
}

// ConfiguredAction retains the trusted manifest location needed to resolve
// relative working directories and commands.
type ConfiguredAction struct {
	PluginName        string
	PluginDisplayName string
	ManifestPath      string
	Action            Action
}

func (a ConfiguredAction) Key() string { return a.PluginName + "/" + a.Action.ID }

type Context struct {
	Region  string
	Profile string
}

func InitialValues(action ConfiguredAction, context Context) map[string]string {
	values := make(map[string]string, len(action.Action.Inputs))
	for _, input := range action.Action.Inputs {
		value := expand(input.Default, context)
		if input.Type == "boolean" && value == "" {
			value = "false"
		}
		values[input.ID] = value
	}
	return values
}

type Invocation struct {
	Executable       string
	Args             []string
	LaunchDirectory  string
	WorkingDirectory string
	Environment      map[string]string
	Terminal         bool
	LoginShell       bool
	Title            string
	Risk             string
}

type Source struct {
	Name string
	Path string
}

// Load reads only the supplied paths. Directories resolve to plugin.yaml.
// Relative paths are resolved against the e9s configuration directory.
func Load(paths []string, configDirectory string) ([]ConfiguredAction, error) {
	sources := make([]Source, len(paths))
	for index, path := range paths {
		sources[index] = Source{Path: path}
	}
	return LoadSources(sources, configDirectory)
}

// LoadSources reads explicitly configured plugin sources and applies optional
// local friendly-name overrides without modifying the shared manifest.
func LoadSources(sources []Source, configDirectory string) ([]ConfiguredAction, error) {
	var actions []ConfiguredAction
	seenPlugins := make(map[string]string)
	seenActions := make(map[string]string)
	for _, source := range sources {
		configuredPath := source.Path
		path, err := resolveConfiguredPath(configuredPath, configDirectory)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("load runbook plugin %q: %w", configuredPath, err)
		}
		if info.IsDir() {
			path = filepath.Join(path, "plugin.yaml")
		}
		manifest, err := loadManifest(path)
		if err != nil {
			return nil, err
		}
		if friendlyName := strings.TrimSpace(source.Name); friendlyName != "" {
			manifest.DisplayName = friendlyName
		}
		if previous, exists := seenPlugins[manifest.Name]; exists {
			return nil, fmt.Errorf("runbook plugin name %q is duplicated in %s and %s", manifest.Name, previous, path)
		}
		seenPlugins[manifest.Name] = path
		for _, action := range manifest.Actions {
			key := manifest.Name + "/" + action.ID
			if previous, exists := seenActions[key]; exists {
				return nil, fmt.Errorf("runbook action %q is duplicated in %s and %s", key, previous, path)
			}
			seenActions[key] = path
			actions = append(actions, ConfiguredAction{
				PluginName: manifest.Name, PluginDisplayName: manifest.DisplayName,
				ManifestPath: path, Action: action,
			})
		}
	}
	sort.SliceStable(actions, func(i, j int) bool {
		left := strings.ToLower(actions[i].PluginDisplayName + "\x00" + actions[i].PluginName + "\x00" + actions[i].Action.Title)
		right := strings.ToLower(actions[j].PluginDisplayName + "\x00" + actions[j].PluginName + "\x00" + actions[j].Action.Title)
		return left < right
	})
	return actions, nil
}

func loadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read runbook plugin %s: %w", path, err)
	}
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse runbook plugin %s: %w", path, err)
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, fmt.Errorf("validate runbook plugin %s: %w", path, err)
	}
	if manifest.DisplayName == "" {
		manifest.DisplayName = manifest.Name
	}
	for index := range manifest.Actions {
		if manifest.Actions[index].Risk == "" {
			manifest.Actions[index].Risk = "normal"
		}
	}
	return manifest, nil
}

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func validateManifest(manifest Manifest) error {
	if manifest.APIVersion != APIVersion {
		return fmt.Errorf("api_version must be %q", APIVersion)
	}
	if !identifierPattern.MatchString(manifest.Name) {
		return fmt.Errorf("name %q must contain only lowercase letters, digits, dots, underscores, or hyphens", manifest.Name)
	}
	if len(manifest.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	seenActions := make(map[string]struct{})
	for actionIndex, action := range manifest.Actions {
		prefix := fmt.Sprintf("actions[%d]", actionIndex)
		if !identifierPattern.MatchString(action.ID) {
			return fmt.Errorf("%s.id %q is invalid", prefix, action.ID)
		}
		if _, exists := seenActions[action.ID]; exists {
			return fmt.Errorf("%s.id %q is duplicated", prefix, action.ID)
		}
		seenActions[action.ID] = struct{}{}
		if strings.TrimSpace(action.Title) == "" {
			return fmt.Errorf("%s.title is required", prefix)
		}
		if strings.TrimSpace(action.Run.Command) == "" {
			return fmt.Errorf("%s.run.command is required", prefix)
		}
		switch action.Risk {
		case "", "normal", "sensitive", "production", "destructive":
		default:
			return fmt.Errorf("%s.risk %q is invalid", prefix, action.Risk)
		}
		seenInputs := make(map[string]struct{})
		for inputIndex, input := range action.Inputs {
			inputPrefix := fmt.Sprintf("%s.inputs[%d]", prefix, inputIndex)
			if !identifierPattern.MatchString(input.ID) {
				return fmt.Errorf("%s.id %q is invalid", inputPrefix, input.ID)
			}
			if _, exists := seenInputs[input.ID]; exists {
				return fmt.Errorf("%s.id %q is duplicated", inputPrefix, input.ID)
			}
			seenInputs[input.ID] = struct{}{}
			if strings.TrimSpace(input.Label) == "" {
				return fmt.Errorf("%s.label is required", inputPrefix)
			}
			switch input.Type {
			case "text", "boolean":
			case "select":
				if len(input.Options) == 0 {
					return fmt.Errorf("%s.options must not be empty for a select input", inputPrefix)
				}
			default:
				return fmt.Errorf("%s.type %q must be text, boolean, or select", inputPrefix, input.Type)
			}
			if input.Pattern != "" {
				if _, err := regexp.Compile(input.Pattern); err != nil {
					return fmt.Errorf("%s.pattern: %w", inputPrefix, err)
				}
			}
		}
		for _, input := range action.Inputs {
			for _, conflict := range input.Conflicts {
				if _, exists := seenInputs[conflict]; !exists {
					return fmt.Errorf("%s input %q conflicts with unknown input %q", prefix, input.ID, conflict)
				}
			}
		}
	}
	return nil
}

func resolveConfiguredPath(value, configDirectory string) (string, error) {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value == "" {
		return "", fmt.Errorf("runbook plugin path cannot be empty")
	}
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve runbook plugin path %q: %w", value, err)
		}
		value = filepath.Join(home, strings.TrimLeft(value[1:], `/\`))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(configDirectory, value)
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve runbook plugin path %q: %w", value, err)
	}
	return filepath.Clean(abs), nil
}

// BuildInvocation validates input values and produces an argv-based launch.
func BuildInvocation(action ConfiguredAction, values map[string]string, context Context) (Invocation, error) {
	resolvedValues := make(map[string]string, len(action.Action.Inputs))
	for _, input := range action.Action.Inputs {
		value, provided := values[input.ID]
		if !provided {
			value = expand(input.Default, context)
		}
		if input.Type != "boolean" {
			value = strings.TrimSpace(value)
		}
		if input.Required && value == "" {
			return Invocation{}, fmt.Errorf("%s is required", input.Label)
		}
		switch input.Type {
		case "boolean":
			parsed, err := strconv.ParseBool(defaultString(value, "false"))
			if err != nil {
				return Invocation{}, fmt.Errorf("%s must be true or false", input.Label)
			}
			value = strconv.FormatBool(parsed)
		case "select":
			if value != "" && !hasOption(input.Options, value) {
				return Invocation{}, fmt.Errorf("%s has invalid value %q", input.Label, value)
			}
		}
		if input.Pattern != "" && value != "" && !regexp.MustCompile(input.Pattern).MatchString(value) {
			return Invocation{}, fmt.Errorf("%s has an invalid value", input.Label)
		}
		resolvedValues[input.ID] = value
	}
	for _, input := range action.Action.Inputs {
		if !isSet(input, resolvedValues[input.ID]) {
			continue
		}
		for _, conflict := range input.Conflicts {
			other, _ := inputByID(action.Action.Inputs, conflict)
			if isSet(other, resolvedValues[conflict]) {
				return Invocation{}, fmt.Errorf("%s cannot be combined with %s", input.Label, other.Label)
			}
		}
	}

	args := make([]string, 0, len(action.Action.Run.Arguments)+len(action.Action.Inputs)*2)
	for _, argument := range action.Action.Run.Arguments {
		args = append(args, expand(argument, context))
	}
	for _, input := range action.Action.Inputs {
		value := resolvedValues[input.ID]
		if !isSet(input, value) {
			continue
		}
		if input.Argument != "" {
			args = append(args, input.Argument)
		}
		if input.Type != "boolean" {
			args = append(args, value)
		}
	}

	manifestDirectory := filepath.Dir(action.ManifestPath)
	launchDirectory, err := resolveDirectory(expand(action.Action.Run.LaunchDirectory, context), manifestDirectory)
	if err != nil {
		return Invocation{}, fmt.Errorf("launch directory: %w", err)
	}
	workingBase := manifestDirectory
	if action.Action.Run.LaunchDirectory != "" {
		workingBase = launchDirectory
	}
	workingDirectory, err := resolveDirectory(expand(action.Action.Run.WorkingDirectory, context), workingBase)
	if err != nil {
		return Invocation{}, fmt.Errorf("working directory: %w", err)
	}
	executable := expand(action.Action.Run.Command, context)
	if !filepath.IsAbs(executable) && strings.ContainsAny(executable, `/\`) {
		executable = filepath.Join(launchDirectory, executable)
	}
	environment := make(map[string]string, len(action.Action.Run.Environment))
	for name, value := range action.Action.Run.Environment {
		if strings.TrimSpace(name) == "" || strings.ContainsRune(name, '=') {
			return Invocation{}, fmt.Errorf("invalid environment variable name %q", name)
		}
		if resolved := expand(value, context); resolved != "" {
			environment[name] = resolved
		}
	}
	return Invocation{
		Executable: executable, Args: args, LaunchDirectory: launchDirectory, WorkingDirectory: workingDirectory,
		Environment: environment, Terminal: action.Action.Run.Terminal, LoginShell: action.Action.Run.LoginShell,
		Title: action.Action.Title, Risk: defaultString(action.Action.Risk, "normal"),
	}, nil
}

func resolveDirectory(value, base string) (string, error) {
	if value == "" {
		value = base
	} else if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	value = filepath.Clean(value)
	info, err := os.Stat(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", value, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: not a directory", value)
	}
	return value, nil
}

func expand(value string, context Context) string {
	replacer := strings.NewReplacer("${e9s.region}", context.Region, "${e9s.profile}", context.Profile)
	return replacer.Replace(value)
}

func hasOption(options []Option, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func inputByID(inputs []Input, id string) (Input, bool) {
	for _, input := range inputs {
		if input.ID == id {
			return input, true
		}
	}
	return Input{}, false
}

func isSet(input Input, value string) bool {
	if input.Type == "boolean" {
		parsed, _ := strconv.ParseBool(value)
		return parsed
	}
	return value != ""
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (invocation Invocation) DisplayCommand() string {
	parts := append([]string{quote(invocation.Executable)}, quoteAll(invocation.Args)...)
	return strings.Join(parts, " ")
}

// ExecutionCommand returns the exact executable and argv used by a runner.
// Login-shell execution uses a fixed trampoline; no invocation value is
// interpolated into shell source.
func (invocation Invocation) ExecutionCommand(configuredShell string) (string, []string, error) {
	if !invocation.LoginShell {
		return invocation.Executable, append([]string(nil), invocation.Args...), nil
	}
	shell := strings.TrimSpace(configuredShell)
	if shell == "" {
		shell = strings.TrimSpace(os.Getenv("SHELL"))
	}
	if shell == "" {
		shell = "sh"
	}
	resolved, err := exec.LookPath(shell)
	if err != nil {
		return "", nil, fmt.Errorf("login shell %q was not found: %w", shell, err)
	}
	args := []string{"-lic", `exec "$@"`, "e9s-plugin", invocation.Executable}
	args = append(args, invocation.Args...)
	return resolved, args, nil
}

func (invocation Invocation) ExecutionMode() string {
	if invocation.LoginShell {
		return "login shell"
	}
	return "direct"
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = quote(value)
	}
	return quoted
}

func quote(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == '@' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
