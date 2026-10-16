package main

// `director demonstrate-surface -check` — the preconditions for one live focused demonstration.
//
// # Why this exists before the experiment does
//
// Because the owner's instruction says what to do when a precondition is false:
//
//	If the live hook is absent or the required foreground evidence is unavailable, stop before
//	execution and report the missing prerequisite rather than weakening a guard.
//
// And because four dogfood runs in a row were once walked against a Director that was not
// watching. `dogfood.cmd` refuses to open when its preconditions are false for exactly that
// reason, and this is the same discipline for a different experiment: print what is actually
// true, exit non-zero when it is not the answer the experiment needs, and interact with nothing.
//
// # What it does, and what it cannot do
//
// It reads. It installs the production input hook, reports whether the hook installed and —
// with `-watch` — whether a press from a hand actually arrives through the production path. It
// collects ONE Stage from the foreground window through the production collector and reports
// whether the instruction the experiment is about would raise a surface question, resolve
// outright, or refuse.
//
// It injects nothing. There is no executor on this path, no Marco runtime, and no authorisation
// of any kind — the runtime it builds is a DRY RUN, so the one host that could reach the desktop
// is `runtime.DryRunHost{}`. The phases that learn and execute are not here, because the
// diagnosis below decides whether they can exist.
//
// # The sandbox
//
// Every phase refuses unless MARCO_HOME names a throwaway directory, through `resetGuard` — the
// same predicate `reset-test-state` uses, for the same reason: a demonstration written into the
// real store is a demonstration nobody can discard without discarding everything else.

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// surfaceWordsShown is how many of the words read on a Stage a diagnosis prints.
//
// Enough to recognise the screen AND to choose an instruction against it, which is what a
// sample of twelve was not: the first two diagnoses of a live Settings page each turned on a word
// the truncation had cut. The count is printed beside them, so a truncated sample never reads as
// the whole Stage.
const surfaceWordsShown = 40

// surfaceCheck is one precondition and what was actually found.
type surfaceCheck struct {
	Name string
	OK   bool
	// Detail is what was found, in words. Always printed, pass or fail.
	Detail string
	// Blocking says the experiment cannot proceed without this one.
	Blocking bool
}

// watchFor is how long a watching phase waits, with a default a person can actually use.
//
// Thirty seconds: long enough to read a sentence, look at the screen and aim deliberately, which
// is slower than clicking and is what is being asked for. Bounded, because an unbounded watch is
// a Director sitting on somebody's input with no end.
func watchFor(given int) int {
	if given > 0 {
		return given
	}
	return 30
}

// runDemonstrateSurface is `director demonstrate-surface`.
func runDemonstrateSurface(args []string) int {
	fs := flag.NewFlagSet("demonstrate-surface", flag.ExitOnError)
	check := fs.Bool("check", false, "report the preconditions and stop, interacting with nothing")
	watch := fs.Int("watch", 0,
		"after the other checks, watch for this many seconds and report whether a press "+
			"from a hand reaches the production input path")
	phrase := fs.String("instruction", "",
		"the instruction the experiment is about, e.g. \"click Bluetooth & devices\"")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	maxNodes := fs.Int("max-nodes", 4000, "node ceiling per snapshot")
	learn := fs.Bool("learn", false,
		"ask, watch a person demonstrate, and keep what an attributed crossing confirms")
	reground := fs.Bool("reground", false,
		"collect a fresh Stage and report what the saved relation picks out of it")
	exec := fs.Bool("execute", false, "perform exactly one guarded click")
	authorise := fs.Bool("authorise-one-click", false,
		"authorise the single click -execute would perform")
	_ = fs.Parse(args)

	// ONE PHASE PER RUN, and never two. The phases are separate commands because
	// separating learning from execution is a safety property rather than a convenience —
	// see surfacelearnphases.go.
	chosen := 0
	for _, on := range []bool{*check, *learn, *reground, *exec} {
		if on {
			chosen++
		}
	}
	if chosen != 1 {
		fmt.Fprintln(os.Stderr, "director demonstrate-surface: choose exactly one of")
		fmt.Fprintln(os.Stderr, "  -check      report the preconditions; interacts with nothing")
		fmt.Fprintln(os.Stderr, "  -learn      ask, watch a person demonstrate, keep what was confirmed")
		fmt.Fprintln(os.Stderr, "  -reground   a fresh Stage, and what the saved relation picks out of it")
		fmt.Fprintln(os.Stderr, "  -execute    one guarded click, with -authorise-one-click")
		return 2
	}
	switch {
	case *learn:
		return runSurfaceLearn(os.Stdout, *phrase, *bridgeFlag, *maxNodes,
			watchFor(*watch), modePixelOnly, true)
	case *reground:
		return runSurfaceReground(os.Stdout, *phrase, *bridgeFlag, *maxNodes,
			modePixelOnly)
	case *exec:
		return runSurfaceExecute(os.Stdout, *phrase, *bridgeFlag, *maxNodes, *authorise,
			modePixelOnly)
	}
	return surfacePreconditions(os.Stdout, *phrase, *bridgeFlag, *maxNodes, *watch,
		modePixelOnly)
}

