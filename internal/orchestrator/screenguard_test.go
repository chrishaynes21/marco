package orchestrator_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// The Marco Moment, and the wrong screen.
//
// Two gates, and neither substitutes for the other:
//
//	MAY I RUN THIS PLAY?          Roadmap 26 — the user is asked
//	DOES IT BELONG ON THIS SCREEN? Roadmap 28 — the play asks, before its first effect
//
// A yes to the first is not an answer to the second. That is the whole of this file.

// stage is a scripted, read-only world for the Screen host.
type stage struct {
	app     string
	names   map[string]string
	current string
	outcome screenhost.Outcome
}

func (s *stage) Application() string { return s.app }

// SubjectNamed answers only for testgame, whatever is in front.
//
// Keyed to a FIXED application rather than to whatever the stage reports, so "another program is
// in front" is a real miss rather than a fixture that moves its own goalposts.
func (s *stage) SubjectNamed(application, name string) (string, bool) {
	if !strings.EqualFold(application, "testgame") {
		return "", false
	}
	id, ok := s.names[strings.ToLower(name)]
	return id, ok
}

func (s *stage) CurrentSubject(string) (string, screenhost.Outcome) {
	return s.current, s.outcome
}

// guardedWorld saves a guarded learned play and returns an orchestrator over the given stage.
//
// The play is LOWERED by the production generator, not hand-written: what runs is the artifact
// Director would actually write, guard and all.
func guardedWorld(t *testing.T, sc *stage, answer string) (
	orchestrator.Deps, *recordingHost) {

	t.Helper()
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if !strings.Contains(src, `do Screen's Showing with "the pause menu"...`) {
		t.Fatalf("the generated play carries no entry condition:\n%s", src)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := reg.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: "testgame",
		From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}

	host := &recordingHost{}
	d := orchestrator.Deps{
		Reg: reg,
		// The ordinary host map, with the Screen act wired exactly as the composition root
		// wires it: the runtime dispatches `Screen's Showing` like any other capability.
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(sc)},
		In:    strings.NewReader(answer + "\n"),
		Out:   &strings.Builder{},
		App:   func() string { return "testgame" },
	}
	d.Authority = orchestrator.AskFirst{Deps: d}
	return d, host
}

// ── THE headline: the wrong screen sends nothing ──────────────────────────────

// Authorized, resolved, compiled — and refused before the first key, because it is the wrong screen.
//
// This is the exact hole Roadmap 26 found. The user asked for the play, the user said yes to
// running it, and Marco still does not press anything, because the play itself says where it
// begins and it does not begin here.
//
// The "said yes" half is spelled out rather than implied. This test used to enter through
// `Deps.Do`, which asked the door on the way past; `Deps.Do` was retired in Phase 3 (see
// runsaved_test.go), so the yes is now taken EXPLICITLY, from the same two calls production makes
// in `cmd/marco/intake.go`'s `performOnePlay`: `Classify`, then `Authorize`. That ordering is the
// whole claim of this file — a yes to "may I run this play?" is not an answer to "does it belong
// on this screen?", and losing the first call would quietly turn this into a test of the guard
// alone.
//
// Deleting the Screen host from the composition, or the guard from lowering, must fail this.
func TestTheWrongScreenSendsNothing(t *testing.T) {
	d, host := guardedWorld(t, &stage{
		app: "testgame", names: map[string]string{"the pause menu": "subj_a"},
		current: "subj_b", outcome: screenhost.Recognised, // a DIFFERENT screen
	}, "y")

	// GATE ONE: the person is asked, and says yes.
	rt, ok := d.Reg.Resolve(d.App(), "volume")
	if !ok {
		t.Fatal("the play did not resolve")
	}
	decision := orchestrator.Authorize(orchestrator.Classify(d.Reg, rt, "volume"), d.Authority)
	if !decision.Allow() {
		t.Fatalf("the door refused before the screen guard could be tested: %+v", decision)
	}

	// GATE TWO: and the play still refuses to begin.
	runSavedPlay(t, d, "volume")
	for _, c := range host.calls {
		if strings.HasPrefix(c, "OS's") {
			t.Fatalf("a key was pressed on the wrong screen: %v", host.calls)
		}
	}
}

