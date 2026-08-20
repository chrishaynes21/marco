package target

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// notepadLike reproduces what a live Notepad window actually exposes: File, Edit and
// View on the menu bar, a toolbar of formatting buttons — and NO Save anywhere,
// because Save does not exist as an element until the File menu is open.
//
// This was measured, not imagined. Inspecting the author's running Notepad found 21
// addressable controls and not one of them was the target of the Director's own
// canonical example command.
func notepadLike() directorapi.WorldState {
	return build(
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(-1800, 0, 1300, 900)),
		obs("uia:2", directorapi.RoleMenu, "", rect(-1784, 44, 159, 32)),
		obs("uia:3", directorapi.RoleMenuItem, "File", rect(-1780, 44, 41, 32)),
		obs("uia:4", directorapi.RoleMenuItem, "Edit", rect(-1731, 44, 44, 32)),
		obs("uia:5", directorapi.RoleMenuItem, "View", rect(-1679, 44, 50, 32)),
		obs("uia:6", directorapi.RoleButton, "Bold (Ctrl+B)", rect(-1183, 44, 32, 32)),
		obs("uia:7", directorapi.RoleButton, "Italic (Ctrl+I)", rect(-1151, 44, 32, 32)),
		obs("uia:8", directorapi.RoleButton, "Settings", rect(-516, 44, 30, 32)),
		obs("uia:9", directorapi.RoleTextField, "Text editor", rect(-1800, 90, 1300, 800)),
	)
}

// The Director's own canonical command — "click Save" — does not resolve in the
// application it was written about, because Save is hidden inside the File menu.
// Treating that as absence makes a large share of ordinary requests impossible, so
// absence must come with somewhere to look.
func TestHiddenMenuItemProducesADiscoveryPlan(t *testing.T) {
	w := notepadLike()

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionAbsent {
		t.Fatalf("status = %s; Save is genuinely not in the observed window", res.Status)
	}
	if res.Discovery.Empty() {
		t.Fatal("an absent target with unopened menus must come with a plan to look")
	}

	// File first: the conventional home of Save, and the ordering that makes the
	// usual case cost one probe instead of four.
	if got := res.Discovery.Probes[0].Label; got != "File" {
		t.Errorf("first probe = %q; Save conventionally lives under File", got)
	}
	// Every menu is still reachable — the convention orders the search, it does not
	// restrict it.
	labels := map[string]bool{}
	for _, p := range res.Discovery.Probes {
		labels[p.Label] = true
	}
	for _, want := range []string{"File", "Edit", "View"} {
		if !labels[want] {
			t.Errorf("the %s menu should still be probeable", want)
		}
	}
}

// Discovery acts on the UI in order to observe it, which is where an agent starts to
// wander. Every bound is checked here.
func TestDiscoveryIsBounded(t *testing.T) {
	// More menus than the probe cap.
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(0, 0, 40, 24)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(50, 0, 40, 24)),
		obs("uia:4", directorapi.RoleMenuItem, "View", rect(100, 0, 40, 24)),
		obs("uia:5", directorapi.RoleMenuItem, "Tools", rect(150, 0, 40, 24)),
		obs("uia:6", directorapi.RoleMenuItem, "Window", rect(200, 0, 50, 24)),
		obs("uia:7", directorapi.RoleMenuItem, "Help", rect(260, 0, 40, 24)),
		obs("uia:8", directorapi.RoleButton, "Go", rect(400, 0, 40, 24)),
	)

	plan := NewDiscoverer().Plan(&w, directorapi.ElementQuery{Label: "Save"})
	if plan == nil {
		t.Fatal("expected a plan")
	}
	if len(plan.Probes) > plan.MaxProbes {
		t.Errorf("%d probes exceeds the cap of %d", len(plan.Probes), plan.MaxProbes)
	}
	if plan.MaxProbes <= 0 {
		t.Error("a discovery plan must declare its own ceiling")
	}
	// Truncation has to be visible. A plan that silently drops half its search space
	// reads as "I looked everywhere" when it did not.
	if len(plan.Probes) < 6 && !contains(plan.Reason, "not tried") {
		t.Errorf("truncation must be stated in the reason, got %q", plan.Reason)
	}
	if plan.Risk != directorapi.RiskLow {
		t.Errorf("opening menus is low risk, got %q", plan.Risk)
	}

	for i, p := range plan.Probes {
		if p.Open == nil {
			t.Errorf("probe %d has no action to open it", i)
		}
		// Without cleanup a failed search leaves a menu hanging over the window, and
		// every later observation describes the menu instead of the application.
		if p.Cleanup == nil {
			t.Errorf("probe %d (%s) has no cleanup", i, p.Label)
		} else if p.Cleanup.ActionType() != directorapi.ActionKey {
			t.Errorf("probe %d cleanup should dismiss the menu, got %s", i, p.Cleanup.ActionType())
		}
	}
}

