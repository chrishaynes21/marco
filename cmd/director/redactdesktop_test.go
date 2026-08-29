package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE REDACTION LOGIC NEEDS ITS OWN TESTS, AND ALMOST DID NOT GET THEM.
//
// The corpus tests next door guard the committed ARTIFACTS: they fail if a published sample
// carries an email address or an offscreen element. That is the right thing to guard, and it
// is not enough. A mutation run over redactdesktop.go killed nothing at all — seven of seven
// survived, including "accept every element as on-screen" and "clear no labels" — because the
// artifacts were already clean and mutating the producer could not make them dirty.
//
// So the code that decides what gets removed had no test, in the one place where the failure
// is silent and permanent: it does not break a build, it publishes a person's name.
//
// These are the mutations that must now fail. Each is named where it is checked.

func TestWithinFrameIsTheVisibleWindow(t *testing.T) {
	cases := []struct {
		name string
		r    observe.Region
		want bool
	}{
		{"dead centre", observe.Region{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.2}, true},
		{"the whole frame", observe.Region{X: 0, Y: 0, Width: 1, Height: 1}, true},
		// Half below the fold IS on screen. A list item whose top edge is visible can be
		// seen, boxed and clicked, and excluding it would charge a detector for missing
		// something that was drawn.
		{"straddling the bottom edge", observe.Region{X: 0.1, Y: 0.9, Width: 0.2, Height: 0.4}, true},
		{"straddling the top edge", observe.Region{X: 0.1, Y: -0.05, Width: 0.2, Height: 0.2}, true},
		// The Explorer nav pane sat here: y = -1.95 of the window height, in the reading
		// and absent from the picture. This is the case the whole filter exists for.
		{"far above the window", observe.Region{X: 0.055, Y: -1.95, Width: 0.08, Height: 0.04}, false},
		{"far below the window", observe.Region{X: 0.1, Y: 2.4, Width: 0.2, Height: 0.05}, false},
		{"off to the right", observe.Region{X: 1.5, Y: 0.4, Width: 0.2, Height: 0.05}, false},
		{"off to the left", observe.Region{X: -0.9, Y: 0.4, Width: 0.2, Height: 0.05}, false},
		// Flush against the far edge and no further: zero visible pixels.
		// Kills "< 1 becomes <= 1".
		{"begins exactly at the right edge", observe.Region{X: 1, Y: 0.4, Width: 0.2, Height: 0.05}, false},
		{"begins exactly at the bottom edge", observe.Region{X: 0.4, Y: 1, Width: 0.2, Height: 0.05}, false},
		// Ends exactly at the near edge: also zero visible pixels.
		// Kills "drop the near-edge clause" and "accept everything".
		{"ends exactly at the left edge", observe.Region{X: -0.2, Y: 0.4, Width: 0.2, Height: 0.05}, false},
		{"ends exactly at the top edge", observe.Region{X: 0.4, Y: -0.05, Width: 0.2, Height: 0.05}, false},
	}
	for _, c := range cases {
		if got := withinFrame(c.r); got != c.want {
			t.Errorf("%s: withinFrame(%+v) = %v, want %v", c.name, c.r, got, c.want)
		}
	}
}

// A region contains an element by its CENTRE, not by its corner.
//
// Kills "centre becomes top-left". The distinction is not academic: the Settings account
// header sits at the top-left of the window, so an element whose top-left is above or left of
// the redaction but whose body is inside it would keep its label under a corner test — and
// the label is the person's name.
func TestInsideRegionUsesTheCentre(t *testing.T) {
	region := observe.Region{X: 0.2, Y: 0.2, Width: 0.4, Height: 0.4}

	// Top-left outside, centre inside. A corner test says no; this must say yes.
	straddling := observe.Region{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.4}
	if !insideRegion(straddling, region) {
		t.Error("an element whose centre is inside the region was not redacted.\n" +
			"insideRegion is testing a corner, not the centre — an element half-outside " +
			"a redaction keeps its label, and that label is the thing being removed.")
	}

	// Top-left inside, centre well outside. A corner test says yes; this must say no.
	escaping := observe.Region{X: 0.55, Y: 0.55, Width: 0.9, Height: 0.9}
	if insideRegion(escaping, region) {
		t.Error("an element whose centre is outside the region was redacted anyway.\n" +
			"Over-redaction is not safe by default: it silently deletes the labels the " +
			"corpus exists to measure against.")
	}

	if insideRegion(observe.Region{X: 0.8, Y: 0.8, Width: 0.1, Height: 0.1}, region) {
		t.Error("an element nowhere near the region was redacted")
	}

	// Outside on ONE axis only, in each direction. A case that is outside on both — the
	// obvious one to write — passes even when a bound has been deleted, because the other
	// axis still rejects it. Each of these kills exactly one dropped bound.
	outside := []struct {
		name string
		r    observe.Region
	}{
		{"past the right edge", observe.Region{X: 0.9, Y: 0.35, Width: 0.05, Height: 0.05}},
		{"left of the left edge", observe.Region{X: 0.0, Y: 0.35, Width: 0.05, Height: 0.05}},
		{"below the bottom edge", observe.Region{X: 0.35, Y: 0.9, Width: 0.05, Height: 0.05}},
		{"above the top edge", observe.Region{X: 0.35, Y: 0.0, Width: 0.05, Height: 0.05}},
	}
	for _, c := range outside {
		if insideRegion(c.r, region) {
			t.Errorf("%s: an element outside the region on one axis was redacted.\n"+
				"A bound is missing, and over-redaction deletes the labels the corpus "+
				"exists to measure against.", c.name)
		}
	}
}

