package observesession_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// A durable identity describes the PLACE, not how much of the session has happened.
//
// # The defect these were written to catch
//
// A demonstration between two places inside one application reported `start_unverifiable` —
// memory could not recognise the place the user was standing on — while the same two places were
// two durable subjects that resolved by name and were told apart by a learned play's guard.
//
// It was never two matchers. Capture, Recall, `Screen's Showing` and relationship endpoints all
// go through `SignatureOfState` → `Memory.Recall`. What differed was WHEN the question was asked:
// the guard asks about a settled world, the capture asks in the middle of the event it is trying
// to record, and one component of the signature moved between those two moments.
//
//	the same place, same roles, same terms, same application
//	  observed alone                 24 members
//	  observed beside its neighbour   12 members      tolerance: 1
//
// `Members` was borrowed from the state's dominant structural group. A group is made of tracks
// persistent in EXACTLY ONE state, so the chrome a place shares with its neighbours counts toward
// it until a neighbour is seen and that structure becomes ambient. A screen is not its dominant
// group; the count was a property of the session's progress. It is gone from a state's
// fingerprint, and groups keep theirs, where it is intrinsic.
//
// These are now the regression.

// Memory recognises both places of one surface, asked with a settled session's evidence.
func TestMemoryRecognisesBothPlacesOfOneSurface(t *testing.T) {
	store := newStore(t)
	journey := journeyScript(placeA(), placeB())
	learn(t, store, journey)
	if n := len(store.Subjects()); n != 2 {
		t.Fatalf("the fixture stored %d subject(s)", n)
	}

	for term, sig := range placeSignatures(t, run(t, journey)) {
		rec := store.Recall("testgame", sig)
		t.Logf("%-8s → %-10s %s   roles=%v", term, rec.Verdict, rec.Subject.ID, sig.Roles)
		if !rec.Verdict.Established() {
			t.Errorf("memory cannot establish the place carrying %s", term)
		}
	}
}

// THE regression. A place is recognised while it is all the session has seen.
//
// This is what a demonstration needs: it is armed, it is standing on its starting place, and it
// has been nowhere else — by definition, because going somewhere else is the thing it is waiting
// to record.
func TestAPlaceIsRecognisedWhileItIsAllTheSessionHasSeen(t *testing.T) {
	store := newStore(t)
	learn(t, store, journeyScript(placeA(), placeB()))

	a := placeA()
	for _, n := range []int{1, 2, 4, 6, 10} {
		got := run(t, script{frames: reading(stayOn(a.surface.Regions(), n), a.terms...)})
		sh := got.Stats.Shadow
		sig, ok := observe.SignatureOfState(
			sh, sh.CurrentState, observe.DefaultHypothesisThresholds())
		if !ok {
			t.Errorf("%d frames on one place produced no signature", n)
			continue
		}
		rec := store.Recall("testgame", sig)
		t.Logf("%2d frames alone  members=%d  terms=%v  → %s",
			n, sig.Members, sig.Terms, rec.Verdict)
		if !rec.Verdict.Established() {
			t.Errorf("after %d frames the place memory stored is %s; a demonstration armed "+
				"here can never begin, and waiting does not help — only going somewhere "+
				"else would, which is what it is waiting to record", n, rec.Verdict)
		}
		if sig.Members != 0 {
			t.Errorf("a screen carries a member count of %d; it borrowed one from a group "+
				"again, and that count moves as the session sees more", sig.Members)
		}
	}
}

// THE headline invariant: observation order does not change durable identity.
//
// Three sessions over the same places, in three orders. What a place IS must not depend on which
// one the session met first, or on whether it met the other at all.
func TestObservationOrderDoesNotChangeDurableIdentity(t *testing.T) {
	a, b := placeA(), placeB()
	orders := map[string]script{
		"A alone":  {frames: reading(stayOn(a.surface.Regions(), 8), a.terms...)},
		"A then B": journeyScript(a, b),
		"B then A": journeyScript(b, a),
	}

	seen := map[string]observe.StructureSignature{}
	for name, s := range orders {
		sigs := placeSignatures(t, run(t, s))
		sig, ok := sigs[observe.TermSettings]
		if !ok {
			t.Fatalf("%s: the session did not recognise the place under test", name)
		}
		t.Logf("%-10s roles=%v members=%d terms=%v", name, sig.Roles, sig.Members, sig.Terms)
		seen[name] = sig
	}

	for from, x := range seen {
		for to, y := range seen {
			if from == to {
				continue
			}
			if v := observe.CompareStructure(x, y); v == observe.MatchDifferent {
				t.Errorf("the same place observed %q and %q compares as %s; durable identity "+
					"depends on what else the session happened to witness", from, to, v)
			}
		}
	}
}

