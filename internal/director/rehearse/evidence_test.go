package rehearse_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
)

// Carrying a proof forward, and every reason not to.
//
// The rule the optimization rests on has two halves and only one of them is interesting:
//
//	Marco may avoid proving the same unchanged fact twice.
//	Marco may not act on a fact it can no longer justify.
//
// The second half is what this file holds. A bug in the first half costs a redundant look; a bug
// in the second half sends real input into a window nobody checked.

// evidenceNow is the instant every case below is judged at.
var evidenceNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// justified is evidence that supports acting on subj_a in testgame, right now, on window 1.
func justified() rehearse.StageEvidence {
	return rehearse.StageEvidence{
		Ref: liveRef(1), Subject: "subj_a", At: evidenceNow.Add(-100 * time.Millisecond),
		From: rehearse.EvidenceVerifiedOutcome,
	}
}

func leads(ref windowref.Ref) bool { return ref.ID == "hwnd:100" && ref.Generation == 1 }

// EVERY ARM IS A REFUSAL, AND THE PREMISE IS ASSERTED FIRST.
//
// A table of "this must be false" passes trivially for a function that always returns false, so
// the base case runs first and must be TRUE. Each case below is that same evidence with exactly
// one thing wrong, so a deleted arm is the only way a case can flip.
//
// Deleting any arm of Justifies must fail one of these.
func TestCarriedEvidenceIsRefusedWhenItCannotBeJustified(t *testing.T) {
	// PREMISE. Without this, everything below passes for a function that returns false.
	if !justified().Justifies(evidenceNow, "testgame", "subj_a", leads) {
		t.Fatal("evidence that is fresh, right, and in front does not justify anything — " +
			"the premise of every case below is that this one CAN be relied on")
	}

	stale := justified()
	stale.At = evidenceNow.Add(-rehearse.MaxEvidenceAge - time.Millisecond)

	future := justified()
	future.At = evidenceNow.Add(time.Second)

	zeroTime := justified()
	zeroTime.At = time.Time{}

	noSubject := justified()
	noSubject.Subject = ""

	noWindow := justified()
	noWindow.Ref = windowref.Ref{}

	// A reference that names the right application and no actual window — what a lookup that
	// half-succeeded leaves behind. It passes every other arm.
	noHandle := justified()
	noHandle.Ref = windowref.Ref{Application: "testgame"}

	otherApp := justified()
	otherApp.Ref.Application = "chrome"

	never := func(windowref.Ref) bool { return false }
	// The PRODUCTION predicate's shape: it answers true for a window it cannot look up,
	// because a handle that has gone is a different guard's business. Anything relying on
	// the foreground check to reject nonsense has to survive this one.
	permissive := func(windowref.Ref) bool { return true }

	for _, c := range []struct {
		name     string
		e        rehearse.StageEvidence
		app      string
		want     string
		inFront  func(windowref.Ref) bool
		explains string
	}{
		{name: "no subject", e: noSubject, app: "testgame", want: "subj_a", inFront: leads,
			explains: "not being able to tell is the absence of evidence, not a weak kind"},
		{name: "no window", e: noWindow, app: "testgame", want: "subj_a", inFront: leads,
			explains: "evidence naming no window cannot be checked against the foreground"},
		{name: "no window, and a foreground that waves it through", e: noHandle,
			app: "testgame", want: "subj_a", inFront: permissive,
			explains: "the production predicate answers true for a window it cannot look " +
				"up, so the emptiness has to be refused before it is asked"},
		{name: "a different Place", e: justified(), app: "testgame", want: "subj_b",
			inFront:  leads,
			explains: "proof about one screen says nothing about another, however fresh"},
		{name: "nothing sought", e: justified(), app: "testgame", want: "", inFront: leads,
			explains: "an empty want must not compare equal to anything"},
		{name: "a different application", e: otherApp, app: "testgame", want: "subj_a",
			inFront:  leads,
			explains: "evidence established in Chrome cannot authorise a step in testgame"},
		{name: "too old", e: stale, app: "testgame", want: "subj_a", inFront: leads,
			explains: "the bound is the admission that the other checks are not omniscient"},
		{name: "from the future", e: future, app: "testgame", want: "subj_a", inFront: leads,
			explains: "a clock that went backwards must not read as freshness"},
		{name: "no timestamp at all", e: zeroTime, app: "testgame", want: "subj_a",
			inFront:  leads,
			explains: "the zero time is two millennia stale and must not pass as recent"},
		{name: "no way to ask about the foreground", e: justified(), app: "testgame",
			want: "subj_a", inFront: nil,
			explains: "an unverifiable claim about which window leads is exactly the claim " +
				"that would put input into somebody else's window"},
		{name: "the window no longer leads", e: justified(), app: "testgame", want: "subj_a",
			inFront:  never,
			explains: "something else came forward while Marco was between edges"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.e.Justifies(evidenceNow, c.app, c.want, c.inFront) {
				t.Fatalf("carried evidence was reused when %s: %s", c.name, c.explains)
			}
		})
	}
}

