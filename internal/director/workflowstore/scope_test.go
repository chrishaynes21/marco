package workflowstore_test

// A WORKFLOW MAY ONLY ACT ON THE APPLICATION IT WAS SHOWN IN.
//
// The owner called this release-blocking for pixel execution, and the reason is the state it
// replaces: `Artifact` recorded no application at all, so a workflow demonstrated against Ark
// and pointed at Solasta would acquire Solasta's window and resolve Ark's controls against it.
// Nothing was in the way but a person answering a screen question.
//
//	Existing artifacts without application scope must remain readable, but they must not
//	silently become valid for every application.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func anArkWorkflow() workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt:     time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"},
		}},
	}
}

// AN ARK WORKFLOW IS REFUSED AGAINST SOLASTA, and the refusal names both.
func TestAnArkWorkflowIsRefusedAgainstAnotherGame(t *testing.T) {
	a := anArkWorkflow()
	ok, why := a.ScopeAllows("solasta")
	if ok {
		t.Fatal("an Ark workflow was permitted to act on Solasta")
	}
	for _, want := range []string{"arkascended", "solasta", "may only act on"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not mention %q: %s", want, why)
		}
	}
	// AND IT IS PERMITTED AGAINST ARK, or the gate would refuse everything and be
	// indistinguishable from a broken one.
	if ok, why := a.ScopeAllows("arkascended"); !ok {
		t.Errorf("an Ark workflow was refused against Ark: %s", why)
	}
	// CASE AND WHITESPACE ARE NOT IDENTITY, because the two sides are written at
	// different times by different code paths.
	if ok, _ := a.ScopeAllows("  ArkAscended  "); !ok {
		t.Error("the same application spelled differently was refused")
	}
}

// AN UNSCOPED WORKFLOW IS VALID FOR NOTHING, which is not the same as valid for everything.
//
// Every artifact saved before this existed is unscoped. The dangerous reading of an empty field
// is "matches anything", and it is the reading a zero value invites.
func TestALegacyUnscopedWorkflowIsValidForNoApplication(t *testing.T) {
	a := anArkWorkflow()
	a.Application = ""

	if a.Scoped() {
		t.Error("an artifact with no application reports that it has one")
	}
	for _, app := range []string{"arkascended", "solasta", "notepad", ""} {
		ok, why := a.ScopeAllows(app)
		if ok {
			t.Errorf("an unscoped workflow was permitted to act on %q", app)
		}
		if !strings.Contains(why, "UNSCOPED") {
			t.Errorf("the refusal for %q does not say it is unscoped: %s", app, why)
		}
	}
	// AND IT STILL LOADS, which is the other half of the requirement: a routine saved
	// last month is readable, and simply not runnable until it is bound.
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatalf("an unscoped artifact no longer loads: %v", err)
	}
	if back.Scoped() {
		t.Error("an unscoped artifact came back scoped")
	}
	// AND IT CARRIES NO `application` KEY AT ALL, so a file saved before today is
	// byte-identical through a round trip.
	if strings.Contains(string(body), `"application"`) {
		t.Errorf("an unscoped artifact writes an application key:\n%s", body)
	}
}

// BINDING TAKES OBSERVED PROVENANCE, AND WILL NOT REBIND.
//
//	Never blindly trust an application name supplied by the caller if it disagrees with
//	observed provenance. […] never silently bind it based on whichever application happens to
//	be foreground.
func TestALegacyWorkflowIsBoundDeliberatelyAndNeverRebound(t *testing.T) {
	legacy := anArkWorkflow()
	legacy.Application = ""

	bound, err := legacy.ScopedTo("arkascended")
	if err != nil {
		t.Fatalf("binding a legacy workflow from observed provenance failed: %v", err)
	}
	if !bound.Scoped() {
		t.Fatal("the bound workflow has no scope")
	}
	if ok, _ := bound.ScopeAllows("arkascended"); !ok {
		t.Error("the bound workflow is not valid for the application it was bound to")
	}
	// AND THE ORIGINAL IS UNTOUCHED, because the caller holds what is on disk.
	if legacy.Scoped() {
		t.Error("binding mutated the artifact passed in")
	}
	// REBINDING IS REFUSED. It is how a workflow would quietly become valid for a
	// second application, and a workflow for another application is another
	// demonstration.
	if _, err := bound.ScopedTo("solasta"); err == nil {
		t.Error("a scoped workflow was rebound to another application")
	} else if !strings.Contains(err.Error(), "another demonstration") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	// AND BINDING TO NOTHING IS REFUSED, so a session that observed no application
	// cannot produce a workflow that is scoped to the empty string — which `Allows`
	// would then treat as unscoped, which is true but arrived at by accident.
	if _, err := legacy.ScopedTo("   "); err == nil {
		t.Error("a workflow was bound to no application at all")
	}
}

// AND AN APPLICATION THAT CANNOT BE IDENTIFIED IS ITS OWN REFUSAL.
//
// Three ways for this to be no, and they send a reader to three different places: the file has
// no scope, the desktop has no answer, or the two disagree.
func TestAnUnidentifiedApplicationIsRefusedSeparately(t *testing.T) {
	a := anArkWorkflow()
	ok, why := a.ScopeAllows("")
	if ok {
		t.Fatal("a scoped workflow was permitted against an unidentified application")
	}
	if !strings.Contains(why, "could not be identified") {
		t.Errorf("the refusal reads as a mismatch rather than as an absence: %s", why)
	}
	// AND THE THREE SENTENCES DIFFER, so a person is not sent to the wrong place.
	_, unscoped := workflowstore.Artifact{Name: "x"}.ScopeAllows("solasta")
	_, mismatch := a.ScopeAllows("solasta")
	if unscoped == why || mismatch == why || unscoped == mismatch {
		t.Errorf("two of the three refusals are the same sentence:\n  %s\n  %s\n  %s",
			why, unscoped, mismatch)
	}
}
