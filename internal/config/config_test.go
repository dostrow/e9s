package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.RefreshInterval != 5 {
		t.Errorf("RefreshInterval = %d, want 5", cfg.Defaults.RefreshInterval)
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

func TestModuleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ModuleECS() {
		t.Error("ModuleECS should default to true")
	}
	if !cfg.ModuleCloudWatch() {
		t.Error("ModuleCloudWatch should default to true")
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

func TestSaveAndLoad(t *testing.T) {
	// Use a temp dir as home
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := DefaultConfig()
	cfg.Defaults.Cluster = "test-cluster"
	cfg.Defaults.Region = "us-west-2"
	cfg.AddSSMPrefix("test", "/my/prefix")

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, ".e9s.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Config file not created")
	}

	// Load it back
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
	if loaded.SSMPrefixes[0].Prefix != "/my/prefix" {
		t.Errorf("Loaded Prefix = %q, want %q", loaded.SSMPrefixes[0].Prefix, "/my/prefix")
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := Load()
	// Should return defaults
	if cfg.Defaults.RefreshInterval != 5 {
		t.Errorf("RefreshInterval = %d, want 5", cfg.Defaults.RefreshInterval)
	}
}
