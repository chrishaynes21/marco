package observe_test

// WHAT A PRESS DID, WHERE THE SCREEN DID NOT CHANGE.
//
// Four terms, each its own test, because each one was added for a measured reason and a single
// "the rule works" test would let any of them be deleted:
//
//	positive      words appeared that were not read before
//	persistent    on every reading of the settled window, so an animation is not an outcome
//	independent   not from inside the control that was pressed
//	stationary    the screen does not do this by itself
//
// The numbers in the comments are from `perceptionreplay -frames -outcome` on the owner's Ark
// capture and on `rl-freeplay-1`. See [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// reading is one look's positioned words, laid out on a tab bar and the panel under it.
//
// The geometry is Ark's, measured: the tab bar at 94.6%x5.0% from y 0.230, and the panel body
// below it. [[Experiment-085-the-graphics-tab-is-a-button]].
var tabBar = observe.Region{X: 0.027, Y: 0.230, Width: 0.946, Height: 0.050}

func inBar(words ...string) []observe.ReadWord {
	out := make([]observe.ReadWord, 0, len(words))
	for i, w := range words {
		out = append(out, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.05 + float64(i)*0.09, Y: 0.245, Width: 0.07, Height: 0.02}})
	}
	return out
}

func inPanel(words ...string) []observe.ReadWord {
	out := make([]observe.ReadWord, 0, len(words))
	for i, w := range words {
		out = append(out, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.10, Y: 0.35 + float64(i)*0.04, Width: 0.20, Height: 0.03}})
	}
	return out
}

// window is N readings of the same content, which is what a settled panel produces.
func window(n int, read []observe.ReadWord) [][]observe.ReadWord {
	out := make([][]observe.ReadWord, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, read)
	}
	return out
}

// join is a bar reading and a panel reading on one look.
func join(a, b []observe.ReadWord) []observe.ReadWord {
	return append(append([]observe.ReadWord{}, a...), b...)
}

// THE POSITIVE TERM: the panel's new words are the outcome, and the tab bar's are not.
func TestWhatAppearedElsewhereAndStayedIsTheOutcome(t *testing.T) {
	bar := inBar("audio", "video", "graphics", "camera")
	before := window(5, join(bar, inPanel("resolution", "window", "mode", "upscaler", "vsync")))
	after := window(5, join(bar, inPanel("presets", "textures", "shadows", "bloom", "foliage")))

	o := observe.OutcomeBetween([]observe.Region{tabBar}, before, after)
	if !o.Verified() {
		t.Fatalf("the panel changed completely and the outcome was not verified: %s\n%+v",
			o.Why(), o)
	}
	for _, want := range []string{"presets", "textures", "shadows", "bloom", "foliage"} {
		if !has(o.Appeared, want) {
			t.Errorf("%q is in the new panel and not in the outcome: %v", want,
				o.Appeared)
		}
	}
	// AND THE PANEL THAT WENT AWAY IS REPORTED, never required: a panel that merely
	// ADDED something makes Appeared true and Vanished empty.
	if !has(o.Vanished, "resolution") {
		t.Errorf("the words that stopped being read are %v", o.Vanished)
	}
	// AND THE TAB BAR CONTRIBUTES NOTHING, because it is inside the excluded region.
	for _, w := range o.Appeared {
		if w == "graphics" || w == "audio" {
			t.Errorf("%q was read inside the control and reached the outcome", w)
		}
	}
}

