package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// "ENTRY 3" HAS TO MEAN THE THIRD ENTRY FROM THE TOP.
//
// # The defect these pin
//
// `InterfaceBelief.Selected` is documented as the chosen entry "counted from the top", and it
// was not. Grouping returns the stack's members top to bottom and then appends the aligned
// STRAYS — a menu's last entry, set apart by a wider gap — in the order deduplication left
// them, which is largest-area first. Any list with a stray handed the reader a sequence in
// which position 5 was somewhere in the middle of the screen.
//
// Measured on 864 frames of a real pause menu against labels naming which of nine entries was
// selected: the belief read a nine-entry list on 139 frames and named the right ordinal on
// NONE of them. It was not wrong about which rectangle was marked. It was wrong about which
// entry that rectangle was, and that is the only form of the answer a route can use — "move
// the mark to entry 5" is meaningless otherwise.

// mixedList draws a list of five rows with three THIN rows interleaved between them.
//
// The thin rows are a separate group — they are nothing like the real rows in height, so
// grouping refuses to join them — and they sit INSIDE the real list's envelope, which is the
// case that matters. `kept` is every grouped row inside the identified candidate's extent, in
// the order grouping emitted them: the stack's members first, top to bottom, then everything
// else. So the thin rows arrive at the end of the sequence while sitting in the middle of the
// screen, and the index the reader returns is an index into THAT.
//
// A stray below the stack cannot show this, which is what the first version of this fixture
// tried: a stray is outside the candidate's extent, so it never joins the belief's entries at
// all. The out-of-order case comes from inside.
func mixedList(seq int, at time.Time, mark int) observe.Sample {
	const (
		rows  = 5
		y0    = 0.10
		pitch = 0.120
		h     = 0.080
	)
	var shapes []observe.ShadowShape
	add := func(y, height, width float64, bright bool) {
		base := 50.0 + float64(len(shapes))*7
		shade := []float64{base, base + 30}
		if bright {
			shade = []float64{base + 150, base + 180}
		}
		shapes = append(shapes, observe.ShadowShape{
			Region:     observe.Region{X: 0.05, Y: y, Width: width, Height: height},
			Confidence: 0.3, Shade: shade,
		})
	}
	for row := range rows {
		add(y0+float64(row)*pitch, h, 0.30, row == mark)
	}
	// THE INTERLOPERS: three thin rows in the gaps, on their own regular beat, and
	// narrow enough to sit INSIDE the real list's extent — which is the condition for
	// them to join the belief's entries at all. A row wider than the candidate is not
	// inside it, and the first version of this fixture drew them 0.32 wide against an
	// extent 0.30 wide, so none of them ever arrived.
	for i := range 3 {
		add(y0+float64(i)*pitch+0.090, 0.022, 0.20, false)
	}
	return observe.Sample{
		Sequence: seq, WindowGeneration: 1, Timestamp: at,
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true, Shapes: shapes,
		},
	}
}

