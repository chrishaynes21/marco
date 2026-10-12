package main

// WHAT AN UNNAMED-GEOMETRY CUE WOULD SEGMENT. A measurement, not a rule.
//
// # The question this exists to answer
//
// On the pixels discovery path Marco's screen model is one vacuous state. `ScreenSignature` is
// built from NAMED roles and the structure proposer names nothing, so every signature is empty
// — and `signatureSimilarity` answers 1 for two empty vectors on purpose, because "two screens
// with no structural evidence are the same kind of screen" is the right answer for sparse
// gameplay. Four menus therefore read as four looks at `state_1`.
//
// That is the gap in the way of click-target navigation: "which clicked target produced which
// screen transition" cannot be answered while every screen is one state. Closing it means
// letting unnamed geometry into a signature, and the subsystem note is explicit that this is a
// decision of its own rather than a widening, because a signature is what reaches durable
// cross-session memory.
//
// So: measure first. This runs the PRODUCTION signature and the PRODUCTION similarity over the
// same frames, with one difference — every proposal carries a generic role instead of none —
// and reports how many screens that would have produced on a real recording. Nothing here
// changes what Marco believes, and nothing here is wired into the live path.
//
// # Why production's own arithmetic rather than a measurement of its own
//
// Because a characterisation whose numbers drifted from the thing it characterises is worse
// than none, which is why `SignatureSimilarity` is exported in the first place. The clustering
// below is greedy best-match at `StateMatchSimilarity`, which is the rule the segmenter applies
// when deciding whether an inference joins a known state. It deliberately omits the promotion
// count: this asks what the signature can SEE, and promotion is about how long to wait before
// trusting it.

