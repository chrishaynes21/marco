package main

import (
	"context"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Deciding whether MARCO'S OWN control surface is the thing in front.
//
// # Why this has to exist
//
// Pressing "Start Learning" is a click. So is pressing Stop, and so is clicking Marco to see what
// it currently thinks. Each is a real physical press the hook sees, and none of them is part of
// the task the person is demonstrating — they are how the person operates Marco. Admitted as
// evidence they are worse than noise: the demonstration acquires an action the person never
// performed on the application, and the transition either side of it is attributed to it.
//
// Geometry does not settle it. A press outside the watched window's rectangle is already refused,
// which covers a panel sitting beside the application — but Marco's overlay is a full-screen
// surface lying OVER that window, so a press on it is inside the rectangle by construction.
//
// # How ownership is actually decided, and the honest limitation
//
// Two clauses, because Marco's surfaces are not all Marco's processes:
//
//  1. The foreground window belongs to one of Marco's own programs. Covers the overlay and any
//     native surface, and needs nothing to be registered.
//  2. The foreground window is the one a Learn session was STARTED from. Covers the control
//     centre, which is a local web page and therefore runs inside the person's own browser —
//     a window whose process is indistinguishable from any other browser window.
//
// The second clause is an inference and is stated as one: the click that started the session
// necessarily came from the window hosting the surface that sent it. It is recorded once, at that
// moment, and never guessed at afterwards.
//
// What this deliberately does NOT do is treat "the browser" as Marco. A person may well be
// demonstrating a task in a browser window, and one of them being Marco's control centre must not
// make the rest of them impossible to learn in.

// marcoPrograms are the applications that ARE Marco.
//
// Matched against the normalised application name the window layer already derives, so this is
// the same vocabulary `director windows` prints. Deliberately a small closed set: a substring
// rule would make somebody's "marco-notes.txt" window part of Marco.
var marcoPrograms = map[string]bool{
	"marco":     true,
	"director":  true,
	"overlay":   true,
	"marco-app": true,
	"web-ui":    true,
}

// surfaceOwner remembers which window, if any, is currently acting as Marco's control surface.
//
// One value, replaced rather than accumulated: there is one Learn session at a time, so there is
// one surface driving it. It is cleared when that session ends, because a browser window that
// once showed the control centre is an ordinary window again the moment it does not.
type surfaceOwner struct {
	mu sync.RWMutex
	// host is the window a UI-initiated session was started from, zero when none.
	host uintptr
}

// adopt records the window a control surface is driving Marco from.
func (o *surfaceOwner) adopt(handle uintptr) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.host = handle
}

// release forgets it. Called when the session it belonged to ends.
func (o *surfaceOwner) release() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.host = 0
}

// hosting reports the adopted window, if there is one.
func (o *surfaceOwner) hosting() (uintptr, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.host, o.host != 0
}

// ownsForeground reports whether Marco's own surface is in front right now.
//
// Answered from CURRENT platform state on every call, like everything else in the window layer:
// a cached answer would be a claim about where the person was looking a moment ago, and the whole
// value of this is that it is true at the instant an event is classified against it.
func (o *surfaceOwner) ownsForeground(ctx context.Context, p windowref.Platform) bool {
	if p == nil {
		return false
	}
	c, res, _ := windowref.Foreground(ctx, p)
	if res != windowref.Resolved {
		// Nothing resolvable in front. NOT owned: the conservative direction here is the
		// one that keeps the person's evidence, and refusing input because Marco could
		// not identify the foreground would discard a real demonstration.
		return false
	}
	return marcoOwns(c, o)
}

// marcoOwns is the decision itself, separated from the platform call so it can be tested against
// a candidate rather than against a desktop.
func marcoOwns(c windowref.Candidate, o *surfaceOwner) bool {
	if marcoPrograms[strings.ToLower(strings.TrimSpace(c.Application))] {
		return true
	}
	if o == nil {
		return false
	}
	host, ok := o.hosting()
	return ok && c.Handle != 0 && c.Handle == host
}

// surfaceOwnsForeground is the Runtime's answer to "is Marco itself in front".
//
// Lives on the Runtime because the platform does, and kept to one line so the decision stays in
// marcoOwns where it can be read and tested.
func (r *Runtime) surfaceOwnsForeground() bool {
	if r == nil {
		return false
	}
	return r.owner.ownsForeground(context.Background(), r.winPlatform)
}

// adoptRequestingSurface records the window a control surface is driving Marco from.
//
// Called when a request declares itself to have come from Marco's own UI. The window is the one
// in FRONT at that moment, and the inference is the sound one: a person pressing a button in a
// surface is looking at it, so the surface's window is the foreground window. See the note at the
// top of this file for why the control centre cannot be recognised by its process.
//
// Nothing is adopted when the foreground is already recognisably Marco's own program — there is
// nothing to learn in that case, and recording a handle would only add something to forget.
func (r *Runtime) adoptRequestingSurface(ctx context.Context) {
	if r == nil || r.winPlatform == nil {
		return
	}
	c, res, _ := windowref.Foreground(ctx, r.winPlatform)
	if res != windowref.Resolved || c.Handle == 0 {
		return
	}
	if marcoPrograms[strings.ToLower(strings.TrimSpace(c.Application))] {
		return
	}
	r.owner.adopt(c.Handle)
}
