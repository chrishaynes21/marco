package fusion

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The evidence/belief boundary.
//
// Every test here is about the same rule from a different angle: fusion is the only
// thing that turns an Observation into an Element, and the Element it produces must be
// able to say what it was believed from.

// cycleOf builds a cycle from a recorded desktop, as the fixture path does.
func cycleOf(t *testing.T, name string) observation.Cycle {
	t.Helper()
	d := fixtures.Load(t, name)
	at := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	c := observation.Cycle{
		ID:          observation.NewCycleID(at),
		StartedAt:   at,
		CompletedAt: at.Add(30 * time.Millisecond),
	}
	for _, o := range d.Observations {
		o.Timestamp = at
		c.Observations = append(c.Observations, observation.NewElement(o))
	}
	c.Observations = append(c.Observations,
		observation.Window{
			ObservationID: "win:1", From: directorapi.SourceAccessibility,
			At: at, Detail: d.Window,
		},
		observation.Application{
			ObservationID: "app:1", From: directorapi.SourceAccessibility,
			At: at, Detail: d.App, Active: true, WindowID: d.Window.ID,
		},
	)
	return c
}

// ── 1. fusion pass-through ────────────────────────────────────────────────────

func TestOneSourceFusesThroughToOneElementEach(t *testing.T) {
	// With a single source there is nothing to merge, and fusion must not invent
	// merging to look busy. One observation, one element.
	cycle := cycleOf(t, "save-dialog")
	elements := len(observation.Elements(cycle.Observations))

	w, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) != elements {
		t.Errorf("%d elements from %d element observations", len(w.Elements), elements)
	}
	if report.Merged != 0 {
		t.Errorf("merged = %d; one source cannot corroborate itself", report.Merged)
	}
	if report.ElementCount != len(w.Elements) {
		t.Errorf("report says %d elements, world has %d", report.ElementCount, len(w.Elements))
	}
	if report.ObservationCount != len(cycle.Observations) {
		t.Errorf("report counted %d observations, cycle had %d",
			report.ObservationCount, len(cycle.Observations))
	}
	if report.Cycle != cycle.ID {
		t.Errorf("the report names cycle %q, want %q", report.Cycle, cycle.ID)
	}

	// The window and application evidence became world state rather than elements.
	if len(w.Windows) != 1 {
		t.Errorf("%d windows in the world", len(w.Windows))
	}
	if w.ActiveApp == nil {
		t.Error("the application observation did not become the active app")
	}
	if w.ActiveWindow == nil {
		t.Error("the application observation did not identify the active window")
	}
}

// ── 2. provenance ─────────────────────────────────────────────────────────────

func TestEveryElementRecordsTheEvidenceItCameFrom(t *testing.T) {
	// The definition-of-done check. An element that cannot say why it exists is not
	// auditable, and "why did you click that?" has no answer once the observations
	// themselves have been discarded.
	cycle := cycleOf(t, "save-dialog")
	w, _, _ := NewEngine().Fuse(cycle)

	if len(w.Elements) == 0 {
		t.Fatal("the fixture produced no elements")
	}
	for _, el := range w.Elements {
		if el.Provenance.Len() == 0 {
			t.Fatalf("element %s (%q) has no provenance", el.ID, el.Label)
		}
		for _, ref := range el.Provenance.Sources {
			if ref.Observation == "" {
				t.Errorf("element %s has a provenance entry naming no observation", el.ID)
			}
			// The source and kind travel WITH the reference. Observations are
			// ephemeral; by the time provenance is read the id alone would name
			// nothing.
			if ref.Source == "" {
				t.Errorf("element %s: provenance entry has no source", el.ID)
			}
			if ref.Kind != directorapi.ObservationElement {
				t.Errorf("element %s: provenance kind = %q", el.ID, ref.Kind)
			}
			if ref.Cycle != string(cycle.ID) {
				t.Errorf("element %s: provenance names cycle %q, want %q",
					el.ID, ref.Cycle, cycle.ID)
			}
		}
		if !el.Provenance.Has(directorapi.SourceAccessibility) {
			t.Errorf("element %s does not credit accessibility", el.ID)
		}
	}
}

