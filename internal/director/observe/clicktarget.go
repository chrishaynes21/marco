package observe

// THE LEARNING UNIT OF A MOUSE DEMONSTRATION, which the owner named exactly:
//
//	Source screen → Visual target → Destination screen
//
//	An observed, attributable click transition — not simply two different screenshots
//	separated by a mouse event.
//
// # What this is for, and what it refuses to do
//
// A press has a position and a frame; [[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]]
// made both reach a replay. On 900 frames of a real demonstration every one of seventeen presses
// landed inside a region the proposer had found, and five of them landed inside MORE than one.
// That is where a mouse demonstration can quietly go wrong: containment says a press was inside
// a rectangle, and it does not say that rectangle is the control that answered.
//
// The owner drew the line:
//
//	For the five ambiguous clicks, I'd preserve multiple possible region matches until visual
//	or transition evidence can disambiguate them. Choosing the smallest containing region
//	might be a useful hypothesis, but it shouldn't silently become a verified target identity.
//
// So every containing region is kept, smallest first, and the smallest is a RANKED HYPOTHESIS
// carried beside the others rather than a decision taken on the reader's behalf.
//
// # Correlation, and the word is chosen
//
// A transition is attributed to the most recent press that could plausibly have caused it. That
// is the same discipline [[ADR-158-the-active-list-is-the-one-that-responds]] applies to a mark
// that moved: the input labels a change the SCREEN already proved, it is never allowed to
// manufacture one, and a press with no transition after it and a transition with no press before
// it are both recorded as themselves rather than quietly dropped.
//
// Session-local, like the screens it refers to. Nothing here may become a durable Place or a
// durable target. See arrangement.go.

import (
	"fmt"
	"sort"
)

// TransitionCause is WHY a screen changed, as far as the evidence goes.
//
// # Three kinds, and the third is the important one
//
// The owner's decision, after the first mouse demonstration exposed the gap:
//
//	I would allow the graph to represent both kinds of transitions explicitly.
//
//	  Click      a grounded target was clicked and a screen change followed
//	  Automatic  screen changed without a newly attributable input
//	  Unknown    evidence insufficient to establish the cause
//
//	The third category is especially important. Marco should be able to learn that
//	something happened without pretending to know why.
//
// Rocket League forced it. Entering Free Play is a LOAD completing: the person clicks, a
// loading screen appears, and the destination arrives with no further input. A planner that
// assumed every edge needs an action would try to click its way through that — and would be
// wrong about ordinary applications, not just games.
//
// But an unprompted change is not automatically a legitimate navigation either. An animation, a
// splash and a segmentation artefact all look the same from here, which is why the third kind
// exists rather than everything unattributed being called automatic.
type TransitionCause string

const (
	// CauseClick is a change that followed a press on a grounded target.
	CauseClick TransitionCause = "click"
	// CauseAutomatic is a change with no attributable press that followed Marco's own
	// ARRIVAL by click — the load-after-a-click shape, where waiting is the right
	// response and clicking again is not.
	CauseAutomatic TransitionCause = "automatic"
	// CauseUnknown is a change with no press and no arrival to hang it on.
	//
	// Not a failure and not a lesser automatic: it is the honest answer, and it is the
	// one that must never become an executable step. An animation, a splash screen and a
	// segmentation artefact are all in here, and nothing in this layer can tell them
	// apart.
	CauseUnknown TransitionCause = "unknown"
)

// THREE DIFFERENT QUESTIONS, AND THEY MUST NOT COLLAPSE INTO ONE.
//
// The first version of this had a single `Executable()`, and the owner caught what that name
// would eventually license:
//
//	I would avoid equating `Executable() == false` with "this step cannot participate in
//	execution". For example, a future route may legitimately be: Click Play → Wait for
//	loading → Verify Training. The wait is a valid operation in an executable plan, even
//	though the automatic transition itself has no executable click.
//
//	If `Executable()` specifically means "may produce an input action", the current design is
//	sensible. If it means "may appear in a runnable plan", it will eventually be too
//	restrictive.
//
// It meant the first. The name invited the second, and a name that invites the wrong reading is
// a defect waiting for a reader in a hurry. So the three questions are three predicates:
//
//	AuthorizesInput   may this step produce a press?
//	Plannable         may this step appear in a route at all?
//	Verifiable        can arriving be confirmed by looking, without acting?

// AuthorizesInput reports whether this cause could ever license a press.
//
// Only a click, and only ever after the target is re-grounded on the screen in front of Marco.
// A statement about the EVIDENCE and not a permission: nothing in this package can act.
func (c TransitionCause) AuthorizesInput() bool { return c == CauseClick }

