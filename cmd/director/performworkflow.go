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
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
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
	file := fs.String("workflow", "", "the saved workflow artifact FILE to perform")
	// ── AND THE NAME THE PERSON GAVE IT ──
	//
	// The gap between the two halves of this product, and it was one flag wide. A person
	// demonstrates a menu action, `demonstrate-visual` scopes it to the application it
	// watched and saves it in the demo store BY NAME, and `correct`, `reacquire` and
	// `what-i-did` all take that name. The one command that ACTS took a file path, so the
	// only way to run what you had just demonstrated was to find its JSON in the config
	// directory and pass that.
	//
	// `--workflow` stays, and stays a file: a derived fixture is a file that was never
	// saved by name, and the controlled execution tests run from one.
	named := fs.String("name", "",
		"the name a demonstration was saved under, as `director sequences` lists "+
			"it. Use "+
			"this to run what you demonstrated; --workflow is for an artifact file")
	events := fs.String("events", "",
		"write this run's structured state to `path`, one JSON object per line: the "+
			"phase, the question in front of the person and the keys it accepts, "+
			"the preparation deadline, and how it ended. For a front end; the "+
			"ordinary output on this terminal is unchanged")
	play := fs.String("play", "",
		"the name of a saved demonstrated ACT: a `.marco` play whose own sentences "+
			"are the order. Its bindings are resolved and the play rebuilt from "+
			"them before anything is grounded, and the routine it performs comes "+
			"from the play rather than from --name")
	granted := fs.String("granted", "",
		"perform the Act under the authorisation in `path`, given in advance: the "+
			"person chose these interactions and said to perform them, so the "+
			"run does not ask again. One Act, one revision, one application, one "+
			"run — and a press of anything the file does not name is refused. "+
			"Requires --play; see actgrant.go for what a grant is and is not")
	live := fs.Bool("live", false,
		"send REAL input. Without this the whole path runs and nothing is pressed")
	fixedLayout := fs.Bool("fixed-layout", false,
		"when a screen cannot be established at all, press where the "+
			"demonstration left the control — ONLY while the window is the "+
			"size it was demonstrated in. A narrow exception for a menu no "+
			"recogniser can name, off unless asked for; it skips no "+
			"confirmation and no authorisation, and every press it plans is "+
			"reported as resting on the layout rather than on recognition")
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
	step := fs.Int("step", -1,
		"perform only this step of the workflow, extracted on its own (-1 = all)")
	verifyFor := fs.Duration("verify-for", workflowrun.DefaultVerifyDeadline,
		"how long after an action to wait for the readings that verify its outcome")
	fg := fs.Bool("foreground", false,
		"act on THE WINDOW IN FRONT, resolved once at the start, instead of searching "+
			"by name. Exact where --application is ambiguous, and it authorises "+
			"nothing: the artifact's scope is still checked against the window "+
			"actually acquired")
	pid := fs.Uint("pid", 0, "act on the window of exactly this process")
	windowID := fs.String("window-id", "",
		"act on the window with this ephemeral id from `director windows`")
	dwell := fs.Duration("dwell", workflowrun.DefaultDwell,
		"how long the pointer rests on a control before the click. 0 sends the click "+
			"alone, which a game menu ignores")
	_ = fs.Parse(flagsFirst(args))

	// ── WHICH APPLICATION, DISCOVERED WHEN IT WAS NOT NAMED ──
	//
	// `--application` is now a TARGET SELECTOR and no longer required: Marco reads the
	// foreground itself, cold, with no daemon running. See foreground.go.
	//
	// It selects what to LOOK AT and decides nothing about whether the workflow may act
	// there — that is the artifact's scope, checked below against the window actually
	// acquired. Keeping the two apart is what stops a discovered application from quietly
	// satisfying a scope check.
	began := time.Now()
	var waited bool
	// ── WHEN NOBODY NAMED THE APPLICATION, WAIT BEFORE LOOKING ──
	//
	// `--delay` exists so a person can start the run and then switch to the thing it
	// watches. Discovery ran HERE, at the first instruction of the command, so it read
	// whatever was in front while they were still typing — their terminal.
	//
	// Measured live, and it blocked the owner's own test outright:
	//
	//	this workflow was demonstrated against "arkascended" and the application
	//	resolved now is "code"
	//
	// with `--delay 10s` on the command line and the game one alt-tab away. The scope
	// gate was right and the question was asked at the wrong moment. Waiting only when
	// the application is DISCOVERED keeps `--application` instant, which is what somebody
	// naming it explicitly is asking for.
	// ── AND CTRL-C REACHES THE WAIT ITSELF ──
	//
	// This was an unconditional `time.Sleep` and the cancellation context was created
	// three hundred lines below it, so the one window in which a person is most likely to
	// change their mind — they have just started a run and are looking at the game — was
	// the one window nothing could interrupt. Ctrl-C killed the process instead of
	// stopping the run, which is a different thing: a stopped run says how far it got.
	//
	// Hoisted rather than duplicated. `signal.NotifyContext` installs a handler and
	// nothing else, so creating it earlier costs nothing and the whole command now runs
	// under one cancellation.
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	// ── AND SO DOES THE WORD "STOP", FROM WHEREVER IT WAS SAID ──
	//
	// Ctrl-C is a terminal's stop and F9 is a desktop's. Neither is the product's: a
	// person who typed or said "stop" to the overlay reaches `marco`'s one stop, which
	// raises the local generation every running Play watches and tells the Director
	// service to drop what it is holding. This run is neither of those things — it is a
	// command, started by a front end, with no terminal to interrupt — so until now the
	// one word a person is most likely to use could not reach the one thing most worth
	// stopping.
	//
	// `stopsignal.Watch` is the same mechanism and the same home every Play already
	// watches, so there is no second broadcast and nothing new to raise. A cancelled
	// context is how the rest of this command already hears a stop, and the classifier
	// already reads `ctx.Err()` as cancellation — so a run stopped this way records
	// itself as cancelled, which is not a run that failed.
	//
	// Deleting this must fail TestAStopSaidAnywhereReachesASupervisedRun.
	ctx, stopWatching := stopsignal.Watch(ctx, stopsignal.Home())
	defer stopWatching()

	// ── AND WHICH WINDOW, when one executable owns two ──
	//
	// An executable name is the weakest selector and the only one that can be ambiguous.
	// Measured live, on this machine: a controlled test page in an Edge app window could
	// not be acted on at all, because the person's own Edge was also showing a floating
	// `Search bar`. The refusal pointed at `director windows --window-id` — a command that
	// needs the daemon running, naming a flag this command did not have.
	//
	// This chooses what to LOOK AT and authorises nothing. The scope check below is asked
	// of `v.Ref.Application`, the executable of the window actually acquired, so naming a
	// pid cannot make an artifact valid anywhere its scope does not already allow.
	switch {
	case *play != "" && *step >= 0:
		// --play AND --step WOULD PERFORM THE WRONG INTERACTION.
		//
		// `--step` extracts one step into a one-step artifact IN MEMORY, which
		// renumbers it to position 0. A play's bindings name ORIGINAL positions, and
		// nothing renumbers them — so `--play X --step 1` leaves the play's first
		// Scene bound to position 0 while position 0 now holds what used to be
		// interaction 1. The sentence a person reads would press the other control,
		// with every gate satisfied and nothing to notice.
		//
		// Refused rather than reconciled: an Act already says which interactions it
		// performs and in what order, so narrowing it with a flag is asking two things
		// to decide one question. Perform the whole Act, or compose one from the
		// interaction you want.
		fmt.Fprintln(os.Stderr, "director: --play and --step cannot be used "+
			"together.\n"+
			"  An Act already says which interactions it performs, and --step "+
			"renumbers\n"+
			"  the demonstration underneath it — so the play's first Scene would "+
			"perform\n"+
			"  whichever interaction --step moved into its place. Run the whole "+
			"Act, or\n"+
			"  compose one from the interaction you want.")
		return 2
	case *file == "" && *named == "" && *play == "":
		fmt.Fprintln(os.Stderr,
			"director: name what to perform.\n"+
				"  --name \"graphics tab\"      a demonstration you saved\n"+
				"  --play \"graphics tab\"      a play composed from one\n"+
				"  --workflow routine.json   an artifact file")
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

	// ── THE ROUTINE IS LOADED BEFORE ANYBODY IS ASKED TO WAIT ──
	//
	// A name that was never saved is knowable without looking at a screen, and finding
	// out after a countdown means somebody alt-tabbed to their game and waited to be told
	// they mistyped. Measured on the first run of this code.
	//
	// AFTER the usage checks above and before the countdown below, and both halves of
	// that matter. Moved above the checks, a bad `--workflow` path exits 1 where a usage
	// refusal should exit 2 — which `TestPerformWorkflowIsDispatchedAndReachable` caught
	// the first time this was reordered.
	art, plan, code := whatToPerform(*file, *named, *play)
	if code != 0 {
		return code
	}

	// ── THE AUTHORISATION, READ AND CHECKED BEFORE ANYTHING IS OBSERVED ──
	//
	// A grant is a decision somebody already made about a thing they were shown, so every
	// way it can fail to be that is answerable here: an unreadable file, a grant for
	// another Act, a revision that has been composed again since, an interaction the play
	// does not perform. All of them exit before a window is acquired, because the one
	// thing this must never do is fall back to performing under a guess about what the
	// person meant.
	//
	// Exit 2 and not 1 for the usage half — `--granted` without `--play` is a wrong
	// invocation — and 1 for a grant that does not match, which is a correct invocation
	// pointed at something that cannot be performed.
	var grantedBy *actGrant
	if *granted != "" {
		if plan == nil {
			fmt.Fprintln(os.Stderr, "director: --granted authorises an ACT, so it "+
				"needs --play; there is nothing for it to authorise here")
			return 2
		}
		g, gerr := readActGrant(*granted)
		if gerr != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", gerr)
			return 2
		}
		if aerr := g.agrees(*play, *plan); aerr != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", aerr)
			return 1
		}
		grantedBy = &g
	}

	// ── THIS RUN'S OWN NAMESPACE, CLAIMED BEFORE ANYTHING IS WRITTEN INTO IT ──
	//
	// The record and the pixels at once. The hover probe writes `01-before.png` and
	// `01-after.png`, so two runs sharing a --frames directory overwrote each other and an
	// earlier record then pointed at a later run's images; and the record's own guard used
	// to be a refusal at SAVE time, by which point both attempts had been printing the
	// same reference for the length of a run.
	//
	// `reserveRun` claims both places by the same id and tries another id if either is
	// taken, so an attempt that gets this far has a reference nothing else can write to.
	// It is claimed before the reference is printed, which is the ordering the whole thing
	// turns on.
	runID, framesDir, rerr := reserveRun(began, *frames, nil)
	if rerr != nil {
		// NO REFERENCE, SO NOTHING TO CITE. Reported as what it is rather than
		// as a run that failed, because nothing has been attempted yet.
		fmt.Fprintf(os.Stderr, "director: a run reference could not be "+
			"reserved: %v\n", rerr)
		return 1
	}

	// ── ONE FINALISATION PATH, FROM HERE TO EVERY EXIT ──
	//
	// `keep` writes the record for this attempt and `endedEarly` is the exit that uses it
	// before a run ever starts. Both exist because the record used to be written at one
	// place — the bottom — and every return above it left nothing behind: pressing F9 at
	// the start prompt, Ctrl-C during either wait, a window that could not be acquired, a
	// scope that refused. Those are the attempts somebody is most likely to ask about,
	// and they were the attempts with no reference and no record.
	//
	// `RunFailed` was declared and never produced for the same reason: nothing on a
	// startup path reached a classifier.
	//
	// A zero `Report` gives zero actions sent, which is what a cancelled-before-anything
	// attempt should say, and the counts are carried through unchanged when there is a
	// report — a run cancelled after an action verified did verify that action.
	keep := func(out RunOutcome, because string, rep workflowrun.Report) (string, error) {
		// THE ACT'S OWN IDENTITY, beside the demonstration it performs. Empty for an
		// ordinary workflow run, which is what `omitempty` is for.
		actName, actSource := "", ""
		if plan != nil {
			actName, actSource = *play, plan.Source
		}
		// AND WHAT PERMITTED THE PRESSES, when it was not a key per press.
		authority := ""
		if grantedBy != nil {
			authority = grantedBy.describe()
		}
		return saveRun(RunRecord{
			Version: 1, Run: runID, Workflow: art.Name,
			Act: actName, ActSource: actSource, Authority: authority,
			Application: string(art.Application), Step: *step,
			Started: began.Format(time.RFC3339),
			Ended:   time.Now().Format(time.RFC3339),
			Outcome: out, Because: because,
			Sent: rep.Pressed, Real: rep.Real,
			Verified: rep.VerifiedOK, Contradicted: rep.Contradicted,
			Unconfirmed: rep.Unconfirmed, Frames: framesDir,
		})
	}
	// ── AND THE EXIT THAT USES IT, WITH THE CODE THE EXIT WOULD HAVE RETURNED ──
	//
	// The code is an argument because a usage refusal past this point is still a usage
	// refusal: `--step` out of range and a selector nobody narrowed both exit 2, and
	// both of them happen AFTER the reference has been printed, because the selector
	// cannot be checked until the foreground has been resolved and that cannot happen
	// until the countdown is over. Folding them into exit 1 to get a record would change
	// what the exit code means; leaving them unrecorded cites a run id nobody kept.
	// THE STRUCTURED ACCOUNT, DECLARED HERE AND OPENED BELOW.
	//
	// Declared before `endedEarly` because every early refusal goes through that, and a
	// refusal is a COMPLETE answer: a run that exits with no final event is one nobody
	// can speak for, and calling an honest refusal that would be a lie about it. Nil
	// until the reference is minted, and `Say` on nil does nothing — an exit before the
	// writer exists leaves no stream at all, which a reader can tell from an incomplete
	// one.
	var say *runstate.Writer
	endedEarly := func(code int, out RunOutcome, because string) int {
		say.Say(runstate.Event{Phase: runstate.Ended, Final: true,
			Outcome: out, Reason: because})
		if p, err := keep(out, because, workflowrun.Report{}); err != nil {
			fmt.Fprintf(os.Stderr,
				"director: the run record could not be written: %v\n", err)
			fmt.Printf("%s\n", citeRun(runID, ""))
		} else {
			fmt.Printf("%s\n", runref.Ended(runID, p, out))
		}
		return code
	}

	// THE REFERENCE, BEFORE ANYBODY IS ASKED TO PREPARE. A run worth citing is usually
	// one that went wrong, and the ones that go wrong earliest used to print it last.
	fmt.Printf("%s\n", citeRun(runID, runPath(runID)))

	// ── THE STRUCTURED ACCOUNT OF THIS RUN, FOR A FRONT END ──
	//
	// Opened here because the reference is minted just above and belongs on every event:
	// a reader of any single line knows which run it is about. Nil when nobody asked for
	// one, and every emission is safe on nil, so no call site below has to check.
	//
	// The ordinary output on this terminal does not change. These are a second account of
	// the same decisions, not a replacement for the one a person reads.
	w, serr := runstate.NewWriter(*events, runID, *play, art.Name, *live)
	if serr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", serr)
		return endedEarly(1, RunFailed, serr.Error())
	}
	say = w
	defer say.Close()

	// stoppedByKey is set by the stop key, which stops the RUN and does not cancel this
	// command's context — so without it an F9-stopped attempt was classified from
	// whatever the report happened to say and could be recorded as refused, unconfirmed
	// or even verified. Atomic because the desk sets it from its own goroutine.
	var stoppedByKey atomic.Bool

	// ── THE KEY SOURCE, OPENED ONCE FOR THE WHOLE RUN ──
	//
	// Needed here because START is a key press, and needed again below for the consent
	// desk. ONE install: `navsource.New` puts a low-level hook on the whole desktop, and
	// two of those is the shape of defect CLAUDE.md names — a second install is a second
	// callback on every keystroke everybody makes. Opened before the countdown, closed
	// when the command returns, owned by one reader at a time.
	var src *navsource.Source
	if *hotkeys && !*explore {
		s, why := newKeySource()
		if why != "" {
			fmt.Fprintf(os.Stderr, "director: no key hook is available, so "+
				"--hotkeys has no way to hear an answer: %s\n", why)
			return endedEarly(1, RunFailed, "no key hook was available, so "+
				"there was no way to hear a start or a stop: "+why)
		}
		src = s
		defer src.Close()
	}

	// ── NOW THE PERSON IS ASKED TO SWITCH, AND ONLY NOW ──
	//
	// Everything above is knowable without looking at a screen: a flag out of range, a
	// name nobody saved, a file that is not an artifact. Asking somebody to alt-tab to
	// their game and then telling them they mistyped is the order this used to run in.
	//
	// WITH A KEY WHEN THERE IS ONE, because a clock is a guess about when somebody is
	// ready and this is the moment the owner asked to stop guessing:
	//
	//	An explicit Start interaction after the user has prepared the trial.
	//
	// The same key the desk uses, so there is one thing to learn, and it is heard while
	// the game keeps the foreground — which is the whole reason the desk uses keys.
	// ── OR THERE IS NOTHING TO PREPARE, BECAUSE SOMEBODY ALREADY DID ──
	//
	// A granted run does not ask anybody to get ready and does not count down to a guess
	// about when they are. Pressing Play IS the "I am ready": the person was looking at
	// the surface, chose the Act, and said to perform it. The run acquires the window,
	// brings it forward and begins.
	//
	// Said as `starting` immediately, which is the same thing the keyed path says the
	// instant its gate ends — so a window has one phase to render for "working on it,
	// nothing is being asked" whichever contract the run is under.
	if grantedBy != nil {
		waited = true
		fmt.Printf("  Performing %q under the authorisation of %s.\n\n", *play,
			grantedBy.describe())
		say.Say(runstate.Event{Phase: runstate.Starting})
	}
	if src != nil && grantedBy == nil {
		// THE PREPARATION DEADLINE, SAID BY THE THING THAT HOLDS IT.
		//
		// A front end may count down to this instant and show what it computes.
		// It may not decide that preparation is over: the run ends its own
		// preparation, and a window acting on its own arithmetic would be
		// deciding something it is not in a position to know.
		say.Say(runstate.Event{Phase: runstate.Preparing,
			Preparation: time.Now().Add(*delay),
			Question: &runstate.Question{ID: "start",
				Asks: "Prepare the application, then say when you are ready.",
				// WHAT START ACTUALLY ACCEPTS, which is two keys and
				// not three. See startKeys.
				Keys: startKeys(), Until: time.Now().Add(*delay)}})
		if started(ctx, src, *delay) == startCancelled {
			say.Say(runstate.Event{Phase: runstate.Ended, Final: true,
				Outcome: RunCancelled,
				Reason: "cancelled at the start prompt, before any window " +
					"was acquired or anything was observed"})
			fmt.Println("  cancelled before anything was observed; no input " +
				"was sent")
			return endedEarly(1, RunCancelled,
				"cancelled at the start prompt, before any window was "+
					"acquired or anything was observed")
		}
		// AND THE PREPARATION IS SPENT. Pressing F7 used to fall through to the
		// `Starting in %s` countdown below and wait the delay all over again —
		// so saying "ready" cost a second wait, and letting Start time out cost
		// two. One preparation gate, however it ends.
		waited = true
		// ── AND IT IS SAID TO BE SPENT, AT THE INSTANT IT WAS ──
		//
		// However it ended: somebody pressed start, or the delay ran out and the
		// run began by itself. Both are preparation OVER, and neither used to be
		// emitted — so the last thing a front end had heard was a deadline and a
		// question, and it went on counting down to an instant that had passed
		// and offering a key that nothing was listening for, through acquisition
		// and provider start-up. Seconds of showing somebody a live prompt that
		// was not live.
		//
		// No question and no deadline, which is what makes this the question
		// coming DOWN rather than a phase change beside it.
		say.Say(runstate.Event{Phase: runstate.Starting})
	}
	if src == nil && grantedBy == nil && lookAfterWaiting(*app, *delay) {
		fmt.Printf("  Switch to the application now — what is in front in %s is what "+
			"this will watch.  Ctrl-C cancels.\n", *delay)
		// THE SAME DEADLINE, ON THE PATH WITH NO KEYS. A clock-only preparation is
		// still preparation, and a front end counting down to it is counting down to
		// something real. No question, because there is nothing anybody can answer:
		// this gate ends when the clock says so.
		say.Say(runstate.Event{Phase: runstate.Preparing,
			Preparation: time.Now().Add(*delay)})
		if !countdown(ctx, *delay) {
			// NOTHING HAS BEEN ACQUIRED, AUTHORISED OR BUILT YET, so there is
			// nothing to wind back and nothing could have been pressed.
			fmt.Println("  cancelled before the application was discovered; " +
				"nothing was observed and no input was sent")
			return endedEarly(1, RunCancelled, "cancelled during the preparation "+
				"wait, before any window was acquired")
		}
		waited = true
		// AND SPENT, said here too: the deadline above has passed and a window
		// still showing it would be showing a countdown that ended.
		say.Say(runstate.Event{Phase: runstate.Starting})
	}
	target, targetWhy := resolveApplication(context.Background(), *app)
	sel := windowSelector(target, *windowID, uint32(*pid), *fg)
	if sel.Zero() {
		fmt.Fprintf(os.Stderr, "director: which application? %s\n"+
			"  name it: director perform-workflow --application solasta "+
			"--workflow routine.json\n", targetWhy)
		return endedEarly(2, RunFailed, "no application was named and none "+
			"could be identified: "+targetWhy)
	}

	// ── ONE STEP, EXTRACTED, WHEN THAT IS WHAT WAS ASKED FOR ──
	//
	// A workflow runs from step 0 forward, and the first step of a real demonstration is
	// not always the one worth performing first: Ark's capture learned three, and step 0
	// presses a target that ABSTAINED — a 73x12 rectangle with no word in it. Reaching
	// step 2 by running the file means pressing that on the way.
	//
	// Extracted IN MEMORY rather than written out, which is the narrowest thing that
	// works: no second file to run by mistake, no new format, and the step keeps its own
	// starting screen, its control's evidence, its outcome expectation, the application
	// scope and the instant that links it to the raw demonstration. See
	// workflowstore/juststep.go for what travels and what does not.
	if *step >= 0 {
		one, serr := art.JustStep(*step)
		if serr != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", serr)
			return endedEarly(2, RunFailed, fmt.Sprintf("--step %d is not a "+
				"step of this routine: %v", *step, serr))
		}
		art = one
		fmt.Printf("Performing step %d of %q on its own: %s -> %s via %s\n",
			*step, art.Name, art.Steps[0].From, art.Steps[0].To,
			art.Steps[0].Control)
		fmt.Printf("  the steps before it are not in this run and are not implied " +
			"by it\n")
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
			return endedEarly(1, RunFailed, fmt.Sprintf("--from named %q, which "+
				"this workflow does not carry", *from))
		}
	}

	interval := time.Second / time.Duration(*fps)
	obs, err := fastobserve.New(interval, observeQueue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return endedEarly(2, RunFailed, "the observer could not be started: "+
			err.Error())
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
			return endedEarly(1, RunFailed,
				"the desktop was not available: "+err.Error())
		}
		defer release()
	}

	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)

	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(),
		sel)
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		if v.State == windowref.Ambiguous {
			fmt.Fprintln(os.Stderr,
				"  one executable can own two windows. Foreground the one you "+
					"mean and pass --foreground, or name its process with "+
					"--pid. Neither authorises anything: the artifact's "+
					"scope is still checked against the window acquired.")
		}
		return endedEarly(1, RunFailed, "the window could not be acquired: "+v.Reason)
	}
	// AND THE TARGET NAME BECOMES THE OBSERVED ONE, not the one that was asked for.
	//
	// Reacquisition searches by application name, so under `--foreground` or `--pid` it
	// had nothing to search by — and under `--application` it would search by a caller's
	// word. The window layer reports the executable of the window it actually acquired,
	// which is the identity reacquisition is meant to use.
	target = v.Ref.Application

	// ── THE SCOPE GATE, BEFORE ANYTHING ELSE IS BUILT ──
	//
	// A workflow may only act on the application it was demonstrated against, and the
	// comparison is against the window that was ACQUIRED — `v.Ref.Application`, the
	// executable image the window layer reports — never against `--application`, which is
	// a request rather than an observation.
	//
	// This is the defect the owner called release-blocking: `Artifact` recorded no
	// application at all, so an Ark workflow pointed at Solasta would acquire Solasta's
	// window and start resolving Ark's controls against it, with nothing in the way but a
	// person answering a screen question. See workflowstore/scope.go.
	//
	// Checked HERE, before the grounder, the granter, the executor or the observer exist:
	// a refusal that happens after a run has been assembled is a refusal somebody has to
	// trust was reached, and this one returns before anything can press.
	if ok, why := art.ScopeAllows(v.Ref.Application); !ok {
		fmt.Fprintf(os.Stderr, "director: %s\n", why)
		if !art.Scoped() {
			fmt.Fprintf(os.Stderr, "  Demonstrate it again to record which "+
				"application it belongs to:\n"+
				"    director demonstrate-visual %q --application %s\n",
				art.Name, v.Ref.Application)
		}
		fmt.Fprintf(os.Stderr, "  Nothing was pressed.\n")
		// REFUSED RATHER THAN FAILED. The scope gate working is not the run
		// breaking, and a record that called it a failure would send somebody
		// looking for a fault in Marco rather than at which application is in front.
		return endedEarly(1, RunRefused, "the workflow's application scope did not "+
			"allow the window acquired; nothing was pressed")
	}
	ask := bufio.NewReader(os.Stdin)
	session := workflowstore.NewSessionBinding()
	// ── WHICH INTERACTION EACH SNAPSHOT IS ABOUT ──
	//
	// Two numberings meet here. The coordinator counts the DEMONSTRATION's steps, and an
	// Act counts its own sentences — a subset, in an order of its own — so an Act that
	// names interaction 2 of a three-step routine is on its FIRST. Publishing the step as
	// the Act's position said "2" about the first of one. See interactionidentity.go.
	//
	// A walk of an artifact is the case where the two agree, and it says so rather than
	// relying on them agreeing.
	scenes := workflowInteractions(art)
	if plan != nil {
		scenes = actInteractions(art, len(plan.Order))
	}
	ground := &promptGrounder{art: art, in: ask, out: os.Stdout,
		// NOBODY CAN BE ASKED in either mode: answering by keyboard would take the
		// foreground, and an exploring session is authorised precisely so that it
		// does not stop to ask. Every screen after --from is inferred from the
		// workflow's own demonstrated transitions, or the run stops and says so.
		claim: *from, silent: *hotkeys || *explore, events: say}

	// ── WHO IS ASKED, AND HOW THEY ANSWER ──
	//
	// `--hotkeys` exists because of a measured failure, not a preference: typing an answer
	// makes the terminal the foreground window, and the press the answer permits is then
	// refused because the target no longer leads the desktop. See consentdesk.go.
	// `run` is declared before the desk so the STOP key can reach it. A desk built with
	// no way to stop the run would make F9 a key that prints something.
	var run *workflowrun.Runner
	// stopper is how the key goroutine reaches the runner: published once, read
	// atomically, and nil until there is something to stop. See the desk callback.
	var stopper atomic.Pointer[workflowrun.Runner]
	var session0 *sessionGrant
	var grant workflowrun.Granter = &promptGranter{in: ask, out: os.Stdout, live: *live}
	var keys *hotkeyGranter
	var desk *consentDesk
	// byGrant is the authorisation given in advance, when there is one.
	//
	// It REPLACES the desk as the thing that permits a press, and it is assigned before
	// the hotkey branch below so that a granted run with `--hotkeys` still installs the
	// hook — for the stop key only. F9 is the one key that means something under this
	// contract, the desk obeys stop whether or not anything is being asked, and F7 or F8
	// pressed at a granted run are counted as strays and change nothing.
	var byGrant *grantedGranter
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
	if src != nil {
		// THE SAME SOURCE THE START KEY WAS HEARD ON, handed over to the desk now
		// that nothing else is reading it. A second `navsource.New` here would
		// install a second desktop-wide hook for the rest of the run.
		// ── THE STOP CALLBACK OUTLIVES NOTHING AND WAITS FOR NOTHING ──
		//
		// It used to call `run.Stop()` directly, and the desk's goroutine starts
		// HERE while `run` is assigned fifty lines below — so a stop key pressed
		// in between dereferenced a nil runner and took the process down with it.
		// Afterwards it was an unsynchronised read of a pointer another goroutine
		// had written, which is a race whether or not it ever misbehaved.
		//
		// An atomic pointer fixes both, and the flag is what makes an early stop
		// mean something rather than being dropped: whoever publishes the runner
		// below asks whether a stop already arrived.
		desk = newConsentDesk(src.Consents(), func() {
			stoppedByKey.Store(true)
			if r := stopper.Load(); r != nil {
				r.Stop()
			}
		})
		go desk.run(ctx)
		// THE DESK SAYS WHAT IT ASKS, in the structured account as well as on the
		// terminal. See consentdesk.go: the thing that puts a question is the thing
		// that knows it was put, and that it came down again.
		// ── AND UNDER A GRANT, THE DESK PERMITS NOTHING ──
		//
		// The hook above is still installed and still reaches the stop callback, so
		// F9 stops a granted run. What the desk does NOT do is hold the
		// authorisation or answer the screen question: neither is built, rather than
		// being built and auto-answered, because a run that answered its own
		// questions would write "the person at the keyboard (F7)" into the record of
		// a press nobody pressed a key for. See actgrant.go.
		if grantedBy == nil {
			keys = &hotkeyGranter{desk: desk, out: os.Stdout, live: *live,
				within: *answer, life: 10 * time.Second, events: say,
				scenes: scenes}
			grant = keys
			// AND THE SAME KEYS ANSWER THE ONE QUESTION THAT IS NOT ABOUT AN
			// ACTION.
			//
			// Which saved screen is in front of you. Without this a hotkey run
			// refuses every first observation with "name the screen with
			// --from and start again", and from a widget there is no terminal
			// to name it in. See promptGrounder.byKey.
			//
			// Deleting this must fail
			// TestAHotkeyRunEstablishesItsScreenByKey.
			ground.byKey = keys.confirmScreen
		}
	}
	// ── THE GRANT IS WHAT PERMITS A PRESS, WITH OR WITHOUT A KEY HOOK ──
	//
	// Outside the branch above because it does not depend on one: a granted run with no
	// `--hotkeys` installs no hook at all, and Stop still works — it is the one stop
	// broadcast this product has, raised by `director stop` and by the surface's own Stop
	// button, and it is watched on this run's context well above here.
	if grantedBy != nil {
		byGrant = &grantedGranter{grant: *grantedBy, out: os.Stdout, live: *live,
			life: 10 * time.Second, events: say, scenes: scenes}
		grant = byGrant
		// AND THE SCREEN IS IDENTIFIED RATHER THAN ASKED ABOUT. See
		// promptGrounder.identify and workflowstore.Standing.
		ground.identify = grantedBy.describe()
	}

	windowNow := func() (directorapi.Window, bool) {
		v := tracker.Acquire(ctx, target)
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
		words := openWordReader(shots, windowNow)
		if !words.working() {
			fmt.Fprintf(os.Stderr, "director: --repair needs the OCR plugin and "+
				"it is not available: %s\n", words.why)
			return endedEarly(1, RunFailed, "--repair needs the OCR plugin and "+
				"it is not available: "+words.why)
		}
		repairs = &regrounder{art: art, out: os.Stdout, words: words.read}
	}

	probe := &hoverProbe{shots: shots, moved: 8, frames: framesDir, window: windowNow}

	// ── THE SLOW TEXT SOURCE, DECLARED BEFORE THE RUNNER THAT CONSULTS IT ──
	//
	// Built further down, with --reading, because OCR costs hundreds of milliseconds and
	// an ordinary run neither needs it nor pays for it. Declared HERE because the
	// authorisation-boundary look wants whatever it has finished reading, and the closure
	// that takes that look is installed on the runner above the place the reader is made.
	// A closure over the variable rather than its value, so the look sees the reader that
	// exists when the look is taken.
	var reader *readingEyes

	run = workflowrun.New(art, nil, ground, grant).
		WithExecutor(marcoexec.New(marco), *live).
		WithHover(probe.look).
		// WHERE THE POINTER ACTUALLY IS, asked of the operating system rather than
		// of a frame, because BitBlt does not composite the cursor. Deleting this
		// call puts back the run that pressed through a pointer 452 px from its
		// target and called the result "the control did not react" — see
		// TestPerformWorkflowInstallsItsGates.
		WithPointer(pointerNow).
		WithRegrounder(regroundOrNil(repairs)).
		WithBudget(*maxActions).
		WithDwell(*dwell).
		// THE WORLD, RE-OBSERVED AT THE INSTANT OF THE PRESS. Deleting this call, or
		// passing nil, must make the runner refuse every action — see
		// TestPerformWorkflowInstallsItsGates.
		// THE DEMONSTRATED WINDOW REACHES THE PRESS BOUNDARY, or nil when this
		// run is not in geometry replay mode. Nil leaves the vision boundary
		// exactly as it was, in every term.
		WithConfirm(stillThere(tracker, windows, target, art.Application,
			replayWindow(*fixedLayout, art),
			freshLook(shots, obs, windows, structure.DefaultOptions(),
				func() *readingEyes { return reader }))).
		// HOW LONG AN ACTION'S OWN EVIDENCE IS WAITED FOR, which is a different
		// scarcity from patience: patience counts LOOKS of an unestablished screen,
		// and this counts TIME for enough readings to decide whether the action did
		// what it did when it was shown. See workflowrun/verify.go.
		WithVerifyDeadline(*verifyFor).
		// THE FIXED-LAYOUT LICENCE, WHICH IS OFF UNLESS SOMEBODY ASKED.
		//
		// Granted here, from the command, because that is where a person's
		// request for it arrives — the runner cannot turn it on for itself and
		// the planner cannot infer it. See workflowstore.LayoutReplay for what
		// it does and does not permit.
		WithFixedLayout(*fixedLayout).
		WithPatience(*patience)

	// AND NOW THERE IS SOMETHING TO STOP. Published before the first observation,
	// and a stop that arrived while it was being built is honoured here rather than
	// dropped — the key was pressed, so the run does not start.
	stopper.Store(run)
	if stoppedByKey.Load() {
		run.Stop()
	}

	fmt.Printf("Performing %q against %s\n", art.Name, v.Ref.Describe())
	fmt.Printf("  %d step(s), %d screen(s), %d control(s)\n",
		len(art.Steps), len(art.Screens), len(art.Controls))
	if *live {
		fmt.Printf("  LIVE: real input will be sent, one authorised action at a time.\n")
	} else {
		fmt.Printf("  DRY: every program is compiled and run, and the Host writes it " +
			"down instead of pressing anything.\n")
	}
	// ── AND SAY SO WHEN THIS RUN MAY PRESS WITHOUT RECOGNISING ──
	//
	// On the run's own first lines, beside LIVE, because it is the same kind of fact: a
	// person watching has to be able to see which of two different things is about to
	// happen. What the demonstration was made in is named, because that is the condition
	// the whole licence hangs on and the one a person can check at a glance.
	if run.FixedLayout() {
		where := "and this routine did not record the window it was demonstrated " +
			"in, so nothing will be pressed on that basis"
		if art.Window.Known() {
			where = fmt.Sprintf("while the window is %dx%d, which is what it "+
				"was demonstrated in", art.Window.Width, art.Window.Height)
		}
		fmt.Printf("  FIXED LAYOUT: a screen that cannot be established will not "+
			"stop this run. Where the demonstration left a control is where it "+
			"will be pressed, %s.\n", where)
	}
	fmt.Printf("  Ctrl-C stops the run. Nothing is written to disk.\n")
	if waited {
		// ALREADY SPENT, before the application was discovered. Waiting a second
		// time would double a delay somebody asked for once.
		fmt.Printf("  Starting now; the delay was spent finding the application.\n\n")
	} else {
		fmt.Printf("  Starting in %s.\n\n", *delay)
		select {
		case <-time.After(*delay):
		case <-ctx.Done():
			return endedEarly(1, RunCancelled, "cancelled during the "+
				"preparation delay, before the run began")
		}
	}

	// A CONSUMER, so the observer's queue is read and its drop count means something.
	go func() {
		for range obs.Updates() {
		}
	}()

	// THE SECOND SOURCE, on its own clock. See learnedeyes.go.
	var eyes *learnedEyes
	if *learned {
		eyes = openLearnedEyes(ctx)
		if !eyes.working() {
			fmt.Fprintf(os.Stderr, "director: --learned asked for the detector "+
				"and it is not available: %s\n", eyes.why)
			return endedEarly(1, RunFailed, "--learned asked for the detector "+
				"and it is not available: "+eyes.why)
		}
		fmt.Printf("  the learned detector is running beside the loop.\n")
	}
	if *reading {
		reader = openReadingEyes(ctx)
		if !reader.working() {
			fmt.Fprintf(os.Stderr, "director: --reading asked for OCR "+
				"and it is not available: %s\n", reader.why)
			return endedEarly(1, RunFailed, "--reading asked for OCR and it is "+
				"not available: "+reader.why)
		}
		fmt.Printf("  the words on each look are read beside the loop.\n")
	}

	// WHAT THE LOOP CARRIES IS THE LOOP'S. `blind`, `blindWhy`, the last reading and its
	// stamps all moved into `looker` with the body that uses them; what is read back here
	// for the report comes off it. See observationloop.go.
	opt := structure.DefaultOptions()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)

	// ── AND THE DEADLINE IS IN THE CONTEXT, so everything that waits inherits it ──
	//
	// # What checking it at the top of a look was not enough for
	//
	// `looker.next` refuses to START another look past the deadline, which stops the loop
	// — and stops nothing that is already waiting. The press-authorisation prompt is the
	// case: it opens before expiry, the desk waits up to `--answer` for a key, and a
	// person pressing F7 four seconds into a `--seconds 3` run was given a FRESH
	// authorisation. The runner's boundary checks ask whether the world still holds; none
	// of them asks what time it is. So the click could still be sent, after the run's
	// advertised limit, with every other guard satisfied.
	//
	// Carrying it in the context is the whole correction, because the context is already
	// what everything that waits is given: the consent desk selects on `ctx.Done()` and
	// WITHDRAWS its question (`consentDesk.ask`), the capture takes it, the coordinator
	// takes it per observation, and the verification windows close with it. One deadline,
	// inherited by all of them, rather than a clock each of them would have to remember
	// to consult.
	//
	// Shadowing `ctx` on purpose: every use below this line is in the run, and a second
	// name would be a second thing to pass correctly at each call site.
	//
	// # And expiry is NOT a cancellation
	//
	// `context.WithDeadline` says which happened — `DeadlineExceeded` against `Canceled` —
	// and the two mean different things to a person. Somebody pressed stop; or the run
	// took longer than they allowed it. See `endedBecause` below, which keeps them apart
	// all the way into the record.
	ctx, pastDeadline := context.WithDeadline(ctx, deadline)
	defer pastDeadline()

	if session0 != nil {
		// THE SESSION'S CLOCK, started with the run. Set here and not where the
		// grant was built, because --delay is time nobody may act in and a session
		// that counted it spent its whole allowance waiting to begin.
		session0.until = deadline
	}
	lost := ""
	// act is the play's account when this run performed one, and nil when it walked an
	// artifact. What the run is judged by, below.
	var act *demonstrated.Account

	// ONE LOOK PER ITERATION, BUILT IN ONE PLACE. See observationloop.go, which is this
	// loop's body with the state it carried turned into fields — moved there verbatim
	// when a second caller arrived, because a second place that builds a Sighting would
	// be two copies of ordering that is several ADRs deep.
	// THE WALL CLOCK GOES WITH THE LOOKS, so both drivers are bound by `--seconds` and
	// not only the one that happened to check it. The closure reads `deadline`, which the
	// cursor loop also moves forward to fall out early with its report intact.
	looks := newLooker(tracker, target, shots, windows, obs, opt, eyes, reader,
		probe, repairs, ground, art, session, func() (bool, string) {
			if time.Now().Before(deadline) {
				return true, ""
			}
			return false, fmt.Sprintf("the run reached its time limit of %ds "+
				"(--seconds)", *seconds)
		})

	// ── AND WHERE THE ORDER COMES FROM, WHICH IS THE ONE THING --play CHANGES ──
	//
	// A workflow's order is the artifact's, and the cursor below walks it. A demonstrated
	// ACT's order is its own sentences', so the play drives and the coordinator is told
	// which interaction each sentence means. Everything else above this line and below the
	// loop is shared: the window, the scope gate, the screens, the desk, the probes, the
	// stop key, the report and the reference.
	//
	// `betweenLooks` is what the cursor loop does after each attempt, handed to the play
	// driver so the step-agreement question is asked in the same place rather than twice.
	betweenLooks := func(c context.Context, a workflowrun.Attempt) (bool, string) {
		reportAttempt(a)
		// WHAT THE COORDINATOR JUST DECIDED, SAID BY IT. See the function.
		sayWhatTheCoordinatorDecided(say, scenes, a)
		if a.Stage == workflowstore.StepUnconfirmed {
			if *explore {
				if err := session.ConfirmStep(a.Step,
					"the person who authorised this exploration"); err != nil {
					return false, err.Error()
				}
			} else if byGrant != nil {
				// AGREED WHEN THE ACT WAS COMPOSED, AND RECORDED AS THAT.
				//
				// The agreement question asked whether a step belongs in
				// this run at all. For an Act that is what composing it
				// answered: these interactions, in this order, named by the
				// person who then pressed Play. So the session records the
				// confirmation with the grant as its authority instead of
				// putting the question a second time — recorded and not
				// skipped, because the session's account of why each step
				// was performed is what the report and the record read.
				if !byGrant.agreed(a.Step, session) {
					return false, "the step confirmation could not be " +
						"recorded for step " + fmt.Sprint(a.Step)
				}
			} else if !agreeToStep(c, keys, ask, os.Stdout, art, a.Step, session) {
				return false, "nobody agreed to step " + fmt.Sprint(a.Step)
			}
		}
		<-ticker.C
		return true, ""
	}
	if plan != nil {
		acc, perr := performDemonstratedPlay(ctx, runID, *plan,
			// THE LOOP'S CONTEXT TRAVELS WITH THE LOOK, so a cancelled run
			// stops capturing rather than finishing its look budget.
			demonstrated.NewSupervised(run, session,
				func() (workflowrun.Sighting, bool) { return looks.next(ctx) },
				*patience*len(plan.Steps)+looksPerPlay).
				WithWatch(betweenLooks).WithRunContext(ctx),
			scenes, os.Stdout)
		if perr != nil {
			// THE RUNTIME's error: a play that could not be compiled or run at
			// all. How the ROUTINE went is the account, printed below.
			fmt.Fprintf(os.Stderr, "director: %v\n", perr)
		}
		// AND THE ACCOUNT IS WHAT THE RUN IS JUDGED BY, not only printed from.
		//
		// It reaches the record, the reference and the exit code below — see
		// outcomeOfAct. Printing it and then classifying from the artifact cursor was
		// the defect: an Act that is a subset of a longer demonstration, or its
		// interactions in another order, verified everything it asked for and was
		// recorded REFUSED because the cursor never reached the artifact's end.
		act = acc
		// AND WHY LOOKING STOPPED, when it was the clock or the window rather than a
		// sentence. `Cut` is how a run that did not reach every sentence says so, and
		// it is what puts the time limit into the account's own account of itself.
		if looks.lost != "" {
			acc.Cut(looks.lost, false)
			if lost == "" {
				lost = looks.lost
			}
		}
		fmt.Printf("\n  %s\n", acc.Short())
		for _, k := range acc.Asked() {
			fmt.Printf("  %-28s %s\n", k.Name, demonstrated.Sentence(k.Verdict,
				k.Because))
		}
		deadline = time.Now() // the report below is written from the same runner
	}
	for plan == nil && time.Now().Before(deadline) {
		at, more := looks.next(ctx)
		if !more {
			break
		}
		a := run.Advance(ctx, session, at)
		// WHAT HAPPENS BETWEEN LOOKS, in the one place both drivers use. The
		// step-agreement question lives in there: agreeing that a step belongs in this
		// run and permitting a press are two different sentences, and a build that put
		// them behind one prompt would let a person answer one and be taken to have
		// answered the other.
		if carryOn, why := betweenLooks(ctx, a); !carryOn {
			lost = why
			break
		}
		switch a.Outcome {
		case workflowrun.OutcomeComplete, workflowrun.OutcomeRefused,
			workflowrun.OutcomeFailed, workflowrun.OutcomeStopped:
			deadline = time.Now() // fall out of the loop with the report intact
		}
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
	// AND WHY LOOKING STOPPED, when it was the loop that stopped it. `looks.lost` is the
	// window going away; `lost` is a refusal this function reached. The first does not
	// overwrite the second: a run that stopped because nobody agreed to a step says that,
	// even if the window closed afterwards.
	if lost == "" {
		lost = looks.lost
	}
	// WHICH OF THE TWO ENDED IT, computed HERE rather than at the record below, because
	// `printRun` is what shows a person the reason and it runs first. The first version
	// of this worked it out after the printout: the record said the clock, and the run
	// itself said nothing about why it stopped.
	expired, stopped := endedBecause(ctx, stoppedByKey.Load())
	if expired && lost == "" {
		lost = fmt.Sprintf("the run reached its time limit of %ds (--seconds)", *seconds)
	}
	if looks.blind > 0 {
		fmt.Printf("\n  %d of %d capture(s) returned nothing. The first said: %s\n",
			looks.blind, rep.Looks, looks.blindWhy)
	}
	printRun(rep, art, book, *live, lost, eyes, reader, obs.Stats(), obs)

	// ── AND THE RECORD OF THIS ATTEMPT, by the reference printed at the start ──
	//
	// Written last, because it carries the outcome, and reported separately because
	// losing the record does not lose the run.
	//
	// WHICH COMPLETION IS BEING ASKED ABOUT MATTERS. `outcomeOfRun` takes "did it finish"
	// from the artifact cursor, which is right for a workflow and wrong for an ACT: an Act
	// may legitimately be a subset of a longer demonstration, or its interactions in
	// another order, and such a run verified everything it asked for while the cursor
	// never reached the artifact's end. It was recorded as REFUSED. See outcomeOfAct,
	// which takes completion from the play's own account and leaves every per-action count
	// exactly where it was.
	// WHICH OF THE TWO ENDED IT, because they are not the same thing to a person.
	//
	// Somebody pressed stop, or the run took longer than they allowed it. Reporting expiry
	// as a cancellation would say the first about the second — and `--seconds` passing is
	// not somebody declining, it is Marco running out of the time it was given. The
	// per-action counts then classify it honestly: an action that went out with nothing
	// answering is `unconfirmed`, and a run that pressed nothing is `refused`.
	out := outcomeOfRun(rep, stopped)
	because := rep.Progress.Summary()
	if act != nil {
		out = outcomeOfAct(act, stopped)
		because = act.Short()
	}
	if p, err := keep(out, because, rep); err != nil {
		fmt.Fprintf(os.Stderr,
			"director: the run record could not be written: %v\n", err)
		fmt.Printf("\n%s\n", citeRun(runID, ""))
	} else {
		fmt.Printf("\n%s\n", runref.Ended(runID, p, out))
	}
	// ── AND THE FINAL EVENT, WHICH IS THE SAME ANSWER THE RECORD GOT ──
	//
	// The same `out` and the same `because` that were just written down, from the same
	// two variables. Computing it again here would be a second classifier, and two
	// accounts of one run that could disagree is worse than either on its own.
	//
	// It is the last thing said, and it is what makes an INCOMPLETE run recognisable: a
	// front end that sees a child exit without this knows the run cannot be spoken for,
	// which is a different thing from a run that failed.
	say.Say(runstate.Event{Phase: runstate.Ended, Final: true,
		Outcome: out, Reason: because})
	if desk != nil && keys != nil {
		stray, drained := desk.counts()
		fmt.Printf("\n  %-42s %d of %d\n", "screens named by key",
			keys.placed, keys.screens)
		fmt.Printf("  %-42s %d of %d\n", "steps agreed to by key",
			keys.agreed, keys.proposed)
		fmt.Printf("  %-42s %d of %d\n", "presses permitted by key",
			keys.granted, keys.asked)
		fmt.Printf("  %-42s %d before a question, %d with none open\n",
			"key presses that answered nothing", drained, stray)
	}
	// ── AND WHAT A GRANTED RUN DID WITH THE AUTHORISATION IT WAS GIVEN ──
	//
	// The same four numbers' worth of account, for the contract that replaced the keys.
	// `refused` is the one to read: it is presses this run proposed and the authorisation
	// did not cover, which should be zero on every run and names what was attempted when
	// it is not.
	if byGrant != nil {
		fmt.Printf("\n  %-42s %s\n", "performed under", byGrant.grant.describe())
		fmt.Printf("  %-42s %d\n", "presses the authorisation permitted",
			byGrant.granted)
		fmt.Printf("  %-42s %d\n", "steps it covered", byGrant.proposed)
		fmt.Printf("  %-42s %d\n", "presses REFUSED as unauthorised",
			byGrant.refused)
		for _, what := range byGrant.outside {
			fmt.Printf("    %s was not authorised\n", what)
		}
		if desk != nil {
			stray, drained := desk.counts()
			fmt.Printf("  %-42s %d before a question, %d with none open\n",
				"key presses that answered nothing", drained, stray)
		}
	}
	// AND THE EXIT SAYS THE SAME THING THE RECORD DOES.
	//
	// For a workflow that is the cursor reaching the end. For an Act it is the account —
	// the same question asked of the thing that knows: every sentence the play has,
	// verified. A subset Act that did exactly what it said would otherwise exit 1 while
	// its own report said every interaction verified.
	if act != nil {
		if out == RunVerified {
			return 0
		}
		return 1
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
	// Application is the executable image the window layer reports for the window
	// acquired NOW, which is what a scope is compared against.
	//
	// Carried on the world rather than closed over, so the check is a case in
	// `worldMoved` — a pure function a test can drive without a desktop. The race it
	// exists for cannot be reproduced any other way: Marco resolves Solasta, the
	// person alt-tabs, and the press is sent to whatever took the foreground.
	Application string
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
	// Bounds is the window's rectangle NOW, which geometry replay compares against the
	// window the demonstration was made in.
	Bounds directorapi.Rect
	// Layout is the window the demonstration was made in, and NIL means this run is not
	// in geometry replay mode.
	//
	// Carried on the world rather than passed beside it so that `worldMoved` stays one
	// pure function a test can drive without a desktop — the same reason `Application`
	// is here. A nil here is the ordinary vision path, unchanged in every term.
	Layout *workflowstore.WindowGeometry
	// Fresh is what the look taken at this instant said about ITSELF.
	//
	// ── WHY THE FOUR TERMS ABOVE ARE NOT ENOUGH ──
	//
	// Every one of them is read off the observer's STANDING claim, and the stabiliser
	// deliberately keeps that claim still while a changed arrangement settles: a look
	// that differs from the standing screen is counted `Unsettled` and the OLD screen is
	// returned, for VisualSettleLooks looks. That damping is correct and is not being
	// changed — ADR-170 measured 150 raw adjacent-frame changes it suppresses.
	//
	// But it means a changed interface answers "unchanged" to all four terms for as long
	// as it is settling, and a person's authorisation arrives inside exactly that window.
	// So the fresh look is asked what the stabiliser made of IT, rather than what the
	// stabiliser is still saying about the screen before it. An unsettled or unreadable
	// look is not evidence that nothing moved; it is the absence of evidence that nothing
	// did, and this check needs the positive kind.
	Fresh freshEvidence
}

// freshEvidence is what one look taken at the authorisation boundary says about itself.
//
// Reported by `freshLook` from the stabiliser's own counters, before and after the look. No new
// threshold and no second similarity measure: the stabiliser has already decided what this look
// was, and the only thing missing was anybody asking it.
type freshEvidence struct {
	// Took says a look was taken at all. False means the boundary had no fresh evidence —
	// which is a refusal, not a pass.
	Took bool
	// Agreed says the stabiliser counted this look as a look at the standing screen.
	//
	// Derived from VisualStability.Settled moving. Every other counter moving means
	// something else happened to this look: Unsettled (it differed and has not become a
	// screen), Warming (there is no screen yet), Unobserved (nothing was found in the
	// pixels), Promotions (it was a DIFFERENT screen, which the epoch term also catches),
	// Interrupted (the thing being looked at changed).
	Agreed bool
	// Counted is which counter moved, for the sentence.
	Counted string
	// Target says the planned point still sits inside something this look proposed.
	//
	// The second half of "current target evidence", and a different question from whether
	// the screen is the same one: an interface can keep its overall arrangement, settle
	// clean, and have nothing under the approved coordinate. A click there lands on
	// whatever is behind it.
	Target bool
	// Needed says words are load-bearing for the screen this point was approved on.
	//
	// True when there is a text source running AND that screen has a vocabulary of its
	// own — which is the state the stabiliser's text rule needs on the screen's side
	// before it can refuse a merge at all. Without a reader, or on a screen nobody has
	// read, the grounding policy is and always was pixels, and demanding words would
	// refuse every press for evidence this run was never going to have.
	Needed bool
	// Said says the stabiliser applied a reading of THIS look's own pixels to it.
	//
	// ── WHY AGREEING GEOMETRY IS NOT ENOUGH WHEN A SCREEN HAS WORDS ──
	//
	// Two panels of one shape agree by arrangement: ADR-194 measured Rocket League's
	// Training page and its Free Play submenu at 0.721 against a threshold of 0.55, and
	// only the words kept them apart. So a boundary look that settles onto the standing
	// screen by arrangement ALONE has established nothing about a same-shape change, and
	// on a screen whose identity words have been separating, that is the one change left
	// for this check to catch.
	//
	// It needs every half, and each one was a way through. The reading has to come back
	// for THIS capture — correlated by `readingFrame.look` — it has to say enough to be a
	// claim, and the stabiliser has to have counted a reading on this look.
	//
	// The last without the first is the original defect: the boundary offered its frame
	// and drained in the same breath, so what it put on its look was a reading of the
	// frame BEFORE it, admissible because the epoch had not moved, agreeing with the
	// screen the plan was made on. The last without the second is the same mistake in
	// miniature: OCR answering a frame and reading nothing on it moves `TextRead` too,
	// and a successful reading of nothing cannot contradict anything. And the first two
	// without the last would mean words that never reached the text rule at all.
	Said bool
	// Epoch and Screen are the observer's standing claim AFTER this look.
	//
	// ── CARRIED HERE SO THE ORDER CANNOT BE GOT WRONG ──
	//
	// The look is what MOVES the standing claim, so reading the claim before taking
	// the look compares the plan against the state the plan was made in — which is
	// the original defect, restated. `stillThere` used to read them itself, and a
	// mutation that swapped the two statements survived every test in the package:
	// two source-pinned strings pin two statements and say nothing about which came
	// first.
	//
	// So there is exactly one place either value is read, and it is after
	// `obs.Observe`. A caller cannot order them wrongly because a caller does not
	// order them at all.
	Epoch  uint64
	Screen observe.VisualScreenID
}

// worldMoved reports whether the world is still the one an action was planned against.
//
// Eight refusals, each with its own sentence, in two groups.
//
// The first four are read off the observer's STANDING claim, and they exist because a person
// was asked something: a prompt is an unbounded pause in the middle of the one sequence that
// must not have one, so by the time the answer arrives the look that found the control may
// describe a screen that has gone, and the point is then a coordinate on a screen that is no
// longer standing.
//
// The last four are read off a look taken HERE, and they exist because the first four cannot
// see a change that is still settling. See worldNow.Fresh.
func worldMoved(app string, scope workflowstore.Scope, now worldNow,
	planned workflowrun.Sighting) (bool, string) {

	// ── GEOMETRY REPLAY IS ITS OWN BOUNDARY, WITH ITS OWN TERMS ──
	//
	// Not the vision boundary with some terms waived. The four terms it keeps are about
	// WHERE a click will land — the window is alive, it belongs to the application this
	// routine is scoped to, it is the same window it was planned against, and it leads
	// the desktop — plus the one this mode exists for: it is the size the demonstration
	// was made in.
	//
	// The terms it does NOT have are the ones that require vision: a screen hypothesis
	// standing, an observation generation unmoved, a look taken at the instant of the
	// press, a reading of that look's own words, and a proposal under the point. Those
	// are prerequisites of recognising a control, and this mode exists precisely for the
	// menus where recognition has no answer — a boundary demanding them would refuse
	// every press this mode was granted for, which is what it did.
	//
	// The vision path below is untouched and is what every run without the mode takes.
	if now.Layout != nil {
		switch {
		case !now.Alive:
			return false, fmt.Sprintf("%s is no longer there (%s)", app, now.State)
		case !allowsNow(scope, now.Application):
			_, why := scope.Allows(now.Application)
			return false, "the application changed after the plan was made: " + why
		case now.Generation != planned.Generation:
			return false, fmt.Sprintf("the window was replaced since this was "+
				"planned (generation %d, was %d)",
				now.Generation, planned.Generation)
		case now.Front != observe.FrontTarget:
			return false, fmt.Sprintf("%s does not lead the desktop (%s), and a "+
				"click lands in whatever is in front", app, now.Front)
		}
		// ── AND THE WINDOW IS STILL THE ONE THIS WAS DEMONSTRATED IN ──
		//
		// Asked HERE and not only when the step was planned, for the reason every
		// other term here is asked twice: a person authorising a press is an
		// unbounded pause in the middle of the one sequence that must not have
		// one, and a window can be resized inside it. The point is normalised, so
		// it would still produce a coordinate — somewhere plausible, in a layout
		// nobody demonstrated.
		same, why := now.Layout.SameAs(workflowstore.WindowGeometry{
			Width: now.Bounds.Width, Height: now.Bounds.Height,
			Scale: workflowstore.ScaleUnknown,
		})
		if !same {
			return false, "the window changed after the plan was made: " + why
		}
		return true, ""
	}

	switch {
	case !now.Alive:
		return false, fmt.Sprintf("%s is no longer there (%s)", app, now.State)
	case !allowsNow(scope, now.Application):
		// ── TIME OF CHECK IS NOT TIME OF USE ──
		//
		// The gate at startup compared the artifact against the window
		// acquired THEN. This is a different moment, and in between the
		// person may have alt-tabbed, the game may have exited and another
		// taken the handle, or `Acquire` may have resolved a different
		// window of a different application.
		//
		// The owner asked for exactly this: a valid application check at
		// invocation must not become permanent authorisation. First among
		// the cases below, because it is the one that decides whether this
		// press may go to this application at all.
		_, why := scope.Allows(now.Application)
		return false, "the application changed after the plan was made: " + why
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
	case !now.Fresh.Took:
		// ── NO FRESH EVIDENCE IS NOT GOOD EVIDENCE ──
		//
		// First of the two fresh terms, because the other two cannot be read
		// without it. A boundary that could not look has nothing to say about
		// now, and the four terms above all answer from the observer's standing
		// claim, which is the claim the plan was made from.
		return false, "no look could be taken at the instant of the press, so " +
			"nothing was asked about the screen this point was approved on"
	case !now.Fresh.Agreed:
		// ── A SETTLING CHANGE IS STILL A CHANGE ──
		//
		// The stabiliser returns the standing screen for a look that differs from
		// it, for as long as the difference is settling. That is correct damping
		// and it is untouched; what is wrong is reading it as "nothing moved". At
		// ten looks a second a settle takes three looks, and a person's F7 lands
		// well inside that, so both confirmations before a click could see the old
		// screen and the old epoch while the interface in front of the pointer had
		// already changed.
		//
		// The stabiliser's own counter says which: this look was Unsettled,
		// Warming, Unobserved, or of a window that had been replaced.
		return false, fmt.Sprintf("the look taken at the instant of the press did "+
			"not agree with the screen this point was approved on (%s), so the "+
			"screen may be changing underneath it and a change that is still "+
			"settling is still a change", now.Fresh.Counted)
	case now.Fresh.Needed && !now.Fresh.Said:
		// ── AGREEING BY ARRANGEMENT ALONE, ON A SCREEN WORDS DECIDE ──
		//
		// The look agreed, and on a screen that has a vocabulary that agreement
		// is only as good as the arrangement threshold. Two panels of one shape
		// clear it — 0.721 against 0.55, measured — and the words are what
		// separate them, so a look nobody read is not evidence that this is still
		// that panel.
		//
		// This is the term the first fix did not have. The boundary offered its
		// capture to OCR and drained in the same breath, so the words on its look
		// described the frame before it, and an unchanged epoch made them
		// admissible. The reading is now asked for BY FRAME and waited for inside
		// a budget; a budget that expires is this refusal, which is the direction
		// to fail in. So is a reading that came back having read nothing: a
		// request completing is not evidence arriving.
		return false, "the look taken at the instant of the press agreed with " +
			"the screen this point was approved on by its arrangement alone, " +
			"and that screen is one words have been separating: no reading of " +
			"this look's own pixels arrived in time, or the one that did said " +
			"too little to be a claim, and an arrangement two panels of one " +
			"shape share is not evidence that this is the same one"
	case !now.Fresh.Target:
		// ── AND THE CONTROL ITSELF, IN THIS LOOK'S OWN PROPOSALS ──
		//
		// An interface can keep its arrangement closely enough to settle clean and
		// still have nothing under the approved coordinate — a panel swapped for
		// one of the same shape, a list that scrolled, a row that went away. Every
		// term above would agree, and the click would land on whatever is now
		// behind that point.
		//
		// Asked of the fresh look's proposals, which is the same evidence a
		// control is found by in the first place; nothing new is measured here.
		return false, "nothing was proposed under the approved point in the look " +
			"taken at the instant of the press, so the control it was found on " +
			"is no longer there and a click would land on whatever is"
	}
	return true, ""
}

// stillThere gathers the world and hands it to worldMoved.
//
// The one line in this file a test cannot supply for itself, for the same reason `windowLeads` is
// a package variable: a real answer needs a live window and a real desktop. Everything it
// decides is in worldMoved, which is tested.
// `layout` non-nil puts this boundary in GEOMETRY REPLAY MODE: it is the window the
// demonstration was made in, and the boundary then asks its own questions rather than the vision
// path's. No look is taken, because nothing it would produce is consulted.
func stillThere(tracker *windowref.Tracker,
	windows *winprovider.Provider, app string, scope workflowstore.Scope,
	layout *workflowstore.WindowGeometry,
	relook func(context.Context, windowref.Validation, observe.PointerAt) freshEvidence) func(
	context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {

	return func(ctx context.Context, planned workflowrun.Sighting,
		point observe.PointerAt) (bool, string) {

		v := tracker.Acquire(ctx, app)
		now := worldNow{Alive: v.State.OK(), State: string(v.State),
			Generation: v.Ref.Generation, Bounds: v.Ref.Bounds, Layout: layout}
		if !now.Alive {
			// NOTHING TO LOOK AT. `worldMoved` refuses on this first, and
			// capturing a window the tracker has given up on would be a
			// screenshot of whatever inherited the handle.
			return worldMoved(app, scope, now, planned)
		}
		now.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref)
		// AND WHAT THE WINDOW LAYER SAYS THIS APPLICATION IS, NOW.
		//
		// The scope is compared against this and never against the argument: the
		// argument is what somebody asked for and cannot have changed, while this
		// is what `Acquire` just resolved. The comparison itself is a case in
		// `worldMoved`, which is a pure function and therefore testable without a
		// desktop — see TestTheApplicationIsRecheckedAtTheInstantOfThePress.
		now.Application = v.Ref.Application

		// ── A FRESH LOOK, BECAUSE THE CACHED ONE IS THE PLAN'S OWN ──
		//
		// `obs.Epoch()` and `obs.Screen()` are whatever the observer last saw, and
		// the observation loop is BLOCKED inside `Advance` while somebody is asked
		// to authorise — so nothing has looked since the plan was made, and asking
		// the observer what is standing returns the same answer that produced the
		// plan. The check compared the plan against itself.
		//
		// What that misses is the case it most needs to catch: the application
		// changes a panel inside the same window, with the same geometry, still in
		// front. Window validity and frontness are refreshed and all three agree,
		// while the point that was approved now sits on a different interface.
		//
		// So a look is TAKEN here rather than remembered. It goes through the same
		// observer as every other look, which is deliberate: the relation that
		// decides what is standing is unchanged, and what changes is only that its
		// inputs describe now. Inventing a similarity threshold for this one
		// moment would be a second opinion about screen identity, and two of those
		// eventually disagree.
		//
		// AND IT REPORTS WHAT THE STABILISER MADE OF IT, which taking the look was
		// not enough on its own to establish. The stabiliser damps a change for
		// VisualSettleLooks looks and keeps answering with the STANDING screen
		// while it settles, so a check reading only `obs.Screen()` and
		// `obs.Epoch()` saw "unchanged" throughout exactly the window a person's
		// authorisation arrives in. See worldNow.Fresh and freshLook.
		//
		// `relook` nil leaves Fresh zero, which `worldMoved` refuses — a boundary
		// with no fresh evidence is not a boundary that passed.
		//
		// AND NOT TAKEN AT ALL IN GEOMETRY REPLAY MODE. Nothing that look
		// produces is consulted there — no screen, no observation generation, no
		// reading of its words, no proposal under the point — so taking one would
		// be a capture, a segmentation and sometimes an OCR request on the one
		// path where none of them has an answer, inside the one moment that must
		// not have an unbounded pause in it.
		if relook != nil && layout == nil {
			// AND THE STANDING CLAIM COMES BACK WITH IT. The look is what
			// moves the claim, so nothing here may read `obs.Epoch()` or
			// `obs.Screen()` — see freshEvidence.Epoch.
			now.Fresh = relook(ctx, v, point)
			now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
		}
		return worldMoved(app, scope, now, planned)
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
	// arrangement is what THIS LOOK looks like — `observe.ArrangeLook` of its own
	// regions — pushed by the loop because this cannot look for itself.
	//
	// Not the standing screen's stored signature, which is what it used to be: that
	// signature is fixed when the screen is minted, so a page that merged onto an older
	// identity is compared as the older page. See the loop, and Experiment-048.
	arrangement observe.VisualArrangement
	// read is the reading admitted for this observation, or unread text when there is
	// none. The loop supplies it already filtered by `observe.Current`.
	read observe.ScreenText
	// readWhy is why `read` is empty, in the loop's words, because the loop is the only
	// thing that can tell "OCR has not answered yet" from "the screen keeps changing
	// under the reader" from "these pixels were not the application's". It goes into the
	// one sentence that refuses for want of a reading. See workflowstore.Standing.
	readWhy string
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
	// firstSeen is the screen the last observation carried, so a claim is spent on one
	// that has stood for two rather than on a fold still settling.
	firstSeen observe.VisualScreenID
	// silent says nobody can be asked IN WORDS, because typing an answer would take the
	// foreground. The ranking is still printed: somebody who cannot answer can still read.
	silent bool
	// identify is the authority a granted run performs under, and the switch that makes
	// this grounder identify a screen instead of asking about one.
	//
	// Non-empty only under a grant — one press of Play over named interactions — where
	// there is nobody mid-run to put the question to. The evidence replacing them is in
	// `workflowstore.Standing`: the shape as a gate, the words as the discriminator, and
	// no second saved screen allowed to fit. This string is what the binding records as
	// the authority, so a run's own account says a press was made under a grant rather
	// than that somebody answered something.
	identify string
	// events is the structured account, for the one thing this grounder has to say to a
	// front end: that the run is PAUSED, and why. Nil when nobody asked for one.
	events *runstate.Writer
	// paused is the last pause reason said, so a reason is emitted when it CHANGES
	// rather than ten times a second. A snapshot stream tolerates repetition and a
	// person reading the file afterwards does not.
	paused string
	// byKey is how a silent run asks anyway: one candidate, one function key.
	//
	// ── WHY THIS EXISTS AT ALL ──
	//
	// `silent` used to mean "nobody can be asked", and the run then refused every first
	// observation with *name the screen with --from and start again*. That was true of a
	// person at a terminal in another window, who can read the ranking and start again
	// with a flag. It is not true of a person at a WIDGET: there is no terminal to name a
	// screen in, so the one path to the half of the product they made with their own hands
	// dead-ended on a question nobody could answer.
	//
	// The keys that authorise every action can answer this one too. Nil where nobody holds
	// them — an exploring session, authorised at launch precisely so that it does not stop
	// to ask, still has `--from` and still refuses without it. See
	// hotkeyGranter.confirmScreen for why one yes/no is enough and why it is not a silent
	// choice.
	byKey func(ctx context.Context, label string, score float64) (navsource.Consent, bool)
}

func (p *promptGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	if _, ok := s.Established(p.standing, at.Epoch, at.Generation); ok {
		// ALREADY SETTLED IN THIS OBSERVATION — AND THAT IS NOT THE WHOLE QUESTION.
		//
		// This used to return here, and returning here is the first of the two gates
		// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured.
		// A binding is live for exactly one observation generation, which moves only
		// when the GLOBAL observer records a screen change. Two pages inside the
		// arrangement measure's own resolution merge, no change is recorded, the
		// source stays live — and the step's destination is never asked about at
		// all. In the live run it was not asked for sixty observations, while Marco
		// stood on the page it was waiting for.
		//
		// So the destination is tried from this look's own evidence before the
		// source's standing binding is allowed to end the question. It is still an
		// error-free path: nothing established here is required, and a refusal is the
		// ordinary case on every look before the one that arrives.
		p.reach(s, at)
		return nil
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
	//
	// AND NOT ON A SCREEN THAT HAS NOT STOOD YET. The claim is about where the person
	// is, and a fold that lasts four looks and is never read is not where anybody is —
	// it is the observer still settling. Spending the one answer there leaves the screen
	// they ARE on unnameable for the rest of the run.
	//
	// Measured live on the owner's Ark session:
	//
	//	visual_1    4 look(s)   nobody read it
	//	visual_2  136 look(s)   ...
	//
	// `--from` was spent on visual_1 and the run then refused for 200 observations.
	//
	// So the claim waits for the same screen twice. That is NOT making it a standing
	// answer — it is still one answer about one screen, spent once and never offered
	// again. It only stops it being spent on a screen nobody was looking at.
	if p.claim != "" && !p.spent && at.Screen != p.firstSeen {
		p.firstSeen = at.Screen
		return fmt.Errorf("%s has only just appeared, so the --from answer is not "+
			"spent on it yet", at.Screen)
	}
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

	// ── OR NOBODY IS ASKED, BECAUSE THE AUTHORISATION WAS GIVEN IN ADVANCE ──
	//
	// A granted run has no question to put: the person pressed Play and went to their
	// game. So the screen is identified from its own evidence or the run pauses saying
	// why, and this is checked before the asking paths below rather than after, because
	// asking is exactly what this contract replaces.
	if p.identify != "" {
		return p.byEvidence(s, at)
	}

	// ── OTHERWISE SOMEBODY IS ASKED, WITH THE CANDIDATES RANKED AND THE RANKING
	//    LABELLED AS WHAT IT IS ──
	fmt.Fprintf(p.out, "\n  Which saved screen are you looking at?\n")
	for _, c := range p.ranked() {
		fmt.Fprintf(p.out, "    %-10s arrangement agrees %.3f\n", c.label, c.score)
	}
	fmt.Fprintf(p.out, "    (a ranking cannot say whether the right answer is in the "+
		"list; that is what you are adding)\n")
	// ── ONE CANDIDATE, ONE KEY, WHERE THERE IS NO TERMINAL TO ANSWER IN ──
	//
	// The ranking proposes and the person decides, which is the same division
	// [[ADR-186-a-person-can-say-where-they-are]] settled for the typed path — the
	// difference is only which device the answer arrives on. The top candidate is named
	// with the score it rests on, because a person about to let Marco act on a screen
	// should be told how sure the arrangement is.
	//
	// A refusal declines this screen exactly as `none` does below: nothing is pressed,
	// the question is not asked again in this observation, and the run goes on looking.
	if p.silent && p.byKey != nil {
		if best := p.ranked(); len(best) > 0 && best[0].label != "" {
			c, answered := p.byKey(ctx, best[0].label, best[0].score)
			switch {
			case !answered:
				p.declined[at.Screen], p.standing = at.Epoch, ""
				return fmt.Errorf("nobody said whether %s is %s, so the "+
					"screen it is standing on is not established",
					at.Screen, best[0].label)
			case c == navsource.ConsentApprove:
				if err := s.Confirm(best[0].label, at.Screen, at.Epoch,
					at.Generation, "the person at the keyboard ("+
						navsource.ConsentKeys[navsource.ConsentApprove]+
						")", time.Now()); err != nil {
					return err
				}
				p.standing = best[0].label
				fmt.Fprintf(p.out, "  %s established by key, in observation "+
					"%d/%d\n", best[0].label, at.Epoch, at.Generation)
				return nil
			case c == navsource.ConsentStop:
				// THE RUN IS ALREADY STOPPING. The desk's stop callback
				// reaches the runner itself; this says what the look made
				// of the question so the report is not silent about it.
				p.declined[at.Screen], p.standing = at.Epoch, ""
				return fmt.Errorf("the run was stopped while %s was being "+
					"named", at.Screen)
			}
			p.declined[at.Screen], p.standing = at.Epoch, ""
			return fmt.Errorf("%s is not %s, said the person at the keyboard, "+
				"so the screen it is standing on is not established",
				at.Screen, best[0].label)
		}
	}

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

// reach tries to establish a destination the workflow demonstrates, from this look's own
// evidence, where the global observer has recorded no change to follow.
//
// Returns nothing. A destination that cannot be established yet is the ordinary case — the
// walk is between screens, or the reading has not landed — and the step's own waiting report
// already says so in the caller's words. What this must not do is fail the establishment path:
// the source IS established, which is all `Establish` promised.
func (p *promptGrounder) reach(s *workflowstore.SessionBinding, at workflowrun.Sighting) {
	for _, to := range p.next(p.standing) {
		b, err := s.Reached(p.art, p.standing, to, at.Screen, p.arrangement, p.read,
			at.Epoch, at.Generation)
		if err != nil {
			continue
		}
		// THE WALK MOVES ON, so the next look asks about the step after this one.
		p.standing = b.Label
		fmt.Fprintf(p.out, "  %s reached: %s\n", to, b.Because)
		return
	}
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

// byEvidence identifies the screen in front of a granted run, or pauses the run saying why not.
//
// # Why every saved screen is tried rather than the one the Act starts on
//
// Because `Standing` already carries the term that makes trying them safe: it refuses if a
// second saved screen fits the same look and reading. So at most one of these calls can succeed,
// and the one that does is the screen this look IS — not the screen the Act hoped for. That
// matters for the step after the first: the walk asks about whatever screen is standing, and a
// grounder that only ever looked for the Act's starting screen would go blind the moment the
// first press changed it.
//
// # Why nothing is recorded in `declined`
//
// That map exists so a question is not put twice in one observation. There is no question here.
// The evidence is re-read on every look, which is what makes the pause RESOLVE by itself: the
// person navigates to the screen their Act starts on and the next look establishes it, with
// nobody pressing anything.
func (p *promptGrounder) byEvidence(s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	var first error
	for _, c := range p.ranked() {
		b, err := s.Standing(p.art, c.label, at.Screen, p.arrangement, p.read,
			p.readWhy, at.Epoch, at.Generation, p.identify, time.Now())
		if err == nil {
			p.standing = b.Label
			p.pause("")
			fmt.Fprintf(p.out, "  %s\n", b.Because)
			return nil
		}
		// THE BEST CANDIDATE'S REASON, because `ranked` is ordered by agreement and
		// the first one is the thing a person is most likely to be looking at. The
		// others are a list of screens they are not on, which is not the sentence
		// they need.
		if first == nil {
			first = err
		}
	}
	if first == nil {
		first = fmt.Errorf("%q has no saved screens with an arrangement, so there is "+
			"nothing to recognise", p.art.Name)
	}
	p.standing = ""
	p.pause(first.Error())
	return first
}

// pause says that the run is not proceeding and why, when the reason changes.
//
// Not a failure and not an ending: a front end renders this and keeps Stop available, and the
// run goes on looking. Emitting on change rather than per look keeps the stream readable — the
// grounder is consulted ten times a second and the answer is the same ten times.
func (p *promptGrounder) pause(why string) {
	if why == p.paused {
		return
	}
	p.paused = why
	if why == "" {
		return
	}
	p.events.Say(runstate.Event{Phase: runstate.Paused, Reason: why})
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
	// THE POINTER FIRST, because it qualifies the line under it: unchanged pixels beneath
	// a pointer that is somewhere else say nothing at all about the control.
	if a.PointerBecause != "" {
		fmt.Printf("    pointer: %s\n", a.PointerBecause)
	}
	if a.HoverBecause != "" {
		fmt.Printf("    hover: %s\n", a.HoverBecause)
	}
	// THE THREE CLAIMS, KEPT APART ON THREE LINES.
	//
	// An input was delivered; a destination was established, by evidence of a named
	// strength; and the action was independently verified. The third is always NO, and
	// printing the first two beside it is what stops a reader taking the first for the
	// third. See workflowstore.Arrival, whose `Verified` is a method precisely so nobody
	// can set it.
	if a.Arrival == nil || !a.Arrival.Established() {
		return
	}
	switch {
	case a.Arrival.Independent():
		fmt.Printf("    destination: established, and INDEPENDENT of the workflow — "+
			"somebody looked at it (%s)\n", a.Arrival.Ground)
	// AN INFERENCE HAS NO READING IN IT, and saying otherwise misnames the evidence.
	//
	// `Positive` and `Thin` are set only by `Reached`, so an inferred arrival used to
	// fall through to the default below and print "on a reading that only FAILED TO
	// CONTRADICT it" — when what established it was a screen change the observer
	// recorded and an arrangement that agreed, with no reading consulted at all. It
	// overstated nothing about strength and misnamed the KIND, in the one report whose
	// job is keeping those apart.
	case a.Arrival.Ground == workflowstore.GroundInferred:
		fmt.Printf("    destination: established as %q — the workflow demonstrates "+
			"this transition and the arrangement agreed. No reading was part "+
			"of that decision\n", a.Arrival.Ground)
	case a.Arrival.Thin:
		fmt.Printf("    destination: established as %q from this look, on a saved "+
			"vocabulary too thin to have refused it\n", a.Arrival.Ground)
	case a.Arrival.Positive:
		fmt.Printf("    destination: established as %q from this look, and a reading "+
			"IDENTIFIED it\n", a.Arrival.Ground)
	default:
		fmt.Printf("    destination: established as %q from this look, on a reading "+
			"that only FAILED TO CONTRADICT it\n", a.Arrival.Ground)
	}
	if ok, why := a.Arrival.Verified(); !ok {
		fmt.Printf("    action: NOT independently verified — %s\n", why)
	}
}

// printRun is the account a person reads afterwards.
//
// The five rows at the end are deliberately five sentences and not one. Collapsing any two of
// them is how a dry run, or a simulated boundary, turns into a success claim.
func printRun(rep workflowrun.Report, art workflowstore.Artifact, book *recordhost.Host,
	live bool, lost string, eyes *learnedEyes, reader *readingEyes,
	readings fastobserve.Stats, obs *fastobserve.Observer) {

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
	printScreensSeen(obs)
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
	case rep.VerifiedOK > 0:
		// THE SCREEN DID NOT CHANGE AND THE APPLICATION PLAINLY RESPONDED.
		//
		// This said "the application did not visibly respond" directly above a
		// verified outcome listing twenty-five words of the panel the press had
		// just opened. The two lines are about different things — one is the
		// global screen identity, the other is what appeared on it — and read
		// together the summary contradicted its own verdict. Measured on the
		// first human-demonstrated action this project verified, where somebody
		// skimming the summary would have concluded nothing had happened.
		fmt.Printf("  %-42s %s\n", "the screen changed after the action",
			"NO — and the outcome verified anyway, which is this whole case")
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
	// AND HOW STRONG THE REST WAS. A destination reached on this look's own evidence is
	// the weaker claim, and a reading that merely failed to contradict it is weaker
	// again — see workflowstore.Bound.Positive. Printed only when there were any, so a
	// run that followed real screen changes throughout says nothing about a path it
	// never took.
	if rep.Reached > 0 {
		fmt.Printf("  %-42s %d\n", "  of those, reached on the look's own evidence",
			rep.Reached)
		fmt.Printf("  %-42s %d of %d\n", "    with a reading that identified it",
			rep.Identified, rep.Reached)
		if rep.ThinWords > 0 {
			fmt.Printf("  %-42s %d of %d\n",
				"    against too few saved words to refuse",
				rep.ThinWords, rep.Reached)
		}
	}
	// ── WHAT EACH ACTION'S OWN EVIDENCE SAID ──
	//
	// The three verdicts, counted apart. An UNCONFIRMED outcome is not a failed action
	// and not a successful one — the question could not be put — and a report with one
	// `verified` number would hide which of the two a run rested on.
	//
	// Printed before the causal claim below, because they are different claims: this is
	// what the application did, and that is whether the press caused it.
	if n := len(rep.Verifications); n > 0 {
		fmt.Printf("  %-42s %d verified, %d contradicted, %d unconfirmed\n",
			"action outcomes checked", rep.VerifiedOK, rep.Contradicted,
			rep.Unconfirmed)
		for _, v := range rep.Verifications {
			fmt.Printf("      %s\n", v.Summary())
		}
		// AND THE SELF-TEST ON THE SCREEN THAT WAS ACTED ON. A screen that
		// produces an outcome with nothing done to it has outcomes that mean
		// nothing, and the verifier refuses on them rather than guessing.
		for _, v := range rep.Verifications {
			if v.Quiet > 0 {
				fmt.Printf("      screen held still in %d of %d window(s) "+
					"with no action in them\n", v.Quiet-v.Moved, v.Quiet)
			}
		}
	}
	// AND THE RATE THE READINGS ARRIVED AT, which is what revises the deadline.
	//
	// The verification deadline was chosen from an OFFLINE reading rate of about one look
	// in three, and the live rate is not that number: `readingEyes` drops a frame
	// whenever a reading is still running. Printed on every run that read anything, so
	// the next deadline comes from data rather than from a second guess.
	if rep.ReadingsPerSecond > 0 {
		fmt.Printf("  %-42s %d at %.2f/second\n", "readings admitted",
			rep.Readings, rep.ReadingsPerSecond)
	}
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

// printScreensSeen is what the observer believed, which no other line of this report says.
//
// # Why a run has to be able to answer this
//
// [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] reached the Training
// page with two correct presses and then spent sixty observations waiting for a destination,
// because the standing screen never changed. Every other number in the report was consistent
// with that AND with half a dozen different causes: an arrangement that never differed, a
// difference that never held for three looks, a vocabulary that had learned the next page's
// words. The run could not say which, and the recording it was replayed against reproduced
// none of them -- so the question could only be put by running the game again, which is the
// one thing a report exists to avoid.
//
// Three facts, and none of them new work: the screens in the order they were settled on, what
// each was READ to say, and the stabiliser's own counters. A screen standing through a
// navigation is a walk that does not move; a vocabulary that learned the next screen is words
// that do not belong to it.
func printScreensSeen(obs *fastobserve.Observer) { printScreensSeenTo(os.Stdout, obs) }

// printScreensSeenTo is the same account, written somewhere a test can read it back.
func printScreensSeenTo(w io.Writer, obs *fastobserve.Observer) {
	if obs == nil {
		return
	}
	seen := obs.Screens()
	if len(seen) == 0 {
		return
	}
	st := obs.Stability()
	fmt.Fprintf(w, "  %-42s %d screen(s) over %d look(s)\n",
		"the screens this run saw", len(seen), st.Looks)
	for _, sc := range seen {
		said := obs.SaidBy(sc.ID)
		words := "nobody read it"
		switch {
		case !said.Read():
		case len(said.Words) == 0:
			words = "read, and no word was there twice"
		default:
			words = strings.Join(said.List(), " ")
		}
		fmt.Fprintf(w, "      %-14s %3d look(s)   %s\n", sc.ID, sc.Looks, words)
	}
	// THE WALK, which the counts cannot reconstruct: two screens visited once each and
	// two visited five times alternately are the same two rows above.
	if walk := obs.Walk(); len(walk) > 0 {
		parts := make([]string, 0, len(walk))
		for _, id := range walk {
			parts = append(parts, string(id))
		}
		fmt.Fprintf(w, "      %-14s %s\n", "in this order",
			strings.Join(parts, " -> "))
	}
	// AND HOW MUCH OF THAT RESTED ON ONE READING. A refusal two readings agreed on and a
	// refusal one reading raised alone are different evidence, and the second is the one
	// whose cost somebody will want to see — see ADR-199.
	alone := ""
	if st.TextRefusedAlone > 0 {
		alone = fmt.Sprintf(" (%d on one reading)", st.TextRefusedAlone)
	}
	fmt.Fprintf(w, "      %-14s settled %d, unsettled %d, warming %d, text refused a "+
		"merge %d time(s)%s\n", "and the fold", st.Settled, st.Unsettled,
		st.Warming, st.TextRefusedMerge, alone)
}

// allowsNow is the scope check at the instant of a press.
//
// Separated so `worldMoved` reads as a list of ways the world can have moved. An UNSCOPED
// workflow answers false here as it does everywhere: a legacy routine is readable and is valid
// for no application until it is bound to one.
func allowsNow(scope workflowstore.Scope, application string) bool {
	ok, _ := scope.Allows(application)
	return ok
}

// workflowToPerform is the artifact this run will perform, from a name or from a file.
//
// # Why a name at all
//
// Because the product's two halves were joined by nothing. A person demonstrates a menu action
// and `demonstrate-visual` saves it in the demo store under the name they gave it, scoped to
// the application it watched, with the raw demonstration beside it. `correct`, `reacquire` and
// `what-i-did` all accept that name. This command — the only one that ACTS — accepted a file
// path, so running what you had just taught meant finding its JSON under the config directory.
//
// That is the whole of the integration gap between "Marco can record what a person
// demonstrates" and "Marco can execute and verify a pixel action".
//
// # Exactly one of the two
//
// Not a fallback and not a search. A name that is also a path, or a path treated as a name,
// turns a typo into a different routine running — and this one presses a mouse button. Both
// given is a refusal; neither is a refusal; and a name that is not saved says so and lists
// nothing, because guessing which saved routine somebody meant is how the wrong one runs.
func workflowToPerform(file, named string) (workflowstore.Artifact, int) {
	switch {
	case file != "" && named != "":
		fmt.Fprintln(os.Stderr, "director: --name and --workflow both given; one "+
			"names a saved demonstration and the other names a file, and\n"+
			"  running whichever was meant is not a guess this command makes")
		return workflowstore.Artifact{}, 2
	case named != "":
		store, err := demo.Open(configDir())
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return workflowstore.Artifact{}, 1
		}
		a, ok := store.Sequence(named)
		if !ok {
			// THE SAME TWO ANSWERS `reacquire` GIVES, because they send somebody
			// to different places: a routine that cannot be READ is a file to
			// look at, and a routine that was never saved is a demonstration to
			// give.
			if why := store.SequenceProblem(named); why != "" {
				fmt.Fprintf(os.Stderr, "director: %q cannot be read: %s\n",
					named, why)
				return workflowstore.Artifact{}, 1
			}
			// NAMING COMMANDS THAT EXIST. The first version of this said
			// `director demos`, which is not a command — a refusal that
			// sends somebody to a second refusal.
			fmt.Fprintf(os.Stderr, "director: nothing called %q has been saved.\n"+
				"  director sequences                   what you have shown it\n"+
				"  director demonstrate-visual %q ...   show it this one\n",
				named, named)
			return workflowstore.Artifact{}, 1
		}
		return a, 0
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return workflowstore.Artifact{}, 1
	}
	art, err := workflowstore.Unmarshal(raw)
	if err != nil {
		// A refusal from the artifact's own validation: an unknown version, a
		// clarification attached to nothing, a step naming a screen it does not carry.
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return workflowstore.Artifact{}, 1
	}
	return art, 0
}

// lookAfterWaiting reports whether the delay belongs BEFORE the application is discovered.
//
// Only when nobody named one. `--application rocketleague` is somebody saying which application
// they mean, and making them wait to be told what they already said would be a delay spent on
// nothing. Discovery is the case where the answer depends on what the person does next — switch
// to the game — and reading the foreground before they have is reading their terminal.
func lookAfterWaiting(app string, delay time.Duration) bool {
	return strings.TrimSpace(app) == "" && delay > 0
}

// countdown waits out a delay, and reports false when it was cancelled instead.
//
// # Why this is a function and not three lines
//
// Because the three lines were an unconditional `time.Sleep` with the cancellation context
// created three hundred lines below them. The one window in which a person is most likely to
// change their mind — they have just started a run and are looking at the application — was the
// one window nothing could interrupt, and Ctrl-C killed the process rather than stopping the
// run. A killed process says nothing about how far it got.
//
// Separated so the cancellation can be tested without a clock: what has to hold is that a
// cancelled countdown returns false and the caller then does nothing at all. Everything the run
// could press is built after this returns.
func countdown(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// startState is how a supervised run began.
type startState int

const (
	// startPressed: somebody said go.
	startPressed startState = iota
	// startTimedOut: nobody pressed anything and the delay elapsed, which is the
	// unattended path and is not a refusal.
	startTimedOut
	// startCancelled: the stop key, or Ctrl-C.
	startCancelled
)

// started waits for the person to say go, and reports how the wait ended.
//
// # Why a key and not a clock
//
//	Goal: make a supervised Marco trial possible without guessing when capture or execution
//	starts. […] An explicit Start interaction after the user has prepared the trial.
//
// `--delay` is a guess about when somebody will be ready, and every live trial tonight was
// either too short — the foreground was still the terminal when discovery ran — or long enough
// to be spent waiting. A key is the person saying they are ready, and it is heard while the
// application keeps the foreground, which is the same reason the consent desk uses keys.
//
// The delay stays as the CEILING, so an unattended run still starts and a forgotten terminal
// still stops by itself. Pressing stop here is a cancellation and not a refusal of a question:
// nothing has been asked yet, nothing acquired, and nothing could have been pressed.
func started(ctx context.Context, src *navsource.Source, within time.Duration) startState {
	if within <= 0 {
		within = time.Minute
	}
	fmt.Printf("  Prepare the application, then press %s to start.  %s or Ctrl-C "+
		"cancels.  (starts by itself in %s)\n",
		navsource.ConsentKeys[navsource.ConsentApprove],
		navsource.ConsentKeys[navsource.ConsentStop], within)
	t := time.NewTimer(within)
	defer t.Stop()
	for {
		select {
		case c, ok := <-src.Consents():
			if !ok {
				return startTimedOut
			}
			switch c {
			case navsource.ConsentApprove:
				return startPressed
			case navsource.ConsentStop:
				return startCancelled
			}
			// A REFUSAL KEY ANSWERS NOTHING HERE. No question is open, and
			// treating it as a cancellation would make the three keys mean
			// something different before the run than during it.
		case <-t.C:
			return startTimedOut
		case <-ctx.Done():
			return startCancelled
		}
	}
}

// newKeySource opens the desktop key hook this command listens to.
//
// A variable so a test can drive the REAL orchestration with a synthetic source. The start
// prompt, the countdown and the record were each tested on their own and all three passed while
// the production path returned before any of them wrote anything — the gap was never in the
// helpers, it was in which of them the command reached. A seam here is what lets a test press
// F9 at the start prompt and then read the record the command left behind.
var newKeySource = navsource.New

// freshLook takes one look right now, gives it to the observer, and reports what the observer
// made of it.
//
// # Why a look has to be taken here at all
//
// The observation loop captures, proposes and observes once a tick — and then calls `Advance`,
// which BLOCKS while somebody is asked to authorise an action. Nothing looks during that wait.
// So the "is the world still as planned" check read the observer's cached epoch and screen,
// which are the very ones the plan was made from, and compared the plan against itself.
//
// # Why taking it was not enough
//
// The stabiliser damps change on purpose: a look that differs from the standing screen is
// counted `Unsettled` and the STANDING screen is returned, for VisualSettleLooks consecutive
// looks. At ten looks a second that is three looks, and a person's F7 lands inside it — so
// both confirmations before an ordinary click could take a fresh look of a CHANGED interface
// and still read back the old screen and the old epoch, and clear the check.
//
// The damping is correct and is not weakened here. What this does instead is ask the
// stabiliser what it made of this particular look, which it already knew: `Stability()` before
// and after says whether the look was counted as a look at the standing screen or as something
// else. Reporting a counter is not a second opinion about screen identity.
//
// # And the control, not only the screen
//
// Geometry similar enough to settle clean can still have nothing under the approved
// coordinate. So the point is checked against this look's own proposals — the same evidence a
// control is found by in the first place.
//
// # Text
//
// On a screen words have been separating, agreeing geometry is not an answer — so the look's
// own reading is asked for BY FRAME and waited for inside a budget. That is `freshFrom`, which
// is where everything this decides lives; the capture and the proposal are here because only
// they need a desktop. `reader` is resolved at call time because the look is built before the
// reader exists.
//
// It costs one capture per authorised action, on the goroutine that is already waiting, and it
// is counted as the look it is.
func freshLook(shots *wincapture.Capture, obs *fastobserve.Observer, windows *winprovider.Provider,
	opt structure.Options, reader func() *readingEyes) func(context.Context,
	windowref.Validation, observe.PointerAt) freshEvidence {

	return func(ctx context.Context, v windowref.Validation,
		point observe.PointerAt) freshEvidence {

		lk := fastobserve.Look{
			At: time.Now(),
			Window: fastobserve.WindowRef{
				ID: v.Ref.ID, Generation: v.Ref.Generation, Bounds: v.Ref.Bounds,
			},
		}
		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		if err != nil {
			// NO PIXELS IS NOT "UNCHANGED". A look nobody could take is
			// recorded as missing and reported as evidence that was not
			// obtained, which `worldMoved` refuses.
			lk.Missing = true
			obs.Observe(lk)
			// NO PIXELS, AND STILL THE CLAIM AS IT STANDS, because the missing
			// look was offered to the observer and may have moved it.
			return freshEvidence{Counted: "no pixels could be captured",
				Epoch: obs.Epoch(), Screen: obs.Screen()}
		}
		lk.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref) == observe.FrontTarget
		res := structure.Propose(img.Image, opt)
		rects := make([]image.Rectangle, 0, len(res.Candidates))
		for _, c := range res.Candidates {
			rects = append(rects, c.Bounds)
		}
		lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)
		var eyes *readingEyes
		if reader != nil {
			eyes = reader()
		}
		return freshFrom(ctx, obs, eyes, lk, img.Image, point, boundaryTextWait)
	}
}

// boundaryTextWait is how long a boundary waits for the words of its own capture.
//
// # The arithmetic
//
// A reading costs 516ms at `readScale` on this machine, and the slowest measured was 920ms —
// see readScale, where both numbers were taken on these frames through this tesseract. At most
// one reading can be ahead of it: the observation loop is BLOCKED inside `Advance` for as long
// as somebody is being asked, so nothing new is offered while this waits, and the bridge
// serialises the one call that was already in flight when the question was posed. That call
// started before a person could answer, so most of it is already spent.
//
// One second and a half is the slowest measured reading plus most of another. Past it the honest
// answer is that the words did not arrive, and the honest answer is a refusal — a person presses
// F7 again and the second attempt has a quiet reader in front of it.
//
// It is paid per boundary, and there are two before an ordinary click (the hover probe and the
// world check). A press that waits three seconds and lands on the control somebody authorised
// is a better press than one that waits none and lands on the panel that replaced it.
const boundaryTextWait = 1500 * time.Millisecond

// boundaryReads numbers boundary captures, so a reading can be tied to the pixels it describes.
//
// Negative and descending, which keeps it out of the loop's own look numbering, and distinct per
// capture so the hover probe's boundary and the world check's boundary — two captures a quarter
// of a second apart, on one screen, with one epoch — cannot be credited with each other's words.
var boundaryReads atomic.Int64

func nextBoundaryRead() int { return int(-1 - boundaryReads.Add(1)) }

// freshFrom folds one boundary look and reports what the stabiliser made of it.
//
// Everything the boundary DECIDES, which is why it is here rather than inside the closure above:
// the capture and the proposal need a desktop and this does not.
//
// # The reading of this look, and not of the one before it
//
// `offer` then `ready` in the same breath is the pipeline the loop uses, and for the loop it is
// right: the queue is one deep, the newest frame wins, and whatever has finished describes a
// frame a few looks back. At a boundary it is the defect. The epoch does not move while the
// stabiliser damps a change, so a reading of the PREVIOUS frame is admissible, agrees with the
// standing screen, and confirms the very screen the plan was made on — and a panel swapped for
// one of the same shape has nothing left to refuse it.
//
// So the reading is asked for by frame, with an id of this capture's own, and waited for inside
// a budget. Nothing here drains `ready()`: a reading that is not of these pixels is not evidence
// about these pixels, however current its stamps are.
//
// # And whether words were needed at all
//
// Asked BEFORE the look, of the screen standing then — which is the screen the point was
// approved on. After the look the question would be about a vocabulary this very look's reading
// had just contributed to. A screen nobody has read, or a run with no reader, is folded exactly
// as it always was: text can only refuse a merge, so where there is no text there is nothing to
// withhold and pixels are the whole of the policy.
func freshFrom(ctx context.Context, obs *fastobserve.Observer, reader *readingEyes,
	lk fastobserve.Look, img image.Image, point observe.PointerAt,
	within time.Duration) freshEvidence {

	needed := reader.working() && obs.SaidBy(obs.Screen()).Said()
	own := false
	// ONLY WHERE THE ANSWER COULD DECIDE ANYTHING. A screen with no vocabulary cannot
	// be contradicted by a reading, so waiting for one would spend a second and a half
	// of a person's time, twice per press, to learn nothing.
	if needed && img != nil {
		id := nextBoundaryRead()
		rd, got := reader.readNow(ctx, readingFrame{
			img: img, look: id,
			epoch:      obs.Epoch(),
			generation: lk.Window.Generation,
		}, within)
		// THIS FRAME'S READING, AND NO OTHER.
		//
		// The id is what says the words came from the pixels this look is made
		// of. Nothing else is asked here: whether a reading is still admissible
		// is the observer's own rule and is applied inside `intakeReadings`,
		// where every late reading goes through it. Asking it twice would be two
		// statements of one rule, and `Said` already requires the stabiliser to
		// have counted this look's words — an inadmissible reading is dropped in
		// there and the boundary refuses for want of it.
		if got && rd.FromLook == id {
			// THE STABILISER GETS IT EITHER WAY, because a reading is a fact
			// about the screen whatever it found, and the vocabulary is built
			// from all of them.
			lk.Readings = []observe.ScreenReading{rd}
			// ── BUT A REQUEST THAT RETURNED IS NOT EVIDENCE THAT ARRIVED ──
			//
			// OCR can answer a frame and read nothing on it: a panel mid-fade,
			// a capture that came out black, a tesseract that found no glyphs
			// it believed. `VisualScreens.Read` counts that look as read,
			// because a source DID answer and that distinction matters to its
			// own accounting — see ReadingsSilent. For this check it is the
			// wrong answer: empty text cannot contradict an identity, so a
			// boundary that took a successful reading of nothing would clear a
			// same-shape change on the strength of a request completing.
			//
			// The bar is the stabiliser's own and not a new one. Below
			// `TextDisagreementWords` words, `TextsDisagree` declines to call
			// either side a claim at all — so a reading that thin could not
			// have refused this merge however different what it said was, and
			// accepting it as the evidence this screen needs would be accepting
			// a measurement that was never going to be consulted.
			own = len(rd.Text.Words) >= observe.TextDisagreementWords
		}
	}
	was := obs.Stability()
	obs.Observe(lk)
	// THE VERDICT AND THE STANDING CLAIM, BOTH FROM AFTER THE LOOK.
	//
	// One statement, because they are one answer: what the stabiliser made of this
	// look, and what it is standing on now that it has seen it. Reading either
	// before `obs.Observe` above would be reading the state the plan was made in.
	e := freshOf(was, obs.Stability(), lk.Regions, point, own)
	e.Epoch, e.Screen = obs.Epoch(), obs.Screen()
	e.Needed = needed
	return e
}

// freshOf reads the stabiliser's verdict on one look out of its counters.
//
// Separate from `freshLook` and pure, because this is the part that can be wrong in a way
// nobody would notice: taking a look is visible in a capture count, and misreading which
// counter moved is not visible anywhere.
//
// `Settled` is the only counter whose movement means "this was a look at the screen that is
// standing". Every other one names something else that happened to the look, and the name goes
// into the refusal so a person is told which.
//
// `ownText` is the caller saying it put a reading of THIS look's own pixels on the look, and
// that the reading said enough to be a claim. Both halves are required for `Said` and neither
// is sufficient: the counter alone cannot tell a reading of these pixels from a stale one the
// epoch rule happened to admit, nor a reading that found words from one that found none — it
// counts a source having ANSWERED. And the caller alone cannot say whether the words ever
// reached the text rule, because an inadmissible or unusable reading is dropped inside the
// observer, silently and correctly.
func freshOf(was, now observe.VisualStability, proposed []observe.Region,
	point observe.PointerAt, ownText bool) freshEvidence {

	e := freshEvidence{Took: true, Said: ownText && now.TextRead > was.TextRead}
	switch {
	case now.Settled > was.Settled:
		e.Agreed, e.Counted = true, "settled"
	case now.Promotions > was.Promotions:
		e.Counted = "it promoted a different screen"
	case now.Interrupted > was.Interrupted:
		e.Counted = "the window being looked at had been replaced"
	case now.Unobservable > was.Unobservable:
		e.Counted = "nothing has been observable for long enough to give the " +
			"screen up"
	case now.Unobserved > was.Unobserved:
		e.Counted = "nothing was found in the pixels"
	case now.Unsettled > was.Unsettled:
		e.Counted = "it differed from the standing screen and has not settled " +
			"into a new one"
	case now.Warming > was.Warming:
		e.Counted = "no screen has settled yet"
	default:
		// NO COUNTER MOVED. The look reached the stabiliser and the
		// stabiliser classified it as none of the above, which is not a
		// case this knows how to read — so it is not an agreement.
		e.Counted = "the look was not classified"
	}
	// AND THE POINT, IN THIS LOOK'S OWN PROPOSALS.
	//
	// Asked whatever the screen verdict was, because the sentence a person gets should
	// name the thing that is actually wrong, and a settled screen with nothing under the
	// point is a different afternoon from a screen that changed.
	for _, r := range proposed {
		if holdsPoint(r, point) {
			e.Target = true
			break
		}
	}
	return e
}

// holdsPoint reports whether a normalised region contains a normalised point.
//
// Half-open on the far edges, which is the convention `observe.Region` is built with: a point
// exactly on the right or bottom edge of one region is inside the next.
func holdsPoint(r observe.Region, p observe.PointerAt) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// ── THE THREE OPTIONAL SOURCES, BEHIND SEAMS ──
//
// Variables rather than direct calls, for the same reason as `newKeySource`: each of these can
// be asked for and be absent, and each absence is a `return` past the printed reference. All
// three of those returns wrote no record until a review traced them, and the only way a test
// can reach one is to make the source unavailable on purpose — the machine running the suite
// has no say in whether a plugin is installed.
var (
	openLearnedEyes = newLearnedEyes
	openReadingEyes = newReadingEyes
	openWordReader  = newWordReader
)

// replayWindow is the window a run in geometry replay mode checks against, and nil otherwise.
//
// Nil for a run that was not granted the mode, and nil for an artifact that never recorded a
// window — which is the same refusal the planner makes, arriving at the boundary as "this is not
// a geometry replay" rather than as a geometry replay with nothing to check. The planner has
// already refused such a step by then; this is here so that a future caller cannot reach the
// boundary with a mode that has no window behind it.
func replayWindow(on bool, a workflowstore.Artifact) *workflowstore.WindowGeometry {
	if !on || !a.Window.Known() {
		return nil
	}
	w := *a.Window
	return &w
}
