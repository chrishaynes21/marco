package orchestrator

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The plan lists the shortcuts and the demonstration, and forgetting as planned removes them.
func TestAForgetAsPlannedRemovesWhatItShowed(t *testing.T) {
	d, rt := aRoutineAskingForAValue(t)
	if _, err := d.AssignShortcut(Assignment{Key: "c", ListenIn: "notepad", Steps: []routes.Route{rt}}); err != nil {
		t.Fatal(err)
	}
	p, err := d.PlanForget(rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Shortcuts) != 1 || p.Shortcuts[0].Key != "c" || !p.Demonstration || !p.Steps || p.Token == "" {
		t.Fatalf("the plan is %+v", p)
	}
	f, err := d.ForgetAsPlanned(p)
	if err != nil || !f.Removed || len(f.Shortcuts) != 1 {
		t.Fatalf("forgetting as planned: %+v, %v", f, err)
	}
	if d.Reg.Has(rt) || d.Reg.HasRecording(rt) || d.Reg.HasSteps(rt) {
		t.Error("a part of the routine is still there")
	}
}

// A shortcut assigned, or the routine saved again, after the plan was read: nothing is removed.
//
// Skipping the check must fail this.
func TestAForgetRemovesNothingItDidNotShow(t *testing.T) {
	d, rt := aRoutineAskingForAValue(t)
	p, err := d.PlanForget(rt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AssignShortcut(Assignment{Key: "c", ListenIn: "notepad", Steps: []routes.Route{rt}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ForgetAsPlanned(p); err == nil || !strings.Contains(err.Error(), "changed since you looked") {
		t.Fatalf("a forget over a shortcut it did not show: %v", err)
	}
	if !d.Reg.Has(rt) || len(d.Reg.Bindings()) != 1 {
		t.Error("something was removed")
	}
	if _, err := d.ForgetAsPlanned(ForgetPlan{Route: rt}); err == nil {
		t.Error("an unreviewed forget was performed")
	}
	_ = routes.Route{}
}