// Plannable reports whether a step with this cause may appear in a route.
//
// A click and an automatic transition both can: one is "press this", the other is "wait for
// this", and `Click Play → Wait for loading → Verify Training` needs both. An unknown cause
// cannot, because there is no operation to put in the plan — not pressing, since nobody pressed,
// and not waiting, since nothing says it will arrive.
//
// A route that needs an unknown step is a route Marco cannot walk, and saying so is the point.
func (c TransitionCause) Plannable() bool {
	return c == CauseClick || c == CauseAutomatic
}

// Verifiable reports whether arriving can be confirmed by observation alone.
//
// Always. Every step names a destination screen, and a destination is something Marco can look
// at — including an unknown one, which is why an unknown transition is still worth recording:
// Marco cannot plan to cause it and can perfectly well notice that it happened.

// MaxArrivalToAutomaticLooks is how soon after arriving somewhere a further change may still be
// called automatic.
//
// The same bound a press gets, deliberately: it is the same question asked one step along — how
// long a consequence may take before it stops being a consequence — and a second number would
// be a second rule to keep in step with the first. The gap is reported per occurrence, so an
// automatic transition that took four seconds is visible as one.
const MaxArrivalToAutomaticLooks = MaxPressToTransitionLooks

// VisualTargetID is a session-local identity for a control somebody clicked.
type VisualTargetID string

// MaxPressToTransitionLooks is how long after a press a transition may still be attributed to
// it.
//
// # Measured rather than chosen
//
// The press at frame 138 of `rl-freeplay-1` is followed by its transition at look 168 — thirty
// looks, three seconds at that recording's rate. A ten-look window reported that press as having
// changed nothing, which is how the limit was found. Fifty is generous against the thirty that
// was measured, and the gap is REPORTED per attribution so an implausible one is visible rather
// than hidden inside the bound.
//
// Being generous is the safer error here only because of the rule beside it: a transition is
// attributed to the MOST RECENT qualifying press, so a long window cannot credit an old press
// for something a newer one did. What it can still do is credit a press for a change the game
// made by itself, which is why the gap is on the report.
const MaxPressToTransitionLooks = 50

// MaxClickCandidates bounds how many containing regions one press keeps.
//
// Smallest first, so the bound drops the largest — an enclosing panel, the window itself —
// which are the candidates least likely to be the control that answered. A real frame proposes
// over two hundred regions and a press inside a deeply nested layout could contain many.
const MaxClickCandidates = 8

// ClickObservation is one press and everything known about it at the time.
type ClickObservation struct {
	// Look is the look the press was folded on, and At where it landed.
	Look int       `json:"look"`
	At   PointerAt `json:"at"`
	// Screen is the screen that was standing when the press was made. The SOURCE of the
	// learning unit, and unknown is a real answer.
	Screen VisualScreenID `json:"screen"`
	// Candidates is every proposed region that contained the press, SMALLEST FIRST and
	// bounded — each with the cue that proposed it.
	//
	// All of them on purpose. The smallest containing region is the live pointer path's
	// rule and a good hypothesis; it is not evidence, and a press that kept only its
	// favourite would have thrown away the thing that could later contradict it.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Target is the identity of the SMALLEST candidate, which is the hypothesis. Empty
	// when the press landed on nothing the proposer found.
	Target VisualTargetID `json:"target,omitempty"`
	// Ambiguous says the press was inside more than one region, so the target above is one
	// of several readings rather than the only one.
	Ambiguous bool `json:"ambiguous,omitempty"`
}

// Landed reports whether the press was inside anything at all.
func (c ClickObservation) Landed() bool { return len(c.Candidates) > 0 }

// ClickTransition is the learning unit: a source screen, a clicked target, a destination.
type ClickTransition struct {
	From   VisualScreenID `json:"from"`
	Target VisualTargetID `json:"target"`
	To     VisualScreenID `json:"to"`
	Count  int            `json:"count"`
	// Looks is the look each occurrence's PRESS happened on, in order.
	Looks []int `json:"looks,omitempty"`
	// Gaps is how many looks each occurrence took between the press and the destination
	// settling. Reported because the attribution window is generous and a long gap is the
	// shape of a coincidence.
	Gaps []int `json:"gaps,omitempty"`
	// Ambiguous counts the occurrences whose target was one of several candidates.
	Ambiguous int `json:"ambiguous,omitempty"`
}

