package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// STUDIO SAYS HOW TO MAKE A VALUE SOMETHING YOU SUPPLY, BEFORE ANYBODY PRESSES RECORD.
//
// The capability was COMPLETE AND SILENT, which is why this test exists and why it asserts the
// sentence rather than the plumbing. A routine can take an argument three ways and Studio wires
// all three: `Deps.Begin` splits the name through `routes.SplitArgs`, so a name ending `with item`
// declares one; `Deps.ArgKey` is passed from the environment and falls back to F9, so the reserved
// key drops a placeholder where the value goes; and a typed step can be turned into a supplied one
// afterwards. Nothing on any surface of this window mentioned any of it. `marco learn` prints the
// key in its own prompt (`orchestrator.argKeyHint`) and the window with the Record button did not,
// so the owner of this repository went looking for the feature and reported it missing — which
// is the only report a silent capability can produce.
//
// THROUGH `Recording`, the act the page polls, in the state the page polls it in before a
// recording. A sentence a function could return and nothing asks for is the shape this repository
// keeps deleting.
func TestTheWindowSaysHowToMakeAValueSomethingYouSupply(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())

	got := a.Recording()
	// ── AND AN UNSET OVERRIDE IS F9, NOT SILENCE ──
	//
	// The defect this half is shaped after already shipped here, about the other reserved
	// key: `$MARCO_STOP_KEY` was read raw on one path and parsed on the other, so the
	// default installation named F12 during a recording and nothing at all before one. An
	// arg key read raw would do the same — go quiet in every installation that never set
	// the variable, which is all of them. This test sets nothing on purpose.
	if got.ArgKey != "F9" {
		t.Fatalf("the window names the arg key as %q with no override set, so the "+
			"default installation is told there is no key", got.ArgKey)
	}
	if got.ArgSays == "" {
		t.Fatal("nothing on the record card says a play can ask for a value, " +
			"which is how a working capability gets reported as missing")
	}
	for _, want := range []string{"F9", "ask for a value", "with item"} {
		if !strings.Contains(got.ArgSays, want) {
			t.Errorf("the sentence does not say %q: %q", want, got.ArgSays)
		}
	}
}

// AND THE BAND NAMES THE DECLARED VALUES, IN THE ORDER THE KEY FILLS THEM.
//
// Because the Nth press of the key fills the Nth name (`simplify.go`, the `argKey` branch), and
// nothing afterwards can tell that two of them went the wrong way round: the routine compiles,
// runs, and types the quantity into the item field. The order is the one fact worth saying while
// a recording is running, and the names come from the phrase the person typed — split the
// same way `Deps.Begin` splits it, so the band cannot list arguments the recorder is not filling.
func TestTheRecordingBandNamesTheDeclaredValuesInOrder(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder { return &heldRecorder{} }

	if why := a.StartRecording("cloud pull stone with item, count", ""); why != "" {
		t.Fatal(why)
	}
	got := a.Recording()
	if !got.Active {
		t.Fatal("nothing is recording")
	}
	if !strings.Contains(got.ArgSays, "in order: item, count") {
		t.Errorf("the band does not name the declared values in order: %q", got.ArgSays)
	}
	// AND IT DOES NOT REPEAT THE ADVICE ABOUT NAMING THEM, which is about a field on a
	// card that is hidden by now and a decision already made.
	if strings.Contains(got.ArgSays, "with item") {
		t.Errorf("the band still tells somebody how to declare a value they have "+
			"already declared: %q", got.ArgSays)
	}
}

// AND A KEY TURNED OFF IS NOT OFFERED.
//
// `$MARCO_ARG_KEY=off` disables it (`simplify.go` treats "off" as no key), and a window that went
// on telling somebody to press it would be naming a key that does nothing. Empty rather than a
// sentence saying it is unavailable: a window with no arg key has no arg key to offer.
func TestAnArgKeyTurnedOffIsNotOffered(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ARG_KEY", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())

	got := a.Recording()
	if got.ArgKey != "" || got.ArgSays != "" {
		t.Errorf("the window offers %q / %q with the arg key turned off",
			got.ArgKey, got.ArgSays)
	}
}

// AND AN OVERRIDE IS THE KEY THE WINDOW NAMES.
//
// One resolution of one variable, in `orchestrator.ArgKeyFor`, which is the function the recorder
// gets its key from as well. A second reading here is how the stop key came to say two different
// things about one keyboard three rows apart on the same card.
func TestTheWindowNamesTheConfiguredArgKey(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ARG_KEY", "f8")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())

	if got := a.Recording(); got.ArgKey != "F8" || !strings.Contains(got.ArgSays, "F8") {
		t.Errorf("the window names %q and says %q with the key configured as f8",
			got.ArgKey, got.ArgSays)
	}
}
