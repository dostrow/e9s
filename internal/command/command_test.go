package command

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestMatchRequiresOrderedSubsequence(t *testing.T) {
	for _, tc := range []struct {
		query, text string
		want        bool
	}{
		{"", "anything", true},
		{"fnd", "Force new deployment", true},
		{"FND", "force new deployment", true},
		{"dnf", "Force new deployment", false},
		{"stopx", "Stop task", false},
		{"longer than text", "short", false},
	} {
		_, _, ok := Match(tc.query, tc.text)
		if ok != tc.want {
			t.Errorf("Match(%q, %q) ok = %v, want %v", tc.query, tc.text, ok, tc.want)
		}
	}
}

func TestMatchPrefersWordBoundariesAndRuns(t *testing.T) {
	_, positions, ok := Match("fnd", "Force new deployment")
	if !ok {
		t.Fatal("expected a match")
	}
	if want := []int{0, 6, 10}; !reflect.DeepEqual(positions, want) {
		t.Fatalf("positions = %v, want %v (word starts)", positions, want)
	}
	_, positions, _ = Match("task", "Stop task")
	if want := []int{5, 6, 7, 8}; !reflect.DeepEqual(positions, want) {
		t.Fatalf("positions = %v, want contiguous run %v", positions, want)
	}
	boundary, _, _ := Match("sg", "Security groups")
	inner, _, _ := Match("sg", "Messages")
	if boundary <= inner {
		t.Fatalf("boundary score %d should beat inner score %d", boundary, inner)
	}
}

func TestFilterBrowseGroupsBySectionAndOmitsUnavailable(t *testing.T) {
	entries := []Entry{
		{Command: Command{ID: "g", Title: "Quit"}, Section: SectionGlobal},
		{Command: Command{ID: "v1", Title: "Stop task"}, Section: SectionView},
		{Command: Command{ID: "x", Title: "Delete image"}, Section: SectionView, Disabled: "Available in ECR"},
		{Command: Command{ID: "v2", Title: "Tail logs"}, Section: SectionView},
		{Command: Command{ID: "g", Title: "Quit (dup)"}, Section: SectionGlobal},
	}
	got := ids(Filter(entries, "  ", nil))
	if want := []string{"v1", "v2", "g"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("browse order = %v, want %v", got, want)
	}
}

func TestFilterBrowseLiftsRecentCommands(t *testing.T) {
	rec := NewRecency()
	rec.Record("g")
	entries := []Entry{
		{Command: Command{ID: "v1", Title: "Stop task"}, Section: SectionView},
		{Command: Command{ID: "g", Title: "Quit"}, Section: SectionGlobal},
	}
	results := Filter(entries, "", rec)
	if got := ids(results); !reflect.DeepEqual(got, []string{"g", "v1"}) {
		t.Fatalf("order = %v", got)
	}
	if results[0].Section != SectionRecent {
		t.Fatalf("recent entry section = %v", results[0].Section)
	}
}

func TestFilterMatchesAcrossFieldsAndRanksViewFirst(t *testing.T) {
	entries := []Entry{
		{Command: Command{ID: "ec2.stop", Title: "Stop instance", Category: "EC2"}, Section: SectionGlobal},
		{Command: Command{ID: "ecs.stop", Title: "Stop task", Category: "ECS · Tasks", Keywords: []string{"kill"}}, Section: SectionView},
		{Command: Command{ID: "ecs.exec", Title: "ECS Exec", Category: "ECS · Tasks", Keywords: []string{"ssh", "shell"}}, Section: SectionView},
	}
	if got := ids(Filter(entries, "stop", nil)); !reflect.DeepEqual(got, []string{"ecs.stop", "ec2.stop"}) {
		t.Fatalf("stop = %v", got)
	}
	if got := ids(Filter(entries, "ssh", nil)); !reflect.DeepEqual(got, []string{"ecs.exec"}) {
		t.Fatalf("keyword search = %v", got)
	}
	if got := ids(Filter(entries, "ec2 stop", nil)); !reflect.DeepEqual(got, []string{"ec2.stop"}) {
		t.Fatalf("category + title = %v", got)
	}
	results := Filter(entries, "ec2 stop", nil)
	if want := []int{0, 1, 2, 3}; !reflect.DeepEqual(results[0].Positions, want) {
		t.Fatalf("only title tokens highlight: %v", results[0].Positions)
	}
}

func TestFilterShowsUnavailableOnlyForStrongTitleMatches(t *testing.T) {
	entries := []Entry{
		{Command: Command{ID: "a", Title: "Tail logs"}, Section: SectionView},
		{Command: Command{ID: "b", Title: "Delete image"}, Section: SectionView, Disabled: "Available in ECR › Images"},
	}
	if got := ids(Filter(entries, "dl", nil)); len(got) != 0 {
		t.Fatalf("weak query should hide unavailable entries, got %v", got)
	}
	results := Filter(entries, "del im", nil)
	if len(results) != 1 || results[0].ID != "b" || results[0].Section != SectionUnavailable {
		t.Fatalf("strong query results = %+v", results)
	}
	results = Filter(append(entries, Entry{Command: Command{ID: "c", Title: "Delete everything"}, Section: SectionGlobal}), "del", nil)
	if got := ids(results); !reflect.DeepEqual(got, []string{"c", "b"}) {
		t.Fatalf("available entries must precede unavailable ones: %v", got)
	}
}

func TestFilterExactKeyBoost(t *testing.T) {
	entries := []Entry{
		{Command: Command{ID: "a", Title: "Export CSV"}, Key: "e", Section: SectionView},
		{Command: Command{ID: "b", Title: "Stop task"}, Key: "x", Section: SectionView},
	}
	if got := ids(Filter(entries, "x", nil)); got[0] != "b" {
		t.Fatalf("exact key should rank first: %v", got)
	}
}

func TestRecencyRoundTripAndDecay(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	rec := NewRecency()
	rec.now = func() time.Time { return now }
	rec.Record("a")
	rec.Record("a")
	rec.Record("b")
	if rec.Score("a") <= rec.Score("b") {
		t.Fatal("more frequent command should score higher")
	}
	path := filepath.Join(t.TempDir(), "nested", "recent.json")
	if err := rec.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := LoadRecency(path)
	loaded.now = func() time.Time { return now.Add(48 * time.Hour) }
	if got, want := loaded.Score("a"), 2.0; got != want {
		t.Fatalf("decayed score = %v, want %v", got, want)
	}
	if LoadRecency(filepath.Join(t.TempDir(), "missing.json")).Score("a") != 0 {
		t.Fatal("missing file should load empty")
	}
}

func TestRecencyPrunesLeastUsed(t *testing.T) {
	rec := NewRecency()
	rec.Record("keep")
	rec.Record("keep")
	for i := range maxRecencyEntries + 10 {
		rec.Record(string(rune('a'+i%26)) + string(rune('A'+i/26)))
	}
	if len(rec.entries) > maxRecencyEntries {
		t.Fatalf("entries = %d", len(rec.entries))
	}
	if rec.Score("keep") == 0 {
		t.Fatal("frequent entry was pruned")
	}
}

func ids(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.ID
	}
	return out
}
