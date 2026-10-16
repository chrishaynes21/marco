package main

// THE PRESS POINT IS DECIDED HERE, AT THE LAST MOMENT, FROM WHAT IS ON SCREEN.
//
// `resolveRef` is the production executor's way of turning a planned reference into a concrete
// target, and it re-observes first: the element the plan named may have moved under the Director
// between resolution and acting, so the point is recomputed rather than carried.
//
// That is also how a demonstrated surface came to be silently discarded once —
// [[Experiment-058-one-question-one-answer-and-the-same-request-again]]'s second defect. Every
// stage of the record said the row and the click landed on the words, because this function
// rebuilt the point from the identity and knew nothing about a surface.
//
// So the rule here is the one the test harness in `internal/director/execute` also applies, and
// these are the cases that make them agree.

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// twoNestedProps is the measured Trackmania pair as a world: the words, and the row around them.
func twoNestedProps() directorapi.WorldState {
	words := &directorapi.Element{
		ID: "word", WindowID: "hwnd:1", Role: directorapi.RoleUnknown, Label: "SETTINGS",
		Bounds:  directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22},
		Enabled: true, Visible: true, Confidence: 0.99,
	}
	row := &directorapi.Element{
		ID: "row", WindowID: "hwnd:1", Role: directorapi.RoleUnknown,
		Bounds:  directorapi.Rect{X: 98, Y: 750, Width: 444, Height: 56},
		Enabled: true, Visible: true, Confidence: 0.99,
	}
	win := directorapi.WindowID("hwnd:1")
	return directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{
			words.ID: words, row.ID: row},
		Windows: []directorapi.Window{{ID: win, Application: "trackmania",
			Bounds:  directorapi.Rect{Width: 1920, Height: 1080},
			Focused: true, Visible: true}},
		ActiveWindow: &win,
	}
}

func TestADemonstratedSurfaceDecidesThePressPointAtTheMomentOfActing(t *testing.T) {
	w := twoNestedProps()
	observe := func(context.Context) (directorapi.WorldState, error) { return w, nil }
	words := w.Elements["word"]
	row := w.Elements["row"]

	for _, tc := range []struct {
		name string
		ref  directorapi.ElementReference
		want directorapi.Point
		// surface is the id the resolved target should report, empty for none.
		surface directorapi.ElementID
	}{
		{"no surface named", directorapi.ElementReference{ID: "word"},
			words.ClickPoint(), ""},
		{"the demonstrated row", directorapi.ElementReference{ID: "word",
			Surface: "row"}, row.ClickPoint(), "row"},
		// A SURFACE THAT HAS GONE leaves the identity's own middle standing, which is
		// where the press would have been with no demonstration at all. Never a
		// remembered number.
		{"a surface that is no longer there", directorapi.ElementReference{ID: "word",
			Surface: "vanished"}, words.ClickPoint(), ""},
		// AND NAMING THE IDENTITY AS ITS OWN SURFACE IS THE ORDINARY CASE, not a special
		// one: it is one of the two relations a demonstration can establish.
		{"the identity as its own surface", directorapi.ElementReference{ID: "word",
			Surface: "word"}, words.ClickPoint(), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveRef(observe, tc.ref)
			if err != nil {
				t.Fatalf("resolveRef: %v", err)
			}
			if got.Point != tc.want {
				t.Errorf("press at %v, want %v", got.Point, tc.want)
			}
			if got.Surface != tc.surface {
				t.Errorf("surface = %q, want %q", got.Surface, tc.surface)
			}
			// THE IDENTITY IS UNCHANGED EITHER WAY. A surface moves the point and
			// nothing else: the label, the role and the element are still the Prop
			// the request named.
			if got.ElementID != "word" || got.Label != "SETTINGS" {
				t.Errorf("the identity moved: %s %q", got.ElementID, got.Label)
			}
		})
	}
}

// AND A SURFACE WITH NO RECTANGLE IS NOT A SURFACE.
//
// The same reason `Region.Locates` refuses a band: a rectangle with no extent offers no point,
// and taking its middle would be arithmetic about nothing.
func TestASurfaceWithNoRectangleDoesNotMoveThePress(t *testing.T) {
	w := twoNestedProps()
	w.Elements["empty"] = &directorapi.Element{
		ID: "empty", WindowID: "hwnd:1", Role: directorapi.RoleUnknown,
		Enabled: true, Visible: true, Confidence: 0.99,
	}
	observe := func(context.Context) (directorapi.WorldState, error) { return w, nil }

	got, err := resolveRef(observe, directorapi.ElementReference{ID: "word",
		Surface: "empty"})
	if err != nil {
		t.Fatalf("resolveRef: %v", err)
	}
	if got.Point != w.Elements["word"].ClickPoint() {
		t.Errorf("press at %v, want the identity's own middle %v", got.Point,
			w.Elements["word"].ClickPoint())
	}
	if got.Surface != "" {
		t.Errorf("surface = %q, want none", got.Surface)
	}
}
