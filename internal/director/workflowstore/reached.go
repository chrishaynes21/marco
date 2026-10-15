package workflowstore

// Did the workflow arrive where its step was going, on the evidence of THIS look?
//
// # Why there is a second establishment path at all
//
// `Infer` answers the same question and [[Experiment-048-the-destination-a-workflow-can-see-for-itself]]
// measured why it cannot be asked when it matters. Two gates stood in front of it, and both were
// the global screen model answering a workflow's question:
//
//   - the grounder returns early while the step's SOURCE is still established, and a binding is
//     live for exactly one observation generation — which moves only when the global observer
//     records a screen CHANGE. Two pages inside the arrangement measure's own resolution merge,
//     no change is recorded, and so the question "is this the destination?" is never put. In the
//     live run of [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] it was not
//     put for sixty observations, while Marco stood on the page it was waiting for.
//   - and what it scored was `obs.Arrangement(standing)`, a signature fixed when that screen was
//     minted and never updated afterwards. Standing on the Training page under the Play page's
//     identity, the saved destination was being compared against the PLAY page's arrangement.
//
// So this exists to ask the question from the look in front of the planner. It is deliberately a
// separate function from `Infer` rather than a loosening of it: `Infer` follows a screen CHANGE
// the observer recorded and is the stronger claim for that reason, and a build that merged the
// two would lose the distinction in the one place a report has to keep it.
//
// # What the evidence has to be
//
// Three terms, measured over the 222 folds of the recorded navigation and a held-out capture:
//
//	the destination agrees with THIS LOOK at VisualScreenMatch   the existing threshold, unmoved
//	and agrees with it better than the SOURCE does               which alone cuts the Play-page
//	                                                             false arrivals from 10 of 34 to 2
//	and a fresh admissible reading does not contradict it        which takes every false arrival
//	                                                             in 222 folds to zero
//
// The third is not optional and the measurement is the argument: the arrangement alone would
// have established the Training page on 10 of 34 looks of the Play page and 21 of 28 looks of
// the Free Play submenu, because a submenu replaces one panel of the page behind it. It is also
// not sufficient alone — on the main menu a reading AGREES with the saved Play page eight times
// in ten, because that page's whole saved vocabulary is four words the menu also says. The two
// halves fail in opposite places, which is why both are required.
//
// One term that was measured and REJECTED: requiring the reading to also contradict the SOURCE.
// It is perfect on the Play → Training step (10 of 10) and destroys the Main Menu → Play step,
// where it drops arrivals from 11 readings in 12 to one.
//
// # And what it still is not
//
// It establishes a binding, which lives in one session and one observation, and it is NOT
// independent: the expectation came from the workflow whose step is being checked, exactly as
// for an inference. `Arrival.Verified()` is still false and still says why. Nothing here
// reaches the global screen model — `internal/director/observe` does not import this package —
// no arrangement is stored, no vocabulary is folded, and no identity is minted. The fact that
// an action was SENT is not consulted anywhere below.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// screenNamed is the saved screen carrying one label.
func (a Artifact) screenNamed(label string) (Screen, bool) {
	for _, s := range a.Screens {
		if s.Label == label {
			return s, true
		}
	}
	return Screen{}, false
}

