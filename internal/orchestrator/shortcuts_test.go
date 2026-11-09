package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The shortcut owner's rules, entered through the functions every surface calls. Synthetic
// library, stated applications; nothing is pressed.

const shortcutSrc = "script main...\n  do nothing.\n"

// shortcutWorld is a library with routines in all three scopes and two applications.
func shortcutWorld(t *testing.T) Deps {
	t.Helper()
	reg := routes.Registry{Dir: t.TempDir()}
	for _, rt := range []routes.Route{
		{App: "cloudapp", Focus: true, Slug: "cloud-pull-stone"}, // From anywhere → cloudapp
		{App: "game", Focus: true, Slug: "enter-freeplay"},       // From anywhere → game
		{Slug: "copy"},                       // current foreground
		{App: "notepad", Slug: "save-draft"}, // Only in notepad
		{Slug: "save"},                       // a global namesake
		{App: "paint", Slug: "save"},         // …and an in-place one
	} {
		if err := reg.Save(rt, shortcutSrc); err != nil {
			t.Fatal(err)
		}
	}
	return Deps{Reg: reg}
}

func assign(t *testing.T, d Deps, a Assignment) routes.Binding {
	t.Helper()
	b, err := d.AssignShortcut(a)
	if err != nil {
		t.Fatalf("assigning %+v: %v", a, err)
	}
	return b
}

var (
	cloudPull = routes.Route{App: "cloudapp", Focus: true, Slug: "cloud-pull-stone"}
	freeplay  = routes.Route{App: "game", Focus: true, Slug: "enter-freeplay"}
	copyAny   = routes.Route{Slug: "copy"}
	draftOnly = routes.Route{App: "notepad", Slug: "save-draft"}
	saveAny   = routes.Route{Slug: "save"}
)

// `e in one application, `e in another: each press runs its own application's routine.
func TestTheSameKeyRunsDifferentRoutinesPerApp(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "cloudapp", Steps: []routes.Route{cloudPull}})
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	for app, want := range map[string]routes.Route{"cloudapp": cloudPull, "game": freeplay} {
		p := d.ShortcutPressed(app, "e")
		if p.Refused != "" || len(p.Steps) != 1 || p.Steps[0].Route != want {
			t.Errorf("`e in %s fires %+v (%s), want %s", app, p.Steps, p.Refused, want.ID())
		}
	}
	if p := d.ShortcutPressed("notepad", "e"); p.Found {
		t.Errorf("`e fired in notepad, where nothing was bound: %+v", p)
	}
}

// A global `c runs copy everywhere; an app's own `c hides it there, and the listing says so.
func TestAnAppKeyShadowsTheGlobalKey(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "c", ListenIn: "", Steps: []routes.Route{copyAny}})
	assign(t, d, Assignment{Key: "c", ListenIn: "game", Steps: []routes.Route{freeplay}})
	if p := d.ShortcutPressed("notepad", "c"); len(p.Steps) != 1 || p.Steps[0].Route != copyAny {
		t.Errorf("the global `c does not run copy in notepad: %+v", p)
	}
	if p := d.ShortcutPressed("game", "c"); len(p.Steps) != 1 || p.Steps[0].Route != freeplay {
		t.Errorf("game's own `c does not win in game: %+v", p)
	}
	here := d.ListShortcuts("game", true)
	if len(here) != 1 || here[0].ListenIn != "game" {
		t.Errorf("listing for game shows %+v, want only game's `c", here)
	}
	all := d.ListShortcuts("game", false)
	shadowed := false
	for _, r := range all {
		if r.ListenIn == "" && r.Key == "c" && r.Shadowed {
			shadowed = true
		}
	}
	if !shadowed {
		t.Errorf("the full listing does not mark the global `c as hidden in game: %+v", all)
	}
}

