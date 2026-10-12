package main

// WHAT A SHORT-LIVED SCREEN IS, AND WHETHER TWO DEMONSTRATIONS ARE ONE ROUTE.
//
// # The brief
//
// Nine screens settled on a recording holding four or five a person would name, and two of them
// held 8 and 2 looks. The owner's instruction about those was explicit:
//
//	A short-lived screen could be a loading state, a genuine transient menu, or a false
//	segmentation. The system should distinguish those outcomes rather than automatically
//	deleting brief states. […] Preserve unknown when evidence is insufficient.
//
// So nothing here deletes anything. It classifies on the EVIDENCE SHAPE — was it clicked on, how
// was it left, does it recur — and reports `unknown` where the shape says nothing.
//
// # Why there is no duration threshold
//
// Because "brief" is the symptom and not the cause. A loading screen is brief; so is a menu
// somebody passed through quickly; so is a segmentation artefact. A number dividing them would
// be invented here and would be wrong on the first application with a slower animation.
//
// What separates them is what HAPPENED on them: somebody pressed something, or the screen left
// by itself, or neither. The duration is reported beside the classification so a reader can
// judge it, and it does not decide anything.

import (
	"fmt"
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// screenRole is what the evidence says a settled screen was.
type screenRole string

const (
	// rolePlace is a screen somebody pressed something on. Whatever else it is, it is
	// somewhere a person acted, and a route can stop there.
	rolePlace screenRole = "place"
	// rolePassage is a screen nobody pressed anything on and that was left AUTOMATICALLY
	// — the loading-state shape. A route passes through it by waiting.
	rolePassage screenRole = "passage"
	// roleUnexplained is a screen nobody pressed anything on and that was left for no
	// reason this layer can establish.
	//
	// An animation, a splash, a flash of mid-transition geometry, or a real transient
	// menu nobody happened to click. NOT deleted and NOT called an artefact: the whole
	// point of the category is that the evidence is insufficient and says so.
	roleUnexplained screenRole = "unexplained"
)

// screenVerdict is one settled screen and what the session can say about it.
type screenVerdict struct {
	ID    observe.VisualScreenID
	Role  screenRole
	Looks int
	First int
	// Settles is how many times the session settled on it, Presses how many presses were
	// made on it, and InAuto/OutAuto how many of its changes in and out were automatic.
	Settles, Presses      int
	InAuto, OutAuto       int
	InUnknown, OutUnknown int
	// Because is the evidence in words.
	Because string
}

// classifyScreens says what each settled screen was, from the evidence and nothing else.
func classifyScreens(r frameResult) []screenVerdict {
	presses := map[observe.VisualScreenID]int{}
	for _, p := range r.Clicks.Inert {
		presses[p.Screen]++
	}
	for _, a := range r.Clicks.Attributed {
		presses[a.From] += a.Count
	}
	settles := map[observe.VisualScreenID]int{}
	inAuto, outAuto := map[observe.VisualScreenID]int{}, map[observe.VisualScreenID]int{}
	inUnk, outUnk := map[observe.VisualScreenID]int{}, map[observe.VisualScreenID]int{}
	for _, st := range r.Clicks.Steps {
		settles[st.To] += st.Count
		switch st.Cause {
		case observe.CauseAutomatic:
			inAuto[st.To] += st.Count
			outAuto[st.From] += st.Count
		case observe.CauseUnknown:
			inUnk[st.To] += st.Count
			outUnk[st.From] += st.Count
		}
	}

	out := make([]screenVerdict, 0, len(r.VisualSeen))
	for _, sc := range r.VisualSeen {
		v := screenVerdict{
			ID: sc.ID, Looks: sc.Looks, First: sc.First,
			Settles: settles[sc.ID], Presses: presses[sc.ID],
			InAuto: inAuto[sc.ID], OutAuto: outAuto[sc.ID],
			InUnknown: inUnk[sc.ID], OutUnknown: outUnk[sc.ID],
		}
		switch {
		case v.Presses > 0 && v.InAuto > 0 && v.InAuto >= v.Settles:
			// APPEARED BY ITSELF AND WAS DISMISSED. Every arrival was automatic and
			// somebody pressed something on it — a dialog, an interstitial, a
			// "press to continue". Still a place, because a person acted on it, and
			// worth naming because a route through one is a WAIT and then a click,
			// which is two steps a planner has to get in the right order.
			v.Role = rolePlace
			v.Because = fmt.Sprintf("it appeared by itself every time (%d) and %d "+
				"press(es) were made on it: the shape of a dialog or an "+
				"interstitial, reached by waiting and left by clicking",
				v.InAuto, v.Presses)
		case v.Presses > 0:
			v.Role = rolePlace
			v.Because = fmt.Sprintf("%d press(es) were made on it", v.Presses)
		case v.OutAuto > 0:
			v.Role = rolePassage
			v.Because = "nothing was pressed on it and it was left automatically, " +
				"which is the shape of a loading state: a route passes through " +
				"it by waiting"
		default:
			v.Role = roleUnexplained
			v.Because = "nothing was pressed on it and nothing explains how it was " +
				"left. An animation, a splash, a flash of mid-transition " +
				"geometry and a transient menu nobody clicked all look like this"
		}
		out = append(out, v)
	}
	return out
}

// reportTransients prints the verdicts, with the duration beside them rather than deciding them.
func reportTransients(r frameResult, verdicts []screenVerdict) {
	if len(verdicts) == 0 {
		return
	}
	fmt.Printf("\n  --- WHAT EACH SETTLED SCREEN WAS (nothing is deleted for being brief) ---\n")
	byRole := map[screenRole]int{}
	for _, v := range verdicts {
		byRole[v.Role]++
	}
	fmt.Printf("  %d place(s), %d passage(s), %d unexplained\n",
		byRole[rolePlace], byRole[rolePassage], byRole[roleUnexplained])
	for _, v := range verdicts {
		fmt.Printf("      %-10s %-11s %4d look(s), settled %d time(s), %d press(es)\n",
			v.ID, v.Role, v.Looks, v.Settles, v.Presses)
		fmt.Printf("                 %s\n", v.Because)
	}
}

// ── ROUTE-LEVEL STABILITY ──

// longestRepeatedRun is the longest contiguous run of screens that occurs at least twice in a
// walk, and where the two occurrences start.
//
// # Why a contiguous run and not an edit distance
//
// Because the question is whether the person walked the same path twice, and a path is an
// order. An edit distance would answer "how similar are these sequences", which is a different
// and softer question — and it would score two walks that visit the same screens in different
// orders as nearly identical, which is exactly the mistake a route must not make.
//
// Non-overlapping occurrences, because a screen that alternates (A B A B A B) would otherwise
// report its own tail as a repetition of its head.
func longestRepeatedRun(walk []observe.VisualScreenID) (run []observe.VisualScreenID, at []int) {
	best := 0
	var bestAt []int
	for i := range walk {
		for j := i + 1; j < len(walk); j++ {
			n := 0
			for i+n < j && j+n < len(walk) && walk[i+n] == walk[j+n] {
				n++
			}
			if n > best {
				best, bestAt = n, []int{i, j}
			}
		}
	}
	if best == 0 {
		return nil, nil
	}
	return walk[bestAt[0] : bestAt[0]+best], bestAt
}

// withoutRoles drops the screens whose verdict is in the given set.
//
// "After accounting for transient states", which is the owner's phrase: a route read at the
// logical level should not be broken by a loading screen that appeared in one demonstration and
// not the other. Collapsing them is a READING of the walk and never a change to it — the full
// walk is reported too, and the comparison between the two is the answer.
func withoutRoles(walk []observe.VisualScreenID, verdicts []screenVerdict,
	drop map[screenRole]bool) []observe.VisualScreenID {

	role := map[observe.VisualScreenID]screenRole{}
	for _, v := range verdicts {
		role[v.ID] = v.Role
	}
	out := make([]observe.VisualScreenID, 0, len(walk))
	for _, id := range walk {
		if drop[role[id]] {
			continue
		}
		// AND NO SCREEN TWICE IN A ROW. Dropping a passage between two looks at one
		// screen leaves `A A`, which is not a step and would make every collapsed
		// walk report a repetition it did not have.
		if len(out) > 0 && out[len(out)-1] == id {
			continue
		}
		out = append(out, id)
	}
	return out
}

// reportRouteStability asks whether the two demonstrations are one logical route.
func reportRouteStability(r frameResult, verdicts []screenVerdict) {
	if len(r.Walk) == 0 {
		return
	}
	fmt.Printf("\n  --- IS IT THE SAME ROUTE TWICE? ---\n")
	fmt.Printf("  the session settled on %d screen(s) in this order:\n      ", len(r.Walk))
	for i, id := range r.Walk {
		if i > 0 {
			fmt.Printf(" -> ")
		}
		fmt.Printf("%s", id)
	}
	fmt.Println()

	raw, rawAt := longestRepeatedRun(r.Walk)
	fmt.Printf("  longest run repeated, as observed:   %d screen(s)", len(raw))
	if len(raw) > 0 {
		fmt.Printf(" at positions %v\n      ", rawAt)
		for i, id := range raw {
			if i > 0 {
				fmt.Printf(" -> ")
			}
			fmt.Printf("%s", id)
		}
	}
	fmt.Println()

	// AND AGAIN WITHOUT THE TRANSIENTS. A loading screen that appeared in one
	// demonstration and not the other breaks a comparison of raw walks, and it is not a
	// difference in the route somebody walked.
	collapsed := withoutRoles(r.Walk, verdicts,
		map[screenRole]bool{rolePassage: true, roleUnexplained: true})
	fmt.Printf("  the same, with passages and unexplained screens dropped (%d screen(s)):\n      ",
		len(collapsed))
	for i, id := range collapsed {
		if i > 0 {
			fmt.Printf(" -> ")
		}
		fmt.Printf("%s", id)
	}
	fmt.Println()
	col, colAt := longestRepeatedRun(collapsed)
	fmt.Printf("  longest run repeated, at that level: %d screen(s)", len(col))
	if len(col) > 0 {
		fmt.Printf(" at positions %v\n      ", colAt)
		for i, id := range col {
			if i > 0 {
				fmt.Printf(" -> ")
			}
			fmt.Printf("%s", id)
		}
	}
	fmt.Println()

	// ── WHERE THE TWO DEMONSTRATIONS DISAGREE ABOUT WHY ──
	//
	// The finding that makes this worth printing: one step of this route was a CLICK in
	// one demonstration and AUTOMATIC in the other, because the load took a different
	// length of time. The route is the same and the causes are not, and a planner reading
	// only the causes would conclude the two walks were different journeys.
	type key struct{ from, to observe.VisualScreenID }
	causes := map[key]map[observe.TransitionCause]int{}
	for _, st := range r.Clicks.Steps {
		k := key{st.From, st.To}
		if causes[k] == nil {
			causes[k] = map[observe.TransitionCause]int{}
		}
		causes[k][st.Cause] += st.Count
	}
	var split []key
	for k, m := range causes {
		if len(m) > 1 {
			split = append(split, k)
		}
	}
	sort.Slice(split, func(i, j int) bool {
		if split[i].from != split[j].from {
			return split[i].from < split[j].from
		}
		return split[i].to < split[j].to
	})
	if len(split) == 0 {
		fmt.Printf("  every step that was observed twice was observed with the same cause.\n")
		return
	}
	fmt.Printf("  %d step(s) were observed with DIFFERENT causes on different occasions:\n",
		len(split))
	for _, k := range split {
		fmt.Printf("      %-10s -> %-10s ", k.from, k.to)
		for _, c := range []observe.TransitionCause{
			observe.CauseClick, observe.CauseAutomatic, observe.CauseUnknown,
		} {
			if n := causes[k][c]; n > 0 {
				fmt.Printf(" %s x%d", c, n)
			}
		}
		fmt.Println()
	}
	fmt.Printf("      (the same change, caused differently on two occasions. A load that\n")
	fmt.Printf("      took longer than the attribution window reads as automatic, and the\n")
	fmt.Printf("      route is the same either way — which is why a planner must not treat\n")
	fmt.Printf("      a cause disagreement as two different journeys.)\n")
}

// ── AMBIGUOUS TARGETS, COMPARED BY GEOMETRY AND NOT BY WHERE THEY LED ──
//
// The owner's instruction, and the trap it closes:
//
//	Investigate ambiguous target pairs using their actual region geometry, visual context,
//	and temporal evidence. Do not merge targets solely because they reach the same
//	destination.
//
// Two identities reaching one destination is how the pair was NOTICED. It is not evidence that
// they are one control — two different entries of a menu can open the same thing, and a menu
// with a "back" on every page proves it. So this reports what the rectangles themselves say,
// and reaching the same place is deliberately not part of the comparison.

// targetComparison is what the geometry says about two identities on one screen.
type targetComparison struct {
	Screen observe.VisualScreenID
	A, B   observe.VisualTargetID
	// Centres is the distance between the closest pair of their rectangles' centres, as a
	// fraction of the screen's diagonal.
	Centres float64
	// AreaRatio is the larger rectangle's area over the smaller, for that closest pair.
	AreaRatio float64
	// Overlap says whether the closest pair overlaps at all.
	Overlap bool
	// Verdict is the reading, in words, and it is allowed to be "cannot say".
	Verdict string
}

// compareTargets measures the pairs that reached the same destination, on geometry alone.
func compareTargets(r frameResult) []targetComparison {
	where := map[observe.VisualTargetID][]observe.Region{}
	for _, t := range r.Targets {
		where[t.ID] = t.Where
	}
	out := make([]targetComparison, 0, len(r.Clicks.MaybeOneControl))
	for _, p := range r.Clicks.MaybeOneControl {
		c := targetComparison{Screen: p.Screen, A: p.A, B: p.B,
			Centres: 2, AreaRatio: 0}
		best := -1.0
		for _, x := range where[p.A] {
			for _, y := range where[p.B] {
				dx := (x.X + x.Width/2) - (y.X + y.Width/2)
				dy := (x.Y + x.Height/2) - (y.Y + y.Height/2)
				d := math.Hypot(dx, dy)
				if best >= 0 && d >= best {
					continue
				}
				best = d
				c.Centres = d
				ax, ay := x.Width*x.Height, y.Width*y.Height
				if ax <= 0 || ay <= 0 {
					c.AreaRatio = 0
				} else if ax > ay {
					c.AreaRatio = ax / ay
				} else {
					c.AreaRatio = ay / ax
				}
				c.Overlap = x.X < y.X+y.Width && y.X < x.X+x.Width &&
					x.Y < y.Y+y.Height && y.Y < x.Y+x.Height
			}
		}
		switch {
		case best < 0:
			c.Verdict = "one of them remembers no rectangle, so nothing can be compared"
		case !c.Overlap:
			c.Verdict = "their rectangles do not overlap at all, so they are TWO " +
				"CONTROLS that happen to lead to the same place"
		case c.AreaRatio > 4:
			c.Verdict = "they overlap and one is much larger: the likely reading is " +
				"a control and the thing containing it, which are not the same " +
				"control either"
		default:
			c.Verdict = "they overlap and are of comparable size, so they may well be " +
				"one control the proposer read two ways — and this evidence " +
				"cannot settle it. A third demonstration would"
		}
		out = append(out, c)
	}
	return out
}

// reportTargetComparison prints the geometry, and says what it cannot settle.
func reportTargetComparison(cs []targetComparison) {
	if len(cs) == 0 {
		return
	}
	fmt.Printf("\n  --- THE AMBIGUOUS TARGET PAIRS, ON GEOMETRY ALONE ---\n")
	fmt.Printf("  (they were noticed by reaching the same destination; that is deliberately\n")
	fmt.Printf("   NOT part of this comparison — two entries can open the same thing)\n")
	for _, c := range cs {
		fmt.Printf("      on %-10s %s / %s\n", c.Screen, c.A, c.B)
		fmt.Printf("                 centres %.3f apart, area ratio %.2f, overlap %v\n",
			c.Centres, c.AreaRatio, c.Overlap)
		fmt.Printf("                 %s\n", c.Verdict)
	}
}
