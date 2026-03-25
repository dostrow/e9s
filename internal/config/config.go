// Package config handles loading and saving the e9s configuration file.
// Uses XDG_CONFIG_HOME (~/.config/e9s/config.yaml) on Linux/macOS,
// %APPDATA%\e9s\config.yaml on Windows, with fallback to ~/.e9s.yaml.
package config

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// configPath is the resolved path to the config file.
var (
	configPath     string
	configPathOnce sync.Once
)

type LogPathEntry struct {
	Name      string   `yaml:"name"`
	LogGroup  string   `yaml:"log_group"`
	LogGroups []string `yaml:"log_groups,omitempty"` // multi-group search
	Stream    string   `yaml:"stream,omitempty"`     // optional — empty means all streams
}

type SQSQueueEntry struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type DynamoTable struct {
	Name  string `yaml:"name"`
	Table string `yaml:"table"`
}

type DynamoQuery struct {
	Name      string `yaml:"name"`
	Statement string `yaml:"statement"`
}

type LambdaSearch struct {
	Name   string `yaml:"name"`
	Filter string `yaml:"filter"`
}

type S3Search struct {
	Name   string `yaml:"name"`
	Filter string `yaml:"filter"`
}

type SMFilter struct {
	Name   string `yaml:"name"`
	Filter string `yaml:"filter"`
}

type SSMPrefix struct {
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
}

