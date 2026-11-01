package orchestrator

// RENAMING A ROUTINE, AND THE THING THE RENAME TESTS FOUND IN THE MOVE ENGINE.
//
// The move contract — both locks, both revalidations, the exclusive create, the destination read
// back, the removal that comes last — is `rescope_test.go`'s and is not restated here. What is
// asked here is what a RENAME adds, and one thing that was wrong for every move.
//
// See [[ADR-285-a-routine-can-be-called-something-else]].

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ── THE DEFECT THE RENAME TESTS FOUND, WHICH IS NOT ABOUT RENAMING ────────────

// A MOVE KEEPS THE VALUES THE ROUTINE ASKS FOR, AND DOES NOT READ THEM OUT OF THE CREDENTIAL STORE.
//
// # What was wrong
//
// A move RE-RENDERS, and `codegen` is told which argument names are declared. An UNDECLARED
// `{{name}}` is not a plain placeholder to it: `splitSecrets` turns it into
// `do OS's Secret with "name"`, which reads the person's credential store at run time.
//
// The declared set came from `routes.SplitArgs` of the routine's NAME, and a saved routine's
// header does not carry the `with …` clause — the declaration lives in the steps, as `{{item}}`
// in the text of a step that types. So every move of a routine that asked for a value produced a
// routine that asked the credential store for it instead, with the move review showing exactly
// that as the version it would write.
//
// `declaredFor` — `SplitArgs` plus `suppliedIn` — is how every other re-render in this package
// reads it. This test is over a RESCOPE, because that is the operation that has been shipping.
func TestAMoveKeepsTheValuesTheRoutineAsksFor(t *testing.T) {
	d, rt := aRoutineAskingForAValue(t)
	before, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "{{item}}") {
		t.Fatalf("the fixture does not ask for a value:\n%s", before)
	}

	m, err := d.PrepareRescope(rt, ScopeEverywhere, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused: %s", m.Refused)
	}
	// ── THE REVIEW ITSELF, which is where somebody would have had to notice ──
	if !strings.Contains(m.Now, "{{item}}") {
		t.Errorf("the version a move would write does not ask for the value:\n%s",
			m.Now)
	}
	if strings.Contains(m.Now, `OS's Secret with "item"`) {
		t.Errorf("a move turned the value the routine asks for into a read of the "+
			"credential store:\n%s", m.Now)
	}

	if _, err := d.Rescope(m, Consent{}); err != nil {
		t.Fatalf("moving: %v", err)
	}
	after, err := os.ReadFile(d.Reg.Path(m.To))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "{{item}}") {
		t.Fatalf("the moved routine no longer asks for the value:\n%s", after)
	}
	if strings.Contains(string(after), `OS's Secret`) {
		t.Errorf("the moved routine reads the credential store:\n%s", after)
	}
}

// AND A MOVE KEEPS THE CREDENTIAL IT WAS READING, UNDER THE NAME IT WAS READING IT UNDER.
//
// # This test recorded the defect and now holds the correction
//
// It was written as a pinned FINDING: a move re-rendered `sign-in:password` as the bare
// `password`, which is a different entry in somebody's credential store, and the half of this
// test that said so was written to fail loudly the day it was fixed. It did. See
// [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].
//
// THE ASSERTION IS THE IDENTIFIER AND NOT THE WORD `Secret`. A routine that still says `Secret`
// and says it about another name signs in with nothing, and every review of it looks right.
func TestAMoveKeepsTheCredentialItWasReading(t *testing.T) {
	d, rt := aRoutineAskingFor(t, "password")
	before, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	want := `OS's Secret with "type-my-address:password"`
	if !strings.Contains(string(before), want) {
		t.Fatalf("the fixture does not read a route-qualified credential:\n%s", before)
	}

	m, err := d.PrepareRescope(rt, ScopeEverywhere, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused: %s", m.Refused)
	}
	// ── THE REVIEW, which is where somebody would have had to notice ──
	if !strings.Contains(m.Now, want) {
		t.Fatalf("the version a move would write does not read the credential it "+
			"reads now (%s):\n%s", want, m.Now)
	}

	if _, err := d.Rescope(m, Consent{}); err != nil {
		t.Fatalf("moving: %v", err)
	}
	after, err := os.ReadFile(d.Reg.Path(m.To))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), want) {
		t.Fatalf("the moved routine reads a different credential:\n%s", after)
	}
	// AND IT IS STILL A CREDENTIAL, not a value somebody types into a box.
	if strings.Contains(string(after), "{{password}}") {
		t.Errorf("the moved routine carries the password as a placeholder:\n%s", after)
	}
}

