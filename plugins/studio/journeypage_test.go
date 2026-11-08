package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// THE ORDER OF THE CONTROLS ON THE JOURNEY IS THE CLAIM, so it is the thing held.
//
// # Why a test about ORDER
//
// Because the defect was order. Nothing on the page was missing: Play existed, the request box
// existed, the source panes existed. Play was simply the EIGHTH control in the panel, below two
// rebuild buttons and two `<details open>` source listings, and the request box was the
// fourteenth control in the view, inside the routines card below a paragraph about the card.
// Measured in [[Studio-UI-audit-2026-10-29]].
//
// A test that only asserted each control is present would have passed before the change and
// after it, and would go on passing through whatever gets inserted above Play next.
//
// It reads the page as text, like the rest of the page tests here: the claim is about the
// document, and a claim about the document is checkable without a window.
func TestTheJourneyPutsTheQuestionFirstAndPlayUnderWhatIsSelected(t *testing.T) {
	page := thePage(t)

	// ── the view, and what comes first in it ──
	routines := between(t, page, `<section id="view-routines"`, "</section>")
	for _, c := range []struct{ first, then, why string }{
		// THE LIBRARY IS THE SCREEN. Record is the one way to make a routine and sits in
		// the header; the find box is the top of the list it searches, directly above
		// it. See [[Studio-coherence-audit-2026-11-02]], finding 1.
		{`id="recordStart"`, `id="askBox"`,
			"the way in to recording is in the header, above everything else on the screen"},
		{`id="askBox"`, `id="routineList"`,
			"the find box sits directly above the list it searches"},
		{`id="routineList"`, `id="routineChosen"`,
			"the panel about the selected routine comes after the list it is selected from"},
	} {
		if !before(routines, c.first, c.then) {
			t.Errorf("%s is not before %s. %s", c.first, c.then, c.why)
		}
	}

	// ── and inside the panel that opens under the routine's own name ──
	//
	// PLAY IS NOT IN THIS MARKUP ANY MORE, and that is the change rather than a gap.
	// It is drawn onto the ROW by `drawRoutines`, which is where the Architect put it:
	// *keep Play on the collapsed row ... keep a single primary Play button per
	// routine*. So what this panel holds is everything a run NEEDS, in the order a
	// person meets it, and the two collapsed things last. That Play comes before all of
	// it is now a fact about the rendered page and is held by
	// TestARoutineOpensWhereItIsAndPlayStaysOnItsRow.
	chosen := between(t, routines, `id="routineChosen"`, `<h2>`)
	order := []string{
		`id="routineLeaving"`,  // what leaving this routine would throw away
		`id="routineWhy"`,      // why it cannot run and what to do, when it cannot
		`id="routineIntoRow"`,  // the window a run would press in
		`id="routineArgsBox"`,  // and the text it asks for
		`id="routineSource"`,   // what it will do, collapsed
		`id="routineMaintain"`, // rebuild and re-record, collapsed
		`id="routineChecks"`,   // and, under More, how its presses are checked
	}
	for i := 0; i+1 < len(order); i++ {
		if !before(chosen, order[i], order[i+1]) {
			t.Errorf("%s must come before %s in the selected routine's panel",
				order[i], order[i+1])
		}
	}
	// AND THE PANEL IS NOT A SECOND PLACE FOR PLAY. One per routine, on its row.
	if strings.Contains(chosen, `id="routinePlayBtn"`) {
		t.Error(`id="routinePlayBtn" is in the panel's markup as well as on the ` +
			`row, which is the duplicate Play the ruling names`)
	}
}

// AND THE THINGS A PERSON MUST BE ABLE TO SEE ARE NOT INSIDE A COLLAPSED DETAIL.
//
// A refusal nobody can see is a Play button that does nothing for a reason somebody has to go
// looking for, and a replacement consent inside a `<details>` is consent given to a sentence
// that was never on screen. The rule is checked by nesting rather than by reading the markup:
// anything between a `<details` and its `</details>` is collapsed.
func TestNoBlockingMessageIsInsideACollapsedDetail(t *testing.T) {
	page := thePage(t)
	for _, id := range []string{
		"routineWhy",           // why this routine cannot be performed
		"routineReviewCaution", // what the chosen rebuild changes
		"routineReviewRefused", // a replacement that was refused
		"routineReplaceBtn",    // and the consent itself
		"routineReplaceNote",
		"capEverywhere", // what a recording will take in
		"readyProblems", // whether this window can do its job at all
	} {
		if d := insideDetails(page, `id="`+id+`"`); d != "" {
			t.Errorf("%s is inside a collapsed <details> (%q). Blocking warnings, "+
				"refusal reasons and replacement consent have to be readable "+
				"without opening anything.", id, d)
		}
	}
}