// emailish matches an address, which is the whole of what the privacy gate can prove.
//
// Without this the pattern can be anything at all: every test that uses it asserts something
// is NOT found, and a regexp that never matches satisfies all of them. That mutation survived
// a full run, which is how this test came to exist.
func TestEmailishFindsAnAddress(t *testing.T) {
	found := []string{
		"someone@example.com",
		"first.last+tag@sub.example.co.uk",
		"Chris Haynes someone@example.com",
		"a_b-c%d@example.io",
	}
	for _, s := range found {
		if !emailish.MatchString(s) {
			t.Errorf("emailish did not match %q — the privacy gate is inert and every "+
				"test that relies on it passes for the wrong reason", s)
		}
	}
	for _, s := range []string{"", "Chris Haynes", "Bluetooth & devices", "@", "a@b"} {
		if emailish.MatchString(s) {
			t.Errorf("emailish matched %q, which is not an address; over-matching scrubs "+
				"labels the corpus needs", s)
		}
	}
}

// redactSample clears the labels it says it cleared, and drops what it says it dropped.
//
// Kills "stop clearing labels" and "count offscreen but keep it" — the two mutations that
// would let the command report a clean pass having done nothing.
func TestRedactSampleRemovesWhatItReports(t *testing.T) {
	s := desktopSample{
		ID: "t", Width: 1000, Height: 1000,
		Elements: []desktopElement{
			{ID: "keep", Role: "button", Label: "Check now",
				Bounds: observe.Region{X: 0.6, Y: 0.6, Width: 0.1, Height: 0.05}},
			{ID: "scrub", Role: "text", Label: "someone@example.com",
				Bounds: observe.Region{X: 0.05, Y: 0.05, Width: 0.1, Height: 0.02}},
			{ID: "gone", Role: "tree_item", Label: "a private folder",
				Bounds: observe.Region{X: 0.05, Y: -1.95, Width: 0.08, Height: 0.04}},
		},
	}
	region := observe.Region{X: 0, Y: 0, Width: 0.3, Height: 0.1}

	res, err := redactSample(&s, []observe.Region{region}, "test", false,
		filepath.Join(t.TempDir(), "absent.png"))
	if err != nil {
		t.Fatalf("redactSample: %v", err)
	}

	if res.dropped != 1 || s.OffscreenDropped != 1 {
		t.Errorf("dropped %d offscreen elements (sample says %d), want 1",
			res.dropped, s.OffscreenDropped)
	}
	if len(s.Elements) != 2 {
		t.Fatalf("kept %d elements, want 2 — the offscreen one is still in the sample",
			len(s.Elements))
	}
	for _, e := range s.Elements {
		if e.ID == "gone" {
			t.Error("the offscreen element survived: it was counted as dropped and kept")
		}
		if e.ID == "scrub" && e.Label != "" {
			t.Errorf("the element inside the redaction still reads %q.\n"+
				"The pass reported success and removed nothing.", e.Label)
		}
		if e.ID == "keep" && e.Label != "Check now" {
			t.Error("an element outside the redaction lost its label")
		}
	}
	if res.labels != 1 {
		t.Errorf("reported %d labels cleared, want 1", res.labels)
	}

	// --keep-offscreen means keep. The flag exists so a sample can be inspected whole, and
	// a pass that drops regardless would quietly discard the evidence it was asked to show.
	whole := desktopSample{ID: "t", Width: 1000, Height: 1000, Elements: []desktopElement{
		{ID: "gone", Role: "tree_item", Label: "a private folder",
			Bounds: observe.Region{X: 0.05, Y: -1.95, Width: 0.08, Height: 0.04}},
	}}
	if res, err := redactSample(&whole, nil, "test", true, ""); err != nil {
		t.Fatalf("redactSample with keepOffscreen: %v", err)
	} else if res.dropped != 0 || len(whole.Elements) != 1 {
		t.Errorf("keepOffscreen dropped %d elements and kept %d; it must keep them all",
			res.dropped, len(whole.Elements))
	}
	if len(s.Redactions) != 1 || s.Redactions[0].Labels != 1 {
		t.Errorf("the sample does not record what was removed: %+v\n"+
			"A redaction nobody can see in the artifact is not reviewable.", s.Redactions)
	}
	if got := scanForEmails(&s); len(got) != 0 {
		t.Errorf("an address survived the pass: %v", got)
	}
}

// blackOut actually paints, and paints the right rectangle.
//
// Without this the JSON can be spotless while the picture beside it still shows the name.
func TestBlackOutPaintsTheRegion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.png")
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// The top-left quarter.
	if err := blackOut(path, []observe.Region{{X: 0, Y: 0, Width: 0.5, Height: 0.5}}); err != nil {
		t.Fatalf("blackOut: %v", err)
	}

	g, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := png.Decode(g)
	_ = g.Close()
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := out.At(25, 25).RGBA(); r != 0 {
		t.Error("the pixel at the centre of the redacted region is not black.\n" +
			"The reading was scrubbed and the picture was not — which is the worse half " +
			"to get wrong, because a person reads the picture.")
	}
	if r, _, _, _ := out.At(75, 75).RGBA(); r == 0 {
		t.Error("a pixel outside the redacted region was painted black too; the region " +
			"geometry is wrong and the frame is being destroyed")
	}
}

// A missing frame is not an error, so the reading can still be cleaned.
func TestBlackOutToleratesAnAbsentFrame(t *testing.T) {
	if err := blackOut(filepath.Join(t.TempDir(), "nope.png"),
		[]observe.Region{{X: 0, Y: 0, Width: 1, Height: 1}}); err != nil {
		t.Errorf("blackOut on an absent frame: %v — a sample whose PNG was already "+
			"removed must still be scrubbable", err)
	}
}
