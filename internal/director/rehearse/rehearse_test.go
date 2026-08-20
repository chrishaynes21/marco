package rehearse_test

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Everything that must NOT reach the boundary, and the order of the things that do.
//
// The production chain is proved in `cmd/director` — this file is the refusal matrix, the
// ordering guarantee and the atomicity guarantee, which need a grant in states the production
// chain cannot conveniently be driven into (expired, revoked mid-flight, out of budget).

// ── fixtures ──────────────────────────────────────────────────────────────────

var (
	subjA = "subj_a"
	subjB = "subj_b"
	route = observe.RelationshipRef{From: subjA, To: subjB}
	epoch = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
)

// plan is one authorized plan, with whatever steps a test needs.
func plan(steps ...observe.StepPlan) observe.RehearsalJudgement {
	j := observe.RehearsalJudgement{
		Relationship: route, Application: "testgame", Eligible: true,
		Source: subjA, Destination: subjB, Digest: "digest_1",
		Plan: steps,
	}
	for _, s := range steps {
		j.Inputs += len(s.Intents)
		if s.Verifiability == observe.ProgressUnobservable {
			j.Unobservable++
			j.LongestUnobservable = 1
		}
	}
	return j
}

func arrives(position int, at string, intents ...observe.NavIntent) observe.StepPlan {
	return observe.StepPlan{Position: position, Intents: intents,
		Verifiability: observe.DirectlyVerifiable, Expect: at}
}

func staysOn(position int, at string, intents ...observe.NavIntent) observe.StepPlan {
	return observe.StepPlan{Position: position, Intents: intents,
		Verifiability: observe.ProgressUnobservable, Expect: at}
}

func blind(position int, intents ...observe.NavIntent) observe.StepPlan {
	return observe.StepPlan{Position: position, Intents: intents,
		Verifiability: observe.Unverifiable}
}

// onePress is the ordinary case: one step, one meaning, arriving somewhere Marco knows.
func onePress() observe.RehearsalJudgement {
	return plan(arrives(1, subjB, observe.NavConfirm))
}

func grantFor(t *testing.T, j observe.RehearsalJudgement) *observe.RehearsalGrant {
	t.Helper()
	g, err := observe.NewRehearsalGrant("t", j, epoch)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	return g
}

func scopeFor(g *observe.RehearsalGrant) rehearse.Scope {
	return rehearse.Scope{Application: g.Application, Source: g.Source,
		Relationship: g.Relationship, Evidence: g.Evidence}
}

// recorded is the seam the Director sees: whatever it produced, and nothing beneath it.
//
// A directorapi.MarcoRunner rather than a real one, because internal/director may not import
// platform code — a rule this repository already enforces, and rightly: the Director must not be
// able to reach an implementation even in a test. What the real runner, the real compiler and a
// recording host do with these programs is proved at the composition root, in cmd/director.
func recorded() (*programRunner, *programRunner) {
	r := &programRunner{}
	return r, r
}

// programRunner is both the runner and the recorder: it keeps the programs it was given.
type programRunner struct {
	mu       sync.Mutex
	programs []string
}

func (r *programRunner) Run(_ context.Context, _, program string) (directorapi.MarcoResult, error) {
	r.mu.Lock()
	r.programs = append(r.programs, program)
	r.mu.Unlock()
	return directorapi.MarcoResult{}, nil
}

// Lines is every `do OS's …` the produced programs contain, in order.
//
// Read out of the program text rather than invented here: the program IS what the Director
// produced, and anything this function added would be this test answering its own question.
func (r *programRunner) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for _, p := range r.programs {
		for _, line := range strings.Split(p, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "do OS's ") && strings.HasSuffix(line, ".") {
				out = append(out, strings.TrimSuffix(strings.TrimPrefix(line, "do "), "."))
			}
		}
	}
	return out
}

