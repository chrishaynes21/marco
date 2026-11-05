package main

// ASKING FOR A VALUE FROM THE REVIEW, BEFORE ANYTHING IS SAVED.
//
// The operation existed and could not be reached from here. A routine can be told to ASK for a
// piece of text instead of typing what was demonstrated, three ways: the name's `with` clause, the
// reserved key during the demonstration, and turning a typed step into a supplied one. The third
// was on SAVED routines only, so somebody who realised at review had to save, find the routine in
// the panel, edit it, and save again — and the first two need foreknowledge nothing on the card
// mentioned. The owner of this repository looked for the feature and reported it missing.
//
// The harness is `simplifydraft_test.go`'s, because this is the same card and the same claim
// underneath: NOTHING IS WRITTEN until Save.
//
// MOST TESTS HERE SIMPLIFY FIRST, which is a fact about recordings and not a convenience. A
// recording made at a human pace holds each keystroke as its own step, so `stone` typed slowly is
// five steps and a value the routine asks for has to be one. The card says so in those words —
// see `TestAFreshRecordingSaysToSimplifyBeforeAskingForAValue`, which is the first thing somebody
// meets.
//
// THE EXCEPTION IS THE WHOLE OF THE LIFECYCLE TEST, and it is why there is one. A run typed at
// typing speed folds into one step in the ORDINARY draft — `simplify.MinWaitMs` is 30, so gaps
// under that leave no wait between the keys and `coalesceTyping` merges them — so a value can be
// marked on a recording nobody simplified. That draft carried no basis, the save wrote no account
// beside it, and the routine that ran correctly could not be edited afterwards. See
// `TestAMarkedOrdinaryRecordingCanStillBeEditedAndRebuilt` and `orchestrator.Draft.Changed`.
//
// See [[ADR-281-a-value-can-be-asked-for-from-the-review]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// briskTypingIntoNotepad types `stone` at typing speed, which folds into ONE step without
// anybody pressing Simplify.
//
// NOT A CONTRIVANCE, and the state the lifecycle defect lived in. `slowTypingIntoNotepad` beside
// it puts 400ms between characters, which is a person thinking; this puts 20ms between them,
// which is a person typing. Under `simplify.MinWaitMs` (30) no wait step is emitted at all, so
// `coalesceTyping` sees a run of adjacent printable keys and merges it — and a draft that was
// never simplified has something to ask for.
//
// The key-ups land before the next key-down, deliberately: overlapping them is typing rollover,
// which `markHolds` currently reads as a held key (ADR-280, open), and this fixture is about
// something else.
func briskTypingIntoNotepad() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	ms := 10
	for _, k := range []string{"s", "t", "o", "n", "e"} {
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: true,
				T: at(ms), App: "notepad"},
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: false,
				T: at(ms + 8), App: "notepad"})
		ms += 20
	}
	return out
}

// typedAndFolded is a recording under review whose typed run has been folded into one step, which
// is the state a value can be asked for in.
//
// Through `SimplifyDraft`, the act the page calls, rather than by building a folded draft here: a
// fixture that manufactured the state would not notice the day the two controls stopped composing.
func typedAndFolded(t *testing.T, name string) *Acts {
	t.Helper()
	a := aRecordingUnderReview(t, name, slowTypingIntoNotepad())
	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	return a
}

// A FRESH RECORDING IS TOLD WHY THERE IS NOTHING TO PICK, AND WHAT TO PRESS.
//
// THE FIRST THING SOMEBODY MEETS, and the half that decides whether this reads as a feature or as
// an absence. `coalesceTyping` folds only adjacent printable keys, and the gaps between somebody's
// keystrokes are not adjacent — so a recording of the word `stone` is five `type` steps with
// waits between them, and "ask for this value" has five values to choose from, none of which is
// the one anybody means.
//
// An empty chooser, or no control at all, is indistinguishable from the capability not existing.
// That is precisely how this slice came to be asked for, so the sentence says what the recording
// is, what a value has to be, and which button closes the gap.
func TestAFreshRecordingSaysToSimplifyBeforeAskingForAValue(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())

	got := a.Recording()
	if len(got.Supply) != 0 {
		t.Fatalf("a recording with one step per character offers %+v", got.Supply)
	}
	if got.SupplySays == "" {
		t.Fatal("the card says nothing at all, which reads as a routine that cannot " +
			"ask for a value — the report this exists to answer")
	}
	for _, want := range []string{"one step per character", "has to be one step", "Simplify"} {
		if !strings.Contains(got.SupplySays, want) {
			t.Errorf("the sentence does not say %q: %q", want, got.SupplySays)
		}
	}
	// AND SIMPLIFYING IS WHAT IT SAYS IT IS: the advice has to be true, or it is a
	// sentence sending somebody to a button that does not help.
	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	if len(a.Recording().Supply) == 0 {
		t.Error("the card told somebody to press Simplify and there is still nothing " +
			"to ask for")
	}
}

