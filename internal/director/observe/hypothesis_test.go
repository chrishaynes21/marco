package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What a hypothesis is NOT allowed to say.
//
// The wiring tests in observesession prove the generator is reachable and that it produces the
// right things from good evidence. These are the other half, and the more important one: every
// test here is a way the layer could produce a confident, plausible, wrong answer.
//
// The standard being defended: "recurring screen, semantics unknown" is a better outcome than
// "controller settings" that happens to be wrong.

// withSemantic is a valid inference that also carries interface vocabulary.
func withSemantic(terms []observe.InterfaceTerm, regions ...observe.ShadowRegion) observe.ShadowSample {
	s := valid(regions...)
	s.Semantic = observe.SemanticEvidence{Terms: terms, Observed: true}
	return s
}

func hypothesesOf(t *testing.T, samples ...observe.ShadowSample) []observe.Hypothesis {
	t.Helper()
	return observe.Hypotheses(fold(samples...), observe.DefaultHypothesisThresholds())
}

func firstOf(hs []observe.Hypothesis, kind observe.HypothesisKind) (observe.Hypothesis, bool) {
	for _, h := range hs {
		if h.Kind == kind {
			return h, true
		}
	}
	return observe.Hypothesis{}, false
}

// settingsMenu is the four-row panel with configuration vocabulary on it.
func settingsMenu() observe.ShadowSample {
	return withSemantic(
		[]observe.InterfaceTerm{observe.TermSettings}, menuRegions()...)
}

// ── contradiction ─────────────────────────────────────────────────────────────

// A change that also happens with no navigation before it cannot be attributed cleanly.
//
// Part 5's example, and the arithmetic has to bite: five transitions with pause before four is
// a materially weaker claim than four out of four, and a generator that reported only the four
// would have deleted its own counter-example.
func TestUnattributedTransitionsContestATransitionAction(t *testing.T) {
	// Three clean cycles, then the screen appears twice on its own.
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples,
			withInput([]observe.InputEvent{nav(observe.NavPause)}, menuRegions()...),
			menu(), gameplay(), gameplay())
	}
	clean := hypothesesOf(t, samples...)
	h, ok := firstOf(clean, observe.PossibleTransitionAction)
	if !ok {
		t.Fatal("no transition action from three clean observations")
	}
	if h.Status != observe.StatusSupported {
		t.Fatalf("three-of-three support gave status %q, want supported", h.Status)
	}
	if len(h.Contradictions) != 0 {
		t.Errorf("clean evidence carried %d contradiction(s)", len(h.Contradictions))
	}

	// Now the same change twice more with nothing observed before it.
	for i := 0; i < 2; i++ {
		samples = append(samples, menu(), menu(), gameplay(), gameplay())
	}
	contested := hypothesesOf(t, samples...)
	h, ok = firstOf(contested, observe.PossibleTransitionAction)
	if !ok {
		t.Fatal("the transition action vanished entirely once contradicted; it should be " +
			"CONTESTED and still inspectable, not deleted")
	}
	if h.Status != observe.StatusContested {
		t.Errorf("status %q with %d unattributed observation(s), want contested. A change "+
			"that happens by itself is evidence AGAINST reading the correlated ones as "+
			"caused", h.Status, h.Unattributed)
	}
	if h.Unattributed == 0 {
		t.Error("the hypothesis reports no unattributed observations, so a reader cannot " +
			"see the case against it")
	}
	// And the contradiction must be readable, not folded into a number.
	var found bool
	for _, c := range h.Contradictions {
		if strings.Contains(c.Statement, "no navigation observed") {
			found = true
		}
	}
	if !found {
		t.Errorf("contradictions %v do not state the unattributed case in words",
			h.Contradictions)
	}
}

// Two intents competing for one change must both survive.
func TestCompetingIntentsContestAnActionAndBothRemainVisible(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples,
			withInput([]observe.InputEvent{nav(observe.NavPause)}, menuRegions()...),
			menu(), gameplay(), gameplay())
	}
	for i := 0; i < 2; i++ {
		samples = append(samples,
			withInput([]observe.InputEvent{nav(observe.NavConfirm)}, menuRegions()...),
			menu(), gameplay(), gameplay())
	}

	h, ok := firstOf(hypothesesOf(t, samples...), observe.PossibleTransitionAction)
	if !ok {
		t.Fatal("no transition action")
	}
	if h.Status != observe.StatusContested {
		t.Errorf("status %q for a 3-to-2 split between two intents, want contested", h.Status)
	}
	var mentionsConfirm bool
	for _, c := range h.Contradictions {
		if strings.Contains(c.Statement, "confirm") {
			mentionsConfirm = true
		}
	}
	if !mentionsConfirm {
		t.Errorf("the losing intent does not appear in the contradictions: %v. An edge with "+
			"a 3-to-2 split is a different finding from a unanimous one", h.Contradictions)
	}
}

