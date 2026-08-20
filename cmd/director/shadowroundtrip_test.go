package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The shadow report must survive the wire.
//
// Mandatory because this repository has lost new fields in protocol decode before: a value
// present in the runtime, absent by the time it reached the CLI, and reported as a zero that
// nobody questioned. Every number below is DISTINCTIVE and non-zero, so a zero-value decode
// cannot accidentally pass — a test asserting `>= 0` would have been green throughout the
// entire period the shadow report was dead.

func distinctiveShadow() observe.ShadowTotals {
	var t observe.ShadowTotals
	t.Add(observe.ShadowSample{
		Detector:     "screenparser",
		Ran:          true,
		TargetProven: true,
		Detections:   41,
		Nameable:     23,
		Unknown:      3,
		Roles:        map[string]int{"button": 17, "text": 11, "bar": 5},
		LatencyMS:    877,
		Comparison: observe.ShadowComparison{
			Agreed: 2, ShadowOnly: 37, AuthoritativeOnly: 1,
			RoleDisagreement: 4, GeometryDisagreement: 6, Uncomparable: 8,
			ShadowOnlyNameable: 19,
		},
	})
	// A distinctive track, so a zero-value decode cannot pass.
	t.Tracks = []observe.ShadowTrack{{
		ID: "shadow_7", Role: "button", Seen: 61, Eligible: 66, Episodes: 3,
		MeanIoU: 0.91, Nameable: true, Shape: observe.ShapeBursty,
	}}
	return t
}

func TestTheShadowReportSurvivesTheServiceRoundTrip(t *testing.T) {
	want := distinctiveShadow()
	view := observationView{
		ID: "observe_9", Samples: 1,
		Stats: observesession.Stats{Shadow: want},
	}

	// runtime → service encode
	wire, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("encoding the session view: %v", err)
	}
	if !strings.Contains(string(wire), "screenparser") {
		t.Fatalf("the encoded view does not mention the detector at all:\n%s", wire)
	}

	// → client decode
	var got observationView
	if err := json.Unmarshal(wire, &got); err != nil {
		t.Fatalf("decoding the session view: %v", err)
	}
	s := got.Stats.Shadow

	if !s.Observed() {
		t.Fatal("the shadow report did not survive the round trip; it decoded as a session " +
			"in which no experiment ran")
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"inferences", s.Inferences, want.Inferences},
		{"opportunities", s.Opportunities, want.Opportunities},
		{"detections", s.Detections, 41},
		{"nameable", s.Nameable, 23},
		{"unknown", s.Unknown, 3},
		{"role button", s.Roles["button"], 17},
		{"role bar", s.Roles["bar"], 5},
		{"agreed", s.Comparison.Agreed, 2},
		{"shadow-only", s.Comparison.ShadowOnly, 37},
		{"authoritative-only", s.Comparison.AuthoritativeOnly, 1},
		{"role-disagreement", s.Comparison.RoleDisagreement, 4},
		{"geometry-disagreement", s.Comparison.GeometryDisagreement, 6},
		{"uncomparable", s.Comparison.Uncomparable, 8},
		{"shadow-only-nameable", s.Comparison.ShadowOnlyNameable, 19},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d after round trip, want %d", c.name, c.got, c.want)
		}
	}
	if s.MedianMS != 877 || s.MaxMS != 877 {
		t.Errorf("latency median %d max %d, want 877/877", s.MedianMS, s.MaxMS)
	}
	if len(s.Tracks) != 1 || s.Tracks[0].ID != "shadow_7" || s.Tracks[0].Seen != 61 ||
		s.Tracks[0].Episodes != 3 || s.Tracks[0].Shape != observe.ShapeBursty {
		t.Errorf("track summaries did not survive the round trip: %+v", s.Tracks)
	}
	if s.Detector != "screenparser" {
		t.Errorf("detector = %q", s.Detector)
	}

	// → CLI re-encode, so a `--json` consumer sees the same thing.
	again, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("re-encoding: %v", err)
	}
	if string(again) != string(wire) {
		t.Errorf("the report is not stable across a second encode:\n first %s\nsecond %s",
			wire, again)
	}
}

// The text renderer must show the report, and must say SHADOW and NONE.
func TestTheSessionRendererShowsTheShadowReport(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, distinctiveShadow())
	out := b.String()

	for _, want := range []string{
		"SCREENPARSER · SHADOW", "no authority",
		"shadow-only 37", "19 shadow-only NAMEABLE",
		"button 17", "median 877ms", "authority    NONE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendered report is missing %q:\n%s", want, out)
		}
	}
}

// No experiment configured must render NOTHING, not a row of zeroes.
//
// A zero row would say "ScreenParser ran and saw nothing" — a far more alarming claim than
// "you did not enable it", and the two must never be confused.
func TestNoShadowRendersNothingRatherThanZeroes(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, observe.ShadowTotals{})
	if b.Len() != 0 {
		t.Fatalf("a session with no experiment rendered:\n%s", b.String())
	}
}

// Requested but unavailable says so, and says nothing about detections.
func TestAnUnavailableShadowSaysWhy(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, observe.ShadowTotals{
		Detector: "screenparser", Unavailable: "$MARCO_ONNXRUNTIME is not set",
	})
	out := b.String()
	if !strings.Contains(out, "$MARCO_ONNXRUNTIME is not set") {
		t.Errorf("the reason is missing:\n%s", out)
	}
	if strings.Contains(out, "detections") {
		t.Errorf("an unavailable experiment reported detections:\n%s", out)
	}
}

// The SESSION renderer must show the report — not merely renderShadow when called directly.
//
// The distinction is the whole point, and it caught a real bug: renderShadow was written,
// unit-tested by calling it, and never invoked from renderObservationSession. A live session
// produced a complete shadow report in JSON and printed nothing in text, and the renderer's
// own test was green throughout. This test enters through the production renderer.
func TestTheSessionRendererIsActuallyCalled(t *testing.T) {
	view := observationView{
		ID: "observe_9", State: "completed", Samples: 22, Complete: true,
		Stats: observesession.Stats{Shadow: distinctiveShadow()},
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	out := renderObservationSession(raw)
	if !strings.Contains(out, "SHADOW") {
		t.Fatalf("the session report does not mention the experiment at all. The renderer "+
			"exists and production does not call it:\n%s", out)
	}
	for _, want := range []string{"shadow-only 37", "19 shadow-only NAMEABLE", "NONE",
		"shadow_7", "bursty", "tracks       1 discovered"} {
		if !strings.Contains(out, want) {
			t.Errorf("the session report is missing %q:\n%s", want, out)
		}
	}
}
