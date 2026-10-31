package main

// MAKING A STEP'S TEXT SOMETHING YOU SUPPLY, FROM THE PANEL THAT SHOWS THE ROUTINE.
//
// The end of this file is the claim the feature exists for: a routine that typed fixed text is
// edited, saved, and then OFFERS A FIELD BESIDE PLAY. Everything the field, the confirmation and
// a request's proposal are built on reads `routes.Arguments`, so an edit that produces a
// placeholder those cannot read back would be a step typing two braces and a word.
//
// It was already possible two ways and both were invisible — name the recording "… with message",
// or tap f9 during the demonstration. This needs neither, and works on routines saved before any
// of it existed.
//
// See [[ADR-276-a-routine-may-be-told-what-to-type]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// typingWordsIntoNotepad types "hello" as five consecutive characters, which `simplify` folds
// into ONE typed run — the step kind this feature is about. A single keypress stays a key press.
func typingWordsIntoNotepad() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	ms := 10
	for _, k := range []string{"h", "e", "l", "l", "o"} {
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: true,
				T: at(ms), App: "notepad"},
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: false,
				T: at(ms + 5), App: "notepad"})
		ms += 20
	}
	return out
}

// typingTwoRunsIntoNotepad types "hello", presses tab, then types "bye" — TWO typed runs, so a
// second edit has something of its own to change.
//
// It exists because the one-run fixture has exactly two steps, an activate and a type, and the
// trailing 250ms is `codegen`'s settle rather than a step: a test that reached for a wait to
// change found none and SKIPPED, which the gate reported as an undeclared skip. The claim it was
// written for — that an edited routine can be edited again — was not being made at all.
func typingTwoRunsIntoNotepad() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	ms := 10
	press := func(k string) {
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: true,
				T: at(ms), App: "notepad"},
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: false,
				T: at(ms + 5), App: "notepad"})
		ms += 20
	}
	for _, k := range []string{"h", "e", "l", "l", "o"} {
		press(k)
	}
	press("tab")
	for _, k := range []string{"b", "y", "e"} {
		press(k)
	}
	return out
}

// theTypingRow is the one row that types text, and the panel it came from.
func theTypingRow(t *testing.T, a *Acts, id string) (StepEditing, StepRow) {
	t.Helper()
	e := a.StepEditing(id)
	if e.Why != "" {
		t.Fatalf("the panel refused: %s", e.Why)
	}
	var found []StepRow
	for _, row := range e.Rows {
		if row.Types {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d rows report typing text; the fixture types one run:\n%+v",
			len(found), e.Rows)
	}
	return e, found[0]
}

// THE ROW SAYS IT TYPES TEXT, AND WHAT THE TEXT IS.
func TestTheStepPanelSaysWhichStepTypesText(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}
	e, row := theTypingRow(t, a, rs[0].ID)

	if row.Text != "hello" {
		t.Errorf("the row says it types %q", row.Text)
	}
	if row.Supplied {
		t.Errorf("fixed text is reported as supplied: %+v", row)
	}
	// AND NO OTHER ROW OFFERS IT. A box beside a click would be a control that refuses.
	for _, other := range e.Rows {
		if other.At == row.At {
			continue
		}
		if other.Types || other.Text != "" {
			t.Errorf("step %d offers text it does not type: %+v", other.At+1, other)
		}
	}
}

