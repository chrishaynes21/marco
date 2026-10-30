package orchestrator

// EDITING A STEP, HELD THROUGH THE PATHS THAT SHIP.
//
// Every routine here is saved by the production save path, rendered by the production generator
// and — where what runs is the question — RUN by the production driver over a host that records
// what it was asked to do. Nothing is hand-written Marco, because the thing being established is
// that an edit of a real routine produces a real routine.
//
// What this file does NOT establish is that an edited routine presses anything on a desktop. The
// host below answers in memory; a real press at a real coordinate in a real window is a live
// trial and is held separately.
//
// See [[Step-editing-proposal-2026-10-30]].

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// ── THE WORLD ────────────────────────────────────────────────────────────────

// aGuardedClickAt is one press that checks the window it was made in, which is what makes the
// generated routine a CHAIN: every press its own capability, calling the next only when its own
// went through.
func aGuardedClickAt(ms, x, y int) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
			App: "arkascended", PID: 4242, T: at(ms), WinRel: true,
			RelX: x - 120, RelY: y - 64, RelTo: "window", WinW: 1920,
			WinH: 1080, Dpi: 144, Process: "arkascended.exe"},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
			App: "arkascended", PID: 4242, T: at(ms + 5)},
	}
}

// aThreePressDemonstration is three guarded clicks with real pauses between them, ended with the
// stop gesture.
//
// Its step list, which every assertion below counts on:
//
//	0 activate   1 wait 100   2 click 842,274   3 wait 2500   4 click 900,300
//	5 wait 1500  6 click 950,360   7 wait 200   8 key esc
func aThreePressDemonstration() []recorder.RecordedEvent {
	ev := []recorder.RecordedEvent{{Kind: recorder.EvAppSwitch,
		KeyName: "arkascended", T: at(0)}}
	ev = append(ev, aGuardedClickAt(100, 842, 274)...)
	ev = append(ev, aGuardedClickAt(2600, 900, 300)...)
	ev = append(ev, aGuardedClickAt(4100, 950, 360)...)
	return append(ev, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "esc",
		Down: true, T: at(4300), App: "arkascended"})
}

// aHeldKeyDemonstration holds shift across a press and lets it go, so the step list carries the
// one gesture `macroir` does not make a single step.
//
//	0 activate  1 wait 100  2 keydown shift  3 wait 200  4 click  5 wait 600
//	6 keyup shift  7 wait 600  8 click  9 wait 300  10 key esc
func aHeldKeyDemonstration() []recorder.RecordedEvent {
	ev := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "shift", Down: true, T: at(100),
			App: "arkascended"},
	}
	ev = append(ev, aGuardedClickAt(300, 500, 500)...)
	ev = append(ev, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "shift",
		Down: false, T: at(900), App: "arkascended"})
	ev = append(ev, aGuardedClickAt(1500, 600, 600)...)
	return append(ev, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "esc",
		Down: true, T: at(1800), App: "arkascended"})
}

// anEditableRoutine saves a demonstration through the production save path and hands back the
// orchestrator over it.
//
// `ScopeAppAnywhere` because a guarded press needs the window brought forward; `WhyNotThisScope`
// refuses the other two, which is a rule of its own and not this file's subject.
func anEditableRoutine(t *testing.T, name string,
	events []recorder.RecordedEvent) (Deps, routes.Route) {

	t.Helper()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf(name, events, "arkascended", "a test", Coordinates)
	if dr == nil || len(dr.Steps) == 0 {
		t.Fatal("the demonstration produced no draft")
	}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	return d, rt
}

// aThreePressRoutine is the chained fixture, saved.
func aThreePressRoutine(t *testing.T) (Deps, routes.Route) {
	t.Helper()
	return anEditableRoutine(t, "open graphics", aThreePressDemonstration())
}

// editing applies the operations through PREPARE then SAVE, which is the production path, and
// fails the test if either refuses.
func editing(t *testing.T, d Deps, rt routes.Route, edits ...StepEdit) EditReview {
	t.Helper()
	e, err := d.PrepareEdit(rt, edits)
	if err != nil {
		t.Fatalf("preparing the edit: %v", err)
	}
	if e.Refused != "" {
		t.Fatalf("the edit was refused: %s", e.Refused)
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving the edit: %v", err)
	}
	return e
}

// askedHost records what a routine asked the OS to do, and can answer not-ok at one of them.
//
// A STAND-IN FOR THE DESKTOP AND NOTHING MORE. It presses nothing; what it establishes is the
// ORDER and the ARGUMENTS of what the edited routine asks for, and that a refusal stops what
// follows — none of which a reading of the source can settle, because the source is what was
// being reviewed.
type askedHost struct {
	calls []string
	// failAt is the one-based call number that answers something other than ok, or 0 for
	// a host that agrees to everything.
	failAt int
}

func (h *askedHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	line := c.Act + "'s " + c.Action
	if !c.Input.IsAbsent() {
		line += " with " + c.Input.String()
	}
	h.calls = append(h.calls, line)
	if h.failAt > 0 && len(h.calls) == h.failAt {
		return "failed", runtime.ErrVal(&runtime.Err{
			Message: "the window it was recorded in is not there"}), nil
	}
	return "ok", runtime.Absent(), nil
}

// whatItAsked runs a saved routine through the production driver over that host.
func whatItAsked(t *testing.T, d Deps, rt routes.Route, failAt int) []string {
	t.Helper()
	h := &askedHost{failAt: failAt}
	out := &strings.Builder{}
	if err := driver.RunFileWithHosts(d.Reg.Path(rt), out,
		map[string]runtime.Host{"OS": h}); err != nil {
		t.Fatalf("running %s: %v\n%s", rt.Slug, err, out.String())
	}
	return h.calls
}

// ── THE DEMONSTRATION ────────────────────────────────────────────────────────

// AN EDIT NEVER TOUCHES THE DEMONSTRATION.
//
// `.rec.json` is the only durable account of what the person did. An edit is a decision about
// what Marco will do, and the two are not the same thing — a surface that rewrote the second to
// express the first would be editing somebody's afternoon. `SaveRecording` is one call away from
// any writer in this package, so this is a rule with a test rather than a property of today's
// code path.
func TestAnEditedRoutineKeepsItsDemonstrationByteForByte(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	before, ok := d.Reg.LoadRecording(rt)
	if !ok {
		t.Fatal("the fixture has no demonstration beside it")
	}
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800},
		StepEdit{Op: EditRemove, At: 4})

	after, ok := d.Reg.LoadRecording(rt)
	if !ok {
		t.Fatal("the demonstration is gone after an edit")
	}
	if string(before) != string(after) {
		t.Errorf("the demonstration changed: %d bytes became %d", len(before),
			len(after))
	}
}

