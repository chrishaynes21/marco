package main

// THE WHOLE PATH, OFFLINE: RECORD -> LEARN -> PLAN -> LOWER -> VERIFY.
//
// # What this answers
//
// The owner put the product back in the middle of the work:
//
//	Marco is supposed to observe a user demonstrating a workflow, remember it, and reproduce it
//	later. […] The next milestone is not another observation benchmark. It is a workflow Marco can
//	remember and eventually perform without making me demonstrate it again.
//
// and set the ground rules: use the recording we already have, build the learning and
// replay-PLANNING path offline first, show what Marco can confidently reproduce and what still
// needs clarification, and inject nothing.
//
// So this drives the production pieces over the 900-frame recording and reports the workflow it
// learned, what it would do on the later passes, whether the recording's own screens agree, and
// the Marco program one step lowers to. Nothing is executed and nothing is written.
//
// # The lowering lives HERE and not in the workflow package
//
// `internal/director/workflow` has an import walk that fails if it can reach `marcoexec`: a draft
// that could lower itself would be an autonomous actuator with a reading interface. So the plan is
// handed OUT, and this file — a replay tool, which cannot be reached by anything live — is what
// takes the last step, to prove the path arrives at legal Marco.
//
// # Where a confirmation comes from, and that it is being faked here
//
// A step becomes executable only when a person confirms it. There is no person in a replay, so
// this file confirms the steps ITSELF and says so in its output, loudly, every time. The point of
// the exercise is the resolution and verification machinery; the authority is the owner's and is
// not being claimed.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// stepTrial is what happened when one learned step met a later look of its own screen.
type stepTrial struct {
	planned  int
	refused  map[string]int
	arrived  int
	elsewhre int
	waiting  int
}

