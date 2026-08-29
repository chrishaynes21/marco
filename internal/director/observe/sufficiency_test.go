package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// FOUR FACTS THAT MUST NOT COLLAPSE INTO ONE ANOTHER.
//
//	"I do not know this Place."                    healthy reading, memory has no match
//	"Accessibility did not show me the Place."     the frame arrived and the page did not
//	"Accessibility showed me the Place, slowly."   a rich reading that cost 1.5 seconds
//	"There was nothing to read."                   the sensors did not report
//
// Each has a different remedy and three of them are routinely reported as the second. This
// file holds them apart.

// A healthy reading of an application nobody has ever seen is SUFFICIENT and unknown.
//
// The headline case, and the one most likely to be got wrong by a classifier that reaches for
// memory. "I have no record of this" is a fact about Marco; "the page did not arrive" is a
// fact about the reading. A never-before-seen application in perfect health must produce the
// first and never the second, or every first encounter looks like a broken sensor.
func TestAHealthyUnknownApplicationIsSufficient(t *testing.T) {
	m := &countingRecogniser{}
	p := observe.PlaceNow(page(), "an-application-nobody-has-seen", m,
		observe.DefaultHypothesisThresholds())

	s := observe.SufficiencyOf(p)
	if s.State != observe.Sufficient {
		t.Errorf("state = %q, want %q: %s\n"+
			"A page that was read perfectly well was called insufficient because memory "+
			"had no match for it. Not knowing where you are is not the same fact as not "+
			"being able to see.", s.State, observe.Sufficient, s.Describe())
	}
	if s.Reason != observe.ReasonContentReached {
		t.Errorf("reason = %q, want %q", s.Reason, observe.ReasonContentReached)
	}
	if p.Established() {
		t.Error("this recogniser remembers nothing and something was established anyway")
	}
	if m.asked == 0 {
		t.Error("memory was never consulted, so `unknown` here proves nothing")
	}
}

// The live Settings failure is INCOMPLETE, and says so in words.
func TestAWindowWithoutItsPageIsIncomplete(t *testing.T) {
	p := observe.PlaceNow(shell(), "settings", &countingRecogniser{},
		observe.DefaultHypothesisThresholds())

	s := observe.SufficiencyOf(p)
	if s.State != observe.Incomplete {
		t.Fatalf("state = %q, want %q: %s", s.State, observe.Incomplete, s.Describe())
	}
	if s.Reason != observe.ReasonClientAreaUnpopulated {
		t.Errorf("reason = %q, want %q", s.Reason, observe.ReasonClientAreaUnpopulated)
	}
	if !s.Vacancy.Found() {
		t.Error("the evidence was dropped; a caller told `incomplete` with no vacancy " +
			"cannot say how much of the window is unaccounted for")
	}
	// The explanation must be about the window, not about a number.
	d := s.Describe()
	for _, want := range []string{"window", "content"} {
		if !strings.Contains(d, want) {
			t.Errorf("the description %q does not mention %q.\n"+
				"An owner reads this. `score 0.42 below threshold 0.5` tells them "+
				"nothing they can act on.", d, want)
		}
	}
}

// Nothing observed is UNOBSERVABLE, and that is not the same as incomplete.
//
// A provider that failed and a provider that succeeded and returned a window frame are
// different facts. The first says the sensor did not run; the second says something about the
// application. Collapsing them sends a machine problem to the page — the mistake ADR-031
// forbids one level up, and the one [Reach] exists because of one level down.
func TestNothingObservedIsNotAnIncompleteReading(t *testing.T) {
	// No tracks, no settled state: the shape a failed accessibility call leaves behind.
	empty := observe.ShadowTotals{CurrentState: "state_1"}
	p := observe.PlaceNow(empty, "anything", &countingRecogniser{},
		observe.DefaultHypothesisThresholds())

	s := observe.SufficiencyOf(p)
	if s.State != observe.Unobservable {
		t.Errorf("state = %q, want %q: %s\n"+
			"A reading that never happened was reported as a reading that came back "+
			"short. Those have different remedies.",
			s.State, observe.Unobservable, s.Describe())
	}
	if s.State == observe.Incomplete {
		t.Error("provider failure collapsed into structural incompleteness")
	}
	if s.Reason != observe.ReasonNothingObserved {
		t.Errorf("reason = %q, want %q", s.Reason, observe.ReasonNothingObserved)
	}
	if s.Vacancy.Found() {
		t.Error("a vacancy was reported for a reading that observed nothing to be vacant")
	}
}