// AND THE WALKER FALLS THROUGH TO LOOKING FOR ITSELF.
//
// The predicate above is only half the claim. The other half is that a walker handed evidence it
// cannot justify does the ordinary thing — establishes its own source and carries on — rather
// than refusing, or worse, using it. The optimization is meant to cost nothing when it declines.
func TestAWalkerHandedUnjustifiableEvidenceEstablishesForItself(t *testing.T) {
	w := newWorld("a", "b")
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	// Evidence about another Place entirely. It cannot justify starting from subj_a.
	wrong := justified()
	wrong.Subject = "subj_zzz"

	res, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, &wrong)
	if err != nil {
		t.Fatalf("a walk offered unusable evidence refused outright: %v", err)
	}
	if !res.Completed() {
		t.Fatalf("terminal = %q; the walk should have looked for itself and carried on",
			res.Terminal)
	}
	if res.Source != "subj_a" {
		t.Fatalf("the walk started from %q — it took the offered evidence rather than "+
			"establishing its own", res.Source)
	}
}

// A COMPLETED WALK HANDS BACK WHAT IT PROVED, AND ONLY THAT.
//
// The proof is the destination perception actually RESOLVED after the action, never what the edge
// said should happen. An attempt that ended somewhere other than planned must not hand the next
// edge a claim about where it was meant to be — that is the one mistake this handoff would make
// expensive, because the next edge would then act on a screen nobody checked.
//
// Deleting the Arrived assignment must fail the first case. The `rec.Observed` / `rec.Expect`
// distinction CANNOT be reached from here — a route completes only on a directly verified last
// step, which is defined as those two agreeing — so it is held directly by
// TestOnlyACompletedWalkProvesAnything instead. Measured: the swap survives this whole file.
func TestAVerifiedWalkReturnsTheStageItProved(t *testing.T) {
	walk := func(t *testing.T, before, after string) rehearse.RehearsalResult {
		t.Helper()
		w := newWorld(before, after)
		j := livePlan()
		g := liveGrant(t, j)
		l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
			WithActuator(w, w, true).
			WithForeground(leads)
		res, err := l.Perform(context.Background(), g, j,
			windowref.Selector{Application: "testgame"}, 1, nil)
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		return res
	}

	t.Run("a completed route", func(t *testing.T) {
		res := walk(t, "a", "b")
		if !res.Completed() {
			t.Fatalf("terminal = %q", res.Terminal)
		}
		if res.Arrived.Subject != "subj_b" {
			t.Fatalf("a verified walk proved %q; the next edge has nothing to start from",
				res.Arrived.Subject)
		}
		if res.Arrived.From != rehearse.EvidenceVerifiedOutcome {
			t.Errorf("the proof is labelled %q, which understates how it was obtained",
				res.Arrived.From)
		}
		if res.Arrived.Ref.ID == "" || res.Arrived.At.IsZero() {
			t.Errorf("the proof names no window or no moment: %+v", res.Arrived)
		}
		// AND IT IS USABLE. Evidence that cannot justify the very thing it just proved
		// would be an optimization that never fires.
		if !res.Arrived.Justifies(res.Arrived.At, "testgame", "subj_b", leads) {
			t.Errorf("what the walk proved cannot justify acting on it: %+v", res.Arrived)
		}
	})

	t.Run("a walk that ended somewhere unverified proves nothing", func(t *testing.T) {
		// The screen does not change when the input lands, so the step cannot verify.
		res := walk(t, "a", "a")
		if res.Completed() {
			t.Fatalf("the premise of this case is a walk that does NOT complete; "+
				"terminal = %q", res.Terminal)
		}
		if res.Arrived.Subject != "" {
			t.Fatalf("a walk that did not arrive handed back %q as proof of where Marco "+
				"is standing", res.Arrived.Subject)
		}
	})

	t.Run("a walk that landed somewhere else proves nothing either", func(t *testing.T) {
		// The step lands on subj_c while the plan expected subj_b. That is a real
		// outcome — a remembered screen, just not the right one — and it is still not
		// a proof of anything the next edge may act from.
		res := walk(t, "a", "c")
		if res.Arrived.Subject != "" {
			t.Fatalf("a walk that went somewhere unplanned handed back %q as proof",
				res.Arrived.Subject)
		}
	})
}