// AND AN OPERATION THAT CANNOT KEEP THE CREDENTIAL REFUSES, RATHER THAN REDIRECTING IT.
//
// A RE-SIMPLIFICATION RENDERS THE DEMONSTRATION, and the demonstration never held the
// placeholder — a secret's value does not enter a recording, which is the whole of why
// `OS's Secret` exists. So there is nothing for preservation to reproduce the credential from,
// and the version this would write reads no credential at all.
//
// The routine would compile, run, and type nothing into the password field. Refused instead,
// with the name it reads now in the sentence.
func TestARebuildThatCannotKeepTheCredentialIsRefused(t *testing.T) {
	d, rt := aRoutineAskingFor(t, "password")
	before, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	want := `OS's Secret with "type-my-address:password"`
	if !strings.Contains(string(before), want) {
		t.Fatalf("the fixture does not read a route-qualified credential:\n%s", before)
	}

	p, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing the rebuild: %v", err)
	}
	if p.Refused == "" {
		t.Fatalf("a re-simplification dropped the credential the routine reads, and "+
			"would write:\n%s", p.Now)
	}
	for _, say := range []string{"type-my-address:password", "credential store",
		"Nothing has been changed"} {
		if !strings.Contains(p.Refused, say) {
			t.Errorf("the refusal does not say %q: %q", say, p.Refused)
		}
	}
	if p.Replaceable() {
		t.Error("the refused rebuild is still replaceable")
	}
	now, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil || string(now) != string(before) {
		t.Error("the refused rebuild changed the routine")
	}
}

// AND A REBUILD UNDER TODAY'S RULES KEEPS IT, because it renders the STORED STEPS, which do hold
// the placeholder.
//
// The two halves of the same question, and they have different answers: what a rebuild can
// preserve depends on what it is rendering FROM.
func TestARebuildUnderTodaysRulesKeepsTheCredential(t *testing.T) {
	d, rt := aRoutineAskingFor(t, "password")
	want := `OS's Secret with "type-my-address:password"`

	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the rebuild: %v", err)
	}
	if p.Refused != "" {
		t.Fatalf("the rebuild was refused: %s", p.Refused)
	}
	if !strings.Contains(p.Now, want) {
		t.Fatalf("the version a rebuild would write does not read the credential it "+
			"reads now (%s):\n%s", want, p.Now)
	}
}

// ── AND THE RENAME ITSELF ─────────────────────────────────────────────────────

// A RENAME IS A MOVE, AND THE ONE THING THAT CHANGES IS THE SLUG.
func TestARenameChangesTheSlugAndNothingElseAboutTheIdentity(t *testing.T) {
	d, rt := aMovableRoutine(t)
	m, err := d.PrepareRename(rt, "post my address")
	if err != nil {
		t.Fatalf("preparing the rename: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the rename was refused: %s", m.Refused)
	}
	if m.To.Slug != "post-my-address" {
		t.Errorf("it would be filed under %q", m.To.Slug)
	}
	if m.To.App != rt.App || m.To.Focus != rt.Focus {
		t.Errorf("a rename changed where it works: %+v became %+v", rt, m.To)
	}
	if m.Renaming != "post my address" {
		t.Errorf("the name it would take is %q", m.Renaming)
	}
	if _, err := d.Rescope(m, Consent{}); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	if _, err := os.Stat(d.Reg.Path(rt)); err == nil {
		t.Error("the routine is still at its old name as well")
	}
	raw, err := os.ReadFile(d.Reg.Path(m.To))
	if err != nil {
		t.Fatalf("the renamed routine is not there: %v", err)
	}
	if !strings.Contains(string(raw), "post my address") {
		t.Errorf("the renamed routine does not call itself by its new name:\n%s", raw)
	}
}

