//go:build gui && windows

package gui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func preparePlatformRuntime() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate Windows executable: %w", err)
	}
	root := filepath.Dir(executable)
	if !isPortableWindowsRuntime(root) {
		// Native development builds run against the active UCRT64 prefix.
		return nil
	}

	for name, value := range windowsRuntimeEnvironment(root) {
		if os.Getenv(name) == "" {
			if err := os.Setenv(name, value); err != nil {
				return fmt.Errorf("configure %s: %w", name, err)
			}
		}
	}
	prependEnvironmentPath("PATH", root)
	prependEnvironmentPath("XDG_DATA_DIRS", filepath.Join(root, "share"))

	if err := rebuildGdkPixbufCache(root); err != nil {
		return err
	}
	return nil
}

func isPortableWindowsRuntime(root string) bool {
	checks := []string{
		filepath.Join(root, "libgtk-4-1.dll"),
		filepath.Join(root, "libgtksourceview-5-0.dll"),
		filepath.Join(root, "share", "glib-2.0", "schemas", "gschemas.compiled"),
		filepath.Join(root, "share", "gtksourceview-5"),
	}
	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func windowsRuntimeEnvironment(root string) map[string]string {
	loaderRoot := filepath.Join(root, "lib", "gdk-pixbuf-2.0", "2.10.0")
	return map[string]string{
		"E9S_DATA_DIR":           filepath.Join(root, "share", "e9s"),
		"FONTCONFIG_FILE":        filepath.Join(root, "etc", "fonts", "fonts.conf"),
		"FONTCONFIG_PATH":        filepath.Join(root, "etc", "fonts"),
		"GDK_PIXBUF_MODULEDIR":   filepath.Join(loaderRoot, "loaders"),
		"GDK_PIXBUF_MODULE_FILE": filepath.Join(loaderRoot, "loaders.cache"),
		"GIO_EXTRA_MODULES":      filepath.Join(root, "lib", "gio", "modules"),
		"GSETTINGS_SCHEMA_DIR":   filepath.Join(root, "share", "glib-2.0", "schemas"),
		"GTK_DATA_PREFIX":        root,
		"GTK_EXE_PREFIX":         root,
	}
}

func prependEnvironmentPath(name, value string) {
	existing := os.Getenv(name)
	if existing == "" {
		_ = os.Setenv(name, value)
		return
	}
	for _, entry := range filepath.SplitList(existing) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(value)) {
			return
		}
	}
	_ = os.Setenv(name, value+string(os.PathListSeparator)+existing)
}

func rebuildGdkPixbufCache(root string) error {
	loaderDirectory := filepath.Join(root, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders")
	loaders, err := filepath.Glob(filepath.Join(loaderDirectory, "*.dll"))
	if err != nil {
		return fmt.Errorf("find bundled GdkPixbuf loaders: %w", err)
	}
	if len(loaders) == 0 {
		return nil
	}
	query := filepath.Join(root, "gdk-pixbuf-query-loaders.exe")
	if _, err := os.Stat(query); err != nil {
		return fmt.Errorf("portable GTK runtime is missing %s", filepath.Base(query))
	}

	command := exec.Command(query, loaders...)
	command.Dir = root
	command.Env = os.Environ()
	contents, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return fmt.Errorf("refresh GdkPixbuf loader cache: %w: %s", err, strings.TrimSpace(string(exitError.Stderr)))
		}
		return fmt.Errorf("refresh GdkPixbuf loader cache: %w", err)
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil || cacheRoot == "" {
		cacheRoot = os.TempDir()
	}
	cacheDirectory := filepath.Join(cacheRoot, "e9s", "runtime")
	if err := os.MkdirAll(cacheDirectory, 0o700); err != nil {
		return fmt.Errorf("create e9s runtime cache: %w", err)
	}
	cacheFile := filepath.Join(cacheDirectory, "gdk-pixbuf-loaders.cache")
	temporary, err := os.CreateTemp(cacheDirectory, "gdk-pixbuf-loaders-*.cache")
	if err != nil {
		return fmt.Errorf("create GdkPixbuf loader cache: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write GdkPixbuf loader cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close GdkPixbuf loader cache: %w", err)
	}
	if err := os.Rename(temporaryName, cacheFile); err != nil {
		// Windows cannot replace an existing destination atomically.
		if removeError := os.Remove(cacheFile); removeError != nil && !os.IsNotExist(removeError) {
			return fmt.Errorf("replace GdkPixbuf loader cache: %w", removeError)
		}
		if err := os.Rename(temporaryName, cacheFile); err != nil {
			return fmt.Errorf("install GdkPixbuf loader cache: %w", err)
		}
	}
	return os.Setenv("GDK_PIXBUF_MODULE_FILE", cacheFile)
}
