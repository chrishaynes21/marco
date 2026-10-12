package observe

// THREE KINDS OF IDENTITY, AND THIS FILE IS THE MIDDLE ONE.
//
// The owner drew the distinction after the first mouse demonstration, and it is the right one:
//
//	1. a visual ARRANGEMENT    frame-local: where rectangles were. Changes with animations,
//	                           hover, and detector noise.
//	2. a session-local SCREEN  a temporally stabilised claim that several frames are looks at
//	                           one interface state.                            <- here
//	3. a durable PLACE         a persistent identity backed by stronger evidence, for planning
//	                           and verification across sessions.
//
// # Why the middle one has to exist separately
//
// [[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] measured what an unnamed
// arrangement can see on 900 real frames: seven distinguishable screens where Marco's own model
// had one, with the demonstrated route visible in both runs — and **150 changes between adjacent
// frames**. The separation is real and the raw representation is unusable as it stands. One
// spurious transition every six frames is not a weaker route; it is a transition storm, and a
// transition storm looks like evidence.
//
// So an arrangement is an observation and a screen is an interpretation, and the gap between
// them is time.
//
// # What this file must never do
//
// Become durable. `VisualScreenID` is minted by a counter in one session and means nothing in
// the next, exactly as `ScreenStateID` does — but a screen state is a composition of ROLES a
// provider vouched for, and this is a count of rectangles in nine boxes. That is a far weaker
// thing to make permanent, and the owner was explicit:
//
//	Raw rectangle arrangements should not automatically become durable Places. […] Do not
//	promote geometry-only screen hypotheses into durable cross-session Places yet.
//
// Enforced by `TestAVisualScreenCannotReachDurableMemory`, which holds these names to this file
// inside `internal/`. An offline replay may use them freely; the live path may not, until that
// is a decision somebody takes on purpose.

import (
	"fmt"
	"sort"
	"time"
)

// VisualScreenID is a session-local screen hypothesis. Never durable. See the file note.
type VisualScreenID string

// VisualScreenUnobserved is the answer for a look that saw no structure at all.
//
// NOT a screen. A frame with no rectangles in it is a failure to observe — a repaint, a fade, a
// detector that found nothing — and treating it as a destination would have Marco record a
// transition INTO an empty place and out of it again, twice per animation. On the measured
// recording exactly that happened: a two-frame cluster holding zero regions, which the raw
// diagnostic made its own screen because two empty signatures are a perfect match.
const VisualScreenUnobserved VisualScreenID = "visual_unobserved"

// VisualScreenUnknown is the answer once the absence has persisted.
//
// The other half of the rule above. A single blank look must not drop the standing claim; a
// long run of them must, because by then Marco genuinely does not know what is in front of it
// and saying otherwise is the kind of confident staleness this subsystem exists to avoid.
const VisualScreenUnknown VisualScreenID = "visual_unknown"

const (
	// VisualSettleLooks is how many consecutive looks an arrangement must hold before it
	// replaces the standing screen.
	//
	// Three, and the number is the whole mechanism. The measured recording changed
	// arrangement 150 times in 900 frames while holding seven distinguishable screens, so
	// most of those changes are one or two frames long — a hover highlight, a repaint, a
	// proposal the detector found once. A change that survives three consecutive looks at
	// 10fps is 300ms of the screen genuinely looking different.
	//
	// It is deliberately NOT StatePromotionCount (2). That number governs how often an
	// ambiguous ROLE composition must recur before it is trusted, on a 3.8-second
	// discovery cadence; this governs consecutive frames at a video rate. Sharing a
	// constant between two different clocks would be a coincidence pretending to be a
	// rule.
	VisualSettleLooks = 3

	// VisualScreenMatch is the overlap at which a look joins a known visual screen.
	//
	// The same 0.55 the segmenter uses for role compositions, on purpose and as a
	// STARTING POINT rather than a conclusion: ADR-170's measurement was taken at that
	// threshold, so keeping it means the stabilised numbers can be compared against the
	// raw ones without a second variable moving. Whether it is the right number for an
	// arrangement is its own measurement.
	VisualScreenMatch = StateMatchSimilarity

	// VisualUnobservableLooks is how long a run of blank looks may be before the standing
	// screen is given up.
	//
	// Longer than the settle bound because the two failures are not symmetric: calling a
	// fade a new screen manufactures evidence, and holding a stale claim through one only
	// delays a correction.
	VisualUnobservableLooks = 10

	// MaxVisualTransitionLooks bounds the per-transition record of when it happened.
	//
	// The occurrences are what a repeated route is read from, so a few are worth keeping
	// and a session's worth is not: an unbounded list grows with the recording and nothing
	// reads past the first several.
	MaxVisualTransitionLooks = 64
)

