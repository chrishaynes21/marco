package fastobserve_test

// The slow text source, through the observer that judges it.
//
// `screenreading_test.go` in `observe` holds what an admitted reading may DO. These hold which
// readings get admitted at all, and that the counters say what became of the rest — because a
// source whose results are usually too late to use must not be reportable as one that worked.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// twoMenusThatMerge is the measured pair: alike enough by arrangement to be folded together.
func twoMenusThatMerge() (a, b []observe.Region) {
	// Regions placed so the arrangement lands in the same cells at different densities,
	// which is what two dark menus with a row of tiles look like to a 3x3 histogram.
	at := func(x, y float64) observe.Region {
		return observe.Region{X: x, Y: y, Width: 0.04, Height: 0.03}
	}
	for i := 0; i < 26; i++ {
		a = append(a, at(0.40+float64(i%4)*0.01, 0.10))
	}
	for i := 0; i < 30; i++ {
		a = append(a, at(0.40+float64(i%4)*0.01, 0.80))
	}
	for i := 0; i < 19; i++ {
		b = append(b, at(0.40+float64(i%4)*0.01, 0.10))
	}
	for i := 0; i < 22; i++ {
		b = append(b, at(0.40+float64(i%4)*0.01, 0.80))
	}
	return a, b
}

func words(w ...string) observe.ScreenText {
	return observe.TextOf(directorapi.SourceOCR, w)
}

// look is one fold's worth of input, with whatever has arrived on it.
func readLook(regions []observe.Region, gen uint64, rs ...observe.ScreenReading) fastobserve.Look {
	return fastobserve.Look{
		At:       time.Unix(1700000000, 0).UTC(),
		Window:   fastobserve.WindowRef{ID: "hwnd:1", Generation: gen},
		Front:    true,
		Regions:  regions,
		Readings: rs,
	}
}

func drained(t *testing.T) *fastobserve.Observer {
	t.Helper()
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range o.Updates() {
		}
	}()
	t.Cleanup(func() { o.Close() })
	return o
}

