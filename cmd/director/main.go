// Command director is the Director's development front-end: the place where the
// Director's logic and Marco's platform implementations are wired together.
//
// It is deliberately the ONLY package that sees both halves. internal/director
// speaks pkg/directorapi and nothing else; internal/platform implements those
// interfaces over real OS code; this is where the two meet. Keeping the wiring in
// one small main package is what makes the boundary real rather than aspirational —
// and it is enforced by internal/director/boundary_test.go.
//
// Today it serves `inspect`, which builds a World Model from the live desktop and
// reports what the Director can actually perceive. That is not a toy: perception
// quality varies enormously between applications, and being able to ask "what do you
// see in Discord?" before writing a planner is how you find out that the answer is
// "almost nothing" while it is still cheap to react to.
//
//	director inspect                      # the foreground window
//	director inspect --window hwnd:131844 # a specific window
//	director inspect --find Save          # show candidate targets
//	director inspect --repeat 5           # identity stability across snapshots
//
// Build:  go build -o director.exe ./cmd/director
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/internal/platform/uiaclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	// The graph is opened once, before any command runs: every command either reads
	// it or appends to it.
	var gerr error
	if graph, gerr = openGraph(); gerr != nil {
		fmt.Fprintf(os.Stderr, "director: opening the action graph: %v\n", gerr)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "inspect":
		os.Exit(runInspect(os.Args[2:]))
	case "execute":
		os.Exit(runExecute(os.Args[2:]))
	case "graph":
		os.Exit(runGraph(os.Args[2:]))
	case "show":
		os.Exit(runShow(os.Args[2:]))
	case "analyze", "analyse":
		os.Exit(runAnalyze(os.Args[2:]))
	case "history":
		os.Exit(runHistory(os.Args[2:]))
	case "last":
		os.Exit(runLast(os.Args[2:]))
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "explain":
		os.Exit(runExplain(os.Args[2:]))
	case "actions":
		os.Exit(runActions(os.Args[2:]))
	case "goals":
		os.Exit(runGoals(os.Args[2:]))
	case "goal":
		os.Exit(runDryRun(os.Args[2:]))
	case "procedures", "procedure":
		os.Exit(runProcedures(os.Args[2:]))
	case "demonstrate":
		os.Exit(runDemonstrate(os.Args[2:]))
	case "demonstrations":
		os.Exit(runDemonstrations(os.Args[2:]))
	case "demonstration":
		os.Exit(runDemonstration(os.Args[2:]))
	case "extract":
		os.Exit(runExtract(os.Args[2:]))
	case "collections":
		os.Exit(runCollections(os.Args[2:]))
	case "game":
		os.Exit(runGame(os.Args[2:]))
	case "capabilities":
		os.Exit(runCapabilities(os.Args[2:]))
	case "vision":
		os.Exit(runVision(os.Args[2:]))
	case "learned":
		// Shows what Marco has learned well enough to write down, as ordinary Marco.
		// Saves nothing, registers nothing, runs nothing.
		os.Exit(runLearned(os.Args[2:]))
	case "rehearse":
		// The ONLY thing that spends a rehearsal grant, and the only path to learned
		// input. Dry unless --live is given.
		os.Exit(runRehearse(os.Args[2:]))
	case "observe-game":
		os.Exit(runObserveGame(os.Args[2:]))
	// LEARN is the word; `teach` remains a compatibility alias. See cmd/marco/main.go.
	case "learn", "teach":
		// Shows Marco something on purpose. Bounded observation with a conversation around
		// it: it grants no authority, drives no input, and writes nothing until the
		// evidence earns it.
		os.Exit(runLearn(os.Args[2:]))
	case "light":
		// Watch what Light Mode currently understands, live. A read: it starts no
		// session, takes no sample and carries no authority.
		os.Exit(runLight(os.Args[2:]))
	case "reach":
		// How Marco would reach a learned outcome from where the person last stood. A
		// read: a plan over verified edges, or an honest refusal. It performs nothing.
		os.Exit(runReach(os.Args[2:]))
	case "perform":
		// Carries out a learned outcome: fresh Stage, plan over verified edges, then the
		// one walker per edge with verification after each. The only command here whose
		// ordinary result is real input from durable knowledge.
		os.Exit(runPerform(os.Args[2:]))
	case "shadow-trace":
		// Reads back a live $MARCO_SHADOW_TRACE capture. Integrity first, then the raw
		// stream, then interpretation — a trace that cannot account for its skipped slots
		// is refused rather than partially believed.
		os.Exit(runShadowTrace(os.Args[2:]))
	case "shadow-replay":
		// Diagnosis only. Replays recorded detections through track matching to say which
		// layer loses an element's identity; changes nothing and drives no input.
		os.Exit(runShadowReplay(os.Args[2:]))
	case "capture-desktop-sample":
		// One coherent perception moment: the screenshot AND what production believed,
		// pinned to one window. Scratch only — see capturedesktop.go.
		os.Exit(runCaptureDesktopSample(os.Args[2:]))
	case "walk-audit":
		// Counts and times the accessibility walks one window costs, through the
		// production collector. Reads; performs nothing. See walkaudit.go.
		os.Exit(runWalkAudit(os.Args[2:]))
	case "assess-desktop-sample":
		// Says whether the PRIMARY sensor represented the interface, why, and what it
		// cost — the three kept separate. Reads; classifies through production. See
		// assessdesktop.go.
		os.Exit(runAssessDesktopSample(os.Args[2:]))
	case "compare-desktop-perception":
		// Reads one captured moment twice — production fusion and the shadow detector — and
		// counts. Decides nothing; see comparedesktop.go.
		os.Exit(runCompareDesktopPerception(os.Args[2:]))
	case "redact-desktop-sample":
		// Removes personal information from a captured sample, geometrically, in place.
		// Scratch only — see redactdesktop.go.
		os.Exit(runRedactDesktopSample(os.Args[2:]))
	case "capture-vision-fixture":
		// Collects CANDIDATE frames into a scratch directory. It cannot approve anything
		// and cannot write into a corpus — see capturefixture.go.
		os.Exit(runCaptureFixture(os.Args[2:]))
	case "benchmark-vision":
		// --fixture runs the frozen-corpus harness; a bare session id scores a
		// completed observation session. Different questions, same verb.
		if hasFlag(os.Args[2:], "--fixture", "-fixture") {
			os.Exit(runBenchmarkFixture(os.Args[2:]))
		}
		os.Exit(runBenchmarkVision(os.Args[2:]))
	case "observation-sessions":
		os.Exit(runObservationSessions(os.Args[2:]))
	case "observation-session":
		os.Exit(runObservationSession(os.Args[2:]))
	case "observation-insights":
		os.Exit(runObservationInsights(os.Args[2:]))
	case "answer":
		// Answers a question a passive observation session asked. Records semantic
		// feedback and nothing else: no action, no capability, no execution.
		os.Exit(runAnswer(os.Args[2:]))
	case "name-screen":
		// Answers the one question whose answer is the user's own word. Writes a name
		// next to a remembered screen: no action, no capability, no execution.
		os.Exit(runNameScreen(os.Args[2:]))
	case "revise":
		// Changes an answer already given. Separate from `answer`, which stays one-shot.
		os.Exit(runRevise(os.Args[2:], false))
	case "knows":
		os.Exit(runKnows(os.Args[2:]))
	case "name-probe":
		// What this screen says it is, and which of its words is the Place. A read: one
		// collection through the production path, then the production naming rule with its
		// reasoning kept. It establishes nothing.
		os.Exit(runNameProbe(os.Args[2:]))
	case "showing":
		// Which durable place is in front, by its identity. A read: one bounded passive
		// look through the production path, resolved against the real store. It
		// establishes nothing and carries no authority.
		os.Exit(runShowing(os.Args[2:]))
	case "sight":
		// "Show me what you're seeing." A read: no session, no answer, no write.
		os.Exit(runSight(os.Args[2:]))
	case "show-me":
		// Marco points at what it is talking about. Reads only — see pointcmd.go.
		os.Exit(runShowMe(os.Args[2:]))
	case "withdraw":
		// Takes an answer back, durably, so a restart does not resurrect it.
		os.Exit(runRevise(os.Args[2:], true))
	case "cancel-observation":
		os.Exit(runCancelObservation(os.Args[2:]))
	case "windows":
		os.Exit(runWindows(os.Args[2:]))
	case "frames":
		os.Exit(runFrames(os.Args[2:]))
	case "edit":
		os.Exit(runEdit(os.Args[2:]))
	case "plan":
		os.Exit(runPlan(os.Args[2:]))
	case "trace":
		os.Exit(runTrace(os.Args[2:]))
	case "lower":
		os.Exit(runLower(os.Args[2:]))
	case "op":
		os.Exit(runOp(os.Args[2:]))
	case "wait":
		os.Exit(runWait(os.Args[2:]))
	case "visual":
		os.Exit(runVisual(os.Args[2:]))
	case "ocr":
		os.Exit(runOCR(os.Args[2:]))
	case "observations":
		os.Exit(runObservations(os.Args[2:]))
	case "fusion":
		os.Exit(runFusion(os.Args[2:]))
	case "confirm":
		os.Exit(runConfirm(os.Args[2:]))
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	case "stop":
		os.Exit(runStop(os.Args[2:]))
	case "shutdown":
		os.Exit(runShutdown(os.Args[2:]))
	case "reset-test-state":
		// A HARNESS operation, not a product one. Refuses unless MARCO_HOME names a
		// sandbox; see resetGuard.
		os.Exit(runReset(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "director: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `director — Marco's desktop-control layer (development front-end)

  director execute "<request>"  observe, plan, act, verify, and record
  director history [-n N]       what has been done this session
  director last                 the most recent action in full
  director inspect [flags]      build a World Model and report what it perceives
  director observations          which providers observed, and what they produced
  director fusion               what fusion made of the evidence
  director explain [id]          why an element exists, and why alternatives lost
  director explain value <name>  a program-local captured value, while its program lives
  director collections           the active program.s bounded semantic sets
  director explain collection <name>  one collection.s query, ordering and progress
  director ocr [--region]        read the active window and report what was seen
  director visual [--region]     look at a region: appearance, and whether it changed
  director vision [--region]     detect UI shapes by sight, and report what was refused
  director explain vision        what the last vision pass made of what it saw
  director frames                the recent frames read, and what came of each
  director windows               which windows exist now, and how to target one
  director observe-game <sel>    watch a window while you play; emits no input
  director learn "<name>" <sel>  show Marco something on purpose; emits no input
  director light [--debug]       watch what Marco's Accessibility perception understands,
                                 live: the place, what it can act on, what it is learning
  director perform "<outcome>"  carry out something Marco has learned (drives real input)
  director reach ["<outcome>"]   what Marco has learned to reach, and how it would get
                                 there FROM WHERE YOU ARE; plans, never performs
  director observation-session <id>  live progress, or how a session ended
  director observation-insights <id> what was observed, then what it might mean
  director cancel-observation <id>   stop early and keep the evidence
  director wait [--follow]       what the Director is currently waiting FOR
  director edit [-n N]           how recent text edits were carried out, and why
  director actions [name]        the semantic action vocabulary and its capability ladders
  director goals                 the high-level goals the Director understands
  director procedures [name]     how each goal expands, and what it will do
  director explain goal "<req>"  expand a request into its steps WITHOUT running it
  director demonstrate start     record the next task you perform, as SEMANTICS
  director demonstrate stop      end the recording and report what it kept
  director demonstrations        what has been demonstrated
  director demonstration <id>    one recorded session, step by step
  director extract <id>          the procedure it suggests (installs nothing)
  director extract <id> --approve  accept it into the procedure registry
  director explain procedure <n>   why a learned procedure has the shape it has
  director game                  which game is detected, and what may be automated in it
  director capabilities          what each capability pack contributes
  director explain game          why that pack was chosen and the others were not
  director explain inventory     what the Director can see of the player.s holdings
  director explain action        which implementation the last action chose, and what it rejected
  director trace [id|last]       where a command spent its time, phase by phase
  director plan "<request>"      the steps a request becomes (never runs them)
  director lower <op>            the Marco an operation compiles to (never runs it)
  director op <op>               execute one operation through the Marco pipeline
  director confirm [yes|no]      answer the question a running command is waiting on
                                 (with no answer it shows the question and decides nothing)

Requests understood so far:
  click <label>        press a control
  focus <label>        move keyboard focus without activating
  move window left     reposition a window (also: right, centre, other monitor)

execute flags:
  --dry-run           plan and verify without performing any real input
  --json              print the full result, including the trace, as JSON

inspect flags:
  --window hwnd:<n>   scope to a window (default: whatever is in front)
  --find <text>       list elements whose label matches, with their evidence
  --repeat <n>        take n snapshots and report element identity stability
  --accessibility <p> path to the accessibility bridge (default: plugins/uia/uia.exe)
  --max-nodes <n>     node ceiling per snapshot (default 4000)
  --labels            print element labels (off by default: a live window's labels
                      are the user's own content)
`)
}

func runInspect(args []string) int {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	windowFlag := fs.String("window", "", "window to scope to, as hwnd:<handle>")
	findFlag := fs.String("find", "", "list elements whose label matches this text")
	repeatFlag := fs.Int("repeat", 1, "number of snapshots to take")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	maxNodes := fs.Int("max-nodes", 4000, "node ceiling per snapshot")
	showLabels := fs.Bool("labels", false, "print element labels")
	chromeFlag := fs.Bool("chrome", false,
		"attribute every element to page content or viewport chrome, by hierarchy")
	explainFlag := fs.String("explain", "", "explain one element by id, source to replay")
	_ = fs.Parse(args)

	if _, err := os.Stat(*bridgeFlag); err != nil {
		fmt.Fprintf(os.Stderr, "director: accessibility bridge not found at %s\n", *bridgeFlag)
		fmt.Fprintf(os.Stderr, "         build it with: powershell -File plugins/uia/build.ps1\n")
		return 1
	}

	host := bridgehost.New(*bridgeFlag)
	defer host.Close()

	provider := uiaclient.New(host)
	provider.MaxNodes = *maxNodes

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !provider.Available(ctx) {
		fmt.Fprintln(os.Stderr, "director: the accessibility provider reports it is unavailable")
		return 1
	}

	// The same perception pipeline the service runs: a collector producing evidence,
	// an engine producing belief. inspect used to assemble a world itself, which meant
	// it was reporting on a world built by different code from the one commands run
	// against — and would have quietly diverged the moment fusion changed.
	collector := providers.NewCollector(
		providers.NewAccessibility(provider),
		providers.NewWindowSystem(winprovider.New()),
	)
	engine := fusion.NewEngine()

	scope := directorapi.WindowID(*windowFlag)
	req := observation.Request{}
	if scope != "" {
		req.Window = &scope
	}

	var worlds []directorapi.WorldState
	var lastCycle observation.Cycle
	for i := range *repeatFlag {
		started := time.Now()
		cycle := collector.Collect(ctx, req)
		lastCycle = cycle
		elapsed := time.Since(started)

		w, rep, err := engine.Fuse(cycle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: fusing: %v\n", err)
			return 1
		}
		worlds = append(worlds, w)

		if i == 0 || *repeatFlag == 1 {
			report(&w, elapsed, cycle, rep, *showLabels)
		}
		if *chromeFlag {
			reportChrome(measureChrome(cycle))
			reportLineage(cycle)
		}
		if i+1 < *repeatFlag {
			time.Sleep(400 * time.Millisecond)
		}
	}

	if *repeatFlag > 1 {
		reportStability(worlds)
	}
	if *findFlag != "" {
		reportCandidates(&worlds[len(worlds)-1], *findFlag)
	}
	if *explainFlag != "" {
		// The same chain `director explain --chain` prints, against a world this
		// command built itself. Same renderer, so the two cannot drift.
		last := &worlds[len(worlds)-1]
		reportElementChain(last, engine.(fusion.Explainer).Explain(lastCycle), directorapi.ElementID(*explainFlag))
	}
	return 0
}

// report prints what the Director perceives in one snapshot.
func report(w *directorapi.WorldState, elapsed time.Duration, cycle observation.Cycle, rep fusion.Report, showLabels bool) {
	c := w.Confidence
	fmt.Printf("World: %s\n", world.Summarise(w))
	// Printed as five numbers rather than one, because one is what hid the Discord
	// case: impeccable observation quality of a window containing nothing usable.
	fmt.Printf("  confidence   quality %.2f  coverage %.2f  actionable %.2f  durable %.2f  fresh %.2f\n",
		c.ObservationQuality, c.Coverage, c.Actionability, c.IdentityDurability, c.Freshness)
	fmt.Printf("               overall %.2f (limited by the weakest of quality/coverage; "+
		"zero if nothing is operable)\n", c.Overall())
	if c.Shallow() {
		fmt.Printf("  SHALLOW      well observed but never seen into — the application " +
			"is likely not exposing its interior\n")
	}
	if c.Blind() {
		fmt.Printf("  BLIND        nothing in this window can be operated\n")
	}
	// The pipeline as counts: evidence in, belief out, and the cost of the conversion.
	fmt.Printf("  snapshot     %v (%d observations → %d elements, fused in %v)\n",
		elapsed.Round(time.Millisecond), rep.ObservationCount, rep.ElementCount,
		rep.Duration.Round(time.Microsecond))
	if rep.Merged > 0 || len(rep.Conflicts) > 0 {
		fmt.Printf("  fusion       %d merged, %d rejected, %d conflicts\n",
			rep.Merged, rep.Rejected, len(rep.Conflicts))
	}
	for _, d := range cycle.Failures {
		fmt.Printf("  PARTIAL      %s: %s\n", d.Source, d.Reason)
	}

	// Role histogram: the honest measure of how much structure an application
	// actually exposes. An app that is all "pane" is one the Director cannot act in,
	// however many nodes it reports.
	counts := map[directorapi.ElementRole]int{}
	actionable, labelled := 0, 0
	for _, el := range w.Elements {
		counts[el.Role]++
		if el.Actionable() {
			actionable++
		}
		if el.Label != "" {
			labelled++
		}
	}
	type rc struct {
		role directorapi.ElementRole
		n    int
	}
	var roles []rc
	for r, n := range counts {
		roles = append(roles, rc{r, n})
	}
	sort.Slice(roles, func(a, b int) bool {
		if roles[a].n != roles[b].n {
			return roles[a].n > roles[b].n
		}
		return roles[a].role < roles[b].role
	})
	var parts []string
	for _, r := range roles {
		parts = append(parts, fmt.Sprintf("%s=%d", r.role, r.n))
	}
	fmt.Printf("  roles        %s\n", strings.Join(parts, " "))
	fmt.Printf("  labelled     %d/%d    actionable %d/%d\n",
		labelled, len(w.Elements), actionable, len(w.Elements))

	// Named, clickable controls are what a request like "click Save" can actually
	// resolve to. Everything else is scenery.
	targets := 0
	for _, el := range w.Elements {
		if el.Actionable() && el.Label != "" && el.Role.Clickable() {
			targets++
		}
	}
	fmt.Printf("  addressable  %d named clickable controls\n", targets)

	reportIdentityRobustness(observation.Elements(cycle.Observations))

	// Provenance. Element counts alone say how much was seen; this says where any of
	// it CAME FROM, which is the question a second source makes urgent and the one an
	// element cannot answer for itself once its observations are gone.
	if !diagnostics.AllElementsHaveProvenance(w) {
		fmt.Println("  PROVENANCE   some elements cannot say what evidence produced them")
	}
	fmt.Print(diagnostics.RenderProvenance(w, 8))

	if showLabels {
		fmt.Println("  elements:")
		for _, el := range world.Elements(w) {
			fmt.Printf("    %-10s %-28s %-4v %s\n", el.Role, truncate(el.Label, 28),
				el.Enabled, rectStr(el.Bounds))
		}
	}
	fmt.Println()
}

// reportElementChain prints the pipeline one element travelled, source to replay.
func reportElementChain(w *directorapi.WorldState, cx explain.CycleExplanation, id directorapi.ElementID) {
	e, ok := cx.Find(id)
	if !ok {
		fmt.Printf("no explanation for element %s\n", id)
		return
	}
	fmt.Print(explain.RenderChain(e, w.Elements[id]))
}

// reportIdentityRobustness estimates how well element identity would survive if the
// platform's own ids changed.
//
// Re-observing a static window is a weak test of identity: every element matches on
// its RuntimeId and the structural matcher never runs. But RuntimeIds do not survive
// RECREATION — close a dialog and reopen it, switch a tab and switch back, and every
// one of them changes. What carries identity then is an application-authored
// AutomationId, or failing that a label unique enough to match structurally.
//
// So this measures the material identity actually has to work with, per application.
// A window where nothing is uniquely nameable is one where "click that again" will
// silently degrade into "click something like that".
func reportIdentityRobustness(obs []directorapi.Observation) {
	total := len(obs)
	if total == 0 {
		return
	}

	native, withAuto, withLabel, withParent := 0, 0, 0, 0
	autoCounts := map[string]int{}
	labelCounts := map[string]int{}
	for _, o := range obs {
		if o.NativeID != "" {
			native++
		}
		if o.ParentNativeID != "" {
			withParent++
		}
		key := string(o.WindowID) + "\x00" + string(o.Role) + "\x00"
		if a, _ := o.Attributes["automation_id"].(string); a != "" {
			withAuto++
			autoCounts[key+a]++
		}
		if o.Label != "" {
			withLabel++
			labelCounts[key+strings.ToLower(o.Label)]++
		}
	}

	// Only a UNIQUE identifier can carry identity. A duplicated one is worse than
	// none, because it would confidently assign the wrong identity.
	uniqueAuto, uniqueLabel := 0, 0
	for _, o := range obs {
		key := string(o.WindowID) + "\x00" + string(o.Role) + "\x00"
		if a, _ := o.Attributes["automation_id"].(string); a != "" && autoCounts[key+a] == 1 {
			uniqueAuto++
		}
		if o.Label != "" && labelCounts[key+strings.ToLower(o.Label)] == 1 {
			uniqueLabel++
		}
	}

	pct := func(n int) float64 { return 100 * float64(n) / float64(total) }

	// An element is re-identifiable after recreation if it has a unique authored id,
	// or a unique label to anchor structural matching.
	reidentifiable := 0
	for _, o := range obs {
		key := string(o.WindowID) + "\x00" + string(o.Role) + "\x00"
		a, _ := o.Attributes["automation_id"].(string)
		if (a != "" && autoCounts[key+a] == 1) ||
			(o.Label != "" && labelCounts[key+strings.ToLower(o.Label)] == 1) {
			reidentifiable++
		}
	}

	fmt.Printf("  identity     native-id %.0f%%  automation-id %.0f%% (%.0f%% unique)  "+
		"labels %.0f%% (%.0f%% unique)  parent-links %.0f%%\n",
		pct(native), pct(withAuto), pct(uniqueAuto), pct(withLabel), pct(uniqueLabel), pct(withParent))
	fmt.Printf("  durable      %.0f%% would survive a recreation (unique authored id or unique label)\n",
		pct(reidentifiable))
}

// reportStability re-observes and reports how many element identities survived —
// the property that "do that again" and "put it back" depend on. Measured against a
// LIVE application, where the tree genuinely churns, rather than against a fixture.
func reportStability(worlds []directorapi.WorldState) {
	fmt.Printf("Identity stability across %d snapshots:\n", len(worlds))
	base := worlds[0]
	for i := 1; i < len(worlds); i++ {
		kept, lost := 0, 0
		for id := range base.Elements {
			if _, ok := worlds[i].Elements[id]; ok {
				kept++
			} else {
				lost++
			}
		}
		fresh := 0
		for id := range worlds[i].Elements {
			if _, ok := base.Elements[id]; !ok {
				fresh++
			}
		}
		pct := 0.0
		if len(base.Elements) > 0 {
			pct = 100 * float64(kept) / float64(len(base.Elements))
		}
		fmt.Printf("  #%d: %d/%d kept (%.0f%%), %d lost, %d new\n",
			i+1, kept, len(base.Elements), pct, lost, fresh)
	}
	fmt.Println()
}

// reportCandidates runs the real resolver and prints its verdict.
//
// It shows the four-way outcome rather than a found/not-found, because that is the
// distinction the whole design turns on: "I cannot see into this application" and
// "that control is not here" look identical from outside and call for opposite
// responses.
func reportCandidates(w *directorapi.WorldState, text string) {
	res := target.NewResolver().Resolve(w, directorapi.ElementQuery{Label: text})

	fmt.Printf("Resolving %q → %s\n", text, strings.ToUpper(string(res.Status)))
	if res.Explanation != "" {
		fmt.Printf("  %s\n", res.Explanation)
	}
	if res.Blocker != "" {
		fmt.Printf("  blocked by: %s\n", res.Blocker)
	}
	if res.Target != nil {
		if el, ok := w.Element(res.Target.ElementID); ok {
			fmt.Printf("  → %s %q at %s (confidence %.2f)\n",
				el.Role, el.Label, rectStr(el.Bounds), res.Target.Confidence)
		}
	}

	if len(res.Candidates) > 0 {
		fmt.Printf("  considered %d:\n", len(res.Candidates))
		for i, c := range res.Candidates {
			if i >= 8 {
				fmt.Printf("    … and %d more\n", len(res.Candidates)-i)
				break
			}
			note := ""
			if c.Rejected != "" {
				note = "  REJECTED: " + c.Rejected
			}
			fmt.Printf("    %.2f  %-10s %-26s%s\n", c.Score, c.Role, truncate(c.Label, 26), note)
		}
	}

	if !res.Discovery.Empty() {
		fmt.Printf("  discovery (%s, max %d probes):\n", res.Discovery.Risk, res.Discovery.MaxProbes)
		for _, p := range res.Discovery.Probes {
			fmt.Printf("    %.2f  open the %q menu, then look again\n", p.Score, p.Label)
		}
	}
	fmt.Println()
}

func rectStr(r directorapi.Rect) string {
	if r.Empty() {
		return "(no bounds)"
	}
	return fmt.Sprintf("(%d,%d %dx%d)", r.X, r.Y, r.Width, r.Height)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// defaultBridge locates the accessibility plugin relative to the executable, then
// relative to the working directory — so it works both from a build tree and from a
// packaged install.
func defaultBridge() string {
	const rel = "plugins/uia/uia.exe"
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.FromSlash(rel)
}
