package orchestrator

// WHAT A RECORDING CAPTURES IS STATED BY ITS CALLER, AND NOT BY THE ENVIRONMENT.
//
// The defect, in one sentence: `$MARCO_CV` and `$MARCO_ANCHORS` are read once when a process
// starts, so Studio inherited whatever shell launched it for the life of the window — and with
// anchors on, a click whose crop happened to be distinctive carried a picture, which makes a
// whole guarded routine unsaveable. The same demonstration therefore saved or was refused
// depending on a variable nobody in the window could see.
//
// Four claims are held here, and the second and third are the ones that make the first worth
// anything: a stated mode reaches the recorder, a caller that states nothing is unchanged, a
// stated mode survives into the saved file, and a file that says nothing reads back as nothing.
//
// See [[ADR-254-what-a-recording-captures-is-stated]].

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aRecorderThatRemembersWhatItWasTold is a recorder that can be told a mode and says what it was.
//
// It records the mode AT START rather than at `Capture`, because that is when the hooks read it:
// a mode stated after the hooks are on would be stated after the first press could already have
// been captured, and a stand-in that remembered the last thing it was told would call that
// correct.
type aRecorderThatRemembersWhatItWasTold struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
	// told is the mode `Capture` last stated, and atStart the mode that was in force when
	// `Start` was called. Two fields, because the gap between them is the rule.
	told    recorder.Mode
	atStart recorder.Mode
	// asked is whether anybody told it anything at all, which is how "stated nothing" is
	// told apart from "stated Inherited".
	asked bool
}

func (r *aRecorderThatRemembersWhatItWasTold) Capture(m recorder.Mode) {
	r.told, r.asked = m, true
}

func (r *aRecorderThatRemembersWhatItWasTold) Start() error {
	r.atStart = r.told
	r.ch = make(chan recorder.RecordedEvent, len(r.events)+1)
	for _, e := range r.events {
		r.ch <- e
	}
	close(r.ch)
	return nil
}

func (r *aRecorderThatRemembersWhatItWasTold) Stop() []recorder.RecordedEvent { return r.events }
func (r *aRecorderThatRemembersWhatItWasTold) Events() <-chan recorder.RecordedEvent {
	return r.ch
}

// recordedWith drives one recording through `Record` under a stated mode, and hands back the
// recorder so what it was told can be read.
func recordedWith(t *testing.T, mode RecordedAs) (*aRecorderThatRemembersWhatItWasTold, *Draft) {
	t.Helper()
	rec := &aRecorderThatRemembersWhatItWasTold{events: aDemonstrationInAGame()}
	press := make(chan struct{})
	d := Deps{
		Reg: routes.Registry{Dir: t.TempDir()}, Rec: rec, Out: &strings.Builder{},
		Owner: "studio", OwnerPID: studioPID, StopWhen: press, Mode: mode,
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(press)
	}()
	dr, err := d.Record("ark graphics")
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	return rec, dr
}

// aDemonstrationInAGame is two clicks and a keystroke in one application, ended by the button.
//
// Deliberately ordinary: this file is about what a recording was TOLD, so the stream is the
// plainest one that produces a saveable guarded routine.
func aDemonstrationInAGame() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, RelTo: "ARK", RelX: 42, RelY: 74,
			WinW: 1920, WinH: 1080, Dpi: 96, Process: "arkascended.exe", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		{Kind: recorder.EvKey, KeyName: "escape", Down: true,
			App: "arkascended", PID: gamePID, T: at(900)},
		{Kind: recorder.EvKey, KeyName: "escape", Down: false,
			App: "arkascended", PID: gamePID, T: at(905)},
	}
}

// A STATED MODE REACHES THE RECORDER, BEFORE THE HOOKS GO ON.
//
// The wiring claim, and the one a mutation can delete without any other test noticing: every
// sentence about coordinate recording below this is about a draft and a file, and a `Begin` that
// never told the recorder anything would still produce both.
func TestTheRecorderIsToldTheModeBeforeItStarts(t *testing.T) {
	// WITH ANCHORS ASKED FOR IN THE ENVIRONMENT, which is the case that matters: a
	// stated mode that only held when nothing contradicted it would hold nowhere.
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	rec, dr := recordedWith(t, Coordinates)
	if !rec.asked {
		t.Fatal("nothing told the recorder what to capture, so the whole mode is a " +
			"word travelling beside a recording that ignored it")
	}
	if rec.atStart != recorder.Coordinates {
		t.Errorf("the recorder began under %v, not recorder.Coordinates", rec.atStart)
	}
	if dr.Mode != Coordinates {
		t.Errorf("the draft says it was recorded as %q", dr.Mode)
	}
}

