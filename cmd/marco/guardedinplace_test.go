package main

// S9: A GUARDED "ONLY IN" ROUTINE IS BOUND TO THE WINDOW IN FRONT (beside inplace_test.go,
// which holds the admission rule itself).
//
// Entered through the intake (doInTheChosenWindow, the body of `marco do`) into the REAL host,
// over a stated desktop (oshost.DesktopForTest): the windows, the foreground, and the input
// device are stated; the admission, the binding and every press guard are production. The routine
// is saved through the production generator, so its presses are exactly what an Only-in save
// writes. Studio's handoff is entered through `--window`, never by setting HandedTo.

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

const (
	notepadA uintptr = 0x100 // the window the routine runs in
	notepadB uintptr = 0x200 // a sibling window of the same program
	chromeW  uintptr = 0x300 // another application
)

// twoGuardedClicks is a routine of two presses, each checked against a 1280x720 notepad window at
// 96 dpi, at offsets (500,400) and (100,50).
func twoGuardedClicks(t *testing.T, d orchestrator.Deps, scope orchestrator.Scope) routes.Route {
	t.Helper()
	click := func(rx, ry int) []recorder.RecordedEvent {
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 640, Y: 480, App: "notepad",
				PID: 50, RelX: rx, RelY: ry, RelTo: macroir.RelToWindow, WinRel: true,
				WinW: 1280, WinH: 720, Dpi: 96, Process: "notepad.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 640, Y: 480, App: "notepad", PID: 50},
		}
	}
	events := append([]recorder.RecordedEvent{{Kind: recorder.EvAppSwitch, KeyName: "notepad"}},
		append(click(500, 400), click(100, 50)...)...)
	saving := orchestrator.Deps{Reg: d.Reg, Out: &strings.Builder{}, Mode: orchestrator.Coordinates}
	dr := saving.DraftOf("two clicks", events, "notepad", "a demonstration", orchestrator.Coordinates)
	if why := saving.WhyNotThisScope(dr, scope); why != "" {
		t.Fatalf("the scope is refused: %s", why)
	}
	rt, err := saving.Save(dr, scope, "notepad")
	if err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(d.Reg.Path(rt))
	if c, _ := orchestrator.ContractOf(string(src)); c != orchestrator.Guarded {
		t.Fatalf("the fixture is %s, not guarded:\n%s", c, src)
	}
	return rt
}

func notepadAt(pid uint32, left, top int) oshost.TestWindow {
	return oshost.TestWindow{PID: pid, Program: "notepad.exe", Left: left, Top: top, W: 1280, H: 720, Dpi: 96}
}

// aDesktop is notepad A at (200,100) in front, its sibling B, and chrome.
func aDesktop() *oshost.TestDesktop {
	return &oshost.TestDesktop{Front: notepadA, Windows: map[uintptr]oshost.TestWindow{
		notepadA: notepadAt(50, 200, 100),
		notepadB: notepadAt(51, 900, 300),
		chromeW:  {PID: 60, Program: "chrome.exe", W: 1600, H: 900, Dpi: 96},
	}}
}

// runOnlyIn runs rt through the intake over desk, with window as the run's --window term.
func runOnlyIn(t *testing.T, d orchestrator.Deps, desk *oshost.TestDesktop, rt routes.Route,
	window, front string) (int, string) {
	t.Helper()
	d.Hosts = map[string]runtime.Host{"*": oshost.DesktopForTest(t, desk)}
	d.App = func() string { return front } // the legacy admission's reading
	d.Out = &strings.Builder{}
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = pw
	read := make(chan string, 1)
	go func() { b, _ := io.ReadAll(pr); read <- string(b) }()
	route := rt
	code := doInTheChosenWindow(d, invoke.Request{Source: invoke.SourceHotkey, Play: &route},
		false, window, &strings.Builder{}, &strings.Builder{})
	os.Stdout = saved
	_ = pw.Close()
	said := <-read
	_ = pr.Close()
	return code, said
}

func anInPlaceWorld(t *testing.T) (orchestrator.Deps, routes.Route) {
	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d := depsForIntake(t)
	return d, twoGuardedClicks(t, d, orchestrator.ScopeApp)
}

// The window in front is bound and pressed at its origin plus each offset, with nothing raised and
// nothing searched for. A window MOVED between the two presses is pressed where it now is.
//
// Deleting the binding in the intake must fail this (every press would refuse with nothing bound),
// and so must binding by a search.
func TestAGuardedOnlyInRunIsBoundToTheWindowInFront(t *testing.T) {
	d, rt := anInPlaceWorld(t)
	desk := aDesktop()
	desk.AfterPress = func(n int, d *oshost.TestDesktop) {
		if n == 1 { // the person drags it between the presses
			w := d.Windows[notepadA]
			w.Left, w.Top = 600, 400
			d.Windows[notepadA] = w
		}
	}
	code, said := runOnlyIn(t, d, desk, rt, "", "chrome") // the legacy reading is not consulted
	presses, _, raised, searched := desk.Read()
	if code != 0 || !strings.Contains(said, outcome.Line(outcome.Performed)) {
		t.Fatalf("the run: %d\n%s", code, said)
	}
	if len(presses) != 2 || presses[0] != [2]int{700, 500} || presses[1] != [2]int{700, 450} {
		t.Errorf("pressed at %v, want (200+500,100+400) and, after the move, (600+100,400+50)", presses)
	}
	if raised != 0 || searched != 0 {
		t.Errorf("a window was raised %d time(s) and searched for %d time(s)", raised, searched)
	}
}

