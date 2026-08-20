package routes

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func newReg(t *testing.T) Registry { return Registry{Dir: t.TempDir()} }

func ctxRoute(app, name string) Route   { return Route{App: app, Slug: Slug(name)} }
func focusRoute(app, name string) Route { return Route{App: app, Focus: true, Slug: Slug(name)} }
func globalRoute(name string) Route     { return Route{Slug: Slug(name)} }

func TestScopePreference(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save(ctxRoute("sea-of-thieves", "switch to sword"), "a"))
	must(t, reg.Save(ctxRoute("forza", "switch to sword"), "b")) // same phrase, different app
	must(t, reg.Save(globalRoute("login to facebook"), "c"))     // global

	// The focused app's context route wins.
	if rt, ok := reg.Resolve("sea-of-thieves", "switch to sword"); !ok || rt.App != "sea-of-thieves" || rt.Focus {
		t.Fatalf("current-app resolve = %+v %v", rt, ok)
	}
	// A global route resolves regardless of focus.
	if rt, ok := reg.Resolve("forza", "login to facebook"); !ok || rt.App != "" {
		t.Fatalf("global resolve = %+v %v", rt, ok)
	}
	// A context route is foreground-only: it does NOT resolve from another app — that's
	// what lets the same phrase mean different things per app.
	if rt, ok := reg.Resolve("chrome", "switch to sword"); ok {
		t.Fatalf("context route should not resolve from another app, got %+v", rt)
	}
	if _, ok := reg.Resolve("forza", "make a sandwich"); ok {
		t.Fatal("unknown command resolved")
	}
}

// TestFocusResolvesAnywhere: a focus route (routes/<app>/focus) resolves from ANOTHER
// app — it's the "reach this app from anywhere" command (it activates the app).
func TestFocusResolvesAnywhere(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save(focusRoute("discord", "mute"), "a"))
	rt, ok := reg.Resolve("rocketleague", "mute")
	if !ok || rt.App != "discord" || !rt.Focus {
		t.Fatalf("focus resolve from another app = %+v %v", rt, ok)
	}
	if got := reg.Path(rt); filepath.Base(filepath.Dir(got)) != FocusDir {
		t.Fatalf("focus route path = %s, want under %s/", got, FocusDir)
	}
}

// TestContextBeatsFocusInApp: in the app itself, a context route outranks a focus
// route of the same name (you're already here — run the in-place one).
func TestContextBeatsFocusInApp(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save(ctxRoute("discord", "go"), "ctx"))
	must(t, reg.Save(focusRoute("discord", "go"), "focus"))
	if rt, ok := reg.Resolve("discord", "go"); !ok || rt.Focus {
		t.Fatalf("context should beat focus in-app, got %+v", rt)
	}
}

// TestFolderLayout: each scope lands in its folder.
func TestFolderLayout(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save(ctxRoute("forza", "drift"), "x"))
	must(t, reg.Save(focusRoute("forza", "open"), "y"))
	must(t, reg.Save(globalRoute("hello"), "z"))
	for _, p := range []string{
		filepath.Join(reg.Dir, "forza", ContextDir, "drift.marco"),
		filepath.Join(reg.Dir, "forza", FocusDir, "open.marco"),
		filepath.Join(reg.Dir, GlobalDir, "hello.marco"),
	} {
		if !fileExists(p) {
			t.Errorf("expected %s to exist", p)
		}
	}
}

// TestLegacyLooseContext: a route taught before the context/ split (loose
// routes/<app>/<slug>.marco) still reads, resolves, and stays put as context.
func TestLegacyLooseContext(t *testing.T) {
	reg := newReg(t)
	must(t, os.MkdirAll(filepath.Join(reg.Dir, "rocketleague"), 0o755))
	must(t, os.WriteFile(filepath.Join(reg.Dir, "rocketleague", "to-menu.marco"), []byte("x"), 0o644))
	rt, ok := reg.Resolve("rocketleague", "to menu")
	if !ok || rt.App != "rocketleague" || rt.Focus {
		t.Fatalf("legacy loose resolve = %+v %v", rt, ok)
	}
	if got := reg.Path(rt); got != filepath.Join(reg.Dir, "rocketleague", "to-menu.marco") {
		t.Fatalf("legacy path = %s, want the loose location", got)
	}
}

func TestSlugsAndList(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save(ctxRoute("sea-of-thieves", "switch to sword"), "a"))
	must(t, reg.Save(ctxRoute("forza", "switch to sword"), "b"))
	must(t, reg.Save(globalRoute("login to facebook"), "c"))

	slugs := reg.Slugs()
	sort.Strings(slugs)
	if len(slugs) != 2 || slugs[0] != "login-to-facebook" || slugs[1] != "switch-to-sword" {
		t.Fatalf("Slugs = %v", slugs)
	}
	if len(reg.List()) != 3 {
		t.Fatalf("List = %v", reg.List())
	}
}