// AND A CALLER THAT STATES NOTHING IS LEFT EXACTLY AS IT WAS.
//
// `marco learn`, the overlay and the visual recorder are started, do a thing and end, so the CV
// switches are the whole of what was asked for there — turning them on and demonstrating
// something is what they are FOR. A change that quietly took the anchor stack away from them
// would pass every assertion above.
func TestACallerThatStatesNothingLeavesItToTheEnvironment(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	rec, dr := recordedWith(t, Unstated)
	if rec.atStart != recorder.Inherited {
		t.Errorf("a recorder nobody gave a mode to began under %v, not "+
			"recorder.Inherited", rec.atStart)
	}
	if dr.Mode != Unstated {
		t.Errorf("a draft nobody stated a mode for says %q", dr.Mode)
	}
}

// THE MODE A SAVE WRITES IS THE MODE THIS READS BACK.
//
// `codegen` writes the line and `orchestrator` reads it, and the two cannot share a constant
// because `codegen` may not import this package. So the pair is held here instead — the same
// discipline `measuredTerms` is under.
func TestTheModeASaveWritesIsTheModeThisReads(t *testing.T) {
	_, dr := recordedWith(t, Coordinates)
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if got := ModeOf(src); got != Coordinates {
		t.Errorf("the saved play reads back as %q, not %q", got, Coordinates)
	}
	// AND IT SAYS SO IN WORDS A PERSON READS, not only in a word a parser finds. The
	// file is the durable account of a routine and somebody opens it.
	if !strings.Contains(src, "Recorded as: coordinates") {
		t.Error("the saved source does not say how it was recorded anywhere a " +
			"person reading the file would see it")
	}
}

// AND A ROUTINE THAT SAYS NOTHING READS BACK AS NOTHING.
//
// Every routine saved before this says nothing, and so does one saved by a caller that states
// nothing. A reader that filled the blank with the likeliest answer would be telling somebody how
// their routine was made on the strength of an average — which is the defect this whole slice is
// an instance of, one layer up.
func TestARoutineThatStatesNoModeReadsBackAsUnstated(t *testing.T) {
	_, dr := recordedWith(t, Unstated)
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if got := ModeOf(string(raw)); got != Unstated {
		t.Errorf("a play saved with no mode stated reads back as %q", got)
	}
	if strings.Contains(string(raw), modeStated) {
		t.Error("a save that was told nothing wrote a mode line anyway, so every " +
			"caller that says nothing now makes a claim it never made")
	}
	// AND THE OLD ENTRANCE WRITES NO LINE EITHER. A dozen callers reach `codegen.Route`
	// and know nothing about a mode; one of them gaining a line would be the same
	// invented claim arriving from underneath.
	plain, _, err := codegen.Route("ark graphics", "arkascended",
		[]macroir.Step{{Kind: macroir.StepKey, Key: "escape", Count: 1}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ModeOf(plain) != Unstated {
		t.Error("codegen.Route invented a mode for a caller that passed none")
	}
}

// AND A WORD THIS BUILD DOES NOT KNOW IS NOT PASSED ON AS ONE.
//
// A newer build's word, or one somebody typed into the file by hand. Read as unstated rather than
// echoed: a reader that handed on whatever it found would put an unverified string into a run
// record, where it reads as something this build checked.
func TestAModeThisBuildDoesNotKnowReadsBackAsUnstated(t *testing.T) {
	for _, line := range []string{
		"// Recorded as: semantic.",
		"// Recorded as: .",
		"// Recorded as: coordinates and pictures.",
	} {
		if got := ModeOf("// Auto-generated.\n" + line + "\nuse os.\n"); got != Unstated {
			t.Errorf("%q read back as %q, not unstated", line, got)
		}
	}
}

// A STATED COORDINATE MODE KEEPS THE RECORDED PAUSES, WHATEVER THE SWITCHES SAY.
//
// The other half of the inherited-environment defect, and the quieter one. With CV on, every
// recorded gap is capped to 50ms because the anchor poll was supposed to do the real waiting — so
// a Studio launched with `$MARCO_CV=on` captured no anchor to wait on AND took the pauses out. A
// menu pressed before it has drawn, from a routine that looks correct.
func TestAStatedCoordinateModeKeepsTheRecordedPauses(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	capped := Deps{}.learnOptions().MaxWaitMs
	kept := Deps{Mode: Coordinates}.learnOptions().MaxWaitMs
	if kept <= capped {
		t.Errorf("a stated coordinate mode capped waits at %dms, the same ceiling CV "+
			"being on imposes (%dms) — so the pauses are gone and nothing is "+
			"waiting on an image either", kept, capped)
	}
	// AND THE SWITCH STILL DOES WHAT IT DID for a caller that states nothing, which is
	// the behaviour every other caller was written against.
	t.Setenv("MARCO_CV", "off")
	if off := (Deps{}).learnOptions().MaxWaitMs; off != kept {
		t.Errorf("$MARCO_CV=off no longer keeps the recorded gaps: %dms", off)
	}
}