// AND CARRIED EVIDENCE IS ACTUALLY USED WHEN IT HOLDS.
//
// Without this, everything above is satisfied by a walker that ignores the parameter entirely.
// The observable difference is the LOOK: reusing a proof means the source is not established
// again, which is the whole point of carrying it.
func TestCarriedEvidenceSparesTheWalkerItsOpeningLook(t *testing.T) {
	count := func(carried *rehearse.StageEvidence) (int, rehearse.RehearsalResult) {
		t.Helper()
		w := newWorld("a", "b")
		j := livePlan()
		g := liveGrant(t, j)
		l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
			WithActuator(w, w, true).
			WithForeground(leads)
		res, err := l.Perform(context.Background(), g, j,
			windowref.Selector{Application: "testgame"}, 1, carried)
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.acquires, res
	}

	without, plain := count(nil)
	proof := justified()
	with, reused := count(&proof)

	if !plain.Completed() || !reused.Completed() {
		t.Fatalf("both walks must complete: %q and %q", plain.Terminal, reused.Terminal)
	}
	if reused.Source != "subj_a" {
		t.Fatalf("the reusing walk started from %q", reused.Source)
	}
	if with >= without {
		t.Fatalf("carrying a proof forward cost %d window acquisitions against %d without "+
			"it — the evidence was accepted and then the look was taken anyway",
			with, without)
	}
}

// WHAT IS ON SCREEN BEATS WHAT MARCO IS HOLDING.
//
// # The race carried evidence would otherwise open
//
// `Justifies` checks everything a stored fact can be checked against: Place, application, window,
// generation, foreground, age. It cannot check the one thing that actually happens between two
// edges — somebody clicked. A person moving from one screen of an application to another changes
// none of those: same window, same process, same generation, still in front, milliseconds old.
//
// Establishing from scratch would have caught it. So trusting carried evidence outright would be
// strictly LESS SAFE than the code it replaces, and it would fail in the worst way available:
// input planned for one screen emitted into another, with the walk reporting honestly afterwards
// that the screen did not respond as expected.
//
// Here the desktop has moved to subj_c while the caller offers a proof of subj_a. The walk must
// refuse — not act — and the refusal must be the ordinary source mismatch, reached through the
// ordinary establish, because the proof has to lose to a look rather than to a special case.
//
// Deleting the Place comparison in confirmCarried must fail this. So must deleting the
// confirmation and using the carried proof directly.
func TestCarriedEvidenceLosesToWhatIsOnScreen(t *testing.T) {
	// Nothing has been emitted, and the screen is already subj_c: the person moved.
	w := newWorld("c", "c")
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	// A proof of subj_a that passes every arm of Justifies: right Place for this grant, right
	// application, right window, in front, seconds old.
	proof := justified()
	proof.At = newLiveClock().Now()

	_, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, &proof)
	if err == nil {
		t.Fatal("a walk acted on a proof of subj_a while subj_c was on screen — the " +
			"carried evidence was never checked against a look")
	}
	if reason, _ := rehearse.RefusalOf(err); reason != rehearse.RefusalSourceMismatch {
		t.Fatalf("it refused with %q (%v), want the ordinary source mismatch: the proof "+
			"has to lose to what is on screen, through the path that already existed",
			reason, err)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the desktop before the mismatch was noticed",
			w.sent())
	}
}