// RouteStep is one observed change with its cause and its evidence.
//
// One type for all three causes rather than a list per cause, because the thing that reads this
// has to walk a route through them: a step whose cause is automatic sits between two steps whose
// cause is a click, and splitting them into separate collections makes the caller reassemble an
// order the evidence already has.
type RouteStep struct {
	From  VisualScreenID  `json:"from"`
	To    VisualScreenID  `json:"to"`
	Cause TransitionCause `json:"cause"`
	// Target is the control that was pressed, and it is EMPTY for every cause but a
	// click. An automatic or unknown step with a target would be an invitation to press
	// something nobody pressed.
	Target VisualTargetID `json:"target,omitempty"`
	Count  int            `json:"count"`
	// Looks is when each occurrence happened — the press for a click, the change itself
	// otherwise — and Gaps how long each took.
	Looks []int `json:"looks,omitempty"`
	Gaps  []int `json:"gaps,omitempty"`
	// Ambiguous counts the occurrences whose target was one reading of several.
	Ambiguous int `json:"ambiguous,omitempty"`
	// Because says why this cause was assigned, in words, so a reader does not have to
	// re-derive it from the numbers.
	Because string `json:"because,omitempty"`
}

// ClickEvidence is everything the session can say about its presses and changes.
type ClickEvidence struct {
	// Steps is every observed change with its cause, most repeated first.
	//
	// The three causes together, because a route walks through all of them. Attributed
	// below is the click-caused subset, kept because the learning unit is a thing in its
	// own right and most of what reads this wants only those.
	Steps []RouteStep `json:"steps,omitempty"`
	// Attributed is the learning units, most repeated first.
	Attributed []ClickTransition `json:"attributed,omitempty"`
	// Inert is presses that were followed by no screen change inside the window. A real
	// outcome: a click on a disabled control, on nothing, or on a thing that opened
	// something the proposer could not see.
	Inert []ClickObservation `json:"inert,omitempty"`
	// Unattributed is screen changes with no qualifying press before them — a timer, a
	// load finishing, an animation settling.
	//
	// Reported beside the attributed ones rather than omitted, for the reason ADR-158's
	// own unattributed count is: a change that usually has no input before it is evidence
	// AGAINST reading the correlated ones as causes.
	Unattributed []VisualTransition `json:"unattributed,omitempty"`
	// Presses is every press the correlator was given, whatever became of it.
	//
	// The total the other counts are subsets of. A report with four subsets and no total
	// cannot tell "nothing was attributed" from "nothing arrived", and those send a
	// reader to opposite ends of the system — one is a correlation question and the other
	// is a question about whether anything is watching the mouse at all.
	Presses int `json:"presses,omitempty"`
	// Blind is presses that landed on nothing the proposer found. They can never be
	// re-grounded, so a route cannot be built over them however reliable the transition.
	Blind int `json:"blind,omitempty"`
	// AmbiguousPresses is presses inside more than one region.
	AmbiguousPresses int `json:"ambiguous_presses,omitempty"`
	// MaybeOneControl is pairs of target identities on one screen that were followed by
	// the same destination.
	//
	// The disambiguation evidence, put in front of somebody rather than acted on. Two
	// identities that lead to the same screen are the shape of one control the proposer
	// read two ways — and they are also the shape of two genuinely different entries that
	// happen to open the same thing, which is why this is a report and not a merge.
	MaybeOneControl []TargetPair `json:"maybe_one_control,omitempty"`
}

// TargetPair is two identities on one screen that led to the same destination.
type TargetPair struct {
	Screen VisualScreenID `json:"screen"`
	A      VisualTargetID `json:"a"`
	B      VisualTargetID `json:"b"`
	To     VisualScreenID `json:"to"`
}

// MaxTargetRegions bounds how many rectangles one target identity remembers having been.
//
// Generous, because every one of them is a way a later press can recognise this control, and
// bounded because a session of five hundred clicks on one button must not accumulate five
// hundred rectangles.
const MaxTargetRegions = 16

