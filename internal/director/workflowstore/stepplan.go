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
	"strings"

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
	// StepControlUnlocated: the control was found again, and what was found does not say
	// where it is.
	//
	// A separate stage from StepControlUnresolved on purpose, because the two tell a person
	// to do opposite things. "Not re-acquired" means look again — the control may be mid
	// animation, or behind something, and the next observation may resolve it. This one
	// will never resolve: the saved target is a band across the window, so every future
	// look that matches it will match it just as well and offer the same useless centre.
	// The workflow needs re-teaching, not more patience.
	StepControlUnlocated StepStage = "what was found does not locate the control"
	// StepControlContradicted: a region WAS matched, and the words on this look say it
	// is not this control while something else on the same look is.
	//
	// A third thing again, and the difference is the whole of why it exists. "Not
	// re-acquired" means look again. "Does not locate" means re-teach. This one means
	// LOOK SOMEWHERE ELSE ON THIS SAME SCREEN — the control is here, the match is not
	// it, and the better candidate has already been seen. It is never resolved by
	// patience, and the repair can act on it immediately.
	StepControlContradicted StepStage = "what was matched is not the control"
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
	// Reground is a region to use INSTEAD of what the file says about a control,
	// supplied by whoever established it in this session.
	//
	// # Why a saved reading is sometimes the broken thing
	//
	// Four of the nine controls in the Rocket League artifact are bands across the whole
	// window. They are not hard to find — they match perfectly, every look — they simply
	// do not say where anything is, and `Region.Locates` refuses them. No amount of
	// looking repairs that: the demonstration recorded a rectangle that locates nothing,
	// and the rectangle is what is wrong.
	//
	// So this is the one case where the file's own evidence is set aside. It is not a
	// correction TO the artifact — nothing here writes one — it is a region established
	// for this session by somebody who could see the screen, in the same shape as a
	// screen binding and with the same lifetime.
	//
	// It replaces the agreement step and NOT the Locates check: a repair that named
	// another band would be the same defect with a different provenance.
	Reground map[string]observe.Region
	// Candidates is every region the sources proposed on this look, and Words every word
	// that was read on it, both normalised to the window.
	//
	// Supplied rather than discovered, like everything else here: this package can see no
	// screen. They are what makes it possible to ask whether a geometric match is the
	// control it was matched to — see observe.ControlIdentityOf. Both empty is the
	// ordinary case for a run with no reader, and means the question is not asked rather
	// than answered badly.
	Candidates []observe.Region
	Words      []observe.ReadWord
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
	// Grounding is WHAT KIND OF EVIDENCE put the target where it is.
	//
	// Separate from `Stage` because a plannable step is plannable either way, and a
	// report that said "re-acquired" for all of them would describe a geometric
	// coincidence in the same words as a corroborated identity. Experiment-044 is
	// exactly that sentence being believed: `re-acquired on this look` was printed over
	// a patch of pitch.
	Grounding Grounding
	// Identity is what the words on this look made of the match, when there were any.
	// Carried so a reader can see the question was asked and what it answered, including
	// when the answer was "there was nothing to ask with".
	Identity observe.ControlIdentity
}

// Grounding is what kind of evidence placed a step's target.
type Grounding string