// AND THERE IS ONE WAY IN TO RECORDING A ROUTINE.
//
// There were two complete entry points on one view, each with its own name field, application
// chooser, refresh button and Record button, and four controls on that view said Record or
// Record it again. `Learn a Scene` is a different thing and lives under Scenes now; what this
// holds is that the routine journey has one.
func TestTheRoutineJourneyHasOneWayIntoRecording(t *testing.T) {
	page := thePage(t)
	routines := between(t, page, `<section id="view-routines"`, "</section>")

	// THE ONE BUTTON THAT REVEALS THE FORM. It is labelled `Record` now, in the header;
	// what is counted is what it DOES, because the form it reveals has a Record button of
	// its own that starts the recording.
	starts := regexp.MustCompile(`<button[^>]*onclick="revealRecord\(\)"`).
		FindAllString(routines, -1)
	if len(starts) != 1 {
		t.Errorf("the journey has %d buttons that open the record form, want exactly 1",
			len(starts))
	}
	// AND THE FORM IT REVEALS IS NOT ALSO OPEN. `captureCard` must start hidden, or the
	// button reveals something that was never away.
	card := between(t, routines, `id="captureCard"`, `</div>`)
	if !strings.Contains(startOf(routines, `id="captureCard"`), "hide") {
		t.Errorf("the record form does not start hidden, so `Record` "+
			"reveals a form that is already on screen: %q", firstLine(card))
	}
	// AND THE OTHER RECORDING FLOW IS NOT ON THIS VIEW.
	if strings.Contains(routines, `id="learnCard"`) {
		t.Error("the Learn-a-Scene card is back on the routine journey, which is the " +
			"two-entry-points defect this slice was about")
	}
	scenes := between(t, page, `<section id="view-scenes"`, "</section>")
	if !strings.Contains(scenes, `id="learnCard"`) {
		t.Error("Learn a Scene is not under Scenes either, so it has lost its home " +
			"rather than moved")
	}
}

// AND NO SOURCE LISTING IS EXPANDED BY DEFAULT.
//
// Two `<details open>` panes of generated Marco were expanded on a screen whose job is to start
// a run. `open` is the thing checked, because that is the thing that was wrong.
func TestNoSourceListingIsExpandedOnArrival(t *testing.T) {
	page := thePage(t)
	for _, m := range regexp.MustCompile(`<details[^>]*\bopen\b[^>]*>`).FindAllString(page, -1) {
		t.Errorf("a <details> is open by default: %s", m)
	}
}

// ── reading the page ────────────────────────────────────────────────────────

// thePage is the page's MARKUP, with its comments removed.
//
// WITHOUT THE COMMENTS ON PURPOSE, and the first run of these tests is why. This page explains
// itself in comments, and those comments quote the markup they are about — including the literal
// `<details open>` that the fix replaced. A structural test reading the raw file therefore found
// two `<details open>` that do not exist and placed three elements inside a `<details>` they are
// outside of, because `<details` in a sentence about markup counts as markup when nothing
// strips it. A test that reads prose as structure reports the prose.
func thePage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := htmlComment.ReplaceAllString(string(raw), "")
	if strings.Contains(page, "<!--") {
		t.Fatal("a comment was not removed, so everything below is reading prose as " +
			"structure")
	}
	return page
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// between is the text from the first `from` to the next `to` after it.
func between(t *testing.T, s, from, to string) string {
	t.Helper()
	i := strings.Index(s, from)
	if i < 0 {
		t.Fatalf("the page has no %q", from)
	}
	rest := s[i:]
	if j := strings.Index(rest, to); j > 0 {
		return rest[:j]
	}
	return rest
}

func before(s, a, b string) bool {
	i, j := strings.Index(s, a), strings.Index(s, b)
	return i >= 0 && j >= 0 && i < j
}

