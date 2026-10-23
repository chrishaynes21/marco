package recorder

// WHAT THE HOOK ACTUALLY STAMPS ON A CLICK.
//
// These drive `newClick` — the production event builder the mouse hook calls — rather than a
// hand-written event. That distinction is the reason this file exists. Twice now, a fixture
// elsewhere described a stream with better attribution than the hook produces, and the code
// consuming it looked correct against a desktop that cannot happen:
//
//   - first, a click tagged with an app the poller had not sampled yet;
//   - then, a release carrying an owner the builder only ever put on the press.
//
// A test that builds its own events cannot catch either. So the claims about what a click
// carries are made here, against the builder, and the strip's tests are entitled to assume them.

import (
	"testing"
)

// onTheGameWithMarcoInFront is the race the exclusion boundary exists for: the app poller's last
// sample still says the game, while the window actually in front belongs to Marco.
//
// Both halves matter. The app tag being WRONG is what makes the process id the only truthful
// thing on the event, and a fixture that got the app tag right would not be posing the question.
func onTheGameWithMarcoInFront(t *testing.T, owner uint32) {
	t.Helper()
	was := clickAttribution
	t.Cleanup(func() { clickAttribution = was })
	clickAttribution = func() (uint32, string) { return owner, "arkascended" }
}

// BOTH EDGES OF A CLICK CARRY THE PROCESS THAT OWNED THE WINDOW.
//
// A button is activated on RELEASE. An unattributed release is where the strip stops walking, so
// a release with no owner hides the press behind it however well that press is stamped.
func TestBothEdgesOfAClickCarryTheirOwner(t *testing.T) {
	const owner uint32 = 9140
	onTheGameWithMarcoInFront(t, owner)

	down := newClick("left", true, 1511, 913)
	up := newClick("left", false, 1511, 913)

	if down.PID != owner {
		t.Errorf("the press carries process %d, want %d", down.PID, owner)
	}
	if up.PID != owner {
		t.Errorf("the RELEASE carries process %d, want %d. A button is activated "+
			"on release, so an unattributed release is exactly the event a "+
			"strip walking back from the end meets first — and it stops "+
			"there, never reaching the press", up.PID, owner)
	}
}

// AND THE APP TAG IS THE POLLER'S, WHICH IS WHY IT IS NOT TRUSTED ALONE.
//
// Recorded as a property of the builder rather than left implicit: the tag can be stale, and
// anything deciding whose a click is from the tag alone is wrong in exactly this case.
func TestTheAppTagCanDisagreeWithTheProcess(t *testing.T) {
	const owner uint32 = 9140
	onTheGameWithMarcoInFront(t, owner)

	down := newClick("left", true, 1511, 913)
	if down.App != "arkascended" {
		t.Fatalf("the fixture's poller sample did not reach the event (%q), so "+
			"this proves nothing", down.App)
	}
	if down.PID == 0 {
		t.Fatal("the click carries no process, so the stale app tag is the only " +
			"attribution on it and the race has no answer")
	}
}

// AND GEOMETRY STAYS ON THE PRESS ALONE.
//
// The other half of the correction, asserted so that stamping attribution on both edges did not
// quietly spread the window offset too. A release lands wherever the press did; a second offset
// for it would be a second place to replay from, which is not a thing.
func TestOnlyThePressRecordsWhereItLanded(t *testing.T) {
	onTheGameWithMarcoInFront(t, 9140)

	up := newClick("left", false, 1511, 913)
	if up.WinRel {
		t.Error("a release recorded a window-relative offset of its own")
	}
	if up.Window != "" {
		t.Errorf("a release recorded the window title %q", up.Window)
	}
}