// visualTarget is one identity and every rectangle it has been clicked as.
type visualTarget struct {
	id     VisualTargetID
	screen VisualScreenID
	// where is the smallest candidate this target was FIRST clicked as. One reading.
	//
	// It was written to accumulate — "every rectangle it was found as is another way a
	// later press can recognise it" — and the accumulation could never fire. `identify`
	// is given the smallest candidate alone, a single rectangle; a later press reaches
	// the loop only by AGREEING with a stored reading, and the loop appended only
	// readings that DISAGREED with all of them. So the condition that admitted the press
	// and the condition that would have stored it were each other's negation, and
	// `MaxTargetRegions` on this field was unreachable too.
	//
	// Deleted rather than excused, which is this repository's rule for an unreachable
	// guard. The tolerance that actually makes a control survive a proposer reading it
	// slightly differently is `sameRect` — mutual centre containment — and that needs one
	// reading, not a collection. A slice because the JSON shape is published and because
	// an accumulation that is REACHED is a change somebody should make deliberately,
	// after measuring what merging does to the identity count.
	// [[ADR-180-finding-a-demonstrated-target-again]]
	where []Region
	seen  int
	// containers is every LARGER region that also contained one of this target's
	// presses, accumulated across them.
	//
	// Not part of the identity — `identify` is given the smallest candidate alone and
	// that rule is measured, so this records beside it rather than changing it. Kept
	// because the control/container ambiguity is what makes a click target hard, and
	// because reacquisition has to be able to say "the panel is still here and the
	// button is not" instead of matching the panel.
	containers []Region
	// ambiguous is how many of its presses landed inside more than one region, and
	// firstLook/lastLook when it was clicked.
	//
	// The looks are what let a reacquisition be scored somewhere other than the
	// demonstration itself: the owner's standard is that a click's own observation does
	// not count as having found the target again.
	ambiguous           int
	firstLook, lastLook int
	// said is what this control's own rectangle has been read to say, over every look
	// of the screen it is on. See controllabel.go.
	said controlWords
}

// ClickTargets correlates presses with the screen changes that followed them.
//
// Zero value is ready. Folded on one goroutine, like everything else here.
type ClickTargets struct {
	targets []visualTarget
	next    int

	// pending is the presses seen since the last screen change, newest last.
	pending []ClickObservation

	attributed   []ClickTransition
	inert        []ClickObservation
	unattributed []VisualTransition
	steps        []RouteStep
	// presses is every press this correlator was given, before any judgement about it.
	//
	// The denominator. Blind, ambiguous, inert and attributed are all subsets of it, and
	// without it a report has four subsets and no total — which is ADR-169's defect
	// exactly: a record that cannot explain its own number. "No clicks were attributed"
	// has to be distinguishable from "no clicks arrived", and only a total does that.
	presses   int
	blind     int
	ambiguous int

	// arrivedAt is the look Marco last arrived somewhere BY A CLICK, and arrivedWhere
	// the screen it arrived at.
	//
	// What separates automatic from unknown. A change out of a screen Marco clicked its
	// way into, soon after, is the load-after-a-click shape; a change out of a screen it
	// has merely been sitting on is something else, and this layer cannot say what.
	arrivedAt    int
	arrivedWhere VisualScreenID
	hasArrived   bool

	// recent is the last positioned readings, with the screen each was taken on.
	//
	// Held because a press is folded before the rectangle it establishes exists, so the
	// readings that name a control are usually the ones taken BEFORE it was clicked.
	// See MaxRecentReadings.
	recent []screenReading
}

// screenReading is one look's positioned reading, and the screen it was taken on.
type screenReading struct {
	screen VisualScreenID
	words  []ReadWord
}

// Reading folds one look's positioned reading against every control known on this screen.
//
// Called once a look by the observer, before the presses. What it does is accumulate what each
// control's OWN rectangle says over the looks of its screen — see controllabel.go for why one
// look is the wrong number to ask, and why `Locates` is what keeps a full-window hypothesis from
// learning the whole screen's vocabulary as a control's name.
//
// Nothing here moves a target, mints one, or changes which rectangle a press identified. A label
// is evidence about a control that already exists.
func (c *ClickTargets) Reading(screen VisualScreenID, read []ReadWord) {
	if len(read) == 0 || screen == "" {
		return
	}
	for i := range c.targets {
		t := &c.targets[i]
		if t.screen != screen || len(t.where) == 0 {
			continue
		}
		t.said.saw(LabelOf(t.where[0], read))
	}
	c.recent = append(c.recent, screenReading{screen: screen,
		words: append([]ReadWord{}, read...)})
	if len(c.recent) > MaxRecentReadings {
		c.recent = c.recent[len(c.recent)-MaxRecentReadings:]
	}
}

// nameFromRecent folds the readings already taken of this screen into a control just created.
func (c *ClickTargets) nameFromRecent(t *visualTarget) {
	if len(t.where) == 0 {
		return
	}
	for _, r := range c.recent {
		if r.screen != t.screen {
			continue
		}
		t.said.saw(LabelOf(t.where[0], r.words))
	}
}

