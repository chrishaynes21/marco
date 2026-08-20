package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The keystroke path, end to end through the REAL Marco pipeline.
//
//	A successful host call does not prove input reached the intended control.
//
// These tests drive lower → lex → parse → graph → compile → runtime → host, so a chord
// that does not survive compilation fails here rather than on a user's desktop. The
// host is fake; everything above it is the production code path.

// recordingHost is an OS host that records the calls Marco actually made.
type recordingHost struct {
	calls []hostCall
	err   error
}

type hostCall struct {
	action string
	chord  string
	text   string
	hold   int
}

func (h *recordingHost) Invoke(call runtime.HostCall) (string, runtime.Value, error) {
	c := hostCall{action: call.Action}
	if set := call.Input.AsSet(); set != nil {
		if k, ok := set.Get("Key"); ok {
			c.chord = k.AsText()
		}
		if n, ok := set.Get("Hold"); ok {
			if num, is := n.AsNumber(); is {
				c.hold = int(num)
			}
		}
	} else {
		c.chord, c.text = call.Input.AsText(), call.Input.AsText()
	}
	h.calls = append(h.calls, c)
	if h.err != nil {
		return "failed", runtime.Absent(), h.err
	}
	return "ok", runtime.Absent(), nil
}

func newExec(t *testing.T, host *recordingHost) *marcoexec.Executor {
	t.Helper()
	return marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": host}))
}

func TestEveryEditingChordSurvivesCompilationAndReachesTheHost(t *testing.T) {
	// The chords the editor actually emits. Each one is lowered to Marco source, run
	// through the real compiler, and executed — so a chord Marco cannot express fails
	// at compile time here instead of silently going nowhere live.
	for _, chord := range []string{
		"ctrl+c", "ctrl+v", "ctrl+a", "ctrl+z", "ctrl+y",
		"enter", "delete", "ctrl+end",
	} {
		host := &recordingHost{}
		x := newExec(t, host)

		if err := x.KeyIn(context.Background(), "hwnd:1", chord, 30*time.Millisecond); err != nil {
			t.Errorf("%q: %v", chord, err)
			continue
		}
		if len(host.calls) != 1 {
			t.Errorf("%q produced %d host calls, want 1", chord, len(host.calls))
			continue
		}
		got := host.calls[0]
		if got.action != "Key" {
			t.Errorf("%q invoked OS's %s, want Key", chord, got.action)
		}
		// The chord arrives WHOLE. A chord split into separate keys, or mangled into
		// text, would press the wrong things and still report success.
		if got.chord != chord {
			t.Errorf("the host received %q, want %q", got.chord, chord)
		}
		if got.hold != 30 {
			t.Errorf("%q hold = %d, want the requested 30ms", chord, got.hold)
		}
	}
}

func TestAChordIsOneChordAndNotTypedText(t *testing.T) {
	// "ctrl+c" reaching OS's Type would type the literal characters into the document.
	host := &recordingHost{}
	if err := newExec(t, host).KeyIn(context.Background(), "hwnd:1", "ctrl+c", 0); err != nil {
		t.Fatalf("key: %v", err)
	}
	for _, c := range host.calls {
		if c.action == "Type" {
			t.Fatalf("the chord was typed as text: %+v", c)
		}
	}
}

func TestTheGeneratedMarcoForAChordIsLegalAndNamesTheRealCapability(t *testing.T) {
	src, err := marcoexec.Lower(marcoexec.Operation{
		Kind: marcoexec.KindKey, Chord: "ctrl+c", Window: "hwnd:1", Hold: 30,
	})
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	// The Press set is what os.marco declares for a held chord — not invented syntax.
	if !strings.Contains(src, "is a Press with Key") || !strings.Contains(src, "OS's Key") {
		t.Fatalf("generated source does not use the declared Press/Key form:\n%s", src)
	}
	// The window is Director bookkeeping for the guard. It must not leak into source:
	// OS's Key takes a key, not a window.
	if strings.Contains(src, "hwnd:1") {
		t.Fatalf("the guard's window was lowered into the program:\n%s", src)
	}
}

// ── the guard ─────────────────────────────────────────────────────────────────

// recordingGuard records what window it was asked about.
type recordingGuard struct {
	asked []string
	allow bool
}

func (g *recordingGuard) Confirm(_ context.Context, window string) (bool, string, error) {
	g.asked = append(g.asked, window)
	if g.allow {
		return true, "", nil
	}
	return false, "the intended window is not in front", nil
}

func TestAKeystrokeNamesItsWindowSoTheGuardCanCheckIt(t *testing.T) {
	// The bug this exists for: the editor knew the window, the guard was wired, and
	// nothing joined them — so Confirm was asked about "" and returned true every time,
	// and Ctrl+C went to whatever happened to be in front.
	host := &recordingHost{}
	guard := &recordingGuard{allow: true}
	x := newExec(t, host).WithGuard(guard)

	if err := x.KeyIn(context.Background(), "hwnd:42", "ctrl+c", 0); err != nil {
		t.Fatalf("key: %v", err)
	}
	if len(guard.asked) != 1 {
		t.Fatalf("the guard was consulted %d times, want 1", len(guard.asked))
	}
	if guard.asked[0] != "hwnd:42" {
		t.Fatalf("the guard was asked about %q, want the intended window", guard.asked[0])
	}
}

func TestARefusedGuardMeansTheKeystrokeIsNeverSent(t *testing.T) {
	// Refusing must happen BEFORE the host is reached. Input delivered to the wrong
	// application succeeds, verifies against nothing, and is invisible.
	host := &recordingHost{}
	guard := &recordingGuard{allow: false}
	x := newExec(t, host).WithGuard(guard)

	err := x.KeyIn(context.Background(), "hwnd:42", "ctrl+c", 0)
	if err == nil {
		t.Fatal("a refused context still sent the keystroke")
	}
	if len(host.calls) != 0 {
		t.Fatalf("the host was reached anyway: %+v", host.calls)
	}
}

func TestTypingAlsoNamesItsWindow(t *testing.T) {
	host := &recordingHost{}
	guard := &recordingGuard{allow: true}
	x := newExec(t, host).WithGuard(guard)

	if err := x.TypeIn(context.Background(), "hwnd:7", "hello"); err != nil {
		t.Fatalf("type: %v", err)
	}
	if len(guard.asked) != 1 || guard.asked[0] != "hwnd:7" {
		t.Fatalf("the guard was asked about %v, want hwnd:7", guard.asked)
	}
}

func TestAnUnaddressedKeystrokeIsNotCheckable(t *testing.T) {
	// Stated as a test because it is the hazard, not a feature. Actuator.Key has no
	// window, so the guard passes trivially — which is exactly why providers.Input
	// requires one and the editor cannot reach this path.
	host := &recordingHost{}
	guard := &recordingGuard{allow: true}
	x := newExec(t, host).WithGuard(guard)

	if err := x.Key(context.Background(), "ctrl+c", 0); err != nil {
		t.Fatalf("key: %v", err)
	}
	if len(guard.asked) != 1 || guard.asked[0] != "" {
		t.Fatalf("asked = %v; the unaddressed form has no window to offer", guard.asked)
	}
}

func TestAHostFailureIsReportedRatherThanSwallowed(t *testing.T) {
	host := &recordingHost{err: errors.New("SendInput rejected key")}
	err := newExec(t, host).KeyIn(context.Background(), "hwnd:1", "ctrl+c", 0)
	if err == nil {
		t.Fatal("a rejected keystroke reported success")
	}
}

var _ directorapi.Actuator = (*marcoexec.Executor)(nil)
