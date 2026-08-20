package observe_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── bounds ────────────────────────────────────────────────────────────────────

func TestBoundsRejectRatherThanTruncate(t *testing.T) {
	// Silently shortening a request leaves somebody waiting for results from a session
	// that ended long ago.
	for _, tc := range []struct {
		name string
		in   observe.Bounds
	}{
		{"too long", observe.Bounds{Duration: 3 * time.Hour}},
		{"too short", observe.Bounds{Duration: time.Second}},
		{"interval faster than a frame", observe.Bounds{Interval: 10 * time.Millisecond}},
		{"interval too slow", observe.Bounds{Interval: time.Minute}},
		{"no frames", observe.Bounds{MaxFrames: -1}},
	} {
		if _, err := tc.in.Normalise(); err == nil {
			t.Errorf("%s was accepted", tc.name)
		}
	}
}

func TestBoundsExplainWhatTheyRefused(t *testing.T) {
	_, err := observe.Bounds{Duration: 3 * time.Hour}.Normalise()
	if err == nil {
		t.Fatal("a three-hour session was accepted")
	}
	if !strings.Contains(err.Error(), observe.MaxDuration.String()) {
		t.Errorf("the refusal %q does not quote the limit", err)
	}
}

func TestDefaultBoundsAreUsable(t *testing.T) {
	b, err := observe.DefaultBounds().Normalise()
	if err != nil {
		t.Fatalf("the defaults do not validate: %v", err)
	}
	if b.Duration != observe.DefaultDuration || b.Interval != observe.DefaultInterval {
		t.Errorf("normalising the defaults changed them: %+v", b)
	}
}

func TestOnlyCompletedCountsAsSuccess(t *testing.T) {
	// A session cut short produced real evidence and an incomplete picture. Presenting
	// it as a completed run lets a reader draw conclusions from a sample size they were
	// never told about.
	if !observe.Completed.Succeeded() {
		t.Error("completed is not success")
	}
	for _, s := range []observe.State{
		observe.TargetUnavailable, observe.Cancelled, observe.TimedOut, observe.Failed,
	} {
		if s.Succeeded() {
			t.Errorf("%q counts as success", s)
		}
		if !s.Terminal() {
			t.Errorf("%q is not terminal", s)
		}
	}
}

// ── privacy ───────────────────────────────────────────────────────────────────

// A distinctive marker, so a leak is unmistakable in any output.
const forbiddenMarker = "TTVX-FINAL-SECRET-6b8d-XYZZYPLOV"

func TestSafeLabelsAreKeptAndEverythingElseIsWithheld(t *testing.T) {
	p := observe.DefaultLabelPolicy()

	// Real control names, from the live Rocket League pause menu.
	for _, safe := range []string{"RESUME GAME", "SETTINGS", "EXIT TO MAIN MENU", "OPTIONS"} {
		got := observe.Classify(safe, 0.9, buttonContext(), p)
		if got.Text != safe {
			t.Errorf("the control name %q was withheld (%+v)", safe, got)
		}
		if got.Redacted {
			t.Errorf("%q was marked redacted", safe)
		}
	}

	// Things a game shows that are about a person, from the same live frames.
	for _, private := range []string{
		"[SILY] Silly Fire",
		"Dynamo — Puppet on Spotify",
		"chris@example.com",
		"joined your party",
		"#general | Sometimes Silly",
		forbiddenMarker,
	} {
		got := observe.Classify(private, 0.95, buttonContext(), p)
		if got.Text != "" {
			t.Errorf("private text %q was kept in the clear as %q", private, got.Text)
		}
		if !got.Redacted {
			t.Errorf("%q was not marked redacted", private)
		}
		if got.Digest == "" {
			t.Errorf("%q has no digest, so a change in it could never be observed", private)
		}
	}
}

func TestAWithheldLabelStillComparesAgainstItself(t *testing.T) {
	// The digest is what makes "this changed" observable without "this said" being kept.
	p := observe.DefaultLabelPolicy()
	first := observe.Classify("[SILY] Silly Fire", 0.9, buttonContext(), p)
	again := observe.Classify("[SILY] Silly Fire", 0.9, buttonContext(), p)
	other := observe.Classify("[SILY] Somebody Else", 0.9, buttonContext(), p)

	if first.Digest != again.Digest {
		t.Error("the same withheld label produced two digests")
	}
	if first.Digest == other.Digest {
		t.Error("two different withheld labels share a digest")
	}
}

