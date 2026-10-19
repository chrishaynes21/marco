package main

// WHAT A DERIVED EXPECTATION IS ALLOWED TO REST ON.
//
// `-fixture` builds a one-step workflow from two captures instead of from a demonstration, and
// the expectation it writes onto the step is what a live run will later be verified against. So
// the terms it applies have to be the SAME terms the live verifier applies, and the first two
// versions of this code proved that saying so is not enough:
//
//	hand-rolled independence   37 words where the live correlator produced 36 on the same Ark
//	                           capture, the extra being the control's own label
//	hand-rolled persistence    12 words of "outcome" on a Trackmania recording whose fold is a
//	                           SINGLE screen over 862 looks, one visit
//
// These tests are over `fixtureOutcome`, which is the one place those terms are now asked, and
// each one fails if a term is removed from it.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// theControl is where the fixture's action site sits, and `insideControl` a word within it.
var theControl = observe.Region{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}

// readWord is one OCR reading of one phrase at a place.
func readWord(text string, x, y float64) observe.ReadWord {
	return observe.ReadWord{
		Text:  text,
		Where: observe.Region{X: x - 0.01, Y: y - 0.005, Width: 0.02, Height: 0.01},
	}
}

// insideControl is a phrase read within the control's own rectangle.
func insideControl(text string) observe.ReadWord { return readWord(text, 0.446, 0.255) }

// onThePanel is a phrase read well away from the control, which is where an outcome lives.
func onThePanel(text string) observe.ReadWord { return readWord(text, 0.50, 0.60) }

// sideReading is one look's reading: the control's label plus whatever the panel said.
func sideReading(panel string) []observe.ReadWord {
	out := []observe.ReadWord{insideControl("GRAPHICS")}
	for _, w := range strings.Fields(panel) {
		out = append(out, onThePanel(w))
	}
	return out
}

// aWindow is `n` identical readings, which is a state that stood.
func aWindow(n int, panel string) [][]observe.ReadWord {
	var out [][]observe.ReadWord
	for i := 0; i < n; i++ {
		out = append(out, sideReading(panel))
	}
	return out
}

// aSide is a fixture side with a given window of readings.
func aSide(dir string, window [][]observe.ReadWord) fixtureSide {
	return fixtureSide{dir: dir, application: "fixtureapp", look: len(window) - 1, window: window}
}

// TestADerivedOutcomeKeepsOnlyTheWordsThatSTOOD is the persistence term.
//
// The Trackmania case, which the single-look version could not even ask about: a word read on
// one look of the settled window is a half-drawn panel, an OCR misread, or a highlight bar
// moving off a row, and a verifier that accepts it is accepting noise as a destination.
func TestADerivedOutcomeKeepsOnlyTheWordsThatSTOOD(t *testing.T) {
	before := aSide("before", aWindow(observe.OutcomeSettleLooks,
		"resolution window mode upscaler"))
	after := aWindow(observe.OutcomeSettleLooks, "textures shadows foliage bloom particles")
	// AND ONE WORD ON THE LAST READING ONLY, which is the whole point.
	after[len(after)-1] = append(after[len(after)-1], onThePanel("flicker"))

	o, _, _, err := fixtureOutcome(before, aSide("after", after), theControl, nil)
	if err != nil {
		t.Fatalf("a panel that replaced another should be a usable outcome: %v", err)
	}
	if got := strings.Join(o.Appeared, " "); got != "bloom foliage particles shadows textures" {
		t.Errorf("appeared = %q, want the five words read on every look", got)
	}
	if got := strings.Join(o.Transient, " "); got != "flicker" {
		t.Errorf("transient = %q, want the one word read on a single look", got)
	}
	for _, w := range o.Appeared {
		if w == "flicker" {
			t.Fatal("a word read on one look of five is in the saved expectation; " +
				"the persistence term is not being applied")
		}
	}
}

