package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// State-relative tracking: an element is judged against the screen it lives on.
//
// Every test here exists because the session-global denominator gave a semantically wrong
// answer for a state-dependent control. The live Rocket League diagnostic measured four pause
// rows at 8 detections over 16 valid inferences — presence 0.50, shape `bursty`, which reads as
// "unreliable". The rows were detected in every single inference during which the menu was
// open. The button was never the problem; the denominator was.

// gameplay is a sparse screen: the structural composition Rocket League actually produces
// during play, which is very little.
func gameplay() observe.ShadowSample { return valid(det("icon", 0.02, 0.86, 0.19, 0.10)) }

// menu is a four-row centre-stacked panel — the structure a pause screen presents.
func menu(extra ...observe.ShadowRegion) observe.ShadowSample {
	rows := []observe.ShadowRegion{
		det("button", 0.414, 0.437, 0.172, 0.037),
		det("button", 0.414, 0.480, 0.172, 0.035),
		det("button", 0.414, 0.520, 0.172, 0.037),
		det("button", 0.414, 0.562, 0.172, 0.035),
		det("icon", 0.02, 0.86, 0.19, 0.10),
	}
	return valid(append(rows, extra...)...)
}

// menuMissingTopRow is the same screen with the detector failing to find one row.
func menuMissingTopRow() observe.ShadowSample {
	return valid(
		det("button", 0.414, 0.480, 0.172, 0.035),
		det("button", 0.414, 0.520, 0.172, 0.037),
		det("button", 0.414, 0.562, 0.172, 0.035),
		det("icon", 0.02, 0.86, 0.19, 0.10),
	)
}

// stateOf resolves the state a track has the most evidence in.
func stateOf(t *testing.T, tr observe.ShadowTrack) observe.TrackState {
	t.Helper()
	s, ok := tr.PrimaryState()
	if !ok {
		t.Fatalf("track %s carries no state evidence at all", tr.ID)
	}
	return s
}

// topRow is the track for the first menu row.
func topRow(t *testing.T, totals observe.ShadowTotals) observe.ShadowTrack {
	t.Helper()
	for _, tr := range totals.Tracks {
		if tr.Role == "button" && tr.Reference.Y > 0.43 && tr.Reference.Y < 0.45 {
			return tr
		}
	}
	t.Fatalf("no top-row button track among %d tracks", len(totals.Tracks))
	return observe.ShadowTrack{}
}

// Part 15. THE semantic correction, on the shape of the real session.
//
// A menu button present in every menu observation must read as reliable within that state, and
// the old global denominator must not be able to produce this answer.
func TestAMenuButtonIsPersistentWithinItsOwnScreenState(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		menu(), menu(), menu(),
		gameplay(), gameplay(),
		menu(), menu(),
	)

	row := topRow(t, totals)
	if row.Seen != 5 {
		t.Fatalf("the row was detected in %d menu inferences, want 5", row.Seen)
	}

	st := stateOf(t, row)
	if st.Seen != 5 || st.Eligible != 5 {
		t.Errorf("state-local %d/%d, want 5/5 — eligibility is still counting inferences "+
			"in which this button's screen was not on screen", st.Seen, st.Eligible)
	}
	if got := st.PresenceRatio(); got != 1.0 {
		t.Errorf("state-local presence %.2f, want 1.00", got)
	}
	if st.Shape != observe.ShapePersistent {
		t.Errorf("state-local shape %q, want %q", st.Shape, observe.ShapePersistent)
	}

	// And the global figures are deliberately unchanged: both statements are true, and a
	// reader gets both. The track is minted at inference 3 and every valid inference from
	// there on is a global opportunity, found or not — seven of them, five taken.
	if row.Eligible != 7 || row.Seen != 5 {
		t.Errorf("global %d/%d, want 5/7 — the session-global denominator must not move",
			row.Seen, row.Eligible)
	}
	if row.Shape != observe.ShapeBursty {
		t.Errorf("global shape %q, want %q", row.Shape, observe.ShapeBursty)
	}
	if row.PresenceRatio() >= 0.80 {
		t.Fatalf("global presence %.2f — this test cannot prove anything unless the global "+
			"answer really is the unfavourable one", row.PresenceRatio())
	}
}