type Config struct {
	Defaults struct {
		Cluster         string `yaml:"cluster"`
		Region          string `yaml:"region"`
		Profile         string `yaml:"profile"`
		RefreshInterval int    `yaml:"refresh_interval"`
		DefaultMode     string `yaml:"default_mode"`      // ECS, CW, SSM, SM, S3, Lambda, DynamoDB, or "" for picker
		SaveDirectory   string `yaml:"save_directory"`    // default directory for file save dialogs
	} `yaml:"defaults"`
	Display struct {
		TimestampFormat string `yaml:"timestamp_format"` // "relative" or "absolute"
		MaxEvents       int    `yaml:"max_events"`
		MaxLogLines     int    `yaml:"max_log_lines"`
	} `yaml:"display"`
	Modules struct {
		ECS             *bool `yaml:"ecs"`
		CloudWatch      *bool `yaml:"cloudwatch"`       // legacy: maps to CWLogs
		CWLogs          *bool `yaml:"cloudwatch_logs"`
		CWAlarms        *bool `yaml:"cloudwatch_alarms"`
		SSM             *bool `yaml:"ssm"`
		SM              *bool `yaml:"sm"`
		S3              *bool `yaml:"s3"`
		Lambda          *bool `yaml:"lambda"`
		DynamoDB        *bool `yaml:"dynamodb"`
		SQS             *bool `yaml:"sqs"`
		CodeBuild       *bool `yaml:"codebuild"`
	} `yaml:"modules"`
	ExcludeServices []string `yaml:"exclude_services"`
	SSMPrefixes     []SSMPrefix    `yaml:"ssm_prefixes"`
	SMFilters       []SMFilter     `yaml:"sm_filters"`
	S3Searches      []S3Search     `yaml:"s3_searches"`
	LambdaSearches  []LambdaSearch `yaml:"lambda_searches"`
	DynamoTables    []DynamoTable  `yaml:"dynamo_tables"`
	DynamoQueries   []DynamoQuery  `yaml:"dynamo_queries"`
	SQSQueues       []SQSQueueEntry `yaml:"sqs_queues"`
	LogPaths        []LogPathEntry `yaml:"log_paths"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	c := Config{}
	c.Defaults.RefreshInterval = 5
	c.Display.TimestampFormat = "relative"
	c.Display.MaxEvents = 50
	c.Display.MaxLogLines = 1000
	return c
}

// resolveConfigPath determines the config file path using XDG conventions.
// Priority: existing XDG path > existing legacy path > new XDG path
func resolveConfigPath() string {
	configPathOnce.Do(func() {
		var xdgPath, legacyPath string

		// Determine XDG path
		if xdgDir, err := os.UserConfigDir(); err == nil {
			xdgPath = filepath.Join(xdgDir, "e9s", "config.yaml")
		}

		// Determine legacy path
		if home, err := os.UserHomeDir(); err == nil {
			legacyPath = filepath.Join(home, ".e9s.yaml")
		}

		// 1. If XDG config already exists, use it
		if xdgPath != "" {
			if _, err := os.Stat(xdgPath); err == nil {
				configPath = xdgPath
				return
			}
		}

		// 2. If legacy config exists, migrate to XDG if possible
		if legacyPath != "" {
			if _, err := os.Stat(legacyPath); err == nil {
				if xdgPath != "" {
					// Try to migrate
					dir := filepath.Dir(xdgPath)
					if err := os.MkdirAll(dir, 0755); err == nil {
						if data, err := os.ReadFile(legacyPath); err == nil {
							if err := os.WriteFile(xdgPath, data, 0644); err == nil {
								// Verify the new file exists and matches before removing legacy
								if newData, err := os.ReadFile(xdgPath); err == nil && len(newData) == len(data) {
									_ = os.Remove(legacyPath)
								}
								configPath = xdgPath
								return
							}
						}
					}
				}
				// Migration failed or no XDG — use legacy
				configPath = legacyPath
				return
			}
		}

		// 3. No existing config — prefer XDG for new files
		if xdgPath != "" {
			configPath = xdgPath
			return
		}
		if legacyPath != "" {
			configPath = legacyPath
		}
	})
	return configPath
}

// Path returns the resolved config file path.
func Path() string {
	return resolveConfigPath()
}

// Load reads the config file, falling back to defaults.
// Creates a default config file if none exists.
func Load() Config {
	cfg := DefaultConfig()

	path := resolveConfigPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// No config file — create one with defaults
		_ = cfg.Save()
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	cfg.applyDefaults()
	return cfg
}

// Reload re-reads the config file from disk, returning a fresh Config.
func Reload() Config {
	// Reset the path cache to pick up any changes
	cfg := DefaultConfig()

	path := resolveConfigPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	cfg.applyDefaults()
	return cfg
}

// ModTime returns the last modification time of the config file.
func ModTime() time.Time {
	path := resolveConfigPath()
	if path == "" {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Save writes the config to the resolved config file path.
func (c *Config) Save() error {
	path := resolveConfigPath()
	if path == "" {
		return os.ErrNotExist
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) applyDefaults() {
	if c.Defaults.RefreshInterval <= 0 {
		c.Defaults.RefreshInterval = 5
	}
	if c.Display.MaxEvents <= 0 {
		c.Display.MaxEvents = 50
	}
	if c.Display.MaxLogLines <= 0 {
		c.Display.MaxLogLines = 1000
	}
}

// SaveDir returns the configured save directory, or "./" as default.
func (c *Config) SaveDir() string {
	if c.Defaults.SaveDirectory != "" {
		return c.Defaults.SaveDirectory
	}
	return "./"
}

// AddSSMPrefix adds or updates an SSM prefix entry. Returns true if it was new.
func (c *Config) AddSSMPrefix(name, prefix string) bool {
	for i, p := range c.SSMPrefixes {
		if p.Name == name {
			c.SSMPrefixes[i].Prefix = prefix
			return false
		}
	}
	c.SSMPrefixes = append(c.SSMPrefixes, SSMPrefix{Name: name, Prefix: prefix})
	return true
}

// RemoveSSMPrefix removes an SSM prefix by name.
func (c *Config) RemoveSSMPrefix(name string) {
	for i, p := range c.SSMPrefixes {
		if p.Name == name {
			c.SSMPrefixes = append(c.SSMPrefixes[:i], c.SSMPrefixes[i+1:]...)
			return
		}
	}
}

// ModuleEnabled returns whether a module is enabled. Nil (unset) defaults to true.
func boolDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func (c *Config) ModuleS3() bool          { return boolDefault(c.Modules.S3, true) }
func (c *Config) ModuleLambda() bool      { return boolDefault(c.Modules.Lambda, true) }
func (c *Config) ModuleDynamoDB() bool    { return boolDefault(c.Modules.DynamoDB, true) }
func (c *Config) ModuleSQS() bool         { return boolDefault(c.Modules.SQS, true) }
func (c *Config) ModuleCodeBuild() bool   { return boolDefault(c.Modules.CodeBuild, true) }
func (c *Config) ModuleECS() bool        { return boolDefault(c.Modules.ECS, true) }
func (c *Config) ModuleCWLogs() bool {
	if c.Modules.CWLogs != nil {
		return *c.Modules.CWLogs
	}
	return boolDefault(c.Modules.CloudWatch, true) // legacy fallback
}
func (c *Config) ModuleCWAlarms() bool { return boolDefault(c.Modules.CWAlarms, true) }
func (c *Config) ModuleSSM() bool         { return boolDefault(c.Modules.SSM, true) }
func (c *Config) ModuleSM() bool          { return boolDefault(c.Modules.SM, true) }

// AddSMFilter adds or updates a saved Secrets Manager filter.
func (c *Config) AddSMFilter(name, filter string) bool {
	for i, f := range c.SMFilters {
		if f.Name == name {
			c.SMFilters[i].Filter = filter
			return false
		}
	}
	c.SMFilters = append(c.SMFilters, SMFilter{Name: name, Filter: filter})
	return true
}

// AddSQSQueue adds or updates a saved SQS queue.
func (c *Config) AddSQSQueue(name, url string) bool {
	for i, q := range c.SQSQueues {
		if q.Name == name {
			c.SQSQueues[i].URL = url
			return false
		}
	}
	c.SQSQueues = append(c.SQSQueues, SQSQueueEntry{Name: name, URL: url})
	return true
}

// RemoveSQSQueue removes a saved SQS queue by name.
func (c *Config) RemoveSQSQueue(name string) {
	for i, q := range c.SQSQueues {
		if q.Name == name {
			c.SQSQueues = append(c.SQSQueues[:i], c.SQSQueues[i+1:]...)
			return
		}
	}
}

// AddDynamoTable adds or updates a saved DynamoDB table.
func (c *Config) AddDynamoTable(name, table string) bool {
	for i, t := range c.DynamoTables {
		if t.Name == name {
			c.DynamoTables[i].Table = table
			return false
		}
	}
	c.DynamoTables = append(c.DynamoTables, DynamoTable{Name: name, Table: table})
	return true
}

// RemoveDynamoTable removes a saved DynamoDB table by name.
func (c *Config) RemoveDynamoTable(name string) {
	for i, t := range c.DynamoTables {
		if t.Name == name {
			c.DynamoTables = append(c.DynamoTables[:i], c.DynamoTables[i+1:]...)
			return
		}
	}
}

// AddDynamoQuery adds or updates a saved PartiQL query.
func (c *Config) AddDynamoQuery(name, statement string) bool {
	for i, q := range c.DynamoQueries {
		if q.Name == name {
			c.DynamoQueries[i].Statement = statement
			return false
		}
	}
	c.DynamoQueries = append(c.DynamoQueries, DynamoQuery{Name: name, Statement: statement})
	return true
}

// RemoveDynamoQuery removes a saved PartiQL query by name.
func (c *Config) RemoveDynamoQuery(name string) {
	for i, q := range c.DynamoQueries {
		if q.Name == name {
			c.DynamoQueries = append(c.DynamoQueries[:i], c.DynamoQueries[i+1:]...)
			return
		}
	}
}

// AddLambdaSearch adds or updates a saved Lambda search filter.
func (c *Config) AddLambdaSearch(name, filter string) bool {
	for i, s := range c.LambdaSearches {
		if s.Name == name {
			c.LambdaSearches[i].Filter = filter
			return false
		}
	}
	c.LambdaSearches = append(c.LambdaSearches, LambdaSearch{Name: name, Filter: filter})
	return true
}

// RemoveLambdaSearch removes a saved Lambda search by name.
func (c *Config) RemoveLambdaSearch(name string) {
	for i, s := range c.LambdaSearches {
		if s.Name == name {
			c.LambdaSearches = append(c.LambdaSearches[:i], c.LambdaSearches[i+1:]...)
			return
		}
	}
}

// AddS3Search adds or updates a saved S3 bucket search.
func (c *Config) AddS3Search(name, filter string) bool {
	for i, s := range c.S3Searches {
		if s.Name == name {
			c.S3Searches[i].Filter = filter
			return false
		}
	}
	c.S3Searches = append(c.S3Searches, S3Search{Name: name, Filter: filter})
	return true
}

// RemoveS3Search removes a saved S3 search by name.
func (c *Config) RemoveS3Search(name string) {
	for i, s := range c.S3Searches {
		if s.Name == name {
			c.S3Searches = append(c.S3Searches[:i], c.S3Searches[i+1:]...)
			return
		}
	}
}

// RemoveSMFilter removes a saved SM filter by name.
func (c *Config) RemoveSMFilter(name string) {
	for i, f := range c.SMFilters {
		if f.Name == name {
			c.SMFilters = append(c.SMFilters[:i], c.SMFilters[i+1:]...)
			return
		}
	}
}

// RemoveLogPath removes a saved log path by name.
func (c *Config) RemoveLogPath(name string) {
	for i, p := range c.LogPaths {
		if p.Name == name {
			c.LogPaths = append(c.LogPaths[:i], c.LogPaths[i+1:]...)
			return
		}
	}
}

// AddLogPath adds or updates a saved log path.
func (c *Config) AddLogPath(name, logGroup, stream string) bool {
	for i, p := range c.LogPaths {
		if p.Name == name {
			c.LogPaths[i].LogGroup = logGroup
			c.LogPaths[i].LogGroups = nil
			c.LogPaths[i].Stream = stream
			return false
		}
	}
	c.LogPaths = append(c.LogPaths, LogPathEntry{Name: name, LogGroup: logGroup, Stream: stream})
	return true
}

// AddLogPathMultiGroup adds or updates a saved multi-group log path.
func (c *Config) AddLogPathMultiGroup(name string, groups []string) bool {
	for i, p := range c.LogPaths {
		if p.Name == name {
			c.LogPaths[i].LogGroup = groups[0]
			c.LogPaths[i].LogGroups = groups
			c.LogPaths[i].Stream = ""
			return false
		}
	}
	c.LogPaths = append(c.LogPaths, LogPathEntry{
		Name:      name,
		LogGroup:  groups[0],
		LogGroups: groups,
	})
	return true
}