func TestLowConfidenceTextIsNeverKeptInTheClear(t *testing.T) {
	// A reading nobody believes is not worth the privacy cost of storing.
	got := observe.Classify("RESUME GAME", 0.2, buttonContext(), observe.DefaultLabelPolicy())
	if got.Text != "" {
		t.Errorf("a 0.2-confidence reading was stored as %q", got.Text)
	}
}

func TestNoForbiddenMarkerSurvivesIntoJSON(t *testing.T) {
	// The end-to-end privacy assertion: whatever the pipeline does internally, what gets
	// written must not contain it.
	p := observe.DefaultLabelPolicy()
	label := observe.Classify(forbiddenMarker+" - Notepad", 0.99, buttonContext(), p)

	encoded, err := json.Marshal(label)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), forbiddenMarker) {
		t.Fatalf("the marker reached persisted JSON: %s", encoded)
	}
}

// ── stability ─────────────────────────────────────────────────────────────────

func TestSomethingSeenTwiceIsNotStable(t *testing.T) {
	th := observe.DefaultThresholds()
	s := observe.Stability{SamplesSeen: 2, SamplesTotal: 100, PresenceRatio: 0.02, ConfidenceMin: 0.9}
	if s.Stable(th) {
		t.Fatal("two appearances out of a hundred counted as stable")
	}
}

func TestAFlickeringDetectionIsNotStable(t *testing.T) {
	// Present in 70% of samples across forty separate appearances is not a stable
	// feature of an interface; it is a detector struggling.
	th := observe.DefaultThresholds()
	s := observe.Stability{
		SamplesSeen: 70, SamplesTotal: 100, PresenceRatio: 0.7,
		ConfidenceMin: 0.8, Flickers: 40,
	}
	if s.Stable(th) {
		t.Fatal("a detection that came and went forty times counted as stable")
	}
}

func TestAPersistentConfidentEntityIsStable(t *testing.T) {
	th := observe.DefaultThresholds()
	s := observe.Stability{
		SamplesSeen: 95, SamplesTotal: 100, PresenceRatio: 0.95,
		ConfidenceMin: 0.7, Flickers: 1,
	}
	if !s.Stable(th) {
		t.Fatal("a thing present in 95% of samples was not called stable")
	}
}

// ── the analyzer over a synthetic timeline ────────────────────────────────────

// scene builds a sample containing the named entities.
func scene(seq int, at time.Time, generation uint64, entities ...observe.EntitySnapshot) observe.Sample {
	return observe.Sample{
		Sequence: seq, Timestamp: at, WindowGeneration: generation,
		Entities: entities,
	}
}

func entity(id string, role string, region observe.Region, label observe.SafeLabel, confidence float64) observe.EntitySnapshot {
	return observe.EntitySnapshot{
		Identity: observe.Digest(id), Role: rolev(role),
		Region: region, Label: label, Confidence: confidence,
	}
}

func TestAPersistentThingBecomesStableAndATransientOneDoesNot(t *testing.T) {
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	meter := observe.Region{X: 0.85, Y: 0.8, Width: 0.1, Height: 0.15}
	popup := observe.Region{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.2}

	for i := 0; i < 20; i++ {
		at := base.Add(time.Duration(i) * 500 * time.Millisecond)
		entities := []observe.EntitySnapshot{
			entity("meter", "icon", meter, observe.SafeLabel{}, 0.8),
		}
		if i == 10 { // one frame only
			entities = append(entities, entity("popup", "panel", popup, observe.SafeLabel{}, 0.7))
		}
		a.Observe(scene(i, at, 1, entities...))
	}

	f := a.Findings()
	if len(f.Stable) != 1 {
		t.Fatalf("%d stable entities, want just the meter: %+v", len(f.Stable), f.Stable)
	}
	if f.Stable[0].Identity != "meter" {
		t.Errorf("stable entity is %q, want the meter", f.Stable[0].Identity)
	}
	foundPopup := false
	for _, u := range f.Unstable {
		if u.Identity == "popup" {
			foundPopup = true
		}
	}
	if !foundPopup {
		t.Error("the one-frame popup is not recorded as unstable evidence")
	}
}

