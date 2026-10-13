package main

// Teaching a workflow in one session and asking for it back in another.
//
// # What this measures, and what it deliberately does not
//
// Three observers over one recording, the arrangement [[Experiment-029-binding-a-screen-across-two-independent-folds]]
// established:
//
//	fold A    looks 1..half    learns the workflow and SAVES it
//	fold B    looks half..end  a fresh observer that has never heard of fold A
//	oracle    every look       the only thing that knows which screens are really the same
//
// A and B share no counter, no registry and no screen identity. The artifact crosses between
// them as a FILE, which is the whole point: everything fold B gets, it gets from bytes.
//
// # The oracle is standing in for a person, and must be read that way
//
// Something has to say which of fold B's screens a saved screen corresponds to, and
// Experiment-029 measured that geometry cannot: the relation binds the wrong screen at 0.886
// while binding right ones at 0.556. So the correspondence is SUPPLIED here, by an oracle that
// can see both folds at once, exactly where a person would say "that is the main menu".
//
// What is being measured is therefore not recognition. It is whether a saved workflow can be
// reconstructed, validated against its own stored evidence, bound, and its controls found again
// by a session that never saw the demonstration — with every refusal still intact.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// learnedFold is a fold that also watched the inputs, so it can learn.
type learnedFold struct {
	fold
	draft workflow.Draft
	obs   *fastobserve.Observer
}

// foldAndLearn runs one fresh observer over a range of looks, folding the presses that happened
// in it, and learns whatever workflow that produced.
func foldAndLearn(frames []frameRecord, inputs []inputRecord, regions [][]observe.Region,
	feed *slowFeed, lo, hi int) (learnedFold, error) {

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return learnedFold{}, err
	}
	go func() {
		for range obs.Updates() {
		}
	}()

	lf := learnedFold{obs: obs}
	lf.at = map[int]observe.VisualScreenID{}
	lf.arr = map[observe.VisualScreenID]observe.VisualArrangement{}

	base := time.Unix(1700000000, 0).UTC()
	next := 0
	for next < len(inputs) && lo > 0 && inputs[next].AtMS < frames[lo].AtMS {
		next++
	}
	for i := lo; i < hi; i++ {
		fr := frames[i]
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front:   true,
			Regions: regions[i],
		}
		for next < len(inputs) && inputs[next].AtMS <= fr.AtMS {
			in := inputs[next]
			next++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				lk.Presses = append(lk.Presses, in.Where)
			}
		}
		if len(regions[i]) == 0 {
			lk.Missing = true
		}
		lk.Slow = feed.due(i + 1)
		if up := obs.Observe(lk); up.Settled {
			feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
		lf.at[i] = obs.Screen()
	}
	for _, s := range obs.Screens() {
		lf.arr[s.ID] = obs.Arrangement(s.ID)
	}
	lf.draft = workflow.Learn("enter free play", obs.Walk(), obs.Clicks(), obs.Registry())
	return lf, nil
}

// runningFold is an observer part-way through a range, with its own slow source.
type runningFold struct {
	obs  *fastobserve.Observer
	feed *slowFeed
}

// foldStart opens a fresh observer for a range the caller will drive itself.
//
// Needed because re-acquisition has to happen BETWEEN looks — before this look is folded, on the
// screen currently believed — and a function that folds the whole range gives a caller no moment
// to ask anything.
func foldStart(frames []frameRecord, regions [][]observe.Region, feed *slowFeed,
	_ int) (runningFold, error) {

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return runningFold{}, err
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	return runningFold{obs: obs, feed: feed}, nil
}

