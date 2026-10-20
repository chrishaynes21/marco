package demonstrated

// THE PRODUCTION COORDINATOR, DRIVEN ONE INTERACTION AT A TIME.
//
// # What this is and is not
//
// It is not a second executor. `director perform-workflow` has an observation loop — acquire the
// window, capture, propose regions, observe, build a `Sighting`, call `Runner.Advance` — and that
// loop is the only one. This drives the SAME call with the SAME runner, bounded to one
// interaction, which is the whole of what the bridge needed from Go:
//
//	Begin(step)                         the caller's order becomes the order performed
//	Advance(ctx, binding, look())       per look, until the interaction has an answer
//	verdict                             what the post-action evidence said
//
// The looks come from outside. In production that closure is the capture-and-observe body of
// `performworkflow.go`; in an acceptance it is a scripted sequence. Everything between the look
// and the press — grounding, the person's answer, the pointer measurement, the hover probe, the
// compile gate, the outcome window — is the production coordinator's and is not reimplemented
// here. That is deliberate: a supervisor that could decide anything would be a second place where
// safety lives.
//
// # Cancellation is checked at the top of every look
//
// Not once at the start. A stop raised while an interaction is in flight has to be seen before
// the next press, which is the last moment it could matter — see
// [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]. The coordinator checks
// its own stop state too; this one is the loop's, so a cancelled run stops consuming looks
// instead of spinning until its patience runs out.

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// Look is one observation, however the caller got it.
//
// Returning ok false means there is nothing more to look at — the window went away, the deadline
// passed, the scripted sequence ended — and the interaction ends unconfirmed rather than the
// loop pretending it saw something.
type Look func() (workflowrun.Sighting, bool)

// Supervised performs interactions through one production coordinator.
type Supervised struct {
	run  *workflowrun.Runner
	bind *workflowstore.SessionBinding
	look Look
	// runCtx is the RUN's context, which is the only thing that knows why a run ended.
	// See WithRunContext.
	runCtx context.Context
	// watch is what the caller does between looks: report the attempt, ask about a step
	// nobody has agreed to, pace the loop. Nil does nothing. See Watch.
	watch Watch
	// looks bounds how many observations ONE interaction may consume. A bound rather than
	// a deadline, because the caller owns the clock: `perform-workflow` has a wall-clock
	// deadline for the whole run, and an acceptance advances a fake clock.
	looks int
}

// NewSupervised wires a coordinator, its session binding and a source of looks.
func NewSupervised(run *workflowrun.Runner, bind *workflowstore.SessionBinding,
	look Look, looks int) *Supervised {

	if looks <= 0 {
		looks = 240
	}
	return &Supervised{run: run, bind: bind, look: look, looks: looks}
}

