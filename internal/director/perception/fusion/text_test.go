package fusion

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The safety rule, tested from every side it can be broken from:
//
//	Accessibility may establish STRUCTURE.
//	OCR may establish VISIBLE TEXT.
//	Fusion decides whether the evidence describes the same entity.
//	Only structural evidence may establish ACTIONABILITY.
//
// The dangerous direction is generosity. A fusion that attaches too little costs a
// user one anonymous button they must describe another way; a fusion that attaches too
// much renames a control after its neighbour, or turns a heading into something the
// planner will click. Nearly every test here is checking that fusion refused something.

var textAt = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

// accEl builds accessibility evidence for a control.
func accEl(id, label string, role directorapi.ElementRole, r directorapi.Rect) directorapi.Observation {
	enabled, visible := true, true
	return directorapi.Observation{
		ID: directorapi.ObservationID(id), Source: directorapi.SourceAccessibility,
		Timestamp: textAt, WindowID: "w1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Confidence: 1, NativeID: id,
	}
}

// ocrText builds OCR evidence.
func ocrText(id, text string, r directorapi.Rect) observation.Text {
	return observation.Text{
		ObservationID: directorapi.ObservationID(id),
		ProviderID:    "ocr",
		From:          directorapi.SourceOCR,
		At:            textAt,
		Box:           r,
		Score:         0.9,
		Content:       observation.NewText(text),
		WindowID:      "w1",
		ApplicationID: "testapp",
		LineID:        "l1",
	}
}

// textCycle assembles a cycle from element and text evidence.
func textCycle(els []directorapi.Observation, texts []observation.Text) observation.Cycle {
	c := observation.Cycle{ID: "cyc-text", StartedAt: textAt, CompletedAt: textAt}
	for _, e := range els {
		c.Observations = append(c.Observations, observation.NewElement(e))
	}
	for _, t := range texts {
		c.Observations = append(c.Observations, t)
	}
	return c
}

func elementLabelled(w directorapi.WorldState, label string) (*directorapi.Element, bool) {
	for _, el := range w.Elements {
		if el.Label == label {
			return el, true
		}
	}
	return nil, false
}

// ── 1. filling an empty label ─────────────────────────────────────────────────

func TestOCRFillsTheLabelOfAnAnonymousButtonWithoutChangingAnythingElse(t *testing.T) {
	// The opening this whole feature exists for: a structurally real, actionable
	// control that nothing can name.
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 100, Y: 100, Width: 120, Height: 40})
	text := ocrText("ocr:1", "Export", directorapi.Rect{X: 125, Y: 110, Width: 65, Height: 20})

	before, _, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, nil))
	after, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))

	el, ok := elementLabelled(after, "Export")
	if !ok {
		t.Fatal("the anonymous button was not named by the text printed on it")
	}
	if el.Role != directorapi.RoleButton {
		t.Errorf("role = %q; text must never change what an element IS", el.Role)
	}
	// Actionability, bounds and state all still come from accessibility alone.
	if len(after.Elements) != len(before.Elements) {
		t.Errorf("%d elements with text, %d without — text created one",
			len(after.Elements), len(before.Elements))
	}
	if el.Bounds != button.Bounds {
		t.Errorf("bounds = %+v, want the structural box — a click must land on the "+
			"control, not on the glyphs inside it", el.Bounds)
	}
	if !el.Actions().Interactive {
		t.Error("the element stopped being interactive")
	}
	if !el.Provenance.Has(directorapi.SourceOCR) {
		t.Error("the element does not credit OCR for its label")
	}
	if !el.Provenance.Has(directorapi.SourceAccessibility) {
		t.Error("the element stopped crediting accessibility")
	}
	if report.Text.FilledLabel != 1 {
		t.Errorf("filled_label = %d, want 1", report.Text.FilledLabel)
	}
}

// ── 2. reinforcement ──────────────────────────────────────────────────────────

func TestOCRReinforcesAMatchingLabelWithoutRewritingIt(t *testing.T) {
	button := accEl("acc:1", "&Save", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	text := ocrText("ocr:1", "Save", directorapi.Rect{X: 20, Y: 10, Width: 50, Height: 20})

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))
	el, ok := elementLabelled(w, "&Save")
	if !ok {
		t.Fatal("the structural label was replaced; agreement is not a reason to rewrite it")
	}
	if report.Text.Reinforced != 1 {
		t.Errorf("reinforced = %d, want 1", report.Text.Reinforced)
	}
	if !el.Provenance.Has(directorapi.SourceOCR) {
		t.Error("the corroboration was not recorded as provenance")
	}
	// Corroboration raises confidence, and it stays bounded.
	solo, _, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, nil))
	soloEl, _ := elementLabelled(solo, "&Save")
	if el.Confidence <= soloEl.Confidence {
		t.Errorf("confidence %.3f did not rise above the uncorroborated %.3f",
			el.Confidence, soloEl.Confidence)
	}
	if el.Confidence > maxConfidence {
		t.Errorf("confidence %.3f exceeded the cap %.3f", el.Confidence, maxConfidence)
	}
}

