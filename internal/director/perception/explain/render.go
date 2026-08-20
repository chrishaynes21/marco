package explain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Rendering an explanation for a person.
//
// The organising idea: read top to bottom and you have followed the same path the
// Director took — evidence, then the merge decisions, then the fields those decisions
// settled, then identity, then confidence. Anything that reads as a bare assertion
// ("confidence 0.90") is a failure of this file, because the whole point is that
// nothing here should have to be taken on trust.

// Render writes a full account of one element.
func Render(e ElementExplanation) string {
	var b strings.Builder

	name := e.Label
	if name == "" {
		name = "(unlabelled)"
	}
	fmt.Fprintf(&b, "%s  %s  %q\n", e.ElementID, e.Role, name)
	b.WriteString(strings.Repeat("─", 64) + "\n")

	// ── evidence ──────────────────────────────────────────────────────────────
	b.WriteString("Evidence\n")
	fmt.Fprintf(&b, "  primary     %s\n", refLine(e.PrimaryObservation))
	if len(e.Supporting) == 0 {
		b.WriteString("  supporting  none — a single source, so nothing corroborated it\n")
	}
	for _, s := range e.Supporting {
		fmt.Fprintf(&b, "  supporting  %s\n", refLine(s))
	}

	if len(e.Rejected) > 0 {
		// The half of fusion that leaves no trace in the result. A duplicate element or
		// a missing one is nearly always explained here.
		b.WriteString("\nConsidered and refused\n")
		for _, r := range e.Rejected {
			label := r.Label
			if label == "" {
				label = "(unlabelled)"
			}
			fmt.Fprintf(&b, "  %-18s %s  score %.2f\n", r.Rule, truncate(label, 24), r.Score)
			fmt.Fprintf(&b, "                     %s\n", r.Reason)
		}
	}

	if len(e.MergeSteps) > 0 {
		b.WriteString("\nMerge decisions\n")
		for _, m := range e.MergeSteps {
			fmt.Fprintf(&b, "  %-10s %-18s %s\n", m.Outcome, m.Rule, m.Reason)
			if m.Against != nil {
				fmt.Fprintf(&b, "             against %s (score %.2f)\n",
					m.Against.Observation, m.Score)
			}
		}
	}

	if len(e.Fields) > 0 {
		b.WriteString("\nWhy these values\n")
		for _, f := range e.Fields {
			fmt.Fprintf(&b, "  %-8s %-24s from %s\n", f.Field, truncate(f.Value, 24), f.From.Source)
			fmt.Fprintf(&b, "           %s\n", f.Reason)
			for _, o := range f.Overruled {
				fmt.Fprintf(&b, "           overruled: %s\n", o)
			}
		}
	}

	// ── identity ──────────────────────────────────────────────────────────────
	b.WriteString("\nIdentity\n")
	id := e.IdentityReason
	if id.MatchedPrevious {
		prev := ""
		if id.PreviousElement != nil {
			prev = " (" + string(*id.PreviousElement) + ")"
		}
		fmt.Fprintf(&b, "  matched the previous cycle%s via %s\n", prev, id.Rule)
	} else {
		fmt.Fprintf(&b, "  new element (%s)\n", id.Rule)
	}
	if id.Reason != "" {
		fmt.Fprintf(&b, "  %s\n", id.Reason)
	}
	if id.Score > 0 {
		fmt.Fprintf(&b, "  structural score %.2f\n", id.Score)
	}
	if id.Stable {
		fmt.Fprintf(&b, "  durable — it would still be findable after the UI is rebuilt\n")
	} else {
		// The failure this makes visible: an element can match perfectly this cycle and
		// still lose its identity the moment the dialog is reopened, and "click that
		// again" fails silently when it does.
		fmt.Fprintf(&b, "  NOT durable — nothing about it survives the platform reissuing ids\n")
	}

	// ── confidence ────────────────────────────────────────────────────────────
	b.WriteString("\nConfidence\n")
	c := e.Confidence
	fmt.Fprintf(&b, "  base       %+.2f\n", c.Base)
	for _, contrib := range c.Contributions {
		if contrib.Delta == 0 {
			fmt.Fprintf(&b, "             %s\n", contrib.Reason)
			continue
		}
		fmt.Fprintf(&b, "  %-10s %+.2f  %s\n", contrib.Source, contrib.Delta, contrib.Reason)
	}
	fmt.Fprintf(&b, "  total       %.2f\n", c.Total)
	if !c.Consistent() {
		// Loud, because an explanation whose arithmetic does not add up is worse than
		// none: it would be believed.
		b.WriteString("  WARNING: this derivation does not sum to its own total\n")
	}
	return b.String()
}

