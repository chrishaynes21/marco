package observe

import "testing"

// A place is never remembered as a composition Marco did not see.
//
// # The defect this closes
//
// The producer used to ask `settledCount(role)` for each role separately and assemble the
// answers. Roles were moded independently, so a state whose samples disagreed emitted a
// composition equal to none of them:
//
//	observed  {18,26,49} {18,26,49} {17,27,49} {17,27,49} {17,26,45}
//	emitted   {17,26,49}   ← no sample ever showed this
//
// That blend is how the last surviving twin got into a live store, and no amount of settling
// could have prevented it: the inputs were stable, the arithmetic was the problem.

// samples renders one composition as regions.
func composition(roles map[string]int) []ShadowRegion {
	var out []ShadowRegion
	for role, n := range roles {
		for i := 0; i < n; i++ {
			out = append(out, ShadowRegion{Role: role, Region: Region{
				X: 0.1, Y: 0.05 + float64(i%12)*0.06, Width: 0.05, Height: 0.02}})
		}
	}
	return out
}

// observe feeds a run of compositions into one segmenter and returns the state it settled on.
func observeRun(t *testing.T, runs ...map[string]int) (ScreenState, bool) {
	t.Helper()
	var g ScreenSegmenter
	var id ScreenStateID
	for i, r := range runs {
		id = g.Observe(i+1, composition(r), nil, SemanticEvidence{})
	}
	for _, st := range g.States() {
		if st.ID == id {
			return st, true
		}
	}
	return ScreenState{}, false
}

// Whatever is emitted, it is a composition that actually occurred.
//
// THE invariant. Restoring independent per-role modes must fail this.
func TestADurableCompositionIsAlwaysOneThatWasObserved(t *testing.T) {
	runs := []map[string]int{
		{"button": 18, "group": 26, "text": 49},
		{"button": 18, "group": 26, "text": 49},
		{"button": 17, "group": 27, "text": 49},
		{"button": 17, "group": 27, "text": 49},
		{"button": 17, "group": 26, "text": 45},
	}
	st, ok := observeRun(t, runs...)
	if !ok {
		t.Fatal("the samples produced no state at all")
	}
	if st.Roles == nil {
		// The deliberate fail-closed case: two compositions, equally frequent and the same
		// size, so nothing can say which the screen is. See settledWhole.
		if st.Settled {
			t.Error("an unresolved state was reported as settled")
		}
		return
	}
	matched := false
	for _, r := range runs {
		same := len(r) == len(st.Roles)
		for role, n := range r {
			if st.Roles[role] != n {
				same = false
			}
		}
		if same {
			matched = true
		}
	}
	if !matched {
		t.Errorf("the emitted composition %v matches none of the samples that produced "+
			"it.\nA place may summarise repeated evidence; it may not invent a "+
			"combination of role counts that never coexisted.", st.Roles)
	}
}

// A clear winner is promoted, and it is the observed one.
func TestTheMostRecurrentObservedCompositionWins(t *testing.T) {
	canonical := map[string]int{"button": 18, "group": 27, "text": 49}
	oneOff := map[string]int{"button": 17, "group": 26, "text": 45}
	st, ok := observeRun(t, canonical, canonical, canonical, oneOff)
	if !ok {
		t.Fatal("no state")
	}
	if !st.Settled {
		t.Fatalf("a composition seen three times did not settle: %v", st.Roles)
	}
	for role, n := range canonical {
		if st.Roles[role] != n {
			t.Errorf("role %s emitted %d, the recurring composition has %d",
				role, st.Roles[role], n)
		}
	}
}

// A one-off never becomes the place, however large it is.
//
// A tooltip or a flyout adds structure for a moment. Preferring the largest composition outright
// would let it define the screen; recurrence decides first, and size only breaks ties.
func TestAOneOffOverlayDoesNotBecomeThePlace(t *testing.T) {
	page := map[string]int{"button": 10, "group": 8, "text": 20}
	overlay := map[string]int{"button": 14, "group": 11, "text": 26, "menu": 1}
	st, ok := observeRun(t, page, page, overlay)
	if !ok {
		t.Fatal("no state")
	}
	if st.Roles["menu"] != 0 {
		t.Errorf("a one-off overlay reached the durable composition: %v", st.Roles)
	}
	if st.Roles["button"] != 10 {
		t.Errorf("the overlay's counts won: %v", st.Roles)
	}
}