// A word seen in one visit and never again is not the screen's identity.
//
// Part 5's second example. A toast, a tooltip or a single misread must not become durable
// semantic identity just because it landed on a screen that does recur.
func TestATransientWordDoesNotBecomeSemanticIdentity(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	// One visit where the word appears...
	samples = append(samples, settingsMenu(), settingsMenu(), gameplay(), gameplay())
	// ...and four more visits to the same screen where it never does.
	for i := 0; i < 4; i++ {
		samples = append(samples, menu(), menu(), gameplay(), gameplay())
	}

	hs := hypothesesOf(t, samples...)
	h, ok := firstOf(hs, observe.PossibleSettingsLikeState)
	if !ok {
		// Suppressing it entirely is also an acceptable answer, and a stricter one.
		return
	}
	if h.Status == observe.StatusSupported {
		t.Errorf("a word seen in 1 of 5 visits produced a SUPPORTED settings hypothesis. "+
			"That is how a transient overlay becomes a screen's name. contradictions=%v",
			h.Contradictions)
	}
	var explains bool
	for _, c := range h.Contradictions {
		if strings.Contains(c.Statement, "transient") {
			explains = true
		}
	}
	if !explains {
		t.Errorf("the hypothesis is not supported but does not say why: %v", h.Contradictions)
	}
}

// ── the limits of each source ─────────────────────────────────────────────────

// Navigation evidence alone must not name a screen.
//
// A player pressing pause before something changes tells you an action and a change are
// related. It tells you nothing whatever about what the destination IS, and a layer that let
// navigation imply "menu" would name every loading screen in every game.
func TestNavigationAloneNeverNamesAScreen(t *testing.T) {
	// A screen change with strong navigation support and NO structure: the destination is
	// a single icon, which is not a set of choices by any reading.
	sparse := []observe.ShadowRegion{det("icon", 0.40, 0.40, 0.20, 0.20)}
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 4; i++ {
		samples = append(samples,
			withInput([]observe.InputEvent{nav(observe.NavPause)}, sparse...),
			valid(sparse...), gameplay(), gameplay())
	}

	hs := hypothesesOf(t, samples...)
	if h, ok := firstOf(hs, observe.PossibleMenuLikeState); ok {
		t.Errorf("navigation evidence alone produced %q about a screen with no grouped "+
			"controls at all: %s", h.Kind, h.Observed)
	}
	if h, ok := firstOf(hs, observe.PossibleSettingsLikeState); ok {
		t.Errorf("navigation evidence alone produced %q: %s", h.Kind, h.Observed)
	}
	// The honest claim — an action related to a change — must still be there.
	if _, ok := firstOf(hs, observe.PossibleTransitionAction); !ok {
		t.Error("no transition action either; the weaker honest claim should survive")
	}
}

// Interface vocabulary alone must not name a screen.
//
// The mirror of the test above, and the one that matters for a game: OCR reads words off HUDs,
// chat and scoreboards constantly. A screen is not a settings screen because the word SETTINGS
// was visible somewhere on it — the structure has to be there too.
func TestTextAloneNeverNamesAScreen(t *testing.T) {
	// The word is present throughout, on a screen with no grouped controls.
	hud := []observe.ShadowRegion{det("icon", 0.02, 0.86, 0.19, 0.10)}
	loud := withSemantic([]observe.InterfaceTerm{
		observe.TermSettings, observe.TermControls, observe.TermAudio,
	}, hud...)

	hs := hypothesesOf(t, loud, loud, loud, loud, loud, loud)
	if h, ok := firstOf(hs, observe.PossibleSettingsLikeState); ok {
		t.Errorf("the word SETTINGS on a screen with no grouped controls produced %q: %s. "+
			"OCR reads scoreboards, chat and HUDs; text without structure names nothing",
			h.Kind, h.Observed)
	}
}

// A single visit establishes nothing, however clean the evidence looks.
func TestOneVisitIsNeverSupported(t *testing.T) {
	// One long look at a rich, vocabulary-bearing screen, and never again.
	hs := hypothesesOf(t,
		gameplay(), gameplay(),
		settingsMenu(), settingsMenu(), settingsMenu(), settingsMenu(),
	)
	for _, h := range hs {
		if h.Episodes <= 1 && h.Status == observe.StatusSupported {
			t.Errorf("%s reached SUPPORTED on a single visit: %s. A transition frame and a "+
				"screen are indistinguishable until one of them comes back",
				h.Kind, h.Observed)
		}
	}
}

// A structure seen once is reported as a one-off, with the reason.
func TestAOneOffArrangementIsContestedNotSupported(t *testing.T) {
	hs := hypothesesOf(t, gameplay(), gameplay(), menu(), menu())
	h, ok := firstOf(hs, observe.PossibleChoiceGroup)
	if !ok {
		return // suppressing it entirely is stricter and also acceptable
	}
	if h.Status == observe.StatusSupported {
		t.Errorf("a group seen in one visit was SUPPORTED: %s", h.Observed)
	}
}

// ── independence ──────────────────────────────────────────────────────────────

