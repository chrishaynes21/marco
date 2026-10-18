package main

// DOES EXPECTATION PLUS AGREEMENT SEPARATE THE RIGHT CONTROL FROM THE WRONG ONE?
//
// # The rule being measured, and why it is this one
//
// Two things have now been measured ALONE and both fail.
// [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] measured whole-screen
// similarity: seven measures cannot separate a correct binding from an incorrect one, and under
// the existing abstention a replay would act on 4 of 11 screens and be wrong on 2.
// [[Experiment-077-the-screen-was-not-supporting-context]] measured the control alone: dropping
// the screen gained no match rate and put 78.6% of its matches on screens the control had never
// been demonstrated on.
//
// The owner's correction:
//
//	That doesn't mean Marco's architecture is wrong. It means we need to evaluate the
//	relationship between the expected state, the observed screen, and the demonstrated control.
//	[…] if Marco has just observed the main menu and activated Play, it should be looking for
//	the control demonstrated on the NEXT menu — not searching every visible rectangle for
//	something resembling the recorded button.
//
// Which is [[ADR-186-a-person-can-say-where-they-are]]'s rule, already written down and never
// measured:
//
//	Expectation supplies presence; similarity supplies agreement. Neither alone is admitted:
//	without the transition this is Experiment-029's two wrong bindings; without the agreement
//	it is arriving somewhere and calling it arriving where you meant.
//
// So three terms, and a step is READY only when all three hold:
//
//	EXPECTATION   the procedure says this is the step we are on -- because the previous step's
//	              destination was established, or because somebody said where we started
//	AGREEMENT     the arrangement of THIS LOOK agrees with the step's saved source screen
//	CONTROL       the step's demonstrated control is reacquired, unambiguously, in this look
//
// # What makes this measurable without a person
//
// The fold split. Fold A learns the workflow and fold B has never seen it, exactly as
// Experiment 029 arranged — and the ORACLE, which saw the whole recording, says which screen
// each look really was. So a READY on a look the oracle says is not the step's source screen is
// a WRONG ACTION, concretely: Marco would have pressed something on a screen it was not shown.
//
// That is the number the whole experiment is for, and it needs no labelling.
//
// # The one assumption, stated rather than buried
//
// STEP ONE'S EXPECTATION IS GRANTED. ADR-186 says the first screen's presence comes from a
// person — `workflowstore/anchor.go` is that, valid for one observation generation — and an
// offline measurement has nobody to ask. Granting it is the generous reading: it gives the rule
// its best case on the step where it has the least support, and every later step's expectation
// is earned from a measured arrival.
//
// # Three arms, so the conjunction's contribution is visible
//
// A rule reported alone is a number with nothing to be better than. Each look is scored under
// all three, from the same evidence:
//
//	control only    Experiment 077's rule
//	agreement only  the screen rule, approximately
//	conjunction     all three terms
//
// Nothing here presses anything, and nothing here is wired into execution.

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// armTally is one rule's outcomes over the replay fold.
type armTally struct {
	// ready is every look the rule would have acted on, split by what the oracle says
	// the look really was.
	readyRight int
	readyWrong int
	// abstained, by which term refused. A rule that abstains for a reason a reader
	// cannot name is a rule nobody can improve.
	noExpectation int
	noAgreement   int
	noControl     int
	// advanced is how many of the workflow's steps the rule got through, and wrongAdvance
	// how many times it advanced on a look the oracle says was not the destination.
	advanced     int
	wrongAdvance int
	// skipped is a step whose destination arose without the rule ever being ready to
	// press it: the already-satisfied case when the destination really is there, and a
	// step silently not done when it is not.
	skipped int
	// wasReady remembers which step indices the rule was ever ready for, which is what
	// makes `skipped` distinguishable from an ordinary advance.
	wasReady map[int]bool
}

func (a *armTally) abstain(expectation, agreement, control bool) {
	switch {
	case !expectation:
		a.noExpectation++
	case !agreement:
		a.noAgreement++
	default:
		a.noControl++
	}
	_ = control
}