// surfacePreconditions runs every check and reports.
//
// Split from the command for the reason every harness in this tree is: the flags are not the
// thing worth testing and the refusals are.
func surfacePreconditions(out io.Writer, phrase, bridge string, maxNodes, watch int,
	mode observationMode) int {

	var checks []surfaceCheck
	add := func(c surfaceCheck) { checks = append(checks, c) }

	// ── 1. THE SANDBOX, AND THE CONFIGURATION IT UNLOCKS ──
	//
	// The experiment's own configuration, enabled here so that the Stage diagnosed below
	// is the Stage the learning phase will see. Diagnosing production defaults and then
	// learning under a different configuration would be a check about the wrong screen.
	add(surfaceSandboxCheck(os.Getenv("MARCO_HOME"), defaultHome()))
	switch {
	case mode == modeProduction:
		// PRODUCTION OBSERVATION, which is not a second test configuration: it is
		// the one every other command uses. Named as a precondition anyway, because
		// a result reported without the configuration it was obtained under is the
		// kind of finding that gets quoted later as though it were about the other.
		add(surfaceCheck{Name: "observation configuration", OK: true,
			Blocking: true, Detail: pixelOnlyNote()})
	case enableTestObservation(os.Getenv("MARCO_HOME"), defaultHome(), mode) != nil:
		add(surfaceCheck{Name: "test observation configuration", Blocking: true,
			Detail: enableTestObservation(os.Getenv("MARCO_HOME"), defaultHome(),
				mode).Error()})
	default:
		add(surfaceCheck{Name: "test observation configuration", OK: true,
			Blocking: true, Detail: pixelOnlyNote()})
	}

	// ── 2. THE PLUGINS THE STAGE NEEDS ──
	if _, err := os.Stat(bridge); err != nil {
		add(surfaceCheck{Name: "accessibility bridge", Blocking: true,
			Detail: fmt.Sprintf("not found at %s; build it with "+
				"`powershell -File plugins/uia/build.ps1`", bridge)})
	} else {
		add(surfaceCheck{Name: "accessibility bridge", OK: true, Blocking: true,
			Detail: bridge})
	}

	// ── 3. THE RUNTIME, DRY ──
	//
	// A dry run, so the one host that could reach the desktop is DryRunHost. Nothing on
	// this path can press anything, which is a property of the object graph rather than a
	// promise in a comment.
	graph, gerr := openGraph()
	if gerr != nil {
		add(surfaceCheck{Name: "action graph", Blocking: true, Detail: gerr.Error()})
		return reportSurfaceChecks(out, checks, phrase)
	}
	rt, rerr := NewRuntime(bridge, maxNodes, true, graph)
	if rerr != nil {
		add(surfaceCheck{Name: "director runtime", Blocking: true, Detail: rerr.Error()})
		return reportSurfaceChecks(out, checks, phrase)
	}
	defer rt.Close()

	// ── 4. THE INPUT HOOK, INSTALLED ──
	//
	// Prerequisite 1's first half, and the owner was explicit that the recorded-event tests
	// are not evidence for it: "Do not treat the existing recorded-event tests as evidence
	// that live input is captured."
	if rt.navUnavailable != "" {
		add(surfaceCheck{Name: "input observation hook installed", Blocking: true,
			Detail: rt.navUnavailable})
	} else {
		add(surfaceCheck{Name: "input observation hook installed", OK: true,
			Blocking: true,
			Detail: "navsource reports the hook is installed; it observes a closed " +
				"vocabulary of intents, refuses injected events, refuses a press " +
				"on Marco's own surface and never suppresses"})
	}

	// ── 5. DURABLE MEMORY ──
	if rt.semanticMemoryUnavailable != "" {
		add(surfaceCheck{Name: "durable memory", Blocking: true,
			Detail: rt.semanticMemoryUnavailable})
	} else {
		add(surfaceCheck{Name: "durable memory", OK: true, Blocking: true,
			Detail: semanticMemoryPath()})
	}

	// ── 6. WHICH PROVIDERS ARE ACTUALLY REGISTERED ──
	//
	// The other half of "accessibility neither requested nor registered": the request's
	// Sources list is a statement of intent and this is what was built. Printed because a
	// configuration that was meant to exclude a provider and did not is exactly the kind of
	// thing that reads correct in a comment.
	add(surfaceProvidersCheck(rt))

	// ── 7. THE SEAMS ADR-207 WIRED ──
	add(surfaceCheck{Name: "surface grounder wired", OK: rt.pipeline.Surfaces != nil,
		Blocking: true,
		Detail: fmt.Sprintf("Pipeline.Surfaces non-nil: %v; Pipeline.Place non-nil: %v",
			rt.pipeline.Surfaces != nil, rt.pipeline.Place != nil)})

	// ── 8. THE STAGE, AND THE DECISIVE QUESTION ──
	add(surfaceStageCheck(rt, phrase))

	// ── 9. AND WHETHER A HAND'S PRESS ACTUALLY ARRIVES ──
	if watch > 0 {
		add(surfaceWatchCheck(out, rt, watch))
	} else {
		add(surfaceCheck{Name: "input observation hook active",
			Detail: "not measured; re-run with -watch 10 and click inside the " +
				"watched window while it runs"})
	}

	return reportSurfaceChecks(out, checks, phrase)
}

