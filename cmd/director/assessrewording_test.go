package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE EVALUATION IS THE GATE.
//
// Not a report somebody runs and reads. The counts that must hold — no wrong control, no
// ambiguity answered without asking, no answer changed that the literal reading already gave
// — are asserted here over the committed evaluation sets and the committed corpus, so the
// next change to rewording either keeps them or fails.
//
// It needs no desktop and no bridge: the corpus is what production believed about real
// windows, read by the real accessibility walk and committed after a person reviewed it.

func rewordingFixtures(t *testing.T) ([]rewordSet, map[string]directorapi.WorldState) {
	t.Helper()
	sets, err := loadRewordSets(filepath.Join("..", "..", "fixtures", "rewording"))
	if err != nil {
		t.Fatalf("reading the evaluation sets: %v", err)
	}
	samples, err := loadDesktopSamples(filepath.Join("..", "..", "fixtures", "perception",
		"desktop", "corpus"))
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	worlds := map[string]directorapi.WorldState{}
	for _, s := range samples {
		worlds[s.ID] = desktopWorld(s)
	}
	return sets, worlds
}

func TestRewordingNeverChoosesTheWrongControl(t *testing.T) {
	sets, worlds := rewordingFixtures(t)
	held := false
	for _, set := range sets {
		var out bytes.Buffer
		if assessRewordSet(&out, set, worlds, false, false) != 0 {
			t.Errorf("%s failed its own evaluation:\n%s", set.Application, out.String())
		}
		if set.HeldOut {
			held = true
		}
	}
	// AND ONE OF THEM MUST BE AN APPLICATION NOBODY TUNED AGAINST, because the numbers
	// that count come from there. This repository has shipped a mechanism that worked on
	// the application it was written against eight times.
	if !held {
		t.Error("no held-out evaluation set, so nothing here measures an untuned application")
	}
}

// AND IT ANSWERS THE QUESTIONS IT EXISTS FOR, which a gate asserting only failures would
// not: a rewording that proposed nothing would pass the test above perfectly.
func TestRewordingAnswersWhatTheLiteralReadingCouldNot(t *testing.T) {
	sets, worlds := rewordingFixtures(t)
	live := target.NewResolver()
	literal := target.NewResolver()
	literal.Rephrasing = nil

	gained := 0
	for _, set := range sets {
		for _, c := range set.Cases {
			w, ok := worlds[c.On]
			if !ok || c.Means.Label == "" {
				continue
			}
			verb := c.Verb
			if verb == "" {
				verb = string(directorapi.SemanticOpen)
			}
			q := directorapi.ElementQuery{Label: c.Say}
			lic := directorapi.OperateLicence{Instruction: verb + " " + c.Say, Verb: verb}
			before := literal.ResolveUnder(&w, q, lic, verb)
			after := live.ResolveUnder(&w, q, lic, verb)
			if before.Status == directorapi.ResolutionAbsent &&
				after.Status == directorapi.ResolutionResolved &&
				strings.EqualFold(after.Target.Label, c.Means.Label) {
				gained++
			}
		}
	}
	// Measured at eight across the two sets when this was written — one ampersand case and
	// the ones a person spelled the way they learnt to spell it.
	if gained < 6 {
		t.Errorf("rewording answered only %d requests the literal reading refused; "+
			"it is not earning its place", gained)
	}
}

// AND EVERY REWORDED ANSWER SAYS SO, which is the half of the contract a person relies on.
func TestARewordedAnswerIsVisiblyReworded(t *testing.T) {
	sets, worlds := rewordingFixtures(t)
	r := target.NewResolver()
	checked := 0
	for _, set := range sets {
		for _, c := range set.Cases {
			w, ok := worlds[c.On]
			if !ok {
				continue
			}
			q := directorapi.ElementQuery{Label: c.Say}
			res := r.ResolveUnder(&w, q,
				directorapi.OperateLicence{Instruction: "open " + c.Say, Verb: "open"}, "open")
			if res.Reworded == "" {
				continue
			}
			checked++
			if !strings.Contains(res.Explanation, res.Reworded) {
				t.Errorf("%q: the sentence a person reads does not carry the rewording: %q",
					c.Say, res.Explanation)
			}
			if res.Target == nil {
				continue
			}
			// The query kept with the target is the PERSON'S, not Marco's reading of
			// it. Otherwise the reading becomes the record and the next observation
			// re-finds the control by words nobody said.
			if res.Target.Query == nil || res.Target.Query.Label != c.Say {
				t.Errorf("%q: the target carries %+v instead of what was asked",
					c.Say, res.Target.Query)
			}
			if res.Target.Confidence >= 0.97 {
				t.Errorf("%q: a reworded match reads as confidently as an exact one (%.2f)",
					c.Say, res.Target.Confidence)
			}
		}
	}
	if checked == 0 {
		t.Error("no answer in either set was reached by rewording, so this checked nothing")
	}
}