// ── WHAT MAY BE EDITED ───────────────────────────────────────────────────────

// THE FIRST EDIT IS AGAINST THE DEMONSTRATION, BECAUSE THERE IS NOTHING ELSE.
//
// Every routine already on disk has no account, so absence has to mean "no edit; the
// demonstration is the whole story" — a one-way inference, exactly as `taughtIfRecorded` reads
// the presence of a recording.
func TestAnEditIsOfferedAgainstTheDemonstrationWhenThereIsNoAccount(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	if d.Reg.HasSteps(rt) {
		t.Fatal("a freshly saved routine already has an account beside it")
	}
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatalf("a routine saved by the production path cannot be edited: %v", err)
	}
	// AND ASKING WITH NO OPERATIONS IS THE QUESTION "may this be edited", answered
	// through the path that would perform one.
	if e.Refused != "" {
		t.Errorf("it was refused: %s", e.Refused)
	}
	if len(e.Steps()) != 9 {
		t.Errorf("the step list has %d steps, so it is not the demonstration's",
			len(e.Steps()))
	}
	if e.Savable() {
		t.Error("a review with no operations in it is savable, so a surface asking " +
			"whether a routine can be edited would write a file")
	}
	if d.Reg.HasSteps(rt) {
		t.Error("asking the question wrote an account")
	}
}

// AND THE SECOND EDIT IS AGAINST THE FIRST, OR THIS IS A ONE-USE EDITOR.
//
// After a save the source differs from the demonstration's rendering ON PURPOSE, and that
// difference IS the edit. A precondition that compared against the demonstration forever would
// refuse every routine the moment it had been edited once.
func TestAnAlreadyEditedRoutineCanBeEditedAgain(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
	if _, st := d.Reg.Steps(rt); st != routes.StepsIntact {
		t.Fatalf("after one edit the account reads %q", st)
	}
	if !strings.Contains(sourceOf(t, d, rt), "Sleep with 800") {
		t.Fatal("the first edit did not land")
	}

	// THE SECOND EDIT, which is the whole point of this test.
	editing(t, d, rt, StepEdit{Op: EditWait, At: 5, To: 900})
	src := sourceOf(t, d, rt)
	for _, want := range []string{"Sleep with 800", "Sleep with 900"} {
		if !strings.Contains(src, want) {
			t.Errorf("the routine does not say %q, so one of the two edits was "+
				"lost:\n%s", want, src)
		}
	}
	// AND A THIRD, over the same wait as the first, to establish that an account is a
	// basis and not a one-off record.
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 1200})
	src = sourceOf(t, d, rt)
	if strings.Contains(src, "Sleep with 800") || !strings.Contains(src, "Sleep with 1200") {
		t.Errorf("the third edit did not replace the first:\n%s", src)
	}
}

// AN EDIT IS REFUSED WHEN THE SOURCE IS NOT WHAT ITS ACCOUNT RENDERS TO.
//
// Two ways that happens, and they are different situations: the file and the stored steps no
// longer agree, or the stored steps are about another demonstration. Both refuse, both name which,
// and neither guesses.
func TestAnEditIsRefusedWhenTheSourceIsNotTheRenderOfItsAccount(t *testing.T) {
	t.Run("the source moved", func(t *testing.T) {
		d, rt := aThreePressRoutine(t)
		editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
		// SOMETHING WROTE THE SOURCE WITHOUT WRITING THE ACCOUNT. `marco edit`
		// does exactly this, and so does a failed account write.
		if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n// a line from somewhere\n"); err != nil {
			t.Fatal(err)
		}
		if _, st := d.Reg.Steps(rt); st != routes.StepsMismatched {
			t.Fatalf("the account reads %q", st)
		}
		_, err := d.PrepareEdit(rt, nil)
		if err == nil {
			t.Fatal("an edit was offered against a source nothing explains")
		}
		if !strings.Contains(err.Error(), "no longer match") {
			t.Errorf("the refusal does not say the two no longer match: %v", err)
		}
	})
	t.Run("the demonstration moved", func(t *testing.T) {
		d, rt := aThreePressRoutine(t)
		editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
		raw, _ := d.Reg.LoadRecording(rt)
		if err := d.Reg.SaveRecording(rt, append(raw, ' ')); err != nil {
			t.Fatal(err)
		}
		if _, st := d.Reg.Steps(rt); st != routes.StepsAdrift {
			t.Fatalf("the account reads %q", st)
		}
		_, err := d.PrepareEdit(rt, nil)
		if err == nil {
			t.Fatal("an edit was offered against steps bound to another " +
				"demonstration")
		}
		if !strings.Contains(err.Error(), "different demonstration") {
			t.Errorf("the refusal does not say which: %v", err)
		}
	})
}

// AND A MISMATCH IS DESCRIBED AS WHAT IT IS, NOT AS WHAT IT SUGGESTS.
//
// A source digest that moved is equally consistent with somebody editing the file by hand and
// with an account write that failed after the source landed (see the write order). Nothing on
// disk distinguishes them, so a sentence naming either sends whoever reads it to the wrong place.
func TestAMismatchIsNotReportedAsAHandEdit(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
	if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n"); err != nil {
		t.Fatal(err)
	}
	_, err := d.PrepareEdit(rt, nil)
	if err == nil {
		t.Fatal("the edit was not refused")
	}
	for _, mustNot := range []string{"hand", "by hand", "you edited", "edited it"} {
		if strings.Contains(strings.ToLower(err.Error()), mustNot) {
			t.Errorf("the refusal claims %q, which the state on disk cannot "+
				"establish: %v", mustNot, err)
		}
	}
	if !strings.Contains(err.Error(), "rebuild") {
		t.Errorf("the refusal does not point anywhere: %v", err)
	}
}

