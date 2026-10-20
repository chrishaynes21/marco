package demonstrated_test

// SAVING A PLAY WHERE PLAYS LIVE, AND NEVER OVER ONE SOMEBODY HAS.
//
// The Architect's storage disposition: reuse route storage, with collision checks before writing
// or registration, and never overwrite or silently shadow an existing play. A name already taken
// is refused here and Studio asks the person for another, because replacing a play somebody has
// is a decision only they can make.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// compiles is a checker a store test can use: every play it is given is legal.
//
// The real one is `driver.CheckSource`, wired in `cmd/director` — `internal/director` may not
// import the engine, which is the whole reason `Compiles` is an interface. What is under test
// here is the three writes and the collision rule, and a real compile would only slow it down.
type compiles struct{}

func (compiles) Check(string) error { return nil }

// aRoutine is one saved demonstration with two interactions, enough to compose from.
func aRoutine() workflowstore.Artifact {
	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{{X: 0.41, Y: y, Width: 0.07, Height: 0.03}}}
	}
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics live",
		SavedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{{Label: "screen_a"}, {Label: "screen_b"}},
		Controls: []workflowstore.Control{
			control("control_1", 0.24), control("control_2", 0.29)},
		Steps: []workflowstore.Step{
			{Position: 0, From: "screen_a", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"bloom"}},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_2",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"master"}},
		},
	}
}

func composed(t *testing.T) (string, demonstrated.Bindings) {
	t.Helper()
	src, b, err := demonstrated.Compose(aRoutine(), "an act", []demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	}, time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC), compiles{})
	if err != nil {
		t.Fatal(err)
	}
	return src, b
}

func aRoute() routes.Route { return routes.Route{Slug: "ark-graphics-live"} }

func TestASavedPlayAndItsBindingsComeBackTogether(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	src, b := composed(t)
	if err := demonstrated.Save(reg, aRoute(), src, b, time.Now()); err != nil {
		t.Fatalf("saving: %v", err)
	}
	back, binds, err := demonstrated.Load(reg, aRoute())
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if back != src {
		t.Fatal("the play that came back is not the play that went in")
	}
	if len(binds.Scenes) != 2 {
		t.Fatalf("the bindings came back with %d interaction(s)", len(binds.Scenes))
	}
	// AND IT PREFLIGHTS, which is the only thing that makes a pair runnable.
	plan, err := demonstrated.Preflight(back, binds,
		aRoutines{aRoutine().Name: aRoutine()}, compiles{})
	if err != nil {
		t.Fatalf("the reloaded pair does not preflight: %v", err)
	}
	if plan.Steps["the graphics tab"] != 0 || plan.Steps["apply changes"] != 1 {
		t.Fatalf("the names bind to %+v", plan.Steps)
	}
	// AND THE PROVENANCE SAYS WHAT IT IS, rather than borrowing `learned`, which means
	// rehearsed and verified — and nothing here has been performed yet.
	o, _ := reg.Origin(aRoute())
	if o.Kind != routes.KindDemonstrated {
		t.Fatalf("the provenance says %q", o.Kind)
	}
}

// NOTHING IS OVERWRITTEN.
func TestSavingOverAPlaySomebodyHasIsRefused(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	src, b := composed(t)
	if err := reg.Save(aRoute(), "// somebody's own play\n"); err != nil {
		t.Fatal(err)
	}
	err := demonstrated.Save(reg, aRoute(), src, b, time.Now())
	if err == nil {
		t.Fatal("a play that was already there was replaced")
	}
	if !strings.Contains(err.Error(), "already a play") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	// AND THEIRS IS STILL THERE, UNTOUCHED.
	raw, rerr := os.ReadFile(reg.Path(aRoute()))
	if rerr != nil || !strings.Contains(string(raw), "somebody's own play") {
		t.Fatalf("their play was changed: %v %q", rerr, string(raw))
	}
	if _, serr := os.Stat(demonstrated.BindingsPath(reg, aRoute())); serr == nil {
		t.Fatal("bindings were written beside a play they do not describe")
	}
}

// ORPHANED BINDINGS ARE REFUSED RATHER THAN REUSED.
//
// Left by a crash between two writes, or by a delete that took the source and not the companion.
// They describe sentences the new play may not have.
func TestSavingOverOrphanedBindingsIsRefused(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	src, b := composed(t)
	if err := os.MkdirAll(reg.ScopeDir(aRoute()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(demonstrated.BindingsPath(reg, aRoute()),
		[]byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := demonstrated.Save(reg, aRoute(), src, b, time.Now())
	if err == nil {
		t.Fatal("a play was saved over bindings describing something else")
	}
	if !strings.Contains(err.Error(), "with no play") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// A PAIR ASSEMBLED FROM TWO SOURCES IS REFUSED.
func TestSavingBindingsForADifferentPlayIsRefused(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	src, b := composed(t)
	err := demonstrated.Save(reg, aRoute(), src+"\n// edited\n", b, time.Now())
	if err == nil {
		t.Fatal("a play was saved with bindings written for a different one")
	}
	if !strings.Contains(err.Error(), "different play") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// A PLAY WITH NO BINDINGS IS NOT RUNNABLE HERE.
//
// Which is the whole reason the bindings are not kept in the provenance sidecar: `routes.Origin`
// is allowed to be absent, because a play with no provenance is an ordinary authored play and
// claims nothing. A play with no bindings would claim to be an ordinary play too — and this
// bridge would have nothing to say about which control each of its sentences presses.
func TestAPlayWithNoBindingsWillNotLoad(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	src, _ := composed(t)
	if err := reg.Save(aRoute(), src); err != nil {
		t.Fatal(err)
	}
	_, _, err := demonstrated.Load(reg, aRoute())
	if err == nil {
		t.Fatal("a play with no bindings loaded as though it were runnable")
	}
	if !strings.Contains(err.Error(), "no bindings") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// aRoutines is where a saved demonstration is found, for the preflight above.
type aRoutines map[string]workflowstore.Artifact

func (s aRoutines) Sequence(name string) (workflowstore.Artifact, bool) {
	a, ok := s[name]
	return a, ok
}
