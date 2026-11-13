package command

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// maxRecencyEntries bounds the persisted history.
const maxRecencyEntries = 200

// RecencyEntry records how often and how recently a command ran.
type RecencyEntry struct {
	Count int       `json:"count"`
	Last  time.Time `json:"last"`
}

// Recency is a frecency store: commands used often and recently rank higher.
// It is safe for concurrent use.
type Recency struct {
	mu      sync.Mutex
	entries map[string]RecencyEntry
	now     func() time.Time
}

// NewRecency returns an empty store.
func NewRecency() *Recency {
	return &Recency{entries: map[string]RecencyEntry{}, now: time.Now}
}

// LoadRecency reads a store from path. A missing or unreadable file yields an
// empty store, since history is a convenience rather than state to protect.
func LoadRecency(path string) *Recency {
	r := NewRecency()
	if path == "" {
		return r
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	var stored map[string]RecencyEntry
	if json.Unmarshal(data, &stored) == nil && stored != nil {
		r.entries = stored
	}
	return r
}

// Save writes the store to path, creating its directory if needed.
func (r *Recency) Save(path string) error {
	if r == nil || path == "" {
		return nil
	}
	r.mu.Lock()
	data, err := json.MarshalIndent(r.entries, "", "  ")
	r.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Record notes that the command with id just ran.
func (r *Recency) Record(id string) {
	if r == nil || id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[id]
	e.Count++
	e.Last = r.now()
	r.entries[id] = e
	r.prune()
}

// Score returns the frecency of id: its use count weighted by how recently it
// was last used. Unknown commands score zero.
func (r *Recency) Score(id string) float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return 0
	}
	return float64(e.Count) * ageWeight(r.now().Sub(e.Last))
}

func ageWeight(age time.Duration) float64 {
	switch {
	case age < time.Hour:
		return 4
	case age < 24*time.Hour:
		return 2
	case age < 7*24*time.Hour:
		return 1
	case age < 30*24*time.Hour:
		return 0.5
	}
	return 0.25
}

func (r *Recency) prune() {
	if len(r.entries) <= maxRecencyEntries {
		return
	}
	type scored struct {
		id    string
		score float64
	}
	all := make([]scored, 0, len(r.entries))
	now := r.now()
	for id, e := range r.entries {
		all = append(all, scored{id, float64(e.Count) * ageWeight(now.Sub(e.Last))})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score < all[j].score })
	for _, s := range all[:len(all)-maxRecencyEntries] {
		delete(r.entries, s.id)
	}
}
