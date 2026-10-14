package observe_test

// A reading that arrives after the look it describes.
//
// OCR costs hundreds of milliseconds against an observation cadence of a hundred, so this is the
// only kind of reading a live source will ever produce. What these hold is that a late reading
// can say something useful about the observation it describes, and nothing at all about a newer
// one.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func reading(t observe.ScreenText, look int, generation, epoch uint64) observe.ScreenReading {
	return observe.ScreenReading{Text: t, FromLook: look,
		Generation: generation, Epoch: epoch}
}

// ── the rule that decides admission ───────────────────────────────────────────

// A READING IS ABOUT THE OBSERVATION IT WAS TAKEN IN, and the rule is the detections' rule.
//
// One statement, two typed wrappers. A copy would eventually disagree, and the shape of that
// disagreement is an integration that refuses stale rectangles while quietly accepting stale
// words.
func TestALateReadingIsJudgedByTheSameRuleAsALateDetection(t *testing.T) {
	words := read("AERIAL", "GOALIE", "STRIKER")
	for _, tc := range []struct {
		name             string
		wasGen, wasEpoch uint64
		nowGen, nowEpoch uint64
		want             bool
	}{
		{"nothing moved", 1, 5, 1, 5, true},
		{"the screen moved on", 1, 5, 1, 6, false},
		{"the window was replaced", 1, 5, 2, 5, false},
		{"both moved", 1, 5, 2, 6, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := reading(words, 30, tc.wasGen, tc.wasEpoch)
			if got := r.Admissible(tc.nowGen, tc.nowEpoch); got != tc.want {
				t.Errorf("admissible %v, want %v", got, tc.want)
			}
			// AND THE SHARED RULE AGREES, which is the point of having it.
			if got := observe.Current(tc.wasGen, tc.wasEpoch, tc.nowGen,
				tc.nowEpoch); got != tc.want {
				t.Errorf("Current says %v, want %v", got, tc.want)
			}
			c := observe.Candidate{Generation: tc.wasGen, Epoch: tc.wasEpoch}
			if got := observe.Admissible(c, tc.nowGen, tc.nowEpoch); got != tc.want {
				t.Errorf("a detection in the same context says %v", got)
			}
		})
	}
}

// ── what an admitted reading may do ───────────────────────────────────────────

// A LATE READING REFUSES THE MERGE ITS OWN FRAME WAS MERGED BY.
//
// The measured case, and the thing that makes it work is counter-intuitive: the merge is what
// keeps the epoch still. No screen change was recorded, so a reading of the submenu's own pixels
// arriving five looks later is still a reading of the screen Marco is standing on — admitted,
// and about the right thing.
func TestALateReadingSeparatesTheScreenItsOwnFrameWasMergedInto(t *testing.T) {
	training, freePlay := twoMenus()
	v := &observe.VisualScreens{}
	say := read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	other := read("CREATE", "FREE", "JOIN", "ONLINE", "PLAY")

	// THE FIRST SCREEN, with two readings so it has a vocabulary.
	var first observe.VisualScreenID
	for i := 0; i < 5; i++ {
		first = v.ObserveWith(training, say, when)
	}
	if first == observe.VisualScreenUnknown {
		t.Fatal("nothing settled")
	}

	// THE SECOND MENU ARRIVES AND IS MERGED, because nothing has read it yet.
	for i := 0; i < 4; i++ {
		if got := v.Observe(freePlay, when); got != first {
			t.Fatalf("look %d separated with nothing read: %s", i, got)
		}
	}

	// AND NOW ITS READINGS ARRIVE. Two of them, because one reading is not evidence.
	for i := 0; i < 2; i++ {
		v.Read(reading(other, 10+i, 0, 0))
	}
	if id, yes := v.Contradicting(); !yes || id != first {
		t.Fatalf("after two readings of another screen, contradicting %q %v",
			id, yes)
	}
	// THE LOOKS THAT FOLLOW ARE NOT THAT SCREEN. Nothing read on them at all --
	// which is the ordinary case -- and the refusal carries.
	var second observe.VisualScreenID
	for i := 0; i < 4; i++ {
		second = v.Observe(freePlay, when)
	}
	if second == first {
		t.Fatalf("the submenu stayed merged into %s", first)
	}
	if second == observe.VisualScreenUnknown || second == observe.VisualScreenUnobserved {
		t.Fatalf("the submenu settled as %q rather than a screen", second)
	}
	if st := v.Stability(); st.TextRefusedMerge == 0 {
		t.Error("two screens separated and the run does not say text refused a merge")
	}
}

