package recorder

import (
	"strings"
	"testing"
	"time"
)

// WHAT CROSSES THE EDGE OF THE CHOSEN WINDOW, AND WHAT MERELY HAPPENS OUTSIDE IT.
//
// These drive `Boundary` itself — the production decision, not a restatement of it beside the
// code. `observe` in the Windows backend is the plumbing that asks it, and
// `confine_windows_test.go` holds the buffer and the channel; this file holds the rules.
//
// The distinction every case here turns on is between input that is simply NOT PART of the
// demonstration — dropped, and that is the whole of it — and input that makes what WAS recorded
// a lie about what was done. The second is a refusal, because the half-gesture left behind says
// something the person did not do.

func at(ms int) time.Time {
	return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC).Add(
		time.Duration(ms) * time.Millisecond)
}

// arrived puts a boundary in the window, which is where every demonstration starts: the person
// pressed Record, went to the application, and the first event there is admitted.
func arrived(b *Boundary, ms int) {
	if k, ok := b.Arrived(true, at(ms)); !ok || k != EvEntered {
		panic("a boundary that was outside did not report entering")
	}
}

// ── THE ORDINARY CASES: OFF-TARGET INPUT IS DROPPED AND NOTHING IS REFUSED ───

func TestInputOutsideTheWindowIsDroppedAndTheRecordingStillSaves(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	if got := b.Pressed("a", true, true, false); got != Admit {
		t.Fatalf("a key in the window was %v, not admitted", got)
	}
	if got := b.Pressed("a", false, true, false); got != Admit {
		t.Fatalf("releasing it in the window was %v", got)
	}
	// AWAY, AND A WHOLE GESTURE PERFORMED SOMEWHERE ELSE.
	b.Arrived(false, at(100))
	if got := b.Pressed("h", true, false, false); got != Drop {
		t.Errorf("a key pressed outside the window was %v, not dropped", got)
	}
	if got := b.Pressed("h", false, false, false); got != Drop {
		t.Errorf("releasing it outside the window was %v, not dropped", got)
	}
	if got := b.Moved(false); got != Drop {
		t.Errorf("a pointer move outside the window was %v, not dropped", got)
	}
	b.Arrived(true, at(400))
	if got := b.Pressed("b", true, true, false); got != Admit {
		t.Errorf("a key after coming back was %v, not admitted", got)
	}
	// AND NOTHING CROSSED: a complete gesture performed elsewhere is not this
	// recording's business, and the recording is still saveable.
	if why := b.Crossing(); why != "" {
		t.Errorf("a whole gesture performed outside the window refused the "+
			"recording: %q", why)
	}
	// THE ABSENCE IS COUNTED, because review says so to the person.
	if away, n := b.Away(); n != 1 || away != 300*time.Millisecond {
		t.Errorf("the absence was recorded as %v over %d times, not 300ms over 1",
			away, n)
	}
}

// AND THE STOP GESTURE WORKS FROM ANYWHERE, which is the one thing an off-target key may say.
func TestTheStopKeyIsPublishedFromOutsideTheWindowAndNothingElseIs(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(50))
	if got := b.Pressed("f12", true, false, true); got != StopOnly {
		t.Errorf("the stop key pressed outside the window was %v, so a person who "+
			"walked away cannot stop the recording", got)
	}
	// THE CONTROL BESIDE IT, without which "everything off-target is published" would
	// pass the assertion above.
	if got := b.Pressed("p", true, false, false); got != Drop {
		t.Errorf("an ordinary key outside the window was %v, so somebody's typing "+
			"reaches a consumer of the live channel", got)
	}
}

// ── CLICKING BACK INTO THE WINDOW IS HOW YOU GET IN, NOT A CROSSING ──────────

