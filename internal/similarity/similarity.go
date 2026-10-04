// Package similarity scores how alike two exercise names are, to suggest
// typo fixes and merges. Pure, no I/O.
package similarity

import (
	"sort"
	"strings"

	"gymbro/internal/workout"
)

// Threshold is the minimum Score for two names to count as similar. See
// similarity_test.go for the pairs that fix its value.
const Threshold = 0.7

// Match is a candidate name with its score against the query.
type Match struct {
	Name  string
	Score float64
}

// Score returns similarity in [0,1] after workout.NameKey normalization:
// 1 for identical keys, otherwise the larger of the character-level
// Damerau-Levenshtein similarity and the same measure over alphabetically
// sorted words (so word order does not matter).
func Score(a, b string) float64 {
	ka, kb := workout.NameKey(a), workout.NameKey(b)
	if ka == kb {
		return 1
	}
	return max(editSimilarity(ka, kb), editSimilarity(sortWords(ka), sortWords(kb)))
}

// Rank returns the candidates scoring at or above Threshold against name,
// best first (ties keep input order), at most limit of them. limit <= 0
// means no limit.
func Rank(name string, candidates []string, limit int) []Match {
	var out []Match
	for _, c := range candidates {
		if s := Score(name, c); s >= Threshold {
			out = append(out, Match{Name: c, Score: s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortWords(s string) string {
	w := strings.Fields(s)
	sort.Strings(w)
	return strings.Join(w, " ")
}

// editSimilarity is 1 - d/max(len) over runes, d being the optimal string
// alignment distance (insert, delete, substitute, adjacent transposition).
func editSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	n := max(len(ra), len(rb))
	if n == 0 {
		return 1
	}
	return 1 - float64(osa(ra, rb))/float64(n)
}

func osa(a, b []rune) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}
