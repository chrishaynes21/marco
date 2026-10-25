//go:build windows

package main

// Command e2e drives the real Studio against a real application on a real desktop.
//
// # What this is for
//
// Studio's flow was testable in two places and provable in neither. The browser tests render the
// real page with the Go bindings STUBBED, so they hold what the page does and nothing about what
// the backend does with it. The backend tests drive the real backend with the recorder and the
// desktop REPLACED, so they hold what the backend does and nothing about whether a button is
// wired to it, enabled when it should be, or on a surface somebody can reach. Between the two sat
// the whole product path — page, binding, backend, library, generated Marco, child executor,
// native input, window guard, run record — and the only thing that ever exercised it was a person
// working through E2E.md by hand.
//
// So this runs it. One command, no hands, and a result that is not a person's recollection.
//
// # Two layers, and they are labelled apart on purpose
//
//	DESKTOP        a SEEDED routine, played through the real Studio UI, asserted against
//	               what the application actually received.
//	RECORDING      draft, review, Save, Record again and replacement, through the backend
//	               with a scripted recorder. Not on this desktop and not through this window.
//
// And a third line that is not a layer but a limit:
//
//	PHYSICAL RECORDING CAPTURE — NOT EXERCISED.
//
// Marco's recorder discards injected input. Both hooks test the injected flag and drop what they
// find, on purpose, because attributing Marco's own presses to a person would let a replay be
// learned as a demonstration. There is no switch for it and there must not be one. So no
// automated run on this machine can prove that Windows captured a physical gesture, and this says
// so as its own line in every report rather than leaving somebody to infer it from the absence of
// a claim. A person demonstrating something in Studio remains the only evidence of that, and
// E2E.md remains where it is written down.
//
// # What it will not do
//
//	It never renames, moves or rebuilds a binary in the tree.
//	It never reads or writes the owner's store or library.
//	It never closes a window it did not open, and refuses rather than ending somebody
//	  else's fixture.
//	It sends no input. The only presses in a run are Marco's own, through `marco do`.
//	It stops rather than pressing when the foreground is not a window it owns.
//
// Usage:
//
//	plugins\e2e\run.cmd              every scenario
//	plugins\e2e\run.cmd -only play   the scenarios whose names contain "play"
//	plugins\e2e\run.cmd -keep        leave the store and bundle behind

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
)

func main() {
	only := flag.String("only", "", "run only the scenarios whose names contain this")
	keep := flag.Bool("keep", false,
		"leave this run's store and binaries behind for inspection")
	flag.Parse()

	started := time.Now()
	run, err := newRunDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		os.Exit(3)
	}
	art, err := newArtifacts(filepath.Join(run, "artifacts"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		os.Exit(3)
	}
	defer art.close()
	// WHAT THIS PROCESS LOGS GOES TO THE ARTIFACTS, NOT TO THE TERMINAL.
	//
	// The harness calls production code in its own process — seeding goes through
	// `orchestrator.Save`, which logs — and those lines are evidence worth keeping and
	// noise in a summary somebody is reading for a verdict. Redirected rather than
	// silenced: a seeded routine's save line says where the file went, which is the first
	// thing anybody wants when a scenario cannot find it.
	//
	// Only THIS process. Nothing is put in the environment, so Studio and the `marco` it
	// spawns log exactly as they would for a person — their words are the run's own
	// account and are not the harness's to turn down.
	if f, err := os.Create(art.path("harness.log")); err == nil {
		mlog.Default = mlog.New(f, mlog.LevelInfo)
		defer f.Close()
	}
	fmt.Printf("marco e2e — artifacts in %s\n\n", art.dir)

	var checks []string
	m, err := checkDesktop()
	if err != nil {
		finish(art, m, checks, nil, err, started, run, *keep)
		return
	}
	// THE STORE, AND THE RULE THAT FINDS IT, SET BEFORE ANYTHING READS A ROUTE.
	//
	// One store for this run: the harness seeds into it, Studio is pointed at it, and the
	// `marco` Studio spawns inherits it. Said through the environment rather than by joining
	// paths in three places, because that IS the production rule — and a library resolved two
	// ways is how a routine saved in one place came to be invisible in another.
	store := storeOf(run)
	if err := os.MkdirAll(store, 0o755); err != nil {
		finish(art, m, checks, nil, err, started, run, *keep)
		return
	}
	os.Setenv(marcopaths.EnvHome, store)
	os.Unsetenv(marcopaths.EnvRoutes)
	art.step("store: %s", store)
	art.step("library: %s", routesDir())

	b, err := makeBundle(m, filepath.Join(run, "bundle"), art)
	if err != nil {
		finish(art, m, checks, nil, err, started, run, *keep)
		return
	}

	// THE HARNESS'S OWN PRECONDITION, BEFORE ANY SCENARIO LEANS ON IT.
	//
	// Four scenarios below assert that something did NOT arrive, and every one of them reads
	// the fixture's log after the sender stopped. Whether that read is an account of what
	// arrived or only of what the fixture had got round to depends entirely on the drain
	// barrier — so the barrier is demonstrated against a fixture that is genuinely behind
	// before the first absence is asserted, and the run does not continue if it cannot be.
	//
	// Not a scenario: it is a claim about this harness, and the layer totals are claims about
	// Marco. See checkTheBarrier.
	checked, err := checkTheBarrier(b, art)
	if err != nil {
		finish(art, m, checks, nil, err, started, run, *keep)
		return
	}
	checks = append(checks, checked)
	fmt.Printf("%s\n", checked)

	var results []result
	// ONE WORLD, AND A NUMBER PER SCENARIO.
	//
	// The number is what keeps each scenario's store, oracles and artifacts apart. A world
	// built fresh per scenario had the counter reset with it, so every scenario used store 1
	// and read the previous one's files — and the first symptom was four refusals in a row
	// about a window that had already closed.
	w := &world{b: b, art: art, store: store, machine: m}
	for i, s := range scenarios() {
		if *only != "" && !strings.Contains(s.name, *only) {
			continue
		}
		// THE SELECTED SET IS WHAT `results` HOLDS, which is what makes a selection
		// that matched nothing distinguishable from a run where everything passed.
		fmt.Printf("\n%s — %s\n", s.name, s.about)
		w.scenarioN = i + 1
		w.art = art.scope(fmt.Sprintf("%02d-%s", i+1, slug(s.name)))
		r := s.run(w)
		r.name, r.layer, r.about = s.name, s.layer, s.about
		results = append(results, r)
		if r.skipped {
			fmt.Printf("  NOT RUN: %s\n", r.why)
			continue
		}
		if r.passed {
			fmt.Printf("  passed in %s\n", r.took.Round(time.Millisecond))
			continue
		}
		fmt.Printf("  FAILED: %s\n", r.why)
	}
	finish(art, m, checks, results, nil, started, run, *keep)
}

