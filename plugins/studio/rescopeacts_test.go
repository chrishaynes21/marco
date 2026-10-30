package main

// THE BACKEND BEHIND THE SCOPE CHOOSER, WITH A REAL LIBRARY.
//
// The page test next door renders the window against a stub, which answers whether the branches
// in the page take. This asks the other half: whether the `Acts` methods the page calls offer what
// can be done, refuse what cannot, and never let the page author the agreement that costs somebody
// a routine.
//
// Both are needed and neither substitutes for the other. A greyed-out button proves nothing about
// a backend that would have obliged, and a refusing backend proves nothing about a button that
// offers it.
//
// The move contract itself — both locks, both revalidations, the exclusive create, the destination
// read back, the removal that takes the source first — is held in `internal/orchestrator`, and
// these do not restate it. What is asked here is this surface's own promises.
//
// See [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aMovableRoutineInStudio is one routine with no guarded press — so all three scopes are open to
// it — saved in a library this window reads, with its demonstration beside it.
//
// Saved through `orchestrator.Deps` rather than through a Studio recording because what is needed
// is a routine whose presses are NOT checked against the window they were made in: Studio's own
// recording fixture glances at a window, which is guarded, and a guarded routine has exactly one
// scope it can be saved under. That one has its own test below.
func aMovableRoutineInStudio(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a := aStudioLibrary(t, "type my address", typingIntoNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines, want the one saved", len(rs))
	}
	return a, rs[0]
}

// aStudioLibrary is a window pointed at a library holding one routine made from these events.
func aStudioLibrary(t *testing.T, name string, events []recorder.RecordedEvent) *Acts {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	d := orchestrator.Deps{Reg: routes.Registry{Dir: dir}, Out: &strings.Builder{}}
	dr := d.DraftOf(name, events, "notepad", "the stop key", orchestrator.Unstated)
	if _, err := d.Save(dr, orchestrator.ScopeAppAnywhere, "notepad"); err != nil {
		t.Fatalf("saving the fixture: %v", err)
	}
	return newActs(t.TempDir())
}

// typingIntoNotepad is a demonstration with no press that checks the window it was made in, so
// every scope is honestly available to it.
func typingIntoNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(10), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: at(20), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(30), App: "notepad"},
	}
}

// clickingInNotepad is a demonstration whose press asks to be checked against the window it was
// made in, which is the one that cannot be moved to a scope bringing no window forward.
func clickingInNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 400, Y: 300,
			App: "notepad", PID: 77, T: at(100), WinRel: true, RelX: 120,
			RelY: 90, RelTo: "window", WinW: 1280, WinH: 800, Dpi: 96,
			Process: "notepad.exe"},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 400, Y: 300,
			App: "notepad", PID: 77, T: at(105)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(200), App: "notepad"},
	}
}

// choiceFor is one of the three answers, by its scope word.
func choiceFor(t *testing.T, cs []ScopeChoice, scope string) ScopeChoice {
	t.Helper()
	for _, c := range cs {
		if c.Scope == scope {
			return c
		}
	}
	t.Fatalf("no choice for %q among %d answers", scope, len(cs))
	return ScopeChoice{}
}

// ── WHAT IS OFFERED, AND WHAT IS NOT ──────────────────────────────────────────

// ALL THREE CHOICES ARE ANSWERED, WITH THE ONE IT HAS MARKED AS THE ONE IT HAS.
//
// Three answers and not a list of what is possible: a chooser showing only the available ones
// answers "why can this not work from anywhere?" by removing the row that would have said.
func TestEveryScopeIsAnsweredForAChosenRoutine(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)

	cs := a.ScopeChoices(r.ID, "")
	if len(cs) != 3 {
		t.Fatalf("%d choices offered for a routine, want all three: %+v", len(cs), cs)
	}
	now := choiceFor(t, cs, string(orchestrator.ScopeAppAnywhere))
	if !now.Current {
		t.Errorf("the scope the routine already has is not marked as the one it "+
			"has: %+v", now)
	}
	if now.Available {
		t.Error("the scope it already has is offered as a move, which is one lock " +
			"taken twice and a removal of what was just written")
	}
	// AND THE OTHER TWO ARE OPEN TO IT, because nothing in this routine checks a
	// window. The positive claim, and it earns its place: a chooser that refused
	// everything would pass every refusal test below.
	for _, scope := range []string{string(orchestrator.ScopeApp),
		string(orchestrator.ScopeEverywhere)} {
		c := choiceFor(t, cs, scope)
		if !c.Available {
			t.Errorf("%s is not available to a routine that checks no window: %q",
				scope, c.Why)
		}
		if c.Where == "" || c.Asked == "" {
			t.Errorf("%s says nothing about what it means: %+v", scope, c)
		}
		// AND WHERE IT WORKS AND WHERE IT CAN BE ASKED FOR ARE TWO SENTENCES.
		// `resolveroute.Eligible` offers an in-place routine only while its
		// application is in front, which is the half somebody discovers a week
		// later with a library that looks like it has lost something.
		if c.Where == c.Asked {
			t.Errorf("%s says one sentence for two facts: %q", scope, c.Where)
		}
	}
}