// THE REVIEW OFFERS THE STEPS IT IS SHOWING, AND A MARK CHANGES WHAT THE ROUTINE WOULD DO.
//
// Through the acts the page calls, in the order the page calls them: the chooser comes from
// `Recording`, the mark goes through `SupplyDraftText`, and what the card then shows comes from
// `Recording` again. A sentence a function could return and nothing asks for is the shape this
// repository keeps deleting.
func TestTheReviewOffersToAskForTextItWouldType(t *testing.T) {
	a := typedAndFolded(t, "say hello")
	dir := routesDirOf(t, a)

	was := a.Recording()
	if len(was.Supply) == 0 {
		t.Fatal("the review offers no way to ask for a value, on a recording whose " +
			"only content is one step of typed text")
	}
	// ── AND THE CHOOSER AND THE LIST ARE ONE LIST ──
	//
	// Every option is a position into the steps on screen, so the control and the thing it
	// operates on cannot describe two different routines. A chooser counting steps for
	// itself is a second reading of the draft, and the one that matters is the one Save
	// writes.
	for _, c := range was.Supply {
		if c.At < 0 || c.At >= len(was.Steps) {
			t.Fatalf("the chooser offers step %d of a list of %d: %+v",
				c.At+1, len(was.Steps), c)
		}
		if !strings.Contains(c.Label, "types") {
			t.Errorf("the option does not say what that step types: %q", c.Label)
		}
	}
	// AND THE CAPTION SAYS WHAT THE CONTROL IS FOR.
	if !strings.Contains(was.SupplySays, "Play will ask for that value") {
		t.Errorf("the caption does not say what happens: %q", was.SupplySays)
	}

	if why := a.SupplyDraftText(was.Basis, was.Supply[0].At, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}
	now := a.Recording()
	if !strings.Contains(strings.Join(now.Steps, "|"), "{{item}}") {
		t.Fatalf("the step still types what was demonstrated: %v", now.Steps)
	}
	// AND IT IS NOT OFFERED AGAIN. A second pass over one step would bury a placeholder
	// inside another and the routine would type the braces.
	for _, c := range now.Supply {
		if c.At == was.Supply[0].At {
			t.Errorf("the chooser still offers a step that is already supplied: %+v", c)
		}
	}
	// NOTHING WAS WRITTEN. The claim the rest of this card rests on.
	if !wroteNothing(t, dir) {
		t.Error("marking a value wrote something")
	}
}

// AND THE SAVED ROUTINE TYPES THE VALUE AND ASKS FOR IT, RATHER THAN READING A CREDENTIAL.
//
// THE WHOLE POINT, AND THE ONE THING THAT COULD HAVE BEEN SILENTLY WRONG. `codegen` keeps a
// `{{name}}` inside a typed literal only when the name is DECLARED, and turns any other one into
// a CREDENTIAL-STORE READ. So a mark that changed the step text and did not also declare the name
// would save a routine that types whatever is in the store under that word — which compiles,
// runs, saves, and is not what anybody asked for. `declaredFor` is what answers it.
//
// Through the production entrances the whole way: review, simplify, mark, Save, and then the
// question the Play card asks of the saved routine.
func TestASuppliedValueSavesAsSomethingTheRoutineAsksFor(t *testing.T) {
	a := typedAndFolded(t, "say hello")

	was := a.Recording()
	if len(was.Supply) == 0 {
		t.Fatal("nothing to mark")
	}
	if why := a.SupplyDraftText(was.Basis, was.Supply[0].At, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}
	if !strings.Contains(rs[0].Source, "do OS's Type with \"{{item}}\"") {
		t.Errorf("the saved routine does not type the supplied value:\n%s", rs[0].Source)
	}
	if strings.Contains(rs[0].Source, "OS's Secret") {
		t.Errorf("the saved routine reads the credential store instead:\n%s", rs[0].Source)
	}
	// AND THE PLAY CARD ASKS FOR IT, BY NAME, AS TEXT AND NOT AS A SECRET.
	needs := a.PlayNeeds(rs[0].ID)
	if !needs.Needs || len(needs.Fields) != 1 || needs.Fields[0].Name != "item" {
		t.Fatalf("the saved routine asks for %+v", needs.Fields)
	}
	if needs.Fields[0].Secret {
		t.Error("the value is asked of the credential store rather than of the person")
	}
}

