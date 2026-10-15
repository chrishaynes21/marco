package main

// The seven stages, each reporting what it actually got and what it actually lacked.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── 1. ARTIFACT RELOAD AND VALIDATION ─────────────────────────────────────────

func stageReload(r *readiness, a learnedFold, half int, frames []frameRecord) (
	workflowstore.Artifact, []byte, workflowstore.Artifact) {

	// THE EVIDENCE FOR EACH SCREEN THE DRAFT NAMES, both halves of it.
	//
	// Through SaveWith rather than Save, so a saved destination carries what it was
	// READ to say and not only where its rectangles were. An artifact without that
	// cannot have an arrival corroborated by a reading, and the arrangement alone was
	// measured to arrive on the wrong page — see
	// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]]. A screen
	// nobody read is still saved, as unread rather than as silent.
	arrangements := map[observe.VisualScreenID]observe.VisualArrangement{}
	for id, ar := range a.arr {
		arrangements[id] = ar
	}
	said := map[observe.VisualScreenID]observe.ScreenText{}
	for id, t := range a.said {
		said[id] = t
	}
	art, err := workflowstore.SaveWith(a.draft, arrangements, said,
		time.Unix(frames[half-1].AtMS/1000, 0).UTC())
	if err != nil {
		r.add(stage{n: 1, name: "artifact reload and validation",
			state: stageRefused, missing: err.Error()})
		return art, nil, workflowstore.Artifact{}
	}
	body, err := art.Marshal()
	if err != nil {
		r.add(stage{n: 1, name: "artifact reload and validation",
			state: stageRefused, missing: err.Error()})
		return art, nil, workflowstore.Artifact{}
	}
	loaded, err := workflowstore.Unmarshal(body)
	if err != nil {
		r.add(stage{n: 1, name: "artifact reload and validation",
			state: stageRefused, missing: err.Error()})
		return art, body, workflowstore.Artifact{}
	}
	r.add(stage{n: 1, name: "artifact reload and validation", state: stageReady,
		detail: fmt.Sprintf("%d bytes, version %d, %d step(s), %d screen(s), "+
			"%d control(s); validated on the way in",
			len(body), loaded.Version, len(loaded.Steps), len(loaded.Screens),
			len(loaded.Controls))})
	return art, body, loaded
}

// ── 2-7. THE STEP-SCOPED WALK ─────────────────────────────────────────────
//
// One walk, because the stages are not separable in a sequential workflow: a step's screen is
// established, its control looked for, its confirmation checked and its action lowered AT THE
// SAME MOMENT, and the next step's turn comes only after the observation has moved.
//
// Stage counts are therefore "how many steps reached this stage at least once", which is the
// honest unit. A step that reached lowering on one look and not on the next reached lowering.

