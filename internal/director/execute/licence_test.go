package execute

// WHAT AN EXPLICIT INSTRUCTION MAY AND MAY NOT AUTHORISE.
//
// `ActuatingSource` admits accessibility, a native integration, a DOM bridge and a capability
// pack. A vision detector and an OCR reader describe pixels, so a Prop only they saw reports
// `Actuable: false` and the resolver refuses it. That rule made every control in an application
// with no accessibility tree unreachable except by demonstration.
//
// The owner's decision, and the whole of what is tested here:
//
//	An explicit user instruction MAY authorize one action on a pixel-only Prop without a
//	prior demonstration. However, authorization and grounding must remain separate: the
//	instruction authorizes the requested action, not a guessed target.
//
// Every case below drives the real `Pipeline.Handle` — the same observe → parse → resolve →
// plan → policy → execute → verify path a live request takes.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// seenBy builds one element reported by the given sources and nothing else.
func seenBy(id, label string, at directorapi.Rect,
	sources ...directorapi.ObservationSource) *directorapi.Element {

	el := &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: directorapi.RoleButton,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
		Bounds:     at,
		Attributes: map[string]any{"patterns": "invoke", "native_id": id},
	}
	for i, s := range sources {
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID(id + "-" + string(s) +
				string(rune('a'+i))),
			Source: s,
		})
	}
	return el
}

// overStage is a harness whose Stage is re-observed as often as the pipeline asks, each time a
// second later than the last.
//
// The pipeline observes more than twice for one request — a before-state, a re-check at the
// instant of acting, and an after-state to compare — and a Stage that stops advancing reads as
// "the after-state is not newer", which is a fixture artefact and not a finding.
func overStage(els func() []*directorapi.Element) *harness {
	var worlds []directorapi.WorldState
	for i := range 6 {
		on := els()
		// AND THE STAGE CHANGES AFTER THE ACTION, because a click that changed
		// nothing is correctly refused — "the click produced no observable
		// change" is the verifier doing its job, and a fixture that never moved
		// would be testing that refusal rather than the licence.
		if i >= 3 {
			on = append(on, seenBy("after", "Afterwards",
				directorapi.Rect{X: 5, Y: 900, Width: 40, Height: 10},
				directorapi.SourceAccessibility))
		}
		worlds = append(worlds, stageOf(on...))
	}
	return newHarness(worlds...)
}

// proposedShape is a structural proposal as the production provider emits one: a rectangle,
// no role, no label, no patterns. `seenBy` above fabricates a button and would not be a
// faithful stand-in — it was tried, and it made this very test pass for the wrong reason.
func proposedShape(id string, at directorapi.Rect) *directorapi.Element {
	el := &directorapi.Element{
		ID: directorapi.ElementID(id), Role: directorapi.RoleUnknown,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 0.3,
		Bounds: at,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: directorapi.ObservationID(id + "-structure"),
		Source:      directorapi.SourceStructure,
	})
	return el
}

// stageTick advances the Stage's clock, so an after-state is newer than the before-state the
// verifier compares it with.
var stageTick int

// stageOf is a world holding the given elements, observed well.
func stageOf(els ...*directorapi.Element) directorapi.WorldState {
	stageTick++
	byID := map[directorapi.ElementID]*directorapi.Element{}
	for _, el := range els {
		byID[el.ID] = el
	}
	w := directorapi.WorldState{
		Timestamp: t0.Add(
			time.Duration(stageTick) * time.Second),
		Elements: byID,
		Windows: []directorapi.Window{
			{ID: "hwnd:1", Application: "fixture", Focused: true},
		},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	w.ActiveWindow = &active
	return w
}

var (
	// A menu entry an accessibility tree described — the Notepad case.
	fileMenu = func() *directorapi.Element {
		return seenBy("file", "File",
			directorapi.Rect{X: 10, Y: 10, Width: 60, Height: 24},
			directorapi.SourceAccessibility)
	}
	// A tile a detector boxed and OCR read — the game case. Nothing can say how to
	// operate it.
	playTile = func() *directorapi.Element {
		return seenBy("play", "Play",
			directorapi.Rect{X: 400, Y: 500, Width: 300, Height: 200},
			directorapi.SourceVision, directorapi.SourceOCR)
	}
	// A banner carrying the same word, read by OCR alone: a glyph run, not a control.
	playBanner = func() *directorapi.Element {
		return seenBy("banner", "Play 10 Competitive Doubles Matches",
			directorapi.Rect{X: 1520, Y: 600, Width: 300, Height: 40},
			directorapi.SourceOCR)
	}
)

// 1. ACCESSIBILITY-BACKED REQUESTS ARE UNCHANGED.
func TestAnAccessibilityBackedClickIsUnaffectedByTheLicence(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{fileMenu()}
	})
	out := h.pipeline.Handle(context.Background(), "click File")

	acted(t, h, out, "file")
}

// 2. A PIXEL-ONLY PROP IS REACHABLE WHEN THE INSTRUCTION NAMES THE ACTION.
func TestAnExplicitInstructionAuthorisesOneActionOnAPixelOnlyProp(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{playTile()}
	})
	out := h.pipeline.Handle(context.Background(), "click Play")

	acted(t, h, out, "play")
}