// AND A ROUTINE THAT READS A CREDENTIAL QUALIFIED BY ITS OWN NAME IS REFUSED.
//
// `codegen.typeSentence` writes `<route>:password` for a DECLARED secret, where `<route>` is the
// routine's own name slugged — and its own comment says what disagreeing about that costs:
// looking up a secret nobody stored. A rename re-renders, so the qualifier moves with the name,
// and the routine would ask the credential store for something nobody has saved.
//
// Refused, not migrated: moving the credential would be this code reaching into somebody's
// credential store, which it does not do.
func TestARenameThatWouldMoveACredentialNameIsRefused(t *testing.T) {
	d, rt := aRoutineAskingFor(t, "password")
	before, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), `:password"`) {
		t.Fatalf("the fixture's credential name is not qualified by the route, so "+
			"this cannot reach what it is about:\n%s", before)
	}

	m, err := d.PrepareRename(rt, "sign in again")
	if err != nil {
		t.Fatalf("preparing the rename: %v", err)
	}
	if m.Refused == "" {
		t.Fatalf("the rename was offered, and it would write:\n%s", m.Now)
	}
	if !strings.Contains(m.Refused, "credential store") {
		t.Errorf("the refusal does not say what is at stake: %q", m.Refused)
	}
	if m.Movable() {
		t.Error("the refused rename is still movable")
	}
	// AND PERFORMING IT ANYWAY IS REFUSED AT THE WRITE TOO.
	if _, err := d.Rescope(m, Consent{}); err == nil {
		t.Fatal("a refused rename was performed")
	}
	after, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil || string(after) != string(before) {
		t.Error("the refused rename changed the routine")
	}
}

