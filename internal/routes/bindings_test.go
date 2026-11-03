package routes

import (
	"os"
	"testing"
	"time"
)

func TestBindings(t *testing.T) {
	r := Registry{Dir: t.TempDir()}

	// Bind `s → say-hello in notepad; `s → confess globally.
	if err := r.Bind("notepad", "s", "say-hello"); err != nil {
		t.Fatal(err)
	}
	if err := r.Bind("", "g", "global-thing"); err != nil {
		t.Fatal(err)
	}

	// App-scoped resolves only in that app.
	if slug, ok := r.HotkeyCmd("notepad", "s"); !ok || slug != "say-hello" {
		t.Fatalf("notepad `s = %q,%v", slug, ok)
	}
	if _, ok := r.HotkeyCmd("chrome", "s"); ok {
		t.Fatal("`s should not resolve in chrome")
	}
	// Global resolves anywhere.
	if slug, ok := r.HotkeyCmd("chrome", "g"); !ok || slug != "global-thing" {
		t.Fatalf("global `g = %q,%v", slug, ok)
	}

	// Rebinding replaces.
	if err := r.Bind("notepad", "s", "other"); err != nil {
		t.Fatal(err)
	}
	if slug, _ := r.HotkeyCmd("notepad", "s"); slug != "other" {
		t.Fatalf("rebind = %q", slug)
	}
	// AND EVERY ENTRY THIS WRITES IS LEGACY: it names no routine, so it does not run. Production
	// assigns through orchestrator, which writes identities.
	if b, ok := r.BindingFor("notepad", "s"); !ok || !b.Legacy() {
		t.Fatalf("a name-only entry is not marked legacy: %+v", b)
	}
}

// A routine identity reads back as the route it spells, and nothing else does.
func TestParseIDReadsBackEveryScope(t *testing.T) {
	for _, rt := range []Route{{Slug: "copy"}, {App: "notepad", Slug: "save"},
		{App: "notepad", Focus: true, Slug: "save"}} {
		got, err := ParseID(rt.ID())
		if err != nil || got != rt {
			t.Errorf("ParseID(%q) = %+v, %v; want %+v", rt.ID(), got, err, rt)
		}
	}
	for _, bad := range []string{"", "save", "notepad/learned/save", "global/", "/focus/x"} {
		if _, err := ParseID(bad); err == nil {
			t.Errorf("ParseID(%q) accepted a non-identity", bad)
		}
	}
}

// The version changes when the entry changes, even with the same routine on the key.
func TestABindingVersionIsTheEntryNotTheRoutine(t *testing.T) {
	a := Binding{App: "notepad", Key: "e", Cmd: "enter freeplay with 3",
		Routes: []string{"notepad/focus/enter-freeplay"}}
	b := a
	b.Cmd = "enter freeplay with 4"
	if a.Version() == b.Version() {
		t.Error("two different bindings for the same routine share a version")
	}
	if a.Version() != a.Version() {
		t.Error("a version is not stable")
	}
}

// An unreadable file is refused, never written over as though it were empty.
func TestAnUnreadableShortcutsFileIsNotOverwritten(t *testing.T) {
	r := Registry{Dir: t.TempDir()}
	if err := os.WriteFile(r.bindingsPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := r.UpdateBindings(time.Second, func(bs []Binding) ([]Binding, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("an unreadable shortcuts file was written over")
	}
	raw, _ := os.ReadFile(r.bindingsPath())
	if string(raw) != "{not json" {
		t.Errorf("the file changed: %q", raw)
	}
}
