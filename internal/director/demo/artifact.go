package demo

import (
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A DEMONSTRATION, SAVED UNDER THE NAME A PERSON GAVE IT.
//
// # The gap this closes, and why it is not a new representation
//
// [[Experiment-074-two-halves-of-teach-and-replay]] measured two learning subsystems, each
// holding the half the other lacked. A Director demonstration knows the verbs, the targets, the
// transitions and the verification; a workflow artifact is named, durable, inert, forbidden from
// carrying handles or coordinates, and already the thing `director perform-workflow` replays. The
// owner's call:
//
//	We shouldn't build a third system. We should connect the two we already have.
//	An arbitrary demonstrated routine should not be forced into a goal template.
//	"my settings tour" doesn't need to mean anything to Marco. It's simply the name of a
//	saved sequence of observed actions.
//
// So this writes one into the other. Nothing here is a new format: `Extract` already turns a
// demonstration into a parameterized procedure, and this turns the same demonstration into a
// named sequence instead — the difference being that a sequence keeps every step as it was
// demonstrated and generalizes nothing.
//
// Two things made the seam small enough to be worth preferring over a third store. `Validate`
// already accepts a step whose `From` and `To` are empty, because a step that names no screen
// names no screen that must exist. And `Control.Text` was already there, described by its own
// doc as *the strongest evidence of identity the artifact can carry* and *empty in every
// artifact recorded so far, because no demonstration writes it yet*. One does now.
//
// # What a sequence refuses to save
//
// Each for a reason, and each asserted:
//
//   - A SENSITIVE STEP. `Unsafe` already refuses a demonstration carrying one, and this refuses
//     it again rather than trusting that, because a password in a durable file is the one
//     mistake here that cannot be taken back.
//   - AN UNVERIFIED STEP. A sequence is a record of things that worked. A step whose outcome
//     Marco could not confirm is not evidence that doing it again will work, and saving it would
//     make the artifact's quietest field — `Seen` — a lie.
//   - A STEP THAT POINTED. "This file" means whatever the person was looking at, and nothing in
//     a durable artifact can say what that was tomorrow. The control is saved with `Deictic` set
//     so the refusal happens at save time with a sentence, rather than at replay time with a
//     binding failure three steps in.
//   - A PROGRAM VALUE REFERENCE. `ValueRef` names something the session captured, which is gone.
//
// # And what it does not claim
//
// Saving is not recognition and not authority — [[ADR-183-remembering-recognising-and-being-allowed-to-act]]
// is unchanged by this and the artifact is as inert as it ever was. A saved sequence is "this is
// what you showed me". Whether the screen in front of Marco is the one it was shown, and whether
// anybody may press anything on it, are the two questions this file does not answer.
//
// Enforced by TestADemonstrationSavesAsANamedArtifact,
// TestASequenceRefusesWhatItCannotReplay and TestNothingMechanicalSurvivesASavedSequence.

// FromDemonstration writes a demonstration as a named, durable artifact.
//
// The name is the PERSON's. It is not derived, not checked against a vocabulary and not required
// to mean anything — see the owner's sentence above. It is required to be non-empty, which
// `workflowstore.SaveWith` has always required for the same reason: the person has to be able to
// ask for it by one.
func FromDemonstration(name string, d *Demonstration, at time.Time) (
	workflowstore.Artifact, error) {

	if d == nil {
		return workflowstore.Artifact{}, fmt.Errorf("there is no demonstration to save")
	}
	if strings.TrimSpace(name) == "" {
		return workflowstore.Artifact{}, fmt.Errorf(
			"a saved sequence needs a name, because you have to be able to ask for it by one")
	}
	if d.Status != Completed {
		return workflowstore.Artifact{}, fmt.Errorf(
			"%s is %s, and only a completed demonstration is a record of anything",
			d.ID, d.Status)
	}
	if why, unsafe := Unsafe(d); unsafe {
		return workflowstore.Artifact{}, fmt.Errorf("%s cannot be saved: %s", d.ID, why)
	}
	// AND THE STEP-BY-STEP REFUSALS, through the same function a caller asks with, so
	// that what a person is told before saving and what stops the save are one sentence.
	if why := RefusalToSave(d); why != "" {
		return workflowstore.Artifact{}, fmt.Errorf("%s cannot be saved as %q: %s",
			d.ID, name, why)
	}
	steps := replayable(d)
	if len(steps) == 0 {
		return workflowstore.Artifact{}, fmt.Errorf(
			"%s has no steps that could be done again", d.ID)
	}

	a := workflowstore.Artifact{
		Version: workflowstore.Version, Name: strings.TrimSpace(name), SavedAt: at.UTC(),
		Notes: []string{
			fmt.Sprintf("demonstrated in %s on %s, as %s",
				describeApp(d.Application), d.Started.UTC().Format("2006-01-02"), d.ID),
		},
	}
	if len(d.Requests) > 0 {
		a.Notes = append(a.Notes, "asked for: "+strings.Join(quoteAll(d.Requests), ", "))
	}

	// ONE CONTROL PER DISTINCT WAY OF ADDRESSING ONE, so a step that acts on the same
	// control twice says so — which is what `Seen` is for, and what a reader comparing
	// two positions needs.
	slot := map[string]int{}
	for i, s := range steps {
		st := workflowstore.Step{
			Position: i,
			Verb:     string(s.Semantic),
			Says:     s.Phrase,
		}
		if s.Kind == StepEdit {
			st.Verb = editVerb
			st.Value = s.Text
		}
		if s.Kind == StepFocus {
			st.Verb = focusVerb
		}
		c := controlOf(s)
		key := controlKey(c)
		at, ok := slot[key]
		if !ok {
			c.Label = fmt.Sprintf("control_%d", len(a.Controls)+1)
			a.Controls = append(a.Controls, c)
			at = len(a.Controls) - 1
			slot[key] = at
		}
		st.Control = a.Controls[at].Label
		a.Steps = append(a.Steps, st)
	}
	// SEEN, counted after the fact: the number of demonstrated steps that addressed this
	// control. A control pressed twice is better evidenced than one pressed once, and the
	// clarification path already reads this field to decide what to ask about.
	seen := map[string]int{}
	for _, st := range a.Steps {
		seen[st.Control]++
	}
	for i := range a.Steps {
		a.Steps[i].Seen = seen[a.Steps[i].Control]
	}
	// VALIDATED BY BEING READ BACK, rather than by a private check. The artifact this
	// returns is therefore one the loader accepts, byte for byte, and a field added here
	// that `Unmarshal` would refuse fails at save time instead of at replay time.
	body, err := a.Marshal()
	if err != nil {
		return workflowstore.Artifact{}, err
	}
	return workflowstore.Unmarshal(body)
}

// The two verbs that are not semantic action kinds. Spelled here because the artifact's
// vocabulary has to be readable in the file, and "" is not a verb.
const (
	editVerb  = "edit"
	focusVerb = "focus"
)

// replayable is the steps of a demonstration that could be done again.
//
// A capture step is dropped rather than refused: reading a value is how a demonstration learned
// something, and not a thing the sequence has to repeat. Everything else that cannot be replayed
// is a refusal, in `refusalFor`, because silently dropping an action from the middle of a
// sequence changes what the sequence does.
func replayable(d *Demonstration) []Step {
	var out []Step
	for _, s := range d.Steps {
		if s.Kind == StepCapture {
			continue
		}
		out = append(out, s)
	}
	return out
}

// RefusalToSave is why this demonstration cannot become a named sequence, or empty.
//
// Separate from FromDemonstration so a caller can ask before offering to save, and so the
// sentence a person reads is written once.
func RefusalToSave(d *Demonstration) string {
	if d == nil {
		return "there is no demonstration to save"
	}
	for _, s := range replayable(d) {
		if why := refusalFor(s); why != "" {
			return fmt.Sprintf("step %d (%s) cannot be done again: %s",
				s.Index, s.Describe(), why)
		}
	}
	return ""
}

// refusalFor is why one step cannot be saved as part of a sequence.
func refusalFor(s Step) string {
	switch {
	case s.Sensitive:
		return "it handled something secret, and a secret must never land in a saved file"
	case s.ValueRef != "":
		return "it used a value the session had captured, which is gone when the " +
			"session ends"
	case !s.Verified:
		return "Marco could not confirm it worked, so there is no evidence that " +
			"doing it again would"
	case s.Clarified:
		return "somebody had to say what it was aimed at, and that answer was about " +
			"that moment rather than about this control"
	case s.Target.Deictic:
		return "it acted on the object you were pointing at, and nothing saved in a " +
			"file can say which object that was tomorrow"
	}
	return ""
}

// controlOf describes the control one step addressed, in the artifact's own vocabulary.
//
// Role first, then the label it bore, then one of the three ways a step addresses a control
// without naming it at all. The ordering is ADR-227's and for its reason: a role is the
// localisable form, a label is what to keep when nothing named the control by meaning, and a
// pixel-only surface supplies only the second.
func controlOf(s Step) workflowstore.Control {
	c := workflowstore.Control{}
	// THE WAY IT IS ADDRESSED, decided before any name is kept — because the name is
	// only evidence for the one way that searches by name.
	//
	// The inline editor is where this matters and where it was wrong first: a rename
	// editor bears the OLD filename, and saving that as the control's text turns "the
	// box this step opened" into "find a control called Alpha.txt" — which, in a details
	// view, finds the Name cell of whatever row still says so.
	switch {
	case s.Target.Deictic:
		c.Deictic = true
		return c
	case s.Target.DerivedEditor:
		c.Editor = true
		return c
	case s.Target.Role != "":
		// A ROLE ALONE. No label beside it: a role is the localisable form, and a
		// saved label next to it is a second answer that can disagree with the first.
		c.Role = string(s.Target.Role)
		return c
	}
	if l := strings.TrimSpace(s.Target.Label); l != "" {
		c.Text = []string{l}
		return c
	}
	// A verb that names no control — confirm, undo — is addressed to the focused
	// context, explicitly. See goal.directiveIntent, which says why leaving it empty is
	// refused downstream as "not specific enough to look for".
	c.Focused = true
	return c
}

// controlKey is what makes two steps' controls the same control.
//
// The addressing, never the position: a sequence that invokes the rename command twice
// addressed one control twice, and two separate entries would halve the evidence for each.
func controlKey(c workflowstore.Control) string {
	return strings.Join([]string{
		c.Role, strings.Join(c.Text, "\x00"),
		fmt.Sprint(c.Deictic), fmt.Sprint(c.Editor), fmt.Sprint(c.Focused),
	}, "|")
}

// quoteAll is the requests, quoted, for the note that records what was asked.
func quoteAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return out
}

