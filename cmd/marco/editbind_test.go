package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The control centre's BIND, driven through the real handler over a real registry in a temp
// directory. The claim under test is not "the handler validates" — it is that the binding it
// stores is one the KEY PRESS can resolve, which is the only property that makes a hotkey work.

// postBind posts one binding through the real handler.
func postBind(t *testing.T, e *editor, app, key, cmd string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(map[string]string{"app": app, "key": key, "cmd": cmd})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	e.handleBind(w, httptest.NewRequest(http.MethodPost, "/api/bind", strings.NewReader(string(b))))
	return w
}

// A binding to a play that cannot be resolved is REFUSED, and nothing is stored.
//
// This is the defect the surface had: `findRouteByName` was consulted only to guess the app, so a
// miss was not a refusal — the binding landed as a global one pointing at nothing, the page said
// "bound", and the key did nothing for ever. `pressHotkey` skips a step it cannot resolve on
// purpose, which is exactly what made the bad binding silent.
func TestABindingThePressCouldNeverResolveIsRefused(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "notepad", false, "greet")

	for _, tc := range []struct{ name, cmd, names string }{
		{"no such play", "wave", "wave"},
		// The nastier one: the FIRST step exists, so the app is inferred and everything looks
		// right, while the second step can never fire.
		{"a chain with one dead step", "greet then wave", "wave"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := postBind(t, e, "", "e", tc.cmd)
			if w.Code == http.StatusOK {
				t.Fatalf("POST /api/bind reported success for %q: %s", tc.cmd, w.Body)
			}
			// The sentence names the STEP that could never fire — the only thing that tells the
			// person what to type instead.
			if body := w.Body.String(); !strings.Contains(body, tc.names) {
				t.Errorf("refusal did not name the unresolvable step %q: %q", tc.names, body)
			}
			if bs := e.reg.Bindings(); len(bs) != 0 {
				t.Fatalf("a refused binding was stored anyway: %+v", bs)
			}
		})
	}
}

// A binding to a play that DOES resolve is stored, in the scope the press will look in.
//
// The guard for "make it always refuse", and the positive half of the real claim: the binding is
// readable back through `HotkeyCmd` with the app the handler inferred, and every step of it
// resolves through the same `Resolve` the press makes.
func TestABindingIsStoredWhereThePressWillFindIt(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "notepad", false, "greet")
	authored(t, e, "notepad", false, "wave")

	w := postBind(t, e, "", "E", "greet then wave")
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/bind = %d: %s", w.Code, w.Body)
	}
	var got struct {
		Ok  bool   `json:"ok"`
		App string `json:"app"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode /api/bind: %v (%s)", err, w.Body)
	}
	// Scoped to the PLAY's app, because the foreground app here is the person's browser.
	if got.App != "notepad" {
		t.Errorf("binding scope = %q, want %q", got.App, "notepad")
	}
	cmd, ok := e.reg.HotkeyCmd("notepad", "e") // the key is stored lower-cased, as pressed
	if !ok {
		t.Fatalf("no binding for `e in notepad: %+v", e.reg.Bindings())
	}
	if cmd != "greet then wave" {
		t.Errorf("stored command = %q, want %q", cmd, "greet then wave")
	}
	// And it can actually fire: each step resolves with the SAME exact Resolve the press makes.
	for _, step := range routes.SplitChain(cmd) {
		base, _, _ := routes.ParseInvocation(step)
		if _, ok := e.reg.Resolve("notepad", base); !ok {
			t.Errorf("stored binding step %q does not resolve in the scope it was bound in", step)
		}
	}
}

