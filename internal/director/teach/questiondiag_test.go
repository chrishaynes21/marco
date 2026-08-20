package teach

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Why there is no question, carried from the tail that knows to the person who is stuck.
//
// # Three silences, one appearance
//
// Tail.Question returning false covers completely different situations: the evidence earned no
// question, the question was asked and answered, or the single-question interruption budget went
// to a different route. The coordinator cannot tell them apart and used to record nothing, so all
// three showed the same phase and the same sentence — "I think I got it. Want me to try?" — with
// no way forward and no explanation on any surface. Found live, twice, in consecutive runs.

// A tail that can say why is asked, and its answer is kept.
//
// Deleting the QuestionDiagnoser probe in awaitGrant must fail this.
func TestNoQuestionSaysWhyThereIsNoQuestion(t *testing.T) {
	c := New("open mouse settings", &noPasses{}, nil, DefaultBounds())
	c.tail = &silentTail{why: "another_question_open"}
	c.s.Phase = ReadyToRehearse
	c.s.Route = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

	c.awaitGrant()

	joined := strings.Join(c.s.Diagnostics, "\n")
	if !strings.Contains(joined, "another_question_open") {
		t.Fatalf("nothing was recorded about why no question exists:\n%s\nThe tail knew "+
			"and was never asked, so every surface shows a stage that will not move "+
			"and a sentence that will not change.", joined)
	}

	// AND ONCE. awaitGrant runs on every Advance; fifty copies of one line is a diagnostic
	// nobody reads, and it would bury the ones that matter.
	before := len(c.s.Diagnostics)
	c.awaitGrant()
	c.awaitGrant()
	if len(c.s.Diagnostics) != before {
		t.Errorf("the same reason was recorded %d more time(s)",
			len(c.s.Diagnostics)-before)
	}
}

// A tail with nothing to say changes nothing.
//
// Optional means optional: a Tail that cannot know keeps exactly the behaviour it had.
func TestATailWithNothingToSayIsHarmless(t *testing.T) {
	c := New("open mouse settings", &noPasses{}, nil, DefaultBounds())
	c.tail = &silentTail{}
	c.s.Phase = ReadyToRehearse
	c.s.Route = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

	c.awaitGrant()
	for _, d := range c.s.Diagnostics {
		if strings.Contains(d, "no rehearsal question") {
			t.Errorf("a tail with nothing to say produced %q", d)
		}
	}
}

// A tail that HAS a question is not asked why it has none.
func TestAQuestionThatExistsIsNotExplainedAway(t *testing.T) {
	c := New("open mouse settings", &noPasses{}, nil, DefaultBounds())
	tail := &silentTail{why: "another_question_open", has: true}
	c.tail = tail
	c.s.Phase = ReadyToRehearse
	c.s.Route = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

	c.awaitGrant()
	if c.s.Question == nil {
		t.Fatal("the open question did not reach the session")
	}
	for _, d := range c.s.Diagnostics {
		if strings.Contains(d, "no rehearsal question") {
			t.Errorf("a session WITH a question is told there is none: %q", d)
		}
	}
}

// ── stubs ─────────────────────────────────────────────────────────────────────

// noPasses watches nothing. These tests drive one phase directly and never observe.
type noPasses struct{}

func (noPasses) Observe(context.Context, time.Duration) (observesession.Result, error) {
	return observesession.Result{}, nil
}
func (noPasses) Finish()                            {}
func (noPasses) AwaitSubject(context.Context) error { return nil }

// silentTail has no question, and says why when asked.
type silentTail struct {
	why string
	has bool
}

func (s *silentTail) Question(observe.RelationshipRef, observe.AskKind) (Question, bool) {
	if s.has {
		return Question{ID: "q_1", SessionID: "observe_1"}, true
	}
	return Question{}, false
}
func (s *silentTail) Granted(observe.RelationshipRef) bool { return false }
func (s *silentTail) QuestionRefusal(observe.RelationshipRef, observe.AskKind) string {
	return s.why
}
func (s *silentTail) Rehearse(context.Context) (Attempt, error) { return Attempt{}, nil }
func (s *silentTail) Lowering(observe.RelationshipRef) (Readiness, error) {
	return Readiness{}, nil
}
func (s *silentTail) Save(observe.RelationshipRef, string, string) (Saved, error) {
	return Saved{}, nil
}

