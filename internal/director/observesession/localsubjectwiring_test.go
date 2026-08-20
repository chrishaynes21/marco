package observesession_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Does the two-level identity reach DURABLE memory, or stop at the session?
//
// The previous milestone split screen identity into a surface and a place inside it, and proved
// the split through the session and out to what a person reads. It did not prove the half that
// matters to everything else: a name, a guard, a relationship and a verification all resolve
// through `SignatureOf(hypothesis)` into the semantic store, and if that derivation cannot tell
// two places inside one application apart then the split is a diagnostic and nothing more.
//
// # What the trace found before any of this was written
//
// Every durable path is ALREADY per-state — `stateFingerprint` reads one `ScreenState`, and the
// store is namespaced by application. `SurfaceID` reaches nothing durable and is not meant to.
// So the question is not "does durable identity describe the surface or the state"; it is
// whether two states of one surface produce signatures the store reads as two subjects.
//
// That is a question about the durable signature's VOCABULARY, and it has two answers depending
// on how the two places differ. Both are below.

// oneSurface is an application shell with a smaller region inside it that carries the state.
//
// Sized so the two states are unambiguously the same surface (0.714, against a bar of 0.55)
// rather than borderline. A fixture sitting on a threshold tests the threshold, not the claim.
func oneSurface() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 60, Content: 12, ContentRole: "list_item"}
}

// twoPlacesInOneSurface runs a session that visits both, twice, with each place read.
//
// Terms are supplied because a screen with neither read text nor an envelope carries no
// discriminator and can never become durable at all — a true and separate limit, and not the one
// under test here.
func twoPlacesInOneSurface(t *testing.T, b screenfixture.Surface,
	readA, readB []observe.InterfaceTerm) (
	observe.ShadowTotals, map[observe.ScreenStateID]observe.StructureSignature) {

	t.Helper()
	a := oneSurface()
	var frames []frame
	for range 2 {
		frames = append(frames, reading(stayOn(a.Regions(), 5), readA...)...)
		frames = append(frames, reading(stayOn(b.Regions(), 5), readB...)...)
	}
	got := run(t, script{frames: frames})

	sigs := map[observe.ScreenStateID]observe.StructureSignature{}
	for _, h := range got.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		id := observe.ScreenStateID(h.Subject.Ref)
		if _, seen := sigs[id]; !seen {
			sigs[id] = observe.SignatureOf(h)
		}
	}
	return got.Stats.Shadow, sigs
}

// L. A local state can become a durable subject of its own.
//
// THE migration test. Two places inside one application shell, distinguished by what their
// state-bearing region is MADE OF, must reach the store as two subjects — because a name, a
// start guard, a destination check and a relationship endpoint all resolve through exactly this
// derivation, and every one of them is wrong if it collapses.
func TestTwoPlacesInOneSurfaceBecomeTwoDurableSubjects(t *testing.T) {
	sh, sigs := twoPlacesInOneSurface(t, oneSurface().ContentReplaced("checkbox"),
		[]observe.InterfaceTerm{observe.TermControls, observe.TermSettings},
		[]observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay})

	if len(sh.States) != 2 {
		t.Fatalf("the session found %d state(s); the premise of this test is two", len(sh.States))
	}
	surfaces := map[observe.SurfaceID]bool{}
	for _, st := range sh.States {
		surfaces[st.SurfaceOf()] = true
	}
	if len(surfaces) != 1 {
		t.Fatalf("the two places were assigned %d surfaces; this test is about two places "+
			"inside ONE", len(surfaces))
	}
	if len(sigs) != 2 {
		t.Fatalf("two states produced %d durable signature(s)", len(sigs))
	}

	var pair []observe.StructureSignature
	for id, sig := range sigs {
		if !sig.Discriminating() {
			t.Errorf("state %s carries no discriminator, so nothing could ever recognise "+
				"it again", id)
		}
		pair = append(pair, sig)
	}
	if v := observe.CompareStructure(pair[0], pair[1]); v != observe.MatchDifferent {
		t.Fatalf("two places inside one application compare as %s.\nMarco would call them "+
			"one screen: naming one names both, a play guarded on one would start on the "+
			"other, and a destination check could not tell arrival from never having left.\n"+
			"  %+v\n  %+v", v, pair[0], pair[1])
	}

	// And the store agrees, through its own path rather than through the comparison above.
	store, _ := semanticmemory.Open(t.TempDir() + "/memory.json")
	for _, sig := range pair {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("remembering a place: %v", err)
		}
	}
	if n := len(store.Subjects()); n != 2 {
		t.Fatalf("two places inside one application were stored as %d subject(s)", n)
	}
	if a, b := store.Subjects()[0].ID, store.Subjects()[1].ID; a == b {
		t.Fatalf("both places were stored under one id %s", a)
	}
}

// And the recorded limit, measured rather than argued.
//
// The durable signature carries role COMPOSITION — which roles, how many, which read terms — and
// no arrangement. The local comparison carries `role@col,row`. So two places that differ only in
// WHERE the same structure sits are two states in the session and one subject in the store.
//
// This is not a bug in either. It is the seam between them, and the honest thing is to know
// exactly where it runs before deciding whether to move it. See the closeout: the missing
// dimension is coarse occupancy in the DURABLE signature, not anything perception is failing to
// observe.
func TestTwoPlacesDifferingOnlyInArrangement(t *testing.T) {
	moved := oneSurface()
	moved.ContentMoved = true

	// The SAME terms on both. Different text would be doing the separating, and the
	// question here is what the STRUCTURE can carry.
	same := []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}
	sh, sigs := twoPlacesInOneSurface(t, moved, same, same)
	t.Logf("%d state(s), %d durable signature(s)", len(sh.States), len(sigs))

	if len(sh.States) < 2 {
		t.Skip("the session did not separate the two arrangements, so there is no seam " +
			"to measure here")
	}
	var pair []observe.StructureSignature
	for _, sig := range sigs {
		pair = append(pair, sig)
	}
	if len(pair) != 2 {
		t.Fatalf("%d states produced %d signatures", len(sh.States), len(pair))
	}

	v := observe.CompareStructure(pair[0], pair[1])
	t.Logf("two arrangements of the same structure compare as %s", v)
	if v == observe.MatchDifferent {
		t.Log("the durable signature now separates arrangement; the seam recorded in " +
			"ADR-039 and the milestone closeout has moved — update both")
	}
}
