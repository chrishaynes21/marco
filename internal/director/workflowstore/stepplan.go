package workflowstore

// Knowing the whole plan, and trusting only the screen you are on.
//
// # The conflict this resolves
//
// [[Experiment-038-how-far-a-reloaded-workflow-gets]] found two contracts that cannot both hold:
//
//	Bind      every screen of the workflow must be established before planning
//	a binding is valid only in the observation generation it was established in
//
// Establishing the second screen expires the first, so at most one is ever live and `Bind` can
// only succeed on a one-screen workflow. Neither rule was wrong; they describe different
// assurances, and they were written against different models of how a run proceeds.
//
// The assurance moves to where an action is actually considered. A workflow is validated
// STRUCTURALLY when it loads — every step present, every label described, nothing missing — and
// its screens are grounded ONE AT A TIME, each against the observation Marco is in when it
// reaches that step.
//
// `Bind` is untouched and keeps its all-or-nothing meaning for the question it answers: could
// this whole workflow be established right now. It simply stops standing between a reloaded
// workflow and its first step.
//
// # What this is not
//
// Step-scoped planning is not step-scoped permission. A step that reaches `StepPlannable` has a
// point and an expectation and nothing else: no press has been authorised, nothing has been
// lowered by this package, and `workflow` still cannot reach the lowering path at all. The five
// operations stay five — confirmation, screen establishment, re-acquisition, authorisation,
// verification — and reaching one says nothing about the next.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// StepStage is how far one step got against the observation in front of it.
type StepStage string

const (
	// StepNotRepeatable: a wait. There is nothing to press and nothing to plan.
	StepNotRepeatable StepStage = "not repeatable"
	// StepScreenUnestablished: this step's saved screen is not established in THIS
	// observation. The commonest honest answer, and the one that must never be relaxed
	// by reaching back to an earlier generation.
	StepScreenUnestablished StepStage = "screen not established here"
	// StepNeedsClarification: the control's identity was never settled by the
	// demonstration and nobody has answered the question.
	StepNeedsClarification StepStage = "clarification unresolved"
	// StepControlUnresolved: the control could not be found on this look.
	StepControlUnresolved StepStage = "control not re-acquired"
	// StepUnconfirmed: nobody has agreed to this step in this session.
	StepUnconfirmed StepStage = "not confirmed in this session"
	// StepUnauthorised: the session carries no live execution authorisation.
	StepUnauthorised StepStage = "not authorised"
	// StepPlannable: a point and an expectation exist. NOT permission.
	StepPlannable StepStage = "plannable"
)

// Now is everything about the observation a step is being considered against.
//
// Passed in rather than discovered, because this package can see no screen: it holds a saved
// workflow and a session's answers, and every fact about the present belongs to the caller that
// is actually looking.
type Now struct {
	Screen     observe.VisualScreenID
	Epoch      uint64
	Generation uint64
	// Outcomes is what re-acquisition said about this look, from the caller's own
	// source-aware candidate set.
	Outcomes []observe.Reacquisition
	// Authorised says the session carries live execution authorisation. NEVER inferred
	// here: a planner that could grant its own permission is the thing every boundary in
	// this subsystem exists to prevent.
	Authorised bool
}

// StepReadiness is one step examined against one observation.
type StepReadiness struct {
	Step    int
	Stage   StepStage
	Because string
	// Action is set only at StepPlannable, and is a point and an expectation.
	Action *workflow.Action
	// Expect is the screen this step would have to arrive at for the action to have
	// worked. Set whenever the step is known, so a verification expectation can be built
	// without the step being plannable.
	Expect string
	// Found is where the control was re-acquired on this look, normalised to the window.
	//
	// Set as soon as the control is found, which is BEFORE confirmation and before
	// authorisation, and it is not permission to press it — `Action` is still the only
	// field that means a step is plannable.
	//
	// It exists because of what happens without it. Authorisation is the last refusal, so
	// at the moment somebody is asked for it the step is not plannable and `Action` is
	// nil; a caller building the question from `Action` alone would show them a press at
	// (0,0). Asking a person to permit a click without telling them where it lands is not
	// an explicit authorisation, and the alternative — the caller computing the centre
	// itself — would be a second statement of this arithmetic, which is this subsystem's
	// recorded failure mode.
	Found *observe.PointerAt
}

// Ready reports whether this step could be lowered.
func (s StepReadiness) Ready() bool { return s.Stage == StepPlannable }