// Reached establishes a step's DESTINATION from the current look's own evidence, where the
// global observer has recorded no change to follow.
//
// `look` is the arrangement of the look in front of the caller — `observe.ArrangeLook` of its
// own regions, never a stored signature — and `read` is a reading already admitted for this
// observation by `observe.Current`. Both are the caller's to supply because this package cannot
// see, and the admission rule stays stated once, where the readings arrive.
func (s *SessionBinding) Reached(a Artifact, from, to string, screen observe.VisualScreenID,
	look observe.VisualArrangement, read observe.ScreenText,
	epoch, generation uint64) (Bound, error) {

	if screen == "" {
		return Bound{}, fmt.Errorf("workflowstore: no screen is standing, so there is " +
			"nothing an arrival could be about")
	}
	// ALREADY SETTLED IN THIS OBSERVATION. Not re-decided: a confirmation somebody gave
	// is the stronger evidence and this must not overwrite it.
	if b, ok := s.Established(to, epoch, generation); ok {
		return b, fmt.Errorf("workflowstore: %s is already established as %s (%s) in "+
			"this observation", to, b.Screen, b.Ground)
	}
	anchor, ok := s.bound[from]
	if !ok {
		return Bound{}, fmt.Errorf("workflowstore: nothing is established about %s, so "+
			"there is no step to have arrived from", from)
	}
	// THE ANCHOR MUST BE FROM THIS WINDOW, the same requirement `Infer` makes and for the
	// same reason: a chain built on a binding made under another window is inference from
	// nothing.
	if anchor.Generation != generation {
		return Bound{}, fmt.Errorf("workflowstore: %s was established under another "+
			"window", from)
	}
	if !a.leadsTo(from, to) {
		return Bound{}, fmt.Errorf("workflowstore: %q does not demonstrate a step from "+
			"%s to %s, so there is nothing to have arrived at", a.Name, from, to)
	}
	dst, ok := a.screenNamed(to)
	if !ok || !dst.Arrangement.Observed() {
		return Bound{}, fmt.Errorf("workflowstore: %s carries no arrangement, so there "+
			"is nothing to recognise it by", to)
	}
	src, ok := a.screenNamed(from)
	if !ok || !src.Arrangement.Observed() {
		return Bound{}, fmt.Errorf("workflowstore: %s carries no arrangement, so there "+
			"is nothing to tell %s apart from", from, to)
	}
	if !look.Observed() {
		return Bound{}, fmt.Errorf("workflowstore: this look has no arrangement, so it " +
			"is not evidence about anything")
	}

	// ── 1. THE DESTINATION AGREES WITH THIS LOOK ──
	toward := observe.ArrangementSimilarity(dst.Arrangement, look)
	away := observe.ArrangementSimilarity(src.Arrangement, look)
	if toward < observe.VisualScreenMatch {
		return Bound{}, fmt.Errorf("workflowstore: this look agrees with %s at %.3f, "+
			"below %.2f; the workflow expected to arrive here and the evidence "+
			"does not agree, which is a reason to wait rather than to assume",
			to, toward, observe.VisualScreenMatch)
	}
	// ── 2. AND BETTER THAN THE SCREEN IT CAME FROM ──
	//
	// The contrast the global observer cannot use: it compares one arrangement against a
	// standing claim, where a workflow names both ends of the step and can ask which of
	// the two this look is closer to.
	if toward <= away {
		return Bound{}, fmt.Errorf("workflowstore: this look agrees with %s at %.3f and "+
			"with %s at %.3f, so it is no more the destination than the screen "+
			"the step started on", to, toward, from, away)
	}
	// ── 3. AND A FRESH READING DOES NOT CONTRADICT IT ──
	//
	// Required, because the arrangement alone was measured to establish this destination on
	// 10 of 34 looks of the page the step started on and 21 of 28 looks of the page after
	// it. A destination nobody read cannot be contradicted, so a saved screen with no words
	// is refused here rather than treated as agreeing: that is `Missing evidence is not
	// contradictory evidence` read the other way round — missing evidence is not
	// corroboration either.
	said := dst.Said()
	if !said.Said() {
		return Bound{}, fmt.Errorf("workflowstore: %s was saved without any words, so a "+
			"reading cannot corroborate it and the arrangement alone has been "+
			"measured to arrive on the wrong page", to)
	}
	if !read.Said() {
		return Bound{}, fmt.Errorf("workflowstore: this look agrees with %s at %.3f and "+
			"nothing has been read on it yet, so there is no corroboration for an "+
			"arrival", to, toward)
	}
	if apart, why := observe.TextsDisagree(said, read); apart {
		return Bound{}, fmt.Errorf("workflowstore: this look agrees with %s at %.3f by "+
			"arrangement and what was read on it says otherwise: %s", to, toward, why)
	}

	// POSITIVELY IDENTIFIED, OR MERELY NOT CONTRADICTED. Two different strengths of the
	// same term and the report keeps them apart.
	//
	// `TextsDisagree` abstains for several honest reasons — a reading that resolved fewer
	// labels, or either side carrying fewer than `TextDisagreementWords` words — and a
	// destination saved with two words can therefore never be contradicted at all. So the
	// positive statement is asked separately: does this reading still say enough of what
	// that screen was read to say to BE it, which is `TextKept`, the same share the
	// disagreement rule uses as its floor.
	kept, _ := observe.TextOverlap(said, read)
	positive := kept >= observe.TextKept
	thin := len(said.Words) < observe.TextDisagreementWords

	because := fmt.Sprintf("the workflow goes %s -> %s, %s was established (%s, depth "+
		"%d), this look agrees with %s at %.3f against %.3f for %s, and what was "+
		"read on it keeps %.0f%% of what %s was read to say",
		from, to, from, anchor.Ground, anchor.Depth, to, toward, away, from,
		kept*100, to)
	switch {
	case !positive:
		because += fmt.Sprintf(" — below the %.0f%% that would identify it, so the "+
			"reading only fails to contradict it", observe.TextKept*100)
	case thin:
		because += fmt.Sprintf(" — on %d saved word(s), fewer than the %d it takes to "+
			"contradict anything, so this reading could not have refused it",
			len(said.Words), observe.TextDisagreementWords)
	}
	b := Bound{
		Label: to, Screen: screen, Ground: GroundReached,
		Epoch: epoch, Generation: generation, Depth: anchor.Depth + 1,
		Positive: positive, Thin: thin, Because: because,
	}
	s.bound[to] = b
	return b, nil
}