// AND A SCOPE THIS ROUTINE'S PRESSES COULD NOT BE PERFORMED UNDER IS UNAVAILABLE, WITH THE REASON.
//
// The reason is the backend's own and it is specific: a press checked against the window it was
// made in needs that window brought forward, and two of the three scopes bring none. A row saying
// only "unavailable" would be a dead end with the answer withheld.
func TestAGuardedRoutineIsRefusedTheScopesThatBringNoWindow(t *testing.T) {
	a := aStudioLibrary(t, "ark graphics", clickingInNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}

	cs := a.ScopeChoices(rs[0].ID, "")
	for _, scope := range []string{string(orchestrator.ScopeApp),
		string(orchestrator.ScopeEverywhere)} {
		c := choiceFor(t, cs, scope)
		if c.Available {
			t.Errorf("%s is offered to a routine whose every press would then "+
				"refuse", scope)
		}
		if !strings.Contains(c.Why, "checks the window it was made in") {
			t.Errorf("%s does not say why it is unavailable: %q", scope, c.Why)
		}
	}
	// AND REVIEWING IT ANYWAY IS REFUSED, with the review still worth showing: the
	// backend is asked again, so a page that offered the row regardless gets a sentence
	// rather than a move.
	v := a.ReviewRescope(rs[0].ID, string(orchestrator.ScopeEverywhere), "")
	if v.Movable {
		t.Fatalf("a refused move is offered as movable: %+v", v)
	}
	if v.Refused == "" {
		t.Error("the review of a refused move says nothing about why")
	}
}

// AND A ROUTINE WHOSE DEMONSTRATION IS GONE CANNOT BE MOVED AT ALL, AND SAYS SO IN EVERY ROW.
//
// The difference between the scopes is in the generated source, so a move renders from the
// recording. Without one there is nothing to render, and that is a fact about the routine rather
// than about one scope.
func TestARoutineWithNoDemonstrationIsOfferedNoMove(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	chosen, _ := a.routineWithID(r.ID)
	if err := os.Remove(a.reg().RecordingPath(chosen.route())); err != nil {
		t.Fatal(err)
	}

	for _, c := range a.ScopeChoices(r.ID, "") {
		if c.Current {
			continue
		}
		if c.Available {
			t.Errorf("%s is offered for a routine with nothing to render from",
				c.Scope)
		}
		if !strings.Contains(c.Why, "demonstration") {
			t.Errorf("%s does not say the demonstration is gone: %q", c.Scope,
				c.Why)
		}
	}
}

// ── THE REVIEW, AND WHAT IT HOLDS ─────────────────────────────────────────────

// A REVIEW SHOWS BOTH IDENTITIES AND BOTH SOURCES, AND WRITES NOTHING.
//
// Both sources because the move re-renders: for a routine somebody edited by hand the left-hand
// side is their own text, and approving the move is approving the loss of it. Both identities
// because the move is between two.
func TestAPreparedMoveShowsBothEndsAndWritesNothing(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	before, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}

	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if v.Why != "" {
		t.Fatalf("the move could not be prepared: %s", v.Why)
	}
	if v.Token == "" {
		t.Fatal("a prepared move carries no token, so nothing could be moved with it")
	}
	if !v.Movable {
		t.Fatalf("the prepared move is not movable: %+v", v)
	}
	if v.From != r.ID || v.To == "" || v.From == v.To {
		t.Errorf("the review does not name two identities: from %q to %q", v.From,
			v.To)
	}
	if v.Was == "" || v.Now == "" {
		t.Error("the review does not carry both sources")
	}
	if v.Where == "" || v.Asked == "" {
		t.Error("the review does not say what the chosen scope means")
	}
	// NOTHING ON DISK HAS CHANGED, which is the claim the other three rest on.
	after, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatalf("the routine is no longer where it was after a review: %v", err)
	}
	if string(after) != string(before) {
		t.Error("preparing a move rewrote the routine it was about")
	}
	if len(a.Routines()) != 1 {
		t.Error("preparing a move wrote a second routine")
	}
}