func TestClickingBackIntoTheWindowIsDroppedAndNotRefused(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	// THE PRESS LANDS WHILE THE WINDOW IS STILL BEHIND, which is what activating it
	// means, so it is not admitted.
	if got := b.Pressed("the left button", true, false, false); got != Drop {
		t.Fatalf("the press of an activating click was %v, not dropped", got)
	}
	// AND BY THE RELEASE THE WINDOW IS IN FRONT. The release on its own is a release of
	// a button nothing pressed, so it goes too — and the recording is still saveable,
	// because clicking on a window to bring it forward is not part of the demonstration.
	b.Arrived(true, at(140))
	if got := b.Pressed("the left button", false, true, false); got != Drop {
		t.Errorf("the release of an activating click was %v: a recording that keeps "+
			"it has a release with no press in it", got)
	}
	if why := b.Crossing(); why != "" {
		t.Errorf("clicking back into the window refused the recording: %q", why)
	}
	// AND THE WINDOW IS LIVE AGAIN: the exception is spent, not a state.
	if got := b.Pressed("a", true, true, false); got != Admit {
		t.Errorf("a key after the activating click was %v", got)
	}
	if why := b.Crossing(); why != "" {
		t.Errorf("a key after an activating click refused the recording: %q", why)
	}
}

// ── BUT IT EXCUSES NOTHING THAT IS STILL HELD ───────────────────────────────

// ENTERING WITH A MODIFIER ALREADY DOWN. Tested SEPARATELY from the ordinary click above,
// because the two differ only in whether the outside press was over by the time the person
// acted — and the first version of this rule got that wrong in the direction that silently
// writes down the wrong demonstration.
func TestEnteringTheWindowWithCtrlHeldRefusesTheRecording(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	// CTRL GOES DOWN OUTSIDE and stays down.
	if got := b.Pressed("ctrl", true, false, false); got != Drop {
		t.Fatalf("ctrl pressed outside the window was %v", got)
	}
	b.Arrived(true, at(200))
	// A CLICK IN THE WINDOW WHILE IT IS HELD. Admitted — it happened here, and it is
	// what the person did — but a recording holding it alone says `click` where the
	// person performed `ctrl+click`.
	if got := b.Pressed("the left button", true, true, false); got != Admit {
		t.Fatalf("a click in the window was %v", got)
	}
	why := b.Crossing()
	if why == "" {
		t.Fatal("a ctrl+click whose ctrl was pressed outside the window saved as an " +
			"ordinary click, which is a different demonstration")
	}
	// AND IT SAYS WHAT CROSSED AND WHAT IT DID, because the person is the only one who
	// can perform it again differently.
	for _, want := range []string{"the left button", "ctrl", "outside"} {
		if !strings.Contains(why, want) {
			t.Errorf("the reason does not mention %q: %q", want, why)
		}
	}
}

// AND A KEY, NOT ONLY A MODIFIER. Q held across the edge is a held key whose press is not in the
// recording, whatever it means in the application.
func TestAnOrdinaryKeyHeldFromOutsideRefusesTheRecording(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	b.Pressed("q", true, false, false)
	b.Arrived(true, at(200))
	if got := b.Pressed("e", true, true, false); got != Admit {
		t.Fatalf("a key in the window was %v", got)
	}
	if why := b.Crossing(); why == "" {
		t.Error("a key pressed in the window while q was held from outside it saved " +
			"as though nothing was held, and q is 'use' in half the games there are")
	}
}

