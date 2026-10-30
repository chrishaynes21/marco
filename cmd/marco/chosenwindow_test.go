package main

// A RUN TOLD WHICH WINDOW TO ACT IN AND UNABLE TO HAVE IT PRESSES NOTHING.
//
// # The defect this closes
//
// `GiveWindow` verified the window, bound it, and then tried to bring it forward. A raise that
// did not work was recorded in `Unperformed` and the function returned SUCCESS — on the reasoning
// `activate` uses, which is that the guarded boundary asks whether the window is in front at the
// instant of the press and that is the answer that decides.
//
// THAT REASONING HOLDS FOR A GUARDED PRESS AND NOT FOR AN UNGUARDED ONE, and the two scopes with
// no `Activate` — context and global — are exactly where unguarded presses live. So a routine
// that types, played into a window Windows would not foreground, typed into whatever stayed in
// front: the person's editor, their chat window, the terminal they launched it from. The account
// afterwards said truthfully that the window could not be brought forward, beside a run that had
// already typed somewhere else.
//
// The command path is where that is stopped, which is why this test is here rather than in
// `internal/oshost`: the host reports, and the entrance decides.
//
// Nothing here presses anything or needs a desktop. The window is STATED through
// `oshost.WindowForTest`, and the routine that would type is counted by `Host.InputActions` —
// which counts what the run ASKED the machine for, including a press its own guard refused, so
// zero is the only honest way to say nothing was attempted.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// theWindowChosen is the handle and owner a surface resolved and handed over. Valid in every case
// below: what differs is whether the foreground handoff is accepted.
const (
	theWindowChosen uintptr = 0x4420
	itsOwner        uint32  = 7788
)

// aRunInTheChosenWindow is the command path, driven with a routine, a window and a real OS host.
//
// The host is `oshost.New()` — the production one — because what is being counted is what the run
// asked the MACHINE for, and a stand-in would count what it asked a stand-in for. It presses
// nothing: either the hand-over refuses before the routine is entered, or the routine logs.
func aRunInTheChosenWindow(t *testing.T, source, window string) (int, string, *oshost.Host) {
	t.Helper()
	dir := t.TempDir()
	where := filepath.Join(dir, "global")
	if err := os.MkdirAll(where, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(where, "types-something.marco"),
		[]byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	h, ok := oshost.New().(*oshost.Host)
	if !ok {
		t.Fatal("oshost.New no longer makes an *oshost.Host, so a run cannot be " +
			"told which window to act in")
	}
	d := depsForIntake(t)
	d.Reg = routes.Registry{Dir: dir}
	var asHost runtime.Host = h
	d.Hosts = map[string]runtime.Host{"*": asHost}
	said := &strings.Builder{}
	d.Out = said

	req := invoke.Request{Source: invoke.SourceCLI,
		Play: &routes.Route{Slug: "types-something"}}
	wire := &strings.Builder{}
	// AND WHAT THE ROUTINE ITSELF SAYS, WHICH GOES TO STDOUT.
	//
	// A routine's `log` writes to the process's own output, as does the outcome a
	// completed run announces — so a test that read only the writers it passed in
	// would see a refusal and never see a run. Captured the same way
	// `explainInvocation`'s test captures it.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = pw
	read := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(pr)
		read <- string(b)
	}()
	code := doInTheChosenWindow(d, req, false, window, wire, &strings.Builder{})
	os.Stdout = saved
	_ = pw.Close()
	printed := <-read
	_ = pr.Close()
	return code, wire.String() + printed + said.String(), h
}

// aTypingRoutine is a routine whose FIRST step types, which is the case the defect is about: no
// `Activate`, nothing checked, and a keystroke that goes wherever the foreground is.
const aTypingRoutine = "// marco learn \"types something\"\n\n" +
	"the App is a script.\n\n" +
	"do OS's Type with \"hello there\".\n"

