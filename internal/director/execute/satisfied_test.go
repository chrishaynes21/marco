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

// AND A STEP OF A PROCEDURE WHOSE STATE IS ALREADY SET STILL RUNS.
//
// The other side of the one line that distinguishes a request from a step, and the case that
// found it. The rename procedure's first step SELECTS the file the person pointed at. In a
// window where that file is already selected, treating the step as satisfied ends the request
// before F2 is ever pressed — and "Alpha.txt is already the one you are on" is a true sentence
// about a rename that did not happen.
//
// A person's request has no next step. A procedure's step one has three.
func TestAProgramStepWhoseStateIsAlreadySetStillRuns(t *testing.T) {
	// "Accounts" is the current page and nothing else carries that name, so the step
	// resolves cleanly and is satisfied — which is exactly the case that must still run.
	page := settingsPage(obs("uia:3", directorapi.RoleListItem, "Apps", rect(16, 136, 280, 36)))
	for i := range page {
		if page[i].Label == "Accounts" {
			page[i] = selectedObs(page[i])
		}
	}
	w := scene(t0, settingsWindow(), page...)
	h := newHarness(w, w, w)

	in := h.pipeline.parseWith(context.Background())("select Accounts")
	// A BARE context, so what is being tested is `runStep`'s own contribution: it is the
	// thing that knows this is a step. Handed a context that already said so, this test
	// would pass with the knowledge deleted.
	var bare program.Context
	bare.EnsureValues()
	bare.EnsureCollections()
	out := h.pipeline.runStep(ensureBindings(context.Background()), "select Accounts", in, bare)

	if out.Status == directorapi.ResultNothingToDo {
		t.Fatalf("a procedure's step was ended as needing nothing: %q", out.Message)
	}
	if out.Record == nil {
		t.Fatalf("the step produced no record (%s: %s)", out.Status, out.Message)
	}
}
