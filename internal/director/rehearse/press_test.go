package rehearse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/production"
)

// How a control gets pressed, now that pressing is a production.
//
// # The live failure this file exists for
//
// A rehearsal reached a Windows Settings navigation item and reported:
//
//	step 1: input_failed — expected subj_543793ccc326, saw nothing
//	        (cannot_express: unsupported: the control does not implement InvokePattern)
//
// Nothing was clicked. Nothing was too fast. The demonstration had been read perfectly and the
// control had been found — the Director asked for InvokePattern, the control is a SELECTION item,
// and the attempt ended there. Settings is built almost entirely from selection items, so this
// was every navigation step on the most obvious application anybody would teach Marco with.
//
// # Why the fix is not in this package any more
//
// It was, for a while: a fallback ladder here, walking Invoke → Select → Expand → Toggle over
// `marcoexec` operations. And there was a second one in the Theater, for saved plays. Two bodies,
// and the first live rehearsal after the Theater landed proved they had already drifted.
//
// So the ladder lives in ONE place now — the Theater's production boundary — and what this file
// checks is the thing this layer is actually responsible for: that a press is handed over whole,
// with authority, and that nothing here presses anything itself.

// ── the seam ──────────────────────────────────────────────────────────────────

// theater records what it was asked to put on, and answers however a test needs.
type theater struct {
	asked   []production.Request
	granted []production.Authority
	checked []production.Verifier
	report  production.Report
	refuse  production.Refusal
	detail  string
}

func (p *theater) Perform(_ context.Context, r production.Request, a production.Authority,
	v production.Verifier) production.Report {

	p.asked = append(p.asked, r)
	p.granted = append(p.granted, a)
	p.checked = append(p.checked, v)
	if a != nil {
		if err := a.Claim(r); err != nil {
			return production.Refuse(production.NotPermitted, "%s", err)
		}
	}
	if p.refuse != "" {
		return production.Refuse(p.refuse, "%s", p.detail)
	}
	out := p.report
	out.Attempted = true
	return out
}

// pointAt is one step that presses a named control, as a demonstration produced it.
func pointAt(role, label, expect string) observe.StepPlan {
	return observe.StepPlan{
		Position: 1, Intents: []observe.NavIntent{observe.NavPoint},
		Targets:       []observe.SemanticTarget{{Role: role, Label: label}},
		Verifiability: observe.DirectlyVerifiable, Expect: expect,
	}
}

// pressing lowers one press step against the given stage.
func pressing(t *testing.T, step observe.StepPlan, stage Stage) (StepEmission, error) {
	t.Helper()
	j := observe.RehearsalJudgement{
		Relationship: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		Application:  "settings", Eligible: true,
		Source: "subj_a", Destination: "subj_b", Digest: "digest_1",
		Plan: []observe.StepPlan{step}, Inputs: len(step.Intents),
	}
	at := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	g, err := observe.NewRehearsalGrant("t", j, at)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	a, err := BeginAttempt(g, j, Scope{Application: g.Application, Source: g.Source,
		Relationship: g.Relationship, Evidence: g.Evidence}, at)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	return a.LowerStep(context.Background(), 1, nil, nil,
		windowref.Ref{ID: "hwnd:100", Handle: 100}, stage)
}

// ── the press is handed over, whole ───────────────────────────────────────────

// A demonstrated click becomes a production, described semantically and nothing else.
//
// # What must NOT be in the request
//
// An element id. The old path resolved the name to a runtime handle here and carried it down —
// which works until the tree redraws between deciding and doing, and then fails in a way that
// looks like flakiness. What crosses now is a name, a kind and a window; the Theater resolves at
// the moment of acting.
//
// Deleting the Perform call must fail this.
func TestALivePressIsPutOnByTheTheater(t *testing.T) {
	th := &theater{report: production.Report{Performed: true, Cast: "accessibility"}}

	got, err := pressing(t, pointAt("item", "Bluetooth & devices", "subj_bt"),
		Stage{Produce: th})
	if err != nil {
		t.Fatalf("the press was refused: %v", err)
	}
	if len(th.asked) != 1 {
		t.Fatalf("the Theater was asked for %d production(s), want one", len(th.asked))
	}
	req := th.asked[0]
	if req.Target.Name != "Bluetooth & devices" {
		t.Errorf("the production asks for %q", req.Target.Name)
	}
	if req.Target.Kind != "item" {
		t.Errorf("the production asks for kind %q; a word shared by a button and a "+
			"caption is only one thing you can press", req.Target.Kind)
	}
	if req.Window != "hwnd:100" {
		t.Errorf("the production is scoped to %q, want the window the attempt holds",
			req.Window)
	}
	if req.Expect != "subj_bt" {
		t.Errorf("the production expects %q", req.Expect)
	}
	if !got.Reached {
		t.Error("a production that was put on is reported as never reaching anything")
	}
}