// stepWalk drives fold B and examines each step against the observation it is in.
func stepWalk(r *readiness, loaded workflowstore.Artifact, a learnedFold, oracle fold,
	regions [][]observe.Region, readable []bool, frames []frameRecord, half int,
	feed *slowFeed, authorised bool) {

	session := workflowstore.NewSessionBinding()
	labels := workflowstore.Labels(a.draft)
	truthOf := map[string]observe.VisualScreenID{}
	for _, sc := range summarise(a.fold, oracle, regions) {
		if l, ok := labels[sc.id]; ok {
			truthOf[l] = sc.truth
		}
	}
	// SIMULATED: a person would be shown each step and its evidence. Every step is
	// confirmed up front because confirmation is about the workflow, not about where
	// Marco is standing — and the label travels with every result below.
	for i := range loaded.Steps {
		_ = session.ConfirmStep(i, "SIMULATED fixture")
	}

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		r.add(stage{n: 2, name: "fresh screen establishment", state: stageRefused,
			missing: err.Error()})
		return
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()

	reached := map[workflowstore.StepStage]map[int]bool{}
	note := func(st workflowstore.StepStage, step int) {
		if reached[st] == nil {
			reached[st] = map[int]bool{}
		}
		reached[st][step] = true
	}
	lowered := map[int]bool{}
	// arrivals is what the observation AFTER each simulated action boundary said
	// about that step's destination. Nothing was executed, so the boundary is a
	// marker and not an event — which is exactly why the result is called a
	// destination established and never an action verified.
	arrivals := map[int]workflowstore.Arrival{}
	pending := -1
	establishedScreens, inferredScreens := map[string]bool{}, map[string]bool{}
	var firstRefusal string
	var firstRefusalStage workflowstore.StepStage
	cursor, reachedSteps := 0, 0

	base := time.Unix(1700000000, 0).UTC()
	lastEpoch := ^uint64(0)
	var lastLabel string
	for i := half; i < len(frames); i++ {
		fr := frames[i]
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front: true, Regions: regions[i],
		}
		if !readable[i] {
			lk.Missing = true
		}
		lk.Slow = feed.due(i + 1)

		standing := obs.Screen()
		here := obs.Candidates(lk)

		// ── ESTABLISH, ONCE PER OBSERVATION ──
		up := obs.Observe(lk)
		if up.Settled {
			feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
		if up.Epoch != lastEpoch {
			lastEpoch = up.Epoch
			now := obs.Screen()
			if now == "" || now == observe.VisualScreenUnknown ||
				now == observe.VisualScreenUnobserved {
				session.Observed(up.Epoch, fr.WindowGeneration)
				lastLabel = ""
				continue
			}
			arr := obs.Arrangement(now)
			done := false
			if lastLabel != "" {
				for _, to := range nextLabels(loaded, lastLabel) {
					agreed := observe.ArrangementSimilarity(
						arrangementOf(loaded, to), arr)
					if b, ierr := session.Infer(loaded, lastLabel, to, now,
						agreed, up.Epoch, fr.WindowGeneration); ierr == nil {
						lastLabel, done = b.Label, true
						inferredScreens[b.Label] = true
						break
					}
				}
			}
			if !done {
				proposed := bestLabel(loaded, arr)
				// SIMULATED: the oracle stands where a person would look.
				if proposed != "" && truthOf[proposed] == oracleName(oracle, i) &&
					oracleName(oracle, i) != "" {
					if cerr := session.Confirm(proposed, now, up.Epoch,
						fr.WindowGeneration, "SIMULATED fixture",
						base); cerr == nil {
						lastLabel, done = proposed, true
						establishedScreens[proposed] = true
					}
				}
			}
			if !done {
				lastLabel = ""
			}
			session.Observed(up.Epoch, fr.WindowGeneration)
			standing = now

			// THE SIMULATED ACTION BOUNDARY, settled against THIS observation.
			// Asked after establishment has run, because it reads what that
			// path grounded and establishes nothing itself.
			if pending >= 0 {
				arrivals[pending] = loaded.DestinationAt(pending, session,
					workflowstore.Now{Screen: now, Epoch: up.Epoch,
						Generation: fr.WindowGeneration})
				pending = -1
			}
		}
		if cursor >= len(loaded.Steps) {
			continue
		}

		// ── AND EXAMINE THE STEP THE WALK IS ON ──
		var outcomes []observe.Reacquisition
		for _, st := range loaded.Targets(session.Binding(up.Epoch, fr.WindowGeneration)) {
			outcomes = append(outcomes, observe.ReacquireStored(st,
				observe.Regions(here)))
		}
		got := loaded.StepReadinessAt(cursor, session, workflowstore.Now{
			Screen: standing, Epoch: up.Epoch, Generation: fr.WindowGeneration,
			Outcomes: outcomes, Authorised: authorised,
		})
		note(got.Stage, cursor)
		if firstRefusal == "" && !got.Ready() &&
			got.Stage != workflowstore.StepNotRepeatable {
			firstRefusal, firstRefusalStage = got.Because, got.Stage
		}
		switch {
		case got.Stage == workflowstore.StepNotRepeatable:
			// A WAIT. It advances on arrival, which is the walk's own rule.
			if string(standing) != "" && loaded.Steps[cursor].To != "" {
				cursor++
				reachedSteps++
			}
		case got.Ready():
			src, lerr := marcoexec.Lower(marcoexec.Operation{
				Kind: marcoexec.KindClick, Count: 1,
				At: marcoexec.Point{
					X: int(got.Action.At.X * float64(fr.Width)),
					Y: int(got.Action.At.Y * float64(fr.Height)),
				},
			})
			if lerr == nil && driver.CheckSource(src) == nil {
				lowered[cursor] = true
				// A SIMULATED ACTION BOUNDARY. Nothing is performed; this
				// marks the point after which the next observation is the
				// one that would say whether the destination is there.
				pending = cursor
			}
			cursor++
			reachedSteps++
		}
	}

	// ── THE STAGES, FROM WHAT THE WALK ACTUALLY REACHED ──
	countOf := func(st workflowstore.StepStage) int { return len(reached[st]) }
	establishedAny := len(establishedScreens) + len(inferredScreens)
	r.add(stage{n: 2, name: "fresh screen establishment", state: stageReady, simulated: true,
		detail: fmt.Sprintf("%d screen(s) established — %d confirmed, %d inferred; "+
			"%d step(s) found their own screen standing",
			establishedAny, len(establishedScreens), len(inferredScreens),
			len(loaded.Steps)-countOf(workflowstore.StepScreenUnestablished))})

	resolved := len(loaded.Steps) - countOf(workflowstore.StepScreenUnestablished) -
		countOf(workflowstore.StepControlUnresolved)
	if countOf(workflowstore.StepControlUnresolved) > 0 || resolved <= 0 {
		r.add(stage{n: 3, name: "stored-control reacquisition", state: stageReady,
			detail: fmt.Sprintf("%d step(s) could not find their control, %d could",
				countOf(workflowstore.StepControlUnresolved),
				countOf(workflowstore.StepUnconfirmed)+
					countOf(workflowstore.StepUnauthorised)+
					countOf(workflowstore.StepPlannable))})
	} else {
		r.add(stage{n: 3, name: "stored-control reacquisition", state: stageReady,
			detail: "every step that found its screen also found its control"})
	}

	if n := countOf(workflowstore.StepNeedsClarification); n > 0 {
		r.add(stage{n: 4, name: "control clarification", state: stageRefused,
			detail:  fmt.Sprintf("%d step(s) wait on a question nobody has answered", n),
			missing: "a person's answer; nothing may answer for them"})
	} else {
		r.add(stage{n: 4, name: "control clarification", state: stageReady,
			detail: "no step is blocked on an unanswered question"})
	}

	unconf, unauth := countOf(workflowstore.StepUnconfirmed), countOf(workflowstore.StepUnauthorised)
	switch {
	case unauth > 0 && !authorised:
		r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
			state: stageRefused, simulated: true,
			detail: fmt.Sprintf("%d step(s) reached authorisation confirmed; %d were "+
				"unconfirmed", unauth, unconf),
			missing: "the session carries no live execution authorisation, and this " +
				"experiment will not manufacture one"})
	case unconf > 0:
		r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
			state: stageRefused, simulated: true,
			missing: fmt.Sprintf("%d step(s) were not confirmed in this session", unconf)})
	default:
		r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
			state: stageReady, simulated: true,
			detail: "every step that got this far was confirmed and authorised"})
	}

	if len(lowered) == 0 {
		r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
			state: stageNotReached, missing: "no step became plannable"})
	} else {
		r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
			state: stageReady, simulated: true,
			detail: fmt.Sprintf("%d step(s) lowered into Marco and compiled; NOT run",
				len(lowered))})
	}
	// ── STAGE 7, AS FIVE SEPARATE SENTENCES ──
	//
	// The owner's table, because collapsing any two of these rows is how a simulated
	// action boundary turns into a success claim.
	var establishedDest, independentDest int
	for _, arr := range arrivals {
		if arr.Established() {
			establishedDest++
		}
		if arr.Independent() {
			independentDest++
		}
	}
	switch {
	case len(lowered) == 0:
		r.add(stage{n: 7, name: "destination established after the action boundary",
			state: stageNotReached, missing: "no action was lowered"})
	case len(arrivals) == 0:
		r.add(stage{n: 7, name: "destination established after the action boundary",
			state: stageUnobservable,
			detail: fmt.Sprintf("%d action boundary/boundaries, and no observation "+
				"followed any of them", len(lowered)),
			missing: "the recording ended before the next observation"})
	case establishedDest == 0:
		var why string
		for _, arr := range arrivals {
			why = arr.Because
			break
		}
		r.add(stage{n: 7, name: "destination established after the action boundary",
			state:   stageRefused,
			detail:  fmt.Sprintf("%d boundary/boundaries examined", len(arrivals)),
			missing: why})
	default:
		r.add(stage{n: 7, name: "destination established after the action boundary",
			state: stageReady,
			detail: fmt.Sprintf("%d of %d destination(s) established, %d of them by "+
				"somebody looking rather than by the workflow's own expectation",
				establishedDest, len(arrivals), independentDest)})
	}
	r.arrivals = arrivals

	p := workflowstore.Progress{Steps: len(loaded.Steps), Reached: reachedSteps, Stopped: -1}
	if reachedSteps < len(loaded.Steps) {
		p.Stopped, p.Because, p.Stage = cursor, firstRefusal, firstRefusalStage
	}
	r.progress = &p
}

