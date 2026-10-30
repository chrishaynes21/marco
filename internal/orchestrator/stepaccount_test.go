package orchestrator

// EVERY OPERATION THAT TOUCHES A ROUTINE, AND WHAT IT DOES TO THE ACCOUNT BESIDE IT.
//
// An account is the step list a routine's saved source is the render of. Once one exists, every
// other thing this package does to a routine has to decide about it — carry it, rewrite it,
// refuse, or sweep it — and a decision left for later is a conflict that ships: a move that
// regenerated from the demonstration would undo somebody's edit, through a surface whose whole
// job is to show them what will be written.
//
// One test per row of the lifecycle table, through the production path in each case.
//
// See [[Step-editing-proposal-2026-10-30]].

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// anEditableUnguardedRoutine is a routine nothing in which checks a window, so it can be saved
// under any scope — which is what makes it the fixture for the MOVE.
//
// The chained fixture cannot be moved at all: a guarded press needs its window brought forward,
// and `WhyNotThisScope` refuses the other two scopes. That is a rule of its own, and arranging a
// move of a routine it refuses would be testing the refusal.
func anEditableUnguardedRoutine(t *testing.T) (Deps, routes.Route) {
	t.Helper()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("press two", aPausedPlainDemonstration(), "notepad", "a test",
		Coordinates)
	rt, err := d.Save(dr, ScopeApp, "notepad")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	return d, rt
}

// aPausedPlainDemonstration presses two keys with a real pause between them, in an application,
// and nothing in it checks a window.
//
// The pause is what makes it a fixture for a WAIT edit: `aPlainDemonstration` has 10ms between
// its events, which `simplify` does not write down as waiting at all.
func aPausedPlainDemonstration() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(600), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: at(610), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "b", Down: true, T: at(2600), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "b", Down: false, T: at(2610), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(3200), App: "notepad"},
	}
}

// aWaitIn is the position of a wait whose duration the generator actually writes down.
//
// NOT SIMPLY THE FIRST ONE. A wait at the very start of a routine is not emitted — there is
// nothing to wait after — so an edit of it is a real edit that changes no bytes, and a fixture
// built on it would be testing `Savable` rather than the operation. So this asks the production
// path which waits make a difference, which is the same question the review answers.
func aWaitIn(t *testing.T, d Deps, rt routes.Route) int {
	t.Helper()
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatalf("reading %q's steps: %v", rt.Slug, err)
	}
	for i, s := range e.Steps() {
		if s.Kind != macroir.StepWait {
			continue
		}
		try, terr := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: i, To: 777}})
		if terr == nil && try.Changes() {
			return i
		}
	}
	t.Fatalf("%q has no wait a change would show: %+v", rt.Slug, e.Steps())
	return -1
}

// ── A MOVE ───────────────────────────────────────────────────────────────────

// MOVING AN EDITED ROUTINE CARRIES THE EDIT.
//
// The difference between the scopes is IN the generated source, so a move renders again — and
// rendering the DEMONSTRATION would hand somebody back a routine with their own change undone.
// The account travels, rebound to the source the move wrote; the same demonstration is written
// beside it, so its other digest does not change.
func TestAMoveOfAnEditedRoutineKeepsItsEdits(t *testing.T) {
	d, rt := anEditableUnguardedRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
	if !strings.Contains(sourceOf(t, d, rt), "Sleep with 777") {
		t.Fatal("the edit did not land")
	}

	m, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused: %s", m.Refused)
	}
	// THE VERSION ON THE SCREEN CARRIES THE EDIT, which is what makes approving it an
	// approval of something somebody recognises.
	if !strings.Contains(m.Now, "Sleep with 777") {
		t.Errorf("the move would write a version with the edit undone:\n%s", m.Now)
	}
	done, err := d.Rescope(m, m.ConsentToReplace())
	if err != nil {
		t.Fatalf("moving: %v", err)
	}
	if !done.Removed {
		t.Fatalf("the move did not complete: %s", done.Says)
	}
	// AND THE ROUTINE AT THE DESTINATION IS EDITABLE AGAIN, which is the whole claim:
	// the account arrived and still describes what is there.
	if !strings.Contains(sourceOf(t, d, done.To), "Sleep with 777") {
		t.Error("the moved routine lost the edit")
	}
	if _, st := d.Reg.Steps(done.To); st != routes.StepsIntact {
		t.Errorf("the account at the destination reads %q", st)
	}
	if _, err := d.PrepareEdit(done.To, nil); err != nil {
		t.Errorf("the moved routine cannot be edited: %v", err)
	}
	// AND NOTHING WAS LEFT UNDER THE OLD SLUG.
	if d.Reg.HasSteps(rt) {
		t.Error("an account was left beside the routine that was moved away")
	}
	if len(done.Residue) != 0 {
		t.Errorf("the move reported residue it should not have: %v", done.Residue)
	}
}