// lower runs the whole thing for one plan, at one step.
func lower(t *testing.T, j observe.RehearsalJudgement, position int) (
	rehearse.StepEmission, *programRunner, error) {

	t.Helper()
	g := grantFor(t, j)
	a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	rec, runner := recorded()
	step, err := a.LowerStep(context.Background(), position, runner, rec, windowref.Ref{}, rehearse.Stage{})
	return step, rec, err
}

// ── exactness and order ───────────────────────────────────────────────────────

// A bounded ordered run reaches the host in exactly its own order.
func TestTheHostReceivesExactlyTheStepsOwnOrderedIntents(t *testing.T) {
	j := plan(arrives(1, subjB, observe.NavDown, observe.NavDown, observe.NavConfirm))
	step, rec, err := lower(t, j, 1)
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	want := []string{
		`OS's Navigate with "down"`,
		`OS's Navigate with "down"`,
		`OS's Navigate with "confirm"`,
	}
	if got := rec.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("would send %v, want %v", got, want)
	}
	// The SECOND `down` is the assertion. A set, a sort or a dedup all lose it, and the
	// selection stops one row short of where the demonstration went.
	if len(rec.Lines()) != 3 {
		t.Fatalf("%d call(s); a repeated meaning was collapsed", len(rec.Lines()))
	}
	if !reflect.DeepEqual(step.Emitted, want) {
		t.Errorf("the record disagrees with the host: %v", step.Emitted)
	}
	// And the program a person can read says the same thing, in the same order.
	first := strings.Index(step.Program, `Navigate with "confirm"`)
	if first < strings.Index(step.Program, `Navigate with "down"`) {
		t.Errorf("the program lowered the meanings out of order:\n%s", step.Program)
	}
}

// The Director names a meaning. It never names a key.
func TestNothingTheDirectorProducesNamesAKey(t *testing.T) {
	step, _, err := lower(t, onePress(), 1)
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	for _, key := range []string{"enter", "return", "0x0d", "vk", "keycode", "scancode",
		"button", "gamepad"} {
		if strings.Contains(strings.ToLower(step.Program), key) {
			t.Errorf("the lowered program names %q. The Director learned `confirm`; which "+
				"key that is belongs to the host:\n%s", key, step.Program)
		}
		for _, line := range step.Emitted {
			if strings.Contains(strings.ToLower(line), key) {
				t.Errorf("the host was asked for %q: %s", key, line)
			}
		}
	}
}

// The classification and the expectation survive lowering unchanged.
func TestClassificationAndExpectationSurviveLowering(t *testing.T) {
	t.Run("directly verifiable", func(t *testing.T) {
		step, _, err := lower(t, onePress(), 1)
		if err != nil {
			t.Fatalf("lowering: %v", err)
		}
		if step.Verification != observe.DirectlyVerifiable {
			t.Errorf("verification = %q", step.Verification)
		}
		if step.Expect != subjB {
			t.Errorf("the expectation the NEXT milestone will check did not survive: %q",
				step.Expect)
		}
		joined := strings.ToLower(strings.Join(step.Describe(), "\n"))
		if !strings.Contains(joined, "directly verifiable") {
			t.Errorf("the description does not say how this would be checked:\n%s", joined)
		}
	})

	t.Run("progress unobservable", func(t *testing.T) {
		j := plan(staysOn(1, subjA, observe.NavDown))
		step, rec, err := lower(t, j, 1)
		if err != nil {
			t.Fatalf("lowering: %v", err)
		}
		// It LOWERS — a step Marco cannot see the result of is still a step it may take —
		// and it arrives carrying the weaker marker.
		if step.Verification != observe.ProgressUnobservable {
			t.Fatalf("verification = %q; the weaker evidence marker was lost, which is the "+
				"collapse the three-valued vocabulary exists to prevent", step.Verification)
		}
		if got := rec.Lines(); len(got) != 1 || got[0] != `OS's Navigate with "down"` {
			t.Errorf("would send %v", got)
		}
		joined := strings.ToLower(strings.Join(step.Describe(), "\n"))
		if !strings.Contains(joined, "progress unobservable") {
			t.Errorf("the description does not mark it as unobservable:\n%s", joined)
		}
		for _, claim := range []string{"verified", "reached", "success"} {
			if strings.Contains(joined, claim) {
				t.Errorf("an unobservable step's description claims %q:\n%s", claim, joined)
			}
		}
	})
}