// THE PERSISTENT TERM: an animation frame is not an outcome.
//
// The case the owner named. Ark's capture has exactly one such look — 250, where the tab bar is
// partly rendered and the body is missing — and a verifier that accepted it would accept every
// frame of a cross-fade.
func TestAWordOnSomeLooksAndNotAllIsTransientAndNotAnOutcome(t *testing.T) {
	bar := inBar("audio", "video")
	before := window(5, join(bar, inPanel("resolution", "window", "mode")))
	// FOUR READINGS OF THE NEW PANEL and one that also caught a fading title.
	after := window(4, join(bar, inPanel("presets", "textures", "shadows", "bloom", "foliage")))
	after = append(after, join(bar, inPanel("presets", "textures", "shadows", "bloom",
		"foliage", "loading")))

	o := observe.OutcomeBetween([]observe.Region{tabBar}, before, after)
	if has(o.Appeared, "loading") {
		t.Errorf("a word read on one look of five is in the outcome: %v", o.Appeared)
	}
	if !has(o.Transient, "loading") {
		t.Errorf("a word read on one look of five is not counted as transient: %v",
			o.Transient)
	}
	// AND THE REST OF THE PANEL STILL VERIFIES. One transient word does not spoil an
	// outcome; it is simply not part of it.
	if !o.Verified() {
		t.Errorf("a transient word alongside five persistent ones refused the "+
			"outcome: %s", o.Why())
	}
}

// AND A WINDOW WHERE EVERYTHING IS TRANSIENT IS REFUSED, with that reason.
//
// Ark's pause menu, measured: everything that appeared after the press was a clock and a ping
// counter — 17:02, 49.9, 702, 99ms. A verifier that accepted those would verify every frame of
// a running game.
func TestAClockAndACounterAreNotAnOutcome(t *testing.T) {
	before := window(5, inPanel("resume", "settings", "exit"))
	after := [][]observe.ReadWord{
		inPanel("resume", "settings", "exit", "1701", "993ms"),
		inPanel("resume", "settings", "exit", "1702", "421ms"),
		inPanel("resume", "settings", "exit", "1703", "887ms"),
		inPanel("resume", "settings", "exit", "1704", "112ms"),
		inPanel("resume", "settings", "exit", "1705", "564ms"),
	}
	o := observe.OutcomeBetween(nil, before, after)
	if o.Verified() {
		t.Errorf("a clock and a counter verified an outcome: %v", o.Appeared)
	}
	if len(o.Appeared) != 0 {
		t.Errorf("a value that is different every look appeared and stayed: %v",
			o.Appeared)
	}
	if !strings.Contains(o.Why(), "transient") {
		t.Errorf("the refusal does not say why: %s", o.Why())
	}
}

// THE SPARSE-READING TERM: absence from the before window is not absence from the screen.
//
// The defect Rocket League exposed. Its capture reads on 300 looks of 900, so a word plainly on
// the screen is read on one before-reading in three — and a rule that only asked for "fewer
// than the stable floor" counted it as new. 473 of 550 windows with no input reported an
// outcome. A word has to be read on NO before-reading at all.
func TestAWordReadOnceBeforeIsNotAWordThatAppeared(t *testing.T) {
	// FIVE READINGS, and `shadows` was caught on exactly one of them.
	before := [][]observe.ReadWord{
		inPanel("presets", "textures"),
		inPanel("presets", "textures", "shadows"),
		inPanel("presets", "textures"),
		inPanel("presets", "textures"),
		inPanel("presets", "textures"),
	}
	after := window(5, inPanel("presets", "textures", "shadows", "bloom", "foliage",
		"epic", "medium"))

	o := observe.OutcomeBetween(nil, before, after)
	if has(o.Appeared, "shadows") {
		t.Errorf("a word read before the action is reported as having appeared: %v",
			o.Appeared)
	}
	// AND THE WORDS THAT WERE GENUINELY ABSENT ARE STILL THERE.
	for _, want := range []string{"bloom", "foliage", "epic", "medium"} {
		if !has(o.Appeared, want) {
			t.Errorf("%q was absent before and is not in the outcome: %v", want,
				o.Appeared)
		}
	}
}

