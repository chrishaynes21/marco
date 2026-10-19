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
	"strconv"
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

// A PRESS THAT CHANGED THE SCREEN STILL HAS AN OUTCOME.
//
// This asserted the opposite, and the opposite cost three live demonstrations. The watch was
// scoped to the screen the press was made ON, so every reading of the thing the press PRODUCED
// was discarded for being a reading of the thing the press produced.
//
// Invisible offline, which is why it survived: on `ark-menu-1` the GRAPHICS tab changes the
// panel and not the global screen ([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]),
// so the watch's screen still matched and 36 words were recorded. Measured live on the same
// game and the same control, the panel changed enough to mint a new screen and the same press
// recorded nothing at all.
func TestAPressThatChangedTheScreenStillHasAnOutcome(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// THE SCREEN THE PRESS LED TO, which is where its outcome is.
	for i := 0; i < 8; i++ {
		c.Reading("visual_2", inPanel("presets", "textures", "shadows", "bloom",
			"foliage", "distance"))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if !outs[0].Verified() {
		t.Fatalf("a press that changed the screen recorded no outcome: %s",
			outs[0].Why())
	}
	for _, want := range []string{"presets", "textures", "shadows", "bloom", "foliage"} {
		if !has(outs[0].Appeared, want) {
			t.Errorf("%q is in the panel the press produced and not in the "+
				"outcome: %v", want, outs[0].Appeared)
		}
	}
}

// AND A THIRD SCREEN IS NOT THIS PRESS'S OUTCOME.
//
// The part of the old scoping that was right, kept at the level its own stated fear belongs to:
// "any later reading of anywhere would fill in the after side, and a press would be credited
// with whatever Marco happened to look at next." Where the press landed, or the one place it
// led — a third place closes the watch.
func TestAThirdScreenIsNotThisPressesOutcome(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// THE SKIP, then ONE reading of where it led, then somewhere else entirely.
	for i := 0; i < 3; i++ {
		c.Reading("visual_2", inPanel("presets", "textures"))
	}
	for i := 0; i < 8; i++ {
		c.Reading("visual_9", inPanel("inventory", "crafting", "engrams",
			"tribe", "tames", "beacons"))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	for _, w := range []string{"inventory", "crafting", "engrams", "tribe", "tames"} {
		if has(outs[0].Appeared, w) {
			t.Fatalf("%q came from a third screen and was credited to this "+
				"press: %v", w, outs[0].Appeared)
		}
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

// A PRESS THE SCREEN NEVER ANSWERED IS REPORTED, AND `Close` IS WHAT DOES IT.
//
// Worth a test because the author spent a round of this work convinced it was broken.
// `ClickTargets.Close` calls `resolve(nil)`, which files every press still pending — so a press
// followed by no screen change for the rest of the session has always come out as inert, and
// the code added to `Evidence` to "fix" it was reverted. The diagnosis was wrong and the
// mutation gate is what said so: the replacement survived every test here, because `Close` had
// already done the work.
func TestAPressTheScreenNeverAnsweredIsInertAtTheClose(t *testing.T) {
	var c observe.ClickTargets
	row := []observe.Region{{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05}}
	c.Press(10, observe.PointerAt{X: 0.45, Y: 0.42}, "visual_1",
		observe.Proposed(row, 0, 0, 0))
	c.Press(14, observe.PointerAt{X: 0.88, Y: 0.70}, "visual_1", nil)

	// BEFORE THE CLOSE the presses are pending, which is right: either could still be
	// attributed to a change that has not happened yet.
	if ev := c.Evidence(); len(ev.Inert) != 0 {
		t.Errorf("a press that could still be attributed is reported inert: %+v",
			ev.Inert)
	}
	c.Close()

	ev := c.Evidence()
	if len(ev.Inert) != 1 {
		t.Fatalf("%d inert press(es) after the close, want the one that landed",
			len(ev.Inert))
	}
	if ev.Inert[0].Look != 10 {
		t.Errorf("the reported press is %+v", ev.Inert[0])
	}
	// AND THE ONE NO PROPOSAL CONTAINED IS KEPT AS ITSELF, never as inert.
	if len(ev.BlindPresses) != 1 || ev.Blind != 1 {
		t.Errorf("%d blind press(es) kept of %d counted", len(ev.BlindPresses),
			ev.Blind)
	}
}

// THE MID-RESPONSE READINGS ARE NOT EVIDENCE, AND INCLUDING THEM INVERTS THE ANSWER.
//
// The defect that made the live correlator answer 0 of 3 where the offline measurement answered
// 2 of 3, on the same recording. The persistence term wants a word on EVERY reading of the
// settled window, and on the readings straight after a press the new panel is not drawn yet —
// so every word of it appears on three readings of five and falls out as transient.
//
// See observe.OutcomeSkipReadings for why the amount is two and how it was measured.
func TestTheReadingsTakenMidResponseAreNotEvidence(t *testing.T) {
	var c observe.ClickTargets
	bar := inBar("audio", "video", "graphics")
	was := inPanel("resolution", "window", "mode")
	// WHAT A HALF-DRAWN FRAME READS AS: the bar, and none of the panel.
	drawing := bar
	now := inPanel("presets", "textures", "shadows", "bloom", "foliage")

	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", join(bar, was))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// THE TRANSITION, then the panel standing.
	for i := 0; i < observe.OutcomeSkipReadings; i++ {
		c.Reading("visual_1", drawing)
	}
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", join(bar, now))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if !outs[0].Verified() {
		t.Fatalf("the panel stood for %d readings after the transition and the "+
			"outcome was not verified: %s\n  transient: %v",
			observe.OutcomeSettleLooks, outs[0].Why(), outs[0].Transient)
	}
	for _, want := range []string{"presets", "textures", "shadows", "bloom", "foliage"} {
		if !has(outs[0].Appeared, want) {
			t.Errorf("%q stood throughout and is not in the outcome: %v", want,
				outs[0].Appeared)
		}
	}
}

// A TRANSITION LANDING INSIDE THE WINDOW STARTS IT AGAIN, ON THE DEMONSTRATION SIDE TOO.
//
// `OutcomeSkipReadings` is two, measured on a recording read at every look. Live, OCR answers
// two or three times a second and an application may take longer than two readings to redraw,
// so the settled window straddles the change and the persistence term correctly calls every
// word of the new panel TRANSIENT. The run side needed this correction first; this is the same
// one, and without it a live demonstration records no expectation for a press that worked.
func TestATransitionInsideTheWindowRestartsItWhileDemonstrating(t *testing.T) {
	var c observe.ClickTargets
	was := inPanel("resolution", "window", "mode")
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", was)
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// THE OLD PANEL IS STILL THERE past the skip, and then the new one arrives.
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", was)
	}
	for i := 0; i < 6; i++ {
		c.Reading("visual_2", inPanel("presets", "textures", "shadows", "bloom",
			"foliage", "distance"))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if !outs[0].Verified() {
		t.Fatalf("the window straddled the redraw and recorded nothing: %s — %v "+
			"appeared, %v were transient", outs[0].Why(), outs[0].Appeared,
			outs[0].Transient)
	}
}

// AND THE READINGS TAKEN MID-RESPONSE ARE NOT EVIDENCE EITHER WAY.
//
// The skip exists because the first readings after a press describe the application
// MID-RESPONSE — on Ark's capture the tab bar is partly drawn and the panel body missing. With
// no skip, every word of the new panel appears on four readings of five and falls out as
// transient: the correlator reported `everything that appeared was transient` for all three of
// that recording's presses and learned ONE step instead of three.
func TestTheReadingsTakenMidResponseAreSkippedWhileDemonstrating(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// TWO HALF-DRAWN READINGS, each carrying a different fragment of the new panel, and
	// then the panel standing. Without the skip the fragments are in the window and
	// nothing persists across all of it.
	c.Reading("visual_1", inPanel("presets"))
	c.Reading("visual_1", inPanel("presets", "textures"))
	for i := 0; i < 6; i++ {
		c.Reading("visual_1", inPanel("presets", "textures", "shadows", "bloom",
			"foliage", "distance"))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if !outs[0].Verified() {
		t.Fatalf("the half-drawn readings were taken as evidence and nothing "+
			"persisted: %s — %v transient", outs[0].Why(), outs[0].Transient)
	}
}

// AND THE RESTART IS BOUNDED, so a screen that never settles still answers.
//
// A screen changing substantially on every reading would start the window again on every
// reading and never conclude. Past the bound the readings are taken as they come — and on a
// screen like that nothing persists, so the answer is a refusal rather than silence. Ark's
// pause menu over live gameplay is exactly this screen: it changed "by unknown" ten times in
// thirty seconds.
func TestAScreenThatNeverSettlesStillAnswersWhileDemonstrating(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 5; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// EVERY READING BRINGS FIVE WORDS THE LAST ONE DID NOT HAVE.
	for i := 0; i < 40; i++ {
		n := strconv.Itoa(i)
		c.Reading("visual_1", inPanel("alpha"+n, "bravo"+n, "delta"+n,
			"gamma"+n, "kilo"+n))
	}

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if outs[0].AfterLooks == 0 {
		t.Fatal("a screen that changes on every reading filled no window at all, " +
			"so the restart never stopped and the press can never be answered")
	}
	if outs[0].Verified() {
		t.Errorf("a screen of churn verified this press: %v", outs[0].Appeared)
	}
}
