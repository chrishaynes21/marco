package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WATCHING IS NOT PERMISSION TO PRESS.
//
// # The conflation
//
// `Role.Clickable()` answers "may Marco treat this as a target for a click it performs
// itself". `pushActionables` used it to answer a different question — "what could the person
// have pressed" — and the two are not the same. The first is about authority and is
// conservative on purpose; the second is about belief, and the person has already pressed the
// thing.
//
// Measured live on Epic Games Launcher, whose accessibility tree offers one window: the
// detector found `menu "Library"` with current bounds, a person clicked it, and the press was
// attributed to the enclosing window. Nothing had rejected the pixel evidence — every gate in
// the path is sensor-blind, and RoleMenu is already on NameablePlaintext so its name was
// admissible. The candidate set was simply narrower than human observation requires.

// clickWorld is a focused window with one control inside it, which is the shape the defect
// needs: an enclosing thing that contains every point, and a specific thing that should win.
func clickWorld(inner *directorapi.Element) directorapi.WorldState {
	return directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"w": {Role: directorapi.RoleWindow, Label: "Epic Games Launcher", Visible: true,
			Focused: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}},
		"e": inner,
	}}
}

var clickFrame = directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}

// A human click on a freshly perceived menu is attributed to the menu, not to the window.
func TestAHumanClickOnAPerceivedMenuIsAttributedToIt(t *testing.T) {
	// PREMISE, asserted: the fix must not have worked by making RoleMenu executable.
	if directorapi.RoleMenu.Clickable() {
		t.Fatal("RoleMenu became clickable. Attribution was supposed to widen what may " +
			"EXPLAIN a human action, never what Marco may press of its own accord.")
	}

	s, press, sub := samplerWithNav(t)
	world := clickWorld(&directorapi.Element{
		Role: directorapi.RoleMenu, Label: "Library", Visible: true, Confidence: 0.9,
		Bounds: directorapi.Rect{X: 150, Y: 250, Width: 200, Height: 20}})

	s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true}, clickFrame, time.Now())
	s.pushActionables(context.Background(), world, directorapi.Window{}, 0, time.Now())
	press(200, 255) // inside the menu, and inside the window
	waitClassified(t, s.rt.navSource, 1)

	got := sub.Drain()
	if len(got) != 1 || got[0].Target == nil {
		t.Fatalf("the press did not resolve at all: %+v", got)
	}
	if got[0].Target.Role != string(directorapi.RoleMenu) {
		t.Fatalf("the press was attributed to a %s, not to the menu the person clicked.\n"+
			"An enclosing window contains every point on itself and explains nothing.",
			got[0].Target.Role)
	}
	if got[0].Target.Label != "Library" {
		t.Errorf("the menu was found and named %q; RoleMenu is on NameablePlaintext, so "+
			"its name should cross", got[0].Target.Label)
	}
}

// INERT CONTENT CANNOT EXPLAIN A CLICK AS A CONTROL.
//
// Something WROTE a heading; nobody operates it. A read word becoming the thing a person
// pressed is the same error the vision package refuses when it declines to turn text into a
// control, and widening the candidate set must not reintroduce it from the other side.
func TestInertContentCannotExplainAClick(t *testing.T) {
	for _, role := range []directorapi.ElementRole{
		directorapi.RoleText, directorapi.RoleHeading, directorapi.RoleImage,
	} {
		s, press, sub := samplerWithNav(t)
		world := clickWorld(&directorapi.Element{
			Role: role, Label: "SETTINGS", Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 150, Y: 250, Width: 200, Height: 20}})

		s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true},
			clickFrame, time.Now())
		s.pushActionables(context.Background(), world, directorapi.Window{}, 0, time.Now())
		press(200, 255)
		waitClassified(t, s.rt.navSource, 1)

		got := sub.Drain()
		if len(got) == 1 && got[0].Target != nil && got[0].Target.Role == string(role) {
			t.Errorf("a press was attributed to a %s reading %q. Reading a word never "+
				"creates a thing somebody pressed.", role, got[0].Target.Label)
		}
	}
}

