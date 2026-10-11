package main

import (
	"encoding/json"
	"image"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// THE RECORDING IS A CONTRACT, AND A SILENT BREAK IS THE DANGEROUS ONE.
//
// A replay reads JSON the recorder wrote. Rename a tag at either end and nothing fails: the
// field decodes to its zero value, the replay reports a session that saw nothing, and the
// report is indistinguishable from a real finding about a blind run. That failure mode has a
// name in this repository — a green test beside a production refusal, two copies of one fact —
// and a regression corpus is worth nothing if it can rot quietly.
//
// So the fields the replay depends on are named here, and a recorded line has to carry every
// one of them.

// goldenSlot is one line in the shape `shadowtrace.go` writes, with every field the replay
// reads populated and distinguishable.
//
// Hand-written rather than copied from a run: a fixture taken from one recording freezes that
// recording's accidents, and a field that happened to be absent there would never be tested.
const goldenSlot = `{
  "n": 42,
  "at_ms": 91173,
  "outcome": "valid",
  "generation": 3,
  "latency_ms": 1969,
  "detections": 7,
  "reason": "",
  "regions": [
    {"role": "button",
     "region": {"x": 0.25, "y": 0.30, "width": 0.20, "height": 0.04},
     "confidence": 0.47, "nameable": true, "shade": [60.8, 94.1]}
  ],
  "shapes": [
    {"region": {"x": 0.25, "y": 0.36, "width": 0.20, "height": 0.04},
     "confidence": 0.17, "shade": [72.0, 157.2]},
    {"region": {"x": 0.25, "y": 0.42, "width": 0.20, "height": 0.04},
     "confidence": 0.17, "shade": [102.7, 176.4]},
    {"region": {"x": 0.25, "y": 0.48, "width": 0.20, "height": 0.04},
     "confidence": 0.17, "shade": [88.1, 250.7]}
  ],
  "inputs": [
    {"intent": "down", "device": "gamepad", "at_ms": 91050}
  ]
}`

// Every field the replay depends on survives a round trip from a recorded line.
func TestTheReplayReadsEveryFieldItDependsOn(t *testing.T) {
	var s slot
	if err := json.Unmarshal([]byte(goldenSlot), &s); err != nil {
		t.Fatalf("a recorded line does not decode: %v", err)
	}

	for _, c := range []struct {
		name string
		got  any
		want any
	}{
		{"n", s.N, 42},
		{"at_ms", s.AtMS, int64(91173)},
		{"outcome", s.Outcome, "valid"},
		{"generation", s.Generation, uint64(3)},
		{"detections", s.Detections, 7},
		{"regions", len(s.Regions), 1},
		{"shapes", len(s.Shapes), 3},
		{"inputs", len(s.Inputs), 1},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s decoded as %v, want %v. A tag that no longer matches the "+
				"recorder makes a replay report a blind session, which is "+
				"indistinguishable from a finding", c.name, c.got, c.want)
		}
	}

	// THE SHADING IN PARTICULAR. It is the only thing that can answer which row holds the
	// mark, it is nested two levels down, and a replay that lost it would still group
	// rows and still report candidates — every one of them permanently quiet. That is
	// exactly Experiment-025's symptom, which is the reason this assertion is spelled out
	// rather than left to the length checks above.
	if len(s.Regions) > 0 && len(s.Regions[0].Shade) != selection.Features {
		t.Errorf("a region's shading decoded as %v; without it every candidate is quiet "+
			"and the replay reproduces the bug it is meant to diagnose",
			s.Regions[0].Shade)
	}
	for i, sh := range s.Shapes {
		if !sh.Shaded() {
			t.Errorf("shape %d decoded unshaded (%v), so it would be dropped before "+
				"grouping", i, sh.Shade)
		}
	}
	// And the geometry, because a zeroed rectangle still groups and still tracks.
	if len(s.Regions) > 0 && s.Regions[0].Region.Width == 0 {
		t.Error("a region decoded with no width: the nested `region` object did not land")
	}
}

