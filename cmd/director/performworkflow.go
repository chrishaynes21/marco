package main

// `director perform-workflow` — a saved workflow, performed one authorised action at a time.
//
// # What it is
//
// The composition root for `internal/director/workflowrun`. It is the only place in the tree
// where the four things that coordinator cannot do for itself are supplied:
//
//	the eyes          the live fast observer, on its own cadence
//	the grounder      a person answering "is this that screen", or an inference from one
//	the granter       a person answering "may I press this, now"
//	the executor      legal Marco over a real OS host — or a notebook
//
// # It is DRY unless told otherwise
//
// Without `--live` the Marco runner is the real front end over `recordhost`, so every program is
// lexed, parsed, graphed, compiled and run, and the last step appends a line instead of pressing
// a mouse button. That is the useful default: it exercises the whole path and moves nothing.
//
// `--live` replaces one Host. Nothing else about the run changes — same coordinator, same
// refusals, same prompts — which is the point: a dry run that took a shortcut past the compiler
// would prove that the shortcut works.
//
// # Every prompt blocks the capture loop, and that is the safe direction
//
// While somebody is reading a question nothing is captured and nothing is pressed. What the pause
// costs is FRESHNESS: the look that found the control is seconds old by the time they answer. So
// the coordinator re-observes the world at the instant of the press, and this is where that
// re-observation is wired — `stillThere` below compares the live observer's generation against
// the one the plan was made from, and a screen that moved refuses the press.
//
// # What it will not do
//
// It will not establish a screen for a person, because nothing can
// ([[Experiment-036-recognising-a-screen-you-were-shown]]); it will not confirm a step on their
// behalf; it will not grant its own authorisation; and it will not report an arrival as a
// verified action. It writes nothing to disk and promotes nothing into durable memory.

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// maxPerformSeconds bounds a run.
//
// The same five minutes `observe-screen` uses and for the same reason: long enough to walk a menu
// several times with somebody answering questions, short enough that a forgotten terminal does
// not become an unattended process that can click.
const maxPerformSeconds = 300