// AND THE ORIGINAL IS NOT REMOVED UNTIL THE ACCOUNT IS THERE AND DESCRIBES WHAT ARRIVED.
//
// Part of reading the destination back rather than a step afterwards: the original is about to be
// deleted and it is the only other copy of the step list. A write that returned no error is not a
// file that is there.
func TestAMoveVerifiesTheAccountAtTheDestinationBeforeRemovingTheOriginal(t *testing.T) {
	t.Run("the account could not be written", func(t *testing.T) {
		d, rt := anEditableUnguardedRoutine(t)
		editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
		m, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
		if err != nil {
			t.Fatalf("preparing: %v", err)
		}
		// A DIRECTORY IN THE DESTINATION ACCOUNT'S PLACE.
		if err := os.MkdirAll(filepath.Join(d.Reg.StepsPath(m.To), "in-the-way"),
			0o755); err != nil {
			t.Fatal(err)
		}
		done, err := d.Rescope(m, m.ConsentToReplace())
		if err != nil {
			t.Fatalf("moving: %v", err)
		}
		if done.Removed {
			t.Fatal("the original was removed although its steps did not " +
				"arrive with it")
		}
		if !strings.Contains(done.Says, "Copied") {
			t.Errorf("the outcome does not say it is a copy: %q", done.Says)
		}
		if !strings.Contains(done.Says, "steps could not be written") {
			t.Errorf("the outcome does not say what did not arrive: %q", done.Says)
		}
		// AND THE ORIGINAL IS WHOLE, edit and all.
		if !strings.Contains(sourceOf(t, d, rt), "Sleep with 777") {
			t.Error("the original lost its edit")
		}
		if _, st := d.Reg.Steps(rt); st != routes.StepsIntact {
			t.Errorf("the original's account reads %q", st)
		}
	})
	t.Run("the account arrived and is not about what arrived", func(t *testing.T) {
		d, rt := anEditableUnguardedRoutine(t)
		editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
		m, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
		if err != nil {
			t.Fatalf("preparing: %v", err)
		}
		// THE ONE INSTANT A MOVE CAN BE ASKED ABOUT: everything at the
		// destination has been written and nothing has been removed. Something
		// else writes over the account in that moment.
		d.afterDestination = func() {
			if err := os.WriteFile(d.Reg.StepsPath(m.To),
				[]byte(`{"version":1,"source":"deadbeef",`+
					`"demonstration":"deadbeef","steps":[{"kind":"wait"}]}`),
				0o644); err != nil {
				t.Fatal(err)
			}
		}
		done, err := d.Rescope(m, m.ConsentToReplace())
		if err != nil {
			t.Fatalf("moving: %v", err)
		}
		if done.Removed {
			t.Fatal("the original was removed although the steps beside the " +
				"destination are not an account of it")
		}
		if !strings.Contains(done.Says, "not an account of what this move wrote") {
			t.Errorf("the outcome does not say what it found: %q", done.Says)
		}
		if _, err := os.ReadFile(d.Reg.Path(rt)); err != nil {
			t.Errorf("the original is gone: %v", err)
		}
	})
}