// AND A NAME THE GENERATOR MEANS AS A CREDENTIAL IS REFUSED, WITH NOTHING CHANGED.
//
// `codegen.MeansASecret` — `password`, `token`, `pin`, `key` and the rest. A placeholder with one
// of those names is read from the CREDENTIAL STORE rather than typed, so somebody asking for text
// they supply and happening to choose one of those words would get a routine that types whatever
// is saved there. They cannot be expected to know the list, so it is refused and named.
//
// THE SAME REFUSAL AS THE SAVED ROUTINE'S EDIT, because it is the same function: `suppliedName`
// is asked by both. A second copy of these checks is a second place for this one to be missing
// from, and it is the one of them that quietly changes what a routine does.
func TestADraftValueCannotBeCalledSomethingTheGeneratorReadsFromTheStore(t *testing.T) {
	a := typedAndFolded(t, "say hello")
	was := a.Recording()
	if len(was.Supply) == 0 {
		t.Fatal("nothing to mark")
	}

	why := a.SupplyDraftText(was.Basis, was.Supply[0].At, "password")
	if why == "" {
		t.Fatal("a value was named after a credential the generator reads from the " +
			"store, so the routine would type whatever is saved there")
	}
	if !strings.Contains(why, "credential store") {
		t.Errorf("the refusal does not say why: %q", why)
	}
	// AND THE DRAFT IS UNTOUCHED.
	if strings.Contains(strings.Join(a.Recording().Steps, "|"), "{{") {
		t.Errorf("the refused mark changed the draft anyway: %v", a.Recording().Steps)
	}
}

// AND A NAME THAT IS NOT A NAME IS REFUSED.
//
// `argName` is the whole vocabulary — letters, digits, dashes, underscores, at least one of them
// — and an empty box is the common way to arrive here, because the button is beside a field
// somebody may not have filled in.
func TestADraftValueNeedsAName(t *testing.T) {
	a := typedAndFolded(t, "say hello")
	at := a.Recording().Supply[0].At

	for _, bad := range []string{"", "   ", "the item", "item!"} {
		if why := a.SupplyDraftText(a.Recording().Basis, at, bad); why == "" {
			t.Errorf("a value was named %q", bad)
		}
	}
	if strings.Contains(strings.Join(a.Recording().Steps, "|"), "{{") {
		t.Errorf("a refused name changed the draft: %v", a.Recording().Steps)
	}
}

// AND SIMPLIFYING TAKES THE MARKS OFF, HAVING SAID SO FIRST.
//
// `SimplifiedDraft` re-derives the steps from the EVENTS, so a value marked as supplied is typed
// text again, and there is no honest way to carry a position across a list that folded. The
// caption names the values while they are still there — "your marks" is not something anybody can
// check — and the mark really does come off, which is what makes the warning true rather than a
// caution about something that does not happen.
func TestSimplifyingTakesTheSuppliedMarksOffAndSaidSoFirst(t *testing.T) {
	a := typedAndFolded(t, "say hello")
	was := a.Recording()
	if len(was.Supply) == 0 {
		t.Fatal("nothing to mark")
	}
	if why := a.SupplyDraftText(was.Basis, was.Supply[0].At, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}

	// SAID BEFORE IT HAPPENS, AND THE VALUE IS NAMED.
	marked := a.Recording()
	if !strings.Contains(marked.SupplySays, "Simplify") ||
		!strings.Contains(marked.SupplySays, "item") {
		t.Errorf("the caption does not say Simplify would type item again: %q",
			marked.SupplySays)
	}

	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	after := a.Recording()
	if strings.Contains(strings.Join(after.Steps, "|"), "{{item}}") {
		t.Fatal("simplifying kept a mark it cannot honestly carry across a folded list")
	}
	// AND THE CAPTION STOPS WARNING ABOUT A MARK THERE IS NO LONGER.
	if strings.Contains(after.SupplySays, "item") {
		t.Errorf("the caption still names a value that is typed text again: %q",
			after.SupplySays)
	}
	// AND THE STEP IS OFFERED AGAIN, because it types text again.
	if len(after.Supply) == 0 {
		t.Error("the simplified draft offers nothing to ask for, though it types text")
	}
	// AND THE REVIEW CAN SAY BOTH HALVES BESIDE THE BUTTON AND BESIDE SAVE: which values
	// are marked before, and which a simplification took off after. The review states the
	// cost where it is approved, so it needs the names and not only the caption.
	if strings.Join(marked.Marked, ",") != "item" {
		t.Errorf("before simplifying, the draft does not say item is marked: %v", marked.Marked)
	}
	if len(after.Marked) != 0 || strings.Join(after.Unmarked, ",") != "item" {
		t.Errorf("after simplifying, the draft does not say item was taken off: marked %v, "+
			"unmarked %v", after.Marked, after.Unmarked)
	}
}

