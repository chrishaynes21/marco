package observe_test

// Whether a region matched again is the control it was matched to.
//
// The measured case these are built from: Rocket League's main menu is bottom-anchored, and
// signed out it loses two entries, so PLAY sat 95px below the rectangle the demonstration
// recorded. A proposer boxed a patch of pitch inside the old rectangle, re-acquisition reported
// MATCHED, and the step pressed grass while the word PLAY was read on the same look.
//
// So what is held here is one asymmetry: text may REFUSE a match and may never propose one, and
// the refusal is a competing candidate rather than a silence.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// the measured geometry, in the window pixels `rect` takes
var (
	// grass is the 128x24 box a proposer drew inside the recorded rectangle,
	// and the point the live run pressed.
	grass = rect(100, 478, 128, 24)
	// play is where PLAY actually was, 95px lower, bottom-anchored.
	play = rect(70, 565, 300, 40)
	// garage is the row under it.
	garage = rect(70, 613, 300, 40)
)

// reads places one word inside a region.
func reads(text string, r observe.Region) observe.ReadWord {
	return observe.ReadWord{Text: text, Where: observe.Region{
		X: r.X + r.Width/4, Y: r.Y + r.Height/4,
		Width: r.Width / 2, Height: r.Height / 2,
	}}
}

// EXPERIMENT-044: THE MATCH WAS GEOMETRY AND THE WORD WAS SOMEWHERE ELSE.
func TestAMatchOnBackgroundIsContradictedByTheLabelElsewhere(t *testing.T) {
	got := observe.ControlIdentityOf(grass,
		[]observe.Region{grass, play, garage},
		[]observe.ReadWord{reads("PLAY", play), reads("GARAGE", garage)},
		observe.Words("enter free play"))

	if !got.Contradicted() {
		t.Fatalf("%s — %s", got.Result, got.Because)
	}
	if len(got.Elsewhere) != 1 || got.Elsewhere[0].Region != play {
		t.Fatalf("the competing candidate is %+v, want the region reading PLAY",
			got.Elsewhere)
	}
	// THE REFUSAL SAYS BOTH HALVES, or nobody can weigh it.
	for _, want := range []string{"nothing was read inside it", "PLAY"} {
		if !strings.Contains(got.Because, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got.Because)
		}
	}
}

// AND AN UNLABELLED CONTROL IS NOT CONTRADICTED BY ITS OWN SILENCE.
//
// The rule that keeps this usable at all. A control may be an icon, its label may be drawn
// outside its clickable rectangle, or OCR may simply have missed it. Missing evidence is not
// contradictory evidence, and a rule that treated it as such would make every unlabelled control
// in every application unpressable on a bad frame.
func TestAnUnlabelledControlIsNotContradictedByItsOwnSilence(t *testing.T) {
	got := observe.ControlIdentityOf(grass,
		[]observe.Region{grass, play, garage},
		// Words on the screen, none of them this control's and none of them PLAY.
		[]observe.ReadWord{reads("GARAGE", garage)},
		observe.Words("enter free play"))

	if got.Contradicted() {
		t.Fatalf("an unlabelled match was refused: %s", got.Because)
	}
	if got.Result != observe.IdentityUnexamined {
		t.Errorf("%s, want the question to have had no answer", got.Result)
	}
	if !strings.Contains(got.Because, "no competing candidate") {
		t.Errorf("the verdict does not say why it concluded nothing:\n%s", got.Because)
	}
}

// AND NEITHER IS A LOOK WITH NO WORDS ON IT AT ALL.
//
// A run with no reader, or a reading that failed. Distinct from the case above and reported
// differently: one says OCR could not answer, the other that it answered and found nothing that
// bears on this control.
func TestWithNoWordsAtAllNothingIsContradicted(t *testing.T) {
	got := observe.ControlIdentityOf(grass, []observe.Region{grass, play}, nil,
		observe.Words("enter free play"))

	if got.Contradicted() {
		t.Fatalf("a look with no words refused a match: %s", got.Because)
	}
	if !strings.Contains(got.Because, "missing evidence is not contradictory evidence") {
		t.Errorf("the verdict does not say which silence this is:\n%s", got.Because)
	}
}