func runPerformWorkflow(args []string) int {
	fs := flag.NewFlagSet("perform-workflow", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to watch and act on")
	file := fs.String("workflow", "", "the saved workflow artifact to perform")
	live := fs.Bool("live", false,
		"send REAL input. Without this the whole path runs and nothing is pressed")
	seconds := fs.Int("seconds", 120, "how long the run may take")
	fps := fs.Int("fps", 10, "observations per second")
	delay := fs.Duration("delay", 5*time.Second, "wait before starting, to switch to the app")
	patience := fs.Int("patience", workflowrun.DefaultPatience,
		"how many observations one step is given before the run stops safely")
	maxActions := fs.Int("max-actions", -1,
		"how many actions this run may send in total; negative is unbounded")
	hotkeys := fs.Bool("hotkeys", false,
		"answer with "+navsource.ConsentKeys[navsource.ConsentApprove]+"/"+
			navsource.ConsentKeys[navsource.ConsentRefuse]+"/"+
			navsource.ConsentKeys[navsource.ConsentStop]+" instead of the "+
			"keyboard, so the application keeps the foreground")
	from := fs.String("from", "",
		"the saved screen you will be standing on when the run starts. Spent ONCE, on "+
			"the first observation; every screen after it is inferred or asked about")
	answer := fs.Duration("answer", 20*time.Second,
		"with -hotkeys, how long a question stays open")
	reading := fs.Bool("reading", false,
		"read the words on each look with OCR, beside the fast loop, so two visually similar screens are not merged into one identity")
	repair := fs.String("repair", "",
		"when a saved control locates nothing, look for it on the screen instead, and "+
			"write a PROPOSED correction to this path. Never writes to the "+
			"workflow")
	learned := fs.Bool("learned", false,
		"offer the learned detector's candidates beside the pixel proposer's. It runs "+
			"on its own clock and never inside the observation loop")
	frames := fs.String("frames", "",
		"write the two captures each hover measurement was made from into this "+
			"directory, so a number can be checked against a picture")
	explore := fs.Bool("explore", false,
		"authorise the whole session at launch instead of asking for each action. "+
			"Bounded by -max-actions and -seconds; every other check is unchanged")
	dwell := fs.Duration("dwell", workflowrun.DefaultDwell,
		"how long the pointer rests on a control before the click. 0 sends the click "+
			"alone, which a game menu ignores")
	_ = fs.Parse(flagsFirst(args))

	switch {
	case *app == "":
		fmt.Fprintln(os.Stderr, "director: --application is required\n"+
			"  example: director perform-workflow --application rocketleague "+
			"--workflow freeplay.workflow.json")
		return 2
	case *file == "":
		fmt.Fprintln(os.Stderr, "director: --workflow is required (a saved artifact)")
		return 2
	case *seconds < 1 || *seconds > maxPerformSeconds:
		fmt.Fprintf(os.Stderr, "director: --seconds must be between 1 and %d\n",
			maxPerformSeconds)
		return 2
	case *fps < 1 || *fps > 60:
		fmt.Fprintln(os.Stderr, "director: --fps must be between 1 and 60")
		return 2
	case *explore && *maxActions < 1:
		fmt.Fprintln(os.Stderr, "director: --explore needs --max-actions, because a "+
			"session nobody is asked about has to be bounded by something")
		return 2
	case *maxActions == 0:
		fmt.Fprintln(os.Stderr, "director: --max-actions 0 would perform nothing; "+
			"leave it out for an unbounded run, or use --live without it to "+
			"send nothing at all")
		return 2
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	art, err := workflowstore.Unmarshal(raw)
	if err != nil {
		// A refusal from the artifact's own validation: an unknown version, a
		// clarification attached to nothing, a step naming a screen it does not carry.
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	if *from != "" {
		known := false
		for _, sc := range art.Screens {
			if sc.Label == *from {
				known = true
			}
		}
		if !known {
			fmt.Fprintf(os.Stderr, "director: %q carries no screen called %q\n",
				art.Name, *from)
			return 1
		}
	}

	interval := time.Second / time.Duration(*fps)
	obs, err := fastobserve.New(interval, observeQueue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 2
	}

	// ── THE EXECUTOR. One Host is the whole of the --live decision. ──
	book := recordhost.New()
	var host runtime.Host = book
	if *live {
		host = oshost.New()
	}
	marco := marcorunner.New(map[string]runtime.Host{"OS": host})

	// AND THE MACHINE'S ANSWER TO "may this runtime drive the screen right now".
	//
	// Claimed for a real run only: a notebook is not a desktop and nothing else on the
	// machine is competing for it. Two Directors serving two homes share one keyboard.
	if claim := desktopLease(*live); claim != nil {
		release, err := claim()
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: the desktop is not available: %v\n", err)
			return 1
		}
		defer release()
	}

	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)

	// CTRL-C STOPS THE RUN RATHER THAN KILLING IT, so a press in flight is refused and
	// the report still says how far it got.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(),
		windowref.Selector{Application: *app})
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}

	ask := bufio.NewReader(os.Stdin)
	session := workflowstore.NewSessionBinding()
	ground := &promptGrounder{art: art, in: ask, out: os.Stdout,
		// NOBODY CAN BE ASKED in either mode: answering by keyboard would take the
		// foreground, and an exploring session is authorised precisely so that it
		// does not stop to ask. Every screen after --from is inferred from the
		// workflow's own demonstrated transitions, or the run stops and says so.
		claim: *from, silent: *hotkeys || *explore}

	// ── WHO IS ASKED, AND HOW THEY ANSWER ──
	//
	// `--hotkeys` exists because of a measured failure, not a preference: typing an answer
	// makes the terminal the foreground window, and the press the answer permits is then
	// refused because the target no longer leads the desktop. See consentdesk.go.
	// `run` is declared before the desk so the STOP key can reach it. A desk built with
	// no way to stop the run would make F9 a key that prints something.
	var run *workflowrun.Runner
	var session0 *sessionGrant
	var grant workflowrun.Granter = &promptGranter{in: ask, out: os.Stdout, live: *live}
	var keys *hotkeyGranter
	var desk *consentDesk
	if *explore {
		// THE CLOCK STARTS WHEN THE RUN DOES, which is why `until` is set below
		// rather than here. Started here it would already be running through
		// --delay, which is time nobody is allowed to act in: the first exploring
		// session spent its whole allowance waiting to begin and then refused its
		// first action because the session's time was up.
		session0 = &sessionGrant{
			by:  "the person who authorised this exploration at launch",
			out: os.Stdout, actions: *maxActions,
		}
		grant = session0
	}
	if *hotkeys && !*explore {
		src, why := navsource.New()
		if why != "" {
			fmt.Fprintf(os.Stderr, "director: no key hook is available, so "+
				"--hotkeys has no way to hear an answer: %s\n", why)
			return 1
		}
		defer src.Close()
		desk = newConsentDesk(src.Consents(), func() { run.Stop() })
		go desk.run(ctx)
		keys = &hotkeyGranter{desk: desk, out: os.Stdout, live: *live,
			within: *answer, life: 10 * time.Second}
		grant = keys
	}

	windowNow := func() (directorapi.Window, bool) {
		v := tracker.Acquire(ctx, *app)
		if !v.State.OK() {
			return directorapi.Window{}, false
		}
		return windowRefToWindow(v.Ref), true
	}

	// ── WHERE A CONTROL IS, WHEN THE FILE CANNOT SAY ──
	//
	// Only built when asked for. The repair needs OCR, which costs hundreds of
	// milliseconds, and an ordinary run neither needs it nor pays for it.
	var repairs *regrounder
	if *repair != "" {
		words := newWordReader(shots, windowNow)
		if !words.working() {
			fmt.Fprintf(os.Stderr, "director: --repair needs the OCR plugin and "+
				"it is not available: %s\n", words.why)
			return 1
		}
		repairs = &regrounder{art: art, out: os.Stdout, words: words.read}
	}

	probe := &hoverProbe{shots: shots, moved: 8, frames: *frames, window: windowNow}

	run = workflowrun.New(art, nil, ground, grant).
		WithExecutor(marcoexec.New(marco), *live).
		WithHover(probe.look).
		WithRegrounder(regroundOrNil(repairs)).
		WithBudget(*maxActions).
		WithDwell(*dwell).
		// THE WORLD, RE-OBSERVED AT THE INSTANT OF THE PRESS. Deleting this call, or
		// passing nil, must make the runner refuse every action — see
		// TestPerformWorkflowRechecksTheWorldBeforeEveryPress.
		WithConfirm(stillThere(obs, tracker, windows, *app)).
		WithPatience(*patience)

	fmt.Printf("Performing %q against %s\n", art.Name, v.Ref.Describe())
	fmt.Printf("  %d step(s), %d screen(s), %d control(s)\n",
		len(art.Steps), len(art.Screens), len(art.Controls))
	if *live {
		fmt.Printf("  LIVE: real input will be sent, one authorised action at a time.\n")
	} else {
		fmt.Printf("  DRY: every program is compiled and run, and the Host writes it " +
			"down instead of pressing anything.\n")
	}
	fmt.Printf("  Ctrl-C stops the run. Nothing is written to disk.\n")
	fmt.Printf("  Starting in %s.\n\n", *delay)
	select {
	case <-time.After(*delay):
	case <-ctx.Done():
		return 1
	}

	// A CONSUMER, so the observer's queue is read and its drop count means something.
	go func() {
		for range obs.Updates() {
		}
	}()

	// THE SECOND SOURCE, on its own clock. See learnedeyes.go.
	var eyes *learnedEyes
	var reader *readingEyes
	if *learned {
		eyes = newLearnedEyes(ctx)
		if !eyes.working() {
			fmt.Fprintf(os.Stderr, "director: --learned asked for the detector "+
				"and it is not available: %s\n", eyes.why)
			return 1
		}
		fmt.Printf("  the learned detector is running beside the loop.\n")
	}
	if *reading {
		reader = newReadingEyes(ctx)
		if !reader.working() {
			fmt.Fprintf(os.Stderr, "director: --reading asked for OCR "+
				"and it is not available: %s\n", reader.why)
			return 1
		}
		fmt.Printf("  the words on each look are read beside the loop.\n")
	}

	var blind int
	var blindWhy string
	var look int
	opt := structure.DefaultOptions()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	if session0 != nil {
		// THE SESSION'S CLOCK, started with the run. Set here and not where the
		// grant was built, because --delay is time nobody may act in and a session
		// that counted it spent its whole allowance waiting to begin.
		session0.until = deadline
	}
	lost := ""

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}
		v := tracker.Acquire(ctx, *app)
		if !v.State.OK() {
			lost = fmt.Sprintf("the target was lost (%s)", v.State)
			break
		}
		lk := fastobserve.Look{
			At: time.Now(),
			Window: fastobserve.WindowRef{
				ID: v.Ref.ID, Generation: v.Ref.Generation, Bounds: v.Ref.Bounds,
			},
		}
		img, cerr := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		if cerr != nil {
			// WHY THE PIXELS DID NOT ARRIVE. Discarding this turned a capture
			// that failed 410 times out of 417 into "no pixels were obtained",
			// which names the symptom and not one cause.
			lk.Missing = true
			blind++
			if blindWhy == "" {
				blindWhy = cerr.Error()
			}
		} else {
			// THE FRAME THE PLAN IS MADE FROM, kept so the hover probe can
			// compare the control against itself a quarter of a second later.
			probe.before = img.Image
			lk.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref) ==
				observe.FrontTarget
			res := structure.Propose(img.Image, opt)
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)

			// THE SLOW SOURCE. Offering is non-blocking and the queue holds
			// one frame, so a busy detector costs detections and never
			// costs cadence; the candidates it returns describe a frame
			// about ten looks old and carry the epoch and generation that
			// says so. See [[ADR-184-two-speeds-of-looking]].
			if eyes.working() {
				eyes.offer(learnedFrame{
					img: img.Image, look: look,
					epoch:      obs.Epoch(),
					generation: v.Ref.Generation,
				})
				lk.Slow = eyes.ready()
			}

			// THE SLOW TEXT SOURCE, offered and drained exactly as the detector
			// is, and for the same reason: OCR costs hundreds of milliseconds
			// against a cadence of a hundred, so what comes back describes a
			// frame several looks old and carries the window generation and
			// observation epoch that say which. The observer refuses one whose
			// context has moved -- see fastobserve.intakeReadings -- and an
			// admitted one may do exactly one thing: refuse to believe the
			// standing screen is the screen the arrangement says.
			if reader.working() {
				reader.offer(readingFrame{
					img: img.Image, look: look,
					epoch:      obs.Epoch(),
					generation: v.Ref.Generation,
				})
				lk.Readings = reader.ready()
			}
		}
		look++
		// RE-ACQUISITION BEFORE THE FOLD, so nothing this look teaches can help
		// recognise what this look was meant to recognise.
		outcomes := reacquireStored(art, session, obs, lk)
		up := obs.Observe(lk)

		at := workflowrun.Sighting{
			Screen: up.Screen, Epoch: up.Epoch, Generation: v.Ref.Generation,
			Window: v.Ref.ID, Bounds: v.Ref.Bounds,
			Front: lk.Front, Missing: lk.Missing, Unobserved: up.Unobserved,
			Outcomes: outcomes, At: lk.At,
		}
		ground.arrangement = obs.Arrangement(up.Screen)

		// WHAT THE SOURCES FOUND ON THIS LOOK, handed over because the repair
		// cannot look for itself. Both sources, and only regions that locate
		// something: a band is never a candidate for replacing a band.
		if repairs != nil {
			repairs.candidates = append(append([]observe.Region{}, lk.Regions...),
				fresh(lk.Slow, obs.Epoch(), v.Ref.Generation)...)
		}

		a := run.Advance(ctx, session, at)
		reportAttempt(a)

		// A STEP NOBODY HAS AGREED TO IS A QUESTION, NOT A REFUSAL.
		//
		// Asked here rather than inside the coordinator because agreeing to a step and
		// permitting a press are two different sentences, and a build that put them
		// behind one prompt would let a person answer one and be taken to have
		// answered the other.
		if a.Stage == workflowstore.StepUnconfirmed {
			if *explore {
				// THE SESSION AGREED TO THE ROUTE, once, at launch. Still
				// recorded per step and still attributed, because a
				// confirmation with no author is indistinguishable from a
				// default — see SessionBinding.ConfirmStep.
				if err := session.ConfirmStep(a.Step,
					"the person who authorised this exploration"); err != nil {
					lost = err.Error()
					break
				}
				continue
			}
			if !agreeToStep(ctx, keys, ask, os.Stdout, art, a.Step, session) {
				lost = "nobody agreed to step " + fmt.Sprint(a.Step)
				break
			}
		}
		switch a.Outcome {
		case workflowrun.OutcomeComplete, workflowrun.OutcomeRefused,
			workflowrun.OutcomeFailed, workflowrun.OutcomeStopped:
			deadline = time.Now() // fall out of the loop with the report intact
		}
		<-ticker.C
	}
	obs.Close()

	rep := run.Report()
	if *repair != "" && len(rep.Repairs) > 0 {
		reached := map[int]bool{}
		for _, a := range rep.Attempts {
			if a.Arrival != nil && a.Arrival.Established() {
				reached[a.Step] = true
			}
		}
		if err := writeProposal(*repair, art, rep.Repairs, reached, rep.Changed); err != nil {
			fmt.Fprintf(os.Stderr, "director: the proposal could not be "+
				"written: %v\n", err)
		} else {
			fmt.Printf("\n  %d proposed correction(s) written to %s. The "+
				"workflow itself is untouched.\n", len(rep.Repairs), *repair)
		}
	}
	if blind > 0 {
		fmt.Printf("\n  %d of %d capture(s) returned nothing. The first said: %s\n",
			blind, rep.Looks, blindWhy)
	}
	printRun(rep, art, book, *live, lost, eyes, reader, obs.Stats())
	if desk != nil {
		stray, drained := desk.counts()
		fmt.Printf("\n  %-42s %d of %d\n", "steps agreed to by key",
			keys.agreed, keys.proposed)
		fmt.Printf("  %-42s %d of %d\n", "presses permitted by key",
			keys.granted, keys.asked)
		fmt.Printf("  %-42s %d before a question, %d with none open\n",
			"key presses that answered nothing", drained, stray)
	}
	if rep.Progress.Complete() {
		return 0
	}
	return 1
}

