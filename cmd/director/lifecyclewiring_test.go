package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// From what Director verified to a file a fresh Marco can find — and no further.
//
// Five boundaries, and this file exists to stop them collapsing into one:
//
//	written down   ordinary readable Marco, in memory        (Roadmap 24)
//	named          a person says what it is and what it does
//	saved          a file, where the resolver cannot see it
//	registered     moved somewhere the resolver looks
//	resolved       a later request finds it — and stops there
//
// None of them runs anything. The last test in this file is the one that says so.

// learnedIn points the lifecycle at a scratch routes tree for the duration of a test.
func learnedIn(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	return dir
}

// nameAndSave drives the real request path: name, save, and optionally register.
func nameAndSave(t *testing.T, g *observationRegistry, name, verb string,
	register bool) service.LearnedView {

	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.LearnedPlay(service.LearnedQuery{
		Application: "testgame", Name: name, Verb: verb, Save: true, Register: register,
	})
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if out.Saved == nil || !out.Saved.Saved {
		t.Fatalf("nothing was saved: %+v", out.Saved)
	}
	return out
}

// ── THE headline ──────────────────────────────────────────────────────────────

// A verified play is named, saved, registered, and found again by a fresh registry.
//
// The restart is real: the second half uses a registry constructed from nothing but the directory
// path, with no session, no proposal, no screen state, no track, no window generation and no
// process id available to it. If any of those were needed to find the play, this cannot pass.
func TestALearnedPlaySurvivesToAFreshProcess(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)

	out := nameAndSave(t, g, "Volume", "Mute", true)
	if !out.Saved.Registered {
		t.Fatal("the play was not registered")
	}
	src := out.Saved.Source
	if !strings.Contains(src, "the Volume is an actor.") ||
		!strings.Contains(src, "do Volume's Mute...") {
		t.Fatalf("the play does not carry the chosen names:\n%s", src)
	}

	// ── a fresh process: a registry built from a path, and nothing else ──
	fresh := routes.Registry{Dir: dir}
	rt, ok := fresh.Resolve("testgame", "volume")
	if !ok {
		t.Fatalf("a fresh registry could not find the play. Tree: %v", tree(t, dir))
	}
	if rt.Slug != "volume" {
		t.Fatalf("resolved to %+v", rt)
	}

	// The source is byte-stable across the trip.
	saved, err := os.ReadFile(fresh.Path(rt))
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(saved) != src {
		t.Fatalf("the file on disk is not what was generated:\n--- generated ---\n%s"+
			"\n--- on disk ---\n%s", src, saved)
	}
	// It is still ordinary Marco.
	if err := compileGenerated(string(saved)); err != nil {
		t.Fatalf("the saved play no longer compiles: %v", err)
	}

	// And its provenance is intact, and says where it came from in durable terms.
	o, state := fresh.Origin(rt)
	if !state.Verified() {
		t.Fatalf("provenance is %q", state)
	}
	if o.Kind != routes.KindLearned {
		t.Errorf("kind = %q", o.Kind)
	}
	if o.From == "" || o.To == "" || o.Evidence == "" {
		t.Errorf("provenance cannot be audited: %+v", o)
	}
	if o.Application != "testgame" {
		t.Errorf("application = %q", o.Application)
	}
}

// tree lists a directory tree, for failure messages.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

// ── saved is not registered ───────────────────────────────────────────────────

// A saved play is on disk, readable, and invisible to the resolver.
//
// Structural rather than a flag: discovery scans `global/`, an app's loose files, `context/` and
// `focus/`. A saved play lives in `<app>/learned/`, which is none of those, so `saved == registered`
// is not a mistake code can make — there is no boolean to get wrong.
func TestASavedPlayIsNotYetFindable(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)
	nameAndSave(t, g, "Volume", "Mute", false)

	fresh := routes.Registry{Dir: dir}
	if _, ok := fresh.Resolve("testgame", "volume"); ok {
		t.Fatal("a saved play was findable before anybody registered it")
	}
	for _, rt := range fresh.List() {
		if rt.Slug == "volume" {
			t.Fatalf("a saved play is listed as a route: %+v", rt)
		}
	}
	// But it IS on disk, and it is a file somebody can read and edit.
	staged := filepath.Join(dir, "testgame", routes.LearnedDir, "volume.marco")
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("the play was not written where it could be read: %v (tree %v)",
			err, tree(t, dir))
	}
}