// Press folds one press: what it landed on, and which target that is.
//
// `cands` is the whole look's proposals — the pixel proposer's, plus any slow detections the
// observer admitted for this look. Containment is asked here rather than by the caller so that
// the ranking and the identity are decided in one place: a caller that picked a favourite
// first would hand this a decision and call it an observation.
//
// THE RANKING RULE IS UNCHANGED, AND THAT IS THE POINT. The smallest containing rectangle
// wins on area, with no reference to which cue proposed it. A detector candidate is not
// preferred for being semantic and not penalised for being late; it competes on the one
// property this layer is willing to judge. Experiment-030 is what makes that sufficient: the
// detected TRAINING tile is under half the area of the stadium slab that had been winning,
// so proposing it is enough and the slab never has to be suppressed.
func (c *ClickTargets) Press(look int, at PointerAt, screen VisualScreenID,
	cands []Candidate) ClickObservation {

	c.presses++
	obs := ClickObservation{Look: look, At: at, Screen: screen}
	for _, cand := range cands {
		if !containsPoint(cand.Region, at) {
			continue
		}
		obs.Candidates = append(obs.Candidates, cand)
	}
	// SMALLEST FIRST. The most specific thing containing a press is the likeliest control,
	// which is the same reasoning the live pointer path applies — and ties break on
	// position so two equal rectangles order the same way in two runs of one recording.
	sort.SliceStable(obs.Candidates, func(i, j int) bool {
		ai := obs.Candidates[i].Width * obs.Candidates[i].Height
		aj := obs.Candidates[j].Width * obs.Candidates[j].Height
		if ai != aj {
			return ai < aj
		}
		if obs.Candidates[i].Y != obs.Candidates[j].Y {
			return obs.Candidates[i].Y < obs.Candidates[j].Y
		}
		return obs.Candidates[i].X < obs.Candidates[j].X
	})
	if len(obs.Candidates) > MaxClickCandidates {
		obs.Candidates = obs.Candidates[:MaxClickCandidates]
	}
	switch {
	case len(obs.Candidates) == 0:
		c.blind++
	default:
		obs.Ambiguous = len(obs.Candidates) > 1
		if obs.Ambiguous {
			c.ambiguous++
		}
		// THE SMALLEST CANDIDATE, which is the stated hypothesis and nothing more.
		// The rest of the set survives on the observation; see identify for the four
		// rules measured and why this is the one that does not resolve anything.
		// GEOMETRIC, DELIBERATELY. identify is handed rectangles and never learns
		// which cue proposed them, so a target means the same thing whether the
		// detector happened to have answered yet.
		obs.Target = c.identify(screen, Regions(obs.Candidates[:1]))
		c.recordTarget(obs)
	}
	c.pending = append(c.pending, obs)
	return obs
}

// identify gives a clicked region a session-local identity, reusing one when the same control
// on the same screen has been clicked before.
//
// # Four rules were measured on rl-freeplay-1 and this is the one that resolves nothing
//
// | identity from | identities | units repeated across both demonstrations |
// |---|---|---|
// | the smallest candidate (this) | 12 | 3 |
// | the whole candidate set | 8 | 4 — and `target_1` merged three different menu entries |
// | the candidate set, matched pairs limited to 4x in area | 8 | 4 — same merge; two distant presses' near-full-screen candidates are comparable to EACH OTHER |
// | the candidates within 4x of the press's own smallest | 13 | 2 — split worse than the smallest alone |
//
// So geometry alone does not settle it. The candidate set recovers one repeated unit and pays
// for it by merging controls a person can see are different; restricting the scale either way
// makes the split worse. **Nothing here chooses**, which is what the owner asked for:
//
//	Choosing the smallest containing region might be a useful hypothesis, but it shouldn't
//	silently become a verified target identity. […] For the five ambiguous clicks, I'd
//	preserve multiple possible region matches until visual or transition evidence can
//	disambiguate them.
//
// The identity is therefore the stated hypothesis — the smallest containing region — and the
// pairs that might be one control are REPORTED as `ClickEvidence.MaybeOneControl` rather than
// merged. Transition evidence is what would settle them, and this is where that evidence is put
// in front of somebody.
//
// Scoped to the screen, because "the third row" means nothing across two different menus, and
// the same place on two screens is two different controls.
func (c *ClickTargets) identify(screen VisualScreenID, cands []Region) VisualTargetID {
	for i := range c.targets {
		t := &c.targets[i]
		if t.screen != screen || !sameControl(t.where, cands) {
			continue
		}
		t.seen++
		return t.id
	}
	c.next++
	id := VisualTargetID(fmt.Sprintf("target_%d", c.next))
	c.targets = append(c.targets, visualTarget{
		id: id, screen: screen, where: append([]Region{}, cands...), seen: 1,
	})
	// AND WHAT THE LOOKS ALREADY TAKEN SAID ABOUT IT. The rectangle did not exist
	// until now, so the readings that name it are the ones before the press.
	c.nameFromRecent(&c.targets[len(c.targets)-1])
	return id
}

