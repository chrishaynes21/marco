package orchestrator

// A RESCOPE IS A MOVE, AND A MOVE IS WHERE ROUTINES GET LOST.
//
// Every test here is about one of the five things that make it a move rather than a save: both
// ends locked in an order neither caller chooses, both revalidated under those locks, a
// destination that is created exclusively unless somebody consented to the version that is
// there, a destination read back before anything is removed, and an original that is never
// deleted when it is newer than the one reviewed — or when the removal did not happen, which is
// reported as a COPY and not as a move.
//
// The interleavings are deterministic. `AfterValidating` stands in the instant between the
// revalidation and the first write, and `afterDestination` in the instant when BOTH routines
// exist — which is the only moment a move has that a save does not. Each of them is where the
// defect would be, and standing in it is the only way to show it is closed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aMovableRoutine is a routine saved to bring its application forward, with its demonstration
// beside it, and nothing in it that checks a window — so all three scopes are open to it.
//
// `aPlainDemonstration` deliberately: a recording whose presses check their own window has ONE
// scope it can be performed under, which is the subject of its own test below rather than the
// fixture for everything else.
func aMovableRoutine(t *testing.T) (Deps, routes.Route) {
	t.Helper()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("type my address", aPlainDemonstration("a"), "notepad",
		"the stop key", Unstated)
	rt, err := d.Save(dr, ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("saving the fixture: %v", err)
	}
	if !d.Reg.HasRecording(rt) {
		t.Fatal("the fixture has no demonstration beside it, so there is nothing a " +
			"move could be rendered from")
	}
	return d, rt
}

// moved is the move, prepared and performed, with the refusals reported the way a caller sees
// them rather than as a panic in a helper.
func moved(t *testing.T, d Deps, rt routes.Route, want Scope, c Consent) (Rescoped, error) {
	t.Helper()
	m, err := d.PrepareRescope(rt, want, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused at preparation: %s", m.Refused)
	}
	return d.Rescope(m, c)
}

// ── THE CONTROL: IT WORKS, AND BOTH ENDS ARE RIGHT ────────────────────────────

// A MOVE LANDS THE ROUTINE AT ITS NEW IDENTITY, WITH ITS DEMONSTRATION, AND ENDS THE OLD ONE.
//
// The successful control, and it earns its place twice over: every other test here asserts that
// something did NOT happen, and a contract that refuses everything passes all of them.
func TestAMoveLandsTheRoutineAtItsNewScopeAndEndsTheOld(t *testing.T) {
	d, rt := aMovableRoutine(t)
	was, _ := os.ReadFile(d.Reg.Path(rt))

	got, err := moved(t, d, rt, ScopeApp, Consent{})
	if err != nil {
		t.Fatalf("the move was refused: %v", err)
	}
	if !got.Removed {
		t.Errorf("the original was not removed: %s", got.Says)
	}
	if !strings.HasPrefix(got.Says, "Moved") {
		t.Errorf("a completed move reads %q", got.Says)
	}
	// ── THE DESTINATION IS THERE, AND IS THE SCOPE THAT WAS ASKED FOR ──
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	if got.To != to {
		t.Fatalf("it went to %+v, want %+v", got.To, to)
	}
	src, rerr := os.ReadFile(d.Reg.Path(to))
	if rerr != nil {
		t.Fatalf("nothing is at the destination: %v", rerr)
	}
	// AND THE SOURCE IS THE ONE FOR THAT SCOPE. A routine that works only in place
	// switches nothing, so the `Activate` a focus routine carries is gone — which is the
	// whole reason a move is a re-render and not a file rename.
	if strings.Contains(string(src), "Activate") {
		t.Errorf("the moved routine still brings its application forward, so it was "+
			"not rendered for the scope it moved to:\n%s", src)
	}
	if !strings.Contains(string(was), "Activate") {
		t.Fatal("the fixture did not bring its application forward to begin with, " +
			"so the assertion above proves nothing")
	}
	// ── AND ITS DEMONSTRATION CAME WITH IT ──
	//
	// A routine that arrives without one cannot be reviewed, rebuilt or moved again, and
	// the original it was about to replace was the only other copy.
	if !d.Reg.HasRecording(to) {
		t.Error("the moved routine has no demonstration beside it, so it can never " +
			"be moved or rebuilt again")
	}
	// ── AND THE ORIGINAL IS GONE, SOURCE AND DEMONSTRATION BOTH ──
	if _, err := os.Stat(d.Reg.Path(rt)); !os.IsNotExist(err) {
		t.Errorf("the original source is still at %s", RouteID(rt))
	}
	if d.Reg.HasRecording(rt) {
		t.Errorf("the original's demonstration is still at %s, so the library holds "+
			"half a routine", RouteID(rt))
	}
}

