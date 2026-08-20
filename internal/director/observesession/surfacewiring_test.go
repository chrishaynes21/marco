package observesession_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Two truths at once, through the production session path.
//
//	the surface is still the same surface
//	the place inside it is not the place it was
//
// A single identity could carry only one of them, and the one it carried was the one that could
// not see a local change. These prove the pair survives a real Runner rather than existing only
// in the comparison that produces it.

// realistic is the shape the live measurement established: a large persistent surface with a
// smaller state-bearing region inside it.
func realisticSurface() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 300, Content: 60, ContentRole: "list_item"}
}

// surfaceScript drives one surface through a sequence of its own variations.
func surfaceScript(steps ...[]observe.ShadowRegion) script {
	var frames []frame
	for _, s := range steps {
		frames = append(frames, stayOn(s, 5)...)
	}
	return script{frames: frames}
}

// K. The enclosing surface stays the same while a meaningful state changes.
//
// THE milestone, in one test. Before this the two were the same question and the answer had to
// be one of "nothing happened" or "you are somewhere unrelated".
func TestASurfaceStaysItselfWhileItsStateChanges(t *testing.T) {
	base := realisticSurface()
	got := run(t, surfaceScript(
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
	))
	sh := got.Stats.Shadow

	if len(sh.States) < 2 {
		t.Fatalf("a surface whose content region was replaced produced %d state(s); "+
			"the change is invisible and nothing downstream can act on it", len(sh.States))
	}
	if len(sh.Transitions) == 0 {
		t.Fatal("two states of one surface produced no transition")
	}

	// The states know they belong to ONE surface.
	surfaces := map[observe.SurfaceID]int{}
	for _, st := range sh.States {
		surfaces[st.SurfaceOf()]++
	}
	if len(surfaces) != 1 {
		t.Errorf("two places inside one application were assigned %d surfaces: %v.\n"+
			"Marco would describe going to a settings page as arriving in an unrelated world",
			len(surfaces), surfaces)
	}
	t.Logf("%d states across %d surface(s)", len(sh.States), len(surfaces))
}

// C / D. Ordinary use of a surface does not move it.
//
// The stability half, and it is the one that must not regress: a viewport scrolling and a tree
// gaining and losing a few nodes are what an application does while somebody uses it.
func TestOrdinaryUseOfASurfaceProducesNoState(t *testing.T) {
	base := realisticSurface()
	got := run(t, surfaceScript(
		base.Regions(),
		base.Scrolled(12.5).Regions(),
		base.Churned(4).Regions(),
		base.Scrolled(40.5).Regions(),
		base.Churned(-6).Regions(),
		base.Regions(),
	))
	sh := got.Stats.Shadow

	if len(sh.States) != 1 {
		t.Fatalf("scrolling and churn produced %d states; every application would mint a "+
			"place every time somebody used it", len(sh.States))
	}
	if len(sh.Transitions) != 0 {
		t.Errorf("%d transitions from a surface nobody left", len(sh.Transitions))
	}
}

// A region growing by a lot of the SAME thing is not a new place.
//
// The case that forced the comparison to be about COMPOSITION rather than amount: a list loading
// more rows changes a substantial amount of structure — enough to clear any magnitude bar — while
// remaining entirely itself. If that became a state, every application with a lazily-loaded list
// would mint a place each time somebody reached the bottom.
//
// This test failed against an earlier model that measured how much the numbers moved, and it is
// what replaced it with one that asks whether a region is now made of something it was not.
func TestARegionGrowingByMoreOfTheSameIsNotANewPlace(t *testing.T) {
	base := realisticSurface()
	loaded := base
	loaded.Content += 40 // the list loaded another page of the same rows

	got := run(t, surfaceScript(
		base.Regions(), loaded.Regions(), base.Regions(), loaded.Regions(),
	))
	if n := len(got.Stats.Shadow.States); n != 1 {
		t.Fatalf("a list growing by 40 rows of the same kind produced %d states; every "+
			"application with a lazily-loaded list would mint a place per page", n)
	}
}

