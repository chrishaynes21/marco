package main

// RENAMING A SAVED ROUTINE, AGAINST A REAL LIBRARY.
//
// The move contract itself — both locks, both revalidations, the exclusive create, the
// destination read back, the removal that comes last — lives in `internal/orchestrator` and is
// held by the rescope tests. These ask what a RENAME adds to it: that the arguments survive the
// box, that a collision is refused with nothing written, that a name nobody changed is refused
// before anything is locked, and that a routine reading a credential under its own name is
// refused rather than quietly pointed at a credential nobody stored.
//
// See [[ADR-285-a-routine-can-be-called-something-else]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A RENAME MOVES THE ROUTINE, KEEPS WHERE IT WORKS, AND LEAVES NOTHING AT THE OLD NAME.
func TestARoutineCanBeCalledSomethingElse(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	was := r.Scope
	if was == "" {
		t.Fatal("the fixture has no scope, so this cannot say one was preserved")
	}

	m := a.ReviewRename(r.ID, "post my address")
	if m.Why != "" || m.Refused != "" {
		t.Fatalf("the rename was refused: %s %s", m.Why, m.Refused)
	}
	if !m.Movable || m.Token == "" {
		t.Fatalf("the review produced nothing to perform: %+v", m)
	}
	if m.Renaming != "post my address" {
		t.Errorf("the review says it would be called %q", m.Renaming)
	}
	// ── AND NOTHING IS WRITTEN UNTIL IT IS ──
	if now := a.Routines(); len(now) != 1 || now[0].Slug != r.Slug {
		t.Fatalf("the review changed the library: %+v", now)
	}

	done := a.RenameRoutine(m.Token)
	if done.Why != "" {
		t.Fatalf("renaming refused: %s", done.Why)
	}
	if !done.Moved {
		t.Fatalf("the original is still there, so this is two routines: %+v", done)
	}
	after := a.Routines()
	if len(after) != 1 {
		t.Fatalf("the library holds %d routines after a rename: %+v", len(after), after)
	}
	if after[0].Slug != "post-my-address" {
		t.Errorf("it is filed under %q", after[0].Slug)
	}
	if after[0].Name != "post my address" {
		t.Errorf("it calls itself %q", after[0].Name)
	}
	if after[0].Scope != was {
		t.Errorf("it works %q now and worked %q before, so a rename moved it as "+
			"well as named it", after[0].Scope, was)
	}
	// ── AND ITS DEMONSTRATION CAME WITH IT, which is what a rebuild needs ──
	reg := routes.Registry{Dir: a.routesDir}
	if _, ok := reg.LoadRecording(after[0].route()); !ok {
		t.Error("the renamed routine has no demonstration beside it, so it can " +
			"never be rebuilt or moved again")
	}
	if _, ok := reg.LoadRecording(r.route()); ok {
		t.Error("the demonstration is still lying under the old slug, where the " +
			"next routine saved there would inherit it")
	}
}

// AND THE VALUES IT ASKS FOR SURVIVE THE BOX.
//
// THE DEFECT THIS IS SHAPED AFTER. The name is where an argument is DECLARED — `routes.SplitArgs`
// reads `with item` off it — so a free-text rename box is a way to drop one by retyping, and the
// routine would then type the literal text of the demonstration the next time somebody played it,
// with nothing said. The box holds the base name and the clause is carried by the backend.
func TestARenameKeepsTheValuesTheRoutineAsksFor(t *testing.T) {
	a, r := aRoutineThatAsksForAValue(t)
	if !strings.Contains(r.Source, "{{item}}") {
		t.Fatalf("the fixture does not ask for a value:\n%s", r.Source)
	}
	needs := a.PlayNeeds(r.ID)
	if !needs.Needs {
		t.Fatal("the fixture's Play card asks for nothing")
	}

	m := a.ReviewRename(r.ID, "post my address")
	if m.Why != "" || m.Refused != "" {
		t.Fatalf("the rename was refused: %s %s", m.Why, m.Refused)
	}
	// THE CLAUSE IS SHOWN, so somebody about to retype a name can see the part they
	// are not being asked to retype.
	if m.Clause != "with item" {
		t.Errorf("the review does not show the clause it keeps: %q", m.Clause)
	}
	if m.Renaming != "post my address" {
		t.Errorf("the name it would be saved under is %q", m.Renaming)
	}
	// AND THE VERSION IT WOULD WRITE STILL ASKS FOR THE VALUE, before anything is
	// saved: a review that showed a source with the placeholder gone would be the
	// one place somebody could have seen it go.
	if !strings.Contains(m.Now, "{{item}}") {
		t.Fatalf("the version a rename would write does not ask for the "+
			"value:\n%s", m.Now)
	}
	if done := a.RenameRoutine(m.Token); done.Why != "" {
		t.Fatalf("renaming refused: %s", done.Why)
	}

	after := a.Routines()[0]
	if after.Name != "post my address" {
		t.Fatalf("the renamed routine is called %q", after.Name)
	}
	if !strings.Contains(after.Source, "{{item}}") {
		t.Errorf("the saved source no longer has the placeholder:\n%s", after.Source)
	}
	if n := a.PlayNeeds(after.ID); !n.Needs || len(n.Fields) != 1 ||
		n.Fields[0].Name != "item" {
		t.Fatalf("the renamed routine's Play card asks for %+v", n.Fields)
	}
}