// AND MOVING IT WHERE IT ALREADY IS DOES NOTHING AT ALL.
//
// Not performed as a no-op move: the destination identity would BE the source identity, which
// means one lock taken twice, an exclusive create onto the file being moved, and a removal of
// what had just been written. Refused before anything is read.
func TestMovingARoutineToTheScopeItAlreadyHasIsNothingToDo(t *testing.T) {
	d, rt := aMovableRoutine(t)
	before := theWholeLibrary(t, d)

	_, err := d.PrepareRescope(rt, ScopeAppAnywhere, "")
	if err == nil {
		t.Fatal("moving a routine to the scope it already has was prepared as a move")
	}
	if !strings.Contains(err.Error(), "nothing to move") {
		t.Errorf("the refusal reads %q", err)
	}
	if after := theWholeLibrary(t, d); !sameLibrary(before, after) {
		t.Error("preparing a move that was refused wrote to the library")
	}
}

// ── THE DESTINATION: EXCLUSIVE, OR CONSENTED TO BY VERSION ────────────────────

// A ROUTINE ALREADY AT THE DESTINATION IS NOT REPLACED BY A MOVE NOBODY CONSENTED TO.
//
// Consent to move THIS routine is not consent to lose THAT one. They are two routines, two
// recordings and two decisions, and the second one has not been made.
func TestAMoveOntoAnOccupiedIdentityNeedsItsOwnConsent(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	// SOMETHING ELSE AT THE DESTINATION, saved through the production path so it has
	// its own demonstration and its own version.
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatalf("saving the routine in the way: %v", err)
	}
	theirs, _ := os.ReadFile(d.Reg.Path(to))

	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	// THE REVIEW SAYS SO, before anybody presses anything.
	if !m.Taken || m.TakenSays == "" {
		t.Errorf("the review does not say something is in the way: %+v", m)
	}
	if _, err := d.Rescope(m, Consent{}); err == nil {
		t.Fatal("a move replaced a routine nobody consented to replacing")
	} else if !strings.Contains(err.Error(), "already a routine at") {
		t.Errorf("the refusal reads %q", err)
	}
	// AND NEITHER END MOVED.
	if now, _ := os.ReadFile(d.Reg.Path(to)); string(now) != string(theirs) {
		t.Error("the routine at the destination was written over")
	}
	if _, err := os.Stat(d.Reg.Path(rt)); err != nil {
		t.Errorf("the original was removed by a move that refused: %v", err)
	}
}

