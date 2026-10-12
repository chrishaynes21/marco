package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// A session says how many of the active list's rows the last look did not show.
//
// # What this is for, and what it is deliberately NOT
//
// A selection claim is made about the rows of this look that fall inside the list, so it can
// only ever name one of those. If the list HAS a row this look did not show, the mark may be on
// it and the claim could not have been right. Counting those apart is the difference between
// the reader's error and the list's.
//
// It is NOT a remembered row count, and that distinction is the whole design. "This list once
// had nine rows" is a false oracle: a menu may legitimately change, and the ninth row may be
// gone rather than missed. What is reported is the membership as it stands, with each member's
// own staleness behind it — so a row that genuinely left stops being a member after
// MaxUnmatched observations and stops counting, and a list too young to have a trustworthy
// membership is not an active list at all.
//
// # And it is reported rather than acted on
//
// Withholding the selection claim whenever a row was unseen was built and measured: accuracy
// 96% to 98% and confidently wrong readings 13 to 7, against retained graph moves 13 to ONE,
// because a move needs two readings of one list and the withheld looks fall between them. On
// the same corpus 65 namings were made while a row was unseen and 6 of them were wrong, so the
// rule costs 59 right answers and 12 moves to remove 6 errors.
// [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]].
//
// Deleting the cold-count loop in Tracker.ActiveMissing must fail this.
func TestASessionSaysHowManyOfTheListsRowsWereNotSeen(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark, seq := 0, 0
	walking := true
	look := func(topGone bool) {
		seq++
		if walking && seq >= 10 && seq%3 == 0 {
			mark = (mark + 1) % 5
		}
		lk := lookOfTwoLists(seq, base.Add(time.Duration(seq)*100*time.Millisecond),
			mark, 3, 1)
		if topGone {
			var kept []observe.ShadowShape
			var dropped bool
			for _, sh := range lk.Shapes {
				if !dropped && sh.Region.X < 0.5 {
					dropped = true
					continue
				}
				kept = append(kept, sh)
			}
			lk.Shapes = kept
		}
		w.Observe(lk)
	}
	for range 120 {
		look(false)
	}
	if b := w.Belief(); b.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified: %s", b.Because)
	}
	missing, of := w.ActiveMissing()
	if of == 0 {
		t.Fatal("the active list reports no rows at all")
	}
	// AND `of` IS WHAT THE LIST HAS, not what it has ever had. Entry identities are never
	// reused, so a count of them is a count of every row the list has ever carried — which
	// is the remembered total this design refuses to treat as ground truth.
	if n := len(w.ActiveMembers()); of != n {
		t.Errorf("the list reports %d rows and has %d members. A total that counts rows "+
			"it no longer has is the oracle this is meant not to be", of, n)
	}
	if missing != 0 {
		t.Errorf("%d of the list's %d rows were unseen on a look that showed all of "+
			"them; a complete list must report none missing", missing, of)
	}

	// THE TOP ROW GOES MISSING. It is still a member — it has not been unfound for long
	// enough to age out — so the list has a row this look did not show.
	walking = false
	look(true)
	missing, of = w.ActiveMissing()
	if missing != 1 {
		t.Errorf("%d of the list's %d rows were unseen after one of them went "+
			"undetected, want 1", missing, of)
	}

	// AND IT STOPS COUNTING ONCE THE ROW HAS GENUINELY GONE. A member nothing finds again
	// is forgotten, and a list that changed is not a list with a permanent hole in it.
	for range selection.MaxUnmatched + 4 {
		look(true)
	}
	missing, of = w.ActiveMissing()
	if missing != 0 {
		t.Errorf("%d of the list's %d rows are still reported unseen after %d looks "+
			"without that row. A remembered row that is never found again is a "+
			"list that changed, not a list that is incomplete", missing, of,
			selection.MaxUnmatched+4)
	}
	// AND IT IS STILL WHAT THE LIST HAS, after a row has genuinely gone. This is where a
	// count of every entry the list has ever carried and a count of the rows it has now
	// come apart, and where a remembered total would start overstating the list.
	if n := len(w.ActiveMembers()); of != n {
		t.Errorf("after a row aged out the list reports %d rows and has %d members", of, n)
	}
	t.Logf("the list has %d rows and reports %d unseen", of, missing)
}

