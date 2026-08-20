package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What the experiment saw, and what it cost.
//
// Deliberately a SEPARATE report rather than extra lines on the perception diagnostic. The
// cheap report is answerable on any connection goroutine at any time and is meant to stay
// that way; folding an experiment's per-cycle comparison into it would make the routine
// question expensive to answer, which is exactly the kind of cost that gets noticed once and
// then blamed on the wrong thing.
//
// Every line here says SHADOW. A reader must never mistake experimental perception for what
// Marco currently believes — that is the one misreading this whole subsystem is built to
// prevent, and a diagnostic that buried the word would undo the structure that guarantees it.

// shadowReport is one session's worth of shadow observation.
type shadowReport struct {
	Detector    string         `json:"detector"`
	Cadence     string         `json:"cadence"`
	Unavailable string         `json:"unavailable,omitempty"`
	Stats       shadow.Stats   `json:"stats"`
	MedianMS    int64          `json:"median_ms"`
	P95MS       int64          `json:"p95_ms"`
	Roles       map[string]int `json:"roles"`
	Comparison  shadow.Summary `json:"comparison"`
	// Uncomparable counts cycles whose shadow evidence could not honestly be compared —
	// a superseded window generation, or a capture that could not be attributed. Reported
	// beside the comparison rather than inside it, because a session in which most cycles
	// were uncomparable has not measured information gain at all, whatever the other
	// numbers say.
	UncomparableCycles int `json:"uncomparable_cycles"`
	ComparedCycles     int `json:"compared_cycles"`
}

// shadowRegions turns a cycle's evidence into comparison regions.
//
// Roles and rectangles only. A label never crosses into the comparison layer: nameability is
// decided by the existing policy on the authoritative side, and a shadow detection that made
// a region OCR-eligible must not become a route by which withheld text arrives somewhere new.
func shadowRegions(obs []observation.Observation) []shadow.Region {
	out := make([]shadow.Region, 0, len(obs))
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		out = append(out, shadow.Region{
			Role:     el.Raw.Role,
			Bounds:   o.Bounds(),
			Nameable: nameableRole(el.Raw.Role),
		})
	}
	return out
}

// nameableRole is the privacy allowlist: which roles may be said in plaintext.
//
// This was a deliberate COPY of the same closed set, on the argument that it states what a
// detector could offer rather than what the Director may store. That argument no longer holds:
// since scoped label reading is gated on the same property, a detector role counted here as
// "nameable" and withheld by the classifier would be a number promising evidence the system
// refuses to keep. One policy now, in directorapi.
func nameableRole(r directorapi.ElementRole) bool { return r.NameablePlaintext() }

// accumulate folds one cycle's shadow evidence into a report.
func (r *shadowReport) accumulate(cycle observation.Cycle) {
	if r.Roles == nil {
		r.Roles = map[string]int{}
	}
	for _, out := range cycle.Shadow {
		// A skipped slot carries no evidence and is already counted by the provider.
		if len(out.Observations) == 0 && out.State == observation.StateEmpty {
			continue
		}
		shadowSide := shadowRegions(out.Observations)
		for _, s := range shadowSide {
			r.Roles[string(s.Role)]++
		}

		// The comparison is honest only if both sides looked at the same world. The
		// provider already established that; this reads its answer rather than
		// re-deriving one, so there is exactly one place that decides it.
		comparable := out.TargetProven()
		admitted, _ := cycle.Admitted()
		_, sum := shadow.Compare(shadowSide, shadowRegions(admitted), comparable)

		if comparable {
			r.ComparedCycles++
		} else {
			r.UncomparableCycles++
		}
		r.Comparison.Agreed += sum.Agreed
		r.Comparison.ShadowOnly += sum.ShadowOnly
		r.Comparison.AuthoritativeOnly += sum.AuthoritativeOnly
		r.Comparison.RoleDisagreement += sum.RoleDisagreement
		r.Comparison.GeometryDisagreement += sum.GeometryDisagreement
		r.Comparison.Uncomparable += sum.Uncomparable
		r.Comparison.ShadowOnlyNameable += sum.ShadowOnlyNameable
	}
}

// render prints the report.
func (r shadowReport) render(w *strings.Builder) {
	w.WriteString(fmt.Sprintf("%s · SHADOW\n", strings.ToUpper(r.Detector)))
	if r.Unavailable != "" {
		w.WriteString("  unavailable  " + r.Unavailable + "\n")
		return
	}
	w.WriteString(fmt.Sprintf("  cadence      %s\n", r.Cadence))
	w.WriteString(fmt.Sprintf("  inferences   %d   skipped %d busy / %d cadence   failures %d\n",
		r.Stats.Inferences, r.Stats.SkippedBusy, r.Stats.SkippedRate, r.Stats.Failures))
	w.WriteString(fmt.Sprintf("  latency      cold %s   median %dms   p95 %dms\n",
		r.Stats.FirstLatency.Round(1e6), r.MedianMS, r.P95MS))

	if len(r.Roles) > 0 {
		keys := make([]string, 0, len(r.Roles))
		for k := range r.Roles {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, r.Roles[k]))
		}
		w.WriteString("  detections   " + strings.Join(parts, "  ") + "\n")
	}

	c := r.Comparison
	w.WriteString(fmt.Sprintf("  comparison   agreed %d   shadow-only %d (%d nameable)   "+
		"authoritative-only %d\n", c.Agreed, c.ShadowOnly, c.ShadowOnlyNameable,
		c.AuthoritativeOnly))
	w.WriteString(fmt.Sprintf("               role-disagreement %d   geometry-disagreement %d\n",
		c.RoleDisagreement, c.GeometryDisagreement))
	w.WriteString(fmt.Sprintf("  compared     %d cycles   uncomparable %d\n",
		r.ComparedCycles, r.UncomparableCycles))
	// The last word, every time.
	w.WriteString("  authority    NONE — this evidence cannot reach belief, planning, " +
		"policy or input\n")
}
