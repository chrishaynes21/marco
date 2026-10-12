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
	// bounded.
	//
	// All of them on purpose. The smallest containing region is the live pointer path's
	// rule and a good hypothesis; it is not evidence, and a press that kept only its
	// favourite would have thrown away the thing that could later contradict it.
	Candidates []Region `json:"candidates,omitempty"`
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

// ClickEvidence is everything the session can say about its presses and changes.
type ClickEvidence struct {
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
	// where is every SMALLEST candidate this target has been clicked as.
	//
	// Accumulated, because the proposer reads one control two ways on two frames and each
	// reading is another way a later press can recognise it. Only the smallest, for the
	// reason in identify.
	where []Region
	seen  int
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
	blind        int
	ambiguous    int
}

// Press folds one press: what it landed on, and which target that is.
//
// `regions` is the whole look's proposals. Containment is asked here rather than by the caller
// so that the ranking and the identity are decided in one place — a caller that picked a
// favourite first would hand this a decision and call it an observation.
func (c *ClickTargets) Press(look int, at PointerAt, screen VisualScreenID,
	regions []Region) ClickObservation {

	obs := ClickObservation{Look: look, At: at, Screen: screen}
	for _, r := range regions {
		if !containsPoint(r, at) {
			continue
		}
		obs.Candidates = append(obs.Candidates, r)
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
		obs.Target = c.identify(screen, obs.Candidates[:1])
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
		// WHAT ELSE THIS CONTROL LOOKS LIKE. Every new rectangle it was found as is
		// another way a later press can recognise it, which is how a target survives
		// a proposer that reads the same button two ways.
		for _, r := range cands {
			if len(t.where) >= MaxTargetRegions {
				break
			}
			if !sameControl(t.where, []Region{r}) {
				t.where = append(t.where, r)
			}
		}
		return t.id
	}
	c.next++
	id := VisualTargetID(fmt.Sprintf("target_%d", c.next))
	keep := cands
	if len(keep) > MaxTargetRegions {
		keep = keep[:MaxTargetRegions]
	}
	c.targets = append(c.targets, visualTarget{
		id: id, screen: screen, where: append([]Region{}, keep...), seen: 1,
	})
	return id
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
func (c *ClickTargets) Arrived(look int, t VisualTransition) {
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
		c.resolve(nil)
		return
	}
	p := c.pending[best]
	c.note(p, t.To, look-p.Look)
	c.resolve(&best)
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
		Attributed:       make([]ClickTransition, len(c.attributed)),
		Inert:            make([]ClickObservation, len(c.inert)),
		Unattributed:     make([]VisualTransition, len(c.unattributed)),
		Blind:            c.blind,
		AmbiguousPresses: c.ambiguous,
	}
	copy(out.Attributed, c.attributed)
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
	// Where is every rectangle this control has been clicked as, which is what a later
	// look would have to find again to re-ground it. More than one is not a defect: it is
	// the proposer reading one control two ways, and keeping them is what let the second
	// demonstration recognise the first one's click.
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