// AND A FILE THAT CANNOT ANSWER IS NOT AN ABSENT FILE.
//
// Treating an unreadable account as "no account" would render the demonstration and discard it
// silently — a routine written by a build that knew something this one does not, quietly reverted.
func TestAnUnreadableAccountIsSaidAndNotTreatedAsAbsent(t *testing.T) {
	for _, c := range []struct{ what, holds string }{
		{"not JSON at all", "{{{"},
		{"a version from the future", `{"version":99,"source":"a","demonstration":"b","steps":[]}`},
		{"no digests", `{"version":1,"steps":[{"kind":"wait","ms":10}]}`},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := aThreePressRoutine(t)
			if err := os.WriteFile(d.Reg.StepsPath(rt), []byte(c.holds), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, st := d.Reg.Steps(rt); st != routes.StepsUnreadable {
				t.Fatalf("the account reads %q", st)
			}
			_, err := d.PrepareEdit(rt, nil)
			if err == nil {
				t.Fatal("an edit was offered over an account nothing could read")
			}
			if !strings.Contains(err.Error(), "cannot be read") {
				t.Errorf("the refusal does not say so: %v", err)
			}
		})
	}
}

// ── WHAT A REMOVAL MEANS ─────────────────────────────────────────────────────

// A GESTURE GOES WHOLE OR NOT AT ALL, AND A HELD KEY IS THE ONE WITH TWO HALVES.
//
// `macroir` already makes a drag, a typed run, an anchored click and a folded repetition one step
// each. A hold is a keydown and a later keyup, and removing either alone leaves a routine that
// holds a key it never releases — which changes what every press after it means.
func TestARemovedHoldTakesBothHalvesOrNothing(t *testing.T) {
	for _, c := range []struct {
		what string
		at   int
	}{
		{"removing the hold takes the release", 2},
		{"removing the release takes the hold", 6},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := anEditableRoutine(t, "hold thing", aHeldKeyDemonstration())
			e := editing(t, d, rt, StepEdit{Op: EditRemove, At: c.at})
			if n := len(e.Steps()); n != 9 {
				t.Errorf("%d steps are left, so one half of the hold was "+
					"removed on its own", n)
			}
			for _, s := range e.Steps() {
				if s.Kind == macroir.StepKeyDown || s.Kind == macroir.StepKeyUp {
					t.Errorf("a %s of %q survived", s.Kind, s.Key)
				}
			}
			// AND IT SAYS SO, because somebody asked for one step and two went.
			said := strings.Join(e.Says, " ")
			if !strings.Contains(said, "two halves") || !strings.Contains(said, "shift") {
				t.Errorf("the review does not say the partner went too: %q",
					said)
			}
			src := sourceOf(t, d, rt)
			if strings.Contains(src, `"shift"`) {
				t.Errorf("the saved routine still mentions shift:\n%s", src)
			}
		})
	}
	// AND A HALF WITH NO PARTNER IS REFUSED, NEVER GUESSED.
	t.Run("an unresolvable half is refused", func(t *testing.T) {
		d, rt := anEditableRoutine(t, "hold thing", aHeldKeyDemonstration())
		// The release is taken out of the ACCOUNT first, leaving a hold with
		// nothing to pair with. Written through the production save so the
		// routine on disk is the render of it.
		editing(t, d, rt, StepEdit{Op: EditRemove, At: 2})
		// With both halves gone there is no hold left to ask about, so the
		// unresolvable case is built directly: an account whose steps hold a key
		// nothing releases.
		stored, _ := d.Reg.Steps(rt)
		var steps []macroir.Step
		if err := json.Unmarshal(stored.Steps, &steps); err != nil {
			t.Fatal(err)
		}
		lone := append([]macroir.Step{}, steps...)
		lone = append(lone[:2], append([]macroir.Step{{Kind: macroir.StepKeyDown,
			Key: "shift"}}, lone[2:]...)...)
		raw, err := json.Marshal(lone)
		if err != nil {
			t.Fatal(err)
		}
		// AND THE ACCOUNT IS REBOUND TO THE SOURCE THAT RENDERS FROM IT, so the
		// refusal under test is the hold's and not the precondition's.
		base, args := routes.SplitArgs(NameIn(sourceOf(t, d, rt), rt.Slug))
		src, _, rerr := d.render(&Draft{Name: base, Args: args, Steps: lone,
			Mode: Coordinates}, ScopeAlreadyOf(rt).mode(), rt.App, rt)
		if rerr != nil {
			t.Fatalf("rendering the lone hold: %v", rerr)
		}
		if err := d.Reg.Replace(rt, src); err != nil {
			t.Fatal(err)
		}
		rec, _ := d.Reg.LoadRecording(rt)
		if err := d.Reg.SaveSteps(rt, routes.StoredSteps{
			Source: routes.DigestOf(src), Demonstration: routes.DigestOf(string(rec)),
			Steps: raw}); err != nil {
			t.Fatal(err)
		}
		e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditRemove, At: 2}})
		if err != nil {
			t.Fatalf("preparing: %v", err)
		}
		if e.Refused == "" {
			t.Fatal("removing a hold nothing releases was allowed")
		}
		if !strings.Contains(e.Refused, "releases it") {
			t.Errorf("the refusal does not say what it could not work out: %q",
				e.Refused)
		}
		if e.Savable() {
			t.Error("a refused review is savable")
		}
	})
}

// A REMOVED STEP IS REMOVED WHOLE, IN BOTH OF THE SHAPES CODEGEN WRITES.
//
// A chained routine makes every press its own capability; an unguarded one emits the steps flat.
// Removing a press is a re-render in both, and line surgery in neither.
func TestARemovedStepIsRemovedWholeInBothShapes(t *testing.T) {
	t.Run("chained", func(t *testing.T) {
		d, rt := aThreePressRoutine(t)
		if !strings.Contains(sourceOf(t, d, rt), "this can Step2.") {
			t.Fatal("the fixture is not the chained shape")
		}
		editing(t, d, rt, StepEdit{Op: EditRemove, At: 4})
		src := sourceOf(t, d, rt)
		if strings.Contains(src, "X 900, Y 300") {
			t.Errorf("the removed press is still there:\n%s", src)
		}
		// AND THE CHAIN IS STILL A CHAIN, with no link calling one that is gone.
		for _, must := range []string{"this can Step2.", "this can Step3.",
			"this can Step4.", "this is ok!"} {
			if !strings.Contains(src, must) {
				t.Errorf("the re-rendered chain has no %q:\n%s", must, src)
			}
		}
		if strings.Contains(src, "this can Step5.") {
			t.Errorf("the chain still has five links after a press went:\n%s", src)
		}
		if err := driver.CheckSource(src); err != nil {
			t.Errorf("the re-rendered routine does not compile: %v\n%s", err, src)
		}
	})
	t.Run("unguarded", func(t *testing.T) {
		// NOTHING IN THIS DEMONSTRATION CHECKS A WINDOW, so codegen writes one
		// flat body — the shape every legacy routine has.
		d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
		dr := d.DraftOf("press a", aPlainDemonstration("a"), "notepad", "a test",
			Coordinates)
		rt, err := d.Save(dr, ScopeApp, "notepad")
		if err != nil {
			t.Fatalf("saving: %v", err)
		}
		if strings.Contains(sourceOf(t, d, rt), "this can Step2.") {
			t.Fatal("the fixture is not the unguarded shape")
		}
		e, perr := d.PrepareEdit(rt, nil)
		if perr != nil {
			t.Fatalf("preparing: %v", perr)
		}
		// The last step is the stop gesture; the one before it is the press.
		pressed := -1
		for i, s := range e.Steps() {
			if s.Kind == macroir.StepKey && s.Key == "a" {
				pressed = i
			}
		}
		if pressed < 0 {
			t.Fatalf("the fixture has no press to remove: %+v", e.Steps())
		}
		editing(t, d, rt, StepEdit{Op: EditRemove, At: pressed})
		src := sourceOf(t, d, rt)
		if strings.Contains(src, `do OS's Key with "a"`) {
			t.Errorf("the removed press is still there:\n%s", src)
		}
		if err := driver.CheckSource(src); err != nil {
			t.Errorf("the re-rendered routine does not compile: %v\n%s", err, src)
		}
	})
}