// AND A CONSENT IS TO A VERSION, NOT TO A NAME.
//
// The identity does not change when the contents do. A consent that named only the route would
// be satisfied by whatever is there at the instant of writing — including a routine saved there
// after the person looked, which they have never seen.
func TestConsentToReplaceTheDestinationIsBoundToTheVersionShown(t *testing.T) {
	for _, c := range []struct {
		what   string
		spoil  func(t *testing.T, d Deps, to routes.Route)
		reads  string
		stands bool // whether the destination still holds what it held
	}{
		{"the destination was saved again after it was shown",
			func(t *testing.T, d Deps, to routes.Route) {
				if err := d.Reg.Replace(to, "the Other is a script. // newer\n"); err != nil {
					t.Fatal(err)
				}
			},
			"saved again since you were shown it", true},
		{"the consent names another routine",
			func(t *testing.T, d Deps, to routes.Route) {},
			"", false},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := aMovableRoutine(t)
			to := routes.Route{App: "notepad", Slug: rt.Slug}
			other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"),
				"notepad", "the stop key", Unstated)
			if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
				t.Fatal(err)
			}
			m, err := d.PrepareRescope(rt, ScopeApp, "")
			if err != nil {
				t.Fatal(err)
			}
			// THE CONSENT SOMEBODY WOULD GIVE, to what they were shown.
			src, rec := d.versionAt(to)
			consent := Consent{ID: m.TakenID, Source: src, Recording: rec}
			if c.reads == "" {
				consent.ID = "somewhere/else/entirely"
			}
			c.spoil(t, d, to)
			held, _ := os.ReadFile(d.Reg.Path(to))

			if _, err := d.Rescope(m, consent); err == nil {
				t.Fatal("the move wrote over the destination anyway")
			}
			if now, _ := os.ReadFile(d.Reg.Path(to)); string(now) != string(held) {
				t.Error("the destination was written over")
			}
			if _, err := os.Stat(d.Reg.Path(rt)); err != nil {
				t.Errorf("the original was removed: %v", err)
			}
		})
	}
}

// AND WITH THE RIGHT CONSENT IT REPLACES, AND THE MOVE COMPLETES.
//
// The control for the three refusals above.
func TestAMoveReplacesTheDestinationWhenTheConsentMatchesIt(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	src, rec := d.versionAt(to)
	got, err := d.Rescope(m, Consent{ID: m.TakenID, Source: src, Recording: rec})
	if err != nil {
		t.Fatalf("a consented move was refused: %v", err)
	}
	if !got.Removed || !strings.HasPrefix(got.Says, "Moved") {
		t.Errorf("the move reads %q", got.Says)
	}
	if now, _ := os.ReadFile(d.Reg.Path(to)); string(now) != m.Now {
		t.Error("what is at the destination is not what the review displayed")
	}
}

// ── THE INTERLEAVINGS ─────────────────────────────────────────────────────────

// A DESTINATION THAT APPEARS INSIDE THE LOCK IS REFUSED BY THE CREATE ITSELF.
//
// The revalidation says the destination is free; the write is the next statement. Between them
// is where another writer that does NOT take the lock lands — a hand edit, a sync, a save from a
// build without this contract — and a caller told the path was free has consented to nothing
// about what is now there.
//
// `O_CREATE|O_EXCL` closes the gap rather than narrowing it: the test and the write are one
// operation. Standing in the instant is the only way to show that.
func TestARoutineThatAppearsAtTheDestinationMidMoveIsNotWrittenOver(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	theirs := "the Interloper is a script. // written between the check and the write\n"
	d.AfterValidating = func() {
		if err := d.Reg.Create(to, theirs); err != nil {
			t.Fatalf("the interleaved save failed: %v", err)
		}
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Rescope(m, Consent{}); err == nil {
		t.Fatal("the move wrote over a routine that appeared while it was writing")
	}
	if now, _ := os.ReadFile(d.Reg.Path(to)); string(now) != theirs {
		t.Errorf("what the other writer saved was overwritten: %q", now)
	}
	if _, err := os.Stat(d.Reg.Path(rt)); err != nil {
		t.Errorf("the original was removed even though the move failed: %v", err)
	}
}

// AND AN ORIGINAL THAT CHANGED WHILE IT WAS BEING REVIEWED IS NOT MOVED.
//
// The review was of a pair of files. A move made from it after one of them changed would write a
// rendering of a recording that is no longer there, over — or rather beside — a source nobody
// looked at, and then delete the thing that was edited.
func TestAMoveIsRefusedWhenTheOriginalChangedAfterItWasReviewed(t *testing.T) {
	for _, c := range []struct {
		what  string
		spoil func(t *testing.T, d Deps, rt routes.Route)
		reads string
	}{
		{"its source was edited",
			func(t *testing.T, d Deps, rt routes.Route) {
				raw, _ := os.ReadFile(d.Reg.Path(rt))
				if err := d.Reg.Replace(rt, string(raw)+"// edited\n"); err != nil {
					t.Fatal(err)
				}
			},
			"saved again"},
		{"it was recorded again",
			func(t *testing.T, d Deps, rt routes.Route) {
				if err := d.Reg.SaveRecording(rt, []byte("[]")); err != nil {
					t.Fatal(err)
				}
			},
			"recorded again"},
		{"it was deleted",
			func(t *testing.T, d Deps, rt routes.Route) {
				if err := d.Reg.Delete(rt); err != nil {
					t.Fatal(err)
				}
			},
			"no longer at"},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := aMovableRoutine(t)
			to := routes.Route{App: "notepad", Slug: rt.Slug}
			m, err := d.PrepareRescope(rt, ScopeApp, "")
			if err != nil {
				t.Fatal(err)
			}
			c.spoil(t, d, rt)

			if _, err := d.Rescope(m, Consent{}); err == nil {
				t.Fatal("the move went ahead against a routine that had changed")
			} else if !strings.Contains(err.Error(), c.reads) {
				t.Errorf("the refusal reads %q and does not mention %q",
					err, c.reads)
			}
			if _, err := os.Stat(d.Reg.Path(to)); !os.IsNotExist(err) {
				t.Error("something was written at the destination anyway")
			}
		})
	}
}

