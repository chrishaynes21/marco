// Package nlu resolves free-text assistant input to a known route. It is a
// deterministic fuzzy matcher (normalize → token overlap + edit distance) — no
// network, no model, fully testable. A natural-language model can later be
// layered on as a fallback for loose paraphrase / argument extraction without
// changing callers (see Resolve's contract).
package nlu

import "strings"

// Match is the resolver's verdict for one input line.
type Match struct {
	Route string  // the matched route slug ("" if no candidate)
	Score float64 // 0..1 confidence
	Exact bool    // normalized input equals the route exactly
}

// stop words that add no routing signal. Verbs (start, open, login…) are kept
// because they are often part of the route name.
var stop = map[string]bool{
	"to": true, "into": true, "in": true, "the": true, "a": true, "an": true,
	"of": true, "on": true, "for": true, "please": true, "my": true, "me": true,
}

// Resolve returns the best-matching route for input among the given route slugs.
// Route is "" when nothing is a reasonable candidate.
func Resolve(input string, routes []string) Match {
	ni := normalize(input)
	if ni == "" {
		return Match{}
	}
	best := Match{}
	for _, r := range routes {
		nr := normalize(strings.ReplaceAll(r, "-", " "))
		if nr == "" {
			continue
		}
		if ni == nr {
			return Match{Route: r, Score: 1, Exact: true}
		}
		score := jaccard(ni, nr)
		if lr := levRatio(ni, nr); lr > score {
			score = lr
		}
		if score > best.Score {
			best = Match{Route: r, Score: score}
		}
	}
	return best
}

// normalize lowercases, keeps only alphanumerics/spaces, drops stop words, and
// collapses whitespace.
func normalize(s string) string {
	var toks []string
	for w := range strings.FieldsSeq(strings.ToLower(s)) {
		var b strings.Builder
		for _, r := range w {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		t := b.String()
		if t != "" && !stop[t] {
			toks = append(toks, t)
		}
	}
	return strings.Join(toks, " ")
}

// jaccard is token-set overlap / union.
func jaccard(a, b string) float64 {
	as, bs := toksOf(a), toksOf(b)
	if len(as) == 0 || len(bs) == 0 {
		return 0
	}
	inter := 0
	for t := range as {
		if bs[t] {
			inter++
		}
	}
	union := len(as) + len(bs) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func toksOf(s string) map[string]bool {
	m := map[string]bool{}
	for t := range strings.FieldsSeq(s) {
		m[t] = true
	}
	return m
}

// levRatio is 1 - editDistance/maxLen over the whole normalized strings.
func levRatio(a, b string) float64 {
	if a == "" && b == "" {
		return 1
	}
	d := levenshtein(a, b)
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	if max == 0 {
		return 0
	}
	return 1 - float64(d)/float64(max)
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