// THE READING-COUNT TERM: one reading on a side cannot say that anything stayed.
func TestNothingIsConcludedFromOneReadingASide(t *testing.T) {
	before := window(1, inPanel("resolution", "window"))
	after := window(1, inPanel("presets", "textures", "shadows", "bloom", "foliage"))

	o := observe.OutcomeBetween(nil, before, after)
	// FIVE WORDS APPEARED, and on one reading "stayed" means nothing. The persistence
	// term is satisfied trivially by a window of one, so the floor on readings is what
	// refuses this — and the caller is the one that enforces it, from BeforeLooks and
	// AfterLooks, which is why they are on the result.
	if o.BeforeLooks != 1 || o.AfterLooks != 1 {
		t.Fatalf("the result does not report its own evidence: %d before, %d after",
			o.BeforeLooks, o.AfterLooks)
	}
	// AND A WINDOW WITH NOTHING IN IT IS NOT A REFUSAL, it is an unasked question.
	none := observe.OutcomeBetween(nil, nil, after)
	if none.Verified() {
		t.Error("an outcome was verified with nothing read before the action")
	}
	if !strings.Contains(none.Why(), "one side") {
		t.Errorf("the reason does not say the question was not asked: %s", none.Why())
	}
}

// THE STATIONARY TERM: a screen that does this by itself has no outcomes worth reading.
//
// Measured per press. Ark's settings panel: 0 of 54 windows with no action in them produced an
// outcome. Rocket League's main menu: 2 to 12 of every 25 to 64, on eleven of seventeen presses
// — including the one press that would otherwise have verified falsely.
func TestAScreenThatChangesByItselfIsNotStationary(t *testing.T) {
	// A PANEL THAT HOLDS STILL across four windows of readings.
	still := [][][]observe.ReadWord{
		window(5, inPanel("presets", "textures", "shadows")),
		window(5, inPanel("presets", "textures", "shadows")),
		window(5, inPanel("presets", "textures", "shadows")),
	}
	quiet, moved := observe.Stationary(nil, still)
	if quiet == 0 {
		t.Fatal("no window was examined, so the test asserts nothing")
	}
	if moved != 0 {
		t.Errorf("a panel that read the same words every time moved %d of %d times",
			moved, quiet)
	}

	// AND A MENU WITH A ROTATING PANEL IN IT, which is Rocket League's: a news strip
	// that swaps a headline and its lines every few seconds, so each window of readings
	// brings more new words than an outcome needs.
	rotating := [][][]observe.ReadWord{
		window(5, inPanel("play", "garage", "profile", "tournament", "season",
			"rank", "placement", "matches", "ends", "soon")),
		window(5, inPanel("play", "garage", "profile", "challenge", "weekly",
			"rewards", "complete", "three", "games", "today")),
		window(5, inPanel("play", "garage", "profile", "shop", "featured",
			"bundle", "limited", "offer", "credits", "discount")),
	}
	quiet, moved = observe.Stationary(nil, rotating)
	if moved == 0 {
		t.Errorf("a menu whose panel rotates every window moved 0 of %d times, so "+
			"the term that refuses Rocket League's main menu does nothing", quiet)
	}
	// AND IT IS BOTH WINDOWS, not one unlucky pair: the panel rotates every time.
	if moved != quiet {
		t.Errorf("%d of %d windows moved on a panel that changes every window",
			moved, quiet)
	}
}

func has(ws []string, w string) bool {
	for _, x := range ws {
		if x == w {
			return true
		}
	}
	return false
}

// ── AND THE WHOLE THING THROUGH THE REAL CORRELATOR ──
//
// Every test above drives `OutcomeBetween` directly, which proves the RULE and not the WIRING.
// A mutation run found the difference: deleting the call that opens a watch, the call that
// feeds it, and the call that hands the results out all survived, because nothing here entered
// through `Press` and `Reading`.
//
// Eight recorded cases in this repository of complete code production never invoked. This is
// the test that stops the outcome verifier being the ninth.
func TestTheCorrelatorRecordsWhatAPressDid(t *testing.T) {
	var c observe.ClickTargets
	bar := inBar("audio", "video", "graphics", "camera")
	was := inPanel("resolution", "window", "mode", "upscaler", "vsync")
	now := inPanel("presets", "textures", "shadows", "bloom", "foliage", "distance")

	// FIVE READINGS OF THE SCREEN AS IT WAS, through the production entry point.
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", join(bar, was))
	}
	// THE PRESS, on the GRAPHICS tab inside the tab bar.
	obs := c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	if obs.Target == "" {
		t.Fatalf("the press resolved to no control, so there is no outcome to record")
	}
	// AND FIVE READINGS OF WHAT STOOD AFTERWARDS.
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", join(bar, now))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	o := outs[0]
	if o.Target != obs.Target || o.Screen != "visual_1" || o.Look != 10 {
		t.Errorf("the outcome is filed against %q on %q at look %d",
			o.Target, o.Screen, o.Look)
	}
	if !o.Verified() {
		t.Fatalf("the panel changed completely and the correlator did not record it: "+
			"%s\n%+v", o.Why(), o)
	}
	for _, want := range []string{"presets", "textures", "shadows", "bloom", "foliage"} {
		if !has(o.Appeared, want) {
			t.Errorf("%q is in the new panel and not in the outcome: %v", want,
				o.Appeared)
		}
	}
	// AND NOTHING FROM INSIDE THE TAB BAR, which is where the press was approved.
	for _, w := range []string{"graphics", "audio", "video", "camera"} {
		if has(o.Appeared, w) {
			t.Errorf("%q was read inside the pressed control and reached the "+
				"outcome", w)
		}
	}
}