// AND AN EDIT LANDING WHILE A MOVE IS BEING READ LEAVES BOTH ENDS ALONE.
func TestAMoveIsRefusedWhenTheRoutineWasEditedWhileItWasOpen(t *testing.T) {
	d, rt := anEditableUnguardedRoutine(t)
	m, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	// AN ACCOUNT THAT MOVED AND A SOURCE THAT DID NOT, which is the case only the third
	// digest can see: a whole edit landing would move both, and the source check would
	// answer first and say the wrong thing about it.
	rec, _ := d.Reg.LoadRecording(rt)
	if err := d.Reg.SaveSteps(rt, routes.StoredSteps{
		Source:        routes.DigestOf(sourceOf(t, d, rt)),
		Demonstration: routes.DigestOf(string(rec)),
		Steps:         []byte(`[{"kind":"wait","ms":10}]`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Rescope(m, m.ConsentToReplace()); err == nil {
		t.Fatal("the move went ahead over a routine whose stored steps had changed")
	} else if !strings.Contains(err.Error(), "step list stored beside") {
		t.Errorf("the refusal does not say what happened: %v", err)
	}
	if _, err := os.ReadFile(d.Reg.Path(rt)); err != nil {
		t.Errorf("the original is gone: %v", err)
	}
}

// AND A MOVE OF A HAND-EDITED ROUTINE STILL SHOWS THE LOSS.
//
// The account changed what a move renders for an edit made THROUGH the step editor, and it
// changed nothing for an edit made to the text. That routine has no account of itself that
// describes it, so a move renders its demonstration exactly as it always did, and approving the
// move is approving the loss of the edit — in front of them, which is what `Rescoping.Was` is
// for. Held so the new behaviour cannot be read as weakening the old warning.
func TestAMoveOfAHandEditedRoutineStillShowsTheLoss(t *testing.T) {
	d, rt := anEditableUnguardedRoutine(t)
	hand := sourceOf(t, d, rt) + "\n// a line somebody typed\n"
	if err := d.Reg.Save(rt, hand); err != nil {
		t.Fatal(err)
	}
	m, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused: %s", m.Refused)
	}
	if !strings.Contains(m.Was, "a line somebody typed") {
		t.Error("the left-hand side is not the file as it stands")
	}
	if strings.Contains(m.Now, "a line somebody typed") {
		t.Error("the move invented source nobody wrote")
	}
	if !m.Movable() {
		t.Error("a hand-edited routine cannot be moved at all, which is a bigger " +
			"change than this decision makes")
	}
	// AND NO ACCOUNT IS INVENTED FOR IT at the destination: there is nothing to carry,
	// and a routine whose source nothing here explains must not arrive looking explained.
	done, err := d.Rescope(m, m.ConsentToReplace())
	if err != nil {
		t.Fatalf("moving: %v", err)
	}
	if d.Reg.HasSteps(done.To) {
		t.Error("the move wrote an account of a routine nobody had one for")
	}
}

// ── A REPAIR ─────────────────────────────────────────────────────────────────

// A REPAIR OF AN EDITED ROUTINE KEEPS THE EDIT.
//
// `Resave` re-renders under today's rules and under the scope the presses need. Rendering the
// demonstration would undo the edit on the way past, which is the one thing the account exists to
// prevent.
func TestARepairOfAnEditedRoutineKeepsItsEdits(t *testing.T) {
	d, rt := savedUnrepaired(t, Coordinates)
	at := aWaitIn(t, d, rt)
	editing(t, d, rt, StepEdit{Op: EditWait, At: at, To: 777})

	repaired, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("repairing: %v", err)
	}
	src := sourceOf(t, d, repaired)
	if !strings.Contains(src, "Sleep with 777") {
		t.Errorf("the repair undid the edit:\n%s", src)
	}
	// AND THE REPAIRED ROUTINE IS EDITABLE, so the account was written back bound to
	// what the repair wrote.
	if _, st := d.Reg.Steps(repaired); st != routes.StepsIntact {
		t.Errorf("the repaired routine's account reads %q", st)
	}
	if _, err := d.PrepareEdit(repaired, nil); err != nil {
		t.Errorf("the repaired routine cannot be edited: %v", err)
	}
	// AND THE REPAIR DID WHAT IT IS FOR: the scope its presses need.
	if ScopeAlreadyOf(repaired) != ScopeAppAnywhere {
		t.Errorf("the repaired routine is under %q", ScopeAlreadyOf(repaired))
	}
}

// AND A REPAIR IS REFUSED RATHER THAN DISCARDING AN ACCOUNT IT CANNOT READ.
//
// This is the one writer here that consents to nothing: it decides on its own that the file in
// front of it refuses every press, and replaces it. A routine being unrunnable is not evidence
// that somebody's edits are disposable, so a step list it cannot make sense of is a refusal with
// the rebuild review named — not a repair that quietly rendered the demonstration.
func TestARepairIsRefusedWhenTheAccountCannotBeRead(t *testing.T) {
	for _, c := range []struct {
		what   string
		break_ func(t *testing.T, d Deps, rt routes.Route)
		says   string
	}{
		{"the source and the steps no longer match", func(t *testing.T, d Deps, rt routes.Route) {
			if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n"); err != nil {
				t.Fatal(err)
			}
		}, "no longer match"},
		{"the steps are about another demonstration", func(t *testing.T, d Deps, rt routes.Route) {
			raw, _ := d.Reg.LoadRecording(rt)
			if err := d.Reg.SaveRecording(rt, append(raw, ' ')); err != nil {
				t.Fatal(err)
			}
		}, "different demonstration"},
		{"the steps cannot be read at all", func(t *testing.T, d Deps, rt routes.Route) {
			if err := os.WriteFile(d.Reg.StepsPath(rt), []byte("{{{"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "cannot be read"},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := savedUnrepaired(t, Coordinates)
			u := theOneToRepair(t, d, rt)
			editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
			c.break_(t, d, rt)

			before := sourceOf(t, d, rt)
			_, err := d.Resave(u)
			if err == nil {
				t.Fatal("the repair went ahead over an account it could not read")
			}
			if !strings.Contains(err.Error(), "cannot be repaired automatically") {
				t.Errorf("the refusal does not say a repair is not the answer: %v",
					err)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say %q: %v", c.says, err)
			}
			if !strings.Contains(err.Error(), "review a rebuild") {
				t.Errorf("the refusal does not point anywhere: %v", err)
			}
			if sourceOf(t, d, rt) != before {
				t.Error("the routine was written anyway")
			}
		})
	}
	// AND A ROUTINE WITH NO ACCOUNT IS REPAIRED EXACTLY AS IT ALWAYS WAS, which is
	// every routine already on disk.
	t.Run("no account at all", func(t *testing.T) {
		d, rt := savedUnrepaired(t, Coordinates)
		if _, err := d.Resave(theOneToRepair(t, d, rt)); err != nil {
			t.Fatalf("an unedited routine could not be repaired: %v", err)
		}
	})
}

// ── A REBUILD, AND A RE-SIMPLIFICATION ───────────────────────────────────────

// A GENERATION REBUILD RENDERS THE ROUTINE'S OWN STEPS, SO THE EDIT SURVIVES IT.
//
// The generator's new rules reach an edited routine. Before there was anywhere to write the
// account, this rendered the demonstration — so what the panel offered was the person's own
// change undone, under a sentence about generations.
func TestARebuildOfAnEditedRoutineKeepsTheEdits(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})

	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the rebuild: %v", err)
	}
	if p.DiscardsEdits() || p.StepsWarning() != "" {
		t.Errorf("a rebuild that renders the stored steps warns about replacing "+
			"them: %q", p.StepsWarning())
	}
	if !strings.Contains(p.Now, "Sleep with 777") {
		t.Errorf("the rebuild would undo the edit:\n%s", p.Now)
	}
	// AND A ROUTINE ALREADY RENDERED BY TODAY'S RULES FROM ITS OWN STEPS REBUILDS TO
	// ITSELF, which is what makes "nothing would change" an honest thing to say.
	if p.Changes() {
		t.Errorf("rebuilding an edited routine changes it:\nwas\n%s\nnow\n%s",
			p.Was, p.Now)
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	if _, st := d.Reg.Steps(rt); st != routes.StepsIntact {
		t.Errorf("after the rebuild the account reads %q", st)
	}
	if !strings.Contains(sourceOf(t, d, rt), "Sleep with 777") {
		t.Error("the replaced routine lost the edit")
	}
}

