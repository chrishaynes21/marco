package theaterhost_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// One answer to "activate this control", and no way around the compile gate.
//
// # What changed, and why these tests moved
//
// The ladder used to live inside the Actor, which called a host directly. It now lives at the
// Production boundary: the Actor writes legal Marco for one way of pressing, and the Theater runs
// it through an injected MarcoRunner.
//
// That is the invariant — every real input passes through Marco compilation, which
// `MarcoRunner.Run` guarantees returns a compile error "BEFORE any desktop mutation". An Actor
// that reached a host itself would be a second route: no compile gate, and nothing for a dry run
// to record. Two routes is exactly how the live rehearsal and the saved play drifted apart, with
// only one of them knowing that a Settings navigation item is a selection item.

// laddered answers each capability the way a particular control would, and records what was asked.
//
// A RUNNER, not a host. It reads which Accessibility capability the Actor's emitted Marco calls —
// the same thing the compiler and the real runtime do with it.
type laddered struct {
	unsupported map[string]bool
	broken      map[string]bool
	tried       []string
}

func (r *laddered) Run(_ context.Context, _, program string) (directorapi.MarcoResult, error) {
	capability := capabilityIn(program)
	r.tried = append(r.tried, capability)
	full := "Accessibility's " + capability
	switch {
	case r.unsupported[capability]:
		return directorapi.MarcoResult{
			Failed: []string{full},
			Returned: map[string]directorapi.MarcoValue{
				full: {Error: "unsupported: the control does not implement " +
					capability + "Pattern"},
			},
		}, nil
	case r.broken[capability]:
		return directorapi.MarcoResult{
			Failed: []string{full},
			Returned: map[string]directorapi.MarcoValue{
				full: {Error: "the bridge went away"},
			},
		}, nil
	}
	return directorapi.MarcoResult{}, nil
}

// capabilityIn reads which Accessibility capability an Actor's program calls.
func capabilityIn(program string) string {
	for _, name := range []string{"Invoke", "Select", "Expand", "Toggle"} {
		if strings.Contains(program, "do Accessibility's "+name+" with ctl.") {
			return name
		}
	}
	return ""
}

// laddering stages a production over a control that behaves as the runner describes.
func laddering(r *laddered) *theaterhost.Theater {
	return staged(r, theaterhost.NewAccessibilityActor(&recordingHost{}, "uia.exe"))
}

// The production falls back to the pattern the control implements.
//
// Deleting the ladder must fail this, and every Settings navigation step goes back to failing on
// a control that implements no InvokePattern.
func TestTheActorFallsBackToThePatternTheControlImplements(t *testing.T) {
	r := &laddered{unsupported: map[string]bool{"Invoke": true}}
	if got := laddering(r).Activate(context.Background(), mouse()); got.Refused != "" {
		t.Fatalf("a selection item could not be activated: %+v", got)
	}
	if len(r.tried) != 2 || r.tried[0] != "Invoke" || r.tried[1] != "Select" {
		t.Errorf("the production tried %v, want Invoke then Select", r.tried)
	}
}

// An implemented pattern that FAILS stops the production.
//
// Pressing a control repeatedly in different ways until something happens is what neither caller
// may ever do.
func TestTheActorDoesNotRetryARealFailure(t *testing.T) {
	r := &laddered{broken: map[string]bool{"Invoke": true}}
	if got := laddering(r).Activate(context.Background(), mouse()); got.Refused == "" {
		t.Fatal("a broken invoke reported success")
	}
	if len(r.tried) != 1 {
		t.Errorf("tried %d more way(s) after a real failure: %v", len(r.tried)-1, r.tried)
	}
}

// An ordinary button costs exactly one round trip.
func TestTheActorOnlyInvokesAnInvokableControl(t *testing.T) {
	r := &laddered{}
	if got := laddering(r).Activate(context.Background(), mouse()); got.Refused != "" {
		t.Fatalf("activation refused: %+v", got)
	}
	if len(r.tried) != 1 || r.tried[0] != "Invoke" {
		t.Errorf("a plain button was pressed %d way(s): %v", len(r.tried), r.tried)
	}
}

// Both callers ask in the same order.
func TestBothPathsShareOneLadder(t *testing.T) {
	r := &laddered{unsupported: map[string]bool{
		"Invoke": true, "Select": true, "Expand": true, "Toggle": true,
	}}
	if got := laddering(r).Activate(context.Background(), mouse()); got.Refused == "" {
		t.Fatal("a control implementing nothing reported success")
	}
	if len(r.tried) != len(activate.Ladder) {
		t.Fatalf("tried %d way(s), the canonical ladder has %d: %v",
			len(r.tried), len(activate.Ladder), r.tried)
	}
	for i, w := range activate.Ladder {
		if r.tried[i] != string(w) {
			t.Errorf("attempt %d asked for %q, the ladder says %q", i+1, r.tried[i], w)
		}
	}
}

// ── the compile gate, and no way around it ────────────────────────────────────

