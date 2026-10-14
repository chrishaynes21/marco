package main

// The seven stages, each reporting what it actually got and what it actually lacked.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── 1. ARTIFACT RELOAD AND VALIDATION ─────────────────────────────────────────

func stageReload(r *readiness, a learnedFold, half int, frames []frameRecord) (
	workflowstore.Artifact, []byte, workflowstore.Artifact) {

	arrangements := map[observe.VisualScreenID]observe.VisualArrangement{}
	for id, ar := range a.arr {
		arrangements[id] = ar
	}
	art, err := workflowstore.Save(a.draft, arrangements,
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

// ── 2. FRESH SCREEN ESTABLISHMENT ─────────────────────────────────────────────

func stageEstablish(r *readiness, loaded workflowstore.Artifact, a learnedFold,
	oracle fold, regions [][]observe.Region, readable []bool, frames []frameRecord,
	half int, feed *slowFeed) (workflowstore.Binding, *workflowstore.SessionBinding) {

	if len(loaded.Steps) == 0 {
		r.notReached(2, "fresh screen establishment", "no artifact to establish")
		return workflowstore.Binding{}, workflowstore.NewSessionBinding()
	}
	session := workflowstore.NewSessionBinding()
	labels := workflowstore.Labels(a.draft)
	truthOf := map[string]observe.VisualScreenID{}
	for _, s := range summarise(a.fold, oracle, regions) {
		if l, ok := labels[s.id]; ok {
			truthOf[l] = s.truth
		}
	}

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		r.add(stage{n: 2, name: "fresh screen establishment",
			state: stageRefused, missing: err.Error()})
		return workflowstore.Binding{}, session
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()

	base := time.Unix(1700000000, 0).UTC()
	lastEpoch := ^uint64(0)
	var lastLabel string
	var lastEpochSeen, lastGenSeen uint64
	confirmed, inferred := 0, 0
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
		up := obs.Observe(lk)
		if up.Settled {
			feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
		lastEpochSeen, lastGenSeen = up.Epoch, fr.WindowGeneration
		if up.Epoch == lastEpoch {
			continue
		}
		lastEpoch = up.Epoch
		standing := obs.Screen()
		if standing == "" || standing == observe.VisualScreenUnknown ||
			standing == observe.VisualScreenUnobserved {
			session.Observed(up.Epoch, fr.WindowGeneration)
			lastLabel = ""
			continue
		}
		here := obs.Arrangement(standing)

		done := false
		if lastLabel != "" {
			for _, to := range nextLabels(loaded, lastLabel) {
				agreed := observe.ArrangementSimilarity(arrangementOf(loaded, to), here)
				if b, ierr := session.Infer(loaded, lastLabel, to, standing,
					agreed, up.Epoch, fr.WindowGeneration); ierr == nil {
					lastLabel, done = b.Label, true
					inferred++
					break
				}
			}
		}
		if !done {
			proposed := bestLabel(loaded, here)
			// SIMULATED. A person would look at the screen; this looks at the
			// oracle, and the result is labelled wherever it is reported.
			if proposed != "" && truthOf[proposed] == oracleName(oracle, i) &&
				oracleName(oracle, i) != "" {
				if cerr := session.Confirm(proposed, standing, up.Epoch,
					fr.WindowGeneration, "SIMULATED fixture", base); cerr == nil {
					lastLabel = proposed
					confirmed++
					done = true
				}
			}
			if !done {
				lastLabel = ""
			}
		}
		session.Observed(up.Epoch, fr.WindowGeneration)
	}

	binding := session.Binding(lastEpochSeen, lastGenSeen)
	live := session.Live(lastEpochSeen, lastGenSeen)
	detail := fmt.Sprintf("%d screen(s) established over the walk — %d confirmed, "+
		"%d inferred; %d still live in the final observation",
		confirmed+inferred, confirmed, inferred, len(live))
	if _, berr := loaded.Bind(binding); berr != nil {
		r.add(stage{n: 2, name: "fresh screen establishment", state: stageRefused,
			detail: detail, simulated: true, missing: berr.Error()})
		return binding, session
	}
	r.add(stage{n: 2, name: "fresh screen establishment", state: stageReady,
		detail: detail, simulated: true})
	return binding, session
}

// ── 3. STORED-CONTROL REACQUISITION ───────────────────────────────────────────

func stageReacquire(r *readiness, loaded workflowstore.Artifact, b workflowstore.Binding,
	frames []frameRecord, regions [][]observe.Region, readable []bool, half int,
	feed *slowFeed) []observe.StoredTarget {

	targets := loaded.Targets(b)
	if len(targets) == 0 {
		r.notReached(3, "stored-control reacquisition",
			"no control is on a screen this session established, so there is nothing "+
				"to look for")
		return nil
	}
	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		r.add(stage{n: 3, name: "stored-control reacquisition",
			state: stageRefused, missing: err.Error()})
		return targets
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()

	found := map[observe.VisualTargetID]bool{}
	base := time.Unix(1700000000, 0).UTC()
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
		here := observe.Regions(obs.Candidates(lk))
		for _, st := range targets {
			if st.Screen != standing {
				continue
			}
			if observe.ReacquireStored(st, here).Result == observe.ReacquireMatched {
				found[st.ID] = true
			}
		}
		if up := obs.Observe(lk); up.Settled {
			feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
	}
	detail := fmt.Sprintf("%d of %d stored control(s) were found again from the file's "+
		"own readings", len(found), len(targets))
	if len(found) == 0 {
		r.add(stage{n: 3, name: "stored-control reacquisition", state: stageUnobservable,
			detail:  detail,
			missing: "no proposal on any look agreed with a stored reading"})
		return targets
	}
	r.add(stage{n: 3, name: "stored-control reacquisition", state: stageReady, detail: detail})
	return targets
}

// ── 4. CONTROL CLARIFICATION ──────────────────────────────────────────────────

func stageClarify(r *readiness, loaded workflowstore.Artifact) {
	qs := loaded.Questions()
	if len(qs) == 0 {
		r.add(stage{n: 4, name: "control clarification", state: stageReady,
			detail: "no control is ambiguous enough to be worth asking about"})
		return
	}
	var names []string
	for _, q := range qs {
		names = append(names, q.Control)
	}
	// A QUESTION IS NOT A FAILURE. It is the mechanism working: the demonstration
	// could not settle this and says so. Nothing is answered here, because nothing
	// should answer on a person's behalf.
	r.add(stage{n: 4, name: "control clarification", state: stageReady,
		detail: fmt.Sprintf("%d open question(s) (%v) — asked, and deliberately "+
			"unanswered: nothing may answer for a person", len(qs), names)})
}

// ── 5. FRESH CONFIRMATION AND AUTHORISATION ───────────────────────────────────

func stageConfirm(r *readiness, loaded workflowstore.Artifact,
	b workflowstore.Binding) workflow.Draft {

	d, err := loaded.Bind(b)
	if err != nil {
		r.notReached(5, "fresh confirmation and per-action authorisation",
			"the workflow is not bound, so there is nothing to confirm")
		return workflow.Draft{}
	}
	if d.Executable() {
		r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
			state: stageRefused,
			missing: "a reloaded workflow arrived executable; stored confirmation " +
				"has become reusable authorisation"})
		return d
	}
	// SIMULATED. A person would be shown each step and its evidence.
	confirmed := 0
	for i := range d.Steps {
		if d.Steps[i].Repeatable() {
			if cerr := d.Confirm(i, "SIMULATED fixture"); cerr == nil {
				confirmed++
			}
		}
	}
	if !d.Executable() {
		r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
			state: stageRefused, simulated: true,
			detail:  fmt.Sprintf("%d step(s) confirmed", confirmed),
			missing: "the draft is still not executable after confirming every repeatable step"})
		return d
	}
	r.add(stage{n: 5, name: "fresh confirmation and per-action authorisation",
		state: stageReady, simulated: true,
		detail: fmt.Sprintf("%d repeatable step(s) confirmed afresh; the reloaded "+
			"workflow arrived unconfirmed, as it must", confirmed)})
	return d
}