// A session says why the mark's last sighting was let go.
//
// A move is only ever recorded between two sightings of one list, so every reset is a move that
// could not be. 349 correct readings produced 13 retained moves on the Trackmania corpus and
// the reason was not separated — this is what separates it. Measured there: 9 sightings dropped
// in the whole session, 5 of them for going unread too long, against the menu candidate's 21
// observed transfers. So move retention is near its ceiling on that recording and the gap to a
// usable graph is elsewhere.
//
// Deleting the drop() call in the unread branch of noteMove must fail this.
func TestASessionSaysWhyTheMarksLastSightingWasLetGo(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark, seq := 0, 0
	walking := true
	look := func(blank bool) {
		seq++
		if walking && !blank && seq >= 10 && seq%3 == 0 {
			mark = (mark + 1) % 5
		}
		// A LOOK WHERE NOTHING STANDS OUT. The list is all there and shaded; no row
		// is marked, so the reader has nothing to name. That is an UNREAD look, and
		// it is a different thing from a look that saw nothing at all — which takes
		// the belief to unknown and drops the sighting for want of a list.
		m := mark
		if blank {
			m = -1
		}
		w.Observe(lookOfTwoLists(seq, base.Add(time.Duration(seq)*100*time.Millisecond),
			m, 3, 1))
	}
	for range 120 {
		look(false)
	}
	// AND THE MARK HELD STILL, so the reading is settled rather than mid-confirmation on
	// whichever look the walk happened to end on.
	walking = false
	for range 4 {
		look(false)
	}
	if b := w.Belief(); b.Selected < 0 {
		t.Fatalf("nothing was read, so there is no sighting to let go of: %s", b.Reading)
	}
	// A SNAPSHOT RATHER THAN A ZERO. Early in a session, before any list has been
	// identified, there is a look or two where a sighting is let go for want of a list to
	// be an entry of — which is correct and is one of the causes being counted.
	const unread = "unread for too long between sightings"
	before := w.HeldSightingsDropped()[unread]

	// LONGER THAN A SIGHTING MAY GO UNREAD. The list is still there and still identified;
	// what is gone is any claim about where the mark is.
	// LONG ENOUGH FOR BOTH BOUNDS. The reader holds a settled claim for HoldFrames
	// unreadable looks before it gives it up, and only then does the gap between
	// sightings start counting toward MaxUnreadBetweenSightings.
	for range 20 {
		look(true)
	}
	dropped := w.HeldSightingsDropped()
	// A SIGHTING LET GO FOR WANT OF A LIST is its own cause, and it happens early in a
	// session before anything has been identified.
	const noList = "no list identified"
	if dropped[noList] == 0 {
		t.Errorf("no sighting was recorded as let go for want of a list, and this "+
			"session spent its first looks without one: %v", dropped)
	}
	// AND NOTHING IS COUNTED WHEN THERE WAS NOTHING TO LET GO. Every look before the
	// first reading has no held sighting, and counting those would make this number the
	// length of the warm-up rather than a count of lost moves.
	if dropped[noList] > 3 {
		t.Errorf("%d sighting(s) were let go for want of a list over a session with one "+
			"warm-up. A look with no held sighting has nothing to lose, and counting "+
			"it makes this the length of the warm-up: %v", dropped[noList], dropped)
	}
	if dropped[unread] <= before {
		t.Errorf("%d sighting(s) were recorded as dropped for going unread, against %d "+
			"before %d looks that read nothing. Every one of those is a move that "+
			"could not be recorded, and a report that cannot count them cannot say "+
			"whether the graph is starved by perception or by a recording that never "+
			"repeats itself", dropped[unread], before, 20)
	}
	t.Logf("sightings dropped: %v", dropped)
}
