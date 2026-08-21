package observesession_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The three permissions a session may hold, and the fact that they are three.
//
// # What this replaces
//
// One boolean, `Episode.EstablishPlaces`, read at three production sites that permit three
// unrelated things: making a Place's identity durable, turning a watched pass into candidate
// route evidence, and widening which controls may keep their name. Learn was the only caller
// that set it, so the weld cost nothing and was invisible — the comment above the field said
// a caller wanting one without the other "would be claiming to be a learn session without
// being one".
//
// That is exactly what an always-on Observe is. It may want to notice a habit it keeps seeing
// without thereby acquiring the right to write down the text of every control somebody clicks.
//
// These tests are about the SHAPE of the permission, not about behaviour: Learn's outcomes are
// unchanged and the existing wiring tests hold that. What is new is that the grant can be read,
// and that holding one permission does not smuggle in the others.

// LEARN STILL RECEIVES ALL THREE, and this is the behaviour-neutrality claim.
//
// Deleting a permission from LearnLicence must fail this.
func TestLearnReceivesEveryPermission(t *testing.T) {
	l := observesession.LearnLicence()
	if !l.EstablishPlaces {
		t.Error("Learn may no longer make a Place's identity durable")
	}
	if !l.AcquireRouteEvidence {
		t.Error("Learn may no longer turn what it watched into candidate route evidence")
	}
	if !l.NameActivatedTargets {
		t.Error("Learn may no longer keep the name of the control the person aimed at")
	}
	if !l.Any() {
		t.Error("Learn's licence reports that it permits nothing")
	}
}

// AN OBSERVATION CONTEXT CAN BE CONSTRUCTED WITH NONE.
//
// The zero Episode is a pure observation: it may look, and it may make nothing durable. This is
// the shape `marco observe` will start from, and it needs no Learn session to exist for it.
func TestAnObservationContextHoldsNoPermissions(t *testing.T) {
	var ep observesession.Episode
	if ep.Any() {
		t.Fatalf("the zero Episode permits something durable: %+v", ep.Licence)
	}
	if ep.EstablishPlaces || ep.AcquireRouteEvidence || ep.NameActivatedTargets {
		t.Errorf("a session nobody granted anything holds %+v", ep.Licence)
	}
}

// AND WITH EXACTLY ONE. Each permission is granted alone, and grants nothing else.
//
// This is the independence claim, and it is the whole point of the split. Mutating one field of
// Licence must not move either of the others — which a single boolean read three times could
// never satisfy, and which is why a future Observe policy could not be expressed at all.
func TestEachPermissionIsHeldAlone(t *testing.T) {
	for _, c := range []struct {
		name  string
		grant observesession.Licence
		// want is the one field that must be true, read back by name.
		read func(observesession.Licence) bool
		// others must all be false.
		others map[string]func(observesession.Licence) bool
	}{
		{
			name:  "may establish a Place, and nothing else",
			grant: observesession.Licence{EstablishPlaces: true},
			read:  func(l observesession.Licence) bool { return l.EstablishPlaces },
			others: map[string]func(observesession.Licence) bool{
				"AcquireRouteEvidence": func(l observesession.Licence) bool { return l.AcquireRouteEvidence },
				"NameActivatedTargets": func(l observesession.Licence) bool { return l.NameActivatedTargets },
			},
		},
		{
			name:  "may acquire route evidence, and nothing else",
			grant: observesession.Licence{AcquireRouteEvidence: true},
			read:  func(l observesession.Licence) bool { return l.AcquireRouteEvidence },
			others: map[string]func(observesession.Licence) bool{
				"EstablishPlaces":      func(l observesession.Licence) bool { return l.EstablishPlaces },
				"NameActivatedTargets": func(l observesession.Licence) bool { return l.NameActivatedTargets },
			},
		},
		{
			name:  "may name the activated target, and nothing else",
			grant: observesession.Licence{NameActivatedTargets: true},
			read:  func(l observesession.Licence) bool { return l.NameActivatedTargets },
			others: map[string]func(observesession.Licence) bool{
				"EstablishPlaces":      func(l observesession.Licence) bool { return l.EstablishPlaces },
				"AcquireRouteEvidence": func(l observesession.Licence) bool { return l.AcquireRouteEvidence },
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			ep := observesession.Episode{Licence: c.grant}
			if !c.read(ep.Licence) {
				t.Fatal("the permission that was granted does not read as granted")
			}
			if !ep.Any() {
				t.Error("a session holding one permission reports that it permits nothing")
			}
			for other, read := range c.others {
				if read(ep.Licence) {
					t.Errorf("granting one permission also granted %s. They are separate "+
						"questions with separate consumers, and a caller that wanted one "+
						"must not receive the rest.", other)
				}
			}
		})
	}
}

// THE PERMISSIONS ARE NOT THE CONTEXT.
//
// SameEpisode and PermissionExpected say what KIND of session this is — how its corroboration is
// counted, and whether somebody is sitting there waiting to be asked. Neither permits anything
// durable, and a caller setting them must not thereby acquire a licence.
func TestSessionContextGrantsNoPermission(t *testing.T) {
	ep := observesession.Episode{SameEpisode: true, PermissionExpected: true}
	if ep.Any() {
		t.Errorf("declaring what kind of session this is granted %+v. Context and permission "+
			"are different questions; that they were one field is what Roadmap 35A separated.",
			ep.Licence)
	}
}
