package fusion

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Every belief justifying itself.
//
// The guiding claim these tests protect: if the Director believes something about the
// desktop, it can justify that belief from structured evidence rather than from
// implementation details. The dangerous failure is not a missing explanation — that is
// obvious — but a PLAUSIBLE one that does not match what actually happened, because a
// plausible explanation gets believed.

// twoSourceCycle is one button seen by accessibility and read by OCR, a neighbouring
// control, and the group that contains them both — none of which may merge with it.
func twoSourceCycle(t *testing.T) observation.Cycle {
	t.Helper()
	at := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	obs := []directorapi.Observation{
		{
			ID: "acc:save", Source: directorapi.SourceAccessibility, Timestamp: at,
			WindowID: "w1", Role: directorapi.RoleButton, Label: "Save",
			Bounds: directorapi.Rect{X: 100, Y: 100, Width: 80, Height: 30}, Confidence: 1,
			NativeID: "uia:1.1",
		},
		{
			ID: "ocr:save", Source: directorapi.SourceOCR, Timestamp: at,
			WindowID: "w1", Text: "Save",
			Bounds: directorapi.Rect{X: 110, Y: 108, Width: 50, Height: 14}, Confidence: 0.9,
		},
		{
			ID: "acc:cancel", Source: directorapi.SourceAccessibility, Timestamp: at,
			WindowID: "w1", Role: directorapi.RoleButton, Label: "Cancel",
			Bounds: directorapi.Rect{X: 190, Y: 100, Width: 80, Height: 30}, Confidence: 1,
			NativeID: "uia:1.2",
		},
		// The containing group. Accessibility trees NEST — a button sits inside a group
		// inside a pane — so an observation overlapping another from the same source is
		// the ordinary case, not a corner one. This is what the same-source rule exists
		// to refuse, and refusing it is what keeps a toolbar from collapsing into one
		// element.
		{
			ID: "acc:group", Source: directorapi.SourceAccessibility, Timestamp: at,
			WindowID: "w1", Role: directorapi.RoleGroup, Label: "Buttons",
			Bounds: directorapi.Rect{X: 95, Y: 95, Width: 180, Height: 40}, Confidence: 1,
			NativeID: "uia:1.0",
		},
	}

	c := observation.Cycle{ID: "cyc-explain", StartedAt: at, CompletedAt: at}
	for _, o := range obs {
		c.Observations = append(c.Observations, observation.NewElement(o))
	}
	return c
}

// explained fuses a cycle and returns its explanation.
func explained(t *testing.T, e Explainer, c observation.Cycle) explain.CycleExplanation {
	t.Helper()
	if _, _, err := e.Fuse(c); err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	return e.Explain(c)
}

// ── 1. every element explains itself ──────────────────────────────────────────

func TestEveryElementCanAccountForItself(t *testing.T) {
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))

	if len(cx.Elements) != 3 {
		t.Fatalf("%d elements explained, want 3", len(cx.Elements))
	}
	if cx.Unexplained != 0 {
		t.Errorf("%d elements could not be named", cx.Unexplained)
	}

	for _, ex := range cx.Elements {
		if ex.ElementID == "" {
			t.Error("an explanation names no element")
		}
		if ex.PrimaryObservation.Observation == "" {
			t.Errorf("%s has no primary observation — nothing to point at as the reason it exists", ex.ElementID)
		}
		// The primary must be the STRONGEST source. Structure outranks pixels, and an
		// explanation that named the OCR read as primary would be describing a
		// different algorithm from the one that ran.
		if ex.PrimaryObservation.Source != directorapi.SourceAccessibility {
			t.Errorf("%s: primary is %q, want the strongest source",
				ex.ElementID, ex.PrimaryObservation.Source)
		}
		if len(ex.MergeSteps) == 0 {
			t.Errorf("%s records no merge decisions — not even the one that created it", ex.ElementID)
		}
		if ex.IdentityReason.Rule == "" {
			t.Errorf("%s cannot say how it got its identity", ex.ElementID)
		}
		if ex.Confidence.Total <= 0 {
			t.Errorf("%s has no confidence derivation", ex.ElementID)
		}
		if !ex.Confidence.Consistent() {
			t.Errorf("%s: confidence derivation does not sum to its own total: %+v",
				ex.ElementID, ex.Confidence)
		}
	}
}

// ── 2. determinism ────────────────────────────────────────────────────────────

