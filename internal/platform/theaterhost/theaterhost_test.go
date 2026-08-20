package theaterhost_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Putting on a production, and refusing to when it cannot be done honestly.
//
// # What these hold
//
// The Theater is the layer that turns "activate the thing called Mouse" into something happening
// on a real machine. Every failure mode it has is a way of being wrong in front of somebody:
// pressing the wrong control, pressing nothing and saying it worked, or requiring a provider the
// machine does not have. Each one has a test here.
//
// The alternate-actor tests are the point of the whole design. A play learned through
// accessibility must run where accessibility is absent and something else can do the job, and the
// play must not change. That is proved with a deterministic actor rather than with Vision,
// because it is a CONTRACT claim: any actor satisfying the interface satisfies the play.

// scriptedActor is an Actor whose whole behaviour is declared by the test.
type scriptedActor struct {
	name      string
	available bool
	finds     []theaterhost.Candidate
	findErr   error
	performed []string
}

func (a *scriptedActor) Name() string { return a.name }

// Availability answers with a reason when it cannot act — the field every surface above an Actor
// renders, and the one a bool could not carry.
func (a *scriptedActor) Availability(context.Context) theaterhost.Availability {
	if a.available {
		return theaterhost.Ready("test", "")
	}
	return theaterhost.Unavailable("test", "", "this scripted actor was told to be unavailable")
}
func (a *scriptedActor) Find(_ context.Context, _ theaterhost.Target) (
	[]theaterhost.Candidate, error) {
	return a.finds, a.findErr
}

// Cast writes the Marco this actor would run. It performs nothing -- the Theater.s
// Production boundary runs it, which is what keeps every real input behind the compile gate.
func (a *scriptedActor) Cast(c theaterhost.Candidate, w activate.Way) (string, bool) {
	a.performed = append(a.performed, c.Handle)
	return "use accessibility.\n// " + a.name + " " + string(w) + " " + c.Handle + "\n", true
}

// found is an actor that can play the part.
func found(name string, handles ...string) *scriptedActor {
	a := &scriptedActor{name: name, available: true}
	for _, h := range handles {
		a.finds = append(a.finds, theaterhost.Candidate{Handle: h, Describes: h})
	}
	return a
}

func mouse() theaterhost.Target {
	return theaterhost.Target{Name: "Mouse", Kind: "button"}
}

// ── the production that goes on ───────────────────────────────────────────────

// One available actor, one match: the part is performed.
func TestOneMatchIsPerformed(t *testing.T) {
	a := found("accessibility", "the-mouse-control")
	got := staged(&castRunner{}, a).Activate(context.Background(), mouse())

	if !got.Performed {
		t.Fatalf("nothing was performed: %+v", got)
	}
	if got.Cast != "accessibility" {
		t.Errorf("cast %q, want the actor that found it", got.Cast)
	}
	if got.Refused != "" {
		t.Errorf("the production was refused: %+v", got)
	}
	if len(a.performed) != 1 || a.performed[0] != "the-mouse-control" {
		t.Errorf("the actor performed %v, want the one candidate it found", a.performed)
	}
}

// ── the refusals, which matter more ───────────────────────────────────────────

// Nothing matches: refused, and nothing is performed.
func TestNoMatchIsRefusedAndNothingIsPerformed(t *testing.T) {
	a := found("accessibility")
	got := staged(&castRunner{}, a).Activate(context.Background(), mouse())

	if got.Performed {
		t.Fatal("something was performed for a target nothing matched")
	}
	if got.Refused != theaterhost.TargetNotFound {
		t.Errorf("refusal %q, want %q", got.Refused, theaterhost.TargetNotFound)
	}
	if len(a.performed) != 0 {
		t.Errorf("the actor acted anyway: %v", a.performed)
	}
}