// ── the user owns the file ────────────────────────────────────────────────────

// Editing a learned play is allowed, and it stops claiming to be the verified artifact.
//
// The promise the generated comment makes — *change it however you like* — is honoured. What
// changes is the CLAIM: an edited play is an ordinary play that remembers where it started, and
// nothing refuses to run it on that account.
func TestEditingALearnedPlayIsAllowedAndChangesWhatItClaims(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)
	nameAndSave(t, g, "Volume", "Mute", true)

	reg := routes.Registry{Dir: dir}
	rt, ok := reg.Resolve("testgame", "volume")
	if !ok {
		t.Fatal("the play was not registered")
	}
	if _, state := reg.Origin(rt); !state.Verified() {
		t.Fatalf("provenance is %q before any edit", state)
	}

	// A person edits it, as they were invited to.
	src, _ := os.ReadFile(reg.Path(rt))
	edited := strings.Replace(string(src), `log "done".`, `log "muted".`, 1)
	if edited == string(src) {
		t.Fatal("the fixture did not change anything")
	}
	if err := os.WriteFile(reg.Path(rt), []byte(edited), 0o644); err != nil {
		t.Fatalf("editing: %v", err)
	}

	o, state := reg.Origin(rt)
	if state != routes.OriginEdited {
		t.Fatalf("provenance is %q after an edit, want %q", state, routes.OriginEdited)
	}
	if state.Verified() {
		t.Fatal("an edited play still claims to be the artifact Director verified")
	}
	// The play is still there, still resolvable, still ordinary Marco. Editing is not a crime.
	if _, ok := reg.Resolve("testgame", "volume"); !ok {
		t.Fatal("editing a play made it disappear")
	}
	if err := compileGenerated(edited); err != nil {
		t.Fatalf("the edited play no longer compiles: %v", err)
	}
	// And it still remembers where it started, which is what an audit needs.
	if o.From == "" {
		t.Error("the edited play forgot where it came from")
	}
}

// ── collisions ────────────────────────────────────────────────────────────────

func TestCollisionsAreRefusedRatherThanResolved(t *testing.T) {
	t.Run("an authored route of the same name", func(t *testing.T) {
		dir := learnedIn(t)
		reg := routes.Registry{Dir: dir}
		author := routes.Route{App: "testgame", Slug: "volume"}
		if err := reg.Save(author, "the App is a script.\n\nlog \"mine\".\n"); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		before, _ := os.ReadFile(reg.Path(author))

		g := verifiedRegistry(t)
		rt := &Runtime{observations: g}
		_, err := rt.LearnedPlay(service.LearnedQuery{
			Application: "testgame", Name: "Volume", Verb: "Mute",
			Save: true, Register: true,
		})
		if err == nil {
			t.Fatal("a learned play overwrote somebody's authored route")
		}
		after, _ := os.ReadFile(reg.Path(author))
		if string(after) != string(before) {
			t.Fatal("the authored route was changed")
		}
	})

	t.Run("provenance for a play that is gone", func(t *testing.T) {
		dir := learnedIn(t)
		g := verifiedRegistry(t)
		nameAndSave(t, g, "Volume", "Mute", true)

		reg := routes.Registry{Dir: dir}
		rt, _ := reg.Resolve("testgame", "volume")
		if err := os.Remove(reg.Path(rt)); err != nil {
			t.Fatalf("removing: %v", err)
		}
		if _, state := reg.Origin(rt); state != routes.OriginOrphaned {
			t.Fatalf("provenance for a missing play is %q", state)
		}
		// A LATER, unrelated file under the same name must not inherit that past.
		if err := reg.Save(rt, "the App is a script.\n\nlog \"different\".\n"); err != nil {
			t.Fatalf("saving: %v", err)
		}
		if _, state := reg.Origin(rt); state.Verified() {
			t.Fatal("an unrelated file inherited a learned play's provenance")
		}
	})

	t.Run("a play with no provenance is simply authored", func(t *testing.T) {
		dir := learnedIn(t)
		reg := routes.Registry{Dir: dir}
		rt := routes.Route{App: "testgame", Slug: "handwritten"}
		if err := reg.Save(rt, "the App is a script.\n\nlog \"mine\".\n"); err != nil {
			t.Fatalf("saving: %v", err)
		}
		o, state := reg.Origin(rt)
		if state != routes.OriginNone {
			t.Fatalf("a play with no sidecar reads as %q", state)
		}
		if o.Kind == routes.KindLearned {
			t.Fatal("a file with no provenance claimed to be learned")
		}
	})
}

