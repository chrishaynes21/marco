package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ONE COLD LEARN LEFT TWO DURABLE PLACES FOR ONE SETTINGS PAGE NOBODY TOUCHED.
//
// # The sequence, measured live on 2026-08-28 with $MARCO_VISION_MODEL configured
//
//	the pass begins, nothing is settled
//	  -> the escalation gate does not know whether the reading suffices, so it buys the
//	     visual pass (ADR-104: ignorance is not a decline)
//	  -> 21 detector boxes join the composition as `icon`, and one element accessibility
//	     reported as `unknown` is renamed `icon` — fusion treats a generic role as no claim,
//	     so the first specific claim wins and the detector makes it
//	  -> that composition settles
//	  -> the reading is now placed and sufficient, so the gate stops buying
//	  -> the composition changes back
//	  -> the segmenter sees a coherent part of the surface replaced and calls it a new state
//	  -> the licence is still open, so PlacesToEstablish makes BOTH durable
//
//	subj_e8447bc75334   … icon 22 …      established first, matched never again
//	subj_71727a02470f   … unknown 1 …    established second, and what everything resolves to
//
// Marco concluded that the world had changed when only its evidence had.
//
// # Why buildSample and not the helper
//
// Because 37E, 37F and 37G each found a correct mechanism wired to nothing, or wired to the
// wrong caller. `observation.KindFromPixels` being right says nothing about whether the sample
// production builds carries the answer, and that is the half that has been wrong three times.

const pixelWin = directorapi.WindowID("win_1")

func pixelObs(id string, src directorapi.ObservationSource, role directorapi.ElementRole,
	x int) directorapi.Observation {

	return directorapi.Observation{
		ID: directorapi.ObservationID(id), Source: src, Role: role,
		Bounds:   directorapi.Rect{X: x, Y: 100, Width: 40, Height: 20},
		WindowID: pixelWin, Timestamp: time.Unix(0, 0),
	}
}

func pixelEl(id string, role directorapi.ElementRole, x int,
	refs ...directorapi.ObservationReference) *directorapi.Element {

	return &directorapi.Element{
		ID: directorapi.ElementID(id), WindowID: pixelWin, Role: role,
		Bounds:     directorapi.Rect{X: x, Y: 100, Width: 40, Height: 20},
		Enabled:    true,
		Visible:    true,
		Confidence: 0.9,
		Provenance: directorapi.Provenance{Sources: refs},
	}
}

func pixelRef(id string, src directorapi.ObservationSource) directorapi.ObservationReference {
	return directorapi.ObservationReference{
		Observation: directorapi.ObservationID(id), Source: src,
	}
}

// pixelWorld is one screen as fusion presents it when a detector is running.
//
// Three kinds of element, and each is a different way for a detector's word to reach identity:
//
//	a button accessibility named        the ordinary case; the detector only corroborates
//	an element accessibility called     `unknown` is not a claim, so the detector gets to name
//	  `unknown` that the detector          it — and `unknown` is a layout role while `icon` is
//	  named `icon`                         not, so this one element alone moves the role SET
//	a detection with nothing beside it  pixels and nothing else
func pixelWorld(withDetector bool) (directorapi.WorldState, observation.Cycle) {
	raw := []directorapi.Observation{
		pixelObs("a-save", directorapi.SourceAccessibility, directorapi.RoleButton, 100),
		pixelObs("a-blob", directorapi.SourceAccessibility, directorapi.RoleUnknown, 200),
		pixelObs("a-note", directorapi.SourceAccessibility, directorapi.RoleText, 300),
	}
	elements := map[directorapi.ElementID]*directorapi.Element{
		"save": pixelEl("save", directorapi.RoleButton, 100,
			pixelRef("a-save", directorapi.SourceAccessibility)),
		"blob": pixelEl("blob", directorapi.RoleUnknown, 200,
			pixelRef("a-blob", directorapi.SourceAccessibility)),
		"note": pixelEl("note", directorapi.RoleText, 300,
			pixelRef("a-note", directorapi.SourceAccessibility)),
	}
	if withDetector {
		raw = append(raw,
			pixelObs("v-save", directorapi.SourceVision, directorapi.RoleButton, 100),
			pixelObs("v-blob", directorapi.SourceVision, directorapi.RoleIcon, 200))
		// The detector corroborates the button — accessibility still named it.
		elements["save"] = pixelEl("save", directorapi.RoleButton, 100,
			pixelRef("a-save", directorapi.SourceAccessibility),
			pixelRef("v-save", directorapi.SourceVision))
		// And NAMES the thing accessibility could not.
		elements["blob"] = pixelEl("blob", directorapi.RoleIcon, 200,
			pixelRef("a-blob", directorapi.SourceAccessibility),
			pixelRef("v-blob", directorapi.SourceVision))
		for i, x := range []int{400, 460, 520} {
			id := fmt.Sprintf("v-only-%d", i)
			raw = append(raw, pixelObs(id, directorapi.SourceVision, directorapi.RoleIcon, x))
			elements[directorapi.ElementID(id)] = pixelEl(id, directorapi.RoleIcon, x,
				pixelRef(id, directorapi.SourceVision))
		}
	}
	cycle := observation.Cycle{}
	for _, o := range raw {
		cycle.Observations = append(cycle.Observations, observation.NewElement(o))
	}
	return directorapi.WorldState{Elements: elements}, cycle
}

