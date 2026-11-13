package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

// helpVariants mirror the state-dependent branches in viewItems. The golden
// file was captured from the hand-written help switch before the registry
// replaced it.
var helpVariants = map[string]func(*App){
	"default":      func(*App) {},
	"stopped-more": func(a *App) { a.taskScopeStopped = true; a.taskNextToken = "x" },
	"stopped-end":  func(a *App) { a.taskScopeStopped = true },
	"metrics-task": func(a *App) { a.metricsTaskScope = true },
	"correlate":    func(a *App) { a.logCorrelationActive = true },
}

type helpLine struct{ key, desc string }

// TestRegistryCoversLegacyHelp guarantees the command registry documents
// every key the legacy help overlay did, in every view and state variant.
// A legacy line passes when the registry has the same key and description,
// or when its key was compound ("[/]", "1/2/3") and every component key is
// now registered on its own.
func TestRegistryCoversLegacyHelp(t *testing.T) {
	golden := readLegacyHelpGolden(t)
	if len(golden) == 0 {
		t.Fatal("legacy help golden is empty")
	}
	for name, legacy := range golden {
		var state viewState
		var variant string
		if _, err := fmt.Sscanf(name, "%d %s", &state, &variant); err != nil {
			t.Fatalf("bad golden header %q: %v", name, err)
		}
		a := App{kb: NewKeyBindings(), state: state}
		helpVariants[variant](&a)

		exact := map[helpLine]bool{}
		keys := map[string]bool{}
		for _, item := range append(a.viewItems(state), a.globalItems()...) {
			exact[helpLine{item.displayKey(), item.Title}] = true
			for _, k := range item.keys {
				keys[k] = true
			}
		}
		for _, line := range legacy {
			if line.key == "" || exact[line] {
				continue
			}
			if !strings.Contains(line.key, "/") || line.key == "/" {
				t.Errorf("%s: legacy %q (%s) missing from registry", name, line.key, line.desc)
				continue
			}
			for _, part := range splitCompoundKey(line.key) {
				if !keys[part] {
					t.Errorf("%s: compound %q (%s) lost component %q", name, line.key, line.desc, part)
				}
			}
		}
	}
}

func splitCompoundKey(key string) []string {
	if key == "[/]" {
		return []string{"[", "]"}
	}
	return strings.Split(key, "/")
}

func readLegacyHelpGolden(t *testing.T) map[string][]helpLine {
	t.Helper()
	f, err := os.Open("testdata/legacy_help.golden")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string][]helpLine{}
	var current string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if name, ok := strings.CutPrefix(line, "== state "); ok {
			current = name
			continue
		}
		key, desc, _ := strings.Cut(line, "\t")
		out[current] = append(out[current], helpLine{key, desc})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRegistryItemsAreWellFormed(t *testing.T) {
	for state := viewClusters; state <= viewPluginSettings; state++ {
		for variant, mutate := range helpVariants {
			a := App{kb: NewKeyBindings(), state: state}
			mutate(&a)
			for _, item := range append(a.viewItems(state), a.globalItems()...) {
				if item.Title == "" || len(item.keys) == 0 || item.keys[0] == "" {
					t.Errorf("state %d %s: incomplete item %+v", state, variant, item)
				}
				if item.legend != (item.ID == "") {
					t.Errorf("state %d %s: %q legend=%v but id=%q", state, variant, item.Title, item.legend, item.ID)
				}
			}
		}
	}
}
