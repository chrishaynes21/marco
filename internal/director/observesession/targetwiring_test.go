package observesession_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// A thing the person pressed becomes something Marco knows, and still knows tomorrow.
//
// # What this page is for
//
// Learn watched somebody click a control and resolved its name. Until now that name lived in the
// demonstration and nowhere else: evidence of one event, not knowledge of a thing. A play could
// therefore only refer to it by describing how it had been found, which welded the provider that
// found it into the behaviour forever.
//
// These hold the other half: the control becomes a durable semantic TARGET — a name, a sort, and
// a place — and it survives a restart with no trace of how it was originally perceived.
//
// See [[ADR-068-the-theater-is-the-durable-semantic-world]].

// clicked is a demonstration in which the person pressed a named control on the way.
func clicked(label, role string) observe.ProcedureCandidate {
	return observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: "subj_here", To: "subj_there"},
		Application:  "testgame",
		Start:        observe.Checkpoint{Subject: "subj_here"},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{observe.NavPoint},
			Targets: []observe.SemanticTarget{{Role: role, Label: label}},
			Arrived: observe.Checkpoint{Subject: "subj_there"},
		}},
		Complete: true,
	}
}

// ── what a demonstration says about targets ───────────────────────────────────

// A demonstrated click yields a target grounded in the place it was pressed ON.
//
// The direction matters and is easy to get backwards. A person on the Bluetooth screen presses
// "Mouse" and ends up on the Mouse screen; the target belongs to BLUETOOTH, because that is where
// it can be found again. Grounding it on the arrival screen would make every target live on the
// page it navigates away to, and no play could ever resolve one.
func TestADemonstratedTargetIsGroundedWhereItWasPressed(t *testing.T) {
	got := observe.TargetsDemonstrated(clicked("Mouse", "button"))

	if len(got) != 1 {
		t.Fatalf("%d target(s) from one click: %+v", len(got), got)
	}
	if got[0].Place != "subj_here" {
		t.Errorf("the target is grounded in %q, want the place it was pressed on "+
			"(subj_here). A target on the screen you arrive at cannot be found from the "+
			"screen you start on.", got[0].Place)
	}
	if got[0].Label != "Mouse" || got[0].Kind != string(observe.KindButton) {
		t.Errorf("target is %+v, want Mouse/button", got[0])
	}
	if got[0].Subject != observe.SubjectTarget {
		t.Errorf("subject kind is %q", got[0].Subject)
	}
}

// A click nobody could name yields no target.
//
// There is nothing to be found by, so storing one would add a record nothing can ever resolve.
func TestAnUnnamedClickYieldsNoTarget(t *testing.T) {
	c := clicked("", "button")
	if got := observe.TargetsDemonstrated(c); len(got) != 0 {
		t.Errorf("%d target(s) from a click with no name: %+v", len(got), got)
	}
}

// A provider's own control-type never reaches the durable kind.
//
// `list_item` is what Windows calls a row. Marco calls it an item, and a target that remembered
// the provider's word would be a target whose meaning depended on the provider that found it.
func TestAProvidersControlTypeIsNarrowedToMarcosVocabulary(t *testing.T) {
	got := observe.TargetsDemonstrated(clicked("Bluetooth & devices", "list_item"))
	if len(got) != 1 {
		t.Fatalf("%d target(s)", len(got))
	}
	if got[0].Kind != string(observe.KindItem) {
		t.Errorf("kind is %q, want %q — a durable target must not remember that one "+
			"operating system calls this a ListItem", got[0].Kind, observe.KindItem)
	}
	// And a role Marco has no word for says nothing rather than inventing one.
	odd := observe.TargetsDemonstrated(clicked("Whatsit", "custom_widget_v2"))
	if len(odd) != 1 || odd[0].Kind != "" {
		t.Errorf("an unrecognised role produced kind %+v, want empty — unknown is not false",
			odd)
	}
}

// ── durability ────────────────────────────────────────────────────────────────

