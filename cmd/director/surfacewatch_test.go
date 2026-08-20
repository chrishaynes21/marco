package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// Can a person watching tell "somewhere else in this application" from "somewhere else"?
//
// The distinction the local comparison exists to make, carried all the way to the sentence
// somebody reads. It is worth a test of its own because the segmenter can get it right and the
// account can still throw it away: `SameSurface` is assembled in one place, from a relation the
// states already carry, and a reader who cannot see it is no better off than before.
//
// These enter through the same registry and the same Playbill assembly as production. Nothing
// here constructs a View.

// The two surfaces these tests move between: one application whose content region is replaced,
// and a genuinely unrelated window. Named for their proportions rather than for any product —
// what decides the answer is how much persists, not whose software it is.
func aSurface() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 300, Content: 60, ContentRole: "list_item"}
}

func anUnrelatedSurface() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 12, Content: 220, ContentRole: "cell"}
}

// Somewhere else INSIDE an application reads as somewhere else inside it.
func TestWatchSaysAChangeStayedInsideOneApplication(t *testing.T) {
	g := newWatchRegistry(t)
	base, elsewhere := aSurface(), aSurface().ContentReplaced("checkbox")

	var frames []watchFrame
	frames = append(frames, watchFrames(base.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(elsewhere.Regions(), 6,
		[]observe.NavIntent{observe.NavConfirm})...)
	frames = append(frames, watchFrames(base.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(elsewhere.Regions(), 6, nil)...)

	v := watchedSession(t, g, frames)
	text := renderWatch(v.Watch())

	if v.Thinking.Changes == 0 {
		t.Fatalf("a surface whose content region was replaced reported no change:\n%s", text)
	}
	if !v.Thinking.SameSurface {
		t.Fatalf("two places inside one application were not reported as one surface:\n%s", text)
	}
	if !strings.Contains(text, "in the same application") {
		t.Errorf("Watch could not say the change stayed inside one application:\n%s", text)
	}

	// The account's claim IS the session's. A surface that decided this for itself could say
	// "same application" of a session that recorded two.
	view, ok := g.Snapshot("")
	if !ok {
		t.Fatal("the session vanished")
	}
	if n := surfacesIn(view.Stats.Shadow); n != 1 {
		t.Errorf("Watch says one surface; the session recorded %d", n)
	}
}

// Diagnostics carries the SECOND comparison, not only the first.
//
// "One screen, no transitions" has two very different causes — an application that held still,
// and one whose local changes were all beneath notice — and the whole-surface figures cannot
// tell them apart. The local numbers were measured for a whole milestone before anything read
// them, which is the failure mode this project keeps rediscovering: a mechanism that exists and
// is never called is indistinguishable from one that does not exist.
func TestDiagnosticsCarriesTheWithinScreenComparison(t *testing.T) {
	g := newWatchRegistry(t)
	base, elsewhere := aSurface(), aSurface().ContentReplaced("checkbox")

	var frames []watchFrame
	frames = append(frames, watchFrames(base.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(elsewhere.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(base.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(elsewhere.Regions(), 6, nil)...)

	v := watchedSession(t, g, frames)
	d := v.Diagnostics

	if d.LocalSeen == 0 {
		t.Fatal("a session of one surface reports no within-screen comparisons; " +
			"a reader cannot tell a still application from a blind one")
	}
	if d.LocalReplaced == 0 {
		t.Errorf("%d comparisons and no part ever read as replaced, in a session whose "+
			"content region was replaced twice", d.LocalSeen)
	}
	if !strings.Contains(renderWatch(v.Deep()), "within a screen:") {
		t.Error("the measurement is in the account but not in what anybody reads")
	}

	// Copied from the session, never re-derived.
	view, ok := g.Snapshot("")
	if !ok {
		t.Fatal("the session vanished")
	}
	m := view.Stats.Shadow.Match
	if d.LocalSeen != m.LocalSeen || d.LocalReplaced != m.LocalReplaced {
		t.Errorf("diagnostics says %d/%d; the session recorded %d/%d",
			d.LocalSeen, d.LocalReplaced, m.LocalSeen, m.LocalReplaced)
	}
}

// Somewhere unrelated does NOT get the reassuring sentence.
//
// The failure this guards against is the comfortable one: a phrase that always appears tells a
// reader nothing, and "in the same application" is exactly the kind of sentence that would go
// unnoticed if it were wrong in the direction of reassurance.
func TestWatchDoesNotClaimOneApplicationWhenTheWorldChanged(t *testing.T) {
	g := newWatchRegistry(t)
	here, there := aSurface(), anUnrelatedSurface()

	// Below the surface threshold, or this tests nothing.
	if s := observe.SignatureSimilarity(
		observe.NewScreenSignature(here.Regions()),
		observe.NewScreenSignature(there.Regions())); s >= observe.StateMatchSimilarity {
		t.Fatalf("the fixtures are not unrelated: they score %.3f against a bar of %.2f",
			s, observe.StateMatchSimilarity)
	}

	var frames []watchFrame
	frames = append(frames, watchFrames(here.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(there.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(here.Regions(), 6, nil)...)
	frames = append(frames, watchFrames(there.Regions(), 6, nil)...)

	v := watchedSession(t, g, frames)
	text := renderWatch(v.Watch())

	if v.Thinking.SameSurface {
		t.Errorf("a session that moved between unrelated windows was reported as one "+
			"application:\n%s", text)
	}
	if strings.Contains(text, "in the same application") {
		t.Errorf("Watch reassured the reader about a change that left the application:\n%s",
			text)
	}
}