func TestExplainingTheSameCycleTwiceGivesIdenticalOutput(t *testing.T) {
	// The property the whole on-demand design rests on. If re-running clustering did
	// not reproduce the original, an explanation would be a plausible story about a
	// different run — and there would be no way to tell from the output.
	e := NewEngine().(Explainer)
	cycle := twoSourceCycle(t)

	first := explained(t, e, cycle)
	second := e.Explain(cycle)

	if !reflect.DeepEqual(first, second) {
		t.Fatal("two explanations of one cycle differ")
	}

	// And byte-identical once serialised, which is what a caller diffing two runs
	// actually depends on. DeepEqual would pass on maps that marshal in either order.
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Error("explanations serialise differently")
	}
}

func TestExplanationOrderDoesNotDependOnMapIteration(t *testing.T) {
	// Go randomises map iteration deliberately. Anything that lists elements must sort,
	// or it produces a different answer every run — and this one is compared across
	// runs by definition.
	e := NewEngine().(Explainer)
	cycle := twoSourceCycle(t)
	base := explained(t, e, cycle)

	for i := 0; i < 20; i++ {
		got := e.Explain(cycle)
		for j := range got.Elements {
			if got.Elements[j].ElementID != base.Elements[j].ElementID {
				t.Fatalf("run %d: element order changed at %d", i, j)
			}
		}
	}
}

// ── 3. merge explanation ──────────────────────────────────────────────────────

func TestAMergeIsExplainedByTheRuleThatCausedIt(t *testing.T) {
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))

	save, ok := findByLabel(cx, "Save")
	if !ok {
		t.Fatal("no Save element")
	}
	if len(save.Supporting) != 1 {
		t.Fatalf("%d supporting observations, want the OCR read", len(save.Supporting))
	}
	if save.Supporting[0].Source != directorapi.SourceOCR {
		t.Errorf("supporting source = %q", save.Supporting[0].Source)
	}

	var seeded, accepted *explain.MergeDecision
	for i := range save.MergeSteps {
		switch save.MergeSteps[i].Outcome {
		case explain.MergeSeeded:
			seeded = &save.MergeSteps[i]
		case explain.MergeAccepted:
			accepted = &save.MergeSteps[i]
		}
	}
	if seeded == nil {
		t.Error("the observation that created this element is not recorded as having done so")
	}
	if accepted == nil {
		t.Fatal("the merge that added OCR is not recorded")
	}
	if accepted.Against == nil {
		t.Error("the merge does not say what it merged with")
	}
	if accepted.Score <= mergeThreshold {
		t.Errorf("an accepted merge scored %.2f, at or below the threshold %.2f",
			accepted.Score, mergeThreshold)
	}
	// The rule name is the machine-readable half and must describe the actual signals.
	if !strings.Contains(accepted.Rule, "bounds") {
		t.Errorf("rule = %q, want it to name the signal that decided", accepted.Rule)
	}
	if accepted.Reason == "" {
		t.Error("the merge has no human-readable reason")
	}
}

// ── 4. rejection explanation ──────────────────────────────────────────────────

func TestAnObservationRefusedByAnElementSaysWhy(t *testing.T) {
	// The half of fusion that leaves no trace in the result. Cancel sits beside Save;
	// the Save element considered it and refused, and without this the only visible
	// fact is that there are two elements.
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))

	save, _ := findByLabel(cx, "Save")
	if len(save.Rejected) == 0 {
		t.Fatal("nothing was recorded as refused, though a neighbouring control was considered")
	}
	var sawGroup bool
	for _, r := range save.Rejected {
		if r.Rule == "" || r.Reason == "" {
			t.Errorf("a rejection has no rule or reason: %+v", r)
		}
		if r.Label == "Buttons" {
			sawGroup = true
			// Same source: one enumeration of the desktop cannot corroborate itself,
			// and this is the rule that keeps a toolbar from collapsing into the
			// container that happens to overlap it.
			if r.Rule != "same_source" {
				t.Errorf("the containing group was refused by %q, want same_source", r.Rule)
			}
		}
	}
	if !sawGroup {
		t.Error("the containing group does not appear among the refused")
	}
}

