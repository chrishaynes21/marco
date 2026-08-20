package teach_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"

	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// An authorised rehearsal waits for its start instead of spending itself on a bad moment.
//
// # What went wrong live
//
// A person demonstrated a route, was asked "want me to try?", said yes — and Marco declined to act
// with `source_unobservable`, because in the seconds between the answer and the attempt the
// Settings window had stopped being the one in front. The permission was gone and the person had
// done nothing wrong.
//
// The grant was never actually claimed: `BeginAttempt` compares scope before it spends, and that
// refusal is raised earlier still. So the authority survived and only the coordinator had given
// up. The demonstration capture has always behaved correctly here — armed, and patient until the
// user is standing on the start — and this makes the rehearsal match it.
//
// The alternative was to instruct the person to be on the right screen at a second they could not
// predict, which is test scaffolding wearing a product's clothes.

// waitingTail is a tail whose rehearsal reports the world, then eventually succeeds.
type waitingTail struct {
	*stubTail
	// refusals is what Rehearse reports, in order; the zero value after them is a success.
	refusals []string
	calls    int
}

func (w *waitingTail) Rehearse(context.Context) (teach.Attempt, error) {
	i := w.calls
	w.calls++
	if i < len(w.refusals) {
		return teach.Attempt{Attempted: false, Refusal: w.refusals[i]}, nil
	}
	w.stubTail.rehearsals++
	return teach.Attempt{Attempted: true, Completed: true, Terminal: "arrived"}, nil
}

// patient drives a granted session and returns where it got to, plus the tail.
func patient(t *testing.T, refusals []string, steps int) (teach.Session, *waitingTail) {
	t.Helper()
	inner := newTail()
	inner.granted = true
	w := &waitingTail{stubTail: inner, refusals: refusals}

	c, _, _ := teachToDemonstration(t, goodCandidate(), realFirstAssessment())
	c.WithTail(w).WithPlayName("Downloads", "Open")

	ctx := context.Background()
	s := c.Advance(ctx)
	for i := 0; i < steps && !s.Phase.Settled(); i++ {
		if s.Phase == teach.Naming {
			inner.name()
		}
		s = c.Advance(ctx)
	}
	return s, w
}

// THE correction: the world not being ready is not a refusal.
func TestAnUnobservableStartMakesMarcoWaitRatherThanGiveUp(t *testing.T) {
	s, w := patient(t, []string{"source_unobservable"}, 1)

	if s.Phase == teach.Refused {
		t.Fatalf("a rehearsal was refused because Marco could not see the screen at that "+
			"moment (%s).\nThe grant was never claimed, so the permission is intact and the "+
			"only thing lost is the person's answer.", s.Refusal)
	}
	if s.Phase != teach.WaitingForStart {
		t.Errorf("phase is %q, want %q", s.Phase, teach.WaitingForStart)
	}
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran; nothing may be emitted before the start is recognised",
			w.stubTail.rehearsals)
	}
	// And it says so in words a person can live with.
	if say := strings.ToLower(s.Say()); !strings.Contains(say, "when we're back there") {
		t.Errorf("Marco says %q; it should say it will try when we are back there", s.Say())
	}
}

// Standing somewhere else Marco DOES recognise waits too, and emits nothing.
func TestBeingOnTheWrongRecognisedPlaceWaits(t *testing.T) {
	s, w := patient(t, []string{"source_unrecognised"}, 1)
	if s.Phase != teach.WaitingForStart {
		t.Errorf("phase is %q (%s), want %q", s.Phase, s.Refusal, teach.WaitingForStart)
	}
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran from the wrong place", w.stubTail.rehearsals)
	}
}

// Arriving at the start later runs the rehearsal — EXACTLY once.
func TestArrivingAtTheStartLaterRunsItExactlyOnce(t *testing.T) {
	// Three moments where the world was not ready, then it is.
	s, w := patient(t, []string{
		"source_unobservable", "source_unrecognised", "target_lost",
	}, 12)

	if w.stubTail.rehearsals != 1 {
		t.Fatalf("%d rehearsal(s) ran, want exactly 1.\nOne authorisation permits one "+
			"attempt however long it waited for the world.", w.stubTail.rehearsals)
	}
	if s.Phase == teach.WaitingForStart {
		t.Error("the start became available and Marco is still waiting")
	}
}

// Wandering through unrelated places does not consume the grant.
func TestVisitingOtherPlacesDoesNotConsumeTheGrant(t *testing.T) {
	_, w := patient(t, []string{
		"source_unrecognised", "source_unrecognised", "source_unrecognised",
		"source_unrecognised", "source_unrecognised",
	}, 5)
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran while the user was elsewhere", w.stubTail.rehearsals)
	}
	if w.calls < 5 {
		t.Errorf("the coordinator stopped asking after %d attempt(s); a patient grant keeps "+
			"looking", w.calls)
	}
}

// A start that appears, goes, and settles produces no premature input.
func TestAFlickeringStartEmitsNothingUntilItSettles(t *testing.T) {
	s, w := patient(t, []string{
		"source_ambiguous", "target_moved", "source_unobservable",
	}, 12)
	if w.stubTail.rehearsals != 1 {
		t.Errorf("%d rehearsal(s); want one, after the screen settled", w.stubTail.rehearsals)
	}
	if s.Phase == teach.Refused {
		t.Errorf("a flickering start refused the whole attempt: %s", s.Refusal)
	}
}

