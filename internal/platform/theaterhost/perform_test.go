package theaterhost_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/production"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// One production body, entered from two places.
//
// # What this file holds
//
// Until now a saved play reached the Theater and a live rehearsal did not — it had its own target
// resolution, its own execution and its own verification, and only one of the two knew that a
// Windows Settings navigation item is a selection item.
//
// These tests are about the seam that ends that: authority in, production out, the same refusal
// words whichever caller asked.

// grant is one permission, spendable once — the shape of a rehearsal grant.
type grant struct {
	forTarget string
	spent     bool
	claims    int
}

func (g *grant) Claim(r production.Request) error {
	g.claims++
	if g.spent {
		return errors.New("this permission has already been spent")
	}
	if g.forTarget != "" && r.Target.Name != g.forTarget {
		return errors.New("permitted for " + g.forTarget + ", asked for " + r.Target.Name)
	}
	g.spent = true
	return nil
}

// says is an actor that answers however a test needs.
type says struct {
	available bool
	found     []theaterhost.Candidate
	findErr   error
}

func (s *says) Name() string                   { return "test" }
func (s *says) Available(context.Context) bool { return s.available }
func (s *says) Find(context.Context, theaterhost.Target) ([]theaterhost.Candidate, error) {
	return s.found, s.findErr
}

// Cast writes the Marco this actor would run; the Production boundary runs it.
func (s *says) Cast(theaterhost.Candidate, activate.Way) (string, bool) {
	return "use accessibility.\n// cast\n", true
}

// checks is a verifier that answers however a test needs.
type checks struct {
	observed string
	ok       bool
	asked    int
}

func (c *checks) Verify(context.Context, production.Request) (string, bool) {
	c.asked++
	return c.observed, c.ok
}

func oneCandidate() []theaterhost.Candidate {
	return []theaterhost.Candidate{{Handle: "Mouse", Describes: "Mouse"}}
}

// sends is the runner these tests stage: it counts productions and can fail one.
type sends struct {
	performed int
	fail      string
}

func (s *sends) Run(context.Context, string, string) (directorapi.MarcoResult, error) {
	s.performed++
	if s.fail != "" {
		return directorapi.MarcoResult{
			Failed: []string{"Accessibility's Invoke"},
			Returned: map[string]directorapi.MarcoValue{
				"Accessibility's Invoke": {Error: s.fail},
			},
		}, nil
	}
	return directorapi.MarcoResult{}, nil
}

func request() production.Request {
	return production.Request{
		Target: production.Target{Name: "Mouse", Kind: "button"},
		Expect: "subj_mouse",
	}
}

// ── authority ─────────────────────────────────────────────────────────────────

// The Theater refuses without authority, before it looks at anything.
//
// # Why the order matters
//
// A production that turns out to be impossible must still not have been permitted. Resolving a
// target before checking permission would leak "what is on your screen" past the boundary, and
// would let a refused production reveal what it would have acted on.
//
// Deleting the Claim call must fail this.
func TestTheaterRefusesWithoutAuthority(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run)

	got := th.Perform(context.Background(), request(), nil, nil)
	if got.Refused != production.NotPermitted {
		t.Fatalf("a production with no authority was refused with %q, want not_permitted",
			got.Refused)
	}
	if got.Attempted || got.Performed {
		t.Error("an unauthorised production was attempted")
	}
	if run.performed != 0 {
		t.Errorf("the actor acted %d time(s) without permission", run.performed)
	}
}

// A refused authority stops the production and nothing is looked at.
func TestARefusedAuthorityStopsTheProduction(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run)

	got := th.Perform(context.Background(), request(), &grant{forTarget: "Something Else"}, nil)
	if got.Refused != production.NotPermitted {
		t.Fatalf("refused with %q, want not_permitted", got.Refused)
	}
	if run.performed != 0 {
		t.Error("the actor acted despite authority refusing")
	}
}

