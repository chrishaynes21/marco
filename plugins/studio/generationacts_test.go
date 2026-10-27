package main

// THE BACKEND BEHIND THE REVIEW BUTTON, THROUGH THE REAL SAVE PATH.
//
// The page test next door renders the window against a stub, which is right for what it asks —
// whether the branches in the page take. This asks the other half: whether the `Acts` methods the
// page calls refuse what they must, with a real library, a real save, and a real demonstration
// beside the routine.
//
// Both are needed and neither substitutes for the other. A greyed-out button proves nothing about
// a backend that would have obliged, and a refusing backend proves nothing about a button that
// offers it.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// aSavedRoutineInStudio is one routine saved through Studio's own path, with its demonstration.
func aSavedRoutineInStudio(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a, _ := aStudioRecording(t, "a thing")
	if why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines after one save", len(rs))
	}
	return a, rs[0]
}

// A ROUTINE STUDIO JUST SAVED READS AS THE CURRENT GENERATION, AND SAYS SO.
//
// Through the real listing, so the card's three fields come off a file this build actually wrote
// rather than off a fixture.
func TestTheListingSaysWhichGenerationWroteARoutine(t *testing.T) {
	_, r := aSavedRoutineInStudio(t)

	if r.Generation != string(orchestrator.GenerationCurrent) {
		t.Errorf("a routine Studio just saved is listed as generation %q", r.Generation)
	}
	if r.Declared == "" {
		t.Error("the listing does not carry what the file itself declares")
	}
	if !strings.Contains(r.Wrote, "generation this build writes") {
		t.Errorf("the sentence is %q", r.Wrote)
	}
	if !r.Rebuildable || !r.Offer {
		t.Errorf("a routine with its demonstration beside it is not offerable "+
			"(rebuildable=%v offer=%v)", r.Rebuildable, r.Offer)
	}
	// AND THE CONTRACT IS A SEPARATE FACT, which is the whole point of the stamp being
	// provenance. Both are listed and neither is read off the other.
	if r.Contract == "" {
		t.Error("the listing stopped saying what its presses promise")
	}
}

// AND A ROUTINE WHOSE DEMONSTRATION IS GONE IS LISTED AS NOT OFFERABLE, WITH THE REASON.
func TestARoutineWithNoDemonstrationIsListedAsNotOfferable(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, ok := a.routineWithID(r.ID)
	if !ok {
		t.Fatal("the routine Studio just saved cannot be found by its own identity")
	}
	route := rt.route()
	if err := os.Remove(a.reg().RecordingPath(route)); err != nil {
		t.Fatal(err)
	}
	again := a.Routines()[0]
	if again.Rebuildable || again.Offer {
		t.Errorf("a routine with no demonstration is offered a rebuilt version "+
			"(rebuildable=%v offer=%v)", again.Rebuildable, again.Offer)
	}
	// AND THE SENTENCE SAYS WHY THE BUTTON IS OFF. A current routine with no
	// demonstration is nothing to worry about today, and the greyed control still
	// needs explaining — an unexplained disabled button is a dead end.
	if !strings.Contains(again.Wrote, "no longer stored beside it") {
		t.Errorf("the sentence does not say why there is nothing to rebuild: %q",
			again.Wrote)
	}
	// AND REVIEWING IT IS REFUSED RATHER THAN FAILING LATER.
	v := a.ReviewRebuilt(r.ID)
	if v.Why == "" {
		t.Fatalf("a routine with no demonstration was reviewed anyway: %+v", v)
	}
	if v.Now != "" {
		t.Error("a refused review carries a rebuilt source")
	}
}

