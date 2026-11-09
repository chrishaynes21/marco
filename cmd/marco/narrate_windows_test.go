//go:build windows

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// F7: a narration session is of the window chosen before it started. The routine is filed under
// that window's application and a click is placed relative to it; with no window chosen it is
// the foreground, as before. Windows only: the process's application name is read from it.
func TestANarrationIsOfTheChosenWindow(t *testing.T) {
	d := newDeps()
	d.App = func() string { return "whatever-is-in-front" }
	d.Window = recorder.Target{HWnd: 0x1234, PID: uint32(os.Getpid())}
	got, env := narrationIn(d)
	if want := winctx.ProcessImage(uint32(os.Getpid())); want == "" || got.App() != want {
		t.Errorf("the play would be filed under %q, not the chosen window's %q", got.App(), want)
	}
	if e, ok := env.(chosenWindowEnv); !ok || e.hwnd != 0x1234 {
		t.Errorf("the session reads positions from %T %+v", env, env)
	}
	d.Window = recorder.Target{}
	if got, _ := narrationIn(d); got.App() != "whatever-is-in-front" {
		t.Errorf("with no window chosen it is filed under %q", got.App())
	}
}

// And runNarrateLearn is the session in that window: entered through it, with narration on stdin
// and the answers to its questions on Deps.In, the routine it saves is filed under the chosen
// window's application, not the one in front.
//
// Building fresh Deps in runNarrateLearn again must fail this.
func TestTheNarratedRoutineIsFiledUnderTheChosenWindow(t *testing.T) {
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_HOME", t.TempDir())
	d := newDeps()
	d.App = func() string { return "whatever-is-in-front" }
	d.Window = recorder.Target{HWnd: 0x1234, PID: uint32(os.Getpid())}
	d.In = strings.NewReader("y\n\n")
	var out strings.Builder
	d.Out = &out
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev; r.Close() })
	_, _ = w.WriteString("type hello\ndone\n")
	w.Close()
	if err := runNarrateLearn(d, "say hello"); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	want := winctx.ProcessImage(uint32(os.Getpid()))
	saved := d.Reg.List()
	if len(saved) != 1 || saved[0].App != want {
		t.Errorf("the narrated play was saved as %+v, not under %q\n%s", saved, want, out.String())
	}
}
