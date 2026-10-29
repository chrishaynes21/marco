package orchestrator

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// ── CHOOSING THE WINDOW, BEFORE ANYTHING IS RECORDED ─────────────────────────
//
// # Why this is a choice and not a guess
//
// The first version of selected-window recording LATCHED the target: the first foreground window
// that was not Marco's became the thing being recorded. It reads as reasonable — you press
// Record and go to the application — and it is not a choice. Whichever window comes forward
// first is whatever Windows does next: a terminal, a notification toast, an updater, the
// Start menu. The demonstration that followed was then admitted from a window nobody picked,
// and the person could not correct a choice they were never shown.
//
// So a window is CHOSEN, by name, before the hooks go on. Every surface asks the same question
// out of the same list: `marco learn` prints it and reads a number, the overlay passes the
// window the person was in when they summoned it, and Studio shows it in a control.

// Pickable is one window a recording could be of.
type Pickable struct {
	// HWnd and PID are the identity a recording is confined to. Both: see
	// recorder.Target.
	HWnd uintptr
	PID  uint32
	// App is the executable name, lowercased, and Title is the caption. Both are for the
	// person reading the list; neither decides anything.
	App   string
	Title string
}

// Target is what this window is as the recorder's confinement.
func (p Pickable) Target() recorder.Target {
	return recorder.Target{HWnd: p.HWnd, PID: p.PID}
}

// Label is one line of the list a person chooses from.
func (p Pickable) Label() string {
	if p.Title == "" {
		return p.App
	}
	if p.App == "" {
		return p.Title
	}
	return fmt.Sprintf("%s — %s", p.App, p.Title)
}

// windowsNow is the live window list. A variable so the choosing can be exercised against a
// known desktop; production never replaces it.
var windowsNow = winctx.LiveWindows

// foregroundNow is the live foreground reading, which is the only judgement `Pickable` makes
// about the order of the list.
//
// A SEAM FOR THE SAME REASON AS `windowsNow`, AND IT IS NEEDED WITH IT. A stated desktop under
// the REAL foreground is worse than either half alone: the handle in front belongs to no row of
// the fixture, so the swap below never happens, and a test of the ordering rule passes while the
// rule does nothing. That is how this one read first — and on a machine with no desktop at all
// it fell back to asserting that a list is a list.
var foregroundNow = winctx.ForegroundProcess

// WindowsForTest states the desktop the choosing sees, for a test in another package.
//
// Exported because `cmd/marco` has to exercise its own prompt and its own flag, and the thing
// those decide is a window list. `testing.TB` rather than a bare setter so the restore cannot
// be forgotten: a package-level seam left pointing at a fixture would make every later test in
// that binary choose out of a desktop that is not there.
func WindowsForTest(t testing.TB, windows []winctx.LiveWindow) {
	t.Helper()
	was := windowsNow
	t.Cleanup(func() { windowsNow = was })
	windowsNow = func() []winctx.LiveWindow { return windows }
}

// ForegroundForTest states which window is in front, for a test of the ordering rule.
//
// Exported beside `WindowsForTest`, and the two belong TOGETHER: see `foregroundNow` for what a
// fixture desktop under a live foreground reading silently stops testing.
func ForegroundForTest(t testing.TB, hwnd uintptr, ok bool) {
	t.Helper()
	was := foregroundNow
	t.Cleanup(func() { foregroundNow = was })
	foregroundNow = func() (uintptr, uint32, bool) { return hwnd, 0, ok }
}

// Pickable lists the windows a recording could be of, most plausible first.
//
// # What is left out, and why each one
//
// A window with no caption and no application name is left out because there is nothing to show
// somebody: a list with three blank lines in it is not a choice. Minimized and off-screen
// windows are left out because a demonstration happens in a window somebody can see. And THIS
// PROCESS is left out, with the owning surface beside it, because a recording of Marco's own
// window is never what was meant — the same exclusion `withoutOwner` has always made, moved to
// the moment where it can be a choice rather than a repair.
func (d Deps) Pickable() []Pickable {
	own := uint32(os.Getpid())
	var out []Pickable
	for _, w := range windowsNow() {
		switch {
		case w.Handle == 0 || w.ProcessID == 0:
			continue
		case w.ProcessID == own || (d.OwnerPID != 0 && w.ProcessID == d.OwnerPID):
			continue
		case w.Minimized || !w.Visible || !w.OnScreen:
			continue
		case strings.TrimSpace(w.Title) == "" && strings.TrimSpace(w.Image) == "":
			continue
		}
		out = append(out, Pickable{HWnd: w.Handle, PID: w.ProcessID,
			App: w.Image, Title: w.Title})
	}
	// THE FOREGROUND FIRST, because it is the likeliest answer and the person reading
	// this list is looking for the window they were just in.
	fg, _, ok := foregroundNow()
	if ok {
		for i, p := range out {
			if p.HWnd == fg {
				out[0], out[i] = out[i], out[0]
				break
			}
		}
	}
	return out
}

// TheWindow finds the one window a person named, out of the list above.
//
// `which` is a 1-based position in the list, a window handle in decimal, or text matched against
// the application name and the caption. AMBIGUITY IS REFUSED rather than resolved by order: two
// windows whose captions both contain "settings" is a question, and answering it by taking the
// first is how the latch chose a window nobody picked.
func (d Deps) TheWindow(which string) (Pickable, error) {
	which = strings.TrimSpace(which)
	if which == "" {
		return Pickable{}, fmt.Errorf("no window was named")
	}
	list := d.Pickable()
	if len(list) == 0 {
		return Pickable{}, fmt.Errorf("no window could be found to record")
	}
	if n, err := strconv.Atoi(which); err == nil {
		if n >= 1 && n <= len(list) {
			return list[n-1], nil
		}
		// A LARGE NUMBER IS A HANDLE, not a position off the end of the list: the
		// overlay passes the handle of the window the person was in.
		for _, p := range list {
			if uintptr(n) == p.HWnd {
				return p, nil
			}
		}
		return Pickable{}, fmt.Errorf("there is no window %d; there are %d to choose from",
			n, len(list))
	}
	lower := strings.ToLower(which)
	var hit []Pickable
	for _, p := range list {
		if strings.Contains(strings.ToLower(p.App), lower) ||
			strings.Contains(strings.ToLower(p.Title), lower) {
			hit = append(hit, p)
		}
	}
	switch len(hit) {
	case 0:
		return Pickable{}, fmt.Errorf("no window matches %q", which)
	case 1:
		return hit[0], nil
	}
	var names []string
	for _, p := range hit {
		names = append(names, strconv.Quote(p.Label()))
	}
	return Pickable{}, fmt.Errorf("%q matches %d windows (%s) — say which one",
		which, len(hit), strings.Join(names, ", "))
}

// WhyNoWindow is why a confined recording cannot start, or "" when it can.
//
// Exported because `Begin` refuses on it and a surface wants to ask BEFORE it tells somebody a
// recording has started. The refusal itself is the point: a recording that was asked to be of
// one window and does not know which one would otherwise be a recording of the desktop, which
// is the one outcome this whole slice exists to prevent, arriving silently.
func (d Deps) WhyNoWindow() string {
	if d.Watch == WholeDesktop {
		return ""
	}
	if d.Window.Chosen() {
		return ""
	}
	// SURFACE-NEUTRAL, because `Begin` is shared: the terminal names its flag, the
	// window shows a control, and neither sentence belongs in the other one.
	return "no window was chosen to record. Choose the window to record before " +
		"starting, or ask for the whole desktop instead"
}