// AND THE EDIT MAKES THE ROUTINE TAKE THAT TEXT, WHICH IS THE WHOLE POINT.
func TestSupplyingAStepsTextMakesTheRoutineAskForIt(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	rs := a.Routines()
	id := rs[0].ID
	e, row := theTypingRow(t, a, id)

	// BEFORE: it needs nothing supplied.
	if before := a.PlayNeeds(id); before.Needs {
		t.Fatalf("it already needs something: %+v", before.Fields)
	}

	p := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: row.At, Name: "message"}})
	if p.Why != "" || p.Token == "" {
		t.Fatalf("the review refused: %q", p.Why)
	}
	// AND SAYS WHAT IT WOULD DO, naming the argument and the text it replaces.
	said := strings.Join(p.Doing, " ")
	if !strings.Contains(said, "message") || !strings.Contains(said, "hello") {
		t.Errorf("the review says %v", p.Doing)
	}
	if done := a.SaveStepEdit(p.Token); done.Why != "" {
		t.Fatalf("the save refused: %s", done.Why)
	}

	// AFTER: the field beside Play, by name, and not a secret.
	after := a.PlayNeeds(id)
	if !after.Needs {
		t.Fatalf("the edited routine needs nothing supplied: %+v", after)
	}
	if len(after.Fields) != 1 {
		t.Fatalf("it offers %d field(s): %+v", len(after.Fields), after.Fields)
	}
	if after.Fields[0].Name != "message" {
		t.Errorf("the field is called %q", after.Fields[0].Name)
	}
	if after.Fields[0].Secret {
		t.Error("supplied text is offered as a secret")
	}
	// AND PLAY REFUSES UNTIL IT IS GIVEN ONE, which is the contract that was already
	// there — this feature's whole job is to get a routine INTO that state.
	if got := a.ReviewPlay(id, nil, nil, ""); got.Token != "" {
		t.Error("the edited routine may be run with no value given")
	}
	if got := a.ReviewPlay(id, map[string]string{"message": "goodbye"}, nil,
		""); got.Token == "" {
		t.Errorf("the edited routine refuses a value it asked for: %q", got.Why)
	}
}

// AND THE ROW THEN SAYS THE TEXT IS SUPPLIED, rather than offering the control again.
func TestAnAlreadySuppliedStepSaysSoInsteadOfOfferingAgain(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	id := a.Routines()[0].ID
	e, row := theTypingRow(t, a, id)
	p := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: row.At, Name: "message"}})
	if p.Why != "" {
		t.Fatalf("the review refused: %s", p.Why)
	}
	if done := a.SaveStepEdit(p.Token); done.Why != "" {
		t.Fatalf("the save refused: %s", done.Why)
	}

	after := a.StepEditing(id)
	if after.Why != "" {
		t.Fatalf("the panel refused after the edit: %s", after.Why)
	}
	var supplied []StepRow
	for _, r := range after.Rows {
		if r.Supplied {
			supplied = append(supplied, r)
		}
		if r.Types {
			t.Errorf("step %d still offers to be supplied: %+v", r.At+1, r)
		}
	}
	if len(supplied) != 1 {
		t.Fatalf("%d rows say the text is supplied:\n%+v", len(supplied), after.Rows)
	}
	if supplied[0].Named != "message" {
		t.Errorf("the row says it is called %q", supplied[0].Named)
	}
}

// THE ROUTINE IT WRITES TYPES THE VALUE, AND DOES NOT READ THE CREDENTIAL STORE.
//
// THE DEFECT THIS FEATURE SHIPPED WITH FOR AN HOUR, and no assertion about the step list could
// see it. `codegen` keeps a `{{name}}` inside a typed literal only when the name is DECLARED, and
// turns any other one into `do OS's Secret …` — so the first version of this rendered
// `do OS's Secret with "message"` where `do OS's Type with "{{message}}"` belonged: a routine
// that reads somebody's credential store where they asked for their own words. Every test passed,
// because every test was about the steps.
//
// So this one reads the SOURCE that would be written.
func TestTheEditedRoutineTypesTheValueRatherThanReadingACredential(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	id := a.Routines()[0].ID
	e, row := theTypingRow(t, a, id)
	p := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: row.At, Name: "message"}})
	if p.Why != "" {
		t.Fatalf("the review refused: %s", p.Why)
	}
	if !strings.Contains(p.Now, `do OS's Type with "{{message}}"`) {
		t.Errorf("the routine it would write does not type the value:\n%s", p.Now)
	}
	if strings.Contains(p.Now, "OS's Secret") {
		t.Errorf("the routine it would write reads the credential store:\n%s", p.Now)
	}
	// AND THE TEXT IT REPLACED IS GONE FROM IT, which is the point of the edit.
	if strings.Contains(p.Now, `with "hello"`) {
		t.Errorf("the fixed text is still in it:\n%s", p.Now)
	}
}