func TestJitterIsNotAMovement(t *testing.T) {
	// A detector's box wanders a pixel or two. Reporting that as a transition buries the
	// handful that matter under hundreds that do not.
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 20; i++ {
		wobble := float64(i%2) * 0.004 // well inside the 0.02 tolerance
		r := observe.Region{X: 0.85 + wobble, Y: 0.8, Width: 0.1, Height: 0.15}
		a.Observe(scene(i, base.Add(time.Duration(i)*time.Second), 1,
			entity("meter", "icon", r, observe.SafeLabel{}, 0.8)))
	}

	for _, tr := range a.Findings().Transitions {
		if tr.Kind == observe.EntityMoved {
			t.Fatalf("jitter of 0.004 produced a movement transition: %+v", tr)
		}
	}
}

func TestARealMoveIsReported(t *testing.T) {
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		x := 0.1
		if i >= 5 {
			x = 0.7 // a genuine relocation
		}
		a.Observe(scene(i, base.Add(time.Duration(i)*time.Second), 1,
			entity("thing", "icon", observe.Region{X: x, Y: 0.5, Width: 0.1, Height: 0.1},
				observe.SafeLabel{}, 0.8)))
	}
	moved := 0
	for _, tr := range a.Findings().Transitions {
		if tr.Kind == observe.EntityMoved {
			moved++
		}
	}
	if moved != 1 {
		t.Fatalf("%d movement transitions, want exactly 1", moved)
	}
}

func TestAWindowGenerationChangeIsRecordedAsASeam(t *testing.T) {
	// Evidence either side of a generation change came from different windows. A session
	// that did not mark the seam would let a reader treat it as one continuous view.
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	a.Observe(scene(0, base, 1, entity("a", "icon", observe.Region{}, observe.SafeLabel{}, 0.8)))
	a.Observe(scene(1, base.Add(time.Second), 2,
		entity("a", "icon", observe.Region{}, observe.SafeLabel{}, 0.8)))

	found := false
	for _, tr := range a.Findings().Transitions {
		if tr.Kind == observe.WindowGenerationChanged {
			found = true
			if tr.Reason == "" {
				t.Error("the seam carries no explanation")
			}
		}
	}
	if !found {
		t.Fatal("a generation change produced no transition")
	}
}

func TestTransitionsAreBounded(t *testing.T) {
	b := observe.DefaultBounds()
	b.MaxTransitions = 5
	a := observe.NewAnalyzer(observe.DefaultThresholds(), b)
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	// A new identity every sample: the shape of a detector producing noise.
	for i := 0; i < 50; i++ {
		a.Observe(scene(i, base.Add(time.Duration(i)*time.Second), 1,
			entity(string(rune('a'+i%26))+string(rune('a'+i/26)), "icon",
				observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1},
				observe.SafeLabel{}, 0.8)))
	}
	f := a.Findings()
	if len(f.Transitions) > 5 {
		t.Fatalf("%d transitions retained, cap was 5", len(f.Transitions))
	}
	if f.DroppedTransitions == 0 {
		t.Fatal("transitions were dropped but the count is 0; a silent cap reads as " +
			"\"nothing more happened\"")
	}
}

// ── insights ──────────────────────────────────────────────────────────────────

func TestEveryHypothesisCarriesAValidation(t *testing.T) {
	f := buildFindings(t)
	insights := observe.Insights(f, observe.DefaultInsightThresholds())
	if len(insights) == 0 {
		t.Fatal("no hypotheses at all from a session with a stable labelled panel")
	}
	for _, in := range insights {
		if in.Validation == "" {
			t.Errorf("%q has no recommended validation; without one it is an assertion, "+
				"not a hypothesis", in.Kind)
		}
		if in.Observed == "" {
			t.Errorf("%q does not say what was actually observed", in.Kind)
		}
	}
}

