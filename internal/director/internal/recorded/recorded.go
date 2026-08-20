package recorded

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The recorded-evidence path.
//
// A test has observations from a file rather than from a live provider, and it needs a
// world to reason about. This is how it gets one — by building the same observation
// cycle a collector would, and handing it to the same fusion engine. Nothing here
// decides what any of the evidence MEANS.
//
// It lives in its own package rather than in world for a reason worth stating: it
// is the only thing outside internal/director/perception and the composition root that
// touches observations at all, and putting it here keeps the rule enforceable — no
// package downstream of fusion may see evidence. See perception_boundary_test.go.

// Perception is recorded evidence for one moment, in the shape a fixture stores it.
//
// This is the RECORDED path: observations that came from a file rather than from a
// live provider. The live path runs perception/providers and produces an
// observation.Cycle directly, and the two converge on the same fusion engine — which
// is the point. A fixture that was fused by different code from the desktop would be
// testing something other than the Director.
type Perception struct {
	Timestamp    time.Time
	Observations []directorapi.Observation
	Windows      []directorapi.Window
	Monitors     []directorapi.Monitor
	ActiveApp    *directorapi.Application
	ActiveWindow *directorapi.WindowID
	Cursor       directorapi.CursorState
	Selection    *directorapi.SelectionState
	Clipboard    *directorapi.ClipboardState
	// Degraded lists sources that were expected and did not report.
	Degraded []directorapi.SourceFailure
}

// Cycle turns recorded evidence into an observation cycle.
//
// Every field becomes what it actually is: element reports become element evidence,
// windows become window evidence, the active application becomes an application
// observation. Nothing here decides what any of it MEANS — that is the engine's, and
// this function exists so a fixture reaches the engine by the same road the desktop
// does.
func (p Perception) Cycle() observation.Cycle {
	ts := p.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	c := observation.Cycle{
		ID:          observation.NewCycleID(ts),
		StartedAt:   ts,
		CompletedAt: ts,
		Failures:    p.Degraded,
		Environment: observation.Environment{
			Monitors:  p.Monitors,
			Cursor:    p.Cursor,
			Selection: p.Selection,
			Clipboard: p.Clipboard,
		},
	}

	for _, o := range p.Observations {
		if o.Timestamp.IsZero() {
			o.Timestamp = ts
		}
		c.Observations = append(c.Observations, observation.NewElement(o))
	}
	for _, w := range p.Windows {
		c.Observations = append(c.Observations, observation.Window{
			ObservationID: directorapi.ObservationID("fixture:win:" + string(w.ID)),
			From:          directorapi.SourceWindowSystem,
			At:            ts,
			Detail:        w,
		})
	}
	if p.ActiveApp != nil {
		app := observation.Application{
			ObservationID: directorapi.ObservationID("fixture:app"),
			From:          directorapi.SourceWindowSystem,
			At:            ts,
			Detail:        *p.ActiveApp,
			Active:        true,
		}
		if p.ActiveWindow != nil {
			app.WindowID = *p.ActiveWindow
		}
		c.Observations = append(c.Observations, app)
	}
	return c
}

// Builder turns recorded Perception into WorldState, carrying element identity
// between successive calls.
//
// A thin adapter over the fusion engine, not a second way to build a world: it holds
// one Engine and hands it cycles. Stateful for the engine's reason — identity is
// carried forward — and so not safe for concurrent use; one Builder belongs to one
// Director session.
type Builder struct {
	engine fusion.Engine
}

// NewBuilder returns a Builder over a fresh fusion engine.
func NewBuilder() *Builder { return &Builder{engine: fusion.NewEngine()} }

// Build produces the next world snapshot from recorded evidence.
//
// Call it once per observation cycle, in order: identity is carried forward from the
// previous call, so building out of order (or building a speculative snapshot in the
// middle of a sequence) corrupts the element history that "do that again" relies on.
func (b *Builder) Build(p Perception) directorapi.WorldState {
	w, _, _ := b.engine.Fuse(p.Cycle())
	return w
}