// ── the correct screen runs, from the compiled file ───────────────────────────

func TestTheRightScreenRunsThePlayFromItsOwnSource(t *testing.T) {
	d, host := guardedWorld(t, &stage{
		app: "testgame", names: map[string]string{"the pause menu": "subj_a"},
		current: "subj_a", outcome: screenhost.Recognised,
	}, "y")

	runSavedPlay(t, d, "volume")
	var pressed []string
	for _, c := range host.calls {
		if strings.HasPrefix(c, "OS's") {
			pressed = append(pressed, c)
		}
	}
	want := []string{`OS's Navigate with down`, `OS's Navigate with confirm`}
	if len(pressed) != len(want) {
		t.Fatalf("the host was asked for %v, want %v", pressed, want)
	}
	for i := range want {
		if pressed[i] != want[i] {
			t.Fatalf("call %d is %q, want %q", i+1, pressed[i], want[i])
		}
	}
	// The Screen question went to the SCREEN host, which is a different host and records
	// nothing here. The proof that it was asked at all is the wrong-screen case above, where
	// these same two calls do not happen.
}

// ── every other answer is zero effects ────────────────────────────────────────

func TestOnlyAPositiveMatchLetsThePlayBegin(t *testing.T) {
	named := map[string]string{"the pause menu": "subj_a"}
	for _, tc := range []struct {
		name  string
		stage *stage
	}{
		{"a different screen", &stage{app: "testgame", names: named,
			current: "subj_b", outcome: screenhost.Recognised}},
		{"an ambiguous screen", &stage{app: "testgame", names: named,
			outcome: screenhost.Ambiguous}},
		{"a screen nobody can see", &stage{app: "testgame", names: named,
			outcome: screenhost.Unobservable}},
		{"no recogniser", &stage{app: "testgame", names: named,
			outcome: screenhost.Unavailable}},
		{"a name nobody has given", &stage{app: "testgame", names: map[string]string{},
			current: "subj_a", outcome: screenhost.Recognised}},
		{"another application in front", &stage{app: "someothergame", names: named,
			current: "subj_a", outcome: screenhost.Recognised}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, host := guardedWorld(t, tc.stage, "y")
			runSavedPlay(t, d, "volume")
			for _, c := range host.calls {
				if strings.HasPrefix(c, "OS's") {
					t.Fatalf("%s let the play begin: %v", tc.name, host.calls)
				}
			}
		})
	}
}

// ── the sidecar enforces nothing ──────────────────────────────────────────────

// Removing the guard from the source removes the guard.
//
// The essential adversarial case. Provenance may record where a play came from; it must never
// carry executable meaning the source omits. A user who deletes the entry condition owns a play
// with no entry condition — and Roadmap 25/26 already say what that means: it is theirs now.
func TestRemovingTheGuardFromTheSourceRemovesTheGuard(t *testing.T) {
	d, host := guardedWorld(t, &stage{
		app: "testgame", names: map[string]string{"the pause menu": "subj_a"},
		current: "subj_b", outcome: screenhost.Recognised, // the WRONG screen
	}, "y")

	// The user edits the guard out.
	rt := routes.Route{App: "testgame", Slug: "volume"}
	unguarded, err := marcoexec.LowerNamedPlay("Volume", "Mute",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if strings.Contains(unguarded, "Screen's Showing") {
		t.Fatal("the fixture still has a guard")
	}
	if err := d.Reg.Save(rt, unguarded); err != nil {
		t.Fatalf("saving: %v", err)
	}

	runSavedPlay(t, d, "volume")
	// It runs, on the wrong screen, because the play no longer says otherwise. The sidecar
	// still exists and still remembers where it came from — and enforces nothing.
	var pressed int
	for _, c := range host.calls {
		if strings.HasPrefix(c, "OS's") {
			pressed++
		}
	}
	if pressed == 0 {
		t.Fatal("the sidecar went on enforcing a condition the source no longer states")
	}
	// And Marco stops claiming it verified this file.
	if _, state := d.Reg.Origin(rt); state.Verified() {
		t.Error("an edited play still claims to be the artifact Director verified")
	}
}