// ── 3. conflict ───────────────────────────────────────────────────────────────

func TestAConflictingReadNeverOverwritesTheStructuralLabel(t *testing.T) {
	// The single most dangerous thing this file could do. "Delete" read as "Cancel" is
	// a click nobody intended, made with full confidence.
	button := accEl("acc:1", "Delete", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	text := ocrText("ocr:1", "Cancel", directorapi.Rect{X: 20, Y: 10, Width: 50, Height: 20})

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))
	el, ok := elementLabelled(w, "Delete")
	if !ok {
		t.Fatal("the structural label was overwritten by a conflicting read")
	}
	if report.Text.RejectedConflict != 1 {
		t.Errorf("rejected_conflict = %d, want the disagreement recorded", report.Text.RejectedConflict)
	}
	// Both observations stay visible: the disagreement is the finding.
	if !el.Provenance.Has(directorapi.SourceOCR) {
		t.Error("the conflicting observation was discarded rather than recorded")
	}

	// Confidence in the LABEL falls; confidence that the element EXISTS does not.
	// Two sources disagreeing about a name still agree there is a control there.
	solo, _, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, nil))
	soloEl, _ := elementLabelled(solo, "Delete")
	if el.Confidence < soloEl.Confidence {
		t.Errorf("existence confidence fell from %.3f to %.3f over a naming dispute",
			soloEl.Confidence, el.Confidence)
	}
	if el.LabelConfidence <= 0 || el.LabelConfidence >= el.Confidence {
		t.Errorf("label confidence %.3f should be assessed and below existence confidence %.3f",
			el.LabelConfidence, el.Confidence)
	}
}

// ── 4. OCR alone creates nothing ──────────────────────────────────────────────

func TestTextWithNoStructureUnderItNeverBecomesAControl(t *testing.T) {
	// The core safety rule. Reading "Export" somewhere is not evidence that an Export
	// button exists — it might be a heading, a log line, or the word in a document.
	text := ocrText("ocr:1", "Export", directorapi.Rect{X: 500, Y: 500, Width: 65, Height: 20})

	w, report, _ := NewEngine().Fuse(textCycle(nil, []observation.Text{text}))

	if len(w.Elements) != 0 {
		t.Fatalf("%d elements from text alone — OCR invented a control", len(w.Elements))
	}
	if report.Text.Standalone != 1 {
		t.Errorf("standalone = %d, want the text to have stayed evidence", report.Text.Standalone)
	}
	// And nothing is actionable, because nothing exists.
	if w.Confidence.Actionability != 0 {
		t.Errorf("actionability = %.2f from text alone", w.Confidence.Actionability)
	}
}

func TestAWindowFullOfTextAndNoStructureStaysUnactionable(t *testing.T) {
	// The Discord case. Anonymous panes, no operable controls, and hundreds of words.
	// Coverage may rise; actionability must not, or policy would believe an
	// unreadable application is safely controllable.
	pane := accEl("acc:1", "", directorapi.RoleGroup, directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600})
	var texts []observation.Text
	for i := 0; i < 40; i++ {
		texts = append(texts, ocrText(
			"ocr:"+itoa(i),
			"word", directorapi.Rect{X: 10, Y: 10 + i*14, Width: 40, Height: 12}))
	}

	w, _, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{pane}, texts))

	for _, el := range w.Elements {
		if el.Actions().Interactive {
			t.Errorf("element %s (%q) became interactive from text", el.ID, el.Label)
		}
	}
	if w.Confidence.Actionability != 0 {
		t.Errorf("actionability = %.2f; a window of words is not a window of controls",
			w.Confidence.Actionability)
	}
}

// ── 5. scope gates ────────────────────────────────────────────────────────────

func TestTheSameWordInADifferentWindowDoesNotMerge(t *testing.T) {
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	text := ocrText("ocr:1", "Export", directorapi.Rect{X: 20, Y: 10, Width: 50, Height: 20})
	text.WindowID = "w2" // read in a different window that happens to overlap

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))

	if el, ok := elementLabelled(w, "Export"); ok {
		t.Fatalf("element %s took a label from another window's text", el.ID)
	}
	if report.Text.RejectedScope != 1 {
		t.Errorf("rejected_scope = %d, want the window mismatch recorded", report.Text.RejectedScope)
	}
}