// The Theater claims authority exactly once, and cannot spend it twice.
//
// One attempt means one. A Theater that retried would be refused by the authority rather than
// trusted not to — which is the point of handing it something spendable rather than a boolean.
func TestTheaterCannotSpendAuthorityTwice(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run)
	g := &grant{}

	if got := th.Perform(context.Background(), request(), g, &checks{ok: true}); got.Refused != "" {
		t.Fatalf("the first production was refused: %s", production.Describe(got))
	}
	if g.claims != 1 {
		t.Errorf("authority was claimed %d times for one production", g.claims)
	}
	second := th.Perform(context.Background(), request(), g, &checks{ok: true})
	if second.Refused != production.NotPermitted {
		t.Errorf("a second production on a spent permission was %q", second.Refused)
	}
	if run.performed != 1 {
		t.Errorf("the actor acted %d times on one permission", run.performed)
	}
}

// ── verification ──────────────────────────────────────────────────────────────

// An Actor reporting success is NOT a verified production.
//
// The invariant the whole seam rests on. An actor saying it sent something is not the application
// having done anything, and a Theater that conflated them would report success for every press
// into a window that ignored it.
func TestActorSuccessIsNotVerifiedSuccess(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	// The verifier says the world did NOT end up where it was expected.
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run)

	got := th.Perform(context.Background(), request(), &grant{}, &checks{observed: "subj_elsewhere"})
	if !got.Performed {
		t.Fatal("the actor did not act")
	}
	if got.Verified {
		t.Fatal("a production the verifier rejected was reported as verified.\nAn actor " +
			"reporting success means it sent something, which is not the application " +
			"having done anything.")
	}
	if got.Refused != production.NotVerified {
		t.Errorf("refused with %q, want not_verified", got.Refused)
	}
	if got.Observed != "subj_elsewhere" {
		t.Errorf("the report says the world is at %q; the Director needs where it "+
			"actually ended to decide what to do next", got.Observed)
	}
}

// With no verifier at all, a production is honest rather than successful.
//
// The standalone saved-play case: no observation stack, nothing to check with. `not_verified` is
// the truthful answer and it must never become success.
func TestNoVerifierMeansNotVerifiedNeverSuccess(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run) // no verifier: a standalone runtime has none

	got := th.Perform(context.Background(), request(), &grant{}, nil)
	if got.Verified {
		t.Fatal("a production with nothing to check it was reported verified")
	}
	if got.Refused != production.NotVerified {
		t.Errorf("refused with %q, want not_verified", got.Refused)
	}
	if !got.Performed {
		t.Error("the production is reported as not performed; it was — that is why the " +
			"answer is not_verified rather than a refusal")
	}
}

// A verified production says so, and carries where it ended.
func TestAVerifiedProductionReportsWhereItEnded(t *testing.T) {
	actor := &says{available: true, found: oneCandidate()}
	v := &checks{observed: "subj_mouse", ok: true}
	th := theaterhost.New(actor).WithRunner(&sends{})

	got := th.Perform(context.Background(), request(), &grant{}, v)
	if !got.Verified || got.Refused != "" {
		t.Fatalf("a good production reported %s", production.Describe(got))
	}
	if got.Observed != "subj_mouse" {
		t.Errorf("the report ended at %q", got.Observed)
	}
	if v.asked != 1 {
		t.Errorf("verification was asked %d time(s)", v.asked)
	}
	if got.Cast != "test" {
		t.Errorf("the report says %q was cast", got.Cast)
	}
}

// ── one refusal vocabulary ────────────────────────────────────────────────────

// The same wall produces the same word, whichever caller hit it.
//
// A rehearsal and a saved play meeting the same failure must say the same thing, or debugging one
// teaches nothing about the other — and two vocabularies is how the two bodies drifted apart in
// the first place.
func TestOneRefusalVocabularyForBothCallers(t *testing.T) {
	cases := []struct {
		name  string
		actor *says
		run   *sends
		want  production.Refusal
	}{
		{"nothing answers to that name",
			&says{available: true}, &sends{}, production.TargetNotFound},
		{"several things answer to it",
			&says{available: true, found: []theaterhost.Candidate{
				{Handle: "Mouse"}, {Handle: "Mouse"},
			}}, &sends{}, production.TargetAmbiguous},
		{"nothing can act",
			&says{available: false}, &sends{}, production.NoActorAvailable},
		{"the production ran and the capability failed",
			&says{available: true, found: oneCandidate()},
			&sends{fail: "the bridge went away"},
			production.PerformFailed},
	}
	for _, c := range cases {
		th := theaterhost.New(c.actor).WithRunner(c.run)
		got := th.Perform(context.Background(), request(), &grant{}, &checks{ok: true})
		if got.Refused != c.want {
			t.Errorf("%s: refused with %q, want %q", c.name, got.Refused, c.want)
		}
		if got.Detail == "" {
			t.Errorf("%s: refused with no sentence a person could read", c.name)
		}
	}
}