// ── 6. LOWERING AND COMPILE VALIDATION ────────────────────────────────────────

func stageLower(r *readiness, d workflow.Draft, targets []observe.StoredTarget,
	b workflowstore.Binding, session *workflowstore.SessionBinding) *workflow.Action {

	if len(d.Steps) == 0 {
		r.notReached(6, "legal Marco lowering and compile validation",
			"no confirmed workflow to lower")
		return nil
	}
	// A PLAN NEEDS AN OUTCOME FOR ITS CONTROL, on the screen standing now. Built from
	// the stored readings so the lowering is of the reloaded workflow and not of the
	// session that taught it.
	var act *workflow.Action
	var refusal string
	for i := range d.Steps {
		if !d.Steps[i].Repeatable() {
			continue
		}
		var got []observe.Reacquisition
		for _, st := range targets {
			if st.Screen != d.Steps[i].From {
				continue
			}
			got = append(got, observe.Reacquisition{
				Target: st.ID, Screen: st.Screen,
				Result: observe.ReacquireMatched,
				Where:  st.Readings[0],
				Because: "SIMULATED: a matched outcome standing in for a live look, so " +
					"the lowering path can be exercised without a desktop",
			})
		}
		a, ref := workflow.Plan(d, i, d.Steps[i].From, got)
		if ref != nil {
			if refusal == "" {
				refusal = ref.Because
			}
			continue
		}
		act = a
		break
	}
	if act == nil {
		r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
			state: stageRefused, missing: refusal})
		return nil
	}
	src, err := marcoexec.Lower(marcoexec.Operation{
		Kind: marcoexec.KindClick, Count: 1,
		At: marcoexec.Point{X: int(act.At.X * 1920), Y: int(act.At.Y * 1080)},
	})
	if err != nil {
		r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
			state: stageRefused, missing: "lowering failed: " + err.Error()})
		return act
	}
	if err := driver.CheckSource(src); err != nil {
		r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
			state: stageRefused, simulated: true,
			missing: "the lowered Marco does not compile: " + err.Error()})
		return act
	}
	r.add(stage{n: 6, name: "legal Marco lowering and compile validation",
		state: stageReady, simulated: true,
		detail: fmt.Sprintf("step %d lowered into %d byte(s) of Marco and compiled; "+
			"NOT run", act.Step, len(src))})
	return act
}

