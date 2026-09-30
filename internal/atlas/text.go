package atlas

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Norm is the site's own normalisation: Unicode NFD, combining marks
// U+0300 to U+036F dropped, lower case, trimmed, runs of white space
// collapsed. So "Jerusalén", "jerusalen" and "JERUSALEN" compare equal, and
// "ñ" matches "n".
func Norm(s string) string {
	d := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(d))
	space := false
	for _, r := range d {
		if r >= 0x0300 && r <= 0x036F {
			continue
		}
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Words splits a normalised text on spaces and hyphens.
func Words(n string) []string {
	return strings.FieldsFunc(n, func(r rune) bool { return r == ' ' || r == '-' })
}

// tokens splits a normalised text on anything that is not a letter or a
// digit. It serves the summary tier, where punctuation sits next to words.
func tokens(n string) []string {
	return strings.FieldsFunc(n, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// Score tiers, the site's own plus one for summaries.
const (
	ScoreExact   = 100
	ScorePrefix  = 80
	ScoreWord    = 60
	ScoreInside  = 30
	ScoreSummary = 20
)

// ScoreText scores one normalised text against a normalised query.
func ScoreText(nq, n string) int {
	switch {
	case n == "" || nq == "":
		return 0
	case n == nq:
		return ScoreExact
	case strings.HasPrefix(n, nq):
		return ScorePrefix
	}
	for _, w := range Words(n) {
		if strings.HasPrefix(w, nq) {
			return ScoreWord
		}
	}
	if len([]rune(nq)) >= 3 && strings.Contains(n, nq) {
		return ScoreInside
	}
	return 0
}

// summaryMatches reports whether every word of the query starts some word of
// the summary, for a query of 4 or more characters in total.
func summaryMatches(queryWords []string, summaryTokens []string) bool {
	total := 0
	for _, w := range queryWords {
		total += len([]rune(w))
	}
	if total < 4 || len(summaryTokens) == 0 {
		return false
	}
	for _, q := range queryWords {
		found := false
		for _, t := range summaryTokens {
			if strings.HasPrefix(t, q) {
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

// levenshtein is the edit distance between two strings, counted in runes.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
