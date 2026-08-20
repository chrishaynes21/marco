package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// ============================================================================
// AUDIT REGRESSION — the invocation-authority door (ADR-029 / Roadmap 26) must
// be on the PRODUCTION `marco do` path, not only on orchestrator.Deps.Do.
//
// `marco do "<phrase>"` runs runAssistantDo → dispatchDo. dispatchDo resolves a
// route and runs it; it must consult Classify/Authorize exactly as Deps.Do does,
// so a learned play the user declines sends zero effects. Before the fix, the
// resolved-route path ran directly (withPanicStop → runRoute →
// driver.RunSourceWithHostsCtx) and never reached the door — every authority
// test entered through Deps.Do, which is why the gap was invisible.
//
// The recognisingStage below simulates a recogniser-equipped Marco so the in-play
// `Screen's Showing` guard PASSES and the ONLY thing that can stop the play is
// the door — isolating the authority layer from the entry guard.
// ============================================================================

// recordHost records the calls asked of it and always succeeds.
type recordHost struct{ calls []string }

func (h *recordHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	call := c.Act + "'s " + c.Action
	if in := c.Input.AsText(); in != "" {
		call += " with " + in
	}
	h.calls = append(h.calls, call)
	return "ok", runtime.Absent(), nil
}

func (h *recordHost) pressed() []string {
	var out []string
	for _, c := range h.calls {
		if strings.HasPrefix(c, "OS's") {
			out = append(out, c)
		}
	}
	return out
}

// recognisingStage simulates a recogniser-equipped Marco: it knows the named screen and reports
// the CURRENT screen IS that screen — so the in-play `Screen's Showing` guard PASSES and the only
// thing that could stop the play is the authority door.
type recognisingStage struct{ app, current string }

func (s *recognisingStage) Application() string { return s.app }
func (s *recognisingStage) SubjectNamed(app, name string) (string, bool) {
	if strings.EqualFold(app, s.app) && strings.EqualFold(name, "the pause menu") {
		return "subj_a", true
	}
	return "", false
}
func (s *recognisingStage) CurrentSubject(string) (string, screenhost.Outcome) {
	return s.current, screenhost.Recognised
}

// declineGate refuses every learned play, unconditionally.
type declineGate struct{}

func (declineGate) Allow(orchestrator.Resolved) orchestrator.Decision {
	return orchestrator.Decision{Verdict: orchestrator.Declined,
		Reason: orchestrator.ReasonUserDeclined, Sentence: "No."}
}

func registerGuardedLearnedPlay(t *testing.T) (orchestrator.Deps, *recordHost) {
	t.Helper()
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "controller settings",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := reg.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: "testgame",
		From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if r := orchestrator.Classify(reg, rt, "volume"); !r.Learned() {
		t.Fatalf("fixture is not a learned play (kind=%s prov=%s)", r.Kind, r.Provenance)
	}
	host := &recordHost{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(&recognisingStage{app: "testgame", current: "subj_a"})},
		In:    strings.NewReader("n\n"),
		Out:   &strings.Builder{},
		App:   func() string { return "testgame" },
	}
	d.Authority = declineGate{}
	return d, host
}

// askingWorld is registerGuardedLearnedPlay wired with the REAL production gate (AskFirst) and a
// scripted stdin, so a test can drive the actual confirmation the user sees on `marco do`.
func askingWorld(t *testing.T, input string) (orchestrator.Deps, *recordHost) {
	t.Helper()
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "controller settings",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := reg.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: "testgame",
		From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	host := &recordHost{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(&recognisingStage{app: "testgame", current: "subj_a"})},
		In:    strings.NewReader(input),
		Out:   &strings.Builder{},
		App:   func() string { return "testgame" },
	}
	d.Authority = orchestrator.AskFirst{Deps: d} // the real production door
	return d, host
}