// AND AN UNREADABLE SCREEN IS NOT A CONFIRMATION EITHER.
//
// The confirmation fails closed on every way of not agreeing, not only on disagreement. A reading
// that cannot be made out must send the walk to the full establish — which will refuse for its
// own reasons — rather than letting the proof stand because nothing contradicted it.
func TestAProofIsNotConfirmedByAScreenNobodyCanRead(t *testing.T) {
	w := newWorld("a", "b")
	// Memory that recognises nothing: every reading resolves to no Place at all.
	blind := newLiveMemory()
	blind.blind = true
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, blind).
		WithActuator(w, w, true).
		WithForeground(leads)

	proof := justified()
	proof.At = newLiveClock().Now()

	_, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, &proof)
	if err == nil {
		t.Fatal("a walk proceeded on a carried proof against a screen it could not read")
	}
	if reason, _ := rehearse.RefusalOf(err); reason != rehearse.RefusalSourceUnrecognised {
		t.Fatalf("it refused with %q (%v); an unreadable screen must reach the ordinary "+
			"establish and be refused there", reason, err)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) were emitted anyway", w.sent())
	}
}

// A SINGLE DISAGREEING FRAME SENDS MARCO TO THE FULL LOOK — IT DOES NOT REFUSE.
//
// # The mistake this shape avoids
//
// The shortened confirmation reads the screen ONCE, with no settle. `establish` reads it six
// times and waits for it to stop moving, and it does that because one frame can catch a screen
// mid-transition: a repaint, a control that has not finished appearing, an animation. Treating a
// single disagreeing frame as proof that Marco has moved would turn a perfectly correct walk into
// an intermittent refusal — worse than the redundancy the whole roadmap set out to remove, and
// invisible until it happened to somebody.
//
// So every way of not agreeing means the same thing: ask the authoritative question. Here the
// first reading shows another screen and every reading after it shows the right one. The walk
// must complete.
//
// Deleting any arm of confirmCarried must fail one half of this: without them, a disagreeing or
// unreadable first frame is returned as the answer and the source guard refuses on it.
func TestADisagreeingFrameSendsMarcoToTheFullLook(t *testing.T) {
	// misleadingFirstFrame runs a walk whose very first reading shows `first` and whose every
	// later reading shows the truth.
	misleadingFirstFrame := func(t *testing.T, first observe.Sample) rehearse.RehearsalResult {
		t.Helper()
		w := newWorld("a", "b")
		seen := 0
		w.sample = func(screen string, after int) (observe.Sample, error) {
			seen++
			if seen == 1 {
				return first, nil
			}
			return liveSample(screen), nil
		}
		j := livePlan()
		g := liveGrant(t, j)
		l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
			WithActuator(w, w, true).
			WithForeground(leads)

		proof := justified()
		proof.At = newLiveClock().Now()

		res, err := l.Perform(context.Background(), g, j,
			windowref.Selector{Application: "testgame"}, 1, &proof)
		if err != nil {
			t.Fatalf("one misleading frame turned a correct walk into a refusal: %v.\n"+
				"The shortened confirmation has no settle, so a single frame can catch "+
				"a transition. Disagreement means ASK PROPERLY, never refuse.", err)
		}
		return res
	}

	t.Run("a frame showing another screen", func(t *testing.T) {
		res := misleadingFirstFrame(t, liveSample("c"))
		if !res.Completed() {
			t.Fatalf("terminal = %q", res.Terminal)
		}
		if res.Source != "subj_a" {
			t.Fatalf("the walk started from %q", res.Source)
		}
	})

	t.Run("a frame nobody can read", func(t *testing.T) {
		// An empty screen: no regions, no terms, nothing to resolve.
		res := misleadingFirstFrame(t, liveSample(""))
		if !res.Completed() {
			t.Fatalf("terminal = %q", res.Terminal)
		}
		if res.Source != "subj_a" {
			t.Fatalf("the walk started from %q", res.Source)
		}
	})
}

