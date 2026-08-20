package sqlworkbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const stateVersion = 1

type TabState struct {
	ID          string    `json:"id"`
	ProfileName string    `json:"profile_name"`
	Title       string    `json:"title,omitempty"`
	ManualTitle bool      `json:"manual_title,omitempty"`
	Query       string    `json:"query,omitempty"`
	AllowWrites bool      `json:"allow_writes,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkbenchState struct {
	Version     int        `json:"version"`
	ActiveTabID string     `json:"active_tab_id,omitempty"`
	Tabs        []TabState `json:"tabs,omitempty"`
}

func DefaultStatePath() (string, error) {
	if directory := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); directory != "" {
		return filepath.Join(directory, "e9s", "sql-workbench.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		if directory, err := os.UserConfigDir(); err == nil {
			return filepath.Join(directory, "e9s", "sql-workbench.json"), nil
		}
	}
	return filepath.Join(home, ".local", "state", "e9s", "sql-workbench.json"), nil
}

func LoadState(path string) (WorkbenchState, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultStatePath()
		if err != nil {
			return WorkbenchState{}, err
		}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return WorkbenchState{Version: stateVersion}, nil
	}
	if err != nil {
		return WorkbenchState{}, err
	}
	var state WorkbenchState
	if err := json.Unmarshal(data, &state); err != nil {
		return WorkbenchState{}, fmt.Errorf("parse SQL workbench state: %w", err)
	}
	if state.Version != 0 && state.Version != stateVersion {
		return WorkbenchState{}, fmt.Errorf("unsupported SQL workbench state version %d", state.Version)
	}
	state.Version = stateVersion
	return state, nil
}

func SaveState(path string, state WorkbenchState) error {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultStatePath()
		if err != nil {
			return err
		}
	}
	state.Version = stateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".sql-workbench-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