// world is what a scenario is given: this run's binaries, store and machine.
//
// No state of its own and nothing shared between scenarios. Each one opens its own windows and
// closes them, because a scenario that inherited a window would be testing whatever the one
// before it left behind — and the first thing a desktop harness has to be is repeatable.
type world struct {
	b       *bundle
	art     *artifacts
	store   string
	machine machine
	// scenarioN numbers the stores, so each scenario gets a library of its own.
	scenarioN int
}

// scenario is one thing this harness can establish.
type scenario struct {
	name  string
	layer string
	about string
	run   func(*world) result
}

// result is what a scenario established, or why it did not.
type result struct {
	name     string
	layer    string
	about    string
	passed   bool
	skipped  bool
	why      string
	evidence []string
	took     time.Duration
}

// pass and fail are the two honest endings, and skip is the third that is neither.
func pass(started time.Time, evidence ...string) result {
	return result{passed: true, evidence: evidence, took: time.Since(started)}
}

func fail(started time.Time, why string, evidence ...string) result {
	return result{why: why, evidence: evidence, took: time.Since(started)}
}

func skip(started time.Time, why string) result {
	return result{skipped: true, why: why, took: time.Since(started)}
}

// ending turns an error into the right kind of ending: a refusal is NOT RUN, anything else failed.
func ending(started time.Time, err error, evidence ...string) result {
	var nr notRun
	if asNotRun(err, &nr) {
		return skip(started, nr.why)
	}
	return fail(started, err.Error(), evidence...)
}

func asNotRun(err error, out *notRun) bool {
	if err == nil {
		return false
	}
	if nr, ok := err.(notRun); ok {
		*out = nr
		return true
	}
	return false
}

// newRunDir is a fresh directory for this run, outside the repository and outside the store.
func newRunDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "marco-e2e", time.Now().Format("20060102-150405"))
	return dir, os.MkdirAll(dir, 0o755)
}

func finish(art *artifacts, m machine, checks []string, results []result, err error,
	started time.Time, run string, keep bool) {

	code := report(art, m, checks, results, err, started)
	if !keep {
		// THE STORE GOES, THE ARTIFACTS STAY. The store held a library and some run
		// records, and both have been copied into the artifacts if anything needed them;
		// leaving it behind would leave a Marco library on the machine that nothing owns.
		_ = os.RemoveAll(storeOf(run))
		_ = os.RemoveAll(filepath.Join(run, "bundle"))
	}
	art.close()
	os.Exit(code)
}

// slug is a name as a directory can hold it.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
