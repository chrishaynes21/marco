package workflowstore

// Asking a person the smallest missing thing.
//
// # Why this exists instead of a better matcher
//
// [[Experiment-035-what-a-control-looks-like]] ended a line of work. `control_6` is a 170x14
// label inside the 352x34 button that contains it, and no descriptor of those rectangles can
// separate them, because the smaller one's pixels are a subset of the larger one's. They are not
// two things that look alike; they are two extents of one thing.
//
// The owner drew the conclusion:
//
//	We shouldn't build increasingly sophisticated visual matching just to avoid acknowledging
//	uncertainty that the system has already identified.
//
// The artifact already says `clicked once, so nothing independent confirms that this control can
// be found again`. So the missing piece is not perception. It is a sentence from the person who
// did the demonstrating, and this file is the contract for asking.
//
// # What a clarification is, and the three things it is not
//
// It is EVIDENCE, attached to one control of one workflow, carrying who said it and when.
//
//   - It is not a RULE. Nothing here learns that a label's container is its button. The answer
//     is about `control_6` of this workflow and reaches nothing else.
//   - It is not RECOGNITION. Choosing a region in a recorded frame says which interpretation the
//     person meant; it says nothing about whether that region is on the screen now. Binding is
//     unchanged and still refuses.
//   - It is not AUTHORISATION. A clarified workflow is still unconfirmed and still unexecutable.
//     Answering a question is not agreeing to press anything.

import (
	"fmt"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Interpretation is one of the competing readings of a control, named so a person can point at
// it and an answer can be checked against it.
type Interpretation struct {
	// Ref is stable and inspectable: `reading_1`, `container_1`.
	Ref  string         `json:"ref"`
	Rect observe.Region `json:"rect"`
	// What this rectangle is, in the demonstration's own terms.
	What string `json:"what"`
}

// Question is what Marco needs settled about one saved control.
type Question struct {
	Control string `json:"control"`
	// Steps is every step of the workflow that presses this control, so a person can see
	// that answering once settles all of them.
	Steps   []int            `json:"steps"`
	Because string           `json:"because"`
	Options []Interpretation `json:"options"`
}

// AnswerOutcome is what the person did with a question.
type AnswerOutcome string

const (
	// AnswerChosen: they named one of the interpretations.
	AnswerChosen AnswerOutcome = "chosen"
	// AnswerRefused: they say none of these is the control. A real answer, and it leaves
	// the workflow honestly stuck rather than wrongly unstuck.
	AnswerRefused AnswerOutcome = "refused"
	// AnswerDeferred: they do not know yet. Recorded so the question is not asked again
	// as though it had never been put.
	AnswerDeferred AnswerOutcome = "deferred"
)

// Answer is a person's response, with the provenance that makes it evidence.
type Answer struct {
	Control string        `json:"control"`
	Outcome AnswerOutcome `json:"outcome"`
	// Ref is the interpretation they named, and is set only when chosen.
	Ref string `json:"ref,omitempty"`
	// By is who said it. An unattributed clarification is indistinguishable from a
	// default, which is the thing this must never become.
	By   string    `json:"by"`
	At   time.Time `json:"at"`
	Note string    `json:"note,omitempty"`
}

// Questions is everything about this workflow that a person could settle.
//
// # Only what is actually ambiguous, which is narrower than "has a container"
//
// Having a container recorded is not the same as being confused with it. `control_2` on
// `rl-freeplay-1` is the TRAINING tile and its container is a slab of stadium crowd; the two are
// not the same rectangle by the matcher's relation, so the tile is found again on 21 looks of 28
// and there is nothing to ask. An earlier version of this asked anyway, and a question about a
// control that already works is how a person learns to click through prompts.
//
// So the matcher decides. A container becomes a question only when a proposal that WAS that
// container would be refused as indistinct from the control — asked by running the real
// `observe.ReacquireStored` over it rather than by restating `sameRect` here, which is the only
// way this and the thing it describes cannot drift apart.
//
// A control settled by a second press — [[ADR-182-two-controls-that-could-never-be-found-again]]'s
// rule — has no container left at all, so it never reaches this.
func (a Artifact) Questions() []Question {
	steps := map[string][]int{}
	for _, st := range a.Steps {
		if st.Control != "" {
			steps[st.Control] = append(steps[st.Control], st.Position)
		}
	}
	answered := map[string]bool{}
	for _, c := range a.Clarifications {
		if c.Outcome != AnswerDeferred {
			answered[c.Control] = true
		}
	}

	var out []Question
	for _, c := range a.Controls {
		if len(c.Readings) == 0 || answered[c.Label] {
			continue
		}
		confusing := confusable(c)
		if len(confusing) == 0 {
			continue
		}
		q := Question{
			Control: c.Label,
			Steps:   steps[c.Label],
			Because: "one press was observed here, and it was inside more than one " +
				"proposed region. Nothing in the demonstration says which of " +
				"them is the control, and on a later look they cannot be told " +
				"apart.",
		}
		for i, r := range c.Readings {
			q.Options = append(q.Options, Interpretation{
				Ref: fmt.Sprintf("reading_%d", i+1), Rect: r,
				What: "the smallest region the press was inside, which is what " +
					"this session took the control to be",
			})
		}
		for _, i := range confusing {
			r := c.Containers[i]
			q.Options = append(q.Options, Interpretation{
				Ref: fmt.Sprintf("container_%d", i+1), Rect: r,
				What: "a larger region that also contained the press",
			})
		}
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Control < out[j].Control })
	return out
}