// AND TYPING A CLAUSE INTO THE BOX IS REFUSED RATHER THAN MERGED.
//
// Somebody typing `with …` means to change what the routine asks for, and a rename is not the
// operation that does that. Merging would produce either a duplicated clause or a quiet
// disagreement between the name and the placeholders in the source.
func TestAClauseTypedIntoTheRenameBoxIsRefused(t *testing.T) {
	a, r := aRoutineThatAsksForAValue(t)
	m := a.ReviewRename(r.ID, "post my address with quantity")
	if m.Why == "" && m.Refused == "" {
		t.Fatal("a rename changed what the routine asks for")
	}
	said := m.Why + m.Refused
	if !strings.Contains(said, "with") {
		t.Errorf("the refusal does not say what to leave off: %q", said)
	}
	if a.Routines()[0].Name != r.Name {
		t.Error("the refused rename changed the routine")
	}
}

// AND A NAME ALREADY TAKEN IS REFUSED, WITH NOTHING OVERWRITTEN AND NOTHING TO AGREE TO.
//
// The move engine can replace what is at a destination under a version-bound consent. A rename
// offers no such press: two routines of one name in one place IS the collision, and the answer is
// a different name.
func TestARenameOntoAnExistingNameIsRefused(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	// A SECOND ROUTINE, IN THE SAME PLACE, under the name the first wants.
	d := orchestrator.Deps{Reg: routes.Registry{Dir: a.routesDir},
		Out: &strings.Builder{}}
	dr := d.DraftOf("post my address", typingIntoNotepad(), "notepad", "the stop key",
		orchestrator.Unstated)
	if _, err := d.Save(dr, orchestrator.ScopeAppAnywhere, "notepad"); err != nil {
		t.Fatalf("saving the second fixture: %v", err)
	}
	other := ""
	for _, x := range a.Routines() {
		if x.Slug == "post-my-address" {
			other = x.Source
		}
	}
	if other == "" {
		t.Fatal("the second fixture is not in the library")
	}

	m := a.ReviewRename(r.ID, "post my address")
	if m.Refused == "" && m.Why == "" {
		t.Fatal("a rename onto an occupied name was offered")
	}
	if m.Movable {
		t.Error("the refused rename is still movable")
	}
	// AND THERE IS NOTHING TO AGREE TO: the page is never shown a replace option.
	if m.Taken || m.TakenSays != "" {
		t.Errorf("a rename offered to replace what is in the way: %+v", m)
	}
	if !strings.Contains(m.Refused+m.Why, "already") {
		t.Errorf("the refusal does not say what is in the way: %q", m.Refused+m.Why)
	}
	// ── AND BOTH ROUTINES ARE EXACTLY AS THEY WERE ──
	after := a.Routines()
	if len(after) != 2 {
		t.Fatalf("the library holds %d routines: %+v", len(after), after)
	}
	for _, x := range after {
		if x.Slug == "post-my-address" && x.Source != other {
			t.Error("the routine that was in the way has been written over")
		}
	}
}

// AND THE NAME IT ALREADY HAS IS REFUSED BEFORE ANYTHING IS LOCKED.
//
// The destination identity would be the source identity: one lock taken twice, an exclusive
// create onto the file being moved, and a removal of what had just been written. Every one of
// those is a way to lose a routine answering a question nobody asked.
func TestRenamingARoutineToItsOwnNameIsRefused(t *testing.T) {
	a, r := aMovableRoutineInStudio(t)
	for _, same := range []string{r.Name, strings.ToUpper(r.Name), "  " + r.Name + " "} {
		m := a.ReviewRename(r.ID, same)
		if m.Why == "" && m.Refused == "" {
			t.Errorf("%q was offered as a rename of %q", same, r.Name)
		}
		if a.Routines()[0].Source != r.Source {
			t.Fatalf("%q changed the routine", same)
		}
	}
	// AND AN EMPTY BOX SAYS SO RATHER THAN DOING NOTHING QUIETLY.
	for _, nothing := range []string{"", "   ", "!!!"} {
		if m := a.ReviewRename(r.ID, nothing); m.Why == "" && m.Refused == "" {
			t.Errorf("%q was offered as a name", nothing)
		}
	}
}

