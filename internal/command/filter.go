package command

import (
	"sort"
	"strings"
	"unicode"
)

// Result is an Entry that survived filtering, with its rank and the rune
// positions of Title to highlight.
type Result struct {
	Entry
	Score     int
	Positions []int
}

// MaxRecent caps how many recently used commands lead the browse view.
const MaxRecent = 3

// Relative weights for matches outside the title. Title matches are the only
// ones highlighted, so they rank highest.
const (
	weightTarget   = 8
	weightKeyword  = 7
	weightCategory = 6
	weightScale    = 10

	bonusExactKey    = 40
	maxRecencyBonus  = 30
	recencyBonusUnit = 4
)

var sectionBonus = map[Section]int{
	SectionView:      20,
	SectionSaved:     6,
	SectionPlugins:   4,
	SectionGlobal:    0,
	SectionGoTo:      -4,
	SectionResources: -10,
}

// Filter returns the entries to show for query.
//
// With an empty query it returns every available entry grouped by section in
// a stable order, so the palette doubles as a scannable help screen; the most
// recently used commands are lifted into a leading Recent section. Resource
// entries are left out of this browse view (except as Recent).
//
// With a query, each whitespace-separated token must match the entry's title,
// target, category, keywords, or exact key. Results are ranked by match
// quality, section, and recency. Unavailable entries appear only when every
// token matches the start of a word in their title, so the palette can explain
// why an obvious command cannot run here without cluttering broad searches.
func Filter(entries []Entry, query string, recency *Recency) []Result {
	entries = dedupe(entries)
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return browse(entries, recency)
	}

	results := make([]Result, 0, len(entries))
	for _, e := range entries {
		if !e.Available() && !strongTitleMatch(tokens, e.Title) {
			continue
		}
		score, positions, ok := matchEntry(tokens, e)
		if !ok {
			continue
		}
		if e.Available() {
			score += sectionBonus[e.Section]
			if recency != nil {
				score += min(int(recency.Score(e.ID)*recencyBonusUnit), maxRecencyBonus)
			}
		} else {
			e.Section = SectionUnavailable
		}
		results = append(results, Result{Entry: e, Score: score, Positions: positions})
	}
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Available() != b.Available() {
			return a.Available()
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		return len(a.Title) < len(b.Title)
	})
	return results
}

// dedupe keeps the first entry for each ID so context-specific registrations
// (which frontends list first) win over their global duplicates. Entries
// without an ID are never collapsed.
func dedupe(entries []Entry) []Entry {
	seen := make(map[string]bool, len(entries))
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.ID != "" {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
		}
		out = append(out, e)
	}
	return out
}

func browse(entries []Entry, recency *Recency) []Result {
	recent := map[string]bool{}
	var results []Result
	if recency != nil {
		var candidates []Entry
		for _, e := range entries {
			if e.Available() && e.ID != "" && recency.Score(e.ID) > 0 {
				candidates = append(candidates, e)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return recency.Score(candidates[i].ID) > recency.Score(candidates[j].ID)
		})
		for _, e := range candidates[:min(len(candidates), MaxRecent)] {
			recent[e.ID] = true
			e.Section = SectionRecent
			results = append(results, Result{Entry: e})
		}
	}
	var rest []Result
	for _, e := range entries {
		// Resources can number in the thousands; they are found by search
		// (or listed as Recent), never browsed.
		if !e.Available() || recent[e.ID] || e.Section == SectionResources {
			continue
		}
		rest = append(rest, Result{Entry: e})
	}
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Section < rest[j].Section })
	return append(results, rest...)
}

func matchEntry(tokens []string, e Entry) (int, []int, bool) {
	total := 0
	var positions []int
	for _, tok := range tokens {
		best, bestPositions, matched := 0, []int(nil), false
		if s, p, ok := Match(tok, e.Title); ok {
			best, bestPositions, matched = s, p, true
		}
		consider := func(text string, weight int) {
			if text == "" {
				return
			}
			if s, _, ok := Match(tok, text); ok {
				if s = s * weight / weightScale; !matched || s > best {
					best, bestPositions, matched = s, nil, true
				}
			}
		}
		consider(e.Target, weightTarget)
		for _, kw := range e.Keywords {
			consider(kw, weightKeyword)
		}
		consider(e.Category, weightCategory)
		if e.Key != "" && tok == e.Key {
			if !matched {
				best, matched = 0, true
			}
			best += bonusExactKey
		}
		if !matched {
			return 0, nil, false
		}
		total += best
		positions = append(positions, bestPositions...)
	}
	return total, uniqueSorted(positions), true
}

// strongTitleMatch reports whether every token abbreviates a single word of
// title: the token starts with the word's first letter and its remaining
// letters appear in order within that word ("del", "msg" for "delete
// message"). Scattered matches across words do not count.
func strongTitleMatch(tokens []string, title string) bool {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, tok := range tokens {
		tok = strings.ToLower(tok)
		found := false
		for _, w := range words {
			if abbreviates(tok, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func abbreviates(tok, word string) bool {
	t, w := []rune(tok), []rune(word)
	if len(t) == 0 || len(w) == 0 || t[0] != w[0] {
		return false
	}
	i := 1
	for _, r := range w[1:] {
		if i < len(t) && r == t[i] {
			i++
		}
	}
	return i == len(t)
}

func uniqueSorted(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	sort.Ints(values)
	out := values[:1]
	for _, v := range values[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