// Only containers that are cheap and reversible to open may be probed. Widening this
// to buttons would let "just looking" submit a form.
func TestDiscoveryOnlyProbesMenus(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Submit Payment", rect(0, 0, 120, 24)),
		obs("uia:3", directorapi.RoleTab, "Advanced", rect(130, 0, 80, 24)),
		obs("uia:4", directorapi.RoleListItem, "Item", rect(0, 40, 200, 20)),
		obs("uia:5", directorapi.RoleMenuItem, "File", rect(220, 0, 40, 24)),
	)

	plan := NewDiscoverer().Plan(&w, directorapi.ElementQuery{Label: "Save"})
	if plan == nil {
		t.Fatal("the File menu should be probeable")
	}
	for _, p := range plan.Probes {
		if p.Label != "File" {
			t.Errorf("probed %q; only menus may be opened during discovery", p.Label)
		}
	}
}

// A window with no menus has nowhere to look, and must say so rather than inventing
// somewhere.
func TestNoMenusMeansNoDiscovery(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 400, 300)),
		obs("uia:2", directorapi.RoleButton, "Open", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Close", rect(100, 10, 80, 24)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionAbsent {
		t.Fatalf("status = %s; want absent", res.Status)
	}
	if !res.Discovery.Empty() {
		t.Error("with no menus there is nowhere to look; discovery must not be proposed")
	}
}

// Discovery is only justified once the world has been observed well enough to say
// the target really is not visible. Proposing it for a window we cannot read would
// have the Director clicking around inside an application it cannot see.
func TestNoDiscoveryForAnUnobservableWorld(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "Discord", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
		obs("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
		obs("uia:4", directorapi.RolePane, "", rect(240, 40, 960, 760)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionUnobservable {
		t.Fatalf("status = %s; want unobservable", res.Status)
	}
	if res.Discovery != nil {
		t.Error("a world we cannot read must not be explored by clicking")
	}
}

// The plan must be deterministic: the same world and query produce the same probes
// in the same order, or a retry explores differently than the first attempt.
func TestDiscoveryPlanIsDeterministic(t *testing.T) {
	w := notepadLike()
	q := directorapi.ElementQuery{Label: "Save"}
	first := NewDiscoverer().Plan(&w, q)
	for range 5 {
		again := NewDiscoverer().Plan(&w, q)
		if len(again.Probes) != len(first.Probes) {
			t.Fatalf("probe count varies: %d vs %d", len(again.Probes), len(first.Probes))
		}
		for i := range first.Probes {
			if again.Probes[i].Label != first.Probes[i].Label {
				t.Fatalf("probe %d varies: %q vs %q", i, again.Probes[i].Label, first.Probes[i].Label)
			}
		}
	}
}

// A menu whose own name matches the request is the obvious first place to look.
func TestMenuNamedLikeTheTargetIsProbedFirst(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(0, 0, 40, 24)),
		obs("uia:3", directorapi.RoleMenuItem, "Bookmarks", rect(50, 0, 80, 24)),
	)
	plan := NewDiscoverer().Plan(&w, directorapi.ElementQuery{Label: "Bookmark this page"})
	if plan == nil || len(plan.Probes) == 0 {
		t.Fatal("expected a plan")
	}
	if plan.Probes[0].Label != "Bookmarks" {
		t.Errorf("first probe = %q; the menu named like the target should lead", plan.Probes[0].Label)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