func TestHypothesesNeverAssert(t *testing.T) {
	// The type itself has to tell the truth about what a timeline can establish.
	f := buildFindings(t)
	for _, in := range observe.Insights(f, observe.DefaultInsightThresholds()) {
		if !strings.HasPrefix(string(in.Kind), "possible") {
			t.Errorf("the concept %q does not read as a possibility", in.Kind)
		}
		for _, banned := range []string{"detected", "confirmed", "is a"} {
			if strings.Contains(strings.ToLower(in.Observed), banned) {
				t.Errorf("%q asserts: %q", in.Kind, in.Observed)
			}
		}
	}
}

func TestAMenuLikePanelIsProposedFromSeveralLabelsInOnePlace(t *testing.T) {
	f := buildFindings(t)
	for _, in := range observe.Insights(f, observe.DefaultInsightThresholds()) {
		if in.Kind == observe.PossibleMenu {
			if len(in.SupportingEntities) < 3 {
				t.Errorf("a menu-like panel cites only %d entities", len(in.SupportingEntities))
			}
			return
		}
	}
	t.Fatal("four labels sharing one region did not suggest a menu-like panel")
}

func TestInsufficientEvidenceProducesNoHypothesis(t *testing.T) {
	empty := observe.Findings{Samples: 3, Thresholds: observe.DefaultThresholds()}
	if got := observe.Insights(empty, observe.DefaultInsightThresholds()); len(got) != 0 {
		t.Fatalf("%d hypotheses from an empty session: %+v", len(got), got)
	}
}

func TestInsightsAreDeterministic(t *testing.T) {
	// A fixture must replay identically, so a change in output means a change in
	// evidence rather than a change in map iteration order.
	f := buildFindings(t)
	first := observe.Insights(f, observe.DefaultInsightThresholds())
	for i := 0; i < 20; i++ {
		again := observe.Insights(f, observe.DefaultInsightThresholds())
		if len(again) != len(first) {
			t.Fatalf("run %d produced %d hypotheses, first run produced %d", i, len(again), len(first))
		}
		for j := range again {
			if again[j].Kind != first[j].Kind || again[j].Observed != first[j].Observed {
				t.Fatalf("run %d differs at %d: %q vs %q", i, j, again[j].Kind, first[j].Kind)
			}
		}
	}
}

func TestHypothesesAreBounded(t *testing.T) {
	f := buildFindings(t)
	th := observe.DefaultInsightThresholds()
	th.MaxCandidates = 2
	if got := observe.Insights(f, th); len(got) > 2 {
		t.Fatalf("%d hypotheses, cap was 2", len(got))
	}
}

// buildFindings runs a synthetic session resembling the live Rocket League pause menu:
// four labelled controls stacked in one region, plus a persistent unlabelled meter.
func buildFindings(t *testing.T) observe.Findings {
	t.Helper()
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	p := observe.DefaultLabelPolicy()

	labels := []string{"RESUME GAME", "CHANGE MODEMATCH", "SETTINGS", "EXIT TO MAIN MENU"}
	for i := 0; i < 30; i++ {
		entities := []observe.EntitySnapshot{
			entity("meter", "icon",
				observe.Region{X: 0.85, Y: 0.78, Width: 0.1, Height: 0.18},
				observe.SafeLabel{}, 0.75),
		}
		for j, text := range labels {
			entities = append(entities, entity("btn"+string(rune('0'+j)), "button",
				observe.Region{X: 0.41, Y: 0.43 + float64(j)*0.01, Width: 0.18, Height: 0.04},
				observe.Classify(text, 0.9, buttonContext(), p), 0.62))
		}
		a.Observe(scene(i, base.Add(time.Duration(i)*500*time.Millisecond), 1, entities...))
	}
	return a.Findings()
}

// rolev converts a test's role word into the API type.
func rolev(s string) directorapi.ElementRole { return directorapi.ElementRole(s) }