func startOf(s, anchor string) string {
	i := strings.Index(s, anchor)
	if i < 0 {
		return ""
	}
	j := strings.LastIndex(s[:i], "<")
	if j < 0 {
		j = i
	}
	k := strings.Index(s[i:], ">")
	if k < 0 {
		return s[j:i]
	}
	return s[j : i+k]
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// insideDetails answers which `<details>` encloses the anchor, or "" when none does.
//
// Counts opens and closes before the anchor rather than parsing: a `<details>` that has not been
// closed by the time the anchor appears is one the anchor is inside of.
func insideDetails(page, anchor string) string {
	i := strings.Index(page, anchor)
	if i < 0 {
		return ""
	}
	head := page[:i]
	opens := regexp.MustCompile(`<details\b[^>]*>`).FindAllString(head, -1)
	closes := strings.Count(head, "</details>")
	if len(opens) > closes {
		return opens[len(opens)-1]
	}
	return ""
}

// AND THE RECORDING STATE NAMES THE WINDOW IT IS RECORDING.
//
// A recording is OF ONE WINDOW, chosen before it starts, and the dropdown that chose it is on
// the card before this one. So this card is the only place the window is named while it matters.
//
// It was already named, as the tail of `Recording "x" in <window>…`; what this holds is that it
// has a line of its own and that the line is filled from `Recording.Window` rather than from
// anything this page worked out.
func TestTheRecordingStateNamesItsWindowAndItsWayOut(t *testing.T) {
	page := thePage(t)
	card := between(t, page, `id="capturing"`, `id="capReview"`)

	for _, id := range []string{"capDoing", "capInWindow", "capPaused", "capStopBtn", "capKey"} {
		if !strings.Contains(card, `id="`+id+`"`) {
			t.Errorf("the recording card has no %s", id)
		}
	}
	for _, c := range []struct{ first, then, why string }{
		{`id="capDoing"`, `id="capInWindow"`, "the window follows the state it belongs to"},
		{`id="capInWindow"`, `id="capPaused"`, "and comes before the pause, which is about it"},
		{`id="capPaused"`, `id="capStopBtn"`, "the state comes before the way out of it"},
	} {
		if !before(card, c.first, c.then) {
			t.Errorf("%s is not before %s: %s", c.first, c.then, c.why)
		}
	}

	// AND THE STOP KEY IS THE CONFIGURED ONE. Never a key named in this page: the button is
	// not the only way out, and the other way belongs to the machine.
	js := page[strings.Index(page, "<script>"):]
	if !strings.Contains(js, "r.stop_key") {
		t.Error("the recording card does not read `stop_key`, so the other way out is " +
			"either unnamed or named by this page")
	}
	for _, key := range []string{"F9", "F10", "F12", "Esc"} {
		if regexp.MustCompile(`['"\s>(]` + key + `['"\s<),.]`).MatchString(card) {
			t.Errorf("the recording card names %q. The stop key is whatever this "+
				"machine is configured with, and the page renders what the "+
				"backend says it is.", key)
		}
	}

	// AND NOTHING SAYS AGAIN WHAT THE BUTTON SAYS. The 21-word instruction that stood here
	// told somebody to press Stop recording when they were done.
	if strings.Contains(card, "Come back here and press") {
		t.Error("the recording card still instructs the person to press the button " +
			"that is in front of them")
	}
}

// AND THE REVIEW PUTS WHAT WAS DEMONSTRATED FIRST.
//
// Seven status lines stood above the step list, so the thing being approved was the eighth
// element in the card. The order here is the whole claim; nothing is hidden, which is the other
// half of it — every line in this card is a reason a save would refuse, surprise or be checked
// differently, and a collapsed detail would be the card deciding which of those a person does
// not need.
func TestTheReviewShowsTheStepsBeforeEverythingAboutThem(t *testing.T) {
	page := thePage(t)
	card := between(t, page, `id="capReview"`, `id="repairCard"`)

	// AND IT NAMES THE ROUTINE SAVE WILL CREATE, above the summary of it. The card opened
	// with "Recorded 6 steps, 4 of them presses." and never said what the thing being saved
	// is called -- the name the person typed, the name Save writes under, and the name a
	// collision is refused against.
	if !before(card, `id="capReviewName"`, `id="capSaid"`) {
		t.Error(`the review card does not name the routine above its summary`)
	}
	js0 := page[strings.Index(page, "<script>"):]
	if !regexp.MustCompile(`capReviewName'\),\s*r\.name`).MatchString(js0) {
		t.Error("the review name is not the draft's own name from the backend")
	}

	order := []string{
		`id="capReviewName"`,     // which routine this will be
		`id="capSaid"`,           // what was recorded
		`id="capSteps"`,          // THE DEMONSTRATED STEPS
		`id="capScopeRow"`,       // the scope they are saved under, beside them
		`id="capMissing"`,        // what would refuse at run time
		`id="capUnsupported"`,    // what cannot be saved at all
		`id="capAgainElsewhere"`, // and where Save succeeds and does the wrong thing
		`id="capSaveBtn"`,        // then Save
		`id="capTaken"`,          // and only then the two ways on from a refusal
	}
	for i := 0; i+1 < len(order); i++ {
		if !before(card, order[i], order[i+1]) {
			t.Errorf("%s must come before %s in the review card", order[i], order[i+1])
		}
	}

	// AND THE SCOPE CHOOSER KEEPS ITS ENFORCEMENT. It is offered only when the choice is
	// the person's: a recording with a guarded press has ONE scope it can be saved under,
	// and the chooser is withdrawn rather than defaulted, because two of the three promise
	// to switch nothing and their routes would refuse every press. Moving the block must
	// not widen that, so the condition is asserted rather than assumed.
	js := page[strings.Index(page, "<script>"):]
	if !regexp.MustCompile(`showing\(\$\('capScopeRow'\),\s*!r\.brings\s*\)`).MatchString(js) {
		t.Error("the scope chooser is no longer shown only when `brings` is empty, so a " +
			"guarded recording may now be offered a scope it cannot be saved under")
	}
}

// EVERY SENTENCE `oneStep` WRITES OPENS WITH A VERB THE PAGE KNOWS — or is rendered whole.
//
// # Why this is a test and not a comment
//
// The page splits a step into a row: number, verb, value. The backend writes a step as one
// SENTENCE (`left-click at 100, 200`), so the split happens in the page, against a closed list of
// prefixes. A list drifts: add a step kind, change a verb, and the page quietly renders the new
// sentence in the wrong column or loses half of it.
//
// So this calls `oneStep` — the real one, in this package — over a step of every kind, and asks
// the page's own table to open the result. A kind the backend has no sentence for falls through
// to its bare name, and must render WHOLE rather than be split by a prefix that happens to fit.
func TestEveryStepSentenceTheBackendWritesHasAVerb(t *testing.T) {
	verbs := stepVerbsInThePage(t)
	if len(verbs) < 8 {
		t.Fatalf("only %d verbs were read out of the page, so this is checking almost "+
			"nothing: %v", len(verbs), verbs)
	}

	for _, c := range []struct {
		why   string
		step  macroir.Step
		whole bool // true when the page must keep the sentence rather than split it
	}{
		{"a left click", macroir.Step{Kind: macroir.StepClick, X: 100, Y: 200}, false},
		{"a right click", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Button: "right"}, false},
		{"a middle click", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Button: "middle"}, false},
		{"a back click", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Button: "back"}, false},
		{"a forward click", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Button: "forward"}, false},
		{"a scroll", macroir.Step{Kind: macroir.StepScroll, X: 1, Y: 2, Wheel: "down",
			Count: 3}, false},
		{"a scroll measured inside the window", macroir.Step{Kind: macroir.StepScroll,
			X: 1, Y: 2, Wheel: "up", Count: 1, WinRel: true, RelX: 3, RelY: 4}, false},
		{"a click measured inside the window", macroir.Step{Kind: macroir.StepClick,
			X: 1, Y: 2, WinRel: true, RelX: 3, RelY: 4}, false},
		{"a key", macroir.Step{Kind: macroir.StepKey, Key: "ctrl+shift+m"}, false},
		{"a repeated key", macroir.Step{Kind: macroir.StepKey, Key: "down",
			Count: 3}, false},
		{"a held key", macroir.Step{Kind: macroir.StepKeyDown, Key: "shift"}, false},
		{"a released key", macroir.Step{Kind: macroir.StepKeyUp, Key: "shift"}, false},
		{"typed text", macroir.Step{Kind: macroir.StepType, Text: "hello"}, false},
		{"a wait", macroir.Step{Kind: macroir.StepWait, Ms: 250}, false},
		{"a drag", macroir.Step{Kind: macroir.StepDrag, X: 1, Y: 2, X2: 3, Y2: 4}, false},
		{"a bare cursor move", macroir.Step{Kind: macroir.StepMove, X: 1, Y: 2}, false},
		{"a cursor move inside the window", macroir.Step{Kind: macroir.StepMove,
			X: 1, Y: 2, WinRel: true, RelX: 3, RelY: 4}, false},
		{"an application switch", macroir.Step{Kind: macroir.StepActivate,
			Text: "Discord"}, false},
		// AND THE ONE WITH NO SENTENCE, which `oneStep` renders as its own name.
		{"a kind with no sentence", macroir.Step{Kind: macroir.StepFind}, true},
		// A LOOP HAS A SENTENCE SINCE BLOCKS COULD BE MADE (ADR-313): it names what repeats.
		{"a loop", macroir.Step{Kind: macroir.StepLoop, Count: 2,
			Steps: []macroir.Step{{Kind: macroir.StepKey, Key: "a"}}}, false},
	} {
		line := oneStep(c.step)
		verb, rest := openedBy(verbs, line)
		switch {
		case c.whole && verb != "":
			t.Errorf("%s is written as %q, which the page splits on %q and shows as "+
				"%q. A step the page has no verb for must keep its whole "+
				"sentence rather than lose the front of it.", c.why, line, verb, rest)
		case !c.whole && verb == "":
			t.Errorf("%s is written as %q and the page has no verb that opens it, so "+
				"it falls back to the whole sentence in the value column. Add "+
				"the prefix to `stepVerbs`.", c.why, line)
		case !c.whole && rest == "":
			t.Errorf("%s is written as %q and the page's verb leaves no value beside "+
				"it", c.why, line)
		}
	}
}