// Press begins one interaction and drives looks until it has an answer.
//
// The returned verdict is the coordinator's, never this function's: every one of them comes off
// an `Attempt` the production runner produced.
func (s *Supervised) Press(ctx context.Context, step int) (*workflowrun.Verification, string, error) {
	if s == nil || s.run == nil {
		return nil, "", fmt.Errorf("there is no run to press in")
	}
	if err := ctx.Err(); err != nil {
		// BEFORE Begin, so a run that ended between two sentences says so rather than
		// being refused for standing on the wrong step.
		return nil, "", s.whyEnded(ctx)
	}
	if err := s.run.Begin(step); err != nil {
		return nil, "", err
	}
	var last workflowrun.Attempt
	for i := 0; i < s.looks; i++ {
		if err := ctx.Err(); err != nil {
			// WHICH ENDED IT. A run that reached its time limit is not a run
			// somebody cancelled, and the report says so. See WhyItEnded.
			return nil, "", s.whyEnded(ctx)
		}
		if s.run.Stopped() {
			// STOPPED BY WHAT. `Halt` raises the coordinator's stop flag, and a
			// cancelled run and an EXPIRED one both reach it — so the flag alone
			// cannot say which, and answering `cancelled` for both says somebody
			// chose to end a run that used the time it was given. The context
			// knows; it is only when it does not that this is a bare stop.
			if why := s.whyEnded(ctx); why != nil {
				return nil, "", why
			}
			return nil, "", fmt.Errorf("%w: it was stopped",
				ErrCancelled)
		}
		at, ok := s.look()
		if !ok {
			break
		}
		last = s.run.Advance(ctx, s.bind, at)
		if s.watch != nil {
			// BETWEEN LOOKS, AND BEFORE THE OUTCOME IS READ. The step-agreement
			// question is asked on an attempt whose stage says nobody has
			// agreed yet, and the next look is what carries that answer in —
			// so it has to be asked before this iteration ends.
			if carryOn, why := s.watch(ctx, last); !carryOn {
				return last.Verified, last.Because, fmt.Errorf("%s", why)
			}
		}
		switch last.Outcome {
		case workflowrun.OutcomeVerified:
			// THE ONLY WAY TO ok. The attempt carries the verdict; this reads it
			// rather than inferring one from the outcome word.
			return last.Verified, last.Because, nil
		case workflowrun.OutcomeArrived:
			// The destination is established and the interaction is finished as far
			// as this walk is concerned. For a step whose demonstration recorded an
			// outcome this cannot happen before the verdict — ADR-242 — so reaching
			// here means the step had nothing to check against, and that is
			// `unasked`, reported as what it is.
			return last.Verified, last.Because, nil
		case workflowrun.OutcomeRefused, workflowrun.OutcomeFailed:
			// A REFUSAL CARRYING A VERDICT IS THE VERDICT.
			//
			// The coordinator stops a walk whose interaction was contradicted or
			// left unconfirmed, and the attempt it stops on is `refused` with the
			// verdict on it. Reporting that as a refusal to act would lose the one
			// fact a person needs — the press went out and the screen disagreed —
			// and call it "Marco would not press anything". The error is still
			// returned, because the play must take its `or?` arm either way.
			return last.Verified, last.Because,
				fmt.Errorf("%s", refusalSentence(last))
		case workflowrun.OutcomeStopped:
			// STOPPED BY WHAT, THOUGH.
			//
			// The coordinator reports `stopped` for its own stop flag, and
			// that flag is raised by `Halt` — which a cancelled run and an
			// EXPIRED one both reach. Answering `cancelled` for both said
			// somebody chose to end a run that had simply used the time it
			// was given, which is the one thing this distinction exists to
			// prevent. The context knows which; ask it.
			if why := s.whyEnded(ctx); why != nil {
				return last.Verified, last.Because,
					fmt.Errorf("%w (%s)", why, last.Because)
			}
			return last.Verified, last.Because,
				fmt.Errorf("%w: %s", ErrCancelled, last.Because)
		case workflowrun.OutcomeComplete:
			return last.Verified, last.Because,
				fmt.Errorf("there is no interaction %d to perform", step)
		}
		// ONE INVOCATION PERFORMS ONE INTERACTION, stated structurally rather than
		// left to the switch above covering every outcome that can advance a walk.
		//
		// If the coordinator is reporting about a different step, this call has
		// outlived the interaction it was asked for, and carrying on would ground,
		// authorise and press the NEXT one inside a sentence that did not ask for it.
		// Nothing reaches this today — the two advancing outcomes both return above —
		// and it is here because what it guards is the requirement itself.
		if last.Step != step {
			return last.Verified, last.Because,
				fmt.Errorf("interaction %d stopped being the one attempted "+
					"(the run is on %d) and this sentence will not "+
					"perform it", step, last.Step)
		}
	}
	// OUT OF LOOKS. Not a failure of the action and not a success: nobody ever got to
	// say, and that is returned as an error so the play takes its `or?` arm rather than
	// carrying on past an interaction nothing is known about.
	if last.Verified != nil {
		return last.Verified, last.Because, fmt.Errorf("%s", refusalSentence(last))
	}
	return nil, last.Because, fmt.Errorf("interaction %d ran out of observations "+
		"before anything could be decided about it: %s", step, last.Because)
}