// AND A RE-SIMPLIFICATION SAYS IT DISCARDS THEM, AND THEN DOES.
//
// The other choice, and the whole point of asking for it is to render the demonstration again.
// One line, and beside it the comparison the review already shows — which is better evidence of
// what would be lost than a count, and a count could not be had honestly anyway.
func TestAReSimplificationSaysItDiscardsTheStepEdits(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})

	p, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.DiscardsEdits() {
		t.Fatal("a re-simplification does not report that it would throw away the " +
			"step list stored beside the routine")
	}
	// THE WORDING IS TRUE WHETHER OR NOT ANYBODY EDITED ANYTHING, which is the whole of
	// why it is this sentence: a stored basis comes from a step edit AND from a
	// re-simplification, and nothing here can tell them apart.
	if p.StepsWarning() != "This rebuild replaces the stored steps with a fresh "+
		"simplification of the demonstration." {
		t.Errorf("the warning is %q", p.StepsWarning())
	}
	for _, mustNot := range []string{"your step edits", "you edited"} {
		if strings.Contains(p.StepsWarning(), mustNot) {
			t.Errorf("the warning claims %q, which a re-simplification of a "+
				"routine nobody edited would be claiming falsely: %q",
				mustNot, p.StepsWarning())
		}
	}
	// AND BOTH SOURCES ARE THERE IN FULL, which is where somebody sees what goes.
	if !strings.Contains(p.Was, "Sleep with 777") {
		t.Errorf("the left-hand side does not show the edit:\n%s", p.Was)
	}
	if strings.Contains(p.Now, "Sleep with 777") {
		t.Errorf("the right-hand side still carries the edit:\n%s", p.Now)
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	// AND THE EDIT IS GONE FROM IT, which is what was promised.
	stored, _ := d.Reg.Steps(rt)
	if strings.Contains(string(stored.Steps), "777") {
		t.Errorf("the edit survived in the stored steps: %s", stored.Steps)
	}
	// AND THE ROUTINE IS STILL EDITABLE, against an account of what the
	// re-simplification wrote.
	//
	// NOT AGAINST ITS DEMONSTRATION, and that is the point: this rendering was made
	// under rules that are not the default, so the demonstration does not produce it and
	// never will. Without an account of what was written, a routine somebody chose to
	// re-simplify would be permanently inexplicable — refused for a disagreement nobody
	// has. See Prepared.rendered.
	if _, st := d.Reg.Steps(rt); st != routes.StepsIntact {
		t.Fatalf("the account after a re-simplification reads %q", st)
	}
	if _, err := d.PrepareEdit(rt, nil); err != nil {
		t.Errorf("the re-simplified routine cannot be edited: %v", err)
	}
}