// VisualArrangement is one look's structure, with no names in it.
//
// Cells are the segmenter's own coarse grid — see cellKey — deliberately, so a stabilised
// measurement can be compared against ADR-170's raw one without the quantisation moving at the
// same time. Whether 3x3 counts are the right representation is an open question that file
// records; this type exists so that question can be asked once rather than in two places.
type VisualArrangement struct {
	Cells map[string]int `json:"cells,omitempty"`
	Total int            `json:"total"`
}

// Observed reports whether this look saw any structure at all.
func (a VisualArrangement) Observed() bool { return a.Total > 0 }

// ArrangeLook summarises a look's regions as an arrangement.
//
// Regions only. No role, no label, no track identity, and nothing that could carry a name into
// an identity — the same exclusions NewScreenSignature makes, for the same reason, minus the one
// thing it requires and this deliberately does without.
func ArrangeLook(regions []Region) VisualArrangement {
	a := VisualArrangement{Cells: map[string]int{}}
	for _, r := range regions {
		if r.Width <= 0 || r.Height <= 0 {
			// A region with no area sits nowhere. Placing it at the origin would pile
			// every unmeasured proposal into one cell and distort the arrangement,
			// which is the same reason NewScreenSignature places only what has a
			// rectangle.
			continue
		}
		a.Total++
		a.Cells[cellKey(arrangementCell, r)]++
	}
	boundKeys(a.Cells, MaxSignatureKeys)
	return a
}

// arrangementCell is the one token every cell key carries.
//
// One token for everything, so the only discriminator is WHERE the rectangles were. A token
// that varied with a region's size or position would make this a richer cue than the proposer
// can offer, and a measurement arranged to come out well is worse than none.
const arrangementCell = "r"

// features flattens an arrangement for comparison.
func (a VisualArrangement) features() map[string]float64 {
	f := make(map[string]float64, len(a.Cells))
	for k, v := range a.Cells {
		f["cell:"+k] = float64(v)
	}
	return f
}

// ArrangementSimilarity compares two arrangements, using the segmenter's own arithmetic.
//
// The SAME function the screen model compares compositions with, so there is one statement of
// what "alike" means in this package rather than two that can drift. Exported for the
// measurements that characterise it.
//
// An unobserved arrangement is never alike anything, INCLUDING another unobserved one. That is
// the one place this departs from signatureSimilarity, which answers 1 for two empty vectors
// because two screens with no structural evidence really are the same kind of screen — true for
// a role composition on a sparse game screen, and false here, where emptiness means the
// proposer found nothing and the honest answer is that nothing is known.
func ArrangementSimilarity(a, b VisualArrangement) float64 {
	if !a.Observed() || !b.Observed() {
		return 0
	}
	return signatureSimilarity(a.features(), b.features())
}

// VisualTransition is one observed change between two settled screens.
type VisualTransition struct {
	From  VisualScreenID `json:"from"`
	To    VisualScreenID `json:"to"`
	Count int            `json:"count"`
	// Looks is the look index of each occurrence, in order and bounded.
	//
	// What a REPEATED route is read from: two demonstrations of one navigation produce the
	// same pair twice, far apart, and a count alone cannot say whether it was twice or one
	// flap counted twice.
	Looks []int `json:"looks,omitempty"`
	// Unsettled is how many looks the change took to settle, summed over occurrences. A
	// transition that always takes three looks is a repaint boundary; one that takes
	// twelve is a screen that fades.
	Unsettled int `json:"unsettled,omitempty"`
}

// VisualStability is what the stabiliser did, and the numbers that say whether to trust it.
type VisualStability struct {
	// Looks is every look offered, Settled those that agreed with the standing screen,
	// and Unsettled those that differed and did not become a transition.
	//
	// Unsettled is the number ADR-170 measured as 150 raw adjacent-frame changes. It is
	// reported rather than hidden because it is the cost of the damping: every one of them
	// is a look Marco declined to call a new screen, and if the real navigation is in
	// there, this is where it was lost.
	Looks, Settled, Unsettled int
	// Warming is looks taken before ANY screen had settled, which are not flaps.
	//
	// Separated because conflating them makes the damping look worse than it is: the
	// opening looks of a session differ from a standing screen that does not exist yet,
	// and counting them as changes that failed to settle would charge the stabiliser for
	// the one thing it could not have done differently.
	Warming int
	// Unobserved is looks that saw no structure, and Unobservable the times a run of them
	// went on long enough to give up the standing screen.
	Unobserved, Unobservable int
	// Promotions is how many times a new arrangement replaced the standing screen, which
	// is the transition count including the first settle.
	Promotions int
}

