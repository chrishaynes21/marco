package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aForgettableRoutine is a routine kept the way a recording is, with its demonstration, an edited
// step account and a shortcut, in a library of the test's own.
func aForgettableRoutine(t *testing.T) (*Acts, routes.Route) {
	t.Helper()
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_OVERLAY_CONFIG", t.TempDir()+"/overlay.json")
	t.Setenv("MARCO_OVERLAY_LEADER", "")
	a := newActs(t.TempDir())
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	steps := []macroir.Step{{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "{{resource}}"}, {Kind: macroir.StepWait, Ms: 250}}
	rt, err := d.Save(&orchestrator.Draft{Name: "cloud pull", Args: []string{"resource"}, Steps: steps,
		App: "notepad", Mode: orchestrator.Unstated}, orchestrator.ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rec, _ := json.Marshal([]recorder.RecordedEvent{{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: t0},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: t0.Add(30 * time.Millisecond), App: "notepad"}})
	if err := d.Reg.SaveRecording(rt, rec); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(d.Reg.Path(rt))
	raw, _ := json.Marshal(steps)
	if err := d.Reg.SaveSteps(rt, routes.StoredSteps{Source: routes.DigestOf(string(src)),
		Demonstration: routes.DigestOf(string(rec)), Steps: raw}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AssignShortcut(orchestrator.Assignment{Key: "c", ListenIn: "notepad",
		Steps: []routes.Route{rt}}); err != nil {
		t.Fatal(err)
	}
	return a, rt
}

// The confirmation names every part, and forgetting it removes them all and says so part by part.
//
// Forgetting through anything but ForgetAsPlanned must fail this.
func TestForgettingOneRoutineIsWhatWasShown(t *testing.T) {
	a, rt := aForgettableRoutine(t)
	v := a.ReviewForgetRoutine(rt.ID())
	if v.Why != "" || v.Token == "" || v.Where != "from anywhere → notepad" {
		t.Fatalf("the review: %+v", v)
	}
	joined := strings.Join(v.Parts, "\n")
	for _, want := range []string{"the shortcut `c in notepad", "the routine itself (" + rt.ID() + ")",
		"the demonstration it was made from", "the steps as you edited them"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the review does not name %q:\n%s", want, joined)
		}
	}
	if !strings.HasPrefix(v.Parts[0], "the shortcut") || !strings.HasPrefix(v.Parts[len(v.Parts)-1], "the routine itself") {
		t.Errorf("the parts are not in the order they go (shortcuts first, the routine last): %q", v.Parts)
	}
	done := a.ForgetRoutine(v.Token)
	if !done.Removed || done.Why != "" || !strings.HasPrefix(done.Says, `Forgot "cloud pull"`) {
		t.Fatalf("the forget: %+v", done)
	}
	for _, p := range done.Parts {
		if !p.Gone {
			t.Errorf("%s is still there", p.What)
		}
	}
	reg := a.reg()
	if reg.Has(rt) || reg.HasRecording(rt) || reg.HasSteps(rt) || len(reg.Bindings()) != 0 {
		t.Error("a part is still on disk")
	}
	if again := a.ForgetRoutine(v.Token); again.Why == "" {
		t.Error("a used review forgot again")
	}
}

// Something changed after the confirmation was read: nothing is removed, and it says so.
func TestAForgetOverAChangeRemovesNothing(t *testing.T) {
	a, rt := aForgettableRoutine(t)
	v := a.ReviewForgetRoutine(rt.ID())
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if _, err := d.AssignShortcut(orchestrator.Assignment{Key: "x", ListenIn: "",
		Steps: []routes.Route{rt}}); err != nil {
		t.Fatal(err)
	}
	done := a.ForgetRoutine(v.Token)
	if done.Removed || !strings.HasPrefix(done.Says, "Nothing was removed") {
		t.Errorf("a forget over a shortcut it did not show: %+v", done)
	}
	if !a.reg().Has(rt) || len(a.reg().Bindings()) != 2 {
		t.Error("something was removed")
	}
}
