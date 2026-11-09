package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ── ORDERING: CLEANUP BEFORE AN IDENTITY CHANGES ──

// corruptBindings makes the shortcuts file unreadable, so every bindings write refuses.
func corruptBindings(t *testing.T, d Deps) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.Reg.Dir, "bindings.json"), []byte("{broken"),
		0o644); err != nil {
		t.Fatal(err)
	}
}

// replacingMove is a move of a routine onto an identity that already holds another one.
func replacingMove(t *testing.T) (Deps, routes.Route, routes.Route) {
	t.Helper()
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	return d, rt, to
}

// At the instant before a replacing move publishes anything, the replaced routine is still at the
// destination and its shortcuts are ALREADY gone.
//
// Moving the invalidation after the write must fail this.
func TestAReplacedRoutinesShortcutsGoBeforeItIsReplaced(t *testing.T) {
	d, rt, to := replacingMove(t)
	assign(t, d, Assignment{Key: "o", ListenIn: "notepad", Steps: []routes.Route{to}})
	was, _ := d.versionAt(to)
	checked := false
	d.AfterShortcutsInvalidated = func() {
		checked = true
		if now, _ := d.versionAt(to); now != was {
			t.Error("the destination was written before its shortcuts were invalidated")
		}
		if _, ok := d.Reg.BindingAt("notepad", "o"); ok {
			t.Error("the replaced play's shortcut was still there when the move went " +
				"on to publish over it")
		}
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	src, rec := d.versionAt(to)
	if _, err := d.Rescope(m, Consent{ID: m.TakenID, Source: src, Recording: rec}); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("the instant between invalidation and the write was never reached")
	}
}

// A replacing move whose shortcut invalidation fails writes NOTHING.
func TestAReplacingMoveWhoseShortcutsCannotBeInvalidatedWritesNothing(t *testing.T) {
	d, rt, to := replacingMove(t)
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	src, rec := d.versionAt(to)
	corruptBindings(t, d)
	if _, err := d.Rescope(m, Consent{ID: m.TakenID, Source: src, Recording: rec}); err == nil {
		t.Fatal("a move went ahead with its replaced play's shortcuts still in place")
	}
	if now, _ := d.versionAt(to); now != src {
		t.Error("the destination was written although its shortcuts were not invalidated")
	}
	if !d.Reg.Has(rt) {
		t.Error("the original was removed")
	}
}

// A move whose shortcuts cannot be carried keeps the original, which they still name.
//
// Moving the carry after the original's removal must fail this.
func TestACarryThatFailsKeepsTheOriginal(t *testing.T) {
	d, rt := aMovableRoutine(t)
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	corruptBindings(t, d)
	got, err := d.Rescope(m, Consent{})
	if err != nil {
		t.Fatalf("the move: %v", err)
	}
	if got.Removed || !d.Reg.Has(rt) {
		t.Fatalf("the original was removed although its shortcuts could not follow: %+v", got)
	}
	if !strings.Contains(got.Says, "shortcuts") {
		t.Errorf("the outcome does not say why the original was kept: %s", got.Says)
	}
}

// A forget whose shortcuts cannot be removed forgets nothing.
//
// Removing the routine before its shortcuts must fail this.
func TestAForgetWhoseShortcutsCannotBeRemovedForgetsNothing(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	corruptBindings(t, d)
	if _, err := d.ForgetRoutine(freeplay); err == nil {
		t.Fatal("the forget reported success with its shortcuts unremoved")
	}
	if !d.Reg.Has(freeplay) {
		t.Error("the play was deleted although its shortcuts could not be removed first")
	}
}

// Forget, then save a routine at the same identity: it does not inherit the old shortcut.
func TestARoutineRecreatedAfterAForgetDoesNotInheritItsShortcuts(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	f, err := d.ForgetRoutine(freeplay)
	if err != nil || !f.Removed || len(f.Shortcuts) != 1 {
		t.Fatalf("forget: %+v %v", f, err)
	}
	if err := d.Reg.Save(freeplay, shortcutSrc); err != nil {
		t.Fatal(err)
	}
	if p := d.ShortcutPressed("game", "e"); p.Found {
		t.Errorf("a play saved where a forgotten one was inherited its key: %+v", p)
	}
}