// SEVERAL match: refused, and emphatically not the first.
//
// THE mutation this exists for. Pressing the first of several controls sharing a name is a coin
// toss performed on somebody's computer, and it would be indistinguishable from working — nobody
// would look, because the play would report success.
func TestSeveralMatchesRefuseRatherThanPickTheFirst(t *testing.T) {
	a := found("accessibility", "first", "second")
	got := staged(&castRunner{}, a).Activate(context.Background(), mouse())

	if got.Performed || len(a.performed) > 0 {
		t.Fatalf("one of two identically-named controls was pressed (%v).\nA coin toss on "+
			"somebody's machine reports success either way, which is worse than failing.",
			a.performed)
	}
	if got.Refused != theaterhost.TargetAmbiguous {
		t.Errorf("refusal %q, want %q", got.Refused, theaterhost.TargetAmbiguous)
	}
	if !strings.Contains(got.Detail, "Mouse") {
		t.Errorf("the refusal does not say which name was ambiguous: %q", got.Detail)
	}
}

// No actor at all: refused as a machine that cannot act, not as a screen that is wrong.
//
// The distinction is the whole reason the vocabulary is closed. "I don't know what you mean" and
// "I know exactly what you mean and cannot do it here" send a person to different places.
func TestNoAvailableActorIsItsOwnRefusal(t *testing.T) {
	off := found("accessibility", "the-mouse-control")
	off.available = false
	got := staged(&castRunner{}, off).Activate(context.Background(), mouse())

	if got.Refused != theaterhost.NoActorAvailable {
		t.Errorf("refusal %q, want %q — nothing could act, which is not the same as the "+
			"target being absent", got.Refused, theaterhost.NoActorAvailable)
	}
}

// Verification is not asked HERE any more, and that is the point.
//
// `Activate` resolves, casts and runs. Whether the world then became what somebody expected is
// asked at `Perform`, of a verifier the CALLER brings — see perform_test.go, which holds the
// "an actor sending something is not the application having done anything" invariant.
//
// There were briefly two verifications in this package: a `Changed(ctx) bool` for the play path
// and a `production.Verifier` for the Director's. Two answers to "did that work" is the drift
// Roadmap 34E exists to end, so the narrower one is gone rather than kept for symmetry.

// ── THE portability proof ─────────────────────────────────────────────────────

// A target learned through one actor is activated by ANOTHER, with the play unchanged.
//
// # Why this is the most important test in the package
//
// Because it is the whole claim. A play learned on a machine with the accessibility bridge says
// `do Theater's Activate with target1` — it names no provider, so the same play, byte for byte,
// must go on when accessibility is gone and something else can find the thing.
//
// Deterministic on purpose. This is a CONTRACT claim, not a Vision milestone: any actor satisfying
// the interface satisfies the play, and proving it with a scripted actor proves it for every
// actor that comes later.
func TestATargetLearnedByOneActorIsActivatedByAnother(t *testing.T) {
	// The actor that trained it, no longer available tonight.
	accessibility := found("accessibility", "the-mouse-control")
	accessibility.available = false
	// Something else entirely, which can find the same semantic thing.
	other := found("sighted-pointer", "a-rectangle-on-screen")

	got := staged(&castRunner{}, accessibility, other).
		Activate(context.Background(), mouse())

	if !got.Performed || got.Refused != "" {
		t.Fatalf("the production did not go on without the actor that trained it: %+v.\n"+
			"A play learned through accessibility must not require accessibility "+
			"forever — that is the entire point of a semantic target.", got)
	}
	if got.Cast != "sighted-pointer" {
		t.Errorf("cast %q, want the actor that was actually available", got.Cast)
	}
	if len(accessibility.performed) != 0 {
		t.Error("the unavailable actor was asked to perform")
	}
}

// An actor that cannot look does not veto the ones that can.
//
// A broken bridge is not a finding. Treating "I could not search" as "it is not there" would hide
// an outage behind a sentence about the person's screen.
func TestAnActorThatCannotLookDoesNotBlockOneThatCan(t *testing.T) {
	broken := found("accessibility")
	broken.findErr = errors.New("the bridge went away")
	other := found("sighted-pointer", "a-rectangle-on-screen")

	got := staged(&castRunner{}, broken, other).
		Activate(context.Background(), mouse())

	if !got.Performed || got.Cast != "sighted-pointer" {
		t.Fatalf("a failed search stopped a working actor from performing: %+v", got)
	}
}