// A SECOND LOOK AT A ROUTINE IS REACHABLE WITHOUT OPENING ANYTHING.
//
// `internal/orchestrator/regenerate.go` is a careful four-step flow — prepare, review, replace or
// cancel, with consent digests and a refusal if either file moved while the review was open — and
// both of its entrances were inside a `<details>` that arrives closed. A complete capability
// reachable only by somebody who already knew it was there.
//
// WHAT CONDITIONS IT IS THE DEMONSTRATION, NOT THE GENERATION. `offer` is the backend's answer
// to "is there a recording beside this routine that a rebuild could be made from, and can this
// build explain its generation" — and `update` answers something else entirely: whether a
// difference between generations bears on this file. A routine today's rules wrote rebuilds byte
// for byte under them and can still be full of typing pauses worth folding, so conditioning the
// simplification on `update` would hide it from most of the library.
func TestASecondLookAtARoutineIsOfferedWithoutOpeningADetail(t *testing.T) {
	page := thePage(t)
	if d := insideDetails(page, `id="routineSimplifyBtn"`); d != "" {
		t.Errorf("the simplification review is inside a collapsed <details> (%q), "+
			"which is where nobody found it", d)
	}
	// AFTER THE THINGS A RUN NEEDS. Play itself is on the row above this panel now,
	// so what this orders against is the window a run presses in -- the first control
	// in the panel and the one immediately under the button.
	if !before(page, `id="routineIntoRow"`, `id="routineSimplifyBtn"`) {
		t.Error("the second look comes before the window a run would press in, " +
			"which is the one thing this panel is for")
	}
	// AND IT IS IN THE TAB ROW, VISIBLE, AND NOT UNDER MORE. The owner asked for this
	// capability and could not find it, twice; the Architect's ruling on the tabs keeps
	// it a visible button for that reason. The promise that nothing is written is the
	// review it opens, titled for what it is, rather than a caption under the button.
	tabs := between(t, page, `id="routineTabs"`, `</div>`)
	if !strings.Contains(tabs, `id="routineSimplifyBtn"`) {
		t.Error("Simplify is not in the routine's tab row, where it is seen without " +
			"opening anything")
	}
	if strings.Contains(between(t, page, `id="routineMaintain"`, `id="routineReviewBtn"`),
		`id="routineSimplifyBtn"`) {
		t.Error("Simplify is under More, which is another place nobody looks")
	}
	// AND THE ACTION IS A VERB. "Review simplification" named the review and buried the
	// thing it does.
	if !strings.Contains(tabs, ">Simplify&hellip;<") {
		t.Errorf("the action is not called Simplify: %q", tabs)
	}
	// AND WHAT IT OPENS IS A REVIEW, said in its heading.
	if !strings.Contains(between(t, page, `id="routineReview"`, `</h3>`), "Review") {
		t.Error("the panel Simplify opens does not say it is a review")
	}

	js := page[strings.Index(page, "<script>"):]
	// AND IT IS SHOWN ON `offer`, NOT ON `update`.
	if !regexp.MustCompile(`showing\(\$\('routineSimplifyBtn'\),[^)]*r\.offer`).MatchString(js) {
		t.Error("Simplify is not shown by `offer`, which is the answer " +
			"about whether the routine still has a demonstration to rebuild from")
	}
	shown := regexp.MustCompile(`showing\(\$\('routineSimplifyBtn'\),\s*(.*)\)`).
		FindStringSubmatch(js)
	if shown != nil && strings.Contains(shown[1], "update") {
		t.Errorf("the simplification is conditioned on `update` (%q), which is about "+
			"the GENERATION — a routine today's rules wrote can still have typing "+
			"pauses in it", shown[1])
	}
	// AND TODAY'S-RULES REVIEW IS THE ONE `update` CONDITIONS, because that IS the
	// question it answers.
	if !regexp.MustCompile(`showing\(\$\('routineReviewBtn'\),[^)]*r\.update`).MatchString(js) {
		t.Error("today's-rules review is not conditioned on `update`")
	}
	// AND THE REVIEW CARD STILL SAYS WHEN A REBUILD WOULD CHANGE NOTHING. Two
	// identical panes with a Replace button over them is a window offering to do
	// nothing, and pressing a button that is safe should still end in a sentence.
	if !strings.Contains(js, "would change nothing") {
		t.Error("the review no longer says when the rebuilt version is identical")
	}
}

