package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A window Marco can see, whose content it could not read.
//
// # The live failure this file preserves
//
// Windows Settings, right application, right window, in front, full screen — and an accessibility
// reading of SIXTEEN structures where the same page had been learned with a hundred and
// forty-eight. Caption buttons, a title strip, an account tile, and one 1594x926 rectangle
// covering three quarters of the frame with nothing observed inside it. Settings is a hosted
// application; when its content is suspended or unpainted the tree collapses to the frame.
//
// Marco reported "I don't recognise this screen" — true, and it sent the diagnosis to the page
// rather than to the reading, for three runs and the better part of an hour.
//
// The numbers below are taken from that run. They are the REGRESSION CASE, not the rule: nothing
// here compares 16 against 148, and nothing may, because a responsive page, a resized window or a
// personalised one legitimately change how rich a screen is.

// shell is the live failure, reconstructed: a window frame with an empty content area.
//
// Geometry from the real reading, normalised to the 1936x1048 frame it was measured in. Two tracks
// share the content rectangle because the provider reported it twice, which is exactly what
// happened and is worth keeping — it means the emptiness test cannot simply count to zero.
func shell() observe.ShadowTotals {
	return totalsOf("state_1",
		seenAt("close", 0.972, 0.008, 0.024, 0.031),
		seenAt("maximise", 0.948, 0.008, 0.024, 0.031),
		seenAt("minimise", 0.925, 0.008, 0.024, 0.031),
		seenAt("corner", 0.004, 0.008, 0.011, 0.021),
		seenAt("appicon", 0.002, 0.006, 0.021, 0.034),
		seenAt("titlestrip", 0.367, 0.015, 0.267, 0.031),
		seenAt("titlerule", 0.030, 0.023, 0.869, 0.015),
		seenAt("accountpic", 0.012, 0.062, 0.031, 0.057),
		seenAt("accountname", 0.050, 0.074, 0.081, 0.018),
		seenAt("accountmail", 0.050, 0.093, 0.081, 0.015),
		seenAt("searchbox", 0.320, 0.059, 0.040, 0.036),
		// AND THE PAGE THAT NEVER ARRIVED.
		seenAt("content", 0.173, 0.109, 0.823, 0.884),
		seenAt("content-again", 0.173, 0.109, 0.823, 0.884),
	)
}

// page is the same window with its content read: the chrome, and structures spread through the
// area the shell reading left empty.
func page() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("close", 0.972, 0.008, 0.024, 0.031),
		seenAt("maximise", 0.948, 0.008, 0.024, 0.031),
		seenAt("titlestrip", 0.367, 0.015, 0.267, 0.031),
		seenAt("content", 0.173, 0.109, 0.823, 0.884),
	}
	// Twenty cards down the page. Far fewer than the real hundred and forty-eight, and that
	// is deliberate: if this needed to be rich to pass, the rule would be about richness.
	for i := 0; i < 20; i++ {
		tracks = append(tracks, seenAt(
			"card", 0.20, 0.15+float64(i)*0.04, 0.70, 0.03))
	}
	return totalsOf("state_1", tracks...)
}

// dialog is a small, legitimate window: a message and a couple of buttons inside a body that
// covers most of it. Sparse, and read perfectly well.
func dialog() observe.ShadowTotals {
	return totalsOf("state_1",
		seenAt("close", 0.94, 0.02, 0.05, 0.06),
		seenAt("title", 0.05, 0.02, 0.60, 0.06),
		seenAt("body", 0.02, 0.10, 0.96, 0.80),
		seenAt("message", 0.06, 0.20, 0.88, 0.10),
		seenAt("icon", 0.06, 0.20, 0.08, 0.10),
		seenAt("ok", 0.60, 0.75, 0.15, 0.08),
		seenAt("cancel", 0.78, 0.75, 0.15, 0.08),
	)
}

