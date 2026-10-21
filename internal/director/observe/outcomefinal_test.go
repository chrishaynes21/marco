package observe_test

// WHEN AN OUTCOME IS THE ANSWER, AND WHEN IT IS THE ANSWER SO FAR.
//
// `Verified` says the four terms hold. NOTHING said whether they could still come to hold, and
// the two are the same question only for a watch that fills: a press whose readings are still
// arriving is a press to wait for, and a press whose window will never grow again is one
// waiting cannot help.
//
// It was found live. The owner demonstrated `enter freeplay` on Rocket League, whose main menu
// carries a rotating news panel and live counters; the second press reached its verdict 2.7
// seconds after the click and the recording card said "Checking what changed… wait for the
// result to settle" for the twenty-nine seconds after that:
//
//	it got stuck on settling for like 30s and I just had to finish the recording rather than
//	waiting
//
// Nothing was wrong with the measurement. See learnstate.Unchecked for the phase this feeds.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A WINDOW STILL FILLING IS NOT FINAL, AND SAYS SO IN THOSE WORDS.
//
// Two readings after the press are dropped as mid-response and five have to stand, so a watch
// spends the first few readings of its life with an answer that is allowed to change. What it
// may not do is present that as a verdict, and it used to: the sentence said the window had
// been "cut short", which is a thing that happened to a window rather than a thing that has
// not happened yet.
func TestAnOutcomeStillAccumulatingReadingsIsNotTheAnswer(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// THE SKIP, AND THEN ONE READING SHORT OF THE WINDOW. Everything the new panel
	// says is there; what has not happened is the readings that show it standing.
	panel := inPanel("presets", "textures", "shadows", "bloom", "foliage")
	for i := 0; i < observe.OutcomeSkipReadings; i++ {
		c.Reading("visual_1", panel)
	}
	for i := 0; i < observe.OutcomeSettleLooks-1; i++ {
		c.Reading("visual_1", panel)
	}

	o := c.Evidence().Outcomes[0]
	if o.Final() {
		t.Fatalf("a window at %d of %d reading(s) is reported as the final answer, "+
			"so a recorder would stop waiting for the one still to come",
			o.AfterLooks, observe.OutcomeSettleLooks)
	}
	if o.Verified() {
		t.Error("a window one reading short of persistence verified")
	}
	// AND THE SENTENCE IS ABOUT WHAT HAS NOT ARRIVED, not about a window somebody
	// cut short. A person reads this ten times a second while demonstrating.
	if why := o.Why(); !strings.Contains(why, "arrived") ||
		strings.Contains(why, "cut short") {
		t.Errorf("a window still filling says %q, which describes a window that "+
			"was ended rather than one still being read", why)
	}

	// AND THE READING THAT COMPLETES IT MAKES IT BOTH FINAL AND VERIFIED.
	c.Reading("visual_1", panel)
	done := c.Evidence().Outcomes[0]
	if !done.Final() || !done.Verified() {
		t.Errorf("the window filled and the outcome is final=%v verified=%v: %s",
			done.Final(), done.Verified(), done.Why())
	}
}

// AND A WINDOW A THIRD SCREEN CLOSED IS FINAL AT WHATEVER IT HAD.
//
// `observeOutcome` folds nothing into a closed watch, so its answer cannot improve — and this
// is the half `Settled` cannot express at all: the window never filled, and it never will.
// Without it a recorder has no way to tell this apart from a window one reading short, and
// tells somebody to wait for a reading that can no longer be taken.
func TestAWindowAThirdScreenClosedIsFinalWithoutHavingFilled(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// WHERE IT LED, for the two skipped readings and one more, and then somewhere
	// else: a third screen is "whatever Marco looked at next" and closes the watch.
	for i := 0; i < observe.OutcomeSkipReadings+1; i++ {
		c.Reading("visual_2", inPanel("presets", "textures"))
	}
	for i := 0; i < observe.OutcomeSettleLooks*2; i++ {
		c.Reading("visual_9", inPanel("inventory", "crafting", "engrams"))
	}

	o := c.Evidence().Outcomes[0]
	if o.Settled {
		t.Fatalf("a window of %d reading(s) is reported as settled", o.AfterLooks)
	}
	if !o.Closed || !o.Final() {
		t.Errorf("a watch a third screen closed is reported as still accumulating "+
			"(closed=%v final=%v), so nothing can stop waiting for it",
			o.Closed, o.Final())
	}
	if why := o.Why(); !strings.Contains(why, "cut short") {
		t.Errorf("a closed window says %q, and what happened to it is that it "+
			"ended early", why)
	}
}