// visualScreen is one minted screen and the arrangement that minted it.
type visualScreen struct {
	id VisualScreenID
	// sig is the arrangement as first settled, and it does not drift.
	//
	// A representative that moved with every matching look would let a screen walk away
	// from itself over a long session — the same slow-drift failure a tracked list has
	// when its identity is recomputed from its current extent.
	sig   VisualArrangement
	looks int
	first int
}

// VisualScreens stabilises arrangements into session-local screen hypotheses.
//
// Zero value is ready. Not safe for concurrent use; it is folded on the same goroutine as the
// look it describes, like everything else in this package.
type VisualScreens struct {
	screens []visualScreen
	next    int

	// held is the standing claim and heldSig the arrangement it was minted from.
	held    VisualScreenID
	heldSig VisualArrangement

	// pending is an arrangement trying to replace the standing one, and run how many
	// consecutive looks have agreed with it.
	pending    VisualArrangement
	pendingRun int
	// pendingFrom is the look the run started at, so a promotion can say how long it took.
	pendingFrom int

	blankRun int
	look     int

	transitions []VisualTransition
	stats       VisualStability

	// promoted is the change the look just folded caused, if it caused one.
	//
	// Reported rather than returned from Observe so the caller can ask about it without
	// every caller having to take a second value it mostly ignores — and because the
	// thing a correlation needs is the PAIR, which the returned id alone cannot give.
	promoted   VisualTransition
	didPromote bool
}

// Observe folds one look and returns what screen it is a look AT.
//
// The returned id is the standing claim, which is the point: a look that differs from it by one
// frame of hover does not get its own answer, and the caller cannot accidentally read a flap as
// a navigation. `at` is accepted and deliberately unused for now — the settle rule counts LOOKS
// rather than time, because a replay at a stride and a live session at a cadence would otherwise
// damp differently, and the one thing this must not do is behave differently in the measurement
// than in production.
func (v *VisualScreens) Observe(a VisualArrangement, at time.Time) VisualScreenID {
	_ = at
	v.look++
	v.stats.Looks++
	v.didPromote = false

	if !a.Observed() {
		// A MISSING OBSERVATION, NOT A SCREEN. The standing claim survives it, and
		// nothing records a transition, because there is nothing to transition to.
		v.stats.Unobserved++
		v.blankRun++
		// A run of them does eventually mean Marco has lost the screen. Giving up is
		// not a transition either: an unknown destination is not a place.
		if v.blankRun >= VisualUnobservableLooks && v.held != "" {
			v.stats.Unobservable++
			v.held, v.heldSig = "", VisualArrangement{}
			v.pending, v.pendingRun = VisualArrangement{}, 0
		}
		if v.held == "" {
			return VisualScreenUnknown
		}
		return VisualScreenUnobserved
	}
	v.blankRun = 0

	// THE STANDING SCREEN FIRST. A look that agrees with it is the ordinary case and must
	// not disturb a pending run — a hover that flickers on and off would otherwise keep
	// resetting the count and never settle either way.
	if v.held != "" && ArrangementSimilarity(v.heldSig, a) >= VisualScreenMatch {
		v.stats.Settled++
		v.pending, v.pendingRun = VisualArrangement{}, 0
		v.touch(v.held)
		return v.held
	}

	// A DIFFERENT ARRANGEMENT. It has to hold for VisualSettleLooks consecutive looks
	// before it is allowed to be a screen, and each of those looks has to agree with the
	// arrangement that STARTED the run rather than merely differ from the standing one —
	// three different flickers in a row are not one new screen.
	if v.pendingRun > 0 && ArrangementSimilarity(v.pending, a) >= VisualScreenMatch {
		v.pendingRun++
	} else {
		v.pending, v.pendingRun, v.pendingFrom = a, 1, v.look
	}
	if v.pendingRun < VisualSettleLooks {
		// A LOOK THAT DIFFERED FROM A SCREEN, or a look taken before there was one.
		// The second is not a flap — see VisualStability.Warming.
		if v.held == "" {
			v.stats.Warming++
			return VisualScreenUnknown
		}
		v.stats.Unsettled++
		return v.held
	}
	return v.promote(a)
}