// CONTROL: the door works WHEN REACHED. d.Do declines and sends nothing. Proves the gate is
// correct, so the bypass below is a wiring gap and not a broken gate.
func TestDoorDeclinesWhenReached(t *testing.T) {
	d, host := registerGuardedLearnedPlay(t)
	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if got := host.pressed(); len(got) != 0 {
		t.Fatalf("the door was reached and declined, yet keys were pressed: %v", got)
	}
}

// REGRESSION: the shipped dispatcher must honour the door. A declined learned play, run through
// dispatchDo (what `marco do` actually calls), sends zero effects.
func TestShippedDispatcherHonoursTheAuthorityDoor(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1") // don't install global hooks in a test
	d, host := registerGuardedLearnedPlay(t)

	if _, err := doAsProduct(t, d, "volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}

	if got := host.pressed(); len(got) != 0 {
		t.Fatalf("AUTHORITY BYPASS: `marco do` ran a DECLINED learned play and pressed %v; "+
			"the resolved-route path did not reach Authorize/AskFirst", got)
	}
}

// EOF/NO-ANSWER MUST NOT AUTHORIZE. A learned play may execute only on an explicit yes.
//
// The invariant: a learned play must never run because nobody answered. Silence (EOF), an empty
// line, an explicit no, and garbage all send zero effects; only "y"/"yes" runs it. Driven through
// the real production door (AskFirst via dispatchDo).
func TestNoExplicitYesDoesNotAuthoriseALearnedPlay(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	for _, tc := range []struct {
		name  string
		input string
		run   bool
	}{
		{"EOF — nobody answered", "", false},
		{"empty line — just Enter", "\n", false},
		{"explicit no", "n\n", false},
		{"garbage", "maybe later\n", false},
		{"explicit yes", "y\n", true},
		{"explicit yes word", "yes\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, host := askingWorld(t, tc.input)
			// An AUTHORIZED learned play is now performed by the Director rather than run
			// here (see performbridge_test.go), so "it ran" is the fake endpoint being
			// asked — not keys pressed locally. Both are counted: the invariant under test
			// is that NOTHING performs the play without an explicit yes, and it must not
			// quietly stop being checked because the performer moved.
			director := useFakeDirector(t, &fakeDirector{view: arrived(1)})
			if _, err := doAsProduct(t, d, "volume", nil, nil); err != nil {
				t.Fatalf("marco do: %v", err)
			}
			ran := len(host.pressed()) > 0 || len(director.asked) > 0
			if ran != tc.run {
				if tc.run {
					t.Fatalf("explicit yes did not perform the play (pressed %v, asked %d)",
						host.pressed(), len(director.asked))
				}
				t.Fatalf("a learned play was PERFORMED without an explicit yes (%q) — "+
					"pressed %v, asked the Director %d time(s); no answer must never "+
					"authorize", tc.input, host.pressed(), len(director.asked))
			}
		})
	}
}

// And an ordinary (authored/taught) play is unaffected: it runs without a prompt, exactly as
// before, because Authorize returns Allowed for it. This proves the fix does not gate the common
// case — only a learned play with intact provenance is stopped at the door.
func TestShippedDispatcherStillRunsOrdinaryPlays(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "controller settings",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	// Saved with NO provenance sidecar → an ordinary authored play. Authored plays are Allowed
	// with no question, so a declining gate must not touch them.
	if err := reg.Save(rt, src); err != nil {
		t.Fatalf("saving: %v", err)
	}
	host := &recordHost{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(&recognisingStage{app: "testgame", current: "subj_a"})},
		In:    strings.NewReader(""), // no answer available; an ordinary play must not need one
		Out:   &strings.Builder{},
		App:   func() string { return "testgame" },
	}
	d.Authority = declineGate{} // would decline a LEARNED play; must be irrelevant here
	if _, err := doAsProduct(t, d, "volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}
	if got := host.pressed(); len(got) != 2 {
		t.Fatalf("an ordinary authored play was gated by the door: pressed %v, want 2 calls", got)
	}
}
