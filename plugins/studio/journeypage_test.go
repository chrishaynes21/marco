package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
		{`id="askBox"`, `id="recordStart"`,
			"the request box must come before the way in to recording: it is the one " +
				"control that answers what somebody came here to do"},
		{`id="recordStart"`, `id="routineList"`,
			"recording comes before the library, which is the order of the journey"},
		{`id="routineList"`, `id="routineChosen"`,
			"the panel about the selected routine comes after the list it is selected from"},
	} {
		if !before(routines, c.first, c.then) {
			t.Errorf("%s is not before %s. %s", c.first, c.then, c.why)
		}
	}

	// ── and inside the selected panel, which is where Play was buried ──
	chosen := between(t, routines, `id="routineChosen"`, `<h2>`)
	order := []string{
		`id="routineIdentity"`, // what is selected
		`id="routineChecks"`,   // whether its presses are checked
		`id="routineWhy"`,      // and why it cannot run, when it cannot
		`id="routinePlayBtn"`,  // PLAY
		`id="routineSource"`,   // what it will do, collapsed
		`id="routineMaintain"`, // rebuild and re-record, collapsed
	}
	for i := 0; i+1 < len(order); i++ {
		if !before(chosen, order[i], order[i+1]) {
			t.Errorf("%s must come before %s in the selected routine's panel",
				order[i], order[i+1])
		}
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
		"routineChecks",        // whether its presses are checked at all
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

	starts := regexp.MustCompile(`<button[^>]*>\s*Record something new`).
		FindAllString(routines, -1)
	if len(starts) != 1 {
		t.Errorf("the journey has %d `Record something new` buttons, want exactly 1",
			len(starts))
	}
	// AND THE FORM IT REVEALS IS NOT ALSO OPEN. `captureCard` must start hidden, or the
	// button reveals something that was never away.
	card := between(t, routines, `id="captureCard"`, `</div>`)
	if !strings.Contains(startOf(routines, `id="captureCard"`), "hide") {
		t.Errorf("the record form does not start hidden, so `Record something new` "+
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

	order := []string{
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