func TestProvenanceSurvivesCorroborationAndNamesEverySource(t *testing.T) {
	// The shape the whole milestone exists for. Two sources describing one button
	// produce ONE element that credits both — not two elements, and not one that
	// forgot where half its evidence came from.
	at := time.Now()
	acc := directorapi.Observation{
		ID: "acc:1", Source: directorapi.SourceAccessibility, Timestamp: at,
		WindowID: "w1", Role: directorapi.RoleButton, Label: "Save",
		Bounds: directorapi.Rect{X: 100, Y: 100, Width: 80, Height: 30}, Confidence: 1,
	}
	ocr := directorapi.Observation{
		ID: "ocr:1", Source: directorapi.SourceOCR, Timestamp: at,
		WindowID: "w1", Text: "Save",
		Bounds: directorapi.Rect{X: 110, Y: 108, Width: 50, Height: 14}, Confidence: 0.9,
	}

	cycle := observation.Cycle{
		ID: "cyc-test", StartedAt: at, CompletedAt: at,
		Observations: []observation.Observation{
			observation.NewElement(acc), observation.NewElement(ocr),
		},
	}
	w, report, _ := NewEngine().Fuse(cycle)

	if len(w.Elements) != 1 {
		t.Fatalf("%d elements; two accounts of one button are one button", len(w.Elements))
	}
	if report.Merged != 1 {
		t.Errorf("merged = %d, want 1", report.Merged)
	}
	for _, el := range w.Elements {
		if el.Provenance.Len() != 2 {
			t.Fatalf("provenance has %d entries, want both sources", el.Provenance.Len())
		}
		if !el.Provenance.Has(directorapi.SourceAccessibility) || !el.Provenance.Has(directorapi.SourceOCR) {
			t.Errorf("provenance = %+v, want both sources credited", el.Provenance.SourceList())
		}
		// Structure outranks pixels: the accessibility bounds win, so the click lands
		// on the control rather than on the text inside it.
		if el.Bounds != acc.Bounds {
			t.Errorf("bounds = %+v, want the structured source's %+v", el.Bounds, acc.Bounds)
		}
	}
}

func TestProvenanceIgnoresARepeatedObservationID(t *testing.T) {
	// Defensive: a source that reported the same id twice must not double its
	// element's apparent corroboration.
	var p directorapi.Provenance
	ref := directorapi.ObservationReference{Observation: "acc:1", Source: directorapi.SourceAccessibility}
	p.Add(ref)
	p.Add(ref)
	if p.Len() != 1 {
		t.Errorf("Len() = %d, want a repeated id to be ignored", p.Len())
	}
}

// ── 3. merge diagnostics ──────────────────────────────────────────────────────

func TestConflictingSourcesAreResolvedAndTheDisagreementIsRecorded(t *testing.T) {
	// Sources disagree constantly and legitimately. The ladder resolves it; the report
	// is what makes the resolution auditable rather than something to take on faith.
	at := time.Now()
	bounds := directorapi.Rect{X: 100, Y: 100, Width: 80, Height: 30}
	acc := directorapi.Observation{
		ID: "acc:1", Source: directorapi.SourceAccessibility, Timestamp: at,
		WindowID: "w1", Role: directorapi.RoleButton, Label: "Save", Value: "on",
		Bounds: bounds, Confidence: 1,
	}
	// A vision detector in the same place, calling it something else. Compatible role
	// (button/icon), so it merges — and then disagrees about the value.
	vis := directorapi.Observation{
		ID: "vis:1", Source: directorapi.SourceVision, Timestamp: at,
		WindowID: "w1", Role: directorapi.RoleIcon, Value: "off",
		Bounds: bounds, Confidence: 0.6,
	}

	_, report, _ := NewEngine().Fuse(observation.Cycle{
		ID: "cyc-test", StartedAt: at,
		Observations: []observation.Observation{
			observation.NewElement(acc), observation.NewElement(vis),
		},
	})

	if len(report.Conflicts) == 0 {
		t.Fatal("two sources disagreed and nothing recorded it")
	}
	var sawValue bool
	for _, c := range report.Conflicts {
		if c.Element == "" {
			t.Errorf("conflict %q names no element", c.Field)
		}
		if c.Winner.Source != directorapi.SourceAccessibility {
			t.Errorf("conflict %q was won by %q; structure outranks pixels",
				c.Field, c.Winner.Source)
		}
		if c.Field == "value" {
			sawValue = true
			if c.WinnerValue != "on" || c.LoserValue != "off" {
				t.Errorf("value conflict recorded %q over %q", c.WinnerValue, c.LoserValue)
			}
		}
	}
	if !sawValue {
		t.Error("the value disagreement was not recorded")
	}
}