// ── ASSIGNMENT INTERLEAVED WITH A FORGET OR A RENAME ──

// interleave runs other in a goroutine at the instant after the assignment revalidated, and gives
// it time to finish if nothing stops it.
func interleave(d *Deps, other func()) chan struct{} {
	done := make(chan struct{})
	d.AfterShortcutChecked = func() {
		go func() { other(); close(done) }()
		time.Sleep(150 * time.Millisecond)
	}
	return done
}

// Without the route locks, the forget completes between the check and the write, and the
// assignment then writes a shortcut naming an identity with nothing there.
//
// Deleting the route locks in AssignShortcut must fail this.
func TestAnAssignmentInterleavedWithAForgetLeavesNoOrphan(t *testing.T) {
	d := shortcutWorld(t)
	done := interleave(&d, func() { _, _ = d.ForgetRoutine(freeplay) })
	_, _ = d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{freeplay}})
	<-done
	if d.Reg.Has(freeplay) {
		t.Fatal("the forget did not happen")
	}
	for _, b := range d.Reg.Bindings() {
		if runs(b, freeplay.ID()) {
			t.Fatalf("a shortcut names %s, which was forgotten: %+v", freeplay.ID(), b)
		}
	}
}

func TestAnAssignmentInterleavedWithARenameFollowsTheRoutine(t *testing.T) {
	d := shortcutWorld(t)
	renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	done := interleave(&d, func() { _ = d.RenameInPlace(freeplay, renamed) })
	if _, err := d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{freeplay}}); err != nil {
		t.Fatal(err)
	}
	<-done
	p := d.ShortcutPressed("game", "e")
	if len(p.Steps) != 1 || p.Steps[0].Route != renamed {
		t.Fatalf("after the rename `e fires %+v (%s), want %s", p.Steps, p.Refused,
			renamed.ID())
	}
}

// A rename whose shortcuts cannot be carried renames nothing.
func TestARenameWhoseShortcutsCannotBeCarriedRenamesNothing(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	corruptBindings(t, d)
	renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	if err := d.RenameInPlace(freeplay, renamed); err == nil {
		t.Fatal("a rename went ahead without its shortcuts")
	}
	if !d.Reg.Has(freeplay) {
		t.Error("the original was removed although its shortcuts could not follow")
	}
}

// A press pins the bytes it read under the routine's lock.
func TestAPressPinsTheRoutineItResolved(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "c", Steps: []routes.Route{copyAny}})
	p := d.ShortcutPressed("game", "c")
	if len(p.Steps) != 1 || p.Steps[0].Pin != routes.DigestOf(shortcutSrc) {
		t.Fatalf("the press did not pin the bytes it read: %+v", p.Steps)
	}
}

// unrelatedSrc is a different routine somebody saves at the renamed identity afterwards.
const unrelatedSrc = "script main...\n  do nothing.\n  do nothing.\n"

