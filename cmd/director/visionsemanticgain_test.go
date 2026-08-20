package main

import (
	"context"
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
)

// What the vision semantic path actually GAINS, measured on a deterministic fixture, and
// whether what it gains is the discriminator cross-session identity has been missing.
//
// These tests report numbers as well as asserting them. A milestone that says "it worked" and
// cannot say where evidence was gained or lost is one nobody can act on — and the previous
// milestone's whole finding was a place evidence was silently lost.

// visionRun is one deterministic pass over a fixture, with its budget accounted for.
type visionRun struct {
	detections  int
	nameable    int
	unsayable   int
	attempted   int
	read        int
	screenTexts int
	ambiguous   int
	skipped     int
	terms       []observe.InterfaceTerm
	observed    bool
}

func runFixture(t *testing.T, dets []vision.Detection, text map[image.Rectangle]string,
	withReader bool) visionRun {

	t.Helper()
	var reader vision.LabelReader
	if withReader {
		reader = newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	}
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})
	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	sh := sample.Shadow
	return visionRun{
		detections: sh.Detections, nameable: sh.Nameable,
		unsayable: sh.Labels.Unsayable, attempted: sh.Labels.Attempted(),
		read: sh.Labels.Read, screenTexts: sh.Labels.ScreenTexts,
		ambiguous: sh.Labels.Ambiguous, skipped: sh.Labels.Skipped,
		terms: sh.Semantic.Terms, observed: sh.Semantic.Observed,
	}
}

// ── PARTS 19 & 20: the deterministic gain ─────────────────────────────────────

// Before and after, on one accessibility-poor screen.
//
// BEFORE is not a hypothetical: it is the same fixture through the same production path with no
// label reader wired, which is exactly what every shadow session ran as until this milestone.
func TestTheDeterministicGainIsMeasuredAndReported(t *testing.T) {
	dets, text := accessibilityPoorFixture()

	before := runFixture(t, dets, text, false)
	after := runFixture(t, dets, text, true)

	t.Logf("accessibility-poor fixture, %d detections", len(dets))
	t.Logf("  before: structure %d (%d nameable), terms %v, observed %v",
		before.detections, before.nameable, before.terms, before.observed)
	t.Logf("  after:  structure %d (%d nameable), terms %v, observed %v",
		after.detections, after.nameable, after.terms, after.observed)
	t.Logf("  budget: unsayable %d, attempted %d, read %d, screen texts %d, "+
		"ambiguous %d, skipped %d",
		after.unsayable, after.attempted, after.read, after.screenTexts,
		after.ambiguous, after.skipped)

	if len(before.terms) != 0 || before.observed {
		t.Fatalf("the BEFORE run already produced semantic evidence (%v); the measurement "+
			"has no baseline", before.terms)
	}
	if before.detections != after.detections {
		t.Errorf("reading text changed how much structure was found (%d → %d). Text must "+
			"enrich structure, never create or destroy it",
			before.detections, after.detections)
	}
	if len(after.terms) == 0 {
		t.Fatal("no gain: the same fixture produced no term with a reader wired")
	}
	// The unsayable regions cost nothing. That is half the point of gating the reading on
	// nameability rather than on the classifier downstream.
	if after.unsayable == 0 {
		t.Error("the fixture's icon and panel were not counted as unsayable, so this run " +
			"does not measure what the budget avoided")
	}
	if after.attempted > after.read+after.screenTexts+after.unsayable {
		t.Errorf("more readings were paid for (%d) than produced evidence", after.attempted)
	}
}

// ── PARTS 23 & 25: is this the discriminator ADR-016 needs? ───────────────────

// settingsVariant is one screen of an accessibility-poor settings menu.
//
// Same STRUCTURE every time — four stacked controls of identical geometry, one heading, one
// icon — and different WORDS. That is the case cross-session identity is most at risk from:
// role composition alone describes a settings screen, a level select, a save-file list and a
// confirmation dialog, in every application ever written.
func settingsVariant(words [4]string, heading string) ([]vision.Detection, map[image.Rectangle]string) {
	text := map[image.Rectangle]string{}
	var dets []vision.Detection
	add := func(class string, r image.Rectangle, s string) {
		dets = append(dets, vision.Detection{Class: class, Bounds: r, Confidence: 0.7})
		if s != "" {
			text[r] = s
		}
	}
	add("text", image.Rect(700, 300, 1104, 350), heading)
	for i, w := range words {
		// Distinct widths so the fixture's size-keyed reader can tell them apart; the
		// geometry stays within the tolerances identity is compared at.
		add("button", image.Rect(700, 400+i*70, 1100-i*4, 450+i*70), w)
	}
	add("icon", image.Rect(100, 900, 180, 980), "")
	return dets, text
}