// RenderSummary writes one line per element — the form for scanning a whole cycle.
func RenderSummary(c CycleExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Cycle %s — %d elements\n\n", c.Cycle, len(c.Elements))
	fmt.Fprintf(&b, "%-8s %-14s %-24s %-5s %-12s %s\n",
		"ID", "ROLE", "LABEL", "CONF", "IDENTITY", "EVIDENCE")

	for _, e := range c.Elements {
		label := e.Label
		if label == "" {
			label = "(unlabelled)"
		}
		identity := e.IdentityReason.Rule
		if !e.IdentityReason.MatchedPrevious && e.IdentityReason.Rule != "unknown" {
			identity = "new"
		}
		evidence := fmt.Sprintf("%d obs", 1+len(e.Supporting))
		if n := len(e.Rejected); n > 0 {
			evidence += fmt.Sprintf(", %d refused", n)
		}
		fmt.Fprintf(&b, "%-8s %-14s %-24s %.2f  %-12s %s\n",
			e.ElementID, e.Role, truncate(label, 24), e.Confidence.Total, identity, evidence)
	}

	if c.Unexplained > 0 {
		fmt.Fprintf(&b, "\n%d element(s) could not be named: the cycle has aged out of the "+
			"identity log.\n", c.Unexplained)
	}
	return b.String()
}

// RenderChain writes the pipeline an element travelled, source to replayability.
//
// The same facts as Render, arranged as a path rather than as sections. Useful when the
// question is "where did this come from" rather than "why did it win".
func RenderChain(e ElementExplanation, el *directorapi.Element) string {
	var b strings.Builder

	label := e.Label
	if label == "" {
		label = "(unlabelled)"
	}
	fmt.Fprintf(&b, "Element    %s  %s  %q\n", e.ElementID, e.Role, label)
	fmt.Fprintf(&b, "  ↓\nSource     %s\n", e.PrimaryObservation.Source)
	for _, s := range e.Supporting {
		fmt.Fprintf(&b, "           + %s\n", s.Source)
	}

	fmt.Fprintf(&b, "  ↓\nObservation %s", e.PrimaryObservation.Observation)
	if el != nil {
		fmt.Fprintf(&b, "  bounds %d,%d %dx%d",
			el.Bounds.X, el.Bounds.Y, el.Bounds.Width, el.Bounds.Height)
	}
	b.WriteString("\n")
	if el != nil {
		if auto, _ := el.Attributes["automation_id"].(string); auto != "" {
			fmt.Fprintf(&b, "           automation id %q\n", auto)
		}
	}

	fmt.Fprintf(&b, "  ↓\nFusion     %d observation(s) fused", 1+len(e.Supporting))
	if n := len(e.Rejected); n > 0 {
		fmt.Fprintf(&b, ", %d refused", n)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "  ↓\nIdentity   ")
	if e.IdentityReason.MatchedPrevious {
		fmt.Fprintf(&b, "carried forward via %s\n", e.IdentityReason.Rule)
	} else {
		fmt.Fprintf(&b, "new (%s)\n", e.IdentityReason.Rule)
	}

	// Replayability is a statement about DURABILITY, not about this cycle. An element
	// that matched perfectly a moment ago is not replayable if nothing about it would
	// survive the dialog being reopened.
	fmt.Fprintf(&b, "  ↓\nReplay     ")
	if e.IdentityReason.Stable {
		b.WriteString("replayable — it has a durable identifier to find it by again\n")
	} else {
		b.WriteString("NOT reliably replayable — it has no identifier that survives a rebuild\n")
	}
	fmt.Fprintf(&b, "  ↓\nConfidence %.2f\n", e.Confidence.Total)
	return b.String()
}

// RenderRules summarises which merge rules fired across a whole cycle.
//
// The aggregate view: not "why is this element like this" but "what is fusion actually
// doing on this desktop". A rule that never fires is either dead or a source is
// missing, and both are worth knowing.
func RenderRules(c CycleExplanation) string {
	fired := map[string]int{}
	refused := map[string]int{}
	for _, e := range c.Elements {
		for _, m := range e.MergeSteps {
			fired[m.Rule]++
		}
		for _, r := range e.Rejected {
			refused[r.Rule]++
		}
	}

	var b strings.Builder
	b.WriteString("Merge rules\n")
	for _, row := range sortedCounts(fired) {
		fmt.Fprintf(&b, "  %-18s %5d applied\n", row.key, row.n)
	}
	if len(refused) == 0 {
		b.WriteString("  (nothing was refused: with one source there is nothing to refuse)\n")
	}
	for _, row := range sortedCounts(refused) {
		fmt.Fprintf(&b, "  %-18s %5d refused\n", row.key, row.n)
	}
	return b.String()
}

type countRow struct {
	key string
	n   int
}

func sortedCounts(m map[string]int) []countRow {
	out := make([]countRow, 0, len(m))
	for k, n := range m {
		out = append(out, countRow{k, n})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].n != out[b].n {
			return out[a].n > out[b].n
		}
		return out[a].key < out[b].key
	})
	return out
}

func refLine(r directorapi.ObservationReference) string {
	s := fmt.Sprintf("%-12s %s", r.Source, r.Observation)
	if r.Kind != "" {
		s += "  (" + string(r.Kind) + ")"
	}
	return s
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
