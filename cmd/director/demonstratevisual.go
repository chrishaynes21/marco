package main

// `director demonstrate-visual --application X "<name>"` — a demonstration from PIXELS.
//
// # What it is for
//
//	Our next milestone is transferring that same workflow artifact to a pixel-only game menu.
//	Investigate the smallest way to record a person's clicks, key actions, and relevant
//	before/after frames during an existing Director demonstration session. Produce the same
//	named workflow artifact used by desktop replay, with visual target evidence where
//	accessibility controls are unavailable.
//
// [[Experiment-075-a-demonstration-saved-named-and-replayed]] made a named routine from a
// DESKTOP demonstration: Marco performed the steps, verified them, and the artifact recorded a
// role or a label per control. A game menu offers neither. What it offers is pixels, and the
// person's own hand on the mouse.
//
// Everything needed for that already existed and nothing joined it up:
//
//	director observe-screen       the live loop -- capture, propose, fold, correlate presses
//	observe.ClickTargets          what was pressed, as a control rather than a coordinate
//	workflow.Learn                a Draft from a walk and its click evidence
//	workflowstore.SaveWith        the durable artifact, with each screen's arrangement
//	demo.Store.SaveSequence       by the name the person gave it
//
// So this command is the join, and it is deliberately four calls long. The loop is shared with
// `observe-screen` (see visualsession.go); the writing happens here, in the file a reader
// looking for the thing that writes will open.
//
// # What it records, and what it refuses to call identity
//
//	Avoid storing absolute coordinates as the durable identity of a control; coordinates may
//	still be recorded as demonstration evidence.
//
// Which is what the artifact has always done and what its tests already assert: a control's
// evidence is its READINGS — normalised rectangles, as the proposer saw them — and the
// containers it must not be confused with. A press coordinate is not written at all, because
// `ADR-181` keeps the correlator out of the planner's reach and a store that wrote them back
// would undo that in one field. `TestASavedPixelDemonstrationCarriesNoCoordinates` holds it.
//
// Timing and ordering come from the walk: `VisualScreens.Walk` is the order the demonstration
// visited its screens, and `workflow.Learn` takes the walk rather than the step list for the
// reason its own doc gives — the steps are reported most-repeated first, and a workflow is an
// order.
//
// # What it does not do
//
// It does not act, and it cannot: nothing in this file or in the loop reaches an actuator, a
// host or the execution pipeline. It does not confirm anything either — a step read back from
// this artifact comes out UNCONFIRMED, as every saved step does, and
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] is untouched. Saving is "this is
// what you showed me".

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func runDemonstrateVisual(args []string) int {
	// THE NAME FIRST, as `director save` takes it, because it is the point: everything
	// else has a sensible default and the name has none.
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("demonstrate-visual", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to watch")
	title := fs.String("window-title", "",
		"narrow to the window whose title contains this")
	seconds := fs.Int("seconds", 45, "how long to watch")
	fps := fs.Int("fps", 10, "observations per second")
	delay := fs.Duration("delay", 5*time.Second, "wait before starting, to switch to the app")
	dry := fs.Bool("dry-run", false, "report what would be saved and write nothing")
	_ = fs.Parse(flagsFirst(rest))

	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr,
			"director: say what to call it, e.g.\n"+
				`  director demonstrate-visual "enter free play" --application rocketleague`)
		return 2
	}
	// ONE OF THE TWO, and not neither: a session that watched "whatever happened to be in
	// front" is the thing `observe-screen` refuses, for the same reason.
	if *app == "" && *title == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application or --window-title is required\n"+
				`  example: director demonstrate-visual "enter free play" `+
				"--application rocketleague --seconds 45")
		return 2
	}
	// THE SAME BOUND the observer has, for the same reason: a forgotten terminal must
	// not become persistent visual observation.
	if *seconds < 1 || *seconds > maxObserveSeconds {
		fmt.Fprintf(os.Stderr, "director: --seconds must be between 1 and %d\n",
			maxObserveSeconds)
		return 2
	}
	if *fps < 1 || *fps > 60 {
		fmt.Fprintln(os.Stderr, "director: --fps must be between 1 and 60")
		return 2
	}
	interval := time.Second / time.Duration(*fps)
	obs, err := fastobserve.New(interval, observeQueue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 2
	}

	ctx := context.Background()
	s := visualSession{
		app: *app, title: *title, seconds: *seconds, interval: interval, delay: *delay,
		announce: func(ref string, settle int) {
			fmt.Printf("Watching %s at %d/second for %ds, to learn %q\n",
				ref, *fps, *seconds, name)
			fmt.Printf("  settle rule %d consecutive looks, derived from the %s interval\n",
				settle, interval)
			fmt.Printf("  Perform the routine once, with your own hands. Nothing is " +
				"injected.\n")
			fmt.Printf("  Only what the proposer saw is kept: no frames, no screenshots, " +
				"no press coordinates.\n")
			fmt.Printf("  Starting in %s.\n\n", *delay)
		},
	}
	res, code := s.watch(ctx, obs, os.Stderr)
	if code != 0 {
		return code
	}

	// ── what the session learned ──
	draft := workflow.Learn(name, obs.Walk(), obs.Clicks(), obs.Registry())
	reportVisualDemonstration(obs, res, draft)
	if len(draft.Steps) == 0 {
		fmt.Fprintf(os.Stderr,
			"\ndirector: nothing was demonstrated — no press landed on a settled screen,\n"+
				"so there is no step to save. Try again more slowly, or check that the\n"+
				"presses above were attributed.\n")
		return 1
	}

	// THE EVIDENCE FOR EACH SCREEN THE DRAFT NAMES: an arrangement, and what anybody
	// read. An unread screen is saved as unread rather than as silent, which is the
	// distinction ScreenText.Read exists for.
	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	said := map[observe.VisualScreenID]observe.ScreenText{}
	for _, sc := range obs.Screens() {
		arr[sc.ID] = obs.Arrangement(sc.ID)
		if t := obs.SaidBy(sc.ID); t.Read() {
			said[sc.ID] = t
		}
	}
	a, err := workflowstore.SaveWith(draft, arr, said, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *dry {
		fmt.Printf("\n--dry-run: %q would be saved with %d step(s), %d screen(s) and "+
			"%d control(s). Nothing was written.\n",
			a.Name, len(a.Steps), len(a.Screens), len(a.Controls))
		return 0
	}

	store, serr := demo.Open(configDir())
	if serr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", serr)
		return 1
	}
	if err := store.SaveSequence(a); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	fmt.Printf("\n%q is saved: %d step(s), %d screen(s), %d control(s).\n",
		a.Name, len(a.Steps), len(a.Screens), len(a.Controls))
	fmt.Printf("  director sequences %q        what it kept\n", a.Name)
	fmt.Printf("  director reacquire %q        whether its controls can be found again\n",
		a.Name)
	return 0
}

