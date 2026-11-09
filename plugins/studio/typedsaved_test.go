package main

// SAYING A RUN IS TYPED TEXT ON A ROUTINE THAT IS ALREADY SAVED.
//
// THE REASON THIS HALF IS NOT OPTIONAL. The recording that exposed the defect is on the owner's
// disk — `cloud pull stone`, saved on 30 October with its word written down as four held keys and
// a tap. A repair reachable only from the recording review would mean recording it again, which is
// asking somebody to redo a demonstration because of how it was filed.
//
// Same validator, same proposals: `orchestrator.TypedRunsIn` offers the runs and
// `orchestrator.EditTyped` applies one through `typedRun`. What is different is the contract
// around it — a saved routine's edit goes through `ReviewStepEdit` and `SaveStepEdit`, which is
// nothing written until somebody has seen both sources.
//
// See [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

import (
	"strings"
	"testing"
)

// aSavedRollover is the owner's measured typing, saved as a routine the ordinary way.
//
// SAVED UNSIMPLIFIED AND UNDECLARED, which is the state the real one is in: held keys, no step
// account beside it, and a source that is exactly what its own demonstration renders to.
func aSavedRollover(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving the fixture refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays", len(rs))
	}
	if !strings.Contains(rs[0].Source, `KeyDown with "s"`) {
		t.Fatalf("the saved fixture does not hold its keys, so these tests cannot "+
			"reach the state they are about:\n%s", rs[0].Source)
	}
	return a, rs[0]
}

// THE PANEL OFFERS THE RUN, AND DECLARING IT REPAIRS THE SAVED ROUTINE.
func TestASavedRolloverCanBeDeclaredTypedText(t *testing.T) {
	a, saved := aSavedRollover(t)

	e := a.StepEditing(saved.ID)
	if e.Why != "" {
		t.Fatalf("the saved play cannot be edited: %s", e.Why)
	}
	if len(e.TypedRuns) != 1 {
		t.Fatalf("%d runs are offered for one word: %+v", len(e.TypedRuns), e.TypedRuns)
	}
	if !strings.Contains(e.TypedRuns[0].Label, `types "stone"`) {
		t.Errorf("the option does not say what the run would become: %q",
			e.TypedRuns[0].Label)
	}
	for _, want := range []string{"held keys", "waits between them go"} {
		if !strings.Contains(e.TypedSays, want) {
			t.Errorf("the panel does not say what declaring costs (%q): %q", want,
				e.TypedSays)
		}
	}

	run := e.TypedRuns[0]
	p := a.ReviewStepEdit(saved.ID, e.Basis, []StepChange{
		{Op: "typed", At: run.From, Upto: run.To}})
	if p.Why != "" || p.Refused != "" {
		t.Fatalf("the declaration was refused: %s %s", p.Why, p.Refused)
	}
	if !p.Savable {
		t.Fatalf("the declaration produced nothing to save: %+v", p)
	}
	if !strings.Contains(p.Now, `do OS's Type with "stone"`) {
		t.Fatalf("the version offered does not type the word:\n%s", p.Now)
	}
	if strings.Contains(p.Now, "KeyDown") {
		t.Errorf("the version offered still holds the keys:\n%s", p.Now)
	}
	// AND NOTHING IS WRITTEN UNTIL IT IS SAVED.
	if now := a.Routines()[0].Source; !strings.Contains(now, `KeyDown with "s"`) {
		t.Error("reviewing the declaration wrote to the library")
	}

	done := a.SaveStepEdit(p.Token)
	if done.Why != "" || !done.Saved {
		t.Fatalf("saving the declaration refused: %+v", done)
	}
	if done.Partial {
		t.Errorf("the play was written without the step list beside it: %s",
			done.Says)
	}
	after := a.Routines()[0].Source
	if !strings.Contains(after, `do OS's Type with "stone"`) {
		t.Fatalf("the saved play does not type the word:\n%s", after)
	}
}

