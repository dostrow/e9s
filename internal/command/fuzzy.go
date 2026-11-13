package command

import (
	"unicode"
)

// Scoring constants. Matches on word boundaries and consecutive runs score
// highest so "fnd" prefers "Force New Deployment" over "found".
const (
	scoreMatch         = 16
	bonusFirstChar     = 12
	bonusBoundary      = 10
	bonusCamel         = 8
	bonusDigit         = 4
	bonusConsecutive   = 8
	penaltyGapStart    = 3
	penaltyGapExtend   = 1
	penaltyLeadingChar = 1
	maxLeadingPenalty  = 6
)

const negInf = -1 << 30

// Match fuzzily matches query against text, case-insensitively. Every rune
// of query must appear in text in order. It returns the alignment score, the
// rune indices of text that matched (for highlighting), and whether a match
// was found. An empty query matches with score zero.
func Match(query, text string) (int, []int, bool) {
	q := []rune(query)
	if len(q) == 0 {
		return 0, nil, true
	}
	t := []rune(text)
	n, m := len(t), len(q)
	if m > n {
		return 0, nil, false
	}
	lq := make([]rune, m)
	for i, r := range q {
		lq[i] = unicode.ToLower(r)
	}
	lt := make([]rune, n)
	for i, r := range t {
		lt[i] = unicode.ToLower(r)
	}

	// Quick rejection: query must be a subsequence of text.
	qi := 0
	for _, r := range lt {
		if qi < m && r == lq[qi] {
			qi++
		}
	}
	if qi < m {
		return 0, nil, false
	}

	bonus := make([]int, n)
	for j := range t {
		bonus[j] = positionBonus(t, j)
	}

	// score[i][j]: best score with query rune i matched at text rune j.
	// from[i][j]: text index where query rune i-1 matched on that path.
	score := make([][]int, m)
	from := make([][]int, m)
	for i := range m {
		score[i] = make([]int, n)
		from[i] = make([]int, n)
		for j := range n {
			score[i][j] = negInf
			from[i][j] = -1
		}
	}
	for j := range n {
		if lt[j] == lq[0] {
			lead := min(j*penaltyLeadingChar, maxLeadingPenalty)
			score[0][j] = scoreMatch + bonus[j] - lead
		}
	}
	for i := 1; i < m; i++ {
		// best over k <= j-2 of score[i-1][k] + k*penaltyGapExtend, so the
		// gap transition stays O(1) per cell.
		bestGap, bestGapK := negInf, -1
		for j := 1; j < n; j++ {
			if k := j - 2; k >= 0 && score[i-1][k] > negInf {
				if v := score[i-1][k] + k*penaltyGapExtend; v > bestGap {
					bestGap, bestGapK = v, k
				}
			}
			if lt[j] != lq[i] {
				continue
			}
			best, bestFrom := negInf, -1
			if prev := score[i-1][j-1]; prev > negInf {
				best, bestFrom = prev+bonusConsecutive, j-1
			}
			if bestGapK >= 0 {
				v := bestGap - penaltyGapStart - (j-1)*penaltyGapExtend
				if v > best {
					best, bestFrom = v, bestGapK
				}
			}
			if bestFrom >= 0 {
				score[i][j] = best + scoreMatch + bonus[j]
				from[i][j] = bestFrom
			}
		}
	}

	end, total := -1, negInf
	for j := range n {
		if score[m-1][j] > total {
			total, end = score[m-1][j], j
		}
	}
	if end < 0 {
		return 0, nil, false
	}
	positions := make([]int, m)
	for i, j := m-1, end; i >= 0; i-- {
		positions[i] = j
		j = from[i][j]
	}
	return total, positions, true
}

func positionBonus(t []rune, j int) int {
	if j == 0 {
		return bonusFirstChar
	}
	prev, cur := t[j-1], t[j]
	switch {
	case isSeparator(prev):
		return bonusBoundary
	case unicode.IsLower(prev) && unicode.IsUpper(cur):
		return bonusCamel
	case unicode.IsLetter(prev) && unicode.IsDigit(cur):
		return bonusDigit
	}
	return 0
}

func isSeparator(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '-', '_', '/', '.', ':', '·', '(', ')', '[', ']', ',', '$', '+', '&', '@':
		return true
	}
	return false
}