// A blank canvas beside a full ribbon is sufficient — the real Paint reading proves it.
//
// This is the custom-surface case as it actually occurs. Paint draws its canvas itself and
// exposes nothing inside it, and the canvas is 68% of the window. What saves it is not a
// special case for drawing applications: it is that the ribbon around it is INSIDE the same
// top-level pane, so somewhere in the window has things in it.
//
// See fixtures/perception/desktop/corpus/custom-canvas-paint — 106 real elements.
func TestACustomCanvasBesideRealControlsIsSufficient(t *testing.T) {
	var paint *corpusSample
	for i, s := range readCorpusSamples(t) {
		if s.ID == "custom-canvas-paint" {
			paint = &readCorpusSamples(t)[i]
		}
	}
	if paint == nil {
		t.Fatal("the Paint capture is missing from the corpus")
	}
	p := observe.PlaceNow(paint.asTotals(), "mspaint", &countingRecogniser{},
		observe.DefaultHypothesisThresholds())
	s := observe.SufficiencyOf(p)
	if s.State != observe.Sufficient {
		t.Errorf("state = %q, want %q: %s\n"+
			"A drawing application with a blank canvas and a full ribbon was called "+
			"incomplete. Its canvas is empty because nothing has been drawn, which is a "+
			"fact about the document and not about the reading.",
			s.State, observe.Sufficient, s.Describe())
	}
}

// A SURFACE WITH NOTHING AROUND IT READS AS INCOMPLETE, AND THAT IS THE INTENDED ANSWER.
//
// # The policy, stated rather than left to a threshold
//
// A window whose accessibility reports only frame furniture around a large empty client area
// classifies as [Incomplete]. A game viewport is the clearest example, and it was worth
// checking whether that is a false positive. It is not, for two reasons.
//
// First, the verdict is about the READING and not about the provider. `client_area_unpopulated`
// says what was observed — the frame arrived and nothing within it did. It does not say the
// provider malfunctioned, and nothing downstream may read it as that. A game that genuinely
// exposes no semantic content produces exactly this evidence, honestly.
//
// Second, the consequence is right. Insufficient means Marco will not operate semantically on
// this window, and it should not: there is nothing there to operate on. And when a later phase
// decides where additional perception is worth spending, a game surface is precisely where it
// is worth spending — 37B measured ScreenParser at its strongest on game frames, and 37C
// measured it adding nothing where accessibility is healthy. A classifier that called this
// window sufficient would close the door on the one case that motivated the whole line of
// work.
//
// So this test exists to pin the policy against a future "fix", not to record a defect.
func TestASurfaceWithNoSemanticContentIsIncomplete(t *testing.T) {
	// Standard window chrome around a client area the application draws itself.
	surface := totalsOf("state_1",
		seenAt("close", 0.96, 0.01, 0.03, 0.04),
		seenAt("maximise", 0.92, 0.01, 0.03, 0.04),
		seenAt("minimise", 0.88, 0.01, 0.03, 0.04),
		seenAt("title", 0.02, 0.01, 0.40, 0.04),
		seenAt("appicon", 0.00, 0.01, 0.03, 0.04),
		seenAt("menu", 0.06, 0.01, 0.06, 0.04),
		seenAt("client", 0.00, 0.06, 1.00, 0.94),
	)
	p := observe.PlaceNow(surface, "a-game", &countingRecogniser{},
		observe.DefaultHypothesisThresholds())
	s := observe.SufficiencyOf(p)
	if s.State != observe.Incomplete {
		t.Errorf("state = %q, want %q: %s", s.State, observe.Incomplete, s.Describe())
	}
	if s.Reason != observe.ReasonClientAreaUnpopulated {
		t.Errorf("reason = %q, want %q", s.Reason, observe.ReasonClientAreaUnpopulated)
	}
	// And the words must not accuse the provider of anything.
	for _, forbidden := range []string{"broken", "failed", "malfunction", "error"} {
		if strings.Contains(strings.ToLower(s.Describe()), forbidden) {
			t.Errorf("the description says %q: %q\n"+
				"An application that draws its own interface is not a broken "+
				"accessibility provider, and this sentence is shown to owners.",
				forbidden, s.Describe())
		}
	}
}

