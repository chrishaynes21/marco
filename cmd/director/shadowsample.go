package main

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"strings"
)

// Turning one cycle's experimental evidence into something a session result can carry.
//
// # The comparison window
//
// Strictly within ONE cycle. The authoritative side is this cycle's admitted evidence and the
// shadow side is this cycle's shadow outcome, and neither is ever compared against a
// neighbour's. A slow inference whose result arrived late is not a disagreement about the
// interface — it is a disagreement about time, and counting it as the former would manufacture
// findings out of latency.
//
// The cadence gate already guarantees this: a shadow provider that is still working when the
// next slot arrives skips the slot rather than queueing, so a cycle either carries a fresh
// inference or carries none.

// shadowSampleFor builds the per-cycle experimental summary, or nil when nothing ran.
func shadowSampleFor(detector string, unavailable string, cycle observation.Cycle,
	frame directorapi.Rect) *observe.ShadowSample {
	if detector == "" {
		return nil
	}
	if unavailable != "" {
		// Requested and could not run. Reported every cycle so a session result can say
		// WHY it is empty — "you asked for an experiment and it never started" must never
		// read as "the experiment ran and found nothing".
		return &observe.ShadowSample{Detector: detector, Unavailable: unavailable}
	}
	if len(cycle.Shadow) == 0 {
		return &observe.ShadowSample{Detector: detector}
	}

	out := cycle.Shadow[0]
	s := &observe.ShadowSample{Detector: detector, Roles: map[string]int{}}

	// A skipped slot carries no evidence and is not a failure. Distinguished from a failed
	// one by state and reason, because collapsing the two would make the cadence look like
	// an error rate.
	skipped := len(out.Observations) == 0 &&
		out.State == observation.StateEmpty &&
		strings.Contains(out.Reason, "shadow slot skipped")
	if skipped {
		return s
	}
	s.Ran = true
	s.TargetProven = out.TargetProven()
	if out.State == observation.StateFailed || out.State == observation.StateUnavailable {
		s.Unavailable = out.Reason
	}
	if !out.StartedAt.IsZero() && !out.FinishedAt.IsZero() {
		s.LatencyMS = out.FinishedAt.Sub(out.StartedAt).Milliseconds()
	}

	// THE ONE chrome classifier, read once per sample.
	chrome := observation.ChromeIn(out.Observations)
	shadowSide := shadowRegions(out.Observations)
	s.Detections = len(shadowSide)
	// Identity-free geometry for the tracker: window-relative, normalised, no labels.
	// Absolute screen coordinates are matching evidence at most and never identity.
	//
	// Built from the OBSERVATIONS rather than from the comparison regions, because the
	// comparison layer deliberately carries roles and rectangles only and the detector's own
	// certainty is dropped on the way in. That loss was invisible until a live trace was read
	// back and every one of 56 detections reported confidence 0.00 — a diagnostic that
	// silently answers a question with a zero is worse than one that refuses to.
	if frame.Width > 0 && frame.Height > 0 {
		for _, o := range out.Observations {
			el, ok := o.(observation.Element)
			if !ok {
				continue
			}
			s.Regions = append(s.Regions, observe.ShadowRegion{
				Role:       string(el.Raw.Role),
				Region:     observe.RelativeTo(o.Bounds(), frame),
				Nameable:   nameableRole(el.Raw.Role),
				Confidence: el.Confidence(),
				// Classified HERE, the last place the accessibility
				// hierarchy still exists. A region carries no parent, so
				// identity could never work this out for itself — which is
				// exactly how a window.s own buttons ended up deciding
				// whether Settings Home was the same place twice.
				Chrome: chrome[el.Raw.NativeID],
			})
		}
	}
	for _, r := range shadowSide {
		role := string(r.Role)
		if !knownMarcoRole(r.Role) {
			s.Unknown++
			continue
		}
		s.Roles[role]++
		if r.Nameable {
			s.Nameable++
		}
	}

	// What the experiment's OWN structure said about itself, in closed-vocabulary concepts.
	//
	// THE seam this milestone exists for. Until now a shadow detection reached the session as
	// a role and a rectangle: the boxes were counted, tracked and segmented into screen
	// states, and whatever was written on them was dropped at this line. On a surface where
	// accessibility exposes nothing — the surface the detector is for — that left the semantic
	// discriminator with no possible source, which is what Experiment-009 measured.
	//
	// Gated on TargetProven, without exception. Semantic evidence from a frame that cannot be
	// shown to describe the window the cycle was about is evidence about a different world,
	// and the durable consequences of a term are larger than those of a box: a box is counted
	// this session, a term reaches cross-session identity.
	if s.TargetProven && frame.Width > 0 && frame.Height > 0 {
		s.Semantic = observe.SemanticEvidenceFrom(shadowEntities(out.Observations, frame)).
			Merge(observe.ScreenTextEvidence(shadowScreenText(out.Observations)))
	}

	// Compared only against evidence this cycle admitted, and only when the experiment can
	// prove it observed the same window generation. Provenance is not waived for being
	// experimental — comparing across generations is comparing two different worlds.
	admitted, _ := cycle.Admitted()
	_, sum := shadow.Compare(shadowSide, shadowRegions(admitted), s.TargetProven)
	s.Comparison = observe.ShadowComparison{
		Agreed:               sum.Agreed,
		ShadowOnly:           sum.ShadowOnly,
		AuthoritativeOnly:    sum.AuthoritativeOnly,
		RoleDisagreement:     sum.RoleDisagreement,
		GeometryDisagreement: sum.GeometryDisagreement,
		Uncomparable:         sum.Uncomparable,
		ShadowOnlyNameable:   sum.ShadowOnlyNameable,
	}
	return s
}