// A target becomes durable, and is still there after a restart, resolved by name.
//
// # Why the store is REOPENED
//
// Because the claim is about a file, not about a map. The second handle is what a later Director
// has: a fresh read of the bytes on disk, with nothing carried over in memory.
func TestADemonstratedTargetSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	store, _ := semanticmemory.Open(path)
	sig := observe.TargetSignature("subj_here", "Mouse", observe.KindButton)
	id, err := store.RememberTarget("testgame", sig, observe.FromAccessible)
	if err != nil {
		t.Fatalf("remembering the target: %v", err)
	}

	// THE RESTART. A different Store over the same file, as a new process would have.
	reopened, note := semanticmemory.Open(path)
	if note != "" {
		t.Fatalf("reopening: %s", note)
	}
	rec := reopened.ResolveTarget("testgame", sig)
	if !rec.Verdict.Established() {
		t.Fatalf("a target remembered before the restart recalls as %q. A play saved "+
			"yesterday cannot refer to anything if the target it names is gone.",
			rec.Verdict)
	}
	if rec.Subject.ID != id {
		t.Errorf("resolved to %q, want the target that was stored (%q)", rec.Subject.ID, id)
	}
	// PROVENANCE survives too — and is still only provenance.
	if rec.Subject.Learned != observe.FromAccessible {
		t.Errorf("provenance after restart is %q, want %q", rec.Subject.Learned,
			observe.FromAccessible)
	}
	// And nothing about HOW it was found came with it.
	if rec.Subject.Structure.Envelope != nil || len(rec.Subject.Structure.Roles) != 0 {
		t.Errorf("the restored target carries perception geometry: %+v",
			rec.Subject.Structure)
	}
}

// The durable target holds no provider handle, and there is nowhere for one to go.
//
// A shape assertion rather than a value one: the failure this guards is somebody ADDING a field
// for a runtime id because it would be convenient, and a test that only checked values would pass
// until the first time one was set.
func TestADurableTargetHasNowhereToPutAProviderHandle(t *testing.T) {
	forbidden := []string{
		"runtimeid", "runtime", "elementid", "element", "node", "handle", "hwnd",
		"rect", "bounds", "box", "x", "y", "point", "coordinate", "pixel",
	}
	for _, bad := range forbidden {
		if fieldNamed(bad) {
			t.Errorf("a target signature has a %q field. That is how Marco FOUND it once, "+
				"not what it IS — a target holding one stops working at the first "+
				"redraw.", bad)
		}
	}
}

// Two things called the same name in DIFFERENT places are different targets.
//
// "Mouse" means something else on another screen, and merging them would let a play resolve to a
// control the person never demonstrated.
func TestTheSameNameInAnotherPlaceIsAnotherTarget(t *testing.T) {
	here := observe.TargetSignature("subj_here", "Mouse", observe.KindButton)
	elsewhere := observe.TargetSignature("subj_elsewhere", "Mouse", observe.KindButton)
	if observe.CompareStructure(here, elsewhere) == observe.MatchSame {
		t.Error("the same word on two different screens was treated as one target")
	}
}

// A name is matched exactly, not tolerantly.
//
// Every other subject here is matched with slack, because a screen legitimately gains a scroll
// bar. A target has no such slack: two differently-named buttons on one screen are two things.
func TestTargetNamesAreMatchedExactly(t *testing.T) {
	mouse := observe.TargetSignature("subj_here", "Mouse", observe.KindButton)
	touchpad := observe.TargetSignature("subj_here", "Touchpad", observe.KindButton)
	if observe.CompareStructure(mouse, touchpad) == observe.MatchSame {
		t.Fatal("two differently-named controls on one screen were treated as one target")
	}
	// Case and surrounding space are how a label was rendered, not what it says.
	spaced := observe.TargetSignature("subj_here", "  mouse ", observe.KindButton)
	if observe.CompareStructure(mouse, spaced) != observe.MatchSame {
		t.Error("the same control stopped matching itself because a provider trimmed " +
			"differently")
	}
}

// fieldNamed reports whether a target signature has a field whose name contains this word.
//
// REFLECTIVE, so a field somebody adds tomorrow is caught. A hard-coded list would only ever
// describe the type as it was on the day the test was written, which is the opposite of a guard.
func fieldNamed(word string) bool {
	rt := reflect.TypeOf(observe.StructureSignature{})
	for i := 0; i < rt.NumField(); i++ {
		if strings.Contains(strings.ToLower(rt.Field(i).Name), word) {
			return true
		}
	}
	return false
}

// ── the production wiring ─────────────────────────────────────────────────────