// REMOVING THE LAST ACTION OF A CHAIN RENDERS A SHORTER CHAIN, AND DOES NOT REFUSE.
//
// The single `this is ok!` and the trailing settle wait live inside the success of the LAST link.
// Removing that link is therefore a re-render that hands both to the link which is now last —
// which is the ordinary case, not an edge the gate should be catching.
func TestRemovingTheLastLinkOfAChainRendersAShorterChain(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	before := sourceOf(t, d, rt)
	if !strings.Contains(before, "this can Step5.") {
		t.Fatalf("the fixture is not five links long:\n%s", before)
	}
	// Step 8 is the stop gesture, which is the last link; step 7 is its wait.
	editing(t, d, rt, StepEdit{Op: EditRemove, At: 8})
	src := sourceOf(t, d, rt)
	if strings.Contains(src, `do OS's Key with "esc"`) {
		t.Errorf("the last press is still there:\n%s", src)
	}
	if strings.Contains(src, "this can Step5.") {
		t.Errorf("the chain still has its fifth link:\n%s", src)
	}
	// AND THE ANSWER THE CHAIN REPORTS MOVED WITH IT. Exactly one `this is ok!`, and it
	// is inside the link that is now last — without that the routine can never succeed.
	if n := strings.Count(src, "this is ok!"); n != 1 {
		t.Errorf("the re-rendered chain has %d `this is ok!` in it:\n%s", n, src)
	}
	tail := src[strings.Index(src, "this can Step4."):]
	if !strings.Contains(tail, "this is ok!") {
		t.Errorf("the answer did not move into the link that is now last:\n%s", src)
	}
	if err := driver.CheckSource(src); err != nil {
		t.Fatalf("the shorter chain does not compile: %v\n%s", err, src)
	}
	// AND IT STILL RUNS, which is the claim a reading of the source cannot make.
	calls := whatItAsked(t, d, rt, 0)
	for _, c := range calls {
		if strings.Contains(c, "Key with esc") {
			t.Errorf("the removed press was still asked for: %v", calls)
		}
	}
}

// AN EDIT THAT WOULD LEAVE NOTHING TO DO IS REFUSED, AND SAYS WHAT TO DO INSTEAD.
//
// A routine that does nothing is not a smaller routine. Deleting one is an operation that already
// exists and already sweeps what sits beside it; an edit that quietly amounted to a delete would
// be offering that without any of it.
func TestAnEditThatWouldEmptyARoutineIsRefusedAndSaysToDeleteItInstead(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, perr := d.PrepareEdit(rt, nil)
	if perr != nil {
		t.Fatalf("preparing: %v", perr)
	}
	all := make([]StepEdit, 0, len(e.Steps()))
	for i := range e.Steps() {
		all = append(all, StepEdit{Op: EditRemove, At: i})
	}
	empty, err := d.PrepareEdit(rt, all)
	if err != nil {
		t.Fatalf("preparing the emptying edit: %v", err)
	}
	if empty.Refused == "" {
		t.Fatal("an edit that removes every step was not refused")
	}
	if !strings.Contains(empty.Refused, "Deleting the routine") {
		t.Errorf("the refusal does not say what to do instead: %q", empty.Refused)
	}
	if err := d.SaveEdit(empty); err == nil {
		t.Fatal("a refused review was saved")
	}
	// AND NOTHING WAS WRITTEN.
	if d.Reg.HasSteps(rt) {
		t.Error("the refusal wrote an account")
	}
}

// AND AN EDITED RENDER IS LEGAL MARCO BEFORE IT CAN BE OFFERED.
//
// `codegen` is trusted everywhere else here because its input came from a recorder. After an edit
// it did not, so the one compile gate is asked — and the refusal is a review somebody reads
// rather than a failure at the moment of writing.
//
// Staged through the seam rather than by inventing a step list that happens to produce illegal
// Marco: that would be a test about `codegen`, and this is a test that the gate is asked at all.
func TestAnEditedRenderThatIsNotLegalMarcoIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	asked := 0
	d.CheckSource = func(string) error {
		asked++
		return fmt.Errorf("line 7: nothing answers to `Step9`")
	}
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditRemove, At: 4}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if asked == 0 {
		t.Fatal("the compile gate was never asked, so an edited routine could be " +
			"offered without being compiled")
	}
	if e.Refused == "" {
		t.Fatal("a render the gate refused was offered anyway")
	}
	if !strings.Contains(e.Refused, "nothing answers to `Step9`") {
		t.Errorf("the refusal does not carry the compiler's own words: %q", e.Refused)
	}
	if e.Savable() {
		t.Error("a review the gate refused is savable")
	}
	if err := d.SaveEdit(e); err == nil {
		t.Fatal("it was saved")
	}
	if d.Reg.HasSteps(rt) {
		t.Error("an account was written for a render that does not compile")
	}
}

// ── WHAT RUNS ────────────────────────────────────────────────────────────────