// AND GOING BACK TO THE ORIGINAL TAKES THEM OFF TOO.
//
// For a plainer reason than Simplify's: the original is the recording as it was MADE, and a mark
// was applied to a list derived from it. A way back that carried the marks would be offering
// somebody the demonstration they gave plus three things they did afterwards.
func TestTheOriginalComesBackWithNoSuppliedMarksOnIt(t *testing.T) {
	a := typedAndFolded(t, "say hello")
	if why := a.SupplyDraftText(a.Recording().Basis, a.Recording().Supply[0].At, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}
	if why := a.UseOriginalDraft(); why != "" {
		t.Fatalf("going back refused: %s", why)
	}
	back := a.Recording()
	if strings.Contains(strings.Join(back.Steps, "|"), "{{item}}") {
		t.Errorf("what came back is not what was demonstrated: %v", back.Steps)
	}
	if strings.Contains(back.SupplySays, "item") {
		t.Errorf("the caption still names a mark that is gone: %q", back.SupplySays)
	}
}

// AND A RECORDING THAT TYPES NOTHING IS OFFERED NOTHING, AND TOLD NOTHING.
//
// The same rule the saved routine's panel follows: a control beside a click would be one whose
// refusal arrives after somebody has typed a name into it. Most recordings of a game are this,
// and they get no control AND no sentence — the advice about simplifying is for a recording that
// types something, and on one that does not it would be a paragraph about a thing nobody is
// trying to do.
func TestARecordingThatTypesNothingIsOfferedNoValueToAskFor(t *testing.T) {
	a := aRecordingUnderReview(t, "pull stone", clickingInNotepad())
	got := a.Recording()
	if len(got.Supply) != 0 {
		t.Errorf("a recording of one click offers %+v", got.Supply)
	}
	if got.SupplySays != "" {
		t.Errorf("a recording that types nothing is told %q", got.SupplySays)
	}
	// AND ASKING ANYWAY IS REFUSED RATHER THAN APPLIED SOMEWHERE.
	if why := a.SupplyDraftText(a.Recording().Basis, 0, "item"); why == "" {
		t.Error("a step that types nothing was made to supply text")
	}
}