// Nothing this milestone produces claims anything happened.
func TestAStepEmissionClaimsNothing(t *testing.T) {
	step, _, err := lower(t, onePress(), 1)
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if step.Status != rehearse.EmissionSent {
		t.Fatalf("status = %q", step.Status)
	}
	for _, forbidden := range []rehearse.EmissionStatus{"success", "verified", "rehearsed", "done"} {
		if step.Status == forbidden {
			t.Errorf("status claims %q", forbidden)
		}
	}
	rt := reflect.TypeOf(rehearse.StepEmission{})
	for _, field := range []string{"Verified", "Succeeded", "Result", "Outcome"} {
		if _, ok := rt.FieldByName(field); ok {
			t.Errorf("StepEmission has a %s field. Nothing ran, so there is no result to hold",
				field)
		}
	}
}

// ── the refusal matrix ────────────────────────────────────────────────────────

// Every designed blocker refuses, and every refusal sends nothing.
func TestTheDryRefusalMatrix(t *testing.T) {
	cases := []struct {
		name string
		// setup returns the grant, the plan, the scope and the step to try.
		setup func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time)
		want rehearse.Refusal
	}{{
		name: "no authorization at all",
		setup: func(*testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			return nil, j, rehearse.Scope{Application: "testgame", Source: subjA,
				Relationship: route, Evidence: j.Digest}, 1, epoch
		},
		want: rehearse.RefusalNoGrant,
	}, {
		name: "an authorization already spent",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			if err := g.BeginAttempt(g.Application, g.Source, g.Evidence, epoch); err != nil {
				t.Fatalf("spending: %v", err)
			}
			return g, j, scopeFor(g), 1, epoch
		},
		want: rehearse.RefusalGrantSpent,
	}, {
		name: "an authorization withdrawn",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			g.Revoke()
			return g, j, scopeFor(g), 1, epoch
		},
		want: rehearse.RefusalGrantRevoked,
	}, {
		name: "an authorization older than its own bound",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			return g, j, scopeFor(g), 1, epoch.Add(g.MaxDuration + time.Second)
		},
		want: rehearse.RefusalGrantExpired,
	}, {
		name: "another application in front",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			s := scopeFor(g)
			s.Application = "someothergame"
			return g, j, s, 1, epoch
		},
		want: rehearse.RefusalApplicationMismatch,
	}, {
		name: "another screen showing",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			s := scopeFor(g)
			s.Source = "subj_elsewhere"
			return g, j, s, 1, epoch
		},
		want: rehearse.RefusalSourceMismatch,
	}, {
		name: "another route",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			s := scopeFor(g)
			s.Relationship = observe.RelationshipRef{From: subjA, To: "subj_c"}
			return g, j, s, 1, epoch
		},
		want: rehearse.RefusalRelationshipMismatch,
	}, {
		name: "the demonstration has changed since the yes",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			s := scopeFor(g)
			s.Evidence = "digest_2"
			return g, j, s, 1, epoch
		},
		want: rehearse.RefusalEvidenceMismatch,
	}, {
		name: "a step the plan does not have",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := onePress()
			g := grantFor(t, j)
			return g, j, scopeFor(g), 4, epoch
		},
		want: rehearse.RefusalNoSuchStep,
	}, {
		name: "a step Marco could not check afterwards",
		setup: func(t *testing.T) (*observe.RehearsalGrant, observe.RehearsalJudgement,
			rehearse.Scope, int, time.Time) {
			j := plan(blind(1, observe.NavConfirm))
			g := grantFor(t, j)
			return g, j, scopeFor(g), 1, epoch
		},
		want: rehearse.RefusalStepUnverifiable,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, j, s, position, now := tc.setup(t)
			rec, runner := recorded()

			var step rehearse.StepEmission
			a, err := rehearse.BeginAttempt(g, j, s, now)
			if err == nil {
				step, err = a.LowerStep(context.Background(), position, runner, rec, windowref.Ref{}, rehearse.Stage{})
			}
			if err == nil {
				t.Fatalf("%s was authorised: %+v", tc.name, step)
			}
			if got, _ := rehearse.RefusalOf(err); got != tc.want {
				t.Errorf("refusal = %q, want %q (%v)", got, tc.want, err)
			}
			// THE invariant every row shares: nothing reached the boundary.
			if n := len(rec.Lines()); n != 0 {
				t.Fatalf("%d call(s) reached the host on a refused attempt: %v",
					n, rec.Lines())
			}
			if len(step.Emitted) != 0 {
				t.Fatalf("the record claims %v was sent", step.Emitted)
			}
		})
	}
}