// TestADerivedOutcomeDoesNotRestOnTheControlsOwnWords is the independence term.
//
// The Ark case. The GRAPHICS panel body also says `Graphics Presets`, so the control's label
// reappears outside the control after the press — and an expectation containing it rests, by
// that word, on the evidence the action site was approved from.
func TestADerivedOutcomeDoesNotRestOnTheControlsOwnWords(t *testing.T) {
	before := aSide("before", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("after", aWindow(observe.OutcomeSettleLooks,
		"textures shadows foliage bloom particles graphics"))

	o, _, _, err := fixtureOutcome(before, after, theControl, nil)
	if err != nil {
		t.Fatalf("five independent words is a usable outcome: %v", err)
	}
	if got := strings.Join(o.Reused, " "); got != "graphics" {
		t.Errorf("reused = %q, want the control's own label", got)
	}
	for _, w := range o.Appeared {
		if w == "graphics" {
			t.Fatal("the control's own label is in the saved expectation; the " +
				"independence term is not being applied")
		}
	}
	if len(o.Appeared) != 5 {
		t.Errorf("appeared = %v, want the five words the control does not say", o.Appeared)
	}
}

// TestADerivedOutcomeIsRefusedOnAScreenThatMovesByItself is the stationarity term.
//
// Asked of the BEFORE side's own readings, with no action anywhere in them. A screen carrying a
// clock, a ping counter or a rotating news panel produces "a word appeared elsewhere and
// stayed" on its own, so an expectation derived from it would be satisfied by nothing happening.
func TestADerivedOutcomeIsRefusedOnAScreenThatMovesByItself(t *testing.T) {
	// FIVE READINGS, HALVED: two calm, then three carrying five words that were not
	// there and then stayed. No action occurred between them.
	window := aWindow(2, "menu entry here")
	window = append(window, aWindow(3, "menu entry here ping clock frames latency jitter")...)
	before := aSide("beforedir", window)
	after := aSide("after", aWindow(observe.OutcomeSettleLooks,
		"textures shadows foliage bloom particles"))

	o, quiet, moved, err := fixtureOutcome(before, after, theControl, nil)
	if err == nil {
		t.Fatalf("a screen that changes by itself must be refused, got outcome %v",
			o.Appeared)
	}
	if !strings.Contains(err.Error(), "changed by themselves") {
		t.Errorf("refusal = %q, want it to name the screen moving on its own", err)
	}
	if quiet != 1 || moved != 1 {
		t.Errorf("quiet, moved = %d, %d; want 1, 1 — one window asked and one moved",
			quiet, moved)
	}
	if !strings.Contains(err.Error(), "beforedir") {
		t.Errorf("refusal = %q, want it to name which capture moved", err)
	}
}

// TestAFixtureWindowTooShortToHalveAsksNothing holds the "not asked" case open.
//
// `Stationary` reports 0 quiet windows for a window it cannot halve, and 0 quiet must mean the
// question was not asked rather than that the screen passed. A single reading is exactly that
// case, and the refusal above requires `quiet > 0`, so it must not fire here.
func TestAFixtureWindowTooShortToHalveAsksNothing(t *testing.T) {
	if got := fixtureHalves(aWindow(1, "only one reading")); got != nil {
		t.Fatalf("fixtureHalves of one reading = %v, want nothing to compare", got)
	}
	before := aSide("before", aWindow(1, "resolution window mode"))
	after := aSide("after", aWindow(1, "textures shadows foliage bloom particles"))

	o, quiet, moved, err := fixtureOutcome(before, after, theControl, nil)
	if err != nil {
		t.Fatalf("one reading each side still measures an outcome: %v", err)
	}
	if quiet != 0 || moved != 0 {
		t.Errorf("quiet, moved = %d, %d; want 0, 0 — the question was not asked",
			quiet, moved)
	}
	if len(o.Appeared) != 5 {
		t.Errorf("appeared = %v, want the five panel words", o.Appeared)
	}
}

// TestADerivedOutcomeTooSmallToVerifyIsRefused is the positive term's floor.
func TestADerivedOutcomeTooSmallToVerifyIsRefused(t *testing.T) {
	before := aSide("before", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("after", aWindow(observe.OutcomeSettleLooks, "textures shadows foliage"))

	_, _, _, err := fixtureOutcome(before, after, theControl, nil)
	if err == nil {
		t.Fatal("three words is below OutcomeWords and must not become an expectation")
	}
	if !strings.Contains(err.Error(), "not distinguishable enough") {
		t.Errorf("refusal = %q, want it to say the two states are too alike", err)
	}
}

// aPress is one observed mouse press in a recording's input timeline.
func aPress(at int64, x, y float64) inputRecord {
	return inputRecord{
		Intent: "point", AtMS: at, Device: "mouse",
		Where: observe.PointerAt{X: x, Y: y},
	}
}

// TestAnOutcomeNeedsAPressSeenInsideTheControl is the attribution term.
//
// The Trackmania defect, and the one no perception term could have caught: all four of
// positive, persistent, independent and stationary passed on two looks of the same menu with
// the highlight on a different row, because a row whose text stops being inverted becomes
// readable. Nobody pressed SETTINGS. An outcome is only an outcome OF something.
func TestAnOutcomeNeedsAPressSeenInsideTheControl(t *testing.T) {
	before := aSide("walk", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("walk", aWindow(observe.OutcomeSettleLooks,
		"textures shadows foliage bloom particles"))
	before.at, after.at = 24501, 26200
	// ARROW KEYS ONLY, which is what the Trackmania recording holds.
	before.inputs = []inputRecord{
		{Intent: "down", AtMS: 25000, Device: "keyboard"},
		{Intent: "down", AtMS: 25500, Device: "keyboard"},
	}

	_, err := fixtureAttribution(before, after, theControl, "walk", "walk", "")
	if err == nil {
		t.Fatal("a recording that saw no press inside the control must be refused; " +
			"the difference between its looks is not the control's doing")
	}
	for _, want := range []string{"saw no mouse press inside this control", "0 press(es)",
		"2 input(s)", "only an outcome OF something"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal = %q, want it to contain %q", err, want)
		}
	}
}

// TestAPressInsideTheControlBetweenTheLooksIsTheAttribution is the Ark shape.
//
// `{"intent":"point","at_ms":24892,"where":{"x":0.414,"y":0.261}}`, inside the GRAPHICS
// rectangle and between look 245 at 24501ms and look 262 at 26200ms.
func TestAPressInsideTheControlBetweenTheLooksIsTheAttribution(t *testing.T) {
	before := aSide("walk", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("walk", aWindow(observe.OutcomeSettleLooks, "textures shadows foliage"))
	before.at, after.at = 24501, 26200
	before.inputs = []inputRecord{
		{Intent: "pause", AtMS: 3664, Device: "keyboard"},
		aPress(12254, 0.505, 0.422), // elsewhere on the screen, and earlier
		aPress(24892, 0.414, 0.261), // inside the control, between the looks
	}

	got, err := fixtureAttribution(before, after, theControl, "walk", "walk", "")
	if err != nil {
		t.Fatalf("a press seen inside the control between the looks attributes it: %v", err)
	}
	for _, want := range []string{"observed mouse press at 0.414,0.261", "24892ms",
		"NOT a replay coordinate"} {
		if !strings.Contains(got, want) {
			t.Errorf("attribution = %q, want it to contain %q", got, want)
		}
	}
}

// TestAPressOutsideTheWindowOrOutsideTheControlDoesNotAttribute holds both bounds.
func TestAPressOutsideTheWindowOrOutsideTheControlDoesNotAttribute(t *testing.T) {
	base := aSide("walk", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("walk", aWindow(observe.OutcomeSettleLooks, "textures shadows foliage"))
	base.at, after.at = 24501, 26200

	for _, c := range []struct {
		name  string
		press inputRecord
	}{
		{"before the first look", aPress(24000, 0.414, 0.261)},
		{"after the second look", aPress(26500, 0.414, 0.261)},
		{"outside the control", aPress(24892, 0.900, 0.900)},
		{"a key rather than a press", inputRecord{Intent: "down", AtMS: 24892}},
	} {
		side := base
		side.inputs = []inputRecord{c.press}
		if _, ok := pressBetween(side, theControl, side.at, after.at); ok {
			t.Errorf("%s: attributed the difference to the control anyway", c.name)
		}
	}
}

// TestTwoSeparateRecordingsMustSayWhoAssertsTheLink is the controlled-menu shape.
//
// Two captures share no input timeline, so the question cannot be asked at all. The test menu
// serves its before and after states from a URL precisely so that nobody has to click to
// capture either one — and there the causal link is somebody's word. It must be said out loud
// and written onto the step, never implied by silence.
func TestTwoSeparateRecordingsMustSayWhoAssertsTheLink(t *testing.T) {
	before := aSide("state-a", aWindow(observe.OutcomeSettleLooks, "resolution window mode"))
	after := aSide("state-b", aWindow(observe.OutcomeSettleLooks, "textures shadows foliage"))

	if _, err := fixtureAttribution(before, after, theControl, "state-a", "state-b", ""); err == nil {
		t.Fatal("two recordings with no stated link must be refused, not assumed")
	}
	got, err := fixtureAttribution(before, after, theControl, "state-a", "state-b",
		"the test menu serves ?panel=graphics, which is where its GRAPHICS tab points")
	if err != nil {
		t.Fatalf("a stated link is accepted: %v", err)
	}
	for _, want := range []string{"NO observed press links these two states",
		"asserted rather than measured", "?panel=graphics"} {
		if !strings.Contains(got, want) {
			t.Errorf("attribution = %q, want it to contain %q", got, want)
		}
	}
}

// aProposedButton is the proposer's rectangle for the control, plus the bar it sits in.
//
// `theControlSaying` wants the SMALLEST proposal holding exactly one word, so the bar is here
// to be rejected as the control and accepted as a container the outcome may not be read from.
func aProposedButton() []observe.Region {
	return []observe.Region{
		theControl,
		{X: 0.05, Y: 0.235, Width: 0.90, Height: 0.040}, // the tab bar
		{X: 0.05, Y: 0.300, Width: 0.90, Height: 0.600}, // the panel below it
	}
}

// aFixtureSide is a state made by hand: proposals, a reading, a window and a timeline.
func aFixtureSide(dir, panel string, at int64) fixtureSide {
	window := aWindow(observe.OutcomeSettleLooks, panel)
	return fixtureSide{
		dir: dir, application: "FixtureApp", look: 42, at: at,
		regions: aProposedButton(),
		words:   window[len(window)-1],
		window:  window,
	}
}

// TestAnAssembledFixtureCarriesWhatWasMeasuredAndNothingElse enters through the assembly.
//
// The call sites, not the rules: `buildFixtureWorkflow` is two disk reads and this, so this is
// where a deleted gate shows up. Each of the four checks below fails on its own if the
// corresponding call is removed from `assembleFixture`.
func TestAnAssembledFixtureCarriesWhatWasMeasuredAndNothingElse(t *testing.T) {
	before := aFixtureSide("walk", "resolution window mode", 24501)
	after := aFixtureSide("walk", "textures shadows foliage bloom particles", 26200)
	before.inputs = []inputRecord{aPress(24892, 0.414, 0.261)}

	art, say, err := assembleFixture(before, after, "GRAPHICS", "ark graphics tab", "",
		"walk", "walk")
	if err != nil {
		t.Fatalf("a measured state, a press inside the control and a changed panel: %v", err)
	}
	// THE SCOPE, normalised from the recording's own application name.
	if art.Application != workflowstore.NormaliseScope("FixtureApp") {
		t.Errorf("scope = %q, want the capture's application lowercased", art.Application)
	}
	if ok, why := art.ScopeAllows("somethingelse"); ok {
		t.Errorf("the derived workflow is valid for another application: %s", why)
	}
	// THE EXPECTATION, which is the outcome and not the whole reading.
	if len(art.Steps) != 1 {
		t.Fatalf("steps = %d, want exactly one", len(art.Steps))
	}
	step := art.Steps[0]
	if got := strings.Join(step.Outcome, " "); got != "bloom foliage particles shadows textures" {
		t.Errorf("step outcome = %q, want the five words that appeared and stood", got)
	}
	// THE CONTROL, which is the smallest proposal holding the label, with the bar as a
	// container the outcome may not be read from.
	if len(art.Controls) != 1 || len(art.Controls[0].Readings) != 1 {
		t.Fatalf("controls = %+v, want one with one reading", art.Controls)
	}
	if art.Controls[0].Readings[0] != theControl {
		t.Errorf("control = %+v, want the smallest proposal holding the label",
			art.Controls[0].Readings[0])
	}
	if len(art.Controls[0].Containers) != 1 {
		t.Errorf("containers = %d, want the one proposal that encloses the control",
			len(art.Controls[0].Containers))
	}
	// THE ATTRIBUTION, written onto the step where a reader of the verdict will see it.
	if !strings.Contains(strings.Join(step.Unresolved, " "), "observed mouse press at 0.414,0.261") {
		t.Errorf("unresolved = %q, want the press that attributes the difference",
			step.Unresolved)
	}
	if !strings.Contains(strings.Join(step.Unresolved, " "), "ASSEMBLED") {
		t.Errorf("unresolved = %q, want it to say no live demonstration produced this",
			step.Unresolved)
	}
	if !strings.Contains(say, "caused by") {
		t.Errorf("report = %q, want it to state what caused the difference", say)
	}
	// AND THE STEP GOES NOWHERE, which is the case this exists for.
	if step.From != step.To {
		t.Errorf("step %s -> %s, want a step that stays on its own screen",
			step.From, step.To)
	}
}

// TestAnAssembledFixtureRefusesWhatItCannotShow is the same call sites, refusing.
func TestAnAssembledFixtureRefusesWhatItCannotShow(t *testing.T) {
	good := func() (fixtureSide, fixtureSide) {
		b := aFixtureSide("walk", "resolution window mode", 24501)
		a := aFixtureSide("walk", "textures shadows foliage bloom particles", 26200)
		b.inputs = []inputRecord{aPress(24892, 0.414, 0.261)}
		return b, a
	}
	for _, c := range []struct {
		name string
		want string
		bend func(*fixtureSide, *fixtureSide)
	}{
		{"no press was seen inside the control", "saw no mouse press inside this control",
			func(b, a *fixtureSide) { b.inputs = nil }},
		{"the two captures watched two applications", "cannot be derived from captures of two",
			func(b, a *fixtureSide) { a.application = "anothergame" }},
		{"the label is on no proposal", "holds exactly the one word",
			func(b, a *fixtureSide) { b.words, b.window = nil, aWindow(5, "nothing here") }},
		{"the panel did not change enough", "not distinguishable enough",
			func(b, a *fixtureSide) {
				a.window = aWindow(observe.OutcomeSettleLooks, "resolution window mode")
			}},
		{"the screen moves by itself", "changed by themselves",
			func(b, a *fixtureSide) {
				w := aWindow(2, "menu entry here")
				w = append(w, aWindow(3, "menu entry here ping clock frames latency jitter")...)
				b.window = w
			}},
	} {
		b, a := good()
		c.bend(&b, &a)
		_, _, err := assembleFixture(b, a, "GRAPHICS", "n", "", "walk", "walk")
		if err == nil {
			t.Errorf("%s: assembled a workflow anyway", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: refusal = %q, want it to contain %q", c.name, err, c.want)
		}
	}
}

// TestTheControlIsTheSmallestProposalHOLDINGONLYTheLabel is the control search.
//
// Two conditions, and a mutation run found both untested: a proposal holding the label AND
// other words is the bar the label sits in rather than the control, and where two proposals
// each hold the label alone the smaller one is the control. Experiment 085 is where those came
// from; the Settings case in [[referent-granularity-role-class]] is what happens without them.
func TestTheControlIsTheSmallestProposalHOLDINGONLYTheLabel(t *testing.T) {
	side := fixtureSide{dir: "walk", application: "fixtureapp", look: 1}
	// THE BAR, holding two tab labels, listed FIRST so a search that takes the first
	// match rather than the smallest would take it.
	bar := observe.Region{X: 0.05, Y: 0.235, Width: 0.90, Height: 0.040}
	loose := observe.Region{X: 0.395, Y: 0.238, Width: 0.100, Height: 0.034}
	side.regions = []observe.Region{bar, loose, theControl}
	side.words = []observe.ReadWord{
		insideControl("GRAPHICS"),
		readWord("DISPLAY", 0.300, 0.255), // in the bar and in no proposal of its own
	}

	got, containers, err := theControlSaying(side, "GRAPHICS")
	if err != nil {
		t.Fatalf("the label sits alone in two proposals: %v", err)
	}
	if got != theControl {
		t.Errorf("control = %+v, want the SMALLEST proposal holding it alone", got)
	}
	if len(containers) != 2 {
		t.Errorf("containers = %d, want both proposals that enclose the control",
			len(containers))
	}
	// AND A LABEL THAT ONLY EVER APPEARS BESIDE ANOTHER is not a control at all.
	if _, _, err := theControlSaying(side, "DISPLAY"); err == nil {
		t.Error("a label read only inside a multi-word proposal was accepted as a " +
			"control; that proposal is the bar, not the button")
	}
}

// TestAWindowRunningBackwardsIsRefused holds the order of the two looks.
//
// An earlier version swapped the timestamps so a backwards window still found its press, which
// makes the GRAPHICS panel closing indistinguishable from it opening.
func TestAWindowRunningBackwardsIsRefused(t *testing.T) {
	before := aFixtureSide("walk", "textures shadows foliage bloom particles", 26200)
	after := aFixtureSide("walk", "resolution window mode", 24501)
	before.inputs = []inputRecord{aPress(24892, 0.414, 0.261)}

	_, err := fixtureAttribution(before, after, theControl, "walk", "walk", "")
	if err == nil {
		t.Fatal("a window that runs backwards must be refused, not reversed")
	}
	if !strings.Contains(err.Error(), "runs backwards") {
		t.Errorf("refusal = %q, want it to name the order of the two looks", err)
	}
	// AND THE SAME INSTANT IS NOT AFTER EITHER, which is the boundary a mutation
	// survived: two looks captured at the same millisecond have nothing between them,
	// so there is no window for a press to be in.
	atOnce := aFixtureSide("walk", "resolution window mode", 24501)
	atOnce.inputs = before.inputs
	same := aFixtureSide("walk", "textures shadows foliage", 24501)
	_, err = fixtureAttribution(atOnce, same, theControl, "walk", "walk", "")
	if err == nil || !strings.Contains(err.Error(), "runs backwards") {
		t.Errorf("two looks captured at the same instant: refusal = %v, want the "+
			"order refusal — nothing happened between them, so there is no "+
			"window for a press to be in", err)
	}
}

// TestOneRecordingNamedTwoWaysIsStillOneRecording keeps the two modes unconfusable.
//
// The asserted mode exists only because two separate recordings share no timeline. A path
// written `walk` and `walk/` is ONE recording, and if those compared unequal the builder would
// quietly switch to the mode that requires no observed press — which is the whole guard gone.
func TestOneRecordingNamedTwoWaysIsStillOneRecording(t *testing.T) {
	before := aFixtureSide("walk", "resolution window mode", 24501)
	after := aFixtureSide("walk", "textures shadows foliage", 26200)
	before.inputs = nil // nothing was seen

	_, err := fixtureAttribution(before, after, theControl, "walk", "walk/", "")
	if err == nil {
		t.Fatal("`walk` and `walk/` were treated as two recordings, so a press that " +
			"was never observed stopped being required")
	}
	if !strings.Contains(err.Error(), "saw no mouse press") {
		t.Errorf("refusal = %q, want the one-recording refusal", err)
	}
}

// TestTheMeasuredWindowIsCappedAndSkipsUnreadableLooks is the window collection.
func TestTheMeasuredWindowIsCappedAndSkipsUnreadableLooks(t *testing.T) {
	words := make([][]observe.ReadWord, 12)
	for i := range words {
		words[i] = []observe.ReadWord{readWord(fmt.Sprintf("look%02d", i), 0.5, 0.5)}
	}
	// LOOKS 8 AND 9 CARRY NOTHING, which is what a frame OCR could not read looks like.
	usable := func(i int) bool { return i >= 0 && i < len(words) && i != 8 && i != 9 }

	got := windowEndingAt(words, usable, 11, 5)
	if len(got) != 5 {
		t.Fatalf("window = %d reading(s), want the cap of 5", len(got))
	}
	// SKIPPING 8 AND 9, the five are 5, 6, 7, 10, 11 — oldest first.
	for i, want := range []int{5, 6, 7, 10, 11} {
		if got[i][0].Text != fmt.Sprintf("look%02d", want) {
			t.Errorf("reading %d is %q, want look %d", i, got[i][0].Text, want)
		}
	}
	if got := windowEndingAt(words, usable, 2, 5); len(got) != 3 {
		t.Errorf("window ending at look 2 = %d reading(s), want the 3 that exist",
			len(got))
	}
	if got := windowEndingAt(words, func(int) bool { return false }, 11, 5); got != nil {
		t.Errorf("window of unreadable looks = %v, want nothing to measure", got)
	}
}