// AND A READING WITH NO SCREEN STANDING IS NOT ABOUT ANYTHING.
func TestAReadingWithNoScreenStandingConcludesNothing(t *testing.T) {
	v := &observe.VisualScreens{}
	refused, why := v.Read(reading(read("AERIAL", "GOALIE", "STRIKER"), 1, 0, 0))
	if refused {
		t.Error("a reading refused a merge with no screen standing")
	}
	if why == "" {
		t.Error("no screen was standing and no reason was given")
	}
}

// AND THE FIRST READING OF A SCREEN CANNOT CONTRADICT IT.
//
// One reading is not evidence: every reading of a dark game menu carries its own garbage, and
// garbage is by definition words the screen never said.
func TestTheFirstReadingOfAScreenConcludesNothing(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	for i := 0; i < 4; i++ {
		v.Observe(training, when)
	}
	refused, why := v.Read(reading(read("CREATE", "FREE", "JOIN", "ONLINE"), 9, 0, 0))
	if refused {
		t.Errorf("a first reading refused a merge: %s", why)
	}
	if id, yes := v.Contradicting(); yes {
		t.Errorf("a first reading contradicted %s", id)
	}
}

// AND A READING THAT AGREES TAKES BACK A REFUSAL BEFORE THE RUN SETTLES.
//
// The evidence correcting itself, across looks rather than within one. A pair of readings caught
// during a transition raises a contradiction; two readings of the menu underneath withdraw it,
// and the screen keeps its identity.
func TestALateReadingThatAgreesWithdrawsTheRefusal(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	say := read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	over := read("LOADING", "PLEASE", "WAIT", "CONNECTING")

	var id observe.VisualScreenID
	for i := 0; i < 5; i++ {
		id = v.ObserveWith(training, say, when)
	}
	settled := id
	for i := 0; i < 2; i++ {
		v.Read(reading(over, 10+i, 0, 0))
	}
	if _, yes := v.Contradicting(); !yes {
		t.Fatal("two readings of something else raised no contradiction")
	}
	// AND THEN THE MENU AGAIN, before the pending run could settle.
	for i := 0; i < 2; i++ {
		v.Read(reading(say, 12+i, 0, 0))
	}
	if cid, yes := v.Contradicting(); yes {
		t.Errorf("the refusal about %s still stands after readings agreed", cid)
	}
	for i := 0; i < 6; i++ {
		if got := v.Observe(training, when); got != settled {
			t.Fatalf("the screen split to %s after the readings agreed", got)
		}
	}
	if st := v.Stability(); st.TextWithdrewRefusal == 0 {
		t.Error("the run does not say the refusal was withdrawn")
	}
}

