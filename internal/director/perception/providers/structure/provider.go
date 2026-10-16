// Package structure publishes the generic shape proposer's output as Stage evidence.
//
// # Why this exists, and what it is careful not to claim
//
// [[Experiment-051-the-stage-two-real-applications-produced]] ran a game menu through the real
// collector and the real fusion engine and got 46 observations and **zero Props**: OCR reports
// strings, a detector below its threshold reports nothing, and accessibility reports nothing at
// all on a surface that has no tree. A Stage that can say only "nothing exists" when it cannot
// prove what everything is has stopped describing the screen.
//
// `perception/structure` already finds the drawn rectangles — it is the proposer the workflow
// stack has used all along, and on that same frame it produced the 444x56 menu row the detector
// had rejected. This is the adapter that lets it reach the Stage, and it is deliberately thin:
// it runs the existing proposer, normalises the rectangles against the window, and emits them.
// There is no second detection algorithm here and no classification of any kind.
//
// # A rectangle is not a control
//
// Every observation this emits carries `RoleUnknown` and no label. That is not caution for its
// own sake, it is the truth: the proposer answers "something was drawn here, and it was drawn as
// a filled block or as a run of marks". It cannot say whether that is a menu row, a decorative
// panel, a background or part of a photograph.
//
// Three existing rules do the rest, and none of them is touched:
//
//   - `directorapi.ActuatingSource` does not list `SourceStructure`, so an element only this
//     reported is not `Actuable`. [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s
//     licence remains necessary for any action on one.
//   - `RoleUnknown` affords nothing — `roleInvokable` and `roleSelectable` both refuse it — so
//     `Actionability.Any()` is false and `Targetable()` is false WHETHER OR NOT a licence is
//     granted. An oversized decorative region cannot become a click target by being named.
//   - fusion's text rules decide what OCR may do with these: `supporting_evidence` attaches a
//     reading as provenance without changing a field, and the smallest container wins over its
//     ancestors. Nothing here associates text with anything.
//
// # And it is asked for, never assumed
//
// Like OCR and the detector, this costs a capture and a pass over the pixels, so an ordinary
// command cycle does not run it. `Request.Includes` is the gate, and an empty `Sources` list
// means "what the providers can cheaply see", which this is not.
package structure

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ActiveWindow supplies the window to look at when a request does not name one.
type ActiveWindow func(ctx context.Context) (directorapi.Window, bool)

// Counters say why a frame produced few observations, which is otherwise indistinguishable
// from a frame containing little. The proposer's own rejection tallies are carried through.
type Counters struct {
	Accepted        int `json:"accepted"`
	Components      int `json:"components"`
	TooSmall        int `json:"too_small"`
	TooLarge        int `json:"too_large"`
	TooThin         int `json:"too_thin"`
	NotRectangular  int `json:"not_rectangular"`
	BelowHumanScale int `json:"below_human_scale"`
}

// Diagnostics is one proposal pass, described.
type Diagnostics struct {
	Available   bool                 `json:"available"`
	WindowID    directorapi.WindowID `json:"window_id,omitempty"`
	Application string               `json:"application,omitempty"`
	ImageSize   string               `json:"image_size,omitempty"`
	Counters    Counters             `json:"counters"`
	Elapsed     time.Duration        `json:"elapsed"`
	Error       string               `json:"error,omitempty"`
}

// Provider proposes drawn regions on the active window and emits them as element evidence.
type Provider struct {
	capture capture.WindowCapture
	active  ActiveWindow
	// Options are the proposer's, unchanged. Exposed so a diagnostic can vary them;
	// the default is `structure.DefaultOptions`, whose every number is documented as a
	// statement about interfaces in general rather than about one application.
	Options      structure.Options
	ProviderName string
}

func New(cap capture.WindowCapture, active ActiveWindow) *Provider {
	return &Provider{capture: cap, active: active,
		Options: structure.DefaultOptions(), ProviderName: "structure"}
}

var _ observation.Provider = (*Provider)(nil)

// Sources says which source this provider speaks for, so a request can ask for it by name.
func (p *Provider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceStructure}
}