// recordTarget files one press's alternatives and provenance against its target.
//
// Separate from identify on purpose. The identity rule was measured four ways on a real
// demonstration and the one in the tree is the one that resolves nothing
// ([[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]]); widening what it is
// given would change a measured decision as a side effect of wanting more bookkeeping.
func (c *ClickTargets) recordTarget(obs ClickObservation) {
	for i := range c.targets {
		t := &c.targets[i]
		if t.id != obs.Target {
			continue
		}
		if obs.Ambiguous {
			t.ambiguous++
		}
		if t.firstLook == 0 {
			t.firstLook = obs.Look
		}
		t.lastLook = obs.Look
		// A RECTANGLE A PRESS IDENTIFIED THE CONTROL AS IS NOT A CONTAINER OF IT.
		//
		// The smallest containing region is the HYPOTHESIS — the thing this
		// press is taken to have hit. A rectangle that has been some press's
		// hypothesis cannot also be something the control merely sits inside:
		// the evidence says a person clicked it AS the control.
		//
		// Measured, and it is the whole of `target_6` on `rl-freeplay-1`. The
		// person clicked EXIT TO MAIN MENU twice. The first press was inside
		// two rectangles — a 170x14 run of the label and the 352x34 button row
		// around it — so the run became the identity and the row became a
		// container. The second press was inside ONE rectangle, the row, which
		// is this control's own hypothesis read at a different extent. Mutual
		// centre containment holds between them, so every later proposal of
		// that row agreed with the identity AND with the container, and the
		// exclusion refused the control on 63 of 96 looks of its own screen:
		// the one rectangle that IS the button was the one rectangle barred
		// from matching it.
		//
		// This is a DROP and not a refusal to file, and the difference is the
		// hazard ADR-180 found. A row sitting at its panel's centre is the
		// same rectangle as the panel, so declining to FILE a container that
		// agrees with this press's own hypothesis would let a look offering
		// only the panel match, and press its middle. One press therefore
		// drops nothing; it takes a SECOND press, landing on the larger
		// rectangle as its own smallest, to establish that somebody clicks
		// that rectangle.
		// [[ADR-182-two-controls-that-could-never-be-found-again]]
		kept := t.containers[:0]
		for _, r := range t.containers {
			if sameRect(r, obs.Candidates[0].Region) {
				continue
			}
			kept = append(kept, r)
		}
		t.containers = kept
		// THE CONTAINERS, which are the candidates that are not the control's own
		// reading. Bounded like `where`, because a session of five hundred clicks
		// must not accumulate five hundred rectangles.
		for _, cand := range obs.Candidates[1:] {
			if len(t.containers) >= MaxTargetRegions {
				break
			}
			if !sameControl(t.containers, []Region{cand.Region}) {
				t.containers = append(t.containers, cand.Region)
			}
		}
		return
	}
}

// sameControl reports whether any rectangle in one set is the same rectangle as any in the
// other.
//
// "The same rectangle" is mutual centre containment: each contains the other's centre. That is
// symmetric, it needs no ratio anybody has to justify, and it is robust to the thing that
// actually varies — a proposer that found the same row slightly larger or smaller on a later
// frame. An overlap FRACTION would have been a number invented here, and this subsystem has a
// measured history of invented thresholds being wrong.
func sameControl(a, b []Region) bool {
	for _, x := range a {
		for _, y := range b {
			if sameRect(x, y) {
				return true
			}
		}
	}
	return false
}

// SameControl reports whether two rectangles are two readings of one control.
//
// The exported statement of the rule re-acquisition has always used, so a caller outside this
// package asks the same question rather than inventing a second one. See `sameRect`.
func SameControl(a, b Region) bool { return sameRect(a, b) }

// sameRect reports whether two rectangles are two readings of one control.
//
// Mutual centre containment: each contains the other's centre. Symmetric, threshold-free, and
// robust to the thing that actually varies — a proposer that found the same row slightly larger
// or smaller on a later frame. An overlap FRACTION would have been a number invented here, and
// this subsystem has a measured history of invented thresholds being wrong.
func sameRect(a, b Region) bool {
	return containsCentre(a, b) && containsCentre(b, a)
}