// reportWorkflowOverRecording learns a workflow from the recording and plans it back.
//
// `onnx` names recorded detections from `vision batch`, replayed `delay` looks behind the frame
// they describe. Empty means the pixel proposer alone, which is what every earlier measurement
// of this workflow was taken with and is the comparison that matters.
func reportWorkflowOverRecording(dir string, limit int, onnx string, delay int) error {
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
	// ONE FEED PER PASS, loaded fresh each time. The two passes are independent
	// observers over one recording; a shared feed would let pass one's requests
	// satisfy pass two's, and pass two would measure a source that never waited.
	reload := func() *slowFeed {
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
	feed := reload()
	if err != nil {
		return err
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	// ── PASS ONE: watch the demonstration ──
	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return err
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	looks, err := foldRecording(dir, frames, inputs, obs, feed, nil)
	if err != nil {
		return err
	}
	fmt.Println(feed.report())
	if feed != nil {
		fmt.Println(admissionLedger(obs.Stats()))
	}
	fmt.Println()
	obs.Close()

	draft := workflow.Learn("enter free play", obs.Walk(), obs.Clicks(), obs.Registry())

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("THE WORKFLOW LEARNED FROM %s\n", filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d look(s), %d screen(s), %d step(s) in the demonstration's own order\n\n",
		looks, len(obs.Screens()), len(draft.Steps))
	fmt.Printf("  %-4s %-10s %-10s %-10s %-10s %-10s %s\n",
		"#", "from", "target", "to", "cause", "support", "repeatable")
	for i, s := range draft.Steps {
		target := string(s.Target)
		if target == "" {
			target = "-"
		}
		fmt.Printf("  %-4d %-10s %-10s %-10s %-10s %-10s %v\n",
			i, s.From, target, s.To, s.Cause, s.Support(), s.Repeatable())
	}

	fmt.Printf("\n  what still needs clarification:\n")
	un := draft.Unresolved()
	if len(un) == 0 {
		fmt.Printf("      nothing, which would be surprising\n")
	}
	for _, u := range un {
		fmt.Printf("      - %s\n", wrapAt(u, 68, "        "))
	}

	// ── THE CONFIRMATION, WHICH IS BEING FAKED ──
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  CONFIRMING EVERY REPEATABLE STEP AS THOUGH A PERSON HAD. There is no\n")
	fmt.Printf("  person in a replay. A step is executable only when somebody agrees to it,\n")
	fmt.Printf("  and this tool agreeing on their behalf is a stand-in for the machinery\n")
	fmt.Printf("  below, never a claim of authority. Nothing is executed either way.\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	confirmed := 0
	for i := range draft.Steps {
		if err := draft.Confirm(i, "perceptionreplay (stand-in)"); err == nil {
			confirmed++
		}
	}
	fmt.Printf("  %d of %d step(s) confirmed; the rest have nothing to press.\n",
		confirmed, len(draft.Steps))
	if !draft.Executable() {
		fmt.Printf("  the draft is still not executable, which is a defect if every " +
			"repeatable step was confirmed\n")
	}

	// ── PASS TWO: plan it back against the same pixels ──
	obs2, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return err
	}
	go func() {
		for range obs2.Updates() {
		}
	}()
	// TWO QUESTIONS, AND THE FIRST VERSION OF THIS CONFLATED THEM.
	//
	// "How far can the workflow get, in order?" is the product question, and the answer
	// is a cursor walk that stops the first time a control cannot be resolved — because
	// that is what a performance must do.
	//
	// "Could each step's control be resolved at all?" is the capability question, and a
	// cursor cannot answer it: the walk stops, and every step after the stop goes
	// unexamined. The first version reported only the prefix and called it the workflow.
	//
	// So both, separately. Reporting one as the other is how a workflow that gets one
	// step in reads as a workflow that works.
	trials := map[int]*stepTrial{}
	var lowered string
	var loweredAt int
	reach, stopAt, stopWhy := 0, -1, ""
	cursor := 0
	var pending *workflow.Action

	// A SECOND FEED FOR THE SECOND PASS. The two passes are separate observers over
	// the same recording, so sharing one feed would let the first pass's requests
	// satisfy the second's — and the second pass would measure a source that
	// answered instantly.
	feed2 := reload()
	_, err = foldRecording(dir, frames, inputs, obs2, feed2, func(look int, lk fastobserve.Look) {
		standing := obs2.Screen()
		// THE SAME SET A PRESS WOULD BE JUDGED AGAINST. A target the detector
		// made findable is only found again if reacquisition can see the
		// detector's rectangles too — and `observe.Admissible` is applied here
		// rather than restated, so the search cannot be more permissive than
		// the fold.
		got := obs2.Reacquire(lk)

		// ── THE CAPABILITY QUESTION, every step, independently ──
		//
		// Steps 0 and 11 are the same control on the same screen — the person clicked
		// it on both passes — so they resolve identically. That is correct and not a
		// double count: they are two steps of the workflow and both have to work.
		for i := range draft.Steps {
			if !draft.Steps[i].Repeatable() || draft.Steps[i].From != standing {
				continue
			}
			t := trial(trials, i)
			if _, ref := workflow.Plan(draft, i, standing, got); ref != nil {
				key := string(ref.Result)
				if key == "" {
					key = "refused"
				}
				t.refused[key]++
				continue
			}
			t.planned++
		}

		// ── THE PRODUCT QUESTION, in order, stopping where a run would ──
		if stopAt >= 0 || cursor >= len(draft.Steps) {
			return
		}
		step := draft.Steps[cursor]
		if !step.Repeatable() {
			// A WAIT. The right response to a load completing is to watch for
			// the destination, and arriving is what advances.
			if standing == step.To {
				trial(trials, cursor).arrived++
				cursor++
				reach = cursor
			} else {
				trial(trials, cursor).waiting++
			}
			return
		}
		if pending != nil {
			switch workflow.Verify(*pending, step.From, standing) {
			case workflow.VerdictArrived:
				trial(trials, cursor).arrived++
				pending = nil
				cursor++
				reach = cursor
			case workflow.VerdictElsewhere:
				// THE EXPECTED TRANSITION DID NOT HAPPEN. A run stops: it
				// has lost its place, and acting on a workflow that does
				// not know where it is presses something nobody asked for.
				trial(trials, cursor).elsewhre++
				stopAt, stopWhy = cursor, fmt.Sprintf(
					"the step expected %s and the screen became %s",
					pending.Expect, standing)
				pending = nil
			case workflow.VerdictWaiting:
				trial(trials, cursor).waiting++
			}
			return
		}
		if standing != step.From {
			return
		}
		act, ref := workflow.Plan(draft, cursor, standing, got)
		if ref != nil {
			// NOT YET A STOP. The control may simply not be drawn on this look —
			// a screen settles before its contents are proposed — so a run waits
			// on its own screen and gives up when the screen itself goes away.
			if standing != step.From {
				stopAt, stopWhy = cursor, ref.Because
			}
			return
		}
		pending = act
		if lowered == "" {
			if src, lerr := lowerClick(*act, lk.Window.Bounds); lerr == nil {
				lowered, loweredAt = src, cursor
			}
		}
	})
	if err != nil {
		return err
	}
	// THE WALK ENDED WITHOUT A STOP AND WITHOUT FINISHING, which is its own outcome: the
	// recording ran out while the workflow was still waiting on a control.
	if stopAt < 0 && cursor < len(draft.Steps) {
		stopAt = cursor
		stopWhy = "the recording ended while this step was still waiting for its " +
			"control to be resolvable"
	}
	obs2.Close()

	printWorkflowTrials(draft, trials, reach, stopAt, stopWhy)
	if lowered != "" {
		fmt.Printf("\n  %s\n", repeat("-", 74))
		fmt.Printf("  STEP %d, LOWERED INTO LEGAL MARCO. Not run. The point computed\n",
			loweredAt)
		fmt.Printf("  from the look in front of the planner, never from the\n")
		fmt.Printf("  demonstration's own press position — which is not stored.\n")
		fmt.Printf("  %s\n", repeat("-", 74))
		for _, line := range strings.Split(strings.TrimRight(lowered, "\n"), "\n") {
			fmt.Printf("  | %s\n", line)
		}
	}
	return nil
}

// trial finds or starts one step's record.
func trial(m map[int]*stepTrial, i int) *stepTrial {
	if t, ok := m[i]; ok {
		return t
	}
	t := &stepTrial{refused: map[string]int{}}
	m[i] = t
	return t
}

// lowerClick turns a planned action into a Marco program.
//
// The normalised point becomes pixels against the window IN FRONT OF THE PLANNER. That is the
// whole reason the demonstration's own coordinates are not stored: the same control in a window
// that moved or resized is a different pixel and the same fraction.
func lowerClick(a workflow.Action, bounds directorapi.Rect) (string, error) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return "", fmt.Errorf("no window to resolve a point against")
	}
	return marcoexec.Lower(marcoexec.Operation{
		Kind:  marcoexec.KindClick,
		Count: 1,
		At: marcoexec.Point{
			X: bounds.X + int(a.At.X*float64(bounds.Width)),
			Y: bounds.Y + int(a.At.Y*float64(bounds.Height)),
		},
	})
}

