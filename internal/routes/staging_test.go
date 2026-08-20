package routes

import (
	"os"
	"path/filepath"
	"testing"
)

const stagingSrc = "script main...\n  do nothing.\n"

// Enumerating the staging area finds every saved play and widens discovery by nothing.
//
// The named test on ListStaged. The companion assertion — that the PRODUCT listing still refuses
// to call these askable — lives in internal/plays, because that is the surface that could get it
// wrong; this one holds the store's half.
func TestStagedPlaysAreListedAndStayUnresolvable(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	app := Route{App: "settings", Focus: LearnedFocus, Slug: "open-mouse-settings"}
	global := Route{Slug: "app-less"}
	for _, rt := range []Route{app, global} {
		if err := reg.SaveStaged(rt, stagingSrc, Origin{Kind: KindLearned, Application: rt.App}); err != nil {
			t.Fatal(err)
		}
	}

	staged := reg.ListStaged()
	if len(staged) != 2 {
		t.Fatalf("ListStaged found %d plays, want 2: %v", len(staged), staged)
	}
	var sawApp, sawGlobal bool
	for _, rt := range staged {
		switch rt.Slug {
		case "open-mouse-settings":
			sawApp = true
			if rt.App != "settings" {
				t.Errorf("a play staged under settings lists under %q", rt.App)
			}
			if !rt.Focus {
				t.Error("a staged app play does not list as the focus play it will register as")
			}
		case "app-less":
			sawGlobal = true
			if rt.App != "" {
				t.Errorf("an app-less staged play lists under %q", rt.App)
			}
		}
	}
	if !sawApp || !sawGlobal {
		t.Fatalf("ListStaged missed a staging directory: app=%v global=%v", sawApp, sawGlobal)
	}

	// And none of it became discoverable.
	for _, rt := range reg.List() {
		if rt.Slug == "open-mouse-settings" || rt.Slug == "app-less" {
			t.Fatal("List can see a staged play")
		}
	}
	for _, name := range []string{"open mouse settings", "app less"} {
		if _, ok := reg.Resolve("settings", name); ok {
			t.Fatalf("%q resolved while staged", name)
		}
		if _, ok := reg.Resolve("", name); ok {
			t.Fatalf("%q resolved with nothing in front", name)
		}
	}
}

// Listing the staging area must not bring one into existence.
func TestListingStagedPlaysCreatesNoDirectory(t *testing.T) {
	dir := t.TempDir()
	reg := Registry{Dir: dir}
	if err := reg.Save(Route{App: "notepad", Slug: "by-hand"}, stagingSrc); err != nil {
		t.Fatal(err)
	}
	if got := reg.ListStaged(); len(got) != 0 {
		t.Fatalf("ListStaged found %v in a tree with nothing staged", got)
	}
	for _, p := range []string{
		filepath.Join(dir, "notepad", LearnedDir),
		filepath.Join(dir, GlobalDir, LearnedDir),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("listing created %s", p)
		}
	}
}

// Renaming a play takes its past with it.
func TestRenamingAPlayCarriesItsPast(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	from := Route{App: "settings", Focus: true, Slug: "old-name"}
	to := Route{App: "settings", Focus: true, Slug: "new-name"}
	if err := reg.SaveWithOrigin(from, stagingSrc, Origin{Kind: KindLearned, From: "a", To: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	kind, state := reg.KindOf(to)
	if kind != KindLearned || state != OriginIntact {
		t.Fatalf("the renamed play is %q/%q; renaming lost its past", kind, state)
	}
	if _, err := os.Stat(reg.OriginPath(from)); !os.IsNotExist(err) {
		t.Fatal("renaming left an orphaned sidecar under the old name")
	}
}

// The one decider says what an unbelievable past is worth: nothing.
func TestKindOfDoesNotBelieveAnUnreadableSidecar(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	rt := Route{App: "settings", Focus: true, Slug: "broken"}
	if err := reg.SaveWithOrigin(rt, stagingSrc, Origin{Kind: KindLearned}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg.OriginPath(rt), []byte(`{"version":99,"kind":"learned"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	kind, state := reg.KindOf(rt)
	if state != OriginUnreadable {
		t.Fatalf("state is %q, want unreadable", state)
	}
	if kind == KindLearned {
		t.Fatal("a sidecar this version cannot read still made its play learned")
	}
}