// AND THE REPAIRED ROUTINE CAN THEN BE ASKED FOR ITS VALUE.
//
// THE JOURNEY FROM A RECORDING THAT WAS ALREADY FILED, which is the one increment 1 could not
// reach. Held keys on disk, declared, and then the word becomes something the routine asks for.
func TestARepairedRoutineCanBeAskedForItsValue(t *testing.T) {
	a, saved := aSavedRollover(t)
	e := a.StepEditing(saved.ID)
	run := e.TypedRuns[0]
	p := a.ReviewStepEdit(saved.ID, e.Basis, []StepChange{
		{Op: "typed", At: run.From, Upto: run.To}})
	if done := a.SaveStepEdit(p.Token); !done.Saved {
		t.Fatalf("saving the declaration refused: %+v", done)
	}

	// ── AND THE PANEL CAN BE ASKED AGAIN, which needs the account the save left ──
	again := a.StepEditing(saved.ID)
	if again.Why != "" {
		t.Fatalf("the repaired play cannot be edited: %s", again.Why)
	}
	typed := -1
	for _, row := range again.Rows {
		if row.Types && row.Text == "stone" {
			typed = row.At
		}
	}
	if typed < 0 {
		t.Fatalf("no step of the repaired play types the word: %+v", again.Rows)
	}
	v := a.ReviewStepEdit(saved.ID, again.Basis, []StepChange{
		{Op: "argument", At: typed, Name: "item"}})
	if v.Why != "" || v.Refused != "" {
		t.Fatalf("asking for the value refused: %s %s", v.Why, v.Refused)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("saving the value refused: %+v", done)
	}
	src := a.Routines()[0].Source
	if !strings.Contains(src, `do OS's Type with "{{item}}"`) {
		t.Fatalf("the play does not ask for the value:\n%s", src)
	}
	if strings.Contains(src, "OS's Secret") {
		t.Errorf("the play reads the credential store instead:\n%s", src)
	}
	needs := a.PlayNeeds(saved.ID)
	if !needs.Needs || len(needs.Fields) != 1 || needs.Fields[0].Name != "item" {
		t.Fatalf("the Play card asks for %+v", needs.Fields)
	}

	// ── AND A REBUILD UNDER TODAY'S RULES KEEPS BOTH ──
	//
	// Which is where an account that had not been written would undo all of it: the
	// demonstration still types held keys, so a rebuild that re-derived would come back
	// with the word shredded and the value gone.
	rb := a.ReviewRebuilt(saved.ID, "current")
	if rb.Why != "" {
		t.Fatalf("the repaired play cannot be rebuilt: %s", rb.Why)
	}
	if !strings.Contains(rb.Now, `{{item}}`) || strings.Contains(rb.Now, "KeyDown") {
		t.Errorf("a rebuild under today's rules undid the repair:\n%s", rb.Now)
	}
}

// AND A DECLARATION ASKED ALONGSIDE ANOTHER CHANGE IS REFUSED, WITH THE REASON.
//
// It is the one operation that changes how many steps there are, so every position the other
// changes were read off moves under it, and nothing says whether a position was read before or
// after the run. Refused before anything is applied rather than translated.
func TestADeclarationAskedWithAnotherChangeIsRefused(t *testing.T) {
	a, saved := aSavedRollover(t)
	e := a.StepEditing(saved.ID)
	run := e.TypedRuns[0]
	wait := -1
	for _, row := range e.Rows {
		if row.Wait {
			wait = row.At
			break
		}
	}
	if wait < 0 {
		t.Fatal("the fixture has no wait to change alongside the declaration")
	}

	p := a.ReviewStepEdit(saved.ID, e.Basis, []StepChange{
		{Op: "typed", At: run.From, Upto: run.To},
		{Op: "wait", At: wait, To: 800}})
	if p.Why == "" {
		t.Fatal("a declaration was applied alongside a change counted against the " +
			"list it shortens")
	}
	if !strings.Contains(p.Why, "on its own") {
		t.Errorf("the refusal does not say what to do instead: %q", p.Why)
	}
	// AND NOTHING WAS WRITTEN.
	if !strings.Contains(a.Routines()[0].Source, `KeyDown with "s"`) {
		t.Error("the refused edit changed the play")
	}
}

// AND A RANGE THE VALIDATOR REFUSES IS REFUSED HERE TOO, IN THE SAME WORDS.
//
// The point being that the saved panel and the recording review share `typedRun` rather than each
// having an opinion: a range holding something that is not typing is not typing in either place.
func TestASavedDeclarationObeysTheSameRules(t *testing.T) {
	a, saved := aSavedRollover(t)
	e := a.StepEditing(saved.ID)

	// STEP 1 IS THE ACTIVATE, which is not typing.
	p := a.ReviewStepEdit(saved.ID, e.Basis, []StepChange{
		{Op: "typed", At: 0, Upto: 2}})
	if p.Why == "" {
		t.Fatal("a range holding an activate was declared typed text")
	}
	if !strings.Contains(p.Why, "not typing") {
		t.Errorf("the refusal does not say why: %q", p.Why)
	}
}
