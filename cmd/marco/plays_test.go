package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The control centre's PLAYS surface, driven through the real handlers over a real registry in a
// temp directory. Nothing here re-implements a listing: every assertion enters through the
// endpoint the page actually calls, so deleting a handler's body fails these rather than a copy.

const playSrc = `use os.

the Opener is an actor.
this can Run.
this's Run does...
    do OS's Key with "enter".
    this is ok!
`

// newTestEditor is one control-centre session over an empty routes tree.
func newTestEditor(t *testing.T) *editor {
	t.Helper()
	return &editor{reg: routes.Registry{Dir: t.TempDir()}}
}

// stagePlay writes a saved-but-not-askable learned play, the way Learn leaves one behind.
func stagePlay(t *testing.T, e *editor, app, slug string) routes.Route {
	t.Helper()
	rt := routes.Route{App: app, Focus: routes.LearnedFocus, Slug: slug}
	o := routes.Origin{Kind: routes.KindLearned, Application: app, From: "a", To: "b"}
	if err := e.reg.SaveStaged(rt, playSrc, o); err != nil {
		t.Fatal(err)
	}
	return rt
}

// registerLearned writes a learned play where the resolver can already see it.
func registerLearned(t *testing.T, e *editor, app string, focus bool, slug string) routes.Route {
	t.Helper()
	rt := routes.Route{App: app, Focus: focus, Slug: slug}
	o := routes.Origin{Kind: routes.KindLearned, Application: app, From: "a", To: "b"}
	if err := e.reg.SaveWithOrigin(rt, playSrc, o); err != nil {
		t.Fatal(err)
	}
	return rt
}

// authored writes an ordinary hand-written play: source, no sidecar, no recording.
func authored(t *testing.T, e *editor, app string, focus bool, slug string) routes.Route {
	t.Helper()
	rt := routes.Route{App: app, Focus: focus, Slug: slug}
	if err := e.reg.Save(rt, playSrc); err != nil {
		t.Fatal(err)
	}
	return rt
}

