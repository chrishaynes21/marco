package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Light Mode is the surface that makes Marco's Accessibility perception understandable.
//
// These hold the two things that made it useless the first time it ran against real
// software: it described the wrong window, and it could not say what it could act on.

func el(role directorapi.ElementRole, label string, focused bool) *directorapi.Element {
	return &directorapi.Element{
		Role: role, Label: label, Visible: true, Confidence: 0.9, Focused: focused,
		Bounds: directorapi.Rect{X: 10, Y: 10, Width: 100, Height: 30},
	}
}

func worldOf(at time.Time, els map[directorapi.ElementID]*directorapi.Element) *directorapi.WorldState {
	return &directorapi.WorldState{Timestamp: at, Elements: els}
}

// Marco says what it could act on, by name — and withholds what it may not name.
//
// A count cannot answer "are you looking at the right screen"; a list of names answers it in
// one glance. The withheld ones are COUNTED rather than dropped, because "twelve things here
// I may not name" is the label gate showing its work.
func TestSightShowsWhatMarcoCanActOn(t *testing.T) {
	rt := &Runtime{}
	rt.lastWorld = worldOf(time.Now(), map[directorapi.ElementID]*directorapi.Element{
		// On the plaintext allowlist: named.
		"a": el(directorapi.RoleButton, "Add device", false),
		"b": el(directorapi.RoleButton, "Bluetooth", true),
		// Activatable but NOT on the allowlist: withheld outside a Learn licence.
		"c": el(directorapi.RoleListItem, "Bluetooth & devices", false),
		// Not actionable at all: absent entirely.
		"d": el(directorapi.RoleText, "some prose", false),
	})

	o := rt.offersFromWorld(false)
	if o.Actionable != 3 {
		t.Fatalf("actionable = %d, want 3 (two buttons and a list item; the text is not "+
			"something a person can aim at)", o.Actionable)
	}
	names := map[string]bool{}
	for _, n := range o.Named {
		names[n.Name] = true
	}
	if !names["Add device"] || !names["Bluetooth"] {
		t.Fatalf("named = %+v, want the allowlisted button names", o.Named)
	}
	if names["Bluetooth & devices"] {
		t.Error("a list item's text was named with no Learn licence in force")
	}
	if o.Withheld != 1 {
		t.Errorf("withheld = %d, want 1 — a name Marco may not say is counted, not dropped",
			o.Withheld)
	}
	if o.Focused.Name != "Bluetooth" {
		t.Errorf("focused = %+v, want the focused control", o.Focused)
	}

	// Under an explicit Learn licence the activatable control may be named — the SAME
	// widening ADR-058 made for semantic targets, not a second one.
	licensed := rt.offersFromWorld(true)
	found := false
	for _, n := range licensed.Named {
		if n.Name == "Bluetooth & devices" {
			found = true
		}
	}
	if !found {
		t.Errorf("under a Learn licence the activated control is still unnamed: %+v",
			licensed.Named)
	}
}

// A name that looks like somebody's content is withheld even when its ROLE is allowlisted.
//
// Measured live: Windows Settings exposes the signed-in account as a `button` whose label is
// the person's name and email address. The role gate passes it; the shape filter is what
// stops it, and it is the reason the two gates are not one.
func TestSightWithholdsPrivateTextEvenFromAnAllowlistedRole(t *testing.T) {
	rt := &Runtime{}
	rt.lastWorld = worldOf(time.Now(), map[directorapi.ElementID]*directorapi.Element{
		"a": el(directorapi.RoleButton, "Chris Haynes chris.haynes2112@gmail.com", false),
	})
	for _, licensed := range []bool{false, true} {
		o := rt.offersFromWorld(licensed)
		for _, n := range o.Named {
			t.Errorf("licensed=%v named %q — an address is content, whatever it is "+
				"attached to", licensed, n.Name)
		}
		if o.Withheld != 1 {
			t.Errorf("licensed=%v withheld = %d, want 1", licensed, o.Withheld)
		}
	}
}

// Light Mode describes the window being WATCHED, not whatever is in front.
//
// THE live defect: a session fuses its own world for its pinned window and never touches the
// foreground pipeline's, so the surface reported "1 control I could aim at" while watching a
// Settings window offering forty. A perception surface describing a different window than
// the one Marco is watching is worse than no surface — it is confidently wrong.
//
// Deleting the lastWatched assignment in liveSampler.Sample must fail this.
func TestLightModeDescribesTheWatchedWindow(t *testing.T) {
	rt := &Runtime{}
	older := time.Now().Add(-2 * time.Second)
	// The foreground: one anonymous pane, which is what a game reports.
	rt.lastWorld = worldOf(older, map[directorapi.ElementID]*directorapi.Element{
		"p": el(directorapi.RolePane, "", false),
	})
	// The session's own world, fused a moment ago.
	rt.lastWatched = worldOf(time.Now(), map[directorapi.ElementID]*directorapi.Element{
		"a": el(directorapi.RoleButton, "Add device", false),
		"b": el(directorapi.RoleButton, "Bluetooth", false),
	})

	o := rt.offersFromWorld(false)
	if o.Actionable != 2 {
		t.Fatalf("actionable = %d, want 2 — the surface is describing the foreground "+
			"instead of the watched window", o.Actionable)
	}

	// And the rule is FRESHNESS, not "a session exists": once the foreground observation
	// is the newer one, that is what a person is looking at.
	rt.lastWorld = worldOf(time.Now().Add(time.Second),
		map[directorapi.ElementID]*directorapi.Element{
			"x": el(directorapi.RoleButton, "Play", false),
		})
	if o := rt.offersFromWorld(false); o.Actionable != 1 {
		t.Errorf("actionable = %d, want 1 — the fresher world should win", o.Actionable)
	}
}
