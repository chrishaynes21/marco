package main

// STUDIO SAYS WHAT IT RECORDS, AND THE SHELL THAT LAUNCHED IT DOES NOT GET A VOTE.
//
// This window records plain coordinates and the real pauses — that is what the path IS, and the
// whole reason it works on a menu drawn over a live 3D world where recognition has no answer. It
// used to be left to `$MARCO_CV` / `$MARCO_ANCHORS`, read once when Studio started, so a window
// launched from a shell with anchors on captured a picture of any click whose crop happened to be
// distinctive. A press found by matching a picture cannot be stopped at, so `Unpropagatable`
// refused the whole recording at Save: the same demonstration saved or was refused depending on a
// variable nobody in the window could see and the pixels under the pointer.
//
// Every test here sets those variables ON. A test that left them unset would pass against a
// window that had gone back to inheriting them, because the default is the answer this states.
//
// See [[ADR-254-what-a-recording-captures-is-stated]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aRecorderThatRemembersWhatItWasTold is `heldRecorder` that can also be told a mode.
//
// `atStart` and not `told`, because the hooks read the mode when the recording BEGINS: a window
// that stated one afterwards would have stated it after the first press could already have been
// captured, and a stand-in that remembered the last thing it heard would call that correct.
type aRecorderThatRemembersWhatItWasTold struct {
	heldRecorder
	told    recorder.Mode
	atStart recorder.Mode
	asked   bool
}

func (r *aRecorderThatRemembersWhatItWasTold) Capture(m recorder.Mode) {
	r.told, r.asked = m, true
}

func (r *aRecorderThatRemembersWhatItWasTold) Start() error {
	r.atStart = r.told
	return r.heldRecorder.Start()
}

// aRecordingWithAnchorsInherited starts one recording in a window launched with the CV stack on.
func aRecordingWithAnchorsInherited(t *testing.T, name string) (*Acts,
	*aRecorderThatRemembersWhatItWasTold) {

	t.Helper()
	// THE SHELL THAT LAUNCHED THIS WINDOW ASKED FOR ANCHORS. The inherited case.
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	if ownerApp == "" {
		t.Fatal("Studio cannot name its own application")
	}
	rec := &aRecorderThatRemembersWhatItWasTold{
		heldRecorder: heldRecorder{events: anUnshieldedGlanceAtStudio(owner, ownerApp)},
	}
	a.recording = func() recorder.Recorder { return rec }
	if why := a.StartRecording(name); why != "" {
		t.Fatalf("StartRecording refused: %s", why)
	}
	return a, rec
}

// STUDIO TELLS THE RECORDER, BEFORE IT STARTS, AND SAYS SO ON THE PAGE.
//
// The wiring claim and the visible one together, because either alone is the defect wearing the
// other's clothes: a window that told the recorder and said nothing leaves somebody unable to
// check it, and a window that said it and told nobody is a sentence over a recording that did
// something else.
func TestStudioStatesCoordinateModeRatherThanInheritingIt(t *testing.T) {
	a, rec := aRecordingWithAnchorsInherited(t, "ark graphics")
	defer a.CancelRecording()

	if !rec.asked {
		t.Fatal("Studio never told the recorder what to capture, so what this " +
			"recording captures is still whatever shell launched the window")
	}
	if rec.atStart != recorder.Coordinates {
		t.Errorf("the recording began under %v, not recorder.Coordinates", rec.atStart)
	}

	r := a.Recording()
	if r.Mode != string(orchestrator.Coordinates) {
		t.Errorf("the page is told the mode is %q", r.Mode)
	}
	if !strings.Contains(r.Captures, "picture") {
		t.Errorf("the sentence beside the recording does not say that nothing is "+
			"pictured: %q", r.Captures)
	}
}

// AND THE PAGE IS TOLD BEFORE ANYBODY PRESSES RECORD.
//
// Which is where it matters: the thing being described is a choice somebody is about to make. A
// window that only said it once a recording was in flight would be telling them what they had
// already done.
func TestTheModeIsStatedBeforeAnythingIsRecorded(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())

	r := a.Recording()
	if r.Active || r.Review {
		t.Fatal("nothing has been recorded and the window thinks something has")
	}
	if r.Mode != string(orchestrator.Coordinates) {
		t.Errorf("with nothing recorded, the line beside Record says %q", r.Mode)
	}
	if r.Captures == "" {
		t.Error("there is no sentence beside Record, so what a recording will " +
			"capture is unsaid until it is too late to choose")
	}
}

// AND A SAVED ROUTINE CARRIES THE WORD, WHERE A PERSON AND A RUN RECORD BOTH FIND IT.
func TestASavedRoutineSaysHowItWasRecorded(t *testing.T) {
	a, _ := aRecordingWithAnchorsInherited(t, "ark graphics")
	if why := a.StopRecording(); why != "" {
		t.Fatalf("StopRecording: %s", why)
	}
	if why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), ""); why != "" {
		t.Fatalf("SaveDraft: %s", why)
	}
	var found *Routine
	for i, r := range a.Routines() {
		if r.Name == "ark graphics" {
			found = &a.Routines()[i]
			break
		}
	}
	if found == nil {
		t.Fatal("the routine that was just saved is not in the library")
	}
	if found.Mode != string(orchestrator.Coordinates) {
		t.Errorf("the saved routine's card says it was recorded as %q", found.Mode)
	}
	if found.Captures == "" {
		t.Error("the card has no sentence for what was captured")
	}
	if orchestrator.ModeOf(found.Source) != orchestrator.Coordinates {
		t.Error("the saved Marco itself does not state the mode, so the card is " +
			"the only record of it and a run record would say `unstated`")
	}
}

// AND A ROUTINE THAT STATES NOTHING IS SHOWN AS STATING NOTHING.
//
// Every routine in every library predates this, and so does one made by the overlay or by `marco
// learn` — and what those captured depended on the environment of a process that has since
// exited. A card that filled the blank with the likeliest answer would be this window telling
// somebody how their routine was made on the strength of an average.
func TestARoutineThatSaysNothingAboutItsModeIsShownAsSayingNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())

	// A ROUTINE AS EVERY OLDER ONE IS: legal, runnable, and silent about its recording.
	app := filepath.Join(dir, "arkascended", "focus")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "// Auto-generated by `marco learn \"old thing\"`. Edit freely.\n" +
		"use os.\n\nthe OldThing is an actor.\n\nthis can Run.\n" +
		"this's Run does...\n  do OS's Key with \"escape\".\n...\n\n" +
		"the Main is a script.\nthis's Start does...\n  do OldThing's Run.\n...\n"
	if err := os.WriteFile(filepath.Join(app, "old-thing.marco"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, r := range a.Routines() {
		if r.Name != "old thing" {
			continue
		}
		if r.Mode != "" || r.Captures != "" {
			t.Errorf("a routine that states no mode is shown as %q / %q",
				r.Mode, r.Captures)
		}
		return
	}
	t.Fatal("the older routine is not listed at all, so this proves nothing")
}