// A shortcut may be narrower than its routine, never wider.
func TestAShortcutCannotBeWiderThanItsRoutine(t *testing.T) {
	d := shortcutWorld(t)
	for _, listenIn := range []string{"", "game"} {
		_, err := d.AssignShortcut(Assignment{Key: "s", ListenIn: listenIn,
			Steps: []routes.Route{draftOnly}})
		if err == nil || !strings.Contains(err.Error(), "Only in notepad") {
			t.Errorf("an Only-in-notepad play was bound listened for %q: %v", listenIn, err)
		}
	}
	assign(t, d, Assignment{Key: "s", ListenIn: "notepad", Steps: []routes.Route{draftOnly}})
	// From anywhere → app may be listened for anywhere, or in another application.
	assign(t, d, Assignment{Key: "p", ListenIn: "game", Steps: []routes.Route{cloudPull}})
}

// A current-foreground routine (Ctrl+C) can be a global shortcut.
func TestAGlobalShortcutForACurrentForegroundRoutine(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "c", Steps: []routes.Route{copyAny}})
	for _, app := range []string{"notepad", "game", "anything"} {
		if p := d.ShortcutPressed(app, "c"); len(p.Steps) != 1 || p.Steps[0].Route != copyAny {
			t.Errorf("`c in %s: %+v", app, p)
		}
	}
}

// Taking a key that is in use needs consent to the version shown, and nothing else will do.
func TestConsentIsToTheBindingVersionShown(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay},
		Cmd: "enter freeplay with 3"})
	shown := d.ShortcutAt("game", "e")

	// No consent: refused, naming what is there.
	_, err := d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{cloudPull}})
	if err == nil || !strings.Contains(err.Error(), "already runs") {
		t.Fatalf("a taken key was overwritten without consent: %v", err)
	}

	// The binding changes WITH THE SAME ROUTINE ON IT — a different configured value.
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay},
		Cmd: "enter freeplay with 4", Consent: shown.Version})

	// Consent to the version that was shown before that change is now stale.
	_, err = d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{cloudPull}, Consent: shown.Version})
	if err == nil || !strings.Contains(err.Error(), "has changed since you were shown it") {
		t.Fatalf("consent to an older version of the binding was accepted: %v", err)
	}
	if p := d.ShortcutPressed("game", "e"); p.Steps[0].Text != "enter freeplay with 4" {
		t.Errorf("the refused replacement changed the binding: %+v", p)
	}
	// And consent to a shortcut that has gone is refused too.
	now := d.ShortcutAt("game", "e")
	if _, err := d.RemoveShortcut("game", "e", now.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{cloudPull}, Consent: now.Version}); err == nil {
		t.Error("consent to replace a shortcut that is no longer there was accepted")
	}
}

// Removal is consent-bound in the same way.
func TestRemovingAShortcutThatChangedIsRefused(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	shown := d.ShortcutAt("game", "e")
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{cloudPull},
		Consent: shown.Version})
	if _, err := d.RemoveShortcut("game", "e", shown.Version); err == nil {
		t.Fatal("a shortcut was removed on consent to a version it no longer has")
	}
}

func TestReservedKeysCannotBeBound(t *testing.T) {
	d := shortcutWorld(t)
	for _, key := range []string{"m", "h", "esc", "`"} {
		_, err := d.AssignShortcut(Assignment{Key: key, Steps: []routes.Route{copyAny},
			Leader: "`"})
		if err == nil {
			t.Errorf("`%s was bound", key)
		}
	}
}