// reacquireStored asks the artifact's own readings about this look.
//
// The saved workflow's `Targets` against the bindings that still hold, matched by the registry's
// own matcher — the same arithmetic the offline replay uses, reached without a registry.
func reacquireStored(art workflowstore.Artifact, s *workflowstore.SessionBinding,
	obs *fastobserve.Observer, lk fastobserve.Look) []observe.Reacquisition {

	// BOTH SOURCES, ONE MATCHER.
	//
	// The detector's candidates join the proposer's and are matched by exactly the same
	// rule — same `sameControl` relation, same container exclusion, same ambiguity
	// refusal when two proposals support one control and describe different ones. A
	// detection is a region like any other region; what makes it a target is agreeing
	// with a reading the demonstration recorded, and that is unchanged.
	//
	// Only the fresh ones. A detection describes pixels from several observations ago and
	// `fresh` drops the ones whose epoch or generation has moved — the same two numbers
	// `admitSlow` asks about, asked here because a target matched against a stale
	// rectangle is a point on a screen that has gone.
	regions := lk.Regions
	if len(lk.Slow) > 0 {
		regions = append(append([]observe.Region{}, regions...),
			fresh(lk.Slow, obs.Epoch(), lk.Window.Generation)...)
	}
	var out []observe.Reacquisition
	for _, st := range art.Targets(s.Binding(obs.Epoch(), lk.Window.Generation)) {
		out = append(out, observe.ReacquireStored(st, regions))
	}
	return out
}