// The app a binding is scoped to is inferred from the play the command STARTS with, arguments and
// chain and all — the same splitting a press does.
//
// A global play gives a global binding; a play in an app gives a binding in that app. The old
// hand-rolled " then " scan handed the whole step, arguments included, to the name lookup, so a
// binding with arguments was silently scoped global and then only fired outside its own app.
func TestABindingTakesItsScopeFromThePlayItStartsWith(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "notepad", false, "greet")
	authored(t, e, "", false, "everywhere")

	for _, tc := range []struct{ cmd, want string }{
		{"greet", "notepad"},
		{"greet name:sam", "notepad"}, // arguments are not part of the play's name
		{"greet then everywhere", "notepad"},
		{"everywhere", ""},
	} {
		if got := bindScope(e.reg, tc.cmd); got != tc.want {
			t.Errorf("bindScope(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

// THE CONTROL CENTRE GOES THROUGH THE SAME OWNER: a taken key is refused unless the page names the
// version it showed, forgetting a routine removes its shortcuts, and moving one carries them.
func TestTheControlCentreTakesAKeyOnlyWithConsentToWhatItShowed(t *testing.T) {
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "overlay.json"))
	e := newTestEditor(t)
	authored(t, e, "notepad", false, "greet")
	authored(t, e, "notepad", false, "wave")
	if w := postBind(t, e, "", "e", "greet"); w.Code != http.StatusOK {
		t.Fatalf("first bind = %d: %s", w.Code, w.Body)
	}
	if w := postBind(t, e, "", "e", "wave"); w.Code == http.StatusOK {
		t.Fatal("the control centre overwrote a taken key with no consent")
	}
	if b, _ := e.reg.BindingAt("notepad", "e"); b.Cmd != "greet" {
		t.Fatalf("the refused bind changed the key: %+v", b)
	}
	shown, _ := e.reg.BindingAt("notepad", "e")
	raw, _ := json.Marshal(map[string]string{"app": "", "key": "e", "cmd": "wave",
		"consent": shown.Version()})
	w := httptest.NewRecorder()
	e.handleBind(w, httptest.NewRequest(http.MethodPost, "/api/bind", strings.NewReader(string(raw))))
	if w.Code != http.StatusOK {
		t.Fatalf("a bind with consent to the shown version was refused: %d %s", w.Code, w.Body)
	}
}

func TestTheControlCentreForgetsAndCarriesShortcuts(t *testing.T) {
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "overlay.json"))
	e := newTestEditor(t)
	authored(t, e, "notepad", false, "greet")
	authored(t, e, "notepad", false, "wave")
	postBind(t, e, "", "g", "greet")
	postBind(t, e, "", "w", "wave")

	// Moving `greet` to From anywhere → notepad carries `g.
	w := postJSON(t, e.handleScope, "/api/scope", map[string]any{
		"name": "greet", "slug": "greet", "curApp": "notepad", "curScope": "context",
		"scope": "focus", "app": "notepad"})
	if w.Code != http.StatusOK {
		t.Fatalf("scope move = %d: %s", w.Code, w.Body)
	}
	if b, _ := e.reg.BindingAt("notepad", "g"); len(b.Routes) != 1 ||
		b.Routes[0] != "notepad/focus/greet" {
		t.Errorf("`g did not follow the move: %+v", b)
	}
	// Forgetting `wave` removes `w.
	w = postJSON(t, e.handleDelete, "/api/delete", map[string]any{
		"name": "wave", "slug": "wave", "app": "notepad", "scope": "context"})
	if w.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", w.Code, w.Body)
	}
	if _, ok := e.reg.BindingAt("notepad", "w"); ok {
		t.Error("forgetting wave left its shortcut")
	}
}

// A control-centre scope move whose shortcuts cannot follow moves nothing: the copy is undone and
// the original keeps its place and its shortcuts.
func TestTheControlCentreMoveCarriesBeforeItDeletes(t *testing.T) {
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "overlay.json"))
	e := newTestEditor(t)
	from := authored(t, e, "notepad", false, "greet")
	postBind(t, e, "", "g", "greet")
	if err := os.WriteFile(filepath.Join(e.reg.Dir, "bindings.json"), []byte("{broken"),
		0o644); err != nil {
		t.Fatal(err)
	}
	w := postJSON(t, e.handleScope, "/api/scope", map[string]any{
		"name": "greet", "slug": "greet", "curApp": "notepad", "curScope": "context",
		"scope": "focus", "app": "notepad"})
	if w.Code == http.StatusOK {
		t.Fatal("the move reported success although its shortcuts could not follow")
	}
	if !e.reg.Has(from) {
		t.Error("the original was removed")
	}
	if e.reg.Has(routes.Route{App: "notepad", Focus: true, Slug: "greet"}) {
		t.Error("the copy was left behind")
	}
}
