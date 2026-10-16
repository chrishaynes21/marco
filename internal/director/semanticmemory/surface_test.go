package semanticmemory_test

// WHERE A PERSON SAID THE PRESS GOES, ACROSS A RESTART.
//
// The store's half of [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]]. What is
// asserted here is durability and the three refusals: a record about a target nothing knows, a
// second record for one target, and a record whose target has since been forgotten.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// aTarget establishes a place and a semantic target in it, the canonical way, and returns the
// target's durable id.
func aTarget(t *testing.T, s *semanticmemory.Store, label string) string {
	t.Helper()
	place, err := s.EstablishPlace("unknown-game", settingsSignature())
	if err != nil {
		t.Fatalf("EstablishPlace: %v", err)
	}
	id, err := s.RememberTarget("unknown-game",
		observe.TargetSignature(place, label, observe.KindButton),
		observe.FromAccessible)
	if err != nil {
		t.Fatalf("RememberTarget: %v", err)
	}
	return id
}

// theRow is the record a settled press on a menu row produces, with its evidence.
func theRow(target string, c observe.SurfaceConfirmation) observe.DemonstratedSurface {
	return observe.DemonstratedSurface{
		Target:   target,
		Relation: observe.RelationContainingRegion,
		Depth:    1,
		Shape: observe.SurfaceShape{
			Surface:     observe.Region{X: 0.051, Y: 0.694, Width: 0.231, Height: 0.052},
			Identity:    observe.Region{X: 0.202, Y: 0.707, Width: 0.084, Height: 0.020},
			Containment: 1, AreaShare: 0.012, Kind: "panel", Fill: 0.81, Parts: 12,
			Siblings: 4, DistinctNames: 4,
		},
		Confirmed: c, By: "chris",
		At: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
	}
}

// A DEMONSTRATED SURFACE SURVIVES A RESTART.
func TestADemonstratedSurfaceSurvivesAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")

	first := open(t, path)
	id := aTarget(t, first, "Mouse")
	if err := first.RememberSurface("unknown-game",
		theRow(id, attributedCrossing(t))); err != nil {
		t.Fatalf("RememberSurface: %v", err)
	}

	// A different process, as far as this store is concerned.
	second := open(t, path)
	got := second.DemonstratedSurfaces("unknown-game")
	if len(got) != 1 {
		t.Fatalf("%d surface(s) after reopening, want 1", len(got))
	}
	d, why := observe.SurfaceFor(got, "unknown-game", id)
	if why != "" {
		t.Fatalf("the stored surface does not apply to its own target: %s", why)
	}
	if d.Relation != observe.RelationContainingRegion || d.Depth != 1 {
		t.Errorf("the relation did not survive: %q at depth %d", d.Relation, d.Depth)
	}
	if ok, why := d.Usable(); !ok {
		t.Errorf("it came back unusable: %s", why)
	}
	// AND THE EVIDENCE CAME BACK TOO, which is what lets a later reading disagree.
	if d.Shape.AreaShare == 0 || d.Shape.Parts == 0 || d.Shape.Siblings == 0 {
		t.Errorf("the supporting evidence did not survive: %+v", d.Shape)
	}
	// AND IT IS NAMESPACED BY APPLICATION, like everything else here.
	if n := len(second.DemonstratedSurfaces("another-game")); n != 0 {
		t.Errorf("%d surface(s) leaked into another application", n)
	}
}

// AND THE FILE HOLDS NO COORDINATE AND NO LABEL.
//
// The reason the target is an id. `TestNothingCapturedCanReachDurableMemory` walks the type, and
// this walks the BYTES — the two catch different mistakes, and a serialised form is what somebody
// actually reads when they want to know what Marco kept about their screen.
func TestTheStoredFileHoldsNoDesktopCoordinateAndNoControlName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	id := aTarget(t, s, "Mouse")
	if err := s.RememberSurface("unknown-game",
		theRow(id, attributedCrossing(t))); err != nil {
		t.Fatalf("RememberSurface: %v", err)
	}

	raw, err := readFile(path)
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	// The surfaces section is what this is about; the target subject above legitimately
	// carries the label under the Learn licence, so the assertion is about the record.
	body := raw[strings.Index(raw, `"surfaces"`):]
	for _, forbidden := range []string{"hwnd", "point", "bounds", "Mouse", "SETTINGS"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the stored surface contains %q:\n%s", forbidden, body)
		}
	}
	t.Logf("%s", body)
}

// A SURFACE FOR A TARGET NOTHING KNOWS IS NOT STORED.
//
// The id is the whole of how it will be found again, so a record pointing at nothing is a saved
// aiming decision with nothing to aim at.
func TestASurfaceForAnUnknownTargetIsNotStored(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "memory.json"))
	err := s.RememberSurface("unknown-game",
		theRow("subject-nobody-has", attributedCrossing(t)))
	if err == nil {
		t.Fatal("a surface for an unknown target was stored")
	}
	if !strings.Contains(err.Error(), "could never be found again") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	if n := len(s.DemonstratedSurfaces("unknown-game")); n != 0 {
		t.Errorf("%d surface(s) stored anyway", n)
	}
}