// AND A NAME SOMETHING ELSE ALREADY HAS IS REFUSED, WITH NOTHING TO AGREE TO.
//
// The move engine can replace what is at a destination under a version-bound consent, and
// `ConsentToReplace` is only non-zero when `Taken` is set. A rename clears it, so there is no
// consent a caller could obtain for one and the create stays EXCLUSIVE whatever is passed — the
// surface that cannot offer the press and the engine that would refuse it anyway are two
// defences, and this is the one that decides.
func TestARenameOntoAnOccupiedNameIsRefusedWithNothingToAgreeTo(t *testing.T) {
	d, rt := aMovableRoutine(t)
	// A SECOND ROUTINE IN THE SAME PLACE, under the name the first one wants.
	other := d.DraftOf("post my address", aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	at, err := d.Save(other, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("saving the second fixture: %v", err)
	}
	before, err := os.ReadFile(d.Reg.Path(at))
	if err != nil {
		t.Fatal(err)
	}

	m, err := d.PrepareRename(rt, "post my address")
	if err != nil {
		t.Fatalf("preparing the rename: %v", err)
	}
	if m.Refused == "" {
		t.Fatalf("a rename onto an occupied name was offered, and it would write "+
			"over:\n%s", before)
	}
	if !strings.Contains(m.Refused, "already a routine") {
		t.Errorf("the refusal does not name what is in the way: %q", m.Refused)
	}
	// NOTHING TO AGREE TO. `Taken` is what makes a consent possible at all.
	if m.Taken || m.TakenSays != "" {
		t.Errorf("a rename offered a consent to replace what is there: %+v", m)
	}
	if c := m.ConsentToReplace(); c.ID != "" || c.Source != "" {
		t.Errorf("a consent could be obtained for a rename: %+v", c)
	}
	// AND PERFORMING IT ANYWAY, WITH A CONSENT A CALLER ASSEMBLED, IS STILL REFUSED.
	if _, err := d.Rescope(m, Consent{ID: RouteID(at)}); err == nil {
		t.Fatal("a refused rename was performed")
	}
	after, err := os.ReadFile(d.Reg.Path(at))
	if err != nil || string(after) != string(before) {
		t.Error("the routine that was in the way has been written over")
	}
	if _, err := os.Stat(d.Reg.Path(rt)); err != nil {
		t.Error("the routine being renamed is gone")
	}
}

// AND THE CLAUSE THAT DECLARES THE ARGUMENTS CANNOT BE TYPED INTO THE BOX.
func TestARenameRefusesAClauseAndRefusesANameWithNothingInIt(t *testing.T) {
	d, rt := aMovableRoutine(t)
	for _, bad := range []string{"post my address with item", "", "   ", "!!!"} {
		m, err := d.PrepareRename(rt, bad)
		if err == nil && m.Refused == "" {
			t.Errorf("%q was offered as a new name", bad)
		}
	}
	// AND THE NAME IT ALREADY HAS, by the slug: `Type My Address` is the same place.
	for _, same := range []string{"type my address", "Type My Address", "type-my-address"} {
		if _, err := d.PrepareRename(rt, same); err == nil {
			t.Errorf("%q was offered as a rename of the routine that has it", same)
		}
	}
}

// ── FIXTURES ──────────────────────────────────────────────────────────────────

// aRoutineAskingForAValue is a saved routine whose typed step takes its text from the person.
func aRoutineAskingForAValue(t *testing.T) (Deps, routes.Route) {
	t.Helper()
	return aRoutineAskingFor(t, "item")
}

// aRoutineAskingFor is a saved routine with one step that types `{{name}}`.
//
// Saved through the ordinary path and then written again with the placeholder in its stored
// steps, which is the state the step editor leaves a routine in — and the state a move re-renders
// from. See `Deps.Replace` and routes.StoredSteps.
func aRoutineAskingFor(t *testing.T, name string) (Deps, routes.Route) {
	t.Helper()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	steps := []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "{{" + name + "}}"},
		{Kind: macroir.StepWait, Ms: 250},
	}
	dr := &Draft{Name: "type my address", Args: []string{name},
		Steps: steps, App: "notepad", Mode: Unstated}
	rt, err := d.Save(dr, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("saving the fixture: %v", err)
	}
	// AND ITS DEMONSTRATION, which a move will not go without.
	//
	// A REAL ONE, because a re-simplification renders THIS rather than the stored steps
	// — and what it cannot render is the placeholder, which was never in a recording:
	// a secret's value does not enter one. That is the operation preservation cannot
	// help, and it has to be reachable.
	events, merr := json.Marshal(aPlainDemonstration("a"))
	if merr != nil {
		t.Fatal(merr)
	}
	rec := events
	if err := d.Reg.SaveRecording(rt, rec); err != nil {
		t.Fatalf("writing the fixture's demonstration: %v", err)
	}
	// AND THE STEPS IT STATES, because a move re-renders the ACCOUNT when there is one,
	// and this list is the only place the placeholder exists: the demonstration above
	// has nothing in it, which is also the state the real one is in for a routine whose
	// value was declared after it was recorded.
	//
	// The two digests are what make the account BELIEVED — see routes.StoredSteps — so
	// they are read off the files just written rather than left empty, which would be an
	// account of some other routine and would send the move back to the demonstration.
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.SaveSteps(rt, routes.StoredSteps{
		Source:        routes.DigestOf(string(src)),
		Demonstration: routes.DigestOf(string(rec)),
		Steps:         raw,
	}); err != nil {
		t.Fatalf("writing the fixture's step account: %v", err)
	}
	return d, rt
}
