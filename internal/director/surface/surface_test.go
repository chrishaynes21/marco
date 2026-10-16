package surface

// EVERY WAY THE QUESTION CAN BE ANSWERED, AND THE ONE IT MUST NEVER BE.
//
// The decision is three-valued and the middle value is the point: AMBIGUOUS is what makes Marco
// teachable, and a design that reached DEFENSIBLE by picking the better-looking rectangle would
// be worse than one that asks. So the tests below are arranged by answer, and the last group is
// the things that must never become defensible however well they are drawn.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

const (
	winW = 1920
	winH = 1080
)

// prop builds one Prop with the sources that reported it.
func prop(id, label string, at directorapi.Rect,
	sources ...directorapi.ObservationSource) *directorapi.Element {

	el := &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: directorapi.RoleUnknown,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 0.99,
		Bounds: at,
	}
	for i, s := range sources {
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID(id + string(rune('a'+i))),
			Source:      s,
		})
	}
	return el
}

func stage(els ...*directorapi.Element) directorapi.WorldState {
	byID := map[directorapi.ElementID]*directorapi.Element{}
	for _, el := range els {
		byID[el.ID] = el
	}
	id := directorapi.WindowID("hwnd:1")
	return directorapi.WorldState{
		Timestamp: time.Now(), Elements: byID,
		Windows: []directorapi.Window{{ID: id, Application: "fixture",
			Bounds:  directorapi.Rect{Width: winW, Height: winH},
			Focused: true, Visible: true}},
		ActiveWindow: &id,
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 0.5, Coverage: 0, Actionability: 0, Freshness: 1},
	}
}

func ask(w directorapi.WorldState, words string) Hypothesis {
	return New().Evaluate(&w, "click "+words, "click",
		directorapi.ElementQuery{Text: words})
}

// rect is the measured Trackmania geometry, which every nesting case here is built from.
var (
	theWord = directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}
	theRow  = directorapi.Rect{X: 98, Y: 750, Width: 444, Height: 56}
)

// ── ONE DEFENSIBLE SURFACE ────────────────────────────────────────────────────

// THE CASE THE WHOLE DESIGN IS FOR: the words identify, and the thing around them is pressed.
//
// The word's region is accounted for by read text alone, so it is a caption and is refused by
// [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s own rule.
// The row is accounted for by the shape proposer, contains nothing else named, and locates
// something — so exactly one surface survives and it is not the one the words were read on.
func TestTheRowIsChosenWhenTheWordsAreOnlyACaption(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
	)
	h := ask(w, "settings")

	if h.Decision != DecisionDefensible {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if h.Chosen.ElementID != "row" {
		t.Errorf("chose %q, want the row", h.Chosen.ElementID)
	}
	if h.Chosen.Point != theRow.Center() {
		t.Errorf("the press is at %v, not the surface's own middle %v",
			h.Chosen.Point, theRow.Center())
	}
	if h.Unsettled != "" {
		t.Errorf("a settled answer offers something to teach: %q", h.Unsettled)
	}
}