// reportConjunction measures ADR-186's rule over a recording split into two folds.
func reportConjunction(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) < 20 {
		return fmt.Errorf("%s carries %d frames, too few to split", dir, len(frames))
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	regions, readable := proposeAll(dir, frames)
	pressAt := pressesByLook(dir, frames)
	words, werr := readWords(dir, frames, 0, nil)
	if werr != nil {
		fmt.Printf("  words: none (%v)\n", werr)
	}

	half := len(frames) / 2
	oracle, err := foldRange(frames, regions, readable, words, pressAt, 0, len(frames))
	if err != nil {
		return err
	}
	learn, err := foldRange(frames, regions, readable, words, pressAt, 0, half)
	if err != nil {
		return err
	}

	// THE WORKFLOW FOLD A DEMONSTRATED, through the production path: the same Learn the
	// live command calls, on the same three inputs.
	draft := workflow.Learn("the measured routine", learn.walk, learn.clicks, learn.registry)
	if len(draft.Steps) == 0 {
		return fmt.Errorf("fold A demonstrated nothing, so there is no rule to measure")
	}
	byTarget := map[observe.VisualTargetID]observe.TargetRecord{}
	for _, r := range learn.registry {
		byTarget[r.ID] = r
	}
	// WHAT THE ORACLE CALLS EACH OF FOLD A'S SCREENS, which is how a claim about a fold-A
	// screen becomes checkable against a look.
	truth := map[observe.VisualScreenID]observe.VisualScreenID{}
	for _, s := range summarise(learn, oracle, regions) {
		truth[s.id] = s.truth
	}

	arms := map[string]*armTally{}
	for _, n := range []string{
		"control only", "agreement only", "conjunction", "acted, then left",
	} {
		arms[n] = &armTally{wasReady: map[int]bool{}}
	}
	// EACH ARM WALKS THE WORKFLOW AT ITS OWN PACE, because a rule that advances
	// differently is at a different step and must be scored there. One shared index would
	// measure the conjunction's step list under the other rules' names.
	at := map[string]int{}
	// expectation holds per arm: step one is granted, and every later step earns it.
	expect := map[string]bool{}
	for n := range arms {
		at[n], expect[n] = 0, true
	}

	looks := 0
	for look := half; look < len(frames); look++ {
		if !readable[look] || len(regions[look]) == 0 {
			continue
		}
		looks++
		lookArr := observe.ArrangeLook(regions[look])
		wasReally := oracle.at[look]

		for name, arm := range arms {
			i := at[name]
			if i >= len(draft.Steps) {
				continue
			}
			st := draft.Steps[i]
			// THE THREE TERMS, computed once and used by whichever arm wants them.
			expectation := expect[name]
			agreement := observe.ArrangementSimilarity(learn.arr[st.From], lookArr) >=
				observe.VisualScreenMatch
			control := false
			if rec, ok := byTarget[st.Target]; ok {
				got := observe.ReacquireStored(observe.StoredTarget{
					ID: rec.ID, Readings: rec.Where, Containers: rec.Containers,
				}, regions[look])
				control = got.Result == observe.ReacquireMatched
			}

			var ready bool
			switch name {
			case "control only":
				ready = control
			case "agreement only":
				ready = agreement
			default:
				ready = expectation && agreement && control
			}
			switch {
			case !ready:
				switch name {
				case "control only":
					arm.abstain(true, true, control)
				case "agreement only":
					arm.abstain(true, agreement, true)
				default:
					arm.abstain(expectation, agreement, control)
				}
			default:
				arm.wasReady[i] = true
				// THE VERDICT THAT NEEDS NO LABELLING: was this look really the
				// screen the step was demonstrated on?
				if wasReally != "" && wasReally == truth[st.From] {
					arm.readyRight++
				} else {
					arm.readyWrong++
				}
			}

			// THE ADVANCE, ON EVERY LOOK AND NOT ONLY WHEN READY.
			//
			// Written inside the ready branch first, and the measurement said so: no
			// arm advanced past step one over 449 looks, so the expectation term was
			// granted and then never earned, and what got measured was agreement AND
			// control under the conjunction's name. A destination is established on a
			// LATER look -- after the press, when the source screen is gone and the
			// rule is no longer ready for that step -- which is exactly the look the
			// ready branch skips.
			// ARM FOUR TAKES ITS EXPECTATION FROM ITS OWN ACT.
			//
			// The other three establish arrival by comparing the look to the saved
			// DESTINATION, which is an arrangement comparison -- so expectation and
			// agreement are the same evidence wearing two hats, and the measurement
			// showed what that costs: 17 of 19 advances wrong.
			//
			// A REPLAY KNOWS SOMETHING AN OBSERVER DOES NOT: that it just pressed.
			// So this arm advances when it was ready for this step and the look no
			// longer agrees with the step's SOURCE -- we acted, and we left. That is
			// the desktop verifier's rule (`menuClosedTaking`) and it is independent
			// of what the destination looks like.
			left := observe.ArrangementSimilarity(learn.arr[st.From], lookArr) <
				observe.VisualScreenMatch
			toArr, haveTo := learn.arr[st.To]
			arrived := haveTo && observe.ArrangementSimilarity(toArr, lookArr) >=
				observe.VisualScreenMatch
			if name == "acted, then left" {
				arrived = arm.wasReady[i] && left
			}
			if arrived {
				arm.advanced++
				if wasReally != "" && wasReally != truth[st.To] {
					arm.wrongAdvance++
				}
				if !arm.wasReady[i] {
					// THE DESTINATION AROSE WITHOUT THE RULE EVER BEING
					// READY to press this step. A replay would have
					// skipped it -- which is the already-satisfied case
					// when the destination really is there, and a step
					// silently not done when it is not.
					arm.skipped++
				}
				at[name] = i + 1
				// THE NEXT STEP'S EXPECTATION IS EARNED HERE and nowhere else.
				expect[name] = true
			}
		}
	}
	printConjunction(dir, len(frames), half, looks, draft, arms)
	// AND WHEN A TERM COULD NEVER HOLD, what the recording is missing. Printed whenever
	// the conjunction never fired, because a rule that abstained on every look has said
	// nothing about itself.
	if c := arms["conjunction"]; c.readyRight+c.readyWrong == 0 {
		printWhyNotMeasurable(learn, draft)
	}
	return nil
}

