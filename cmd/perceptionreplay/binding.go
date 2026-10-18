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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// foldScreen is one screen as one fold knew it.
type foldScreen struct {
	id    observe.VisualScreenID
	arr   observe.VisualArrangement
	looks int
	// said is what this screen was READ to say, from the same fold that minted it.
	//
	// Collected by `fold` from the beginning and never consulted by `bind`, which is
	// what this experiment's second half is about: the two wrong bindings it found were
	// not ambiguous, they were UNOPPOSED, and a score cannot separate unopposed from
	// right. A second term that is not a function of the first can.
	said observe.ScreenText
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

	// ── the second term ──
	//
	// What the WORDS say about the binding the arrangement proposed. Reported beside the
	// score rather than folded into it, because the question is whether text carries a
	// distinction the score does not -- and a combined number would hide the answer.
	//
	// textSim is 0 and bothRead false whenever either screen went unread, which is the
	// honest case and not a disagreement: a screen nobody read can never contradict
	// anything, and treating silence as agreement is the mistake observe's own
	// ScreenText.Read exists to prevent.
	bothRead bool
	textSim  float64
	// offered is how many candidates were put forward for this saved screen. The
	// denominator of the one rule `BindScreens` already acts on: one qualifier of many
	// offered is "the only candidate the evidence supports", and whether THAT is enough
	// is the question this experiment's second half exists to answer.
	offered int
	// disagree is observe.TextsDisagree on the two readings: a positive statement that
	// these are not the same screen, from evidence the arrangement never saw.
	disagree bool
	disWhy   string
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
	regions, readable := proposeAll(dir, frames)
	pressAt := pressesByLook(dir, frames)
	// AND WHAT EACH LOOK SAID, from the cache beside the recording. Optional: a run with
	// no reading answers the text question with "nothing was read", which is a fact about
	// the recording rather than about the hypothesis.
	words, werr := readWords(dir, frames, 0, nil)
	if werr != nil {
		fmt.Printf("  words: none (%v)\n", werr)
	}

	half := len(frames) / 2
	oracle, err := foldRange(frames, regions, readable, words, pressAt, 0, len(frames))
	if err != nil {
		return err
	}
	a, err := foldRange(frames, regions, readable, words, pressAt, 0, half)
	if err != nil {
		return err
	}
	b, err := foldRange(frames, regions, readable, words, pressAt, half, len(frames))
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
	// said is what each screen was READ to say, from the same observer.
	//
	// Collected beside the arrangement because an artifact saved without it is an
	// artifact whose destinations can never be corroborated by a reading, and
	// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured
	// what that costs: the arrangement alone establishes the wrong page on 10 of
	// 34 looks of the page a step starts on. Empty for a fold nobody read, which
	// `SaveWith` writes as unread rather than as silent.
	said map[observe.VisualScreenID]observe.ScreenText
	// walk, clicks and registry are what this fold DEMONSTRATED, taken from the observer
	// before it is closed.
	//
	// Carried because a fold that learns a workflow has to hand the whole demonstration
	// over, and `workflow.Learn` wants all three: the walk is the ORDER, the evidence is
	// the edges, and the registry is what each control has been seen as.
	walk     []observe.VisualScreenID
	clicks   observe.ClickEvidence
	registry []observe.TargetRecord
}