func TestARefusalBetweenUnrelatedObservationsIsNotRecorded(t *testing.T) {
	// Noise control, and the reason the explanation is readable at all. With a few
	// hundred observations there are tens of thousands of pairs that never had anything
	// to do with each other; recording them would bury the handful that matter.
	at := time.Now()
	far := observation.Cycle{ID: "cyc-far", StartedAt: at}
	for i, r := range []directorapi.Rect{
		{X: 0, Y: 0, Width: 20, Height: 20},
		{X: 900, Y: 900, Width: 20, Height: 20},
	} {
		far.Observations = append(far.Observations, observation.NewElement(directorapi.Observation{
			ID:     directorapi.ObservationID("acc:" + string(rune('a'+i))),
			Source: directorapi.SourceAccessibility, Timestamp: at,
			WindowID: "w1", Role: directorapi.RoleButton, Label: "Go",
			Bounds: r, Confidence: 1,
		}))
	}

	e := NewEngine().(Explainer)
	cx := explained(t, e, far)
	for _, ex := range cx.Elements {
		if len(ex.Rejected) != 0 {
			t.Errorf("%s recorded %d refusals against observations it never plausibly matched",
				ex.ElementID, len(ex.Rejected))
		}
	}
}

// ── 5. confidence explanation ─────────────────────────────────────────────────

func TestConfidenceIsDerivedRatherThanAsserted(t *testing.T) {
	e := NewEngine().(Explainer)
	world, _, _ := e.Fuse(twoSourceCycle(t))
	cx := e.Explain(twoSourceCycle(t))

	save, _ := findByLabel(cx, "Save")
	c := save.Confidence

	if c.Base <= 0 {
		t.Error("no base confidence recorded")
	}
	if len(c.Contributions) < 2 {
		t.Fatalf("%d contributions; a corroborated element has a base and a corroboration",
			len(c.Contributions))
	}
	if !c.Consistent() {
		t.Errorf("the derivation does not sum to its own total: %+v", c)
	}

	// The explanation must match the element the Director actually planned against.
	// A derivation that is internally consistent and disagrees with the world would be
	// the worst outcome here: entirely plausible and wrong.
	var actual float64
	for _, el := range world.Elements {
		if el.Label == "Save" {
			actual = el.Confidence
		}
	}
	if diff := actual - c.Total; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("explained confidence %.4f, element has %.4f", c.Total, actual)
	}

	// Corroboration always helps and never hurts.
	for _, contrib := range c.Contributions[1:] {
		if contrib.Delta < 0 && contrib.Source != "cap" {
			t.Errorf("%s contributed %.3f — corroboration must not reduce confidence",
				contrib.Source, contrib.Delta)
		}
		if contrib.Reason == "" {
			t.Errorf("contribution from %s has no reason", contrib.Source)
		}
	}
}

func TestALoneSourceExplainsItsConfidenceWithoutInventingCorroboration(t *testing.T) {
	at := time.Now()
	lone := observation.Cycle{ID: "cyc-lone", StartedAt: at,
		Observations: []observation.Observation{
			observation.NewElement(directorapi.Observation{
				ID: "acc:1", Source: directorapi.SourceAccessibility, Timestamp: at,
				WindowID: "w1", Role: directorapi.RoleButton, Label: "Only",
				Bounds: directorapi.Rect{X: 0, Y: 0, Width: 40, Height: 20}, Confidence: 1,
			}),
		},
	}
	e := NewEngine().(Explainer)
	cx := explained(t, e, lone)

	c := cx.Elements[0].Confidence
	if len(c.Contributions) != 1 {
		t.Fatalf("%d contributions for a single source, want just the base", len(c.Contributions))
	}
	if c.Contributions[0].Delta != 0 {
		t.Errorf("the base contribution has a delta of %.2f; it IS the base", c.Contributions[0].Delta)
	}
	if c.Total != c.Base {
		t.Errorf("total %.2f differs from base %.2f with nothing corroborating", c.Total, c.Base)
	}
}

// ── 6. identity explanation ───────────────────────────────────────────────────