// Support must come from more than one KIND of evidence before it is `supported`.
//
// Two structural facts about the same rectangle are one observation described twice, and a
// status that treated them as two would be counting its own restatement as corroboration.
func TestSupportedStatusRequiresIndependentSources(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 4; i++ {
		samples = append(samples, menu(), menu(), gameplay(), gameplay())
	}
	for _, h := range hypothesesOf(t, samples...) {
		if h.Status != observe.StatusSupported {
			continue
		}
		if len(h.Sources()) < 2 {
			t.Errorf("%s is SUPPORTED on a single kind of evidence (%v): %s",
				h.Kind, h.Sources(), h.Observed)
		}
	}
}

// ── the text boundary ─────────────────────────────────────────────────────────

// A typed name must never become semantic evidence.
//
// THE privacy regression for this milestone. The eventual capability "invite user <name>" takes
// the name as a runtime parameter; discovery has no reason to know it and no way to record it,
// because a name matches nothing in a vocabulary of interface concepts.
func TestATypedNameCannotBecomeAnInterfaceTerm(t *testing.T) {
	entities := []observe.EntitySnapshot{
		{Label: observe.SafeLabel{Text: "xX_ShadowSniper_Xx", Digest: "d1"}},
		{Label: observe.SafeLabel{Text: "hunter2", Digest: "d2"}},
		{Label: observe.SafeLabel{Text: "dave@example.com", Digest: "d3"}},
		{Label: observe.SafeLabel{Text: "192.168.0.14", Digest: "d4"}},
	}
	got := observe.SemanticEvidenceFrom(entities)
	if len(got.Terms) != 0 {
		t.Errorf("personal text produced interface terms %v; the vocabulary is supposed to "+
			"match interface concepts and nothing else", got.Terms)
	}
}

// A redacted label is not consulted at all.
//
// The structural classifier decided that text may not be released in the clear. This layer does
// not get a second opinion on that decision.
func TestARedactedLabelIsNotMatched(t *testing.T) {
	got := observe.SemanticEvidenceFrom([]observe.EntitySnapshot{
		{Label: observe.SafeLabel{Text: "settings", Digest: "d1", Redacted: true}},
	})
	if len(got.Terms) != 0 {
		t.Errorf("a redacted label was matched into %v, overriding the privacy classifier "+
			"that withheld it", got.Terms)
	}
}

// Matching is word-level, so a longer word containing a term does not match it.
func TestVocabularyMatchingIsWordLevel(t *testing.T) {
	cases := map[string][]observe.InterfaceTerm{
		// Longer words that merely CONTAIN a term match nothing. This is the whole
		// reason matching is word-level: "backpack" is not a back button, "researcher"
		// is not a search box, and a soundtrack unlock is not the audio settings.
		"backpack":          nil,
		"researcher":        nil,
		"soundtrack unlock": nil,
		// Case, punctuation and separators do not hide a word.
		"BACK":             {observe.TermBack},
		"Audio / Video":    {observe.TermAudio, observe.TermDisplay},
		"controller_setup": {observe.TermControls},
	}
	for label, want := range cases {
		got := observe.SemanticEvidenceFrom([]observe.EntitySnapshot{
			{Label: observe.SafeLabel{Text: label, Digest: "d"}},
		})
		if len(got.Terms) != len(want) {
			t.Errorf("%q produced %v, want %v", label, got.Terms, want)
			continue
		}
		for i := range want {
			if got.Terms[i] != want[i] {
				t.Errorf("%q produced %v, want %v", label, got.Terms, want)
				break
			}
		}
	}
}

// Only terms from the closed vocabulary reach a state's tally.
func TestAnUnknownTermIsDroppedAtTheStateBoundary(t *testing.T) {
	s := withSemantic([]observe.InterfaceTerm{
		"settings", "a player just typed this into chat",
	}, menuRegions()...)
	totals := fold(gameplay(), gameplay(), s, s, gameplay(), gameplay(), s, s)

	for _, st := range totals.States {
		for term := range st.Terms {
			if !term.Known() {
				t.Errorf("state %s carries term %q, which is outside the closed vocabulary",
					st.ID, term)
			}
		}
	}
}

// ── determinism ───────────────────────────────────────────────────────────────

func TestHypothesisOrderIsStable(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, settingsMenu(), settingsMenu(), gameplay(), gameplay())
	}
	first := hypothesesOf(t, samples...)
	if len(first) == 0 {
		t.Fatal("no hypotheses to compare")
	}
	for i := 0; i < 20; i++ {
		again := hypothesesOf(t, samples...)
		if len(again) != len(first) {
			t.Fatalf("run %d: %d hypotheses, want %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Kind != first[j].Kind || again[j].Status != first[j].Status ||
				again[j].Observed != first[j].Observed {
				t.Fatalf("run %d differs at %d: %s vs %s", i, j, again[j].Kind, first[j].Kind)
			}
		}
	}
}

// Nothing may be emitted from an empty session.
func TestAnEmptySessionProducesNoHypotheses(t *testing.T) {
	if got := observe.Hypotheses(observe.ShadowTotals{},
		observe.DefaultHypothesisThresholds()); len(got) != 0 {
		t.Errorf("%d hypotheses from no evidence at all", len(got))
	}
}
