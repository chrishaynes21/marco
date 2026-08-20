// Package diagnostics renders what perception did.
//
// Fusion is the one place in the Director where information is deliberately destroyed:
// several accounts of a control become one element, and the losing accounts leave no
// trace in the result. That is correct, and it is also completely opaque from the
// outside — an element cannot tell you about the observation that lost to it.
//
// These renderings are the window into that. They answer the questions that are
// otherwise unanswerable without a debugger: how much evidence went in, how much
// belief came out, what merged, what disagreed, and which provider is quietly
// contributing nothing.
package diagnostics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ProviderInfo is one registered provider.
type ProviderInfo struct {
	Name string `json:"name"`
	// Sources are the sources it may emit.
	Sources []string `json:"sources"`
	// Observations is how many it contributed to the most recent cycle. A registered
	// provider sitting at zero is the thing worth noticing here.
	Observations int `json:"observations"`
	// Refiner is true for a source that decorates other evidence rather than
	// producing its own, which is why it can legitimately report zero.
	Refiner bool `json:"refiner,omitempty"`
}

// CycleInfo summarises one observation cycle. It deliberately carries counts rather
// than the observations themselves: a warm VS Code cycle is thousands of them, and a
// diagnostic that shipped all of it over the wire would be unusable.
type CycleInfo struct {
	ID          observation.CycleID `json:"id"`
	StartedAt   time.Time           `json:"started_at"`
	Duration    time.Duration       `json:"duration"`
	Count       int                 `json:"count"`
	ByKind      map[string]int      `json:"by_kind,omitempty"`
	BySource    map[string]int      `json:"by_source,omitempty"`
	Failures    []string            `json:"failures,omitempty"`
	Scoped      bool                `json:"scoped,omitempty"`
	MonitorsSet int                 `json:"monitors,omitempty"`
}

// ConflictInfo is one recorded disagreement, flattened for display.
type ConflictInfo struct {
	Field   string `json:"field"`
	Element string `json:"element,omitempty"`
	Winner  string `json:"winner"`
	Loser   string `json:"loser"`
	Values  string `json:"values,omitempty"`
}

// FusionInfo summarises the most recent fusion.
type FusionInfo struct {
	Cycle            observation.CycleID `json:"cycle,omitempty"`
	ObservationCount int                 `json:"observation_count"`
	ElementCount     int                 `json:"element_count"`
	Merged           int                 `json:"merged"`
	Rejected         int                 `json:"rejected"`
	Conflicts        []ConflictInfo      `json:"conflicts,omitempty"`
	Duration         time.Duration       `json:"duration"`
	Degraded         []string            `json:"degraded,omitempty"`
	// Text is what became of the OCR evidence, when any arrived.
	Text fusion.TextSummary `json:"text"`
}

// ObservationInfo is one observation, with what fusion did with it.
//
// The browser's row. Deliberately says what HAPPENED to the observation, not just what
// it said: an observation you cannot follow into an element (or into a rejection) is
// evidence that vanished, which is the thing this view exists to make impossible.
type ObservationInfo struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Kind   string `json:"kind"`
	Label  string `json:"label,omitempty"`
	Role   string `json:"role,omitempty"`
	Bounds string `json:"bounds,omitempty"`

	Confidence float64 `json:"confidence"`

	// MergedInto is the element it became part of, empty when it became nothing.
	MergedInto string `json:"merged_into,omitempty"`
	// Primary is whether it was the winning account of that element.
	Primary bool `json:"primary,omitempty"`
	// RefusedBy lists the elements that considered it and did not take it, with why.
	RefusedBy []string `json:"refused_by,omitempty"`

	// Resource is the object this control stands for, when the source established one:
	// the canonical path, how it was obtained, its confidence and the evidence.
	//
	// Absent for almost everything, and that absence is the answer rather than a gap —
	// a button stands for nothing. For a File Explorer item it is what lets a reader,
	// and a live-validation harness, confirm the Director is about to act on the file it
	// thinks it is BEFORE any input is sent.
	Resource *directorapi.ResourceIdentity `json:"resource,omitempty"`
}

