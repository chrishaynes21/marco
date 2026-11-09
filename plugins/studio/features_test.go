package main

// WHAT THE ALPHA OFFERS, AND WHAT THIS WINDOW CAN EXPLAIN ABOUT ITSELF.
//
// Two claims, and the second is the one with a defect behind it. Help is not decoration here:
// the audit found four abilities that were COMPLETE AND INVISIBLE, and two of the sentences that
// make them findable name a key that is READ FROM THE ENVIRONMENT. This repository has already
// shipped the version of that bug where a page answered from the raw variable — so the default
// installation, which is every installation, was told there was no shortcut. These hold the line
// at the only place that can hold it: the sentence, with the override set to something else.
//
// See [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]].

import (
	"os"
	"strings"
	"testing"
)

// THE RAIL IS THE ONE JOURNEY, WHAT IT DID, HELP, AND THE WIDGET'S SETTINGS.
//
// NOTHING IS SET, which is the state that matters: a flag whose ABSENCE turned a surface on
// would be a surface nobody chose. Acts and Scenes keep every capability they had — this is about
// the way in, and the way in is the whole of what an alpha can promise.
func TestTheAlphaRailOffersOneJourney(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_STUDIO_ACTS", "")
	t.Setenv("MARCO_STUDIO_SCENES", "")
	a := newActs(t.TempDir())

	f := a.Features()
	if f.Acts || f.Scenes {
		t.Fatalf("a window with nothing set offers acts=%v scenes=%v", f.Acts, f.Scenes)
	}
	if got := strings.Join(f.Views, " "); got != "routines history help widget" {
		t.Fatalf("the rail is %q, and the alpha journey is Plays", got)
	}
}

// AND EITHER ONE COMES BACK WHEN IT IS ASKED FOR, IN ITS PLACE IN THE RAIL.
//
// Several spellings on purpose: this is set by hand in a shell and not by a form, and a flag that
// quietly ignores `true` is a flag somebody will believe they turned on.
func TestAFlagPutsAWithheldViewBackInTheRail(t *testing.T) {
	for _, on := range []string{"1", "on", "true", "yes", "YES"} {
		t.Run(on, func(t *testing.T) {
			t.Setenv("MARCO_CV", "off")
			t.Setenv("MARCO_ROUTES", t.TempDir())
			t.Setenv("MARCO_STUDIO_ACTS", on)
			t.Setenv("MARCO_STUDIO_SCENES", on)
			f := newActs(t.TempDir()).Features()
			if !f.Acts || !f.Scenes {
				t.Fatalf("%q turned nothing on: %+v", on, f)
			}
			// IN THE ORDER THE RAIL READ BEFORE THIS, plus Help — so turning
			// both on gives back exactly the window that shipped, rather than
			// a rearranged one.
			if got := strings.Join(f.Views, " "); got !=
				"routines acts scenes history help widget" {
				t.Fatalf("the rail is %q", got)
			}
		})
	}
	for _, off := range []string{"", "0", "off", "no", "maybe", " "} {
		t.Setenv("MARCO_CV", "off")
		t.Setenv("MARCO_ROUTES", t.TempDir())
		t.Setenv("MARCO_STUDIO_ACTS", off)
		if newActs(t.TempDir()).Features().Acts {
			t.Errorf("%q turned Acts on", off)
		}
	}
}

// AND THE FLAGS ARE READ WHEN THE WINDOW OPENS, NOT WHILE SOMEBODY IS USING IT.
//
// The ruling was "read once at Studio startup", and the reason is not tidiness: a rail that could
// change between two polls is a rail that moves under a cursor. Editing the variable in the shell
// that launched this window must do nothing until it is opened again.
func TestTheFlagsAreReadWhenTheWindowOpensAndNotAfter(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_STUDIO_ACTS", "")
	a := newActs(t.TempDir())
	if a.Features().Acts {
		t.Fatal("Acts was on before the variable was set at all")
	}

	// THE VARIABLE CHANGES UNDER THE OPEN WINDOW.
	if err := os.Setenv("MARCO_STUDIO_ACTS", "1"); err != nil {
		t.Fatal(err)
	}
	if a.Features().Acts {
		t.Fatal("the rail changed under an open window, so what it offers is " +
			"whatever the environment said at the moment of the last poll")
	}
	// AND THE NEXT WINDOW HAS IT.
	if !newActs(t.TempDir()).Features().Acts {
		t.Fatal("a window opened after the variable was set does not offer Acts, " +
			"so the flag cannot be used at all")
	}
}