// A ROUTINE FROM A LATER BUILD IS NOT OFFERED A REVIEW, AND IS LEFT EXACTLY AS IT IS.
//
// Two claims in one, because the second is what makes the first acceptable: withholding the offer
// is the whole of what happens. The routine stays listed, stays runnable, and its source is
// untouched.
func TestARoutineFromALaterBuildIsNotOfferedAReviewAndIsUntouched(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, _ := a.routineWithID(r.ID)
	route := rt.route()
	path := a.reg().Path(route)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ahead := strings.Replace(string(raw), "// Generation: "+r.Declared+".",
		"// Generation: 97.", 1)
	if ahead == string(raw) {
		t.Fatalf("the routine does not state its generation where this expects:\n%s",
			raw)
	}
	if err := os.WriteFile(path, []byte(ahead), 0o644); err != nil {
		t.Fatal(err)
	}
	later := a.Routines()[0]
	if later.Generation != string(orchestrator.GenerationUnknown) {
		t.Errorf("a routine stating generation 97 is listed as %q", later.Generation)
	}
	if later.Declared != "97" {
		t.Errorf("the listing declares %q and the file says 97 — which is the only "+
			"fact in it that identifies the routine", later.Declared)
	}
	if later.Offer {
		t.Error("a routine whose generation this build cannot explain is offered a " +
			"rebuilt version")
	}
	// AND IT IS STILL A ROUTINE. Nothing is refused over a comment: a build that would
	// not list or run a forward-dated file would make every one of them unusable.
	if !later.Runnable {
		t.Errorf("a routine from a later build is listed as unable to run: %q",
			later.Why)
	}
	// AND REVIEWING IT IS DECLINED WITH THE CARD'S OWN SENTENCE, so the greyed button
	// and the refusal agree.
	v := a.ReviewRebuilt(r.ID)
	if v.Why != later.Wrote {
		t.Errorf("the refusal says %q and the card says %q, so a person pressing "+
			"past the greyed button is told something else", v.Why, later.Wrote)
	}
	if v.Now != "" || v.Replaceable {
		t.Errorf("a declined review carries a version to replace with: %+v", v)
	}
	if now, _ := os.ReadFile(path); string(now) != ahead {
		t.Errorf("the routine was rewritten by being looked at:\n%s", now)
	}
}

// NOTHING IS REPLACED WITHOUT A REVIEW IN HAND.
//
// The flow's one load-bearing rule at this layer: `ReplaceWithRebuilt` does not build anything.
// A version that was never on the screen is a version nobody approved, and a method that
// helpfully built one would turn the review into decoration.
func TestNothingIsReplacedWithoutAReviewInHand(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, _ := a.routineWithID(r.ID)
	route := rt.route()
	was, err := os.ReadFile(a.reg().Path(route))
	if err != nil {
		t.Fatal(err)
	}
	why := a.ReplaceWithRebuilt(r.ID)
	if why == "" {
		t.Fatal("a routine was replaced without anybody reviewing a version of it")
	}
	if !strings.Contains(why, "review it first") {
		t.Errorf("the refusal is %q and does not say what to do", why)
	}
	if now, _ := os.ReadFile(a.reg().Path(route)); string(now) != string(was) {
		t.Error("the refused replacement wrote to the routine anyway")
	}
	// AND WITH ONE IN HAND IT GOES THROUGH, or the rule above is satisfied by a method
	// that refuses everything. The routine is current, so the replacement is its own
	// source — which is exactly what makes it checkable: byte for byte.
	v := a.ReviewRebuilt(r.ID)
	if v.Why != "" {
		t.Fatalf("reviewing refused: %s", v.Why)
	}
	if why := a.ReplaceWithRebuilt(r.ID); why != "" {
		t.Fatalf("replacing after a review refused: %s", why)
	}
	if now, _ := os.ReadFile(a.reg().Path(route)); string(now) != v.Now {
		t.Errorf("the file holds something other than the version that was reviewed:"+
			"\n--- on disk ---\n%s\n--- reviewed ---\n%s", now, v.Now)
	}
	// AND THE REVIEW IS SPENT. A second press of a button whose version has already
	// been written is not a second replacement.
	if why := a.ReplaceWithRebuilt(r.ID); why == "" {
		t.Error("the same review replaced the routine twice")
	}
}

// AND CANCELLING LETS GO OF THE VERSION, SO A LATER PRESS CANNOT WRITE IT.
//
// Cancel has nothing to undo — preparation wrote nothing — so what it does is let go. If it did
// not, a person who dismissed a review could still have it written by a stale button press, which
// is a replacement they explicitly declined.
func TestCancellingAReviewLetsGoOfTheVersion(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	if v := a.ReviewRebuilt(r.ID); v.Why != "" {
		t.Fatalf("reviewing refused: %s", v.Why)
	}
	a.CancelReview(r.ID)
	why := a.ReplaceWithRebuilt(r.ID)
	if why == "" {
		t.Fatal("a review that was cancelled was still written, so declining a " +
			"replacement does not decline it")
	}
	if !strings.Contains(why, "review it first") {
		t.Errorf("the refusal is %q", why)
	}
}