// Perception is the whole diagnostic picture: who observed, what they produced, and
// what fusion made of it.
type Perception struct {
	Providers    []ProviderInfo `json:"providers"`
	Cycles       []CycleInfo    `json:"cycles"`
	CyclesTotal  int            `json:"cycles_total"`
	CyclesKept   int            `json:"cycles_kept"`
	Fusion       FusionInfo     `json:"fusion"`
	ProvenanceOK bool           `json:"provenance_ok"`

	// Explanation accounts for every element of the most recent cycle. Present only
	// when the caller asked for it: reconstructing it is the expensive part of this
	// package, and `status` has no use for it.
	Explanation *explain.CycleExplanation `json:"explanation,omitempty"`
	// Observations is the browser view of the most recent cycle, bounded.
	Observations []ObservationInfo `json:"observations,omitempty"`
	// ObservationsTotal is how many the cycle held, before the display bound.
	ObservationsTotal int `json:"observations_total,omitempty"`
}

// Registry is the part of a collector diagnostics needs.
type Registry interface {
	Providers() []observation.Provider
}

// maxObservationRows bounds the browser view. A warm editor reports thousands, and a
// diagnostic that shipped all of them over the wire would be unusable — the count is
// reported separately so the truncation is never mistaken for the whole picture.
const maxObservationRows = 200

// Build assembles the diagnostic picture, without explanations.
func Build(reg Registry, history *observation.History, report fusion.Report) Perception {
	return build(reg, history, report, nil)
}

// BuildWithExplanation adds the account of every element in the most recent cycle.
//
// Separate from Build because reconstructing an explanation is the expensive part of
// this package — quadratic in the observation count — and `status` runs constantly
// while `explain` runs when someone is debugging. Merging them would make the common
// path pay for the rare one.
func BuildWithExplanation(reg Registry, history *observation.History,
	report fusion.Report, ex fusion.Explainer) Perception {
	return build(reg, history, report, ex)
}

func build(reg Registry, history *observation.History, report fusion.Report,
	ex fusion.Explainer) Perception {

	out := Perception{}

	latest, hasLatest := observation.Cycle{}, false
	if history != nil {
		latest, hasLatest = history.Latest()
		out.CyclesTotal = history.Total()
		for _, c := range history.Recent() {
			out.Cycles = append(out.Cycles, describeCycle(c))
		}
		out.CyclesKept = len(out.Cycles)
	}

	perProvider := map[string]int{}
	if hasLatest {
		for _, o := range latest.Observations {
			perProvider[string(o.Source())]++
		}
	}

	if reg != nil {
		for _, p := range reg.Providers() {
			info := ProviderInfo{Name: p.Name()}
			_, info.Refiner = p.(observation.Refiner)
			for _, s := range p.Sources() {
				info.Sources = append(info.Sources, string(s))
				info.Observations += perProvider[string(s)]
			}
			out.Providers = append(out.Providers, info)
		}
	}

	out.Fusion = describeFusion(report)

	if ex != nil && hasLatest {
		cx := ex.Explain(latest)
		out.Explanation = &cx
		out.Observations, out.ObservationsTotal = browse(latest, cx)
	}
	return out
}