// Part 16. State segmentation must not launder a real detector failure.
//
// The whole change is a denominator correction, and the risk it carries is that a favourable
// denominator hides a genuine miss. Same screen, same state, detector finds the row three times
// out of five: that is 0.60 and must stay 0.60.
func TestARealDetectorMissInsideTheStateStillCounts(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		menu(), menu(),
		menuMissingTopRow(), menuMissingTopRow(),
		menu(),
		gameplay(),
	)

	row := topRow(t, totals)
	st := stateOf(t, row)
	if st.Seen != 3 || st.Eligible != 5 {
		t.Fatalf("state-local %d/%d, want 3/5", st.Seen, st.Eligible)
	}
	if got := st.PresenceRatio(); got < 0.599 || got > 0.601 {
		t.Errorf("state-local presence %.2f, want 0.60 — a miss inside the element's own "+
			"state is real absence evidence and must not be excused", got)
	}
}

// Part 17. THE core regression, stated as the mutation it must kill.
//
// Reverting state-local eligibility to "every valid inference" — the behaviour this milestone
// exists to change — makes this fail. A button that belongs to the menu must not lose presence
// across ten inferences of gameplay.
func TestGameplayInferencesDoNotErodeAMenuButton(t *testing.T) {
	samples := []observe.ShadowSample{menu(), menu(), menu()}
	for i := 0; i < 10; i++ {
		samples = append(samples, gameplay())
	}
	totals := fold(samples...)

	row := topRow(t, totals)
	st := stateOf(t, row)
	if st.Eligible != 3 {
		t.Fatalf("state-local eligible %d, want 3 — ten gameplay inferences became "+
			"opportunities for a control that cannot exist during gameplay", st.Eligible)
	}
	if got := st.PresenceRatio(); got != 1.0 {
		t.Errorf("state-local presence %.2f, want 1.00", got)
	}
	// The global view still shows the absence, honestly.
	if row.Eligible != 13 {
		t.Errorf("global eligible %d, want 13 — the global denominator must keep counting "+
			"every valid inference", row.Eligible)
	}
}

// Part 18. An inference Marco cannot place is not evidence about anything.
//
// A half-drawn menu resembles the menu without matching it. Forcing it into a state would make
// a transition frame count as a miss for every control on the screen it was becoming.
func TestAnUnplaceableTransitionCountsAgainstNobody(t *testing.T) {
	// Two rows of four: enough structure to resemble the menu, not enough to be it.
	partial := valid(
		det("button", 0.414, 0.437, 0.172, 0.037),
		det("icon", 0.02, 0.86, 0.19, 0.10),
	)
	totals := fold(gameplay(), gameplay(), menu(), menu(), menu(), partial, menu())

	var unknownSeen bool
	for _, s := range totals.States {
		if s.ID == observe.ScreenStateUnknown {
			unknownSeen = true
		}
	}
	if unknownSeen {
		t.Errorf("state_unknown was materialised as a state; it must be the absence of one")
	}

	row := topRow(t, totals)
	st := stateOf(t, row)
	if st.Eligible != 4 {
		t.Fatalf("state-local eligible %d, want 4 — the transition inference became an "+
			"opportunity", st.Eligible)
	}
	if got := st.PresenceRatio(); got != 1.0 {
		t.Errorf("state-local presence %.2f, want 1.00", got)
	}
}