// The entry the belief names is the one at that position from the top.
//
// Deleting the sort in entriesInside must fail this.
func TestTheEntriesOfAListAreNumberedFromTheTop(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	feed := func(mark int) {
		seq++
		totals.Observe(mixedList(seq,
			base.Add(time.Duration(seq)*100*time.Millisecond), mark))
	}
	for range 14 {
		feed(0)
	}
	// Several passes down the list: it is not identified until it has been seen handing
	// its mark around, which takes most of the first pass.
	checked, mixed := 0, false
	for step := range 40 {
		mark := 1 + step%4
		for range 3 {
			feed(mark)
		}
		b := totals.Interface
		if b.Status != observe.InterfaceProvisional || b.Selected < 0 {
			continue
		}
		if b.Entries > 5 {
			// The precondition: the interlopers are inside the envelope, so the
			// sequence the index counts is one grouping did not emit in order.
			mixed = true
		}
		checked++
		if b.SelectedWhere.Height <= 0 {
			t.Errorf("entry %d of %d named with no region", b.Selected, b.Entries)
			continue
		}
		// THE ASSERTION. The believed entry's region must be the row the mark is on.
		// With the entries in grouping order rather than screen order, these disagree.
		want := 0.10 + float64(mark)*0.120
		if got := b.SelectedWhere.Y; got < want-0.01 || got > want+0.01 {
			t.Errorf("the mark is on the row at y %.3f and the belief names entry %d of "+
				"%d, whose region is at y %.3f. The entries are not in the order the "+
				"index counts them in", want, b.Selected, b.Entries, got)
		}
		// AND THE INDEX ITSELF, which is the half a route uses. With three thin rows
		// above or between the real ones, the mark's ordinal is its position among ALL
		// the entries the belief has, counted from the top.
		if b.Entries > 5 {
			ordinal := mark
			for i := range 3 {
				if 0.10+float64(i)*0.120+0.090 < want {
					ordinal++
				}
			}
			if b.Selected != ordinal {
				t.Errorf("the mark is the %dth entry from the top of %d and the belief "+
					"calls it entry %d", ordinal, b.Entries, b.Selected)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("the belief never named an entry, so nothing here was checked: %s",
			totals.Interface.Because)
	}
	if !mixed {
		t.Fatalf("the belief never held more than the five real rows, so grouping emitted "+
			"them in order anyway and this test proves nothing. entries=%d",
			totals.Interface.Entries)
	}
	t.Logf("checked %d readings of a list with interleaved rows from another group", checked)
}

// ── the window a press stays attributable for ─────────────────────────────────

// A press stays attributable for as long as the gap between the looks that could see it.
//
// # Why this is not a loosening
//
// It is what [[ADR-158-the-active-list-is-the-one-that-responds]] says the rule is, and the
// code implemented half of it. The attributable interval is "the gap since the last look OR the
// latency bound, whichever is longer", and only the bound was ever set. At the recording's own
// 66ms that is invisible, because 400ms spans six looks either way. At a live cadence it is the
// whole thing: the menu repaints just as fast, but a response cannot be SEEN before the look
// that sees it, and the next look is two seconds away.
//
// # What it is measured on, and what it is not
//
// On the CANDIDATE's attribution — the tracker's own count of transfers that followed observed
// navigation, which is the evidence that tells two real lists apart. That is where the window
// does its work: the tracker sees a transfer as soon as the shading changes, which at a 2s
// cadence is the very next look.
//
// NOT on the retained moves. Those are recorded when the selection reader CONFIRMS a new row,
// which it does a look or two later still, and no defensible window reaches that far. That gap
// is real, measured at seventeen moves with one attributed, and recorded as a known gap rather
// than closed by widening this.
//
// Deleting the window() call in Navigated, so the debt is always MaxResponseLatency, must fail
// this.
func TestAPressStaysAttributableAcrossACoarseCadence(t *testing.T) {
	// A cadence far coarser than the latency bound, like a live Director's.
	const cadence = 2 * time.Second
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	feed := func(mark int, intents []observe.NavIntent) {
		seq++
		totals.Observe(oneListSample(seq,
			base.Add(time.Duration(seq)*cadence), mark, intents))
	}
	for range 14 {
		feed(0, nil)
	}
	mark := 0
	for range 30 {
		// The press, on one look; the shading different on the NEXT, a whole cadence
		// later and five times the latency bound after the press.
		feed(mark, []observe.NavIntent{observe.NavDown})
		mark = (mark + 1) % 5
		feed(mark, nil)
		feed(mark, nil)
	}

	transfers, prompted := 0, 0
	for _, c := range totals.Candidates {
		transfers += c.Transfers
		prompted += c.Prompted
	}
	if transfers == 0 {
		t.Fatalf("no transfer was measured at a %s cadence, so there is nothing to "+
			"attribute: %s", cadence, totals.Interface.Because)
	}
	// MOST of them, not merely one. Every transfer in this fixture is the look after a
	// press, so a window that reaches the next look attributes nearly all of them and a
	// window of 400ms against a 2s cadence attributes only the handful the tracker
	// happens to see on the press's own look.
	if prompted*2 < transfers {
		t.Errorf("%d of %d transfers attributed at a %s cadence. Every one of them was "+
			"the look after a press, and a window shorter than the gap between looks "+
			"cannot reach them", prompted, transfers, cadence)
	}
	t.Logf("%s cadence: %d transfers, %d attributed", cadence, transfers, prompted)
}

// And a gap far longer than any sampling interval does not license an attribution.
//
// The other side of the window. If Marco last looked a minute ago, "something was pressed
// during that minute and the mark is now elsewhere" is not an attribution, it is a coincidence
// with a long name. TrackStaleness bounds it, reusing the existing answer to how old an
// observation may be and still describe the same arrangement rather than inventing a second
// number for the same question.
//
// The looks here carry REAL GEOMETRY, which the first version of this test did not — and
// without it `navRecent` is never reached, no interval is ever recorded, and the test passes
// because the window fell back to the latency bound rather than because the bound held.
//
// Deleting the TrackStaleness clamp in window() must fail this.
func TestAPressIsNotAttributableAcrossAnUnboundedGap(t *testing.T) {
	watch := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	rows := func(mark int) []observe.ShadowShape {
		var out []observe.ShadowShape
		for row := range 5 {
			b := 50 + float64(row)*9
			shade := []float64{b, b + 30}
			if row == mark {
				shade = []float64{b + 150, b + 180}
			}
			out = append(out, observe.ShadowShape{
				Region: observe.Region{
					X: 0.05, Y: 0.10 + float64(row)*0.120, Width: 0.30, Height: 0.080,
				},
				Confidence: 0.3, Shade: shade,
			})
		}
		return out
	}
	// Three looks A MINUTE APART, so the remembered interval is a minute.
	for i := range 3 {
		watch.Observe(observe.InterfaceLook{
			Sequence: i + 1, At: base.Add(time.Duration(i) * time.Minute),
			Observed: true, Shapes: rows(0),
		})
	}
	// THE PRESS, on the last of them.
	watch.Observe(observe.InterfaceLook{
		Sequence: 4, At: base.Add(3 * time.Minute),
		Observed: true, Shapes: rows(0), Navs: 1,
	})
	// Ten seconds later: past TrackStaleness three times over, and far inside the
	// minute-long window an unbounded rule would have opened.
	at := base.Add(3*time.Minute + 10*time.Second)
	if observe.NavWindowOpenForTest(watch, at) {
		t.Errorf("a press is still attributable %s later because the looks happened to be "+
			"a minute apart. The bound is %s", 10*time.Second, observe.TrackStaleness)
	}
	// And the control: it IS attributable a moment after the press, or this test would
	// pass against a window of zero.
	soon := observe.NewInterfaceWatch()
	for i := range 3 {
		soon.Observe(observe.InterfaceLook{
			Sequence: i + 1, At: base.Add(time.Duration(i) * time.Minute),
			Observed: true, Shapes: rows(0),
		})
	}
	soon.Observe(observe.InterfaceLook{
		Sequence: 4, At: base.Add(3 * time.Minute),
		Observed: true, Shapes: rows(0), Navs: 1,
	})
	if !observe.NavWindowOpenForTest(soon, base.Add(3*time.Minute+100*time.Millisecond)) {
		t.Error("a press is not attributable 100ms after it, so the window is closed " +
			"altogether and the bound above proves nothing")
	}
}