// An Actor never reaches a host directly for a production action.
//
// Reads are allowed — an Actor may look however it likes. ACTING is not: it must be expressed as
// Marco and run through the injected runner, or there is no compile gate and nothing for a dry
// run to record.
//
// Deleting the runner — performing inside the Actor — must fail this.
func TestAnActorNeverReachesAHostDirectly(t *testing.T) {
	host := &recordingHost{}
	r := &laddered{}
	th := staged(r, theaterhost.NewAccessibilityActor(host, "uia.exe"))

	if got := th.Activate(context.Background(), mouse()); got.Refused != "" {
		t.Fatalf("activation refused: %+v", got)
	}
	if len(r.tried) != 1 {
		t.Fatalf("the runner ran %d program(s); the production did not go through Marco",
			len(r.tried))
	}
	for i, c := range host.calls {
		if !isRead(c.Action) {
			t.Errorf("bridge call %d was %q. An Actor calling a host to ACT is a second "+
				"route to an effect: no compile gate, and nothing a dry run records.",
				i+1, c.Action)
		}
	}
}

// A production with no runner performs nothing.
//
// Fail-closed. A Theater that could act without a runner would be one wiring mistake away from
// having no compile gate at all.
func TestAProductionWithoutARunnerPerformsNothing(t *testing.T) {
	host := &recordingHost{}
	got := theaterhost.New(
		theaterhost.NewAccessibilityActor(host, "uia.exe")).Activate(context.Background(), mouse())

	if got.Performed {
		t.Fatal("a Theater with no Marco runner performed a production")
	}
	for i, c := range host.calls {
		if !isRead(c.Action) {
			t.Errorf("bridge call %d was %q with no runner wired", i+1, c.Action)
		}
	}
}

// A cast program terminates at the concrete act and never names the Theater.
//
// Half of the recursion guarantee. A cast program that could call `Theater's Activate` would
// re-enter the production it is part of — unbounded, every level authorised by the one above. The
// other half is structural: the runner a Theater is given excludes the Theater act, which
// cmd/marco holds.
func TestACastProgramNamesOnlyTheConcreteAct(t *testing.T) {
	actor := theaterhost.NewAccessibilityActor(&recordingHost{}, "uia.exe")
	for _, w := range activate.Ladder {
		program, ok := actor.Cast(theaterhost.Candidate{Handle: "Mouse"}, w)
		if !ok {
			t.Fatalf("the actor cannot express %s", w)
		}
		if strings.Contains(program, "Theater") {
			t.Errorf("the cast program for %s names the Theater:\n%s", w, program)
		}
		if !strings.Contains(program, "use accessibility.") {
			t.Errorf("the cast program for %s does not use the accessibility act:\n%s",
				w, program)
		}
		if !strings.Contains(program, "do Accessibility's "+string(w)+" with ctl.") {
			t.Errorf("the cast program for %s does not call it:\n%s", w, program)
		}
	}
}

// A name that cannot be written as a Marco literal is inexpressible, not escaped.
//
// The one place user text becomes program text in this Actor. A quote or a backslash must not be
// able to change what the program says.
func TestANameThatCannotBeWrittenIsRefused(t *testing.T) {
	actor := theaterhost.NewAccessibilityActor(&recordingHost{}, "uia.exe")
	bad := []string{"Mouse\"", "Mouse\\", "Mouse\nSettings", ""}
	for _, name := range bad {
		if _, ok := actor.Cast(theaterhost.Candidate{Handle: name}, activate.Invoke); ok {
			t.Errorf("%q was written into a program", name)
		}
	}
}

// A program the compiler rejects stops the production, and no other way is tried.
//
// # Why this is the whole reason the runner is here
//
// `MarcoRunner.Run` compiles before it executes and returns the error "BEFORE any desktop
// mutation". That is what makes attempting an unsupported activation safe at all: the ladder can
// try a way the control might not implement, because a way that cannot even be expressed never
// reaches a machine.
//
// A compile error is NOT an unsupported control. It is Marco saying the sentence is wrong, and
// trying three more sentences would be pressing a control repeatedly in different ways until
// something happened.
func TestACompileFailureStopsTheProductionAtOnce(t *testing.T) {
	r := &rejects{}
	got := staged(r, theaterhost.NewAccessibilityActor(&recordingHost{}, "uia.exe")).
		Activate(context.Background(), mouse())

	if got.Performed {
		t.Fatal("a program the compiler rejected was reported as performed")
	}
	if got.Refused != theaterhost.PerformFailed {
		t.Errorf("refused with %q", got.Refused)
	}
	if !strings.Contains(got.Detail, "no capability") {
		t.Errorf("the refusal lost the compiler's sentence: %q", got.Detail)
	}
	if r.runs != 1 {
		t.Errorf("%d program(s) were compiled after the first was rejected", r.runs)
	}
}

// rejects is a runner whose compiler refuses everything.
type rejects struct{ runs int }

func (r *rejects) Run(context.Context, string, string) (directorapi.MarcoResult, error) {
	r.runs++
	return directorapi.MarcoResult{}, errors.New(`Accessibility has no capability "Invoke"`)
}

// isRead names the bridge calls that ASK the provider something and change nothing.
//
// The invariant these tests hold is "an Actor never reaches a host to ACT" — because acting
// outside the cast program would be a second route to an effect, with no compile gate and nothing
// for a dry run to record. It is NOT "an Actor only ever calls Snapshot", which is what they
// happened to say while Snapshot was the only read there was.
//
// `Available` is the second. It asks the provider whether it can act at all, which is the question
// the roster and the refusal `no_actor_available` are both built on, and it changes nothing.
//
// Keeping this an ALLOW-LIST rather than a deny-list is the point: a new action added to the Actor
// fails these tests until somebody says out loud which kind it is.
func isRead(action string) bool {
	switch action {
	case "Snapshot", "Available":
		return true
	}
	return false
}
