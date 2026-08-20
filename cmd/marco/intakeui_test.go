package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A Run pressed in the control centre, driven through the REAL handler over a real registry.
//
// # What these hold
//
// The Plays surface already knows which play a row is: it was given the slug, the app and the
// scope by the listing that drew it. Turning that back into a display name and asking the engine
// to match it again is throwing an identity away and guessing it back — and the guess can miss,
// because the shown name is derived from the slug and that derivation is not reversible.
//
// Nothing here executes `marco do`: `runSpawn` is swapped for a recorder, so the assertions are
// about WHAT WOULD BE LAUNCHED. `marco do` performs real input.

// captureRun replaces the spawn with a recorder and returns where the argv lands.
func captureRun(t *testing.T) *[]string {
	t.Helper()
	var got []string
	prev := runSpawn
	// Records the argv and starts NOTHING. A nil process is a real answer to the run account:
	// there is nothing to watch, so the run is recorded as unavailable rather than left pending
	// for ever. These tests are about what WOULD be launched, never about running it.
	runSpawn = func(args []string) (*exec.Cmd, io.Reader, error) {
		got = append([]string(nil), args...)
		return nil, nil, nil
	}
	t.Cleanup(func() { runSpawn = prev })
	return &got
}

// postDo presses Run with the payload the page would send.
func postDo(t *testing.T, e *editor, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	e.handleDo(w, httptest.NewRequest(http.MethodPost, "/api/do", strings.NewReader(string(b))))
	return w
}

// runRow reads one row out of the REAL Plays listing, so a test posts the handle the page actually
// holds rather than one it made up. `rowFor` (plays_test.go) stays the one row picker.
func runRow(t *testing.T, e *editor, slug string, registered bool) playRow {
	t.Helper()
	rows, _ := listPlays(t, e)
	return rowFor(t, rows, slug, registered)
}

// A clicked Run launches an IDENTITY — slug, app, scope — and never a phrase to be matched again.
func TestAClickedRunSpawnsAnExplicitIdentity(t *testing.T) {
	e := newTestEditor(t)
	registerLearned(t, e, "settings", true, "open-mouse-settings")
	p := runRow(t, e, "open-mouse-settings", true)

	got := captureRun(t)
	if w := postDo(t, e, map[string]string{"slug": p.Slug, "app": p.App, "scope": p.Scope}); w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d: %s", w.Code, w.Body.String())
	}
	want := []string{"do", "--source=control-centre", "--play=open-mouse-settings",
		"--app=settings", "--focus"}
	if strings.Join(*got, " ") != strings.Join(want, " ") {
		t.Fatalf("a clicked Run spawns %q, want %q", *got, want)
	}
	// And it carries NO free text. A bare word in this argv is a phrase, and a phrase is read
	// by the intake as something to interpret — which is the whole defect this replaces.
	for _, a := range (*got)[1:] {
		if !strings.HasPrefix(a, "--") {
			t.Errorf("the argv carries the loose word %q — the surface is describing the play "+
				"instead of naming it", a)
		}
	}
	if strings.Contains(strings.Join(*got, " "), p.Name) {
		t.Errorf("the argv contains the display name %q; the row already held the slug", p.Name)
	}
}

// A play whose shown name does not survive being turned back into a slug still runs — the RIGHT
// one — because the row never gave the name up in the first place.
//
// `plays.Pretty` only turns dashes into spaces, so a slug carrying anything else (an underscore in
// a hand-written `.marco`, which nothing forbids) renders to a name that `routes.Slug` maps to a
// DIFFERENT slug. Both plays exist here on purpose: the name path has somewhere wrong to land.
func TestARunSurvivesANameThatDoesNotRoundTripThroughItsSlug(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open_mouse_settings")
	authored(t, e, "", false, "open-mouse-settings")
	p := runRow(t, e, "open_mouse_settings", true)

	// The premise, stated rather than assumed: this name is not a handle for this play.
	if routes.Slug(p.Name) == p.Slug {
		t.Fatalf("the premise is gone: %q now slugs back to %q", p.Name, p.Slug)
	}
	got := captureRun(t)
	if w := postDo(t, e, map[string]string{"slug": p.Slug, "app": p.App, "scope": p.Scope}); w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d: %s", w.Code, w.Body.String())
	}
	if !contains(*got, "--play=open_mouse_settings") {
		t.Fatalf("Run launched %q — it did not run the play the row was showing", *got)
	}
	if contains(*got, "--play=open-mouse-settings") {
		t.Fatal("Run landed on the neighbouring play: the identity was turned back into words")
	}
}

// A STAGED play cannot be run through the endpoint — not because the page draws no button, but
// because the endpoint refuses it.
//
// The button's absence is a rendering decision, and a rendering decision is not enforcement:
// anything can post to this URL, and a staged play is one `Resolve` cannot find, so starting it
// would be starting nothing while reporting success.
func TestAStagedPlayCannotBeRunThroughTheEndpoint(t *testing.T) {
	e := newTestEditor(t)
	stagePlay(t, e, "settings", "open-mouse-settings")
	p := runRow(t, e, "open-mouse-settings", false)
	if p.Registered {
		t.Fatal("the fixture is registered; this test needs a staged play")
	}
	got := captureRun(t)
	w := postDo(t, e, map[string]string{"slug": p.Slug, "app": p.App, "scope": p.Scope})
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST /api/do on a staged play = %d, want 404: %s", w.Code, w.Body.String())
	}
	if len(*got) != 0 {
		t.Fatalf("a staged play spawned %q", *got)
	}
}