// A rename that fails, leaving whatever it wrote at the new name, must leave no shortcut that the next routine
// saved under the new name would answer.
//
// The case the previous order lost: shortcuts carried first, the rename failing, the carry back
// failing. Restoring carry-first, with a rollback that fails, must fail this. It is
// checked AFTER an unrelated routine is created at the destination, because a refusal while that
// identity is empty proves nothing about what happens once something is there.
func TestAFailedRenameLeavesNoShortcutForTheNextRoutineThere(t *testing.T) {
	for _, failing := range []string{"create", "carry"} {
		t.Run(failing, func(t *testing.T) {
			d := shortcutWorld(t)
			assign(t, d, Assignment{Key: "e", ListenIn: "game",
				Steps: []routes.Route{freeplay}})
			renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
			d.RenameFault = func(step string) error {
				if step == failing {
					return fmt.Errorf("injected: %s fails", step)
				}
				return nil
			}
			if err := d.RenameInPlace(freeplay, renamed); err == nil {
				t.Fatal("the rename reported success although a step failed")
			}
			d.RenameFault = nil
			// Somebody saves an unrelated routine at the new name (over any stray copy).
			if err := d.Reg.Save(renamed, unrelatedSrc); err != nil {
				t.Fatal(err)
			}
			p := d.ShortcutPressed("game", "e")
			for _, s := range p.Steps {
				if s.Route == renamed || s.Pin == routes.DigestOf(unrelatedSrc) {
					t.Fatalf("`e runs the unrelated play saved at %s: %+v",
						renamed.ID(), p.Steps)
				}
			}
			if len(p.Steps) != 1 || p.Steps[0].Route != freeplay ||
				p.Steps[0].Pin != routes.DigestOf(shortcutSrc) {
				t.Fatalf("`e should still run the original %s, fires %+v (%s)",
					freeplay.ID(), p.Steps, p.Refused)
			}
			if got, err := os.ReadFile(d.Reg.Path(freeplay)); err != nil ||
				string(got) != shortcutSrc {
				t.Fatalf("the original was not left intact: %q, %v", got, err)
			}
		})
	}
}

// A rename whose last step fails is a finished rename with the original left over: the shortcuts
// run the routine at its new name, which holds exactly the routine they were bound to.
func TestARenameWhoseOriginalCannotBeRemovedStillRunsTheSameRoutine(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	d.RenameFault = func(step string) error {
		if step == "finish" {
			return fmt.Errorf("injected: finish fails")
		}
		return nil
	}
	err := d.RenameInPlace(freeplay, renamed)
	if err == nil || !strings.Contains(err.Error(), "still at "+freeplay.ID()) {
		t.Fatalf("the partial rename was not said: %v", err)
	}
	d.RenameFault = nil
	if !d.Reg.Has(freeplay) {
		t.Error("the original was removed although that step failed")
	}
	p := d.ShortcutPressed("game", "e")
	if len(p.Steps) != 1 || p.Steps[0].Route != renamed ||
		p.Steps[0].Pin != routes.DigestOf(shortcutSrc) {
		t.Fatalf("`e should run the same play at %s, fires %+v (%s)", renamed.ID(),
			p.Steps, p.Refused)
	}
}

// A rename carries its shortcuts only once the copy exists: at the instant of the carry the new
// name already holds the routine's bytes, and before it the shortcuts still name the original.
func TestARenameCarriesOnlyOnceTheNewNameHoldsTheRoutine(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	reached := false
	d.RenameFault = func(step string) error {
		if step != "carry" {
			return nil
		}
		reached = true
		if got, err := os.ReadFile(d.Reg.Path(renamed)); err != nil || string(got) != shortcutSrc {
			t.Errorf("the shortcuts were about to move to %s before it held the play",
				renamed.ID())
		}
		if b, ok := d.Reg.BindingAt("game", "e"); !ok || !runs(b, freeplay.ID()) {
			t.Errorf("the shortcut had already left the original before the carry: %+v", b)
		}
		return nil
	}
	if err := d.RenameInPlace(freeplay, renamed); err != nil {
		t.Fatal(err)
	}
	if !reached {
		t.Fatal("the carry was never reached")
	}
	if d.Reg.Has(freeplay) {
		t.Error("the original is still there after a rename that succeeded")
	}
}

// A rename whose carry fails deletes nothing: its copy is left at the new name and reported, no
// shortcut names it, and the original keeps its shortcuts.
func TestARenameWhoseCarryFailsLeavesItsCopyAndSaysSo(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	d.RenameFault = func(step string) error {
		if step == "carry" {
			return fmt.Errorf("injected: carry fails")
		}
		return nil
	}
	err := d.RenameInPlace(freeplay, renamed)
	if err == nil || !strings.Contains(err.Error(), "left at "+renamed.ID()) {
		t.Fatalf("what was left at the new name was not reported: %v", err)
	}
	if !d.Reg.Has(freeplay) || !d.Reg.Has(renamed) {
		t.Error("a failed rename removed a file")
	}
	for _, b := range d.Reg.Bindings() {
		if runs(b, renamed.ID()) {
			t.Fatalf("a shortcut names the copy a failed rename left: %+v", b)
		}
	}
}

