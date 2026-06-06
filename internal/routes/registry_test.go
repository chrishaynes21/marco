package routes

import (
	"sort"
	"testing"
)

func newReg(t *testing.T) Registry {
	return Registry{Dir: t.TempDir(), OS: "the OS is an act.\n"}
}

func TestScopePreference(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save("sea-of-thieves", "switch to sword", "a"))
	must(t, reg.Save("forza", "switch to sword", "b")) // same phrase, different app
	must(t, reg.Save("", "login to facebook", "c"))    // global

	// Focused app wins.
	if rt, ok := reg.Resolve("sea-of-thieves", "switch to sword"); !ok || rt.App != "sea-of-thieves" {
		t.Fatalf("current-app resolve = %+v %v", rt, ok)
	}
	// A global route resolves regardless of focus.
	if rt, ok := reg.Resolve("forza", "login to facebook"); !ok || rt.App != "" {
		t.Fatalf("global resolve = %+v %v", rt, ok)
	}
	// No app/global match → fall back to a scoped route (deterministic).
	if rt, ok := reg.Resolve("chrome", "switch to sword"); !ok || rt.App == "" {
		t.Fatalf("fallback resolve = %+v %v", rt, ok)
	}
	// Unknown command.
	if _, ok := reg.Resolve("forza", "make a sandwich"); ok {
		t.Fatal("unknown command resolved")
	}
}

func TestSlugsAndList(t *testing.T) {
	reg := newReg(t)
	must(t, reg.Save("sea-of-thieves", "switch to sword", "a"))
	must(t, reg.Save("forza", "switch to sword", "b"))
	must(t, reg.Save("", "login to facebook", "c"))

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
	must(t, reg.Save("forza", "drift", "x"))
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