// AND AN EDITED ROUTINE CAN BE EDITED AGAIN, WITH THE FIRST EDIT INTACT.
//
// The regression the render fix had to carry: the panel re-renders the stored steps to check they
// still reproduce the saved source, and a `{{message}}` with nothing declaring it renders to a
// credential read — so the SECOND edit of an edited routine refused, saying the routine no longer
// agreed with itself. The placeholder in the step list is the declaration now.
//
// TWO TYPED RUNS, so the second edit is one of this feature's own rather than a wait's duration.
// The first version of this test reached for a wait, found none — the fixture is an activate and a
// type, and the trailing settle is not a step — and SKIPPED, which the gate reported as an
// undeclared skip. It was passing while making no claim.
func TestAnEditedRoutineCanBeEditedAgain(t *testing.T) {
	a := aStudioLibrary(t, "say two things", typingTwoRunsIntoNotepad())
	id := a.Routines()[0].ID

	// THE FIRST RUN BECOMES SOMETHING SUPPLIED.
	first := a.StepEditing(id)
	if first.Why != "" {
		t.Fatalf("the panel refused: %s", first.Why)
	}
	var typing []StepRow
	for _, r := range first.Rows {
		if r.Types {
			typing = append(typing, r)
		}
	}
	if len(typing) != 2 {
		t.Fatalf("%d rows type text; the fixture types two runs:\n%+v",
			len(typing), first.Rows)
	}
	p := a.ReviewStepEdit(id, first.Basis, []StepChange{
		{Op: "argument", At: typing[0].At, Name: "message"}})
	if p.Why != "" {
		t.Fatalf("the first review refused: %s", p.Why)
	}
	if done := a.SaveStepEdit(p.Token); done.Why != "" {
		t.Fatalf("the first save refused: %s", done.Why)
	}

	// AND THE SECOND RUN, ON THE ROUTINE AS THE FIRST EDIT LEFT IT.
	again := a.StepEditing(id)
	if again.Why != "" {
		t.Fatalf("the panel refused the second edit: %s", again.Why)
	}
	var next StepRow
	var supplied int
	for _, r := range again.Rows {
		if r.Types {
			next = r
		}
		if r.Supplied {
			supplied++
		}
	}
	if supplied != 1 {
		t.Fatalf("%d rows report supplied text after one edit:\n%+v",
			supplied, again.Rows)
	}
	if !next.Types {
		t.Fatalf("no row still types fixed text:\n%+v", again.Rows)
	}
	p2 := a.ReviewStepEdit(id, again.Basis, []StepChange{
		{Op: "argument", At: next.At, Name: "farewell"}})
	if p2.Why != "" || p2.Token == "" {
		t.Fatalf("the second review refused: %q", p2.Why)
	}
	// AND BOTH ARE TYPED, NEITHER IS READ FROM THE CREDENTIAL STORE.
	for _, want := range []string{
		`do OS's Type with "{{message}}"`, `do OS's Type with "{{farewell}}"`,
	} {
		if !strings.Contains(p2.Now, want) {
			t.Errorf("the second edit would not write %s:\n%s", want, p2.Now)
		}
	}
	if strings.Contains(p2.Now, "OS's Secret") {
		t.Errorf("the second edit reads the credential store:\n%s", p2.Now)
	}
	if done := a.SaveStepEdit(p2.Token); done.Why != "" {
		t.Fatalf("the second save refused: %s", done.Why)
	}

	// AND THE ROUTINE NOW ASKS FOR BOTH, in the order its steps hold them.
	after := a.PlayNeeds(id)
	if !after.Needs || len(after.Fields) != 2 {
		t.Fatalf("it asks for %+v", after.Fields)
	}
	if after.Fields[0].Name != "message" || after.Fields[1].Name != "farewell" {
		t.Errorf("it asks for %q then %q", after.Fields[0].Name, after.Fields[1].Name)
	}
}