// confusable is the indices of the containers a proposal could not be told apart from.
//
// Asked of the matcher itself: offer the container AS a proposal and see whether the control is
// refused for indistinctness. That is the exact condition that makes the control unfindable, so
// it is the exact condition worth a person's time.
func confusable(c Control) []int {
	var out []int
	for i, container := range c.Containers {
		got := observe.ReacquireStored(observe.StoredTarget{
			ID: observe.VisualTargetID(c.Label), Screen: "probe",
			Readings: c.Readings, Containers: c.Containers,
		}, []observe.Region{container})
		if got.Result == observe.ReacquireUnobservable && got.Indistinct > 0 {
			out = append(out, i)
		}
	}
	return out
}

// Clarify records an answer, refusing one that does not correspond to a question asked.
//
// Validated rather than trusted, for the same reason a binding is: an answer naming a control
// this artifact does not describe, or an interpretation it never offered, is not a clarification
// — it is a claim about something else that would be filed as though it were about this.
func (a *Artifact) Clarify(ans Answer) error {
	if ans.By == "" {
		return fmt.Errorf("workflowstore: a clarification needs somebody to have given it")
	}
	if ans.At.IsZero() {
		return fmt.Errorf("workflowstore: a clarification needs a time")
	}
	var q *Question
	for _, got := range a.Questions() {
		if got.Control == ans.Control {
			g := got
			q = &g
			break
		}
	}
	// A control with no open question may still be DEFERRED again or re-answered only if
	// nothing was settled; otherwise there is nothing this could be about.
	if q == nil {
		return fmt.Errorf("workflowstore: %q has no open question in %q",
			ans.Control, a.Name)
	}
	switch ans.Outcome {
	case AnswerChosen:
		ok := false
		for _, o := range q.Options {
			if o.Ref == ans.Ref {
				ok = true
			}
		}
		if !ok {
			return fmt.Errorf("workflowstore: %q is not one of the interpretations "+
				"offered for %s", ans.Ref, ans.Control)
		}
	case AnswerRefused, AnswerDeferred:
		if ans.Ref != "" {
			return fmt.Errorf("workflowstore: a %s answer names %q; it must name "+
				"nothing", ans.Outcome, ans.Ref)
		}
	default:
		return fmt.Errorf("workflowstore: %q is not an answer", ans.Outcome)
	}
	ans.At = ans.At.UTC()
	a.Clarifications = append(a.Clarifications, ans)
	return nil
}

// Resolution is what a clarification did to a control, in words.
type Resolution struct {
	Control string
	// Settled says the control can now be matched. An answer can be recorded and leave
	// this false — see clarified.
	Settled bool
	Because string
}