// AND A REVIEW IS OF THE ROUTINE THAT WAS ASKED ABOUT, NOT WHICHEVER ANSWERED FIRST.
//
// Three routines saved for one application under one scope, so the only thing telling them apart
// is the identity. The version of this that looked a routine up by name answered with whichever
// row came first — and here that would mean reviewing one routine and replacing another.
func TestAReviewIsOfTheRoutineThatWasAskedAbout(t *testing.T) {
	a, rs := aLibraryOfThreeRoutines(t)
	want := rs[2]

	v := a.ReviewRebuilt(want.ID)
	if v.Why != "" {
		t.Fatalf("reviewing %q refused: %s", want.Name, v.Why)
	}
	if v.ID != want.ID || v.Name != want.Name {
		t.Errorf("asked to review %q (%s) and got %q (%s)", want.Name, want.ID,
			v.Name, v.ID)
	}
	// AND THE SOURCE IN IT IS THAT ROUTINE'S. The names are in the generated source,
	// so a review of the wrong routine shows the wrong actor.
	if !strings.Contains(v.Was, "CThing") {
		t.Errorf("the review shows a source that is not the chosen routine's:\n%s",
			v.Was)
	}
	if why := a.ReplaceWithRebuilt(want.ID); why != "" {
		t.Fatalf("replacing refused: %s", why)
	}
	// AND THE OTHER TWO ARE UNTOUCHED.
	for _, other := range rs[:2] {
		rt, _ := a.routineWithID(other.ID)
		raw, err := os.ReadFile(a.reg().Path(rt.route()))
		if err != nil {
			t.Fatalf("%q is no longer where it was: %v", other.Name, err)
		}
		if string(raw) != other.Source {
			t.Errorf("%q was rewritten by a review of another routine", other.Name)
		}
	}
}

// AND A REVIEW IS ADDRESSED BY IDENTITY, WHICH IS THE ONLY WAY TO REACH ONE OF TWO SAME-NAMED
// ROUTINES.
//
// One name saved for two applications is two routines, and this is where getting it wrong is
// worst: a lookup that took the first row answering to a name would show one routine's source,
// have somebody approve it, and replace the OTHER one's file — with a success message naming the
// routine they were looking at. Every part of that is correct except the subject.
func TestAReviewOfTwoSameNamedRoutinesIsOfTheOneAddressed(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	a.capture = nil
	aSecondRoutineCalled(t, a, "ark graphics", "discord", "app-anywhere")

	const second = "discord/focus/ark-graphics"
	var first Routine
	for _, r := range a.Routines() {
		if r.ID == "arkascended/focus/ark-graphics" {
			first = r
		}
	}
	if first.ID == "" {
		t.Fatal("the first routine is not in the listing, so this is not the case " +
			"it says it is")
	}
	// AND A NAME IS NOT AN ADDRESS. Asked for by the name both of them answer to, the
	// review has to REFUSE rather than choose one — because choosing is how a person
	// comes to approve one routine's source and have another one's file replaced, with a
	// message naming the one they were looking at. A lookup widened to accept a name is
	// invisible until something passes one, which is why something here does.
	for _, loose := range []string{"ark graphics", "ark-graphics", "Ark Graphics"} {
		v := a.ReviewRebuilt(loose)
		if v.Why == "" {
			t.Errorf("a review asked for by %q answered with %q rather than "+
				"refusing an address that names two routines", loose, v.ID)
		}
		if why := a.ReplaceWithRebuilt(loose); why == "" {
			t.Errorf("a replacement asked for by %q went through", loose)
		}
	}
	v := a.ReviewRebuilt(second)
	if v.Why != "" {
		t.Fatalf("reviewing %s refused: %s", second, v.Why)
	}
	if v.ID != second {
		t.Errorf("asked to review %s and got %s", second, v.ID)
	}
	if why := a.ReplaceWithRebuilt(second); why != "" {
		t.Fatalf("replacing %s refused: %s", second, why)
	}
	// AND THE OTHER ROUTINE OF THAT NAME IS UNTOUCHED, read off its own file.
	raw, err := os.ReadFile(a.reg().Path(first.route()))
	if err != nil {
		t.Fatalf("the first routine is no longer where it was: %v", err)
	}
	if string(raw) != first.Source {
		t.Errorf("replacing %s rewrote %s, which shares its name:\n%s", second,
			first.ID, raw)
	}
}
