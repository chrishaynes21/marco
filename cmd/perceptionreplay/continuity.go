package main

// Why did the previous binding disappear?
//
// # The question, and why it is a measurement rather than a change
//
// [[ADR-186-a-person-can-say-where-they-are]] measured seven questions over one route, and six
// of them were about a screen already established earlier in the same walk. That is either a
// real cost of the safety rule or an artefact of how the observer decides a visit has ended, and
// the two want opposite responses.
//
// So nothing here changes the observation generation, and nothing restores a binding. Each
// repeated question is reconstructed from the looks around it and classified by WHAT ENDED the
// previous one:
//
//	A -> unobserved -> A   nothing else was ever seen. Possibly one visit with a gap.
//	A -> B -> A            something else was positively observed. Two visits, and a
//	                       confirmation that crossed it would be wrong.
//
// The second is the case the rule exists for. The first is the one worth knowing the size of.

import (
	"fmt"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// walkLook is one look of the walk, kept so a gap can be reconstructed afterwards.
type walkLook struct {
	look       int
	atMS       int64
	epoch      uint64
	generation uint64

	// returned is the id `Observe` gave back for this look, and standing the screen the
	// stabiliser still holds.
	//
	// They are different, and the difference is the whole investigation. The observation
	// generation moves on `returned`; a transient `unobserved` moves it twice while
	// `standing` never wavers. An earlier version of this recorded only `standing` and
	// duly classified four gaps as "indeterminate" in which, by that record, nothing
	// whatever had happened.
	returned observe.VisualScreenID
	standing observe.VisualScreenID

	// missing says no pixels and front that the target window was in front. Together they
	// separate a capture gap from an observer that saw plenty and settled on nothing.
	missing   bool
	front     bool
	proposals int
}

// gapKind is why one established screen stopped being established.
type gapKind string

const (
	// gapTransient: the returned id went unobserved and came back, the stabiliser never
	// let go, and nothing else was ever positively observed.
	gapTransient gapKind = "transient loss"
	// gapNavigated: another screen settled in between. A real visit elsewhere.
	gapNavigated gapKind = "navigated away"
	// gapWindow: the window generation changed.
	gapWindow gapKind = "window replaced"
	// gapBlind: the looks in between had no pixels at all — a capture gap rather than an
	// interface that moved.
	gapBlind gapKind = "no pixels"
	// gapUnsettled: the identity went and the stabiliser let go too, with pixels present
	// throughout. The interface was visible and its identity was not.
	gapUnsettled gapKind = "observer could not hold a screen"
	// gapUnknown: the recording cannot say.
	gapUnknown gapKind = "indeterminate"
)

// gapReport is one repeated question, reconstructed.
type gapReport struct {
	label    string
	from, to int
	kind     gapKind

	looks int
	ms    int64
	// blind is looks with no pixels, covered looks with another window in front, and
	// unsettled looks whose returned id was no identity at all.
	blind, covered, unsettled int
	// other is every screen positively observed during the gap.
	other []observe.VisualScreenID
	// windowChanged says the window generation moved at any point in the gap.
	windowChanged bool
	// heldThroughout says the stabiliser never let go of the previous screen, even while
	// the returned id went unobserved. The difference between a gap in WHAT WAS SEEN and
	// a gap in WHAT WAS THERE.
	heldThroughout bool
	// agrees is how well the returning screen's arrangement matches the one established
	// before — INDEPENDENT of the person's earlier answer, which is the only evidence a
	// continuity rule could honestly use.
	agrees float64
	// recoverable says a suspended-confirmation rule could safely bridge this.
	recoverable bool
	because     string
}

// classifyGap reconstructs what happened between two establishments of one screen.
//
// Deliberately pessimistic at every fork. A gap is bridgeable only when nothing else was ever
// settled, the window never moved, the stabiliser never let go, and the returning arrangement
// independently agrees — because the cost of being wrong is a confirmation carried across a real
// navigation, which is the exact failure this subsystem has spent several experiments avoiding.
func classifyGap(label string, timeline []walkLook, from, to int,
	prevScreen observe.VisualScreenID, prevArr, nowArr observe.VisualArrangement) gapReport {

	g := gapReport{label: label, from: from, to: to, heldThroughout: true}
	seen := map[observe.VisualScreenID]bool{}
	var firstGen, lastGen uint64
	var firstMS, lastMS int64
	started := false

	for _, r := range timeline {
		// THE CLOSING LOOK IS PART OF THE GAP, not a boundary of it. The epoch
		// moves ON the look whose identity changed, so that look IS the event
		// that ended the previous binding — excluding it left four gaps in which,
		// by the record, nothing had happened.
		if r.look <= from || r.look > to {
			continue
		}
		g.looks++
		if !started {
			firstGen, firstMS, started = r.generation, r.atMS, true
		}
		lastGen, lastMS = r.generation, r.atMS
		if r.standing != prevScreen {
			g.heldThroughout = false
		}
		switch {
		case r.missing:
			g.blind++
		case !r.front:
			g.covered++
		case r.returned == "" || r.returned == observe.VisualScreenUnknown ||
			r.returned == observe.VisualScreenUnobserved:
			g.unsettled++
		case r.returned != prevScreen:
			if !seen[r.returned] {
				seen[r.returned] = true
				g.other = append(g.other, r.returned)
			}
		}
	}
	if started {
		g.ms = lastMS - firstMS
	} else {
		g.heldThroughout = false
	}
	sort.Slice(g.other, func(i, j int) bool { return g.other[i] < g.other[j] })
	// AGAINST THE WINDOW THE BINDING WAS ESTABLISHED UNDER, not merely against
	// itself. A gap whose looks all share a generation has not “not changed” if
	// that generation is different from the one the person was looking at — which
	// is the whole case, and comparing within the gap missed it.
	anchorGen := firstGen
	for _, r := range timeline {
		if r.look == from {
			anchorGen = r.generation
		}
	}
	g.windowChanged = started && (firstGen != lastGen || firstGen != anchorGen)
	g.agrees = observe.ArrangementSimilarity(prevArr, nowArr)

	switch {
	case g.windowChanged:
		g.kind = gapWindow
		g.because = "the window generation moved, so the pixels either side are of " +
			"different windows"
	case len(g.other) > 0:
		g.kind = gapNavigated
		g.because = fmt.Sprintf("%v was positively observed in between, so this is a "+
			"second visit and a confirmation carried across it would be wrong",
			g.other)
	case g.blind > 0 && g.unsettled == 0:
		g.kind = gapBlind
		g.because = fmt.Sprintf("%d look(s) had no pixels at all; nothing was seen, "+
			"which is not the same as nothing being there", g.blind)
	case g.unsettled > 0 && g.heldThroughout:
		// THE CASE THIS INVESTIGATION EXISTS FOR.
		g.kind = gapTransient
		g.because = fmt.Sprintf("%d look(s) returned no identity while the stabiliser "+
			"held %s throughout, and nothing else was ever observed",
			g.unsettled, prevScreen)
	case g.unsettled > 0:
		g.kind = gapUnsettled
		g.because = fmt.Sprintf("%d look(s) returned no identity and the stabiliser let "+
			"go as well; the interface was visible and its identity was not",
			g.unsettled)
	case g.looks == 0:
		g.kind = gapUnknown
		g.because = "the two establishments are adjacent looks, so there was no gap " +
			"to examine"
	default:
		g.kind = gapUnknown
		g.because = "nothing in the recording accounts for the change"
	}

	// ── COULD A SUSPENDED CONFIRMATION SAFELY BRIDGE THIS? ──
	//
	// Four conditions, all required. The last is the one that matters: the person's
	// earlier answer is not evidence about now, so the returning observation has to agree
	// with the stored arrangement on its own before anything is carried across.
	g.recoverable = (g.kind == gapTransient || g.kind == gapBlind) &&
		len(g.other) == 0 && !g.windowChanged && g.heldThroughout &&
		g.agrees >= observe.VisualScreenMatch
	return g
}

// pairReturnLegs marks the second of two questions caused by ONE interruption.
//
// # Why a single blind look costs two questions
//
// The observation generation moves when the returned identity changes. A look with no pixels
// returns no identity, so the generation moves on the way out AND on the way back: one
// interruption, two expired bindings, two questions. The second gap looks empty when examined on
// its own, because nothing happened during it — what happened was the look before it.
//
// So a gap that begins exactly where a bridgeable one ended, and contains no contradicting
// evidence of its own, is the return leg of that same interruption and stands or falls with it.
// This is bookkeeping about one event, not a second rule: it cannot bridge anything the first
// gap could not.
func pairReturnLegs(gaps []gapReport) {
	for i := 1; i < len(gaps); i++ {
		g, prev := &gaps[i], gaps[i-1]
		if g.kind != gapUnknown || g.from != prev.to || g.label != prev.label {
			continue
		}
		if !prev.recoverable || len(g.other) > 0 || g.windowChanged || !g.heldThroughout {
			continue
		}
		g.kind = gapTransient
		g.recoverable = g.agrees >= observe.VisualScreenMatch
		g.because = fmt.Sprintf("the return leg of the interruption at look %d: the "+
			"identity went on the way out and came back on the way in, and one "+
			"interruption moves the generation twice", prev.to)
	}
}

// printContinuity reports every repeated question and what a continuity rule could do.
func printContinuity(gaps []gapReport, timeline []walkLook) {
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHY THE PREVIOUS BINDING DISAPPEARED\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	if len(gaps) == 0 {
		fmt.Printf("      No question repeated a screen established earlier in this walk.\n")
		return
	}
	fmt.Printf("  Every question about a screen already established, reconstructed from\n")
	fmt.Printf("  the looks between the two. Nothing here changes a generation or\n")
	fmt.Printf("  restores a binding.\n\n")

	pairReturnLegs(gaps)

	counts := map[gapKind]int{}
	recoverable := 0
	for _, g := range gaps {
		counts[g.kind]++
		if g.recoverable {
			recoverable++
		}
		fmt.Printf("  %s: looks %d -> %d  (%d look(s), %dms)\n",
			g.label, g.from, g.to, g.looks, g.ms)
		fmt.Printf("    %-36s %s\n", string(g.kind), tick(g.recoverable))
		fmt.Printf("    no-identity %d  blind %d  covered %d  held throughout %t\n",
			g.unsettled, g.blind, g.covered, g.heldThroughout)
		fmt.Printf("    other screens observed %v   window moved %t\n",
			g.other, g.windowChanged)
		fmt.Printf("    the returning observation agrees with the stored arrangement at %.3f\n",
			g.agrees)
		fmt.Printf("    %s\n\n", wrapAt(g.because, 66, "    "))
	}

	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT A SUSPENDED CONFIRMATION COULD AND COULD NOT DO\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Printf("      %-36s %d\n", k, counts[gapKind(k)])
	}
	fmt.Printf("\n      %d of %d repeated question(s) could be bridged safely\n",
		recoverable, len(gaps))
	fmt.Printf("      %d remain genuinely uncertain or are real navigation\n",
		len(gaps)-recoverable)
	fmt.Printf("\n      The rule tested requires ALL of: nothing else positively observed,\n")
	fmt.Printf("      the window unchanged, the stabiliser never having let go, and the\n")
	fmt.Printf("      returning observation agreeing with the stored arrangement ON ITS\n")
	fmt.Printf("      OWN. The last is the one that matters — a person's earlier answer is\n")
	fmt.Printf("      not evidence about now.\n")
	printGenerationChurn(timeline)
}

// printGenerationChurn sizes the candidate change this investigation is NOT allowed to make.
//
// The observation generation moves when the RETURNED identity changes. The stabiliser exists to
// hold a screen through exactly the noise that makes an identity flicker, so the generation is
// jumpier than the screen it describes. Counting the moves where the standing screen never
// wavered says how much of the cost is that difference — without changing anything, which is the
// constraint this ran under.
func printGenerationChurn(timeline []walkLook) {
	moves, held := 0, 0
	for i := 1; i < len(timeline); i++ {
		if timeline[i].epoch == timeline[i-1].epoch {
			continue
		}
		moves++
		if timeline[i].standing == timeline[i-1].standing && timeline[i].standing != "" {
			held++
		}
	}
	fmt.Printf("\n      of %d observation-generation move(s) in this walk, %d happened\n",
		moves, held)
	fmt.Printf("      while the stabiliser never let go of the screen. Those are moves\n")
	fmt.Printf("      in WHAT WAS SEEN rather than in what was there.\n")
}

func tick(ok bool) string {
	if ok {
		return "-> could be bridged"
	}
	return "-> needs a fresh answer"
}