// worldNow is the desktop, as it is at the instant of a press.
//
// A struct rather than four arguments so the DECISION below can be tested. Gathering these four
// facts needs a live window tracker and something actually in front, which no test has; deciding
// what they mean is the part that can be wrong in a way nobody would notice, so the two are
// separate functions and only the gathering is untestable.
type worldNow struct {
	// Alive says the target window is still there, and State says what the tracker made
	// of it when it is not.
	Alive bool
	State string
	// Generation is the window generation now, and Epoch the observation generation now.
	Generation uint64
	Epoch      uint64
	// Screen is the screen hypothesis standing now.
	//
	// Carried as well as Epoch, and not instead of it, because the two catch different
	// things. The epoch moves when the observation is superseded; the identity changes
	// when what is standing is a DIFFERENT screen. A re-settle inside one epoch is the
	// case where the first is unchanged and the second is not, and it is the one that
	// would spend an authorisation on a point found on a screen nobody is looking at.
	Screen observe.VisualScreenID
	// Front is what the desktop says about the target leading it.
	Front observe.Frontness
}

// worldMoved reports whether the world is still the one an action was planned against.
//
// Four refusals, each with its own sentence, and the last of them is the one that only exists
// because a person was asked something. A prompt is an unbounded pause in the middle of the one
// sequence that must not have one: by the time the answer arrives, the look that found the
// control may describe a screen that has gone, and the point is then a coordinate on a screen
// that is no longer standing.
func worldMoved(app string, now worldNow, planned workflowrun.Sighting) (bool, string) {
	switch {
	case !now.Alive:
		return false, fmt.Sprintf("%s is no longer there (%s)", app, now.State)
	case now.Generation != planned.Generation:
		return false, fmt.Sprintf("the window was replaced since this was planned "+
			"(generation %d, was %d)", now.Generation, planned.Generation)
	case now.Front != observe.FrontTarget:
		return false, fmt.Sprintf("%s does not lead the desktop (%s), and a click "+
			"lands in whatever is in front", app, now.Front)
	case now.Epoch != planned.Epoch:
		return false, fmt.Sprintf("the screen changed since this was planned "+
			"(observation %d, was %d), so the point belongs to a screen that is "+
			"no longer standing", now.Epoch, planned.Epoch)
	case now.Screen != planned.Screen:
		return false, fmt.Sprintf("a different screen is standing now (%s, was %s), "+
			"so the control this point was found on is not what is in front of "+
			"the pointer", now.Screen, planned.Screen)
	}
	return true, ""
}