// Part 19. A screen that comes back is the SAME screen.
//
// If each menu episode minted a fresh state, every state would have exactly one episode, no
// state would ever recur, and state-local presence would be a more elaborate way of saying
// nothing. Recurrence is what makes the model worth having.
func TestAScreenThatReturnsResumesItsOwnStateIdentity(t *testing.T) {
	totals := fold(gameplay(), menu(), gameplay(), menu())

	if len(totals.States) != 2 {
		var ids []observe.ScreenStateID
		for _, s := range totals.States {
			ids = append(ids, s.ID)
		}
		t.Fatalf("segmented into %d states %v, want 2 — a recurring screen is not being "+
			"recognised as the one it already discovered", len(totals.States), ids)
	}
	for _, s := range totals.States {
		if s.Inferences != 2 || s.Episodes != 2 {
			t.Errorf("state %s: %d inferences over %d episodes, want 2 over 2",
				s.ID, s.Inferences, s.Episodes)
		}
	}

	row := topRow(t, totals)
	st := stateOf(t, row)
	if st.Seen != 2 || st.Eligible != 2 {
		t.Errorf("state-local %d/%d, want 2/2 across both episodes", st.Seen, st.Eligible)
	}
}

// Part 20. Two different menus are two different screens.
//
// The cheap version of this model would key state on "are there buttons", which would merge
// every dialog in the game into one identity and make state-local eligibility meaningless in
// exactly the situation it is meant for.
func TestStructurallyDifferentPanelsStayDifferentStates(t *testing.T) {
	other := valid(
		det("button", 0.30, 0.62, 0.10, 0.04),
		det("button", 0.60, 0.62, 0.10, 0.04),
		det("checkbox", 0.30, 0.40, 0.02, 0.02),
		det("panel", 0.25, 0.30, 0.50, 0.40),
	)
	totals := fold(menu(), menu(), other, other)

	if len(totals.States) != 2 {
		t.Fatalf("segmented into %d states, want 2 — composition is being reduced to the "+
			"presence of buttons", len(totals.States))
	}
	for _, s := range totals.States {
		if s.Inferences != 2 {
			t.Errorf("state %s saw %d inferences, want 2", s.ID, s.Inferences)
		}
	}
}

// Part 8. State identity must not be downstream of tracking.
//
// If a state's signature were built from the tracks present, tracks would define the state that
// defines their own eligibility and a track could never be absent in the state it was minting.
// The guard is structural: segmentation reads raw detections only, so identical detections
// segment identically no matter what tracking has concluded — including when the tracker is
// carrying a long, unrelated history.
func TestStateIdentityDoesNotDependOnTracking(t *testing.T) {
	plain := fold(gameplay(), menu(), gameplay(), menu())

	// The same four screens, reached after a pile of unrelated tracks has accumulated.
	var noise []observe.ShadowSample
	for i := 0; i < 6; i++ {
		x := 0.05 + float64(i)*0.12
		noise = append(noise, valid(det("button", x, 0.05, 0.03, 0.02)))
	}
	loaded := fold(append(noise, gameplay(), menu(), gameplay(), menu())...)

	menuState := func(totals observe.ShadowTotals) observe.ScreenState {
		t.Helper()
		row := topRow(t, totals)
		st := stateOf(t, row)
		for _, s := range totals.States {
			if s.ID == st.State {
				return s
			}
		}
		t.Fatalf("track's state %s is not in the state table", st.State)
		return observe.ScreenState{}
	}

	a, b := menuState(plain), menuState(loaded)
	if a.Inferences != b.Inferences || a.Episodes != b.Episodes {
		t.Errorf("the menu segmented as %d inferences/%d episodes alone but %d/%d behind "+
			"unrelated tracks — state identity is reading tracking",
			a.Inferences, a.Episodes, b.Inferences, b.Episodes)
	}
	if a.Inferences != 2 || a.Episodes != 2 {
		t.Errorf("menu state %d inferences over %d episodes, want 2 over 2",
			a.Inferences, a.Episodes)
	}
}