// A shortcut set up by name before identities does not run, and says how to fix it.
func TestALegacyBindingRefusesUntilReassigned(t *testing.T) {
	d := shortcutWorld(t)
	if err := d.Reg.Bind("game", "e", "enter freeplay with 3"); err != nil { // the old shape
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(d.Reg.Dir, "bindings.json"))
	p := d.ShortcutPressed("game", "e")
	if !p.Found || p.Refused == "" || len(p.Steps) != 0 {
		t.Fatalf("a legacy binding was going to run: %+v", p)
	}
	for _, want := range []string{"set up by name", "reassign", "enter freeplay with 3"} {
		if !strings.Contains(p.Refused, want) {
			t.Errorf("the refusal does not say %q: %s", want, p.Refused)
		}
	}
	if rows := d.ListShortcuts("", false); len(rows) != 1 || rows[0].State != "needs-reassigning" {
		t.Errorf("the listing does not mark it: %+v", rows)
	}
	after, _ := os.ReadFile(filepath.Join(d.Reg.Dir, "bindings.json"))
	if string(before) != string(after) {
		t.Error("reading a legacy binding changed the file")
	}
	// Reassigning keeps its words and configured values, and names the routine.
	shown := d.ShortcutAt("game", "e")
	b := assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay},
		Cmd: shown.Binding.Command(), Consent: shown.Version})
	if b.Cmd != "enter freeplay with 3" || b.Legacy() {
		t.Errorf("reassignment lost the configured value or the identity: %+v", b)
	}
}

// A shortcut whose routine is gone refuses and names it — it never finds another of that name.
func TestAStaleShortcutRefusesAndSaysWhich(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "s", ListenIn: "game", Steps: []routes.Route{saveAny}})
	if err := d.Reg.Delete(saveAny); err != nil {
		t.Fatal(err)
	}
	p := d.ShortcutPressed("game", "s")
	if p.Refused == "" || !strings.Contains(p.Refused, "global/save") {
		t.Fatalf("a shortcut to a deleted play did not refuse naming it: %+v", p)
	}
}

// A namesake never takes a shortcut over: bound to the global save, it stays the global save in an
// application that has its own save.
func TestAShortcutRunsTheRoutineItWasAssignedNotItsNamesake(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "s", Steps: []routes.Route{saveAny}})
	p := d.ShortcutPressed("paint", "s")
	if len(p.Steps) != 1 || p.Steps[0].Route != saveAny {
		t.Fatalf("in paint `s fires %+v, want global/save — the namesake won", p.Steps)
	}
}

// Forgetting a routine removes its shortcuts — chains that include it, and a legacy entry whose
// words named it.
func TestForgettingARoutineRemovesItsShortcuts(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	assign(t, d, Assignment{Key: "x", Steps: []routes.Route{copyAny, freeplay},
		Cmd: "copy then enter freeplay"})
	assign(t, d, Assignment{Key: "c", Steps: []routes.Route{copyAny}})
	if err := d.Reg.Bind("", "f", "enter freeplay"); err != nil {
		t.Fatal(err)
	}
	gone, err := d.ForgetShortcutsOf(freeplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone) != 3 {
		t.Errorf("forgot %d shortcuts, want 3 (`e, the chain `x, the legacy `f): %+v",
			len(gone), gone)
	}
	left := d.Reg.Bindings()
	if len(left) != 1 || left[0].Key != "c" {
		t.Errorf("left %+v, want only `c", left)
	}
}

// A moved routine's shortcuts follow it.
func TestCarryingPointsShortcutsAtTheNewIdentity(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	to := routes.Route{App: "game", Focus: true, Slug: "enter-freeplay-now"}
	if err := d.Reg.Save(to, shortcutSrc); err != nil {
		t.Fatal(err)
	}
	if n, err := d.CarryShortcuts(freeplay, to); err != nil || n != 1 {
		t.Fatalf("carried %d, %v", n, err)
	}
	if p := d.ShortcutPressed("game", "e"); len(p.Steps) != 1 || p.Steps[0].Route != to {
		t.Errorf("`e still fires %+v after the move", p.Steps)
	}
}

