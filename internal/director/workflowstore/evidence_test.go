package workflowstore_test

// THE RAW DEMONSTRATION SURVIVES THE INTERPRETATION OF IT.
//
// Four properties, and each one is a thing that was lost before this record existed:
//
//	a press that WORKED kept its coordinate              it kept only a look
//	a hold was distinguishable from a tap                both were one counter
//	the record could be told from a later session's       nothing linked the two files
//	the executable artifact still carried no coordinate   and must not start carrying one

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aDemonstration is one session's raw record: two presses and three keystrokes.
//
// The position is Ark's, measured. [[Experiment-085-the-graphics-tab-is-a-button]] found the
// press that landed on the GRAPHICS tab at 0.414,0.261, inside a 7.2%x2.8% boundary.
func aDemonstration(name string, at time.Time) workflowstore.Evidence {
	return workflowstore.Evidence{
		Name: name, SavedAt: at,
		Window: workflowstore.WindowWatched{Application: "arkascended", Generations: 1},
		Looks:  300,
		Moments: []workflowstore.Moment{
			{Look: 20, Screen: "screen_a", AtMS: 2000, Inputs: []observe.InputEvent{
				// A HOLD: pressed, leaned on, released 420ms later.
				{Intent: observe.NavBack, AtMS: 2000, Device: observe.DeviceKeyboard},
				{Intent: observe.NavBack, AtMS: 2100, Edge: observe.EdgeRepeat},
				{Intent: observe.NavBack, AtMS: 2420, Edge: observe.EdgeRelease},
			}},
			{Look: 24, Screen: "screen_a", AtMS: 2400, Inputs: []observe.InputEvent{
				// A TAP, and then a press of the mouse.
				{Intent: observe.NavConfirm, AtMS: 2440},
				{Intent: observe.NavConfirm, AtMS: 2495, Edge: observe.EdgeRelease},
				{Intent: observe.NavPoint, AtMS: 2500,
					Where:  observe.PointerAt{X: 0.414, Y: 0.261},
					Device: observe.DeviceMouse},
			}},
		},
		Presses: []workflowstore.DemonstratedPress{
			// THE PRESS THAT WORKED, with its coordinate. The one the old record threw
			// away, because it was the one Marco understood.
			{Look: 24, At: observe.PointerAt{X: 0.414, Y: 0.261}, Screen: "screen_a",
				Control: "control_1", Containing: 1},
			// AND THE PRESS NOBODY COULD SEGMENT, kept the same way.
			{Look: 90, At: observe.PointerAt{X: 0.880, Y: 0.700}, Screen: "screen_a"},
		},
		Counts: observe.InputStats{Received: 9, Classified: 3,
			Ignored: map[observe.IgnoreReason]int{
				observe.IgnoreKeyRelease: 2, observe.IgnoreRepeat: 1,
			}},
	}
}