// stillThere gathers the world and hands it to worldMoved.
//
// The one line in this file a test cannot supply for itself, for the same reason `windowLeads` is
// a package variable: a real answer needs a live window and a real desktop. Everything it
// decides is in worldMoved, which is tested.
func stillThere(obs *fastobserve.Observer, tracker *windowref.Tracker,
	windows *winprovider.Provider, app string) func(context.Context,
	workflowrun.Sighting) (bool, string) {

	return func(ctx context.Context, planned workflowrun.Sighting) (bool, string) {
		v := tracker.Acquire(ctx, app)
		now := worldNow{
			Alive: v.State.OK(), State: string(v.State),
			Generation: v.Ref.Generation, Epoch: obs.Epoch(),
			Screen: obs.Screen(),
		}
		if now.Alive {
			now.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref)
		}
		return worldMoved(app, now, planned)
	}
}

// ── the person, three times ───────────────────────────────────────────────────

// promptGrounder establishes a screen by inference, or by asking.
//
// The order matters and is the one [[ADR-186-a-person-can-say-where-they-are]] settled: an
// inference is tried first because it costs nobody anything, and it is allowed only where the
// workflow itself demonstrates the transition AND the arrangement agrees. Everything else is a
// question, because no measure can say whether the right answer is in the list.
type promptGrounder struct {
	art workflowstore.Artifact
	in  *bufio.Reader
	out *os.File
	// arrangement is what the live observer makes of the screen standing now, pushed by
	// the loop because this cannot look for itself.
	arrangement observe.VisualArrangement
	// standing is the label last established, which is what an inference follows from.
	standing string
	// declined remembers a screen nobody could name, so the same question is not asked
	// again in the same observation.
	declined map[observe.VisualScreenID]uint64
	// claim is what somebody said, before the run started, about where they would be
	// standing, and spent says it has been used.
	//
	// # Why a claim made in advance is still a person establishing correspondence
	//
	// [[ADR-186-a-person-can-say-where-they-are]] requires a saved screen to be matched to
	// a live one by somebody who can see both, because no measure can say whether the
	// right answer is in the list. Somebody typing --from before switching to the
	// application is doing exactly that, in the only moment a keyboard is available to
	// them: once the run starts, typing takes the foreground away from the thing being
	// watched and the press it would permit is refused. See consentdesk.go.
	//
	// What keeps it honest is that it is spent ONCE, on the first observation it can be
	// spent in, and is stamped with that observation like any other confirmation. A claim
	// about one screen, not a standing answer: every screen after it is inferred from the
	// workflow's own demonstrated transitions, or nobody has said what it is and the run
	// stops.
	claim string
	spent bool
	// silent says nobody can be asked, because answering would take the foreground. The
	// ranking is still printed: somebody who cannot answer can still read.
	silent bool
}

