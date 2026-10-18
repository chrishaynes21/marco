package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHETHER THE RECTANGLE WE MATCHED STILL SAYS WHAT THE CONTROL SAID.
//
//	Marco has learned the word "Settings." It has not yet proved that it uses that word to
//	recognize the Settings button.
//
// Three outcomes, and the owner's rule about two of them is the whole test:
//
//	Importantly, unread is not corroborated, and OCR silence is not a contradiction.

func TestAMatchIsCorroboratedByAWordItShares(t *testing.T) {
	var f observe.FreshWords
	// Four readings of the matched rectangle, three of which say "settings". OCR missed
	// it once, which is the ordinary case: a highlighted entry renders differently.
	f.Saw([]string{"settings"})
	f.Saw([]string{"settings"})
	f.Saw([]string{"general"})
	f.Saw([]string{"settings"})

	got, why := f.Against([]string{"settings"})
	if got != observe.TextCorroborated {
		t.Fatalf("%s — %s", got, why)
	}
	if !strings.Contains(why, `"settings"`) {
		t.Errorf("the reason does not name the word it agreed on: %s", why)
	}
	if f.Looks() != 4 {
		t.Errorf("%d reading(s) folded, want 4", f.Looks())
	}
	// AND THE WORD THAT APPEARED ONCE IS NOT WHAT IT SAYS. TextCommon is half the
	// readings — controlWords' own rule, which exists because one look is the worst
	// sample there is.
	common := f.Common()
	if len(common) != 1 || common[0] != "settings" {
		t.Errorf("it says %v; the once-seen word is this reader's noise", common)
	}
}

// A CONTRADICTION NEEDS BOTH SIDES TO SAY SOMETHING, AND SAYS HOW MUCH EVIDENCE IT HAS.
func TestAContradictionNeedsFreshRepeatedEvidence(t *testing.T) {
	var f observe.FreshWords
	f.Saw([]string{"inventory"})
	f.Saw([]string{"inventory"})
	f.Saw([]string{"inventory"})

	got, why := f.Against([]string{"settings"})
	if got != observe.TextContradicted {
		t.Fatalf("%s — %s", got, why)
	}
	for _, want := range []string{`"settings"`, `"inventory"`, "3 readings"} {
		if !strings.Contains(why, want) {
			t.Errorf("the reason does not say %s: %s", want, why)
		}
	}

	// ONE READING IS STILL A CONTRADICTION — half of one reading is one — and it is
	// reported with its count, which is what a reader needs to discount it. A rule that
	// refused a single sample would need a number of its own, and this file has none.
	var once observe.FreshWords
	once.Saw([]string{"inventory"})
	if got, why := once.Against([]string{"settings"}); got != observe.TextContradicted {
		t.Errorf("one reading: %s — %s", got, why)
	} else if !strings.Contains(why, "1 reading") {
		t.Errorf("a one-sample contradiction does not say so: %s", why)
	}
}

// AND SILENCE IS NEVER A CONTRADICTION. Four ways to be unread, each one the owner named.
func TestSilenceIsUnreadAndNotContradicted(t *testing.T) {
	for _, c := range []struct {
		name  string
		fold  func(*observe.FreshWords)
		saved []string
		why   string
	}{
		{
			name:  "nothing was saved",
			fold:  func(*observe.FreshWords) {},
			saved: nil,
			why:   "nothing to agree with",
		},
		{
			name:  "no reading arrived",
			fold:  func(*observe.FreshWords) {},
			saved: []string{"settings"},
			why:   "no reading of this rectangle",
		},
		{
			name: "readings arrived and read nothing in it",
			fold: func(f *observe.FreshWords) {
				f.Saw(nil)
				f.Saw([]string{})
			},
			saved: []string{"settings"},
			why:   "no reading of this rectangle",
		},
		{
			name: "no word survived half the readings",
			fold: func(f *observe.FreshWords) {
				f.Saw([]string{"a"})
				f.Saw([]string{"b"})
				f.Saw([]string{"c"})
				f.Saw([]string{"d"})
			},
			saved: []string{"settings"},
			why:   "no word was there on half of them",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var f observe.FreshWords
			c.fold(&f)
			got, why := f.Against(c.saved)
			if got != observe.TextUnread {
				t.Fatalf("%s — %s", got, why)
			}
			if !strings.Contains(why, c.why) {
				t.Errorf("the reason does not say %q: %s", c.why, why)
			}
		})
	}
}

