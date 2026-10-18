package main

// `director reacquire "<name>"` — can the controls of a saved routine be found again?
//
// # The question, and why it is not the one we were asking
//
//	I wouldn't conclude that screen identity is fundamentally useless. I'd conclude that the
//	existing global screen classifier cannot be trusted as the sole authority for choosing
//	where to act. […] I'd shift the question from "which previously seen screen does this
//	frame resemble?" to "can I find the particular control I was shown, in the expected local
//	context, with enough evidence to act?"
//
// [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] is the reason: seven
// measures now fail to separate a correct screen binding from an incorrect one, and under the
// existing abstention a pixel replay would have acted on 4 of 11 screens and been wrong on 2.
//
// So this asks per CONTROL, through `observe.ReacquireStored` — which never needed a screen —
// and reports the three outcomes that layer has always had:
//
//	matched       one control, agreed on by every supporting proposal
//	ambiguous     more than one could be it, reported and never resolved
//	unobservable  nothing the proposer found supports it
//
// # What a report from this command can and cannot say
//
//	Do not assume local matching is reliable merely because global screen matching failed.
//	Measure false actions, correct reacquisitions, and abstentions.
//
// A match rate cannot say whether a match is RIGHT, and this command is honest about that in the
// only way available: a rate of matches is printed as a rate of matches, and the column that
// would be a correctness claim is left to a person pressing where it points. ADR-180 measured
// ground truth from SECOND PRESSES for exactly this reason and got 3 verified, 0 contradicted,
// 2 declined — a real answer on five data points and not a rate.
//
// So the summary leads with the thing that is checkable without a person: **a control that
// matched in two places at once cannot have been right in both.** Collisions are counted across
// every look, and ADR-180's recording produced zero over 900.
//
// # It cannot act
//
// Nothing here presses anything. `Reacquisition.Actionable()` says COULD, and the authorisation
// is somebody else's to give:
//
//	Do not authorize live replay based on unvalidated visual confidence.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

func runReacquire(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("reacquire", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to watch")
	title := fs.String("window-title", "",
		"narrow to the window whose title contains this")
	seconds := fs.Int("seconds", 30, "how long to watch")
	fps := fs.Int("fps", 10, "observations per second")
	delay := fs.Duration("delay", 5*time.Second, "wait before starting, to switch to the app")
	blind := fs.Bool("blind", false,
		"do not read the words on each look, even when OCR is available")
	_ = fs.Parse(flagsFirst(rest))

	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr,
			`director: say which routine, e.g. director reacquire "enter free play"`)
		return 2
	}
	if *app == "" && *title == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application or --window-title is required")
		return 2
	}
	if *seconds < 1 || *seconds > maxObserveSeconds {
		fmt.Fprintf(os.Stderr, "director: --seconds must be between 1 and %d\n",
			maxObserveSeconds)
		return 2
	}
	if *fps < 1 || *fps > 60 {
		fmt.Fprintln(os.Stderr, "director: --fps must be between 1 and 60")
		return 2
	}

	ctx0 := context.Background()
	store, err := demo.Open(configDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	a, ok := store.Sequence(name)
	if !ok {
		if why := store.SequenceProblem(name); why != "" {
			fmt.Fprintf(os.Stderr, "director: %q cannot be read: %s\n", name, why)
			return 1
		}
		fmt.Fprintf(os.Stderr, "director: nothing called %q has been saved\n", name)
		return 1
	}
	targets := a.VisualTargets()
	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr,
			"director: %q carries no visual control evidence, so there is nothing to\n"+
				"look for by sight. It was demonstrated through the accessibility tree,\n"+
				"where a control is found by its role or its name — run it with:\n"+
				"  director execute \"run %s\"\n", a.Name, a.Name)
		return 1
	}

	interval := time.Second / time.Duration(*fps)
	obs, oerr := fastobserve.New(interval, observeQueue)
	if oerr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", oerr)
		return 2
	}
	// THE WORDS, BESIDE THE LOOP. Here they inform the SCREEN model rather than the
	// per-control match: `ReacquireStored` compares rectangles and nothing else, and
	// wiring a reading into it would be the extra score term the owner ruled out. What
	// they buy is a screen model that can tell two menus apart while this runs, which is
	// the context the abstentions below are measured in.
	var reader *readingEyes
	if !*blind {
		reader = newReadingEyes(ctx0)
		if !reader.working() {
			fmt.Printf("WARNING: the words on each look will NOT be read: %s\n\n",
				reader.why)
		}
	}

	// THE TALLY, over every look, per target.
	tally := map[observe.VisualTargetID]*storedTally{}
	for _, t := range targets {
		tally[t.ID] = &storedTally{}
	}
	looks := 0
	collisions := 0

	s := visualSession{
		app: *app, title: *title, seconds: *seconds, interval: interval, delay: *delay,
		reader: reader,
		announce: func(ref string, settle int) {
			fmt.Printf("Looking for the %d control(s) of %q in %s, at %d/second for %ds\n",
				len(targets), a.Name, ref, *fps, *seconds)
			fmt.Printf("  settle rule %d consecutive looks, derived from the %s interval\n",
				settle, interval)
			fmt.Printf("  Per CONTROL, with no screen bound — see Experiment 076.\n")
			fmt.Printf("  Nothing is pressed. Open the menu you demonstrated.\n")
			fmt.Printf("  Starting in %s.\n\n", *delay)
		},
		// EVERY LOOK, BEFORE IT IS FOLDED. The same ordering the observer's own
		// reacquisition uses and for the same reason: nothing a look teaches may help
		// recognise what that look was meant to recognise.
		each: func(lk fastobserve.Look) {
			if lk.Missing || len(lk.Regions) == 0 {
				return
			}
			looks++
			// WHERE EACH MATCH LANDED THIS LOOK, so two controls matching one
			// place can be counted. A control that matched in two places at once
			// cannot have been right in both, and that is the one correctness
			// statement available without a person pressing anything.
			at := map[observe.VisualTargetID]observe.Region{}
			for _, t := range targets {
				got := observe.ReacquireStored(t, lk.Regions)
				tally[t.ID].fold(got)
				if got.Result == observe.ReacquireMatched {
					at[t.ID] = got.Where
				}
			}
			collisions += collidingPairs(at)
		},
	}
	res, code := s.watch(ctx0, obs, os.Stderr)
	if code != 0 {
		return code
	}
	reportStoredReacquire(a.Name, targets, tally, looks, collisions, res, reader)
	return 0
}