// Cold start, from disk, having seen nothing else.
//
// The product consequence, and the one a person would notice: Marco remembers a place today and
// recognises it tomorrow without first having to visit its neighbours.
func TestAStoredPlaceIsRecognisedColdFromDisk(t *testing.T) {
	store := newStore(t)
	learn(t, store, journeyScript(placeA(), placeB()))
	path := store.Path()

	reopened, why := semanticmemory.Open(path)
	if reopened == nil {
		t.Fatalf("memory did not reopen: %s", why)
	}

	for _, p := range []struct {
		name string
		in   placeIn
	}{
		{"A", placeA()},
		{"B", placeB()},
	} {
		// A brand new session that visits ONLY this place.
		got := run(t, script{
			frames: reading(stayOn(p.in.surface.Regions(), 6), p.in.terms...)})
		sh := got.Stats.Shadow
		sig, ok := observe.SignatureOfState(
			sh, sh.CurrentState, observe.DefaultHypothesisThresholds())
		if !ok {
			t.Fatalf("%s: no signature", p.name)
		}
		rec := reopened.Recall("testgame", sig)
		t.Logf("cold %s → %s %s", p.name, rec.Verdict, rec.Subject.ID)
		if !rec.Verdict.Established() {
			t.Errorf("cold-start %s is %s; recognition still needs session context",
				p.name, rec.Verdict)
		}
	}
}

// One screen must not become two records.
//
// ORIGINAL_MEMBERS_REGRESSION. `Members` entered durable identity because four fingerprint
// constructors populated it inconsistently — the SAME screen described by two hypotheses reported
// 4 members under one and 0 under the other, compared as different, and was stored twice.
//
// The repair for that was ONE constructor, not the field. This proves the guarantee survives the
// field's removal: every hypothesis about one screen must agree about what it IS, however much
// they disagree about what it MEANS.
func TestOneScreenIsOneRecordHoweverManyHypothesesDescribeIt(t *testing.T) {
	got := run(t, journeyScript(placeA(), placeB()))

	byState := map[string][]observe.StructureSignature{}
	for _, h := range got.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		byState[h.Subject.Ref] = append(byState[h.Subject.Ref], observe.SignatureOf(h))
	}
	if len(byState) == 0 {
		t.Fatal("the session produced no screen hypotheses")
	}

	store := newStore(t)
	stored := 0
	for ref, sigs := range byState {
		t.Logf("%s described by %d hypothes(es)", ref, len(sigs))
		for i := 1; i < len(sigs); i++ {
			if v := observe.CompareStructure(sigs[0], sigs[i]); v == observe.MatchDifferent {
				t.Errorf("two hypotheses about %s disagree about what it IS (%s); memory "+
					"would store one screen twice", ref, v)
			}
		}
		kept := false
		for _, sig := range sigs {
			if !sig.Discriminating() {
				continue
			}
			if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
				Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
			}); err == nil {
				kept = true
			}
		}
		if kept {
			stored++
		}
	}
	if n := len(store.Subjects()); n != stored {
		t.Errorf("%d screen(s) with a discriminator produced %d durable subject(s)", stored, n)
	}
}

// placeSignatures is one durable signature per recognisable place a session found.
func placeSignatures(t *testing.T,
	got observesession.Result) map[observe.InterfaceTerm]observe.StructureSignature {

	t.Helper()
	out := map[observe.InterfaceTerm]observe.StructureSignature{}
	for _, h := range got.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		sig := observe.SignatureOf(h)
		for _, term := range sig.Terms {
			if term == observe.TermSettings || term == observe.TermAudio {
				if _, seen := out[term]; !seen {
					out[term] = sig
				}
			}
		}
	}
	return out
}