func (p *Provider) Name() string {
	if p.ProviderName != "" {
		return p.ProviderName
	}
	return "structure"
}

// Observe emits nothing unless the request asked for this source. See the package note.
func (p *Provider) Observe(ctx context.Context, req observation.Request) (
	[]observation.Observation, error) {

	if !req.Includes(directorapi.SourceStructure) {
		return nil, nil
	}
	obs, _, err := p.Propose(ctx, req)
	return obs, err
}

// Propose performs one pass and reports what happened.
//
// Separate from Observe for the same reason the other providers split theirs: a diagnostic
// wants the counters and the collector wants only the evidence, and both must take the same
// path or the diagnostic describes a different pass from the one that runs.
func (p *Provider) Propose(ctx context.Context, req observation.Request) (
	[]observation.Observation, Diagnostics, error) {

	started := time.Now()
	diag := Diagnostics{Available: true}

	window, ok := p.window(ctx, req)
	if !ok {
		diag.Error = "no window to look at: nothing is in front, or the requested " +
			"window is gone"
		return nil, diag, fmt.Errorf("structure: %s", diag.Error)
	}
	diag.WindowID, diag.Application = window.ID, window.Application

	img, err := p.capture.CaptureWindow(ctx, window)
	if err != nil {
		diag.Error = "capture failed: " + err.Error()
		return nil, diag, fmt.Errorf("structure: capturing %s: %w", window.ID, err)
	}
	if img.Image == nil {
		diag.Error = "capture returned no image"
		return nil, diag, fmt.Errorf("structure: %s", diag.Error)
	}
	b := img.Image.Bounds()
	diag.ImageSize = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())

	res := structure.Propose(img.Image, p.Options)
	diag.Counters = Counters{
		Components: res.Components, TooSmall: res.TooSmall, TooLarge: res.TooLarge,
		TooThin: res.TooThin, NotRectangular: res.NotRectangular,
		BelowHumanScale: res.BelowHumanScale,
	}

	out := make([]observation.Observation, 0, len(res.Candidates))
	for i, c := range res.Candidates {
		// THE RECTANGLE, IN THE SAME COORDINATES EVERY OTHER SOURCE USES. The
		// proposer works in the frame's own pixel space and says so; the capture
		// knows where the frame came from, and its transform is what every other
		// provider on this window goes through.
		at := img.Transform.Apply(c.Bounds)
		if at.Empty() {
			continue
		}
		o := directorapi.Observation{
			ID:        directorapi.ObservationID(fmt.Sprintf("structure:%s:%d", window.ID, i)),
			Source:    directorapi.SourceStructure,
			Timestamp: img.CapturedAt,
			WindowID:  window.ID,
			Bounds:    at,
			// NO ROLE AND NO LABEL. The proposer found a shape; what it IS, and
			// what it is called, are claims it cannot make. See the package note.
			Role: directorapi.RoleUnknown,
			// The source's own confidence in the OBSERVATION, which for this
			// proposer is how far the region stood out from its surroundings —
			// the evidence that something was drawn rather than photographed.
			Confidence: c.Contrast,
			// AND ITS OWN EVIDENCE, carried rather than reduced to one number. A
			// reader weighing one of these wants the shape class the proposer
			// assigned, how rectangular the component was, and how many parts
			// were merged to make it.
			Attributes: map[string]any{
				"structure_kind":  string(c.Kind),
				"structure_fill":  c.Fill,
				"structure_parts": c.Parts,
			},
		}
		out = append(out, observation.NewElement(o))
	}
	diag.Counters.Accepted = len(out)
	diag.Elapsed = time.Since(started)
	return out, diag, nil
}

// window is the window the request names, or the active one.
func (p *Provider) window(ctx context.Context, req observation.Request) (
	directorapi.Window, bool) {

	if p.active == nil {
		return directorapi.Window{}, false
	}
	w, ok := p.active(ctx)
	if !ok {
		return directorapi.Window{}, false
	}
	if req.Window != nil && *req.Window != w.ID {
		return directorapi.Window{}, false
	}
	return w, true
}