// foldRange runs ONE fresh observer over one slice of the recording.
//
// Fresh is the point: a new `fastobserve.Observer` has no screens, no targets and a counter that
// starts at one, so `visual_1` in one fold and `visual_1` in another are two unrelated names that
// happen to collide. Nothing is passed between them.
// `words` is what each look was READ to say, by index, and may be nil or short -- a recording
// nobody ran OCR over produces folds whose screens are unread, which is the honest record and
// not a claim that they were silent.
// `presses` is each look's mouse presses, by look index, and may be nil.
//
// Nil for the folds that only need screens, which is what every caller wanted until a
// measurement needed the CONTROLS a fold demonstrated. Without them a fold identifies no
// control, every screen change comes back `unknown`, and a rule with a control term abstains on
// every look -- which reads as a finding about the rule and is a fact about the harness. It cost
// one round of believing the corpus could not answer the question.
func foldRange(frames []frameRecord, regions [][]observe.Region, readable []bool,
	words []observe.ScreenText, presses [][]observe.PointerAt, lo, hi int) (fold, error) {
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
		arr:  map[observe.VisualScreenID]observe.VisualArrangement{},
		said: map[observe.VisualScreenID]observe.ScreenText{}}
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
		// MISSING MEANS NO PIXELS, not no proposals. A black frame is a
		// look that saw nothing, not one nobody could take.
		if !readable[i] {
			lk.Missing = true
		}
		// AND WHAT IT SAID, synchronously. The delayed-arrival path semantic.go
		// exercises is a question about when a reading lands; this one is about
		// whether the words separate two screens at all, and mixing the two would
		// leave a failure attributable to either.
		if i < len(words) && words[i].Read() {
			lk.Text = words[i]
		}
		if i < len(presses) {
			lk.Presses = presses[i]
		}
		obs.Observe(lk)
		f.at[i] = obs.Screen()
	}
	for _, s := range obs.Screens() {
		f.arr[s.ID] = obs.Arrangement(s.ID)
		if t := obs.SaidBy(s.ID); t.Read() {
			f.said[s.ID] = t
		}
	}
	f.walk = obs.Walk()
	f.clicks = obs.Clicks()
	f.registry = obs.Registry()
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
		fs := foldScreen{id: id, arr: f.arr[id], said: f.said[id], looks: a.looks}
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
		o := bindingOutcome{from: s, result: "unobservable", offered: len(to)}
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
		// AND WHAT THE WORDS SAY ABOUT IT, computed for the binding the arrangement
		// chose and reported separately. Nothing here changes the binding: the point
		// is to find out whether this term would have, before anything depends on it.
		if s.said.Read() && o.best.said.Read() {
			o.bothRead = true
			o.textSim = observe.TextSimilarity(s.said, o.best.said)
			o.disagree, o.disWhy = observe.TextsDisagree(s.said, o.best.said)
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
	printTextTerm(out)
	printMargin(out)
}

// printTextTerm asks whether the WORDS carry the distinction the score does not.
//
// Reported as its own section because it answers a different question from everything above.
// The score section asks whether a better threshold exists; this asks whether a second term
// exists. Experiment 029's two wrong bindings were not ambiguous — they were UNOPPOSED, bound to
// a screen the other fold had never seen — and no ranking of one measure can reject a candidate
// that nothing competes with. Only evidence the first measure never saw can.
func printTextTerm(out []bindingOutcome) {
	fmt.Printf("\n  and would the WORDS have separated them?\n")
	var read, unread int
	for _, o := range out {
		if o.result != "bound" {
			continue
		}
		if o.bothRead {
			read++
		} else {
			unread++
		}
	}
	if read == 0 {
		fmt.Printf("      NOTHING WAS READ. %d binding(s) were made and no screen on either\n",
			unread)
		fmt.Printf("      side carries a reading, so this question cannot be answered from\n")
		fmt.Printf("      this run. That is a fact about the recording and not about the\n")
		fmt.Printf("      hypothesis: a screen nobody read can never contradict anything,\n")
		fmt.Printf("      and treating silence as agreement is the mistake ScreenText.Read\n")
		fmt.Printf("      exists to prevent.\n")
		return
	}
	fmt.Printf("      A          oracle       result   arr     text   words\n")
	for _, o := range out {
		if o.result != "bound" {
			continue
		}
		verdict := "CORRECT"
		if !o.correct {
			verdict = "WRONG  "
		}
		words := "both read"
		switch {
		case !o.bothRead:
			words = "one side unread -- no statement"
		case o.disagree:
			words = "DISAGREE: " + o.disWhy
		}
		fmt.Printf("      %-10s %-12s %-8s %.3f  %.3f   %s\n",
			o.from.id, o.from.truth, verdict, o.bestSim, o.textSim, words)
	}
	// THE ONE NUMBER THAT MATTERS: does a text disagreement reject the wrong bindings
	// without rejecting the right ones? Anything less is a measure that costs more than
	// it buys.
	var wrongRejected, wrongKept, rightRejected, rightKept int
	for _, o := range out {
		if o.result != "bound" || !o.bothRead {
			continue
		}
		switch {
		case !o.correct && o.disagree:
			wrongRejected++
		case !o.correct:
			wrongKept++
		case o.disagree:
			rightRejected++
		default:
			rightKept++
		}
	}
	fmt.Printf("\n      of the bindings where both sides were read:\n")
	fmt.Printf("        wrong, and the words reject it    %d\n", wrongRejected)
	fmt.Printf("        wrong, and the words allow it     %d\n", wrongKept)
	fmt.Printf("        correct, and the words reject it  %d   <- the cost\n", rightRejected)
	fmt.Printf("        correct, and the words allow it   %d\n", rightKept)
	if unread > 0 {
		fmt.Printf("        unreadable on one side            %d   <- no statement either way\n",
			unread)
	}
}