// The replayed look reaches the production grouper, and the fixture's rows group.
//
// Not a test of GroupRows — a test that the look the replay BUILDS is one production would
// recognise. The fixture is four aligned rows at one pitch, which is a stack by any reading;
// if the replay's look construction were wrong, this is where it shows.
func TestAReplayedLookGroupsThroughProductionRules(t *testing.T) {
	var s slot
	if err := json.Unmarshal([]byte(goldenSlot), &s); err != nil {
		t.Fatal(err)
	}
	look := observe.InterfaceLook{
		Sequence: s.N, At: time.Unix(1700000000, 0), Observed: true,
		Regions: s.Regions, Shapes: s.Shapes,
	}
	sizes := observe.GroupedForLook(look)
	if len(sizes) == 0 {
		t.Fatal("four evenly spaced aligned rows produced no group at all")
	}
	if sizes[0] < minUsableStack {
		t.Errorf("the biggest group is %d rows, below the %d the tracker requires, from "+
			"a fixture of four aligned rows at one pitch: %v",
			sizes[0], minUsableStack, sizes)
	}
}

// minUsableStack is the number the tracker actually uses.
//
// It mirrors selection's unexported minStack, and the mirror is checked BEHAVIOURALLY rather
// than by exporting a perception threshold so a diagnostic can print it. A diagnostic wrong
// about this reports fragmentation that is not there, or misses the fragmentation that is.
func TestMinUsableStackIsWhatTheTrackerRequires(t *testing.T) {
	// One below: the watch must learn no candidate from it.
	short := observe.NewInterfaceWatch()
	feed(t, short, minUsableStack-1, 12)
	if n := len(short.Candidates()); n != 0 {
		t.Errorf("a stack of %d produced %d candidate(s); minUsableStack claims the "+
			"tracker ignores groups that small", minUsableStack-1, n)
	}
	// Exactly at it: a candidate exists.
	exact := observe.NewInterfaceWatch()
	feed(t, exact, minUsableStack, 12)
	if n := len(exact.Candidates()); n == 0 {
		t.Errorf("a stack of %d produced no candidate; minUsableStack claims that is "+
			"enough for the tracker to watch", minUsableStack)
	}
}

// feed gives a watch `looks` looks at `rows` evenly spaced shaded rectangles.
func feed(t *testing.T, w *observe.InterfaceWatch, rows, looks int) {
	t.Helper()
	base := time.Unix(1700000000, 0)
	for i := range looks {
		var shapes []observe.ShadowShape
		for r := range rows {
			shapes = append(shapes, observe.ShadowShape{
				Region: observe.Region{
					X: 0.25, Y: 0.30 + float64(r)*0.06, Width: 0.20, Height: 0.04,
				},
				Confidence: 0.2,
				// A level that differs per row but does not single any row out, so
				// this fixture tests grouping and not the mark.
				Shade: []float64{70 + float64(r), 150 + float64(r)},
			})
		}
		w.Observe(observe.InterfaceLook{
			Sequence: i + 1, At: base.Add(time.Duration(i) * time.Second),
			Observed: true, Shapes: shapes,
		})
	}
}

// A run whose slots carry no shading reports it as such rather than as a quiet screen.
//
// The distinction the whole L1 replay rests on: unshaded geometry is dropped before grouping,
// so a recording that lost its shading produces a session that looks blind. A replay has to be
// able to tell that from a screen with nothing on it.
func TestAReplayDistinguishesLostShadingFromAnEmptyScreen(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1700000000, 0)
	for i := range 6 {
		w.Observe(observe.InterfaceLook{
			Sequence: i + 1, At: base.Add(time.Duration(i) * time.Second),
			Observed: true,
			Shapes: []observe.ShadowShape{
				{Region: observe.Region{X: 0.25, Y: 0.3, Width: 0.2, Height: 0.04}},
				{Region: observe.Region{X: 0.25, Y: 0.4, Width: 0.2, Height: 0.04}},
				{Region: observe.Region{X: 0.25, Y: 0.5, Width: 0.2, Height: 0.04}},
			},
		})
	}
	if n := len(w.Candidates()); n != 0 {
		t.Errorf("unshaded geometry produced %d candidate(s); a rectangle nobody "+
			"measured cannot say whether it holds the mark", n)
	}
	if !strings.Contains(w.Belief().Because, "nothing shaded") {
		t.Errorf("the belief explains a run of unmeasurable geometry as %q, which does "+
			"not say the geometry could not be measured", w.Belief().Because)
	}
}

var _ = image.Rect // the slot's geometry is normalised; image is here for future pixel replay