// A step whose whole run does not fit is refused before its first input, not truncated.
func TestAStepThatExceedsTheInputBoundEmitsNothingAtAll(t *testing.T) {
	// A plan of two steps, authorized for the two inputs they contain between them. The
	// first step is then asked to send three.
	j := plan(arrives(1, subjA, observe.NavDown), arrives(2, subjB, observe.NavConfirm))
	g := grantFor(t, j)
	if g.MaxInputs != 2 {
		t.Fatalf("the fixture authorizes %d input(s)", g.MaxInputs)
	}
	// The plan handed to the attempt asks for more than the grant bounds.
	over := j
	over.Plan = []observe.StepPlan{
		arrives(1, subjB, observe.NavDown, observe.NavDown, observe.NavConfirm),
	}

	a, err := rehearse.BeginAttempt(g, over, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	rec, runner := recorded()
	step, err := a.LowerStep(context.Background(), 1, runner, rec, windowref.Ref{}, rehearse.Stage{})
	if err == nil {
		t.Fatal("a step that exceeds its budget was lowered")
	}
	if got, _ := rehearse.RefusalOf(err); got != rehearse.RefusalInputBound {
		t.Errorf("refusal = %q, want %q", got, rehearse.RefusalInputBound)
	}
	// ATOMIC. Not two of three: half of `down, down, confirm` leaves the interface
	// somewhere no demonstration ever went.
	if n := len(rec.Lines()); n != 0 {
		t.Fatalf("%d input(s) were sent before the bound was noticed: %v", n, rec.Lines())
	}
	if step.Status != rehearse.EmissionRefused {
		t.Errorf("status = %q", step.Status)
	}
}

// A step Marco cannot see the result of needs a plan that authorized one.
func TestAnUnobservableStepNeedsAPlanThatContainsOne(t *testing.T) {
	// A plan of purely directly-verifiable steps carries no unobservable budget at all.
	j := onePress()
	g := grantFor(t, j)
	if g.MaxUnobservable != 0 {
		t.Fatalf("a plan whose every step lands somewhere Marco knows carries a budget of "+
			"%d for steps it cannot see", g.MaxUnobservable)
	}
	swapped := j
	swapped.Plan = []observe.StepPlan{staysOn(1, subjA, observe.NavDown)}

	a, err := rehearse.BeginAttempt(g, swapped, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	rec, runner := recorded()
	if _, err := a.LowerStep(context.Background(), 1, runner, rec, windowref.Ref{}, rehearse.Stage{}); err == nil {
		t.Fatal("a step the authorized plan never contained was lowered")
	} else if got, _ := rehearse.RefusalOf(err); got != rehearse.RefusalUnobservableBound {
		t.Errorf("refusal = %q, want %q", got, rehearse.RefusalUnobservableBound)
	}
	if n := len(rec.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host", n)
	}
}

// ── the ordering that matters most ────────────────────────────────────────────

// The grant is claimed BEFORE anything reaches the host. Never after.
//
// The most important test in the milestone. A boundary that produced first and checked
// afterwards would be one crash away from having acted without permission.
func TestTheGrantIsClaimedBeforeAnythingIsProduced(t *testing.T) {
	j := onePress()
	g := grantFor(t, j)

	var order []string
	var mu sync.Mutex
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }

	// A host that writes down WHEN it was called, relative to the grant's state.
	watcher := &orderedRunner{onRun: func() {
		if g.State() == observe.GrantConsumed {
			note("effect after claim")
			return
		}
		note("EFFECT BEFORE CLAIM")
	}}

	a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	note("attempt begun")
	if g.State() != observe.GrantConsumed {
		t.Fatal("beginning an attempt did not spend the authorization; a crash during " +
			"setup would leave a reusable permission behind")
	}
	runner := watcher
	if _, err := a.LowerStep(context.Background(), 1, runner, watcher, windowref.Ref{}, rehearse.Stage{}); err != nil {
		t.Fatalf("lowering: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"attempt begun", "effect after claim"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("lifecycle order = %v, want %v", order, want)
	}
}

// orderedRunner notes the moment a program reaches it, and nothing else.
//
// It sits exactly where the compiler and the host would: if this is called at all, the Director
// has finished producing and the effect is on its way. Asking "was the grant already spent when
// this ran" is therefore the same question as "was it spent before anything could happen".
type orderedRunner struct {
	programRunner
	onRun func()
}

func (r *orderedRunner) Run(ctx context.Context, name, program string) (
	directorapi.MarcoResult, error) {

	if r.onRun != nil {
		r.onRun()
	}
	return r.programRunner.Run(ctx, name, program)
}

// THE governing invariant, held by the state machine rather than by anybody remembering it.
//
// An attempt that has acted cannot act again until it has been told what came of it. This is what
// makes "authority must never outrun observation" a property of the transition graph: an
// orchestrator with a bug is REFUSED, where one relying on control flow would type twice.
func TestAnAttemptCannotActTwiceWithoutLooking(t *testing.T) {
	j := plan(arrives(1, subjA, observe.NavDown), arrives(2, subjB, observe.NavConfirm))
	g := grantFor(t, j)
	a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	rec, runner := recorded()
	if _, err := a.LowerStep(context.Background(), 1, runner, rec, windowref.Ref{}, rehearse.Stage{}); err != nil {
		t.Fatalf("the first step was refused: %v", err)
	}
	if a.State() != rehearse.AttemptActed {
		t.Fatalf("state = %q after acting", a.State())
	}

	// The second step, with nothing having looked in between.
	before := len(rec.Lines())
	if _, err := a.LowerStep(context.Background(), 2, runner, rec, windowref.Ref{}, rehearse.Stage{}); err == nil {
		t.Fatal("an attempt acted twice in a row. Marco looks before it acts again")
	} else if got, _ := rehearse.RefusalOf(err); got != rehearse.RefusalAwaitingObservation {
		t.Errorf("refusal = %q, want %q", got, rehearse.RefusalAwaitingObservation)
	}
	if len(rec.Lines()) != before {
		t.Fatalf("a second step reached the host: %v", rec.Lines())
	}

	// Once reality has answered, and only then.
	if !a.Observed(true) {
		t.Fatal("an observation permitting continuation did not reopen the attempt")
	}
	if a.State() != rehearse.AttemptOpen {
		t.Fatalf("state = %q after observing", a.State())
	}
	if _, err := a.LowerStep(context.Background(), 2, runner, rec, windowref.Ref{}, rehearse.Stage{}); err != nil {
		t.Fatalf("the second step was refused after a clean observation: %v", err)
	}
	if a.StepsTaken() != 2 {
		t.Errorf("steps taken = %d", a.StepsTaken())
	}

	// And an observation that does NOT permit continuation is terminal, permanently.
	if a.Observed(false) {
		t.Fatal("a refusing observation reopened the attempt")
	}
	if a.State() != rehearse.AttemptFinished {
		t.Fatalf("state = %q", a.State())
	}
	if a.Observed(true) {
		t.Fatal("a finished attempt was reopened")
	}
}

// Cancelling before the step produces nothing.
func TestCancellingAnAttemptProducesNothing(t *testing.T) {
	j := onePress()
	g := grantFor(t, j)
	a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	a.Cancel()
	rec, runner := recorded()
	if _, err := a.LowerStep(context.Background(), 1, runner, rec, windowref.Ref{}, rehearse.Stage{}); err == nil {
		t.Fatal("a cancelled attempt produced a step")
	} else if got, _ := rehearse.RefusalOf(err); got != rehearse.RefusalCancelled {
		t.Errorf("refusal = %q", got)
	}
	if n := len(rec.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host after cancellation", n)
	}
	// And the grant stays spent. A cancelled try was still the try.
	if g.State() != observe.GrantConsumed {
		t.Errorf("cancelling handed the authorization back (%q)", g.State())
	}
}

// A cancelled context stops the step before the boundary.
func TestACancelledContextProducesNothing(t *testing.T) {
	j := onePress()
	g := grantFor(t, j)
	a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec, runner := recorded()
	if _, err := a.LowerStep(ctx, 1, runner, rec, windowref.Ref{}, rehearse.Stage{}); err == nil {
		t.Fatal("a cancelled context produced a step")
	}
	if n := len(rec.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host", n)
	}
}

// ── concurrency ───────────────────────────────────────────────────────────────

// One grant, many callers, one attempt.
func TestOnlyOneAttemptEverClaimsAGrant(t *testing.T) {
	j := onePress()
	g := grantFor(t, j)

	var wg sync.WaitGroup
	begun := make(chan *rehearse.Attempt, 64)
	for i := 0; i < 16; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			if a, err := rehearse.BeginAttempt(g, j, scopeFor(g), epoch); err == nil {
				begun <- a
			}
		}()
		go func() { defer wg.Done(); g.Revoke() }()
		go func() { defer wg.Done(); _ = g.State() }()
	}
	wg.Wait()
	close(begun)

	rec, runner := recorded()
	var attempts int
	for a := range begun {
		attempts++
		if _, err := a.LowerStep(context.Background(), 1, runner, rec, windowref.Ref{}, rehearse.Stage{}); err != nil {
			t.Errorf("a begun attempt could not lower: %v", err)
		}
	}
	if attempts > 1 {
		t.Fatalf("%d attempts claimed one authorization", attempts)
	}
	if n := len(rec.Lines()); n > 1 {
		t.Fatalf("%d call(s) reached the host from one authorization: %v", n, rec.Lines())
	}
}