// A sibling window of the application that is already in front at admission is the one bound: the
// run's target is the window in front, not the window it was recorded in.
func TestASiblingInFrontAtAdmissionIsTheOneBound(t *testing.T) {
	d, rt := anInPlaceWorld(t)
	desk := aDesktop()
	desk.Front = notepadB
	code, said := runOnlyIn(t, d, desk, rt, "", "")
	presses, _, _, _ := desk.Read()
	if code != 0 || len(presses) != 2 || presses[0] != [2]int{1400, 700} {
		t.Errorf("with the sibling in front: %d, pressed at %v\n%s", code, presses, said)
	}
}

// What refuses: another application in front at admission (nothing bound, nothing pressed), and,
// once bound, a window that is resized, rescaled, closed, put behind another window, handed to
// another owner, or replaced in front by a sibling. Each refuses at the press after the change.
func TestAGuardedOnlyInRunRefusesAWindowThatIsNotItsTarget(t *testing.T) {
	t.Run("another application in front", func(t *testing.T) {
		d, rt := anInPlaceWorld(t)
		desk := aDesktop()
		desk.Front = chromeW
		code, said := runOnlyIn(t, d, desk, rt, "", "notepad") // even with a stale legacy reading
		presses, _, _, _ := desk.Read()
		if code != outcome.Refused.Exit() || len(presses) != 0 || !strings.Contains(said, "chrome is in front") {
			t.Errorf("%d, pressed %v\n%s", code, presses, said)
		}
	})
	for _, c := range []struct {
		what   string
		change func(d *oshost.TestDesktop)
		says   string
	}{
		{"resized", func(d *oshost.TestDesktop) { w := d.Windows[notepadA]; w.W = 1600; d.Windows[notepadA] = w }, "1600"},
		{"rescaled", func(d *oshost.TestDesktop) { w := d.Windows[notepadA]; w.Dpi = 144; d.Windows[notepadA] = w }, "dpi"},
		{"closed", func(d *oshost.TestDesktop) { delete(d.Windows, notepadA) }, "closed"},
		{"behind another application", func(d *oshost.TestDesktop) { d.Front = chromeW }, "no longer in front"},
		{"a sibling took the front", func(d *oshost.TestDesktop) { d.Front = notepadB }, "no longer in front"},
		{"another owner", func(d *oshost.TestDesktop) { w := d.Windows[notepadA]; w.PID = 99; d.Windows[notepadA] = w }, "process"},
	} {
		t.Run(c.what, func(t *testing.T) {
			d, rt := anInPlaceWorld(t)
			desk := aDesktop()
			desk.AfterPress = func(n int, d *oshost.TestDesktop) {
				if n == 1 {
					c.change(d)
				}
			}
			code, said := runOnlyIn(t, d, desk, rt, "", "")
			presses, _, _, _ := desk.Read()
			if len(presses) != 1 || code == 0 || !strings.Contains(said, c.says) {
				t.Errorf("after the window was %s: %d, pressed %v\n%s", c.what, code, presses, said)
			}
		})
	}
}

// Studio's test Play, entered through --window: the window it chose is raised (the person's own
// press in Studio, which is in front) and validated, the routine is admitted by it, and the
// presses are checked against it. A window of another program is refused before anything runs.
func TestStudiosHandoffRunsAGuardedOnlyInRoutine(t *testing.T) {
	d, rt := anInPlaceWorld(t)
	desk := aDesktop()
	desk.Windows[0x400] = oshost.TestWindow{PID: 70, Program: "studio.exe", W: 1000, H: 800, Dpi: 96}
	desk.Front = 0x400 // Studio is in front when Play is pressed
	term := orchestrator.Pickable{HWnd: notepadA, PID: 50}.Term()
	code, said := runOnlyIn(t, d, desk, rt, term, "studio")
	presses, _, raised, searched := desk.Read()
	if code != 0 || len(presses) != 2 || presses[0] != [2]int{700, 500} || raised != 1 || searched != 0 {
		t.Errorf("the handoff: %d, pressed %v, raised %d, searched %d\n%s", code, presses, raised, searched, said)
	}

	d2, rt2 := anInPlaceWorld(t)
	desk2 := aDesktop()
	desk2.Front = chromeW
	code, said = runOnlyIn(t, d2, desk2, rt2, orchestrator.Pickable{HWnd: chromeW, PID: 60}.Term(), "")
	presses, _, _, _ = desk2.Read()
	if code != outcome.Refused.Exit() || len(presses) != 0 || !strings.Contains(said, "belongs to chrome") {
		t.Errorf("a handoff of another program's window: %d, pressed %v\n%s", code, presses, said)
	}
}

// A legacy (unguarded) Only-in routine attempts no binding: it is admitted by the application in
// front, read as it always was, even when the window in front could not be bound at all.
func TestALegacyOnlyInRoutineIsAdmittedAsBefore(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	d := depsForIntake(t)
	rt := routes.Route{App: "notepad", Slug: "say-hi"}
	if err := d.Reg.Save(rt, "// marco learn \"say hi\"\n\nthe App is a script.\n\nlog \"hi\".\n"); err != nil {
		t.Fatal(err)
	}
	desk := &oshost.TestDesktop{} // nothing in front that could be bound
	code, said := runOnlyIn(t, d, desk, rt, "", "notepad")
	if code != 0 || !strings.Contains(said, "hi") {
		t.Errorf("a legacy Only-in routine with notepad in front: %d\n%s", code, said)
	}
	if code, said := runOnlyIn(t, d, desk, rt, "", "chrome"); code != outcome.Refused.Exit() {
		t.Errorf("a legacy Only-in routine with chrome in front: %d\n%s", code, said)
	}
}