// I. A single changed frame is not a state.
//
// Persistence is what makes a thin threshold safe. Something that changed and changed back is a
// redraw; only something that STAYS is a place.
func TestASingleChangedFrameDoesNotBecomeAState(t *testing.T) {
	base := realisticSurface()
	var frames []frame
	frames = append(frames, stayOn(base.Regions(), 6)...)
	frames = append(frames, stayOn(base.ContentReplaced("checkbox").Regions(), 1)...)
	frames = append(frames, stayOn(base.Regions(), 8)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	if len(sh.States) != 1 {
		t.Errorf("one changed frame in fourteen became %d states", len(sh.States))
	}
}

// The other side of I: a composition seen TWICE is a place.
//
// Persistence is the whole safety margin on a thin local threshold, so the boundary it sits on
// has to be pinned from both directions. Only the "once is not enough" side was, and it held by
// construction — a first sighting has nothing to compare itself with, so it can never promote
// whatever the count says. That made the count itself untested: raising it survived every test
// here, and a session would quietly have needed three visits to somewhere before it existed.
//
// Twice, non-consecutively, with the surface returning in between — a person going somewhere,
// coming back, and going again, which is how anybody uses an application.
func TestACompositionSeenTwiceBecomesAState(t *testing.T) {
	base := realisticSurface()
	elsewhere := base.ContentReplaced("checkbox").Regions()

	var frames []frame
	frames = append(frames, stayOn(base.Regions(), 6)...)
	frames = append(frames, stayOn(elsewhere, 1)...)
	frames = append(frames, stayOn(base.Regions(), 4)...)
	frames = append(frames, stayOn(elsewhere, 1)...)
	frames = append(frames, stayOn(base.Regions(), 4)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	if len(sh.States) < 2 {
		t.Fatalf("somewhere visited twice produced %d state(s); a session needs more visits "+
			"than a person makes before it can hold what it saw", len(sh.States))
	}
}

// J. Neither state needs to be recognised, named or read.
func TestBothStatesOfASurfaceMayBeUnknown(t *testing.T) {
	base := realisticSurface()
	got := run(t, surfaceScript(
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
	))
	if len(got.Stats.Shadow.Transitions) == 0 {
		t.Fatal("no transition between two unknown states")
	}
	for _, st := range got.Stats.Shadow.States {
		if st.TermObservations != 0 {
			t.Errorf("state %s carries term evidence from a session that read nothing", st.ID)
		}
	}
	for _, p := range got.Proposals.Proposals {
		if p.Recognised {
			t.Error("a session with no memory recognised something")
		}
	}
}

// The two-level identity carries no content, and cannot.
//
// A surface relation is the first thing in this system that says two screens BELONG together,
// and "belong together" is exactly the kind of claim that tempts an implementation to key on
// something recognisable — a window title, a URL, an application name — because that would be
// so much easier than structure. Everything here is a counter or a grid coordinate by
// construction; this is what would notice the day it stopped being.
func TestTheSurfaceRelationCarriesNothingObservable(t *testing.T) {
	base := realisticSurface()
	got := run(t, surfaceScript(
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
		base.Regions(),
		base.ContentReplaced("checkbox").Regions(),
	))

	surfaceID := regexp.MustCompile(`^state_[0-9]+$`)
	cell := regexp.MustCompile(`^[0-9],[0-9]$`)
	for _, st := range got.Stats.Shadow.States {
		if s := string(st.SurfaceOf()); !surfaceID.MatchString(s) {
			t.Errorf("a surface is identified by %q, which is not a session counter", s)
		}
		if st.LocalFrom != "" && !surfaceID.MatchString(string(st.LocalFrom)) {
			t.Errorf("a state was distinguished from %q", st.LocalFrom)
		}
		if st.LocalCell != "" && !cell.MatchString(st.LocalCell) {
			t.Errorf("a change was located at %q, which is not a grid cell", st.LocalCell)
		}
	}

	// And the whole record still refuses content, with the new fields in it. Checked against
	// the fixture's own vocabulary: `list_item` and `checkbox` are roles and belong; anything
	// resembling a label, a title or a coordinate does not.
	blob, err := json.Marshal(got.Stats.Shadow)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		"local_text", "title", "label", "caption", "screenx", "screeny", "hwnd",
	} {
		if strings.Contains(strings.ToLower(string(blob)), forbidden) {
			t.Errorf("the session record names %q", forbidden)
		}
	}
}

// P. Moving or resizing the enclosing window preserves state identity.
//
// Everything downstream is window-relative and normalised, so a window that moves or scales
// proportionally is a no-op by construction. Asserted rather than assumed, because the local
// comparison is new and it reads geometry.
func TestMovingOrResizingTheWindowPreservesTheState(t *testing.T) {
	base := realisticSurface().Regions()
	// A proportional resize leaves normalised geometry unchanged; what a real one does is
	// reflow, which is churn, and that is covered above. What is asserted here is that a
	// sub-cell shift of everything — a border thickening, a scrollbar appearing — does not
	// move the state.
	shifted := screenfixture.Jitter(base, 0.01)
	got := run(t, surfaceScript(base, shifted, base, shifted))

	if n := len(got.Stats.Shadow.States); n != 1 {
		t.Errorf("a window shifting by 1%% produced %d states", n)
	}
}

var _ = context.Background
var _ = observesession.Result{}