// A REJECTED FOREGROUND HANDOFF REFUSES THE RUN AND PRESSES NOTHING.
//
// Four claims, and the third is the one that matters: a refusal somebody reads afterwards is not
// the same thing as a keystroke that never happened.
func TestARejectedHandoffRefusesAndPressesNothing(t *testing.T) {
	// THE WINDOW IS VALID AND WINDOWS WILL NOT BRING IT FORWARD. The foreground lock is
	// not negotiable: it refuses `SetForegroundWindow` to a process that does not
	// already own the foreground, which is the ordinary state of a child nobody is
	// looking at.
	oshost.WindowForTest(t, theWindowChosen, itsOwner, "notepad.exe", false,
		os.ErrPermission)

	code, said, h := aRunInTheChosenWindow(t, aTypingRoutine,
		orchestrator.Pickable{HWnd: theWindowChosen, PID: itsOwner}.Term())

	// NOTHING WAS ASKED OF THE MACHINE.
	if n := h.InputActions(); n != 0 {
		t.Errorf("the run asked the machine for %d input action(s) although the "+
			"window it was told to act in was never brought forward — those "+
			"keystrokes went into whatever stayed in front", n)
	}
	// AND THE OUTCOME IS PUBLISHED AS REFUSED, on the wire line every surface reads.
	if !strings.Contains(said, outcome.Line(outcome.Refused)) {
		t.Errorf("the run did not announce %q:\n%s", outcome.Line(outcome.Refused),
			said)
	}
	if code != outcome.Refused.Exit() {
		t.Errorf("the command exited %d, want %d", code, outcome.Refused.Exit())
	}
	// AND THE ROUTINE WAS NEVER ENTERED. Its own text would be in the account if it
	// had been: a run that typed and then reported a refusal is the exact state this
	// closes.
	if strings.Contains(said, "hello there") {
		t.Errorf("the routine ran:\n%s", said)
	}
	// AND THE FAILURE IS STILL IN THE ACCOUNT, which is worth keeping and is not what
	// stops the run.
	if !strings.Contains(strings.Join(h.Unperformed(), "\n"), "brought to the front") {
		t.Errorf("the run does not record why nothing happened: %v", h.Unperformed())
	}
}

// AND THE CONTROL: AN ACCEPTED HANDOFF RUNS THE ROUTINE.
//
// Without this the fix above is satisfied by refusing every run, which would be a worse product
// than the defect. The routine LOGS rather than types, because `marco do` performs real input and
// a test that typed on whoever's desktop ran it is a test nobody may run — so what this shows is
// that the hand-over succeeded, the routine was entered, and the run reached its own answer.
func TestAnAcceptedHandoffRunsTheRoutine(t *testing.T) {
	oshost.WindowForTest(t, theWindowChosen, itsOwner, "notepad.exe", true, nil)

	code, said, h := aRunInTheChosenWindow(t,
		"// marco learn \"types something\"\n\nthe App is a script.\n\n"+
			"log \"it was entered\".\n",
		orchestrator.Pickable{HWnd: theWindowChosen, PID: itsOwner}.Term())

	if code != 0 {
		t.Errorf("an accepted hand-over exited %d:\n%s", code, said)
	}
	if strings.Contains(said, outcome.Line(outcome.Refused)) {
		t.Errorf("an accepted hand-over refused the run:\n%s", said)
	}
	if !strings.Contains(said, "it was entered") {
		t.Errorf("the routine was not entered:\n%s", said)
	}
	if n := h.InputActions(); n != 0 {
		t.Errorf("a routine that only logs asked for %d input action(s)", n)
	}
	// AND THE WINDOW IT WAS GIVEN IS THE ONE IT IS BOUND TO, which is what a guarded
	// press would be checked against.
	if len(h.Unperformed()) != 0 {
		t.Errorf("an accepted hand-over recorded something as not done: %v",
			h.Unperformed())
	}
}

// AND A WINDOW THAT IS NOT THERE REFUSES THE SAME WAY, which is the case the end-to-end Studio
// test covers across two processes and this one covers in the function that decides.
func TestAHandoverOfAWindowThatIsNotThereRefusesAndPressesNothing(t *testing.T) {
	// THE DESKTOP KNOWS ONE WINDOW AND IT IS NOT THE ONE HANDED OVER.
	oshost.WindowForTest(t, theWindowChosen, itsOwner, "notepad.exe", true, nil)

	code, said, h := aRunInTheChosenWindow(t, aTypingRoutine,
		orchestrator.Pickable{HWnd: 0x9999, PID: itsOwner}.Term())

	if n := h.InputActions(); n != 0 {
		t.Errorf("the run asked the machine for %d input action(s) with no window "+
			"to act in", n)
	}
	if !strings.Contains(said, outcome.Line(outcome.Refused)) {
		t.Errorf("the run did not announce refused:\n%s", said)
	}
	if code != outcome.Refused.Exit() {
		t.Errorf("the command exited %d, want %d", code, outcome.Refused.Exit())
	}
	if strings.Contains(said, "hello there") {
		t.Errorf("the routine ran:\n%s", said)
	}
}

