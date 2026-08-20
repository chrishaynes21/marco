package main

import (
	"context"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/referent"
)

// Showing a person the two decisions a learn session makes before it asks them for anything.
//
// # What is grounded
//
// START, at the moment Marco settles on where you are standing, and DESTINATION, at the moment it
// works out where you ended up. Those are the two places a misunderstanding is cheap to fix and
// expensive to discover later — a wrong start is found after a demonstration, a wrong destination
// after two demonstrations and a rehearsal.
//
// # Why the frame is recorded here
//
// A referent's regions are normalised against the window rectangle they were MEASURED against, and
// `pkg/referent` refuses to convert them if the window has moved since. That check only means
// something if the frame travels with the measurement: reading the newest sampled frame at display
// time would compare the window with itself and satisfy the freshness rule by construction.
//
// [observe.VisualReferent] deliberately carries no desktop rectangle — the privacy boundary that
// TestAReferentCarriesNothingCaptured holds — so the frame is kept beside it, here, in live process
// state. Nothing is written anywhere and a restart loses it, which is correct: a learn session
// does not survive a restart either.

// learnGrounding answers "where is that screen" for the Learn coordinator.
//
// It decides nothing about Learn. It converts a screen the coordinator has already settled on
// into the regions it currently occupies, through the same resolver every other referent uses.
type learnGrounding struct {
	rt *Runtime

	mu sync.Mutex
	// frames is the window rectangle each grounded role was measured against.
	frames map[observe.ReferentRole]sampledFrame
}

var _ learn.Grounding = (*learnGrounding)(nil)

func newLearnGrounding(rt *Runtime) *learnGrounding {
	return &learnGrounding{rt: rt, frames: map[observe.ReferentRole]sampledFrame{}}
}

// Ground resolves one pinned screen to where it currently is.
func (g *learnGrounding) Ground(t observe.ShadowTotals, application string,
	state observe.ScreenStateID, role observe.ReferentRole) observe.VisualReferent {

	if g.rt == nil || g.rt.observations == nil {
		return observe.VisualReferent{Role: role, Unavailable: observe.ReferentNothingWatched}
	}
	frame, ok := g.rt.observations.lastSampledFrame()
	v := observe.ReferentForPlace(state, role, g.rt.liveGeometry(application, t))
	if ok && v.CanPoint() {
		g.mu.Lock()
		g.frames[role] = frame
		g.mu.Unlock()
	}
	return v
}

// frameFor is the rectangle a role's regions were measured against.
func (g *learnGrounding) frameFor(role observe.ReferentRole) (sampledFrame, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, ok := g.frames[role]
	return f, ok
}

// groundedView is one endpoint, ready to be shown.
//
// The same shape as a pointing view and for the same reason: a surface converts nothing, chooses
// nothing and reconstructs nothing. Boxes are empty whenever the conversion was refused, and the
// sentence says why.
type groundedView struct {
	Role  string `json:"role"`
	Label string `json:"label"`
	// Say is the Normal-mode sentence, whether or not there is anything to draw.
	Say   string `json:"say"`
	About string `json:"about,omitempty"`
	// Unavailable is the RESOLVER's reason: Director could not say where the screen is.
	// Unmappable is the MAPPER's reason: Director could say, and the conversion to this
	// desktop was refused. Two layers, two vocabularies, and a reader chasing a missing
	// highlight has to know which one stopped.
	Unavailable string `json:"unavailable,omitempty"`
	Unmappable  string `json:"unmappable,omitempty"`
	// Diagnosis is the resolver's step-by-step account, present whenever grounding ran.
	// Developer-facing; a surface showing the Normal sentence ignores it.
	Diagnosis *observe.ReferentDiagnosis `json:"diagnosis,omitempty"`

	// Current says the endpoint's presentation belongs to the phase the session is in NOW.
	// A surface draws only current endpoints and dismisses what it drew when this goes
	// false — grounding is ephemeral presentation of one decision, never durable state.
	Current bool `json:"current,omitempty"`

	Regions []referent.Norm `json:"regions,omitempty"`
	Boxes   []referent.Box  `json:"boxes,omitempty"`
}

// CanShow reports whether there is something to draw.
func (v groundedView) CanShow() bool { return len(v.Boxes) > 0 }

// groundingViews converts the session's two endpoints into desktop rectangles.
//
// # Where each frame comes from
//
// The one it was MEASURED against comes from this grounding's own record, taken at the moment the
// decision was made. The one the window is at NOW comes from a fresh platform resolution through
// the ordinary selector. `referent.Map` compares them and refuses when they differ, which is how a
// start highlighted after the user dragged the window becomes a sentence rather than a wrong box.
func (r *Runtime) groundingViews(ctx context.Context, s learn.Session,
	sel windowref.Selector) []groundedView {

	var out []groundedView
	for _, e := range s.Grounded() {
		v := groundedView{Role: string(e.Role), Label: e.Label, Say: e.Say,
			Current: e.Current}
		if e.Referent == nil {
			out = append(out, v)
			continue
		}
		v.About, v.Unavailable = e.Referent.About, string(e.Referent.Unavailable)
		d := e.Referent.Diagnosis
		v.Diagnosis = &d
		if e.Referent.CanPoint() {
			r.mapGrounded(ctx, &v, e.Referent, e.Role, sel)
		}
		out = append(out, v)
	}
	return out
}

// mapGrounded fills in the desktop rectangles, or the reason there are none.
func (r *Runtime) mapGrounded(ctx context.Context, v *groundedView,
	ref *observe.VisualReferent, role observe.ReferentRole, sel windowref.Selector) {

	g := r.learnGround
	if g == nil {
		v.Unmappable = string(referent.NoWindow)
		return
	}
	at, ok := g.frameFor(role)
	if !ok {
		v.Unmappable = string(referent.NoWindow)
		return
	}
	for _, reg := range ref.Regions {
		v.Regions = append(v.Regions,
			referent.Norm{X: reg.X, Y: reg.Y, Width: reg.Width, Height: reg.Height})
	}
	now, why := r.frameNow(ctx, sel)
	if why != "" {
		v.Unmappable = string(referent.NoWindow)
		v.Say = referent.NoWindow.Say()
		return
	}
	m := referent.Map(v.Regions,
		referent.Frame{X: at.Bounds.X, Y: at.Bounds.Y,
			Width: at.Bounds.Width, Height: at.Bounds.Height}, now)
	if !m.Drawable() {
		v.Unmappable = string(m.Reason)
		v.Say = v.Label + " — settled, but " + m.Reason.Say()
		return
	}
	v.Boxes = m.Boxes
}