// AND NOTHING PROMISES AN APPLICATION THE ROUTINE DOES NOT NAME.
//
// The sentence under Play read `Play brings the application forward and presses for real`, shown
// for every runnable routine — including one saved to work ANYWHERE, which names no application
// and brings nothing forward. For that routine the promise was simply false, and the truth is
// the thing worth saying: nothing is raised, so the presses go to whatever holds the foreground.
func TestPlayNeverPromisesAnApplicationARoutineDoesNotName(t *testing.T) {
	page := thePage(t)
	// NOT IN THE MARKUP. A sentence typed into the element is one sentence for every
	// routine, which is how this went wrong.
	el := between(t, page, `id="routinePlayMeans"`, "</div>")
	if strings.Contains(el, "application forward") {
		t.Errorf("the sentence under Play is written into the page, so it says the "+
			"same thing about every routine: %q", el)
	}
	js := page[strings.Index(page, "<script>"):]
	means := between(t, js, "function playMeans(r){", "\n }")
	if !strings.Contains(means, "r.app") {
		t.Error("what Play means is not decided by whether the routine names an " +
			"application")
	}
	// THE APP-LESS ARM SAYS WHERE THE PRESSES GO, and does not name an application.
	arms := strings.SplitN(means, "?", 2)
	if len(arms) != 2 {
		t.Fatalf("playMeans is not one choice between two sentences: %q", means)
	}
	appless := arms[1][strings.Index(arms[1], ":"):]
	for _, must := range []string{"in front", "names no application"} {
		if !strings.Contains(appless, must) {
			t.Errorf("a routine with no application is told %q, which does not "+
				"mention %q", appless, must)
		}
	}
}

