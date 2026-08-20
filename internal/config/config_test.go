package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/dostrow/e9s/internal/model"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.RefreshInterval != 5 {
		t.Errorf("RefreshInterval = %d, want 5", cfg.Defaults.RefreshInterval)
	}
	if cfg.Defaults.IdleTimeout != 300 {
		t.Errorf("IdleTimeout = %d, want 300", cfg.Defaults.IdleTimeout)
	}
	if cfg.Defaults.CostGuardUSD != 1 {
		t.Errorf("CostGuardUSD = %v, want 1", cfg.Defaults.CostGuardUSD)
	}
	if cfg.Display.TimestampFormat != "relative" {
		t.Errorf("TimestampFormat = %q, want %q", cfg.Display.TimestampFormat, "relative")
	}
	if cfg.Display.MaxEvents != 50 {
		t.Errorf("MaxEvents = %d, want 50", cfg.Display.MaxEvents)
	}
	if cfg.Display.MaxLogLines != 1000 {
		t.Errorf("MaxLogLines = %d, want 1000", cfg.Display.MaxLogLines)
	}
}

func TestValidateSQLConnections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SQL.Connections = []SQLConnection{{Name: "production", Database: "app", Auth: "secrets-manager"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected a Secrets Manager profile without secret_arn to fail")
	}
	cfg.SQL.Connections[0].SecretARN = "arn:aws:secretsmanager:us-east-2:123:secret:db"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid SQL connection, got %v", err)
	}
	cfg.SQL.Connections = append(cfg.SQL.Connections, cfg.SQL.Connections[0])
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate SQL profile names to fail")
	}
}

func TestModuleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ModuleECS() {
		t.Error("ModuleECS should default to true")
	}
	if !cfg.ModuleCWLogs() {
		t.Error("ModuleCWLogs should default to true")
	}
	if !cfg.ModuleCWAlarms() {
		t.Error("ModuleCWAlarms should default to true")
	}
	if !cfg.ModuleSSM() {
		t.Error("ModuleSSM should default to true")
	}
	if !cfg.ModuleSM() {
		t.Error("ModuleSM should default to true")
	}
	if !cfg.ModuleS3() {
		t.Error("ModuleS3 should default to true")
	}
	if !cfg.ModuleLambda() {
		t.Error("ModuleLambda should default to true")
	}
	if !cfg.ModuleDynamoDB() {
		t.Error("ModuleDynamoDB should default to true")
	}
	if !cfg.ModuleCostExplorer() {
		t.Error("ModuleCostExplorer should default to true")
	}
	if !cfg.ModuleElastiCache() {
		t.Error("ModuleElastiCache should default to true")
	}
	if !cfg.ModuleAPIGateway() {
		t.Error("ModuleAPIGateway should default to true")
	}
}

func TestCostViewValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CostViews = []CostView{{Name: "", Days: 30}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty Cost Explorer saved-view name to fail validation")
	}
	cfg.CostViews = []CostView{{Name: "services", Days: -1}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative Cost Explorer saved-view days to fail validation")
	}
}

func TestModuleDisable(t *testing.T) {
	cfg := DefaultConfig()
	f := false
	cfg.Modules.SSM = &f

	if cfg.ModuleSSM() {
		t.Error("ModuleSSM should be false when explicitly set")
	}
	if !cfg.ModuleECS() {
		t.Error("ModuleECS should still be true")
	}
}

func TestSSMPrefixCRUD(t *testing.T) {
	cfg := DefaultConfig()

	// Add
	isNew := cfg.AddSSMPrefix("test", "/my/path")
	if !isNew {
		t.Error("AddSSMPrefix should return true for new entry")
	}
	if len(cfg.SSMPrefixes) != 1 {
		t.Fatalf("SSMPrefixes count = %d, want 1", len(cfg.SSMPrefixes))
	}
	if cfg.SSMPrefixes[0].Prefix != "/my/path" {
		t.Errorf("Prefix = %q, want %q", cfg.SSMPrefixes[0].Prefix, "/my/path")
	}

	// Update
	isNew = cfg.AddSSMPrefix("test", "/my/new/path")
	if isNew {
		t.Error("AddSSMPrefix should return false for existing entry")
	}
	if cfg.SSMPrefixes[0].Prefix != "/my/new/path" {
		t.Errorf("Prefix = %q, want %q", cfg.SSMPrefixes[0].Prefix, "/my/new/path")
	}

	// Remove
	cfg.RemoveSSMPrefix("test")
	if len(cfg.SSMPrefixes) != 0 {
		t.Errorf("SSMPrefixes count = %d, want 0", len(cfg.SSMPrefixes))
	}

	// Remove non-existent (no panic)
	cfg.RemoveSSMPrefix("nonexistent")
}

func TestSMFilterCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddSMFilter("secrets", "prod")
	if len(cfg.SMFilters) != 1 {
		t.Fatalf("SMFilters count = %d, want 1", len(cfg.SMFilters))
	}

	cfg.RemoveSMFilter("secrets")
	if len(cfg.SMFilters) != 0 {
		t.Errorf("SMFilters count = %d, want 0", len(cfg.SMFilters))
	}
}

func TestLogPathCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddLogPath("api", "/aws/ecs/api", "stream1")
	if len(cfg.LogPaths) != 1 {
		t.Fatalf("LogPaths count = %d, want 1", len(cfg.LogPaths))
	}
	if cfg.LogPaths[0].Stream != "stream1" {
		t.Errorf("Stream = %q, want %q", cfg.LogPaths[0].Stream, "stream1")
	}

	cfg.RemoveLogPath("api")
	if len(cfg.LogPaths) != 0 {
		t.Errorf("LogPaths count = %d, want 0", len(cfg.LogPaths))
	}
}

func TestCompleteLogPathCRUD(t *testing.T) {
	cfg := DefaultConfig()
	entry := LogPathEntry{
		Name: "errors", LogGroup: "/aws/ecs/api", LogGroups: []string{"/aws/ecs/api"},
		Streams: []string{"api/one", "api/two"}, Filter: `"error"`, Lookback: "1h",
		HighlightRules: []model.LogHighlightRule{{Pattern: "ERROR", Match: model.LogHighlightLiteral, Style: model.LogHighlightError}},
		HiddenStreams:  []string{"api/noisy"},
	}
	if !cfg.UpsertLogPath(entry) {
		t.Fatal("UpsertLogPath() new = false")
	}
	entry.Streams[0] = "mutated"
	entry.HighlightRules[0].Pattern = "mutated"
	entry.HiddenStreams[0] = "mutated"
	if cfg.LogPaths[0].Streams[0] != "api/one" {
		t.Fatal("UpsertLogPath() retained caller slice")
	}
	if cfg.LogPaths[0].HighlightRules[0].Pattern != "ERROR" {
		t.Fatal("UpsertLogPath() retained caller highlight rules")
	}
	if cfg.LogPaths[0].HiddenStreams[0] != "api/noisy" {
		t.Fatal("UpsertLogPath() retained caller hidden streams")
	}
	if !cfg.RenameLogPath("errors", "API errors") {
		t.Fatal("RenameLogPath() = false")
	}
	cfg.UpsertLogPath(LogPathEntry{Name: "deployments", LogGroup: "/aws/ecs/api"})
	if !cfg.MoveLogPath("deployments", -1) || cfg.LogPaths[0].Name != "deployments" {
		t.Fatalf("MoveLogPath() paths = %#v", cfg.LogPaths)
	}
	if cfg.RenameLogPath("deployments", "API errors") {
		t.Fatal("RenameLogPath() allowed a duplicate name")
	}
}

func TestLogHighlightRulesYAMLCompatibility(t *testing.T) {
	var legacy Config
	if err := yaml.Unmarshal([]byte("log_paths:\n  - name: api\n    log_group: /aws/ecs/api\n"), &legacy); err != nil {
		t.Fatal(err)
	}
	if got := legacy.LogPaths[0].HighlightRules; len(got) != 0 {
		t.Fatalf("legacy highlight rules = %#v, want none", got)
	}

	want := []model.LogHighlightRule{{Pattern: "timeout", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightWarning}}
	cfg := Config{LogPaths: []LogPathEntry{{Name: "api", LogGroup: "/aws/ecs/api", HighlightRules: want}}}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Config
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip.LogPaths[0].HighlightRules, want) {
		t.Fatalf("round-trip highlight rules = %#v, want %#v", roundTrip.LogPaths[0].HighlightRules, want)
	}
}

func TestS3SearchCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddS3Search("data", "my-bucket")
	if len(cfg.S3Searches) != 1 {
		t.Fatalf("S3Searches count = %d, want 1", len(cfg.S3Searches))
	}

	cfg.RemoveS3Search("data")
	if len(cfg.S3Searches) != 0 {
		t.Errorf("S3Searches count = %d, want 0", len(cfg.S3Searches))
	}
}

func TestLambdaSearchCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddLambdaSearch("workers", "worker")
	if len(cfg.LambdaSearches) != 1 {
		t.Fatalf("LambdaSearches count = %d, want 1", len(cfg.LambdaSearches))
	}

	cfg.RemoveLambdaSearch("workers")
	if len(cfg.LambdaSearches) != 0 {
		t.Errorf("LambdaSearches count = %d, want 0", len(cfg.LambdaSearches))
	}
}

func TestDynamoTableCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddDynamoTable("users", "prod-users")
	if len(cfg.DynamoTables) != 1 {
		t.Fatalf("DynamoTables count = %d, want 1", len(cfg.DynamoTables))
	}

	cfg.RemoveDynamoTable("users")
	if len(cfg.DynamoTables) != 0 {
		t.Errorf("DynamoTables count = %d, want 0", len(cfg.DynamoTables))
	}
}

func TestDynamoQueryCRUD(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddDynamoQuery("find user", "SELECT * FROM users WHERE id = '123'")
	if len(cfg.DynamoQueries) != 1 {
		t.Fatalf("DynamoQueries count = %d, want 1", len(cfg.DynamoQueries))
	}

	cfg.RemoveDynamoQuery("find user")
	if len(cfg.DynamoQueries) != 0 {
		t.Errorf("DynamoQueries count = %d, want 0", len(cfg.DynamoQueries))
	}
}

func resetConfigPath() {
	configPathOnce = sync.Once{}
	configPath = ""
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	resetConfigPath()
	defer resetConfigPath()

	cfg := DefaultConfig()
	cfg.Defaults.Cluster = "test-cluster"
	cfg.Defaults.Region = "us-west-2"
	cfg.AddSSMPrefix("test", "/my/prefix")

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists at XDG path
	path := filepath.Join(tmpDir, "e9s", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Config file not created at %s", path)
	}

	// Load it back
	resetConfigPath()
	loaded := Load()
	if loaded.Defaults.Cluster != "test-cluster" {
		t.Errorf("Loaded Cluster = %q, want %q", loaded.Defaults.Cluster, "test-cluster")
	}
	if loaded.Defaults.Region != "us-west-2" {
		t.Errorf("Loaded Region = %q, want %q", loaded.Defaults.Region, "us-west-2")
	}
	if len(loaded.SSMPrefixes) != 1 {
		t.Fatalf("Loaded SSMPrefixes count = %d, want 1", len(loaded.SSMPrefixes))
	}
}

func TestSavePreservesUnknownKeysAndComments(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	resetConfigPath()
	defer resetConfigPath()

	path := filepath.Join(tmpDir, "e9s", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := "# operator notes\ndefaults:\n  # keep this explanation\n  region: us-east-1\ncustom_plugin:\n  enabled: true\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	cfg.Defaults.Region = "us-west-2"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, fragment := range []string{"# operator notes", "# keep this explanation", "custom_plugin:", "enabled: true", "region: us-west-2"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("saved configuration is missing %q:\n%s", fragment, text)
		}
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup changed:\n%s", backup)
	}
}

func TestSaveRawRejectsInvalidConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	resetConfigPath()
	defer resetConfigPath()

	cfg := DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmpDir, "e9s", "config.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveRaw([]byte("display:\n  timestamp_format: sometimes\n")); err == nil {
		t.Fatal("SaveRaw accepted an invalid timestamp format")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid raw save changed the active configuration")
	}
}

func TestGUIAppearanceRoundTrip(t *testing.T) {
	cfg, err := Parse([]byte(`
gui:
  appearance:
    preset: gruvbox-material-dark
    interface_font: Inter 11
    monospace_font: JetBrainsMono Nerd Font 10
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GUI.Appearance.Preset != "gruvbox-material-dark" || cfg.GUI.Appearance.InterfaceFont != "Inter 11" || cfg.GUI.Appearance.MonospaceFont != "JetBrainsMono Nerd Font 10" {
		t.Fatalf("appearance did not round-trip through YAML: %+v", cfg.GUI.Appearance)
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	resetConfigPath()
	defer resetConfigPath()

	cfg := Load()
	if cfg.Defaults.RefreshInterval != 5 {
		t.Errorf("RefreshInterval = %d, want 5", cfg.Defaults.RefreshInterval)
	}
}

func TestSaveDir(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SaveDir() != "./" {
		t.Errorf("Default SaveDir = %q, want %q", cfg.SaveDir(), "./")
	}

	cfg.Defaults.SaveDirectory = "/tmp/downloads"
	if cfg.SaveDir() != "/tmp/downloads" {
		t.Errorf("SaveDir = %q, want %q", cfg.SaveDir(), "/tmp/downloads")
	}
}

func TestConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	resetConfigPath()
	defer resetConfigPath()

	path := Path()
	if path == "" {
		t.Fatal("Path() should not be empty")
	}
	if !strings.Contains(path, "e9s") {
		t.Errorf("Path should contain 'e9s': %q", path)
	}
}
