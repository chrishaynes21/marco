package observe

import "testing"

// A page that says it has not finished is not a place.
//
// # The live failure
//
// One acceptance store held THREE durable Homes:
//
//	button 18  group 27  text 49                   the settled page
//	button 17  group 26  text 45                   one render stage short
//	button 17  group 26  text 45  progress_bar 1   caught mid-load
//
// The third carries the progress bar in its durable signature. Settings holds one up during
// navigation and takes it down when the content lands, and it holds it steady long enough for
// the composition to settle — so the maturity gate passed and a moment became a place.
//
// Stopping changing is not the same as being finished.

// remembersNothing recognises no place, so every candidate is a new one.
type remembersNothing struct{ Memory }

func (remembersNothing) Recall(string, StructureSignature) Recollection {
	return Recollection{Verdict: MatchDifferent}
}

// page builds a settled state, optionally mid-load, seen in `episodes` separate visits.
func loadingPage(id ScreenStateID, episodes int, loading bool) ScreenState {
	roles := map[string]int{"button": 17, "group": 26, "text": 45, "list_item": 22}
	if loading {
		roles["progress_bar"] = 1
	}
	return ScreenState{
		ID: id, Episodes: episodes, Inferences: 8, TermObservations: 4,
		Settled: true, Roles: roles,
		Terms: map[InterfaceTerm]int{TermSettings: 4, TermBack: 4},
	}
}

func totals(current ScreenStateID, states ...ScreenState) ShadowTotals {
	return ShadowTotals{CurrentState: current, States: states}
}

// A page caught arriving is refused, and says why.
//
// Deleting the stillLoading gate must fail this.
func TestAPageStillLoadingDoesNotBecomeAPlace(t *testing.T) {
	tot := totals("state_1", loadingPage("state_1", 1, true))
	_, why := PlaceToEstablish(tot, "settings", remembersNothing{},
		DefaultHypothesisThresholds())
	if why != PlaceStillLoading {
		t.Errorf("a half-drawn page was refused with %q, want %q.\nIt settled, so the "+
			"maturity gate passes; settling is not finishing.", why, PlaceStillLoading)
	}
}

// The same page, finished, still becomes a place.
//
// The rule may not cost Marco the screens it is supposed to learn.
func TestTheFinishedPageStillBecomesAPlace(t *testing.T) {
	tot := totals("state_1", loadingPage("state_1", 1, false))
	sig, why := PlaceToEstablish(tot, "settings", remembersNothing{},
		DefaultHypothesisThresholds())
	if why != "" {
		t.Fatalf("a settled finished page was refused with %q", why)
	}
	if len(sig.Roles) == 0 {
		t.Error("the signature carries no composition")
	}
}

// A page whose content legitimately includes a progress control is NOT banned forever.
//
// A loading page and a page that really has a progress bar look identical in one visit. They
// differ in what happens next: nobody returns to a moment. So recurrence decides, reusing the
// rule the hypothesis layer already states rather than inventing a clock.
//
// Banning progress_bar outright must fail this.
func TestAStablePageWithAProgressControlCanEstablishOnceItRecurs(t *testing.T) {
	th := DefaultHypothesisThresholds()
	tot := totals("state_1", loadingPage("state_1", th.MinEpisodes, true))
	sig, why := PlaceToEstablish(tot, "settings", remembersNothing{}, th)
	if why != "" {
		t.Fatalf("a page that came back with its progress control was refused with %q.\n"+
			"Some screens really do contain one, and they must not be unlearnable.", why)
	}
	if sig.Roles["progress_bar"] != 1 {
		t.Error("the progress control was stripped from the signature; the rule is about " +
			"eligibility to establish, not about what a place is made of")
	}
}

// The transient does not stop a real place elsewhere in the same pass being established.
//
// The live consequence: a walk through Home, a half-drawn Home and Bluetooth must still make
// Bluetooth durable.
func TestATransientDoesNotCostTheRealPlacesInTheSamePass(t *testing.T) {
	settled := loadingPage("state_home", 1, false)
	settled.FirstInference = 1
	loading := loadingPage("state_loading", 1, true)
	loading.FirstInference = 2
	dest := ScreenState{
		ID: "state_bt", Episodes: 1, Inferences: 8, TermObservations: 4, Settled: true,
		FirstInference: 3,
		Roles:          map[string]int{"button": 10, "group": 12, "text": 26, "list_item": 21},
		Terms:          map[InterfaceTerm]int{TermSettings: 4, TermBack: 4},
	}
	tot := totals("state_bt", settled, loading, dest)

	got, _ := PlacesToEstablish(tot, "settings", remembersNothing{},
		DefaultHypothesisThresholds())
	var ids []ScreenStateID
	for _, c := range got {
		ids = append(ids, c.State)
	}
	for _, id := range ids {
		if id == "state_loading" {
			t.Errorf("the half-drawn page was established: %v", ids)
		}
	}
	var sawHome, sawDest bool
	for _, id := range ids {
		sawHome = sawHome || id == "state_home"
		sawDest = sawDest || id == "state_bt"
	}
	if !sawHome || !sawDest {
		t.Errorf("the real places were lost with the transient: %v", ids)
	}
}

// The raw observation keeps its progress bar.
//
// This is an ELIGIBILITY rule. What Marco may see, show and act on is untouched — and a stable
// page's durable composition still contains the control, as the recurrence test asserts.
func TestTheProgressControlIsNotStrippedFromWhatMarcoSees(t *testing.T) {
	sig := NewScreenSignature([]ShadowRegion{
		{Role: "progress_bar", Region: Region{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.01}},
		{Role: "button", Region: Region{X: 0.2, Y: 0.3, Width: 0.05, Height: 0.02}},
	})
	if sig.Roles["progress_bar"] != 1 {
		t.Error("a progress bar was removed from the composition Marco observed")
	}
}
