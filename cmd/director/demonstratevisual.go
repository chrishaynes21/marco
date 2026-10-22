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
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/preview"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// keepPreviews writes one picture per screen and one per pressed control, under the labels the
// artifact itself uses.
//
// # Why the labels are recomputed from the draft and the artifact together
//
// Because the session knows screens as `visual_7` and controls as target identities, and the
// saved artifact knows them as `screen_a` and `control_1`. The mapping is made inside `SaveWith`
// — in walk order for screens, in first-appearance order for controls — and nothing exported
// hands it back. What IS true is that `a.Steps[i]` is `draft.Steps[i]`: the artifact is built by
// walking the draft's steps in order and stamping `Position: i`. So the two sides of each
// mapping sit at the same index, and reading them off together needs no second copy of the
// labelling rule that could drift from it.
//
// Returns how many were written, and the first reason one was not.
func keepPreviews(store *demo.Store, a workflowstore.Artifact, draft workflow.Draft,
	res visualResult, ev observe.ClickEvidence) (int, string) {

	// ── THE SCREENS, by the label the artifact gave each one ──
	seen, kept, why := map[string]bool{}, 0, ""
	fail := func(err error) {
		if why == "" {
			why = err.Error()
		}
	}
	for i, st := range draft.Steps {
		if i >= len(a.Steps) {
			break
		}
		for _, side := range []struct {
			id    observe.VisualScreenID
			label string
		}{{st.From, a.Steps[i].From}, {st.To, a.Steps[i].To}} {
			if side.id == "" || side.label == "" || seen[side.label] {
				continue
			}
			thumb, ok := res.screens[side.id]
			if !ok {
				continue
			}
			seen[side.label] = true
			raw, err := preview.PNG(thumb)
			if err != nil {
				fail(err)
				continue
			}
			if err := store.SavePreview(a, side.label, raw); err != nil {
				fail(err)
				continue
			}
			kept++
		}
	}

	// ── AND THE CONTROLS, cropped from the frame of the look they were pressed on ──
	//
	// The look comes from the correlator's own record of every press — `Observations`
	// carries the target and the look it arrived on — so the crop is taken from the frame
	// the person was looking at when they pressed, and not from a picture of the screen
	// taken at some other moment.
	at := map[observe.VisualTargetID]int{}
	for _, p := range ev.Observations {
		if p.Target == "" {
			continue
		}
		if _, had := at[p.Target]; !had {
			at[p.Target] = p.Look
		}
	}
	done := map[string]bool{}
	for i, st := range draft.Steps {
		if i >= len(a.Steps) || st.Target == "" {
			continue
		}
		label := a.Steps[i].Control
		if label == "" || done[label] {
			continue
		}
		look, ok := at[st.Target]
		if !ok {
			continue
		}
		frame, ok := res.pressed[look]
		if !ok {
			continue
		}
		where, ok := regionsOf(a, label)
		if !ok {
			continue
		}
		if len(where) == 0 {
			// NO REGION, NO CROP. A press the proposer found nothing under has
			// no boundary, which is exactly the case that can never become
			// something to press either.
			continue
		}
		done[label] = true
		crop, err := preview.Control(frame, where)
		if err != nil {
			fail(err)
			continue
		}
		raw, err := preview.PNG(crop)
		if err != nil {
			fail(err)
			continue
		}
		if err := store.SavePreview(a, label, raw); err != nil {
			fail(err)
			continue
		}
		kept++
	}
	return kept, why
}

// recordingNow is what the recorder knows, as one snapshot.
//
// # Why it asks the correlator rather than counting clicks
//
// Because the question a person needs answered is not "did I click" but "has Marco got the
// evidence". `observe.LocalOutcome.Verified` is the same four terms `workflow.Learn` will use to
// decide whether the interaction can be checked at all, and `Why` is the same sentence the end
// report gives. So a window showing this is showing the thing that will actually decide, half a
// second earlier — not an estimate of how long OCR takes.
//
// `Missing` is the LAST unverified outcome's reason rather than a summary of all of them: it is
// the press whose evidence is still in flight, and the one the person is waiting on.
func recordingNow(obs *fastobserve.Observer, left int) learnstate.Event {
	return recordingFrom(obs.Clicks(), obs.Screen(), obs.SaidBy, left)
}

