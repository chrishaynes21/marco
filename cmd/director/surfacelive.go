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
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

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
		return runSurfaceLearn(os.Stdout, *phrase, *bridgeFlag, *maxNodes, watchFor(*watch))
	case *reground:
		return runSurfaceReground(os.Stdout, *phrase, *bridgeFlag, *maxNodes)
	case *exec:
		return runSurfaceExecute(os.Stdout, *phrase, *bridgeFlag, *maxNodes, *authorise)
	}
	return surfacePreconditions(os.Stdout, *phrase, *bridgeFlag, *maxNodes, *watch)
}

// surfacePreconditions runs every check and reports.
//
// Split from the command for the reason every harness in this tree is: the flags are not the
// thing worth testing and the refusals are.
func surfacePreconditions(out io.Writer, phrase, bridge string, maxNodes, watch int) int {
	var checks []surfaceCheck
	add := func(c surfaceCheck) { checks = append(checks, c) }

	// ── 1. THE SANDBOX, AND THE CONFIGURATION IT UNLOCKS ──
	//
	// The experiment's own configuration, enabled here so that the Stage diagnosed below
	// is the Stage the learning phase will see. Diagnosing production defaults and then
	// learning under a different configuration would be a check about the wrong screen.
	add(surfaceSandboxCheck(os.Getenv("MARCO_HOME"), defaultHome()))
	if err := enablePixelOnly(os.Getenv("MARCO_HOME"), defaultHome()); err != nil {
		add(surfaceCheck{Name: "pixel-only test configuration", Blocking: true,
			Detail: err.Error()})
	} else {
		add(surfaceCheck{Name: "pixel-only test configuration", OK: true,
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
	if pixelOnly != nil && accessible {
		return surfaceCheck{Name: "registered observation providers", Blocking: true,
			Detail: fmt.Sprintf("%v — accessibility is registered under the "+
				"pixel-only configuration, so a fall back to it is possible",
				names)}
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
				"experiment has something to demonstrate."}
	}
	// AND WHY NOT, which is the prerequisite report rather than a failure of this command.
	structural := 0
	for _, el := range world.Elements {
		if el == nil {
			continue
		}
		for _, ref := range el.Provenance.Sources {
			if ref.Source == directorapi.SourceStructure {
				structural++
				break
			}
		}
	}
	return surfaceCheck{Name: "the instruction against the live Stage", Blocking: true,
		Detail: detail + fmt.Sprintf("\n        NO SURFACE QUESTION WOULD BE ASKED. "+
			"%d of %d Props were described by the shape proposer. A question "+
			"arises only where identity and interaction surface come apart, "+
			"which is where nothing but pixels describes the control — and no "+
			"production observation request includes SourceStructure.",
			structural, len(world.Elements))}
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