// AND THE PANEL SAYS WHERE A ROUTINE CAN BE ASKED FOR, WHICH IS NOT WHERE IT WORKS.
//
// `resolveroute.Eligible` is one rule for the ask box, the widget and `marco dispatch`: a global
// routine and one that brings its own application forward are eligible from anywhere, and an
// in-place routine ONLY while its application is the context. Nothing in this window said so, so
// a routine saved in place was simply missing when somebody asked for it from anywhere else —
// and the library looked as though it had lost something.
func TestThePanelSaysWhereARoutineCanBeAskedFor(t *testing.T) {
	page := thePage(t)
	js := page[strings.Index(page, "<script>"):]
	asked := between(t, js, "function askedFor(r){", "\n }")
	if !strings.Contains(asked, "r.app") || !strings.Contains(asked, "app-anywhere") {
		t.Errorf("where a routine can be asked for is not decided by its scope: %q",
			asked)
	}
	// AND PLAY IS UNAFFECTED, SAID IN BOTH ARMS. Choosing a card is not asking for
	// anything by name, and a person reading "only while Notepad is in front" beside a
	// live Play button has been told two things that contradict each other.
	if strings.Count(asked, "Play works from") != 2 {
		t.Errorf("one of the two answers does not say that Play is unaffected: %q",
			asked)
	}
	// AND THE SCOPE CHOOSER SAYS IT TOO, at the moment the choice is made rather than
	// a week later.
	opts := between(t, js, "[['app',", "].forEach")
	if !strings.Contains(opts, "only offered while") {
		t.Errorf("the scope chooser says where a routine works and not where it can "+
			"be reached: %q", opts)
	}
}

// AND THE REVIEW SAYS IT WILL NOT RUN, IN THE PAGE, ABOVE THE REASON.
//
// This is the other end of a copy cut. `whyItWillNotRun` used to say three things — the count,
// the measurement, and that the routine would REFUSE — under a heading that had just said the
// third of them and above a button that said it again. The refusal clause came out of the
// backend sentence, and a test in `recording_test.go` stopped asserting the word.
//
// Which leaves the SCREEN as the only thing that says it, so the screen is where it is held: the
// heading exists, it reads as a refusal, it is immediately above the reason, and it appears under
// the same condition — a box shown on `r.missing` with the reason inside it, rather than two
// elements with two conditions, one of which can be false while the other is true.
func TestTheReviewSaysItWillNotRunAboveTheReason(t *testing.T) {
	page := thePage(t)
	box := between(t, page, `id="capWontRunBox"`, `id="capUnsupported"`)
	if !strings.Contains(box, "This routine won't run.") {
		t.Errorf("the review does not say the routine will not run above the reason "+
			"it will not: %q", box)
	}
	if !before(box, "won't run", `id="capMissing"`) {
		t.Error("the reason comes before the heading that makes sense of it")
	}
	// ONE CONDITION FOR BOTH, which is what the box is for: a heading that can stand
	// over nothing, or a reason that can stand under nothing, is the state this
	// arrangement exists to make impossible.
	js := page[strings.Index(page, "<script>"):]
	if !regexp.MustCompile(`showing\(\$\('capWontRunBox'\),\s*!!r\.missing\)`).MatchString(js) {
		t.Error("the won't-run heading is no longer shown by the same answer that " +
			"fills the reason")
	}
	if regexp.MustCompile(`showing\(\$\('capMissing'\)`).MatchString(js) {
		t.Error("the reason has acquired a condition of its own, so it can be shown " +
			"without the heading or hidden under it")
	}
}

