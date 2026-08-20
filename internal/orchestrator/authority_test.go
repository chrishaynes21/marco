package orchestrator_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Knowing which play the user means, and being allowed to perform it.
//
// Until ADR-029 there was no seam between those two: `Deps.Do` resolved a phrase and called
// `Deps.Run` in the same breath. These tests hold the door open and look through it.
//
// # How they enter, and why it changed in Phase 3
//
// They used to enter through `Deps.Do`. It had ZERO production callers — and that is the sharp
// end of it, because this was the densest authority coverage in the repository and it was proving
// that a dead function honoured the door. `cmd/marco/authoritybypass_test.go` is the standing
// record of what that costs: a real bypass lived on the shipped `marco do` path while every test
// here passed, because every test here entered somewhere else.
//
// So they now call the PRODUCTION units — `Classify`, then `Authorize`, in that order — which is
// literally what `cmd/marco/intake.go`'s `performOnePlay` does before anything happens. The `door`
// helper below is those two calls and nothing else; it deliberately performs nothing, so no test
// in this file can quietly start depending on a wrapper again.

// ── a world of plays ──────────────────────────────────────────────────────────

// recordingHost is the last thing before a computer, replaced with a notebook.
//
// The SAME `runtime.Host` interface the OS host implements, so everything above it — resolution,
// authority, the lexer, the parser, the graph builder, the compiler, the frame scheduler — is the
// production path with one substitution at the bottom.
type recordingHost struct{ calls []string }

func (h *recordingHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	line := c.Act + "'s " + c.Action
	if !c.Input.IsAbsent() {
		line += " with " + c.Input.String()
	}
	h.calls = append(h.calls, line)
	return "ok", runtime.Absent(), nil
}

// aPlay is an ordinary Marco program that presses two things.
const aPlay = `use os.

the Volume is an actor.

this can Mute.
this's Mute does...
    do OS's Navigate with "down".
    do OS's Navigate with "confirm".
    this is ok!

the App is a script.

do Volume's Mute...
    when ok?
        log "done".
    or?
        log that's error.
`

