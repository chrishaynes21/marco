package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The decision about whether a place may be made durable, and the CLOSED reasons it may not.
//
// Every branch below is a sentence somebody trying to show Marco something will read, and they
// mean different things: "you have not asked me to learn anything" is not "hold still somewhere
// more distinctive" is not "I already know this screen". A single false would collapse all three,
// and the live failure this milestone came from was invisible for exactly that reason.

// placeTotals is one settled screen, described the way the segmenter describes one.
func placeTotals(terms ...observe.InterfaceTerm) observe.ShadowTotals {
	st := observe.ScreenState{
		ID: "state_1", Roles: map[string]int{"button": 4},
		Inferences: 12, Episodes: 3, TermObservations: 12,
		// SETTLED. The comment above already called this "one settled screen"; the
		// segmenter now says so in a field, and establishment reads it.
		Settled: true,
	}
	if len(terms) > 0 {
		st.Terms, st.TermEpisodes = map[observe.InterfaceTerm]int{}, map[observe.InterfaceTerm]int{}
		for _, term := range terms {
			st.Terms[term] = 12
			st.TermEpisodes[term] = 3
		}
	}
	return observe.ShadowTotals{CurrentState: "state_1", States: []observe.ScreenState{st}}
}

// recallStub answers Recall with a fixed verdict and records nothing else.
type recallStub struct {
	observe.Memory
	verdict observe.MatchVerdict
	subject string
}

func (m recallStub) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{
		Verdict: m.verdict,
		Subject: observe.RememberedSubject{ID: m.subject},
	}
}

func TestADiscriminatingUnknownPlaceMayBeEstablished(t *testing.T) {
	sig, refusal := observe.PlaceToEstablish(
		placeTotals(observe.TermSettings, observe.TermControls), "unknown-game",
		recallStub{verdict: observe.MatchDifferent}, observe.DefaultHypothesisThresholds())

	if refusal != "" {
		t.Fatalf("refused with %q a screen that is settled, discriminating and unknown", refusal)
	}
	if !sig.Discriminating() {
		t.Errorf("the signature offered for storage carries no discriminator: %+v", sig)
	}
	if sig.Subject != observe.SubjectState {
		t.Errorf("subject kind %q, want %q — a place is a screen",
			sig.Subject, observe.SubjectState)
	}
}

func TestAPlaceMarcoAlreadyRecognisesIsNotEstablishedAgain(t *testing.T) {
	_, refusal := observe.PlaceToEstablish(
		placeTotals(observe.TermSettings), "unknown-game",
		recallStub{verdict: observe.MatchSame, subject: "subj_known"},
		observe.DefaultHypothesisThresholds())

	if refusal != observe.PlaceAlreadyKnown {
		t.Fatalf("refusal %q, want %q. A second record for a screen Marco already knows is "+
			"how recall becomes ambiguous and stops recognising either of them",
			refusal, observe.PlaceAlreadyKnown)
	}
}

// A CANDIDATE match is not "already known", and must not suppress establishing.
//
// The bar the identity layer sets: `candidate` means the structure agrees and nothing distinctive
// confirms it, which is not recognition. Treating it as such would leave Learn blocked by a
// screen that merely has the same number of buttons.
func TestAMerelyStructuralResemblanceDoesNotCountAsKnown(t *testing.T) {
	_, refusal := observe.PlaceToEstablish(
		placeTotals(observe.TermSettings), "unknown-game",
		recallStub{verdict: observe.MatchCandidate, subject: "subj_similar"},
		observe.DefaultHypothesisThresholds())

	if refusal != "" {
		t.Fatalf("refused with %q because another screen has a similar shape. `candidate` is "+
			"Marco saying it cannot tell, and it may not stand in for knowing", refusal)
	}
}

func TestAPlaceWithNothingToRecogniseItByIsRefused(t *testing.T) {
	// No terms at all: no interface concepts were read, and a state subject carries no
	// envelope. There is nothing that could ever match this again.
	_, refusal := observe.PlaceToEstablish(
		placeTotals(), "unknown-game",
		recallStub{verdict: observe.MatchDifferent}, observe.DefaultHypothesisThresholds())

	if refusal != observe.PlaceNotDiscriminating {
		t.Fatalf("refusal %q, want %q", refusal, observe.PlaceNotDiscriminating)
	}
}

func TestAnUnsettledScreenIsNotDescribable(t *testing.T) {
	// A transition frame: the segmenter has settled nothing, so there is no place to name.
	totals := placeTotals(observe.TermSettings)
	totals.CurrentState = observe.ScreenStateUnknown

	_, refusal := observe.PlaceToEstablish(totals, "unknown-game",
		recallStub{verdict: observe.MatchDifferent}, observe.DefaultHypothesisThresholds())

	if refusal != observe.PlaceNotDescribable {
		t.Fatalf("refusal %q, want %q — \"I could not tell where you were\" and \"I could "+
			"tell and it is not worth storing\" are different sentences",
			refusal, observe.PlaceNotDescribable)
	}
}

func TestWithNoMemoryThereIsNowhereToEstablishAPlace(t *testing.T) {
	_, refusal := observe.PlaceToEstablish(placeTotals(observe.TermSettings), "unknown-game",
		nil, observe.DefaultHypothesisThresholds())

	if refusal != observe.PlaceNoMemory {
		t.Fatalf("refusal %q, want %q", refusal, observe.PlaceNoMemory)
	}
}

// The signature offered for storage is the one PlaceNow will look it up by.
//
// THE property the whole bootstrap rests on. Two derivations of "what screen is this" would give
// two answers, and a place stored under one and recalled under the other would be a place Marco
// established and then could not find.
func TestAPlaceIsStoredUnderTheSignatureRecallWillUse(t *testing.T) {
	totals := placeTotals(observe.TermSettings, observe.TermControls)
	th := observe.DefaultHypothesisThresholds()

	offered, refusal := observe.PlaceToEstablish(totals, "unknown-game",
		recallStub{verdict: observe.MatchDifferent}, th)
	if refusal != "" {
		t.Fatalf("nothing was offered for storage: %q", refusal)
	}
	looked, ok := observe.SignatureOfState(totals, totals.CurrentState, th)
	if !ok {
		t.Fatal("the canonical lookup produced no signature")
	}
	if observe.CompareStructure(offered, looked) != observe.MatchSame {
		t.Fatalf("the signature a place is STORED under does not match the one it is LOOKED "+
			"UP by:\n  stored: %+v\n  looked: %+v", offered, looked)
	}
}