func TestIdentityExplainsWhetherItWasCarriedForwardAndHow(t *testing.T) {
	e := NewEngine().(Explainer)
	cycle := twoSourceCycle(t)

	first := explained(t, e, cycle)
	for _, ex := range first.Elements {
		if ex.IdentityReason.MatchedPrevious {
			t.Errorf("%s claims to have matched a previous cycle on the first cycle", ex.ElementID)
		}
		if ex.IdentityReason.Rule != "new" {
			t.Errorf("%s: rule = %q, want new", ex.ElementID, ex.IdentityReason.Rule)
		}
		if !strings.Contains(ex.IdentityReason.Reason, "first cycle") {
			t.Errorf("%s: reason = %q, want it to name the absence of a prior cycle",
				ex.ElementID, ex.IdentityReason.Reason)
		}
	}

	// A second identical cycle: every element should be recognised, and say by which
	// tier. The fixture carries native ids, which settle continuity outright.
	second := explained(t, e, cycle)
	for _, ex := range second.Elements {
		if !ex.IdentityReason.MatchedPrevious {
			t.Errorf("%s was not recognised across identical cycles", ex.ElementID)
		}
		if ex.IdentityReason.Rule != "native_id" {
			t.Errorf("%s matched via %q, want native_id", ex.ElementID, ex.IdentityReason.Rule)
		}
		if ex.IdentityReason.PreviousElement == nil {
			t.Errorf("%s says it matched but not what it matched", ex.ElementID)
		} else if *ex.IdentityReason.PreviousElement != ex.ElementID {
			t.Errorf("%s inherited %s — an inherited id must BE the id",
				ex.ElementID, *ex.IdentityReason.PreviousElement)
		}
	}
}

func TestDurabilityIsReportedSeparatelyFromMatching(t *testing.T) {
	// The distinction that makes "click that again" debuggable. An element can match
	// perfectly this cycle and still have nothing that survives the dialog being
	// reopened — and that failure is silent, because a re-resolved target looks like a
	// successful lookup.
	at := time.Now()
	anon := observation.Cycle{ID: "cyc-anon", StartedAt: at,
		Observations: []observation.Observation{
			observation.NewElement(directorapi.Observation{
				ID: "acc:1", Source: directorapi.SourceAccessibility, Timestamp: at,
				WindowID: "w1", Role: directorapi.RoleButton,
				Bounds: directorapi.Rect{X: 0, Y: 0, Width: 40, Height: 20}, Confidence: 1,
				NativeID: "uia:9.9",
			}),
		},
	}
	e := NewEngine().(Explainer)
	explained(t, e, anon)
	cx := explained(t, e, anon)

	id := cx.Elements[0].IdentityReason
	if !id.MatchedPrevious {
		t.Fatal("the element was not recognised via its runtime id")
	}
	if id.Stable {
		t.Error("an unlabelled element with no authored id is not durable, " +
			"however well its runtime id matched")
	}
}

// ── 7. missing element and provider failure ───────────────────────────────────

func TestExplainingAnUnknownElementFindsNothingRatherThanInventingIt(t *testing.T) {
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))

	if _, ok := cx.Find("e999"); ok {
		t.Error("an element that does not exist was explained")
	}
	if _, ok := cx.Find(""); ok {
		t.Error("an empty id matched something")
	}
}

func TestACycleTheEngineNeverFusedIsExplainedWithoutInventingIdentity(t *testing.T) {
	// Clustering is reproducible; identity is not. An explanation for a cycle the
	// engine has not seen must say so rather than mint an id that never existed.
	e := NewEngine().(Explainer)
	cx := e.Explain(twoSourceCycle(t))

	if cx.Unexplained == 0 {
		t.Fatal("a cycle that was never fused was explained as though it had been")
	}
	for _, ex := range cx.Elements {
		if ex.ElementID != "" {
			t.Errorf("an element id was invented: %s", ex.ElementID)
		}
		if ex.IdentityReason.Rule != "unknown" {
			t.Errorf("identity rule = %q, want unknown", ex.IdentityReason.Rule)
		}
		// The reproducible half is still there, which is the point of saying so.
		if len(ex.MergeSteps) == 0 {
			t.Error("the clustering, which IS reproducible, was not explained")
		}
	}
}

func TestAnEmptyCycleExplainsNothingAndDoesNotFail(t *testing.T) {
	e := NewEngine().(Explainer)
	cx := e.Explain(observation.Cycle{ID: "cyc-empty"})
	if len(cx.Elements) != 0 {
		t.Errorf("%d elements explained from no evidence", len(cx.Elements))
	}
	if cx.Cycle != "cyc-empty" {
		t.Errorf("cycle = %q", cx.Cycle)
	}
}

func TestADegradedCycleStillExplainsWhatItDidSee(t *testing.T) {
	// A provider failed; the evidence that did arrive is real and must still be
	// accountable. An explanation layer that gave up on a degraded cycle would be
	// unavailable exactly when it is most wanted.
	cycle := twoSourceCycle(t)
	cycle.Failures = []directorapi.SourceFailure{
		{Source: directorapi.SourceAccessibility, Reason: "node cap reached"},
	}
	e := NewEngine().(Explainer)
	cx := explained(t, e, cycle)

	if len(cx.Elements) == 0 {
		t.Fatal("a degraded cycle explained nothing, though it carried evidence")
	}
	for _, ex := range cx.Elements {
		if ex.PrimaryObservation.Observation == "" {
			t.Errorf("%s has no primary observation", ex.ElementID)
		}
	}
}