// EVIDENCE IS NOT AUTHORITY, AND HOLDING A PROOF DOES NOT LET A WALK BEGIN.
//
// The tempting shape this refuses is one line long:
//
//	if sourceEvidence.Valid { performWithoutAuthority() }
//
// Carrying proof forward changes how Marco knows WHERE IT IS. It changes nothing about whether
// Marco MAY ACT, and those are different questions with different owners: a proof is a reading of
// the world, and a grant is a bounded permission somebody issued. The shortcut would be easy to
// write precisely because valid evidence feels like enough.
//
// Deleting the grant check must fail this — and it must fail with the same refusal a walk holding
// no proof at all gets, because nothing about the proof is supposed to matter here.
func TestAProofIsNotPermissionToAct(t *testing.T) {
	w := newWorld("a", "b")
	j := livePlan()
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	proof := justified()
	proof.At = newLiveClock().Now()

	// PREMISE: this proof is good. It justifies acting on subj_a right now.
	if !proof.Justifies(newLiveClock().Now(), "testgame", "subj_a", leads) {
		t.Fatal("the proof this test is about cannot justify anything")
	}

	if _, err := l.Perform(context.Background(), nil, j,
		windowref.Selector{Application: "testgame"}, 1, &proof); err == nil {
		t.Fatal("a walk with perfectly good evidence and NO GRANT went ahead. Knowing " +
			"where you are is not being allowed to act.")
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) were emitted without any authority", w.sent())
	}
}

// AND A STOP REACHES A WALK WHILE IT IS CHECKING ITS PROOF.
//
// Every wait and every reading this roadmap added has to remain cancellable. The shortened
// confirmation reads the screen, and a reading is a place a walk can sit; a `stop` arriving there
// must end the walk rather than be noticed after the input has gone out.
//
// It must also not be swallowed into the fallback. A confirmation that fails for cancellation
// falls through to `establish` like any other failure — and `establish` has to refuse for the same
// reason rather than start a fresh six-reading look on a walk somebody has already stopped.
func TestStoppingReachesAWalkCheckingItsProof(t *testing.T) {
	w := newWorld("a", "b")
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	proof := justified()
	proof.At = newLiveClock().Now()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := l.Perform(ctx, g, j, windowref.Selector{Application: "testgame"}, 1, &proof)
	if err == nil {
		t.Fatal("a stopped walk went ahead because it was holding a proof")
	}
	if reason, _ := rehearse.RefusalOf(err); reason != rehearse.RefusalCancelled {
		t.Fatalf("it refused with %q (%v), want cancelled: a stop arriving during the "+
			"confirmation must end the walk, not be absorbed into the fallback",
			reason, err)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) were emitted after the stop", w.sent())
	}
}

// A REUSED PROOF DOES NOT SPEND A PERMISSION ON A WINDOW THAT HAS FALLEN BEHIND.
//
// # The gate sits before the claim, and carrying a proof must not move it
//
// The foreground check runs BEFORE `BeginAttempt` on purpose: a window that is not in front is a
// reason to WAIT, and waiting costs nothing only while the permission is unspent. The person
// answered yes in some other window; the watched one comes forward the moment they click back
// into it, and Marco can simply try again.
//
// Skipping that check for a walk holding a valid proof looks harmless, because the identical
// check runs again inside the step loop and refuses with the same word before any input goes out.
// It is not harmless. By then `BeginAttempt` has run, and `RehearsalGrant.BeginAttempt` sets
// GrantConsumed — which `Attempt.Cancel` does not undo, because one authorization permits one
// attempt. The refusal reads the same and the person has to be asked again.
//
// The race is real rather than theoretical: `Justifies` asks about the foreground at one instant
// and the walk begins at a slightly later one, which is precisely why the check is repeated
// instead of inferred.
//
// Deleting the foreground gate for a reused proof must fail this.
func TestAReusedProofDoesNotSpendAPermissionOnAWindowBehind(t *testing.T) {
	w := newWorld("a", "b")
	j := livePlan()
	g := liveGrant(t, j)

	// In front when the proof is justified, behind by the time the walk begins. One call
	// each, in that order.
	asked := 0
	slipping := func(ref windowref.Ref) bool {
		asked++
		return asked == 1
	}
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(slipping)

	proof := justified()
	proof.At = newLiveClock().Now()

	// PREMISE: the grant is unspent going in.
	if !g.Active() {
		t.Fatal("the grant is already spent")
	}

	_, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, &proof)
	if err == nil {
		t.Fatal("a walk went ahead against a window that had fallen behind")
	}
	if reason, _ := rehearse.RefusalOf(err); reason != rehearse.RefusalWindowBehind {
		t.Fatalf("it refused with %q (%v), want window_not_in_front", reason, err)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) were emitted", w.sent())
	}
	if !g.Active() {
		t.Fatal("the permission was SPENT refusing. One grant permits one attempt and " +
			"Attempt.Cancel does not give it back, so the person now has to be asked " +
			"again — for a window that had only to be clicked back into.")
	}
}

