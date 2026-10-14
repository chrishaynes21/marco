package workflowstore

// A person saying "yes, this is that screen", and exactly what it buys.
//
// # Why this is not in the Artifact
//
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] separates three objects with
// three lifetimes, and this is the middle one:
//
//	Workflow        lives durably     "this is what you showed me"
//	Binding         ONE SESSION       "this is where I think I am"
//	Authorisation   one action        "this press, now, here"
//
// So a confirmation lives here, in a type the artifact cannot reach and `Marshal` cannot write.
// A workflow reloaded tomorrow has no bindings, however many questions were answered today, and
// that is checked by a test over the bytes rather than trusted.
//
// # What one Yes establishes
//
// Exactly one saved screen, as exactly one current screen, in exactly one observation
// generation. [[Experiment-036-recognising-a-screen-you-were-shown]] is why the last clause is
// there: similarity ranks a present screen correctly six times out of six and nominates an
// absent one just as confidently, so what a person adds is not a better ranking — it is the one
// thing no measure supplied, a statement that the screen is THERE.
//
// That statement is about what they were looking at. The observation generation moves whenever
// the standing screen changes, so leaving a screen and coming back costs two increments and the
// old Yes no longer holds. This is the same rule a delayed detection lives under, and for the
// same reason: an arrangement recurs, a moment does not.
//
// # And what it does not
//
//   - It does not bind any other screen of the workflow. Six correct rankings on one recording
//     are not a reason to carry one answer to seven screens.
//   - It does not survive the observation it was given in.
//   - It does not confirm the workflow, re-acquire a control, or authorise a press. Those stay
//     three separate operations, as they were.