// The Theater never picks one of several.
//
// Ambiguity is a fact about the stage, and choosing the first would attach a person's
// demonstration to a coin toss.
func TestTheaterNeverPicksAmongAmbiguousTargets(t *testing.T) {
	actor := &says{available: true, found: []theaterhost.Candidate{
		{Handle: "Mouse", Describes: "Mouse"},
		{Handle: "Mouse", Describes: "Mouse (another)"},
	}}
	run := &sends{}
	th := theaterhost.New(actor).WithRunner(run)

	got := th.Perform(context.Background(), request(), &grant{}, &checks{ok: true})
	if got.Refused != production.TargetAmbiguous {
		t.Fatalf("refused with %q, want target_ambiguous", got.Refused)
	}
	if run.performed != 0 {
		t.Error("the Theater acted on one of several things called the same name")
	}
	if !strings.Contains(got.Detail, "Mouse") {
		t.Errorf("the refusal does not say what was ambiguous: %q", got.Detail)
	}
}

// A production the Theater refuses before acting is not reported as performed.
func TestARefusedProductionIsNotPerformed(t *testing.T) {
	th := theaterhost.New(&says{available: true}).
		WithRunner(&sends{})
	got := th.Perform(context.Background(), request(), &grant{}, &checks{ok: true})
	if got.Performed {
		t.Error("a target that was never found was reported as performed")
	}
	if !got.Attempted {
		t.Error("an authorised production that found nothing is still an attempt")
	}
}

// ── the window a production belongs to ────────────────────────────────────────

// notes is an actor that records the candidate it was asked to cast.
type notes struct {
	cast []theaterhost.Candidate
}

func (n *notes) Name() string                   { return "notes" }
func (n *notes) Available(context.Context) bool { return true }
func (n *notes) Find(_ context.Context, t theaterhost.Target) ([]theaterhost.Candidate, error) {
	// A candidate with NO window, as an Actor that forgot the scope would return.
	return []theaterhost.Candidate{{Handle: t.Name, Describes: t.Name}}, nil
}

func (n *notes) Cast(c theaterhost.Candidate, _ activate.Way) (string, bool) {
	n.cast = append(n.cast, c)
	return "use accessibility.\n// cast\n", true
}

// The Theater scopes the production, whatever the Actor returned.
//
// # Why the boundary insists rather than trusting
//
// Which window a production belongs to is the caller's knowledge. An Actor that resolved without
// it — or that simply did not carry it back — would have its program act in whatever is in front,
// which on a desktop finds a control of the right name in somebody's email and presses it.
//
// Deleting the assignment in Activate must fail this.
func TestTheWindowTravelsToTheCastProgram(t *testing.T) {
	actor := &notes{}
	req := request()
	req.Window = "hwnd:100"

	got := theaterhost.New(actor).WithRunner(&sends{}).
		Perform(context.Background(), req, &grant{}, &checks{ok: true})
	if !got.Performed {
		t.Fatalf("nothing was performed: %s", production.Describe(got))
	}
	if len(actor.cast) != 1 {
		t.Fatalf("%d candidate(s) cast", len(actor.cast))
	}
	// And the program that ran comes back, because the Director is the caller that shows it:
	// a dry run prints what it would have sent, and the Actor writes that now, not the
	// Director. Dropping it in run() must fail this.
	if !strings.Contains(got.Program, "// cast") {
		t.Errorf("the report does not carry the Marco that ran: %q", got.Program)
	}
	if actor.cast[0].Window != "hwnd:100" {
		t.Errorf("the cast candidate is scoped to %q, want the request's window.\n"+
			"Without it the program acts on whatever is in front, which finds a control "+
			"of the right name in some other application entirely.", actor.cast[0].Window)
	}
}