// A PROOF ABOUT ONE WINDOW OF A SHARED PROCESS SAYS NOTHING ABOUT ANOTHER.
//
// # The ambiguity this repository has already fixed once
//
// Settings, XBOX and Realtek Audio Console are all `applicationframehost`. They share an
// executable name, a process image, and everything a selector matches on — which is how asking
// for Mouse settings once raised XBOX. ADR-078 fixed the activation half of that.
//
// Carried evidence would let it back in through a different door if application name were enough
// to validate a proof: a proof taken on Settings, checked against "is applicationframehost in
// front", would be waved through while XBOX led the desktop.
//
// Two things stop it, and the test asserts both because either alone would be a partial claim.
// `Justifies` asks whether THAT REFERENCE still leads — a handle, not a name. And when it does
// pass, the shortened confirmation reads the screen and compares the Place, so a second window
// showing something else is caught by looking rather than by identity.
func TestAProofDoesNotCrossWindowsOfOneProcess(t *testing.T) {
	// Settings and XBOX: one application name, one process image, two windows.
	settings := windowref.Ref{ID: "hwnd:100", Handle: 100, ProcessID: 7,
		Application: "applicationframehost", Generation: 1}
	xbox := windowref.Ref{ID: "hwnd:200", Handle: 200, ProcessID: 7,
		Application: "applicationframehost", Generation: 1}

	proof := rehearse.StageEvidence{
		Ref: settings, Subject: "subj_a", At: evidenceNow.Add(-50 * time.Millisecond),
		From: rehearse.EvidenceVerifiedOutcome,
	}
	// The desktop's honest answer: XBOX leads, Settings does not.
	leading := func(ref windowref.Ref) bool { return ref.ID == xbox.ID }

	// PREMISE: a name-based check would pass this. Both windows answer to the same
	// application, so nothing about the application can tell them apart.
	if proof.Ref.Application != xbox.Application {
		t.Fatal("the premise of this test is two windows sharing one application name")
	}
	if proof.Justifies(evidenceNow, "applicationframehost", "subj_a", leading) {
		t.Fatal("a proof taken on Settings justified acting while XBOX led the desktop. " +
			"They share an executable name, so only the REFERENCE can tell them " +
			"apart — this is ADR-078's ambiguity arriving through the evidence.")
	}
	// AND THE CONTROL: the same proof, with Settings back in front, is usable. Without this
	// the assertion above would pass for a rule that refuses every shared-process window.
	back := func(ref windowref.Ref) bool { return ref.ID == settings.ID }
	if !proof.Justifies(evidenceNow, "applicationframehost", "subj_a", back) {
		t.Fatal("the proof is unusable even on the window it was taken on")
	}
}

// A WINDOW MARCO CANNOT READ REFUSES FOR ITS OWN REASON, AND EMITS NOTHING.
//
// # The refusal that sent three diagnoses to the wrong place
//
// The walker's source switch had three arms: nothing observable, ambiguous, and unrecognised. A
// reading that reached the window frame and not the page fell through to the last of them, so
// Marco said "I do not recognise the screen it is looking at" — about a page it had never read.
//
// Measured live on Windows Settings. It cost three runs and the better part of an hour, because
// the sentence sends a person to open a different page and the reading is what is broken.
//
// It refuses exactly as hard. Nothing here makes Marco more willing to act: what changed is what
// it says about why it will not.
func TestAnUnreadableWindowRefusesForItsOwnReason(t *testing.T) {
	w := newWorld("shell", "shell")
	w.sample = func(string, int) (observe.Sample, error) { return shellSample(), nil }
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	_, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, nil)
	if err == nil {
		t.Fatal("a walk went ahead against a window whose content was never read")
	}
	reason, _ := rehearse.RefusalOf(err)
	if reason == rehearse.RefusalSourceUnrecognised {
		t.Fatal("it refused with source_unrecognised — 'I don't recognise this screen' — " +
			"about a page nobody read. That sentence sends somebody to open a " +
			"different page, and the reading is what is broken.")
	}
	if reason != rehearse.RefusalSourceUnreadable {
		t.Fatalf("it refused with %q (%v), want source_unreadable", reason, err)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) were emitted", w.sent())
	}
}