func TestStaleTextDoesNotMerge(t *testing.T) {
	// Text read from a screen that has since changed is not weaker evidence; it is
	// evidence about a different screen.
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	text := ocrText("ocr:1", "Export", directorapi.Rect{X: 20, Y: 10, Width: 50, Height: 20})
	text.At = textAt.Add(-time.Minute)

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))

	if _, ok := elementLabelled(w, "Export"); ok {
		t.Fatal("a minute-old read named a control on the current screen")
	}
	if report.Text.RejectedStale != 1 {
		t.Errorf("rejected_stale = %d", report.Text.RejectedStale)
	}
}

// ── 6. geometry and ambiguity ─────────────────────────────────────────────────

func TestTextOutsideEveryElementStaysStandalone(t *testing.T) {
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	text := ocrText("ocr:1", "Heading", directorapi.Rect{X: 400, Y: 400, Width: 80, Height: 20})

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, []observation.Text{text}))
	if _, ok := elementLabelled(w, "Heading"); ok {
		t.Fatal("text nowhere near a control was attached to it")
	}
	if report.Text.Standalone != 1 {
		t.Errorf("standalone = %d", report.Text.Standalone)
	}
}

func TestTextInsideTwoEquallyPlausibleElementsIsRefused(t *testing.T) {
	// Nested containers make this the common case, not a corner one. Choosing
	// arbitrarily would label a container after the button inside it half the time,
	// and the half it got wrong would be invisible.
	box := directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40}
	a := accEl("acc:1", "", directorapi.RoleButton, box)
	b := accEl("acc:2", "", directorapi.RoleGroup, box)
	text := ocrText("ocr:1", "Which", directorapi.Rect{X: 20, Y: 10, Width: 50, Height: 20})

	w, report, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{a, b}, []observation.Text{text}))

	if el, ok := elementLabelled(w, "Which"); ok {
		t.Fatalf("element %s claimed text that two elements could equally own", el.ID)
	}
	if report.Text.RejectedAmbiguous != 1 {
		t.Errorf("rejected_ambiguous = %d", report.Text.RejectedAmbiguous)
	}
	// The candidates are named, so the ambiguity is diagnosable rather than merely
	// reported.
	for _, d := range report.Text.Decisions {
		if d.Outcome == TextRejectedAmbiguous && len(d.Candidates) < 2 {
			t.Errorf("an ambiguous rejection named %d candidates", len(d.Candidates))
		}
	}
}

// ── 7. grouping ───────────────────────────────────────────────────────────────

func TestAdjacentWordsOnOneLineGroupIntoAPhrase(t *testing.T) {
	save := ocrText("ocr:1", "Save", directorapi.Rect{X: 10, Y: 10, Width: 40, Height: 16})
	as := ocrText("ocr:2", "As", directorapi.Rect{X: 54, Y: 10, Width: 20, Height: 16})

	got := GroupLine([]observation.Text{save, as})
	if len(got) != 1 {
		t.Fatalf("%d groups, want the two words joined", len(got))
	}
	if got[0].Content.Raw != "Save As" {
		t.Errorf("phrase = %q", got[0].Content.Raw)
	}
	if got[0].Box.Width < 60 {
		t.Errorf("the group's box %+v does not span both words", got[0].Box)
	}
}

func TestDistantWordsAreNotConcatenated(t *testing.T) {
	// "Save" beside "Cancel" must not become "Save Cancel" — a string that appears
	// nowhere, matches nothing, and makes both controls unfindable by their real names.
	save := ocrText("ocr:1", "Save", directorapi.Rect{X: 10, Y: 10, Width: 40, Height: 16})
	cancel := ocrText("ocr:2", "Cancel", directorapi.Rect{X: 300, Y: 10, Width: 60, Height: 16})

	got := GroupLine([]observation.Text{save, cancel})
	if len(got) != 2 {
		t.Fatalf("%d groups, want the distant words kept apart", len(got))
	}
}

func TestWordsOnDifferentLinesAreNotGrouped(t *testing.T) {
	a := ocrText("ocr:1", "First", directorapi.Rect{X: 10, Y: 10, Width: 40, Height: 16})
	b := ocrText("ocr:2", "Second", directorapi.Rect{X: 54, Y: 60, Width: 50, Height: 16})
	b.LineID = "l2"

	if got := GroupLine([]observation.Text{a, b}); len(got) != 2 {
		t.Fatalf("%d groups, want two lines kept apart", len(got))
	}
}