const (
	// GroundingNone: no target.
	GroundingNone Grounding = ""
	// GroundingGeometric: a proposer drew a box in the same place as the saved reading,
	// and nothing on this look either corroborated or contradicted it. This is the
	// WEAKEST thing that may still be pressed, and it is what every run before
	// Experiment-044 called "re-acquired".
	GroundingGeometric Grounding = "geometrically re-acquired"
	// GroundingCorroborated: and the words inside it say what this control is recorded
	// as saying.
	GroundingCorroborated Grounding = "geometrically re-acquired and semantically corroborated"
	// GroundingRegrounded: the saved reading was set aside and the region was
	// established from this screen, by the repair.
	GroundingRegrounded Grounding = "semantically re-grounded for this session"
)

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
	regrounded, repaired := now.Reground[st.Control]

	// A REPAIR IS ITSELF AN ANSWER TO "CAN IT BE SEEN NOW"
	//
	// Not a way around the question. The repair was established by looking at THIS screen
	// — the regions the sources proposed on it and the words read inside them — so a
	// control re-grounded here was seen, by something other than the file's own readings.
	// Those readings are the thing that failed, and asking them again is how the measured
	// defect produced sixty identical refusals: Rocket League's main menu is
	// bottom-anchored, and signed out it loses two entries, so PLAY sits 95px below where
	// the demonstration recorded it. Matching against the recorded pixels of a
	// semi-transparent button over a different car and a different arena cannot succeed,
	// and nothing about waiting changes it.
	//
	// What this does NOT skip is 4a. A repair that names a band across the window is
	// refused exactly as a recorded one is — see the comment there.
	switch {
	case repaired:
	case outcome == nil:
		r.Stage = StepControlUnresolved
		r.Because = fmt.Sprintf("no outcome was offered for %s on this look", st.Control)
		return r
	case !outcome.Actionable():
		r.Stage = StepControlUnresolved
		r.Because = fmt.Sprintf("%s: %s", outcome.Result, outcome.Because)
		return r
	}

	// ── 4a. DOES WHAT WAS FOUND SAY WHERE THE CONTROL IS ──
	//
	// Matching is about rectangles; pressing is about a point. A region that runs edge to
	// edge across the window is matched again perfectly well and names nowhere — see
	// observe.Region.Locates.
	//
	// Asked BEFORE the agreement below, and the order is load-bearing. A window-spanning
	// match overlaps every saved reading, so narrowing it would produce the saved reading
	// back and present a recorded coordinate as a grounded one. A match that locates
	// nothing corroborates nothing, and there is no arithmetic that repairs it.
	//
	// Refused before `Found` is set, so there is no position to show and no position to
	// press. A step that cannot say where its control is must not offer a point that looks
	// exactly like one it found.
	// A REGION ESTABLISHED FOR THIS SESSION REPLACES WHAT THE FILE SAYS, and skips both
	// of the rules below, because both of them are questions about the file's readings
	// and this exists for the case where those readings are the broken thing.
	//
	// It does NOT skip Locates. A repair that named another band across the window would
	// be the same defect with a better provenance.
	var where observe.Region
	switch {
	case repaired:
		where = regrounded
	default:
		where = outcome.Where
	}
	if ok, why := where.Locates(); !ok {
		r.Stage = StepControlUnlocated
		if repaired {
			r.Because = fmt.Sprintf("%s was re-grounded for this session and %s; "+
				"a repair that locates nothing is the defect it was meant "+
				"to fix", st.Control, why)
			return r
		}
		r.Because = fmt.Sprintf("%s was matched again, but %s; this step has no point "+
			"to press and no substitute for one will be invented", st.Control, why)
		return r
	}

	// ── 4b. THE PART THE DEMONSTRATION AND THIS LOOK BOTH DESCRIBE ──
	//
	// Re-acquisition is generous on purpose — a proposal that spills past a control is
	// still evidence the control is there — and a generous rectangle has a misleading
	// middle. Measured: a live look matched PLAY with a region running from the window's
	// left edge to 132px, and its middle landed three pixels inside a button that starts
	// at 62. See observe.Agreed.
	if saved := a.readingsOf(st.Control); len(saved) > 0 && !repaired {
		agreed, ok := observe.Agreed(where, saved)
		if !ok {
			r.Stage = StepControlUnlocated
			r.Because = fmt.Sprintf("%s was matched against a region the saved "+
				"readings do not touch, so the two disagree about where it "+
				"is and nothing here chooses between them", st.Control)
			return r
		}
		where = agreed
	}

	// ── 4c. AND IS WHAT WAS MATCHED THIS CONTROL ──
	//
	// Everything above establishes OVERLAP. `sameControl` is mutual centre containment
	// and consults no pixels and no text, so a proposer that boxed a patch of background
	// inside the recorded rectangle re-acquires perfectly — which is what Experiment-044
	// pressed, 75px above a PLAY button whose label was read on the very same look.
	//
	// So this is the last question before a point exists, and it is asked in one
	// direction only: text may REFUSE a match and may never propose one. A refusal sends
	// the step to the repair, which is the thing allowed to nominate a region.
	//
	// NOT asked about a repaired region. The repair established it by reading this screen
	// — asking the same words to confirm themselves would be a second vote from one
	// source, and `GroundingRegrounded` already says what the evidence was.
	r.Grounding = GroundingGeometric
	if repaired {
		r.Grounding = GroundingRegrounded
	} else if len(now.Words) > 0 {
		wanted, from := a.ExpectedWords(st.Control)
		r.Identity = observe.ControlIdentityOf(where, now.Candidates, now.Words, wanted)
		switch {
		case r.Identity.Contradicted():
			r.Stage = StepControlContradicted
			r.Because = fmt.Sprintf("%s was matched by position only, and %s. The "+
				"words it is being checked against are %s",
				st.Control, r.Identity.Because, from)
			return r
		case r.Identity.Result == observe.IdentityCorroborated:
			r.Grounding = GroundingCorroborated
		}
	}

	// THE CONTROL IS WHERE IT WAS FOUND, which the person asked for authorisation needs
	// to be shown and which nothing downstream may compute for itself. See Found.
	centre := observe.PointerAt{
		X: where.X + where.Width/2,
		Y: where.Y + where.Height/2,
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
		Step: at, Where: where, Support: workflow.SupportGeometric,
		Expect: observe.VisualScreenID(st.To),
		At:     centre,
		Because: func() string {
			if repaired {
				return "re-grounded for this session; the saved reading of " +
					st.Control + " locates nothing"
			}
			return outcome.Because
		}(),
	}
	r.Because = fmt.Sprintf("%s is established here, %s was found again, %s agreed, and "+
		"the session is authorised. A point and an expectation; nothing has been "+
		"pressed", st.From, st.Control, by)
	return r
}