// 3. AND THE SAME PROP IS REFUSED WHEN NOBODY ASKED FOR IT.
//
// The licence is a fact about the request. Without one — here, a resolution asked directly, the
// way every other caller in the tree asks — the pixel-only Prop is not a target.
func TestWithoutAnInstructionAPixelOnlyPropIsStillRefused(t *testing.T) {
	w := stageOf(playTile())
	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Text: "play"})

	if res.Status == directorapi.ResolutionResolved {
		t.Fatalf("a pixel-only Prop resolved with no licence: %+v", res.Target)
	}
	if !mentions(res, "only pixels describe it") {
		t.Errorf("the refusal does not say why: %s / %+v", res.Explanation,
			res.Candidates)
	}
}

// 4. AMBIGUITY STILL REFUSES, LICENCE OR NOT.
//
// "Click Play" authorises a click. It does not say WHICH play, and an instruction that named
// the action must not be read as having chosen the target.
func TestAnExplicitInstructionDoesNotResolveAmbiguity(t *testing.T) {
	second := func() *directorapi.Element {
		return seenBy("play2", "Play",
			directorapi.Rect{X: 900, Y: 500, Width: 300, Height: 200},
			directorapi.SourceVision, directorapi.SourceOCR)
	}
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{playTile(), second()}
	})
	out := h.pipeline.Handle(context.Background(), "click Play")

	if out.Status == directorapi.ResultDone {
		t.Fatalf("two Props named Play and one was chosen anyway: %+v",
			out.Resolution)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Fatalf("%d action(s) reached the runtime", n)
	}
}

// 5. READ TEXT ALONE IS NOT A CLICKABLE SURFACE.
//
// OCR reading a word says a glyph run is at a rectangle. The rectangle of a word is a caption,
// not the control it labels — the lesson of ADR-201 one tier up, where pressing the middle of a
// word put the pointer 17 px from the edge of the tile it named.
func TestReadTextAloneIsNotEnoughToAuthoriseAClick(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{playBanner()}
	})
	out := h.pipeline.Handle(context.Background(), "click Play")

	if out.Status == directorapi.ResultDone {
		t.Fatalf("a region only OCR saw was clicked: %+v", out.Resolution)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Fatalf("%d action(s) reached the runtime", n)
	}
}

// AND THE BANNER LOSES TO THE TILE when both are on the Stage.
//
// The ambiguity refusal must not fire here: only one of the two is a control at all.
func TestAWordOnABannerIsNotAContenderAgainstTheControlThatSaysIt(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{playTile(), playBanner()}
	})
	out := h.pipeline.Handle(context.Background(), "click Play")

	acted(t, h, out, "play")
}

// 6. MISSING OR STALE GEOMETRY REFUSES.
func TestAPixelOnlyPropWithNoUsableGeometryIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		el   *directorapi.Element
	}{{
		name: "no bounds at all",
		el: seenBy("play", "Play", directorapi.Rect{},
			directorapi.SourceVision, directorapi.SourceOCR),
	}, {
		name: "offscreen",
		el: func() *directorapi.Element {
			el := playTile()
			el.Offscreen = true
			return el
		}(),
	}, {
		name: "not visible",
		el: func() *directorapi.Element {
			el := playTile()
			el.Visible = false
			return el
		}(),
	}} {
		t.Run(c.name, func(t *testing.T) {
			h := overStage(func() []*directorapi.Element {
				return []*directorapi.Element{c.el}
			})
			out := h.pipeline.Handle(context.Background(), "click Play")
			// REFUSED AT GROUNDING, not later by the capability ladder.
			//
			// Where a thing is is a grounding question, so it has to be
			// answered where grounding is answered. A Prop that resolves
			// and is then refused for having no bounds has been offered
			// as a target it was never eligible to be.
			if out.Resolution != nil &&
				out.Resolution.Status == directorapi.ResolutionResolved {
				t.Fatalf("a Prop with no usable geometry resolved: %+v",
					out.Resolution.Target)
			}
			if len(h.operations.ops) != 0 {
				t.Fatalf("%d action(s) reached the runtime",
					len(h.operations.ops))
			}
		})
	}
}

// 7. A LICENCE COVERS ONE VERB, AND CANNOT BE SPENT ON ANOTHER.
func TestALicenceForOneVerbDoesNotCoverAnother(t *testing.T) {
	w := stageOf(playTile())
	click := directorapi.OperateLicence{Instruction: "click Play", Verb: "click"}

	ok, _ := w.Elements["play"].OperableUnder(click, "click")
	if !ok {
		t.Fatal("the licence does not cover the verb it was granted for")
	}
	if ok, why := w.Elements["play"].OperableUnder(click, "type"); ok {
		t.Errorf("a licence to click authorised a type: %s", why)
	}
	// AND THE ZERO VALUE AUTHORISES NOTHING.
	if ok, _ := w.Elements["play"].OperableUnder(directorapi.OperateLicence{},
		"click"); ok {
		t.Error("the zero licence authorised an action")
	}
}