import (
	"fmt"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// pixelRole is the stand-in kind every proposal is given for this measurement.
//
// One token for everything, so the only thing distinguishing two signatures is ARRANGEMENT —
// how many proposals sit in each cell of the segmenter's 3x3 grid. That is the weakest possible
// version of the cue and therefore the honest one to measure: anything a richer role vocabulary
// could add is upside on top of this number.
const pixelRole = "pixels"

// pixelSignature is what this frame's proposals would look like to the segmenter if unnamed
// geometry counted.
func pixelSignature(shapes []observe.ShadowShape) observe.ScreenSignature {
	regs := make([]observe.ShadowRegion, 0, len(shapes))
	for _, sh := range shapes {
		regs = append(regs, observe.ShadowRegion{
			Role:   pixelRole,
			Region: sh.Region,
			// Pixel-only because that is what the proposer produces. It is INERT
			// here — a frame whose every region comes from one source takes the
			// signature's "pixels are the only account" branch either way — and it
			// is set so the measurement is shaped like what production would be
			// handed, rather than like something arranged to come out well.
			Kind: directorapi.KindPixelOnly,
		})
	}
	return observe.NewScreenSignature(regs)
}

// screenCluster is one distinct arrangement and how many frames joined it.
type screenCluster struct {
	sig    observe.ScreenSignature
	frames int
	// first is the frame index that opened it, so a reader can line the clusters up
	// against a route they remember walking.
	first int
}

// clusterScreens assigns each frame to a cluster, greedily, at the segmenter's own threshold.
//
// First best match wins and ties go to the lowest index, so the result does not depend on map
// iteration — the same property the segmenter's own sums are arranged to have.
func clusterScreens(sigs []observe.ScreenSignature) ([]int, []screenCluster) {
	per := make([]int, len(sigs))
	var clusters []screenCluster
	for i, sig := range sigs {
		best, bestSim := -1, 0.0
		for j := range clusters {
			if sim := observe.SignatureSimilarity(clusters[j].sig, sig); sim > bestSim {
				best, bestSim = j, sim
			}
		}
		if best >= 0 && bestSim >= observe.StateMatchSimilarity {
			per[i] = best
			clusters[best].frames++
			continue
		}
		per[i] = len(clusters)
		clusters = append(clusters, screenCluster{sig: sig, frames: 1, first: i})
	}
	return per, clusters
}

// screenChanges counts adjacent frames that landed in different clusters.
//
// Adjacent, because that is what a transition is: the thing before and the thing after. A count
// of distinct clusters says how many screens were distinguishable; this says how often the
// screen was seen to CHANGE, and a model that cannot do the second cannot hold a route.
func screenChanges(per []int) int {
	n := 0
	for i := 1; i < len(per); i++ {
		if per[i] != per[i-1] {
			n++
		}
	}
	return n
}

// pressOutcome is what happened to the screen around one press.
type pressOutcome struct {
	// Frame is the frame the press was attributed to, Before the cluster the screen was
	// in at that point, and After the cluster it was in once it had time to settle.
	Frame         int
	Before, After int
	// Changed says the two differ, which is the whole claim a click-target route rests on:
	// this press was followed by a different screen.
	Changed bool
}

// pressesAcrossScreens asks, for each press, whether the screen it was made on differs from the
// screen a short while later.
//
// `settle` frames later rather than the next frame, because a menu does not repaint instantly
// and the frame immediately after a click is as likely to be mid-transition as either endpoint.
// Nothing here claims the press CAUSED the change — that is what [[ADR-158]] and the activation
// evidence are for, and neither is available on a mouse demonstration. It says only whether the
// evidence a causal claim would need is present at all.
func pressesAcrossScreens(pressFrames []int, per []int, settle int) []pressOutcome {
	out := make([]pressOutcome, 0, len(pressFrames))
	for _, f := range pressFrames {
		if f < 0 || f >= len(per) {
			continue
		}
		after := f + settle
		if after >= len(per) {
			after = len(per) - 1
		}
		o := pressOutcome{Frame: f, Before: per[f], After: per[after]}
		o.Changed = o.Before != o.After
		out = append(out, o)
	}
	return out
}

// reportScreens prints the measurement, labelled as one.
func reportScreens(r frameResult) {
	if len(r.PixelClusters) == 0 {
		return
	}
	fmt.Printf("\n  --- MEASUREMENT, not Marco's belief ---\n")
	fmt.Printf("  if unnamed geometry counted toward a screen's identity, this recording\n")
	fmt.Printf("  would have segmented into:\n")
	fmt.Printf("  pixel screens       %d distinct arrangement(s), %d change(s) between "+
		"adjacent frames\n", len(r.PixelClusters), r.PixelScreenChanges)
	for i, c := range r.PixelClusters {
		if i >= 8 {
			fmt.Printf("                      ... and %d more\n", len(r.PixelClusters)-8)
			break
		}
		fmt.Printf("                      #%d  %4d frame(s), first at %d, %d region(s) "+
			"over %d cell(s)\n", i, c.frames, c.first, c.sig.Total, len(c.sig.Cells))
	}
	if len(r.PressOutcomes) > 0 {
		changed := 0
		for _, o := range r.PressOutcomes {
			if o.Changed {
				changed++
			}
		}
		fmt.Printf("  presses followed by\n")
		fmt.Printf("  a different screen  %d of %d\n", changed, len(r.PressOutcomes))
		for _, o := range r.PressOutcomes {
			mark := "same"
			if o.Changed {
				mark = "CHANGED"
			}
			fmt.Printf("                      frame %4d  #%d -> #%d  %s\n",
				o.Frame, o.Before, o.After, mark)
		}
	}
	fmt.Printf("  Marco's own screen model on this recording is %d state(s), %d "+
		"transition(s).\n", r.States, r.Transitions)
}

// regionsOf is a look's rectangles, which is all an arrangement is made of.
func regionsOf(shapes []observe.ShadowShape) []observe.Region {
	out := make([]observe.Region, 0, len(shapes))
	for _, sh := range shapes {
		out = append(out, sh.Region)
	}
	return out
}

// reportVisualScreens prints the stabilised layer, beside the raw measurement above it.
//
// Both, deliberately. The raw numbers are what an arrangement can SEE and the stabilised ones
// are what it would be worth acting on, and the gap between them is the whole argument for
// the layer existing. A report that showed only the second would be a claim nobody could check.
func reportVisualScreens(r frameResult) {
	st := r.VisualStability
	if st.Looks == 0 {
		return
	}
	fmt.Printf("\n  --- SESSION-LOCAL SCREEN HYPOTHESES (stabilised, never durable) ---\n")
	fmt.Printf("  settled screens     %d, over %d look(s)\n", len(r.VisualSeen), st.Looks)
	fmt.Printf("  looks               %d agreed with the standing screen, %d differed and "+
		"did NOT become one, %d before the first settled\n",
		st.Settled, st.Unsettled, st.Warming)
	fmt.Printf("  blank looks         %d saw no structure (not screens), %d run(s) long "+
		"enough to give up the screen\n", st.Unobserved, st.Unobservable)
	for i, sc := range r.VisualSeen {
		if i >= 10 {
			fmt.Printf("                      ... and %d more\n", len(r.VisualSeen)-10)
			break
		}
		fmt.Printf("                      %-10s %4d look(s), settled at %d, %d region(s) "+
			"over %d cell(s)\n", sc.ID, sc.Looks, sc.First, sc.Total, sc.Cells)
	}
	fmt.Printf("  transitions         %d distinct, %d promotion(s) in all\n",
		len(r.VisualTransitions), st.Promotions)
	for _, t := range r.VisualTransitions {
		looks := t.Looks
		if len(looks) > 8 {
			looks = looks[:8]
		}
		fmt.Printf("                      %-10s -> %-10s x%-3d at looks %v (%d look(s) "+
			"to settle in all)\n", t.From, t.To, t.Count, looks, t.Unsettled)
	}
	if len(r.PressScreens) > 0 {
		fmt.Printf("  presses             %d, by the screen they were made on and the "+
			"screen that followed\n", len(r.PressScreens))
		for _, p := range r.PressScreens {
			mark := "same screen"
			if p.Before != p.After {
				mark = "CHANGED"
			}
			fmt.Printf("                      frame %4d  %-10s -> %-10s  %s\n",
				p.Frame, p.Before, p.After, mark)
		}
	}
}

// pressScreen is one press and the settled screens around it.
type pressScreen struct {
	Frame         int
	Before, After observe.VisualScreenID
}

// reportClicks prints the learning units, and everything that failed to be one.
//
// The failures are not an appendix. A press that changed nothing, a change nobody pressed for,
// and a press that landed on nothing the proposer found are each a reason a route cannot be
// built over that step — and a report that listed only the successes would describe a route
// Marco cannot walk.
func reportClicks(r frameResult) {
	e := r.Clicks
	if len(e.Attributed) == 0 && len(e.Inert) == 0 && len(e.Unattributed) == 0 && e.Blind == 0 {
		return
	}
	fmt.Printf("\n  --- CLICK TRANSITIONS (source screen -> target -> destination) ---\n")
	fmt.Printf("  targets             %d session-local control identit(ies)\n", len(r.Targets))
	fmt.Printf("  presses             %d landed on nothing (never re-groundable), "+
		"%d inside more than one region\n", e.Blind, e.AmbiguousPresses)
	fmt.Printf("  attributed          %d distinct unit(s)\n", len(e.Attributed))
	for _, a := range e.Attributed {
		amb := ""
		if a.Ambiguous > 0 {
			amb = fmt.Sprintf("  [%d of them AMBIGUOUS: the target is one reading "+
				"of several]", a.Ambiguous)
		}
		fmt.Printf("                      %-10s --%-10s--> %-10s x%-2d presses at %v, "+
			"%v look(s) to arrive%s\n",
			a.From, a.Target, a.To, a.Count, a.Looks, a.Gaps, amb)
	}
	if len(e.Inert) > 0 {
		fmt.Printf("  changed nothing     %d press(es): ", len(e.Inert))
		for i, p := range e.Inert {
			if i >= 10 {
				fmt.Printf("...")
				break
			}
			fmt.Printf("look %d on %s (%s) ", p.Look, p.Screen, p.Target)
		}
		fmt.Println()
	}
	// ── BY CAUSE, which is what a planner would read ──
	byCause := map[observe.TransitionCause]int{}
	for _, st := range e.Steps {
		byCause[st.Cause] += st.Count
	}
	fmt.Printf("  observed changes    %d click, %d automatic, %d unknown cause\n",
		byCause[observe.CauseClick], byCause[observe.CauseAutomatic],
		byCause[observe.CauseUnknown])
	for _, st := range e.Steps {
		if st.Cause == observe.CauseClick {
			continue
		}
		fmt.Printf("                      %-10s -> %-10s  %-9s x%-2d at %v, %v look(s) "+
			"after arriving\n", st.From, st.To, st.Cause, st.Count, st.Looks, st.Gaps)
	}
	if len(e.MaybeOneControl) > 0 {
		fmt.Printf("  MAYBE ONE CONTROL   %d pair(s) of identities on one screen that led "+
			"to the same place:\n", len(e.MaybeOneControl))
		for _, p := range e.MaybeOneControl {
			fmt.Printf("                      on %-10s %s and %s both reached %s\n",
				p.Screen, p.A, p.B, p.To)
		}
		fmt.Printf("                      (reported, NOT merged: this is also the shape " +
			"of two different\n                      entries that happen to open the " +
			"same thing)\n")
	}
	if len(e.Unattributed) > 0 {
		fmt.Printf("  nobody pressed for  %d change(s): ", len(e.Unattributed))
		for i, t := range e.Unattributed {
			if i >= 10 {
				fmt.Printf("...")
				break
			}
			fmt.Printf("%s->%s ", t.From, t.To)
		}
		fmt.Printf("\n                      (a change that usually has no press before it is " +
			"evidence AGAINST\n                      reading the correlated ones as causes)\n")
	}
}

// reportClickRoute walks the evidence into a route somebody can read, with the uncertainty in it.
//
// # Why the uncertainty is in the line rather than a footnote
//
// The acceptance goal is "an inspectable, evidence-backed session-local route through the two
// demonstrations, with ambiguous or missing transitions explicitly represented". A route printed
// without its doubts is the thing this whole subsystem exists not to produce: it would read as a
// plan, and three of its steps would be guesses wearing a plan's clothes.
//
// So every step carries what is known about it, and a step that is only a screen change with no
// press behind it says so in the place where its target would have been. Nothing is inferred to
// fill a gap: a chain stops where the evidence stops.
//
// It starts at the screen that held the most looks, which on a menu recording is the screen
// somebody kept coming back to. That is a heuristic about where to begin READING and not a
// claim about the route; a different start would print the same steps in a different order.
func reportClickRoute(r frameResult) {
	if len(r.VisualSeen) == 0 {
		return
	}
	start := r.VisualSeen[0].ID
	best := 0
	for _, sc := range r.VisualSeen {
		if sc.Looks > best {
			start, best = sc.ID, sc.Looks
		}
	}
	// What is attributed, and what merely happened.
	outgoing := map[observe.VisualScreenID][]observe.RouteStep{}
	for _, st := range r.Clicks.Steps {
		outgoing[st.From] = append(outgoing[st.From], st)
	}
	// Which pairs two identities competed over, so a step can say so.
	competing := map[string]bool{}
	for _, p := range r.Clicks.MaybeOneControl {
		competing[string(p.Screen)+">"+string(p.To)] = true
	}

	fmt.Printf("\n  --- THE ROUTE, AS EVIDENCE (session-local; nothing here is durable) ---\n")
	fmt.Printf("  reading from %s, which held the most looks\n\n", start)

	seen := map[observe.VisualScreenID]bool{}
	at := start
	for n := 0; n < len(r.VisualSeen)+2; n++ {
		if seen[at] {
			fmt.Printf("      %-10s (already visited — the walk closes here)\n", at)
			return
		}
		seen[at] = true
		steps := outgoing[at]
		if len(steps) == 0 {
			fmt.Printf("      %-10s NO STEP OUT: nothing was pressed on this screen "+
				"that was followed by a change\n", at)
			return
		}
		// The best-evidenced step first: executable over not, then repeated over
		// once-seen. A click is preferred for READING, not because it is better
		// evidence — an automatic step beside it is a real alternative and is printed.
		sort.SliceStable(steps, func(i, j int) bool {
			ei, ej := steps[i].Cause.Executable(), steps[j].Cause.Executable()
			if ei != ej {
				return ei
			}
			return steps[i].Count > steps[j].Count
		})
		for i, st := range steps {
			lead := "  -> "
			if i > 0 {
				lead = "   or "
			}
			note := "seen ONCE — not confirmed"
			if st.Count > 1 {
				note = fmt.Sprintf("confirmed in %d demonstrations", st.Count)
			}
			if st.Ambiguous > 0 {
				note += "; the target is one reading of several"
			}
			if competing[string(at)+">"+string(st.To)] {
				note += "; ANOTHER identity reached the same place, so these " +
					"may be one control"
			}
			switch st.Cause {
			case observe.CauseClick:
				fmt.Printf("      %-10s%sCLICK  %-10s  via %-10s  (%s)\n",
					at, lead, st.To, st.Target, note)
			case observe.CauseAutomatic:
				fmt.Printf("      %-10s%sWAIT   %-10s  no target: it arrived by "+
					"itself after %v look(s)  (%s)\n",
					at, lead, st.To, st.Gaps, note)
			default:
				fmt.Printf("      %-10s%s?      %-10s  CAUSE UNKNOWN — nothing may "+
					"be executed here  (%s)\n", at, lead, st.To, note)
			}
		}
		at = steps[0].To
	}
}

// reportShapeVsCount asks whether any two settled screens are one screen at two yields.
//
// The proposer finds a different number of rectangles on frames that look the same to a person,
// and a comparison that counts them is sensitive to that. This prints, for every pair of settled
// screens, the similarity BY COUNT (the rule the stabiliser uses) beside the similarity BY SHAPE
// (each cell as a proportion of that arrangement's own total) — and names the pairs that would
// be one screen under the second and are two under the first.
//
// A measurement. Nothing here changes what settled; the point is to find out what normalising
// would cost and what it would buy before anybody puts it in the rule.
func reportShapeVsCount(r frameResult, arr func(observe.VisualScreenID) observe.VisualArrangement) {
	if len(r.VisualSeen) < 2 {
		return
	}
	type pair struct {
		a, b         observe.VisualScreenID
		count, shape float64
	}
	var only []pair
	var worst float64
	for i := range r.VisualSeen {
		for j := i + 1; j < len(r.VisualSeen); j++ {
			a, b := r.VisualSeen[i].ID, r.VisualSeen[j].ID
			c := observe.ArrangementSimilarity(arr(a), arr(b))
			sh := observe.ArrangementShapeSimilarity(arr(a), arr(b))
			if sh > worst {
				worst = sh
			}
			if sh >= observe.VisualScreenMatch && c < observe.VisualScreenMatch {
				only = append(only, pair{a, b, c, sh})
			}
		}
	}
	fmt.Printf("\n  --- MEASUREMENT: would normalising the proposal COUNT merge any screens? ---\n")
	fmt.Printf("  highest shape-similarity between any two settled screens: %.3f "+
		"(threshold %.2f)\n", worst, observe.VisualScreenMatch)
	if len(only) == 0 {
		fmt.Printf("  no pair of settled screens would merge under normalisation, so the\n")
		fmt.Printf("  proposer's varying yield is NOT what is separating them here.\n")
		return
	}
	fmt.Printf("  %d pair(s) would be ONE screen by shape and are two by count:\n", len(only))
	for _, p := range only {
		fmt.Printf("                      %-10s / %-10s  count %.3f  shape %.3f\n",
			p.a, p.b, p.count, p.shape)
	}
	fmt.Printf("  Reported, not applied: a menu that genuinely gained a column also\n")
	fmt.Printf("  changes its count, and normalising would merge that too.\n")
}