func printWorkflowTrials(d workflow.Draft, trials map[int]*stepTrial,
	reach, stopAt int, stopWhy string) {
	// ── HOW FAR IT GOT ──
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  HOW FAR THE WORKFLOW GETS, in order, stopping where a run would:\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("      %d of %d step(s) completed before it stopped.\n", reach, len(d.Steps))
	if stopAt >= 0 {
		fmt.Printf("      stopped at step %d (%s -> %s):\n        %s\n",
			stopAt, d.Steps[stopAt].From, d.Steps[stopAt].To,
			wrapAt(stopWhy, 66, "        "))
	}
	fmt.Printf("      This is the product answer and it is the one that matters. A\n")
	fmt.Printf("      workflow that resolves most of its controls somewhere is still a\n")
	fmt.Printf("      workflow that cannot be performed.\n")

	fmt.Printf("\n  every step's control, asked independently on every look of its own\n")
	fmt.Printf("  screen — the capability question, which a stopped walk cannot answer:\n\n")
	fmt.Printf("  %-4s %-10s %-10s %8s  %s\n",
		"#", "from", "target", "resolved", "refusals")
	idx := make([]int, 0, len(trials))
	for i := range trials {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	var planned int
	resolvable := 0
	repeatable := 0
	for _, i := range idx {
		t := trials[i]
		if !d.Steps[i].Repeatable() {
			continue
		}
		repeatable++
		planned += t.planned
		if t.planned > 0 {
			resolvable++
		}
		var refs []string
		for k, n := range t.refused {
			refs = append(refs, fmt.Sprintf("%s=%d", k, n))
		}
		sort.Strings(refs)
		fmt.Printf("  %-4d %-10s %-10s %8d  %s\n",
			i, d.Steps[i].From, d.Steps[i].Target, t.planned,
			strings.Join(refs, " "))
	}
	fmt.Printf("\n      %d of %d repeatable step(s) could be resolved on at least one\n",
		resolvable, repeatable)
	fmt.Printf("      look of their own screen, over %d resolution(s) in total.\n", planned)
	fmt.Printf("      `resolved` counts LOOKS and consecutive looks of one screen are\n")
	fmt.Printf("      not independent trials — a step resolved on 40 looks of one visit\n")
	fmt.Printf("      was resolved once, forty times. The column that matters is how\n")
	fmt.Printf("      many steps have a non-zero count at all.\n")
}

// wrapAt breaks a long line for the report.
func wrapAt(s string, width int, indent string) string {
	var out []string
	line := ""
	for _, w := range strings.Fields(s) {
		if len(line)+len(w)+1 > width && line != "" {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		out = append(out, line)
	}
	return strings.Join(out, "\n"+indent)
}

// foldRecording folds every frame through an observer, calling `each` before the fold.
//
// With a slow feed it also plays the second source at the delay it really has: deliver
// whatever is due onto this look, fold, and ask again if the look settled a screen. The
// request is stamped with the context it was made in, which is what lets the observer
// decide later whether that context survived.
func foldRecording(dir string, frames []frameRecord, inputs []inputRecord,
	obs *fastobserve.Observer, feed *slowFeed,
	each func(look int, lk fastobserve.Look)) (int, error) {

	opt := structure.DefaultOptions()
	base := time.Unix(1700000000, 0).UTC()
	nextInput, look := 0, 0
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
		// WHAT THE SLOW SOURCE HAS FINISHED, before the look it arrives on is
		// folded. Nothing is reprojected or adjusted: these are rectangles from
		// an earlier frame, offered as such.
		lk.Slow = feed.due(look)
		if each != nil {
			each(look, lk)
		}
		up := obs.Observe(lk)
		// ASK ON A SETTLE, which is the cheap change detection the observer already
		// computes for its own reasons. Not per look: that would be the thing the
		// two-speed design exists to avoid.
		if up.Settled {
			feed.request(look, fr.WindowGeneration, up.Epoch)
		}
	}
	return look, nil
}