// A demonstrated target becomes durable through the REAL session, not a helper.
//
// # Why this enters through a session
//
// Because everything above this line is about the pieces: what a demonstration says, what a
// signature is, what the store does when asked. None of that fires if nobody asks. This is the
// asking — a licensed session that watched somebody click a named control, and a store that holds
// the target afterwards.
//
// Mutations that must fail this: deleting the rememberTargets call from watchedDemonstration;
// deleting the WithTargets wiring in the observation registry; making the establishment
// conditional on anything the demonstration licence does not already guarantee.
func TestADemonstratedTargetBecomesDurable(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	// The person walks the route, and on the way presses something Marco can name.
	script := targetedScript("Mouse", "button")
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store).WithTargets(store)
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// THE store, reopened, because durability is the claim.
	targets := durableTargets(memoryAt(t, dir))
	if len(targets) == 0 {
		t.Fatalf("the person pressed a control Marco named and nothing durable came of it.\n"+
			"A play can only refer to a target that exists; without this the whole "+
			"provider-neutral chain has nothing to point at.\nsubjects: %d",
			len(memoryAt(t, dir).Subjects()))
	}
	var found bool
	for _, s := range targets {
		if s.Structure.Label == "Mouse" {
			found = true
			if s.Structure.Place == "" {
				t.Error("the target is grounded nowhere, so nothing could ever find it")
			}
			if s.Structure.Kind != string(observe.KindButton) {
				t.Errorf("kind is %q, want button", s.Structure.Kind)
			}
			// PROVENANCE recorded, and it is only provenance.
			if s.Structure.Subject != observe.SubjectTarget {
				t.Errorf("subject kind is %q", s.Structure.Subject)
			}
			// NO judgement. A target is remembered; nothing is claimed about it.
			if len(s.Knowledge) != 0 {
				t.Errorf("the target carries %d interpretation(s); nobody was asked "+
					"anything about it", len(s.Knowledge))
			}
			if s.Called != "" {
				t.Errorf("the target is Called %q; no person named it, and an observed "+
					"label must never be recorded as somebody's own word", s.Called)
			}
		}
	}
	if !found {
		t.Errorf("no durable target is called Mouse: %+v", targets)
	}
}

// An UNLICENSED session establishes no target, however much it watches.
//
// The control, and the privacy boundary in one line: a target becomes durable only because a
// person explicitly asked to be taught and then acted. Passive observation persists nothing.
func TestAnUnlicensedSessionEstablishesNoTarget(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	// A store that ALREADY holds the edge, so this session grows a durable route and the
	// discovery path has something to build a candidate from. Without that the session
	// would establish nothing for want of evidence rather than for want of a licence, and
	// the test would pass while proving nothing.
	seedRelationshipIn(t, store, 3, strongEvidence())

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: targetedScript("Mouse", "button")}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store).WithTargets(store)
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := len(durableTargets(memoryAt(t, dir))); n != 0 {
		t.Fatalf("an unlicensed session made %d target(s) durable. Marco remembers what a "+
			"person showed it on purpose, not everything it happened to see somebody "+
			"click.", n)
	}
}

// targetedScript walks the happy route with a NAMED click on the way.
func targetedScript(label, role string) []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, demoFrame{
		screen:  "x",
		inputs:  []observe.NavIntent{observe.NavPoint},
		targets: []observe.SemanticTarget{{Role: role, Label: label}},
	})
	out = append(out, hold("x", 3)...)
	out = append(out, press("b", observe.NavConfirm))
	out = append(out, hold("b", 4)...)
	return out
}

// durableTargets is every target subject a store holds.
func durableTargets(s *semanticmemory.Store) []observe.RememberedSubject {
	var out []observe.RememberedSubject
	for _, r := range s.Subjects() {
		if r.Structure.Subject == observe.SubjectTarget {
			out = append(out, r)
		}
	}
	return out
}

// A resolver that cannot tell what SORT of thing it found still matches the target.
//
// # Why this is load-bearing for portability
//
// Unknown is not false. The accessibility tree says "button" because it has a control-type; a
// sighted resolver may find the same thing and have no opinion about what kind it is. If a missing
// kind counted as a disagreement, every target learned through accessibility would be unmatchable
// by anything else — which is exactly the provider lock-in the semantic target exists to prevent.
//
// A kind that CONTRADICTS is still a disagreement. Saying nothing and saying something else are
// different, and only one of them is evidence.
func TestAnUnknownKindDoesNotDisagree(t *testing.T) {
	known := observe.TargetSignature("subj_here", "Mouse", observe.KindButton)
	silent := observe.TargetSignature("subj_here", "Mouse", "")

	if got := observe.CompareStructure(silent, known); got != observe.MatchSame {
		t.Errorf("a resolver with no opinion about the kind got %q against a target learned "+
			"as a button.\nUnknown is not false — treating it as a disagreement makes "+
			"every accessibility-trained target unreachable by anything else.", got)
	}
	if got := observe.CompareStructure(known, silent); got != observe.MatchSame {
		t.Errorf("the comparison is not symmetric: %q", got)
	}
	// And a contradiction still is one.
	other := observe.TargetSignature("subj_here", "Mouse", observe.KindField)
	if got := observe.CompareStructure(known, other); got == observe.MatchSame {
		t.Error("a button and a text field sharing a name were treated as one target")
	}
}
