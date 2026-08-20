package visionbench_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Temporal truth must not be able to contradict the frames it describes.
//
// The defect: `pause-close` declared six identities gone across four frames, two of which show
// the pause menu at full opacity. Two statements about the same pixels, and nothing compared
// them — so the corpus asserted "no interface" over a menu, and the detector was charged with
// false positives for reading the screen correctly.

// frames builds a sequence whose annotations follow a presence pattern.
func annotatedFrames(sequence, pattern string) []visionbench.FrameTruth {
	var out []visionbench.FrameTruth
	for i := range pattern {
		ft := visionbench.FrameTruth{
			Schema: 1, Frame: string(rune('a' + i)), Sequence: sequence, Index: i,
			InterfacePresent: true,
		}
		if pattern[i] == 'X' {
			ft.Regions = []visionbench.TruthRegion{{
				Kind:     visionbench.TruthButton,
				Bounds:   visionbench.NormRect{X: 0.4, Y: 0.4, W: 0.2, H: 0.1},
				Identity: "subject",
			}}
		}
		out = append(out, ft)
	}
	return out
}

func declare(sequence string, spans ...visionbench.Span) visionbench.SequenceTruth {
	return visionbench.SequenceTruth{
		Schema: visionbench.SequenceSchema, Sequence: sequence,
		Tracks: []visionbench.TrackTruth{{Identity: "subject", Present: spans}},
	}
}

func problems(errs []error) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// The exact pause-close defect: truth claims absence where the frame annotates presence.
func TestDeclaredAbsenceCannotContradictAnAnnotatedFrame(t *testing.T) {
	seq := annotatedFrames("s", "XX..XX")                 // the menu really is back at 4 and 5
	bad := declare("s", visionbench.Span{From: 0, To: 1}) // ...but truth says it left for good

	errs := bad.CheckAnnotations(seq)
	if len(errs) == 0 {
		t.Fatal("a declaration asserting absence over two annotated frames was accepted; " +
			"this is the pause-close defect exactly")
	}
	got := problems(errs)
	for _, want := range []string{"declared ABSENT at frame 4", "declared ABSENT at frame 5"} {
		if !strings.Contains(got, want) {
			t.Errorf("no complaint mentioning %q; got:\n%s", want, got)
		}
	}
}

// And the other direction: truth claims presence the frame does not show.
func TestDeclaredPresenceCannotExceedTheAnnotations(t *testing.T) {
	seq := annotatedFrames("s", "XX..")
	bad := declare("s", visionbench.Span{From: 0, To: 3})

	errs := bad.CheckAnnotations(seq)
	if len(errs) == 0 {
		t.Fatal("a declaration claiming presence on two unannotated frames was accepted")
	}
	if got := problems(errs); !strings.Contains(got, "declared present at frame 2") {
		t.Errorf("no complaint about frame 2; got:\n%s", got)
	}
}

// The agreeing case must pass, including a recurrence.
func TestAgreeingDeclarationIsAccepted(t *testing.T) {
	seq := annotatedFrames("s", "XX..XX")
	good := declare("s",
		visionbench.Span{From: 0, To: 1}, visionbench.Span{From: 4, To: 5})
	if errs := good.CheckAnnotations(seq); len(errs) != 0 {
		t.Fatalf("a declaration matching the frames was rejected:\n%s", problems(errs))
	}
	if errs := good.Validate(6); len(errs) != 0 {
		t.Fatalf("a valid recurrence was rejected:\n%s", problems(errs))
	}
}

// An identity that comes and goes but declares no intervals is refused. Without them the
// scorer cannot tell "not there" from "nobody annotated it", so it would silently skip the
// timing measurement the sequence exists to provide.
func TestARecurringIdentityMustDeclareItsIntervals(t *testing.T) {
	seq := annotatedFrames("s", "XX..XX")
	silent := visionbench.SequenceTruth{Schema: visionbench.SequenceSchema, Sequence: "s"}
	errs := silent.CheckAnnotations(seq)
	if len(errs) == 0 {
		t.Fatal("an identity present in 4 of 6 frames was accepted with no declaration")
	}
	if got := problems(errs); !strings.Contains(got, "declares no presence intervals") {
		t.Errorf("unexpected complaint:\n%s", got)
	}
	// A truly static identity needs no declaration — it cannot be mistimed.
	if errs := silent.CheckAnnotations(annotatedFrames("s", "XXXX")); len(errs) != 0 {
		t.Errorf("a static identity was required to declare intervals:\n%s", problems(errs))
	}
}

func TestSpansMustBeOrderedDisjointAndInRange(t *testing.T) {
	cases := []struct {
		name  string
		spans []visionbench.Span
		want  string
	}{
		{"backwards", []visionbench.Span{{From: 3, To: 1}}, "runs backwards"},
		{"out of range", []visionbench.Span{{From: 0, To: 9}}, "outside the sequence"},
		{"overlapping", []visionbench.Span{{From: 0, To: 3}, {From: 2, To: 4}}, "overlaps"},
		{"unordered", []visionbench.Span{{From: 3, To: 4}, {From: 0, To: 1}}, "overlaps or precedes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := declare("s", c.spans...)
			errs := s.Validate(6)
			if len(errs) == 0 {
				t.Fatalf("%v was accepted", c.spans)
			}
			if got := problems(errs); !strings.Contains(got, c.want) {
				t.Errorf("want a complaint containing %q, got:\n%s", c.want, got)
			}
		})
	}
}

// Shape is reporting only, but a wrong label in a report is still a wrong statement.
func TestShapeNamesThePattern(t *testing.T) {
	cases := []struct {
		pattern string
		want    string
	}{
		{"XXXX", "static"},
		{"..XX", "appearance"},
		{"XX..", "disappearance"},
		{".XX.", "transient"},
		{"XX..XX", "recurring"},
	}
	for _, c := range cases {
		track := visionbench.TrackTruth{Identity: "subject", Present: spansOf(c.pattern)}
		if got := track.Shape(len(c.pattern)); got != c.want {
			t.Errorf("Shape(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}