// surfaceProvidersCheck reports which observation providers this Director registered.
//
// Under the pixel-only configuration accessibility must be absent, and this is the only place
// that is asserted about the object that was actually built rather than about the request that
// will be sent to it.
//
// Deleting the exclusion in NewRuntime must fail TestThePixelOnlyRuntimeRegistersNoAccessibility.
func surfaceProvidersCheck(rt *Runtime) surfaceCheck {
	var names []string
	accessible := false
	for _, p := range rt.collector.Providers() {
		names = append(names, p.Name())
		for _, src := range p.Sources() {
			if src == directorapi.SourceAccessibility {
				accessible = true
			}
		}
	}
	return surfaceProvidersVerdict(names, accessible)
}

// surfaceProvidersVerdict judges a provider list against what the configuration requires.
//
// # The defect this is a correction of
//
// This asked `pixelOnly != nil`, which was the same question as "is accessibility excluded"
// right up until there were three configurations rather than two. The augmented one sets the
// variable and requires accessibility to be PRESENT, so the check failed it and explained the
// failure by naming the pixel-only configuration — a report that was wrong twice over, in a
// run where the only other false precondition was the real one.
//
// So the question is now the one that was always meant: does THIS configuration exclude
// accessibility? One function answers it, `pixelOnlyExcludesAccessibility`, and it is the same
// function `NewRuntime` consults when building the list — so the check and the thing it checks
// cannot disagree about what was required.
//
// Split from the collector walk so the verdict can be tested for all three configurations
// without a runtime, which is what was missing: the pixel-only case had a test and the other
// two had none.
//
// It reads the predicate itself rather than taking it as an argument, because the first
// version took it as an argument and a mutation gate immediately showed why that was wrong:
// the verdict was held by a test and the CALLER's choice of predicate was not, so the defect
// this function exists to correct could be reintroduced one line above it and nothing would
// fail. One predicate, read in one place, leaves nothing to pass wrongly.
//
// Deleting the distinction must fail TestTheProviderCheckAsksWhatTheConfigurationRequires.
func surfaceProvidersVerdict(names []string, accessible bool) surfaceCheck {
	excluded := pixelOnlyExcludesAccessibility()
	switch {
	case excluded && accessible:
		return surfaceCheck{Name: "registered observation providers", Blocking: true,
			Detail: fmt.Sprintf("%v — this configuration excludes accessibility "+
				"and it is registered anyway, so a fall back to it is possible",
				names)}
	case !excluded && !accessible:
		// THE OTHER DIRECTION, which had no check at all. A configuration that
		// needs accessibility and did not get it cannot recognise a screen, and
		// [[Experiment-063-the-configuration-that-cannot-close-its-own-loop]] is
		// what that costs: a demonstration with nowhere to be kept.
		return surfaceCheck{Name: "registered observation providers", Blocking: true,
			Detail: fmt.Sprintf("%v — this configuration needs accessibility and "+
				"no provider offers it, so no screen could be recognised", names)}
	}
	return surfaceCheck{Name: "registered observation providers", OK: true,
		Blocking: true, Detail: fmt.Sprintf("%v", names)}
}

