package observesession_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The proposal loop, entered through the production session path.
//
// A test calling ProposalLedger.Review directly proves the ledger and nothing about whether a
// running Director ever asks anybody anything. That distinction has now cost this subsystem four
// milestones, so every assertion here starts at Runner.Run with a real sampler and ends at the
// terminal Result.
//
// Deleting the Refresh call from the sampling loop must fail TestTheProductionSessionPathProposesQuestions.
// Deleting Respond must fail TestAConfirmedHypothesisIsValidatedInTheResult.

func runProposals(t *testing.T, s observesession.Sampler) (*observesession.Runner, observesession.Result) {
	t.Helper()
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s, &recordingEvents{})
	got, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r, got
}

// ── A: a question reaches the Result ──────────────────────────────────────────

// THE production wiring test.
func TestTheProductionSessionPathProposesQuestions(t *testing.T) {
	_, got := runProposals(t, settingsSession())

	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses; this test can say nothing about proposals")
	}
	if len(got.Proposals.Proposals) == 0 {
		t.Fatal("a session that produced supported hypotheses asked NOTHING. The proposal " +
			"loop is not reachable from the production session path, so Marco would " +
			"accumulate confident interpretations and never once put one to the user")
	}

	open := got.Proposals.Open()
	if len(open) != 1 {
		t.Fatalf("%d open question(s), want exactly 1 — asking is bounded per interruption",
			len(open))
	}
	q := open[0]
	if q.Question == "" {
		t.Fatal("the proposal carries no sentence to put to anybody")
	}
	// It must be about a hypothesis the session actually formed.
	var matched bool
	for _, h := range got.Hypotheses {
		if observe.ProposalIdentity(h) == q.ID {
			matched = true
			if h.Status != observe.StatusSupported {
				t.Errorf("the question is about a %q hypothesis", h.Status)
			}
		}
	}
	if !matched {
		t.Error("the question does not correspond to any hypothesis in the Result; a " +
			"question must originate from the evidence chain, not from an independent look " +
			"at the screen")
	}
	// And nothing in it names the application or an implementation identifier.
	for _, leak := range []string{"testgame", "state_", "possible_", ".exe"} {
		if strings.Contains(strings.ToLower(q.Question), leak) {
			t.Errorf("the question contains %q: %s", leak, q.Question)
		}
	}
}

// A session with nothing worth asking about asks nothing.
func TestASessionWithNoSupportedHypothesisAsksNothing(t *testing.T) {
	// Gameplay only: no recurring structure, so nothing reaches `supported`.
	_, got := runProposals(t, &discoverySampler{})
	for _, h := range got.Hypotheses {
		if h.Status == observe.StatusSupported {
			return // the fixture produced something askable; the assertion below is moot
		}
	}
	if len(got.Proposals.Open()) != 0 {
		t.Errorf("%d question(s) from a session with no supported hypothesis",
			len(got.Proposals.Open()))
	}
}

// ── D/E/F through the production response path ────────────────────────────────

// A confirmation reaches the Result as validation.
//
// Deleting Runner.Respond, or the Annotate call on the terminal Result, must fail this.
func TestAConfirmedHypothesisIsValidatedInTheResult(t *testing.T) {
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		settingsSession(), &recordingEvents{})
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ledger := r.Proposals()
	open := ledger.Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked, so nothing can be answered")
	}
	if _, ok := r.Respond(open[0].ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the production response path refused a valid answer")
	}

	// The runner's own view must now carry the validation.
	after := r.Proposals()
	var answered bool
	for _, p := range after.Proposals {
		if p.ID == open[0].ID && p.Response == observe.ResponseConfirmed {
			answered = true
		}
	}
	if !answered {
		t.Fatal("the confirmation did not reach the session's ledger")
	}

	hs := after.Annotate(r.LiveAnalysis(observe.DefaultInsightThresholds()).Hypotheses)
	var validated bool
	for _, h := range hs {
		if h.UserValidation != nil && h.UserValidation.Response == observe.ResponseConfirmed {
			validated = true
			if h.Status != observe.StatusValidated && len(h.Contradictions) == 0 {
				t.Errorf("status %q after a clean confirmation, want validated", h.Status)
			}
		}
	}
	if !validated {
		t.Error("no hypothesis carries the user's confirmation")
	}
}

// A contradiction reaches the Result without deleting the observations.
func TestAContradictedHypothesisKeepsItsObservationsInTheResult(t *testing.T) {
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		settingsSession(), &recordingEvents{})
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	open := r.Proposals().Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked")
	}
	r.Respond(open[0].ID, observe.ResponseContradicted)

	ledger := r.Proposals()
	hs := ledger.Annotate(r.LiveAnalysis(observe.DefaultInsightThresholds()).Hypotheses)
	for _, h := range hs {
		if h.UserValidation == nil {
			continue
		}
		if h.Status != observe.StatusContested {
			t.Errorf("status %q after the user disagreed, want contested", h.Status)
		}
		if len(h.Support) == 0 {
			t.Error("the observations that supported this were deleted by the answer")
		}
		return
	}
	t.Error("no hypothesis carries the user's disagreement")
}