// ── forgetting ────────────────────────────────────────────────────────────────

// Forgetting a play removes it and its provenance — and nothing Director observed.
func TestForgettingAPlayLeavesWhatDirectorObserved(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)
	nameAndSave(t, g, "Volume", "Mute", true)

	store := g.memory
	subjectsBefore := len(store.Topology("testgame").Subjects)
	rehearsalsBefore := len(store.(observe.RehearsalStore).Rehearsals("testgame"))

	rt := &Runtime{observations: g}
	if _, err := rt.LearnedPlay(service.LearnedQuery{
		Application: "testgame", Name: "Volume", Forget: true,
	}); err != nil {
		t.Fatalf("forgetting: %v", err)
	}

	reg := routes.Registry{Dir: dir}
	if _, ok := reg.Resolve("testgame", "volume"); ok {
		t.Fatal("the play is still findable")
	}
	if _, err := os.Stat(reg.OriginPath(routes.Route{App: "testgame", Slug: "volume"})); err == nil {
		t.Fatal("provenance was left behind, pointing at nothing")
	}
	// And what Director SAW is untouched. Those are different operations.
	if got := len(store.Topology("testgame").Subjects); got != subjectsBefore {
		t.Errorf("forgetting a play forgot %d subject(s)", subjectsBefore-got)
	}
	if got := len(store.(observe.RehearsalStore).Rehearsals("testgame")); got != rehearsalsBefore {
		t.Errorf("forgetting a play discarded rehearsal evidence")
	}
}

// ── the lifecycle grants itself nothing ───────────────────────────────────────

// Nothing in the lifecycle can perform the play, and resolution is not permission.
//
// The load-bearing test. Naming, saving, registering and resolving are four things a person may
// do to a file; performing it is a fifth, and this milestone deliberately does not build it. A
// lifecycle that quietly acquired execution would be authority laundering: evidence in one end, a
// running keyboard out the other, with nobody having said yes to the last part.
func TestTheLifecycleGrantsItselfNoAuthority(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)

	// Whatever authority existed before is gone: the rehearsal grant was spent by the attempt
	// that produced the evidence, and nothing here creates another.
	// The grant is UNCHANGED — not created, not replaced, not spent, not renewed. Whatever
	// authority existed before the lifecycle ran exists after it, and no more.
	before := g.last.Grant()
	var stateBefore string
	if before != nil {
		stateBefore = string(before.State())
	}
	nameAndSave(t, g, "Volume", "Mute", true)
	after := g.last.Grant()
	if after != before {
		t.Fatal("the lifecycle created or replaced a rehearsal grant")
	}
	if after != nil && string(after.State()) != stateBefore {
		t.Fatalf("the lifecycle changed the authorization from %q to %q",
			stateBefore, after.State())
	}

	// Resolution finds the play and produces nothing that can run it.
	reg := routes.Registry{Dir: dir}
	rt, ok := reg.Resolve("testgame", "volume")
	if !ok {
		t.Fatal("the play was not registered")
	}
	// A Route is a name and a scope. There is nothing on it to invoke.
	v := reflect.TypeOf(rt)
	for _, forbidden := range []string{"Run", "Execute", "Invoke", "Perform", "Do", "Play"} {
		if _, has := v.MethodByName(forbidden); has {
			t.Errorf("routes.Route has a %s method; resolving a play would then be "+
				"performing it", forbidden)
		}
	}
	// And what a person is told keeps the two apart.
	rtm := &Runtime{observations: g}
	out, err := rtm.LearnedPlay(service.LearnedQuery{
		Application: "testgame", Name: "Volume", Verb: "Mute", Register: true,
	})
	_ = err
	if out.Saved != nil {
		joined := strings.ToLower(strings.Join(out.Saved.Lines, "\n"))
		if strings.Contains(joined, "running") || strings.Contains(joined, "performed") {
			t.Errorf("registering claims something ran:\n%s", joined)
		}
	}
}