// AND A READING OF ANOTHER SCREEN IS NOT THIS PRESS'S OUTCOME.
//
// The watch is scoped to the screen that was standing. Without that, any later reading of
// anywhere would fill in the after side, and a press would be credited with whatever Marco
// happened to look at next.
func TestAnotherScreensReadingsAreNotThisPressesOutcome(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// FIVE READINGS OF A DIFFERENT SCREEN ENTIRELY.
	for i := 0; i < 5; i++ {
		c.Reading("visual_2", inPanel("presets", "textures", "shadows", "bloom",
			"foliage", "distance"))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if outs[0].Verified() {
		t.Errorf("another screen's readings verified this press: %v", outs[0].Appeared)
	}
	if outs[0].AfterLooks != 0 {
		t.Errorf("%d reading(s) of another screen were folded into this press's "+
			"outcome", outs[0].AfterLooks)
	}
}

// A WORD INSIDE THE PRESSED CONTROL CANNOT VERIFY ITS OWN PRESS.
//
// The independence term, with the fixture that actually exercises it — which is Ark's own case
// and the reason the term exists. A word read inside the control on BOTH sides never reaches
// the outcome anyway, because it was read before; the term only bites when the control's word
// turns up somewhere ELSE afterwards.
//
// On Ark the GRAPHICS tab is inside the tab bar, and the panel it opens says `Advanced
// Graphics` and `Graphics Presets`. So `graphics` is excluded before (it was only in the bar)
// and present after (it is in the panel), which makes it look like the press's outcome — and
// it is the one word the press's own approval rested on.
//
// A mutation run found this unguarded: deleting the filter survived every other test here.
func TestTheControlsOwnWordCannotBeItsOutcome(t *testing.T) {
	var c observe.ClickTargets
	bar := inBar("audio", "video", "graphics", "camera")
	// BEFORE: the tab bar says `graphics`, and the panel below does not.
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", join(bar, inPanel("resolution", "window", "mode")))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// AFTER: the panel says `graphics` too, in `Advanced Graphics` and `Graphics
	// Presets`, beside five words that are genuinely new.
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", join(bar, inPanel("graphics", "presets", "advanced",
			"textures", "shadows", "bloom", "foliage")))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	o := outs[0]
	if has(o.Appeared, "graphics") {
		t.Errorf("the word inside the pressed control is part of its own outcome: %v",
			o.Appeared)
	}
	if !has(o.Reused, "graphics") {
		t.Errorf("the word is not counted as the action site's own: %v", o.Reused)
	}
	// AND THE REST STILL VERIFIES. The term filters the evidence and does not veto the
	// verdict: refusing on one shared word cost 43 good ones when it did.
	if !o.Verified() {
		t.Errorf("one shared word refused an outcome with five independent ones: %s",
			o.Why())
	}
	for _, want := range []string{"presets", "textures", "shadows", "bloom", "foliage"} {
		if !has(o.Appeared, want) {
			t.Errorf("%q is independent of the control and not in the outcome: %v",
				want, o.Appeared)
		}
	}
}
