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
	blind := fs.Bool("blind", false,
		"do not read the words on each look, even when OCR is available")
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
		reader: reader,
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
			fmt.Printf("  Only what the proposer saw is kept: no frames, no screenshots, " +
				"no press coordinates.\n")
			fmt.Printf("  Starting in %s. Press ENTER when you are done.\n\n", *delay)
		},
	}
	// "THAT'S IT" IS A LINE ON ITS OWN, so a demonstration ends when the person is done
	// rather than when a clock runs out. `--seconds` stays the ceiling, so a forgotten
	// terminal still stops by itself.
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}()
	s.stop = stop

	res, code := s.watch(ctx, obs, os.Stderr)
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