// Arrived folds one settled screen change and attributes it, or records that it could not.
//
// Attribution is to the MOST RECENT pending press that was made on the screen being left and is
// inside the window. Most recent, because an older press cannot have caused what a newer one is
// sitting in front of; on the screen being left, because a press made somewhere else is about
// something else; and bounded, because a press four seconds earlier explains nothing.
//
// Every pending press that is not the one attributed is resolved here too — as INERT if it was
// left behind by this change, so a press that did nothing is a recorded outcome and not a
// silence.
// It RETURNS the step it recorded, because the caller cannot work it out afterwards: two steps
// can share a source and a destination and differ only in cause — `visual_3 → visual_4` was
// measured as a click on one occasion and automatic on the other, when the load crossed the
// attribution window one time and not the other. A caller that looked the step up by its
// endpoints would publish the wrong cause for one of them, and the cause is the field that
// decides whether anything may ever be pressed.
func (c *ClickTargets) Arrived(look int, t VisualTransition) RouteStep {
	best := -1
	for i := range c.pending {
		p := c.pending[i]
		if p.Screen != t.From || !p.Landed() {
			continue
		}
		gap := look - p.Look
		if gap < 0 || gap > MaxPressToTransitionLooks {
			continue
		}
		best = i
	}
	if best < 0 {
		// NOBODY PRESSED ANYTHING THIS COULD BE. Recorded as itself: a change that
		// usually has no input before it is evidence against reading the correlated
		// ones as causes.
		c.unattributed = append(c.unattributed, t)
		// AUTOMATIC, OR UNKNOWN. The distinction is whether Marco got here by
		// clicking: a change out of a screen it clicked its way into, soon after, is
		// a load completing, and the right response to one is to WAIT. A change out of
		// a screen it has merely been sitting on could be an animation, a splash or a
		// segmentation artefact, and nothing here can tell those apart.
		cause, why := CauseUnknown, "no press before it, and Marco did not click its "+
			"way to this screen either"
		gap := 0
		if c.hasArrived && c.arrivedWhere == t.From {
			if g := look - c.arrivedAt; g >= 0 && g <= MaxArrivalToAutomaticLooks {
				cause, gap = CauseAutomatic, g
				why = "no press before it, and it followed an arrival by click: " +
					"the shape of a load completing, so the response is to " +
					"wait and verify rather than to press anything"
			}
		}
		st := c.step(t.From, t.To, cause, "", look, gap, false, why)
		c.resolve(nil)
		return st
	}
	p := c.pending[best]
	c.note(p, t.To, look-p.Look)
	st := c.step(p.Screen, t.To, CauseClick, p.Target, p.Look, look-p.Look, p.Ambiguous,
		"a press on a grounded target was followed by this change")
	// ARRIVED BY CLICKING, which is what lets the next change be called automatic.
	c.arrivedAt, c.arrivedWhere, c.hasArrived = look, t.To, true
	c.resolve(&best)
	return st
}

// resolve clears the pending presses, filing everything that was not attributed.
func (c *ClickTargets) resolve(attributed *int) {
	for i := range c.pending {
		if attributed != nil && i == *attributed {
			continue
		}
		if !c.pending[i].Landed() {
			// A press on nothing is already counted in Blind. Filing it as inert as
			// well would count one press twice in a report whose whole purpose is
			// to add up.
			continue
		}
		c.inert = append(c.inert, c.pending[i])
	}
	c.pending = c.pending[:0]
}

// step records one occurrence of a route step, by cause.
// step records one occurrence and returns THIS occurrence, not the accumulated row.
//
// This occurrence, because a caller publishing a change wants what just happened: the row's
// count and its list of looks are the session's history, and handing those to somebody who
// asked "what did that click do" would answer a different question.
func (c *ClickTargets) step(from, to VisualScreenID, cause TransitionCause,
	target VisualTargetID, look, gap int, ambiguous bool, why string) RouteStep {

	this := RouteStep{
		From: from, To: to, Cause: cause, Target: target, Count: 1,
		Looks: []int{look}, Gaps: []int{gap}, Because: why,
	}
	if ambiguous {
		this.Ambiguous = 1
	}
	for i := range c.steps {
		x := &c.steps[i]
		if x.From != from || x.To != to || x.Cause != cause || x.Target != target {
			continue
		}
		x.Count++
		x.Looks = append(x.Looks, look)
		x.Gaps = append(x.Gaps, gap)
		if ambiguous {
			x.Ambiguous++
		}
		return this
	}
	c.steps = append(c.steps, this)
	return this
}

// note records one occurrence of a learning unit.
func (c *ClickTargets) note(p ClickObservation, to VisualScreenID, gap int) {
	for i := range c.attributed {
		a := &c.attributed[i]
		if a.From != p.Screen || a.Target != p.Target || a.To != to {
			continue
		}
		a.Count++
		a.Looks = append(a.Looks, p.Look)
		a.Gaps = append(a.Gaps, gap)
		if p.Ambiguous {
			a.Ambiguous++
		}
		return
	}
	t := ClickTransition{
		From: p.Screen, Target: p.Target, To: to, Count: 1,
		Looks: []int{p.Look}, Gaps: []int{gap},
	}
	if p.Ambiguous {
		t.Ambiguous = 1
	}
	c.attributed = append(c.attributed, t)
}