// A PRESS THAT WORKED KEEPS ITS COORDINATE, which is the press that used to lose it.
func TestTheRawRecordKeepsTheCoordinateOfThePressThatWorked(t *testing.T) {
	e := aDemonstration("ark graphics", time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	body, err := workflowstore.MarshalEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.UnmarshalEvidence(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Presses) != 2 {
		t.Fatalf("%d press(es) came back of 2", len(back.Presses))
	}
	// THE ONE WITH A TARGET, which is the case that proves the point: an attributed
	// press kept only its look before this, so the coordinate of the one click in a
	// session that actually worked was the one coordinate the session discarded.
	worked := back.Presses[0]
	if worked.Control != "control_1" {
		t.Fatalf("the first press resolved to %q", worked.Control)
	}
	if worked.At.X != 0.414 || worked.At.Y != 0.261 {
		t.Errorf("it was pressed at %v and the record says %v",
			observe.PointerAt{X: 0.414, Y: 0.261}, worked.At)
	}
	if worked.Screen != "screen_a" || worked.Look != 24 {
		t.Errorf("the press is on %q at look %d, which is not where it was made",
			worked.Screen, worked.Look)
	}
	// AND THE PRESS NOBODY SEGMENTED, which must be the same kind of fact and not a
	// lesser one: distinguished by having no candidate, never by being filed elsewhere.
	if blind := back.Presses[1]; blind.Control != "" || blind.Containing != 0 {
		t.Errorf("the unsegmented press came back with a control: %+v", blind)
	}
	if back.Presses[1].At.X != 0.880 {
		t.Errorf("the unsegmented press lost its position: %+v", back.Presses[1])
	}
}

// AND THE RECORD SPEAKS THE ROUTINE'S LANGUAGE, not the session's.
//
// `visual_1` and `target_2` are session-local and mean nothing in a file — and a durable file
// carrying one invites a later reader to match on it across sessions, which is the failure
// `TestAVisualScreenCannotReachDurableMemory` refuses. So the record carries `screen_a` and
// `control_1`, which point at real entries in the routine beside it. Asserted over the bytes,
// because the point is what lands on disk.
func TestTheRawRecordNamesScreensTheWayTheRoutineDoes(t *testing.T) {
	body, err := workflowstore.MarshalEvidence(
		aDemonstration("ark graphics", time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"screen_a"`) || !strings.Contains(s, `"control_1"`) {
		t.Errorf("the record does not use the routine's own labels:\n%s", s)
	}
	for _, forbidden := range []string{"visual_", "target_"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the durable record carries the session-local identity %q:\n%s",
				forbidden, s)
		}
	}
}

// A HOLD IS DISTINGUISHABLE FROM A TAP, which is what the counters could never say.
func TestAHoldAndATapAreDifferentInTheRecord(t *testing.T) {
	e := aDemonstration("ark graphics", time.Now().UTC())
	ks := e.Keystrokes()
	if len(ks) != 2 {
		t.Fatalf("%d keystroke(s) of 2: %+v", len(ks), ks)
	}
	hold, tap := ks[0], ks[1]
	if hold.Intent != observe.NavBack || hold.HeldMS != 420 || hold.Repeats != 1 {
		t.Errorf("the hold reads %q held %dms with %d repeat(s), want back/420/1",
			hold.Intent, hold.HeldMS, hold.Repeats)
	}
	if tap.Intent != observe.NavConfirm || tap.HeldMS != 55 || tap.Repeats != 0 {
		t.Errorf("the tap reads %q held %dms with %d repeat(s), want confirm/55/0",
			tap.Intent, tap.HeldMS, tap.Repeats)
	}
	// AND NEITHER IS A CHORD: the hold was released before the tap began.
	for i, k := range ks {
		if len(k.Overlaps) != 0 {
			t.Errorf("keystroke %d overlaps %v, and the two presses did not overlap",
				i, k.Overlaps)
		}
	}
	// AND A POINTER PRESS IS NOT A KEYSTROKE. It is in Presses, with a position.
	for _, k := range ks {
		if k.Intent == observe.NavPoint {
			t.Error("a pointer press came back as a keystroke")
		}
	}
}

// TWO PRESSES HELD TOGETHER OVERLAP, and the record says so without naming a key.
//
// The observation and not the conclusion. Two intents held at the same time are what a chord
// looks like and are also what a slow hand looks like, so this asserts the overlap is VISIBLE
// — the ambiguity is preserved rather than resolved by a default.
func TestTwoKeysHeldTogetherAreVisibleAsAnOverlap(t *testing.T) {
	e := workflowstore.Evidence{
		Name: "chord", SavedAt: time.Now().UTC(),
		Moments: []workflowstore.Moment{{Look: 1, Inputs: []observe.InputEvent{
			{Intent: observe.NavDown, AtMS: 0},
			{Intent: observe.NavConfirm, AtMS: 15},
			{Intent: observe.NavConfirm, AtMS: 60, Edge: observe.EdgeRelease},
			{Intent: observe.NavDown, AtMS: 75, Edge: observe.EdgeRelease},
		}}},
	}
	ks := e.Keystrokes()
	if len(ks) != 2 {
		t.Fatalf("%d keystroke(s) of 2", len(ks))
	}
	if len(ks[0].Overlaps) != 1 || ks[0].Overlaps[0] != 1 {
		t.Errorf("the first press overlaps %v, want [1]", ks[0].Overlaps)
	}
	if len(ks[1].Overlaps) != 1 || ks[1].Overlaps[0] != 0 {
		t.Errorf("the second press overlaps %v, want [0]", ks[1].Overlaps)
	}
	// AND A KEY NEVER RELEASED IS NOT A ZERO-LENGTH PRESS. It is a key held past the end
	// of the session, which is a real outcome.
	e.Moments[0].Inputs = e.Moments[0].Inputs[:1]
	if k := e.Keystrokes(); len(k) != 1 || k[0].HeldMS != -1 {
		t.Errorf("an unreleased press reads %+v, want HeldMS -1", k)
	}
}

// THE RECORD BELONGS TO ONE SAVE OF ONE ROUTINE, and says so.
//
// Without this a routine re-demonstrated would leave the earlier session's hands on disk under
// the same name, and reading those as evidence of the new one is the single way a companion
// record could lie about what somebody did.
func TestEvidenceOfAnEarlierSessionDoesNotDescribeALaterOne(t *testing.T) {
	first := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	e := aDemonstration("ark graphics", first)

	if !e.Describes(workflowstore.Artifact{Name: "ark graphics", SavedAt: first}) {
		t.Error("the record does not describe the artifact it was written beside")
	}
	later := workflowstore.Artifact{Name: "ark graphics", SavedAt: first.Add(time.Hour)}
	if e.Describes(later) {
		t.Error("a record of an earlier session describes a later save of the same name")
	}
	other := workflowstore.Artifact{Name: "ark audio", SavedAt: first}
	if e.Describes(other) {
		t.Error("a record describes a routine of a different name")
	}
}

// AND A VERSION THIS BUILD CANNOT VOUCH FOR IS REFUSED RATHER THAN HALF-READ.
func TestAnEvidenceVersionThisBuildDoesNotKnowIsRefused(t *testing.T) {
	if _, err := workflowstore.UnmarshalEvidence([]byte(`{"version":99,"name":"x"}`)); err == nil {
		t.Error("a future version was read as though this build understood it")
	}
	if _, err := workflowstore.MarshalEvidence(workflowstore.Evidence{}); err == nil {
		t.Error("a record belonging to no routine was written")
	}
}

// AND THE RECORD CARRIES NO TEXT ANYBODY TYPED, which is the privacy boundary holding.
//
// A keyboard observer is the highest-privacy surface in the system, and this is the first thing
// that writes its output to disk DURABLY. The events are `observe.InputEvent`, whose own tests
// assert it cannot hold a character; this asserts the file it lands in does not either.
func TestTheRawRecordCannotCarryWhatSomebodyTyped(t *testing.T) {
	e := aDemonstration("ark graphics", time.Now().UTC())
	body, err := workflowstore.MarshalEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	// THE INTENTS ARE THERE, so this is not passing by writing nothing.
	if !strings.Contains(s, `"back"`) || !strings.Contains(s, `"release"`) {
		t.Fatalf("the record does not carry the intents it was given:\n%s", s)
	}
	for _, forbidden := range []string{"code", "scan", "\"key\"", "char", "rune", "vk"} {
		if strings.Contains(strings.ToLower(s), forbidden) {
			t.Errorf("the durable raw record contains %q:\n%s", forbidden, s)
		}
	}
}
