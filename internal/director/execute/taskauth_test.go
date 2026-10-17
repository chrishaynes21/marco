package execute

// A REQUEST AUTHORIZES THE TASK IT ASKED FOR.
//
// Measured live. Three completed workflows, and every one stopped partway through to ask
// whether it might press Enter:
//
//	make a new folder called Reports
//	  [4/4] plan    confirm "Reports" (high risk)
//	        confirm the confirmation could not be put to you
//	  BLOCKED: step 4 of 4 did not complete
//
// The person asked for a folder. Marco opened the New menu, chose Folder and typed the name
// without asking, then stopped at the keystroke that commits it — because `confirm` is
// high-risk by its verb, and which irreversible thing a confirm commits is not knowable from
// the verb. It is knowable from the TASK.

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// askedFor is the licence a request mints for one declared step of a routine procedure.
func askedFor(step string) *directorapi.TaskLicence {
	return &directorapi.TaskLicence{
		Request: "make a new folder called Reports", Goal: "create_folder",
		Procedure: "explorer create folder", Step: step, Application: "explorer",
		Declared: directorapi.TaskEffects{Risk: directorapi.RiskLow, Mutations: 1},
	}
}

// committing is the step the live runs stopped at: a high-risk verb on an ordinary control.
func committing(lic *directorapi.TaskLicence) actionConfirmation {
	return actionConfirmation{
		Action: `confirm "Reports"`, Risk: directorapi.RiskHigh,
		Task: lic, Application: "explorer",
	}
}

func TestARequestAuthorizesTheStepsOfTheTaskItAskedFor(t *testing.T) {
	got := coveredByTheRequest(committing(askedFor("confirm the name")))
	if !got.Covered {
		t.Fatalf("the step the person asked for was not authorized: %s", got.Reason)
	}
	// AND THE ACCOUNT QUOTES THEM, because "we did not ask" needs a reason a person can
	// recognise as their own.
	if !contains(got.Reason, "make a new folder called Reports") {
		t.Errorf("the reason does not quote the request: %q", got.Reason)
	}
	if !contains(got.Reason, "confirm the name") {
		t.Errorf("the reason does not name the step: %q", got.Reason)
	}
}

// AND NOTHING WITHOUT A TASK IS AUTHORIZED BY ONE.
//
// Four absences, each of which makes a step something nobody declared: no licence at all, no
// request, no procedure, and no step phrase. The last is the important one — the licence is
// minted per step, so a step reached by a path that does not fill it in cannot be covered by a
// licence lying on the context.
func TestAStepNobodyDeclaredIsNotAuthorized(t *testing.T) {
	for name, lic := range map[string]*directorapi.TaskLicence{
		"no licence":   nil,
		"no request":   {Procedure: "explorer create folder", Step: "confirm the name"},
		"no procedure": {Request: "make a folder", Step: "confirm the name"},
		"no step":      {Request: "make a folder", Procedure: "explorer create folder"},
	} {
		if got := coveredByTheRequest(committing(lic)); got.Covered {
			t.Errorf("%s: authorized anyway — %s", name, got.Reason)
		}
	}
}

// AND A CONFIRMATION THE PLANNER ASKED FOR IS NEVER WAIVED.
//
// Risk has two sources: the verb, and the control it is aimed at. "Confirm" on a field holding
// a folder name and "confirm" on a button labelled "Delete account" are the same verb. The
// planner raises the second and sets Plan.RequiresConfirmation, whose doc has always said
// policy may add one and never remove one — and this is the first thing to rely on it.
func TestAConfirmationAboutTheControlItselfIsNeverWaived(t *testing.T) {
	act := committing(askedFor("confirm the name"))
	act.PlannerAsked = true
	got := coveredByTheRequest(act)
	if got.Covered {
		t.Fatalf("a question about the control was waived by the task: %s", got.Reason)
	}
	if !contains(got.Reason, "control") {
		t.Errorf("the refusal does not say what made it one: %q", got.Reason)
	}
}