// AND A MARK ON AN ORDINARY RECORDING SURVIVES THE SAVE, THE EDIT PANEL AND A REBUILD.
//
// THE LIFECYCLE, END TO END, and the path every other test in this file steps around. The mark
// changes a step's text; from that moment the steps are not a rendering of this demonstration
// under any rules, because the demonstration types `stone` and the steps ask for a value. A save
// writes the step list down beside the routine exactly when that is true — and it asked the wrong
// question: `accountFor` read `Draft.Basis`, which a SIMPLIFIED draft carries and an ordinary one
// does not, so a mark made on the recording as it was taken saved with nothing beside it.
//
// The save itself was right, which is what made this worth finding. The routine ran and asked for
// its value. What broke was everything afterwards, because an absent account LICENSES rendering
// the demonstration and treating the result as what the file is:
//
//	the edit panel REFUSED      — `basisFor` rendered the demonstration, compared it with the
//	                              saved source, and reported the two as no longer agreeing
//	a rebuild DISCARDED the mark — it re-derived from the events and said nothing beforehand
//
// Three claims, in the order somebody meets them, plus the demonstration being the one that was
// made. One mutation, killed: dropping `next.Changed = true` from `SuppliedInDraft` fails this at
// the panel. See `orchestrator.Draft.Changed` and `Draft.ownSteps`.
func TestAMarkedOrdinaryRecordingCanStillBeEditedAndRebuilt(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", briskTypingIntoNotepad())

	was := a.Recording()
	if was.Simplified {
		t.Fatal("the fixture is simplified, which is the path this test is not about")
	}
	if len(was.Supply) != 1 {
		t.Fatalf("a run typed at typing speed offers %d steps to ask for, so this "+
			"cannot reach the state it is about: %v", len(was.Supply), was.Steps)
	}
	if why := a.SupplyDraftText(was.Basis, was.Supply[0].At, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}
	saved := rs[0]
	if !strings.Contains(saved.Source, "{{item}}") {
		t.Fatalf("the saved routine does not ask for the value:\n%s", saved.Source)
	}
	rec, recErr := os.ReadFile(strings.TrimSuffix(saved.Path, ".marco") + ".rec.json")
	if recErr != nil {
		t.Fatalf("the demonstration is not stored beside the routine: %v", recErr)
	}
	// AND THE MARK IS NOT IN IT. The demonstration is stored as it was recorded; a mark
	// is a change to the steps that were derived from it.
	if strings.Contains(string(rec), "item") {
		t.Errorf("the mark was written into the demonstration:\n%s", rec)
	}

	// ── THE EDIT PANEL, WHICH IS WHERE THIS SHOWED ──
	e := a.StepEditing(saved.ID)
	if e.Why != "" {
		t.Fatalf("the routine that was just saved cannot be edited: %s", e.Why)
	}
	if !e.Editable || e.Basis == "" {
		t.Fatalf("the panel offers nothing to ask against: %+v", e)
	}
	var marked *StepRow
	for i := range e.Rows {
		if e.Rows[i].Supplied {
			marked = &e.Rows[i]
		}
	}
	if marked == nil {
		t.Fatalf("no step of the saved routine takes its text from the person: %+v",
			e.Rows)
	}
	if marked.Named != "item" {
		t.Errorf("the supplied step is called %q", marked.Named)
	}

	// ── AND A REBUILD UNDER TODAY'S RULES KEEPS WHAT THE ROUTINE SAYS IT DOES ──
	v := a.ReviewRebuilt(saved.ID, "current")
	if v.Why != "" {
		t.Fatalf("the saved routine cannot be rebuilt: %s", v.Why)
	}
	if !strings.Contains(v.Now, "{{item}}") {
		t.Errorf("a rebuild under today's rules loses the value the routine asks "+
			"for:\n%s", v.Now)
	}
	if strings.Contains(v.Now, "OS's Secret") {
		t.Errorf("a rebuild reads the credential store instead:\n%s", v.Now)
	}

	// ── AND THE DEMONSTRATION IS UNTOUCHED BY ANY OF IT ──
	again, err := os.ReadFile(strings.TrimSuffix(saved.Path, ".marco") + ".rec.json")
	if err != nil {
		t.Fatalf("the demonstration is gone after a review that writes nothing: %v", err)
	}
	if string(again) != string(rec) {
		t.Error("the demonstration beside the routine changed")
	}
}

// AND THE ORIGINAL OF A RECORDING MARKED BEFORE IT WAS SIMPLIFIED COMES BACK DESCRIBED.
//
// A draft can be marked before Simplify is ever pressed, which is the same fact the test above
// turns on — so the draft kept as the way back can be one with placeholders in it. It comes back
// with its marks AND with the sentence about them, as one pair: a restored draft whose steps hold
// `{{item}}` under a card that says nothing about it is the same wrongness as a card warning
// about a mark that is gone, only quieter. One mutation, killed: restoring `wasDraft` and
// clearing `supplied` leaves the placeholder on screen undescribed. See capture.wasSupplied.
func TestTheOriginalOfAMarkedRecordingComesBackWithItsMarkDescribed(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", briskTypingIntoNotepad())
	at := a.Recording().Supply[0].At
	if why := a.SupplyDraftText(a.Recording().Basis, at, "item"); why != "" {
		t.Fatalf("marking refused: %s", why)
	}
	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	// THE SIMPLIFICATION RE-DERIVED, so the mark is gone from the steps it was on.
	if mid := a.Recording(); strings.Contains(strings.Join(mid.Steps, "|"), "{{item}}") {
		t.Fatalf("a re-derivation kept a mark it cannot place: %v", mid.Steps)
	}
	if why := a.UseOriginalDraft(); why != "" {
		t.Fatalf("going back refused: %s", why)
	}
	back := a.Recording()
	if !strings.Contains(strings.Join(back.Steps, "|"), "{{item}}") {
		t.Fatalf("the draft that came back is not the one that was marked: %v",
			back.Steps)
	}
	if !strings.Contains(back.SupplySays, "item") {
		t.Errorf("the card says nothing about the placeholder in the steps in front "+
			"of somebody: %q", back.SupplySays)
	}
}