func TestGroupingIsDeterministic(t *testing.T) {
	words := []observation.Text{
		ocrText("ocr:3", "Three", directorapi.Rect{X: 100, Y: 10, Width: 40, Height: 16}),
		ocrText("ocr:1", "One", directorapi.Rect{X: 10, Y: 10, Width: 30, Height: 16}),
		ocrText("ocr:2", "Two", directorapi.Rect{X: 44, Y: 10, Width: 30, Height: 16}),
	}
	first := GroupLine(words)
	for i := 0; i < 10; i++ {
		got := GroupLine(words)
		if len(got) != len(first) {
			t.Fatalf("run %d produced %d groups, first produced %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j].Content.Raw != first[j].Content.Raw {
				t.Fatalf("run %d group %d = %q, first = %q",
					i, j, got[j].Content.Raw, first[j].Content.Raw)
			}
		}
	}
}

// ── 8. determinism and explanation ────────────────────────────────────────────

func TestTextFusionIsDeterministic(t *testing.T) {
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	texts := []observation.Text{
		ocrText("ocr:2", "B", directorapi.Rect{X: 60, Y: 10, Width: 20, Height: 16}),
		ocrText("ocr:1", "A", directorapi.Rect{X: 10, Y: 10, Width: 20, Height: 16}),
	}
	cycle := textCycle([]directorapi.Observation{button}, texts)

	var labels []string
	for i := 0; i < 8; i++ {
		w, _, _ := NewEngine().Fuse(cycle)
		for _, el := range w.Elements {
			labels = append(labels, el.Label)
		}
	}
	for i := 1; i < len(labels); i++ {
		if labels[i] != labels[0] {
			t.Fatalf("run %d produced label %q, first produced %q", i, labels[i], labels[0])
		}
	}
}

func TestTheExplanationNamesTheActualTextFusionRule(t *testing.T) {
	button := accEl("acc:1", "", directorapi.RoleButton, directorapi.Rect{X: 100, Y: 100, Width: 120, Height: 40})
	text := ocrText("ocr:1", "Export", directorapi.Rect{X: 125, Y: 110, Width: 65, Height: 20})
	cycle := textCycle([]directorapi.Observation{button}, []observation.Text{text})

	e := NewEngine().(Explainer)
	if _, _, err := e.Fuse(cycle); err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	cx := e.Explain(cycle)

	var found bool
	for _, ex := range cx.Elements {
		for _, m := range ex.MergeSteps {
			if m.Rule == "empty_structural_label" {
				found = true
				if m.Outcome != explain.MergeAccepted {
					t.Errorf("outcome = %q", m.Outcome)
				}
				if m.Against == nil || m.Against.Source != directorapi.SourceOCR {
					t.Error("the decision does not name the OCR observation it merged")
				}
			}
		}
		// And the field choice says where the label came from and what it did NOT
		// change — the sentence a person reads to check the safety rule held.
		for _, f := range ex.Fields {
			if f.Field == "label" && f.From.Source == directorapi.SourceOCR {
				if !strings.Contains(f.Reason, "accessibility") {
					t.Errorf("the label explanation does not say where actionability came "+
						"from: %q", f.Reason)
				}
			}
		}
	}
	if !found {
		t.Error("the explanation does not name the rule that filled the label")
	}
}

// ── 9. confidence stays bounded ───────────────────────────────────────────────

func TestConfidenceNeverExceedsTheCapHoweverMuchTextAgrees(t *testing.T) {
	// Naive addition would put 0.9 + several 0.9s well past certainty, and a threshold
	// downstream would read agreement as proof.
	button := accEl("acc:1", "Save", directorapi.RoleButton, directorapi.Rect{X: 0, Y: 0, Width: 200, Height: 40})
	var texts []observation.Text
	for i := 0; i < 12; i++ {
		texts = append(texts, ocrText(
			"ocr:"+itoa(i),
			"Save", directorapi.Rect{X: 10 + i, Y: 10, Width: 40, Height: 16}))
	}

	w, _, _ := NewEngine().Fuse(textCycle([]directorapi.Observation{button}, texts))
	for _, el := range w.Elements {
		if el.Confidence > maxConfidence {
			t.Errorf("confidence %.4f exceeded the cap %.4f", el.Confidence, maxConfidence)
		}
		if el.Confidence < 0 {
			t.Errorf("confidence %.4f is negative", el.Confidence)
		}
		if el.LabelConfidence > maxConfidence {
			t.Errorf("label confidence %.4f exceeded the cap", el.LabelConfidence)
		}
	}
}