// ── WHAT THE WINDOW EXPLAINS ──────────────────────────────────────────────────

// HELP NAMES THE SHORTCUTS THIS INSTALLATION IS ACTUALLY USING.
//
// The overrides are set to something OTHER than the defaults, which is the only arrangement that
// can tell a composed sentence from a written-down one: a page holding the literal F12 passes
// every test run on a machine that never set the variable.
func TestHelpNamesTheConfiguredShortcuts(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_STOP_KEY", "esc")
	t.Setenv("MARCO_ARG_KEY", "f8")
	h := newActs(t.TempDir()).Help()

	all := helpText(h)
	for _, want := range []string{"Esc", "F8"} {
		if !strings.Contains(all, want) {
			t.Errorf("Help does not name the configured key %q:\n%s", want, all)
		}
	}
	for _, stale := range []string{"F12", "F9"} {
		if strings.Contains(all, stale) {
			t.Errorf("Help names %q, which this installation does not use — so the "+
				"sentence is written into the window rather than composed "+
				"from the setting:\n%s", stale, all)
		}
	}
}

// AND IT SAYS WHAT THE RECORDER ACTUALLY DOES.
//
// THE CLAIM THE ARCHITECT CORRECTED, kept here so it cannot come back. Marco records ONE window
// and THE WINDOW IS CHOSEN — `Deps.Begin` REFUSES a confined recording that was not told which
// one — so "the window you go to" is false, and it is the kind of false that teaches somebody to
// click into something else and then wonder where their demonstration went. See the recording
// invariant in CLAUDE.md and [[ADR-268-record-only-the-chosen-window]].
func TestHelpSaysTheWindowIsChosenBeforeRecordingStarts(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	h := newActs(t.TempDir()).Help()

	rec := ""
	for _, s := range h.Sections {
		if s.Key == "recording" {
			rec = strings.Join(s.Lines, " ")
		}
	}
	if rec == "" {
		t.Fatal("Help explains everything except how to record")
	}
	for _, want := range []string{"choose the window", "asks which one before it starts"} {
		if !strings.Contains(rec, want) {
			t.Errorf("the recording section does not say %q: %q", want, rec)
		}
	}
	if strings.Contains(rec, "window you go to") {
		t.Errorf("Help says Marco records the window you go to, which is the "+
			"behaviour `Deps.Begin` refuses: %q", rec)
	}
}

// AND EVERY SECTION AN INLINE LINK NAMES IS A SECTION THAT EXISTS.
//
// The links beside Record, beside the argument sentence and beside Edit its steps carry a key,
// and a key with no section behind it is a link that goes to Help and leaves somebody at the top
// of a page of everything — which is the explanation moved rather than kept.
func TestEverySectionTheWindowLinksToExists(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	h := newActs(t.TempDir()).Help()

	has := map[string]bool{}
	for _, s := range h.Sections {
		if s.Title == "" || len(s.Lines) == 0 {
			t.Errorf("the %q section is empty", s.Key)
		}
		has[s.Key] = true
	}
	// Every key a control in this window names. `checks` joined them when the card
	// stopped enumerating what a guarded routine checks before each press.
	for _, key := range []string{"recording", "arguments", "editing", "measured",
		"checks"} {
		if !has[key] {
			t.Errorf("nothing in Help answers to %q", key)
		}
	}
	page := thePage(t)
	for key := range has {
		// `scope` is reached from the rail on purpose: the three choices are
		// explained on the card that offers them, and a link beside that card
		// would point at a shorter version of what is already there.
		if !strings.Contains(page, "helpOn('"+key+"')") && key != "scope" {
			t.Errorf("Help explains %q and nothing in the window links to it, "+
				"so it is reachable only by browsing", key)
		}
	}
}

// helpText is everything Help says, for a test that asks whether a word appears anywhere in it.
func helpText(h Help) string {
	var b strings.Builder
	for _, s := range h.Sections {
		b.WriteString(s.Title)
		b.WriteString("\n")
		for _, l := range s.Lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}
	return b.String()
}
