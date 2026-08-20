package plays_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A tree with one of every kind of play in it, which is what the product has to be able to show.
type tree struct {
	reg routes.Registry
	dir string
}

func newTree(t *testing.T) tree {
	t.Helper()
	d := t.TempDir()
	return tree{reg: routes.Registry{Dir: d}, dir: d}
}

const src = "script main...\n  do nothing.\n"

// authored writes a play with no past: somebody's own writing.
func (tr tree) authored(t *testing.T, rt routes.Route) {
	t.Helper()
	if err := tr.reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}
}

// recorded writes a play with the demonstration still beside it.
func (tr tree) recorded(t *testing.T, rt routes.Route) {
	t.Helper()
	tr.authored(t, rt)
	if err := tr.reg.SaveRecording(rt, []byte(`{"events":[]}`)); err != nil {
		t.Fatal(err)
	}
}

// staged writes a learned play where the resolver cannot see it.
func (tr tree) staged(t *testing.T, rt routes.Route) {
	t.Helper()
	err := tr.reg.SaveStaged(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: rt.App, From: "a", To: "b", Evidence: "e",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// learned writes a staged play and registers it, exactly as the product does.
func (tr tree) learned(t *testing.T, rt routes.Route) {
	t.Helper()
	tr.staged(t, rt)
	if err := tr.reg.Register(rt); err != nil {
		t.Fatal(err)
	}
}

func find(list []plays.Play, slug string) (plays.Play, bool) {
	for _, p := range list {
		if p.Slug == slug {
			return p, true
		}
	}
	return plays.Play{}, false
}

func mustFind(t *testing.T, list []plays.Play, slug string) plays.Play {
	t.Helper()
	p, ok := find(list, slug)
	if !ok {
		var have []string
		for _, x := range list {
			have = append(have, x.Slug)
		}
		t.Fatalf("no play %q in the listing; it holds %v", slug, have)
	}
	return p
}

// A staged play is on the Plays surface AND is still unreachable by asking for it.
//
// Both halves, in one test, on purpose: the cheapest way to make a staged play visible is to widen
// Registry.List by one directory, and Resolve walks List — so a listing test alone would pass
// while every staged play became answerable from every application.
func TestStagedPlaysAreListedAndStayUnresolvable(t *testing.T) {
	tr := newTree(t)
	tr.staged(t, routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"})

	p := mustFind(t, plays.List(tr.reg), "open-mouse-settings")
	if p.Registered {
		t.Error("a staged play is listed as registered")
	}
	if p.Life.Askable() {
		t.Errorf("a staged play is listed as askable (life %q)", p.Life)
	}

	if _, ok := tr.reg.Resolve("settings", "open mouse settings"); ok {
		t.Fatal("a staged play resolved in its own application")
	}
	if _, ok := tr.reg.Resolve("discord", "open mouse settings"); ok {
		t.Fatal("a staged play resolved from another application")
	}
	if _, ok := tr.reg.Resolve("", "open mouse settings"); ok {
		t.Fatal("a staged play resolved with nothing in front")
	}
	for _, rt := range tr.reg.List() {
		if rt.Slug == "open-mouse-settings" {
			t.Fatal("Registry.List can see a staged play; discovery is no longer a filter")
		}
	}
}

// Registering is what turns "saved" into "ready", and the listing has to follow it.
func TestRegisteringChangesHowAPlayIsPresented(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"}
	tr.staged(t, rt)

	before := mustFind(t, plays.List(tr.reg), "open-mouse-settings")
	if before.Life != plays.LifeSaved || before.Registered {
		t.Fatalf("staged play presented as %q registered=%v", before.Life, before.Registered)
	}
	if !before.Life.Registerable() {
		t.Error("a verified staged play does not offer registration")
	}

	if err := tr.reg.Register(rt); err != nil {
		t.Fatal(err)
	}

	after := mustFind(t, plays.List(tr.reg), "open-mouse-settings")
	if after.Life != plays.LifeReady || !after.Registered {
		t.Fatalf("registered play presented as %q registered=%v", after.Life, after.Registered)
	}
	if after.Kind != routes.KindLearned {
		t.Errorf("registering lost the play's kind: %q", after.Kind)
	}
	if _, ok := tr.reg.Resolve("discord", "open mouse settings"); !ok {
		t.Error("a registered learned play does not resolve from another application")
	}
	if len(plays.Staged(tr.reg)) != 0 {
		t.Error("the play is still listed as staged after registration")
	}
}

// Nothing may call a play ready on the strength of it existing on disk.
func TestAStagedPlayIsNeverReady(t *testing.T) {
	for _, state := range []routes.OriginState{
		routes.OriginNone, routes.OriginIntact, routes.OriginEdited,
		routes.OriginOrphaned, routes.OriginUnreadable,
	} {
		if life := plays.LifeOf(false, state); life.Askable() {
			t.Errorf("provenance %q made an unregistered play askable (%q)", state, life)
		}
		if life := plays.LifeOf(false, state); life == plays.LifeReady {
			t.Errorf("provenance %q made an unregistered play ready", state)
		}
	}
}

// The badge beside a staged play may not read like the badge beside a working one.
func TestNoSurfaceCallsAStagedPlayReady(t *testing.T) {
	for _, life := range []plays.Life{plays.LifeSaved, plays.LifeStuck} {
		word := life.Word()
		if !strings.Contains(strings.ToLower(word), "saved") {
			t.Errorf("%q reads %q, which does not say it was only saved", life, word)
		}
		if strings.Contains(strings.ToLower(word), "ready") {
			t.Errorf("%q reads %q, which claims it is ready", life, word)
		}
		if !strings.Contains(life.Says(), "Saved") {
			t.Errorf("%q says %q", life, life.Says())
		}
	}
	if plays.LifeSaved.Registerable() != true {
		t.Error("a verified staged play must offer registration")
	}
	if plays.LifeStuck.Registerable() {
		t.Error("a staged play that cannot be registered must not offer to be")
	}
}

// A staged play whose file has been edited cannot be registered as learned, and must not be
// offered as though it could.
func TestAStagedPlayThatCannotBeRegisteredSaysSo(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"}
	tr.staged(t, rt)
	if err := os.WriteFile(tr.reg.StagedPath(rt), []byte(src+"-- changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := mustFind(t, plays.List(tr.reg), "open-mouse-settings")
	if p.Life != plays.LifeStuck {
		t.Fatalf("an edited staged play is presented as %q", p.Life)
	}
	if p.Life.Registerable() {
		t.Fatal("Marco offers to register a play it will refuse to register")
	}
	if err := tr.reg.Register(rt); err == nil {
		t.Fatal("the registry accepted a play the surface said it would not")
	}
}

// Each of the three kinds is shown as itself.
func TestEveryKindIsPresentedAsItself(t *testing.T) {
	tr := newTree(t)
	tr.authored(t, routes.Route{Slug: "by-hand"})
	tr.recorded(t, routes.Route{App: "notepad", Slug: "demonstrated"})
	tr.learned(t, routes.Route{App: "settings", Focus: true, Slug: "worked-out"})

	list := plays.List(tr.reg)
	for _, c := range []struct {
		slug string
		kind routes.Kind
		word string
	}{
		{"by-hand", routes.KindAuthored, "Authored"},
		{"demonstrated", routes.KindTaught, "Recorded"},
		{"worked-out", routes.KindLearned, "Learned"},
	} {
		p := mustFind(t, list, c.slug)
		if p.Kind != c.kind {
			t.Errorf("%s is kind %q, want %q", c.slug, p.Kind, c.kind)
		}
		if got := plays.KindWord(p.Kind); got != c.word {
			t.Errorf("%s reads %q, want %q", c.slug, got, c.word)
		}
	}
	// The three words are three words. Collapsing any pair loses the distinction the
	// repository already stores.
	seen := map[string]bool{}
	for _, k := range []routes.Kind{routes.KindAuthored, routes.KindTaught, routes.KindLearned} {
		w := plays.KindWord(k)
		if seen[w] {
			t.Fatalf("two kinds present as %q", w)
		}
		seen[w] = true
	}
	// TEACH is reserved for Marco guiding the person (Glossary, ADR-048), so it may not be
	// the word for a play a person demonstrated.
	if strings.Contains(strings.ToLower(plays.KindWord(routes.KindTaught)), "taught") {
		t.Error("the demonstrated kind borrows the reserved word Teach")
	}
}

// A demonstrated play has no sidecar; only the recording beside it says where it came from.
func TestARecordedPlayIsNotShownAsAuthored(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "notepad", Slug: "demonstrated"}
	tr.authored(t, rt)
	if p := mustFind(t, plays.List(tr.reg), "demonstrated"); p.Kind != routes.KindAuthored {
		t.Fatalf("a play with neither sidecar nor recording is %q", p.Kind)
	}
	if err := tr.reg.SaveRecording(rt, []byte(`{"events":[]}`)); err != nil {
		t.Fatal(err)
	}
	if p := mustFind(t, plays.List(tr.reg), "demonstrated"); p.Kind != routes.KindTaught {
		t.Fatalf("a play with its demonstration beside it is %q", p.Kind)
	}
}

// A learned play's sidecar sits in the staging directory; reading the registered location instead
// reports it as somebody's own writing.
func TestAStagedPlayIsNotReportedAsAuthored(t *testing.T) {
	tr := newTree(t)
	tr.staged(t, routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"})
	p := mustFind(t, plays.Staged(tr.reg), "open-mouse-settings")
	if p.Kind != routes.KindLearned {
		t.Fatalf("a staged learned play is presented as %q", p.Kind)
	}
	if p.Provenance != routes.OriginIntact {
		t.Fatalf("a freshly staged play's provenance reads %q", p.Provenance)
	}
}

// Provenance that describes nothing does not make a play learned — and the listing and the
// authority door reach the same verdict, because they read the same function.
func TestAnOrphanedSidecarDoesNotMakeAPlayLearned(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "settings", Focus: true, Slug: "ghost"}
	if err := tr.reg.SaveWithOrigin(rt, src, routes.Origin{Kind: routes.KindLearned}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tr.reg.Path(rt)); err != nil {
		t.Fatal(err)
	}
	kind, state := tr.reg.KindOf(rt)
	if state != routes.OriginOrphaned {
		t.Fatalf("state is %q, want orphaned", state)
	}
	if kind == routes.KindLearned {
		t.Fatal("a sidecar with no play behind it still calls its play learned")
	}
}

// One decider, so a surface cannot call a play learned that the door calls authored.
func TestTheDoorAndTheListingAgreeAboutEveryProvenance(t *testing.T) {
	tr := newTree(t)
	app := "settings"

	intact := routes.Route{App: app, Focus: true, Slug: "intact"}
	tr.learned(t, intact)

	edited := routes.Route{App: app, Focus: true, Slug: "edited"}
	tr.learned(t, edited)
	if err := os.WriteFile(tr.reg.Path(edited), []byte(src+"-- changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unreadable := routes.Route{App: app, Focus: true, Slug: "unreadable"}
	tr.learned(t, unreadable)
	if err := os.WriteFile(tr.reg.OriginPath(unreadable), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	authored := routes.Route{App: app, Slug: "authored"}
	tr.authored(t, authored)

	for _, p := range plays.Registered(tr.reg) {
		door := orchestrator.Classify(tr.reg, p.Route, p.Name)
		if door.Kind != p.Kind {
			t.Errorf("%s: the listing says kind %q, the authority door says %q",
				p.Slug, p.Kind, door.Kind)
		}
		if door.Provenance != p.Provenance {
			t.Errorf("%s: the listing says %q, the authority door says %q",
				p.Slug, p.Provenance, door.Provenance)
		}
	}
	// And the four provenances really were distinct, or the loop above proved nothing.
	got := map[routes.OriginState]bool{}
	for _, p := range plays.Registered(tr.reg) {
		got[p.Provenance] = true
	}
	for _, want := range []routes.OriginState{
		routes.OriginNone, routes.OriginIntact, routes.OriginEdited, routes.OriginUnreadable,
	} {
		if !got[want] {
			t.Fatalf("the fixture produced no play whose provenance is %q", want)
		}
	}
}

// Editing your own play is allowed and is not damage; it must not read like breakage.
func TestAnEditedPlayStillRunsAndSaysWhy(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "settings", Focus: true, Slug: "edited"}
	tr.learned(t, rt)
	if err := os.WriteFile(tr.reg.Path(rt), []byte(src+"-- mine now\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := mustFind(t, plays.List(tr.reg), "edited")
	if p.Life != plays.LifeEdited {
		t.Fatalf("an edited registered play is presented as %q", p.Life)
	}
	if !p.Life.Askable() {
		t.Fatal("an edited play stopped being askable; editing a play does not unregister it")
	}
	if !strings.Contains(p.Life.Says(), "edited") {
		t.Errorf("the sentence does not mention the edit: %q", p.Life.Says())
	}
}

// FOCUS is not CONTEXT, and the difference is the capability.
func TestFocusIsPresentedDistinctlyFromContext(t *testing.T) {
	tr := newTree(t)
	tr.authored(t, routes.Route{App: "settings", Focus: true, Slug: "from-anywhere"})
	tr.authored(t, routes.Route{App: "settings", Slug: "in-place"})
	tr.authored(t, routes.Route{Slug: "everywhere"})

	list := plays.List(tr.reg)
	focus := mustFind(t, list, "from-anywhere")
	context := mustFind(t, list, "in-place")
	global := mustFind(t, list, "everywhere")

	if focus.Scope != plays.ScopeFocus || context.Scope != plays.ScopeContext ||
		global.Scope != plays.ScopeGlobal {
		t.Fatalf("scopes read %q / %q / %q", focus.Scope, context.Scope, global.Scope)
	}
	if focus.Activates != "settings" {
		t.Errorf("a focus play does not say which application it brings forward: %q",
			focus.Activates)
	}
	if context.Activates != "" || global.Activates != "" {
		t.Error("a play that switches nothing claims to bring an application forward")
	}
	if focus.Scope.Word() == context.Scope.Word() {
		t.Fatalf("focus and context share the badge %q", focus.Scope.Word())
	}
	if focus.Scope.Says("settings") == context.Scope.Says("settings") {
		t.Fatal("focus and context say the same sentence")
	}
	if !strings.Contains(focus.Scope.Says("settings"), "settings") {
		t.Errorf("the focus sentence does not name the application: %q",
			focus.Scope.Says("settings"))
	}
	if !strings.Contains(strings.ToLower(focus.Scope.Says("settings")), "forward") {
		t.Errorf("the focus sentence does not say it brings the application forward: %q",
			focus.Scope.Says("settings"))
	}
	if !strings.Contains(strings.ToLower(context.Scope.Says("settings")), "already") {
		t.Errorf("the context sentence does not say the application must already be in "+
			"front: %q", context.Scope.Says("settings"))
	}
}

// A learned play is staged as a play that will be asked for from anywhere, and the listing says
// the same thing registration will do.
func TestAStagedPlayListsAsBringingItsApplicationForward(t *testing.T) {
	tr := newTree(t)
	rt := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "open-mouse-settings"}
	tr.staged(t, rt)

	p := mustFind(t, plays.Staged(tr.reg), "open-mouse-settings")
	if p.Scope != plays.ScopeFocus {
		t.Fatalf("a staged learned play lists as scope %q", p.Scope)
	}
	if p.Activates != "settings" {
		t.Fatalf("a staged learned play does not name the application it will bring "+
			"forward: %q", p.Activates)
	}
	// And registering it really does put it where the listing promised.
	if err := tr.reg.Register(p.Route); err != nil {
		t.Fatal(err)
	}
	got, ok := tr.reg.Resolve("discord", "open mouse settings")
	if !ok {
		t.Fatal("the registered play does not resolve from another application")
	}
	if !got.Focus {
		t.Fatal("the listing promised a focus play and registration produced something else")
	}
}

// An app-less play is global whatever its Focus bit says.
func TestAnAppLessPlayIsGlobalWhateverItsFocusBit(t *testing.T) {
	if s := plays.ScopeOf(routes.Route{Focus: true, Slug: "x"}); s != plays.ScopeGlobal {
		t.Fatalf("an app-less route reads as scope %q", s)
	}
}

// Browsing must not touch the tree it is browsing.
func TestBrowsingPlaysChangesNothingOnDisk(t *testing.T) {
	tr := newTree(t)
	tr.authored(t, routes.Route{Slug: "by-hand"})
	tr.recorded(t, routes.Route{App: "notepad", Slug: "demonstrated"})
	tr.learned(t, routes.Route{App: "settings", Focus: true, Slug: "worked-out"})
	tr.staged(t, routes.Route{App: "settings", Focus: true, Slug: "waiting"})
	tr.staged(t, routes.Route{Slug: "app-less"})

	before := snapshot(t, tr.dir)
	for i := 0; i < 3; i++ {
		_ = plays.List(tr.reg)
		_ = plays.Staged(tr.reg)
		_ = plays.Registered(tr.reg)
		_, _ = plays.Find(tr.reg, "waiting", "settings", true)
	}
	after := snapshot(t, tr.dir)
	if before != after {
		t.Fatalf("listing plays changed the tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// An application that has never staged a play must not gain a staging directory just by
	// being listed — listing may not create what only saving needs.
	if _, err := os.Stat(filepath.Join(tr.dir, "notepad", routes.LearnedDir)); !os.IsNotExist(err) {
		t.Fatal("listing created a staging directory for an application that has none")
	}
}

// snapshot is every path under dir with its bytes, as one comparable string.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			lines = append(lines, "dir  "+rel)
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		lines = append(lines, "file "+rel+" "+routes.DigestOf(string(data)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// A binding reaches a play; it is not one.
func TestABindingReachesAPlayWithoutBecomingOne(t *testing.T) {
	tr := newTree(t)
	tr.authored(t, routes.Route{App: "rocketleague", Slug: "enter-freeplay"})

	before := plays.List(tr.reg)
	if err := tr.reg.Bind("rocketleague", "e", "enter freeplay"); err != nil {
		t.Fatal(err)
	}
	after := plays.List(tr.reg)

	if len(before) != len(after) {
		t.Fatalf("binding a hotkey changed the number of plays: %d then %d",
			len(before), len(after))
	}
	for _, p := range after {
		if p.Slug == "e" || p.Name == "e" {
			t.Fatal("a hotkey is listed as a play")
		}
	}
	if cmd, ok := tr.reg.HotkeyCmd("rocketleague", "e"); !ok || cmd != "enter freeplay" {
		t.Fatalf("the binding no longer reaches its play: %q %v", cmd, ok)
	}
	// And a binding pointing at nothing does not conjure a play either.
	if err := tr.reg.Bind("", "q", "nothing at all"); err != nil {
		t.Fatal(err)
	}
	for _, p := range plays.List(tr.reg) {
		if p.Slug == "nothing-at-all" {
			t.Fatal("a binding to a play that does not exist put one in the listing")
		}
	}
}

// A hand-written legacy bindings.json — the `slug` field, before `cmd` existed — still reaches a
// play, and reading it changes nothing.
func TestALegacyBindingStillReachesItsPlay(t *testing.T) {
	tr := newTree(t)
	tr.authored(t, routes.Route{App: "rocketleague", Slug: "enter-freeplay"})
	legacy := []byte(`[{"app":"rocketleague","key":"e","slug":"enter freeplay"}]`)
	if err := os.WriteFile(filepath.Join(tr.dir, "bindings.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, ok := tr.reg.HotkeyCmd("rocketleague", "e")
	if !ok || cmd != "enter freeplay" {
		t.Fatalf("a legacy binding no longer resolves: %q %v", cmd, ok)
	}
	var raw []map[string]any
	data, err := os.ReadFile(filepath.Join(tr.dir, "bindings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, still := raw[0]["slug"]; !still {
		t.Fatal("reading the bindings rewrote the file and dropped the legacy field")
	}
}

// Learn's completion and the Plays listing read the same words out of the same function.
func TestLearnAndThePlaysListingSayTheSameThing(t *testing.T) {
	tr := newTree(t)
	registered := routes.Route{App: "settings", Focus: true, Slug: "registered-one"}
	tr.learned(t, registered)
	staged := routes.Route{App: "settings", Focus: true, Slug: "staged-one"}
	tr.staged(t, staged)

	learnReady, ok := plays.AfterLearn(true, true)
	if !ok {
		t.Fatal("Learn reports nothing about a play it saved and registered")
	}
	learnSaved, ok := plays.AfterLearn(true, false)
	if !ok {
		t.Fatal("Learn reports nothing about a play it saved and could not register")
	}
	if _, ok := plays.AfterLearn(false, false); ok {
		t.Fatal("Learn claims a standing for a play it never saved")
	}

	list := plays.List(tr.reg)
	if got := mustFind(t, list, "registered-one").Life; got != learnReady {
		t.Errorf("Learn says %q, the listing says %q", learnReady, got)
	}
	if got := mustFind(t, list, "staged-one").Life; got != learnSaved {
		t.Errorf("Learn says %q, the listing says %q", learnSaved, got)
	}
	if learnSaved.Askable() {
		t.Fatal("Learn calls a play it could not register askable")
	}
}

// A play named after a scope folder does not disappear or land somewhere strange.
func TestAnApplicationNamedLikeAFolderIsStillListed(t *testing.T) {
	tr := newTree(t)
	tr.staged(t, routes.Route{App: "learned", Focus: true, Slug: "odd"})
	if _, ok := find(plays.Staged(tr.reg), "odd"); !ok {
		t.Fatal("a play staged under an application called `learned` is not listed")
	}
}
