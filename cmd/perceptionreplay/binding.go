package main

// CAN A SCREEN LEARNED IN ONE OBSERVATION SESSION BE RECOGNISED IN ANOTHER?
//
// # The question, and why it comes before any store
//
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] proposes a durable workflow that
// carries the ARRANGEMENT of each screen it names, so a later session can work out where it is
// without inheriting a session's own counters. The whole design rests on an assumption nobody
// had tested: that an arrangement — a count of rectangles in nine boxes — identifies the same
// screen when a different session looked at it. The owner named it as the prerequisite:
//
//	Take the existing recording, split it into independent folds, and determine whether screen
//	arrangements learned from one segment can be recognized in another without sharing
//	session-local IDs. […] Importantly, don't allow the two folds to share the registry or
//	screen identities.
//
// So: three completely separate observers over the same pixels. Two folds that each see half the
// recording and nothing of each other, and one ORACLE over the whole thing whose only job is to
// say which screens are really the same. Nothing is shared between the folds — not a counter, not
// a registry, not a signature.
//
// # The caveat belongs in the report, not under it
//
// The owner stated it before the experiment ran, and it is repeated in the output rather than
// left to a reader:
//
//	splitting a single recording creates independent folds, not genuinely independent
//	application sessions. It can validate the binding mechanism, but not prove robustness across
//	restarts, UI changes, or display configurations.
//
// A fold has the same resolution, the same window, the same build of the game and the same
// theme. What it does test is whether the representation survives a different observer arriving
// at a different moment with a different history — which is the part the mechanism controls.
//
// Nothing here promotes, stores, or binds anything durably. It measures and prints.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// foldScreen is one screen as one fold knew it.
type foldScreen struct {
	id    observe.VisualScreenID
	arr   observe.VisualArrangement
	looks int
	// proposals is the mean number of rectangles the proposer offered on the looks this
	// fold called this screen. Carried because the owner asked whether identities survive
	// a change in proposal count, and a fold that saw a screen at a different yield is the
	// only way to answer it from one recording.
	proposals float64
	// truth is the ORACLE's name for this screen: the id the whole-recording observer gave
	// to most of the looks this fold called this screen.
	//
	// The oracle is the same stabiliser run with continuity, which is the honest limit of
	// this ground truth: it says two folds are looking at what one uninterrupted observer
	// would have called one screen. It is not an independent labelling, and no independent
	// one exists without a person.
	truth observe.VisualScreenID
}

// bindingOutcome is one attempt to recognise a fold-A screen among fold B's.
type bindingOutcome struct {
	from      foldScreen
	best      foldScreen
	bestSim   float64
	second    observe.VisualScreenID
	secondSim float64
	// result is one of bound / ambiguous / unobservable.
	result string
	// correct is whether the bound screen is the one the oracle says it is. Only
	// meaningful when the result is bound AND the oracle knows of a counterpart.
	correct bool
	// counterpart is whether fold B saw this screen at all. A screen with no counterpart
	// MUST NOT bind, and binding one is a false positive rather than a miss.
	counterpart bool
}

// reportScreenBinding folds the recording three times and tries to bind one fold's screens to
// the other's by arrangement alone.
func reportScreenBinding(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) < 10 {
		return fmt.Errorf("%s carries %d frames, too few to split", dir, len(frames))
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	// THE PROPOSALS, COMPUTED ONCE. The three folds must see identical pixels and
	// identical rectangles, or a difference in binding could be a difference in
	// segmentation and nobody could tell which.
	regions := make([][]observe.Region, len(frames))
	for i, fr := range frames {
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			continue
		}
		res := structure.Propose(img, structure.DefaultOptions())
		rects := make([]image.Rectangle, 0, len(res.Candidates))
		for _, c := range res.Candidates {
			rects = append(rects, c.Bounds)
		}
		regions[i] = observe.NormaliseRects(img.Bounds(), rects)
	}

	half := len(frames) / 2
	oracle, err := foldRange(frames, regions, 0, len(frames))
	if err != nil {
		return err
	}
	a, err := foldRange(frames, regions, 0, half)
	if err != nil {
		return err
	}
	b, err := foldRange(frames, regions, half, len(frames))
	if err != nil {
		return err
	}

	screensA := summarise(a, oracle, regions)
	screensB := summarise(b, oracle, regions)
	out := bind(screensA, screensB)
	printBinding(dir, len(frames), half, screensA, screensB, out)
	return nil
}