// signatureFrom runs a fixture through the production path often enough for it to become a
// recurring screen state, and returns the durable signature that comes out.
func signatureFrom(t *testing.T, dets []vision.Detection,
	text map[image.Rectangle]string) (observe.StructureSignature, bool) {

	t.Helper()
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	req := sampleRequest()
	req.ReadLabels = true

	var totals observe.ShadowTotals
	for i := 0; i < 8; i++ {
		sample, err := live.Sample(context.Background(), req)
		if err != nil {
			t.Fatalf("Sample: %v", err)
		}
		if sample.Shadow == nil {
			t.Fatal("no shadow record")
		}
		// The menu, then something else, so the screen RECURS rather than being stared at.
		// A hypothesis needs episodes, and eight uninterrupted readings are one episode.
		totals.Add(*sample.Shadow)
		totals.Add(gameplayShadow())
	}

	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		sig := observe.SignatureOf(h)
		if sig.Subject == observe.SubjectState && sig.TermsKnown && len(sig.Terms) > 0 {
			return sig, true
		}
	}
	return observe.StructureSignature{}, false
}

// THE product question: does vision-derived semantics separate two screens that structure
// alone cannot?
//
// This is the strongest positive evidence available for the new path, and the strongest
// negative if it fails. Two settings screens with identical composition — audio and display —
// are the case ADR-016 refuses to merge on structure and has never had a way to separate.
func TestSimilarStructuresAreSeparatedByVisionDerivedTerms(t *testing.T) {
	audioDets, audioText := settingsVariant(
		[4]string{"VOLUME", "MUSIC", "SOUND", "BACK"}, "AUDIO SETTINGS")
	displayDets, displayText := settingsVariant(
		[4]string{"RESOLUTION", "BRIGHTNESS", "GRAPHICS", "BACK"}, "DISPLAY SETTINGS")

	audio, ok := signatureFrom(t, audioDets, audioText)
	if !ok {
		t.Fatal("the audio fixture produced no signature carrying terms")
	}
	display, ok := signatureFrom(t, displayDets, displayText)
	if !ok {
		t.Fatal("the display fixture produced no signature carrying terms")
	}

	t.Logf("case B — similar structure, different words")
	t.Logf("  audio:   roles %v members %d terms %v", audio.Roles, audio.Members, audio.Terms)
	t.Logf("  display: roles %v members %d terms %v", display.Roles, display.Members, display.Terms)

	// Structure alone. Strip the semantics and the two are indistinguishable — which is the
	// baseline this whole milestone is measured against.
	structureOnly := func(s observe.StructureSignature) observe.StructureSignature {
		s.Terms, s.TermsKnown = nil, false
		return s
	}
	blind := observe.CompareStructure(structureOnly(audio), structureOnly(display))
	t.Logf("  structure-only verdict:  %s", blind)
	if blind == observe.MatchDifferent {
		t.Fatalf("the two fixtures differ structurally (%s), so this measures geometry "+
			"rather than semantics", blind)
	}

	// Structure plus the words the detector's own boxes carried.
	withTerms := observe.CompareStructure(audio, display)
	t.Logf("  structure+semantic verdict: %s (terms known: %v / %v)",
		withTerms, audio.TermsKnown, display.TermsKnown)
	if withTerms != observe.MatchDifferent {
		t.Fatalf("two settings screens with different words came out %s. Vision-derived "+
			"terms are not a discriminator, and a user's answer about one would be "+
			"inherited by the other", withTerms)
	}
}