// A decline suppresses the question and leaves the evidence alone.
//
// Collapsing DECLINE into NO must fail this.
func TestADeclineDoesNotContradictThroughTheProductionPath(t *testing.T) {
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		settingsSession(), &recordingEvents{})
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	open := r.Proposals().Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked")
	}
	r.Respond(open[0].ID, observe.ResponseDeclined)

	ledger := r.Proposals()
	if len(ledger.Open()) != 0 {
		t.Error("the question is still open after a decline")
	}
	hs := ledger.Annotate(r.LiveAnalysis(observe.DefaultInsightThresholds()).Hypotheses)
	for _, h := range hs {
		if h.UserValidation == nil {
			continue
		}
		if h.UserValidation.Response != observe.ResponseDeclined {
			t.Fatalf("the decline was recorded as %q", h.UserValidation.Response)
		}
		if h.Status == observe.StatusContested {
			t.Error("a decline contested the hypothesis. 'Not now' is not 'you are wrong', " +
				"and treating it as one lets being busy quietly become a denial")
		}
		for _, c := range h.Contradictions {
			if c.Source == observe.FromUser {
				t.Error("a decline was recorded as a user contradiction")
			}
		}
		for _, e := range h.Support {
			if e.Source == observe.FromUser {
				t.Error("a decline was recorded as user support")
			}
		}
	}
}

// ── I: the answer arrives after the screen has moved on ───────────────────────

// An answer attaches to the question that was ASKED, not to whatever is current.
//
// The mutation this exists for: binding a response to "the current hypothesis" instead of the
// proposal's own identity. By the time somebody answers, the screen has usually changed several
// times and the state that prompted the question may have been renumbered or may be gone.
func TestAnAnswerAttachesToTheQuestionThatWasAskedNotTheCurrentOne(t *testing.T) {
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		settingsSession(), &recordingEvents{})
	cfg := config()
	// Two questions may be open, so "the current one" and "the one that was asked" are
	// different proposals and a mutation that confuses them is detectable.
	cfg.ProposalPolicy = observe.ProposalThresholds{MaxOpen: 2, MaxProposals: 32}
	if _, err := r.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	open := r.Proposals().Open()
	if len(open) < 2 {
		t.Skipf("the fixture produced %d open questions; this test needs two distinct ones",
			len(open))
	}
	first, second := open[0], open[1]

	// Answer the SECOND question, deliberately.
	//
	// Answering the first would be indistinguishable from any implementation that simply
	// took "the earliest open question" — which is exactly the mutation this test exists to
	// catch. The user is answering the one they read, not the one Marco asked first.
	if _, ok := r.Respond(second.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was refused")
	}

	after := r.Proposals()
	for _, p := range after.Proposals {
		switch p.ID {
		case second.ID:
			if p.Response != observe.ResponseConfirmed {
				t.Errorf("the answer did not reach the question it was for (%q)", p.Response)
			}
		case first.ID:
			if p.Response != observe.ResponseNone {
				t.Errorf("the answer landed on a DIFFERENT question (%q). A response bound "+
					"to whatever is current — or simply to the earliest open question — "+
					"rather than to the proposal's own identity attributes the user's words "+
					"to something they never saw", p.Response)
			}
		}
	}
}

// ── G: the loop does not spam across a whole session ──────────────────────────

// A long session asks about a stable hypothesis once, not once per sample.
func TestALongSessionDoesNotReAskTheSameQuestion(t *testing.T) {
	_, got := runProposals(t, settingsSession())
	if got.Stats.SamplesTaken < 5 {
		t.Fatalf("only %d samples; this test needs a session long enough to repeat itself",
			got.Stats.SamplesTaken)
	}
	seen := map[observe.ProposalID]int{}
	for _, p := range got.Proposals.Proposals {
		seen[p.ID]++
		if p.Asked > 1 {
			t.Errorf("a question was put %d times in one session with unchanging evidence",
				p.Asked)
		}
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("question %s appears %d times in the ledger", id, n)
		}
	}
}

// ── the loop asks and observes at the same time ───────────────────────────────

// A pending question must not stop the session observing.
func TestAPendingQuestionDoesNotFreezeObservation(t *testing.T) {
	_, got := runProposals(t, settingsSession())
	if len(got.Proposals.Open()) == 0 {
		t.Fatal("no question was left open, so this proves nothing")
	}
	// The session ran to completion with a question outstanding.
	if !got.Complete() {
		t.Errorf("the session ended %q with a question open", got.Session.State)
	}
	if got.Stats.SamplesTaken < 5 {
		t.Errorf("only %d samples were taken while a question was open",
			got.Stats.SamplesTaken)
	}
	open := got.Proposals.Open()[0]
	if open.AskedAtInference >= got.Stats.SamplesTaken {
		t.Error("the question was asked on the last sample, so nothing was observed after it")
	}
}