// THE PREDICATE WIDENS BELIEF AND NOTHING ELSE.
func TestPointerAttributionIsWiderThanClickabilityAndGrantsNothing(t *testing.T) {
	// Everything Marco would press itself must remain explicable as a human action.
	for _, r := range []directorapi.ElementRole{
		directorapi.RoleButton, directorapi.RoleCheckbox, directorapi.RoleRadio,
		directorapi.RoleComboBox, directorapi.RoleListItem, directorapi.RoleMenuItem,
		directorapi.RoleTab, directorapi.RoleLink, directorapi.RoleIcon,
		directorapi.RoleToggle, directorapi.RoleTreeItem, directorapi.RoleRow,
	} {
		if !r.Clickable() {
			t.Fatalf("%s left Clickable; this test no longer covers what it claims", r)
		}
		if !r.PointerAttributable() {
			t.Errorf("%s may be clicked by Marco but cannot explain a human click", r)
		}
	}
	// The demonstrated widening, and the guarantee that comes with it.
	if !directorapi.RoleMenu.PointerAttributable() {
		t.Error("a menu cannot explain a human click, which is the defect this fixes")
	}
	if directorapi.RoleMenu.Clickable() {
		t.Error("RoleMenu became executable")
	}
	// Inert content and unknown roles explain nothing.
	for _, r := range []directorapi.ElementRole{
		directorapi.RoleText, directorapi.RoleHeading, directorapi.RoleImage,
		directorapi.RoleUnknown, directorapi.RoleProgressBar,
	} {
		if r.PointerAttributable() {
			t.Errorf("%s can explain a human click; a bounded shape is not a control", r)
		}
	}
	// SENSOR-AGNOSTIC BY CONSTRUCTION. The predicate is a method on a role and can reach
	// no provenance, so an accessibility menu and a pixel-derived menu are one case.
	if reflect.TypeOf(directorapi.RoleMenu.PointerAttributable).NumIn() != 0 {
		t.Error("PointerAttributable takes an argument; it can now be told where the " +
			"element came from, which is a sensor-specific rule waiting to happen")
	}
}

// THE DIAGNOSTIC SAYS WHAT ONE PRESS RESOLVED TO, AND CARRIES NO DESKTOP COORDINATE.
//
// Four diagnoses in this subsystem have been reconstructed from aggregate tallies and one of
// them was wrong: `0 landed on nothing` was true while the thing the person pressed had never
// been a candidate. A reader must be able to see one press and its candidates.
func TestOnePressIsExplainedByItsOwnRecord(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	world := clickWorld(&directorapi.Element{
		Role: directorapi.RoleMenu, Label: "Library", Visible: true, Confidence: 0.9,
		Bounds: directorapi.Rect{X: 150, Y: 250, Width: 200, Height: 20}})

	s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true}, clickFrame, time.Now())
	s.pushActionables(context.Background(), world, directorapi.Window{}, 0, time.Now())
	press(200, 255)
	waitClassified(t, s.rt.navSource, 1)
	_ = sub.Drain()

	recs := s.rt.navSource.Stats().PointerAttributions
	if len(recs) != 1 {
		t.Fatalf("%d press records, want 1: the diagnostic cannot explain a single click",
			len(recs))
	}
	r := recs[0]
	if r.Chosen < 0 || r.Chosen >= len(r.Candidates) {
		t.Fatalf("the record names no chosen candidate: %+v", r)
	}
	if r.Candidates[r.Chosen].Role != string(directorapi.RoleMenu) {
		t.Errorf("the record says the press landed on a %s", r.Candidates[r.Chosen].Role)
	}
	// The losing candidate is reported too — a record that showed only the winner could
	// not distinguish "the window was never offered" from "the window lost on size".
	if len(r.Candidates) < 2 {
		t.Errorf("only %d candidate recorded; the enclosing window contained the press "+
			"too and its absence from the record hides why the menu won", len(r.Candidates))
	}
	if r.Why == "" {
		t.Error("the record gives no reason for the decision")
	}
	// COORDINATES STAY TRANSIENT AND WINDOW-RELATIVE. A candidate carries a SIZE, which is
	// why one beat another, and never a position — nothing here may become a way to write
	// down where something was on the desktop.
	ct := reflect.TypeOf(observe.AttributionCandidate{})
	for _, banned := range []string{"X", "Y"} {
		if _, ok := ct.FieldByName(banned); ok {
			t.Errorf("AttributionCandidate carries %s. Absolute geometry does not cross "+
				"this boundary and must never reach a durable edge.", banned)
		}
	}
	if r.At.X < 0 || r.At.X > 1 || r.At.Y < 0 || r.At.Y > 1 {
		t.Errorf("the recorded press position %v is not window-relative", r.At)
	}
}