// AND A RECORD IT CANNOT VOUCH FOR IS REFUSED RATHER THAN REPAIRED.
func TestAnIncoherentSurfaceIsRefused(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "memory.json"))
	id := aTarget(t, s, "Mouse")

	for _, tc := range []struct {
		name string
		edit func(*observe.DemonstratedSurface)
	}{
		{"no relation", func(d *observe.DemonstratedSurface) { d.Relation = "" }},
		{"an invented relation", func(d *observe.DemonstratedSurface) {
			d.Relation = "somewhere about there"
		}},
		{"the identity's region at depth 1", func(d *observe.DemonstratedSurface) {
			d.Relation = observe.RelationIdentityRegion
			d.Depth = 1
		}},
		{"a containing region at depth 0", func(d *observe.DemonstratedSurface) {
			d.Depth = 0
		}},
		{"nobody demonstrated it", func(d *observe.DemonstratedSurface) { d.By = "" }},
		{"no time", func(d *observe.DemonstratedSurface) { d.At = time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := theRow(id, attributedCrossing(t))
			tc.edit(&d)
			if err := s.RememberSurface("unknown-game", d); err == nil {
				t.Error("it was stored")
			}
		})
	}
	if n := len(s.DemonstratedSurfaces("unknown-game")); n != 0 {
		t.Errorf("%d incoherent surface(s) stored", n)
	}
}

// A SECOND DEMONSTRATION REPLACES THE FIRST.
//
// A person who changes their mind has corrected a fact, not added one — and two records for one
// target is the contradiction `observe.SurfaceFor` refuses rather than resolves, so the store must
// never produce the situation.
func TestASecondDemonstrationReplacesTheFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	id := aTarget(t, s, "Mouse")

	if err := s.RememberSurface("unknown-game",
		theRow(id, attributedCrossing(t))); err != nil {
		t.Fatalf("first: %v", err)
	}
	corrected := theRow(id, attributedCrossing(t))
	corrected.Relation = observe.RelationIdentityRegion
	corrected.Depth = 0
	corrected.By = "someone else"
	if err := s.RememberSurface("unknown-game", corrected); err != nil {
		t.Fatalf("second: %v", err)
	}

	got := open(t, path).DemonstratedSurfaces("unknown-game")
	if len(got) != 1 {
		t.Fatalf("%d record(s) for one target, want 1: %+v", len(got), got)
	}
	d, why := observe.SurfaceFor(got, "unknown-game", id)
	if why != "" {
		t.Fatalf("the surviving record does not apply: %s", why)
	}
	if d.Relation != observe.RelationIdentityRegion || d.By != "someone else" {
		t.Errorf("the correction did not win: %+v", d)
	}
}

// AND AN UNCONFIRMED DEMONSTRATION IS KEPT, AND REFUSES TO AIM.
//
// The owner's rule held in the two places it belongs: the store keeps the evidence, because
// throwing it away would make the person show Marco again for nothing, and reading it says no.
func TestAnUnconfirmedDemonstrationIsKeptAndRefusesToAim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	id := aTarget(t, s, "Mouse")

	if err := s.RememberSurface("unknown-game",
		theRow(id, pressOnly(t))); err != nil {
		t.Fatalf("RememberSurface: %v", err)
	}
	got := open(t, path).DemonstratedSurfaces("unknown-game")
	if len(got) != 1 {
		t.Fatalf("%d record(s), want the demonstration kept", len(got))
	}
	if ok, why := got[0].Usable(); ok {
		t.Error("an unconfirmed demonstration is usable")
	} else if !strings.Contains(why, "observed") {
		t.Errorf("the refusal does not name what was missing: %s", why)
	}
}

// A SURFACE WHOSE TARGET IS FORGOTTEN GOES WITH IT, AT LOAD.
//
// The same referential rule every other record here takes, and here it is load-bearing: a
// surface record is where a press GOES, so one whose target is gone is the most dangerous kind of
// orphan this file could hold.
func TestASurfaceWhoseTargetIsForgottenIsDroppedAtLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	id := aTarget(t, s, "Mouse")
	if err := s.RememberSurface("unknown-game",
		theRow(id, attributedCrossing(t))); err != nil {
		t.Fatalf("RememberSurface: %v", err)
	}

	// The subject removed by hand, which is what editing or pruning the file looks like.
	raw, err := readFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	gutted := strings.Replace(raw, `"`+id+`"`, `"a-different-id"`, 1)
	if gutted == raw {
		t.Fatalf("the target id %q is not in the file, so this test edits nothing", id)
	}
	if err := writeFile(path, gutted); err != nil {
		t.Fatalf("writing: %v", err)
	}

	reopened := open(t, path)
	if n := len(reopened.DemonstratedSurfaces("unknown-game")); n != 0 {
		t.Errorf("%d orphaned surface(s) survived the load", n)
	}
	if reopened.Orphaned() == 0 {
		t.Error("and the drop was not counted, so a store that lost it would say nothing")
	}
}

// readFile and writeFile are the store's own file, as a person editing it would see it.
func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}

// attributedCrossing is a confirmation built the only way one can be: from a crossing witness
// through `observe.TransitionAttributed`. No test here asserts a level.
func attributedCrossing(t *testing.T) observe.SurfaceConfirmation {
	t.Helper()
	c, err := observe.TransitionAttributed(observe.TransitionWitness{
		Application: "fixture", From: "subj_menu", To: "subj_settings",
		ByHuman: true, Acts: 1, At: time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("building an attributed confirmation: %v", err)
	}
	return c
}

// pressOnly is the weaker one: a press was seen and nothing confirmed its effect.
func pressOnly(t *testing.T) observe.SurfaceConfirmation {
	t.Helper()
	c, err := observe.PressWasObserved("a press landed on one surface and not the other")
	if err != nil {
		t.Fatalf("building an observed confirmation: %v", err)
	}
	return c
}