// AND NEITHER IS A CONTROL NOTHING SAYS ANYTHING ABOUT.
func TestWithNothingRecordedToMatchAgainstTheQuestionIsNotAsked(t *testing.T) {
	got := observe.ControlIdentityOf(grass, []observe.Region{grass, play},
		[]observe.ReadWord{reads("PLAY", play)}, nil)

	if got.Result != observe.IdentityUnexamined {
		t.Fatalf("%s — %s", got.Result, got.Because)
	}
	if !strings.Contains(got.Because, "nothing recorded says") {
		t.Errorf("the verdict does not say what was missing:\n%s", got.Because)
	}
}

// A MATCH THAT SAYS WHAT IT SHOULD IS CORROBORATED, AND NOTHING TAKES THAT AWAY.
//
// Asked before the competing candidates on purpose. Two rows of one menu can both be named in a
// workflow called "enter free play" — the FREE PLAY tile and a FREE PLAY heading above it — and
// a rule that refused whenever anything else also qualified would refuse the right answer for
// having company.
func TestAMatchThatSaysWhatItShouldIsCorroboratedEvenWithCompany(t *testing.T) {
	for _, tc := range []struct {
		name  string
		words []observe.ReadWord
	}{
		{"alone", []observe.ReadWord{reads("PLAY", play)}},
		{"with another candidate saying the same", []observe.ReadWord{
			reads("PLAY", play), reads("PLAY", garage)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := observe.ControlIdentityOf(play,
				[]observe.Region{grass, play, garage}, tc.words,
				observe.Words("enter free play"))

			if got.Result != observe.IdentityCorroborated {
				t.Fatalf("%s — %s", got.Result, got.Because)
			}
			if got.Said != "PLAY" {
				t.Errorf("it was read as %q", got.Said)
			}
		})
	}
}

// TWO CANDIDATES NAMED THE SAME ARE REPORTED, AND NOT CHOSEN BETWEEN.
//
// The match is still refused — whatever is right, the matched region is not it — but nothing
// here picks the replacement. That is the repair's question, and it refuses ambiguity too.
func TestTwoCandidatesNamedTheSameAreReportedAndNotChosenBetween(t *testing.T) {
	got := observe.ControlIdentityOf(grass,
		[]observe.Region{grass, play, garage},
		[]observe.ReadWord{reads("PLAY", play), reads("PLAY", garage)},
		observe.Words("enter free play"))

	if !got.Contradicted() {
		t.Fatalf("%s — %s", got.Result, got.Because)
	}
	if len(got.Elsewhere) != 2 {
		t.Fatalf("%d competing candidate(s), want both reported", len(got.Elsewhere))
	}
	if !strings.Contains(got.Because, "nothing chooses between them") {
		t.Errorf("the refusal reads as though it had an answer:\n%s", got.Because)
	}
}

// A BAND ACROSS THE WINDOW IS NOT A COMPETING CANDIDATE, HOWEVER WELL ITS TEXT MATCHES.
//
// It locates nothing, so a word inside it says nothing about where anything is — and letting one
// contradict a match would refuse a good target on the authority of a region that could never
// replace it.
func TestABandIsNotACompetingCandidate(t *testing.T) {
	band := rect(0, 432, 1920, 324)
	got := observe.ControlIdentityOf(grass, []observe.Region{grass, band},
		[]observe.ReadWord{reads("PLAY", band)},
		observe.Words("enter free play"))

	if got.Contradicted() {
		t.Fatalf("a band refused a match: %s", got.Because)
	}
}

// THE SMALLEST CONTAINING CANDIDATE TAKES THE WORD.
//
// A tile and the panel behind it must not both claim one label: a panel that inherited every
// word on the screen would be named as every control at once.
func TestTheSmallestContainingCandidateTakesTheWord(t *testing.T) {
	panel := rect(40, 540, 576, 216)
	got := observe.NameCandidates([]observe.Region{panel, play, garage},
		[]observe.ReadWord{reads("PLAY", play), reads("GARAGE", garage)})

	for _, n := range got {
		switch n.Region {
		case play:
			if n.Text != "PLAY" {
				t.Errorf("the PLAY row reads %q", n.Text)
			}
		case garage:
			if n.Text != "GARAGE" {
				t.Errorf("the GARAGE row reads %q", n.Text)
			}
		case panel:
			if n.Text != "" {
				t.Errorf("the panel behind them claimed %q", n.Text)
			}
		}
	}
}
