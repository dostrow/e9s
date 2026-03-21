package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type LogPathEntry struct {
	Name     string `yaml:"name"`
	LogGroup string `yaml:"log_group"`
	Stream   string `yaml:"stream,omitempty"` // optional — empty means all streams
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
	} `yaml:"defaults"`
	Display struct {
		TimestampFormat string `yaml:"timestamp_format"` // "relative" or "absolute"
		MaxEvents       int    `yaml:"max_events"`
		MaxLogLines     int    `yaml:"max_log_lines"`
	} `yaml:"display"`
	Modules struct {
		ECS        *bool `yaml:"ecs"`
		CloudWatch *bool `yaml:"cloudwatch"`
		SSM        *bool `yaml:"ssm"`
		SM         *bool `yaml:"sm"`
		S3         *bool `yaml:"s3"`
		Lambda     *bool `yaml:"lambda"`
	} `yaml:"modules"`
	ExcludeServices []string `yaml:"exclude_services"`
	SSMPrefixes     []SSMPrefix    `yaml:"ssm_prefixes"`
	SMFilters       []SMFilter     `yaml:"sm_filters"`
	S3Searches      []S3Search     `yaml:"s3_searches"`
	LambdaSearches  []LambdaSearch `yaml:"lambda_searches"`
	LogPaths        []LogPathEntry `yaml:"log_paths"`
}

func DefaultConfig() Config {
	c := Config{}
	c.Defaults.RefreshInterval = 5
	c.Display.TimestampFormat = "relative"
	c.Display.MaxEvents = 50
	c.Display.MaxLogLines = 1000
	return c
}

func Load() Config {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".e9s.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	if cfg.Defaults.RefreshInterval <= 0 {
		cfg.Defaults.RefreshInterval = 5
	}
	if cfg.Display.MaxEvents <= 0 {
		cfg.Display.MaxEvents = 50
	}
	if cfg.Display.MaxLogLines <= 0 {
		cfg.Display.MaxLogLines = 1000
	}

	return cfg
}

func (c *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	path := filepath.Join(home, ".e9s.yaml")
	return os.WriteFile(path, data, 0644)
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
func (c *Config) ModuleECS() bool        { return boolDefault(c.Modules.ECS, true) }
func (c *Config) ModuleCloudWatch() bool  { return boolDefault(c.Modules.CloudWatch, true) }
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

func (c *Config) RemoveLambdaSearch(name string) {
	for i, s := range c.LambdaSearches {
		if s.Name == name {
			c.LambdaSearches = append(c.LambdaSearches[:i], c.LambdaSearches[i+1:]...)
			return
		}
	}
}

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

// AddLogPath adds or updates a saved log path.
// RemoveLogPath removes a saved log path by name.
func (c *Config) RemoveLogPath(name string) {
	for i, p := range c.LogPaths {
		if p.Name == name {
			c.LogPaths = append(c.LogPaths[:i], c.LogPaths[i+1:]...)
			return
		}
	}
}

func (c *Config) AddLogPath(name, logGroup, stream string) bool {
	for i, p := range c.LogPaths {
		if p.Name == name {
			c.LogPaths[i].LogGroup = logGroup
			c.LogPaths[i].Stream = stream
			return false
		}
	}
	c.LogPaths = append(c.LogPaths, LogPathEntry{Name: name, LogGroup: logGroup, Stream: stream})
	return true
}