// fold is one observer's whole record over a range of looks.
type fold struct {
	// at maps a look index (0-based into frames) to the screen standing after that look
	// was folded.
	at map[int]observe.VisualScreenID
	// arr is each screen's stored arrangement, read from the observer that minted it.
	arr map[observe.VisualScreenID]observe.VisualArrangement
}

// foldRange runs ONE fresh observer over one slice of the recording.
//
// Fresh is the point: a new `fastobserve.Observer` has no screens, no targets and a counter that
// starts at one, so `visual_1` in one fold and `visual_1` in another are two unrelated names that
// happen to collide. Nothing is passed between them.
func foldRange(frames []frameRecord, regions [][]observe.Region, lo, hi int) (fold, error) {
	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return fold{}, err
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()

	f := fold{at: map[int]observe.VisualScreenID{},
		arr: map[observe.VisualScreenID]observe.VisualArrangement{}}
	base := time.Unix(1700000000, 0).UTC()
	for i := lo; i < hi; i++ {
		fr := frames[i]
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front:   true,
			Regions: regions[i],
		}
		if len(regions[i]) == 0 {
			lk.Missing = true
		}
		obs.Observe(lk)
		f.at[i] = obs.Screen()
	}
	for _, s := range obs.Screens() {
		f.arr[s.ID] = obs.Arrangement(s.ID)
	}
	return f, nil
}

// summarise turns one fold into its screens, with the oracle's name for each.
func summarise(f fold, oracle fold, regions [][]observe.Region) []foldScreen {
	type acc struct {
		looks     int
		proposals int
		truth     map[observe.VisualScreenID]int
	}
	by := map[observe.VisualScreenID]*acc{}
	for look, id := range f.at {
		if id == "" || id == observe.VisualScreenUnknown ||
			id == observe.VisualScreenUnobserved {
			continue
		}
		a, ok := by[id]
		if !ok {
			a = &acc{truth: map[observe.VisualScreenID]int{}}
			by[id] = a
		}
		a.looks++
		a.proposals += len(regions[look])
		// THE ORACLE'S VERDICT ON THE SAME LOOK, which is what makes two folds'
		// screens comparable without either of them knowing about the other.
		if t := oracle.at[look]; t != "" && t != observe.VisualScreenUnknown &&
			t != observe.VisualScreenUnobserved {
			a.truth[t]++
		}
	}
	var out []foldScreen
	for id, a := range by {
		fs := foldScreen{id: id, arr: f.arr[id], looks: a.looks}
		if a.looks > 0 {
			fs.proposals = float64(a.proposals) / float64(a.looks)
		}
		best := 0
		for t, n := range a.truth {
			if n > best || (n == best && t < fs.truth) {
				fs.truth, best = t, n
			}
		}
		out = append(out, fs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].looks > out[j].looks })
	return out
}

// bind tries to recognise each of one fold's screens among the other's, by arrangement only.
//
// The relation is the production one — `ArrangementSimilarity` against `VisualScreenMatch` — so
// this measures what the screen model already believes rather than a rule invented for the
// experiment. The one thing added is the SECOND-best score, because production has no abstention:
// `promote` takes the best match above the floor and never reports that two were close. A durable
// binding would have to, and this is where that becomes visible.
func bind(from, to []foldScreen) []bindingOutcome {
	var out []bindingOutcome
	for _, s := range from {
		o := bindingOutcome{from: s, result: "unobservable"}
		for _, t := range to {
			if t.truth == s.truth && s.truth != "" {
				o.counterpart = true
			}
			sim := observe.ArrangementSimilarity(s.arr, t.arr)
			switch {
			case sim > o.bestSim:
				o.second, o.secondSim = o.best.id, o.bestSim
				o.best, o.bestSim = t, sim
			case sim > o.secondSim:
				o.second, o.secondSim = t.id, sim
			}
		}
		switch {
		case o.bestSim >= observe.VisualScreenMatch &&
			o.secondSim >= observe.VisualScreenMatch:
			o.result = "ambiguous"
		case o.bestSim >= observe.VisualScreenMatch:
			o.result = "bound"
			o.correct = o.best.truth == s.truth && s.truth != ""
		}
		out = append(out, o)
	}
	return out
}

// scores prints a sorted list of similarities compactly.
func scores(v []float64) string {
	if len(v) == 0 {
		return "(none)"
	}
	out := make([]string, 0, len(v))
	for _, x := range v {
		out = append(out, fmt.Sprintf("%.3f", x))
	}
	return strings.Join(out, " ")
}