// AND A SCREEN THAT CHANGES BY ITSELF REACHES A FINAL ANSWER TOO.
//
// The owner's case, with the fixture that produces it: a menu whose words churn restarts the
// window until the restart allowance is spent, and then fills it with words that cannot all
// stand. The answer is no, and the point of this test is that it is an ANSWER — a recorder that
// read `Verified` alone would wait for the rest of the recording.
func TestAScreenOfChurnReachesAnAnswerAndNotAnOpenQuestion(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", inPanel("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	for i := 0; i < 40; i++ {
		n := strconv.Itoa(i)
		c.Reading("visual_1", inPanel("alpha"+n, "bravo"+n, "delta"+n,
			"gamma"+n, "kilo"+n))
	}

	o := c.Evidence().Outcomes[0]
	if o.Verified() {
		t.Fatalf("a screen of churn verified this press: %v", o.Appeared)
	}
	if !o.Final() {
		t.Errorf("after %d reading(s) of churn the answer is still reported as "+
			"pending, so the recorder goes on asking for patience: %s",
			o.AfterLooks, o.Why())
	}
}

// AND A PRESS WITH NO BEFORE SIDE IS FINAL BEFORE IT STARTS.
//
// The before side is taken ONCE, by `watch`, from the readings already held of the screen the
// press was made on. Nothing afterwards appends to it. So a press made on a screen nothing has
// been read on has no before side for the rest of the session, however long anybody waits —
// and the question `Verified` answers was never asked rather than answered no.
//
// This is what the owner hit, and the recorder was calling it a press to wait for:
//
//	Three presses seen, only one worked. Settling is ass, it doesn't finish in time.
//
// Their Trackmania recording passed through a screen too fast for a single reading to be taken
// on it — the saved artifact has it with 0 words — and the press made there spent seventeen
// seconds being described as "checking what changed… wait for the result to settle".
func TestAPressOnAScreenNothingHasBeenReadOnIsAnsweredAtOnce(t *testing.T) {
	var c observe.ClickTargets
	// NO READING AT ALL BEFORE THE PRESS. The screen was there and gone.
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// AND THEN A WINDOW STILL FILLING. Deliberately short: a FULL one would make the
	// outcome final by `Settled` whatever this term said, which is what the first
	// version of this test did — a mutation removing the term survived it.
	panel := inPanel("presets", "textures", "shadows", "bloom", "foliage")
	for i := 0; i < observe.OutcomeSkipReadings+2; i++ {
		c.Reading("visual_1", panel)
	}

	o := c.Evidence().Outcomes[0]
	if o.Settled || o.Closed {
		t.Fatalf("the fixture's after window is settled=%v closed=%v, so this "+
			"says nothing about a press with no before side", o.Settled,
			o.Closed)
	}
	if o.BeforeLooks != 0 {
		t.Fatalf("the fixture read %d reading(s) before the press and was meant to "+
			"read none", o.BeforeLooks)
	}
	if o.Verified() {
		t.Error("a press with nothing read before it verified, so the words that " +
			"appeared were compared against nothing")
	}
	if !o.Final() {
		t.Errorf("a press with no before side is still reported as pending after "+
			"%d reading(s) after it, so a recorder goes on asking somebody to "+
			"wait for a comparison that cannot be made: %s", o.AfterLooks, o.Why())
	}
	if why := o.Why(); !strings.Contains(why, "one side") {
		t.Errorf("the reason is %q, and what happened is that there was nothing "+
			"on one side of the press", why)
	}

	// AND A FULL WINDOW AFTER IT STILL DOES NOT VERIFY: the words that appeared have
	// nothing to have appeared AGAINST.
	for i := 0; i < observe.OutcomeSettleLooks*2; i++ {
		c.Reading("visual_1", panel)
	}
	if full := c.Evidence().Outcomes[0]; full.Verified() {
		t.Error("a press with nothing read before it verified on a full window, " +
			"so the words that appeared were compared against nothing")
	}

	// AND THE SAME PRESS WITH ONE READING BEFORE IT IS AN ORDINARY QUESTION AGAIN.
	var ok observe.ClickTargets
	ok.Reading("visual_1", inPanel("resolution", "window", "mode"))
	ok.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	for i := 0; i < observe.OutcomeSkipReadings+observe.OutcomeSettleLooks; i++ {
		ok.Reading("visual_1", panel)
	}
	if got := ok.Evidence().Outcomes[0]; !got.Verified() {
		t.Errorf("one reading before the press was not enough to ask the question "+
			"at all: %s", got.Why())
	}
}