// AND WHEN THE SURVIVOR IS THE WORDS THEMSELVES, IT SAYS SO.
//
// A toolbar button is about the size of its label, so pressing the words can be exactly right —
// and [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] is the record of it being
// exactly wrong. The evidence cannot tell those apart, so the sentence must not pretend to.
func TestPressingTheWordsIsReportedAsPressingTheWords(t *testing.T) {
	w := stage(prop("word", "Details",
		directorapi.Rect{X: 1028, Y: 100, Width: 42, Height: 28},
		directorapi.SourceStructure, directorapi.SourceOCR))
	h := ask(w, "details")

	if h.Decision != DecisionDefensible {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if !h.Chosen.Evidence.IsIdentity {
		t.Fatal("the fixture no longer exercises the identity-as-surface case")
	}
	for _, want := range []string{"the region the words themselves were read on",
		"may be the control or may be its caption"} {
		if !strings.Contains(h.Explanation, want) {
			t.Errorf("the explanation does not say %q: %s", want, h.Explanation)
		}
	}
	if h.Unsettled == "" {
		t.Error("a press on the words is exactly what a person could settle, and " +
			"nothing is offered to teach")
	}
}

// ── AMBIGUOUS ─────────────────────────────────────────────────────────────────

// THE MEASURED TRACKMANIA CASE, and the answer the owner asked for.
//
// Both the word's own region and the row around it are accounted for by the shape proposer, both
// contain nothing else named, both locate something. Nothing in the evidence chooses, so nothing
// here chooses: it reports two and names what a person would settle in one click.
func TestTwoNestedSurfacesAreAmbiguousAndNotRanked(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
	)
	h := ask(w, "settings")

	if h.Decision != DecisionAmbiguous {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if h.Chosen != nil {
		t.Errorf("an ambiguous hypothesis chose %q anyway", h.Chosen.ElementID)
	}
	if h.Unsettled != "which surface is the control" {
		t.Errorf("teachable is %q", h.Unsettled)
	}
	defensible := 0
	for _, c := range h.Candidates {
		if c.Defensible() {
			defensible++
		}
	}
	if defensible != 2 {
		t.Errorf("%d defensible candidate(s), want both", defensible)
	}
	// AND THE EVIDENCE FOR EACH IS REPORTED, because the explanation is the deliverable
	// when the decision is not.
	for _, c := range h.Candidates {
		if c.Evidence.Containment <= 0 {
			t.Errorf("%s reports no containment", c.ElementID)
		}
	}
}

// AND TWO PEERS ARE AMBIGUOUS BY FUSION'S OWN RULE, not by a second one written here.
func TestTwoComparableSurfacesAreAmbiguous(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceStructure, directorapi.SourceOCR),
		prop("almost", "", directorapi.Rect{X: 386, Y: 762, Width: 170, Height: 26},
			directorapi.SourceStructure),
	)
	h := ask(w, "settings")

	if h.Decision != DecisionAmbiguous {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if !strings.Contains(h.Explanation, "comparable size") &&
		!strings.Contains(h.Explanation, "equally defensible") {
		t.Errorf("the explanation does not say why: %s", h.Explanation)
	}
}