// A CHANGED WAIT IS THE WAIT THAT RUNS.
//
// Not "appears in the source" — asked for, by the engine, with that number in it.
func TestAChangedWaitIsTheWaitThatRuns(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	if !hasCall(whatItAsked(t, d, rt, 0), "OS's Sleep with 2500") {
		t.Fatal("the fixture never asked for the wait being changed")
	}
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
	calls := whatItAsked(t, d, rt, 0)
	if !hasCall(calls, "OS's Sleep with 800") {
		t.Errorf("the new duration was never asked for: %v", calls)
	}
	if hasCall(calls, "OS's Sleep with 2500") {
		t.Errorf("the old duration was still asked for: %v", calls)
	}
}

// AND WHAT IS LEFT IS PRESSED, IN ORDER.
//
// The order is the part a reading of the source cannot settle for a chained routine: each link
// calls the next from inside its own success, so "the steps are in the file" and "the presses
// happen in that order" are two claims.
func TestAnEditedRoutinePressesWhatIsLeftInOrder(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditRemove, At: 4})
	calls := whatItAsked(t, d, rt, 0)
	// AND THE WAITS AROUND THE REMOVED PRESS BOTH STAY, which is what "remove one step,
	// whole" means: a wait is its own step, and deciding how long to wait once a press
	// has gone is a second operation somebody makes on purpose. An editor that quietly
	// collapsed them would be choosing a duration nobody asked for.
	want := []string{
		"OS's Activate with arkascended",
		"OS's Sleep with 100",
		"OS's Click with",
		"OS's Sleep with 2500",
		"OS's Sleep with 1500",
		"OS's Click with",
		"OS's Sleep with 200",
		"OS's Key with esc",
	}
	if len(calls) != len(want)+1 { // +1 for the trailing settle wait
		t.Errorf("the edited routine made %d calls, not the %d expected: %v",
			len(calls), len(want)+1, calls)
	}
	for i, w := range want {
		if i >= len(calls) || !strings.HasPrefix(calls[i], w) {
			t.Fatalf("call %d is %q, expected %q\nall: %v", i+1,
				callAt(calls, i), w, calls)
		}
	}
	// AND THE PRESS THAT WENT IS NOWHERE IN IT.
	for _, c := range calls {
		if strings.Contains(c, "X 900") {
			t.Errorf("the removed press was asked for: %v", calls)
		}
	}
}

// AND A FAILED STEP STOPS WHAT FOLLOWS, WHICH THE RE-RENDER MUST NOT HAVE FLATTENED.
//
// The chained shape exists so that a press which cannot be checked against its window stops the
// routine. An edit re-renders that shape, and a re-render that emitted the steps flat would
// produce a routine which carries on pressing after a refusal — a routine typing into whatever
// happened to be in front.
func TestAFailedStepStopsWhatFollowsInAnEditedRoutine(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})
	all := whatItAsked(t, d, rt, 0)

	// The third call is the first Click. Refusing it must end the run there.
	if !strings.HasPrefix(all[2], "OS's Click with") {
		t.Fatalf("call 3 is %q, so this test is refusing the wrong thing: %v",
			all[2], all)
	}
	stopped := whatItAsked(t, d, rt, 3)
	if len(stopped) != 3 {
		t.Errorf("%d calls were made after the first press was refused, so nothing "+
			"stopped: %v", len(stopped), stopped)
	}
	if len(stopped) >= len(all) {
		t.Errorf("a refused press cost the routine nothing: %d calls of %d",
			len(stopped), len(all))
	}
}