// A routine a move replaced loses its shortcuts BEFORE the moved routine's are carried, so they
// do not silently start running the routine now at that identity.
func TestAReplacedRoutinesShortcutsAreNotInheritedByWhatReplacedIt(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "r", Steps: []routes.Route{saveAny}}) // bound to global/save
	if _, err := d.dropShortcutsNaming(saveAny); err != nil {
		t.Fatal(err)
	}
	if p := d.ShortcutPressed("game", "r"); p.Found {
		t.Errorf("a replaced play's shortcut survived: %+v", p)
	}
	// And the move does it in that order, under its locks.
	src, err := os.ReadFile("rescope.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	drop, carry := strings.Index(s, "d.dropShortcutsNaming(m.To)"),
		strings.Index(s, "d.CarryShortcuts(m.From, m.To)")
	if drop < 0 || carry < 0 || drop > carry {
		t.Error("Rescope does not drop the replaced play's shortcuts before carrying")
	}
}

// ONE LOCK ORDER: route, then bindings. The bindings lock is held only inside UpdateBindings, and
// nothing that runs inside it takes a route lock; the move takes its route locks before carrying.
func TestTheLockOrderIsRouteThenBindings(t *testing.T) {
	// THE BINDINGS LOCK IS TAKEN ONLY BY UpdateBindings, and nothing that runs inside an
	// UpdateBindings call takes a route lock. Every Go file in the engine is walked, so a new
	// call site is held without being named here.
	root := filepath.Join("..", "..")
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() && (e.Name() == ".git" || e.Name() == "plugins" ||
			e.Name() == ".tmp") {
			if e != nil && e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if e.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		src := string(raw)
		slash := filepath.ToSlash(p)
		if strings.Contains(src, "LockBindings(") &&
			!strings.HasSuffix(slash, "internal/routes/bindings.go") {
			t.Errorf("%s takes the bindings lock directly; only UpdateBindings may", p)
		}
		for i := 0; ; {
			j := strings.Index(src[i:], ".UpdateBindings(")
			if j < 0 {
				break
			}
			start := i + j + len(".UpdateBindings(")
			end := matchingParen(src, start)
			if body := src[start:end]; strings.Contains(body, ".Lock(") ||
				strings.Contains(body, "LockRoutes(") {
				t.Errorf("%s takes a route lock inside UpdateBindings, which holds the "+
					"bindings lock: the order is route, then bindings", p)
			}
			i = end
		}
		return nil
	})
	// And the routes package's own UpdateBindings takes no route lock while it holds its own.
	rb, err := os.ReadFile(filepath.Join("..", "routes", "bindings.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(rb)
	i := strings.Index(body, "func (r Registry) UpdateBindings(")
	j := strings.Index(body[i:], "\n}\n")
	if i < 0 || j < 0 || strings.Contains(body[i:i+j], ".Lock(") {
		t.Error("UpdateBindings takes a route lock while holding the bindings lock")
	}
}

// matchingParen is the index of the ')' closing a call whose arguments start at i.
func matchingParen(s string, i int) int {
	depth := 1
	for k := i; k < len(s); k++ {
		switch s[k] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return k
			}
		}
	}
	return len(s)
}

// THROUGH A REAL MOVE: the moved routine's shortcut follows it, and the replaced routine's
// shortcut is gone rather than silently running the routine now in its place.
func TestARealMoveCarriesShortcutsAndDropsTheReplacedRoutines(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	assign(t, d, Assignment{Key: "e", Steps: []routes.Route{rt}})                      // the one moving
	assign(t, d, Assignment{Key: "o", ListenIn: "notepad", Steps: []routes.Route{to}}) // the one replaced
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	src, rec := d.versionAt(to)
	if _, err := d.Rescope(m, Consent{ID: m.TakenID, Source: src, Recording: rec}); err != nil {
		t.Fatalf("the move was refused: %v", err)
	}
	if p := d.ShortcutPressed("notepad", "e"); len(p.Steps) != 1 || p.Steps[0].Route != to {
		t.Errorf("`e does not follow the moved play: %+v", p)
	}
	if p := d.ShortcutPressed("notepad", "o"); p.Found {
		t.Errorf("the replaced play's `o now runs the play that replaced it: %+v", p)
	}
}