func TestPresenceNeverExceedsTheSampleCount(t *testing.T) {
	// Found live, not here: a real session reported "present in 925% of samples".
	// Identity is role + label + coarse quadrant, so nine indistinguishable icons in one
	// quadrant share one identity — intended — but counting each OCCURRENCE made presence
	// a multiple of the sample count and every ratio meaningless.
	a := observe.NewAnalyzer(observe.DefaultThresholds(), observe.DefaultBounds())
	base := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 12; i++ {
		// Nine identical unnamed icons in the same place, every sample.
		var entities []observe.EntitySnapshot
		for j := 0; j < 9; j++ {
			entities = append(entities, entity("same", "icon",
				observe.Region{X: 0.1, Y: 0.1, Width: 0.05, Height: 0.05},
				observe.SafeLabel{}, 0.8))
		}
		a.Observe(scene(i, base.Add(time.Duration(i)*time.Second), 1, entities...))
	}

	for _, s := range append(a.Findings().Stable, a.Findings().Unstable...) {
		if s.PresenceRatio > 1.0 {
			t.Errorf("presence ratio %.2f exceeds 1.0 (%d seen of %d samples)",
				s.PresenceRatio, s.SamplesSeen, s.SamplesTotal)
		}
		if s.SamplesSeen > s.SamplesTotal {
			t.Errorf("seen in %d samples out of %d", s.SamplesSeen, s.SamplesTotal)
		}
		if s.Occurrences <= s.SamplesSeen {
			t.Errorf("occurrences %d should exceed samples %d when nine things share an "+
				"identity; that difference is what tells one control from nine",
				s.Occurrences, s.SamplesSeen)
		}
	}
}

// buttonContext is the structural context a real control has: the label is the name
// written on a button, which is a fact about the interface.
func buttonContext() observe.LabelContext {
	return observe.LabelContext{
		Role: directorapi.RoleButton, Sources: []string{"accessibility"},
	}
}

// iconContext is what a detected box has when nothing structural vouches for it. Its
// "label" is whatever text an OCR pass found inside it.
func iconContext() observe.LabelContext {
	return observe.LabelContext{Role: directorapi.RoleIcon, Sources: []string{"vision"}}
}

func TestOnlyStructuralControlsMayKeepPlaintext(t *testing.T) {
	// The defect that reordered this whole milestone. A live Chrome session reported
	// "Chris Haynes Plus" as a stable entity in the clear: three ordinary capitalised
	// words, indistinguishable BY SHAPE from "Exit To Menu". No rule about the string
	// could have caught it.
	//
	// What separates them is what they are attached to. A button's name is a fact about
	// the interface; text inside an icon is a fact about the person using it.
	p := observe.DefaultLabelPolicy()

	got := observe.Classify("Chris Haynes Plus", 0.95, iconContext(), p)
	if got.Text != "" {
		t.Fatalf("a person's name was kept in the clear as %q", got.Text)
	}
	if !got.Redacted {
		t.Error("it was not marked redacted")
	}
	if got.Digest == "" {
		t.Error("no digest, so a change in it could never be observed")
	}

	// The same words on an actual button remain readable.
	if got := observe.Classify("Exit To Menu", 0.95, buttonContext(), p); got.Text != "Exit To Menu" {
		t.Errorf("a button's name was withheld: %+v", got)
	}
}

func TestUnknownRolesDefaultToPrivate(t *testing.T) {
	p := observe.DefaultLabelPolicy()
	for _, role := range []directorapi.ElementRole{
		directorapi.RoleIcon, directorapi.RoleText, directorapi.RoleImage,
		directorapi.RoleUnknown, directorapi.ElementRole("something new"),
	} {
		got := observe.Classify("SETTINGS", 0.99,
			observe.LabelContext{Role: role, Sources: []string{"accessibility"}}, p)
		if got.Text != "" {
			t.Errorf("role %q kept plaintext; the default must be privacy", role)
		}
	}
}

func TestNoContextAtAllIsPrivate(t *testing.T) {
	got := observe.Classify("SETTINGS", 0.99, observe.LabelContext{}, observe.DefaultLabelPolicy())
	if got.Text != "" {
		t.Fatalf("a label with no structural context was kept as %q", got.Text)
	}
}

func TestShapeIsStillCheckedForEligibleRoles(t *testing.T) {
	// Defence in depth: even a button's name is refused if it looks like a token.
	p := observe.DefaultLabelPolicy()
	if got := observe.Classify(forbiddenMarker, 0.99, buttonContext(), p); got.Text != "" {
		t.Fatalf("a token on a button was kept as %q", got.Text)
	}
}