// A reason waiting cannot fix still refuses. The patience is not blanket.
func TestARefusalWaitingCannotFixStillRefuses(t *testing.T) {
	for _, refusal := range []string{
		"no_actuator", "grant_spent", "grant_revoked", "grant_expired",
		"application_mismatch", "relationship_mismatch", "evidence_mismatch",
		// NOT source_mismatch: that is a person standing two clicks from the start,
		// and waiting is exactly what it wants. See notReadyYet.
	} {
		t.Run(refusal, func(t *testing.T) {
			s, w := patient(t, []string{refusal}, 3)
			if s.Phase != teach.Refused {
				t.Errorf("phase is %q for %q; waiting will never change this answer, and "+
					"retrying is a loop asking the same forbidden question", s.Phase, refusal)
			}
			if w.stubTail.rehearsals != 0 {
				t.Errorf("%d rehearsal(s) ran anyway", w.stubTail.rehearsals)
			}
		})
	}
}

// Cancelling while waiting never runs it.
func TestCancellingWhileWaitingNeverRunsTheRehearsal(t *testing.T) {
	inner := newTail()
	inner.granted = true
	w := &waitingTail{stubTail: inner, refusals: []string{"source_unobservable"}}

	c, _, _ := teachToDemonstration(t, goodCandidate(), realFirstAssessment())
	c.WithTail(w).WithPlayName("Downloads", "Open")

	ctx := context.Background()
	c.Advance(ctx) // the grant is picked up
	if s := c.Advance(ctx); s.Phase != teach.WaitingForStart {
		t.Fatalf("phase is %q, want to be waiting", s.Phase)
	}
	c.Cancel()
	for range 5 {
		c.Advance(ctx)
	}
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran after the session was cancelled", w.stubTail.rehearsals)
	}
	if len(inner.saves) != 0 {
		t.Error("a cancelled session saved a play")
	}
}

// Waiting emits nothing, so there is nothing for Marco to learn from itself.
//
// The invariant that must survive every change to this path: a rehearsal is Marco's own input and
// is never evidence. Waiting makes the window between permission and action longer, which is
// exactly when an accidental observation pass would be easiest to add.
func TestWaitingNeverWatchesAnything(t *testing.T) {
	inner := newTail()
	inner.granted = true
	w := &waitingTail{stubTail: inner, refusals: []string{
		"source_unobservable", "source_unobservable", "source_unobservable",
	}}

	c, _, passes := teachToDemonstration(t, goodCandidate(), realFirstAssessment())
	c.WithTail(w).WithPlayName("Downloads", "Open")

	ctx := context.Background()
	// Reach the waiting phase first; the passes before it are the ordinary ones.
	c.Advance(ctx)
	c.Advance(ctx)
	before := passes.calls
	for range 4 {
		c.Advance(ctx)
	}
	if passes.calls != before {
		t.Errorf("%d observation pass(es) ran while waiting to rehearse (was %d).\nA pass "+
			"here would watch Marco's own input and could feed it back as evidence.",
			passes.calls-before, before)
	}
}

// Being on a screen Marco KNOWS, that is not the start, waits like any other wrong place.
//
// `source_unrecognised` and `source_mismatch` are the same situation reached through different
// checks — the person is somewhere else — and only one of them used to wait. Live, a granted
// rehearsal of a route beginning on the Settings category page refused outright because the
// person happened to be on the home page at that second, with the grant still unspent.
func TestBeingOnAKnownButWrongScreenWaits(t *testing.T) {
	s, w := patient(t, []string{"source_mismatch"}, 1)
	if s.Phase != teach.WaitingForStart {
		t.Errorf("phase is %q (%s), want %q.\nScope is compared before the grant is claimed, "+
			"so nothing was spent and the person is simply not there yet.",
			s.Phase, s.Refusal, teach.WaitingForStart)
	}
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran from the wrong screen", w.stubTail.rehearsals)
	}
}

// A grant outlives the walk to the start.
//
// Patience is only patience if the permission survives it. Live, a granted rehearsal waited
// correctly through four wrong-place checks and then expired: the wall-clock guard was thirty
// seconds, chosen when a grant was spent the instant it was given, and it silently became the
// bound on how long Marco may wait for a person to cross a room.
func TestAGrantOutlivesTheWalkToTheStart(t *testing.T) {
	if got := observe.DefaultRehearsalDuration; got < 5*time.Minute {
		t.Errorf("a rehearsal grant lasts %s.\nIt is read only by BeginAttempt, so what it "+
			"bounds is how long after saying yes an attempt may begin — which, now that the "+
			"rehearsal waits for its start, is how long Marco may wait for a person to walk "+
			"there. Nobody does that in %s.", got, got)
	}
}

// A window that is not in front waits like any other not-yet-ready world.
//
// The live shape of this: a person answers "yes, try it" in a terminal, the terminal holds
// the foreground, and the rehearsal fires its keys into the terminal. The runner now refuses
// before claiming the grant — input has no address, so a window that is behind cannot be
// acted on — and the only honest response up here is the patient one: the window comes
// forward the moment the person clicks back into it.
func TestAWindowBehindTheForegroundWaits(t *testing.T) {
	s, w := patient(t, []string{"window_not_in_front"}, 1)
	if s.Phase != teach.WaitingForStart {
		t.Errorf("phase is %q (%s), want %q.\nNothing was spent — the person simply has not "+
			"brought the window forward yet.", s.Phase, s.Refusal, teach.WaitingForStart)
	}
	if w.stubTail.rehearsals != 0 {
		t.Errorf("%d rehearsal(s) ran while the window was behind", w.stubTail.rehearsals)
	}
}
