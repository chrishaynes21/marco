package workflowstore

// ONE STEP OF A DEMONSTRATION, ON ITS OWN.
//
// # Why this exists
//
// `perform-workflow` starts at step 0 and walks forward. Ark's capture learned three steps, and
// step 0 is the pause-menu press whose target ABSTAINED —
// [[Experiment-085-the-graphics-tab-is-a-button]] measured it as a 73 × 12 rectangle with no
// word inside it, nearest label `RESUME` 0.052 of the window away. Running the whole artifact
// to reach step 2 means pressing that first, which is the last thing a first supervised action
// should do.
//
// So a single demonstrated action is extracted into a workflow of one step. The owner's
// constraint, and it is the whole difficulty:
//
//	Do not simply drop step zero and accidentally retain the wrong initial-state assumption.
//
// A step's own `From` is the screen the run must be standing on. Dropping the steps before it
// does not change that — but it does change which screens the artifact needs to carry, and an
// extraction that kept all of them would describe a route through screens no step visits, while
// one that kept too few would name a screen the file cannot describe. Both are caught by
// `Marshal`, which refuses a step naming a screen the artifact does not have.
//
// # What travels, and what deliberately does not
//
//	the step                  with Position 0, because it is the only one
//	its two screens           with their arrangement and whatever was read on them, which is
//	                          the pre-state evidence the step needs to be grounded at all
//	its control               every reading and container, which is the action-site evidence
//	the outcome expectation   on the step, which is how it is finished
//	the application scope     unchanged: an extraction is the same demonstration, narrowed
//	the SavedAt instant       unchanged, which keeps the raw demonstration findable beside it
//	clarifications about that control, and nothing said about any other
//
// NOT a press coordinate: there was never one in an artifact and this does not add one.
// NOT the other steps' controls, screens or answers.
// NOT a new format — the result is an ordinary v3 artifact that `Unmarshal` loads.

import "fmt"

// JustStep extracts one step into a workflow of its own.
//
// The name is kept and marked, because two files under one name is how somebody runs the wrong
// one: a person reading `director sequences` has to be able to see which is the whole routine
// and which is one action out of it.
func (a Artifact) JustStep(step int) (Artifact, error) {
	if step < 0 || step >= len(a.Steps) {
		return Artifact{}, fmt.Errorf("workflowstore: step %d is not in %q, which has "+
			"%d", step, a.Name, len(a.Steps))
	}
	st := a.Steps[step]
	out := Artifact{
		Version: Version,
		// THE NAME SAYS WHAT IT IS. A one-step extraction and the routine it came
		// from are different things to perform, and a store keyed on names cannot
		// tell them apart for you.
		Name: fmt.Sprintf("%s (step %d)", a.Name, step),
		// THE SAME INSTANT, so `demo.Store.Evidence` still finds the raw
		// demonstration this was extracted from. An extraction is not a new
		// demonstration and must not claim to be one.
		SavedAt: a.SavedAt,
		// AND THE SAME APPLICATION. Narrowing a routine cannot widen where it may
		// act, and an extraction that lost its scope would be UNSCOPED — readable
		// and runnable nowhere, which is a confusing way to fail.
		Application: a.Application,
		Notes: append([]string{fmt.Sprintf("step %d of %q, extracted so one "+
			"demonstrated action can be performed on its own; the steps "+
			"before it are not in this file and are not implied by it",
			step, a.Name)}, a.Notes...),
	}
	// ── THE SCREENS THIS STEP NAMES, AND ONLY THOSE ──
	//
	// Both ends, even when they are the same label — which is the panel case, where a
	// step's two ends are one screen. Their arrangement and text are the pre-state
	// evidence: without the `From` screen's evidence nothing can establish that the run
	// is in the right place, and the step would be unplannable rather than unsafe.
	for _, label := range []string{st.From, st.To} {
		if label == "" || hasScreen(out.Screens, label) {
			continue
		}
		sc, ok := a.screenNamed(label)
		if !ok {
			return Artifact{}, fmt.Errorf("workflowstore: step %d of %q goes "+
				"%s -> %s and %q is not a screen this artifact describes, so "+
				"the extraction would have no evidence for it",
				step, a.Name, st.From, st.To, label)
		}
		out.Screens = append(out.Screens, sc)
	}
	// ── ITS CONTROL, WITH EVERY READING AND CONTAINER ──
	if st.Control != "" {
		c, ok := a.controlNamed(st.Control)
		if !ok {
			return Artifact{}, fmt.Errorf("workflowstore: step %d of %q presses "+
				"%q and no such control is described, so there would be "+
				"nothing to look for", step, a.Name, st.Control)
		}
		out.Controls = append(out.Controls, c)
	}
	// ── THE STEP, RENUMBERED, WITH ITS OUTCOME AND ITS UNRESOLVED QUESTIONS ──
	one := st
	one.Position = 0
	out.Steps = []Step{one}
	// ── AND WHAT ANYBODY SAID ABOUT THAT CONTROL, which is evidence about it rather
	// than about the routine, so it narrows with the control.
	for _, ans := range a.Clarifications {
		if ans.Control == st.Control {
			out.Clarifications = append(out.Clarifications, ans)
		}
	}
	return out, nil
}

// hasScreen reports whether a label is already carried.
func hasScreen(screens []Screen, label string) bool {
	for _, s := range screens {
		if s.Label == label {
			return true
		}
	}
	return false
}