func compositionOf(t *testing.T, withDetector bool) map[string]int {
	t.Helper()
	world, cycle := pixelWorld(withDetector)
	sample := buildSample(world, cycle,
		directorapi.Window{ID: pixelWin, Bounds: directorapi.Rect{Width: 1920, Height: 1080}},
		sampleRequest())
	// THE SIGNATURE, not the regions. The regions deliberately still carry everything —
	// KindFromPixels is a label, not a removal, exactly like Chrome — and the one consumer
	// that reads it is the durable place signature. Asserting on the regions would pass
	// against a build that had lost the filter entirely.
	return observe.NewScreenSignature(sample.Structure.Regions).Roles
}

// The composition is the same screen whether or not the detector ran.
//
// Deleting the KindFromPixels line in buildSample must fail this. So must reverting
// observation.KindFromPixels to ask only whether a structural source described the element,
// which was the first version and which removed every accessibility text node from the
// composition — `text x29` and `unknown x1` off one real Settings page.
func TestTheSampleProductionBuildsSaysWhoNamedEachThing(t *testing.T) {
	primary := compositionOf(t, false)
	rich := compositionOf(t, true)

	if len(primary) == 0 {
		t.Fatal("the primary reading produced no composition at all; the fixture is wrong")
	}
	if diff := roleDiff(primary, rich); diff != "" {
		t.Errorf("the detector running changed what the screen is made of:%s\n"+
			"Nothing on the screen moved. A composition that answers differently because a "+
			"sensor appeared makes the perception budget into a semantic transition, and one "+
			"cold Learn then leaves two durable Places for one page.", diff)
	}
	// And the accessibility account is all there, including the parts accessibility itself
	// could not name. `text` and `unknown` are poor claims; they are not absent evidence.
	for _, want := range []string{"button", "unknown", "text"} {
		if primary[want] != 1 {
			t.Errorf("the primary composition has %d %q, want 1 — an element accessibility "+
				"described is part of the screen even when it could not say what it was",
				primary[want], want)
		}
	}
}

// Where pixels are the only account of anything, they are the composition.
//
// The case the detector exists for: a game, a canvas, an application with no accessibility
// tree. Excluding pixel-named structures there would leave the screen made of nothing, and
// nothing is not recognisable — which would take game perception back to before it worked.
func TestAScreenNothingButPixelsDescribedIsStillAComposition(t *testing.T) {
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	cycle := observation.Cycle{}
	for i, x := range []int{100, 200, 300, 400} {
		id := fmt.Sprintf("v-%d", i)
		o := pixelObs(id, directorapi.SourceVision, directorapi.RoleIcon, x)
		cycle.Observations = append(cycle.Observations, observation.NewElement(o))
		world.Elements[directorapi.ElementID(id)] = pixelEl(id, directorapi.RoleIcon, x,
			pixelRef(id, directorapi.SourceVision))
	}
	sample := buildSample(world, cycle,
		directorapi.Window{ID: pixelWin, Bounds: directorapi.Rect{Width: 1920, Height: 1080}},
		sampleRequest())
	if len(sample.Structure.Regions) != 4 {
		t.Fatalf("a window only a detector could describe produced %d structural regions, "+
			"want 4.\nThe filter is meant to prefer a structural account where there is one, "+
			"not to delete the reading where there is not.", len(sample.Structure.Regions))
	}
}

func roleDiff(a, b map[string]int) string {
	out := ""
	seen := map[string]bool{}
	for _, m := range []map[string]int{a, b} {
		for k := range m {
			if seen[k] {
				continue
			}
			seen[k] = true
			if a[k] != b[k] {
				out += fmt.Sprintf(" %s=%d/%d", k, a[k], b[k])
			}
		}
	}
	return out
}