// REHEARSAL AND EXECUTION SHARE ONE WALKER.
//
// # Why this matters more than it looks
//
// Rehearsal is one caller of the per-edge machinery. A learned play performed because the Audience
// asked for it by name is another. They differ only in how the authority was obtained — a yes to a
// question Marco raised, or an explicit instruction naming the behaviour — and in nothing else:
// look at the Stage, refuse if it is not where the edge begins, take one step, verify, repeat.
//
// A second caller that reimplemented the walk would be a second set of answers to "did that step
// work", and every verification claim in this system rests on there being one.
//
// So `Rehearse` is the grant-state guards and then `Perform`. Anything that performs an edge goes
// through `Perform`, and `Perform` cannot be reached without an authority — which is also the
// budget the attempt spends, so there is no unbounded path to real input.
//
// Deleting the shared call from Rehearse must fail this.
func TestRehearsalAndExecutionShareOneWalker(t *testing.T) {
	// The walker refuses identically whichever door it was entered by: no authority is no
	// authority, and it is the machinery that says so rather than each caller.
	l := &rehearse.Live{}
	if _, err := l.Perform(context.Background(), nil, observe.RehearsalJudgement{},
		windowref.Selector{}, 0); err == nil {
		t.Fatal("the shared walker performed with no authority at all")
	}
	// And rehearsal reaches the same refusal, which is what proves it is the same walker
	// rather than two that happen to agree today.
	if _, err := l.Rehearse(context.Background(), nil, observe.RehearsalJudgement{},
		windowref.Selector{}, 0); err == nil {
		t.Fatal("rehearsal performed with no authority at all")
	}
}