// AND IT IS NOT RETAINED BETWEEN REQUESTS.
//
// Structural rather than enforced: the licence is a local value built inside one call of
// `prepare` from that call's own request. There is no field to store it in and nothing stores
// it. This holds the structure by asking the second request to stand on its own.
func TestALicenceIsNotCarriedIntoTheNextRequest(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{playTile(), playBanner()}
	})

	acted(t, h, h.pipeline.Handle(context.Background(), "click Play"), "play")
	spent := len(h.operations.ops) + len(h.actuator.clicks)
	// A SECOND REQUEST NAMING NOTHING ON THE STAGE. If the first request's licence
	// had been kept, this would still be carrying permission.
	out := h.pipeline.Handle(context.Background(), "click Quit")
	if out.Resolution != nil && out.Resolution.Status == directorapi.ResolutionResolved {
		t.Fatalf("a request for an absent control resolved: %+v", out.Resolution)
	}
	if now := len(h.operations.ops) + len(h.actuator.clicks); now != spent {
		t.Fatalf("%d more operation(s) reached the runtime for a control that is "+
			"not on the Stage", now-spent)
	}
}

// acted asserts that the request resolved to `want` and that an operation reached the runtime.
//
// Not that the whole request reported `done`: verification compares world states, and whether a
// fixture's Stage changes convincingly after a click is a property of the fixture rather than of
// the licence. What the licence decides is whether this Prop could be targeted and the action
// performed at all, and that is what is asserted.
func acted(t *testing.T, h *harness, out Outcome, want directorapi.ElementID) {
	t.Helper()
	switch {
	case out.Resolution == nil || out.Resolution.Target == nil:
		t.Fatalf("nothing resolved: %s — %s", out.Status, out.Message)
	case out.Resolution.Status != directorapi.ResolutionResolved:
		t.Fatalf("resolution %s: %s", out.Resolution.Status,
			out.Resolution.Explanation)
	case out.Resolution.Target.ElementID != want:
		t.Fatalf("resolved to %q, want %q", out.Resolution.Target.ElementID, want)
	case out.Status == directorapi.ResultBlocked:
		t.Fatalf("policy refused: %s", out.Message)
	case len(h.operations.ops) == 0 && len(h.actuator.clicks) == 0:
		// EITHER RECORDER. A control with an invoke pattern is operated through
		// the operations runner; one that can only be clicked by its bounds --
		// which is every pixel-only Prop -- reaches the actuator as a point.
		t.Fatalf("nothing reached the runtime: %s — %s", out.Status, out.Message)
	}
}

// mentions reports whether a resolution says `what`, in its explanation or in why it rejected
// the candidates it found.
func mentions(res directorapi.Resolution, what string) bool {
	if strings.Contains(res.Explanation, what) {
		return true
	}
	for _, c := range res.Candidates {
		if strings.Contains(c.Rejected, what) {
			return true
		}
	}
	return false
}

// A STRUCTURAL PROPOSAL DOES NOT DISTURB A PROP AN ACTUATING SOURCE DESCRIBED.
//
// The shape proposer finds drawn rectangles everywhere, including over controls accessibility
// already reported. The integration is only safe if those Props keep winning: a request for a
// control an application vouched for must not start resolving to a rectangle beside it, and
// must not become ambiguous because a second source drew a box in the same place.
func TestAStructuralProposalDoesNotDisturbAnAccessibilityProp(t *testing.T) {
	// The File menu as accessibility reports it, and a structural region over the same
	// part of the screen — the ordinary case, not a contrived one.
	overlapping := proposedShape("shape",
		directorapi.Rect{X: 8, Y: 8, Width: 70, Height: 28})
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{fileMenu(), overlapping}
	})

	out := h.pipeline.Handle(context.Background(), "click File")
	acted(t, h, out, "file")

	// AND THE STRUCTURAL PROP IS STILL THERE, still not operable: present on the Stage
	// and not a target. A Prop may exist without being operable.
	w := h.worlds[0]
	shape := w.Elements["shape"]
	if shape == nil {
		t.Fatal("the structural Prop vanished from the Stage")
	}
	if shape.Actions().Targetable() {
		t.Error("a structural Prop is a valid click target")
	}
	if ok, _ := shape.OperableUnder(directorapi.OperateLicence{
		Instruction: "click File", Verb: "click"}, "click"); ok &&
		shape.Actions().Any() {
		t.Error("an explicit instruction made a bare rectangle operable")
	}
}

// AND AN UNNAMED STRUCTURAL PROP DOES NOT ANSWER A NAMED REQUEST.
func TestAnUnnamedStructuralPropDoesNotMatchANamedInstruction(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{proposedShape("shape",
			directorapi.Rect{X: 400, Y: 500, Width: 300, Height: 60})}
	})
	out := h.pipeline.Handle(context.Background(), "click Resume")

	if out.Resolution != nil && out.Resolution.Status == directorapi.ResolutionResolved {
		t.Fatalf("an unnamed rectangle answered a request for \"Resume\": %+v",
			out.Resolution.Target)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Fatalf("%d action(s) reached the runtime", n)
	}
}