// LEAVING WITH SOMETHING STILL HELD IS REPORTED AND NOT JUDGED HERE.
//
// The same fact as before — the recording has the press and whatever happened to the release
// happened somewhere else — and no longer a verdict, because at this instant there is not enough
// to reach one with. The key is either a gesture the demonstration depends on or the Alt-Tab
// somebody used to reach the Stop button, and what separates them is what happens NEXT.
// `orchestrator.exitGesture` is where that is decided; this is where the fact is reported.
//
// Deleting the report must fail the orchestrator's own cases both ways round: a stranded key
// nothing followed saves, and one the demonstration carried on past refuses.
func TestLeavingTheWindowWithAKeyHeldIsReportedForJudgingLater(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	if got := b.Pressed("shift", true, true, false); got != Admit {
		t.Fatalf("shift in the window was %v", got)
	}
	if _, ok := b.Arrived(false, at(100)); !ok {
		t.Fatal("leaving the window was not reported")
	}
	if why := b.Crossing(); why != "" {
		t.Errorf("the boundary refused the recording at the instant somebody left it, "+
			"which is before it can know what they were doing: %q", why)
	}
	if held := b.HeldAcross(); len(held) != 1 || held[0] != "shift" {
		t.Fatalf("the keys stranded by the leaving are %q, not shift", held)
	}
	// DRAINED, so the doors that read it do not write the same marker on every event
	// afterwards — which is the rule the crossing marker already had.
	if held := b.HeldAcross(); len(held) != 0 {
		t.Errorf("the same stranded key came back a second time: %q", held)
	}
	if got := b.Pressed("shift", false, false, false); got != Drop {
		t.Errorf("the release outside the window was %v", got)
	}
	// AND THE RELEASE REPORTS IT TOO, from the other door: whichever of the two the
	// recorder notices first, the fact reaches the buffer.
	if held := b.HeldAcross(); len(held) != 1 || held[0] != "shift" {
		t.Errorf("the release outside the window reported %q", held)
	}
}

// EVERY KEY THE LEAVING STRANDED, not the first one a map handed back.
//
// Alt-Tab strands both of them, and a recording that reported one would have the other left in
// it — a press with no release, in a demonstration that was allowed to be saved.
func TestLeavingReportsEveryKeyItStranded(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Pressed("alt", true, true, false)
	b.Pressed("tab", true, true, false)
	b.Arrived(false, at(100))
	held := b.HeldAcross()
	if len(held) != 2 || held[0] != "alt" || held[1] != "tab" {
		t.Errorf("the leaving stranded %q, not alt and tab in that order", held)
	}
}

// AND THE ACTIVATION CLICK EXCUSES ITSELF AND NOTHING ELSE.
//
// This is the bound on the exception, and it is a separate case from entering with Ctrl held
// because the two differ in a way a single test cannot ask about: here the outside press that
// gets forgiven is forgiven CORRECTLY, and the question is whether forgiving it also forgives
// what was held beside it. A version that cleared the held state on an activating release
// survived a gate of every other case in this file.
func TestTheActivatingClickDoesNotExcuseWhatIsStillHeldBesideIt(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	// TWO THINGS GO DOWN OUT THERE: ctrl, and the button that will be used to click
	// back into the window.
	b.Pressed("ctrl", true, false, false)
	b.Pressed("the left button", true, false, false)
	// THE WINDOW COMES FORWARD and the click's release lands inside it. Dropped, and
	// not a crossing — that much is TestClickingBackIntoTheWindowIsDroppedAndNotRefused.
	b.Arrived(true, at(140))
	if got := b.Pressed("the left button", false, true, false); got != Drop {
		t.Fatalf("the release of the activating click was %v", got)
	}
	if why := b.Crossing(); why != "" {
		t.Fatalf("the activating click itself refused the recording: %q", why)
	}
	// AND CTRL IS STILL DOWN. What the person does next depends on a press that is not
	// in the recording, whatever the click that brought them here was.
	b.Pressed("x", true, true, false)
	why := b.Crossing()
	if why == "" {
		t.Fatal("an action in the window saved as though nothing was held, because " +
			"the activating click cleared what was held beside it")
	}
	if !strings.Contains(why, "ctrl") {
		t.Errorf("the reason does not name what was still held: %q", why)
	}
}

// A DRAG THAT LEAVES THE WINDOW'S RECTANGLE IS NOT A CROSSING, because membership is the
// foreground and a drag holds the foreground for its whole life. This is the case the
// rectangle-based version of this rule would have refused, and it is the ordinary way to drag
// something onto a taskbar button or across two monitors.
func TestADragThatLeavesTheRectangleStaysOneGesture(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Pressed("the left button", true, true, false)
	for ms := 10; ms < 50; ms += 10 {
		if got := b.Moved(true); got != Admit {
			t.Fatalf("a move during a drag was %v", got)
		}
	}
	if got := b.Pressed("the left button", false, true, false); got != Admit {
		t.Fatalf("the end of the drag was %v", got)
	}
	if why := b.Crossing(); why != "" {
		t.Errorf("a drag inside the window refused the recording: %q", why)
	}
}