// storedTally is one control's outcomes over a session.
type storedTally struct {
	matched      int
	ambiguous    int
	unobservable int
	// indistinct is the subset of unobservable where a proposal could not be told from a
	// region that merely CONTAINED the demonstrated press. A different refusal from
	// finding nothing, and it sends a reader somewhere else entirely — to the control's
	// own evidence rather than to the screen.
	indistinct int
}

func (s *storedTally) fold(r observe.Reacquisition) {
	switch r.Result {
	case observe.ReacquireMatched:
		s.matched++
	case observe.ReacquireAmbiguous:
		s.ambiguous++
	default:
		s.unobservable++
		if r.Indistinct > 0 {
			s.indistinct++
		}
	}
}

// collidingPairs counts pairs of distinct controls that matched the same rectangle.
//
// The one check that can call a match WRONG without a person: two controls cannot both be the
// thing in one place. ADR-180 measured zero such pairs over 900 looks on its recording, which is
// the number this is here to contradict if it ever stops being zero.
func collidingPairs(at map[observe.VisualTargetID]observe.Region) int {
	ids := make([]observe.VisualTargetID, 0, len(at))
	for id := range at {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	n := 0
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if observe.SameRegion(at[ids[i]], at[ids[j]]) {
				n++
			}
		}
	}
	return n
}

// reportStoredReacquire prints the per-control tally.
func reportStoredReacquire(name string, targets []observe.StoredTarget,
	tally map[observe.VisualTargetID]*storedTally, looks, collisions int, res visualResult,
	reader *readingEyes) {

	fmt.Printf("%s\n", strings.Repeat("-", 74))
	fmt.Printf("CAN THE CONTROLS OF %q BE FOUND AGAIN?\n\n", name)
	if res.lost != "" {
		fmt.Printf("  stopped early: %s\n\n", res.lost)
	}
	if looks == 0 {
		fmt.Printf("  NO LOOK PRODUCED A PROPOSAL. The window was missing or the proposer\n")
		fmt.Printf("  found nothing in it, so this says nothing about the controls.\n")
		return
	}
	fmt.Printf("  %d usable look(s)\n", looks)
	// WHAT THE READER DID. An abstention rate means something different depending on
	// whether the screen model could tell two menus apart while this ran.
	fmt.Printf("  words: %s\n\n", reader.describe())
	fmt.Printf("  %-14s %8s %10s %13s %s\n",
		"CONTROL", "matched", "ambiguous", "unobservable", "of those, indistinct")
	var anyMatched int
	for _, t := range targets {
		s := tally[t.ID]
		if s.matched > 0 {
			anyMatched++
		}
		fmt.Printf("  %-14s %7d%% %9d%% %12d%% %d\n", t.ID,
			pctOf(s.matched, looks), pctOf(s.ambiguous, looks),
			pctOf(s.unobservable, looks), s.indistinct)
	}
	fmt.Printf("\n  %d of %d control(s) were found on at least one look.\n",
		anyMatched, len(targets))

	// THE ONE CORRECTNESS STATEMENT AVAILABLE HERE. A rate of matches is not a rate of
	// right answers, and saying so is the whole of this paragraph.
	fmt.Printf("\n  colliding matches: %d\n", collisions)
	switch {
	case collisions > 0:
		fmt.Printf("    AT LEAST %d WRONG. Two controls matched one rectangle on the same\n",
			collisions)
		fmt.Printf("    look, and they cannot both be the thing in that place.\n")
	default:
		fmt.Printf("    none — no two controls claimed one rectangle. That is the only\n")
		fmt.Printf("    correctness statement this command can make on its own: a match\n")
		fmt.Printf("    rate is not a rate of right answers, and a person pressing where\n")
		fmt.Printf("    a match points is what would settle it.\n")
	}
	fmt.Printf("\n  ambiguous and unobservable are ABSTENTIONS, not errors. A control that\n")
	fmt.Printf("  cannot be told from its container is reported as indistinct rather than\n")
	fmt.Printf("  as missing, because the two send a reader to different places.\n")
	fmt.Printf("\n  Nothing was pressed. A match is not permission.\n")
}

// pctOf is a whole-number percentage of n out of total.
//
// Named for its arguments rather than `pct`, which selectionread.go already has: two helpers
// with one name in one package is a compile error, and the one that moved is the newer.
func pctOf(n, total int) int {
	if total == 0 {
		return 0
	}
	return n * 100 / total
}
