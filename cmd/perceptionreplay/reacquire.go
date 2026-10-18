package main

// CAN A DEMONSTRATED TARGET BE FOUND AGAIN? MEASURED ON THE RECORDING.
//
// # What this scores, and what it refuses to count
//
// The owner drew the line that shapes the whole file:
//
//	Report actual reacquisition success, false matches, abstentions, and how much repeated
//	evidence is available. Do not count a click's original observation as successful
//	reacquisition.
//
// So the look a press was folded on is excluded from every rate below. A matcher given the frame
// it learned from would report near-perfect recognition and would have measured nothing but its
// own memory.
//
// # The one piece of real ground truth
//
// A match rate alone cannot say whether the match is RIGHT. Three targets in `rl-freeplay-1` were
// clicked on two separate occasions, and the second press is independent evidence about where the
// first one's control was: if reacquisition — computed from everything known BEFORE the second
// press is folded — names a rectangle that is the same rectangle the second press actually landed
// on, the control was genuinely found again.
//
// That is the only check here that a human did not have to label, and it is the number worth
// reading.
//
// # And two kinds of wrongness are counted
//
//	collision   two different targets on one screen matched the same rectangle on one look.
//	            At least one of them is wrong, whichever it is.
//	drift       a target matched on some looks of a screen and not others. Not an error on its
//	            own — a dialog covers a button — but a target that flickers is not one a plan
//	            could rely on.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// targetScore is one target's reacquisition record over the session.
type targetScore struct {
	id     observe.VisualTargetID
	screen observe.VisualScreenID
	// looks the target's own screen was standing, excluding the looks it was pressed on.
	looks                            int
	matched, ambiguous, unobservable int
	indistinct                       int
	// elsewhere is matches made on a look whose standing screen is NOT the one this
	// control was demonstrated on.
	//
	// The number that decides whether per-control grounding is safe on its own. A replay
	// looking for one control at a time has no screen to scope by, so every one of these
	// is a look on which it would have been prepared to press something that is not
	// there -- and unlike a collision it takes only one control to be wrong.
	elsewhere int
	uncertain bool
	presses   int
	// verified is a second press that landed where reacquisition said the control was,
	// contradicted one that landed somewhere else, and declined a second press on a look
	// where reacquisition had abstained.
	verified, contradicted, declined int
}