// AND AN ORIGINAL THAT CHANGED WHILE THE DESTINATION WAS BEING WRITTEN IS NOT DELETED.
//
// THE ONE INTERVAL A MOVE HAS THAT A SAVE DOES NOT: the destination exists and the original is
// still there. Everything after it — reading the destination back, revalidating the original,
// removing it — is about what can happen in exactly this instant.
//
// Both locks are held across it, so the writer this protects against is the one that does not
// take them: `marco edit`, a text editor, a sync. Deleting that edit is deleting work nobody
// reviewed, and the move has already succeeded at the other end — so this is the case that must
// report a COPY.
func TestAMoveNeverDeletesAnOriginalThatChangedAndSaysItCopied(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	edited := ""
	d.afterDestination = func() {
		// ── AND BOTH ENDS ARE HELD WHILE THIS IS TRUE ──
		//
		// The one instant when two routines exist is also the one instant in which
		// BOTH locks can be observed: taking either here must fail, because the
		// move has it. A move that locked only the route it came from would pass
		// every other test in this file — nothing else here contends for a lock —
		// and would leave another writer free to create the destination between
		// the revalidation and the write.
		for _, held := range []routes.Route{rt, to} {
			if h, err := d.Reg.Lock(held, 50*time.Millisecond); err == nil {
				h.Release()
				t.Errorf("%s is not locked while the move is writing",
					RouteID(held))
			}
		}
		raw, _ := os.ReadFile(d.Reg.Path(rt))
		edited = string(raw) + "// somebody edited this while the move ran\n"
		if err := d.Reg.Replace(rt, edited); err != nil {
			t.Fatalf("the interleaved edit failed: %v", err)
		}
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Rescope(m, Consent{})
	if err != nil {
		t.Fatalf("the move reported an error rather than a copy: %v", err)
	}
	// ── IT IS A COPY, AND IT SAYS SO ──
	if got.Removed {
		t.Error("the move reports the original as removed")
	}
	for _, says := range []string{"Copied", "original retained"} {
		if !strings.Contains(got.Says, says) {
			t.Errorf("the result reads %q and does not say %q", got.Says, says)
		}
	}
	if strings.HasPrefix(got.Says, "Moved") {
		t.Errorf("two routines exist and the result calls it a move: %q", got.Says)
	}
	// ── AND THE EDIT IS STILL THERE ──
	now, rerr := os.ReadFile(d.Reg.Path(rt))
	if rerr != nil {
		t.Fatalf("the edited original was deleted: %v", rerr)
	}
	if string(now) != edited {
		t.Error("the original is not the edited version")
	}
	// ── AND THE DESTINATION IS WHAT WAS REVIEWED ──
	if dst, _ := os.ReadFile(d.Reg.Path(to)); string(dst) != m.Now {
		t.Error("the destination is not the source the review displayed")
	}
}

// AND A DESTINATION THAT CANNOT BE READ BACK IS NOT A MOVE EITHER.
//
// A write that returned no error is not a routine that is there. The original is about to be
// deleted on the strength of this one reading, so it is taken — source AND demonstration — and
// anything else leaves both ends alone and says so.
func TestAMoveReadsTheDestinationBackBeforeRemovingTheOriginal(t *testing.T) {
	for _, c := range []struct {
		what  string
		spoil func(t *testing.T, d Deps, to routes.Route)
	}{
		{"the source that landed is not the one written",
			func(t *testing.T, d Deps, to routes.Route) {
				if err := d.Reg.Replace(to, "// something else entirely\n"); err != nil {
					t.Fatal(err)
				}
			}},
		{"the demonstration that landed is not the one written",
			func(t *testing.T, d Deps, to routes.Route) {
				if err := d.Reg.SaveRecording(to, []byte("[]")); err != nil {
					t.Fatal(err)
				}
			}},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := aMovableRoutine(t)
			to := routes.Route{App: "notepad", Slug: rt.Slug}
			d.afterDestination = func() { c.spoil(t, d, to) }
			m, err := d.PrepareRescope(rt, ScopeApp, "")
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Rescope(m, Consent{})
			if err != nil {
				t.Fatalf("the move errored rather than reporting a copy: %v", err)
			}
			if got.Removed {
				t.Error("the original was removed on the strength of a write " +
					"that did not land")
			}
			if !strings.Contains(got.Says, "Copied") {
				t.Errorf("the result reads %q", got.Says)
			}
			if _, err := os.Stat(d.Reg.Path(rt)); err != nil {
				t.Errorf("the original is gone: %v", err)
			}
		})
	}
}