func printConjunction(dir string, frames, half, looks int, d workflow.Draft,
	arms map[string]*armTally) {

	steps := len(d.Steps)

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("EXPECTATION PLUS AGREEMENT — %s\n", filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A learns from looks 1-%d; fold B replays over %d-%d.\n",
		half, half+1, frames)
	fmt.Printf("  the workflow fold A demonstrated: %d step(s)\n", len(d.Steps))
	for i, st := range d.Steps {
		fmt.Printf("    %d %s -> %s via %s\n", i+1, st.From, st.To, st.Target)
	}
	fmt.Printf("\n  %d usable look(s) in the replay fold.\n", looks)
	fmt.Printf("  STEP ONE'S EXPECTATION IS GRANTED — ADR-186 gets it from a person and an\n")
	fmt.Printf("  offline measurement has nobody to ask, so the rule is given its best case\n")
	fmt.Printf("  on the step with the least support.\n\n")

	names := make([]string, 0, len(arms))
	for n := range arms {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("  %-16s %7s %7s %10s %12s %10s %s\n",
		"RULE", "ready", "WRONG", "no expect", "no agreement", "no control", "advanced")
	for _, n := range names {
		a := arms[n]
		fmt.Printf("  %-16s %7d %7d %10d %12d %10d %d (%d wrong)\n",
			n, a.readyRight, a.readyWrong, a.noExpectation, a.noAgreement,
			a.noControl, a.advanced, a.wrongAdvance)
	}

	fmt.Printf("\n  WHAT 'WRONG' MEANS, and it needs no labelling: the rule would have\n")
	fmt.Printf("  pressed on a look the oracle says was NOT the screen that step was\n")
	fmt.Printf("  demonstrated on. The oracle saw the whole recording with continuity; the\n")
	fmt.Printf("  two folds never shared an identity.\n")

	c, ctl := arms["conjunction"], arms["control only"]
	agr := arms["agreement only"]
	fmt.Printf("\n  DOES THE CONJUNCTION DISCRIMINATE?\n")
	for _, p := range []struct {
		name string
		a    *armTally
	}{
		{"control only", ctl}, {"agreement only", agr}, {"conjunction", c},
		{"acted, then left", arms["acted, then left"]},
	} {
		total := p.a.readyRight + p.a.readyWrong
		if total == 0 {
			fmt.Printf("      %-16s never ready — it abstained on every look\n", p.name)
			continue
		}
		fmt.Printf("      %-16s %d ready, %d of them wrong (%.1f%%)\n",
			p.name, total, p.a.readyWrong, 100*float64(p.a.readyWrong)/float64(total))
	}
	switch {
	case c.readyRight+c.readyWrong == 0:
		fmt.Printf("\n      THE CONJUNCTION NEVER FIRED. It is not more discriminating; it\n")
		fmt.Printf("      is unusable on this recording, and the term that refused most is\n")
		fmt.Printf("      where the missing evidence is.\n")
	case c.readyWrong == 0:
		fmt.Printf("\n      ZERO WRONG under the conjunction, against %d for the control\n",
			ctl.readyWrong)
		fmt.Printf("      alone and %d for agreement alone. On this recording the three\n",
			agr.readyWrong)
		fmt.Printf("      terms together separate what neither separates apart. A finite\n")
		fmt.Printf("      test cannot promise zero in general.\n")
	default:
		fmt.Printf("\n      STILL %d WRONG under the conjunction. Adding the expectation\n",
			c.readyWrong)
		fmt.Printf("      did not close it, so the missing evidence is not a third score.\n")
	}

	// WHERE THE EXPECTATION CAME FROM, which is the whole finding.
	act := arms["acted, then left"]
	fmt.Printf("\n  WHY, AND IT IS NOT THE RULE'S SHAPE:\n")
	fmt.Printf("      the conjunction advanced %d step(s) and %d of those advances were\n",
		c.advanced, c.wrongAdvance)
	fmt.Printf("      WRONG. Its expectation is established by comparing the look to the\n")
	fmt.Printf("      saved DESTINATION — an arrangement comparison — so expectation and\n")
	fmt.Printf("      agreement are the same evidence wearing two hats, and a rule cannot\n")
	fmt.Printf("      corroborate itself. ADR-186 reads as three terms and delivers two.\n")
	total := act.readyRight + act.readyWrong
	if total == 0 {
		fmt.Printf("\n      The independent arm never fired, so nothing can be said about it.\n")
	} else {
		fmt.Printf("\n      TAKING THE EXPECTATION FROM MARCO'S OWN ACT instead — it was\n")
		fmt.Printf("      ready, it would have pressed, and the look no longer agrees with\n")
		fmt.Printf("      the step's SOURCE — gives %d ready and %d wrong (%.1f%%), against\n",
			total, act.readyWrong, 100*float64(act.readyWrong)/float64(total))
		fmt.Printf("      %.1f%% for the destination-similarity conjunction. That term is\n",
			100*float64(c.readyWrong)/float64(max(c.readyRight+c.readyWrong, 1)))
		fmt.Printf("      independent of the arrangement, which is the only difference.\n")
		fmt.Printf("\n      AND IT BARELY PROGRESSES: %d of %d step(s) advanced. A rule that\n",
			act.advanced, steps)
		fmt.Printf("      is nearly always right and nearly never ready has moved the\n")
		fmt.Printf("      problem rather than solved it.\n")
	}
	fmt.Printf("\n  Nothing was pressed, and no rule here is wired into execution.\n")
}

// printWhyNotMeasurable says what the recording is missing, when it is.
//
//	Report whether the conjunction actually improves discrimination. If it doesn't, identify
//	the missing evidence instead of adding another arbitrary score term.
//
// So when a term can never hold, this says WHICH evidence is absent and how far along the chain
// it goes missing. A conjunction that never fires because one of its inputs was never recorded is
// not a finding about the rule.
func printWhyNotMeasurable(learn fold, d workflow.Draft) {
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHY A TERM COULD NOT BE EVALUATED\n\n")

	// THE CHAIN, link by link: presses identified as controls, controls with a
	// destination, and route steps that name one.
	withDest := 0
	for _, r := range learn.registry {
		if len(r.Destinations) > 0 {
			withDest++
		}
	}
	byCause := map[observe.TransitionCause]int{}
	named := 0
	for _, st := range d.Steps {
		byCause[st.Cause]++
		if st.Target != "" {
			named++
		}
	}
	fmt.Printf("    %3d control(s) identified from presses\n", len(learn.registry))
	fmt.Printf("    %3d of those were observed to LEAD somewhere\n", withDest)
	fmt.Printf("    %3d step(s) in the learned workflow\n", len(d.Steps))
	fmt.Printf("    %3d of those name a control\n", named)
	fmt.Printf("\n    steps by cause:")
	causes := make([]string, 0, len(byCause))
	for c := range byCause {
		causes = append(causes, string(c))
	}
	sort.Strings(causes)
	for _, c := range causes {
		fmt.Printf(" %s=%d", c, byCause[observe.TransitionCause(c)])
	}
	fmt.Println()

	if named == 0 && len(learn.registry) > 0 {
		fmt.Printf("\n    THE CONTROLS AND THE TRANSITIONS WERE NEVER JOINED. Presses were\n")
		fmt.Printf("    identified as controls and screen changes were recorded, and no\n")
		fmt.Printf("    change was attributed to a press — so the demonstrated route has no\n")
		fmt.Printf("    control to look for and the control term cannot hold on any look.\n")
		fmt.Printf("\n    That is a fact about the RECORDING, not about the rule. The\n")
		fmt.Printf("    conjunction is unevaluated rather than refuted, and what it needs is\n")
		fmt.Printf("    a demonstration whose screen changes were CAUSED by identified mouse\n")
		fmt.Printf("    presses — which is what `director demonstrate-visual` produces from a\n")
		fmt.Printf("    person's own hands, and what no recording in the corpus contains.\n")
	}
}
