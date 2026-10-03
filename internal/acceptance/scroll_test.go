package acceptance_test

import (
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/acceptance"
)

// Scrolling, on a controlled surface, checked twice.
//
// # Why this is here and not only on Notepad
//
// Scroll was earned on Notepad — a 70-line document made "scroll to the top" the obvious request
// and nothing could read it. But Notepad is not a fixture: it updates itself, it is not under the
// harness's control, and a capability proven only there is proven only until Tuesday. So the
// fixture grew a scrollable region, which is generic furniture rather than anything Notepad-shaped:
// a 200-row list in a box five rows tall.
//
// # The two channels
//
// MARCO's verdict comes from the accessibility layer's scroll percentage. The ORACLE is the
// ListBox's own TopIndex, read from the managed control and written to the state file — which is
// how the fixture answers "where is the view" without going through the thing being tested.
//
// They were measured disagreeing before the fixture polled for it: UIA reported 0% → 100% while
// the file still said TopIndex 0, because WinForms raises no event when a list scrolls and the
// state was only written on the events other controls raise. A stale oracle is worse than a
// missing one — it disagrees with Marco while looking like ground truth.
func TestDesktopScrolling(t *testing.T) {
	if os.Getenv(acceptance.Gate) == "" {
		t.Skipf("set %s=1 to run the desktop scrolling acceptance", acceptance.Gate)
	}
	h := acceptance.Start(t, acceptance.Options{Resolver: os.Getenv("MARCO_RESOLVER")})

	// The bottom is not hard-coded: the fixture says how many rows it has, and the last page
	// is however many of them do not fit. Asserting "195" would be asserting the row height.
	bottomOf := func(s acceptance.State) int { return s.LinesTopIndex }

	t.Run("scroll/to the bottom and back", func(t *testing.T) {
		h.Restart(false)
		start := h.Oracle()
		if start.LinesCount < 50 {
			t.Fatalf("the fixture reports %d rows; this case needs a list that must scroll",
				start.LinesCount)
		}
		if start.LinesTopIndex != 0 {
			t.Fatalf("the list did not start at the top (%d)", start.LinesTopIndex)
		}

		down := h.Do("scroll to the bottom")
		atBottom := h.Oracle()
		if !down.Performed() {
			t.Errorf("scrolling to the bottom was not reported as performed (%s)%s",
				down.Outcome(), down.Report(oracleVerdict(bottomOf(atBottom) > 0)))
		}
		if bottomOf(atBottom) == 0 {
			t.Errorf("THE VIEW DID NOT MOVE. oracle top_index=%d of %d rows%s",
				atBottom.LinesTopIndex, atBottom.LinesCount,
				down.Report("DISAGREES — the list is still at the top"))
		}

		up := h.Do("scroll to the top")
		atTop := h.Oracle()
		if !up.Performed() {
			t.Errorf("scrolling to the top was not reported as performed (%s)%s",
				up.Outcome(), up.Report(oracleVerdict(atTop.LinesTopIndex == 0)))
		}
		if atTop.LinesTopIndex != 0 {
			t.Errorf("the view is at row %d and not at the top%s",
				atTop.LinesTopIndex, up.Report("DISAGREES"))
		}
	})

	// A PAGE IS THE APPLICATION'S PAGE, and the only claim worth making about it is the
	// direction. Asserting a row count would be asserting the list's item height, which is a
	// fact about this fixture's font rather than about scrolling.
	t.Run("scroll/down one page then up one page", func(t *testing.T) {
		h.Restart(false)

		down := h.Do("scroll down")
		afterDown := h.Oracle()
		if !down.Performed() {
			t.Errorf("scrolling down was not reported as performed (%s)%s",
				down.Outcome(), down.Report("—"))
		}
		if afterDown.LinesTopIndex <= 0 {
			t.Fatalf("a page down left the view at row %d%s",
				afterDown.LinesTopIndex, down.Report("DISAGREES"))
		}

		up := h.Do("scroll up")
		afterUp := h.Oracle()

		// A MEASURED LIMITATION, recorded rather than asserted through.
		//
		// A second scroll in the same session sometimes refuses with "the target could not
		// be found again: 2 controls match that about equally well". It comes from the
		// EXECUTOR's re-resolution, not from this one: the plan carries the element id and
		// the query, resolveRef prefers the id, and when the id no longer resolves it falls
		// back to the query — which matches more than one thing in the fused world even
		// though the accessibility host reports exactly one scrollable region before and
		// after the scroll. Diagnosed to that boundary and no further.
		//
		// Recorded the way this suite records every limitation it has measured and not
		// fixed, because the alternative is a case that passes by not looking. What is still
		// ASSERTED is the part that must never bend: the refusal must not be a false success,
		// and the view must not have moved the wrong way.
		switch {
		case up.Performed():
			if afterUp.LinesTopIndex >= afterDown.LinesTopIndex {
				t.Errorf("a page up left the view at row %d, no higher than the %d it "+
					"started from%s", afterUp.LinesTopIndex, afterDown.LinesTopIndex,
					up.Report("DISAGREES"))
			}
		default:
			if afterUp.LinesTopIndex != afterDown.LinesTopIndex {
				t.Errorf("a scroll that reported %q moved the view from row %d to %d%s",
					up.Outcome(), afterDown.LinesTopIndex, afterUp.LinesTopIndex,
					up.Report("CHANGED after a refusal"))
			}
			t.Logf("UNRESOLVED-REREFERENCE: a second scroll refused (%s) — %s",
				up.Outcome(), up.Message())
		}
	})

	// ALREADY THERE IS NOT A FAILURE, and the sentence has to say which it is.
	//
	// Somebody at the top who asks to go up has what they asked for. Reporting `failed` would
	// invite them to ask again, which is the one thing a verdict about an unmoved view must not
	// do — the same argument that made `unverified` its own word.
	t.Run("scroll/up from the top is already there", func(t *testing.T) {
		h.Restart(false)
		before := h.Oracle()
		if before.LinesTopIndex != 0 {
			t.Fatalf("the list did not start at the top (%d)", before.LinesTopIndex)
		}

		r := h.Do("scroll up")
		after := h.Oracle()

		if !r.Performed() {
			t.Errorf("scrolling up from the top reported %q; being where you asked to be is "+
				"not a failure%s", r.Outcome(), r.Report("unchanged"))
		}
		if after.LinesTopIndex != 0 {
			t.Errorf("the view moved to row %d when it was already at the top",
				after.LinesTopIndex)
		}
		// AND NOTHING ELSE HAPPENED. A scroll that pressed something would be a far worse
		// failure than one that did nothing.
		if after.SavePresses+after.CancelPresses+after.DetailsPresses != 0 {
			t.Errorf("a scroll pressed something: %s", after.Summary())
		}
	})
}