// AND A MOVE CARRIES OUT THE ONE THAT WAS REVIEWED, THEN SAYS WHERE IT WENT.
//
// The successful control. Every other test here asserts something did not happen, and a surface
// that refused everything would pass all of them.
func TestAReviewedMoveLandsAtTheNewIdentityAndSaysSo(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if v.Why != "" {
		t.Fatalf("the move could not be prepared: %s", v.Why)
	}

	done := a.MoveRoutine(v.Token, false)
	if done.Why != "" {
		t.Fatalf("the move was refused: %s", done.Why)
	}
	if !done.Moved || done.Copied {
		t.Errorf("an ordinary move is not reported as moved: %+v", done)
	}
	if done.To != v.To {
		t.Errorf("the move landed at %q, having been reviewed as going to %q",
			done.To, v.To)
	}
	if !strings.Contains(done.Says, "Moved") {
		t.Errorf("the outcome sentence is %q", done.Says)
	}
	if done.Left != "" {
		t.Errorf("it reports litter it did not leave: %q", done.Left)
	}
	// AND THE LIBRARY HOLDS ONE ROUTINE, AT THE NEW IDENTITY. A copy left behind is the
	// failure this reports as `Copied`, and it is not what happened here.
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines after a move: %+v", len(rs), rs)
	}
	if rs[0].ID != v.To {
		t.Errorf("the routine is at %q after a move to %q", rs[0].ID, v.To)
	}
	// AND THE PREPARATION IS SPENT, so a second press is not a second move.
	//
	// THE WINDOW'S OWN SENTENCE, and not merely A refusal: with the preparation kept,
	// the second press is refused by the contract underneath — the original is no
	// longer where the review said it was — and an assertion that something was
	// refused passes while this window holds a review it has already spent. A
	// mutation deleting the release survived exactly that. See rescope.go.
	again := a.MoveRoutine(v.Token, false)
	if again.Why == "" {
		t.Fatalf("the same review moved twice: %+v", again)
	}
	if !strings.Contains(again.Why, "no longer the one this window holds") {
		t.Errorf("a spent review is still held by this window: it was refused "+
			"with %q, which is the contract underneath answering rather than "+
			"this window having let go", again.Why)
	}
}

// AND A MOVE IS PERFORMED WITH A TOKEN THIS WINDOW MINTED, NEVER WITH ONE HANDED IN.
func TestAMoveWithATokenThisWindowNeverPreparedIsRefused(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if v.Why != "" {
		t.Fatalf("the move could not be prepared: %s", v.Why)
	}

	if done := a.MoveRoutine(v.Token+"x", false); done.Why == "" {
		t.Error("a move went ahead on a token this window never prepared")
	}
	if len(a.Routines()) != 1 || a.Routines()[0].ID != r.ID {
		t.Error("a refused move touched the library")
	}
	// AND CANCEL LETS GO OF THE REAL ONE, so what is left cannot be moved with either.
	a.CancelMove(v.Token)
	if done := a.MoveRoutine(v.Token, false); done.Why == "" {
		t.Error("a cancelled move was performed anyway")
	}
	if len(a.Routines()) != 1 || a.Routines()[0].ID != r.ID {
		t.Error("cancelling a move changed the library")
	}
}

// ── AN OCCUPIED DESTINATION ───────────────────────────────────────────────────