// "Nothing changed" says WHICH window it was watching.
//
// # The live failure
//
// A person named a behaviour, pressed Start, walked to Settings and did the whole thing. Marco
// said "I didn't see anything change, so there's nothing for me to learn." It had fixed on File
// Explorer — crossed on the way — and watched that for the entire pass.
//
// The sentence was true and unusable. It reads as "your demonstration was not good enough" when
// the fact was "Marco was pointed at the wrong window", and those need opposite responses from
// the person. Marco knew which window the whole time.
func TestNothingChangedSaysWhichWindowItWatched(t *testing.T) {
	s := Session{
		Name: "open mouse settings", Application: "explorer",
		Phase: Refused, Refusal: NothingChanged,
	}
	said := s.Say()
	if !strings.Contains(said, "explorer") {
		t.Fatalf("Marco says %q without naming the window it watched.\nThe person reads it "+
			"as a criticism of their demonstration and shows it again, into the same "+
			"wrong window.", said)
	}
	// And it still says what happened, in plain words.
	if !strings.Contains(strings.ToLower(said), "didn't see anything change") {
		t.Errorf("the reading no longer says what went wrong: %q", said)
	}
}

// With no application known, it falls back rather than naming nothing.
//
// "I didn't see anything change in " is worse than the original sentence.
func TestNothingChangedWithNoApplicationStillReads(t *testing.T) {
	s := Session{Phase: Refused, Refusal: NothingChanged}
	said := s.Say()
	if strings.Contains(said, " in ") || strings.TrimSpace(said) == "" {
		t.Errorf("the reading is malformed with no application: %q", said)
	}
}

// Other refusals are not decorated with a window name.
//
// They are about what was SEEN, and naming the window there is noise. A rule that applied to
// every refusal would make the one that needs it indistinguishable.
func TestOtherRefusalsDoNotNameTheWindow(t *testing.T) {
	for _, r := range []Refusal{NoObservation, DestinationNotRecognised, SeveralRoutes} {
		s := Session{Application: "explorer", Phase: Refused, Refusal: r}
		if strings.Contains(s.Say(), "explorer") {
			t.Errorf("%q names the window: %q", r, s.Say())
		}
	}
}

// A repeated wait is recorded once, not once per cycle.
//
// # The live failure
//
// A patient rehearsal re-attempts on every Advance and refuses for the same reason each time.
// The panel showed `waiting for the start: window_not_in_front` ten times in a row, and the lines
// that explained the run — the route, the grant, and the sentence saying what the attempt actually
// refused — were pushed out of sight behind them.
//
// The reason has to be there. Ten copies of it is not ten times as useful.
func TestARepeatedWaitIsRecordedOnce(t *testing.T) {
	c := New("open mouse settings", &noPasses{}, nil, DefaultBounds())
	c.tail = &refusingTail{refusal: "window_not_in_front"}
	c.s.Phase = Rehearsing
	c.s.Route = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

	for range 10 {
		c.s.Phase = Rehearsing
		c.rehearse(context.Background())
	}

	n := 0
	for _, d := range c.s.Diagnostics {
		if strings.Contains(d, "window_not_in_front") {
			n++
		}
	}
	if n == 0 {
		t.Fatal("the wait was never recorded, so nothing says why the rehearsal is not firing")
	}
	if n > 1 {
		t.Errorf("the same wait was recorded %d times.\nIt buries the lines that explain "+
			"the run — the route, the grant, and what the attempt refused — behind "+
			"copies of one sentence.", n)
	}
	if c.s.Phase != WaitingForStart {
		t.Errorf("the session is in %q, want it patiently waiting", c.s.Phase)
	}
}

// A DIFFERENT reason is still recorded.
//
// Deduplication that swallowed a change would be worse than the repetition: "not in front" giving
// way to "source unobservable" is the whole story of a run, and losing it leaves one stale line
// describing a situation that has moved on.
func TestAChangedWaitReasonIsStillRecorded(t *testing.T) {
	c := New("open mouse settings", &noPasses{}, nil, DefaultBounds())
	tail := &refusingTail{refusal: "window_not_in_front"}
	c.tail = tail
	c.s.Route = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

	for range 3 {
		c.s.Phase = Rehearsing
		c.rehearse(context.Background())
	}
	tail.refusal = "source_unobservable"
	for range 3 {
		c.s.Phase = Rehearsing
		c.rehearse(context.Background())
	}

	joined := strings.Join(c.s.Diagnostics, "\n")
	if !strings.Contains(joined, "window_not_in_front") {
		t.Error("the first reason was lost")
	}
	if !strings.Contains(joined, "source_unobservable") {
		t.Error("the reason CHANGED and the change was swallowed. One stale line then " +
			"describes a situation that has moved on.")
	}
}

// refusingTail always declines to act, patiently, for a reason that can be changed.
type refusingTail struct {
	silentTail
	refusal string
}

func (r *refusingTail) Rehearse(context.Context) (Attempt, error) {
	return Attempt{Attempted: false, Refusal: r.refusal}, nil
}