// printMargin asks whether the GAP to the runner-up separates right from wrong.
//
// The third candidate discriminator, after the score and the words, and the last one available
// from what the artifact carries. A confident binding might be one that beat its nearest rival
// by a wide margin — "the only candidate, and nothing else came close" — which is a different
// claim from "the highest score" and is not implied by it.
func printMargin(out []bindingOutcome) {
	fmt.Printf("\n  and would the MARGIN to the runner-up have separated them?\n")
	var right, badly []float64
	for _, o := range out {
		if o.result != "bound" {
			continue
		}
		gap := o.bestSim - o.secondSim
		if o.correct {
			right = append(right, gap)
		} else {
			badly = append(badly, gap)
		}
	}
	sort.Float64s(right)
	sort.Float64s(badly)
	fmt.Printf("      correct bindings had gaps %s\n", scores(right))
	fmt.Printf("      wrong   bindings had gaps %s\n", scores(badly))
	switch {
	case len(badly) == 0 || len(right) == 0:
		fmt.Printf("      not enough of both to say.\n")
	case badly[len(badly)-1] < right[0]:
		fmt.Printf("      YES: every wrong binding had a narrower margin than every\n")
		fmt.Printf("      correct one. A margin floor would separate them here.\n")
	default:
		beaten := 0
		for _, r := range right {
			if r < badly[len(badly)-1] {
				beaten++
			}
		}
		fmt.Printf("      NO. The widest WRONG margin was %.3f, wider than %d of the %d\n",
			badly[len(badly)-1], beaten, len(right))
		fmt.Printf("      correct ones. Being unopposed is not evidence of being right.\n")
	}
	// AND HOW MANY CANDIDATES QUALIFIED AT ALL, which is the fourth thing one might
	// hope separates them and the one `BindScreens` already acts on.
	fmt.Printf("\n      how many candidates qualified, for each binding that was made:\n")
	for _, o := range out {
		if o.result != "bound" {
			continue
		}
		verdict := "CORRECT"
		if !o.correct {
			verdict = "WRONG  "
		}
		qualified := 1
		if o.secondSim >= observe.VisualScreenMatch {
			qualified = 2
		}
		fmt.Printf("        %-10s %s  %d qualified of %d offered\n",
			o.from.id, verdict, qualified, o.offered)
	}
}

// pressesByLook is each look's mouse presses, read from the recording's input log.
//
// One statement of it, because two harnesses now need it and they must agree: a fold that was
// fed presses and one that was not are different observers of the same pixels, and a difference
// between them would be attributed to whatever the measurement was about.
//
// Only POINT intents. A key or a controller button is navigation and carries no position, so it
// cannot identify a control — and treating one as a press would attribute a screen change to a
// rectangle nobody clicked.
func pressesByLook(dir string, frames []frameRecord) [][]observe.PointerAt {
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })
	out := make([][]observe.PointerAt, len(frames))
	next := 0
	for i, fr := range frames {
		for next < len(inputs) && inputs[next].AtMS <= fr.AtMS {
			in := inputs[next]
			next++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				out[i] = append(out[i], in.Where)
			}
		}
	}
	return out
}