func TestAgreementDecoratedDifferentlyIsNotAConflict(t *testing.T) {
	// "&Save" against "Save" is two toolkits' punctuation, not two controls. Recording
	// it as a conflict would bury the real ones.
	at := time.Now()
	bounds := directorapi.Rect{X: 10, Y: 10, Width: 80, Height: 30}
	_, report, _ := NewEngine().Fuse(observation.Cycle{
		ID: "cyc-test", StartedAt: at,
		Observations: []observation.Observation{
			observation.NewElement(directorapi.Observation{
				ID: "acc:1", Source: directorapi.SourceAccessibility, Timestamp: at,
				WindowID: "w1", Role: directorapi.RoleButton, Label: "&Save...",
				Bounds: bounds, Confidence: 1,
			}),
			observation.NewElement(directorapi.Observation{
				ID: "ocr:1", Source: directorapi.SourceOCR, Timestamp: at,
				WindowID: "w1", Text: "Save", Bounds: bounds, Confidence: 0.9,
			}),
		},
	})
	for _, c := range report.Conflicts {
		if c.Field == "label" {
			t.Errorf("decoration was reported as a label conflict: %q vs %q",
				c.WinnerValue, c.LoserValue)
		}
	}
}

func TestTheReportBreaksEvidenceDownBySourceAndKind(t *testing.T) {
	cycle := cycleOf(t, "save-dialog")
	_, report, _ := NewEngine().Fuse(cycle)

	if report.ByKind[observation.ElementObservation] == 0 {
		t.Error("no element evidence counted")
	}
	if report.ByKind[observation.WindowObservation] != 1 {
		t.Errorf("window evidence counted %d times", report.ByKind[observation.WindowObservation])
	}
	if report.ByKind[observation.ApplicationObservation] != 1 {
		t.Errorf("application evidence counted %d times",
			report.ByKind[observation.ApplicationObservation])
	}
	// A missing provider must show up as a zero rather than as an unexplained
	// shortfall in the total.
	if report.BySource[directorapi.SourceOCR] != 0 {
		t.Error("OCR was counted and nothing emits it")
	}
	total := 0
	for _, n := range report.ByKind {
		total += n
	}
	if total != report.ObservationCount {
		t.Errorf("the breakdown sums to %d, the total says %d", total, report.ObservationCount)
	}
}

// ── 4. degradation reaches belief ─────────────────────────────────────────────

func TestAFailedSourceReachesTheWorldAsDegradationNotAsAbsence(t *testing.T) {
	cycle := cycleOf(t, "save-dialog")
	cycle.Failures = []directorapi.SourceFailure{
		{Source: directorapi.SourceAccessibility, Reason: "node cap reached"},
	}
	w, report, _ := NewEngine().Fuse(cycle)

	if len(w.Degraded) != 1 {
		t.Fatalf("%d degradations in the world, want the failure carried through", len(w.Degraded))
	}
	if len(report.Degraded) != 1 {
		t.Errorf("the report lost the degradation")
	}
	// Coverage is discounted, so a policy gate downstream cannot read a truncated walk
	// as a complete one.
	full, _, _ := NewEngine().Fuse(cycleOf(t, "save-dialog"))
	if w.Confidence.Coverage >= full.Confidence.Coverage {
		t.Errorf("degraded coverage %.2f is not below the clean %.2f",
			w.Confidence.Coverage, full.Confidence.Coverage)
	}
}