// A staged play does not become runnable by sharing its name with a registered one.
//
// This is the NORMAL position for a staged play — registration is refused on a name collision — so
// the endpoint has to distinguish the two rows rather than the two names.
func TestRunningTheStagedRowOfACollidingNameStillRefusesTheStagedFile(t *testing.T) {
	e := newTestEditor(t)
	registerLearned(t, e, "settings", true, "open-mouse-settings")
	stagePlay(t, e, "settings", "open-mouse-settings")
	got := captureRun(t)
	// The staged row's scope is the learned one; the registered row answers for the same
	// slug and app. Whatever runs, it must be the REGISTERED file — never the staged one.
	if w := postDo(t, e, map[string]string{"slug": "open-mouse-settings", "app": "settings",
		"scope": "focus"}); w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d: %s", w.Code, w.Body.String())
	}
	if !contains(*got, "--play=open-mouse-settings") || !contains(*got, "--app=settings") {
		t.Fatalf("Run launched %q", *got)
	}
}

// The SCOPE on the row decides which of two registrations runs.
//
// One slug can be registered under one app twice — once in-context, once as a focus play — and
// they are two files that do two things. The listing draws them as two rows, so the row's own
// scope has to reach the endpoint; without it a Run is a coin toss between them.
//
// Deleting the scope arm of doTarget must fail this.
func TestTheScopeOnTheRowDecidesWhichOfTwoRegistrationsRuns(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "settings", false, "open-mouse-settings") // context
	authored(t, e, "settings", true, "open-mouse-settings")  // focus
	for _, want := range []struct{ scope, flag string }{{"context", ""}, {"focus", "--focus"}} {
		got := captureRun(t)
		if w := postDo(t, e, map[string]string{"slug": "open-mouse-settings", "app": "settings",
			"scope": want.scope}); w.Code != http.StatusOK {
			t.Fatalf("POST /api/do (%s) = %d: %s", want.scope, w.Code, w.Body.String())
		}
		if hasFocus := contains(*got, "--focus"); hasFocus != (want.flag == "--focus") {
			t.Errorf("Run on the %s row spawned %q — it ran the other registration",
				want.scope, *got)
		}
	}
}

// A name-only payload — the shape this endpoint used to take — still leaves as an identity.
//
// Kept working on purpose: anything outside this page may still post one. It is resolved HERE,
// against the registry, so even the old shape does not hand words to the intake to match again.
func TestANameOnlyRunPayloadIsResolvedToAnIdentity(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open-mouse-settings")
	got := captureRun(t)
	if w := postDo(t, e, map[string]string{"name": "open mouse settings"}); w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d: %s", w.Code, w.Body.String())
	}
	if !contains(*got, "--play=open-mouse-settings") {
		t.Fatalf("a name-only Run launched %q, which is not an identity", *got)
	}
}

// A Run for a play Marco does not have is refused, and starts nothing.
func TestRunningAPlayThatIsNotThereStartsNothing(t *testing.T) {
	e := newTestEditor(t)
	got := captureRun(t)
	if w := postDo(t, e, map[string]string{"slug": "not-a-play", "app": "", "scope": "global"}); w.Code != http.StatusNotFound {
		t.Fatalf("POST /api/do for an unknown play = %d, want 404", w.Code)
	}
	if len(*got) != 0 {
		t.Fatalf("an unknown play spawned %q", *got)
	}
}

// The Edit view runs the play it has open BY ITS IDENTITY: /api/route carries the slug, and the
// page hands that straight back.
func TestTheEditViewRunsThePlayItHasOpenByIdentity(t *testing.T) {
	e := newTestEditor(t)
	rt := registerLearned(t, e, "settings", true, "open-mouse-settings")
	e.rt, e.path = rt, e.reg.Path(rt)
	if err := e.loadSrc(); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	e.handleRoute(w, httptest.NewRequest(http.MethodGet, "/api/route", nil))
	var got struct{ Slug, App, Scope string }
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Slug != rt.Slug || got.App != rt.App || got.Scope != "focus" {
		t.Fatalf("/api/route says slug=%q app=%q scope=%q — the Edit view cannot name what it "+
			"has open", got.Slug, got.App, got.Scope)
	}
	// The page's half of it. Both views post the same identity through the same function.
	// The button, the function behind it, the identity it is handed, and the Plays row's Run —
	// each named separately, because the button's markup alone survives losing the function.
	for _, want := range []string{`id="editrun"`, `onclick="doOpenPlay()"`,
		"function doOpenPlay(", "slug:r.slug", "doPlay(p)"} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the control centre page is missing %q; a Run there is a phrase again", want)
		}
	}
	if strings.Contains(editPage, "doRoute(p.name)") {
		t.Error("a Run still posts the display name")
	}
}

// The argv the intake is handed matches the flags `marco do` actually parses.
//
// Deleting the scope arms — spawning a focus play without --focus, or a context play with it —
// must fail here: the three scopes are three different files and the flags are how they are told
// apart on a command line.
func TestTheRunArgvSpellsEachScope(t *testing.T) {
	for _, c := range []struct {
		name string
		rt   routes.Route
		want string
	}{
		{"global", routes.Route{Slug: "s"}, "do --source=control-centre --play=s"},
		{"context", routes.Route{App: "a", Slug: "s"}, "do --source=control-centre --play=s --app=a"},
		{"focus", routes.Route{App: "a", Focus: true, Slug: "s"}, "do --source=control-centre --play=s --app=a --focus"},
	} {
		if got := strings.Join(doArgv(c.rt), " "); got != c.want {
			t.Errorf("doArgv(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
