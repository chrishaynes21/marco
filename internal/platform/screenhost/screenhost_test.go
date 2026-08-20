package screenhost_test

import (
	"go/build"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Does this play belong on this screen?
//
// One question, and every way of not being able to answer it is a refusal. Silence is never yes.

// world is a scripted, read-only view.
type world struct {
	app     string
	names   map[string]string // name → subject id
	current string
	outcome screenhost.Outcome
}

func (w *world) Application() string { return w.app }

func (w *world) SubjectNamed(application, name string) (string, bool) {
	if !strings.EqualFold(application, w.app) {
		// The store scopes by application; a fixture that ignored that would prove nothing
		// about cross-application safety.
		return "", false
	}
	id, ok := w.names[strings.ToLower(name)]
	return id, ok
}

func (w *world) CurrentSubject(string) (string, screenhost.Outcome) {
	return w.current, w.outcome
}

func ask(h *screenhost.Host, name string) string {
	status, _, _ := h.Invoke(runtime.HostCall{
		Act: "Screen", Action: "Showing", Input: runtime.Text(name),
	})
	return status
}

// ── the one case that answers ok ──────────────────────────────────────────────

func TestTheNamedScreenInFrontAnswersOk(t *testing.T) {
	h := screenhost.New(&world{
		app: "testgame", names: map[string]string{"the pause menu": "subj_a"},
		current: "subj_a", outcome: screenhost.Recognised,
	})
	if got := ask(h, "the pause menu"); got != "ok" {
		t.Fatalf("status = %q (%s)", got, h.Why())
	}
}

// ── every other case refuses ──────────────────────────────────────────────────

func TestEveryWayOfNotKnowingRefuses(t *testing.T) {
	named := map[string]string{"the pause menu": "subj_a"}
	for _, tc := range []struct {
		name  string
		world *world
		ask   string
		// why is a fragment of the diagnostic, which must keep "I could not look" apart
		// from "I looked and it was different".
		why string
	}{{
		name: "a different screen",
		world: &world{app: "testgame", names: named, current: "subj_b",
			outcome: screenhost.Recognised},
		ask: "the pause menu", why: "different screen",
	}, {
		name:  "a screen that could be several",
		world: &world{app: "testgame", names: named, outcome: screenhost.Ambiguous},
		ask:   "the pause menu", why: "more than one",
	}, {
		name:  "a screen nobody can see",
		world: &world{app: "testgame", names: named, outcome: screenhost.Unobservable},
		ask:   "the pause menu", why: "could not see",
	}, {
		name:  "no recogniser",
		world: &world{app: "testgame", names: named, outcome: screenhost.Unavailable},
		ask:   "the pause menu", why: "could not check",
	}, {
		name:  "a screen matching nothing remembered",
		world: &world{app: "testgame", names: named, outcome: screenhost.Unknown},
		ask:   "the pause menu", why: "does not recognise",
	}, {
		name: "a name nobody has given",
		world: &world{app: "testgame", names: named, current: "subj_a",
			outcome: screenhost.Recognised},
		ask: "the inventory", why: "is called",
	}, {
		name:  "no application in front",
		world: &world{names: named, current: "subj_a", outcome: screenhost.Recognised},
		ask:   "the pause menu", why: "no application",
	}, {
		// A guard whose name resolved to nothing. The play asked, and a question with
		// no subject cannot be answered yes.
		name: "no name at all",
		world: &world{app: "testgame", names: named, current: "subj_a",
			outcome: screenhost.Recognised},
		ask: "", why: "a screen name is needed",
	}, {
		// THE NAME IS RESOLVED INSIDE THE APPLICATION IN FRONT. `the pause menu` in
		// one program is not `the pause menu` in another, and a play carrying a name
		// from somewhere else must refuse rather than find the nearest match.
		name:  "a name that means nothing in the application in front",
		world: &world{app: "othergame", current: "subj_a", outcome: screenhost.Recognised},
		ask:   "the pause menu", why: `nothing in othergame is called "the pause menu"`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			h := screenhost.New(tc.world)
			if got := ask(h, tc.ask); got != "failed" {
				t.Fatalf("status = %q; every way of not knowing is a refusal", got)
			}
			if !strings.Contains(h.Why(), tc.why) {
				t.Errorf("the diagnostic is %q, which does not say %q", h.Why(), tc.why)
			}
		})
	}
}

// A host with no recogniser at all refuses rather than assuming.
func TestAHostThatCannotLookRefuses(t *testing.T) {
	h := screenhost.New(nil)
	if got := ask(h, "the pause menu"); got != "failed" {
		t.Fatalf("status = %q; a Marco that cannot see must not guess", got)
	}
	if got := ask(h, ""); got != "failed" {
		t.Fatalf("an empty name answered %q", got)
	}
	// And an unknown capability is a refusal, never a silent ok.
	status, _, _ := h.Invoke(runtime.HostCall{Act: "Screen", Action: "Press"})
	if status != "failed" {
		t.Errorf("an unknown Screen capability answered %q", status)
	}
}

// ── authority ─────────────────────────────────────────────────────────────────

// The Screen host is perception. It cannot act.
//
// Structural, not a promise: the interface it is given has three methods and every one is a read,
// and the package it lives in reaches nothing that could press, focus, run or write.
func TestTheScreenHostCannotAct(t *testing.T) {
	rt := reflect.TypeOf((*screenhost.Recognition)(nil)).Elem()
	if rt.NumMethod() != 3 {
		t.Errorf("the world a Screen host sees has %d methods; it should be able to ask "+
			"which application, which screen, and what a name refers to — and nothing else",
			rt.NumMethod())
	}
	for _, forbidden := range []string{"Press", "Key", "Click", "Type", "Activate", "Focus",
		"Run", "Execute", "Invoke", "Remember", "Save", "Register", "Grant"} {
		if _, has := rt.MethodByName(forbidden); has {
			t.Errorf("a Screen host can %s", forbidden)
		}
	}

	const pkg = "github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	reachable := map[string]bool{}
	walk(pkg, reachable, 0)
	for _, f := range []struct{ frag, why string }{
		{"internal/oshost", "keyboard, mouse, clipboard"},
		{"internal/driver", "drives input"},
		{"internal/winctx", "window activation and focus"},
		{"internal/recorder", "installs input hooks"},
		{"internal/director/rehearse", "rehearsal grants"},
		{"internal/director/execute", "the execution pipeline"},
		{"internal/orchestrator", "route invocation"},
		{"os/exec", "starting processes"},
	} {
		for path := range reachable {
			if strings.Contains(path, f.frag) {
				t.Errorf("the Screen host reaches %s (%s); asking where you are must not "+
					"be a way of doing something", path, f.why)
			}
		}
	}
}

func walk(path string, seen map[string]bool, depth int) {
	if depth > 10 || seen[path] {
		return
	}
	seen[path] = true
	p, err := build.Import(path, ".", 0)
	if err != nil {
		return
	}
	for _, imp := range p.Imports {
		walk(imp, seen, depth+1)
	}
}