// promote makes the pending arrangement the standing screen, reusing an identity when it is one
// Marco has settled on before.
//
// Reuse is what makes a route possible: walking back out of a submenu has to arrive at the
// screen it left, or every demonstration is a chain of new places and no repetition is visible.
func (v *VisualScreens) promote(a VisualArrangement) VisualScreenID {
	id := VisualScreenID("")
	best, bestSim := -1, 0.0
	for i := range v.screens {
		if sim := ArrangementSimilarity(v.screens[i].sig, a); sim > bestSim {
			best, bestSim = i, sim
		}
	}
	if best >= 0 && bestSim >= VisualScreenMatch {
		id = v.screens[best].id
	} else {
		v.next++
		id = VisualScreenID(fmt.Sprintf("visual_%d", v.next))
		// MINTED FROM THE ARRANGEMENT THAT SETTLED, not from the first look of the
		// run: the run's opening look may have caught a half-painted screen, and the
		// one that completed it is the one three looks agreed about.
		v.screens = append(v.screens, visualScreen{id: id, sig: a, first: v.look})
	}
	from := v.held
	v.held, v.heldSig = id, v.screenSig(id)
	v.pending, v.pendingRun = VisualArrangement{}, 0
	v.stats.Promotions++
	v.touch(id)
	if from != "" && from != id {
		v.note(from, id, v.look-v.pendingFrom)
		v.promoted = VisualTransition{From: from, To: id, Count: 1,
			Looks: []int{v.look}, Unsettled: v.look - v.pendingFrom}
		v.didPromote = true
	}
	return id
}

// screenSig is the stored arrangement for an id.
func (v *VisualScreens) screenSig(id VisualScreenID) VisualArrangement {
	for i := range v.screens {
		if v.screens[i].id == id {
			return v.screens[i].sig
		}
	}
	return VisualArrangement{}
}

// touch counts a look against a screen.
func (v *VisualScreens) touch(id VisualScreenID) {
	for i := range v.screens {
		if v.screens[i].id == id {
			v.screens[i].looks++
			return
		}
	}
}

// note records one occurrence of a change between two settled screens.
func (v *VisualScreens) note(from, to VisualScreenID, unsettled int) {
	for i := range v.transitions {
		t := &v.transitions[i]
		if t.From != from || t.To != to {
			continue
		}
		t.Count++
		t.Unsettled += unsettled
		if len(t.Looks) < MaxVisualTransitionLooks {
			t.Looks = append(t.Looks, v.look)
		}
		return
	}
	v.transitions = append(v.transitions, VisualTransition{
		From: from, To: to, Count: 1, Looks: []int{v.look}, Unsettled: unsettled,
	})
}

// Promoted reports the change the last folded look caused, if it caused one.
//
// Only a change BETWEEN two screens. Settling on the first screen of a session is a promotion
// and not a transition — there is nothing it came from, and a pair with an invented origin is
// the manufactured edge the blank rule exists to refuse.
func (v *VisualScreens) Promoted() (VisualTransition, bool) {
	return v.promoted, v.didPromote
}

// Screen is the standing claim, or unknown when there is none.
func (v *VisualScreens) Screen() VisualScreenID {
	if v.held == "" {
		return VisualScreenUnknown
	}
	return v.held
}

// Transitions is every observed change between settled screens, most repeated first.
//
// Sorted by count and then by name, so a reader looking for the repeated navigation finds it at
// the top and two runs of one session produce the same listing.
func (v *VisualScreens) Transitions() []VisualTransition {
	out := make([]VisualTransition, len(v.transitions))
	copy(out, v.transitions)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// Stability is what the stabiliser did. See VisualStability.
func (v *VisualScreens) Stability() VisualStability { return v.stats }

// VisualScreenSeen is one settled screen and how much of the session it accounted for.
type VisualScreenSeen struct {
	ID    VisualScreenID `json:"id"`
	Looks int            `json:"looks"`
	First int            `json:"first"`
	Total int            `json:"regions"`
	Cells int            `json:"cells"`
}

// Seen is every screen that settled, in the order they were first settled on.
func (v *VisualScreens) Seen() []VisualScreenSeen {
	out := make([]VisualScreenSeen, 0, len(v.screens))
	for _, s := range v.screens {
		out = append(out, VisualScreenSeen{
			ID: s.id, Looks: s.looks, First: s.first,
			Total: s.sig.Total, Cells: len(s.sig.Cells),
		})
	}
	return out
}