// shadowLabelBudget reads the experiment's last pass for where its reading budget went.
//
// Reaching through Inner() rather than threading counters back through the outcome: the
// counters are the vision provider's own diagnostic and belong to it, and a second copy
// travelling through the observation protocol would be one more thing to keep in step.
func shadowLabelBudget(p *shadow.Provider) observe.ShadowLabels {
	inner, ok := p.Inner().(*vision.Provider)
	if !ok {
		return observe.ShadowLabels{}
	}
	c := inner.LastDiagnostics().Counters
	return observe.ShadowLabels{
		Unsayable:   c.LabelsUnsayable,
		Ambiguous:   c.LabelsAmbiguous,
		Skipped:     c.LabelsSkipped,
		Read:        c.LabelsRead,
		Unreadable:  c.LabelsUnreadable,
		ScreenTexts: c.ScreenTextsRead,
	}
}

// shadowEntities converts the experiment's structural observations into safe snapshots.
//
// Through observe.Classify — the SAME privacy classifier the authoritative path uses, with no
// second policy and no shadow exemption. A detector's box earns a readable name exactly when
// its role is nameable, which is the same rule that decided whether it was worth reading in
// the first place, and being experimental buys no relaxation of it: a name withheld from
// belief and released to an experiment would be released, and the experiment is the thing that
// writes a durable file.
//
// The snapshots are transient. They exist for the length of this function so that
// SemanticEvidenceFrom can read them, and nothing about them is retained: the ShadowSample
// carries terms and counts, never entities and never text.
func shadowEntities(obs []observation.Observation, frame directorapi.Rect) []observe.EntitySnapshot {
	policy := observe.DefaultLabelPolicy()
	out := make([]observe.EntitySnapshot, 0, len(obs))
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		region := observe.RelativeTo(o.Bounds(), frame)
		label := observe.Classify(el.Raw.Label, el.Confidence(),
			observe.LabelContext{Role: el.Raw.Role, Sources: []string{string(o.Source())}}, policy)
		out = append(out, observe.EntitySnapshot{
			Identity:   observe.IdentityOf(el.Raw.Role, label, "", region),
			Role:       el.Raw.Role,
			Label:      label,
			Confidence: el.Confidence(),
			Region:     region,
		})
	}
	return out
}

// shadowScreenText collects the experiment's TEXT observations.
//
// Text the detector found as text, at a text region's own coordinates. It names nothing and
// becomes nothing: the strings go straight into ScreenTextEvidence, which returns concepts,
// and the caller never sees them again. There is deliberately no path by which one of these
// becomes a control — that is the rule the whole vision package is arranged around, and it is
// enforced here by there being no element to attach anything to.
func shadowScreenText(obs []observation.Observation) []string {
	var out []string
	for _, o := range obs {
		t, ok := o.(observation.Text)
		if !ok {
			continue
		}
		if s := strings.TrimSpace(t.Content.Raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// knownMarcoRole reports whether a role is in Marco's closed vision vocabulary.
//
// Anything else is counted as unknown rather than tallied under its own name. A detector's
// native word ends at the plugin boundary; a role distribution that quietly grew a `Heading`
// column would mean the boundary had leaked.
func knownMarcoRole(r directorapi.ElementRole) bool {
	switch strings.ToLower(string(r)) {
	case "button", "icon", "text", "field", "checkbox", "radio",
		"slot", "bar", "panel", "menu", "menuitem", "menu_item", "image", "tab":
		return true
	}
	return false
}

// shadowDetectorName is what the report calls the experiment, empty when none is configured.
func (r *Runtime) shadowDetectorName() string {
	if r.shadowVision == nil && r.shadowUnavailable == "" {
		return ""
	}
	return shadowVisionRequested()
}
