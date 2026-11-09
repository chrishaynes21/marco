package main

// RUNNING WHAT YOU TAUGHT, BY THE NAME YOU GAVE IT.
//
// The integration gap between the product's two proven halves, and it was one flag wide.
// `demonstrate-visual` saves a demonstration in the demo store under the person's own name,
// scoped to the application it watched, with the raw demonstration beside it; `correct`,
// `reacquire` and `what-i-did` all take that name. The one command that ACTS took a file path,
// so running what you had just taught meant finding its JSON under the config directory.
//
// These drive `workflowToPerform`, which is the whole of the resolution, against a real store
// in a temporary MARCO_HOME.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aNamedDemonstration is the shape `demonstrate-visual` saves: named, scoped, one step with an
// outcome expectation, and a control carrying the pixels it was recognised by.
func aNamedDemonstration(name string) workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: name,
		SavedAt:     time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}},
			Text: []string{"GRAPHICS"}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"},
		}},
	}
}

// aStoreWith puts a demonstration in a temporary MARCO_HOME and returns nothing but the
// guarantee that `configDir()` now points at it.
func aStoreWith(t *testing.T, arts ...workflowstore.Artifact) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatalf("opening a store in a temporary home: %v", err)
	}
	for _, a := range arts {
		if err := store.SaveSequence(a); err != nil {
			t.Fatalf("saving %q: %v", a.Name, err)
		}
	}
}

// TestADemonstrationIsPerformedByTheNameItWasSavedUnder.
//
// The whole point: a person says what to call it, and that is what they type to run it. And
// what comes back is the SAVED artifact — its application scope, its control's pixels and its
// outcome expectation — because a routine that lost any of those on the way to being performed
// could not be performed safely or verified at all.
func TestADemonstrationIsPerformedByTheNameItWasSavedUnder(t *testing.T) {
	aStoreWith(t, aNamedDemonstration("graphics tab"))

	art, code := workflowToPerform("", "graphics tab")
	if code != 0 {
		t.Fatalf("exit %d resolving a saved demonstration by name", code)
	}
	if art.Name != "graphics tab" {
		t.Errorf("name %q, want the one it was saved under", art.Name)
	}
	if art.Application != workflowstore.NormaliseScope("arkascended") {
		t.Errorf("scope %q — a play performed by name must keep the application "+
			"it was demonstrated against", art.Application)
	}
	if len(art.Steps) != 1 || len(art.Steps[0].Outcome) != 5 {
		t.Fatalf("steps %+v — the outcome expectation is what makes the action "+
			"verifiable, and it must survive being looked up", art.Steps)
	}
	if len(art.Controls) != 1 || len(art.Controls[0].Readings) != 1 {
		t.Errorf("controls %+v — the control's pixels are how it is grounded again",
			art.Controls)
	}
	// AND THE NAME IS NOT CASE-SENSITIVE TO THE PERSON WHO TYPED IT, if the store says
	// so: whatever `Sequence` resolves is what runs, and this asserts only that the
	// command adds no second rule of its own.
	if a2, code := workflowToPerform("", "Graphics Tab"); code == 0 && a2.Name != art.Name {
		t.Errorf("a differently-cased name resolved to %q", a2.Name)
	}
}

// TestANameNobodyTaughtIsRefusedWithSomewhereToGo.
//
// Not a search and not a nearest match. The refusal says what to type to see what HAS been
// taught, because guessing which saved routine somebody meant is how the wrong one runs — and
// this command presses a mouse button.
func TestANameNobodyTaughtIsRefusedWithSomewhereToGo(t *testing.T) {
	aStoreWith(t, aNamedDemonstration("graphics tab"))

	art, code := workflowToPerform("", "graphics tap")
	if code == 0 {
		t.Fatalf("a name nobody taught resolved to %q", art.Name)
	}
	if code != 1 {
		t.Errorf("exit %d, want 1 — the store was readable and the name was not in it",
			code)
	}
}

// TestAFileAndANameAreNotBothAnswers.
//
// `--workflow` stays, and stays a FILE: a derived fixture is an artifact nobody saved by name,
// and the controlled execution tests run from one. Given both, this refuses rather than
// choosing — a name that is also a path would turn a typo into a different routine running.
func TestAFileAndANameAreNotBothAnswers(t *testing.T) {
	aStoreWith(t, aNamedDemonstration("graphics tab"))

	if _, code := workflowToPerform("routine.json", "graphics tab"); code != 2 {
		t.Errorf("exit %d with both a file and a name, want 2 — a usage refusal", code)
	}
}

// TestAnArtifactFileStillPerforms, because the fixture path depends on it.
func TestAnArtifactFileStillPerforms(t *testing.T) {
	a := aNamedDemonstration("from a file")
	raw, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "routine.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	art, code := workflowToPerform(path, "")
	if code != 0 {
		t.Fatalf("exit %d performing an artifact file", code)
	}
	if art.Name != "from a file" || art.Application == "" {
		t.Errorf("the file's artifact came back as %+v", art)
	}
}

