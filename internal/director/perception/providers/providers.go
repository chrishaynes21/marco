// Package providers holds the Director's perception sources and the collector that
// runs them.
//
// A provider's entire job is "here is what I saw". It does not assign element ids,
// does not decide that two of its reports are the same object, and cannot construct a
// WorldState — the fusion engine owns all three. What a provider knows about is its
// own source: how to talk to the accessibility bridge, how to ask the window manager
// for monitor bounds, and one day how to run OCR over a rectangle.
//
// Today exactly one source produces element evidence. That is not a reason to skip the
// separation; it is the reason to do it now, while there is one implementation to keep
// honest instead of four.
package providers

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// obsSeq numbers observations that arrive without an id of their own.
var obsSeq atomic.Int64

// mintID returns a fresh observation id with the given prefix.
func mintID(prefix string) directorapi.ObservationID {
	return directorapi.ObservationID(fmt.Sprintf("%s:%d", prefix, obsSeq.Add(1)))
}

// ── accessibility ─────────────────────────────────────────────────────────────

// AccessibilitySource is the bridge the accessibility provider talks to. An interface
// so the provider can be tested against recorded snapshots without a desktop, and so
// the platform client stays on the platform side of the boundary test.
type AccessibilitySource interface {
	Snapshot(ctx context.Context, scope directorapi.WindowID) (directorapi.AccessibilitySnapshot, error)
}

// Accessibility turns the OS accessibility tree into evidence.
//
// It used to be the world model. The whole of this milestone, from the perspective of
// this file, is that it now returns observations and stops — what those observations
// mean, whether two of them are one control, and which element they become is decided
// somewhere it cannot reach.
type Accessibility struct {
	src AccessibilitySource
	// resolver turns the window the bridge says it walked into Director provenance.
	// Optional — see WithTargetResolver.
	resolver TargetResolver
}

// NewAccessibility wraps an accessibility source.
func NewAccessibility(src AccessibilitySource) *Accessibility {
	return &Accessibility{src: src}
}

var _ observation.Provider = (*Accessibility)(nil)

func (a *Accessibility) Name() string { return "accessibility" }

func (a *Accessibility) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceAccessibility}
}

// Observe walks the tree and reports what it found.
//
// Region is ignored, and honestly so: a tree walk enumerates objects and has no notion
// of a rectangle to restrict itself to. A provider that silently pretended to honour
// the narrowing would be worse than one that admits it cannot — the caller would
// believe it had scoped the work.
func (a *Accessibility) Observe(ctx context.Context, req observation.Request) ([]observation.Observation, error) {
	scope := directorapi.WindowID("")
	if req.Window != nil {
		scope = *req.Window
	}
	snap, err := a.src.Snapshot(ctx, scope)
	if err != nil {
		return nil, err
	}

	out := a.observationsFrom(snap)
	if snap.Partial {
		// A truncated walk is not a failure — the evidence collected is real — but it
		// must not be read as "the Save button does not exist". Reporting it as a
		// degradation is what keeps those two apart all the way to the confidence
		// score a policy gate reads.
		return out, &Degraded{
			Source: directorapi.SourceAccessibility,
			Reason: snap.Reason,
		}
	}
	return out, nil
}

// observationsFrom converts one snapshot into evidence.
//
// Shared by Observe and ObserveTargeted so the two paths cannot drift: a targeted cycle
// and an ordinary one must see the same tree described the same way.
func (a *Accessibility) observationsFrom(snap directorapi.AccessibilitySnapshot) []observation.Observation {
	at := snap.Timestamp
	if at.IsZero() {
		at = time.Now()
	}

	out := make([]observation.Observation, 0, len(snap.Observations)+len(snap.Windows)+1)
	for _, o := range snap.Observations {
		if o.ID == "" {
			o.ID = mintID("acc")
		}
		if o.Timestamp.IsZero() {
			o.Timestamp = at
		}
		out = append(out, observation.NewElement(o))
	}

	// Windows are evidence too, and this is the source that enumerates them — the
	// accessibility bridge sees the window list as part of the same walk. When the
	// window system becomes an enumerator in its own right, it emits these as well and
	// fusion merges the two accounts. Nothing here changes on that day.
	for _, w := range snap.Windows {
		out = append(out, observation.Window{
			ObservationID: mintID("win"),
			From:          directorapi.SourceAccessibility,
			At:            at,
			Detail:        w,
		})
	}

	// The foreground application. The bridge reports the focused window first, which
	// is the only claim about which application is in front that anything currently
	// makes.
	if len(snap.Windows) > 0 {
		front := snap.Windows[0]
		out = append(out, observation.Application{
			ObservationID: mintID("app"),
			From:          directorapi.SourceAccessibility,
			At:            at,
			Detail: directorapi.Application{
				ID: front.Application, Name: front.Application,
			},
			Active:   true,
			WindowID: front.ID,
		})
	}

	// Partiality is reported by the CALLER, which knows whether it is returning an
	// error (Observe) or a provider state (ObserveTargeted). Converting evidence and
	// judging its completeness are different jobs.
	return out
}