func hasCall(calls []string, prefix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func callAt(calls []string, i int) string {
	if i < len(calls) {
		return calls[i]
	}
	return "<nothing>"
}

// ── THE WRITE ────────────────────────────────────────────────────────────────

// THE ACCOUNT IS WRITTEN AFTER THE SOURCE, AND IT CANNOT BE WRITTEN BEFORE.
//
// The digest it records is of the source it describes, so there is nothing to record until that
// source is on disk. Observed through the failure rather than asserted: with the account's path
// made unwritable, the save reports that the steps did not land — and the edited source IS in
// force, which is the order.
func TestTheAccountIsWrittenAfterTheSourceAndNeverBefore(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	anAccountNothingCanWriteOver(t, d, rt)

	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	serr := d.SaveEdit(e)
	if serr == nil {
		t.Fatal("the account could not be written and the save said nothing")
	}
	if !strings.Contains(serr.Error(), "not written down beside it") {
		t.Errorf("the failure does not say what did not land: %v", serr)
	}
	// AND THE SOURCE IS THE EDITED ONE, which is only possible if it went first.
	if !strings.Contains(sourceOf(t, d, rt), "Sleep with 800") {
		t.Error("the source was not written, so the account would have had to be " +
			"written first to be the thing that failed")
	}
	// AND THE ACCOUNT STILL DESCRIBES THE VERSION BEFORE IT, which is exactly why the
	// next edit must refuse: the file and the stored steps no longer agree.
	if _, st := d.Reg.Steps(rt); st != routes.StepsMismatched {
		t.Errorf("the account reads %q after a write that did not land", st)
	}
}

// AND A SOURCE IN FORCE WITHOUT ITS ACCOUNT STILL RUNS, AND REFUSES THE NEXT EDIT.
//
// The conservative half of the write order. Execution reads only the `.marco`, so the routine
// does exactly what the person asked for; what is blocked is further editing, and it is reported
// in the same words as any other source nothing here explains.
func TestASourceWrittenWithoutItsAccountStillRunsAndRefusesTheNextEdit(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	anAccountNothingCanWriteOver(t, d, rt)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if err := d.SaveEdit(e); err == nil {
		t.Fatal("the blocked account write was not reported")
	}
	// IT RUNS, and it runs the edit.
	calls := whatItAsked(t, d, rt, 0)
	if !hasCall(calls, "OS's Sleep with 800") {
		t.Errorf("the edited routine did not run its edit: %v", calls)
	}
	// AND THE NEXT EDIT IS REFUSED, in the neutral words.
	if _, err := d.PrepareEdit(rt, nil); err == nil {
		t.Error("a routine whose steps are not written down beside it was offered " +
			"for editing anyway")
	} else if !strings.Contains(err.Error(), "no longer match") {
		t.Errorf("the refusal is not about the source and its steps: %v", err)
	}
}

// AND AN ACCOUNT THAT IS THERE AND CANNOT BE READ IS NOT AN ABSENT ONE.
//
// This was wrong and it was wrong in the one direction that matters: every read error answered
// "there is none", which is the single answer that licenses rendering the demonstration instead
// and writing over whatever the file says. A permission, a directory in its place, a handle
// something else holds — each of them meant "no edit here" to every caller.
//
// The control is in the same test on purpose: a routine with genuinely no account must still be
// editable, because that is every routine already on disk.
func TestAnAccountThatCannotBeReadIsNotTakenForAnAbsentOne(t *testing.T) {
	t.Run("there and unreadable", func(t *testing.T) {
		d, rt := aThreePressRoutine(t)
		if err := os.MkdirAll(filepath.Join(d.Reg.StepsPath(rt), "in-the-way"),
			0o755); err != nil {
			t.Fatal(err)
		}
		if _, readable := d.Reg.StepsDigest(rt); readable {
			t.Error("a path that cannot be read reports a digest")
		}
		if _, st := d.Reg.Steps(rt); st != routes.StepsUnreadable {
			t.Errorf("it reads %q, so a read failure is being taken for an "+
				"absent file", st)
		}
		for _, ask := range []struct {
			what string
			err  func() error
		}{
			{"an edit", func() error { _, e := d.PrepareEdit(rt, nil); return e }},
			{"a rebuild", func() error { _, e := d.Prepare(rt, RebuildCurrent); return e }},
		} {
			err := ask.err()
			if err == nil {
				t.Errorf("%s was offered over an account nothing read", ask.what)
				continue
			}
			if !strings.Contains(err.Error(), "could not be read") {
				t.Errorf("%s refused for another reason: %v", ask.what, err)
			}
		}
	})
	t.Run("genuinely absent", func(t *testing.T) {
		d, rt := aThreePressRoutine(t)
		at, readable := d.Reg.StepsDigest(rt)
		if !readable || at != "" {
			t.Errorf("no file at all reports (%q, %v)", at, readable)
		}
		if _, err := d.PrepareEdit(rt, nil); err != nil {
			t.Errorf("a routine with no account cannot be edited: %v", err)
		}
		if _, err := d.Prepare(rt, RebuildCurrent); err != nil {
			t.Errorf("a routine with no account cannot be rebuilt: %v", err)
		}
	})
}

// AND THE STEP LIST A REVIEW HANDS OUT IS A DEEP COPY.
//
// A `macroir.Step` carries two references — `Template`, the picture an anchored click looks for,
// and `Steps`, a repetition's body. A copy of the outer slice leaves both SHARED with the review
// it came from, so a caller rewriting a byte of an anchor image or a step inside a loop changes
// what the review says it is while its token goes on saying otherwise.
//
// The step list here is installed by hand, because no fixture in this package produces either
// reference: anchors are off unless computer vision is explicitly turned on, and nothing in these
// demonstrations repeats.
func TestTheStepListAReviewHandsOutIsADeepCopy(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	e.steps = []macroir.Step{
		{Kind: macroir.StepLoop, Count: 2, Steps: []macroir.Step{
			{Kind: macroir.StepKey, Key: "a"}}},
		{Kind: macroir.StepClick, X: 10, Y: 20, Template: []byte{1, 2, 3}},
	}
	written, merr := json.Marshal(e.steps)
	if merr != nil {
		t.Fatal(merr)
	}
	e.stepsToWrite = written
	e = e.sealed()

	got := e.Steps()
	got[0].Steps[0].Key = "z"
	got[1].Template[0] = 99
	got[0].Count = 7

	again := e.Steps()
	switch {
	case again[0].Steps[0].Key != "a":
		t.Error("a step inside a repetition changed in the review")
	case again[1].Template[0] != 1:
		t.Error("an anchor's picture changed in the review")
	case again[0].Count != 2:
		t.Error("a plain field changed in the review")
	}
	// AND THE BYTES THAT WOULD LAND BESIDE THE ROUTINE ARE THE ONES THE REVIEW WAS
	// SEALED OVER, which is the half that matters on disk.
	if string(e.stepsToWrite) != string(written) {
		t.Errorf("the account a save would write changed:\n%s\n%s",
			written, e.stepsToWrite)
	}
	if e.Token != e.sealed().Token {
		t.Error("the review no longer matches its own content")
	}
}

// anAccountNothingCanWriteOver leaves a routine with one edit saved and its account READ-ONLY, so
// the next save reaches the account write and fails there.
//
// Not a directory in the file's place, which is what this used to be: that is an account nobody
// can READ, and every caller now refuses such a routine before it writes anything — which is the
// right answer and not the state being staged here. What is wanted is a file that reads perfectly
// and cannot be replaced, so the save gets as far as the last of its three writes.
func anAccountNothingCanWriteOver(t *testing.T, d Deps, rt routes.Route) {
	t.Helper()
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 700})
	at := d.Reg.StepsPath(rt)
	if err := os.Chmod(at, 0o444); err != nil {
		t.Fatal(err)
	}
	// AND PUT IT BACK, or this test leaves a file its own temporary directory cannot
	// remove and the failure is reported against whatever runs next.
	t.Cleanup(func() { _ = os.Chmod(at, 0o644) })
}

// AN EDIT IS REFUSED WHEN ANY OF THE THREE FILES MOVED WHILE THE REVIEW WAS OPEN.
//
// A review is of a set of files and any of them can change while somebody reads it: a hand edit
// in `marco edit`, the same routine recorded again in another window, another edit of it saved
// from somewhere else. Writing anyway would land a rendering of something that is no longer
// there, and silently — the result looks exactly like an ordinary successful save.
func TestAnEditIsRefusedWhenTheWorldMovedWhileItWasOpen(t *testing.T) {
	for _, c := range []struct {
		what, says string
		move       func(t *testing.T, d Deps, rt routes.Route)
	}{
		{"the source", "saved again", func(t *testing.T, d Deps, rt routes.Route) {
			if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n"); err != nil {
				t.Fatal(err)
			}
		}},
		{"the demonstration", "recorded again", func(t *testing.T, d Deps, rt routes.Route) {
			raw, _ := d.Reg.LoadRecording(rt)
			if err := d.Reg.SaveRecording(rt, append(raw, ' ')); err != nil {
				t.Fatal(err)
			}
		}},
		{"its own steps", "step list stored beside", func(t *testing.T, d Deps, rt routes.Route) {
			// AN ACCOUNT THAT MOVED AND A SOURCE THAT DID NOT, which is the
			// case only the third digest can see: another edit of this routine
			// landing would move both, and the source check would answer
			// first and say the wrong thing about it.
			rec, _ := d.Reg.LoadRecording(rt)
			if err := d.Reg.SaveSteps(rt, routes.StoredSteps{
				Source:        routes.DigestOf(sourceOf(t, d, rt)),
				Demonstration: routes.DigestOf(string(rec)),
				Steps:         []byte(`[{"kind":"wait","ms":10}]`),
			}); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.what+" moved", func(t *testing.T) {
			d, rt := aThreePressRoutine(t)
			e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
			if err != nil {
				t.Fatalf("preparing: %v", err)
			}
			c.move(t, d, rt)
			before := sourceOf(t, d, rt)
			serr := d.SaveEdit(e)
			if serr == nil {
				t.Fatalf("the edit was saved although %s had moved", c.what)
			}
			if !strings.Contains(serr.Error(), c.says) {
				t.Errorf("the refusal does not say %q: %v", c.says, serr)
			}
			if sourceOf(t, d, rt) != before {
				t.Error("the routine was written anyway")
			}
		})
	}
}