// listPlays reads the Plays surface exactly as the page does.
func listPlays(t *testing.T, e *editor) ([]playRow, []bindingRow) {
	t.Helper()
	w := httptest.NewRecorder()
	e.handlePlays(w, httptest.NewRequest(http.MethodGet, "/api/plays", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/plays = %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Plays    []playRow    `json:"plays"`
		Bindings []bindingRow `json:"bindings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode /api/plays: %v (%s)", err, w.Body.String())
	}
	return got.Plays, got.Bindings
}

// rowFor picks one row out of a listing. Registered is part of the key because the same slug can
// legitimately appear twice — once askable, once still staged — and that is exactly the case a
// collision produces.
func rowFor(t *testing.T, rows []playRow, slug string, registered bool) playRow {
	t.Helper()
	for _, p := range rows {
		if p.Slug == slug && p.Registered == registered {
			return p
		}
	}
	t.Fatalf("no registered=%v row for %q in %+v", registered, slug, rows)
	return playRow{}
}

// postJSON drives one handler with a JSON body, the way the page's fetch does.
func postJSON(t *testing.T, h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw)))
	return w
}

// treeDigest is every path and every byte under root, so "nothing changed" can be asserted rather
// than spot-checked.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		fmt.Fprintf(h, "%s|%v\n", filepath.ToSlash(rel), d.IsDir())
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The listing shows both halves of the product and never lets them look alike.
func TestThePlaysListingShowsRegisteredAndStagedPlaysDifferently(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "copy-all")
	registerLearned(t, e, "settings", routes.LearnedFocus, "open-bluetooth")
	stagePlay(t, e, "settings", "open-mouse-settings")

	rows, _ := listPlays(t, e)
	if len(rows) != 3 {
		t.Fatalf("listing has %d rows, want 3: %+v", len(rows), rows)
	}

	own := rowFor(t, rows, "copy-all", true)
	if own.KindWord != "Authored" {
		t.Errorf("a hand-written play reads as %q, want Authored", own.KindWord)
	}
	if own.Scope != "global" || own.ScopeWord == "" {
		t.Errorf("an app-less play reads as scope %q (%q), want global", own.Scope, own.ScopeWord)
	}

	learned := rowFor(t, rows, "open-bluetooth", true)
	if learned.KindWord != "Learned" {
		t.Errorf("a learned play reads as %q, want Learned", learned.KindWord)
	}
	if !learned.Askable || learned.Life != "ready" || learned.LifeWord != "Ready" {
		t.Errorf("a registered learned play = %+v, want askable and ready", learned)
	}
	if learned.Activates != "settings" {
		t.Errorf("a focus play activates %q, want settings — the capability the scope word "+
			"alone would drop", learned.Activates)
	}

	staged := rowFor(t, rows, "open-mouse-settings", false)
	if staged.KindWord != "Learned" {
		t.Errorf("a staged learned play reads as %q, want Learned", staged.KindWord)
	}
	if staged.Askable {
		t.Error("a staged play is listed as askable; nothing can ask for it")
	}
	if staged.Life != "saved" {
		t.Errorf("a staged play's life = %q, want saved", staged.Life)
	}
	if strings.EqualFold(staged.LifeWord, "ready") {
		t.Errorf("a staged play's badge says %q — no surface may call a saved play ready",
			staged.LifeWord)
	}
	if staged.LifeSays == "" {
		t.Error("a staged play carries no sentence saying what it is waiting for")
	}
}

// Register is offered where it can be kept, and only there.
func TestAStagedRowOffersRegisterAndAnAskableRowDoesNot(t *testing.T) {
	e := newTestEditor(t)
	registerLearned(t, e, "settings", routes.LearnedFocus, "open-bluetooth")
	stagePlay(t, e, "settings", "open-mouse-settings")

	rows, _ := listPlays(t, e)
	if staged := rowFor(t, rows, "open-mouse-settings", false); !staged.Registerable {
		t.Error("a saved, verified play offers no Register button — the one action it needs")
	}
	askable := rowFor(t, rows, "open-bluetooth", true)
	if askable.Registerable {
		t.Error("an already-askable play offers Register; registering it again means nothing")
	}
}

// Registering makes a saved play something Marco can be asked for, and the listing says so.
func TestRegisteringAStagedPlayMakesItReadyAndResolvable(t *testing.T) {
	e := newTestEditor(t)
	stagePlay(t, e, "settings", "open-mouse-settings")

	before, _ := listPlays(t, e)
	row := rowFor(t, before, "open-mouse-settings", false)

	w := postJSON(t, e.handleRegister, "/api/register",
		map[string]any{"slug": row.Slug, "app": row.App})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/register = %d: %s", w.Code, w.Body.String())
	}

	after, _ := listPlays(t, e)
	if len(after) != 1 {
		t.Fatalf("after registering the listing has %d rows, want 1: %+v", len(after), after)
	}
	got := rowFor(t, after, "open-mouse-settings", true)
	if !got.Askable || got.Life != "ready" {
		t.Errorf("a registered play = %+v, want askable and ready", got)
	}
	if got.KindWord != "Learned" {
		t.Errorf("registering changed the kind to %q; its provenance moved with it", got.KindWord)
	}
	// And the resolver — the thing "askable" is a claim about — can actually find it.
	if _, ok := e.reg.Resolve("settings", "open mouse settings"); !ok {
		t.Fatal("the listing calls it askable and Resolve cannot find it")
	}
}

// Registration acts on the slug the listing carried, not on the name it displayed.
func TestRegisteringActsOnTheSlugTheListingCarried(t *testing.T) {
	e := newTestEditor(t)
	// A slug that does NOT survive a round trip through its display name. Slugging the shown
	// name would target a different file — or none — which is why the row carries the slug.
	const slug = "open--mouse-settings"
	stagePlay(t, e, "settings", slug)

	rows, _ := listPlays(t, e)
	row := rowFor(t, rows, slug, false)
	if routes.Slug(row.Name) == row.Slug {
		t.Fatalf("this test no longer discriminates: %q re-slugs to itself", row.Name)
	}

	w := postJSON(t, e.handleRegister, "/api/register",
		map[string]any{"slug": row.Slug, "app": row.App})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/register = %d: %s", w.Code, w.Body.String())
	}
	if !e.reg.Has(routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: slug}) {
		t.Fatal("the play registered under a slug other than the one the row carried")
	}
}

// A refused registration says so, and the play is still shown as saved — never as ready.
func TestARefusedRegistrationStillShowsThePlayAsSaved(t *testing.T) {
	e := newTestEditor(t)
	// Somebody's own play already answers to this name, so registering the learned one behind
	// it would put a second play behind one word.
	authored(t, e, "settings", false, "open-mouse-settings")
	stagePlay(t, e, "settings", "open-mouse-settings")

	w := postJSON(t, e.handleRegister, "/api/register",
		map[string]any{"slug": "open-mouse-settings", "app": "settings"})
	if w.Code == http.StatusOK {
		t.Fatal("a colliding registration reported success")
	}
	msg := strings.TrimSpace(w.Body.String())
	if msg == "" {
		t.Fatal("a refused registration returned no reason to show")
	}
	if strings.HasPrefix(msg, "routes: ") {
		t.Errorf("the refusal leaks a package name into a user sentence: %q", msg)
	}

	rows, _ := listPlays(t, e)
	staged := rowFor(t, rows, "open-mouse-settings", false)
	if staged.Askable || staged.Life != "saved" {
		t.Errorf("after a refused registration the play reads %+v, want still saved", staged)
	}
	if rowFor(t, rows, "open-mouse-settings", true).KindWord != "Authored" {
		t.Error("the play the collision was with is no longer the one that answers the name")
	}
}

// Moving a play between scopes moves what it IS, and leaves nothing behind.
func TestChangingScopeKeepsALearnedPlayLearned(t *testing.T) {
	e := newTestEditor(t)
	from := registerLearned(t, e, "settings", true, "open-mouse-settings")
	oldOrigin := e.reg.OriginPath(from)
	if _, err := os.Stat(oldOrigin); err != nil {
		t.Fatalf("the fixture has no provenance to lose: %v", err)
	}

	w := postJSON(t, e.handleScope, "/api/scope", map[string]any{
		"name": "open mouse settings", "slug": from.Slug,
		"curApp": "settings", "curScope": "focus", "scope": "context", "app": "settings",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/scope = %d: %s", w.Code, w.Body.String())
	}

	rows, _ := listPlays(t, e)
	if len(rows) != 1 {
		t.Fatalf("after a scope change the listing has %d rows, want 1: %+v", len(rows), rows)
	}
	got := rows[0]
	if got.KindWord != "Learned" {
		t.Errorf("changing scope re-listed the play as %q — its past did not travel with the "+
			"file", got.KindWord)
	}
	if got.Scope != "context" {
		t.Errorf("the play sits at scope %q, want context", got.Scope)
	}
	if got.Life != "ready" {
		t.Errorf("the moved play's life = %q, want ready", got.Life)
	}
	// AND NO ORPHAN. A sidecar left at the old location describes a play that is not there, and
	// no command a person has could remove it.
	if _, err := os.Stat(oldOrigin); !os.IsNotExist(err) {
		t.Errorf("provenance was left behind at %s (stat err %v)", oldOrigin, err)
	}
}

// Forgetting a play removes everything that described it, registered or staged.
func TestForgettingAPlayLeavesNoOrphanedProvenance(t *testing.T) {
	e := newTestEditor(t)
	rt := registerLearned(t, e, "settings", true, "open-bluetooth")
	origin := e.reg.OriginPath(rt)

	w := postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"name": "open bluetooth", "slug": rt.Slug, "app": "settings", "scope": "focus",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/delete = %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(origin); !os.IsNotExist(err) {
		t.Errorf("forgetting left provenance behind at %s (stat err %v)", origin, err)
	}
	if rows, _ := listPlays(t, e); len(rows) != 0 {
		t.Fatalf("the forgotten play is still listed: %+v", rows)
	}
}

// A staged play can be forgotten too — the registered-location guard must not refuse it.
func TestForgettingAStagedPlayReachesTheStagingDirectory(t *testing.T) {
	e := newTestEditor(t)
	stagePlay(t, e, "settings", "open-mouse-settings")

	w := postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"name": "open mouse settings", "slug": "open-mouse-settings", "app": "settings",
		"scope": "focus", "staged": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/delete (staged) = %d: %s", w.Code, w.Body.String())
	}
	if rows, _ := listPlays(t, e); len(rows) != 0 {
		t.Fatalf("the forgotten staged play is still listed: %+v", rows)
	}
	// Forgetting something that was never there is a 404, not a silent success.
	again := postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"slug": "open-mouse-settings", "app": "settings", "staged": true,
	})
	if again.Code != http.StatusNotFound {
		t.Errorf("forgetting a play that is gone = %d, want 404", again.Code)
	}
}

// Browsing is browsing: reading the listing writes nothing.
func TestBrowsingThePlaysListingChangesNothingOnDisk(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "copy-all")
	registerLearned(t, e, "settings", true, "open-bluetooth")
	stagePlay(t, e, "settings", "open-mouse-settings")
	if err := e.reg.Bind("settings", "e", "open bluetooth"); err != nil {
		t.Fatal(err)
	}

	before := treeDigest(t, e.reg.Dir)
	for i := 0; i < 3; i++ {
		listPlays(t, e)
	}
	if after := treeDigest(t, e.reg.Dir); after != before {
		t.Fatal("reading /api/plays changed the routes tree")
	}
}

// A hotkey is a way in, and it is never a play.
func TestAHotkeyIsShownAsATriggerAndNotAsAPlayRow(t *testing.T) {
	e := newTestEditor(t)
	registerLearned(t, e, "settings", true, "open-bluetooth")
	if err := e.reg.Bind("settings", "e", "open bluetooth"); err != nil {
		t.Fatal(err)
	}

	rows, binds := listPlays(t, e)
	if len(rows) != 1 {
		t.Fatalf("a binding became a play row: %+v", rows)
	}
	if len(binds) != 1 || binds[0].Key != "e" || binds[0].Cmd != "open bluetooth" {
		t.Fatalf("bindings = %+v, want one ⌨e → open bluetooth", binds)
	}
	if binds[0].App != "settings" {
		t.Errorf("the binding is scoped to %q, want settings", binds[0].App)
	}
}

// `marco ui plays` opens the same view `marco ui routes` always has.
func TestMarcoUiPlaysOpensThePlaysView(t *testing.T) {
	for _, arg := range []string{"plays", "Plays", " plays "} {
		if got := uiView([]string{arg}); got != "routes" {
			t.Errorf("uiView(%q) = %q, want routes", arg, got)
		}
	}
	if got := uiView([]string{"routes"}); got != "routes" {
		t.Errorf("uiView(routes) = %q — the argument that always worked must keep working", got)
	}
	if got := uiView([]string{"nonsense"}); got != "" {
		t.Errorf("uiView(nonsense) = %q, want the default view", got)
	}
}

// The tab a person reads says Plays; the identifier underneath it does not move.
func TestThePlaysTabIsLabelledPlaysOverTheRoutesIdentifier(t *testing.T) {
	for _, want := range []string{
		`data-view="routes"`, `id="view-routes"`, `nav('routes')`, `▤ Plays`, `<h2>Plays</h2>`,
	} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the control centre page is missing %q", want)
		}
	}
	if strings.Contains(editPage, "▤ Routes") {
		t.Error("the nav still labels the tab Routes")
	}
}

// With no play named, the front door is the list of plays — not the step editor.
func TestTheControlCentreLandsOnPlaysWithNoPlayNamed(t *testing.T) {
	const landing = `nav(r.view || (r.loaded ? 'edit' : 'routes'));`
	if !strings.Contains(editPage, landing) {
		t.Fatalf("the startup landing is not %q — a step editor with nothing loaded is not a "+
			"front door", landing)
	}
}

// Learn and the Plays list say the same words about the same file.
func TestTheLearnPanelRendersTheLifecycleWords(t *testing.T) {
	for _, want := range []string{"v.life_word", "v.life_says"} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the Learn panel does not render %s; it is composing its own status", want)
		}
	}
	// The sentence that was a claim about discovery made from a fact about storage.
	for _, gone := range []string{"It is in the Routes tab", "it is not in the Routes tab"} {
		if strings.Contains(editPage, gone) {
			t.Errorf("the Learn panel still says %q", gone)
		}
	}
}

// Moving a play between scopes carries its past; it does not rewrite it.
//
// # The failure this guards
//
// A play the person has edited reads `edited`, and the Plays surface says so: "you have changed
// this since Marco wrote it down". Writing the destination through SaveWithOrigin recomputes the
// digest from the source it is handed, so the move would recompute a match and the play would come
// out the other side reading `ready` — Marco quietly re-vouching for an artifact it never verified,
// with nothing on screen to say the fact had been lost.
//
// Mutation: replace reg.MoveOrigin in handleScope with reg.SaveWithOrigin. This fails.
func TestChangingScopeDoesNotReVerifyAnEditedPlay(t *testing.T) {
	e := newTestEditor(t)
	from := registerLearned(t, e, "settings", true, "open-mouse-settings")
	if err := os.WriteFile(e.reg.Path(from), []byte(playSrc+"-- mine now\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, _ := listPlays(t, e)
	if got := rowFor(t, rows, "open-mouse-settings", true).Life; got != "edited" {
		t.Fatalf("the fixture reads %q; it was supposed to be an edited play", got)
	}

	w := postJSON(t, e.handleScope, "/api/scope", map[string]any{
		"name": "open mouse settings", "slug": from.Slug,
		"curApp": "settings", "curScope": "focus", "scope": "context", "app": "settings",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/scope = %d: %s", w.Code, w.Body.String())
	}

	rows, _ = listPlays(t, e)
	got := rowFor(t, rows, "open-mouse-settings", true)
	if got.Life != "edited" {
		t.Errorf("after the move the play reads %q — moving it re-verified a play the person "+
			"had edited", got.Life)
	}
	if got.KindWord != "Learned" {
		t.Errorf("after the move the play reads %q, want Learned", got.KindWord)
	}
}

// Forgetting one row forgets one row.
//
// # The failure this guards
//
// A registered play and a staged play can share a name — and that is the ORDINARY position for a
// staged play, because `Register` refuses a collision and leaves it where it is. The Plays list
// shows them as two rows. Forgetting either row through `reg.Unregister` reached both files:
// Unregister means "Marco no longer has this play at all". So pressing forget on the saved row
// deleted the working play with no warning and a 200, and doing what the registry advised —
// "rename the learned play or remove the other one first" — destroyed the learned play it was
// telling you to preserve.
//
// Mutation: call reg.Unregister in either branch of handleDelete. This fails.
func TestForgettingAStagedPlayLeavesTheRegisteredOneAlone(t *testing.T) {
	e := newTestEditor(t)
	// Both under one name, which is exactly how a refused registration leaves things.
	registered := authored(t, e, "chrome", true, "open-tabs")
	if err := e.reg.SaveRecording(registered, []byte(`{"events":[]}`)); err != nil {
		t.Fatal(err)
	}
	stagePlay(t, e, "chrome", "open-tabs")
	if err := e.reg.Register(routes.Route{App: "chrome", Focus: routes.LearnedFocus, Slug: "open-tabs"}); err == nil {
		t.Fatal("the fixture registered; there was supposed to be a collision")
	}

	w := postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"slug": "open-tabs", "app": "chrome", "scope": "focus", "staged": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/delete (staged) = %d: %s", w.Code, w.Body.String())
	}

	// The staged copy is gone…
	if _, ok := plays.Find(e.reg, "open-tabs", "chrome", true); ok {
		t.Error("the staged play survived being forgotten")
	}
	// …and the play people actually use is untouched, recording and all.
	if !e.reg.Has(registered) {
		t.Fatal("forgetting the SAVED row deleted the REGISTERED play of the same name")
	}
	if !e.reg.HasRecording(registered) {
		t.Error("forgetting the saved row deleted the registered play's recording")
	}
	if _, ok := e.reg.Resolve("somewhere-else", "open tabs"); !ok {
		t.Error("the registered play stopped answering after the staged one was forgotten")
	}
}

// And the other direction: removing the play a learned one collided with must leave the learned
// one there, or the registry's own remedy destroys what it is protecting.
func TestForgettingARegisteredPlayLeavesAStagedOneAlone(t *testing.T) {
	e := newTestEditor(t)
	registered := authored(t, e, "chrome", false, "open-tabs")
	stagePlay(t, e, "chrome", "open-tabs")

	w := postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"slug": "open-tabs", "app": "chrome", "scope": "context", "staged": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/delete = %d: %s", w.Code, w.Body.String())
	}
	if e.reg.Has(registered) {
		t.Fatal("the registered play survived being forgotten")
	}
	staged, ok := plays.Find(e.reg, "open-tabs", "chrome", true)
	if !ok {
		t.Fatal("forgetting the registered play destroyed the learned play waiting behind it — " +
			"the collision the registry told you to resolve is now unresolvable")
	}
	if !staged.Life.Registerable() {
		t.Errorf("the saved play survived but cannot be registered: %q", staged.Life)
	}
	// And the remedy now works.
	if err := e.reg.Register(staged.Route); err != nil {
		t.Fatalf("registering after clearing the collision: %v", err)
	}
}

// A play that cannot be registered is not offered registration.
//
// The listing carries `registerable` as its own field, and the cheap wrong answer is !registered —
// which would put a Register button beside a staged play whose file has changed since Marco wrote
// down where it came from, and the registry refuses exactly that.
//
// Mutation: set Registerable from !p.Registered in handlePlays. This fails.
func TestAStuckPlayIsNotOfferedRegistrationInTheListing(t *testing.T) {
	e := newTestEditor(t)
	rt := stagePlay(t, e, "settings", "open-bluetooth")
	if err := os.WriteFile(e.reg.StagedPath(rt), []byte(playSrc+"-- changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, _ := listPlays(t, e)
	row := rowFor(t, rows, "open-bluetooth", false)
	if row.Registerable {
		t.Fatal("the listing offers Register for a play the registry will refuse to register")
	}
	if row.Askable {
		t.Error("a staged play is listed as askable")
	}
	if row.LifeSays == "" {
		t.Error("the row offers no button and no reason either")
	}
	// And the registry really would refuse, so the row was right.
	if err := e.reg.Register(rt); err == nil {
		t.Fatal("the registry accepted a play the listing said it would not")
	}
}

// The Learn panel shows a refusal as words, not as an identifier.
//
// Two of the Director's refusal codes spell "route" — the retired product noun — and the panel used
// to print the enum verbatim: "Refused: several_routes". Marco's own sentence about the refusal is
// already on screen above that line, so the label had no business being a symbol.
//
// Mutation: render v.refused raw again, or drop either LREFUSED entry. This fails.
func TestTheLearnPanelDoesNotPrintRawRefusalCodes(t *testing.T) {
	if !strings.Contains(editPage, "refusalWords(v.refused)") {
		t.Fatal("the Learn panel prints the refusal code without turning it into words")
	}
	// The two codes whose machine word is the retired product noun must be named explicitly;
	// the underscore fallback alone would still read "several routes" at a person.
	for _, code := range []string{"several_routes", "route_not_remembered"} {
		i := strings.Index(editPage, "LREFUSED={")
		if i < 0 {
			t.Fatal("there is no refusal vocabulary in the page")
		}
		tail := editPage[i:]
		end := strings.Index(tail, "};")
		if end < 0 || !strings.Contains(tail[:end], code+":") {
			t.Errorf("%s has no words, so the panel shows a person the code", code)
		}
	}
	// And every one of these codes really is produced by the Director, or the mapping is
	// guarding nothing.
	src := readRepoFile(t, "internal/director/learn/learn.go")
	for _, code := range []string{"several_routes", "route_not_remembered"} {
		if !strings.Contains(src, `"`+code+`"`) {
			t.Errorf("%s is not a refusal the Director raises; the mapping is stale", code)
		}
	}
}

// Forgetting a play from the CLI takes its provenance with it, exactly as the Plays surface does.
func TestForgettingAPlayTakesItsPastWithIt(t *testing.T) {
	e := newTestEditor(t)
	rt := registerLearned(t, e, "settings", true, "open-mouse-settings")
	origin := e.reg.OriginPath(rt)
	if _, err := os.Stat(origin); err != nil {
		t.Fatalf("the fixture has no provenance to lose: %v", err)
	}
	// The CLI.s own door, not a re-implementation of it.
	if err := forgetPlay(e.reg, rt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(origin); !os.IsNotExist(err) {
		t.Errorf("provenance survived the play it described (stat err %v)", err)
	}
	// AND IT STOPS THERE. A staged play of the same name is a different file, and being blocked
	// by a collision with this one is the ordinary reason it is still waiting — so forgetting
	// this play is what CLEARS that collision, not an instruction to destroy what was waiting.
	second := registerLearned(t, e, "settings", true, "open-bluetooth")
	stagePlay(t, e, "settings", "open-bluetooth")
	if err := forgetPlay(e.reg, second); err != nil {
		t.Fatal(err)
	}
	staged, ok := plays.Find(e.reg, "open-bluetooth", "settings", true)
	if !ok {
		t.Fatal("forgetting a play destroyed the saved play waiting behind its name")
	}
	if err := e.reg.Register(staged.Route); err != nil {
		t.Fatalf("the collision was cleared and registration still refuses: %v", err)
	}
}