// Close files the presses still pending at the end of a session.
//
// A demonstration's last press is the one that opened the thing being demonstrated, and a
// session that ended before the screen settled would otherwise lose it entirely.
func (c *ClickTargets) Close() { c.resolve(nil) }

// Evidence is everything the session can say, most repeated first.
func (c *ClickTargets) Evidence() ClickEvidence {
	out := ClickEvidence{
		Steps:            make([]RouteStep, len(c.steps)),
		Attributed:       make([]ClickTransition, len(c.attributed)),
		Inert:            make([]ClickObservation, len(c.inert)),
		Unattributed:     make([]VisualTransition, len(c.unattributed)),
		Presses:          c.presses,
		Blind:            c.blind,
		AmbiguousPresses: c.ambiguous,
	}
	copy(out.Steps, c.steps)
	copy(out.Attributed, c.attributed)
	sort.SliceStable(out.Steps, func(i, j int) bool {
		if out.Steps[i].Count != out.Steps[j].Count {
			return out.Steps[i].Count > out.Steps[j].Count
		}
		if out.Steps[i].From != out.Steps[j].From {
			return out.Steps[i].From < out.Steps[j].From
		}
		return out.Steps[i].To < out.Steps[j].To
	})
	copy(out.Inert, c.inert)
	copy(out.Unattributed, c.unattributed)
	// THE PAIRS THAT MIGHT BE ONE CONTROL. Derived from the transitions rather than
	// stored, because it is a reading OF the evidence and must not be able to drift from
	// it — and because it is the one thing here that would be wrong to act on silently.
	for i := range c.attributed {
		for j := i + 1; j < len(c.attributed); j++ {
			a, b := c.attributed[i], c.attributed[j]
			// No Target test: two entries sharing From, Target and To cannot both
			// exist, because note() merges them into one occurrence count. A guard
			// against it would read as a case somebody handled.
			if a.From != b.From || a.To != b.To {
				continue
			}
			out.MaybeOneControl = append(out.MaybeOneControl, TargetPair{
				Screen: a.From, A: a.Target, B: b.Target, To: a.To,
			})
		}
	}
	sort.SliceStable(out.Attributed, func(i, j int) bool {
		if out.Attributed[i].Count != out.Attributed[j].Count {
			return out.Attributed[i].Count > out.Attributed[j].Count
		}
		if out.Attributed[i].From != out.Attributed[j].From {
			return out.Attributed[i].From < out.Attributed[j].From
		}
		return out.Attributed[i].Target < out.Attributed[j].Target
	})
	return out
}

// Targets is every identity minted, with the screen it belongs to and how often it was clicked.
type TargetSeen struct {
	ID     VisualTargetID `json:"id"`
	Screen VisualScreenID `json:"screen"`
	// Where is the rectangle this control was clicked as, which is what a later look has
	// to find again to re-ground it.
	//
	// One reading, always. It was documented as a collection and the code that would have
	// grown it was unreachable; see visualTarget.where. What lets a second demonstration
	// recognise the first one's click is `sameRect`'s tolerance, not a list.
	Where  []Region `json:"where"`
	Clicks int      `json:"clicks"`
}

// Targets is every target identity this session minted.
func (c *ClickTargets) Targets() []TargetSeen {
	out := make([]TargetSeen, 0, len(c.targets))
	for _, t := range c.targets {
		out = append(out, TargetSeen{
			ID: t.id, Screen: t.screen, Clicks: t.seen,
			Where: append([]Region{}, t.where...),
		})
	}
	return out
}

// containsPoint reports whether a region contains a normalised point.
//
// Half-open on the right and bottom, the same convention windowBounds.place uses, so a press on
// the boundary between two stacked rows belongs to one of them and not to both.
func containsPoint(r Region, at PointerAt) bool {
	if r.Width <= 0 || r.Height <= 0 {
		return false
	}
	return at.X >= r.X && at.Y >= r.Y && at.X < r.X+r.Width && at.Y < r.Y+r.Height
}

// containsCentre reports whether a contains b's centre.
func containsCentre(a, b Region) bool {
	return containsPoint(a, PointerAt{X: b.X + b.Width/2, Y: b.Y + b.Height/2})
}

// Verifiable is true for every cause. See the note above.
func (c TransitionCause) Verifiable() bool { return c != "" }