// The casting order is tried in order, and the first that finds it performs.
func TestTheFirstActorThatFindsItPerforms(t *testing.T) {
	first := found("accessibility", "the-mouse-control")
	second := found("sighted-pointer", "a-rectangle-on-screen")

	got := staged(&castRunner{}, first, second).
		Activate(context.Background(), mouse())

	if got.Cast != "accessibility" {
		t.Errorf("cast %q, want the first available actor that found it", got.Cast)
	}
	if len(second.performed) != 0 {
		t.Error("a later actor performed as well")
	}
}

// A target with no name is refused before anything is asked.
func TestATargetWithNoNameIsRefused(t *testing.T) {
	a := found("accessibility", "x")
	got := staged(&castRunner{}, a).
		Activate(context.Background(), theaterhost.Target{Kind: "button"})
	if got.Performed || got.Refused == "" {
		t.Errorf("a nameless target produced %+v", got)
	}
}

// ── the Marco boundary ────────────────────────────────────────────────────────

// `do Theater's Activate with target` goes through the Theater, and casts an Actor.
//
// # Why the host needs its own test
//
// Because everything above exercises the Theater directly, and the host is what a running play
// actually reaches. A host that answered `ok` without asking the Theater anything would satisfy
// every test on this page and would activate nothing — the play would report success, and the
// person's computer would sit there.
//
// Mutations: have Invoke return ok without calling Activate; have it call an Actor directly
// instead of casting.
func TestThePlaySentenceGoesThroughTheTheater(t *testing.T) {
	a := found("accessibility", "the-mouse-control")
	h := theaterhost.NewHost(staged(&castRunner{}, a))

	status, _, err := h.Invoke(hostCall("Activate", "Mouse", "button"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if status != "ok" {
		t.Fatalf("status %q, want ok. last: %s", status, h.Last())
	}
	if len(a.performed) != 1 {
		t.Fatalf("the actor was asked to perform %d time(s); the host did not cast anybody",
			len(a.performed))
	}
}

// A refusal reaches the play as a refusal, with the closed reason in the sentence.
func TestARefusedProductionFailsTheSentence(t *testing.T) {
	h := theaterhost.NewHost(staged(&castRunner{}, found("accessibility")))

	status, _, err := h.Invoke(hostCall("Activate", "Mouse", "button"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if status != "failed" {
		t.Fatalf("status %q for a target nothing matched, want failed", status)
	}
	if !strings.Contains(h.Last(), string(theaterhost.TargetNotFound)) {
		t.Errorf("the refusal reads %q and does not name the closed reason", h.Last())
	}
}

// A capability the act does not declare is refused, never silently accepted.
func TestAnUndeclaredCapabilityIsRefused(t *testing.T) {
	h := theaterhost.NewHost(staged(&castRunner{}, found("accessibility", "x")))
	if status, _, _ := h.Invoke(hostCall("Rehearse", "Mouse", "")); status != "failed" {
		t.Errorf("status %q for a capability the Theater act has no sentence for", status)
	}
}

// hostCall builds the sentence a compiled play makes.
func hostCall(action, name, kind string) runtime.HostCall {
	set := runtime.NewSet()
	set.Put("Name", runtime.Text(name))
	if kind != "" {
		set.Put("Kind", runtime.Text(kind))
	}
	return runtime.HostCall{
		Act: "Theater", Action: action, Input: runtime.SetVal(set),
		Ctx: context.Background(),
	}
}

// ── the Production boundary's runner, in tests ────────────────────────────────

// castRunner records the Marco each production actually ran.
//
// Actors no longer perform: they write legal Marco and the Theater's Production boundary runs it
// through an injected runner, so every real input passes the compile gate and a dry run has
// something to record. These tests assert against what was RUN rather than what an actor was
// asked to do, which is the same fact one layer closer to the machine.
type castRunner struct {
	ran  []string
	fail error
}

func (r *castRunner) Run(_ context.Context, _, program string) (directorapi.MarcoResult, error) {
	r.ran = append(r.ran, program)
	if r.fail != nil {
		return directorapi.MarcoResult{}, r.fail
	}
	return directorapi.MarcoResult{}, nil
}

// staged builds a Theater with a runner installed, as production always has one.
func staged(r directorapi.MarcoRunner, actors ...theaterhost.Actor) *theaterhost.Theater {
	return theaterhost.New(actors...).WithRunner(r)
}