// Nothing in this package presses anything itself.
//
// One production per press. A rehearsal that retried under another pattern would be Marco
// pressing a control repeatedly in different ways until something happened, and the ladder that
// legitimately does try several ways belongs to the Theater, which knows which failures license
// it. Two ladders is what this milestone removed.
func TestARehearsalNeverPressesTwice(t *testing.T) {
	th := &theater{refuse: production.PerformFailed, detail: "the control refused"}

	if _, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"), Stage{Produce: th}); err == nil {
		t.Fatal("a failed production reported success")
	}
	if len(th.asked) != 1 {
		t.Errorf("the Theater was asked %d times for one press.\nRetrying a press here "+
			"would be a second activation ladder, which is what the Theater owns.",
			len(th.asked))
	}
}

// ── authority ─────────────────────────────────────────────────────────────────

// The Theater is handed a permission, not a promise.
//
// It is spendable exactly once and only for the target the plan authorised. Every bound that
// makes a rehearsal safe was checked before this and none of it is visible from inside the
// Theater, so what crosses is something that refuses rather than something that trusts.
//
// Deleting the grant — passing nil — must fail this.
func TestAPressCarriesAuthorityForThatTargetOnly(t *testing.T) {
	th := &theater{report: production.Report{Performed: true}}

	if _, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"), Stage{Produce: th}); err != nil {
		t.Fatalf("the press was refused: %v", err)
	}
	if len(th.granted) != 1 || th.granted[0] == nil {
		t.Fatal("the Theater was handed no authority for a real press")
	}
	auth := th.granted[0]
	// SPENT. A second claim on the same permission must be refused.
	if err := auth.Claim(production.Request{Target: production.Target{Name: "Mouse"}}); err == nil {
		t.Error("a spent permission was claimed a second time")
	}
}

// A permission does not cover a different target.
func TestAPressPermissionIsBoundToItsTarget(t *testing.T) {
	g := &pressGrant{target: "Mouse"}
	if err := g.Claim(production.Request{Target: production.Target{Name: "Bluetooth"}}); err == nil {
		t.Fatal("permission to press Mouse was spent on Bluetooth")
	}
	if g.spent {
		t.Error("a refused claim spent the permission anyway")
	}
	if err := g.Claim(production.Request{Target: production.Target{Name: "Mouse"}}); err != nil {
		t.Errorf("the permission it was minted for was refused: %v", err)
	}
}

// ── refusals keep their reason ────────────────────────────────────────────────

// The Theater's word survives the boundary, translated rather than flattened.
//
// A step that ends `input_failed` with no reason is the reporting gap this session kept finding:
// the explanation existed one layer down and nothing carried it.
func TestAProductionRefusalKeepsItsReason(t *testing.T) {
	cases := []struct {
		name   string
		refuse production.Refusal
		detail string
		want   Refusal
	}{
		{"nothing answers to that name", production.TargetNotFound,
			"nothing here is called \"Mouse\"", RefusalControlNotFound},
		{"several things answer to it", production.TargetAmbiguous,
			"2 things here are called \"Mouse\"", RefusalControlNotFound},
		{"nothing can act", production.NoActorAvailable,
			"nothing on this machine can act", RefusalLowering},
		{"the control refused", production.PerformFailed,
			"the control does not implement InvokePattern", RefusalLowering},
	}
	for _, c := range cases {
		th := &theater{refuse: c.refuse, detail: c.detail}
		_, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"), Stage{Produce: th})
		if err == nil {
			t.Errorf("%s: reported success", c.name)
			continue
		}
		got, _ := RefusalOf(err)
		if got != c.want {
			t.Errorf("%s: refused with %q, want %q", c.name, got, c.want)
		}
		if !strings.Contains(err.Error(), c.detail) {
			t.Errorf("%s: the sentence lost the Theater's reason: %q", c.name, err)
		}
	}
}