// AND THE BUTTON SAYS `Save anyway` ONLY WHEN SAVING IS ALLOWED AND THE ROUTINE WILL NOT RUN.
//
// Two states that read alike and are not: `incomplete` SAVES and then refuses to press, so
// keeping it is a real choice somebody can make knowingly. `unsupported` does not save at all, so
// a button offering to save it anyway would offer something that cannot happen.
//
// The label decides nothing — an invalid scope and a failed replacement-consent check are refused
// at the write, after this — so what is held here is the label itself and the guard on it.
func TestSaveAnywayIsOfferedOnlyWhereSavingIsAllowed(t *testing.T) {
	js := thePage(t)[strings.Index(thePage(t), "<script>"):]

	label := regexp.MustCompile(
		`text\(\$\('capSaveBtn'\),\s*\n?\s*\(([^)]*)\)\s*\?\s*'Save anyway'\s*:\s*'Save'\)`)
	m := label.FindStringSubmatch(js)
	if m == nil {
		t.Fatal("the Save button's label is not chosen between `Save anyway` and `Save` " +
			"in one expression, so what decides it cannot be read here")
	}
	cond := strings.ReplaceAll(m[1], " ", "")
	if !strings.Contains(cond, "r.incomplete") {
		t.Error("`Save anyway` is not conditioned on the recording being incomplete")
	}
	if !strings.Contains(cond, "!r.unsupported") {
		t.Errorf("`Save anyway` is not guarded against an unsupported recording (%q). "+
			"That one does not save at all, so offering to save it anyway offers "+
			"something that cannot happen.", m[1])
	}

	// AND REPLACEMENT STILL SAYS REPLACE, naming what it replaces. `Save anyway` is about
	// a routine that will not run; it is not a second way to overwrite one.
	if !regexp.MustCompile(`id="capReplaceBtn"[^>]*>Replace<`).MatchString(thePage(t)) {
		t.Error("the replacement button no longer says `Replace`")
	}
}

// AND EVERY KIND THAT PRESSES SOMEWHERE HAS A VERB — all of them, not a list written here.
//
// # Why this is a second test
//
// The one above asks whether the page can OPEN each sentence the backend writes, and a kind with
// no sentence is allowed to fall through to its own name and be rendered whole. That tolerance is
// right for a row of text and wrong for a press, because the step editor draws a press
// differently: it REPLACES the value half of the row with the two boxes somebody moves it by. So
// the only part of the sentence left on the row is the verb, and a sentence with no verb leaves a
// number, a badge, two fields and nothing saying what happens at that place.
//
// `move` was exactly that. `orchestrator.PointIn` has always offered a bare cursor move the same
// edit a click gets, and `oneStep` had no sentence for it — so it fell through to the word `move`,
// which the page then threw away. Nobody could have read that row.
//
// The kinds are read out of `macroir`'s own const block rather than listed here, because the next
// one of these is a twelfth kind that presses somewhere.
func TestEveryKindThatPressesSomewhereHasAVerb(t *testing.T) {
	kinds := stepKindsInTheEngine(t)
	if len(kinds) < 10 {
		t.Fatalf("only %d step kinds were read out of macroir, so this is checking "+
			"almost nothing: %v", len(kinds), kinds)
	}
	verbs := stepVerbsInThePage(t)
	pressing := 0
	for _, k := range kinds {
		// EVERY FIELD A POSITION COULD COME FROM, because what is being asked is
		// whether this KIND presses anywhere, not whether this particular step does.
		s := macroir.Step{Kind: k, X: 100, Y: 200, X2: 300, Y2: 400, Count: 2,
			Ms: 250, Key: "down", Text: "hello"}
		if !orchestrator.PointIn([]macroir.Step{s}, 0).Is {
			continue
		}
		pressing++
		line := oneStep(s)
		if verb, _ := openedBy(verbs, line); verb == "" {
			t.Errorf("a %q step presses somewhere, so the editor replaces what its "+
				"sentence says about where it is with two boxes — and %q is "+
				"not a sentence the page has a verb for, so the row ends up "+
				"saying nothing at all about what it does there.", k, line)
		}
	}
	if pressing < 3 {
		t.Fatalf("only %d of %d kinds press anywhere, and a click, a move and a drag "+
			"all do — so this test is not reaching the thing it is about",
			pressing, len(kinds))
	}
}

// stepKindsInTheEngine reads the kind names out of macroir's own const block.
//
// By the module's replace directive the engine is at `../..`, which is also how the page's own
// file is found. A list written here instead would be a list that silently stops being complete.
func stepKindsInTheEngine(t *testing.T) []macroir.StepKind {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "macroir", "step.go"))
	if err != nil {
		t.Fatalf("the engine's step kinds cannot be read: %v", err)
	}
	var out []macroir.StepKind
	for _, m := range regexp.MustCompile(`StepKind = "([a-z]+)"`).
		FindAllStringSubmatch(string(src), -1) {
		out = append(out, macroir.StepKind(m[1]))
	}
	return out
}