// reportReacquisitionOverRecording folds the recording and scores target reacquisition.
func reportReacquisitionOverRecording(dir string, limit int, onnx string, delay int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("%s carries no frames", dir)
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	const interval = 100 * time.Millisecond
	obs, err := fastobserve.New(interval, 64)
	if err != nil {
		return err
	}
	go func() {
		for range obs.Updates() {
		}
	}()

	opt := structure.DefaultOptions()
	base := time.Unix(1700000000, 0).UTC()
	nextInput := 0
	scores := map[observe.VisualTargetID]*targetScore{}
	collisions, collisionLooks := 0, 0
	// THE SAME MEASUREMENT WITH NO SCREEN, side by side.
	//
	// `obs.Reacquire` asks "which of the controls on the screen I think I am standing on
	// can I find", and [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]]
	// measured that the screen half of that question has an error rate nothing can detect.
	// So the owner moved it:
	//
	//	Make individual learned controls and transitions the basis for game-menu replay.
	//	Use whole-screen matching as supporting context, not proof of identity.
	//
	// This scores `observe.ReacquireStored` over EVERY demonstrated target on every look,
	// with the screen dropped, which is what a replay grounded per control would do. Both
	// tallies come from one pass over one recording, so a difference between them is the
	// screen and nothing else.
	free := map[observe.VisualTargetID]*targetScore{}
	freeCollisions, freeCollisionLooks := 0, 0
	// THE SLOW SOURCE, if one was recorded. Same discipline as the workflow replay:
	// ask on a settle, one outstanding, deliver late, and let the observer decide
	// whether the answer is still about the screen in front of it.
	var feed *slowFeed
	if onnx != "" {
		fd, ferr := loadSlowFeed(onnx, delay)
		if ferr != nil {
			return ferr
		}
		feed = fd
	}
	look := 0
	for _, fr := range frames {
		look++
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front: true,
		}
		for nextInput < len(inputs) && inputs[nextInput].AtMS <= fr.AtMS {
			in := inputs[nextInput]
			nextInput++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				lk.Presses = append(lk.Presses, in.Where)
			}
		}
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			lk.Missing = true
		} else {
			res := structure.Propose(img, opt)
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Bounds(), rects)
		}

		// REACQUISITION BEFORE THE FOLD, so nothing this look teaches can be used to
		// recognise what this look was supposed to recognise. On a look with a press
		// that ordering is the whole measurement; on the others it costs nothing and
		// keeps one rule.
		lk.Slow = feed.due(look)
		got := obs.Reacquire(lk)
		pressed := map[observe.VisualTargetID]bool{}
		if len(lk.Presses) > 0 && len(lk.Regions) > 0 {
			// WHERE THE PRESS ACTUALLY LANDED, worked out here rather than taken
			// from the correlator, because the correlator has not seen it yet.
			//
			// AND WHICH TARGET THIS PRESS IS, which the first version of this got
			// wrong in a way worth recording: it scored every reacquisition on the
			// standing screen against every press, so a press on one control
			// "contradicted" the three other controls on the same menu. Seven
			// contradictions of ten, and all of them an artefact of the scoring
			// rather than a finding about the matcher — the red twin of a green
			// measurement of nothing.
			//
			// A press is a repeat of the target whose stored reading agrees with
			// the region the press landed on, which is identify's own rule. A
			// press that agrees with nothing is a NEW identity and there is
			// nothing to verify.
			for _, w := range lk.Presses {
				landed, ok := smallestContaining(lk.Regions, w)
				if !ok {
					continue
				}
				for _, rec := range obs.Registry() {
					if rec.Screen != obs.Screen() {
						continue
					}
					if !agreesWithAny(rec.Where, landed) {
						continue
					}
					r, ok := outcomeFor(got, rec.ID)
					if !ok {
						continue
					}
					sc := score(scores, r)
					pressed[r.Target] = true
					if r.Result != observe.ReacquireMatched {
						// AN ABSTENTION ON THE ONE LOOK THAT COULD
						// HAVE PROVED IT. Not a wrong answer; counted
						// apart, because "it declined" and "it was
						// wrong" are different results.
						sc.declined++
						continue
					}
					// THE CHECK NOBODY LABELLED. Is the rectangle
					// reacquisition already named the same rectangle
					// this press went on to land on?
					if sameRegion(r.Where, landed) {
						sc.verified++
					} else {
						sc.contradicted++
					}
				}
			}
		}
		// THE ORDINARY LOOKS. Every target on the standing screen that was not pressed
		// on this look, counted whatever the outcome — a report that tallied only the
		// matches would be measuring its own optimism.
		seen := map[observe.VisualTargetID]observe.Region{}
		for _, r := range got {
			if pressed[r.Target] {
				continue
			}
			sc := score(scores, r)
			sc.looks++
			sc.uncertain = sc.uncertain || r.Uncertain
			switch r.Result {
			case observe.ReacquireMatched:
				sc.matched++
				seen[r.Target] = r.Where
			case observe.ReacquireAmbiguous:
				sc.ambiguous++
			default:
				sc.unobservable++
				if r.Indistinct > 0 {
					sc.indistinct++
				}
			}
		}
		// ── AND THE SAME LOOK, WITH NO SCREEN ──
		//
		// Every demonstrated target, not only the ones on the screen the observer
		// believes it is standing on. A replay grounded per control has no screen to
		// scope by, so this is what it would see.
		//
		// The ground-truth check is the same one and it is the number that matters: a
		// second press on a control is independent evidence about where the first one's
		// control was, and it does not care which screen anybody thought they were on.
		if len(lk.Regions) > 0 {
			freeSeen := map[observe.VisualTargetID]observe.Region{}
			for _, rec := range obs.Registry() {
				st := observe.StoredTarget{
					ID: rec.ID, Readings: rec.Where, Containers: rec.Containers,
				}
				r := observe.ReacquireStored(st, lk.Regions)
				sc := score(free, r)
				if pressed[rec.ID] {
					// THE LOOK THAT COULD PROVE IT. Scored as verified,
					// contradicted or declined, never as an ordinary look.
					if r.Result != observe.ReacquireMatched {
						sc.declined++
						continue
					}
					var landed observe.Region
					for _, w := range lk.Presses {
						if l, ok := smallestContaining(lk.Regions, w); ok &&
							agreesWithAny(rec.Where, l) {
							landed = l
						}
					}
					if sameRegion(r.Where, landed) {
						sc.verified++
					} else {
						sc.contradicted++
					}
					continue
				}
				sc.looks++
				switch r.Result {
				case observe.ReacquireMatched:
					sc.matched++
					freeSeen[rec.ID] = r.Where
					if rec.Screen != obs.Screen() {
						sc.elsewhere++
					}
				case observe.ReacquireAmbiguous:
					sc.ambiguous++
				default:
					sc.unobservable++
					if r.Indistinct > 0 {
						sc.indistinct++
					}
				}
			}
			// TWO TARGETS, ONE RECTANGLE — across the whole registry this time, so a
			// control from another menu colliding with this one is counted. That is
			// the cost the screen was paying for, and it has to be visible.
			if n := collidingPairs(freeSeen); n > 0 {
				freeCollisions += n
				freeCollisionLooks++
			}
		}
		// TWO TARGETS, ONE RECTANGLE. At least one is wrong.
		if n := collidingPairs(seen); n > 0 {
			collisions += n
			collisionLooks++
		}
		if up := obs.Observe(lk); up.Settled {
			feed.request(look, fr.WindowGeneration, up.Epoch)
		}
	}

	if feed != nil {
		fmt.Println(feed.report())
		fmt.Println(admissionLedger(obs.Stats()))
		fmt.Println()
	}
	obs.Close()

	for _, t := range obs.Registry() {
		if sc, ok := scores[t.ID]; ok {
			sc.presses = t.Presses
			sc.screen = t.Screen
		}
	}
	printReacquisition(dir, obs, scores, collisions, collisionLooks, look)
	printScreenFreeReacquisition(free, freeCollisions, freeCollisionLooks, look)
	return nil
}