// A modal is judged on what it shows, not on what it covers up.
//
// The controls underneath a modal legitimately stop being available. A classifier comparing
// what is visible now against what the window had a moment ago would call every dialog a
// collapse.
func TestAModalIsJudgedOnItself(t *testing.T) {
	p := observe.PlaceNow(dialog(), "any-app", &countingRecogniser{},
		observe.DefaultHypothesisThresholds())
	if s := observe.SufficiencyOf(p); s.State != observe.Sufficient {
		t.Errorf("state = %q, want %q: %s\n"+
			"A small modal is sparse on purpose. Seven controls that fill their own "+
			"window is a complete reading of that window.",
			s.State, observe.Sufficient, s.Describe())
	}
}

// The assessment reads the reading, and nothing else about the moment.
//
// # Why this is a structural test rather than two timed runs
//
// Latency, application identity and the presence of any optional sensor must not move the
// answer. The obvious test — classify the same snapshot twice with different timings — proves
// nothing, because it would pass just as well on a classifier that had never been given a
// timing to ignore.
//
// So this asserts the honest version: the identical structural evidence, under four different
// application names, classifies identically. Nothing here can consult a clock or a plugin
// registry because [SufficiencyOf] is handed a [Place] and a Place carries neither.
func TestClassificationIgnoresEverythingButTheReading(t *testing.T) {
	want := observe.SufficiencyOf(observe.PlaceNow(page(), "settings",
		&countingRecogniser{}, observe.DefaultHypothesisThresholds()))

	for _, app := range []string{"explorer", "msedge", "a-game", ""} {
		got := observe.SufficiencyOf(observe.PlaceNow(page(), app,
			&countingRecogniser{}, observe.DefaultHypothesisThresholds()))
		if got.State != want.State || got.Reason != want.Reason {
			t.Errorf("application %q classified %q/%q; %q classified %q/%q.\n"+
				"The evidence was identical. An application's name is not evidence "+
				"about whether its page was read.",
				app, got.State, got.Reason, "settings", want.State, want.Reason)
		}
	}
}

// The reason set is closed, and every member describes itself.
//
// A bounded set is the point: an unbounded evidence trace grows until nobody reads it, and the
// consumer this exists for has to branch on the answer rather than print it.
func TestEveryReasonIsDescribable(t *testing.T) {
	all := []observe.SufficiencyReason{
		observe.ReasonContentReached,
		observe.ReasonPopulatedPanel,
		observe.ReasonTooLittleToJudge,
		observe.ReasonClientAreaUnpopulated,
		observe.ReasonNothingObserved,
	}
	seen := map[string]bool{}
	for _, r := range all {
		d := observe.Sufficiency{Reason: r}.Describe()
		if d == "" || d == string(r) {
			t.Errorf("%q has no description of its own; an owner would be shown the "+
				"enum value", r)
		}
		if seen[d] {
			t.Errorf("%q describes itself the same way as another reason: %q", r, d)
		}
		seen[d] = true
	}
}