// THE LIVE FAILURE, AND THE THREE THINGS THAT MUST NOT BE CONFUSED WITH IT.
func TestASparseWindowIsNotADegradedOne(t *testing.T) {
	for _, c := range []struct {
		name  string
		given observe.ShadowTotals
		want  observe.Reach
		why   string
	}{
		{name: "a window whose content never arrived", given: shell(),
			want: observe.ReachShell,
			why: "three quarters of the frame came back as one rectangle with nothing " +
				"in it, and every structure observed was window furniture"},
		{name: "the same window with its page read", given: page(),
			want: observe.ReachContent,
			why:  "the content area is full of things, which is what reading a page means"},
		{name: "a small dialog, sparse and perfectly readable", given: dialog(),
			want: observe.ReachContent,
			why: "few controls is not the same fact as no controls. A dialog puts a large " +
				"SHARE of what it has inside its body; the degraded window put one " +
				"structure of thirteen inside a region covering most of the frame"},
		{name: "a window with more furniture than page", given: toolbarHeavy(),
			want: observe.ReachContent,
			why: "a quarter of what was observed is inside the content area — far more " +
				"than an empty window and far less than a dialog. A bar set anywhere " +
				"between those two calls this degraded, and it was read perfectly well"},
		{name: "a big empty panel beside a full one", given: emptyPanel(),
			want: observe.ReachContent,
			why: "the reading pane is blank because nothing is selected, and the message " +
				"list beside it is full. A window with a populated panel was READ; the " +
				"Settings failure had nowhere at all with anything in it"},
		{name: "a floating palette with an empty swatch", given: palette(),
			want: observe.ReachContent,
			why: "nothing here is big enough to be a populated panel, so the panel rule " +
				"cannot save it. What does is that its empty space is a SWATCH and not " +
				"a page — which is the only thing minVacantShare is for"},
		{name: "too little observed to judge", given: totalsOf("state_1",
			seenAt("window", 0.0, 0.0, 1.0, 1.0), seenAt("label", 0.1, 0.1, 0.2, 0.05)),
			want: observe.ReachContent,
			why: "two structures say nothing about arrangement, and an observation that " +
				"thin is already refused for not settling"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, evidence := observe.ReachOfState(c.given, "state_1")
			if got != c.want {
				t.Fatalf("reach = %q, want %q: %s\n(vacancy %+v)", got, c.want, c.why,
					evidence)
			}
		})
	}
}

// AND THE EVIDENCE IS KEPT, because the diagnosis is the useful part.
//
// A person told "I can see Settings but not the page" wants to know how much of the window came
// back empty; a future observer deciding whether another way of looking is worth trying wants the
// same fact. A bare boolean would answer neither.
func TestAShellReadingKeepsWhatMadeItOne(t *testing.T) {
	got, v := observe.ReachOfState(shell(), "state_1")
	if got != observe.ReachShell {
		t.Fatalf("reach = %q", got)
	}
	if !v.Found() {
		t.Fatal("a shell reading reported no vacancy; there is nothing to explain it with")
	}
	if v.Share < 0.5 {
		t.Errorf("the empty space covers %.0f%% of the window", v.Share*100)
	}
	if v.Structures != 13 {
		t.Errorf("it counted %d structures, and the reading had 13", v.Structures)
	}
	if v.Inside > 1 {
		t.Errorf("%d structures were found inside the empty space", v.Inside)
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// seenAt is one observed structure, at a normalised place in its window.
func seenAt(id string, x, y, w, h float64) observe.ShadowTrack {
	return observe.ShadowTrack{
		ID: id, Role: "unknown", Present: true,
		Reference: observe.Region{X: x, Y: y, Width: w, Height: h},
		States:    []observe.TrackState{{State: "state_1", Seen: 3, Eligible: 3}},
	}
}

func totalsOf(state observe.ScreenStateID, tracks ...observe.ShadowTrack) observe.ShadowTotals {
	return observe.ShadowTotals{CurrentState: state, Tracks: tracks,
		States: []observe.ScreenState{{ID: state, Inferences: 3, Settled: true}}}
}

// A SHELL-ONLY READING IS NOT AN UNKNOWN PLACE, AND MEMORY IS NOT ASKED ABOUT IT.
//
// # Why the recall is skipped rather than allowed to miss
//
// A shell reading describes the frame every page of an application shares. Handing it to memory
// asks "which screen is this" about evidence containing no screen, and the answer would be honest
// and useless: MatchDifferent, reported upwards as "I looked and did not know it" about a page
// nobody ever read.
//
// The miss is the lie. So the question is not asked, and the Place says why instead.
//
// The recogniser here records whether it was consulted, because a version that asks and then
// discards the answer would satisfy every assertion about the outcome while still spending the
// lookup and still being wrong about what it knows.
func TestAShellOnlyReadingIsNotAnUnknownPlace(t *testing.T) {
	t.Run("the window with no page in it", func(t *testing.T) {
		m := &countingRecogniser{}
		p := observe.PlaceNow(shell(), "settings", m, observe.DefaultHypothesisThresholds())

		if p.Readable() {
			t.Fatal("a reading that got no further than the window frame was treated as " +
				"a page Marco simply does not recognise")
		}
		if p.Reach != observe.ReachShell {
			t.Errorf("reach = %q", p.Reach)
		}
		if !p.Placed {
			t.Error("the reading described SOMETHING — the window — and Placed says so; " +
				"collapsing it into `could not look` loses the window")
		}
		if p.Established() {
			t.Fatal("a shell reading established a Place")
		}
		if m.asked != 0 {
			t.Errorf("memory was asked to identify a screen from evidence with no screen "+
				"in it (%d time(s))", m.asked)
		}
		if !p.Vacancy.Found() {
			t.Error("no evidence was kept for why")
		}
	})

	t.Run("and a page Marco really does not know still asks", func(t *testing.T) {
		// The negative control. Without it everything above passes for a PlaceNow that
		// stopped consulting memory at all.
		m := &countingRecogniser{}
		p := observe.PlaceNow(page(), "settings", m, observe.DefaultHypothesisThresholds())

		if !p.Readable() {
			t.Fatal("a page that was read perfectly well was called unreadable")
		}
		if m.asked == 0 {
			t.Fatal("memory was never asked about a screen that was read; every unknown " +
				"page would now report as unreadable")
		}
		if p.Established() {
			t.Error("it matched something, and this recogniser remembers nothing")
		}
	})
}

// countingRecogniser remembers nothing and counts being asked.
type countingRecogniser struct{ asked int }

func (c *countingRecogniser) Recall(string, observe.StructureSignature) observe.Recollection {
	c.asked++
	return observe.Recollection{Verdict: observe.MatchDifferent}
}

// toolbarHeavy is a window with more furniture than page: ribbons, a status bar, a side rail,
// and a document with only a few things in it.
//
// The case that lives in the gap between the two above, and the reason the occupancy bar is where
// it is. A quarter of this window's structures are inside its content area -- comfortably more
// than the empty window's one-in-thirteen, and far less than a dialog's two-thirds. A bar set
// anywhere in the middle calls this degraded, and it is a page that was read perfectly well.
func toolbarHeavy() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("content", 0.14, 0.12, 0.84, 0.80),
	}
	// A ribbon across the top, a rail down the left, a status bar along the bottom.
	for i := 0; i < 12; i++ {
		tracks = append(tracks, seenAt("ribbon", 0.02+float64(i)*0.07, 0.03, 0.06, 0.05))
	}
	for i := 0; i < 6; i++ {
		tracks = append(tracks, seenAt("rail", 0.02, 0.15+float64(i)*0.09, 0.09, 0.06))
	}
	for i := 0; i < 3; i++ {
		tracks = append(tracks, seenAt("status", 0.05+float64(i)*0.20, 0.95, 0.15, 0.03))
	}
	// And the document, with a handful of things in it.
	for i := 0; i < 8; i++ {
		tracks = append(tracks, seenAt("para", 0.20, 0.20+float64(i)*0.07, 0.60, 0.04))
	}
	return totalsOf("state_1", tracks...)
}