// AND THE REVALIDATION HAPPENS INSIDE THE ROUTE'S LOCK.
//
// Making a comparison atomic is not something a file write can do; what closes the gap is holding
// the route across the reading and the writing both. Asked by trying to take the same lock from
// inside the one instant a test can stand in — if it could be had, the comparison and the writes
// would not be protected from the next writer along.
func TestAnEditRevalidatesInsideTheLock(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	ran := false
	d.AfterValidating = func() {
		ran = true
		// NOTHING IS WRITTEN YET, which is what this instant is.
		if strings.Contains(sourceOf(t, d, rt), "Sleep with 800") {
			t.Error("the source was written before the revalidation finished")
		}
		if hold, lerr := d.Reg.Lock(rt, 50_000_000); lerr == nil {
			hold.Release()
			t.Error("the route's lock could be taken while an edit was being " +
				"written, so the comparison and the write are not one")
		}
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if !ran {
		t.Error("the seam was never reached, so this test establishes nothing")
	}
}

// AN EDIT CANNOT BE SAVED TWICE, AND TWO EDITS OF ONE ROUTINE ARE TWO REVIEWS.
//
// The token's whole job. A slot keyed by the routine writes whichever arrived last, and the two
// calls are asynchronous — so a page can display one review while the backend holds another.
func TestAnEditCannotBeSavedTwice(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if err := d.SaveEdit(e); err == nil {
		t.Fatal("the same review was saved a second time")
	}
}

func TestTwoEditsOfOneRoutineAreTwoReviews(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	a, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	b, err := d.PrepareEdit(rt, []StepEdit{{Op: EditRemove, At: 4}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if a.Token == b.Token {
		t.Fatalf("two different edits of one routine share the token %q", a.Token)
	}
	// AND A REVIEW WHOSE FIELDS WERE CHANGED AFTER IT WAS SEALED IS NOT ITS OWN REVIEW.
	c := a
	c.Edits = b.Edits
	if err := d.SaveEdit(c); err == nil {
		t.Fatal("a review assembled from two was saved")
	}
	// AND THE LIBRARY IS PART OF THE TARGET.
	elsewhere := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	if err := elsewhere.SaveEdit(a); err == nil {
		t.Fatal("a review prepared in one library was saved into another")
	}
}

// AND THE ACCOUNT'S DIGEST IS PART OF WHAT A REVIEW IS, not only of what a save re-reads.
//
// A MUTATION FOUND THIS, and it is the same shape as the eighth rule of the move contract. Deleting
// the account digest from either token changed nothing any test could see, because every writer
// writes the source as well — source first — so the source digest answers a moment later and the
// field looks redundant.
//
// It is not redundant, and what it protects is the review's IDENTITY rather than the write. A
// surface holds pending reviews in a map keyed by token, and a review whose account has moved is
// one the save will refuse while a review taken afterwards is one it will accept: sharing a token
// would let a window holding the first be resolved to the second. So the case is built the only way
// it can be — the account's BYTES moved and its meaning did not, which is what another build
// rewriting that file would do — and the tokens are asked to differ.
func TestAReviewsTokenCoversWhatTheRoutineSaysAboutItself(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	editing(t, d, rt, StepEdit{Op: EditWait, At: 3, To: 800})

	before, err := d.PrepareEdit(rt, []StepEdit{{Op: EditRemove, At: 4}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	rebuiltBefore, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the rebuild: %v", err)
	}

	// THE SAME ACCOUNT, WRITTEN DIFFERENTLY. Every other digest and both renderings are
	// unchanged, so this is the one thing the third digest can be about.
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
	if _, st := d.Reg.Steps(rt); st != routes.StepsIntact {
		t.Fatalf("rewriting the account changed what it says: %q", st)
	}

	after, err := d.PrepareEdit(rt, []StepEdit{{Op: EditRemove, At: 4}})
	if err != nil {
		t.Fatalf("preparing again: %v", err)
	}
	if after.Now != before.Now || after.Was != before.Was {
		t.Fatal("this fixture no longer changes ONLY the account, so it says nothing " +
			"about what the account's digest is for")
	}
	if after.Token == before.Token {
		t.Error("two edit reviews of one routine share a token across a change to " +
			"what the routine says about itself, so a window holding one and " +
			"handed the other's token would be allowed to write with it")
	}
	rebuiltAfter, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the rebuild again: %v", err)
	}
	if rebuiltAfter.Now != rebuiltBefore.Now {
		t.Fatal("the rebuild fixture changed more than the account")
	}
	if rebuiltAfter.Token == rebuiltBefore.Token {
		t.Error("two rebuild reviews of one routine share a token across a change " +
			"to what the routine says about itself")
	}

	// AND FOR A REBUILD THAT IS NOT FROM THE ACCOUNT EITHER, which is the case the
	// basis digest does not cover: with nothing being rendered from the stored steps,
	// the only thing that moved is the file itself.
	if err := d.Reg.Save(rt, sourceOf(t, d, rt)+"\n// from somewhere else\n"); err != nil {
		t.Fatal(err)
	}
	if _, st := d.Reg.Steps(rt); st != routes.StepsMismatched {
		t.Fatalf("the fixture's account reads %q, so this is not the case being "+
			"asked about", st)
	}
	overBefore, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	stepsRaw, serr := os.ReadFile(d.Reg.StepsPath(rt))
	if serr != nil {
		t.Fatal(serr)
	}
	if err := os.WriteFile(d.Reg.StepsPath(rt), append(stepsRaw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	overAfter, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	switch {
	case overAfter.Now != overBefore.Now || overAfter.Was != overBefore.Was:
		t.Fatal("this fixture changed more than the file beside the routine")
	case overAfter.Token == overBefore.Token:
		t.Error("two rebuilds that would REMOVE the stored steps share a token " +
			"across a change to those steps, so the digest of the file is " +
			"doing nothing in the token")
	}
}

// AND THE BYTES A SAVE WOULD WRITE ARE PART OF WHAT WAS REVIEWED.
//
// A SECOND MUTATION SURVIVOR. The step list is marshalled when the review is made, and its digest
// looked redundant with the rendered source's: the steps produce the source, so a change to them
// usually changes it. Usually is not always — `codegen` ignores fields it has no use for, so two
// step lists can render identical Marco and still be two different things to write down beside the
// routine. And the field is reachable from inside this package, which is where a value assembled
// from parts of two reviews comes from.
//
// Ruling: what a derived answer is derived FROM belongs in the token. So it is in, and this is the
// test that says why.
func TestTheStepListASaveWouldWriteIsPartOfWhatWasReviewed(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	damaged := e
	damaged.stepsToWrite = []byte(`[{"kind":"key","key":"f12"}]`)
	if err := d.SaveEdit(damaged); err == nil {
		t.Fatal("a review whose step list was changed after it was sealed was saved")
	} else if !strings.Contains(err.Error(), "not the one that was reviewed") {
		t.Errorf("the refusal is not about the review's identity: %v", err)
	}
	if d.Reg.HasSteps(rt) {
		t.Error("it wrote an account anyway")
	}
	// AND THE UNDAMAGED ONE IS SAVED, or this passes for a contract that refuses
	// everything.
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("the review as prepared was refused: %v", err)
	}
	stored, st := d.Reg.Steps(rt)
	if st != routes.StepsIntact {
		t.Fatalf("the account reads %q", st)
	}
	if strings.Contains(string(stored.Steps), "f12") {
		t.Errorf("the damaged step list reached the file: %s", stored.Steps)
	}
}

// CANCELLING WRITES NOTHING, BECAUSE NOTHING WAS WRITTEN.
//
// All four files: the source, the demonstration, the images and the account.
func TestCancellingAnEditWritesNothing(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	srcBefore := sourceOf(t, d, rt)
	recBefore, _ := d.Reg.LoadRecording(rt)
	listBefore := inTheFolder(t, d, rt)

	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 800},
		StepEdit{Op: EditRemove, At: 4}})
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !e.Savable() {
		t.Fatalf("the review has nothing to save: %q", e.Refused)
	}
	// AND THEN NOBODY SAVES IT. Cancel is the absence of an operation.

	if got := sourceOf(t, d, rt); got != srcBefore {
		t.Error("the source changed")
	}
	if after, _ := d.Reg.LoadRecording(rt); string(after) != string(recBefore) {
		t.Error("the demonstration changed")
	}
	if d.Reg.HasSteps(rt) {
		t.Error("an account was written")
	}
	if got := inTheFolder(t, d, rt); got != listBefore {
		t.Errorf("the folder changed:\n%s\nbecame\n%s", listBefore, got)
	}
}

// inTheFolder is every file beside a routine, as one comparable string.
func inTheFolder(t *testing.T, d Deps, rt routes.Route) string {
	t.Helper()
	entries, err := os.ReadDir(d.Reg.ScopeDir(rt))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil {
			t.Fatal(ierr)
		}
		out = append(out, fmt.Sprintf("%s %d", e.Name(), info.Size()))
	}
	return strings.Join(out, "\n")
}

