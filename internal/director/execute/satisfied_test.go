package execute

// "APPS IS ALREADY OPEN. NO ACTION NEEDED."
//
// Measured on a live Settings window during Alpha 1: asked to open Apps while the Apps page
// was already open, Marco found two operable controls labelled "Apps" — a navigation member
// and a page-header button — and asked which one was meant. Every answer to that question was
// wrong in the same way, because pressing either changes nothing, and the question itself was
// the wrong thing to say.
//
// What is held here is not only that Marco says so. It is that it says so WITHOUT PERFORMING
// INPUT and without inventing a success: a no-op outcome rests on the application's own
// selection report, nothing is planned, no policy verdict is taken, no step is recorded, and
// the actuator is never touched.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// seenOnly marks an observation as coming from a VISUAL detector rather than from structure —
// a region that LOOKS selected. Enough to describe a Prop, and never enough to tell somebody
// their request is already done.
func seenOnly(o directorapi.Observation) directorapi.Observation {
	o.Source = directorapi.SourceVision
	o.ID = directorapi.ObservationID("vis:" + string(o.ID))
	return o
}

// settingsPage is the measured shape: a navigation list whose current member is reported
// selected, and a page-header button carrying the same word.
func settingsPage(current directorapi.Observation) []directorapi.Observation {
	return []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Settings", rect(0, 0, 1200, 900)),
		obs("uia:2", directorapi.RoleListItem, "System", rect(16, 100, 280, 36)),
		current,
		obs("uia:4", directorapi.RoleListItem, "Accounts", rect(16, 172, 280, 36)),
		obs("uia:5", directorapi.RoleButton, "Apps", rect(400, 60, 180, 28)),
		obs("uia:6", directorapi.RoleButton, "Installed apps", rect(400, 120, 300, 48)),
	}
}

func settingsWindow() []directorapi.Window {
	return []directorapi.Window{{
		ID: "hwnd:1", Application: "applicationframehost", Title: "Settings",
		Bounds: rect(0, 0, 1200, 900), Focused: true, Visible: true, MonitorID: "monitor:1",
	}}
}

func TestARequestForAPageThatIsOpenPerformsNothingAndSaysSo(t *testing.T) {
	apps := selectedObs(obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36)))
	h := newHarness(scene(t0, settingsWindow(), settingsPage(apps)...))

	out := h.pipeline.Handle(context.Background(), "open Apps")

	if out.Status != directorapi.ResultNothingToDo {
		t.Fatalf("status %s — %s", out.Status, out.Message)
	}
	if !strings.Contains(out.Message, "Apps") ||
		!strings.Contains(out.Message, "nothing needs pressing") {
		t.Errorf("the message does not say what is already so: %q", out.Message)
	}
	// NOTHING WAS PERFORMED. The point of the whole thing.
	if len(h.actuator.clicks) > 0 || len(h.operations.ops) > 0 {
		t.Errorf("a satisfied request still touched the desktop: %d clicks, %d operations",
			len(h.actuator.clicks), len(h.operations.ops))
	}
	if len(h.focuser.targets) > 0 {
		t.Errorf("a satisfied request still moved focus: %v", h.focuser.targets)
	}
	// AND NOTHING WAS INVENTED EITHER. No plan, no policy verdict, no step record — a
	// request that performed nothing must not be accounted for as one that performed
	// something.
	if out.Plan != nil {
		t.Errorf("a plan was built for a request that needed no action: %+v", out.Plan)
	}
	if out.Policy != nil {
		t.Errorf("a policy verdict was taken for an action nobody proposed: %+v", out.Policy)
	}
	if out.Record != nil {
		t.Errorf("a step record exists for a request that performed nothing: %+v", out.Record)
	}
	// AND THE EVIDENCE IS IN THE ACCOUNT, because "nothing needed doing" without a
	// receipt is indistinguishable from "I did not try".
	found := false
	for _, s := range out.Stages {
		if strings.Contains(s.Detail, "already") &&
			strings.Contains(s.Detail, string(directorapi.SourceAccessibility)) {
			found = true
		}
	}
	if !found {
		t.Errorf("the account does not say which sensor reported the state: %+v", out.Stages)
	}
}

// AND THE SAME WINDOW WITH NOTHING SELECTED ASKS, as it always did.
//
// The other half of the contract. Two operable controls called Apps and no report of which is
// current is a genuine question, and this must not have turned every ambiguity into a quiet
// "already done".
func TestARequestForAPageThatIsNotOpenIsHandledAsBefore(t *testing.T) {
	apps := obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36))
	h := newHarness(scene(t0, settingsWindow(), settingsPage(apps)...))

	out := h.pipeline.Handle(context.Background(), "open Apps")

	if out.Status == directorapi.ResultNothingToDo {
		t.Fatalf("a page nothing reported as current was called already open: %q",
			out.Message)
	}
	if out.Status != directorapi.ResultNeedsClarification &&
		out.Status != directorapi.ResultDone &&
		out.Status != directorapi.ResultNeedsConfirmation {
		t.Errorf("unexpected status %s — %s", out.Status, out.Message)
	}
}