// ── 8. JSON ───────────────────────────────────────────────────────────────────

func TestTheExplanationSurvivesJSONWithoutLosingItsReasoning(t *testing.T) {
	// The wire format is how a client sees this at all — the CLI is a separate process
	// from the service. A field that silently drops in transit would make the JSON
	// output quietly less trustworthy than the human one.
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))

	raw, err := json.Marshal(cx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back explain.CycleExplanation
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(cx, back) {
		t.Error("the explanation did not survive a round trip intact")
	}

	// Spot-check the fields a reader actually depends on, because DeepEqual on two
	// empty structures would also pass.
	save, ok := back.Find(mustFindID(t, cx, "Save"))
	if !ok {
		t.Fatal("the Save element did not survive")
	}
	if save.Confidence.Total == 0 || len(save.Confidence.Contributions) == 0 {
		t.Error("the confidence derivation was lost")
	}
	if len(save.MergeSteps) == 0 {
		t.Error("the merge decisions were lost")
	}
	if save.IdentityReason.Reason == "" {
		t.Error("the identity reasoning was lost")
	}
	if !strings.Contains(string(raw), "primary_observation") {
		t.Error("the JSON does not expose the primary observation")
	}
}

// ── 9. history eviction ───────────────────────────────────────────────────────

func TestTheIdentityLogIsEvictedInLockstepWithTheObservationHistory(t *testing.T) {
	// Retaining identity for a cycle whose evidence is gone would be retaining
	// something nothing can be said about. Both bounds are five, and they must agree.
	e := NewEngine().(Explainer)

	var cycles []observation.Cycle
	for i := 0; i < observation.DefaultHistory+3; i++ {
		c := twoSourceCycle(t)
		c.ID = observation.CycleID("cyc-" + string(rune('a'+i)))
		cycles = append(cycles, c)
		if _, _, err := e.Fuse(c); err != nil {
			t.Fatalf("Fuse: %v", err)
		}
	}

	// The newest are still explainable, with real element ids.
	for _, c := range cycles[len(cycles)-observation.DefaultHistory:] {
		cx := e.Explain(c)
		if cx.Unexplained != 0 {
			t.Errorf("cycle %s is within the bound but %d elements are unexplained",
				c.ID, cx.Unexplained)
		}
	}
	// The oldest have aged out, and say so rather than reporting a stale id.
	for _, c := range cycles[:3] {
		cx := e.Explain(c)
		if cx.Unexplained == 0 {
			t.Errorf("cycle %s should have aged out of the identity log", c.ID)
		}
	}
}

// ── 10. rendering ─────────────────────────────────────────────────────────────

func TestTheRenderedExplanationCarriesTheReasoningNotJustTheNumbers(t *testing.T) {
	e := NewEngine().(Explainer)
	cx := explained(t, e, twoSourceCycle(t))
	save, _ := findByLabel(cx, "Save")

	out := explain.Render(save)
	for _, want := range []string{
		"Evidence", "primary", "accessibility",
		"Merge decisions", "Identity", "Confidence",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendering is missing %q:\n%s", want, out)
		}
	}
	// A bare number with no derivation is the thing this milestone exists to remove.
	if !strings.Contains(out, "base") {
		t.Errorf("confidence was rendered without its derivation:\n%s", out)
	}

	chain := explain.RenderChain(save, nil)
	for _, want := range []string{"Element", "Source", "Observation", "Fusion", "Identity", "Replay"} {
		if !strings.Contains(chain, want) {
			t.Errorf("the chain is missing %q:\n%s", want, chain)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func findByLabel(cx explain.CycleExplanation, label string) (explain.ElementExplanation, bool) {
	for _, e := range cx.Elements {
		if e.Label == label {
			return e, true
		}
	}
	return explain.ElementExplanation{}, false
}

func mustFindID(t *testing.T, cx explain.CycleExplanation, label string) directorapi.ElementID {
	t.Helper()
	e, ok := findByLabel(cx, label)
	if !ok {
		t.Fatalf("no element labelled %q", label)
	}
	return e.ElementID
}