// AND AN EMPTY READING IS NOT A LOOK'S WORTH OF EVIDENCE.
//
// Folding it would push every real word below the share that makes it the control's — so a
// rectangle read four times, twice as "settings" and twice as nothing, still says "settings".
func TestAnEmptyReadingIsNotFolded(t *testing.T) {
	var f observe.FreshWords
	f.Saw([]string{"settings"})
	f.Saw(nil)
	f.Saw([]string{"settings"})
	f.Saw(nil)

	if f.Looks() != 2 {
		t.Errorf("%d reading(s) folded, want the 2 that read something", f.Looks())
	}
	if got, why := f.Against([]string{"settings"}); got != observe.TextCorroborated {
		t.Errorf("%s — %s", got, why)
	}
}

// A WORD TWICE ON ONE LOOK IS ONE LOOK'S EVIDENCE.
//
// `controlWords`' rule, and it matters because a button's label can be read as two overlapping
// boxes. Three readings, so a word needs two: "inventory" has two and "settings" has ONE —
// unless the three copies on one look were counted as three, which would make "settings" what
// the rectangle usually says on the strength of a single look.
func TestAWordTwiceOnOneLookCountsOnce(t *testing.T) {
	var f observe.FreshWords
	f.Saw([]string{"settings", "settings", "settings"})
	f.Saw([]string{"inventory"})
	f.Saw([]string{"inventory"})

	if got := f.Common(); len(got) != 1 || got[0] != "inventory" {
		t.Errorf("it says %v; three copies on one look outvoted two separate looks", got)
	}
	if f.Looks() != 3 {
		t.Errorf("%d reading(s) folded, want 3", f.Looks())
	}
}

// AND AT PANEL SCALE THE WHOLE THING INVERTS.
//
// [[Experiment-082-a-control-that-is-half-the-window]]: a human click in Ark's menu produced a
// control 932x690 pixels across, and `controlWords` read the thirteen words inside it — the
// menu's whole vocabulary, not including the button that was pressed.
//
// `Against` asks `agreedWords`, which needs ONE word in common. So a thirteen-word identity
// agrees with any menu carrying any one of them, and the evidence that refused a different real
// menu in [[Experiment-081-the-first-real-negative]] stops discriminating.
//
// This is not a defect in this file and nothing here should be changed to paper over it: the
// rule is `controlWords`' own, a one-word button is the case it was built for, and a reader of
// `corroborated` has to know it is a statement about the rectangle they were given. It is
// recorded here because the inversion was asserted in an experiment write-up before it was
// measured, and an assertion about one's own code is the cheapest kind to get wrong.
func TestAPanelsVocabularyAgreesWithAlmostAnything(t *testing.T) {
	panel := []string{"ark", "clug", "discord", "exit", "friends", "main", "menu", "mode",
		"on", "photo", "settings", "to", "toggle"}

	// A DIFFERENT MENU ENTIRELY. One word in common — the word that is on every screen
	// of the game with a menu button in the corner — and the verdict is agreement.
	var elsewhere observe.FreshWords
	for i := 0; i < 3; i++ {
		elsewhere.Saw([]string{"inventory", "cryofridge", "20000", "pcs", "exit"})
	}
	got, why := elsewhere.Against(panel)
	if got != observe.TextCorroborated {
		t.Fatalf("%s — %s", got, why)
	}
	if !strings.Contains(why, `"exit"`) {
		t.Errorf("the reason does not name the one word it rested on: %s", why)
	}

	// AND THE SAME READING CONTRADICTS THE BUTTON-SCALE IDENTITY of Experiment 080,
	// which is the comparison that makes the point: one control, one word, and the
	// evidence does its job.
	if got, why := elsewhere.Against([]string{"settings"}); got != observe.TextContradicted {
		t.Errorf("a one-word control was not contradicted by a menu without it: "+
			"%s — %s", got, why)
	}
}