// surfaceSandboxCheck is the owner's first safeguard: a disposable learning record.
//
//	Use a disposable learning record. We should be able to inspect and discard the
//	demonstration without contaminating future sessions.
//
// THE SAME PREDICATE `reset-test-state` USES, deliberately: a home this refuses to learn into
// is a home that cannot be emptied afterwards, and those have to be the same set. A second
// opinion about what counts as a sandbox would eventually let one command write where the other
// could not clean up.
func surfaceSandboxCheck(home, realHome string) surfaceCheck {
	if why := resetGuard(home, realHome); why != "" {
		return surfaceCheck{Name: "disposable learning record", Blocking: true,
			Detail: why}
	}
	return surfaceCheck{Name: "disposable learning record", OK: true, Blocking: true,
		Detail: fmt.Sprintf("MARCO_HOME is %s, which is not the real store; a "+
			"demonstration written here can be inspected and discarded with "+
			"`director reset-test-state`", home)}
}

// surfaceStageCheck collects ONE Stage from the foreground window and reports what the
// instruction would do against it.
//
// # The check the whole experiment turns on
//
// A surface question exists only where identity and interaction surface have come apart, which is
// where nothing but pixels describes the screen. Where an accessibility tree reports a button,
// the thing that is named and the thing that is pressed are one object and there is nothing to
// ask about.
//
// `internal/director/perception/providers/structure` is the provider that finds drawn rectangles,
// and it is OPT-IN: `req.Includes(SourceStructure)` is its gate, and no production observation
// request includes it. So this reports which sources actually described the target, which is the
// fact that decides whether the experiment can run at all on this application.
// observeStage collects a Stage the pixel providers could actually read.
//
// TWO CYCLES, deliberately. Every pixel provider is scoped to a window by
// `Runtime.activeWindow`, which takes its candidate from the LAST OBSERVED WORLD — because
// text has to be fused against elements from a world, and photographing a different window
// from the one those elements came from would compare things that were never on screen
// together. A window therefore has to be in a world before anything can look inside it.
//
// With accessibility collected that ordering is invisible: the walk reports the window and
// its controls in the same breath, and the pixel providers are opt-in and asked for later,
// by which time a world exists. Without it, the first cycle is the one that establishes the
// window and the second is the first that can see anything inside it.
//
// So this is not a workaround for the test configuration. It is what "the last observed
// world" has always meant, said out loud in the one place where the first cycle is also the
// only cycle.
//
// The first cycle's error is not reported, because the first cycle is EXPECTED to be partial
// — there is no window yet, so the providers that need one say so. The second is the one
// whose failure is a finding.
//
// Deleting the warm-up must fail TestAStageIsCollectedWithAWindowAlreadyInIt.
func observeStage(ctx context.Context, rt *Runtime) (directorapi.WorldState, error) {
	_, _ = rt.pipeline.Observe(ctx)
	return rt.pipeline.Observe(ctx)
}