// readingsOf is every saved reading of one control.
//
// The CONTROL's readings only, never its containers: a container is a region that also held the
// press, and agreeing with one would put the point in the middle of the thing the control sits
// in rather than in the control.
func (a Artifact) readingsOf(label string) []observe.Region {
	for _, c := range a.Controls {
		if c.Label == label {
			return c.Readings
		}
	}
	return nil
}

// ExpectedWords is what one control is recorded as saying, and where that came from.
//
// # Two sources, and they are not equally good
//
// A demonstration that recorded the text ON the control has somebody's actual intent in it: they
// clicked the thing that said PLAY. The workflow's NAME has only the task, and matching a label
// against it is the workflow corroborating itself — enough to nominate a candidate, and reported
// everywhere as weaker than somebody pointing at it.
//
// So the saved text wins whenever there is any, and the provenance is returned beside the words
// rather than left for a caller to guess, because a report that did not distinguish them would
// make the weak case read like the strong one. No demonstration writes `Control.Text` yet; see
// the field.
func (a Artifact) ExpectedWords(label string) (words []string, from string) {
	for _, c := range a.Controls {
		if c.Label != label {
			continue
		}
		var saved []string
		for _, t := range c.Text {
			saved = append(saved, observe.Words(t)...)
		}
		if len(saved) > 0 {
			return saved, fmt.Sprintf("the text the demonstration read on this "+
				"control (%q)", strings.Join(c.Text, " "))
		}
		break
	}
	// THE WORDS THEMSELVES, not just where they came from. A reader weighing a repair
	// has to see what was matched against, and a provenance without its content makes
	// the decision unauditable from the report alone.
	if w := observe.Words(a.Name); len(w) > 0 {
		return w, fmt.Sprintf("the workflow's own name (%q), which is weaker than "+
			"somebody pointing", a.Name)
	}
	return nil, "nothing recorded says what this control should say"
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
	// Acted is how many actions were performed.
	//
	// Separate from Reached because they count different things and the difference is
	// the whole subject of this type: Reached counts steps that ARRIVED, and a press
	// that went out and produced nothing advances neither. Without this, a run that
	// lowered, compiled, ran and sent an action reports itself as having stopped before
	// the first step, which reads as a run where nothing happened.
	Acted int
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
	case p.Reached == 0 && p.Acted > 0:
		return fmt.Sprintf("stopped at the first step, which was acted on and did "+
			"not arrive: %s", p.Because)
	case p.Reached == 0:
		return fmt.Sprintf("stopped before the first step: %s", p.Because)
	default:
		return fmt.Sprintf("STOPPED SAFELY after %d of %d step(s), at step %d: %s. "+
			"The earlier steps were reached; this is neither a completed workflow "+
			"nor a failed one", p.Reached, p.Steps, p.Stopped, p.Because)
	}
}