// AND A CONSEQUENTIAL PROCEDURE IS COVERED ONLY ONCE ITS OWN QUESTION IS ANSWERED.
func TestAConsequentialProcedureNeedsItsGoalQuestionAnswered(t *testing.T) {
	for _, declared := range []directorapi.TaskEffects{
		{Risk: directorapi.RiskHigh},
		{Risk: directorapi.RiskLow, Destructive: true},
		{Risk: directorapi.RiskLow, Irreversible: true},
		{Risk: directorapi.RiskLow, External: true},
	} {
		lic := askedFor("confirm the name")
		lic.Declared = declared
		if got := coveredByTheRequest(committing(lic)); got.Covered {
			t.Errorf("%+v was authorized without being agreed to: %s", declared, got.Reason)
		}
		lic.Confirmed = true
		if got := coveredByTheRequest(committing(lic)); !got.Covered {
			t.Errorf("%+v was agreed to and still asked per step: %s", declared, got.Reason)
		}
	}
}

// AND AN AUTHORIZATION CANNOT TRAVEL TO ANOTHER APPLICATION, or survive the target changing
// materially under it.
func TestAnAuthorizationDoesNotTravel(t *testing.T) {
	act := committing(askedFor("confirm the name"))
	act.Application = "notepad"
	if got := coveredByTheRequest(act); got.Covered {
		t.Errorf("a task authorized against Explorer covered a step in Notepad: %s",
			got.Reason)
	}
	act = committing(askedFor("confirm the name"))
	act.MaterialChange = "it now points at Bravo.txt"
	if got := coveredByTheRequest(act); got.Covered {
		t.Errorf("the target changed and the authorization survived: %s", got.Reason)
	}
}

// AND THE GATE ACTUALLY CONSULTS IT, through the real pipeline.
//
// Everything above tests the RULE. The mutation gate then found what nothing tested: deleting
// the call from `confirmAction`, or the line in `runStep` that stamps the step phrase, changed
// no test at all. This repository has eight recorded cases of complete code production never
// being invoked, and a rule nothing reaches is the ninth.
//
// So this enters through the production path, with a confirmer that RECORDS every question. The
// step is `confirm`, which is high-risk by its verb and the one the live runs stopped at.
func TestTheGateAsksNothingForAStepTheRequestAuthorized(t *testing.T) {
	asked := &recordingConfirmer{answer: true}

	// The same step, twice: once with the request's authorization and once without.
	run := func(withTask bool) (ProgramOutcome, int) {
		asked.asked = 0
		h := newHarness(namedAfterwards("Reports")...)
		h.pipeline.Confirmer = asked

		in := h.pipeline.parseWith(context.Background())("confirm Name")
		var pctx program.Context
		pctx.EnsureValues()
		pctx.EnsureCollections()
		if withTask {
			pctx.Task = &directorapi.TaskLicence{
				Request: "make a new folder called Reports", Goal: "create_folder",
				Procedure: "explorer create folder",
				Declared:  directorapi.TaskEffects{Risk: directorapi.RiskLow, Mutations: 1},
			}
		}
		prog := program.Program{Goal: "one step", Steps: []program.Step{{
			ID: "s1", Phrase: "confirm the name", Operation: in,
			FailurePolicy: program.Stop,
		}}}
		return h.pipeline.RunProgram(ensureBindings(context.Background()), prog, pctx, 0),
			asked.asked
	}

	// WITHOUT a task, the high-risk verb is asked about — which is the behaviour every
	// live run showed and which must not change for a request nobody made.
	if _, questions := run(false); questions == 0 {
		t.Fatal("a high-risk step outside any task was performed without asking")
	}
	// WITH one, nothing is asked.
	out, questions := run(true)
	if questions != 0 {
		t.Errorf("%d question(s) asked about a step the request authorized: %+v",
			questions, asked.request)
	}
	if out.StoppedAt != 0 {
		t.Errorf("the step did not run: %s", out.Message)
	}
	// AND THE ACCOUNT SAYS WHY IT DID NOT ASK, rather than silently not asking.
	said := false
	for _, step := range out.Steps {
		for _, stage := range step.Stages {
			if stage.Name == "policy" && contains(stage.Detail, "authorized this") {
				said = true
			}
		}
	}
	if !said {
		t.Error("nothing in the account says the request authorized the step")
	}
}