// emptyPanel is a mail client with nothing selected: a full list of messages down the left, and a
// large blank reading pane beside it.
//
// The case the emptiness test alone gets WRONG. That pane is big and empty for a perfectly good
// reason — the application has nothing to show there — and the window was read correctly. What
// separates it from the live Settings failure is that somewhere in it IS populated.
func emptyPanel() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("close", 0.97, 0.01, 0.02, 0.03),
		seenAt("toolbar", 0.02, 0.02, 0.90, 0.04),
		seenAt("list", 0.02, 0.08, 0.30, 0.88),
		// AND THE READING PANE, empty because nothing is selected.
		seenAt("reading", 0.34, 0.08, 0.64, 0.88),
	}
	for i := 0; i < 12; i++ {
		tracks = append(tracks, seenAt("message", 0.04, 0.10+float64(i)*0.07, 0.26, 0.05))
	}
	return totalsOf("state_1", tracks...)
}

// palette is a floating tool window: a grid of small buttons and a modest preview swatch that
// happens to be empty.
//
// The case where minVacantShare is the only thing standing between a correct reading and a wrong
// verdict. Nothing in this window is big enough to be a populated panel, so the panel rule cannot
// save it; what saves it is that its empty space is a SWATCH rather than a page. Drop the bar and
// a perfectly readable palette is called unreadable.
func palette() observe.ShadowTotals {
	var tracks []observe.ShadowTrack
	for row := 0; row < 5; row++ {
		for col := 0; col < 4; col++ {
			tracks = append(tracks, seenAt("tool",
				0.05+float64(col)*0.16, 0.05+float64(row)*0.09, 0.14, 0.07))
		}
	}
	tracks = append(tracks, seenAt("preview", 0.35, 0.60, 0.30, 0.25))
	return totalsOf("state_1", tracks...)
}