// ── the report ────────────────────────────────────────────────────────────────

func printReadiness(r readiness) {
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE CHAIN\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	var firstBlock *stage
	for i := range r.stages {
		s := r.stages[i]
		mark := ""
		if s.simulated {
			mark = "  [SIMULATED decision]"
		}
		fmt.Printf("  %d. %-44s %s%s\n", s.n, s.name, s.state, mark)
		if s.detail != "" {
			fmt.Printf("     %s\n", wrapAt(s.detail, 66, "     "))
		}
		if s.missing != "" {
			fmt.Printf("     missing: %s\n", wrapAt(s.missing, 58, "              "))
		}
		if firstBlock == nil && (s.state == stageRefused || s.state == stageUnobservable) {
			firstBlock = &r.stages[i]
		}
	}

	// ── THE FIVE CLAIMS, KEPT APART ──
	//
	// Collapsing any two of these rows is how a simulated action boundary becomes a
	// success claim, so each is printed as its own sentence with its own answer.
	lowered, established, independent := 0, 0, 0
	for _, s := range r.stages {
		if s.n == 6 && s.state == stageReady {
			lowered = 1
		}
	}
	for _, arr := range r.arrivals {
		if arr.Established() {
			established++
		}
		if arr.Independent() {
			independent++
		}
	}
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT IS AND IS NOT CLAIMED\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	yn := func(ok bool, yes, no string) string {
		if ok {
			return yes
		}
		return no
	}
	fmt.Printf("  %-42s %s\n", "action lowered and compiled",
		yn(lowered > 0, "ready", "not reached"))
	fmt.Printf("  %-42s %s\n", "action executed", "NOT EXECUTED")
	fmt.Printf("  %-42s %s\n", "simulated action boundary",
		yn(len(r.arrivals) > 0, "present", "none"))
	fmt.Printf("  %-42s %s\n", "destination established",
		yn(established > 0,
			fmt.Sprintf("%d of %d", established, len(r.arrivals)), "neither"))
	fmt.Printf("  %-42s %s\n", "  of those, independent of the workflow",
		fmt.Sprintf("%d", independent))
	fmt.Printf("  %-42s %s\n", "action independently verified", "NOT ESTABLISHED")
	for _, arr := range r.arrivals {
		if ok, why := arr.Verified(); !ok {
			fmt.Printf("     %s\n", wrapAt(why, 66, "     "))
		}
		break
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE EARLIEST GENUINE BLOCKER\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	if r.progress != nil {
		fmt.Printf("      how far it got: %s\n\n", wrapAt(r.progress.Summary(), 64, "      "))
	}
	if firstBlock == nil {
		fmt.Printf("      None. Every stage completed, with the decisions a person would\n")
		fmt.Printf("      make supplied by fixtures and marked SIMULATED above.\n")
		return
	}
	fmt.Printf("      Stage %d, %s.\n\n      %s\n", firstBlock.n, firstBlock.name,
		wrapAt(firstBlock.missing, 66, "      "))
}