// What a failed creation finds at the new name is left alone, INCLUDING a file whose bytes look
// like a partial copy: an empty file, or a prefix of the source. Bytes are not evidence of who
// wrote a file, so neither may be deleted on them.
func TestAFailedRenameLeavesSomebodyElsesFileAlone(t *testing.T) {
	for name, theirs := range map[string]string{
		"unrelated": unrelatedSrc,
		"empty":     "",
		"prefix":    shortcutSrc[:len(shortcutSrc)/2],
		"identical": shortcutSrc,
	} {
		t.Run(name, func(t *testing.T) {
			d := shortcutWorld(t)
			renamed := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
			d.RenameFault = func(step string) error {
				if step == "create" {
					// Another writer, not taking the lock, put this there.
					if err := os.WriteFile(d.Reg.Path(renamed), []byte(theirs),
						0o644); err != nil {
						t.Fatal(err)
					}
					return fmt.Errorf("injected: create fails")
				}
				return nil
			}
			if err := d.RenameInPlace(freeplay, renamed); err == nil {
				t.Fatal("the rename reported success although its creation failed")
			}
			got, err := os.ReadFile(d.Reg.Path(renamed))
			if err != nil || string(got) != theirs {
				t.Fatalf("a refused rename removed or changed another writer's file: "+
					"%q, %v", got, err)
			}
			if !d.Reg.Has(freeplay) {
				t.Error("the original was removed by a rename that failed")
			}
		})
	}
}

// A replacement somebody agrees to keeps the routine's shortcuts, the question says so, and the
// key then runs the new version (owner's ruling, 2026-11-03: a new version of the same routine,
// unlike a forget followed by an unrelated routine saved under the name).
func TestAReplacementKeepsItsShortcutsAndSaysSo(t *testing.T) {
	d, rt := aMovableRoutine(t)
	assign(t, d, Assignment{Key: "a", ListenIn: "notepad", Steps: []routes.Route{rt}})
	again := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	at := d.Collision(again, ScopeAppAnywhere, "notepad")
	if !at.Exists || len(at.Shortcuts) != 1 || at.Shortcuts[0] != "a in notepad" {
		t.Fatalf("the question does not name the shortcut it keeps: %+v", at)
	}
	if !strings.Contains(at.Says, "(a in notepad) is kept and will run the new version") {
		t.Fatalf("the replacement question does not say the shortcut is kept: %s", at.Says)
	}
	if _, err := d.Keep(again, ScopeAppAnywhere, "notepad", Consent{ID: at.ID,
		Source: at.Source, Recording: at.Recording}); err != nil {
		t.Fatal(err)
	}
	now, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	p := d.ShortcutPressed("notepad", "a")
	if len(p.Steps) != 1 || p.Steps[0].Route != rt || p.Steps[0].Pin != routes.DigestOf(string(now)) {
		t.Fatalf("after the replacement `a fires %+v (%s), want the new version of %s",
			p.Steps, p.Refused, rt.ID())
	}
}

// With no shortcut, the question says nothing about shortcuts; with an unreadable shortcuts file,
// it says that, and never that there are none.
func TestAReplacementQuestionNeverSaysNoShortcutsWhenItCannotTell(t *testing.T) {
	d, rt := aMovableRoutine(t)
	again := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if at := d.Collision(again, ScopeAppAnywhere, "notepad"); strings.Contains(at.Says, "shortcut") {
		t.Errorf("a play with no shortcuts was said to have some: %s", at.Says)
	}
	corruptBindings(t, d)
	at := d.Collision(again, ScopeAppAnywhere, "notepad")
	if !strings.Contains(at.Says, "Whether it has shortcuts could not be read") {
		t.Errorf("an unreadable shortcuts file was not said: %s", at.Says)
	}
}
