package winprovider

import (
	"context"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The window system, answering windowref's questions about right now.
//
// Every method here goes to the operating system. None of them may consult a cache, and
// there is deliberately nowhere for one to live: the whole point of the type is that its
// answers are current, and a Live that could be stale would be worse than none.

// Live reports what a handle currently is.
func (p *Provider) Live(_ context.Context, handle uintptr) (windowref.Candidate, bool) {
	w, ok := winctx.LookUpWindow(handle)
	if !ok {
		return windowref.Candidate{}, false
	}
	return candidate(w), true
}

// ProcessAlive reports whether a process currently exists.
func (p *Provider) ProcessAlive(_ context.Context, pid uint32) bool {
	return winctx.ProcessAlive(pid)
}

// Candidates lists the current top-level windows belonging to an application.
//
// Matched on the executable name, which is the identity that survives a restart — a
// relaunched program has a new process and a new window and the same image name. Titles are
// not used for matching: they change while a window lives, and two applications can share
// one.
func (p *Provider) Candidates(_ context.Context, application string) []windowref.Candidate {
	if application == "" {
		return nil
	}
	want := strings.ToLower(strings.TrimSpace(application))
	var out []windowref.Candidate
	for _, w := range offered() {
		if w.Image != "" && w.Image == want {
			out = append(out, candidate(w))
		}
	}
	return out
}

// candidate converts a platform window into windowref's view of one.
func candidate(w winctx.LiveWindow) windowref.Candidate {
	return windowref.Candidate{
		ID:          WindowID(w.Handle),
		Handle:      w.Handle,
		ProcessID:   w.ProcessID,
		Application: w.Image,
		Title:       w.Title,
		Bounds:      rect(w.Bounds),
		Foreground:  w.Foreground,
		Visible:     w.Visible,
		Minimized:   w.Minimized,
		OnScreen:    w.OnScreen,
	}
}

// WindowID renders a handle in the Director's "hwnd:<n>" form.
//
// The inverse of ParseHandle. Present so nothing else has to build the string by hand and
// get the casing or the prefix subtly wrong.
func WindowID(handle uintptr) directorapi.WindowID {
	if handle == 0 {
		return ""
	}
	digits := make([]byte, 0, 12)
	for n := handle; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return directorapi.WindowID("hwnd:" + string(digits))
}

// AllCandidates lists every capturable top-level window, whatever it belongs to.
//
// What `director windows` shows and what a --title or --process selector searches. Kept
// separate from Candidates because the common case is "windows of this application" and
// enumerating the whole desktop to answer it would be wasteful.
func (p *Provider) AllCandidates(_ context.Context) []windowref.Candidate {
	live := offered()
	out := make([]windowref.Candidate, 0, len(live))
	for _, w := range live {
		out = append(out, candidate(w))
	}
	return out
}

// offered is the live windows a person could be talking about.
//
// THE candidate chokepoint, and the only one: both `Candidates` and `AllCandidates` read the
// desktop through here, so the listing a person sees, the window a request resolves to, and every
// fuzzy match are drawn from the same set. A Marco-owned presentation surface is not one of them.
//
// # Why exclusion belongs here rather than in windowref
//
// `windowref` is about identity across time — which window is still the one we meant — and it is
// deliberately platform-neutral. Ownership is a platform fact read from a window property, so the
// platform side answers it and hands up a set that is already only the user's world.
//
// # Raw enumeration is left honest
//
// `winctx.LiveWindows` still reports Marco's own windows, and should: a diagnostic that could not
// see them could not tell you why something was excluded. The OS may know Marco is on screen. The
// question "which application do you mean?" must never be answered with one of ours.
//
// Deleting this must fail TestAnOwnedSurfaceIsNeverOfferedAsACandidate.
func offered() []winctx.LiveWindow {
	return theUsersWindows(winctx.LiveWindows(), winctx.IsOwnedSurface)
}

// theUsersWindows drops Marco's own surfaces from a live window list.
//
// Split from `offered` so the RULE can be tested without a desktop: the desktop supplies the
// windows and the ownership query, and this decides what a person could have meant.
//
// Positive ownership evidence only. A property that cannot be read is not a claim that a window is
// ours, and treating it as one would make somebody's real application disappear because a query
// failed -- which is a far worse failure than showing one of ours.
func theUsersWindows(live []winctx.LiveWindow, owned func(uintptr) bool) []winctx.LiveWindow {
	out := make([]winctx.LiveWindow, 0, len(live))
	for _, w := range live {
		if owned(w.Handle) {
			continue
		}
		out = append(out, w)
	}
	return out
}