func (p *promptGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	if _, ok := s.Established(p.standing, at.Epoch, at.Generation); ok {
		return nil // already settled in this observation
	}
	// ── AN INFERENCE, WHERE THE WORKFLOW DEMONSTRATES THE TRANSITION ──
	if p.standing != "" {
		for _, to := range p.next(p.standing) {
			agreed := observe.ArrangementSimilarity(p.arrangementOf(to), p.arrangement)
			if b, err := s.Infer(p.art, p.standing, to, at.Screen, agreed,
				at.Epoch, at.Generation); err == nil {
				p.standing = b.Label
				fmt.Fprintf(p.out, "  %s inferred: %s\n", to, b.Because)
				return nil
			}
		}
	}
	if p.declined == nil {
		p.declined = map[observe.VisualScreenID]uint64{}
	}
	if e, ok := p.declined[at.Screen]; ok && e == at.Epoch {
		return fmt.Errorf("nobody could say which saved screen %s is", at.Screen)
	}

	// ── OR THE CLAIM SOMEBODY MADE BEFORE THE RUN STARTED, SPENT ONCE ──
	if p.claim != "" && !p.spent {
		if err := s.Confirm(p.claim, at.Screen, at.Epoch, at.Generation,
			"the person who started the run (--from)", time.Now()); err != nil {
			return err
		}
		p.spent, p.standing = true, p.claim
		fmt.Fprintf(p.out, "  %s established from --from, in observation %d/%d\n",
			p.claim, at.Epoch, at.Generation)
		return nil
	}

	// ── OTHERWISE SOMEBODY IS ASKED, WITH THE CANDIDATES RANKED AND THE RANKING
	//    LABELLED AS WHAT IT IS ──
	fmt.Fprintf(p.out, "\n  Which saved screen are you looking at?\n")
	for _, c := range p.ranked() {
		fmt.Fprintf(p.out, "    %-10s arrangement agrees %.3f\n", c.label, c.score)
	}
	fmt.Fprintf(p.out, "    (a ranking cannot say whether the right answer is in the "+
		"list; that is what you are adding)\n")
	// NOBODY CAN BE ASKED. The ranking above is printed anyway, because it is what
	// somebody reading the other monitor needs in order to decide whether to start again
	// with a different --from. Refusing silently would make an unanswerable question look
	// like a perception failure.
	if p.silent {
		p.declined[at.Screen] = at.Epoch
		p.standing = ""
		return fmt.Errorf("nobody could say which saved screen %s is, and answering "+
			"here would take the foreground away from the application; name "+
			"the screen with --from and start again", at.Screen)
	}
	fmt.Fprintf(p.out, "  label, or `none`: ")
	answer := strings.TrimSpace(readLine(p.in))
	if answer == "" || strings.EqualFold(answer, "none") {
		p.declined[at.Screen] = at.Epoch
		p.standing = ""
		return fmt.Errorf("nobody could say which saved screen %s is", at.Screen)
	}
	if err := s.Confirm(answer, at.Screen, at.Epoch, at.Generation, "the person at the "+
		"keyboard", time.Now()); err != nil {
		return err
	}
	p.standing = answer
	return nil
}

// next is the screens the workflow demonstrates arriving at from one label.
func (p *promptGrounder) next(from string) []string {
	var out []string
	for _, st := range p.art.Steps {
		if st.From == from && st.To != "" {
			out = append(out, st.To)
		}
	}
	return out
}

func (p *promptGrounder) arrangementOf(label string) observe.VisualArrangement {
	for _, sc := range p.art.Screens {
		if sc.Label == label {
			return sc.Arrangement
		}
	}
	return observe.VisualArrangement{}
}

