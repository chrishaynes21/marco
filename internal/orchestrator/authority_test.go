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
// Until this milestone there was no seam between those two: `Do` resolved a phrase and called
// `Run` in the same breath. These tests hold the door open and look through it.

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

// ── THE headline: the Marco Moment, dry ───────────────────────────────────────

// A learned play is resolved, authorized, compiled by the ordinary compiler, and would emit
// exactly what the `.marco` says.
//
// # What this proves and what it does not
//
// It proves the whole chain from a natural request to the last thing before a computer, with the
// ONE substitution being which Host was installed. It does not prove anything happened: the host
// is a notebook, so the honest name for this is a dry path, not a performance.
//
// The operations come from the FILE. Nothing here reads a ProcedureCandidate, a rehearsal record
// or any Director evidence — the play is the only input, which is the whole point of having
// written it down.
func TestTheMarcoMomentDryPath(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, out := deps(t, reg, "y", true)

	// A fresh orchestrator over a directory path: no session, no proposal, no screen state,
	// no window generation, no process id. Just the tree.
	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
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

	// And then the ORDINARY runtime performed the play into a notebook.
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
// The seam, held open. `Resolve` answers WHICH play and can be inspected, logged and refused
// without a host being asked for anything.
func TestResolvingAPlayPerformsNothing(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, _ := deps(t, reg, "y", true)

	r, ok := d.Resolve("volume")
	if !ok {
		t.Fatal("the play was not resolved")
	}
	if !r.Learned() {
		t.Fatalf("resolved as %s/%s", r.Kind, r.Provenance)
	}
	if len(host.calls) != 0 {
		t.Fatalf("resolving performed %v", host.calls)
	}
	// And there is nothing on the answer that could perform it.
	if r.Route.Slug != "volume" {
		t.Errorf("resolved to %+v", r.Route)
	}
}

// Saying no leaves the play alone, and is not the same as not finding it.
func TestDecliningIsNotTheSameAsNotFinding(t *testing.T) {
	reg, _ := world(t, routes.KindLearned, aPlay)
	d, host, out := deps(t, reg, "n", true)

	if err := d.Do("volume"); err != nil {
		t.Fatalf("declining is not an error: %v", err)
	}
	if len(host.calls) != 0 {
		t.Fatalf("a declined play performed %v", host.calls)
	}
	// The user is not told the play is missing, and not told it is dangerous.
	said := strings.ToLower(out.String())
	for _, wrong := range []string{"don't know", "do not know", "unsafe", "refus"} {
		if strings.Contains(said, wrong) {
			t.Errorf("declining was reported as %q:\n%s", wrong, out.String())
		}
	}
	// The decision itself keeps the two apart.
	r, _ := d.Resolve("volume")
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

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
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
	}{
		{"authored", routes.KindAuthored, false, false},
		{"taught", routes.KindTaught, false, false},
		{"learned", routes.KindLearned, false, true},
		{"learned then edited", routes.KindLearned, true, false},
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
			if err := d.Do("volume"); err != nil {
				t.Fatalf("do: %v", err)
			}

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
	host := &recordingHost{}
	_ = host

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

	// Yes once.
	d, host, _ := deps(t, reg, "y", true)
	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if len(host.calls) == 0 {
		t.Fatal("the first invocation did not run")
	}

	// A fresh orchestrator over the SAME tree, and a no this time. Nothing on disk
	// remembers the earlier yes.
	again, host2, out := deps(t, reg, "n", true)
	if err := again.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if len(host2.calls) != 0 {
		t.Fatalf("a previous yes authorised a later invocation: %v", host2.calls)
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
