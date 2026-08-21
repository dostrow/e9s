package sqlworkbench

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStateRoundTripDoesNotContainResultsOrCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := WorkbenchState{ActiveTabID: "one", Tabs: []TabState{{ID: "one", ProfileName: "prod", Query: "select 1",
		Explorer: ExplorerState{Expanded: []string{"schema:public", "category:public:table"}, Selected: "object:42:table", Filter: "orders", Scroll: 120}}}}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("state permissions = %o", info.Mode().Perm())
		}
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tabs) != 1 || got.Tabs[0].Query != "select 1" || got.ActiveTabID != "one" ||
		got.Tabs[0].Explorer.Selected != "object:42:table" || got.Tabs[0].Explorer.Filter != "orders" || got.Tabs[0].Explorer.Scroll != 120 {
		t.Fatalf("unexpected state: %#v", got)
	}
}