// AND A REBUILD OF A ROUTINE WHOSE ACCOUNT NO LONGER DESCRIBES IT SWEEPS IT TOO.
//
// That routine is exactly the one the rebuild review exists for. Its stored steps stop being an
// account of anything the moment the rebuild is written, so they go — and the review says so
// before anybody chooses it.
func TestARebuildOverAnAccountThatNoLongerDescribesTheRoutineSweepsIt(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
	if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n// from somewhere else\n"); err != nil {
		t.Fatal(err)
	}
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.DiscardsEdits() {
		t.Error("the review does not say the stored steps would go")
	}
	// AND THE OTHER SENTENCE, because this is the other situation: nothing replaces
	// them, so saying a fresh simplification takes their place would be wrong.
	if p.StepsWarning() != "This rebuild removes the stored steps, which are no longer "+
		"a description of this routine." {
		t.Errorf("the warning is %q", p.StepsWarning())
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	if d.Reg.HasSteps(rt) {
		t.Error("the account survived")
	}
}

// AND A REPLACEMENT IS REFUSED WHEN THE STORED STEPS MOVED WHILE THE REVIEW WAS OPEN.
//
// The third of the three files a rebuild is bound to. Built the only way it can be: the account's
// bytes moved and its meaning did not, so every other digest and both renderings are unchanged and
// this is the one thing being asked about.
func TestAReplacementIsRefusedWhenTheStoredStepsMovedWhileItWasOpen(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	raw, rerr := os.ReadFile(d.Reg.StepsPath(rt))
	if rerr != nil {
		t.Fatal(rerr)
	}
	var flat bytes.Buffer
	if err := json.Compact(&flat, raw); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.Reg.StepsPath(rt), flat.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	before := sourceOf(t, d, rt)
	rerr2 := d.Replace(p)
	if rerr2 == nil {
		t.Fatal("the replacement went ahead over a step list that had changed")
	}
	if !strings.Contains(rerr2.Error(), "step list stored beside") {
		t.Errorf("the refusal does not say what moved: %v", rerr2)
	}
	if sourceOf(t, d, rt) != before {
		t.Error("the routine was written anyway")
	}
}