// Case A: the same subject, observed twice, must be recognised — and case C: one missing term
// must not silently become a different screen.
//
// Case C is the measurement that decides whether the exact-set rule is now the bottleneck. It
// is deliberately NOT fixed here: the rule is measured as it stands, and if equivalent screens
// commonly differ by one term the next task is to make matching robust to partial evidence,
// with this number in hand.
func TestTheSameSubjectAndTheOneMissingTerm(t *testing.T) {
	dets, text := settingsVariant(
		[4]string{"VOLUME", "MUSIC", "SOUND", "BACK"}, "AUDIO SETTINGS")

	first, ok := signatureFrom(t, dets, text)
	if !ok {
		t.Fatal("no signature from the first observation")
	}
	second, ok := signatureFrom(t, dets, text)
	if !ok {
		t.Fatal("no signature from the second observation")
	}

	caseA := observe.CompareStructure(first, second)
	t.Logf("case A — same subject twice: %s, terms %v", caseA, first.Terms)
	if caseA != observe.MatchSame {
		t.Fatalf("the same screen observed twice came out %s. Terms are stable and structure "+
			"is identical; if this is not `same` the path gains nothing", caseA)
	}

	// One control unreadable this time — the ordinary case, not an exotic one. An engine
	// misses a stylised glyph, a control is mid-fade, a reading falls below confidence.
	partialText := map[image.Rectangle]string{}
	for k, v := range text {
		partialText[k] = v
	}
	// BACK, deliberately: it is the only control on this screen whose concept nothing else
	// supplies, so losing it loses a TERM rather than a reading. Dropping one of the three
	// audio-flavoured controls would change nothing and would measure nothing.
	for box := range partialText {
		if partialText[box] == "BACK" {
			delete(partialText, box)
			break
		}
	}
	partial, ok := signatureFrom(t, dets, partialText)
	if !ok {
		t.Fatal("no signature from the partial observation")
	}

	caseC := observe.CompareStructure(partial, first)
	t.Logf("case C — one term missing: %s", caseC)
	t.Logf("  full:    %v", first.Terms)
	t.Logf("  partial: %v", partial.Terms)
	if caseC == observe.MatchSame {
		t.Log("  the exact-set rule tolerated the loss (the terms happened to coincide)")
		return
	}
	if caseC != observe.MatchDifferent {
		t.Logf("  the exact-set rule degraded to %s rather than declaring a difference", caseC)
		return
	}
	// Recorded, not repaired. Changing the matcher in this milestone would be changing the
	// thing being measured — see the note above.
	t.Log("  MEASURED: a PERSISTENT loss turns a recognised screen into a DIFFERENT one. " +
		"Whether that is a real bottleneck depends on how often a reading is lost, which " +
		"is what TestATransientlyLostReadingDoesNotChangeIdentity measures")
}

// How often does a lost reading actually change identity?
//
// Case C above removes a control's words from EVERY observation, which is the worst case and
// not the ordinary one. The ordinary one is an engine missing a stylised glyph on some passes:
// the control is read four times in six and not at all in the other two.
//
// This distinction decides whether the exact-set rule is the next bottleneck or a non-issue,
// and it is not a matter of opinion — the per-state term ratio exists precisely to absorb
// intermittent evidence, and whether it does is a measurement.
func TestATransientlyLostReadingDoesNotChangeIdentity(t *testing.T) {
	dets, text := settingsVariant(
		[4]string{"VOLUME", "MUSIC", "SOUND", "BACK"}, "AUDIO SETTINGS")

	full, ok := signatureFrom(t, dets, text)
	if !ok {
		t.Fatal("no signature from the stable observation")
	}

	// The same screen, with BACK unreadable on a THIRD of the passes.
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	reader.dropWord, reader.dropEvery = "BACK", 3
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	req := sampleRequest()
	req.ReadLabels = true

	var totals observe.ShadowTotals
	for i := 0; i < 8; i++ {
		sample, err := live.Sample(context.Background(), req)
		if err != nil {
			t.Fatalf("Sample: %v", err)
		}
		totals.Add(*sample.Shadow)
		totals.Add(gameplayShadow())
	}
	var flaky observe.StructureSignature
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if sig := observe.SignatureOf(h); sig.Subject == observe.SubjectState &&
			sig.TermsKnown && len(sig.Terms) > 0 {
			flaky = sig
			break
		}
	}
	if len(flaky.Terms) == 0 {
		t.Fatal("no signature from the intermittent observation")
	}

	verdict := observe.CompareStructure(flaky, full)
	t.Logf("case C' — one control unreadable on 1 pass in %d: %s", reader.dropEvery, verdict)
	t.Logf("  stable:       %v", full.Terms)
	t.Logf("  intermittent: %v", flaky.Terms)
	if verdict != observe.MatchSame {
		t.Errorf("an intermittently unreadable control changed the screen's identity (%s). "+
			"The per-state term ratio is not absorbing missing readings, and the exact-set "+
			"rule IS the next bottleneck", verdict)
	}
}