// world builds a routes tree with one play, optionally learned.
func world(t *testing.T, kind routes.Kind, source string) (routes.Registry, routes.Route) {
	t.Helper()
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if kind == routes.KindAuthored {
		if err := reg.Save(rt, source); err != nil {
			t.Fatalf("saving: %v", err)
		}
		return reg, rt
	}
	if err := reg.SaveWithOrigin(rt, source, routes.Origin{
		Kind: kind, Application: "testgame", From: "subj_a", To: "subj_b",
		Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	return reg, rt
}

// deps builds an orchestrator over a routes tree, with a notebook at the bottom.
func deps(t *testing.T, reg routes.Registry, answer string, gated bool) (
	orchestrator.Deps, *recordingHost, *bytes.Buffer) {

	t.Helper()
	host := &recordingHost{}
	out := &bytes.Buffer{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host},
		In:    strings.NewReader(answer + "\n"),
		Out:   out,
		App:   func() string { return "testgame" },
	}
	if gated {
		d.Authority = orchestrator.AskFirst{Deps: d}
	}
	return d, host, out
}

// door is the production sequence, spelled out: which play, then may it run.
//
// The same two calls in the same order as `performOnePlay`. It performs NOTHING — which is the
// property this whole milestone is about, and the reason the helper stops here instead of running
// the play the way the retired `Deps.Do` did.
func door(t *testing.T, d orchestrator.Deps, phrase string) (
	orchestrator.Resolved, orchestrator.Decision) {

	t.Helper()
	rt, ok := d.Reg.Resolve(d.App(), phrase)
	if !ok {
		t.Fatalf("no play answers to %q in %q", phrase, d.App())
	}
	r := orchestrator.Classify(d.Reg, rt, phrase)
	return r, orchestrator.Authorize(r, d.Authority)
}

// ── THE headline: the Marco Moment, dry ───────────────────────────────────────

// A learned play is resolved, authorized, compiled by the ordinary compiler, and would emit
// exactly what the `.marco` says.
//
// # What this proves and what it does not
//
// It proves the chain from a phrase to the last thing before a computer, with the ONE substitution
// being which Host was installed. It does not prove anything happened: the host is a notebook, so
// the honest name for this is a dry path, not a performance.
//
// It also does not prove this is how a learned play reaches a keyboard TODAY. Since Phase 0
// `performOnePlay` forks: a play with intact learned provenance is performed by the Director,
// which can see, and everything else runs through the local runner. What survives here is the pair
// of claims still true of the FILE — the question asked before it, and the operations it would
// emit — and the wiring claim lives beside `runInvocation`, in `cmd/marco/authoritybypass_test.go`,
// which drives the real fork.
//
// The operations come from the FILE. Nothing here reads a ProcedureCandidate, a rehearsal record
// or any Director evidence — the play is the only input, which is the whole point of having
// written it down.
func TestTheMarcoMomentDryPath(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, out := deps(t, reg, "y", true)

	// A fresh orchestrator over a directory path: no session, no proposal, no screen state,
	// no window generation, no process id. Just the tree.
	r, decision := door(t, d, "volume")
	if !r.Learned() {
		t.Fatalf("the fixture resolved as %s/%s", r.Kind, r.Provenance)
	}
	if !decision.Allow() {
		t.Fatalf("an explicit yes was not honoured: %+v", decision)
	}

	// It asked, in the user's words, without a single backstage term.
	asked := out.String()
	if !strings.Contains(asked, "learned") || !strings.Contains(asked, "Run it now?") {
		t.Errorf("Marco did not ask before running a play it wrote:\n%s", asked)
	}
	for _, backstage := range []string{"candidate", "digest", "evidence", "rehears",
		"provenance", "subj_", "verdict"} {
		if strings.Contains(strings.ToLower(asked), backstage) {
			t.Errorf("the question mentions %q:\n%s", backstage, asked)
		}
	}
	// Asking performed nothing. The notebook is still blank here.
	if len(host.calls) != 0 {
		t.Fatalf("putting the question performed %v", host.calls)
	}

	// And then the ORDINARY runtime performed the play into a notebook.
	runSavedPlay(t, d, "volume")
	want := []string{`OS's Navigate with down`, `OS's Navigate with confirm`}
	if len(host.calls) != len(want) {
		t.Fatalf("the host was asked for %v, want %v", host.calls, want)
	}
	for i := range want {
		if host.calls[i] != want[i] {
			t.Fatalf("call %d is %q, want %q", i+1, host.calls[i], want[i])
		}
	}
}

// ── resolution is not permission ──────────────────────────────────────────────

// Resolving a play performs nothing.
//
// The seam, held open. `Classify` answers WHICH play and can be inspected, logged and refused
// without a host being asked for anything. (It answered through `Deps.Resolve` until Phase 3;
// `Classify` is the function `performOnePlay` actually calls, and `Deps.Resolve` was a wrapper
// nothing called.)
func TestResolvingAPlayPerformsNothing(t *testing.T) {
	reg, rt := world(t, routes.KindLearned, aPlay)
	d, host, _ := deps(t, reg, "y", true)

	found, ok := d.Reg.Resolve(d.App(), "volume")
	if !ok {
		t.Fatal("the play was not resolved")
	}
	r := orchestrator.Classify(d.Reg, found, "volume")
	if !r.Learned() {
		t.Fatalf("resolved as %s/%s", r.Kind, r.Provenance)
	}
	if len(host.calls) != 0 {
		t.Fatalf("resolving performed %v", host.calls)
	}
	// And there is nothing on the answer that could perform it.
	if r.Route.Slug != rt.Slug {
		t.Errorf("resolved to %+v", r.Route)
	}
}

// Saying no leaves the play alone, and is not the same as not finding it.
func TestDecliningIsNotTheSameAsNotFinding(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, out := deps(t, reg, "n", true)

	r, decision := door(t, d, "volume")
	if decision.Allow() {
		t.Fatal("a no was read as a yes")
	}
	if len(host.calls) != 0 {
		t.Fatalf("a declined play performed %v", host.calls)
	}
	// The user is not told the play is missing, and not told it is dangerous. Both halves of
	// what they read are checked — the question, and the sentence `performOnePlay` prints from
	// the decision — because the refusal wording lives on the decision, not in the prompt.
	said := strings.ToLower(out.String() + "\n" + decision.Sentence)
	for _, wrong := range []string{"don't know", "do not know", "unsafe", "refus"} {
		if strings.Contains(said, wrong) {
			t.Errorf("declining was reported as %q:\n%s", wrong, said)
		}
	}
	// The decision itself keeps the two apart.
	dec := orchestrator.Authorize(r, declining{})
	if dec.Verdict != orchestrator.Declined {
		t.Errorf("verdict = %q", dec.Verdict)
	}
	if dec.Reason != orchestrator.ReasonUserDeclined {
		t.Errorf("reason = %q", dec.Reason)
	}
}

type declining struct{}

func (declining) Allow(orchestrator.Resolved) orchestrator.Decision {
	return orchestrator.Decision{Verdict: orchestrator.Declined,
		Reason: orchestrator.ReasonUserDeclined}
}

// With no way to ask, a learned play is refused rather than assumed.
func TestALearnedPlayIsRefusedWhenThereIsNoWayToAsk(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, _ := deps(t, reg, "", false) // no Authority wired

	_, decision := door(t, d, "volume")
	if decision.Allow() {
		t.Fatal("a learned play was allowed with nothing able to ask")
	}
	// Refused, not Declined. Marco declining on its own account is not a claim the person
	// said no, and the two send a front end to different places.
	if decision.Verdict != orchestrator.Refused {
		t.Errorf("verdict = %q, want %q", decision.Verdict, orchestrator.Refused)
	}
	if len(host.calls) != 0 {
		t.Fatalf("a learned play ran with nothing able to ask: %v", host.calls)
	}
}

// ── authored, taught, learned, edited — one execution path ────────────────────

// Once authorized, every kind of play converges on the ordinary runtime.
//
// The engine does not care where a play came from. What differs is only whether it was asked
// about first, which is a decision made before any of this.
func TestEveryKindOfPlayConvergesOnTheOrdinaryRuntime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      routes.Kind
		edit      bool
		expectAsk bool
		reason    string
	}{
		{"authored", routes.KindAuthored, false, false, orchestrator.ReasonOrdinary},
		{"taught", routes.KindTaught, false, false, orchestrator.ReasonOrdinary},
		{"learned", routes.KindLearned, false, true, orchestrator.ReasonLearnedFirstUse},
		{"learned then edited", routes.KindLearned, true, false,
			orchestrator.ReasonEditedSinceLearned},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg, rt := world(t, tc.kind, aPlay)
			if tc.edit {
				edited := strings.Replace(aPlay, `log "done".`, `log "muted".`, 1)
				if err := os.WriteFile(reg.Path(rt), []byte(edited), 0o644); err != nil {
					t.Fatalf("editing: %v", err)
				}
			}
			d, host, out := deps(t, reg, "y", true)
			_, decision := door(t, d, "volume")
			if !decision.Allow() {
				t.Fatalf("a %s play was not allowed: %+v", tc.name, decision)
			}
			// The REASON is checked, not only the verdict. All four are allowed, and a
			// policy that allowed them for the wrong reason would pass a verdict-only
			// test while having stopped telling the four kinds apart.
			if decision.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", decision.Reason, tc.reason)
			}
			runSavedPlay(t, d, "volume")

			// The same two calls, whatever the play's history.
			want := []string{`OS's Navigate with down`, `OS's Navigate with confirm`}
			if len(host.calls) != len(want) {
				t.Fatalf("the host was asked for %v, want %v", host.calls, want)
			}
			for i := range want {
				if host.calls[i] != want[i] {
					t.Fatalf("call %d is %q", i+1, host.calls[i])
				}
			}
			asked := strings.Contains(out.String(), "Run it now?")
			if asked != tc.expectAsk {
				t.Errorf("asked = %v, want %v (output %q)", asked, tc.expectAsk, out.String())
			}
		})
	}
}