// recordingFrom is the snapshot itself, over what the observer said rather than over the
// observer.
//
// Split out because everything here is a reading of three answers and nothing here needs to
// see: the observer is a capture loop with a window behind it, and the rules below — which
// screen an Act would start on, which presses can become steps, whether the start screen can be
// recognised — are the part that can be wrong. `saidBy` is `fastobserve.Observer.SaidBy`.
func recordingFrom(ev observe.ClickEvidence, now observe.VisualScreenID,
	saidBy func(observe.VisualScreenID) observe.ScreenText, left int) learnstate.Event {

	// ── WHICH PRESSES ARE BEING MEASURED, AND HOW EACH ONE CAME OUT ──
	//
	// One pass over the watches, counting only those with a target. A watch is opened
	// for every press including a blind one — the record of what a press did is worth
	// keeping whether or not anything was under it — so `len(ev.Outcomes)` made
	// `Watched` equal `Presses` always, which made `Presses - Watched` zero always and
	// the `Blind` phase unreachable: a person clicking beside a control was told
	// "checking what changed… wait for the result to settle" about a press that could
	// never have one.
	//
	// The WATCHES and not `Presses - Blind`, which this asked next. They agree except at
	// `observe.MaxOutcomeWatches`, where a press gets no watch at all — and there the
	// arithmetic says a press is being measured that nothing is measuring, so the phase
	// is `Checking` for the rest of the recording. Counting what exists cannot say that.
	//
	// `unchecked` is the third answer and the reason this is three counters rather than
	// two: an outcome that is FINAL and not verified is not one to wait for. See
	// observe.LocalOutcome.Final and learnstate.Unchecked.
	watched, captured, unchecked, missing := 0, 0, 0, ""
	for _, o := range ev.Outcomes {
		// A BLIND PRESS'S WATCH IS NOT ONE OF THESE. It can never become a step,
		// so its outcome is not evidence anybody is waiting on.
		if o.Target == "" {
			continue
		}
		watched++
		switch {
		case o.Verified():
			captured++
		case o.Final():
			unchecked++
			missing = o.Why()
		default:
			missing = o.Why()
		}
	}

	// ── AND THE SCREEN AN ACT BUILT FROM THIS WOULD START ON ──
	//
	// Which is the screen the FIRST press that can become a step was made on, because
	// that press becomes step 1 and its screen becomes the Act's starting screen — the
	// one `workflowstore.Standing` will have to recognise before anything is pressed.
	// Before any such press it is the screen standing now, because that is the screen the
	// next press would be made on.
	//
	// Not `Walk()[0]`, which this asked for first: the walk begins where the SESSION
	// began, and a person who opened the menu with a key before clicking anything has a
	// first screen the Act never starts on. Judging its vocabulary judged the wrong
	// screen.
	start := now
	for _, o := range ev.Outcomes {
		if o.Target != "" {
			start = o.Screen
			break
		}
	}
	// AND THE RUN'S OWN TERM FOR IT, not "has anybody read it". See
	// observe.ScreenText.Identifies, which is the call Standing makes.
	startKnown := saidBy(start).Identifies()
	// AND WHETHER THE SCREEN IN FRONT HAS BEEN READ AT ALL, which is what the NEXT press
	// needs. `Said` and not `Identifies`: the before side of an outcome is readings, and
	// one word is a reading. See learnstate.Event.ScreenRead.
	screenRead := saidBy(now).Said()

	return learnstate.Event{
		Phase: learnstate.PhaseOf(startKnown, ev.Presses, watched, captured,
			unchecked),
		Presses:    ev.Presses,
		Watched:    watched,
		Captured:   captured,
		Unchecked:  unchecked,
		Missing:    missing,
		StartKnown: startKnown,
		ScreenRead: screenRead,
		Left:       left,
	}
}

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
	blind := fs.Bool("blind", false,
		"do not read the words on each look, even when OCR is available")
	previews := fs.Bool("previews", false,
		"keep a picture of each screen and of each control that was pressed, beside "+
			"the demonstration, so it can be reviewed by eye. Cropped from the "+
			"window that was watched, stored in this machine's own Marco "+
			"directory, read by nothing that runs, and never sent anywhere")
	progress := fs.String("progress", "",
		"write this recording's structured state to `path`, one JSON object per "+
			"change: whether the screen you started on has been read, whether a "+
			"press has been seen, whether Marco could place it, and whether its "+
			"outcome is captured yet. For a front end; the ordinary output on "+
			"this terminal is unchanged")
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
	// THE WORDS, ON BY DEFAULT.
	//
	// A demonstration without them produces a control whose only identity is a
	// rectangle, and the first human game-menu demonstration measured what that is
	// worth: nothing was read on any of three controls and the one that was learned was
	// a 650x214 block. So OCR is the default and `--blind` is the opt-out, which is the
	// right way round for a default that makes the artifact better.
	//
	// ITS ABSENCE IS REPORTED AND NEVER FATAL. A machine with no OCR plugin records a
	// demonstration with unread controls, which is the honest record and exactly what
	// this command produced before today.
	var reader *readingEyes
	if !*blind {
		reader = newReadingEyes(ctx)
		if !reader.working() {
			fmt.Printf("WARNING: the words on each look will NOT be read: %s\n"+
				"         Every control will be saved with no name.\n\n", reader.why)
		}
	}
	s := visualSession{
		app: *app, title: *title, seconds: *seconds, interval: interval, delay: *delay,
		reader: reader, pictures: *previews,
		announce: func(ref string, settle int) {
			fmt.Printf("Watching %s at %d/second for %ds, to learn %q\n",
				ref, *fps, *seconds, name)
			fmt.Printf("  settle rule %d consecutive looks, derived from the %s interval\n",
				settle, interval)
			fmt.Printf("  Perform the routine once, with your own hands. Nothing is " +
				"injected.\n")
			// CLICK, NOT KEYS. A control is identified from a pointer press landing
			// inside something the proposer found; a highlight moved by the arrow
			// keys is navigation and names nothing. The first live game-menu
			// attempt was 29 events of which one was a pointer press, and the
			// report could only be decoded by somebody who knew the ignore
			// vocabulary. See whyNothingWasDemonstrated.
			fmt.Printf("  CLICK the controls with the MOUSE. Keys are watched " +
				"and cannot name a control.\n")
			// WHAT IS KEPT, SAID EXACTLY. This line used to say "no press
			// coordinates", and that promise is no longer true: the raw
			// demonstration keeps every press with its position, because
			// otherwise the only account of a click Marco could not identify
			// was a counter. A person watching their own screen be recorded is
			// owed the accurate sentence and not the reassuring one.
			// WHAT IS KEPT, SAID EXACTLY, AND IT DEPENDS ON THE FLAG.
			//
			// This line used to say "no press coordinates", and that promise
			// stopped being true when the raw demonstration began keeping every
			// press with its position. It is the same situation again: a person
			// watching their own screen be recorded is owed the accurate
			// sentence and not the reassuring one, so --previews changes what
			// this says rather than being quietly true underneath it.
			if *previews {
				fmt.Printf("  PICTURES ARE KEPT: one of each screen and one " +
					"of each control you press, cropped from this\n  " +
					"window, in your own Marco directory. Nothing is " +
					"sent anywhere, and nothing that RUNS\n  reads " +
					"them — they are there so you can see what you " +
					"showed me.\n")
			} else {
				fmt.Printf("  No frames and no screenshots are kept.\n")
			}
			fmt.Printf("  Your presses ARE kept, with where you made them, as " +
				"the record of what you\n  did — see `director " +
				"what-i-did`.\n")
			fmt.Printf("  Starting in %s. Press ENTER when you are done.\n\n", *delay)
		},
	}
	// ── WHAT THE RECORDER KNOWS, WHILE IT STILL MATTERS ──
	//
	// This terminal says everything at the two ends and nothing in between, which is
	// right for the person watching their own screen and useless to a window. The stream
	// is for the window: see learnstate, and plugins/studio/learn.go.
	//
	// A failure to open it is NOT a reason to refuse the recording. The demonstration is
	// the thing of value and the report still arrives at the end; a surface with no
	// structured progress shows a recording running, which is what it showed before this
	// existed.
	told, terr := learnstate.NewWriter(*progress, name, *app)
	if terr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n         The recording goes ahead "+
			"without structured progress.\n\n", terr)
	}
	defer told.Close()
	if told != nil {
		// THE DELAY, SAID BEFORE IT IS SPENT. The loop has not started, so nothing
		// else will say anything for five seconds.
		told.Say(learnstate.Event{Phase: learnstate.Preparing,
			Left: *seconds})
		s.told = func(left int) {
			told.Say(recordingNow(obs, left))
		}
	}
	// "THAT'S IT" IS A LINE ON ITS OWN, so a demonstration ends when the person is done
	// rather than when a clock runs out. `--seconds` stays the ceiling, so a forgotten
	// terminal still stops by itself.
	stop := endsWhenSomebodySaysSo(os.Stdin)
	s.stop = stop

	res, code := s.watch(ctx, obs, os.Stderr)
	// ── AND THE RECORDING IS OVER, SAID AS THE LAST THING ON THE STREAM ──
	//
	// Before the early return below, so a session whose window was lost still ends its
	// stream rather than leaving a window polling a phase that will never change.
	// `Final` is what tells a reader this ENDED rather than that the child vanished.
	ended := res.ended
	if ended == "" {
		ended = "it ran to time"
	}
	if res.lost != "" {
		ended = res.lost
	}
	last := recordingNow(obs, 0)
	last.Phase, last.Final, last.Reason = learnstate.Ended, true, ended
	told.Say(last)
	if code != 0 {
		return code
	}

	// ── what the session learned ──
	draft := workflow.Learn(name, obs.Walk(), obs.Clicks(), obs.Registry())
	reportVisualDemonstration(os.Stdout, obs, res, draft, reader)
	if len(draft.Steps) == 0 {
		// WHICH LINK OF THE CHAIN BROKE, rather than a guess at one of them. The
		// first live game-menu attempt was told "try again more slowly" when what
		// had happened was that the menu was navigated with the keyboard.
		fmt.Fprintf(os.Stderr, "\ndirector: nothing was demonstrated, so there is no "+
			"step to save.\n  %s\n", whyNothingWasDemonstrated(obs, res))
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
	// ── AND WHICH APPLICATION IT BELONGS TO ──
	//
	// From the window that was actually WATCHED, never from `--application`: a selector is
	// what somebody asked for and the image name is what they got, and the one time those
	// differ is the one time it matters. An unscoped workflow is valid for no application,
	// so failing to bind one saves a routine that cannot be run live — which is why this
	// is an error rather than a warning. See workflowstore.Scope.
	scoped, serr := a.ScopedTo(res.application)
	if serr != nil {
		fmt.Fprintf(os.Stderr, "director: the demonstration cannot be scoped to an "+
			"application: %v\n", serr)
		return 1
	}
	a = scoped
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
	// ── AND THE RAW DEMONSTRATION, BESIDE IT ──
	//
	// Written second and reported separately, because a failure here is not a failure of
	// the routine: a person who demonstrated something and got working steps must not be
	// told the demonstration failed because a companion file could not be written.
	//
	// What goes in it is what the person DID — every press with its coordinate, every
	// keypress with its edges and its own timestamp, the look each arrived on, and the
	// window the session ran against. The artifact above is unchanged and still carries
	// no coordinate. See workflowstore/evidence.go for why the two are separate files
	// and [[ADR-234-the-person-does-not-have-to-prove-the-click]] for why the raw record
	// outranks what Marco made of it.
	// ── AND THE PICTURES OF IT, BESIDE BOTH ──
	//
	// Third, and reported separately for the same reason the evidence is: a person who
	// demonstrated something and got working steps must not be told the demonstration
	// failed because a picture could not be written. Nothing that runs reads these.
	if *previews {
		if n, why := keepPreviews(store, a, draft, res, obs.Clicks()); why != "" {
			fmt.Fprintf(os.Stderr, "director: the routine is saved and some "+
				"pictures of it are not: %s\n", why)
		} else if n > 0 {
			fmt.Printf("  %d picture(s) kept, to look at in Studio under "+
				"this Scene.\n", n)
		}
		// ── AND WHAT EACH PRESSED CONTROL IS WORTH AS EVIDENCE ──
		//
		// Measured now, at the end, over the looks kept while it was being
		// demonstrated — see visualevidence.go for why not in the loop. Under
		// the same `--previews` licence as the pictures, because it keeps the
		// same kind of thing: a crop of somebody's screen. Reported the same
		// way too, and a control nothing could be concluded about says so in
		// the words somebody standing at the screen can act on.
		if n, said, why := keepControls(store, a, draft, res, obs.Clicks()); why != "" {
			fmt.Fprintf(os.Stderr, "director: the routine is saved and what "+
				"its controls are worth is not: %s\n", why)
		} else if n > 0 {
			fmt.Printf("  %d control(s) measured against what is beside "+
				"them.\n", n)
			for _, s := range said {
				fmt.Printf("    %s\n", s)
			}
		}
	}
	kept := ""
	if err := store.SaveEvidence(rawDemonstration(a, *app, *title, obs, draft, res)); err != nil {
		fmt.Fprintf(os.Stderr, "director: the routine is saved and the raw "+
			"demonstration is not: %v\n", err)
	} else {
		kept = fmt.Sprintf("  director what-i-did %q    what you actually did\n", a.Name)
	}
	fmt.Printf("\n%q is saved: %d step(s), %d screen(s), %d control(s).\n",
		a.Name, len(a.Steps), len(a.Screens), len(a.Controls))
	fmt.Print(kept)
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
func reportVisualDemonstration(w io.Writer, obs *fastobserve.Observer, res visualResult,
	d workflow.Draft, reader *readingEyes) {
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 74))
	fmt.Fprintf(w, "WHAT THE DEMONSTRATION TAUGHT\n\n")
	if res.lost != "" {
		fmt.Fprintf(w, "  stopped early: %s\n\n", res.lost)
	}
	fmt.Fprintf(w, "  %d screen(s) seen, %d visited in order, %d demonstrated control(s)\n",
		len(obs.Screens()), len(obs.Walk()), len(obs.Registry()))
	// AND WHAT THE STABILISER DID WITH THE LOOKS, because "1 screen seen in 331 looks"
	// has THREE causes and the report named two. A second screen that appeared and never
	// held for the settle window is neither "the menu did not move" nor "the two menus
	// folded into one": it is a screen Marco declined to believe, and `Unsettled` is the
	// count of precisely that. [[Experiment-082-a-control-that-is-half-the-window]].
	if sb := obs.Stability(); sb.Looks > 0 {
		fmt.Fprintf(w, "  looks: %d settled, %d unsettled, %d warming, %d promotion(s)\n",
			sb.Settled, sb.Unsettled, sb.Warming, sb.Promotions)
	}
	// WHAT THE READER DID, including when it did nothing. A demonstration whose controls
	// have no names is a different report depending on whether OCR was absent, too slow,
	// or ran and found no words — and the three send a reader to three different places.
	fmt.Fprintf(w, "  words: %s\n", reader.describe())
	if res.readings > 0 {
		fmt.Fprintf(w, "  %d reading(s) reached the observer over %d look(s)\n",
			res.readings, res.looks)
	}
	in := res.in
	// CLASSIFIED, which is what the producer turned into an intent -- and the ignore
	// reasons beside it, because "no press reached the correlator" and "every press was
	// refused as unplaceable" send a reader to different places.
	//
	// EVENTS, not presses. This line said `presses:` and counted navigation intents of
	// every kind, so a session of four Escapes and no clicks read as four presses -- and
	// it was read that way twice, by the owner and by the author, on the two live runs
	// that produced no control. `Classified` is incremented on both the key path and the
	// pointer path of the producer; only the clicks line below is about clicks.
	fmt.Fprintf(w, "  input: %d of %d event(s) classified", in.Classified, in.Received)
	for why, n := range in.Ignored {
		fmt.Fprintf(w, ", %d %s", n, why)
	}
	if in.Unavailable != "" {
		fmt.Fprintf(w, " — the mouse was not watched: %s", in.Unavailable)
	}
	fmt.Fprintln(w)
	// AND WHAT BECAME OF THE CLICKS, SEPARATELY, from the observer and the correlator
	// rather than from the producer. `ClickEvidence.Presses` exists for exactly this and
	// says so in its own doc: a report with four subsets and no total cannot tell
	// "nothing was attributed" from "nothing arrived".
	fmt.Fprintln(w)
	if len(d.Steps) == 0 {
		return
	}
	fmt.Fprintf(w, "\n  Steps\n")
	for i, st := range d.Steps {
		// WHAT STANDS BEHIND IT, which is the only thing that decides whether this step
		// could ever be repeated. `SupportGeometric` is the best value there is and it
		// is not permission — see workflow's own doc.
		fmt.Fprintf(w, "    %d %s -> %s via %s   [%s]\n",
			i+1, st.From, st.To, shortTarget(st.Target), st.Support())
	}
	// WHICH CONTROL EACH PRESS WAS ASSOCIATED WITH, which is the thing a person
	// demonstrating wants to see: did Marco understand what I clicked on?
	//
	// From the registry rather than from the steps, because a control pressed twice is one
	// control and two steps, and the question is about the control. The words are the
	// strongest identity evidence a demonstration can record and the first thing a person
	// can check against their own memory of the menu.
	//
	// AND THE SCREEN EACH ONE IS ON, because a control identity is scoped to a screen
	// on purpose -- the same place on two screens is two controls. So when a button
	// demonstrated on two separate visits comes back as TWO controls, the screen is what
	// separates the two causes: two controls on ONE screen means the presses did not
	// merge, and two on TWO screens means the revisited menu was not recognised as the
	// menu. Those send a reader to different places, and without the screen printed the
	// report cannot tell them apart.
	if reg := obs.Registry(); len(reg) > 0 {
		fmt.Fprintf(w, "\n  Controls, and what each press was taken to be\n")
		for _, r := range reg {
			fmt.Fprintf(w, "    %s\n", controlLine(r, reg))
		}
	} else {
		// AND WHY THERE IS NONE, HERE, not only when there was no step either. The
		// live run that prompted this had three steps and zero controls and the
		// report said nothing about the zero: the person had toggled Ark's pause
		// menu with Escape, which moves between screens perfectly well and can
		// never name a control. A session that walked four screens and learned
		// nothing to press is the case most in need of a sentence.
		fmt.Fprintf(w, "\n  No control was identified.\n    %s\n",
			whyNoControlWasIdentified(obs, res))
	}
	// AND WHAT THE PERSON DID THAT MARCO CANNOT REPEAT, as the thing they did rather
	// than as a number.
	//
	// A press no proposal contained used to be one integer on the `clicks:` line. The
	// person pressed somewhere, on a screen, at a known moment, and Marco watched them do
	// it; reporting that as `1 landed on nothing the proposer found` tells them their
	// click was wrong when what happened is that Marco could not see what they pressed.
	// See [[ADR-234-the-person-does-not-have-to-prove-the-click]].
	if blind := obs.Clicks().BlindPresses; len(blind) > 0 {
		fmt.Fprintf(w, "\n  You pressed these, and Marco saw nothing there to press\n")
		for _, p := range blind {
			fmt.Fprintf(w, "    %s\n", blindPressLine(p))
		}
		fmt.Fprintf(w, "    Kept as evidence of what you did. None of them can be "+
			"replayed:\n    a press with no region has no boundary to find again.\n")
	}
	if un := d.Unresolved(); len(un) > 0 {
		fmt.Fprintf(w, "\n  Unresolved — a person could answer these:\n")
		for _, q := range un {
			fmt.Fprintf(w, "    %s\n", q)
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

// controlLine is one demonstrated control as the report shows it.
//
// Its own function because the report around it needs a live observer and a running reader to
// produce, and the claim being made here is about this one line. See
// TestTheReportNamesTheScreenEachControlWasDemonstratedOn.
//
// `all` is every control of the session, because two of the things most worth saying about one
// control are statements about the others — see notAButton.
func controlLine(r observe.TargetRecord, all []observe.TargetRecord) string {
	said := "nothing was read on it"
	if len(r.Text) > 0 {
		said = fmt.Sprintf("read as %q", strings.Join(r.Text, " "))
	}
	out := fmt.Sprintf("%-12s on %-10s %s  %d press(es), %s",
		r.ID, r.Screen, regionSize(r.Where), r.Presses, said)
	if len(r.Destinations) > 0 {
		out += ", led somewhere"
	} else {
		out += ", NOTHING CHANGED after it"
	}
	if len(r.MaybeOneControlWith) > 0 {
		out += fmt.Sprintf("  [could not be told from %v on this screen]",
			r.MaybeOneControlWith)
	}
	if why := notAButton(r, all); why != "" {
		out += "\n                  NOT A BUTTON: " + why
	}
	return out
}

// regionSize is a control's rectangle as a share of the window.
//
// Printed on every line, with no judgement attached, because it is the whole difference between
// the two game-menu demonstrations and neither report showed it:
// [[Experiment-080-it-learned-the-word-settings]] learned 16%x5% and
// [[Experiment-082-a-control-that-is-half-the-window]] learned 49%x64%, and the two reports were
// indistinguishable. A share rather than pixels because that is what the artifact stores; the
// window's size is not in it, on purpose.
func regionSize(where []observe.Region) string {
	if len(where) == 0 {
		return "   no shape"
	}
	w := where[0]
	return fmt.Sprintf("%3.0f%%x%3.0f%%", w.Width*100, w.Height*100)
}

// notAButton says why this control cannot be the thing that was pressed, or nothing.
//
// Two statements, and NEITHER is a threshold. This subsystem has a measured history of invented
// numbers being wrong, and "a control bigger than X% of the window is a panel" would be one — so
// both of these are facts the session already recorded about rectangles it already has:
//
//	it is a container     some control of this session recorded this exact rectangle as a
//	                      container it was distinguished FROM. Containers is screen-local, so
//	                      on a screen where the proposer offered nothing smaller the same
//	                      rectangle became a control by default. That is
//	                      [[Experiment-082-a-control-that-is-half-the-window]], where
//	                      control_1.containers[0] and control_2.readings[0] are the same
//	                      rectangle to the last decimal digit.
//	it contains a control the stronger form of the same thing, and it needs no other screen:
//	                      a rectangle holding another control of this session is that
//	                      control's container, whatever the proposer called it.
//
// Reporting only. Nothing here refuses a save, changes a match or withholds an identity; the
// owner's standing instruction is that pixel execution is not authorised, and a report that
// silently dropped evidence would be a worse answer to it than one that prints the problem.
func notAButton(r observe.TargetRecord, all []observe.TargetRecord) string {
	if len(r.Where) == 0 {
		return ""
	}
	mine := r.Where[0]
	for _, o := range all {
		for _, c := range o.Containers {
			if observe.SameRegion(c, mine) {
				return fmt.Sprintf("this is the rectangle %s was distinguished "+
					"from, recorded there as a container it is not", o.ID)
			}
		}
	}
	for _, o := range all {
		if o.ID == r.ID || len(o.Where) == 0 {
			continue
		}
		if observe.Encloses(mine, o.Where[0]) {
			return fmt.Sprintf("it contains %s, so it is that control's container",
				o.ID)
		}
	}
	return ""
}

// rawDemonstration is the companion record: what the person physically did.
//
// The CONTENT comes from the session and never from the draft, deliberately: the draft is
// Marco's reading of the demonstration and leaves things out — a press no proposal contained, a
// key whose effect was never observed, a release that bounded a hold. Those are exactly the
// things a person asks about afterwards, and the record they are in must not be derived from
// the reading that discarded them.
//
// The draft is taken for its LABELS alone, below. Nothing is dropped because the draft does not
// mention it: a press on a screen the route never kept is recorded without a label.
//
// The window SELECTOR as the person typed it, never the live title: a game window's title is
// the commonest place an account name appears, which is why `capture-game-session` refuses to
// record one.
func rawDemonstration(a workflowstore.Artifact, app, title string,
	obs *fastobserve.Observer, d workflow.Draft, res visualResult) workflowstore.Evidence {

	// ── THE ROUTINE'S OWN LABELS, NOT THE SESSION'S IDENTITIES ──
	//
	// `visual_1` and `target_2` are session-local and mean nothing in a file; worse, a
	// durable file carrying one invites a later reader to match on it across sessions,
	// which is what `TestAVisualScreenCannotReachDurableMemory` refuses. So the record
	// speaks the artifact's language — `screen_a`, `control_1` — and the translation is
	// the same function `SaveWith` used, so the two files cannot disagree.
	//
	// A press on a screen the route does not contain gets no label and keeps its LOOK,
	// which is the stronger reference anyway.
	screens, controls := workflowstore.Labels(d), workflowstore.ControlLabels(d)
	presses := make([]workflowstore.DemonstratedPress, 0, len(obs.Clicks().Observations))
	for _, p := range obs.Clicks().Observations {
		presses = append(presses, workflowstore.DemonstratedPress{
			Look: p.Look, At: p.At,
			Screen: screens[p.Screen], Control: controls[p.Target],
			Containing: len(p.Candidates), Ambiguous: p.Ambiguous,
		})
	}
	moments := make([]workflowstore.Moment, 0, len(res.moments))
	for _, m := range res.moments {
		m.Screen = screens[observe.VisualScreenID(m.Screen)]
		moments = append(moments, m)
	}
	return workflowstore.Evidence{
		Name: a.Name,
		// THE ARTIFACT'S OWN INSTANT, which is the link. A second clock here would
		// drift from the thing this file claims to describe.
		SavedAt: a.SavedAt,
		Window: workflowstore.WindowWatched{
			Application: app, Title: title,
			Generations: len(res.generations),
		},
		Moments: moments,
		// EVERY PRESS, including the ones Marco understood. `Observations` is the
		// record of the act; Attributed, Inert and BlindPresses are readings of it.
		Presses: presses,
		Counts:  res.in,
		Looks:   res.looks,
	}
}

// endsWhenSomebodySaysSo closes when a line is read, and never when there is nobody to read.
//
// # EOF is not somebody saying they are done
//
// It is a session with no terminal attached. This used to end the session on any error from
// `ReadString`, EOF included — so run from a shell with no console, the reader returned
// immediately, the channel closed, and the session ended before it began. The report said
//
//	0 screen(s) seen, 0 visited in order, 0 demonstrated control(s)
//	words: offered 0, read 0, dropped while busy 0, 0 word(s)
//
// which reads as a perception failure and is a closed stdin. It cost two of the owner's own
// clicks into a live game before anybody looked at the reader.
//
// So a session nobody can end by hand runs to its `--seconds` ceiling, which is what that
// ceiling is for: "a forgotten terminal still stops by itself".
func endsWhenSomebodySaysSo(r io.Reader) chan struct{} {
	stop := make(chan struct{})
	go func() {
		if _, err := bufio.NewReader(r).ReadString('\n'); err != nil {
			return
		}
		close(stop)
	}()
	return stop
}

// regionsOf is where the artifact says one control was read, for cropping a picture of it.
//
// # Why this returns regions and not the control
//
// Because this command may not NAME `workflowstore.Control` at all, and the test that holds that
// line is right to: a command that can mint a Control can put anything in one, and what goes in
// one is a control's durable identity. A lookup returning a zero value on a miss is still a
// command with the type in its hands. So it hands back the only part a picture needs — where to
// crop — and the artifact remains the only thing that decided what a control is.
//
// See TestASavedPixelDemonstrationCarriesNoCoordinates, which caught exactly this.
func regionsOf(a workflowstore.Artifact, label string) ([]observe.Region, bool) {
	for _, c := range a.Controls {
		if c.Label == label {
			return c.Readings, true
		}
	}
	return nil, false
}