// reportVisualDemonstration says what the session saw, before anything is written.
//
// Printed whether or not the save succeeds, and before it, because a session that watched for
// forty seconds and learned nothing is evidence about the menu and the person is entitled to it
// without digging in a file.
func reportVisualDemonstration(obs *fastobserve.Observer, res visualResult, d workflow.Draft) {
	fmt.Printf("%s\n", strings.Repeat("-", 74))
	fmt.Printf("WHAT THE DEMONSTRATION TAUGHT\n\n")
	if res.lost != "" {
		fmt.Printf("  stopped early: %s\n\n", res.lost)
	}
	fmt.Printf("  %d screen(s) seen, %d visited in order, %d demonstrated control(s)\n",
		len(obs.Screens()), len(obs.Walk()), len(obs.Registry()))
	in := res.in
	// CLASSIFIED, which is what the producer turned into an intent -- and the ignore
	// reasons beside it, because "no press reached the correlator" and "every press was
	// refused as unplaceable" send a reader to different places.
	fmt.Printf("  presses: %d of %d classified", in.Classified, in.Received)
	for why, n := range in.Ignored {
		fmt.Printf(", %d %s", n, why)
	}
	if in.Unavailable != "" {
		fmt.Printf(" — the mouse was not watched: %s", in.Unavailable)
	}
	fmt.Println()
	if len(d.Steps) == 0 {
		return
	}
	fmt.Printf("\n  Steps\n")
	for i, st := range d.Steps {
		// WHAT STANDS BEHIND IT, which is the only thing that decides whether this step
		// could ever be repeated. `SupportGeometric` is the best value there is and it
		// is not permission — see workflow's own doc.
		fmt.Printf("    %d %s -> %s via %s   [%s]\n",
			i+1, st.From, st.To, shortTarget(st.Target), st.Support())
	}
	if un := d.Unresolved(); len(un) > 0 {
		fmt.Printf("\n  Unresolved — a person could answer these:\n")
		for _, q := range un {
			fmt.Printf("    %s\n", q)
		}
	}
}

// shortTarget names a demonstrated control, or says there was none.
func shortTarget(id observe.VisualTargetID) string {
	if id == "" {
		return "(no control — the screen changed for another reason)"
	}
	return string(id)
}