func printBinding(dir string, frames, half int, a, b []foldScreen, out []bindingOutcome) {
	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("BINDING A SCREEN ACROSS TWO INDEPENDENT FOLDS — %s\n", filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A: looks 1-%d      fold B: looks %d-%d\n", half, half+1, frames)
	fmt.Printf("  three separate observers; no counter, registry or signature is shared.\n")
	fmt.Printf("  the relation is production's own: ArrangementSimilarity >= %.2f\n",
		observe.VisualScreenMatch)
	fmt.Printf("\n  A FOLD IS NOT A SESSION. Same resolution, same window, same build, same\n")
	fmt.Printf("  theme. This measures whether the representation survives a different\n")
	fmt.Printf("  observer with a different history — not a restart, a patch or a monitor.\n")

	show := func(name string, s []foldScreen) {
		fmt.Printf("\n  %s saw %d screen(s):\n", name, len(s))
		fmt.Printf("    %-10s %6s %10s  %s\n", "id", "looks", "proposals", "the oracle calls it")
		for _, x := range s {
			fmt.Printf("    %-10s %6d %10.0f  %s\n", x.id, x.looks, x.proposals, x.truth)
		}
	}
	show("fold A", a)
	show("fold B", b)

	fmt.Printf("\n  binding each of fold A's screens among fold B's:\n")
	fmt.Printf("    %-10s %-12s %-10s %6s %6s  %s\n",
		"A", "oracle", "result", "best", "2nd", "bound to")
	var bound, correct, wrong, ambiguous, unobservable, falsePositive, missed int
	var right, badly []float64
	for _, o := range out {
		note := ""
		switch o.result {
		case "bound":
			bound++
			switch {
			case o.correct:
				correct++
				right = append(right, o.bestSim)
				note = "CORRECT"
			case !o.counterpart:
				falsePositive++
				badly = append(badly, o.bestSim)
				note = "WRONG — and fold B never saw this screen at all"
			default:
				wrong++
				badly = append(badly, o.bestSim)
				note = "WRONG — bound to a different screen"
			}
		case "ambiguous":
			ambiguous++
			note = "two candidates above the floor; production would have taken the best"
			if o.counterpart {
				note += " (a counterpart exists)"
			}
		default:
			unobservable++
			if o.counterpart {
				missed++
				note = "MISSED — fold B saw this screen and nothing matched"
			} else {
				note = "correctly unbound: fold B never saw it"
			}
		}
		fmt.Printf("    %-10s %-12s %-10s %6.3f %6.3f  %-10s %s\n",
			o.from.id, o.from.truth, o.result, o.bestSim, o.secondSim,
			o.best.id, note)
	}
	fmt.Printf("\n  %d screen(s) of fold A:\n", len(out))
	fmt.Printf("      %d bound: %d CORRECT, %d WRONG (%d of those to a screen fold B\n",
		bound, correct, wrong+falsePositive, falsePositive)
	fmt.Printf("        never saw at all, where the only right answer was to refuse)\n")
	fmt.Printf("      %d ambiguous — two candidates above the floor, which production\n",
		ambiguous)
	fmt.Printf("        resolves silently by taking the best\n")
	fmt.Printf("      %d unobservable, of which %d were screens fold B DID see\n",
		unobservable, missed)

	// CAN A SCORE TELL THEM APART? The only question that decides whether a floor is a
	// usable safeguard, asked of the numbers rather than hoped for.
	fmt.Printf("\n  could a higher floor have separated them?\n")
	sort.Float64s(right)
	sort.Float64s(badly)
	fmt.Printf("      correct bindings scored %s\n", scores(right))
	fmt.Printf("      wrong   bindings scored %s\n", scores(badly))
	switch {
	case len(badly) == 0:
		fmt.Printf("      no wrong binding to separate.\n")
	case len(right) == 0:
		fmt.Printf("      no correct binding to keep.\n")
	case badly[len(badly)-1] < right[0]:
		fmt.Printf("      YES: every wrong binding scored below every correct one.\n")
	default:
		beaten := 0
		for _, r := range right {
			if r < badly[len(badly)-1] {
				beaten++
			}
		}
		fmt.Printf("      NO. The best WRONG binding scored %.3f, above %d of the %d\n",
			badly[len(badly)-1], beaten, len(right))
		fmt.Printf("      correct ones. A floor that excluded it would throw those away\n")
		fmt.Printf("      too, so the score does not carry the distinction and raising\n")
		fmt.Printf("      the threshold cannot make it carry it.\n")
	}
	fmt.Printf("\n  A WRONG BINDING IS THE EXPENSIVE ONE. A miss says \"I do not know where I\n")
	fmt.Printf("  am\"; a wrong binding performs a saved workflow on the wrong screen.\n")
}