// AND AN UNSETTLED IDENTITY IS NOT A SURFACE PROBLEM.
//
// Two Props named alike about equally well: no amount of surface reasoning repairs that, and the
// resolver's own margin is the authority so the same request refuses here and there for one
// reason.
func TestTwoPropsNamedAlikeRefuseBeforeAnySurface(t *testing.T) {
	w := stage(
		prop("a", "Settings", theWord, directorapi.SourceStructure, directorapi.SourceOCR),
		prop("b", "Settings", directorapi.Rect{X: 1200, Y: 300, Width: 162, Height: 22},
			directorapi.SourceStructure, directorapi.SourceOCR),
	)
	h := ask(w, "settings")

	if h.Decision != DecisionAmbiguous {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if h.Unsettled != "which of these the request meant" {
		t.Errorf("teachable is %q; an identity problem is a different question "+
			"from a surface one", h.Unsettled)
	}
}

// ── INSUFFICIENT ──────────────────────────────────────────────────────────────

func TestNothingNamedLikeTheRequest(t *testing.T) {
	h := ask(stage(prop("a", "Scoreboard", theWord, directorapi.SourceStructure)), "settings")
	if h.Decision != DecisionInsufficient {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if h.Identity != nil {
		t.Errorf("it identified %q for a word that is not there", h.Identity.Label)
	}
}

func TestAnEmptyStageAnswersInsufficient(t *testing.T) {
	w := stage()
	if h := ask(w, "settings"); h.Decision != DecisionInsufficient {
		t.Errorf("%s — %s", h.Decision, h.Explanation)
	}
	if h := New().Evaluate(nil, "click settings", "click",
		directorapi.ElementQuery{Text: "settings"}); h.Decision != DecisionInsufficient {
		t.Errorf("a nil world: %s", h.Decision)
	}
}

// A MATCH THE RESOLVER WOULD NOT OFFER IS NOT AN IDENTITY.
//
// Measured on the recorded Windows Settings page: "click pointer" matched a Prop OCR had labelled
// "te", because a label CONTAINED IN the request scores 0.5 and "te" is inside "pointer". It
// scored 0.174 against the resolver's MinScore of 0.25, and this package reported a confident
// "one defensible surface" naming a two-letter fragment. The resolver's own threshold was the
// right answer; asking it is the fix.
func TestAFragmentTheResolverWouldNotOfferIsNotAnIdentity(t *testing.T) {
	w := stage(prop("frag", "te", directorapi.Rect{X: 32, Y: 392, Width: 92, Height: 24},
		directorapi.SourceStructure, directorapi.SourceOCR))
	h := ask(w, "pointer")

	if h.Decision != DecisionInsufficient {
		t.Fatalf("a two-letter OCR fragment answered %q for \"pointer\": %s",
			h.Decision, h.Explanation)
	}
	if h.Identity != nil {
		t.Errorf("it identified %q", h.Identity.Label)
	}
}

// ── AND THE THINGS THAT MUST NEVER BE DEFENSIBLE ──────────────────────────────

// AN OVERSIZED REGION STANDS FOR SEVERAL THINGS AT ONCE.
//
// The measured 48%-of-window Trackmania background, which carries a NAME of its own — which is
// the strongest reading of the safety test, because every rule keyed on having a name admits it.
func TestAnOversizedNamedRegionIsNotASurface(t *testing.T) {
	w := stage(
		prop("big", "OPPONENTS",
			directorapi.Rect{X: 482, Y: 388, Width: 1438, Height: 692},
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("inner", "TRACKMANIA ASIA CUP",
			directorapi.Rect{X: 1266, Y: 744, Width: 354, Height: 60},
			directorapi.SourceStructure, directorapi.SourceOCR),
	)
	h := ask(w, "opponents")

	if h.Decision == DecisionDefensible {
		t.Fatalf("a 48%%-of-window region is a defensible surface: %+v", h.Chosen)
	}
	if !strings.Contains(h.Explanation, "several named things") {
		t.Errorf("the refusal does not say why: %s", h.Explanation)
	}
}

// A TOOLBAR BAND CONTAINS EVERY CONTROL ON THE TOOLBAR.
//
// The owner's own test of readiness: "if the new logic can justify clicking that toolbar band
// simply because it is rectangular and near text, it isn't ready."
func TestABandOfChromeIsNotASurface(t *testing.T) {
	w := stage(
		prop("details", "Details",
			directorapi.Rect{X: 1028, Y: 100, Width: 42, Height: 28},
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("band", "", directorapi.Rect{X: 8, Y: 90, Width: 1900, Height: 44},
			directorapi.SourceStructure),
		prop("sort", "Sort", directorapi.Rect{X: 300, Y: 100, Width: 60, Height: 28},
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("view", "View", directorapi.Rect{X: 400, Y: 100, Width: 60, Height: 28},
			directorapi.SourceStructure, directorapi.SourceOCR),
	)
	h := ask(w, "details")

	for _, c := range h.Candidates {
		if c.ElementID == "band" && c.Defensible() {
			t.Error("a band holding every control on the toolbar is a defensible " +
				"surface")
		}
	}
	if h.Chosen != nil && h.Chosen.ElementID == "band" {
		t.Fatal("the band was chosen as the place to press")
	}
}

// A BAND ACROSS THE WINDOW HAS NO POSITION IN IT.
//
// `observe.Region.Locates` is the workflow stack's rule and this is the same rule, reached
// through the same function: the middle of a full-width band is the middle of the WINDOW, which
// says nothing about where anything inside it sits.
func TestARegionSpanningTheWindowLocatesNothing(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceOCR),
		prop("band", "", directorapi.Rect{X: 0, Y: 750, Width: winW, Height: 56},
			directorapi.SourceStructure),
	)
	h := ask(w, "settings")

	for _, c := range h.Candidates {
		if c.ElementID != "band" {
			continue
		}
		if c.Defensible() {
			t.Error("a band running the full width of the window is a defensible " +
				"surface")
		}
		if !strings.Contains(c.Rejected, "full width") {
			t.Errorf("the refusal does not say why: %q", c.Rejected)
		}
	}
	if h.Decision == DecisionDefensible {
		t.Errorf("with only a caption and a band, something was still chosen: %+v",
			h.Chosen)
	}
}

// AND A CAPTION ALONE IS NOT A SURFACE.
func TestReadTextAloneIsNotASurface(t *testing.T) {
	h := ask(stage(prop("word", "SETTINGS", theWord, directorapi.SourceOCR)), "settings")
	if h.Decision != DecisionInsufficient {
		t.Fatalf("%s — %s", h.Decision, h.Explanation)
	}
	if !strings.Contains(h.Explanation, "locates a word and not the control") {
		t.Errorf("the refusal does not use ADR-202's own finding: %s", h.Explanation)
	}
}

// ── AND IT CHANGES NOTHING ────────────────────────────────────────────────────

// READ-ONLY, asserted rather than claimed.
//
// A Prop whose role or label this had quietly adjusted would be a perception change wearing a
// reasoning change's clothes, and the whole premise of
// [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] is that the Stage claims
// nothing.
func TestTheEvaluatorChangesNothing(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
	)
	type snap struct {
		role  directorapi.ElementRole
		label string
		b     directorapi.Rect
		conf  float64
		srcs  int
	}
	before := map[directorapi.ElementID]snap{}
	for id, el := range w.Elements {
		before[id] = snap{el.Role, el.Label, el.Bounds, el.Confidence,
			len(el.Provenance.Sources)}
	}

	for range 3 {
		ask(w, "settings")
		ask(w, "scoreboard")
	}

	if len(w.Elements) != len(before) {
		t.Fatalf("the Stage went from %d Props to %d", len(before), len(w.Elements))
	}
	for id, el := range w.Elements {
		was := before[id]
		if el.Role != was.role || el.Label != was.label || el.Bounds != was.b ||
			el.Confidence != was.conf || len(el.Provenance.Sources) != was.srcs {
			t.Errorf("%s was changed by being reasoned about", id)
		}
	}
}

// AND IT NEVER INVENTS A POSITION.
//
// Every proposed point is a candidate's own middle. A point chosen anywhere else inside a
// rectangle would be Marco deciding where a control is on evidence it does not have, which is
// what `Region.Locates` refuses a band for in the first place.
func TestEveryProposedPointIsASurfacesOwnMiddle(t *testing.T) {
	w := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
		prop("panel", "", directorapi.Rect{X: 80, Y: 600, Width: 500, Height: 300},
			directorapi.SourceStructure),
	)
	h := ask(w, "settings")
	if len(h.Candidates) == 0 {
		t.Fatal("no candidates, so this proves nothing")
	}
	for _, c := range h.Candidates {
		if c.Point != c.Bounds.Center() {
			t.Errorf("%s proposes %v, which is not its own middle %v",
				c.ElementID, c.Point, c.Bounds.Center())
		}
	}
}

// AND NEITHER IS A LINE OF TEXT AROUND A WORD.
//
// The companion to the case above, and the one that found a gap in it: the identity itself can be
// properly structural while the thing AROUND it is read text and nothing else. OCR reading a
// whole line — "SETTINGS GRAPHICS", one box — produces exactly that, a larger region containing
// the word and accounted for by nothing but the reading of it.
//
// A caption is not the control whichever end of the nesting it sits at, so the ancestor must be
// refused for the reason it deserves rather than passing because the identity was checked.
func TestALineOfReadTextIsNotASurfaceEither(t *testing.T) {
	// The request is "mouse" and the line around it reads something else, so the line is an
	// ancestor and NOT a competing identity — which is the distinction this case is about.
	// A line that repeated the request's own words would be refused one step earlier, as an
	// unsettled identity, and the surface rule would never be reached.
	w := stage(
		prop("word", "Mouse",
			directorapi.Rect{X: 348, Y: 746, Width: 160, Height: 34},
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("line", "Pointer speed and scrolling",
			directorapi.Rect{X: 340, Y: 740, Width: 700, Height: 60},
			directorapi.SourceOCR),
	)
	h := ask(w, "mouse")

	found := false
	for _, c := range h.Candidates {
		if c.ElementID != "line" {
			continue
		}
		found = true
		if c.Defensible() {
			t.Error("a region only read text accounts for is a defensible surface")
		}
		if !strings.Contains(c.Rejected, "nothing but read text") {
			t.Errorf("the refusal does not say why: %q", c.Rejected)
		}
	}
	if !found {
		t.Fatal("the line was never considered, so this proves nothing")
	}
}