// Degraded is a partial result: evidence was collected, and some was missed.
//
// An error type rather than a second return value because the collector already has to
// handle a provider that failed outright, and "partly failed" belongs on the same path.
// Errors.As distinguishes them.
type Degraded struct {
	Source directorapi.ObservationSource
	Reason string
}

func (d *Degraded) Error() string {
	if d.Reason == "" {
		return string(d.Source) + " reported partial results"
	}
	return string(d.Source) + ": " + d.Reason
}

// ── window system ─────────────────────────────────────────────────────────────

// WindowSource is the window-system client the provider talks to.
type WindowSource interface {
	Monitors(ctx context.Context) ([]directorapi.Monitor, error)
	Enrich(windows []directorapi.Window) []directorapi.Window
}

// WindowEnumerator is the half of a window source that can walk the desktop itself.
//
// OPTIONAL, like `observation.Refiner`: `WindowSource` is what every backend must do -- the
// display layout, and detail added to windows somebody else found -- and this is what a
// backend that can list the desktop may additionally offer. A stub platform implements the
// first and not the second, and the provider falls back to refining rather than failing to
// compile.
type WindowEnumerator interface {
	AllCandidates(ctx context.Context) []windowref.Candidate
}

// WindowSystem contributes window and display detail, and enumerates windows when asked.
//
// It REFINES on every cycle: the accessibility bridge reports the window list as a side
// effect of walking the tree, and this adds the bounds, state and monitor placement an
// accessibility tree does not know. That is the production path, and it is unchanged.
//
// It ENUMERATES only when a request names this source, which is the day the paragraph that
// used to sit here anticipated. Opt-in not out of caution but because there is normally
// nothing to add: production already hears about every window from the walk, so a second
// account of each on every cycle would be work for nothing, and fusion would spend the day
// merging two descriptions of one window.
//
// What makes the opt-in LOAD-BEARING is the case it was written for. Where accessibility is
// not collected at all the walk is not happening, and then this is the only thing that knows
// a window exists — and every pixel provider is scoped to a window, so without it there
// is no Stage to look at rather than merely a thinner one. See `cmd/director/pixelonly.go`.
type WindowSystem struct {
	src WindowSource
}

// NewWindowSystem wraps a window-system client.
func NewWindowSystem(src WindowSource) *WindowSystem { return &WindowSystem{src: src} }

var (
	_ observation.Provider            = (*WindowSystem)(nil)
	_ observation.EnvironmentProvider = (*WindowSystem)(nil)
	_ observation.Refiner             = (*WindowSystem)(nil)
)

func (w *WindowSystem) Name() string { return "window_system" }

func (w *WindowSystem) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceWindowSystem}
}