// agreesWithAny reports whether a region is the same rectangle as any of a set.
func agreesWithAny(set []observe.Region, r observe.Region) bool {
	for _, x := range set {
		if sameRegion(x, r) {
			return true
		}
	}
	return false
}

// outcomeFor picks one target's outcome out of a look's reacquisitions.
func outcomeFor(got []observe.Reacquisition, id observe.VisualTargetID) (observe.Reacquisition, bool) {
	for _, r := range got {
		if r.Target == id {
			return r, true
		}
	}
	return observe.Reacquisition{}, false
}

// score finds or starts one target's record.
func score(m map[observe.VisualTargetID]*targetScore, r observe.Reacquisition) *targetScore {
	if sc, ok := m[r.Target]; ok {
		return sc
	}
	sc := &targetScore{id: r.Target, screen: r.Screen}
	m[r.Target] = sc
	return sc
}

// collidingPairs counts pairs of targets matched to the same rectangle on one look.
func collidingPairs(seen map[observe.VisualTargetID]observe.Region) int {
	ids := make([]observe.VisualTargetID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	n := 0
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if sameRegion(seen[ids[i]], seen[ids[j]]) {
				n++
			}
		}
	}
	return n
}

// smallestContaining is the least-area region containing a press.
func smallestContaining(regions []observe.Region, at observe.PointerAt) (observe.Region, bool) {
	var best observe.Region
	found := false
	for _, r := range regions {
		if !containsNormalised(r, at) {
			continue
		}
		if !found || r.Width*r.Height < best.Width*best.Height {
			best, found = r, true
		}
	}
	return best, found
}

// containsNormalised is containment in normalised coordinates, half-open like the production rule.
func containsNormalised(r observe.Region, at observe.PointerAt) bool {
	if r.Width <= 0 || r.Height <= 0 {
		return false
	}
	return at.X >= r.X && at.Y >= r.Y && at.X < r.X+r.Width && at.Y < r.Y+r.Height
}