// refusalSentence says what happened in the words the coordinator used, with the stage it
// reached, because "nothing happened" has several explanations and they call for different
// responses.
func refusalSentence(a workflowrun.Attempt) string {
	if a.Stage != "" {
		return fmt.Sprintf("%s at %s — %s", a.Outcome, a.Stage, a.Because)
	}
	return fmt.Sprintf("%s — %s", a.Outcome, a.Because)
}

// Supervisor performs ONE interaction under supervision and says what came of it.
//
// The seam the observation loop sits behind. In production it is `Supervised` above, driving the
// production coordinator over a live capture; in an acceptance it is the same `Supervised` over a
// scripted sequence of looks. What is never behind it is the decision: a Supervisor does not get
// to say an action was verified.
//
// A nil verification means nothing was decided, which is not the same as a verdict of
// `VerdictUnasked`. See `Ask.Decided`.
type Supervisor interface {
	Press(ctx context.Context, step int) (*workflowrun.Verification, string, error)
	// Halt stops the run, and it is why a Supervisor is an interface rather than a
	// function.
	//
	// # The defect this closes
	//
	// Cancelling the run cancels the RUNTIME, and `HostCall.Ctx` is the calling frame's
	// context rather than the caller's. There is a window between the two in which a
	// frame's context is not yet cancelled — and in that window the next interaction was
	// grounded and PUT TO THE PERSON. Measured: one run in roughly six authorised
	// interaction 1 after the cancel, with the account correctly recording the sentence
	// as cancelled afterwards. A prompt nobody should have seen is not made acceptable
	// by the run failing a moment later.
	//
	// So the stop reaches the COORDINATOR too, where `Begin` refuses outright and the
	// loop stops consuming looks. Which is
	// [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] again: the
	// last moment a stop could matter is before the next thing is asked of somebody.
	Halt()
}

// Halt stops the coordinator, so nothing else is grounded, asked about or pressed.
func (s *Supervised) Halt() {
	if s != nil && s.run != nil {
		s.run.Stop()
	}
}

// Watch is told about every attempt, and may end the interaction.
//
// # Why this exists
//
// Because driving looks is not the only thing the production loop does between them. It reports
// the attempt, and — the part that matters — it asks about a step NOBODY HAS AGREED TO:
//
//	A STEP NOBODY HAS AGREED TO IS A QUESTION, NOT A REFUSAL.
//
// That question is deliberately outside the coordinator, because agreeing that a step belongs in
// this run and permitting a press are two different sentences, and one prompt for both would let
// a person answer one and be taken to have answered the other. A play-driven run must ask it in
// the same place, so the loop's between-looks work is handed in rather than reimplemented.
//
// Returning false ends the interaction with `why` — which is what "nobody agreed" looks like. It
// is not an error in the action: nothing was refused by the machine, somebody declined.
type Watch func(ctx context.Context, a workflowrun.Attempt) (carryOn bool, why string)

// WithWatch hands over what the caller does between looks. Nil is the default and does nothing.
func (s *Supervised) WithWatch(w Watch) *Supervised {
	s.watch = w
	return s
}

// WithRunContext tells the supervisor the RUN's own context, so it can say why a run ended.
//
// # Why the frame's context cannot answer that
//
// `HostCall.Ctx` is the calling frame's, and the runtime cancels the frame tree when the run's
// context ends — with `context.Canceled`, whatever the reason upstream was. So a run that reached
// its `--seconds` limit arrives here as a cancellation, and the distinction this package takes
// care to preserve is destroyed one layer above it: the report said somebody chose to stop a run
// that had used the time it was given.
//
// The run's context still knows (`DeadlineExceeded`), so it is asked first. Optional, and without
// it the frame's answer stands — which is right for a caller that has no separate run context.
func (s *Supervised) WithRunContext(ctx context.Context) *Supervised {
	s.runCtx = ctx
	return s
}

// whyEnded is which ending to report: the run's reason where there is one, else the frame's.
func (s *Supervised) whyEnded(frame context.Context) error {
	if s.runCtx != nil {
		if why := WhyItEnded(s.runCtx.Err()); why != nil {
			return why
		}
	}
	return WhyItEnded(frame.Err())
}