import (
	"fmt"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Ground is how a binding came to be believed.
type Ground string

const (
	// GroundConfirmed: a person said yes, while looking at it.
	GroundConfirmed Ground = "confirmed"
	// GroundInferred: it followed from an established binding through a transition the
	// workflow itself demonstrates, AND the arrangement agreed.
	//
	// Kept distinct from the first because it is a weaker claim with a different failure
	// mode, and a report that counted them together would say a person had answered
	// questions they were never asked.
	GroundInferred Ground = "inferred"
)

// Bound is one saved screen, established as one current screen, in one observation.
type Bound struct {
	Label  string
	Screen observe.VisualScreenID
	Ground Ground
	// Epoch and Generation are the observation generation and window generation this was
	// established in. Both must still hold for it to mean anything.
	Epoch      uint64
	Generation uint64
	// By is who said it, for a confirmation. Empty for an inference, which nobody said.
	By string
	At time.Time
	// Depth is how many inferences away from a person's answer this is. 0 is somebody
	// looking at it; 3 is three transitions of trust, and a report that did not show this
	// would make a chain look like a confirmation.
	Depth int
	// Because is the reasoning, for a person reading a log afterwards.
	Because string
}

// Live reports whether this binding still describes the observation in front of us.
func (b Bound) Live(epoch, generation uint64) bool {
	return b.Epoch == epoch && b.Generation == generation
}

// SessionBinding is what one session has established about one workflow's screens.
//
// Session-local by construction: it has no JSON tags, is not a field of Artifact, and nothing
// marshals it. A workflow loaded in a new session starts with an empty one.
type SessionBinding struct {
	bound map[string]Bound
	// rejected records that somebody said a candidate was NOT a screen, per observation.
	// Kept so a rejection is not quietly re-proposed within the same observation, and
	// dropped with everything else when the observation moves.
	rejected map[string][]observe.VisualScreenID
	// asked counts the questions actually put to a person, which is the number the
	// experience is judged by.
	asked, confirmed, denied, unsure int

	// steps is who agreed to which step, IN THIS SESSION.
	//
	// Here rather than on the Draft because a draft is rebuilt from the artifact
	// every time the observation moves — that is what step-scoped planning does —
	// and a confirmation that lived on the draft would evaporate with it. It is a
	// fact about the session, not about where Marco happens to be standing.
	//
	// It still never reaches a file. The thing forbidden is a confirmation
	// surviving a RELOAD, and a SessionBinding has no JSON tags and is not a field
	// of Artifact.
	steps map[int]string
}

func NewSessionBinding() *SessionBinding {
	return &SessionBinding{
		bound:    map[string]Bound{},
		rejected: map[string][]observe.VisualScreenID{},
		steps:    map[int]string{},
	}
}

// Asked, Confirmed, Denied and Unsure are what a person was put through.
func (s *SessionBinding) Asked() int     { return s.asked }
func (s *SessionBinding) Confirmed() int { return s.confirmed }
func (s *SessionBinding) Denied() int    { return s.denied }
func (s *SessionBinding) Unsures() int   { return s.unsure }

// ConfirmStep records that somebody agreed to one step of the workflow, in this session.
//
// Separate from `Confirm`, which is about a screen. Agreeing that this is the Play menu and
// agreeing to press something on it are two different sentences, and a build that conflated them
// would let a person answer one and be taken to have answered the other.
func (s *SessionBinding) ConfirmStep(at int, by string) error {
	if by == "" {
		return fmt.Errorf("workflowstore: a confirmation needs somebody to have given it")
	}
	if at < 0 {
		return fmt.Errorf("workflowstore: step %d is not a step", at)
	}
	s.steps[at] = by
	return nil
}

// StepConfirmation says who agreed to a step in this session, if anybody has.
func (s *SessionBinding) StepConfirmation(at int) (string, bool) {
	by, ok := s.steps[at]
	return by, ok
}

// Confirm records a person's yes.
//
// `epoch` and `generation` are the observation they were looking at, supplied by the caller that
// showed them the question. A confirmation with no name attached is refused, for the reason
// every other provenance in this package is: an unattributed answer is indistinguishable from a
// default.
func (s *SessionBinding) Confirm(label string, screen observe.VisualScreenID,
	epoch, generation uint64, by string, at time.Time) error {

	switch {
	case label == "" || screen == "":
		return fmt.Errorf("workflowstore: a confirmation needs a saved screen and a " +
			"current one")
	case by == "":
		return fmt.Errorf("workflowstore: a confirmation needs somebody to have given it")
	case at.IsZero():
		return fmt.Errorf("workflowstore: a confirmation needs a time")
	}
	for _, no := range s.rejected[label] {
		if no == screen {
			return fmt.Errorf("workflowstore: %s was already rejected as %s in this "+
				"observation", screen, label)
		}
	}
	s.asked++
	s.confirmed++
	s.bound[label] = Bound{
		Label: label, Screen: screen, Ground: GroundConfirmed,
		Epoch: epoch, Generation: generation, By: by, At: at.UTC(),
		Because: fmt.Sprintf("%s confirmed this is %s while looking at it", by, label),
	}
	return nil
}

// Reject records a person's no.
//
// It establishes nothing, and in particular it does NOT promote the runner-up. A measure that
// ranked the wrong screen first has not thereby earned belief in its second choice; Experiment
// -036's wrong winners beat their runners-up by 0.419, so the ordering below a wrong answer is
// not evidence of anything.
func (s *SessionBinding) Reject(label string, screen observe.VisualScreenID, by string) error {
	if by == "" {
		return fmt.Errorf("workflowstore: a rejection needs somebody to have given it")
	}
	s.asked++
	s.denied++
	s.rejected[label] = append(s.rejected[label], screen)
	return nil
}

// Unsure records that a person did not know, which changes nothing at all.
func (s *SessionBinding) Unsure(label string, by string) error {
	if by == "" {
		return fmt.Errorf("workflowstore: an answer needs somebody to have given it")
	}
	s.asked++
	s.unsure++
	return nil
}

// Rejected reports whether a candidate has already been turned down for this screen.
func (s *SessionBinding) Rejected(label string, screen observe.VisualScreenID) bool {
	for _, no := range s.rejected[label] {
		if no == screen {
			return true
		}
	}
	return false
}

// Established is the binding for one saved screen, if one still holds in this observation.
//
// The staleness test is here rather than at the call sites, so a caller cannot forget it. A
// binding from a previous observation is not returned, not repaired and not downgraded — it is
// simply not there, which is the same answer as never having been given.
func (s *SessionBinding) Established(label string, epoch, generation uint64) (Bound, bool) {
	b, ok := s.bound[label]
	if !ok || !b.Live(epoch, generation) {
		return Bound{}, false
	}
	return b, true
}

// Observed drops everything that the current observation has outrun.
//
// Called when the observation moves. Not aged, not re-scored, not carried forward on the
// grounds that the screen looks the same — the whole point of an observation generation is that
// it can tell a second visit from the first, and an arrangement cannot.
func (s *SessionBinding) Observed(epoch, generation uint64) (dropped int) {
	for label, b := range s.bound {
		if !b.Live(epoch, generation) {
			delete(s.bound, label)
			dropped++
		}
	}
	for label := range s.rejected {
		delete(s.rejected, label)
	}
	return dropped
}

// Binding is the established screens as the planner's own type.
//
// Only what still holds, so a draft bound from this cannot plan against a screen whose
// confirmation has expired.
func (s *SessionBinding) Binding(epoch, generation uint64) Binding {
	b := Binding{Screens: map[string]observe.VisualScreenID{}}
	for label, bound := range s.bound {
		if bound.Live(epoch, generation) {
			b.Screens[label] = bound.Screen
		}
	}
	return b
}

// Live is every binding that still holds, strongest provenance first.
func (s *SessionBinding) Live(epoch, generation uint64) []Bound {
	var out []Bound
	for _, b := range s.bound {
		if b.Live(epoch, generation) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Depth != out[j].Depth {
			return out[i].Depth < out[j].Depth
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// Infer establishes a screen that FOLLOWED from one already established.
//
// # Why this is allowed at all, when similarity alone is not
//
// Experiment-036 located the failure precisely: similarity ranks a present screen correctly and
// nominates an absent one just as confidently, so it cannot say whether the right answer is in
// the list. A transition answers exactly that and nothing else — the workflow demonstrates that
// `screen_a` leads to `screen_b`, so if we were established on `screen_a` and the screen
// changed, `screen_b` is the thing to expect.
//
// Expectation supplies presence; similarity supplies agreement. Each covers the other's
// weakness, and NEITHER IS SUFFICIENT ALONE:
//
//   - without the transition, similarity would nominate something for a screen that is not
//     there, which is the two wrong bindings of Experiment-029;
//   - without the similarity, this would assert identity from expectation, which is the oldest
//     rule in this subsystem broken — delivery is not success.
//
// So both are required, and the result is marked `inferred` with its distance from the person
// who actually looked at something.
func (s *SessionBinding) Infer(a Artifact, from string, to string,
	screen observe.VisualScreenID, agreed float64, epoch, generation uint64) (Bound, error) {

	anchor, ok := s.bound[from]
	if !ok {
		return Bound{}, fmt.Errorf("workflowstore: nothing is established about %s, so "+
			"nothing follows from it", from)
	}
	// THE ANCHOR MUST STILL HOLD, in the observation BEFORE this one moved. A chain built
	// on a binding that had already expired would be inference from nothing.
	if anchor.Generation != generation {
		return Bound{}, fmt.Errorf("workflowstore: %s was established under another "+
			"window", from)
	}
	if !a.leadsTo(from, to) {
		return Bound{}, fmt.Errorf("workflowstore: %q does not demonstrate a step from "+
			"%s to %s, so there is nothing to follow", a.Name, from, to)
	}
	if agreed < observe.VisualScreenMatch {
		return Bound{}, fmt.Errorf("workflowstore: the screen now agrees with %s at "+
			"%.3f, below %.2f; the workflow expected to arrive here and the "+
			"evidence does not agree, which is a reason to ask rather than to "+
			"assume", to, agreed, observe.VisualScreenMatch)
	}
	b := Bound{
		Label: to, Screen: screen, Ground: GroundInferred,
		Epoch: epoch, Generation: generation, Depth: anchor.Depth + 1,
		Because: fmt.Sprintf("the workflow goes %s -> %s, %s was established (%s, "+
			"depth %d), and the screen now agrees with %s at %.3f",
			from, to, from, anchor.Ground, anchor.Depth, to, agreed),
	}
	s.bound[to] = b
	return b, nil
}

// leadsTo reports whether the workflow demonstrates a step from one saved screen to another.
func (a Artifact) leadsTo(from, to string) bool {
	for _, st := range a.Steps {
		if st.From == from && st.To == to {
			return true
		}
	}
	return false
}
