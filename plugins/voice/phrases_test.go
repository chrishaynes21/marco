package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func w(s string, start, end float64) word { return word{Word: s, Start: start, End: end} }

// THE PHRASE LIST CORRECTS A NAME AND LEAVES EVERYTHING ELSE TO THE OPEN TRANSCRIPT. Timings are
// the shapes measured from the real model on synthetic speech ("cloud poll stone", "opensocial",
// "open [unk] [unk]").
func TestThePhraseListCorrectsOnlyWhatItRecognised(t *testing.T) {
	for _, c := range []struct {
		name         string
		open, listed []word
		want         string
	}{
		{"a misheard name, and the value kept",
			[]word{w("cloud", .09, .45), w("poll", .45, .69), w("stone", .69, 1.17)},
			[]word{w("cloud", .09, .45), w("pull", .45, .69), w(unk, .69, 1.17)},
			"cloud pull stone"},
		{"one open word that is two listed words",
			[]word{w("opensocial", .10, .90)},
			[]word{w("open", .10, .42), w("social", .42, .90)},
			"open social"},
		{"a free request keeps its words",
			[]word{w("open", .1, .4), w("the", .4, .5), w("settings", .5, 1), w("menu", 1, 1.3), w("please", 1.3, 1.7)},
			[]word{w("open", .1, .4), w(unk, .4, 1.0), w(unk, 1.0, 1.7)},
			"open the settings menu please"},
		{"a listed word over silence is not added",
			[]word{w("hello", .1, .5)},
			[]word{w(unk, .1, .5), w("stop", 2, 2.4)},
			"hello"},
		{"nothing heard is nothing, whatever the list says",
			nil, []word{w("stop", .1, .4)}, ""},
		{"no list is the open transcript exactly",
			[]word{w("cloud", 0, .3), w("paul", .3, .6)}, nil, "cloud paul"},
		{"a listed word under two open words is said once",
			[]word{w("clow", .1, .25), w("d", .25, .4)},
			[]word{w("cloud", .1, .4)},
			"cloud"},
	} {
		if got := merge(c.open, c.listed); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// The grammar holds only whole phrases whose every word the model knows, and "[unk]" so that
// anything else is not forced onto a listed word.
func TestTheGrammarHoldsOnlyWordsTheModelKnows(t *testing.T) {
	known := func(s string) bool { return s != "zqx" }
	g, set := grammarOf([]string{"Cloud-Pull", "open zqx", "why didn't that work", "cloud pull"}, known)
	if g != `["cloud pull","why didn't that work","[unk]"]` {
		t.Errorf("grammar %s", g)
	}
	if len(set) != 2 || !set["cloud pull"] || set["open"] {
		t.Errorf("phrases %v", set)
	}
}

// A listed word counts only inside a whole listed phrase: "black pearl" came back "black pull"
// from the real model when a lone "pull" was allowed to correct it.
func TestOnlyAWholePhraseCorrectsWhatWasHeard(t *testing.T) {
	set := listedPhrases{"cloud pull": true, "computer": true}
	open := []word{w("computer", 0, .5), w("cloud", .5, .8), w("pull", .8, 1), w("black", 1, 1.3), w("pearl", 1.3, 1.6)}
	listed := []word{w("computer", 0, .5), w("cloud", .5, .8), w("pull", .8, 1), w(unk, 1, 1.3), w("pull", 1.3, 1.6)}
	if got := merge(open, whole(listed, set)); got != "computer cloud pull black pearl" {
		t.Errorf("got %q", got)
	}
	if got := merge([]word{w("cloud", 0, .3), w("poll", .3, .6)}, whole([]word{w("cloud", 0, .3), w("pull", .3, .6)}, set)); got != "cloud pull" {
		t.Errorf("a whole phrase did not correct: %q", got)
	}
}

// The list is re-read when the widget rewrites it, and only then.
func TestThePhraseListIsReReadWhenItChanges(t *testing.T) {
	p := filepath.Join(t.TempDir(), "phrases.json")
	f := &phraseFile{path: p}
	if _, ok := f.changed(); ok {
		t.Fatal("a missing list changed")
	}
	if err := os.WriteFile(p, []byte(`["cloud pull"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if ps, ok := f.changed(); !ok || len(ps) != 1 || ps[0] != "cloud pull" {
		t.Fatalf("the list was not read: %q %v", ps, ok)
	}
	if _, ok := f.changed(); ok {
		t.Fatal("an unchanged list changed")
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(p, []byte(`["cloud pull","open social"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(p, later, later)
	if ps, ok := f.changed(); !ok || len(ps) != 2 {
		t.Fatalf("a rewritten list was not re-read: %q %v", ps, ok)
	}
}