// Observe enumerates the desktop's top-level windows, and only when asked by name.
//
// The gate is `Includes` rather than `Wants`, which is the gate ADR-203 put on the shape
// proposer and for the same reason: an empty `Sources` list means "everything the providers
// can cheaply see", and being PERMITTED to run is not being ASKED to. So
// `observation.Request{}` — every production cycle — gets exactly what it got before
// this existed, which is nothing, and the window evidence production fuses still comes from
// the accessibility walk alone.
//
// A source that cannot walk the desktop says so by not implementing `WindowEnumerator`, and
// then this answers nothing. That is not a silent failure: a request that asked for this
// source and got no windows produces a world with no windows in it, which every consumer
// downstream already reports rather than papers over.
//
// Deleting the gate must fail TestTheWindowSystemEnumeratesNothingUnlessAsked.
func (w *WindowSystem) Observe(ctx context.Context, req observation.Request) (
	[]observation.Observation, error) {

	if !req.Includes(directorapi.SourceWindowSystem) {
		return nil, nil
	}
	walker, ok := w.src.(WindowEnumerator)
	if !ok {
		return nil, nil
	}
	at := time.Now()
	var out []observation.Observation
	for i, c := range walker.AllCandidates(ctx) {
		out = append(out, observation.Window{
			ObservationID: mintID("win"),
			From:          directorapi.SourceWindowSystem,
			At:            at,
			Detail: directorapi.Window{
				ID:          c.ID,
				Application: c.Application,
				Title:       c.Title,
				Bounds:      c.Bounds,
				Focused:     c.Foreground,
				Visible:     c.Visible,
				Minimized:   c.Minimized,
				// The desktop enumerates front to back, so the position IS the
				// z-order. Refine supplies the rest of the geometry and does not
				// supply this.
				ZOrder: i,
			},
		})
		// THE FOREGROUND APPLICATION, a separate claim from the window and the only thing
		// that sets `WorldState.ActiveWindow`. The platform answers it per window, so
		// exactly one candidate carries it, and a desktop with nothing in front produces
		// no claim at all rather than a guess at one.
		if !c.Foreground {
			continue
		}
		out = append(out, observation.Application{
			ObservationID: mintID("app"),
			From:          directorapi.SourceWindowSystem,
			At:            at,
			Detail: directorapi.Application{
				ID: c.Application, Name: c.Application,
				ProcessID: int(c.ProcessID),
			},
			Active:   true,
			WindowID: c.ID,
		})
	}
	return out, nil
}

// Refine adds window-system detail to window evidence.
func (w *WindowSystem) Refine(obs []observation.Observation) []observation.Observation {
	// Collect the window evidence, enrich it in one call, and put it back. One call
	// because Enrich resolves handles in a batch; doing it per observation would turn
	// one platform round trip into dozens.
	var windows []directorapi.Window
	var at []int
	for i, o := range obs {
		if win, ok := o.(observation.Window); ok {
			at = append(at, i)
			windows = append(windows, win.Detail)
		}
	}
	if len(windows) == 0 {
		return obs
	}
	enriched := w.src.Enrich(windows)
	for n, i := range at {
		if n >= len(enriched) {
			break
		}
		win := obs[i].(observation.Window)
		win.Detail = enriched[n]
		obs[i] = win
	}
	return withoutOwnedSurfaces(obs, w.src)
}

// withoutOwnedSurfaces drops Marco's own presentation surfaces from the evidence.
//
// THE exclusion, and it is a removal rather than a relabelling: a window Director can see is one
// it can target, resolve against, and describe a place in terms of. A pointer Marco can see is not
// a pointer — it is part of the scene it is pointing at.
//
// Done HERE, after enrichment, and not inside `Enrich`. `Enrich` is one-to-one and its results are
// mapped back onto the observations by POSITION, so dropping an entry there does not remove a
// window: it shifts every later one onto the wrong observation and silently mislabels the rest of
// the desktop. That was written, run, and caught by a live check; this is where a filter belongs.
//
// A source that cannot report ownership is left alone, which is the safe direction — the cost of
// missing one of ours on such a platform is contamination by a surface that does not exist there,
// and the cost of guessing is somebody's real window disappearing.
//
// Deleting this must fail TestAMarcoOwnedSurfaceNeverReachesTheEvidence.
func withoutOwnedSurfaces(obs []observation.Observation,
	src WindowSource) []observation.Observation {

	owner, ok := src.(interface {
		Owned(directorapi.WindowID) bool
	})
	if !ok {
		return obs
	}
	out := make([]observation.Observation, 0, len(obs))
	for _, o := range obs {
		if win, isWindow := o.(observation.Window); isWindow && owner.Owned(win.Detail.ID) {
			continue
		}
		out = append(out, o)
	}
	return out
}

// Environment reports the display layout.
func (w *WindowSystem) Environment(ctx context.Context) observation.Environment {
	env := observation.Environment{}
	if mons, err := w.src.Monitors(ctx); err == nil {
		env.Monitors = mons
	}
	return env
}