// A READING OF THE OBSERVATION STANDING NOW IS ADMITTED AND SEPARATES THE SCREENS.
//
// End to end through the production observer: the second menu merges into the first because
// nothing has read it, two readings of its own pixels arrive several looks late, and the merge is
// refused. This is the live path — the readings never touch `Look.Text`.
func TestALateReadingReachesTheScreenModelAndSeparatesTwoMenus(t *testing.T) {
	first, second := twoMenusThatMerge()
	o := drained(t)
	menu := words("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	submenu := words("CREATE", "FREE", "JOIN", "ONLINE", "PLAY")

	// THE FIRST SCREEN, read on its own looks so it has a vocabulary. A real source
	// would be late here too; the point under test is the second screen.
	var settled observe.VisualScreenID
	for i := 0; i < 6; i++ {
		lk := readLook(first, 1)
		lk.Text = menu
		settled = o.Observe(lk).Screen
	}
	if settled == observe.VisualScreenUnknown {
		t.Fatal("nothing settled")
	}
	epoch := o.Epoch()

	// THE SECOND MENU, merged because nothing has read it.
	for i := 0; i < 4; i++ {
		if got := o.Observe(readLook(second, 1)).Screen; got != settled {
			t.Fatalf("look %d separated with nothing read: %s", i, got)
		}
	}
	// AND NOW ITS READINGS, stamped with the context its frames were captured in.
	for i := 0; i < 2; i++ {
		o.Observe(readLook(second, 1, observe.ScreenReading{
			Text: submenu, FromLook: 7 + i,
			Generation: 1, Epoch: epoch,
		}))
	}
	var apart observe.VisualScreenID
	for i := 0; i < 4; i++ {
		apart = o.Observe(readLook(second, 1)).Screen
	}
	if apart == settled {
		t.Fatalf("the submenu stayed merged into %s", settled)
	}

	st := o.Stats()
	if st.ReadingsOffered != 2 {
		t.Errorf("%d reading(s) offered, want 2", st.ReadingsOffered)
	}
	if st.ReadingsAdmitted != 2 {
		t.Errorf("%d admitted of 2 offered in the same observation", st.ReadingsAdmitted)
	}
	if st.ReadingsRefusedMerge == 0 {
		t.Error("the screens separated and no reading is credited with refusing a merge")
	}
}

// AND A READING WHOSE CONTEXT HAS MOVED IS REFUSED, with the counter saying which part.
//
// Two different findings about a deployment: a window replaced under the observer should be
// rare, and a screen that moved on while OCR ran is the ordinary case.
func TestAReadingWhoseContextMovedIsRefusedAndCounted(t *testing.T) {
	first, _ := twoMenusThatMerge()
	submenu := words("CREATE", "FREE", "JOIN", "ONLINE", "PLAY")

	for _, tc := range []struct {
		name       string
		gen, epoch uint64
		wantWindow int
		wantEpoch  int
	}{
		{"the screen moved on", 1, 999, 0, 1},
		{"the window was replaced", 7, 999, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := drained(t)
			var settled observe.VisualScreenID
			for i := 0; i < 6; i++ {
				lk := readLook(first, 1)
				lk.Text = words("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
				settled = o.Observe(lk).Screen
			}
			for i := 0; i < 3; i++ {
				o.Observe(readLook(first, 1, observe.ScreenReading{
					Text: submenu, FromLook: 2,
					Generation: tc.gen, Epoch: tc.epoch,
				}))
			}
			st := o.Stats()
			if st.ReadingsAdmitted != 0 {
				t.Errorf("%d stale reading(s) admitted", st.ReadingsAdmitted)
			}
			if st.ReadingsStaleWindow != tc.wantWindow*3 {
				t.Errorf("stale-window %d, want %d",
					st.ReadingsStaleWindow, tc.wantWindow*3)
			}
			if st.ReadingsStaleEpoch != tc.wantEpoch*3 {
				t.Errorf("stale-epoch %d, want %d",
					st.ReadingsStaleEpoch, tc.wantEpoch*3)
			}
			// AND THE SCREEN IS UNTOUCHED BY THEM.
			for i := 0; i < 4; i++ {
				if got := o.Observe(readLook(first, 1)).Screen; got != settled {
					t.Fatalf("a refused reading moved the screen to %s", got)
				}
			}
		})
	}
}

// AND A READING ON A LOOK THAT IS NOT EVIDENCE IS NOT EVIDENCE EITHER.
//
// A look with no pixels, or one taken while another window was in front, cannot host a pixel
// proposal — and admitting a reading on it would let a covered window's words in by the back
// door. A covered window's words are another application's.
func TestAReadingOnAnUnusableLookIsRefused(t *testing.T) {
	first, _ := twoMenusThatMerge()
	submenu := words("CREATE", "FREE", "JOIN", "ONLINE", "PLAY")

	for _, tc := range []struct {
		name  string
		spoil func(*fastobserve.Look)
	}{
		{"no pixels were obtained", func(lk *fastobserve.Look) {
			lk.Missing, lk.Regions = true, nil
		}},
		{"another window was in front", func(lk *fastobserve.Look) {
			lk.Front = false
		}},
		{"nothing was proposed on it", func(lk *fastobserve.Look) {
			lk.Regions = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := drained(t)
			for i := 0; i < 6; i++ {
				lk := readLook(first, 1)
				lk.Text = words("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
				o.Observe(lk)
			}
			for i := 0; i < 2; i++ {
				lk := readLook(first, 1, observe.ScreenReading{
					Text: submenu, FromLook: 2,
					Generation: 1, Epoch: o.Epoch(),
				})
				tc.spoil(&lk)
				o.Observe(lk)
			}
			st := o.Stats()
			if st.ReadingsAdmitted != 0 {
				t.Errorf("%d reading(s) admitted on an unusable look",
					st.ReadingsAdmitted)
			}
			if st.ReadingsUnusable != 2 {
				t.Errorf("unusable %d, want 2", st.ReadingsUnusable)
			}
		})
	}
}

// AND A RUN WITH NO READINGS REPORTS NONE AND FOLDS AS IT ALWAYS DID.
func TestARunWithNoReadingsIsTheRunItAlwaysWas(t *testing.T) {
	first, second := twoMenusThatMerge()
	o := drained(t)

	var settled observe.VisualScreenID
	for i := 0; i < 6; i++ {
		settled = o.Observe(readLook(first, 1)).Screen
	}
	var after observe.VisualScreenID
	for i := 0; i < 6; i++ {
		after = o.Observe(readLook(second, 1)).Screen
	}
	// THE TWO MENUS STILL MERGE, which is the behaviour the pixel path keeps.
	if after != settled {
		t.Errorf("two merging arrangements settled as %q and %q with nothing read; "+
			"the pixel-only path has changed", settled, after)
	}
	st := o.Stats()
	if st.ReadingsOffered != 0 || st.ReadingsAdmitted != 0 ||
		st.ReadingsRefusedMerge != 0 {
		t.Errorf("a run with no readings reports offered=%d admitted=%d refused=%d",
			st.ReadingsOffered, st.ReadingsAdmitted, st.ReadingsRefusedMerge)
	}
}

// AND A READING THAT FAILED IS COUNTED APART FROM ONE THAT FOUND NO WORDS.
func TestAFailedReadingIsCountedApartFromASilentScreen(t *testing.T) {
	first, _ := twoMenusThatMerge()
	o := drained(t)
	for i := 0; i < 6; i++ {
		lk := readLook(first, 1)
		lk.Text = words("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
		o.Observe(lk)
	}
	// A SOURCE THAT ANSWERED AND READ NOTHING: admitted, and says nothing.
	o.Observe(readLook(first, 1, observe.ScreenReading{
		Text: observe.TextOf(directorapi.SourceOCR, nil), FromLook: 2,
		Generation: 1, Epoch: o.Epoch(),
	}))
	st := o.Stats()
	if st.ReadingsAdmitted != 1 {
		t.Errorf("a silent reading was not admitted: %d", st.ReadingsAdmitted)
	}
	if st.ReadingsSilent != 1 {
		t.Errorf("silent %d, want 1", st.ReadingsSilent)
	}
	if st.ReadingsRefusedMerge != 0 {
		t.Errorf("a silent reading refused %d merge(s)", st.ReadingsRefusedMerge)
	}
}