func TestHasDelete(t *testing.T) {
	reg := newReg(t)
	rt := Route{App: "forza", Slug: "drift"}
	if reg.Has(rt) {
		t.Fatal("Has true before save")
	}
	must(t, reg.Save(rt, "x"))
	if !reg.Has(rt) {
		t.Fatal("Has false after save")
	}
	must(t, reg.Delete(rt))
	if reg.Has(rt) {
		t.Fatal("Has true after delete")
	}
	must(t, reg.Delete(rt)) // idempotent
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ── one copy, always ──────────────────────────────────────────────────────────

// A refused registration leaves the staged play and nothing else.
//
// Two directories hold one play. If a refusal could leave a copy in both, the same play would
// answer two different questions — and the retry would refuse on a collision with itself, which
// is a dead end nobody can get out of.
//
// The refusal here is the ordinary one: the slug is already taken by an authored play.
func TestARefusedRegistrationLeavesOneCopy(t *testing.T) {
	reg := newReg(t)
	rt := Route{App: "forza", Slug: "drift"}

	// Somebody's own play is already called that.
	must(t, reg.Save(rt, "-- theirs\n"))
	must(t, reg.SaveStaged(rt, "-- learned\n", Origin{Kind: KindLearned, Application: "forza"}))

	if err := reg.Register(rt); err == nil {
		t.Fatal("registering over an existing play was allowed")
	}
	// The refusal says what to do about it, because a name collision a person cannot see is
	// indistinguishable from the play having been lost.
	// (The message itself is asserted where it is produced; what matters here is the disk.)

	// The staged copy is intact and still the learned one.
	src, ok := reg.StagedSource(rt)
	if !ok || src != "-- learned\n" {
		t.Fatalf("the staged play did not survive the refusal: %q %v", src, ok)
	}
	// And the registered scope still holds THEIRS, untouched.
	got, err := os.ReadFile(reg.Path(rt))
	if err != nil || string(got) != "-- theirs\n" {
		t.Fatalf("the authored play was disturbed: %q %v", got, err)
	}
}

// A successful registration leaves the staged copy behind it.
func TestRegisteringMovesThePlayRatherThanCopyingIt(t *testing.T) {
	reg := newReg(t)
	rt := Route{App: "forza", Slug: "drift"}
	must(t, reg.SaveStaged(rt, "-- learned\n", Origin{Kind: KindLearned, Application: "forza"}))
	must(t, reg.Register(rt))

	if _, ok := reg.StagedSource(rt); ok {
		t.Error("the play is registered AND staged; one play, one place")
	}
	if _, err := os.Stat(reg.StagedPath(rt)); !os.IsNotExist(err) {
		t.Errorf("the staged file is still there: %v", err)
	}
	if !reg.Has(rt) {
		t.Error("the play did not arrive in the scope the resolver reads")
	}
}

// Forgetting reaches the staged copy, which nothing else can.
//
// A play whose registration was refused sits in `<app>/learned/`, where discovery does not scan.
// Before this, `Unregister` removed only the registered scope — so that copy could not be removed
// by any command a person has. Unreachable and unforgettable at the same time.
func TestForgettingRemovesAStagedPlayNothingCouldReach(t *testing.T) {
	reg := newReg(t)
	rt := Route{App: "forza", Slug: "drift"}
	must(t, reg.SaveStaged(rt, "-- learned\n", Origin{Kind: KindLearned, Application: "forza"}))

	must(t, reg.Unregister(rt))

	if _, ok := reg.StagedSource(rt); ok {
		t.Fatal("forgetting left the staged play on disk, where nothing can reach it again")
	}
	if _, state := reg.StagedOrigin(rt); state != OriginNone {
		t.Errorf("staged provenance is %q, want it gone with the play it described", state)
	}
	must(t, reg.Unregister(rt)) // idempotent
}

// A registration that half-lands is undone.
//
// `SaveWithOrigin` writes the source and then the provenance. If the second write fails, the
// source alone is sitting in the scope the resolver reads — an ordinary-looking play Director
// wrote, which nobody chose to register — WHILE the staged copy is still in `learned/`. One play,
// two places, and the retry then refuses on a collision with itself.
//
// The failure is forced the only honest way available: a directory standing where the provenance
// file has to go.
//
// Deleting the rollback must fail this.
func TestAPartlyWrittenRegistrationIsUndone(t *testing.T) {
	reg := newReg(t)
	rt := Route{App: "forza", Slug: "drift"}
	must(t, reg.SaveStaged(rt, "-- learned\n", Origin{Kind: KindLearned, Application: "forza"}))

	// The provenance cannot be written, and the source can.
	if err := os.MkdirAll(reg.OriginPath(rt), 0o755); err != nil {
		t.Fatalf("blocking the provenance path: %v", err)
	}

	if err := reg.Register(rt); err == nil {
		t.Fatal("a registration that could not write provenance reported success")
	}

	if reg.Has(rt) {
		t.Error("the source was left in the resolver's scope without its provenance, next " +
			"to the staged copy it came from")
	}
	src, ok := reg.StagedSource(rt)
	if !ok || src != "-- learned\n" {
		t.Fatalf("the staged play did not survive the failure: %q %v — there is now no "+
			"copy of it anywhere", src, ok)
	}
}