// ── THE RULES THAT DO NOT NEED AN INTERLEAVING ────────────────────────────────

// BOTH ENDS ARE LOCKED IN THE SAME ORDER WHICHEVER WAY THE MOVE GOES.
//
// Two moves in opposite directions is the case: one window moving `settings` out of in-place
// while another moves it back takes the same two locks in opposite orders, and each waits for
// the one the other holds. A pure function so the rule can be read from BOTH input orders — a
// test that ran one move would pass with the caller's order either way.
func TestBothEndsOfAMoveAreLockedInTheSameOrderWhicheverWayItGoes(t *testing.T) {
	inPlace := routes.Route{App: "notepad", Slug: "type-my-address"}
	anywhere := routes.Route{App: "notepad", Focus: true, Slug: "type-my-address"}
	global := routes.Route{Slug: "type-my-address"}

	for _, pair := range [][2]routes.Route{
		{inPlace, anywhere}, {inPlace, global}, {anywhere, global},
	} {
		a1, b1 := lockOrder(pair[0], pair[1])
		a2, b2 := lockOrder(pair[1], pair[0])
		if a1 != a2 || b1 != b2 {
			t.Errorf("moving %s -> %s locks %s first and moving it back locks "+
				"%s first, so two moves of one routine can wait on each "+
				"other for ever", RouteID(pair[0]), RouteID(pair[1]),
				RouteID(a1), RouteID(a2))
		}
		if a1 == b1 {
			t.Errorf("both locks are the same route (%s)", RouteID(a1))
		}
	}
}