// A PRESS THAT OPENED SOMETHING UNREADABLE SAYS SO, rather than describing its words.
//
// The owner drove into a Trackmania map:
//
//	entered a map, marco couldn't tell that we opened a map and never settled
//
// Their recording has the destination as two screens with 0 words between them — live 3D, where
// OCR finds glyphs off a moving world that are gone by the next reading. The verdict was
// "everything that appeared was transient", which is true of the words and says nothing about
// the thing that happened: the press worked, and opened a screen that cannot be read at all.
//
// The two are different situations with different answers, and `Appeared` alone cannot tell
// them apart — both are empty when a still screen produced nothing and when a map did.
func TestAPressThatOpenedAnUnreadableScreenSaysThatIsWhatHappened(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_1", inPanel("solo", "local", "online", "create"))
	}
	c.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	// AND THEN A MAP: a different screen, and nothing on it that two readings agree on.
	for i := 0; i < observe.OutcomeSkipReadings+observe.OutcomeSettleLooks; i++ {
		c.Reading("visual_2", inPanel("glyph"+strconv.Itoa(i)))
	}

	o := c.Evidence().Outcomes[0]
	if o.To != "visual_2" {
		t.Fatalf("the press led to %q and the outcome says %q", "visual_2", o.To)
	}
	if o.Verified() {
		t.Fatal("a screen with nothing readable on it verified a press")
	}
	why := o.Why()
	if !strings.Contains(why, "opened a different screen") {
		t.Errorf("the reason does not say the press opened something: %q", why)
	}
	if strings.Contains(why, "nothing elsewhere on the window") {
		t.Errorf("the reason says nothing happened, and a different screen is "+
			"what happened: %q", why)
	}

	// AND A PRESS THE SCREEN SAT STILL THROUGH IS STILL DESCRIBED BY ITS WORDS, which
	// is the case this whole rule was measured for and must not be swallowed by the
	// sentence above.
	var still observe.ClickTargets
	for i := 0; i < observe.OutcomeSettleLooks; i++ {
		still.Reading("visual_1", inPanel("solo", "local", "online"))
	}
	still.Press(10, observe.PointerAt{X: 0.414, Y: 0.251}, "visual_1",
		observe.Proposed([]observe.Region{tabBar}, 0, 0, 0))
	for i := 0; i < observe.OutcomeSkipReadings+observe.OutcomeSettleLooks; i++ {
		still.Reading("visual_1", inPanel("solo", "local", "online"))
	}
	if got := still.Evidence().Outcomes[0]; got.To != "" ||
		!strings.Contains(got.Why(), "nothing elsewhere on the window") {
		t.Errorf("a press the screen sat still through says to=%q, %q",
			got.To, got.Why())
	}
}
