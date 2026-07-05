package routes

import "testing"

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

	// Rebinding replaces; unbinding removes.
	if err := r.Bind("notepad", "s", "other"); err != nil {
		t.Fatal(err)
	}
	if slug, _ := r.HotkeyCmd("notepad", "s"); slug != "other" {
		t.Fatalf("rebind = %q", slug)
	}
	if err := r.Unbind("notepad", "s"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.HotkeyCmd("notepad", "s"); ok {
		t.Fatal("`s should be unbound")
	}
}