// AND TWO READINGS THAT STRADDLE A SCREEN CHANGE ARE NOT PAIRED.
//
// Their agreement would be whatever junk they happen to share, attributed to whichever screen
// was standing second. A pair has to be two readings OF ONE SCREEN.
func TestTwoReadingsThatStraddleAScreenChangeAreNotPaired(t *testing.T) {
	training, _ := twoMenus()
	// AN ARRANGEMENT THE STABILISER CANNOT CONFUSE WITH THE FIRST, so the screen really
	// does change. The two menus of the other fixtures merge by design -- that is what
	// they are for -- and a screen that never changed would make this test pass for the
	// wrong reason.
	elsewhere := observe.VisualArrangement{Total: 80, Cells: map[string]int{
		"2,2": 60, "1,2": 20,
	}}
	v := &observe.VisualScreens{}
	say := read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	// Two readings sharing exactly three words, which is enough to be a claim.
	a := read("ALPHA", "BETA", "GAMMA", "DELTA")
	b := read("ALPHA", "BETA", "GAMMA", "EPSILON")

	var first observe.VisualScreenID
	for i := 0; i < 5; i++ {
		first = v.ObserveWith(training, say, when)
	}
	// ONE reading, then a screen change, then the second.
	v.Read(reading(a, 10, 0, 0))
	var second observe.VisualScreenID
	for i := 0; i < 5; i++ {
		second = v.Observe(elsewhere, when)
	}
	if second == first {
		t.Fatalf("the fixture did not change screen: both are %s", first)
	}
	before := v.Stability().TextRefusedMerge
	v.Read(reading(b, 16, 0, 0))
	if got := v.Stability().TextRefusedMerge; got != before {
		t.Errorf("two readings either side of a screen change were paired and "+
			"refused a merge (%d -> %d)", before, got)
	}
	if id, yes := v.Contradicting(); yes {
		t.Errorf("a pair that straddles a screen change contradicted %s", id)
	}
}

// ── missing, and silent ───────────────────────────────────────────────────────

// A SOURCE THAT COULD NOT READ IS NOT A SCREEN THAT SAID NOTHING.
func TestAReadingThatFailedIsNotASilentScreen(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	say := read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	for i := 0; i < 5; i++ {
		v.ObserveWith(training, say, when)
	}
	refused, why := v.Read(reading(observe.ScreenText{}, 10, 0, 0))
	if refused {
		t.Errorf("a failed reading refused a merge: %s", why)
	}
	if id, yes := v.Contradicting(); yes {
		t.Errorf("a failed reading contradicted %s", id)
	}
	// AND THE SCREEN IS UNCHANGED BY IT.
	for i := 0; i < 4; i++ {
		if got := v.Observe(training, when); got == observe.VisualScreenUnknown {
			t.Fatal("the screen was lost after a failed reading")
		}
	}
}

// AND A RUN WHERE NOTHING WAS EVER READ FOLDS EXACTLY AS IT ALWAYS DID.
//
// The pixel-only path has to be bit-identical: every other caller in the tree is on it.
func TestWithNoReadingsAtAllTheFoldIsUnchanged(t *testing.T) {
	training, freePlay := twoMenus()
	plain, withReader := &observe.VisualScreens{}, &observe.VisualScreens{}
	var a, b []observe.VisualScreenID
	for i := 0; i < 6; i++ {
		a = append(a, plain.Observe(training, when))
		b = append(b, withReader.Observe(training, when))
	}
	for i := 0; i < 6; i++ {
		a = append(a, plain.Observe(freePlay, when))
		b = append(b, withReader.Observe(freePlay, when))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("look %d: %q against %q", i, a[i], b[i])
		}
	}
	if a[5] != a[11] {
		t.Errorf("the two arrangements settled as %q and %q with nothing read",
			a[5], a[11])
	}
	if st := withReader.Stability(); st.TextRead != 0 || st.TextRefusedMerge != 0 {
		t.Errorf("a run with no readings reports %+v", st)
	}
}

// ── and the provenance it carries ─────────────────────────────────────────────

// A READING SAYS WHICH SOURCE READ IT, AND WHICH LOOK IT IS OF.
func TestAReadingCarriesItsProvenance(t *testing.T) {
	r := reading(read("AERIAL", "GOALIE", "STRIKER"), 42, 3, 7)
	if r.FromLook != 42 || r.Generation != 3 || r.Epoch != 7 {
		t.Errorf("the reading does not carry its context: %+v", r)
	}
	if r.Text.From != directorapi.SourceOCR {
		t.Errorf("the reading is attributed to %q", r.Text.From)
	}
	// AND THE LOOK INDEX DECIDES NOTHING. Only the two generations do: a look index
	// is a position in a sequence, and the question is whether the OBSERVATION moved.
	near := reading(r.Text, 0, 3, 7)
	far := reading(r.Text, 9999, 3, 7)
	if near.Admissible(3, 7) != far.Admissible(3, 7) {
		t.Error("the look index changed the admission decision")
	}
}