// AND A ROUTINE WHOSE PRESSES CHECK THEIR WINDOW CANNOT BE MOVED SOMEWHERE THEY WOULD REFUSE.
//
// The same rule the save path applies, asked at preparation so it is a sentence in a review
// rather than a refusal after somebody decided. Two of the three scopes bring no window forward,
// and a press that asked to be checked against one has nothing to be checked against.
func TestAGuardedRoutineCannotBeMovedWhereItsPressesWouldRefuse(t *testing.T) {
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("ark settings", aWindowCheckingDemonstration(), "arkascended",
		"the stop key", Unstated)
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Scope{ScopeApp, ScopeEverywhere} {
		m, err := d.PrepareRescope(rt, want, "")
		if err != nil {
			t.Fatalf("%q: %v", want, err)
		}
		if m.Refused == "" {
			t.Fatalf("%q was offered for a routine whose every press would "+
				"then refuse", want)
		}
		if m.Movable() {
			t.Errorf("%q is refused and still movable", want)
		}
		// AND A REFUSED REVIEW CANNOT BE ACTED ON, whatever a caller does with it.
		if _, err := d.Rescope(m, Consent{}); err == nil {
			t.Errorf("%q: a refused move was performed", want)
		}
	}
}

// AND A REVIEW THAT WAS EDITED AFTER IT WAS PREPARED IS NOT ACTED ON.
//
// The token is derived from everything the review showed and everything the move would touch, so
// a `Rescoping` assembled from parts of two — by a caller, by a bug, by a surface holding a slot
// keyed on the routine — no longer matches its own content. What this prevents is writing bytes
// nobody was shown.
func TestAMoveOfferedWithAnAlteredReviewIsRefused(t *testing.T) {
	d, rt := aMovableRoutine(t)
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		what  string
		alter func(Rescoping) Rescoping
	}{
		{"the source it would write", func(m Rescoping) Rescoping {
			m.Now += "// and this line, which nobody reviewed\n"
			return m
		}},
		{"where it would go", func(m Rescoping) Rescoping {
			m.To = routes.Route{App: "discord", Slug: m.To.Slug}
			return m
		}},
		{"the library it was prepared in", func(m Rescoping) Rescoping {
			m.library = filepath.Join(m.library, "elsewhere")
			return m
		}},
	} {
		t.Run(c.what, func(t *testing.T) {
			before := theWholeLibrary(t, d)
			if _, err := d.Rescope(c.alter(m), Consent{}); err == nil {
				t.Fatal("an altered review was acted on")
			}
			if after := theWholeLibrary(t, d); !sameLibrary(before, after) {
				t.Error("a refused move wrote to the library")
			}
		})
	}
}

// AND TWO REVIEWS OF ONE MOVE THAT SAW DIFFERENT DESTINATIONS ARE TWO REVIEWS.
//
// # Why this is a test and not a reading of `sealed`
//
// Because the token is an IDENTITY, and a surface holds reviews in a map keyed by it — Studio
// does exactly that for the rebuild flow, and for the same reason: a window can have two
// preparations in flight and the page can be showing one while the window holds the other.
//
// A review prepared when the destination was empty and one prepared after a routine appeared
// there are two different things to approve: one says "this will be created", the other says
// "this will replace that". If they share a token, resolving one to the other hands back the
// review somebody did not look at — and the destination's version is the only field that differs
// between them.
//
// Everything else the token binds is already covered by the refusals above. This one was found
// by a mutation that removed the destination's digests from the token and broke nothing: the
// revalidation under the locks refuses the same case a moment later, so the binding is defence in
// depth for the WRITE — and is load-bearing for the IDENTITY, which is what this holds.
func TestTwoMovesThatSawDifferentDestinationsAreNotTheSameReview(t *testing.T) {
	d, rt := aMovableRoutine(t)
	empty, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Taken {
		t.Fatal("the fixture's destination is occupied, so the two preparations " +
			"below differ in nothing")
	}
	// AND NOW SOMETHING IS THERE. The same routine, the same scope, the same
	// rendering — and a different thing to agree to.
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	taken, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	if !taken.Taken {
		t.Fatal("the second preparation does not see the routine in the way")
	}
	if taken.Now != empty.Now {
		t.Fatal("the two preparations render different sources, so sharing a token " +
			"would be caught by something else and this proves nothing")
	}
	if taken.Token == empty.Token {
		t.Error("a review that would CREATE at the destination and one that would " +
			"REPLACE there share a token, so a surface holding both by token " +
			"has one review standing for the other")
	}
}

