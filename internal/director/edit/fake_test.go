package edit_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// control is a fake text field that behaves like a real one.
//
// It is a simulation rather than a set of call recorders, and that is the point: a
// test that only asserts "Type was called with hello" passes for an editor that types
// into the wrong control, types over a selection it forgot to make, or types into a
// field that never took focus. Making the fake actually MODEL selection, focus, undo
// and length caps is what lets the tests assert the thing that matters — the resulting
// text state.
type control struct {
	value    string
	focused  bool
	selected bool // the whole value is selected

	// supportsValue is whether the control implements a writable value API. The
	// switch that decides which rung of the ladder the editor lands on.
	supportsValue bool
	readOnlyValue bool
	// maxLen models a field with a length cap: it accepts a write and keeps less.
	maxLen int
	// undo is the application's own history. Empty means "nothing to undo", which
	// real applications respond to by silently doing nothing.
	undo []string
	redo []string

	typed []string
	keys  []string
	// stray is input that arrived while the control was unfocused — i.e. input that
	// went to some other window entirely. Never empty in a correct run.
	stray []string
	// windows records the window each input call named. An empty entry means the
	// keystroke was sent with no destination, which is the state in which the
	// foreground guard cannot check anything and passes trivially.
	windows []directorapi.WindowID
	// valueCalls counts value-API writes, so a test can prove the top rung was tried.
	valueCalls int
	failValue  bool
}

func (c *control) push() { c.undo = append(c.undo, c.value); c.redo = nil }

func (c *control) set(v string) {
	if c.maxLen > 0 && len(v) > c.maxLen {
		v = v[:c.maxLen]
	}
	c.value = v
}

// ── the provider faces ────────────────────────────────────────────────────────

type fakeValues struct{ c *control }

func (f fakeValues) SetValue(_ context.Context, _ directorapi.WindowID, _, value string) (string, error) {
	f.c.valueCalls++
	if f.c.failValue {
		return "", fmt.Errorf("the bridge is not answering")
	}
	if !f.c.supportsValue {
		return "", &directorapi.ValueUnsupportedError{Reason: "the control does not implement ValuePattern"}
	}
	if f.c.readOnlyValue {
		return "", &directorapi.ValueUnsupportedError{Reason: "the control is read-only"}
	}
	f.c.push()
	f.c.set(value)
	return f.c.value, nil
}

func (f fakeValues) GetValue(_ context.Context, _ directorapi.WindowID, _ string) (string, bool, error) {
	if !f.c.supportsValue {
		return "", false, nil
	}
	return f.c.value, true, nil
}

type fakeFocus struct {
	c *control
	// refuse models an application that accepts the focus request and does not act
	// on it — which is why focus is confirmed by observation rather than by nil.
	refuse bool
	err    error
}

func (f *fakeFocus) Focus(context.Context, directorapi.WindowID, string) error {
	if f.err != nil {
		return f.err
	}
	if !f.refuse {
		f.c.focused = true
	}
	return nil
}

type fakeObserver struct{ c *control }

func (f fakeObserver) Element(context.Context, directorapi.WindowID, directorapi.ElementID) (directorapi.Element, bool, error) {
	return directorapi.Element{
		ID: "e1", Role: directorapi.RoleTextField,
		Value: f.c.value, Focused: f.c.focused,
	}, true, nil
}

type fakeInput struct {
	c     *control
	board *fakeBoard
}

func (f *fakeInput) Type(_ context.Context, window directorapi.WindowID, text string) error {
	f.c.windows = append(f.c.windows, window)
	if !f.c.focused {
		// Input sent to an unfocused control does NOT fail. It lands somewhere else —
		// another window, a game, a chat box — and returns success, which is precisely
		// what makes it dangerous. Modelling it as an error would let a test pass for
		// an editor that never checked focus at all.
		f.c.stray = append(f.c.stray, text)
		return nil
	}
	f.c.typed = append(f.c.typed, text)
	f.c.push()
	if f.c.selected {
		f.c.set(text)
		f.c.selected = false
		return nil
	}
	f.c.set(f.c.value + text)
	return nil
}

func (f *fakeInput) Key(_ context.Context, window directorapi.WindowID, chord string, _ time.Duration) error {
	f.c.windows = append(f.c.windows, window)
	if !f.c.focused {
		f.c.stray = append(f.c.stray, chord)
		return nil
	}
	f.c.keys = append(f.c.keys, chord)
	switch strings.ToLower(chord) {
	case "ctrl+a":
		f.c.selected = true
	case "ctrl+end":
		f.c.selected = false
	case "delete", "backspace":
		if f.c.selected {
			f.c.push()
			f.c.set("")
			f.c.selected = false
		}
	case "ctrl+v":
		f.c.push()
		if f.c.selected {
			f.c.set(f.board.text)
			f.c.selected = false
		} else {
			f.c.set(f.c.value + f.board.text)
		}
	case "ctrl+c":
		if f.c.selected {
			f.board.text, f.board.isText = f.c.value, true
		}
	case "ctrl+z":
		// Nothing to undo: the application consumes the keystroke and does nothing.
		// Exactly the case that "the key was sent" would wrongly call a success.
		if len(f.c.undo) > 0 {
			f.c.redo = append(f.c.redo, f.c.value)
			f.c.value = f.c.undo[len(f.c.undo)-1]
			f.c.undo = f.c.undo[:len(f.c.undo)-1]
		}
	case "ctrl+y":
		if len(f.c.redo) > 0 {
			f.c.undo = append(f.c.undo, f.c.value)
			f.c.value = f.c.redo[len(f.c.redo)-1]
			f.c.redo = f.c.redo[:len(f.c.redo)-1]
		}
	}
	return nil
}

// fakeBoard is a clipboard that records every write, so a test can prove the original
// contents were put back rather than merely that the last write matched.
type fakeBoard struct {
	text   string
	isText bool
	writes []string
	// empty distinguishes a clipboard with nothing on it from one holding an image.
	empty bool
	// failRead and failWrite model a clipboard held by another process.
	failRead  bool
	failWrite bool
}

func (b *fakeBoard) Read(context.Context) (directorapi.ClipboardContents, error) {
	if b.failRead {
		return directorapi.ClipboardContents{}, fmt.Errorf("another process is holding the clipboard")
	}
	return directorapi.ClipboardContents{Text: b.text, IsText: b.isText, Empty: b.empty}, nil
}

func (b *fakeBoard) Write(_ context.Context, text string) error {
	if b.failWrite {
		return fmt.Errorf("another process is holding the clipboard")
	}
	b.writes = append(b.writes, text)
	b.text, b.isText = text, true
	return nil
}

// immediate is a settler that evaluates the condition without waiting, which is what a
// test wants: the fake world is already up to date, and a real wait would only add
// wall-clock time.
type immediate struct{}

func (immediate) Settle(ctx context.Context, want func(context.Context) bool) (bool, error) {
	return want(ctx), nil
}