// A Director with no Theater refuses before emitting rather than reaching for something else.
//
// Fail-closed, and it is the reason the old direct path could be deleted outright: there is no
// fallback to drift.
func TestAPressWithNoTheaterRefusesBeforeEmitting(t *testing.T) {
	got, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"), Stage{})
	if err == nil {
		t.Fatal("a press with nothing to put it on reported success")
	}
	if got.Reached {
		t.Error("a refused press is reported as having reached something")
	}
	if r, _ := RefusalOf(err); r != RefusalLowering {
		t.Errorf("refused with %q", r)
	}
}

// ── verification travels with the request ─────────────────────────────────────

// A dry press brings no verifier, and a live one brings the Director's.
//
// Nothing reached a computer on the dry path, so there is nothing to look at; a verifier there
// would be inventing an observation. What must never happen is the absence of one reading as
// success, which is `production.Verifier`'s own invariant.
func TestTheVerifierTravelsWithThePress(t *testing.T) {
	th := &theater{report: production.Report{Performed: true}}
	if _, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"), Stage{Produce: th}); err != nil {
		t.Fatalf("the press was refused: %v", err)
	}
	if th.checked[0] != nil {
		t.Error("a caller that brought no verification handed one over anyway")
	}

	checked := &theater{report: production.Report{Performed: true, Verified: true}}
	if _, err := pressing(t, pointAt("item", "Mouse", "subj_mouse"),
		Stage{Produce: checked, Verify: nothingHappened{}}); err != nil {
		t.Fatalf("the press was refused: %v", err)
	}
	if checked.checked[0] == nil {
		t.Fatal("the caller's verification was not handed to the production")
	}
}

// nothingHappened is a verifier that says the world did not move.
type nothingHappened struct{}

func (nothingHappened) Verify(context.Context, production.Request) (string, bool) {
	return "", false
}

// ── a screen still painting is not a screen somewhere else ────────────────────

// An unsettled screen is not reported as a wrong state.
//
// # The live failure
//
// A rehearsal recognised Home, clicked through to Bluetooth, and reported:
//
//	wrong_state — expected subj_543793ccc326, saw subj_892a4cc30f41
//
// Those two subjects are the same Settings page: one read after it had finished painting, one
// read part-way through, four controls short. Marco had already noticed — the settle wait ran out
// and the record says `still_changing` — and the classification compared identities anyway,
// turning "I looked too early" into "you went somewhere else".
//
// It is also how the twin subjects were minted in the first place, so the same mistake produced
// the confusion and then reported it as the person's fault.
func TestAnUnsettledScreenIsNotAWrongState(t *testing.T) {
	verified := func(settle SettleOutcome, observed, expect string) Outcome {
		rec := &StepRecord{
			Settle: settle, Observed: observed, Expect: expect,
			Verification: observe.DirectlyVerifiable,
		}
		classifyOutcome(rec)
		return rec.Outcome
	}

	// SETTLED and different: a real disagreement, reported as one.
	if got := verified(SettleStable, "subj_elsewhere", "subj_wanted"); got != WrongState {
		t.Errorf("a settled screen that is somewhere else reads as %q, want wrong_state — "+
			"withholding a real disagreement would let a route be promoted for going "+
			"somewhere nobody asked for", got)
	}
	// STILL CHANGING and different: nothing has been shown.
	if got := verified(SettleChanging, "subj_elsewhere", "subj_wanted"); got != Unobservable {
		t.Errorf("a screen still painting that does not yet match reads as %q.\nMarco "+
			"looked too early; that is not evidence the person went somewhere else, and "+
			"an identity read mid-render is what minted the twin subjects.", got)
	}
	// STILL CHANGING and MATCHING: agreement is still agreement. The screen reached the
	// expected place and kept working, which is an ordinary page finishing.
	if got := verified(SettleChanging, "subj_wanted", "subj_wanted"); got != DirectlyVerified {
		t.Errorf("a screen that reached the expected place before it stopped moving reads "+
			"as %q, want verified", got)
	}
	if got := verified(SettleStable, "subj_wanted", "subj_wanted"); got != DirectlyVerified {
		t.Errorf("a settled screen at the expected place reads as %q", got)
	}
}

// The same rule for a step whose progress could never be seen.
//
// Containment that appears broken on a screen still painting has not been shown to be broken.
func TestAnUnsettledScreenDoesNotBreakContainment(t *testing.T) {
	rec := &StepRecord{
		Settle: SettleChanging, Observed: "subj_elsewhere", Expect: "subj_wanted",
		Verification: observe.ProgressUnobservable,
	}
	classifyOutcome(rec)
	if rec.Outcome != Unobservable {
		t.Errorf("a contained step reads as %q on a screen that never settled", rec.Outcome)
	}
}
