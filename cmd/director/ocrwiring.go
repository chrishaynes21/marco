package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Wiring the OCR runtime, and being honest when there isn't one.
//
// The Director must work without OCR. Most applications expose an accessibility tree,
// most commands need no screen reading, and tesseract is an external program a user may
// simply not have. So the absence of an OCR runtime is an ordinary, fully-supported
// state — not a startup failure, and not something to paper over.
//
// What it must NOT do is look like a successful pass that found nothing. An
// application with no text and an application the Director cannot read are opposite
// findings, and the second one is the one that should send someone to install
// tesseract.

// defaultOCRBridge locates the OCR plugin, next to the binaries like the UIA bridge.
func defaultOCRBridge() string {
	if p := os.Getenv("DIRECTOR_OCR"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(exe), "ocr.exe"),
			filepath.Join(filepath.Dir(exe), "plugins", "ocr", "ocr.exe"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	if _, err := os.Stat(filepath.Join("plugins", "ocr", "ocr.exe")); err == nil {
		return filepath.Join("plugins", "ocr", "ocr.exe")
	}
	return ""
}

// newOCREngine returns an engine and, when there isn't one, the reason.
//
// The reason is returned rather than logged because it has to reach the user through
// `director ocr`, which may run hours later in a different process. A message printed
// at startup would be gone by then.
func newOCREngine(bridgePath string) (ocr.Engine, *bridgehost.Host, string) {
	if bridgePath == "" {
		return ocr.UnavailableEngine{
			Reason: "no OCR plugin found — build plugins/ocr and set $DIRECTOR_OCR to ocr.exe",
		}, nil, "no OCR plugin found"
	}
	if _, err := os.Stat(bridgePath); err != nil {
		return ocr.UnavailableEngine{
				Reason: "the OCR plugin is not at " + bridgePath,
			}, nil,
			"the OCR plugin is not at " + bridgePath
	}
	host := bridgehost.New(bridgePath)
	return ocrclient.New(host), host, ""
}

// newCapture returns the window photographer, wired to read live window bounds.
//
// The bounds lookup is what lets the provider detect a window that MOVED during the
// capture. Without it the check silently cannot run, which is why it is wired here
// rather than left optional in practice.
func newCapture(windows *winprovider.Provider) *wincapture.Capture {
	c := wincapture.New()
	// Liveness, not geometry. Enrich would happily return the rectangle it was handed
	// for a window that no longer exists; Live asks the operating system whether there
	// is a window at all, which is the question the incident turned on.
	c.Bounds = func(id directorapi.WindowID) (directorapi.Rect, bool) {
		handle, ok := winprovider.ParseHandle(id)
		if !ok {
			return directorapi.Rect{}, false
		}
		live, ok := windows.Live(context.Background(), handle)
		if !ok || live.Bounds.Width <= 0 || live.Bounds.Height <= 0 || !live.OnScreen {
			return directorapi.Rect{}, false
		}
		return live.Bounds, true
	}
	c.Owner = func(id directorapi.WindowID) (string, bool) {
		handle, ok := winprovider.ParseHandle(id)
		if !ok {
			return "", false
		}
		live, ok := windows.Live(context.Background(), handle)
		if !ok || live.Application == "" {
			return "", false
		}
		return live.Application, true
	}
	return c
}

// activeWindow is the window OCR and vision read when a request does not name one.
//
// The CANDIDATE comes from the last observed world, because text has to be fused against
// elements from a world and reading a different window from the one those elements came
// from would compare things that were never on screen together.
//
// The candidate is then VALIDATED against the live desktop before anybody photographs it,
// and that is the part that was missing. The world is a snapshot; a window in it may have
// been destroyed since, and its rectangle is then a description of where something used to
// be. Rocket League was closed and relaunched, the snapshot kept naming the dead handle,
// and the capture path pointed a camera at its old coordinates on another monitor.
//
// The bounds returned here are the platform's current ones, never the snapshot's.
func (r *Runtime) activeWindow(ctx context.Context) (directorapi.Window, bool) {
	// An explicitly selected window wins outright. Nothing about focus, and nothing the
	// last observed world believes, may override a window the user named.
	if pinned := r.pinnedWindow; pinned != nil {
		return directorapi.Window{
			ID: pinned.ID, Application: pinned.Application,
			Title: pinned.Title, Bounds: pinned.Bounds,
		}, true
	}
	candidate, ok := r.candidateWindow()
	if !ok {
		return directorapi.Window{}, false
	}
	if r.winTracker == nil {
		// No tracker wired (tests, and non-Windows builds). The candidate is returned
		// unvalidated, exactly as it was before — which is why the tracker is wired at
		// the composition root rather than left optional in production.
		return candidate, true
	}

	r.proposeWindow(candidate)
	v := r.winTracker.Acquire(ctx, candidate.Application)
	if !v.State.OK() {
		return directorapi.Window{}, false
	}
	// Everything geometric comes from validation; only the semantic fields are carried
	// across from the snapshot.
	out := candidate
	out.ID = v.Ref.ID
	out.Bounds = v.Ref.Bounds
	if v.Ref.Application != "" {
		out.Application = v.Ref.Application
	}
	if v.Ref.Title != "" {
		out.Title = v.Ref.Title
	}
	return out, true
}

// candidateWindow is the window the last observed world suggests.
func (r *Runtime) candidateWindow() (directorapi.Window, bool) {
	r.diagMu.RLock()
	w := r.lastWorld
	r.diagMu.RUnlock()
	if w == nil {
		return directorapi.Window{}, false
	}
	if win, ok := w.FocusedWindow(); ok && win.Bounds.Width > 0 {
		return *win, true
	}
	if len(w.Windows) > 0 && w.Windows[0].Bounds.Width > 0 {
		return w.Windows[0], true
	}
	return directorapi.Window{}, false
}

// OCRUnavailable is why OCR cannot run, empty when it can.
func (r *Runtime) OCRUnavailable() string { return r.ocrUnavailable }

// ReadText runs one OCR pass for diagnostics and returns the evidence it produced.
//
// It does NOT execute anything and cannot: it returns observations, and an observation
// has no way to become an action. That is the whole reason `director ocr` is safe to
// run against any window.
func (r *Runtime) ReadText(ctx context.Context, region *directorapi.Rect) ocr.Diagnostics {
	if r.ocr == nil {
		return ocr.Diagnostics{Engine: "ocr", Available: false,
			Unavailable: "no OCR provider is configured"}
	}
	// A plain observation first, so the OCR provider knows which window is in front —
	// it reads the LAST OBSERVED world rather than asking the platform, so that text
	// and structure come from the same moment.
	if _, err := r.pipeline.Observe(ctx); err != nil {
		return ocr.Diagnostics{Engine: "ocr", Available: false,
			Unavailable: "could not observe before reading: " + err.Error()}
	}

	// Then a FULL cycle including OCR, through the ordinary collector and engine.
	//
	// Not a bare provider call, which would produce text nothing ever fused. Going
	// round the normal loop is what makes `director ocr` show the same thing the
	// Director would see, and what leaves the cycle in the history for `director
	// fusion` and `director explain` to account for afterwards.
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ocr.Invalidate()

	cycle := r.collector.Collect(ctx, observation.WithOCR(region))
	if w, report, err := r.engine.Fuse(cycle); err == nil {
		r.record(cycle, report, &w)
	}
	return r.ocr.LastDiagnostics()
}

// proposeWindow offers the world's candidate to the tracker for validation.
//
// The handle is parsed here because the "hwnd:<n>" spelling is a platform detail and
// windowref deliberately knows nothing about it.
func (r *Runtime) proposeWindow(w directorapi.Window) {
	handle, ok := winprovider.ParseHandle(w.ID)
	if !ok {
		return
	}
	r.winTracker.Propose(windowref.Ref{
		ID: w.ID, Handle: handle, Application: w.Application,
		Title: w.Title, Bounds: w.Bounds,
	})
}