// stepVerbsInThePage reads the page's own prefix table, in its own order.
func stepVerbsInThePage(t *testing.T) [][2]string {
	t.Helper()
	page := thePage(t)
	i := strings.Index(page, "const stepVerbs = [")
	if i < 0 {
		t.Fatal("the page has no `stepVerbs` table")
	}
	// THE ARRAY'S OWN CLOSING BRACKET, not the first `]` in it: every entry ends with one,
	// so cutting at the first left a table of nothing and the guard read zero verbs.
	block := page[i:]
	end := strings.Index(block, "\n ]")
	if end < 0 {
		t.Fatal("the `stepVerbs` table does not end where this expects it to")
	}
	block = block[:end]
	var out [][2]string
	for _, m := range regexp.MustCompile(`\['([^']*)',\s*'([^']*)'\]`).
		FindAllStringSubmatch(block, -1) {
		out = append(out, [2]string{m[1], m[2]})
	}
	return out
}

// openedBy is the page's rule, in Go: the first prefix that opens the line wins.
func openedBy(verbs [][2]string, line string) (verb, rest string) {
	for _, v := range verbs {
		if strings.HasPrefix(line, v[0]) {
			return v[1], line[len(v[0]):]
		}
	}
	return "", line
}

// A ROUTINE ROW SAYS ONLY WHAT CHANGES THE NEXT ACTION.
//
// The one exception that does is a routine that cannot run, and it is badged `Can't run`.
// The window checks are not on the row in either direction: `Window-checked` read as an outcome
// somebody could rely on, and `Unchecked` as a verification step somebody had missed. The
// owner: "Window checks are implementation detail, not a useful routine status." So the line
// that chooses the word may not read the contract at all, which is also what stops a new
// classification taking their place. The checks happen underneath, unchanged.
//
// And no word that claims an outcome, which is the rule this test used to be: Marco presses
// and reports what the press returned, and does not know a routine did what somebody wanted.
// The rendered half is TestARoutineRowShowsABadgeOnlyWhenItCannotRun.
func TestARoutineRowSaysOnlyWhatChangesTheNextAction(t *testing.T) {
	js := thePage(t)[strings.Index(thePage(t), "<script>"):]
	i := strings.Index(js, "const status = ")
	if i < 0 {
		t.Fatal("the row's status words are not chosen in one place any more")
	}
	line := js[i : strings.Index(js[i:], "\n")+i]

	for _, word := range []string{"Verified", "Confirmed", "Works", "Passed",
		"Succeeded", "Checked", "checked", "Unchecked", "Window-checked"} {
		if strings.Contains(line, "'"+word+"'") || strings.Contains(line, `"`+word+`"`) {
			t.Errorf("a routine row is labelled %s. A row says only what changes "+
				"what somebody does next: %s", word, line)
		}
	}
	if strings.Contains(line, "contract") {
		t.Errorf("the row's badge reads the routine's contract, which is how its "+
			"presses are checked. That is not a status to put on a row: %s", line)
	}
	if !strings.Contains(line, `!r.runnable ? ['no', "Can't run"] : null`) {
		t.Errorf("the row's badge is not `Can't run` for a routine that cannot run "+
			"and nothing otherwise: %s", line)
	}
	// AND THE LINE BESIDE PLAY DOES NOT SAY IT EITHER.
	for _, s := range []string{"and checks it before each press.'",
		"Its presses are not checked.'"} {
		if strings.Contains(js, s) {
			t.Errorf("the page still says %q beside Play", s)
		}
	}
	if !strings.Contains(js, "const says = row ? 'Brings this window forward.' :") {
		t.Error("the line beside Play no longer just names the window it brings forward")
	}
}

// AND SPLITTING A STEP INTO A ROW NEVER OMITS PART OF THE ACTION.
//
// The page shows a step as number, verb and value, by stripping a known prefix off the sentence
// the backend wrote. That is only safe while the prefix is a VERB PHRASE: the moment a prefix
// could contain a digit, a quote or a bracket, stripping it would delete part of what the step
// does, and the row would describe a different action from the one that was recorded.
//
// So the invariant is about the prefixes themselves rather than about any one step: letters,
// spaces and hyphens only. A prefix that cannot carry data cannot drop any.
func TestSplittingAStepCanNeverDropPartOfIt(t *testing.T) {
	verbs := stepVerbsInThePage(t)
	safe := regexp.MustCompile(`^[a-z][a-z \-]*$`)
	for _, v := range verbs {
		if !safe.MatchString(v[0]) {
			t.Errorf("the page strips %q off a step to find its verb. A prefix that "+
				"is not plain words could carry part of the action, and "+
				"stripping it would show a step that does less than the one "+
				"recorded.", v[0])
		}
		if v[1] == "" {
			t.Errorf("the prefix %q has no verb to show in its place", v[0])
		}
	}
	// AND AN UNRECOGNISED SENTENCE IS KEPT WHOLE, which is the other half: a step this
	// page has no verb for still has to be shown in full.
	page := thePage(t)
	if !strings.Contains(page, "v.className = 'val whole'") ||
		!strings.Contains(page, "v.textContent = s") {
		t.Error("an unrecognised step is no longer rendered as its whole sentence")
	}
}