// Part 13. The model must not privilege menus.
//
// The inverse case has to work identically or the correction is a special case dressed as a
// principle: a HUD element that lives during play should be persistent in the gameplay state
// and simply absent from the menu one.
func TestAGameplayElementIsPersistentInTheGameplayState(t *testing.T) {
	// A HUD icon present during play and gone behind the menu.
	hud := func() observe.ShadowSample { return valid(det("icon", 0.45, 0.90, 0.10, 0.06)) }
	menuNoHUD := func() observe.ShadowSample {
		return valid(
			det("button", 0.414, 0.437, 0.172, 0.037),
			det("button", 0.414, 0.480, 0.172, 0.035),
			det("button", 0.414, 0.520, 0.172, 0.037),
			det("button", 0.414, 0.562, 0.172, 0.035),
		)
	}
	totals := fold(hud(), hud(), hud(), menuNoHUD(), menuNoHUD(), menuNoHUD(), hud())

	var icon observe.ShadowTrack
	for _, tr := range totals.Tracks {
		if tr.Role == "icon" {
			icon = tr
		}
	}
	if icon.ID == "" {
		t.Fatal("no icon track")
	}
	st := stateOf(t, icon)
	if st.Seen != 4 || st.Eligible != 4 {
		t.Errorf("HUD icon state-local %d/%d, want 4/4", st.Seen, st.Eligible)
	}
	if st.Shape != observe.ShapePersistent {
		t.Errorf("HUD icon state-local shape %q, want %q", st.Shape, observe.ShapePersistent)
	}
	if _, ok := icon.StateIn(stateOf(t, topRow(t, totals)).State); ok {
		t.Error("the HUD icon acquired evidence in the menu state it never appeared in")
	}
}

// Part 5/26. Transitions are counted, and never interpreted.
func TestScreenTransitionsAreCountedWithoutBeingNamed(t *testing.T) {
	totals := fold(gameplay(), menu(), gameplay(), menu(), gameplay())

	var total int
	for _, tr := range totals.Transitions {
		total += tr.Count
		if tr.From == tr.To {
			t.Errorf("a state transitioned to itself: %s → %s", tr.From, tr.To)
		}
	}
	if total != 4 {
		t.Errorf("counted %d transitions, want 4", total)
	}
	if len(totals.Transitions) != 2 {
		t.Errorf("%d distinct transitions, want 2 (there and back)", len(totals.Transitions))
	}
}

// Part 14. The four-way rule the global path already obeys, extended by one.
//
// Skipped, failed and unproven inferences carry no state evidence, exactly as they carry no
// global evidence. A cadence decision must never become a screen state.
func TestNonEvidenceSlotsDoNotSegmentAnything(t *testing.T) {
	failed := observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true,
		Unavailable: "capture failed"}
	unproven := observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: false}

	totals := fold(menu(), skipped(), failed, unproven, menu(), skipped(), menu())

	row := topRow(t, totals)
	st := stateOf(t, row)
	if st.Seen != 3 || st.Eligible != 3 {
		t.Errorf("state-local %d/%d, want 3/3 — a slot that produced no evidence was "+
			"treated as an observation of a screen", st.Seen, st.Eligible)
	}
	if len(totals.States) != 1 {
		t.Errorf("%d states, want 1 — non-evidence slots minted screen states",
			len(totals.States))
	}
}

// Part 23. The model plateaus.
//
// A long session must not grow a state per inference. Past the bound nothing new is minted and
// the overflow is REPORTED — a capped model that read as a complete one would be worse than an
// unbounded one.
func TestTheStateModelIsBoundedAndReportsWhatItDropped(t *testing.T) {
	var samples []observe.ShadowSample
	for i := 0; i < observe.MaxScreenStates+12; i++ {
		// Each screen structurally unlike every other: a distinct role in a distinct cell.
		x := float64(i%3)/3 + 0.05
		y := float64((i/3)%3)/3 + 0.05
		samples = append(samples, valid(
			det("button", x, y, 0.04, 0.03),
			det("icon", x, y+0.02, float64(i%7)*0.01+0.01, 0.02),
		))
	}
	totals := fold(samples...)

	if len(totals.States) > observe.MaxScreenStates {
		t.Fatalf("%d states, bound is %d", len(totals.States), observe.MaxScreenStates)
	}
	for _, tr := range totals.Tracks {
		if len(tr.States) > observe.MaxTrackStates {
			t.Fatalf("track %s carries %d state associations, bound is %d",
				tr.ID, len(tr.States), observe.MaxTrackStates)
		}
	}
}
