package main

// THE WORDS THE WIDGET EXPECTS, AND HOW THEY CORRECT WHAT WAS HEARD.
//
// The small Vosk model hears open English and knows nothing about the routines somebody has: "cloud
// pull stone" came back as "cloud paul stone" live and "cloud poll stone" from synthetic speech. So a
// SECOND recogniser listens to the same audio restricted to a phrase list the widget writes (routine
// names, the widget's own words, the wake word, stop and the "why" questions) plus "[unk]" for
// anything else. Its words win where it recognised one; every "[unk]" stretch is filled from the open
// transcript over the same span of time. So a routine name snaps to its spelling and a value or a
// free request ("open the settings menu please") keeps the open recogniser's words.
//
// Pure Go: the recognisers are cgo (voice_cgo.go), and this is what is tested without them.

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// unk is Vosk's word for "none of the grammar's words".
const unk = "[unk]"

// word is one word of a Vosk result with its timing, in seconds since the stream began.
type word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// resultWords reads the words of a Vosk result object (the recogniser must have SetWords on).
func resultWords(s string) []word {
	var r struct {
		Result []word `json:"result"`
	}
	if json.Unmarshal([]byte(s), &r) != nil {
		return nil
	}
	return r.Result
}

// merge is the phrase list's words where it recognised one, and the open transcript's elsewhere.
//
// An open word is REPLACED by the phrase-list words that cover it — each overlapping it by more
// than half of the shorter of the two — so "poll" becomes "pull" and one open word "opensocial"
// becomes "open social". Every other open word, inside an "[unk]" or in a gap, is kept. A
// phrase-list word that covers no open word is NOT added: the open recogniser is what says
// something was said, and a list word with nothing under it is the list hearing noise.
// With no phrase-list words it is the open transcript exactly.
func merge(open, listed []word) string {
	var out []string
	used := make([]bool, len(listed))
	for _, o := range open {
		covered := false
		for i, g := range listed {
			if g.Word == unk || !covers(o, g) {
				continue
			}
			covered = true
			if !used[i] { // a list word covering two open words is said once
				out = append(out, g.Word)
				used[i] = true
			}
		}
		if !covered {
			out = append(out, o.Word)
		}
	}
	return strings.Join(out, " ")
}

// covers is whether two words are the same stretch of sound: they overlap by more than half of
// the shorter one.
func covers(a, b word) bool {
	lo, hi := max(a.Start, b.Start), min(a.End, b.End)
	short := min(a.End-a.Start, b.End-b.Start)
	return hi > lo && short > 0 && hi-lo > short/2
}

// listedPhrases is the set of whole phrases a grammar holds.
type listedPhrases map[string]bool

// grammarOf is the phrase list as a Vosk grammar: every phrase whose words the model knows, and
// "[unk]". known is the model's vocabulary check (vosk FindWord); a phrase with a word it does not
// know is left out, because a part of a name is not a name (see whole).
func grammarOf(phrases []string, known func(string) bool) (string, listedPhrases) {
	set := listedPhrases{}
	var out []string
	for _, p := range phrases {
		ws := strings.Fields(spoken(p))
		all := len(ws) > 0
		for _, w := range ws {
			all = all && known(w)
		}
		if s := strings.Join(ws, " "); all && !set[s] {
			set[s] = true
			out = append(out, s)
		}
	}
	b, _ := json.Marshal(append(out, unk))
	return string(b), set
}

// whole keeps the phrase list's words only where they make up one of its phrases, consecutively,
// and makes every other one "[unk]". Vosk's grammar lets any sequence of the listed words through,
// so a lone "pull" can be the list forcing its nearest word onto something else: measured on the
// real model, "black pearl" came back "black pull". The longest phrase at each place wins.
func whole(listed []word, set listedPhrases) []word {
	out := make([]word, len(listed))
	copy(out, listed)
	keep := make([]bool, len(listed))
	for i := 0; i < len(listed); {
		matched := 0
		for j := len(listed); j > i; j-- {
			ws := make([]string, 0, j-i)
			for _, w := range listed[i:j] {
				ws = append(ws, w.Word)
			}
			if set[strings.Join(ws, " ")] {
				matched = j - i
				break
			}
		}
		if matched == 0 {
			i++
			continue
		}
		for k := i; k < i+matched; k++ {
			keep[k] = true
		}
		i += matched
	}
	for i := range out {
		if !keep[i] {
			out[i].Word = unk
		}
	}
	return out
}

// spoken is a phrase the way the recogniser writes words: lower case, letters, digits and
// apostrophes, with anything else a space ("cloud-pull" is "cloud pull").
func spoken(p string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '\'':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		}
		return ' '
	}, p)
}

// phraseFile is the widget's list, re-read when it changes.
type phraseFile struct {
	path string
	mod  time.Time
}

// changed reads the list when the file is newer than the last read. ok is false when it has not
// changed or cannot be read; a missing file is an empty list.
func (f *phraseFile) changed() ([]string, bool) {
	if f.path == "" {
		return nil, false
	}
	st, err := os.Stat(f.path)
	if err != nil {
		if !f.mod.IsZero() {
			f.mod = time.Time{}
			return nil, true
		}
		return nil, false
	}
	if !st.ModTime().After(f.mod) {
		return nil, false
	}
	b, err := os.ReadFile(f.path)
	if err != nil {
		return nil, false
	}
	var ps []string
	if json.Unmarshal(b, &ps) != nil {
		return nil, false
	}
	f.mod = st.ModTime()
	return ps, true
}