// sameRegion is mutual centre containment, the production relation for "one control".
func sameRegion(a, b observe.Region) bool {
	in := func(r observe.Region, o observe.Region) bool {
		return containsNormalised(r, observe.PointerAt{
			X: o.X + o.Width/2, Y: o.Y + o.Height/2,
		})
	}
	return in(a, b) && in(b, a)
}

func printReacquisition(dir string, obs *fastobserve.Observer,
	scores map[observe.VisualTargetID]*targetScore, collisions, collisionLooks, looks int) {

	reg := obs.Registry()
	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("TARGET REACQUISITION OVER %s\n", filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d look(s), %d screen(s), %d demonstrated target(s)\n",
		looks, len(obs.Screens()), len(reg))

	repeated := 0
	for _, t := range reg {
		if t.Repeated() {
			repeated++
		}
	}
	fmt.Printf("  repeated evidence: %d of %d target(s) were clicked more than once\n",
		repeated, len(reg))
	fmt.Printf("  (a target clicked once has NO independent evidence that it can be " +
		"found again)\n")

	ids := make([]observe.VisualTargetID, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var looksTotal, matched, ambiguous, unobservable int
	var verified, contradicted, declined, indistinct int
	fmt.Printf("\n  %-10s %-10s %6s %7s %6s %7s %5s  %s\n",
		"target", "screen", "looks", "match", "ambig", "unobs", "ver", "notes")
	for _, id := range ids {
		sc := scores[id]
		looksTotal += sc.looks
		matched += sc.matched
		ambiguous += sc.ambiguous
		unobservable += sc.unobservable
		verified += sc.verified
		contradicted += sc.contradicted
		declined += sc.declined
		indistinct += sc.indistinct
		notes := ""
		if sc.presses > 1 {
			notes += fmt.Sprintf("%d presses ", sc.presses)
		}
		if sc.uncertain {
			notes += "uncertain-identity "
		}
		if sc.contradicted > 0 {
			notes += fmt.Sprintf("CONTRADICTED x%d ", sc.contradicted)
		}
		if sc.declined > 0 {
			notes += fmt.Sprintf("declined-on-%d-repeat(s) ", sc.declined)
		}
		if sc.indistinct > 0 {
			notes += fmt.Sprintf("indistinct-from-container x%d ", sc.indistinct)
		}
		fmt.Printf("  %-10s %-10s %6d %7d %6d %7d %5d  %s\n",
			sc.id, sc.screen, sc.looks, sc.matched, sc.ambiguous,
			sc.unobservable, sc.verified, notes)
	}

	fmt.Printf("\n  over %d target-look(s) on a target's own screen, excluding the "+
		"looks it was pressed on:\n", looksTotal)
	pc := func(n int) float64 {
		if looksTotal == 0 {
			return 0
		}
		return 100 * float64(n) / float64(looksTotal)
	}
	fmt.Printf("      matched      %6d  %5.1f%%\n", matched, pc(matched))
	fmt.Printf("      ambiguous    %6d  %5.1f%%   (abstention, not an error)\n",
		ambiguous, pc(ambiguous))
	fmt.Printf("      unobservable %6d  %5.1f%%   (abstention, not an error)\n",
		unobservable, pc(unobservable))
	fmt.Printf("        of which %d could not be told from a region that merely "+
		"CONTAINED the press\n", indistinct)
	fmt.Printf("\n  GROUND TRUTH, from second presses only — the one check nobody " +
		"labelled:\n")
	fmt.Printf("      %d verified, %d contradicted, %d declined\n",
		verified, contradicted, declined)
	fmt.Printf("      a second press that landed on the rectangle reacquisition had " +
		"already named,\n      computed before that press was folded.\n")
	fmt.Printf("\n  false matches: %d colliding pair(s) over %d look(s) — two targets "+
		"on one screen\n                 matched to the same rectangle, so at least "+
		"one was wrong\n", collisions, collisionLooks)
}

// printScreenFreeReacquisition reports the same measurement with the screen dropped.
//
// Side by side with the screen-scoped one above, because the only useful reading is the
// difference. Three questions, in the order they decide anything:
//
//  1. Did dropping the screen produce WRONG answers? Collisions and contradictions are the two
//     kinds of wrongness available without a person, and they are printed first.
//
//  2. Did it gain reacquisitions? A control found anywhere is a weaker claim than one found on
//     the screen it was demonstrated on, and the gain is what that weakening buys.
//
//  3. Did it gain them where it mattered — on the looks a second press could verify?
//
//     Do not assume local matching is reliable merely because global screen matching failed.
func printScreenFreeReacquisition(scores map[observe.VisualTargetID]*targetScore,
	collisions, collisionLooks, looks int) {

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE SAME CONTROLS, WITH NO SCREEN BOUND\n\n")
	fmt.Printf("  What a replay grounded per control would see: every demonstrated target\n")
	fmt.Printf("  against every look, with no screen scoping anything. Experiment 076 is\n")
	fmt.Printf("  why the question moved — seven measures fail to separate a correct screen\n")
	fmt.Printf("  binding from an incorrect one.\n\n")

	ids := make([]observe.VisualTargetID, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var matched, ambiguous, unobservable, indistinct, total, elsewhere int
	var verified, contradicted, declined int
	fmt.Printf("  %-12s %7s %7s %9s %12s %11s %s\n",
		"TARGET", "looks", "matched", "ambiguous", "unobservable", "indistinct",
		"matched elsewhere")
	for _, id := range ids {
		s := scores[id]
		matched += s.matched
		ambiguous += s.ambiguous
		unobservable += s.unobservable
		indistinct += s.indistinct
		total += s.looks
		elsewhere += s.elsewhere
		verified += s.verified
		contradicted += s.contradicted
		declined += s.declined
		fmt.Printf("  %-12s %7d %7d %9d %12d %11d %d\n",
			id, s.looks, s.matched, s.ambiguous, s.unobservable, s.indistinct,
			s.elsewhere)
	}
	if total == 0 {
		fmt.Printf("\n  no target-look to score.\n")
		return
	}
	fmt.Printf("\n  over %d target-look(s), excluding the looks a target was pressed on:\n", total)
	fmt.Printf("      matched        %5d   %4.1f%%\n", matched, 100*float64(matched)/float64(total))
	fmt.Printf("      ambiguous      %5d   %4.1f%%   (abstention, not an error)\n",
		ambiguous, 100*float64(ambiguous)/float64(total))
	fmt.Printf("      unobservable   %5d   %4.1f%%   (abstention, not an error)\n",
		unobservable, 100*float64(unobservable)/float64(total))
	fmt.Printf("        of which %d could not be told from a region that merely CONTAINED "+
		"the press\n", indistinct)

	// THE NUMBER THAT DECIDES IT, and it takes only one control to be wrong.
	fmt.Printf("\n  matched on a look of a screen it was NOT demonstrated on: %d"+
		" (%.1f%% of matches)\n", elsewhere, 100*float64(elsewhere)/float64(max(matched, 1)))
	if elsewhere > 0 {
		fmt.Printf("      Every one is a look on which a replay looking for this control\n")
		fmt.Printf("      alone would have been prepared to press something that is not\n")
		fmt.Printf("      there. Unlike a collision it takes only one control to be wrong.\n")
	}

	// THE WRONGNESS, FIRST AMONG THE CONCLUSIONS.
	fmt.Printf("\n  GROUND TRUTH, from second presses only:\n")
	fmt.Printf("      %d verified, %d contradicted, %d declined\n",
		verified, contradicted, declined)
	fmt.Printf("\n  collisions: %d pair(s) over %d look(s)\n", collisions, collisionLooks)
	if collisions == 0 {
		fmt.Printf("      no two controls claimed one rectangle, even with every control\n")
		fmt.Printf("      of every menu competing. That is the cost the screen was paying\n")
		fmt.Printf("      for, and on this recording it was zero.\n")
	} else {
		fmt.Printf("      AT LEAST %d WRONG. Dropping the screen let controls from\n", collisions)
		fmt.Printf("      different menus claim one rectangle, which the screen scoping\n")
		fmt.Printf("      was preventing.\n")
	}
	fmt.Printf("\n  A match is not permission, and a match rate is not a rate of right\n")
	fmt.Printf("  answers. The two numbers above that a person did not label are the\n")
	fmt.Printf("  contradictions and the collisions.\n")
}