// AND A SIBLING WINDOW IN FRONT IS A REFUSAL, THOUGH EVERY PROCESS-LEVEL CHECK PASSES.
//
// THE DISCRIMINATING CASE, and the ambiguity this whole slice exists to remove. Two windows share
// one process — two Explorer windows are one `explorer.exe`, and so are two windows of most
// editors. A was chosen; B stays in front; `winctx.Raise` reports SUCCESS, because it waits for
// the PROGRAM to lead and the program is leading through B.
//
// Nothing downstream would notice. An unguarded press goes to whatever is in front, and B is the
// same program at the same size and the same scale as A — so the run would have typed into the
// other window with every check it has passing. The guarded boundary asks `Front` of its own
// handle and refuses; an unguarded press asks nothing, which is the whole reason the handoff has
// to ask.
//
// Set up exactly as the finding describes: the selected handle is alive and still owned by the
// selected PID, the lower-level raise reports success, and the selected window is NOT the one in
// front.
func TestASiblingWindowInFrontRefusesAndPressesNothing(t *testing.T) {
	// A IS CHOSEN, OWNED BY THE PROCESS IT WAS CHOSEN FROM, AND NOT IN FRONT. The raise
	// says yes: its question is about `notepad.exe`, and B is a notepad window.
	oshost.WindowForTest(t, theWindowChosen, itsOwner, "notepad.exe", false, nil)

	code, said, h := aRunInTheChosenWindow(t, aTypingRoutine,
		orchestrator.Pickable{HWnd: theWindowChosen, PID: itsOwner}.Term())

	if n := h.InputActions(); n != 0 {
		t.Errorf("the run asked the machine for %d input action(s) while another "+
			"window of the same program was in front — those keystrokes went "+
			"into the window nobody chose", n)
	}
	if !strings.Contains(said, outcome.Line(outcome.Refused)) {
		t.Errorf("the run did not announce %q:\n%s", outcome.Line(outcome.Refused),
			said)
	}
	if code != outcome.Refused.Exit() {
		t.Errorf("the command exited %d, want %d", code, outcome.Refused.Exit())
	}
	if strings.Contains(said, "hello there") {
		t.Errorf("the routine ran:\n%s", said)
	}
	// AND THE ACCOUNT SAYS WHICH WINDOW, because "could not be brought to the front" is
	// not what happened: the program came forward and the wrong window of it is in
	// front, and the way out is to click the one they meant.
	saidNot := strings.Join(h.Unperformed(), "\n")
	if !strings.Contains(saidNot, "not the one in front") {
		t.Errorf("the account does not say a sibling window is in front: %v",
			h.Unperformed())
	}
}

// AND THE EXACT-WINDOW CONTROL: THE CHOSEN WINDOW IN FRONT RUNS.
//
// The same program, the same owner, the same successful raise — and A itself in front. Without
// this the rule above is satisfied by refusing every handoff, and the difference between the two
// tests is exactly one term: WHICH window of the program ended up in front.
func TestTheChosenWindowInFrontRunsTheRoutine(t *testing.T) {
	oshost.WindowForTest(t, theWindowChosen, itsOwner, "notepad.exe", true, nil)

	code, said, h := aRunInTheChosenWindow(t,
		"// marco learn \"types something\"\n\nthe App is a script.\n\n"+
			"log \"it was entered\".\n",
		orchestrator.Pickable{HWnd: theWindowChosen, PID: itsOwner}.Term())

	if code != 0 {
		t.Errorf("a hand-over to the window that is in front exited %d:\n%s", code,
			said)
	}
	if !strings.Contains(said, "it was entered") {
		t.Errorf("the routine was not entered:\n%s", said)
	}
	if len(h.Unperformed()) != 0 {
		t.Errorf("a hand-over to the right window recorded something as not done: "+
			"%v", h.Unperformed())
	}
}

// AND NO `--window` AT ALL IS THE COMMAND THIS HAS ALWAYS BEEN.
//
// The legacy path, which this contract preserves: a saved routine performed from a terminal or by
// the overlay is not somebody choosing a window, and nothing is handed over or refused.
func TestWithoutAChosenWindowTheCommandIsUnchanged(t *testing.T) {
	code, said, _ := aRunInTheChosenWindow(t,
		"// marco learn \"types something\"\n\nthe App is a script.\n\n"+
			"log \"it was entered\".\n", "")

	if code != 0 {
		t.Errorf("a command with no window exited %d:\n%s", code, said)
	}
	if strings.Contains(said, outcome.Line(outcome.Refused)) {
		t.Errorf("a command that named no window was refused one:\n%s", said)
	}
	if !strings.Contains(said, "it was entered") {
		t.Errorf("the routine was not entered:\n%s", said)
	}
}