// reportReloadOverRecording is the whole milestone.
func reportReloadOverRecording(dir string, limit int, onnx string, delay int) error {
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
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	// THE PROPOSALS, COMPUTED ONCE, so a difference between the folds can never be a
	// difference in segmentation.
	regions := make([][]observe.Region, len(frames))
	for i, fr := range frames {
		img, rerr := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if rerr != nil {
			continue
		}
		res := structure.Propose(img, structure.DefaultOptions())
		rects := make([]image.Rectangle, 0, len(res.Candidates))
		for _, c := range res.Candidates {
			rects = append(rects, c.Bounds)
		}
		regions[i] = observe.NormaliseRects(img.Bounds(), rects)
	}
	newFeed := func() *slowFeed {
		if onnx == "" {
			return nil
		}
		fd, ferr := loadSlowFeed(onnx, delay)
		if ferr != nil {
			err = ferr
			return nil
		}
		return fd
	}

	half := len(frames) / 2
	a, aerr := foldAndLearn(frames, inputs, regions, newFeed(), 0, half)
	if aerr != nil {
		return aerr
	}
	defer a.obs.Close()
	b, berr := foldAndLearn(frames, inputs, regions, newFeed(), half, len(frames))
	if berr != nil {
		return berr
	}
	defer b.obs.Close()
	oracle, oerr := foldRange(frames, regions, 0, len(frames))
	if oerr != nil {
		return oerr
	}
	if err != nil {
		return err
	}

	// ── SAVE, FROM FOLD A ──
	arrangements := map[observe.VisualScreenID]observe.VisualArrangement{}
	for id, ar := range a.arr {
		arrangements[id] = ar
	}
	art, serr := workflowstore.Save(a.draft, arrangements,
		time.Unix(frames[half-1].AtMS/1000, 0).UTC())
	if serr != nil {
		return serr
	}
	body, merr := art.Marshal()
	if merr != nil {
		return merr
	}
	// AND READ IT BACK, because what fold B gets, it gets from bytes.
	loaded, uerr := workflowstore.Unmarshal(body)
	if uerr != nil {
		return uerr
	}

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("A WORKFLOW TAUGHT IN ONE SESSION, ASKED FOR IN ANOTHER — %s\n",
		filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A  looks 1-%d      learned %d step(s) and saved them\n",
		half, len(a.draft.Steps))
	fmt.Printf("  fold B  looks %d-%d    a fresh observer that never saw fold A\n",
		half+1, len(frames))
	fmt.Printf("  the artifact crosses between them as %d bytes and nothing else\n",
		len(body))

	// ── THE CORRESPONDENCE, SUPPLIED ──
	screensA := summarise(a.fold, oracle, regions)
	screensB := summarise(b.fold, oracle, regions)
	truthA := map[observe.VisualScreenID]observe.VisualScreenID{}
	for _, s := range screensA {
		truthA[s.id] = s.truth
	}
	labels := workflowstore.Labels(a.draft)

	byTruth := map[observe.VisualScreenID][]workflowstore.ScreenCandidate{}
	for _, s := range screensB {
		if s.truth == "" {
			continue
		}
		byTruth[s.truth] = append(byTruth[s.truth], workflowstore.ScreenCandidate{
			ID: s.id, Arrangement: s.arr,
		})
	}
	var props []workflowstore.Proposal
	for id, label := range labels {
		props = append(props, workflowstore.Proposal{
			Label: label, Candidates: byTruth[truthA[id]],
		})
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT THE SAVED EVIDENCE SAYS ABOUT THE CORRESPONDENCE OFFERED\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  The oracle supplies the correspondence, standing where a person would\n")
	fmt.Printf("  say \"that is the main menu\". The store VALIDATES it against the stored\n")
	fmt.Printf("  arrangement and may refuse. Agreement is necessary and not sufficient.\n\n")
	outcomes := loaded.BindScreens(props)
	counts := map[workflowstore.BindResult]int{}
	for _, o := range outcomes {
		counts[o.Result]++
		fmt.Printf("  %-9s %-15s %s\n", o.Label, o.Result, scoreList(o.Scores))
	}
	fmt.Printf("\n      %d bound, %d ambiguous, %d with no counterpart\n",
		counts[workflowstore.BindBound], counts[workflowstore.BindAmbiguous],
		counts[workflowstore.BindNoCounterpart])

	binding := workflowstore.BindingFrom(outcomes)
	bound, berr2 := loaded.Bind(binding)
	if berr2 != nil {
		fmt.Printf("\n      the workflow as a whole is NOT bound:\n        %s\n",
			wrapAt(berr2.Error(), 64, "        "))
		fmt.Printf("      Which is the right answer and not a failure: fold B did not\n")
		fmt.Printf("      visit every screen fold A demonstrated on, and a workflow\n")
		fmt.Printf("      bound where it could would act before discovering it was lost.\n")
	}

	// ── RE-ACQUIRING THE SAVED CONTROLS, FROM STORED EVIDENCE ──
	targets := loaded.Targets(binding)
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  FINDING THE SAVED CONTROLS AGAIN, FROM THE FILE'S OWN READINGS\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  No session registry is involved. Each control is matched from the\n")
	fmt.Printf("  readings the artifact kept, against what fold B's proposer found.\n\n")

	type tally struct {
		matched, ambiguous, unobservable, looks int
	}
	per := map[observe.VisualTargetID]*tally{}
	diag := map[observe.VisualTargetID]*controlDiagnosis{}
	for _, st := range targets {
		per[st.ID] = &tally{}
		diag[st.ID] = &controlDiagnosis{target: st}
	}
	// ── A THIRD FOLD, AND IT HAS TO BE A FOLD ──
	//
	// The first version of this re-read fold B's looks and asked its finished observer
	// what was on each one. That is wrong in a way that produced confident numbers: an
	// observer holds admitted detections until its observation moves on, so after the
	// fold its held set is whatever survived to the LAST look of the recording. Asking it
	// about look 460 handed back the detections of look 900, on every look, and the
	// report said "detector evidence available on 28 of 28 looks" about a screen whose
	// detections were never there.
	//
	// So re-acquisition runs INSIDE a fold, against the candidate set as of that look.
	// Fold C is a fresh observer over fold B's range with a fresh feed; it is deterministic
	// over the same frames, so the binding computed from B applies to it — and that is
	// asserted below rather than assumed.
	c, cerr := foldStart(frames, regions, newFeed(), half)
	if cerr != nil {
		return cerr
	}
	defer c.obs.Close()
	if err != nil {
		return err
	}
	base := time.Unix(1700000000, 0).UTC()
	drift := 0
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
		if len(regions[i]) == 0 {
			lk.Missing = true
		}
		lk.Slow = c.feed.due(i + 1)

		// THE CANDIDATE SET AS OF THIS LOOK, read before folding it, which is the
		// same order the live path uses: a reacquisition is about the look in front
		// of you, judged on the screen you currently believe you are on.
		standing := c.obs.Screen()
		here := c.obs.Candidates(lk)
		regionsHere := observe.Regions(here)
		for _, st := range targets {
			if st.Screen != standing {
				continue
			}
			t := per[st.ID]
			t.looks++
			diag[st.ID].observeControl(here)
			switch observe.ReacquireStored(st, regionsHere).Result {
			case observe.ReacquireMatched:
				t.matched++
			case observe.ReacquireAmbiguous:
				t.ambiguous++
			default:
				t.unobservable++
			}
		}
		if up := c.obs.Observe(lk); up.Settled {
			c.feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
		if c.obs.Screen() != b.at[i] {
			drift++
		}
	}
	if drift > 0 {
		fmt.Printf("\n  *** fold C disagreed with fold B on %d of %d look(s). The binding\n",
			drift, len(frames)-half)
		fmt.Printf("  *** was computed from B and applied to C, so these numbers are\n")
		fmt.Printf("  *** about a screen correspondence that does not hold.\n")
	}

	ids := make([]string, 0, len(per))
	for id := range per {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	fmt.Printf("  %-10s %-10s %7s %8s %9s %12s\n",
		"control", "on", "looks", "matched", "ambiguous", "unobservable")
	resolvable := 0
	for _, id := range ids {
		t := per[observe.VisualTargetID(id)]
		var on observe.VisualScreenID
		for _, st := range targets {
			if string(st.ID) == id {
				on = st.Screen
			}
		}
		if t.matched > 0 {
			resolvable++
		}
		fmt.Printf("  %-10s %-10s %7d %8d %9d %12d\n",
			id, on, t.looks, t.matched, t.ambiguous, t.unobservable)
	}
	fmt.Printf("\n      %d of %d saved control(s) on a bound screen were found again\n",
		resolvable, len(targets))
	fmt.Printf("      by a session that never saw the demonstration.\n")
	printControlDiagnoses(diag)

	// ── AND STILL NOT AUTHORITY ──
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT IS STILL REFUSED\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	if berr2 == nil {
		fmt.Printf("      bound, and still not executable: %t\n", !bound.Executable())
		if _, ref := workflow.Plan(bound, 0, bound.Steps[0].From, nil); ref != nil {
			fmt.Printf("        %s\n", wrapAt(ref.Because, 64, "        "))
		}
	} else {
		fmt.Printf("      the workflow is not bound, so there is nothing to confirm and\n")
		fmt.Printf("      nothing that could be planned. Fresh confirmation would come\n")
		fmt.Printf("      after a complete binding, never before it.\n")
	}
	return nil
}

// scoreList renders a candidate's scores, strongest first.
func scoreList(m map[observe.VisualScreenID]float64) string {
	if len(m) == 0 {
		return "(nothing proposed)"
	}
	type kv struct {
		id observe.VisualScreenID
		v  float64
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
	out := ""
	for i, e := range all {
		if i > 0 {
			out += "  "
		}
		out += fmt.Sprintf("%s=%.3f", e.id, e.v)
	}
	return out
}