// AND WHAT A REPLACEMENT WOULD DO TO THE STORED STEPS IS PART OF WHAT WAS APPROVED.
//
// `DiscardsEdits` is DERIVED and may therefore stay out of the token; what it is derived FROM may
// not, because `Replace` branches on exactly those two things to decide whether the stored step
// list is rewritten, replaced or removed. A `Prepared` whose basis was changed after it was sealed
// would do one of those three to a file the review never showed anybody.
//
// Reached from inside the package, which is where a value assembled from parts of two reviews, or
// damaged by a bug, comes from. There is no way to produce one through the public calls, which is
// the whole reason the token checks the struct against its own content.
func TestWhatAReplacementWouldDoToTheStoredStepsIsPartOfWhatWasApproved(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.fromAccount || len(p.accountToWrite) == 0 {
		t.Fatal("this fixture is not a rebuild from the stored steps, so it says " +
			"nothing about what decides one")
	}
	for _, c := range []struct {
		what   string
		damage func(Prepared) Prepared
	}{
		{"it would render from the demonstration instead", func(q Prepared) Prepared {
			q.fromAccount = false
			return q
		}},
		{"it would write a different step list", func(q Prepared) Prepared {
			q.accountToWrite = []byte(`[{"kind":"key","key":"f12"}]`)
			return q
		}},
		{"it would leave no step list at all", func(q Prepared) Prepared {
			q.accountToWrite = nil
			return q
		}},
	} {
		t.Run(c.what, func(t *testing.T) {
			before := sourceOf(t, d, rt)
			if err := d.Replace(c.damage(p)); err == nil {
				t.Fatal("it was written")
			} else if !strings.Contains(err.Error(), "not the one that was prepared") {
				t.Errorf("the refusal is not about the review's identity: %v", err)
			}
			if sourceOf(t, d, rt) != before {
				t.Error("the routine was written anyway")
			}
		})
	}
	// AND THE UNDAMAGED ONE IS WRITTEN, or every assertion above passes for a contract
	// that refuses everything.
	if err := d.Replace(p); err != nil {
		t.Fatalf("the review as prepared was refused: %v", err)
	}
}

