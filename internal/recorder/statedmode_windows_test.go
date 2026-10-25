//go:build windows

package recorder

import "testing"

// A STATED MODE BEATS THE ENVIRONMENT THE PROCESS WAS LAUNCHED WITH.
//
// `captureAnchors` is read ONCE, at process start, from $MARCO_CV / $MARCO_ANCHORS — which is why
// `t.Setenv` is no use here and why the variable is set directly instead. That is not the test
// cheating around the real thing: being read at process start IS the defect. A window that
// inherited anchors on carried them for the life of the window, and a stated mode such a variable
// could overrule would not be a stated mode.
//
// # And the question is put to the production decision, not restated beside it
//
// The first version of the arming test below evaluated `anchoring() && anchorArmed.Load()` in its
// own body and asserted on the answer. It passed, and it was worthless: a mutation that took
// `anchoring()` out of `recordClickDown` changed nothing it could see, because the conjunction
// under test was the one the test had typed. So these drive `recordClickDown` itself and watch
// what reaches the capture queue, with the one real-desktop read behind it substituted.
//
// See [[ADR-254-what-a-recording-captures-is-stated]].

// aRecordingOf drives one click through the production hook body and says whether the capture
// worker was asked for a picture of it.
//
// `recordClickDown` is the function the mouse hook calls. Everything it needs is package state —
// the event buffer, the capture queue, the arm — and the one thing it needs a desktop for is
// `clickAttribution`, which is already a seam for exactly this reason.
func aRecordingOf(t *testing.T, anchorsInherited, stateCoordinates, armed bool) bool {
	return aRecordedClick(t, "left", anchorsInherited, stateCoordinates, armed)
}

// aRecordedClick is that, for one named button.
//
// The button matters to the arm and only to the arm: the global switch anchors LEFT clicks (every
// button only in kitchen-sink mode), so a right-click is captured if and only if the anchor key
// armed it. That makes a right-click the one press whose picture the arm alone decides — and so
// the only press that can tell "the arm is honoured" from "the switch anchored it anyway".
func aRecordedClick(t *testing.T, button string, anchorsInherited, stateCoordinates,
	armed bool) bool {

	t.Helper()
	wasAnchors, wasAttr, wasShape := captureAnchors, clickAttribution, clickShape
	t.Cleanup(func() {
		captureAnchors, clickAttribution, clickShape = wasAnchors, wasAttr, wasShape
		plainCoordinates.Store(false)
		anchorArmed.Store(false)
		recMu.Lock()
		recBuf, recPub, captureReqs = nil, nil, nil
		recMu.Unlock()
	})
	// A DESKTOP THAT ANSWERS, so the click is built without asking Windows anything.
	clickAttribution = func() (uintptr, uint32, string) { return 0x4210, 4812, "arkascended" }
	clickShape = func(uintptr) (int, int, int, int, int, bool) {
		return 0, 0, 1920, 1080, 96, true
	}
	recMu.Lock()
	recBuf, recPub = nil, nil
	recMu.Unlock()
	captureReqs = make(chan captureReq, 4)

	captureAnchors = anchorsInherited
	plainCoordinates.Store(stateCoordinates)
	anchorArmed.Store(armed)

	recordClickDown(button, 842, 274)

	// THE CLICK IS ALWAYS RECORDED. Whatever the mode, the press itself is the thing
	// being demonstrated; only the picture beside it is in question. Asserted here so a
	// mutation that dropped the click entirely cannot read as "no picture was captured".
	recMu.Lock()
	presses := len(recBuf)
	recMu.Unlock()
	if presses != 1 {
		t.Fatalf("the click itself was not recorded (%d events), so what this "+
			"measures is not a picture being withheld", presses)
	}
	select {
	case <-captureReqs:
		return true
	default:
		return false
	}
}

// A WINDOW LAUNCHED WITH ANCHORS ON RECORDS NO PICTURE WHEN IT SAYS COORDINATES.
func TestAStatedCoordinateModeOverridesInheritedAnchors(t *testing.T) {
	if aRecordingOf(t, true, true, false) {
		t.Error("a recording told to capture coordinates captured a picture anyway, " +
			"because the environment the window was launched in won")
	}
	// AND THE OTHER DIRECTION, which is what makes the assertion above mean anything. A
	// recorder that had quietly stopped anchoring for everybody would pass the first half
	// and would have taken the CV stack away from the overlay and from `marco learn`.
	if !aRecordingOf(t, true, false, false) {
		t.Error("a recorder that stated nothing stopped honouring $MARCO_ANCHORS, so " +
			"every other caller lost the anchor capture the switch is for")
	}
	// AND NEITHER DOES ANCHORS-OFF SUDDENLY CAPTURE. The default, unchanged.
	if aRecordingOf(t, false, false, false) {
		t.Error("with anchors off and nothing stated, a click captured a picture")
	}
}

// AND A STATED MODE TAKES THE ANCHOR KEY WITH IT.
//
// The anchor key is the one path that captured a picture with anchors OFF — tap the key, and the
// next click is captured whatever `captureAnchors` says. So a gate that asked only about the
// global switch would have left exactly one way for a coordinate recording to come back carrying
// a picture, and it is a way a person reaches with one keystroke.
func TestACoordinateRecordingCannotBeArmedIntoCapturingAPicture(t *testing.T) {
	// A RIGHT-CLICK, because that is the press the arm alone decides. A left click is
	// anchored by the switch whether or not anything armed it, so asking about one would
	// have been asking a question the arm does not answer.
	if aRecordedClick(t, "right", true, true, true) {
		t.Error("an armed anchor key captured a picture in a recording that asked " +
			"for coordinates only")
	}
	// AND THE ARM STILL DOES WHAT IT DID when no mode is stated. This is the assertion
	// that caught the first version of the rule: written as the same conjunction the
	// left-click gate uses, an armed press did nothing unless the switch was also on —
	// which is the tap-then-click anchor taken away from every caller rather than from
	// this recording.
	if !aRecordedClick(t, "right", true, false, true) {
		t.Error("an armed anchor key no longer captures a non-left press, so the " +
			"tap-then-click anchor is gone for every caller")
	}
	// AND AN UNARMED RIGHT-CLICK IS STILL NOT ANCHORED, which is what makes the line
	// above about the arm rather than about right-clicks.
	if aRecordedClick(t, "right", true, false, false) {
		t.Error("a right-click nobody armed was captured, so the arm is not what " +
			"decided the press above")
	}
}

// AND STARTING A RECORDING DOES NOT INHERIT AN ARM FROM THE ONE BEFORE IT.
//
// The arm is a one-shot consumed by the next click, so an unconsumed one outlives the recording it
// was tapped in. Without this, tapping the anchor key and then stopping would put a picture in the
// first click of the NEXT recording — which, in Studio, is a recording that asked for none.
func TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording(t *testing.T) {
	t.Cleanup(func() { anchorArmed.Store(false); plainCoordinates.Store(false) })
	anchorArmed.Store(true)
	r := &winRecorder{}
	r.Capture(Coordinates)
	if err := r.Start(); err != nil {
		t.Skipf("the hooks would not install on this machine: %v", err)
	}
	armed := anchorArmed.Load()
	stated := plainCoordinates.Load()
	r.Stop()
	if armed {
		t.Error("a recording began with the anchor key still armed from before it")
	}
	if !stated {
		t.Error("Start did not publish the stated mode, so the hooks read whatever " +
			"the last recording left behind")
	}
}