// ── no authority laundering ───────────────────────────────────────────────────

// A copied origin record does not manufacture a learned play.
//
// The digest is of the SOURCE. Dropping somebody else's provenance beside a different file
// describes a play that does not exist there, and it reads as `edited` — which gets the ordinary
// policy, not the learned one.
func TestACopiedOriginRecordManufacturesNoTrust(t *testing.T) {
	learnedReg, learnedRt := world(t, routes.KindLearned, aPlay)
	origin, err := os.ReadFile(learnedReg.OriginPath(learnedRt))
	if err != nil {
		t.Fatalf("reading provenance: %v", err)
	}

	// Somewhere else entirely: a different play, with that record dropped beside it.
	other := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	different := strings.Replace(aPlay, `"down"`, `"back"`, 1)
	if err := other.Save(rt, different); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(other.OriginPath(rt)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(other.OriginPath(rt), origin, 0o644); err != nil {
		t.Fatalf("copying: %v", err)
	}

	r := orchestrator.Classify(other, rt, "volume")
	if r.Learned() {
		t.Fatal("a copied origin record made an unrelated file a learned play")
	}
	if r.Provenance != routes.OriginEdited {
		t.Errorf("provenance reads %q", r.Provenance)
	}
}

// Nothing in the authority path can perform anything by itself.
func TestTheAuthoritySeamPerformsNothing(t *testing.T) {
	reg, rt := world(t, routes.KindLearned, aPlay)

	// Classify, authorize, and refuse — all without a host anywhere in sight.
	r := orchestrator.Classify(reg, rt, "volume")
	if d := orchestrator.Authorize(r, nil); d.Allow() {
		t.Fatal("a learned play was allowed with no way to ask")
	}
	if d := orchestrator.Authorize(r, allowing{}); !d.Allow() {
		t.Fatal("a yes was not honoured")
	}
	// The decision is data. There is nothing on it to run.
	d := orchestrator.Authorize(r, allowing{})
	if d.Sentence != "" && strings.Contains(strings.ToLower(d.Sentence), "running") {
		t.Errorf("the decision claims something ran: %q", d.Sentence)
	}
}

type allowing struct{}

func (allowing) Allow(orchestrator.Resolved) orchestrator.Decision {
	return orchestrator.Decision{Verdict: orchestrator.Allowed}
}

// Permission is per invocation and is not written down anywhere.
func TestPermissionIsNotRemembered(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)

	// Yes once, and it ran.
	d, host, _ := deps(t, reg, "y", true)
	if _, decision := door(t, d, "volume"); !decision.Allow() {
		t.Fatalf("the first invocation was not allowed: %+v", decision)
	}
	runSavedPlay(t, d, "volume")
	if len(host.calls) == 0 {
		t.Fatal("the first invocation did not run")
	}

	// A fresh orchestrator over the SAME tree, and a no this time. Nothing on disk
	// remembers the earlier yes.
	again, host2, out := deps(t, reg, "n", true)
	if _, decision := door(t, again, "volume"); decision.Allow() {
		t.Fatal("a previous yes authorised a later invocation")
	}
	if len(host2.calls) != 0 {
		t.Fatalf("a declined invocation performed %v", host2.calls)
	}
	if !strings.Contains(out.String(), "Run it now?") {
		t.Errorf("the second invocation was not asked about:\n%s", out.String())
	}

	// And nothing durable claims permission.
	var all strings.Builder
	_ = filepath.Walk(reg.Dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			data, _ := os.ReadFile(p)
			all.Write(data)
		}
		return nil
	})
	for _, word := range []string{"authorized", "authorised", "trusted", "\"safe\"", "allowed"} {
		if strings.Contains(strings.ToLower(all.String()), word) {
			t.Errorf("something durable claims %q", word)
		}
	}
}