// AND AN OPERATION THAT CANNOT BE EXPRESSED AT ALL IS AN ERROR, NOT A REVIEW.
//
// The split: an edit whose RESULT is not a routine is reviewable, because there is something to
// show somebody. An edit that does not refer to anything is not — a position that is not there,
// a duration on something that has none — and there is nothing to display about it.
func TestAnOperationThatRefersToNothingIsAnError(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	for _, c := range []struct {
		what string
		edit StepEdit
		says string
	}{
		{"a step that is not there", StepEdit{Op: EditWait, At: 99, To: 10},
			"there is no step 100"},
		{"a duration on a press", StepEdit{Op: EditWait, At: 2, To: 10},
			"is not a wait"},
		{"a negative duration", StepEdit{Op: EditWait, At: 3, To: -5},
			"cannot be -5ms long"},
		{"an operation nobody named", StepEdit{Op: "reorder", At: 1},
			"is not something an edit can do"},
	} {
		t.Run(c.what, func(t *testing.T) {
			_, err := d.PrepareEdit(rt, []StepEdit{c.edit})
			if err == nil {
				t.Fatal("it was accepted")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the error does not say %q: %v", c.says, err)
			}
		})
	}
	// AND ONE STEP CANNOT BE BOTH CHANGED AND REMOVED.
	if _, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 3, To: 10},
		{Op: EditRemove, At: 3}}); err == nil {
		t.Error("a step was asked to be both changed and removed")
	}
}

// AND EVERY POSITION IN ONE REVIEW IS INTO THE LIST THAT WAS DISPLAYED.
//
// Applying the operations one after another would move the later positions under the earlier
// ones, so the two operations somebody asked for would not be the two that happened — silently,
// because the result is a perfectly good routine that does the wrong thing.
func TestEveryPositionInOneEditIsIntoTheListThatWasShown(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	// Remove step 2 (the first click) AND change the wait at 5. If the removal were
	// applied first, "5" would be the third click's wait instead.
	editing(t, d, rt, StepEdit{Op: EditRemove, At: 2}, StepEdit{Op: EditWait, At: 5, To: 777})
	src := sourceOf(t, d, rt)
	if strings.Contains(src, "X 842, Y 274") {
		t.Errorf("the removed press is still there:\n%s", src)
	}
	if !strings.Contains(src, "Sleep with 777") {
		t.Errorf("the changed wait is not there:\n%s", src)
	}
	// AND IT IS THE RIGHT WAIT: the one that was 1500, before the third press — not the
	// 2500 that would have moved into position 5 if the removal had gone first.
	if strings.Contains(src, "Sleep with 1500") {
		t.Errorf("the wait at position 5 of the shown list was not the one "+
			"changed:\n%s", src)
	}
	if !strings.Contains(src, "Sleep with 2500") {
		t.Errorf("a wait that nobody asked about changed:\n%s", src)
	}
}