func TestAnEmptyCycleProducesAnEmptyWorldAndNotAnError(t *testing.T) {
	w, report, err := NewEngine().Fuse(observation.Cycle{ID: "cyc-empty", StartedAt: time.Now()})
	if err != nil {
		t.Fatalf("an empty cycle is a legitimate state, not an error: %v", err)
	}
	if len(w.Elements) != 0 {
		t.Errorf("%d elements from no evidence", len(w.Elements))
	}
	if report.ElementCount != 0 || report.ObservationCount != 0 {
		t.Errorf("report = %+v, want zeroes", report)
	}
	if w.Timestamp.IsZero() {
		t.Error("even an empty world needs a timestamp — verification compares them")
	}
}

// ── 5. identity across cycles ─────────────────────────────────────────────────

func TestElementIdentityIsStableAcrossCyclesAndOwnedByFusion(t *testing.T) {
	// Providers emit fresh evidence with fresh observation ids every cycle. The
	// ELEMENT ids must not move — "click that again" is built on nothing else.
	e := NewEngine()

	first, _, _ := e.Fuse(cycleOf(t, "save-dialog"))
	second, _, _ := e.Fuse(cycleOf(t, "save-dialog"))

	if len(first.Elements) != len(second.Elements) {
		t.Fatalf("element count changed between identical cycles: %d then %d",
			len(first.Elements), len(second.Elements))
	}
	for id := range first.Elements {
		if _, ok := second.Elements[id]; !ok {
			t.Fatalf("element %s lost its identity between identical cycles", id)
		}
	}
	// A fresh engine starts fresh — identity belongs to a session, not to the process.
	other, _, _ := NewEngine().Fuse(cycleOf(t, "save-dialog"))
	if len(other.Elements) != len(first.Elements) {
		t.Errorf("a new engine produced a different world from the same evidence")
	}
}

// ── 6. merge candidacy ────────────────────────────────────────────────────────

func TestMergeCandidateScoresEvidenceAgainstAnExistingBelief(t *testing.T) {
	el := &directorapi.Element{
		WindowID: "w1", Role: directorapi.RoleButton, Label: "Save",
		Bounds: directorapi.Rect{X: 100, Y: 100, Width: 80, Height: 30},
	}
	inside := observation.NewElement(directorapi.Observation{
		ID: "ocr:1", Source: directorapi.SourceOCR, WindowID: "w1", Text: "Save",
		Bounds: directorapi.Rect{X: 110, Y: 108, Width: 50, Height: 14}, Confidence: 0.9,
	})
	elsewhere := observation.NewElement(directorapi.Observation{
		ID: "ocr:2", Source: directorapi.SourceOCR, WindowID: "w1", Text: "Save",
		Bounds: directorapi.Rect{X: 900, Y: 900, Width: 50, Height: 14}, Confidence: 0.9,
	})

	if s := (MergeCandidate{Observation: inside, ExistingElement: el}).Score(); s <= 0 {
		t.Errorf("score = %.2f; a word inside the button is evidence about the button", s)
	}
	if s := (MergeCandidate{Observation: elsewhere, ExistingElement: el}).Score(); s != 0 {
		t.Errorf("score = %.2f; the same word across the screen is a different object", s)
	}
	// Nothing to reinforce.
	if s := (MergeCandidate{Observation: inside}).Score(); s != 0 {
		t.Errorf("score = %.2f with no existing element", s)
	}
	// The same source cannot corroborate itself: one enumeration reporting two nodes
	// means two nodes.
	same := observation.NewElement(directorapi.Observation{
		ID: "acc:2", Source: directorapi.SourceAccessibility, WindowID: "w1",
		Role: directorapi.RoleButton, Label: "Save", Bounds: el.Bounds,
	})
	if s := (MergeCandidate{Observation: same, ExistingElement: el}).Score(); s != 0 {
		t.Errorf("score = %.2f; accessibility cannot corroborate accessibility", s)
	}
}
