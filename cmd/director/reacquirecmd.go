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
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
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
			`director: say which play, e.g. director reacquire "enter free play"`)
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
	// readLooks is how many looks carried a reading the observer would admit, and `fresh`
	// what each matched rectangle has been read to say over those looks.
	readLooks := 0
	fresh := map[observe.VisualTargetID]*observe.FreshWords{}
	for _, t := range targets {
		fresh[t.ID] = &observe.FreshWords{}
	}

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
			// A LOOK THAT CANNOT HOST A READING IS NOT EVIDENCE ABOUT THE WORDS.
			//
			// `intakeReadings` refuses a reading on a look whose window is not in
			// FRONT, because window capture is not occlusion-safe: a covered window
			// returns the covering window's pixels under the target's name, and a
			// covered window's words are another application's.
			//
			// The first version of this corroborated from readings the observer had
			// refused — it asked `Admissible`, which checks the window generation and
			// the observation epoch and not usability — and the live run said so in
			// its own output: `37 offered, 0 admitted, 37 on a look that could host
			// none`, printed beside a confident "corroborated" built from 28 of them.
			// Corroborating against the terminal's pixels is worse than not
			// corroborating at all.
			usable := lk.Front
			// THE WORDS THIS LOOK CARRIES, if the observer would admit them.
			//
			// Asked with the observer's own rule rather than a second one:
			// `Admissible` is the method `intakeReadings` calls, with the same two
			// arguments, one step earlier in the same look. A reading whose window
			// generation or observation epoch has moved describes a screen that has
			// gone, and corroborating a match against it would be corroborating
			// against the last menu.
			//
			// Counted separately from what the observer admitted, because OCR
			// producing words and the observer accepting them are two different
			// facts and the report shows both.
			var words []observe.ReadWord
			for _, r := range lk.Readings {
				if !usable || !r.Admissible(lk.Window.Generation, obs.Epoch()) {
					continue
				}
				if len(r.Words) > 0 {
					words = r.Words
				}
			}
			if len(words) > 0 {
				readLooks++
			}

			// WHERE EACH MATCH LANDED THIS LOOK, so two controls matching one
			// place can be counted. A control that matched in two places at once
			// cannot have been right in both, and that is the one correctness
			// statement available without a person pressing anything.
			at := map[observe.VisualTargetID]observe.Region{}
			for _, t := range targets {
				got := observe.ReacquireStored(t, lk.Regions)
				tally[t.ID].fold(got)
				if got.Result != observe.ReacquireMatched {
					continue
				}
				at[t.ID] = got.Where
				// AND WHAT THE MATCHED RECTANGLE SAYS, from the words inside
				// IT rather than from the screen or from the container.
				// `LabelOf` is what named the control when it was
				// demonstrated, so the question is asked the way the answer
				// was recorded.
				if len(words) > 0 {
					fresh[t.ID].Saw(observe.LabelOf(got.Where, words))
				}
			}
			collisions += collidingPairs(at)
		},
	}
	res, code := s.watch(ctx0, obs, os.Stderr)
	if code != 0 {
		return code
	}
	reportStoredReacquire(a, targets, tally, looks, collisions, readLooks, fresh, res,
		reader, obs)
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
func reportStoredReacquire(a workflowstore.Artifact, targets []observe.StoredTarget,
	tally map[observe.VisualTargetID]*storedTally, looks, collisions, readLooks int,
	said map[observe.VisualTargetID]*observe.FreshWords, res visualResult,
	reader *readingEyes, obs *fastobserve.Observer) {

	name := a.Name

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
	// WHAT OCR PRODUCED, which is a different fact from what the observer admitted —
	// printed first so the pair reads as a chain.
	fmt.Printf("  words: %s\n", reader.describe())
	// WHAT THE OBSERVER DID WITH THE READINGS, which is a different fact from what OCR
	// produced and is the distinction that could not be made in Experiment 080: the
	// screens carried no text though thirty readings had been delivered, and the report
	// said only how many reached the loop.
	st := obs.Stats()
	fmt.Printf("  readings: %d offered, %d admitted", st.ReadingsOffered, st.ReadingsAdmitted)
	if st.ReadingsStaleEpoch > 0 || st.ReadingsStaleWindow > 0 {
		fmt.Printf(", %d stale (epoch %d, window %d)",
			st.ReadingsStaleEpoch+st.ReadingsStaleWindow,
			st.ReadingsStaleEpoch, st.ReadingsStaleWindow)
	}
	if st.ReadingsSilent > 0 {
		fmt.Printf(", %d read nothing", st.ReadingsSilent)
	}
	if st.ReadingsRefusedMerge > 0 {
		fmt.Printf(", %d refused a merge", st.ReadingsRefusedMerge)
	}
	if st.ReadingsUnusable > 0 {
		fmt.Printf(", %d on a look that could host none", st.ReadingsUnusable)
	}
	fmt.Printf("\n  %d of %d look(s) carried a reading this report could use\n\n",
		readLooks, looks)

	fmt.Printf("  %-14s %8s %10s %13s %10s %s\n",
		"CONTROL", "matched", "ambiguous", "unobservable", "indistinct", "words")
	var anyMatched, corroborated, contradicted int
	for _, t := range targets {
		s := tally[t.ID]
		if s.matched > 0 {
			anyMatched++
		}
		// THE FOURTH COLUMN, and it decides nothing. The match above was made by
		// rectangle and is unchanged; this says what it was worth.
		verdict, why := said[t.ID].Against(savedWords(a, t.ID))
		switch verdict {
		case observe.TextCorroborated:
			corroborated++
		case observe.TextContradicted:
			contradicted++
		}
		fmt.Printf("  %-14s %7d%% %9d%% %12d%% %10d %s\n", t.ID,
			pctOf(s.matched, looks), pctOf(s.ambiguous, looks),
			pctOf(s.unobservable, looks), s.indistinct, verdict)
		fmt.Printf("  %-14s %s\n", "", why)
	}
	fmt.Printf("\n  %d of %d control(s) were found on at least one look.\n",
		anyMatched, len(targets))
	fmt.Printf("  %d text-corroborated, %d text-contradicted, %d unread.\n",
		corroborated, contradicted, len(targets)-corroborated-contradicted)

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

// savedWords is what one control of a saved routine was read to say when it was demonstrated.
//
// From the ARTIFACT rather than from the StoredTarget, because `StoredTarget` carries the
// rectangles a match is made from and deliberately not the words: the matcher compares geometry,
// and handing it text would make the comparison depend on a reading that may not have arrived.
// The words are evidence ABOUT the match, fetched where they live.
func savedWords(a workflowstore.Artifact, id observe.VisualTargetID) []string {
	for _, c := range a.Controls {
		if c.Label == string(id) {
			return c.Text
		}
	}
	return nil
}