// AND A REGION THAT MERELY LOOKS SELECTED IS NOT EVIDENCE OF ANYTHING BEING DONE.
//
// A no-op success is a claim that the request is already satisfied. Appearance is good enough
// to say "this looks like the current tab" and not good enough to tell somebody their request
// is complete — so a `Selected` that only a visual detector filled must not produce one. The
// same rule ADR-202 applies to operability, applied to accounting.
func TestAppearanceAloneCannotReportARequestAsAlreadyDone(t *testing.T) {
	apps := seenOnly(selectedObs(
		obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36))))
	// The same Prop, also reported by accessibility but WITHOUT a selection claim, so
	// fusion has a structural element for appearance to fill the state of.
	structural := obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36))
	structural.Selected = nil

	page := append(settingsPage(structural), apps)
	h := newHarness(scene(t0, settingsWindow(), page...))

	out := h.pipeline.Handle(context.Background(), "open Apps")
	if out.Status == directorapi.ResultNothingToDo {
		t.Errorf("a request was reported as already done on the strength of what a "+
			"region looks like: %q", out.Message)
	}
}

// AND A PROCEDURE CONTINUES PAST A STEP THAT NEEDED NOTHING.
//
// Measured live, and it is the same defect class as the menu one step over. "Rename this file
// to summary.txt" with the file ALREADY SELECTED: step one selected it again, nothing changed,
// and verification reported
//
//	! verify inconclusive: "notes.txt" reports itself selected and nothing else
//	  changed to confirm it
//	UNVERIFIED: step 1 of 4 could not be verified
//
// A four-step rename abandoned at step one because step one had nothing to do. The step was not
// unverifiable; it was unnecessary, and those are different sentences with opposite
// consequences.
func TestAProcedureContinuesPastAStepThatNeededNothing(t *testing.T) {
	// "Accounts" is the current page and nothing else carries that name, so the step
	// resolves cleanly and needs nothing — which must not end the procedure.
	page := settingsPage(obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36)))
	for i := range page {
		if page[i].Label == "Accounts" {
			page[i] = selectedObs(page[i])
		}
	}
	// Distinct moments, because verification refuses to compare a snapshot with itself.
	at := func(n int) directorapi.WorldState {
		return scene(t0.Add(time.Duration(n)*time.Second), settingsWindow(), page...)
	}
	h := newHarness(at(0), at(1), at(2), at(3), at(4), at(5))

	in := h.pipeline.parseWith(context.Background())("select Accounts")
	var bare program.Context
	bare.EnsureValues()
	bare.EnsureCollections()
	step := h.pipeline.runStep(ensureBindings(context.Background()), "select Accounts", in, bare)

	// The STEP says nothing needed doing, truthfully, and performs nothing.
	if step.Status != directorapi.ResultNothingToDo {
		t.Fatalf("status %s — %s", step.Status, step.Message)
	}
	if step.Record != nil {
		t.Errorf("a record was written for a step that performed nothing: %+v", step.Record)
	}
	if len(h.actuator.clicks) > 0 || len(h.operations.ops) > 0 {
		t.Errorf("a step that needed nothing still touched the desktop")
	}

	// AND THE PROGRAM CARRIES ON, which is the half the live run lost. Counted as
	// completed, because the state the next step needs is the state that was already
	// there.
	prog := program.Program{Goal: "two steps", Steps: []program.Step{
		{ID: "s1", Phrase: "select Accounts", Operation: in, FailurePolicy: program.Stop},
		{ID: "s2", Phrase: "select System", FailurePolicy: program.Stop,
			Operation: h.pipeline.parseWith(context.Background())("select System")},
	}}
	var runCtx program.Context
	runCtx.EnsureValues()
	runCtx.EnsureCollections()
	out := h.pipeline.RunProgram(ensureBindings(context.Background()), prog, runCtx, 0)
	// PAST STEP ONE is the whole claim. Step two then does whatever this scripted world
	// lets it do — the fake actuator changes nothing, so it cannot verify a selection —
	// and asserting its outcome would be asserting the harness rather than the loop.
	if out.StoppedAt == 1 {
		t.Fatalf("the procedure stopped at the step that needed nothing: %s", out.Message)
	}
	if out.Completed < 1 {
		t.Errorf("the step that needed nothing was not counted: %s", out.Message)
	}
}