type labelScore struct {
	label string
	score float64
}

func (p *promptGrounder) ranked() []labelScore {
	out := make([]labelScore, 0, len(p.art.Screens))
	for _, sc := range p.art.Screens {
		out = append(out, labelScore{sc.Label,
			observe.ArrangementSimilarity(sc.Arrangement, p.arrangement)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// promptGranter asks for the right to perform one action.
type promptGranter struct {
	in   *bufio.Reader
	out  *os.File
	live bool
}

func (g *promptGranter) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	what := "write down"
	if g.live {
		what = "SEND"
	}
	fmt.Fprintf(g.out, "\n  %s\n", in.Describe())
	if in.Because != "" {
		fmt.Fprintf(g.out, "    found: %s\n", in.Because)
	}
	fmt.Fprintf(g.out, "    target: %s\n", in.Target(1920, 1080))
	fmt.Fprintf(g.out, "  May Marco %s a left click there, once? [y/N]: ", what)
	if !strings.EqualFold(strings.TrimSpace(readLine(g.in)), "y") {
		return workflowrun.Authorisation{}, false
	}
	return workflowrun.Authorisation{
		Step: in.Step, By: "the person at the keyboard",
		// THE OBSERVATION THIS WAS ASKED ABOUT, carried from the question. Copying it
		// is not evidence that the world stood still — `stillThere` asks that — and it
		// is what makes a cached permission from an earlier screen unspendable.
		Epoch: in.Epoch, Generation: in.Generation,
		// SHORT. A permission that outlived the answer would be a session flag.
		Expires: time.Now().Add(10 * time.Second),
	}, true
}

// agreeToStep puts the agreement question to whoever can answer it.
//
// The desk when a hook is listening, stdin otherwise. The QUESTION is the same either way, and
// in both it stays separate from authorisation: agreeing that a step belongs in the workflow and
// permitting it to happen on the screen in front of you are two sentences, and a build that took
// one answer for both would let somebody answer one and be taken to have answered the other.
func agreeToStep(ctx context.Context, keys *hotkeyGranter, in *bufio.Reader, out *os.File,
	art workflowstore.Artifact, at int, s *workflowstore.SessionBinding) bool {

	if at < 0 || at >= len(art.Steps) {
		return false
	}
	if keys != nil {
		st := art.Steps[at]
		if !keys.agree(ctx, at, st.Control, st.From, st.To) {
			return false
		}
		return s.ConfirmStep(at, "the person at the keyboard") == nil
	}
	return confirmStep(in, out, art, at, s)
}

// confirmStep asks whether somebody agrees to a step at all, which is not the same question as
// whether Marco may press it now.
func confirmStep(in *bufio.Reader, out *os.File, art workflowstore.Artifact, at int,
	s *workflowstore.SessionBinding) bool {

	if at < 0 || at >= len(art.Steps) {
		return false
	}
	st := art.Steps[at]
	fmt.Fprintf(out, "\n  Step %d of the saved workflow: press %s on %s, expecting %s.\n",
		at, st.Control, st.From, st.To)
	fmt.Fprintf(out, "  Do you agree this is a step Marco should perform? [y/N]: ")
	if !strings.EqualFold(strings.TrimSpace(readLine(in)), "y") {
		return false
	}
	return s.ConfirmStep(at, "the person at the keyboard") == nil
}

func readLine(in *bufio.Reader) string {
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return line
}

// ── what it says ──────────────────────────────────────────────────────────────

// reportAttempt prints one observation's worth of work, and only when something happened.
func reportAttempt(a workflowrun.Attempt) {
	switch a.Outcome {
	case workflowrun.OutcomeWaiting, workflowrun.OutcomeUnobserved:
		return // the ordinary case; printing it would bury everything else
	}
	fmt.Printf("  step %d: %s — %s\n", a.Step, a.Outcome, a.Because)
	if a.HoverBecause != "" {
		fmt.Printf("    hover: %s\n", a.HoverBecause)
	}
}

// printRun is the account a person reads afterwards.
//
// The five rows at the end are deliberately five sentences and not one. Collapsing any two of
// them is how a dry run, or a simulated boundary, turns into a success claim.
func printRun(rep workflowrun.Report, art workflowstore.Artifact, book *recordhost.Host,
	live bool, lost string, eyes *learnedEyes, reader *readingEyes,
	readings fastobserve.Stats) {

	fmt.Printf("\n%s\n", strings.Repeat("-", 72))
	fmt.Printf("%s\n", rep.Progress.Summary())
	if lost != "" {
		fmt.Printf("%s\n", lost)
	}
	fmt.Printf("\n")
	fmt.Printf("  %-42s %d of %d\n", "observations Marco could use",
		rep.Usable, rep.Looks)
	if eyes != nil {
		fmt.Printf("  %-42s %s\n", "the learned detector", eyes.describe())
	}
	if reader != nil {
		fmt.Printf("  %-42s %s\n", "the words on each look", reader.describe())
		// WHAT BECAME OF THEM, which is the only honest way to report a source
		// whose results are usually too late to use. A run that read three
		// hundred frames and admitted none is a run where OCR cost minutes and
		// changed no decision, and it must not look like a run where it worked.
		st := readings
		fmt.Printf("      offered %d, admitted %d, refused as stale %d "+
			"(window %d, screen moved on %d), unusable look %d\n",
			st.ReadingsOffered, st.ReadingsAdmitted,
			st.ReadingsStaleWindow+st.ReadingsStaleEpoch,
			st.ReadingsStaleWindow, st.ReadingsStaleEpoch, st.ReadingsUnusable)
		if st.ReadingsRefusedMerge > 0 {
			fmt.Printf("      %d time(s) a reading refused to believe the standing "+
				"screen was the screen the arrangement said\n",
				st.ReadingsRefusedMerge)
		} else if st.ReadingsAdmitted > 0 {
			fmt.Printf("      no reading ever contradicted the standing screen\n")
		}
	}
	for because, n := range rep.Unusable {
		fmt.Printf("      %d x %s\n", n, because)
	}
	fmt.Printf("  %-42s %s\n", "actions lowered, compiled and run",
		fmt.Sprint(rep.Pressed))
	// WHAT WAS SENT, not which mode was asked for.
	//
	// A run that performed nothing sent no real input, whatever --live said, and a report
	// that answered the flag instead of the run said "real input sent YES" above a line
	// counting zero actions. That is the exact shape of claim this report exists to refuse.
	switch {
	case rep.Sent == 0 && live:
		fmt.Printf("  %-42s %s\n", "real input sent",
			"NO — nothing was performed, though --live was asked for")
	case rep.Sent == 0:
		fmt.Printf("  %-42s %s\n", "real input sent", "NO — nothing was performed")
	case live:
		fmt.Printf("  %-42s %s\n", "real input sent", "YES")
	default:
		fmt.Printf("  %-42s %s\n", "real input sent",
			"NO — the Host wrote it down")
	}
	if rep.Dwell > 0 {
		fmt.Printf("  %-42s %s before each press\n", "the pointer was moved and rested",
			rep.Dwell)
	} else {
		fmt.Printf("  %-42s %s\n", "the pointer was moved and rested",
			"NO — the click was sent on its own")
	}
	// VISIBLE CHANGE IS ITS OWN SENTENCE, and it is the one missing from a run that
	// pressed something and saw nothing. "The destination was not established" is true of
	// an interface that ignored the press and of one that responded in a way nobody
	// recognised, and those want opposite things done about them.
	switch {
	case rep.Watched == 0:
		fmt.Printf("  %-42s %s\n", "the control reacted to the pointer",
			"n/a — nobody looked")
	default:
		fmt.Printf("  %-42s %d of %d action(s)\n",
			"the control reacted to the pointer", rep.Hovered, rep.Watched)
	}
	switch {
	case rep.Sent == 0:
		fmt.Printf("  %-42s %s\n", "the screen changed after the action",
			"n/a — nothing was performed")
	case rep.Changed:
		fmt.Printf("  %-42s %s\n", "the screen changed after the action",
			"YES — something happened; what, is the next line's question")
	default:
		fmt.Printf("  %-42s %s\n", "the screen changed after the action",
			"NO — the application did not visibly respond")
	}
	if rep.Budget >= 0 {
		fmt.Printf("  %-42s %d of %d allowed\n", "actions this run was allowed to send",
			rep.Sent, rep.Budget)
	}
	fmt.Printf("  %-42s %d of %d\n", "destinations established", rep.Established,
		len(art.Steps))
	// THE GAP BETWEEN REACHING AND RECOGNISING, which is a product number and not a
	// diagnostic. An action that moved the application somewhere Director could not name
	// is a navigation that worked and a recognition that did not, and the two have to be
	// counted apart or the first is filed as a failure of the second.
	if rep.StepsPressed > 0 {
		fmt.Printf("  %-42s %d of %d\n",
			"  actions that moved an unrecognised screen",
			rep.Unrecognised, rep.StepsPressed)
	}
	fmt.Printf("  %-42s %d\n", "  of those, independent of the workflow",
		rep.Independent)
	ok, why := rep.Verified()
	if ok {
		// Unreachable by construction; printed rather than assumed so that the day it
		// changes, this line is the one that has to be rewritten.
		fmt.Printf("  %-42s %s\n", "actions independently verified", "ESTABLISHED")
	} else {
		fmt.Printf("  %-42s %s\n", "actions independently verified", "NOT ESTABLISHED")
		fmt.Printf("      %s\n", why)
	}
	if !live {
		fmt.Printf("\n")
		for _, line := range book.Describe() {
			fmt.Printf("  %s\n", line)
		}
	}
}