// AND A ROUTINE WITH NO DEMONSTRATION CANNOT BE MOVED AT ALL.
//
// The difference between the scopes is IN the generated source — the `Activate` a focus route
// carries and the other two have stripped — so a move is a re-render, and there is nothing to
// render from. Rewriting generated Marco by hand is how two things that have to agree stop
// agreeing.
func TestARoutineWithNoDemonstrationCannotBeMoved(t *testing.T) {
	d, rt := aMovableRoutine(t)
	if err := os.Remove(d.Reg.RecordingPath(rt)); err != nil {
		t.Fatal(err)
	}
	_, err := d.PrepareRescope(rt, ScopeApp, "")
	if err == nil {
		t.Fatal("a routine with no demonstration was offered a move")
	}
	if !strings.Contains(err.Error(), "Record it again") {
		t.Errorf("the refusal reads %q and does not say what to do", err)
	}
}

// AND A HAND EDIT IS SHOWN AS WHAT THE MOVE WOULD COST, BEFORE IT COSTS IT.
//
// A move re-renders from the demonstration, so somebody's own edit to the generated file is not
// carried across — there is no third thing this could do that would not be inventing source
// nobody wrote. What it can do is show them: `Was` is their file, edit and all, and `Now` is what
// would be written. Approving that is approving the loss, which is a decision rather than a
// surprise.
func TestAHandEditIsShownBeforeAMoveReplacesIt(t *testing.T) {
	d, rt := aMovableRoutine(t)
	raw, _ := os.ReadFile(d.Reg.Path(rt))
	mine := string(raw) + "// my own note, which I typed\n"
	if err := d.Reg.Replace(rt, mine); err != nil {
		t.Fatal(err)
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Was != mine {
		t.Error("the review does not show the file as it stands, so a hand edit is " +
			"lost without being seen")
	}
	if strings.Contains(m.Now, "my own note") {
		t.Error("the rendered version claims to carry the hand edit")
	}
	if m.Now == m.Was {
		t.Fatal("the two sides of the review are identical, so there is nothing to " +
			"decide about")
	}
}

// sameLibrary reports whether two readings of the store are the same files with the same
// contents.
func sameLibrary(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, digest := range before {
		if after[path] != digest {
			return false
		}
	}
	return true
}

// AND A GLOBAL ROUTINE NEEDS AN APPLICATION NAMED BEFORE IT CAN BE MOVED INTO ONE.
//
// A routine that works anywhere is about no application, and the two application scopes are
// about one. Nothing here may invent it: the demonstration is asked first — a global routine's
// recording still knows what it was made in — and if that says nothing, the move is refused with
// what to supply rather than filed under a guess.
func TestMovingAGlobalRoutineIntoAnApplicationNeedsOneNamed(t *testing.T) {
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	// A DEMONSTRATION WITH NO SWITCH IN IT AT ALL, so nothing names an application.
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(10)},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: at(20)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(30)},
	}
	dr := d.DraftOf("paste my email", events, "", "the stop key", Unstated)
	rt, err := d.Save(dr, ScopeEverywhere, "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Refused == "" {
		t.Fatal("a global routine was moved into an application nobody named")
	}
	if !strings.Contains(m.Refused, "name the application") {
		t.Errorf("the refusal reads %q", m.Refused)
	}
	// AND WITH ONE NAMED IT GOES. The control: the refusal is about the missing
	// application and not about global routines.
	named, err := d.PrepareRescope(rt, ScopeApp, "notepad")
	if err != nil {
		t.Fatalf("preparing with an application named: %v", err)
	}
	if named.Refused != "" {
		t.Fatalf("naming the application did not help: %s", named.Refused)
	}
	got, err := d.Rescope(named, Consent{})
	if err != nil {
		t.Fatalf("the move was refused: %v", err)
	}
	if got.To.App != "notepad" || !got.Removed {
		t.Errorf("it went to %+v (removed=%v)", got.To, got.Removed)
	}
}