// One sighting is a transition frame, not a screen.
func TestACompositionSeenOnceIsNotPromoted(t *testing.T) {
	st, ok := observeRun(t, map[string]int{"button": 10, "group": 8, "text": 20})
	if !ok {
		t.Fatal("no state")
	}
	if st.Settled {
		t.Error("a composition seen exactly once was settled; a transition frame and a " +
			"screen are indistinguishable until one of them comes back")
	}
}

// The answer does not depend on the order the samples arrived in.
func TestTheWinningCompositionIsOrderIndependent(t *testing.T) {
	a := map[string]int{"button": 10, "group": 8, "text": 20}
	b := map[string]int{"button": 11, "group": 8, "text": 20}
	orders := [][]map[string]int{
		{a, a, a, b},
		{b, a, a, a},
		{a, b, a, a},
		{a, a, b, a},
	}
	var first map[string]int
	for i, run := range orders {
		st, ok := observeRun(t, run...)
		if !ok {
			t.Fatalf("order %d produced no state", i)
		}
		if first == nil {
			first = st.Roles
			continue
		}
		for role, n := range first {
			if st.Roles[role] != n {
				t.Errorf("order %d emitted %v, the first order emitted %v",
					i, st.Roles, first)
				break
			}
		}
	}
}

// More of the same evidence does not change what the place is.
func TestExtraIdenticalObservationsDoNotChangeIdentity(t *testing.T) {
	page := map[string]int{"button": 10, "group": 8, "text": 20}
	short, _ := observeRun(t, page, page)
	long, _ := observeRun(t, page, page, page, page, page, page, page, page)
	for role, n := range short.Roles {
		if long.Roles[role] != n {
			t.Errorf("a longer session changed the place: %v vs %v", long.Roles, short.Roles)
		}
	}
	if !short.Settled || !long.Settled {
		t.Error("a composition that recurred did not settle")
	}
}

// A tie is resolved the same way every time, or not at all.
//
// # Why this is its own test
//
// Two compositions can be equally frequent and the same size. Nothing about the screen says which
// it is, and picking whichever Go's map iteration reaches first would make a place's identity
// depend on the order a hash table happened to walk — the same session, replayed, remembering a
// different screen.
//
// So it FAILS CLOSED: unresolved, not promoted. Size breaks a tie on frequency because a screen
// caught part-way through rendering is smaller than the screen; nothing breaks a tie on both.
//
// Deciding a full tie by map order must fail this.
func TestATieIsResolvedTheSameWayEveryTime(t *testing.T) {
	a := map[string]int{"button": 18, "group": 26, "text": 49} // total 93
	b := map[string]int{"button": 17, "group": 27, "text": 49} // total 93
	var first map[string]int
	var firstSettled bool
	for i := 0; i < 40; i++ {
		st, ok := observeRun(t, a, a, b, b)
		if !ok {
			t.Fatalf("run %d produced no state", i)
		}
		if i == 0 {
			first, firstSettled = st.Roles, st.Settled
			continue
		}
		if st.Settled != firstSettled {
			t.Fatalf("run %d settled=%v, the first run settled=%v",
				i, st.Settled, firstSettled)
		}
		if len(st.Roles) != len(first) {
			t.Fatalf("run %d emitted %v, the first run emitted %v", i, st.Roles, first)
		}
		for role, n := range first {
			if st.Roles[role] != n {
				t.Fatalf("run %d emitted %v, the first run emitted %v\n"+
					"A place's identity must not depend on the order a hash table "+
					"happened to walk.", i, st.Roles, first)
			}
		}
	}
	if firstSettled {
		t.Error("two equally-supported compositions of the same size settled on one of " +
			"them; nothing about the screen says which it is")
	}
}