// A MOVE NEVER REFUSES A RECORDING. It is a sample of where the pointer was, not an intent — and
// a rule that let one refuse would make a recording unsaveable because somebody's mouse was
// nudged while a key was held in another window.
func TestAPointerMoveWhileSomethingIsHeldOutsideRefusesNothing(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	b.Pressed("ctrl", true, false, false)
	b.Arrived(true, at(200))
	if got := b.Moved(true); got != Admit {
		t.Fatalf("a move in the window was %v", got)
	}
	if why := b.Crossing(); why != "" {
		t.Errorf("a pointer move refused the recording: %q", why)
	}
	// AND THE RULE IS STILL ARMED for the next thing the person actually does, which is
	// what keeps this from being a hole in the one above.
	b.Pressed("the left button", true, true, false)
	if why := b.Crossing(); why == "" {
		t.Error("a click after the move did not trip the crossing, so a move " +
			"disarmed the rule instead of merely not tripping it")
	}
}

// THE FIRST REASON WINS, and the later ones do not overwrite it: the thing that crossed first is
// the one the person can do something about.
func TestTheFirstCrossingIsTheReasonGiven(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	b.Arrived(false, at(100))
	b.Pressed("shift", true, false, false) // held from outside
	b.Arrived(true, at(200))
	b.Pressed("x", true, true, false) // used in the window with shift held — the first
	first := b.Crossing()
	if first == "" || !strings.Contains(first, "shift") {
		t.Fatalf("the first crossing is %q", first)
	}
	b.Arrived(false, at(300))
	b.Pressed("ctrl", true, false, false)
	b.Arrived(true, at(400))
	b.Pressed("z", true, true, false) // another, later
	if b.Crossing() != first {
		t.Errorf("the reason changed to %q", b.Crossing())
	}
}

// A RELEASE OF SOMETHING THIS RECORDING NEVER SAW GO DOWN IS NOT WRITTEN DOWN.
//
// It used to be admitted, on the reasoning that a recording may begin with somebody resting a
// finger on a key and `simplify` refuses the cases it can name. That was the hole:
// `simplify.modifierTrouble` names MODIFIERS, so a release of anything else was recorded as a
// keystroke nobody made — and the press it belonged to had already modified an admitted
// action without appearing anywhere.
//
// What was down at Start is seeded instead (`HeldBefore`, and
// `TestSomethingHeldBeforeTheRecordingBeganRefusesTheRecording` for what that then refuses), so
// the legitimate case is covered at its source and what reaches here is a press this recording
// genuinely never saw.
func TestAReleaseOfSomethingNeverSeenGoingDownIsNotRecorded(t *testing.T) {
	b := NewBoundary()
	arrived(b, 0)
	if got := b.Pressed("ctrl", false, true, false); got != Drop {
		t.Errorf("the release of a key nothing saw pressed was %v, so the "+
			"recording holds a keystroke nobody made", got)
	}
	// AND IT IS NOT A REFUSAL EITHER: a release on its own modified nothing, and
	// refusing the recording for it would refuse one for a stray key.
	if why := b.Crossing(); why != "" {
		t.Errorf("a stray release refused the recording: %q", why)
	}
	// WHAT REFUSES IS AN ACTION TAKEN WHILE IT IS HELD, which is the seeded case and is
	// held in the Windows file beside the scan that seeds it.
	held := NewBoundary()
	held.HeldBefore([]string{"ctrl"})
	arrived(held, 0)
	held.Pressed("the left button", true, true, false)
	if why := held.Crossing(); why == "" {
		t.Error("a click made while something held before the recording was still " +
			"down saved as an ordinary click")
	}
}