func surfaceStageCheck(rt *Runtime, phrase string) surfaceCheck {
	if phrase == "" {
		return surfaceCheck{Name: "the instruction against the live Stage",
			Detail: "no -instruction given, so the Stage was not collected"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	world, err := observeStage(ctx, rt)
	if err != nil {
		return surfaceCheck{Name: "the instruction against the live Stage",
			Blocking: true, Detail: "could not observe: " + err.Error()}
	}
	app := ""
	if world.ActiveWindow != nil {
		if win, ok := world.Window(*world.ActiveWindow); ok {
			app = fmt.Sprintf("%s (%q)", win.Application, win.Title)
		}
	}
	words := instructionWords(phrase)
	q := directorapi.ElementQuery{Text: words}

	// WHAT THE EVALUATOR MAKES OF IT, which is the same object the pipeline consults.
	h := surface.New().Evaluate(&world, phrase, "click", q)
	detail := fmt.Sprintf("foreground %s; %d Props; %q → %s",
		app, len(world.Elements), words, h.Decision)
	if h.Identity != nil {
		detail += fmt.Sprintf("\n        identity %s %q role=%s sources=%v rejected=%q",
			h.Identity.ElementID, h.Identity.Label, h.Identity.Role,
			h.Identity.Sources, h.Identity.Rejected)
	}
	detail += "\n        " + h.Explanation
	if _, askable := h.Question(); askable {
		return surfaceCheck{Name: "the instruction against the live Stage", OK: true,
			Blocking: true,
			Detail: detail + "\n        A SURFACE QUESTION WOULD BE ASKED, so the " +
				"experiment has something to demonstrate." +
				whichSurfacesWereRefused(h)}
	}
	// AND WHY NOT, which is the prerequisite report rather than a failure of this command.
	return surfaceCheck{Name: "the instruction against the live Stage", Blocking: true,
		Detail: detail + "\n        NO SURFACE QUESTION WOULD BE ASKED. " +
			whatTheStageContains(&world) + whichSurfacesWereRefused(h)}
}

// —— SURVEY ————————————————————————————————————————————————————————————

// surfaceWordsSurveyed is how many of a Stage's words one survey will try.
//
// A ceiling rather than a judgement: the evaluation is in-process against a Stage already
// collected, so the cost is small, and a screen with four hundred words is a screen where a
// person would rather see the first hundred than wait.
const surfaceWordsSurveyed = 100

// surveyTheStage asks of every word on one Stage: would THIS one raise a surface question?
//
// # Why a survey rather than another guess
//
// Because three instructions against one live Settings page produced three different
// near-misses, and each one cost a full Stage collection to discover. "Bluetooth devices" had
// one defensible surface and a card around it that held a second name. "Bluetooth" matched
// three Props at once, which is an unsettled IDENTITY and not something a press can answer.
// "Accounts" had no surface around it at all, because a Windows 11 navigation row draws no
// rectangle unless it is selected or hovered, so the proposer had nothing to propose.
//
// Every one of those is a fact about the SCREEN, and the screen was willing to say all of them
// at once. Guessing a word, collecting a Stage and reading one answer is the slow way to ask a
// question the Stage can answer in full.
//
// # What it does not do
//
// It does not choose. It reports which words would raise a question and which would not, with
// the reason, and the point of that is to find a screen where the thing being tested exists —
// never to pick whichever word happens to get past the rules. Nothing here evaluates anything
// the ordinary path would not; it is the same `surface.Evaluate` the pipeline consults, asked
// once per word against one world.
//
// The words it prints are held by TestASurveyReportsTheAskableWordsFirst. The surveying
// itself needs a desktop, so nothing in the suite catches the deletion of the loop — said
// plainly rather than pointed at a test that would not fail.
func surveyTheStage(out io.Writer, rt *Runtime) int {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	world, err := observeStage(ctx, rt)
	if err != nil {
		fmt.Fprintf(out, "STOP: could not observe: %v\n", err)
		return 1
	}
	app := ""
	if world.ActiveWindow != nil {
		if win, ok := world.Window(*world.ActiveWindow); ok {
			app = fmt.Sprintf("%s (%q)", win.Application, win.Title)
		}
	}
	fmt.Fprintf(out, "\nSURVEY OF ONE STAGE — foreground %s, %d Props\n", app,
		len(world.Elements))

	asked, settled, byReason := []string{}, []string{}, map[string]int{}
	ev := surface.New()
	for i, word := range stageWords(&world) {
		if i >= surfaceWordsSurveyed {
			fmt.Fprintf(out, "  (stopping at %d words)\n", surfaceWordsSurveyed)
			break
		}
		request := "click " + word
		h := ev.Evaluate(&world, request, "click",
			directorapi.ElementQuery{Text: instructionWords(request)})
		if q, ok := h.Question(); ok {
			asked = append(asked, fmt.Sprintf("%q — %d options, depth 0 %v",
				word, len(q.Options), q.Identity.Bounds))
			continue
		}
		if h.Chosen != nil && h.Unsettled == "" {
			settled = append(settled, word)
			continue
		}
		byReason[surveyReason(h)]++
	}

	fmt.Fprint(out, surveyReport(asked, settled, byReason))
	if len(asked) == 0 {
		return 1
	}
	return 0
}

// surveyReport is what a survey says, separated from the surveying.
//
// THE ANSWER FIRST. A survey whose useful line is buried under the tallies is a survey
// somebody reads the wrong way round, and the tallies are long: twenty-six words came back
// under three reasons on the first live Settings page, with the two that mattered on top.
//
// Separated from the collection because the collection needs a desktop and the words do not.
// Every refusal in this subsystem is tested that way and this is the same discipline: the
// thing worth holding to is what it SAYS.
//
// Deleting the askable report must fail TestASurveyReportsTheAskableWordsFirst.
func surveyReport(asked, settled []string, byReason map[string]int) string {
	out := ""
	if len(asked) == 0 {
		out += "  NOT ONE WORD ON THIS STAGE WOULD RAISE A SURFACE QUESTION.\n"
	} else {
		out += fmt.Sprintf("  %d word(s) WOULD raise a surface question:\n", len(asked))
		for _, a := range asked {
			out += fmt.Sprintf("    %s\n", a)
		}
	}
	if len(settled) > 0 {
		out += fmt.Sprintf("  %d word(s) resolve outright, so there is nothing to ask: "+
			"%s\n", len(settled), strings.Join(settled, " | "))
	}
	for _, why := range sortedSurveyReasons(byReason) {
		out += fmt.Sprintf("  %3d — %s\n", byReason[why], why)
	}
	return out
}

// stageWords is the distinct labels on a Stage, in a stable order.
//
// Short ones are dropped. OCR on a real screen reads punctuation and single glyphs — "Q",
// ">", "@", "eS" came off one Settings page — and an instruction built from one of those is
// not a question anybody would ask.
func stageWords(world *directorapi.WorldState) []string {
	seen := map[string]bool{}
	var out []string
	for _, el := range world.Elements {
		if el == nil {
			continue
		}
		label := strings.TrimSpace(el.Label)
		if len(label) < surfaceShortestWord || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// surfaceShortestWord is the shortest label a survey will build an instruction from.
const surfaceShortestWord = 4

// surveyReason is the one-line reason a word raised no question.
func surveyReason(h surface.Hypothesis) string {
	switch {
	case h.Identity == nil:
		return "no Prop is named like the word"
	case h.Unsettled == "which of these the request meant":
		return "several Props are named alike, which is an unsettled identity and not " +
			"something a press can answer"
	case h.Unsettled == "whether the words are the control or its caption":
		return "only the words' own region is defensible, so there is one place a " +
			"press could land and nothing to ask about"
	case h.Unsettled != "":
		return "left open: " + h.Unsettled
	}
	return "no surface: " + string(h.Decision)
}

// sortedSurveyReasons orders the tallied reasons, commonest first.
func sortedSurveyReasons(by map[string]int) []string {
	out := make([]string, 0, len(by))
	for why := range by {
		out = append(out, why)
	}
	sort.Slice(out, func(i, j int) bool {
		if by[out[i]] != by[out[j]] {
			return by[out[i]] > by[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// runSurfaceSurvey builds the dry-run runtime a survey needs and surveys one Stage.
func runSurfaceSurvey(out io.Writer, bridge string, maxNodes int,
	mode observationMode) int {

	rt, err := surfacePhaseUnder(out, bridge, maxNodes, false, mode)
	if err != nil {
		fmt.Fprintf(out, "STOP: %v\n", err)
		return 1
	}
	defer rt.Close()
	return surveyTheStage(out, rt)
}

// whichSurfacesWereRefused prints every surface the evaluator weighed and why it lost.
//
// `Hypothesis.Candidates` already carries them with their reasons — "the losers are the
// explanation", as its own comment puts it — and a precondition report that said only "1
// surface(s) around it were considered and refused" was withholding the one fact that decides
// what to do next.
//
// Because there are two completely different situations behind that sentence. Either this
// screen has no nesting to ask about, in which case the answer is a different screen; or the
// nesting is there and the rule that refused it is the thing to look at, which is a decision
// for the owner and not something to quietly relax. One report cannot tell them apart; the
// reasons can.
//
// Deleting this must fail TestTheRefusedSurfacesAreNamedWithTheirReasons.
func whichSurfacesWereRefused(h surface.Hypothesis) string {
	if len(h.Candidates) == 0 {
		return ""
	}
	out := fmt.Sprintf("\n        %d surface(s) were weighed:", len(h.Candidates))
	for _, c := range h.Candidates {
		why := c.Rejected
		if why == "" {
			why = "DEFENSIBLE"
		}
		role := "a surface around the words"
		if c.Evidence.IsIdentity {
			role = "the words' own region"
		}
		out += fmt.Sprintf("\n          %s %v press %v — %s — %s",
			c.ElementID, c.Bounds, c.Point, role, why)
	}
	if h.Unsettled != "" {
		out += fmt.Sprintf("\n        the evaluator left this open: %s", h.Unsettled)
	}
	return out
}

// whatTheStageContains says what was on the Stage, not only that the instruction missed.
//
// Written because the first operated run reported "nothing in this window is named like
// \"Bluetooth & devices\"" against a Stage of 65 Props, and that is two completely different
// findings wearing one sentence: the words might not be on the screen, or they might be on
// the screen and unread. One of those is a precondition and the other is a defect, and an
// operator cannot act on either until they know which.
//
// So both halves are counted by SOURCE — shapes from the proposer, words from OCR — and a
// sample of the words actually read is printed. A Stage with shapes and no words cannot
// identify anything by name, whatever is on it.
//
// Deleting the words must fail TestTheStageDiagnosisNamesTheWordsItRead, and the
// source counts must fail TestTheStageDiagnosisSeparatesShapesFromWords.
func whatTheStageContains(world *directorapi.WorldState) string {
	structural, textual := 0, 0
	var words []string
	seen := map[string]bool{}
	for _, el := range world.Elements {
		if el == nil {
			continue
		}
		for _, ref := range el.Provenance.Sources {
			switch ref.Source {
			case directorapi.SourceStructure:
				structural++
			case directorapi.SourceOCR:
				textual++
			}
		}
		if label := strings.TrimSpace(el.Label); label != "" && !seen[label] {
			seen[label] = true
			words = append(words, label)
		}
	}
	sort.Strings(words)

	out := fmt.Sprintf("%d of %d Props carry a shape from the proposer and %d carry "+
		"words from OCR.", structural, len(world.Elements), textual)
	if len(words) == 0 {
		// THE DIAGNOSIS THAT MATTERS MOST, because it is not about the
		// instruction. A Stage with no words on it cannot identify anything by
		// name, so no instruction would have matched and the next thing to look at
		// is the reader rather than the screen.
		return out + "\n        NOT ONE WORD WAS READ on this Stage, so nothing on it " +
			"can be identified by name and no instruction would have matched. " +
			"That is about the text reader, not about this screen."
	}
	shown := words
	if len(shown) > surfaceWordsShown {
		shown = shown[:surfaceWordsShown]
	}
	return out + fmt.Sprintf("\n        %d distinct word(s) were read, including: %s",
		len(words), strings.Join(shown, " | "))
}

// instructionWords is the target phrase out of "click <something>".
//
// Deliberately crude: this is a diagnostic, and the real parse happens in `intent`. Taking the
// instruction apart properly here would be a second parser to keep in step with the first.
func instructionWords(phrase string) string {
	for _, verb := range []string{"click ", "press ", "open "} {
		if len(phrase) > len(verb) && equalFoldASCII(phrase[:len(verb)], verb) {
			return phrase[len(verb):]
		}
	}
	return phrase
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// surfaceWatchCheck watches for a hand's press and reports whether one arrived.
//
// Prerequisite 1's second half. It enables ambient watching, which is what opens the navigation
// subscription — the input log `ambientObserver.drain` reads is populated only by an observation
// session that subscribed, so a hook that installed and a hook whose events reach the correlator
// are two different claims.
//
// COUNTS ONLY. `observe.InputStats` carries no key identity and no content by construction; what
// is printed is how many events arrived, how many were placed in the window, and why the rest
// were ignored. Nothing about what was typed exists to print.
func surfaceWatchCheck(out io.Writer, rt *Runtime, seconds int) surfaceCheck {
	rt.EnableAmbient()
	defer rt.DisableAmbient()
	// THE CHECKPOINT BELONGS HERE, not in whatever started this: it marks the moment the
	// hook is actually watching, and an operator that announced it earlier would be
	// telling somebody to click at a time when nothing would see them.
	checkpoint(out, checkObserving)
	fmt.Fprintf(out, "watching for %ds — click inside the application you want to "+
		"demonstrate in, then wait…\n", seconds)

	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	presses, placed := 0, 0
	var last observe.InputStats
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if rt.navSource == nil {
			break
		}
		last = rt.navSource.Stats()
		presses, placed = last.Received, last.Classified
	}
	detail := fmt.Sprintf("%d admitted event(s), %d placed in the watched window; "+
		"resolved=%d unnamed=%d unresolved=%d injected=%d declined=%d",
		presses, placed, last.PointerResolved, last.PointerUnnamed,
		last.PointerUnresolved, last.PointerInjected, last.PointerDeclined)
	if last.Unavailable != "" {
		return surfaceCheck{Name: "input observation hook active", Blocking: true,
			Detail: "the producer is unavailable: " + last.Unavailable}
	}
	if placed == 0 {
		return surfaceCheck{Name: "input observation hook active", Blocking: true,
			Detail: detail + "\n        No press was placed inside a watched " +
				"window. Either nothing was clicked, or the window being " +
				"watched is not the one clicked in."}
	}
	return surfaceCheck{Name: "input observation hook active", OK: true, Blocking: true,
		Detail: detail}
}

// reportSurfaceChecks prints the account and returns the exit code.
//
// Non-zero when any BLOCKING check failed, so this can gate a script — and the whole account is
// printed either way, because "which precondition is false" is the answer, not a detail.
func reportSurfaceChecks(out io.Writer, checks []surfaceCheck, phrase string) int {
	fmt.Fprintf(out, "\nPRECONDITIONS FOR ONE LIVE FOCUSED DEMONSTRATION")
	if phrase != "" {
		fmt.Fprintf(out, " — %q", phrase)
	}
	fmt.Fprint(out, "\n\n")
	blocked := 0
	for _, c := range checks {
		mark := "  no "
		switch {
		case c.OK:
			mark = "  yes"
		case !c.Blocking:
			mark = "  --?"
		}
		fmt.Fprintf(out, "%s  %s\n        %s\n", mark, c.Name, c.Detail)
		if c.Blocking && !c.OK {
			blocked++
		}
	}
	if blocked == 0 {
		fmt.Fprintln(out, "\nEvery precondition holds. Nothing has been pressed and "+
			"nothing has been learned: this command only reads.")
		return 0
	}
	fmt.Fprintf(out, "\n%d precondition(s) are false. The experiment does not run, and "+
		"the right response is to fix the precondition rather than to relax a "+
		"guard.\n", blocked)
	return 1
}
