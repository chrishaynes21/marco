// Package oshost implements a native, in-process Host (see spec/Hosts.md) that
// performs real OS effects — keystrokes, mouse, sleep, pixel reads — for foreign
// acts. The platform primitives live behind a small `backend` interface with a
// Windows implementation (SendInput) and a stub for other platforms, so the
// package builds everywhere; only Windows does real work.
package oshost

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// backend is the platform-specific primitive surface. Implemented by the
// Windows SendInput backend and a no-op stub elsewhere.
type backend interface {
	key(ctx context.Context, name string) error
	typeText(ctx context.Context, text string) error
	click(ctx context.Context, button string) error
	move(ctx context.Context, x, y int) error
	color(ctx context.Context, x, y int) (uint32, error)
	activeExe(ctx context.Context) (string, error)
}

// Host adapts a backend to runtime.Host, dispatching foreign action names to
// platform primitives and marshalling the call's Input/Output Values.
type Host struct{ b backend }

// New returns the native OS host for the current platform.
func New() runtime.Host { return Host{b: newBackend()} }

func (h Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch strings.ToLower(c.Action) {
	case "key":
		return ok(h.b.key(c.Ctx, c.Input.AsText()))
	case "type":
		return ok(h.b.typeText(c.Ctx, c.Input.AsText()))
	case "click":
		return h.doClick(c)
	case "move":
		x, y, present := point(c.Input)
		if !present {
			return fail("move needs a Point with X and Y")
		}
		return ok(h.b.move(c.Ctx, x, y))
	case "sleep":
		return h.doSleep(c)
	case "color":
		return h.doColor(c)
	case "focus":
		return h.doFocus(c)
	case "repeat":
		return h.doRepeat(c)
	default:
		return fail(fmt.Sprintf("OS host has no action %q", c.Action))
	}
}

// doClick: a Point input moves there first; a text input names the button;
// absent input clicks left at the current position.
func (h Host) doClick(c runtime.HostCall) (string, runtime.Value, error) {
	if x, y, present := point(c.Input); present {
		if err := h.b.move(c.Ctx, x, y); err != nil {
			return fail(err.Error())
		}
		return ok(h.b.click(c.Ctx, "left"))
	}
	button := "left"
	if t := c.Input.AsText(); t != "" {
		button = t
	}
	return ok(h.b.click(c.Ctx, button))
}

func (h Host) doSleep(c runtime.HostCall) (string, runtime.Value, error) {
	ms, present := c.Input.AsNumber()
	if !present {
		return fail("sleep needs a number of milliseconds")
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return "ok", runtime.Absent(), nil
	case <-c.Ctx.Done():
		return "ok", runtime.Absent(), nil
	}
}

func (h Host) doColor(c runtime.HostCall) (string, runtime.Value, error) {
	x, y, present := point(c.Input)
	if !present {
		return fail("color needs a Point with X and Y")
	}
	col, err := h.b.color(c.Ctx, x, y)
	if err != nil {
		return fail(err.Error())
	}
	return "ok", runtime.Text(fmt.Sprintf("0x%06X", col)), nil
}

// doFocus resolves ok when the active window's exe matches the given substring
// (case-insensitive), failed otherwise — used to gate window-scoped macros.
func (h Host) doFocus(c runtime.HostCall) (string, runtime.Value, error) {
	spec := c.Input.AsText()
	exe, err := h.b.activeExe(c.Ctx)
	if err != nil {
		return fail(err.Error())
	}
	if spec == "" || strings.Contains(strings.ToLower(exe), strings.ToLower(spec)) {
		return "ok", runtime.Text(exe), nil
	}
	return "failed", runtime.Absent(), nil
}

// doRepeat presses a key on an interval until the frame is canceled. Input is a
// set { Key: text, Every: number-of-ms }. This mirrors the original MacroMarco
// Runners.Repeat continuous-spam pattern; cancellation (Esc/stop) ends it ok.
func (h Host) doRepeat(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("repeat needs a set with Key and Every")
	}
	keyVal, _ := set.Get("Key")
	everyVal, _ := set.Get("Every")
	key := keyVal.AsText()
	if key == "" {
		return fail("repeat needs a Key")
	}
	every, ok2 := everyVal.AsNumber()
	if !ok2 || every <= 0 {
		every = 50
	}
	interval := time.Duration(every) * time.Millisecond
	for {
		select {
		case <-c.Ctx.Done():
			return "ok", runtime.Absent(), nil
		default:
		}
		if err := h.b.key(c.Ctx, key); err != nil {
			return fail(err.Error())
		}
		select {
		case <-time.After(interval):
		case <-c.Ctx.Done():
			return "ok", runtime.Absent(), nil
		}
	}
}

// ── helpers ──

// ok turns a backend error into a (status, data) pair: ok on nil, failed
// (with the message) otherwise.
func ok(err error) (string, runtime.Value, error) {
	if err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: err.Error()}), nil
	}
	return "ok", runtime.Absent(), nil
}

func fail(msg string) (string, runtime.Value, error) {
	return "failed", runtime.ErrVal(&runtime.Err{Message: msg}), nil
}

// point reads X and Y number fields from a set Value. Returns present=false if
// the value isn't a set with both fields.
func point(v runtime.Value) (x, y int, present bool) {
	set := v.AsSet()
	if set == nil {
		return 0, 0, false
	}
	xv, okx := set.Get("X")
	yv, oky := set.Get("Y")
	if !okx || !oky {
		return 0, 0, false
	}
	xn, okxn := xv.AsNumber()
	yn, okyn := yv.AsNumber()
	if !okxn || !okyn {
		return 0, 0, false
	}
	return int(xn), int(yn), true
}