// clarified applies the answers to one control's evidence.
//
// # The asymmetry, which is the whole of this function
//
// Naming the LARGER region settles the control. The person has said the button is what they
// pressed; the label inside it is not a region that merely contained the press, it is part of
// the thing, so there is no container left to be confused with and a proposal agreeing with the
// button matches.
//
// Naming the SMALLER region does NOT settle it, and saying so is the honest part. Marco still
// cannot tell, on a later look, whether a proposal describes the label or the button around it —
// that is what Experiment-035 measured — so the exclusion that refuses the pair stands. The
// answer is kept, because it is true and a second demonstration would make it usable; it just
// does not make this control findable today.
//
// Nothing here is a rule about labels and buttons. It is arithmetic on two rectangles of one
// control: whichever the person named, anything strictly inside it stops being a container.
func (a Artifact) clarified(c Control) (Control, Resolution) {
	res := Resolution{Control: c.Label}
	var ans *Answer
	for i := range a.Clarifications {
		if a.Clarifications[i].Control == c.Label {
			ans = &a.Clarifications[i]
		}
	}
	switch {
	case ans == nil:
		res.Because = "nobody has been asked, so the competing readings stand and " +
			"the control is refused"
		return c, res
	case ans.Outcome == AnswerDeferred:
		res.Because = fmt.Sprintf("%s deferred this; the competing readings stand",
			ans.By)
		return c, res
	case ans.Outcome == AnswerRefused:
		res.Because = fmt.Sprintf("%s says none of these is the control, so there is "+
			"nothing to match and the step cannot be performed", ans.By)
		return c, res
	}

	chosen, ok := a.interpretation(c, ans.Ref)
	if !ok {
		res.Because = "the recorded answer names an interpretation this control does " +
			"not have, so it is ignored"
		return c, res
	}

	// WHAT THE CHOICE LEAVES BEHIND: every other interpretation that is not strictly
	// inside the chosen one is still something that merely contained the press.
	out := Control{Label: c.Label, Readings: []observe.Region{chosen}}
	for _, r := range append(append([]observe.Region{}, c.Readings...), c.Containers...) {
		if r == chosen || within(r, chosen) {
			continue
		}
		out.Containers = append(out.Containers, r)
	}
	if len(out.Containers) == 0 {
		res.Settled = true
		res.Because = fmt.Sprintf("%s says the control is %s, and nothing recorded "+
			"around it is larger, so there is no longer a region it could be "+
			"confused with", ans.By, ans.Ref)
		return out, res
	}
	res.Because = fmt.Sprintf("%s says the control is %s, which is recorded; but %d "+
		"larger region(s) still contained the press and cannot be told apart from "+
		"it on a later look, so the control is still refused. A second "+
		"demonstration would settle that.", ans.By, ans.Ref, len(out.Containers))
	return out, res
}

// interpretation resolves a ref to the rectangle it names.
func (a Artifact) interpretation(c Control, ref string) (observe.Region, bool) {
	for i, r := range c.Readings {
		if ref == fmt.Sprintf("reading_%d", i+1) {
			return r, true
		}
	}
	for i, r := range c.Containers {
		if ref == fmt.Sprintf("container_%d", i+1) {
			return r, true
		}
	}
	return observe.Region{}, false
}

// within reports whether `inner` is wholly inside `outer`, allowing a pixel of slack at
// 1920x1080 so a rectangle recorded a hair outside is not treated as enclosing.
func within(inner, outer observe.Region) bool {
	const slack = 1.0 / 1920
	return inner.X >= outer.X-slack &&
		inner.Y >= outer.Y-slack &&
		inner.X+inner.Width <= outer.X+outer.Width+slack &&
		inner.Y+inner.Height <= outer.Y+outer.Height+slack
}

// Resolutions is what every clarified control's evidence now says.
func (a Artifact) Resolutions() []Resolution {
	var out []Resolution
	for _, c := range a.Controls {
		if len(c.Containers) == 0 {
			continue
		}
		_, r := a.clarified(c)
		out = append(out, r)
	}
	return out
}