// TestAnUnreadableFileAndAnUnreadableNameSayDifferentThings.
//
// The two answers `reacquire` gives, kept here for the same reason: a routine that cannot be
// READ is a file to go and look at, and a routine that was never saved is a demonstration to go
// and give. One sentence for both would send somebody to the wrong place.
func TestAnUnreadableFileAndAnUnreadableNameSayDifferentThings(t *testing.T) {
	if _, code := workflowToPerform(filepath.Join(t.TempDir(), "absent.json"), ""); code != 1 {
		t.Errorf("exit %d for a file that is not there, want 1", code)
	}
	aStoreWith(t)
	if _, code := workflowToPerform("", "never taught"); code != 1 {
		t.Errorf("exit %d for a name that was never saved, want 1", code)
	}
}

// TestARoutineThatCannotBeReadSaysSoRatherThanThatItWasNeverTaught.
//
// The two answers send somebody to different places, so the refusal has to pick the right one:
// a routine whose file is corrupt or is from a future version is a file to go and look at, and
// a name nobody saved is a demonstration to go and give. "Nothing called that has been saved"
// about a routine sitting right there, with the person's own name on it, is the worse of the two
// errors by a long way — they would re-teach it.
func TestARoutineThatCannotBeReadSaysSoRatherThanThatItWasNeverTaught(t *testing.T) {
	// SAVED PROPERLY FIRST, so the file is wherever the store puts it under whatever name
	// the store makes of "graphics tab" — then corrupted in place. Writing the file by a
	// path this test guessed would test the guess.
	aStoreWith(t, aNamedDemonstration("graphics tab"))
	dir := filepath.Join(configDir(), "sequences")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the store kept %d file(s) in %s: %v", len(entries), dir, err)
	}
	saved := filepath.Join(dir, entries[0].Name())
	if err := os.WriteFile(saved,
		[]byte(`{"version":999,"name":"graphics tab"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	said := stderrOf(t, func() { workflowToPerform("", "graphics tab") })
	if strings.Contains(said, "has been saved") {
		t.Errorf("a corrupt play was reported as never taught:\n%s", said)
	}
	if !strings.Contains(said, "cannot be read") {
		t.Errorf("refusal = %q, want it to say the play cannot be read", said)
	}
}

// TestDiscoveryWaitsForThePersonToSwitchAndNamingDoesNot.
//
// `--delay` exists so a person can start a run and then switch to the thing it watches.
// Foreground discovery ran at the command's first instruction, so it read whatever was in front
// while they were still typing — their terminal. Measured live, and it blocked a test outright:
//
//	this workflow was demonstrated against "arkascended" and the application resolved now is
//	"code"
//
// with `--delay 10s` on the command line and the game one alt-tab away. The scope gate was
// right; the question was asked at the wrong moment.
func TestDiscoveryWaitsForThePersonToSwitchAndNamingDoesNot(t *testing.T) {
	if !lookAfterWaiting("", 10*time.Second) {
		t.Error("a run that has to DISCOVER the application looked before the person " +
			"could switch to it, which reads their terminal")
	}
	// NAMING IT IS SOMEBODY SAYING WHICH ONE THEY MEAN. Making them wait to be told
	// what they already said is a delay spent on nothing.
	if lookAfterWaiting("arkascended", 10*time.Second) {
		t.Error("a named application still waited before resolving")
	}
	if lookAfterWaiting("  arkascended  ", 10*time.Second) {
		t.Error("a named application with spaces around it was treated as unnamed")
	}
	// AND NO DELAY IS NOTHING TO SPEND.
	if lookAfterWaiting("", 0) {
		t.Error("a run with no delay waited anyway")
	}
}

// TestACancelledCountdownProceedsToNothing.
//
// The countdown was an unconditional `time.Sleep`, and the cancellation context was created
// three hundred lines below it — so the window in which somebody is most likely to change their
// mind was the one window nothing could interrupt. Ctrl-C killed the process instead of
// stopping the run, and a killed process says nothing about how far it got.
//
// What has to hold is that a cancelled countdown returns false: everything the run could press
// is built after it returns, so false is the whole of "zero input was emitted".
func TestACancelledCountdownProceedsToNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if countdown(ctx, time.Hour) {
		t.Error("a cancelled countdown ran to completion, so the run would have " +
			"gone on to acquire a window and ask to press something")
	}
	// AND IT DOES NOT WAIT OUT THE CLOCK TO NOTICE. An hour-long delay that returns
	// immediately is the point: cancellation is heard, not polled for.
	start := time.Now()
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel2() }()
	if countdown(ctx2, time.Hour) {
		t.Error("a countdown cancelled while waiting ran to completion")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("cancellation took %s to be noticed", took)
	}
}

// TestAnUninterruptedCountdownProceeds, because a cancel that fires on its own stops every run.
func TestAnUninterruptedCountdownProceeds(t *testing.T) {
	if !countdown(context.Background(), time.Millisecond) {
		t.Error("an uninterrupted countdown reported cancellation")
	}
	// AND NO DELAY IS NOT A CANCELLATION EITHER.
	if !countdown(context.Background(), 0) {
		t.Error("a run with no delay was treated as cancelled")
	}
	// BUT AN ALREADY-CANCELLED RUN WITH NO DELAY IS STILL CANCELLED: the absence of a
	// wait is not permission to proceed.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if countdown(ctx, 0) {
		t.Error("a cancelled run with no delay proceeded")
	}
}