// AND A REBUILD OF AN EDITED ROUTINE NEVER PRODUCES A CREDENTIAL READ EITHER.
//
// The third place the Type-never-Secret rule has to hold. A rebuild re-renders from the
// DEMONSTRATION, which is deliberately never edited — so what it produces is the routine as it was
// recorded, with the fixed text back. That is correct and is not a loss of the edit: the review
// shows both sources and nothing is written until somebody replaces it.
//
// What must never happen in any of it is a `do OS's Secret` appearing where supplied text
// belonged. This asks both rebuild kinds.
//
// Both halves are asserted, because "the placeholder is gone" and "it reads a credential" are
// different things and only one of them is correct.
func TestARebuildOfAnEditedRoutineNeverReadsACredential(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	id := a.Routines()[0].ID
	e, row := theTypingRow(t, a, id)
	p := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: row.At, Name: "message"}})
	if p.Why != "" {
		t.Fatalf("the edit refused: %s", p.Why)
	}
	if done := a.SaveStepEdit(p.Token); done.Why != "" {
		t.Fatalf("the save refused: %s", done.Why)
	}
	// THE EDIT LANDED: the routine asks for text.
	if before := a.PlayNeeds(id); !before.Needs {
		t.Fatalf("the edit did not land: %+v", before)
	}

	for _, kind := range []string{"current", "simplify-typing"} {
		r := a.ReviewRebuilt(id, kind)
		if r.Why != "" {
			t.Fatalf("the %s rebuild refused: %s", kind, r.Why)
		}
		// NEITHER SOURCE IT SHOWS READS A CREDENTIAL. Both, because the review is
		// a comparison and somebody reads the left side too.
		for what, src := range map[string]string{"was": r.Was, "now": r.Now} {
			if strings.Contains(src, "OS's Secret") {
				t.Errorf("the %s rebuild's %s source reads the credential "+
					"store:\n%s", kind, what, src)
			}
		}
		// AND IT TYPES SOMETHING. A rebuild comes from the demonstration, so the
		// fixed text is back — which is the rebuild working, not the edit leaking.
		if !strings.Contains(r.Now, "OS's Type") {
			t.Errorf("the %s rebuild types nothing:\n%s", kind, r.Now)
		}
		// AND THE REVIEW IS PUT AWAY, so the next kind is prepared against a
		// routine with nothing already pending.
		a.CancelReview(r.Token)
	}

	// AND NOTHING WAS REPLACED BY LOOKING, so the edited routine still asks for text.
	if after := a.PlayNeeds(id); !after.Needs || len(after.Fields) != 1 ||
		after.Fields[0].Name != "message" {
		t.Errorf("reviewing a rebuild changed the saved routine: %+v", after)
	}
}

