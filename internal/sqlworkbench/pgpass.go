package sqlworkbench

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrPGPassNoMatch = errors.New("no matching .pgpass entry")

type PGPassMatch struct {
	Host     string
	Port     string
	Database string
	User     string
}

type PGPassEntry struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

// ResolvePGPass checks files in order and returns the first matching entry in
// the first matching file. Missing files are skipped, while malformed or
// insecure files are reported so a credential mistake cannot go unnoticed.
func ResolvePGPass(paths []string, match PGPassMatch) (string, string, error) {
	if len(paths) == 0 {
		paths = DefaultPGPassFiles()
	}
	var checked []string
	for _, configuredPath := range paths {
		path, err := expandUserPath(configuredPath)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(path) == "" {
			continue
		}
		checked = append(checked, path)
		entries, err := LoadPGPass(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		for _, entry := range entries {
			if entry.matches(match) {
				return entry.Password, path, nil
			}
		}
	}
	return "", "", fmt.Errorf("%w in %s", ErrPGPassNoMatch, strings.Join(checked, ", "))
}

func DefaultPGPassFiles() []string {
	if path := strings.TrimSpace(os.Getenv("PGPASSFILE")); path != "" {
		return []string{path}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(home, "AppData", "Roaming", "postgresql", "pgpass.conf")}
	}
	return []string{filepath.Join(home, ".pgpass")}
}

func LoadPGPass(path string) ([]PGPassEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pgpass file %s is not a regular file", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("pgpass file %s has permissions %04o; PostgreSQL requires 0600 or stricter", path, info.Mode().Perm())
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var entries []PGPassEntry
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields, err := splitPGPassLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse pgpass file %s line %d: %w", path, lineNumber, err)
		}
		entries = append(entries, PGPassEntry{Host: fields[0], Port: fields[1], Database: fields[2], User: fields[3], Password: fields[4]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pgpass file %s: %w", path, err)
	}
	return entries, nil
}

func (entry PGPassEntry) matches(match PGPassMatch) bool {
	return pgpassFieldMatches(entry.Host, match.Host) && pgpassFieldMatches(entry.Port, match.Port) &&
		pgpassFieldMatches(entry.Database, match.Database) && pgpassFieldMatches(entry.User, match.User)
}

func pgpassFieldMatches(pattern, value string) bool { return pattern == "*" || pattern == value }

func splitPGPassLine(line string) ([]string, error) {
	fields := make([]string, 0, 5)
	var field strings.Builder
	escaped := false
	for _, character := range line {
		if escaped {
			field.WriteRune(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == ':' && len(fields) < 4 {
			fields = append(fields, field.String())
			field.Reset()
			continue
		}
		field.WriteRune(character)
	}
	if escaped {
		field.WriteRune('\\')
	}
	fields = append(fields, field.String())
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 colon-delimited fields, found %d", len(fields))
	}
	return fields, nil
}

func expandUserPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
