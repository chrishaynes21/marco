package rephrase

import (
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// vocabulary is every word and every label the Stage is currently showing.
//
// It is the whole of what rewording is allowed to know. A word this does not contain cannot
// be substituted into a request and a label this does not contain cannot be proposed, which
// is the mechanical form of "it cannot name a control that was not observed": there is
// nowhere else for a word to come from.
//
// Built over the WHOLE world rather than over the query's scope, deliberately. A label read
// off another window would be filtered out by the resolver's own scoping the moment the
// attempt is retried, so the worst an out-of-scope word can do is cost one attempt that
// finds nothing — and duplicating the scope rule here would be a second copy of it, which
// is how two definitions of the same thing drift apart.
type vocabulary struct {
	// words is every distinct word, normalised, that appears in a label.
	words map[string]bool
	// labels maps a normalised label to the text as the interface writes it, which is
	// what a proposed query carries: the control's own spelling, not the person's.
	labels map[string]string
	// labelWords is the content words of each normalised label.
	labelWords map[string][]string
}

func vocabularyOf(w *directorapi.WorldState) vocabulary {
	v := vocabulary{
		words:      map[string]bool{},
		labels:     map[string]string{},
		labelWords: map[string][]string{},
	}
	for _, el := range world.Elements(w) {
		if el.Label == "" {
			continue
		}
		words := contentWords(el.Label)
		if len(words) == 0 {
			continue
		}
		key := strings.Join(words, " ")
		if _, seen := v.labels[key]; !seen {
			v.labels[key] = el.Label
			v.labelWords[key] = words
		}
		for _, word := range words {
			v.words[word] = true
		}
	}
	return v
}

// contentWords is the words of a label, lowercased, with punctuation and the wrapping words
// set aside — the same reduction the request goes through, so the two are comparable.
func contentWords(s string) []string {
	var out []string
	for _, raw := range strings.Fields(strings.ToLower(s)) {
		w := strings.Trim(raw, ".,;:!?\"'()[]{}")
		w = strings.Trim(w, "“”‘’")
		if w == "" || functionWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// shows reports whether the screen is currently showing this word.
func (v vocabulary) shows(word string) bool { return v.words[word] }

// respell returns the observed word a request word was probably a spelling of.
//
// REFUSED WHENEVER MORE THAN ONE IS EQUALLY CLOSE. "personalisation" has exactly one
// neighbour on a Settings page and repairing it is a correction; a word with two neighbours
// has no correction, only a choice, and making that choice silently is the behaviour this
// whole package is meant to avoid. The second return value is the difference between a
// repair and a guess.
//
// Bounded at one edit and at seven letters, because below that the distance stops meaning
// "a spelling of the same word": "tab" and "tap", "copy" and "cope", "save" and "safe" are
// all one edit apart and none of them is a misspelling of the other.
func (v vocabulary) respell(word string) (string, bool) {
	if len(word) < minRespellLength || v.words[word] {
		return "", false
	}
	var found string
	for _, candidate := range sortedWords(v.words) {
		if abs(len(candidate)-len(word)) > 1 || !withinOneEdit(candidate, word) {
			continue
		}
		if found != "" && found != candidate {
			// Two observed words are equally close. No repair.
			return "", false
		}
		found = candidate
	}
	if found == "" {
		return "", false
	}
	return found, true
}

const minRespellLength = 7

// soleLabelContaining names the one observed label that contains all of the request's
// remaining words, or nothing when no label does or several do.
//
// Containment rather than equality, because the words a person keeps are usually a subset
// of what the control is called: "bluetooth devices" is inside "Bluetooth & devices", and
// "explorer" is inside "Explorer (Ctrl+Shift+E)". Order is not required — a person who says
// "devices bluetooth" means the same control — and that is safe here only because the
// answer must be UNIQUE: a set of words that picks out two different labels picks out
// neither.
func (v vocabulary) soleLabelContaining(words []string) (label, because string) {
	if len(words) == 0 {
		return "", ""
	}
	var found string
	for _, key := range sortedKeys(v.labels) {
		if !containsAll(v.labelWords[key], words) {
			continue
		}
		if found != "" {
			// Several labels contain these words. Rewording has no way to choose
			// between them that is not a preference, so it does not choose.
			return "", ""
		}
		found = key
	}
	if found == "" {
		return "", ""
	}
	text := v.labels[found]
	if strings.EqualFold(strings.Join(words, " "), found) {
		// The reduced request already IS the label's content words. Proposing the
		// label adds the interface's own punctuation and nothing else, which is not
		// worth a sentence of its own.
		return text, ""
	}
	return text, "which is the only thing on screen called \"" + text + "\""
}

func containsAll(have, want []string) bool {
	index := make(map[string]bool, len(have))
	for _, w := range have {
		index[w] = true
	}
	for _, w := range want {
		if !index[w] {
			return false
		}
	}
	return true
}

// withinOneEdit reports whether a and b differ by at most one insertion, deletion or
// substitution. Written out rather than taken from a distance matrix because one edit is the
// only distance this package is allowed to cross, and a general distance would invite a
// second caller to raise the bound.
func withinOneEdit(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) < len(rb) {
		ra, rb = rb, ra
	}
	if len(ra)-len(rb) > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < len(ra) && j < len(rb) {
		if ra[i] == rb[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		if len(ra) == len(rb) {
			i++
			j++
			continue
		}
		i++
	}
	if i < len(ra) || j < len(rb) {
		edits++
	}
	return edits <= 1
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// sortedKeys and sortedWords exist for the determinism the evaluation depends on: two runs
// over the same world must propose the same attempts in the same order, and a map range
// would not.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedWords(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