// shellSample is a window frame with an empty content area — the live Settings failure, in the
// shape this package's fixtures speak.
//
// Chrome across the top, and one region covering most of the frame with nothing in it. Deliberately
// NOT built from liveSample: the point is a reading that has structure and no page.
func shellSample() observe.Sample {
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
	}
	chrome := []observe.Region{
		{X: 0.972, Y: 0.008, Width: 0.024, Height: 0.031},
		{X: 0.948, Y: 0.008, Width: 0.024, Height: 0.031},
		{X: 0.925, Y: 0.008, Width: 0.024, Height: 0.031},
		{X: 0.367, Y: 0.015, Width: 0.267, Height: 0.031},
		{X: 0.030, Y: 0.023, Width: 0.869, Height: 0.015},
		{X: 0.012, Y: 0.062, Width: 0.031, Height: 0.057},
		{X: 0.050, Y: 0.074, Width: 0.081, Height: 0.018},
		{X: 0.050, Y: 0.093, Width: 0.081, Height: 0.015},
	}
	for _, r := range chrome {
		sh.Regions = append(sh.Regions, observe.ShadowRegion{
			Role: "button", Confidence: 0.6, Region: r})
	}
	// THE PAGE THAT NEVER ARRIVED, reported twice exactly as the provider reported it.
	for i := 0; i < 2; i++ {
		sh.Regions = append(sh.Regions, observe.ShadowRegion{
			Role: "pane", Confidence: 0.6,
			Region: observe.Region{X: 0.173, Y: 0.109, Width: 0.823, Height: 0.884}})
	}
	sh.Detections = len(sh.Regions)
	sh.Roles = map[string]int{}
	for _, r := range sh.Regions {
		sh.Roles[r.Role]++
	}
	sh.Semantic = observe.SemanticEvidence{Observed: true,
		Terms: []observe.InterfaceTerm{observe.TermSettings}}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}
}

// AND IT DOES NOT NAME A PROVIDER.
//
// # Why the sentence matters as much as the word
//
// `source_unreadable` is provider-neutral by construction — nothing in `observe.Reach` knows what
// UIA is, and an OCR or fused observer producing the same geometry produces the same verdict.
// The prose beside it is where that would leak, because the person diagnosing this HAS a
// provider-shaped explanation in their head and it is tempting to write it down.
//
// A refusal that says "the UIA tree returned only ApplicationFrameHost chrome" is a true sentence
// that makes the semantic result Windows-only in every reader's mind, and it will be wrong the
// first time OCR sees the same thing. The evidence belongs in diagnostics, where `Vacancy` keeps
// it; this is the sentence the outcome carries.
func TestAnUnreadableRefusalNamesNoProvider(t *testing.T) {
	w := newWorld("shell", "shell")
	w.sample = func(string, int) (observe.Sample, error) { return shellSample(), nil }
	j := livePlan()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).
		WithActuator(w, w, true).
		WithForeground(leads)

	_, err := l.Perform(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1, nil)
	if err == nil {
		t.Fatal("the walk went ahead")
	}
	said := strings.ToLower(err.Error())
	for _, provider := range []string{
		"uia", "accessibility", "applicationframehost", "uwp", "win32", "msaa", "ocr",
	} {
		if strings.Contains(said, provider) {
			t.Errorf("the refusal names %q: %q\nThe verdict is provider-neutral and the "+
				"sentence must be too, or the first observer that is not "+
				"Accessibility inherits a lie.", provider, err)
		}
	}
}