// The durable bytes hold nothing captured.
func TestTheSavedBytesHoldNothingCaptured(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)
	nameAndSave(t, g, "Volume", "Mute", true)

	var all strings.Builder
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			data, _ := os.ReadFile(p)
			all.Write(data)
			all.WriteString("\n")
		}
		return nil
	})
	blob := strings.ToLower(all.String())

	// The SOURCE is readable Marco. The SIDECAR may name durable subjects and one digest —
	// that is what an audit needs — but neither may hold anything ephemeral or captured.
	for _, forbidden := range []string{
		"keycode", "scancode", "vk_", "screenshot", "pixel", "png", "confidence",
		"state_", "shadow_", "track_", "hwnd", "session_", "proposal_", "generation",
		"processid", "\"pid\"", "ocr",
	} {
		if strings.Contains(blob, forbidden) {
			t.Errorf("the durable bytes contain %q", forbidden)
		}
	}
	// And the play itself contains nothing but the play.
	reg := routes.Registry{Dir: dir}
	rt, _ := reg.Resolve("testgame", "volume")
	src, _ := os.ReadFile(reg.Path(rt))
	for _, forbidden := range []string{"subj_", "digest", "evidence", "candidate", "rehears"} {
		if strings.Contains(strings.ToLower(string(src)), forbidden) {
			t.Errorf("the play mentions %q:\n%s", forbidden, src)
		}
	}
}

// A provisional name cannot become a durable one.
func TestTheProvisionalNameCannotBeSaved(t *testing.T) {
	learnedIn(t)
	g := verifiedRegistry(t)
	rt := &Runtime{observations: g}
	if _, err := rt.LearnedPlay(service.LearnedQuery{
		Application: "testgame", Name: marcoexec.ProvisionalActor, Verb: "Run",
		Save: true,
	}); err == nil {
		t.Fatal("the placeholder Marco uses before anybody has named it was saved as a name")
	}
	for _, bad := range []string{"", "volume mute", "1Volume", "volume", "Vol-ume"} {
		if _, err := rt.LearnedPlay(service.LearnedQuery{
			Application: "testgame", Name: bad, Verb: "Mute", Save: true,
		}); err == nil {
			t.Errorf("%q was accepted as a play's name", bad)
		}
	}
}

// A route nobody has verified cannot be named and saved.
//
// The lifecycle inherits Roadmap 24's gate rather than re-implementing it: `pickPlay` only ever
// looks at routes the lowering judgement called eligible, so an unverified one has nothing to save.
func TestAnUnverifiedRouteCannotBeSaved(t *testing.T) {
	dir := learnedIn(t)
	g := authorizedRegistry(t) // demonstrated twice, never rehearsed

	rt := &Runtime{observations: g}
	if _, err := rt.LearnedPlay(service.LearnedQuery{
		Application: "testgame", Name: "Volume", Verb: "Mute", Save: true,
	}); err == nil {
		t.Fatal("a route nobody has tried was named and saved")
	}
	if files := tree(t, dir); len(files) != 0 {
		t.Fatalf("an unverified route left files behind: %v", files)
	}
}

// A staged play edited before registration cannot be registered AS a learned play.
//
// Fail-closed: registering is what makes a play findable, and doing that on Director's authority
// for a file Director did not write would be vouching for somebody else's edit.
func TestAnEditedStagedPlayCannotBeRegisteredAsLearned(t *testing.T) {
	dir := learnedIn(t)
	g := verifiedRegistry(t)
	nameAndSave(t, g, "Volume", "Mute", false)

	reg := routes.Registry{Dir: dir}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	src, ok := reg.StagedSource(rt)
	if !ok {
		t.Fatal("nothing was staged")
	}
	edited := strings.Replace(src, `log "done".`, `log "changed".`, 1)
	if err := os.WriteFile(reg.StagedPath(rt), []byte(edited), 0o644); err != nil {
		t.Fatalf("editing: %v", err)
	}
	if _, state := reg.StagedOrigin(rt); state != routes.OriginEdited {
		t.Fatalf("the staged play reads as %q after an edit", state)
	}

	if err := reg.Register(rt); err == nil {
		t.Fatal("an edited play was registered as one Director verified")
	}
	if _, found := reg.Resolve("testgame", "volume"); found {
		t.Fatal("the edited play became findable anyway")
	}
}