// AND A ROUTINE THAT READS A CREDENTIAL UNDER ITS OWN NAME CANNOT BE RENAMED AT ALL.
//
// `codegen.typeSentence` writes `do OS's Secret with "<route>:password"` for a declared secret,
// where `<route>` is the routine's own name slugged — and its own comment says what disagreeing
// about that costs: looking up a secret nobody stored. A rename re-renders, so the qualifier
// would move and the routine would ask the credential store for something that was never saved
// there. The alternative is moving the credential, which this window will not do.
func TestARoutineThatReadsACredentialUnderItsOwnNameRefusesToBeRenamed(t *testing.T) {
	a, r := aRoutineThatReadsAPassword(t)
	if !strings.Contains(r.Source, `OS's Secret with "`) {
		t.Fatalf("the fixture reads no credential:\n%s", r.Source)
	}

	m := a.ReviewRename(r.ID, "sign in again")
	if m.Refused == "" && m.Why == "" {
		t.Fatal("a routine whose credential name is made from its own name was " +
			"offered a rename, so the rename would send it to the credential " +
			"store under a name nobody stored anything under")
	}
	said := m.Refused + m.Why
	// THE NAME IT READS NOW, what it would read instead, and that nothing was done.
	// This fixture's credential is written onto the SOURCE and is not in its steps,
	// so a re-render drops it entirely rather than renaming it — which the sentence
	// says, and which is still a routine that would sign in with nothing.
	for _, want := range []string{"type-my-address:password", "credential store",
		"no credential at all", "Nothing has been changed"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not say %q: %q", want, said)
		}
	}
	if m.Movable {
		t.Error("the refused rename is still movable")
	}
	if a.Routines()[0].Source != r.Source {
		t.Error("the refused rename changed the routine")
	}
}

// ── FIXTURES ──────────────────────────────────────────────────────────────────

// aRoutineThatAsksForAValue is a saved routine whose typed step takes its text from the person.
//
// Made the way somebody would make one — save the recording, then turn the typed step into a
// supplied one through the panel — rather than by writing the source here: a fixture that
// manufactured the state would not notice the day the two stopped composing.
func aRoutineThatAsksForAValue(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a := aStudioLibrary(t, "type my address with item", briskTypingIntoNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}
	e := a.StepEditing(rs[0].ID)
	if e.Why != "" {
		t.Fatalf("the fixture cannot be edited: %s", e.Why)
	}
	// AND IF THE TYPING CAME BACK AS HELD KEYS, IT IS DECLARED TYPED TEXT FIRST, which
	// is the state a value can be asked for in — see ADR-282. Only then: this
	// demonstration's presses do not overlap, so simplification folds them by itself and
	// a declaration over a run that is already one step is an edit with nothing in it.
	typed := false
	for _, row := range e.Rows {
		if row.Types {
			typed = true
		}
	}
	if !typed && len(e.TypedRuns) == 1 {
		run := e.TypedRuns[0]
		p := a.ReviewStepEdit(rs[0].ID, e.Basis, []StepChange{
			{Op: "typed", At: run.From, Upto: run.To}})
		if p.Why != "" {
			t.Fatalf("declaring the typed run refused: %s", p.Why)
		}
		if done := a.SaveStepEdit(p.Token); !done.Saved {
			t.Fatalf("saving the declaration refused: %+v", done)
		}
		e = a.StepEditing(a.Routines()[0].ID)
	}
	at := -1
	for _, row := range e.Rows {
		if row.Types {
			at = row.At
		}
	}
	if at < 0 {
		t.Fatalf("the fixture has no step that types text: %+v", e.Rows)
	}
	id := a.Routines()[0].ID
	v := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: at, Name: "item"}})
	if v.Why != "" || v.Refused != "" {
		t.Fatalf("asking for the value refused: %s %s", v.Why, v.Refused)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("saving the value refused: %+v", done)
	}
	return a, a.Routines()[0]
}

// aRoutineThatReadsAPassword is a saved routine whose source reads a credential qualified by the
// routine's own name — the shape `codegen` writes for a declared secret argument.
//
// WRITTEN ONTO THE SAVED FILE rather than demonstrated, because what is being tested is a
// property of the SOURCE: a rename re-renders, and the question is whether the credential name
// comes out the same. Nothing here touches a credential store.
func aRoutineThatReadsAPassword(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a, r := aMovableRoutineInStudio(t)
	reg := routes.Registry{Dir: a.routesDir}
	path := reg.Path(r.route())
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	// THE SECRET SENTENCE, QUALIFIED BY THE SLUG, exactly as the generator writes it.
	line := "  do OS's Secret with \"" + r.Slug + ":password\".\n"
	i := strings.LastIndex(src, "\n")
	if i < 0 {
		t.Fatalf("the fixture source has no lines:\n%s", src)
	}
	src = src[:i+1] + line
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = filepath.Dir(path)
	for _, x := range a.Routines() {
		if x.ID == r.ID {
			return a, x
		}
	}
	t.Fatal("the edited fixture is no longer in the library")
	return nil, Routine{}
}