// ── replaying one ─────────────────────────────────────────────────────────────

// Directives turns a saved sequence into the steps the Director already knows how to run.
//
//	Replay through existing Director grounding, policy, and Marco execution mechanisms.
//	Reacquire targets from fresh observations.
//
// Which is what a `goal.Directive` is for, so a sequence produces directives and stops. Every
// guarantee downstream of this point — a fresh observation per step, a target re-resolved from
// its query, policy, the task licence, per-step verification, cancellation between steps — is
// the one an ordinary goal expansion gets, because it IS that path. Nothing in this package
// lowers anything or touches a host.
//
// Enforced by TestASavedSequenceReplaysThroughTheOrdinaryProgramPath.
func Directives(a workflowstore.Artifact) ([]goal.Directive, error) {
	byLabel := map[string]workflowstore.Control{}
	for _, c := range a.Controls {
		byLabel[c.Label] = c
	}
	out := make([]goal.Directive, 0, len(a.Steps))
	for _, st := range a.Steps {
		c, ok := byLabel[st.Control]
		if st.Control != "" && !ok {
			return nil, fmt.Errorf("step %d names control %q, which %q does not describe",
				st.Position, st.Control, a.Name)
		}
		if c.Deictic {
			return nil, fmt.Errorf("step %d of %q acted on the object you were "+
				"pointing at, which a saved sequence cannot know", st.Position, a.Name)
		}
		d := goal.Directive{Phrase: st.Says}
		switch st.Verb {
		case "":
			return nil, fmt.Errorf("step %d of %q says nothing to do -- it is a "+
				"recorded transition rather than a demonstrated action",
				st.Position, a.Name)
		case editVerb:
			d.SetText, d.Text = true, st.Value
		case focusVerb:
			d.Focus = true
		default:
			kind := directorapi.SemanticActionKind(st.Verb)
			if !kind.Known() {
				return nil, fmt.Errorf("step %d of %q does %q, which this Director "+
					"has no semantic action for", st.Position, a.Name, st.Verb)
			}
			d.Semantic = kind
		}
		switch {
		case c.Editor:
			d.TargetEditor = true
		case c.Role != "":
			d.Role = goal.ControlRole(c.Role)
		case len(c.Text) > 0:
			// THE LABEL, which is what a pixel-only surface will supply too. Never
			// `Target`: that field means the object the procedure is ABOUT and a step
			// carrying one is bound to it. See ADR-227.
			d.TargetLabel = c.Text[0]
		}
		if d.Phrase == "" {
			d.Phrase = describeStep(st, c)
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%q has no steps to run", a.Name)
	}
	return out, nil
}

// describeStep is the sentence a step gets when the artifact saved none.
func describeStep(st workflowstore.Step, c workflowstore.Control) string {
	what := "the focused control"
	switch {
	case c.Editor:
		what = "the editor it opened"
	case c.Role != "":
		what = goal.ControlRole(c.Role).Describe()
	case len(c.Text) > 0:
		what = fmt.Sprintf("%q", c.Text[0])
	}
	if st.Verb == editVerb {
		return fmt.Sprintf("set %s to %q", what, st.Value)
	}
	return fmt.Sprintf("%s %s", st.Verb, what)
}

// SequenceKind is what a saved sequence's goal is called, and it is deliberately not a goal.
//
// `Kind.Known()` is false for it, so `goal.Expand` refuses it, `goal.Parse` can never produce it
// and no pattern table mentions it. It exists to make the plan line read as a sentence —
// *"run my settings tour"* — and to keep `goal.Goal` a complete value rather than one with a
// hole where its kind should be.
//
//	Named macros can use a separate invocation path, such as "Marco, run my settings tour."
//	Do not add broad goal patterns that hijack existing UI commands.
//
// Which is what a kind nothing parses to is for.
const SequenceKind = goal.Kind("run")

// ExpandSequence turns a saved sequence into a validated program.
//
// The whole of replay. Everything the request path gives an ordinary goal it gives this, because
// after this function returns there is no difference between the two: `goal.ExpandProcedure`
// builds the references, one binding serves the whole expansion, the ordinary program validator
// accepts or refuses it, and `execute` runs it under policy, a task licence, per-step
// verification and cancellation.
//
// What a sequence does NOT get is a procedure registry entry, an application override or a
// phrase. It is addressed by its name and nothing else.
func ExpandSequence(a workflowstore.Artifact, times int, b goal.Binder) (goal.Expansion, error) {
	one, err := Directives(a)
	if err != nil {
		return goal.Expansion{}, err
	}
	if times < 1 {
		times = 1
	}
	if times > MostRunsOfASequence {
		return goal.Expansion{}, fmt.Errorf(
			"%d runs of %q is more than the %d this Director will do in one request",
			times, a.Name, MostRunsOfASequence)
	}
	// THE REPEAT IS THE STEPS AGAIN, not a loop.
	//
	// Which means the bound is the ordinary step ceiling, the plan a person reads shows
	// every step that will happen, validation covers the whole thing before the first
	// one, and cancellation between runs is the cancellation between steps that already
	// works. A loop here would have needed its own bound, its own report and its own
	// cancellation check, and all three exist.
	//
	// What it costs, stated: a failure in the second run stops the third, because every
	// step carries FailurePolicy.Stop. That is the right default for a routine somebody
	// asked for three times and the wrong one for a routine that is idempotent, and
	// nothing yet distinguishes them.
	ds := make([]goal.Directive, 0, len(one)*times)
	for run := 0; run < times; run++ {
		for _, d := range one {
			if times > 1 && d.Phrase != "" {
				d.Phrase = fmt.Sprintf("%s (run %d of %d)", d.Phrase, run+1, times)
			}
			ds = append(ds, d)
		}
	}
	proc := goal.Procedure{
		Name:   fmt.Sprintf("saved sequence %q", a.Name),
		Goal:   SequenceKind,
		Safety: sequenceSafety(a),
		// LEARNED, because it was: the flag is what makes a plan say where its steps
		// came from, and a person about to watch Marco act on their desktop is entitled
		// to see that nobody wrote these steps by hand.
		Learned: true,
		Why: fmt.Sprintf("saved from a demonstration on %s",
			a.SavedAt.Format("2006-01-02")),
		Steps: func(goal.Goal) ([]goal.Directive, error) { return ds, nil },
	}
	g := goal.Goal{
		Kind:    SequenceKind,
		Phrase:  fmt.Sprintf("run %s", a.Name),
		Context: goal.Context{Target: a.Name},
	}
	return goal.ExpandProcedure(proc, g, b)
}

// sequenceSafety is what a saved sequence declares it will do.
//
// Counted from the artifact rather than carried in it, for the reason every other derived
// judgement here is derived: a stored number can disagree with the steps beside it, and this one
// decides whether a person is asked before anything happens.
func sequenceSafety(a workflowstore.Artifact) goal.Safety {
	mutations := 0
	for _, st := range a.Steps {
		if st.Verb == editVerb || st.Verb == string(directorapi.SemanticConfirm) {
			mutations++
		}
	}
	s := goal.Safety{Mutations: mutations, Risk: directorapi.RiskLow}
	if mutations > 0 {
		s.Risk = directorapi.RiskMedium
	}
	return s
}