// StepReadinessAt examines ONE step against the observation in front of it.
//
// The order of the refusals is the order a person would want them: where am I, is this control's
// identity even settled, can I see it, has anybody agreed, am I allowed. Each is reported with
// the exact thing that is missing rather than a single "cannot".
func (a Artifact) StepReadinessAt(at int, s *SessionBinding, now Now) StepReadiness {
	r := StepReadiness{Step: at}
	if at < 0 || at >= len(a.Steps) {
		r.Stage = StepScreenUnestablished
		r.Because = fmt.Sprintf("step %d is not in a workflow of %d", at, len(a.Steps))
		return r
	}
	st := a.Steps[at]
	r.Expect = st.To

	// ── 1. THIS STEP'S SCREEN, IN THIS OBSERVATION ──
	//
	// Only this step's. Nothing is asked about the screens the workflow visits later,
	// and nothing established earlier is reached back for.
	bound, ok := s.Established(st.From, now.Epoch, now.Generation)
	switch {
	case !ok:
		r.Stage = StepScreenUnestablished
		r.Because = fmt.Sprintf("%s is not established in this observation; it has to "+
			"be settled here, and an earlier answer about it is not evidence now",
			st.From)
		return r
	case bound.Screen != now.Screen:
		r.Stage = StepScreenUnestablished
		r.Because = fmt.Sprintf("%s is established as %s and the screen standing now "+
			"is %s", st.From, bound.Screen, now.Screen)
		return r
	}

	// ── 2. IS THERE ANYTHING TO PRESS ──
	if st.Control == "" {
		r.Stage = StepNotRepeatable
		r.Because = "the screen changed by something other than a click, so the " +
			"response is to wait and verify rather than to press"
		return r
	}

	// ── 3. WAS THE CONTROL'S IDENTITY EVER SETTLED ──
	//
	// Before re-acquisition, because an unanswered question produces an `indistinct`
	// refusal that reads like a matcher failure and is not one.
	for _, q := range a.Questions() {
		if q.Control == st.Control {
			r.Stage = StepNeedsClarification
			r.Because = fmt.Sprintf("%s was pressed once and was inside more than "+
				"one proposed region; nobody has said which is the control, "+
				"so there is nothing definite to look for", st.Control)
			return r
		}
	}

	// ── 4. CAN IT BE SEEN NOW ──
	var outcome *observe.Reacquisition
	for i := range now.Outcomes {
		if string(now.Outcomes[i].Target) == st.Control &&
			now.Outcomes[i].Screen == bound.Screen {
			outcome = &now.Outcomes[i]
		}
	}
	switch {
	case outcome == nil:
		r.Stage = StepControlUnresolved
		r.Because = fmt.Sprintf("no outcome was offered for %s on this look", st.Control)
		return r
	case !outcome.Actionable():
		r.Stage = StepControlUnresolved
		r.Because = fmt.Sprintf("%s: %s", outcome.Result, outcome.Because)
		return r
	}

	// THE CONTROL IS WHERE IT WAS FOUND, which the person asked for authorisation needs
	// to be shown and which nothing downstream may compute for itself. See Found.
	centre := observe.PointerAt{
		X: outcome.Where.X + outcome.Where.Width/2,
		Y: outcome.Where.Y + outcome.Where.Height/2,
	}
	r.Found = &centre

	// ── 5. HAS ANYBODY AGREED, IN THIS SESSION ──
	by, confirmed := s.StepConfirmation(at)
	if !confirmed {
		r.Stage = StepUnconfirmed
		r.Because = fmt.Sprintf("nobody has agreed to step %d in this session. A "+
			"confirmation recorded in the file is audit and is never reusable "+
			"authorisation", at)
		return r
	}

	// ── 6. IS THE SESSION ALLOWED TO ACT ──
	if !now.Authorised {
		r.Stage = StepUnauthorised
		r.Because = fmt.Sprintf("step %d is confirmed by %s and the session carries no "+
			"live execution authorisation", at, by)
		return r
	}

	// ── 7. A POINT AND AN EXPECTATION. NOT PERMISSION. ──
	r.Stage = StepPlannable
	r.Action = &workflow.Action{
		Step: at, Where: outcome.Where, Support: workflow.SupportGeometric,
		Expect:  observe.VisualScreenID(st.To),
		At:      centre,
		Because: outcome.Because,
	}
	r.Because = fmt.Sprintf("%s is established here, %s was found again, %s agreed, and "+
		"the session is authorised. A point and an expectation; nothing has been "+
		"pressed", st.From, st.Control, by)
	return r
}

// ── progress ──────────────────────────────────────────────────────────────────

// Progress is how far a run got, and whether it finished.
//
// # Why finishing and stopping safely are different words
//
// A workflow interacts with something that changes after every action, so a run that performs
// three steps and then cannot establish the fourth screen has not failed and has not succeeded.
// It has stopped safely, partway. Reporting that as either would be a lie in a different
// direction, and the one place it would hurt is the only place it matters: a person deciding
// whether the thing they asked for happened.
//
// Nothing here recovers or rolls back. The distinction is kept so that it can be.
type Progress struct {
	Steps int
	// Reached is how many steps were plannable, in order, before it stopped.
	Reached int
	// Stopped is the step it stopped at, or -1 when it ran out of workflow.
	Stopped int
	// Because is why.
	Because string
	// Stage is the refusal that stopped it.
	Stage StepStage
}

// Complete reports whether every step was reached.
func (p Progress) Complete() bool { return p.Stopped < 0 && p.Reached == p.Steps }

// Partial reports whether it stopped after doing some of the work.
func (p Progress) Partial() bool { return p.Reached > 0 && !p.Complete() }

// Summary says what happened in a sentence a person could act on.
func (p Progress) Summary() string {
	switch {
	case p.Complete():
		return fmt.Sprintf("all %d step(s) reached", p.Steps)
	case p.Reached == 0:
		return fmt.Sprintf("stopped before the first step: %s", p.Because)
	default:
		return fmt.Sprintf("STOPPED SAFELY after %d of %d step(s), at step %d: %s. "+
			"The earlier steps were reached; this is neither a completed workflow "+
			"nor a failed one", p.Reached, p.Steps, p.Stopped, p.Because)
	}
}