// AND AN ACCOUNT THAT IS THERE AND CANNOT BE READ STOPS A MOVE BEING PREPARED.
//
// A move is bound to the same three files, and the same correction applies: a read failure is not
// absence, and a move may not be prepared against a file nothing has read.
func TestAnUnreadableAccountStopsAMoveBeingPrepared(t *testing.T) {
	d, rt := anEditableUnguardedRoutine(t)
	if err := os.MkdirAll(filepath.Join(d.Reg.StepsPath(rt), "in-the-way"),
		0o755); err != nil {
		t.Fatal(err)
	}
	_, err := d.PrepareRescope(rt, ScopeAppAnywhere, "notepad")
	if err == nil {
		t.Fatal("a move was prepared over an account nothing read")
	}
	if !strings.Contains(err.Error(), "could not be read") {
		t.Errorf("the refusal does not say so: %v", err)
	}
}

// ── RECORDING THE ROUTINE AGAIN ──────────────────────────────────────────────

// SAVING OVER A ROUTINE TAKES ITS STEP EDITS, AND THE QUESTION SAYS SO FIRST.
//
// The one lifecycle operation where losing them is the right answer: somebody has demonstrated
// the thing again, and the account describes a demonstration that is gone. Right, and still owed
// a sentence — the edit is in neither of the two files the question already names.
func TestRecordingARoutineAgainRemovesItsAccountAndSaysSo(t *testing.T) {
	d, rt := anEditableUnguardedRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: aWaitIn(t, d, rt), To: 777})

	// THE SAME NAME, DEMONSTRATED AGAIN.
	again := d.DraftOf("press two", aPlainDemonstration("b"), "notepad", "a test",
		Coordinates)
	at := d.Collision(again, ScopeApp, "notepad")
	if !at.Exists {
		t.Fatal("saving the same name again is not reported as a collision")
	}
	if !at.HasStoredSteps {
		t.Error("the question does not report that the routine there keeps a step " +
			"list of its own")
	}
	if !strings.Contains(at.Says, "keeps a step list of its own") {
		t.Errorf("the sentence does not say what else goes: %q", at.Says)
	}
	if _, err := d.Keep(again, ScopeApp, "notepad", Consent{ID: at.ID,
		Source: at.Source, Recording: at.Recording}); err != nil {
		t.Fatalf("keeping: %v", err)
	}
	// AND THE ACCOUNT IS GONE, because it is an account of a routine nobody has any more.
	if d.Reg.HasSteps(rt) {
		t.Error("the account survived the routine being demonstrated again")
	}
	if strings.Contains(sourceOf(t, d, rt), "Sleep with 777") {
		t.Error("the replaced routine still carries the old edit")
	}
	// AND THE NEW ROUTINE IS EDITABLE, against its own demonstration.
	if _, err := d.PrepareEdit(rt, nil); err != nil {
		t.Errorf("the re-recorded routine cannot be edited: %v", err)
	}
	// AND A ROUTINE WITH NO EDITS SAYS NOTHING ABOUT ANY.
	plain := d.Collision(again, ScopeApp, "notepad")
	if plain.HasStoredSteps || strings.Contains(plain.Says, "step list of its own") {
		t.Errorf("a routine with no stored step list is reported as keeping one: %q",
			plain.Says)
	}
}

// ── AND THE STEPS A REVIEW HOLDS ARE ITS OWN ─────────────────────────────────

// A CALLER CANNOT CHANGE WHAT A SEALED REVIEW SAYS IT IS.
func TestTheStepsAReviewHandsOutAreACopy(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	got := e.Steps()
	got[0] = macroir.Step{Kind: macroir.StepKey, Key: "f12"}
	if again := e.Steps(); again[0].Kind != macroir.StepActivate {
		t.Error("changing the slice a review handed out changed the review")
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving: %v", err)
	}
	stored, st := d.Reg.Steps(rt)
	if st != routes.StepsIntact {
		t.Fatalf("the account reads %q", st)
	}
	var steps []macroir.Step
	if err := json.Unmarshal(stored.Steps, &steps); err != nil {
		t.Fatal(err)
	}
	if steps[0].Kind != macroir.StepActivate {
		t.Errorf("the written account starts with a %s", steps[0].Kind)
	}
}