// ── 7. VERIFICATION PLAN ──────────────────────────────────────────────────────

func stageVerify(r *readiness, d workflow.Draft, act *workflow.Action) {
	if act == nil {
		r.notReached(7, "verification plan",
			"no action was lowered, so there is nothing to verify")
		return
	}
	// THE PLAN, NOT A RESULT. `Verify` is asked what it WOULD say, against a screen
	// that has not changed because nothing was done.
	v := workflow.Verify(*act, d.Steps[act.Step].From, d.Steps[act.Step].From)
	if v != workflow.VerdictWaiting {
		r.add(stage{n: 7, name: "verification plan", state: stageRefused,
			missing: fmt.Sprintf("with nothing performed, verification says %q "+
				"rather than waiting; an unexecuted action has been scored", v)})
		return
	}
	r.add(stage{n: 7, name: "verification plan", state: stageReady,
		detail: fmt.Sprintf("the plan expects %s and reports %q until it arrives; "+
			"nothing was performed and nothing is claimed",
			act.Expect, v)})
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

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE EARLIEST GENUINE BLOCKER\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	if firstBlock == nil {
		fmt.Printf("      None. Every stage completed, with the decisions a person would\n")
		fmt.Printf("      make supplied by fixtures and marked SIMULATED above.\n")
		return
	}
	fmt.Printf("      Stage %d, %s.\n\n      %s\n", firstBlock.n, firstBlock.name,
		wrapAt(firstBlock.missing, 66, "      "))
}