// browse turns a cycle into the observation browser's rows, following each observation
// forward into whatever fusion did with it.
func browse(c observation.Cycle, cx explain.CycleExplanation) ([]ObservationInfo, int) {
	// Index the explanation by observation, so each row can say where its evidence
	// ended up rather than leaving the reader to cross-reference by hand.
	type fate struct {
		element string
		primary bool
		refused []string
	}
	fates := map[string]*fate{}
	at := func(id string) *fate {
		f, ok := fates[id]
		if !ok {
			f = &fate{}
			fates[id] = f
		}
		return f
	}
	for _, e := range cx.Elements {
		f := at(string(e.PrimaryObservation.Observation))
		f.element, f.primary = string(e.ElementID), true
		for _, s := range e.Supporting {
			at(string(s.Observation)).element = string(e.ElementID)
		}
		for _, r := range e.Rejected {
			g := at(string(r.Observation.Observation))
			g.refused = append(g.refused, string(e.ElementID)+": "+r.Rule)
		}
	}

	total := len(c.Observations)
	rows := make([]ObservationInfo, 0, min(total, maxObservationRows))
	for _, o := range c.Observations {
		if len(rows) >= maxObservationRows {
			break
		}
		row := ObservationInfo{
			ID:         string(o.ID()),
			Source:     string(o.Source()),
			Kind:       string(o.Kind()),
			Confidence: o.Confidence(),
		}
		if b := o.Bounds(); !b.Empty() {
			row.Bounds = fmt.Sprintf("%d,%d %dx%d", b.X, b.Y, b.Width, b.Height)
		}
		if el, ok := o.(observation.Element); ok {
			row.Label = el.Raw.Label
			if row.Label == "" {
				row.Label = el.Raw.Text
			}
			row.Role = string(el.Raw.Role)
			// The object the control stands for, when a source established one. The
			// single most useful line in this table for a request that points at
			// something: it is the difference between "a control captioned Alpha.txt"
			// and "the file C:\tmp\live-1\Alpha.txt", and until it was here nobody
			// outside the bridge could tell which they had.
			row.Resource = el.Raw.Resource.Clone()
		}
		if f, ok := fates[row.ID]; ok {
			row.MergedInto, row.Primary, row.RefusedBy = f.element, f.primary, f.refused
		}
		rows = append(rows, row)
	}
	return rows, total
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func describeCycle(c observation.Cycle) CycleInfo {
	info := CycleInfo{
		ID:          c.ID,
		StartedAt:   c.StartedAt,
		Duration:    c.Duration(),
		Count:       len(c.Observations),
		ByKind:      map[string]int{},
		BySource:    map[string]int{},
		Scoped:      c.Request.Scoped(),
		MonitorsSet: len(c.Environment.Monitors),
	}
	for k, n := range observation.CountByKind(c.Observations) {
		info.ByKind[string(k)] = n
	}
	for s, n := range observation.CountBySource(c.Observations) {
		info.BySource[string(s)] = n
	}
	for _, f := range c.Failures {
		info.Failures = append(info.Failures, string(f.Source)+": "+f.Reason)
	}
	return info
}

func describeFusion(r fusion.Report) FusionInfo {
	info := FusionInfo{
		Cycle:            r.Cycle,
		ObservationCount: r.ObservationCount,
		ElementCount:     r.ElementCount,
		Merged:           r.Merged,
		Rejected:         r.Rejected,
		Text:             r.Text,
		Duration:         r.Duration,
	}
	for _, c := range r.Conflicts {
		info.Conflicts = append(info.Conflicts, ConflictInfo{
			Field:   c.Field,
			Element: string(c.Element),
			Winner:  string(c.Winner.Source),
			Loser:   string(c.Loser.Source),
			Values:  fmt.Sprintf("%q beat %q", c.WinnerValue, c.LoserValue),
		})
	}
	for _, d := range r.Degraded {
		info.Degraded = append(info.Degraded, string(d.Source)+": "+d.Reason)
	}
	return info
}

// ── rendering ─────────────────────────────────────────────────────────────────

// RenderObservations renders `director observations`.
func RenderObservations(p Perception) string {
	var b strings.Builder
	b.WriteString("Providers\n")
	if len(p.Providers) == 0 {
		b.WriteString("  (none registered)\n")
	}
	for _, pr := range p.Providers {
		role := ""
		if pr.Refiner {
			// Worth saying out loud. A refiner reporting zero observations is working
			// correctly; a producer reporting zero is broken, and the two look
			// identical without this.
			role = "  (refines other evidence, produces none)"
		}
		fmt.Fprintf(&b, "  %-16s %-16s %5d observations%s\n",
			pr.Name, strings.Join(pr.Sources, ","), pr.Observations, role)
	}

	if len(p.Observations) > 0 {
		// Every observation, followed FORWARD into what became of it. An observation
		// with an empty "became" column is evidence that vanished, which is the one
		// thing this view exists to make impossible to miss.
		fmt.Fprintf(&b, "\nObservations  showing %d of %d\n", len(p.Observations), p.ObservationsTotal)
		fmt.Fprintf(&b, "  %-14s %-14s %-11s %-22s %-4s %s\n",
			"ID", "SOURCE", "KIND", "LABEL", "CONF", "BECAME")
		for _, o := range p.Observations {
			label := o.Label
			if label == "" {
				label = "(unlabelled)"
			}
			became := o.MergedInto
			switch {
			case became == "":
				became = "— nothing"
			case o.Primary:
				became += " (primary)"
			default:
				became += " (supporting)"
			}
			fmt.Fprintf(&b, "  %-14s %-14s %-11s %-22s %.2f %s\n",
				truncate(o.ID, 14), o.Source, o.Kind, truncate(label, 22), o.Confidence, became)
			// The object this control STANDS FOR, when a source established one. Printed
			// under its observation rather than in a column because it is the answer to
			// a different question — not "what is this called?" but "what is it?" — and
			// because a path does not fit in twenty-two characters.
			if r := o.Resource; r.Known() {
				fmt.Fprintf(&b, "      %s\n", r.Describe())
				fmt.Fprintf(&b, "      identity  %s, confidence %.2f\n", r.Source, r.Confidence)
				for _, why := range r.Evidence {
					fmt.Fprintf(&b, "        · %s\n", why)
				}
			}
			for _, r := range o.RefusedBy {
				fmt.Fprintf(&b, "      refused by %s\n", r)
			}
		}
	}

	fmt.Fprintf(&b, "\nCycles  %d retained of %d observed\n", p.CyclesKept, p.CyclesTotal)
	if len(p.Cycles) == 0 {
		b.WriteString("  (nothing observed yet)\n")
		return b.String()
	}
	for i, c := range p.Cycles {
		marker := " "
		if i == 0 {
			marker = "*"
		}
		fmt.Fprintf(&b, "%s %-26s %6s  %5d observations  %s\n",
			marker, c.ID, dur(c.Duration), c.Count, kindSummary(c.ByKind))
		for _, f := range c.Failures {
			fmt.Fprintf(&b, "    degraded: %s\n", f)
		}
	}
	return b.String()
}

// RenderFusion renders `director fusion`.
func RenderFusion(p Perception) string {
	var b strings.Builder
	f := p.Fusion

	// The shape of the pipeline, top to bottom, because the whole point of the
	// architecture is that it IS a pipeline and each stage is countable.
	b.WriteString("Providers\n")
	for _, pr := range p.Providers {
		fmt.Fprintf(&b, "  %-16s %5d\n", pr.Name, pr.Observations)
	}
	fmt.Fprintf(&b, "        ↓\nFusion    %5d observations in\n", f.ObservationCount)
	fmt.Fprintf(&b, "        ↓\nElements  %5d\n", f.ElementCount)
	fmt.Fprintf(&b, "  merged     %3d   (observations absorbed into an element another had established)\n", f.Merged)
	fmt.Fprintf(&b, "  rejected   %3d   (evidence that produced no element and reinforced none)\n", f.Rejected)
	fmt.Fprintf(&b, "  conflicts  %3d\n", len(f.Conflicts))

	if f.Text.Any() {
		// The OCR section is laid out so the SAFETY property reads off the page: text
		// filled and reinforced labels, and nowhere is there a line saying it created
		// an element, because that number cannot be anything but zero.
		t := f.Text
		fmt.Fprintf(&b, "\nOCR text  %d observations\n", t.Observations)
		fmt.Fprintf(&b, "  filled label     %4d   (a structurally real control that had no name)\n", t.FilledLabel)
		fmt.Fprintf(&b, "  reinforced       %4d   (an independent source read the same words)\n", t.Reinforced)
		fmt.Fprintf(&b, "  standalone       %4d   (no structure under it — stays evidence, never a control)\n", t.Standalone)
		fmt.Fprintf(&b, "  rejected conflict %3d   (structural label kept; the disagreement is recorded)\n", t.RejectedConflict)
		fmt.Fprintf(&b, "  rejected ambiguous %2d   (more than one element could own it)\n", t.RejectedAmbiguous)
		fmt.Fprintf(&b, "  rejected geometry %3d\n", t.RejectedGeometry)
		fmt.Fprintf(&b, "  rejected stale    %3d\n", t.RejectedStale)
		fmt.Fprintf(&b, "  rejected scope    %3d   (different window or application)\n", t.RejectedScope)
		b.WriteString("  elements created    0   (structural, by construction — text cannot create one)\n")
	}
	for _, c := range f.Conflicts {
		fmt.Fprintf(&b, "    %-8s %-10s %s over %s: %s\n",
			c.Field, c.Element, c.Winner, c.Loser, c.Values)
	}
	for _, d := range f.Degraded {
		fmt.Fprintf(&b, "  degraded   %s\n", d)
	}
	// Fusion is routinely faster than the platform clock can measure — this milestone
	// added a pipeline, not desktop work. Rendering that as "-" would read as missing
	// data, which is the opposite of the point.
	took := dur(f.Duration)
	if f.Duration == 0 && f.ObservationCount > 0 {
		took = "<1ms (below the clock's resolution)"
	}
	fmt.Fprintf(&b, "  took       %s\n", took)

	if f.ObservationCount > 0 && f.ElementCount == 0 {
		b.WriteString("\n  Evidence arrived and no belief came out of it — fusion rejected everything.\n")
	}
	if p.ProvenanceOK {
		b.WriteString("\n  Every element records the evidence it came from.\n")
	}

	// Which rules actually fired. A rule that never fires is either dead code or a
	// missing source, and both are worth seeing without reading the clusterer.
	if p.Explanation != nil {
		b.WriteString("\n" + explain.RenderRules(*p.Explanation))
	}
	return b.String()
}

// RenderProvenance renders the evidence behind the elements of a world, for
// `director fusion --provenance`. Bounded, because a warm editor has thousands.
func RenderProvenance(w *directorapi.WorldState, limit int) string {
	var b strings.Builder
	els := make([]*directorapi.Element, 0, len(w.Elements))
	for _, el := range w.Elements {
		els = append(els, el)
	}
	fusion.SortElements(els)

	b.WriteString("Element                    provenance\n")
	shown := 0
	for _, el := range els {
		if limit > 0 && shown >= limit {
			fmt.Fprintf(&b, "  ... and %d more\n", len(els)-shown)
			break
		}
		refs := make([]string, 0, el.Provenance.Len())
		for _, r := range el.Provenance.Sources {
			refs = append(refs, fmt.Sprintf("%s/%s", r.Source, r.Observation))
		}
		label := el.Label
		if label == "" {
			label = "(" + string(el.Role) + ")"
		}
		fmt.Fprintf(&b, "  %-8s %-16s %s\n", el.ID, truncate(label, 16), strings.Join(refs, " + "))
		shown++
	}
	return b.String()
}

// AllElementsHaveProvenance reports whether every element in w records where it came
// from. The definition-of-done check, expressed as something runnable rather than as
// a claim in a document.
func AllElementsHaveProvenance(w *directorapi.WorldState) bool {
	for _, el := range w.Elements {
		if el.Provenance.Len() == 0 {
			return false
		}
	}
	return true
}

func kindSummary(byKind map[string]int) string {
	if len(byKind) == 0 {
		return ""
	}
	keys := make([]string, 0, len(byKind))
	for k := range byKind {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, byKind[k]))
	}
	return strings.Join(parts, " ")
}

func dur(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