// MOVING ONTO AN OCCUPIED IDENTITY NEEDS ITS OWN YES, AND THE PAGE CANNOT AUTHOR THE VERSION.
//
// Three claims in one test because they are one rule seen from three sides: the review SAYS what
// is in the way, a move without the yes is refused with nothing written, and the yes replaces the
// version the review was prepared against — which the caller never names.
func TestAMoveOntoAnOccupiedIdentityNeedsItsOwnYes(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	// THE SAME SLUG AT THE DESTINATION, saved there by something else. Written through
	// the registry because what is being arranged is the collision, not a second save.
	chosen, _ := a.routineWithID(r.ID)
	inTheWay := routes.Route{App: chosen.App, Slug: chosen.Slug}
	if err := a.reg().Create(inTheWay, "// somebody else's routine\nuse os.\n"); err != nil {
		t.Fatal(err)
	}

	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if v.Why != "" {
		t.Fatalf("the move could not be prepared: %s", v.Why)
	}
	if !v.Taken || v.TakenID == "" || v.TakenSays == "" {
		t.Fatalf("the review does not say something is already there: %+v", v)
	}
	// WITHOUT THE YES, NOTHING IS WRITTEN. The destination is created exclusively, so
	// this is a refusal rather than a replacement.
	if done := a.MoveRoutine(v.Token, false); done.Why == "" {
		t.Fatalf("a move replaced an occupied destination with no agreement: %+v",
			done)
	}
	at := a.reg().Path(inTheWay)
	kept, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kept), "somebody else") {
		t.Fatalf("the routine at the destination was written over: %q", kept)
	}
	// AND WITH IT, THE ONE THAT WAS SHOWN IS THE ONE REPLACED.
	done := a.MoveRoutine(v.Token, true)
	if done.Why != "" {
		t.Fatalf("the agreed move was refused: %s", done.Why)
	}
	if !done.Moved {
		t.Errorf("the agreed move did not report moving: %+v", done)
	}
	now, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(now), "somebody else") {
		t.Error("the agreed move left the routine it agreed to replace")
	}
}

// AND THE YES IS BOUND TO THE VERSION THAT WAS SHOWN, THOUGH THE PAGE NEVER NAMES ONE.
//
// This is the whole reason `MoveRoutine` takes a boolean rather than digests. The agreement is to
// lose WHAT WAS ON THE SCREEN; if something else is saved there in the meantime, a surface that
// forwarded "yes, replace" would have agreed to lose a routine nobody looked at.
func TestAYesToReplaceDoesNotCoverARoutineSavedSince(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	chosen, _ := a.routineWithID(r.ID)
	inTheWay := routes.Route{App: chosen.App, Slug: chosen.Slug}
	if err := a.reg().Create(inTheWay, "// the one that was shown\nuse os.\n"); err != nil {
		t.Fatal(err)
	}
	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if !v.Taken {
		t.Fatalf("the destination is not reported as occupied: %+v", v)
	}
	// AND THEN SOMETHING ELSE LANDS THERE. Exactly what a second window, or a terminal,
	// does between the question and the answer.
	if err := a.reg().Replace(inTheWay, "// saved since, by somebody else\nuse os.\n"); err != nil {
		t.Fatal(err)
	}

	done := a.MoveRoutine(v.Token, true)
	if done.Why == "" {
		t.Fatalf("a yes to replace one routine replaced a different one: %+v", done)
	}
	now, err := os.ReadFile(a.reg().Path(inTheWay))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(now), "saved since") {
		t.Errorf("the newer routine at the destination was written over: %q", now)
	}
}

// AND A MOVE OF A ROUTINE THAT WAS SAVED AGAIN SINCE THE REVIEW IS REFUSED, NOT PERFORMED.
//
// The stale review, from this surface's side. The orchestrator refuses it; what is asked here is
// that the refusal arrives as a sentence this window can show rather than as a move of something
// else.
func TestAMoveOfARoutineSavedAgainSinceTheReviewIsRefused(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	v := a.ReviewRescope(r.ID, string(orchestrator.ScopeApp), "")
	if v.Why != "" {
		t.Fatalf("the move could not be prepared: %s", v.Why)
	}
	chosen, _ := a.routineWithID(r.ID)
	if err := a.reg().Replace(chosen.route(), v.Was+"// edited by hand since\n"); err != nil {
		t.Fatal(err)
	}

	done := a.MoveRoutine(v.Token, false)
	if done.Why == "" {
		t.Fatalf("a move went ahead on a review of a routine that has changed: %+v",
			done)
	}
	src, err := os.ReadFile(a.reg().Path(chosen.route()))
	if err != nil {
		t.Fatalf("the hand-edited routine is gone: %v", err)
	}
	if !strings.Contains(string(src), "edited by hand since") {
		t.Error("the hand edit was moved away from under them")
	}
	if len(a.Routines()) != 1 {
		t.Error("a refused move wrote a second routine")
	}
}