// AND THE TWO REBUILD ARMS DO DIFFERENT THINGS TO THE EDIT, AND NEITHER READS A CREDENTIAL.
//
// Written after this test asserted the wrong thing, which is worth recording because the truth is
// the better behaviour:
//
//   - TODAY'S RULES rebuilds from the routine's stored step ACCOUNT when it has one, and the
//     account is what the edit changed — so the supplied value SURVIVES a rebuild. The review
//     says so (`DiscardsEdits` is false).
//   - SIMPLIFY-TYPING re-derives from the DEMONSTRATION, which is never edited, so it gives back
//     the text as it was typed and the routine asks for nothing. The review says THAT too, which
//     is what `DiscardsEdits` is for.
//
// Both are correct and they are not the same, so a person is told which one they are looking at.
// What must hold either way is that the text is TYPED and never read from the credential store.
func TestTodaysRulesKeepsTheSuppliedValueAndSimplifyingGivesBackTheText(t *testing.T) {
	supplied := func(t *testing.T) (*Acts, string) {
		t.Helper()
		a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
		id := a.Routines()[0].ID
		e, row := theTypingRow(t, a, id)
		p := a.ReviewStepEdit(id, e.Basis, []StepChange{
			{Op: "argument", At: row.At, Name: "message"}})
		if p.Why != "" {
			t.Fatalf("the edit refused: %s", p.Why)
		}
		if done := a.SaveStepEdit(p.Token); done.Why != "" {
			t.Fatalf("the save refused: %s", done.Why)
		}
		return a, id
	}

	// ── TODAY'S RULES: THE EDIT SURVIVES ──
	a, id := supplied(t)
	r := a.ReviewRebuilt(id, "current")
	if r.Why != "" {
		t.Fatalf("the rebuild refused: %s", r.Why)
	}
	// AND THE REVIEW SAYS IT KEEPS THEM, before anything is pressed.
	if r.Discards {
		t.Error("today's-rules rebuild says it discards the edit, and it does not")
	}
	if why := a.ReplaceWithRebuilt(r.Token); why != "" {
		t.Fatalf("replacing refused: %s", why)
	}
	kept := a.Routines()[0].Source
	if !strings.Contains(kept, `do OS's Type with "{{message}}"`) {
		t.Errorf("today's rules lost the supplied value:\n%s", kept)
	}
	if strings.Contains(kept, "OS's Secret") {
		t.Errorf("today's rules rendered a credential read:\n%s", kept)
	}
	if after := a.PlayNeeds(id); !after.Needs || len(after.Fields) != 1 ||
		after.Fields[0].Name != "message" {
		t.Errorf("the rebuilt routine no longer asks for the value: %+v", after)
	}

	// ── SIMPLIFY-TYPING: THE DEMONSTRATION COMES BACK ──
	b, bid := supplied(t)
	s := b.ReviewRebuilt(bid, "simplify-typing")
	if s.Why != "" {
		t.Fatalf("the simplifying rebuild refused: %s", s.Why)
	}
	// AND IT SAYS IT DISCARDS THE EDIT, which is the whole reason that field exists.
	if !s.Discards {
		t.Error("the simplifying rebuild does not say it discards the edit")
	}
	if strings.Contains(s.Now, "OS's Secret") {
		t.Errorf("the simplifying rebuild rendered a credential read:\n%s", s.Now)
	}
	if !strings.Contains(s.Now, `do OS's Type with "hello"`) {
		t.Errorf("the simplifying rebuild does not type what was demonstrated:\n%s",
			s.Now)
	}
	if why := b.ReplaceWithRebuilt(s.Token); why != "" {
		t.Fatalf("replacing refused: %s", why)
	}
	if after := b.PlayNeeds(bid); after.Needs {
		t.Errorf("after going back to the demonstration it still asks for text: %+v",
			after.Fields)
	}
}

// AND A NAME THE CONTRACT WILL NOT HAVE IS REFUSED BY THE WINDOW TOO.
//
// In the contract's own words. A window with its own copy of the rule is a window that offers a
// press and is then told off for it — and the pattern that decides is `routes`' own.
func TestTheWindowRefusesANameThePlaceholderCannotHave(t *testing.T) {
	a := aStudioLibrary(t, "say hello", typingWordsIntoNotepad())
	id := a.Routines()[0].ID
	e, row := theTypingRow(t, a, id)

	for _, name := range []string{"the message", "message!", "{{message}}", ""} {
		p := a.ReviewStepEdit(id, e.Basis, []StepChange{
			{Op: "argument", At: row.At, Name: name}})
		if p.Token != "" {
			t.Errorf("%q was accepted", name)
			continue
		}
		if !strings.Contains(p.Why, "not a name") {
			t.Errorf("%q was refused as %q", name, p.Why)
		}
	}
	// AND NOTHING WAS WRITTEN BY ANY OF THEM.
	if after := a.PlayNeeds(id); after.Needs {
		t.Errorf("a refused name left the routine asking for something: %+v", after.Fields)
	}
}
