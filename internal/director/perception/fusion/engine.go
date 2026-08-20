package fusion

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The evidence/belief boundary.
//
// This file is the only place in the Director where an Observation becomes an Element.
// Everything above it — the planner, the target resolver, policy, verification, replay,
// the action graph — sees Elements and has no way to ask where they came from beyond
// the provenance each one carries. Everything below it emits Observations and has no
// way to create an Element at all.
//
// That asymmetry is the milestone. It is what makes OCR an additive change rather than
// a rewrite: an OCR provider is a new Contributor emitting a new kind of evidence, and
// the engine is the single component that has to learn what to do with it.

// Engine converts one cycle's evidence into the Director's belief.
type Engine interface {
	Fuse(cycle observation.Cycle) (directorapi.WorldState, Report, error)
}

// engine is the default Engine.
//
// STATEFUL, and necessarily so: element identity is carried between cycles, which is
// the only reason "click that again" can mean anything. One engine belongs to one
// Director session and is not safe for concurrent use — building a speculative
// snapshot in the middle of a sequence would corrupt the identity history.
type engine struct {
	tracker *Tracker
	// identities is the one thing an explanation cannot recompute: which element each
	// cluster became, and why it kept or acquired that id. Everything else about a
	// cycle is a pure function of its evidence; identity depends on the cycle BEFORE,
	// which is gone by the time anyone asks.
	identities *identityLog
}

// NewEngine returns an Engine with a fresh identity tracker.
func NewEngine() Engine {
	return &engine{tracker: New(), identities: newIdentityLog(observation.DefaultHistory)}
}

var _ Engine = (*engine)(nil)

// Fuse builds the world from a cycle.
//
// The error return is part of the interface and is never used by this implementation,
// which is deliberate rather than an oversight. A cycle in which every provider failed
// is not an error: it is a world with no elements and a full Degraded list, and that
// distinction — "I could not see" as a first-class observable state rather than as a
// failed call — is what keeps the Director from reporting that a button is absent when
// the truth is that it never looked. Returning an error here would let a caller treat
// blindness as a transport problem and retry it forever.
func (e *engine) Fuse(cycle observation.Cycle) (directorapi.WorldState, Report, error) {
	start := time.Now()

	ts := cycle.Timestamp()
	if ts.IsZero() {
		ts = time.Now()
	}

	// The target provenance guard, and the only place it is applied.
	//
	// The request recorded intent, each provider recorded what it could prove, and this is
	// the comparison. Everything below this line works from `admitted`, never from
	// cycle.Observations — a single missed substitution would reopen the hole, so the
	// original list is deliberately not used again in this function.
	admitted, refused := cycle.Admitted()

	raw := observation.Elements(admitted)
	fused, conflicts := cluster(raw, cycle.ID, nil)

	// Text fusion runs AFTER clustering and BEFORE identity. After clustering because
	// text must never be able to create an element — there is no code path from a text
	// observation to a new cluster. Before identity because a label filled from OCR is
	// part of what makes an element recognisable next cycle, and assigning identity
	// first would match on a name the element is about to acquire.
	texts := observation.Texts(admitted)
	textDecisions := fuseText(fused, GroupLine(texts), cycle.ID, ts, nil)

	visualDecisions := fuseVisual(fused, observation.Visuals(admitted), cycle.ID, ts, nil)

	identities := e.tracker.Assign(fused, ts)
	nameTextDecisions(textDecisions, fused)
	nameVisualDecisions(visualDecisions, fused)

	// The identity log, and nothing else, is written on the hot path. It is O(elements)
	// and a few words each; the expensive part of an explanation is reconstructed on
	// demand, because clustering is deterministic and this is not.
	if cycle.ID != "" {
		byPrimary := make(map[directorapi.ObservationID]identityEntry, len(fused))
		for i, f := range fused {
			byPrimary[f.Observations[0].ID] = identityEntry{
				element: f.Element.ID, identity: identities[i],
			}
		}
		e.identities.put(cycle.ID, identityRecord{byPrimary: byPrimary})
	}

	elements := make(map[directorapi.ElementID]*directorapi.Element, len(fused))
	byObservation := make(map[directorapi.ObservationID]directorapi.ElementID, len(raw))
	for _, f := range fused {
		elements[f.Element.ID] = f.Element
		for _, ref := range f.Element.Provenance.Sources {
			byObservation[ref.Observation] = f.Element.ID
		}
	}
	// Conflicts are detected during clustering, before identity exists. Naming the
	// element they were about is only possible now.
	for i := range conflicts {
		conflicts[i].Element = byObservation[conflicts[i].Winner.Observation]
	}

	w := directorapi.WorldState{
		Timestamp:    ts,
		Windows:      observation.Windows(admitted),
		Monitors:     append([]directorapi.Monitor(nil), cycle.Environment.Monitors...),
		Elements:     elements,
		Cursor:       cycle.Environment.Cursor,
		Selection:    cycle.Environment.Selection,
		Clipboard:    cycle.Environment.Clipboard,
		Observations: raw,
		Degraded:     append([]directorapi.SourceFailure(nil), cycle.Failures...),
	}
	// Refused evidence degrades the world. Without this the guard would make a stale
	// provider's contribution disappear silently, and a confidently EMPTY world is a worse
	// failure than a stale one — it reads as "the button is not there" when the truth is
	// "I could not establish that what I saw was the right window".
	w.Degraded = append(w.Degraded, degradedBy(refused)...)

	if app, ok := observation.ActiveApplication(admitted); ok {
		detail := app.Detail
		w.ActiveApp = &detail
		if app.WindowID != "" {
			id := app.WindowID
			w.ActiveWindow = &id
		}
	}

	assignMonitors(&w)
	resolveCursorTarget(&w)
	w.Confidence = snapshotConfidence(&w)

	report := Report{
		Cycle:            cycle.ID,
		ObservationCount: len(admitted),
		ElementCount:     len(elements),
		Merged:           mergedCount(fused),
		Rejected:         rejectedCount(admitted, textDecisions),
		Conflicts:        conflicts,
		Text:             summariseText(textDecisions),
		Visual:           summariseVisual(visualDecisions),
		ByKind:           observation.CountByKind(admitted),
		BySource:         observation.CountBySource(admitted),
		Degraded:         w.Degraded,
		Provenance:       summariseProvenance(cycle, refused, len(admitted)),
		Duration:         time.Since(start),
	}
	return w, report, nil
}

// degradedBy turns refused outcomes into the world's own vocabulary for "I could not see".
func degradedBy(refused []observation.ProviderOutcome) []directorapi.SourceFailure {
	out := make([]directorapi.SourceFailure, 0, len(refused))
	for _, o := range refused {
		out = append(out, directorapi.SourceFailure{
			Source: directorapi.ObservationSource(o.Source),
			Reason: refusalReason(o),
		})
	}
	return out
}

// refusalReason says what was refused and why, in terms a person can act on.
//
// The provider's own reason is preferred where it gave one — it knows what happened. The
// fallbacks name the actual defect rather than restating the rule, because "provenance
// mismatch" tells a reader nothing they can do anything about.
func refusalReason(o observation.ProviderOutcome) string {
	if o.Reason != "" {
		return o.Reason
	}
	if !o.ObservedTarget.Known() {
		return "this provider could not establish which window its evidence came from, " +
			"so it cannot be attributed to the target"
	}
	return fmt.Sprintf(
		"this evidence describes window generation %d; the target is generation %d",
		o.ObservedTarget.WindowGeneration, o.ExpectedTarget.WindowGeneration)
}

// mergedCount is how many observations were absorbed into an element another
// observation had already established — the corroboration count.
func mergedCount(fused []Fused) int {
	n := 0
	for _, f := range fused {
		if len(f.Observations) > 1 {
			n += len(f.Observations) - 1
		}
	}
	return n
}

// rejectedCount is the evidence that produced no element and reinforced none.
//
// Text that stayed STANDALONE counts here: a page of body text with nothing structural
// under it is exactly what fusion should refuse to believe anything about, and a large
// standalone count against a small element count is the signature of an application
// whose interior the Director can read but not operate.
//
// Text that filled or reinforced a label does NOT count — it was used.
func rejectedCount(all []observation.Observation, texts []TextDecision) int {
	rejected := 0
	for _, o := range all {
		switch o.Kind() {
		case observation.ElementObservation, observation.WindowObservation,
			observation.ApplicationObservation, observation.TextObservation:
			// Consumed: elements are fused, windows and applications are carried into
			// the world directly, and text is accounted for below.
		default:
			// Anything later: nothing consumes it yet.
			rejected++
		}
	}
	for _, d := range texts {
		switch d.Outcome {
		case TextFilledMissingLabel, TextReinforcedLabel, TextSupportingEvidence:
		default:
			rejected++
		}
	}
	return rejected
}

// ── world assembly ────────────────────────────────────────────────────────────

// assignMonitors places each window on the monitor its centre falls in. Centre
// rather than overlap, because a window straddling two screens is "on" the one
// showing most of it, which is what a user means by "the other monitor".
func assignMonitors(w *directorapi.WorldState) {
	if len(w.Monitors) == 0 {
		return
	}
	for i := range w.Windows {
		if w.Windows[i].MonitorID != "" || w.Windows[i].Bounds.Empty() {
			continue
		}
		centre := w.Windows[i].Bounds.Center()
		for _, m := range w.Monitors {
			if m.Bounds.Contains(centre) {
				w.Windows[i].MonitorID = m.ID
				break
			}
		}
		// A window whose centre is off every monitor (dragged mostly off-screen, or
		// minimised to a sentinel position) keeps an empty MonitorID rather than
		// being assigned to the primary. Guessing here would send "move it to the
		// other monitor" somewhere the user is not looking.
	}
}

// resolveCursorTarget fills in which element the pointer is over — the referent of
// "this" in "click this".
//
// When elements nest, the SMALLEST containing element wins. The pointer is over a
// dialog, a pane, a group and a button all at once; the button is the one the user
// means.
func resolveCursorTarget(w *directorapi.WorldState) {
	if w.Cursor.Over != nil {
		return // a provider already knew
	}
	var best *directorapi.Element
	for _, el := range w.Elements {
		if !el.Visible || el.Bounds.Empty() || !el.Bounds.Contains(w.Cursor.Position) {
			continue
		}
		if best == nil || el.Bounds.Area() < best.Bounds.Area() {
			best = el
		}
	}
	if best != nil {
		id := best.ID
		w.Cursor.Over = &id
	}
}

// ── snapshot confidence ───────────────────────────────────────────────────────

// snapshotConfidence rates the snapshot across the five dimensions of WorldConfidence.
//
// Each answers a different question, and they are kept apart because live testing
// showed that combining them hides exactly the case that matters: an application
// reporting a handful of impeccably-observed elements, none of which can be used.
// See directorapi.WorldConfidence for why this is not one number.
func snapshotConfidence(w *directorapi.WorldState) directorapi.WorldConfidence {
	c := directorapi.WorldConfidence{
		// Computed against the snapshot's own moment. Policy re-derives it against
		// the wall clock at decision time via WorldState.ConfidenceAt.
		Freshness: 1,
	}

	if len(w.Elements) == 0 {
		// Nothing seen. An empty desktop is a legitimate observation, so quality is
		// not zero — but there is no coverage and nothing to act on, and a caller
		// must not read this as licence to conclude an element is absent.
		c.ObservationQuality = 0.5
		if len(w.Degraded) > 0 {
			c.ObservationQuality = 0
		}
		return c
	}

	total := 0.0
	structured := 0
	content, interactive, addressable, durable := 0, 0, 0, 0

	labels, autoIDs := countKeys(w)

	for _, el := range w.Elements {
		total += el.Confidence
		for _, s := range el.Sources {
			if s.Structured() {
				structured++
				break
			}
		}
		if el.Role.Content() {
			content++
		}
		acts := el.Actions()
		if acts.Interactive {
			interactive++
			if el.Addressable() {
				addressable++
			}
		}
		if identityDurable(el, labels, autoIDs) {
			durable++
		}
	}
	n := float64(len(w.Elements))

	// ── Observation quality: how much the reporting sources are worth. ───────────
	structuredShare := float64(structured) / n
	c.ObservationQuality = (total / n) * (0.6 + 0.4*structuredShare)

	// ── Coverage: did we see INTO the application, or only its shell? ────────────
	//
	// The content share is the load-bearing signal. An application that has not
	// enabled accessibility does not report an empty tree — it reports a handful of
	// anonymous containers, which looks superficially healthy. Measured live:
	// Discord 0/8 content, Chrome 0.55 (its own UI, with the entire page missing),
	// readable desktop applications above 0.6.
	c.Coverage = float64(content) / n
	// A truncated walk saw part of what exists, and an expected source that never
	// reported saw none of it. Both are missing evidence, not absence of evidence.
	for range w.Degraded {
		c.Coverage *= 0.6
	}

	// ── Actionability: of what can be operated, how much can we reach? ───────────
	//
	// A share of the INTERACTIVE elements, not of all of them, so a two-button
	// dialog scores as well as a full toolbar — small is not the same as unusable.
	// No interactive elements at all means zero: there is nothing here to act on.
	if interactive > 0 {
		c.Actionability = float64(addressable) / float64(interactive)
	}

	// ── Identity durability: would these still be recognisable after a rebuild? ──
	c.IdentityDurability = float64(durable) / n

	return c
}

// countKeys counts how many elements share each label and each authored id, scoped
// by window and role. Only a UNIQUE identifier can carry identity forward; a
// duplicated one is worse than none, because it would confidently assign the wrong
// one.
func countKeys(w *directorapi.WorldState) (labels, autoIDs map[string]int) {
	labels, autoIDs = map[string]int{}, map[string]int{}
	for _, el := range w.Elements {
		if el.Label != "" {
			labels[identityKeyOf(el, strings.ToLower(el.Label))]++
		}
		if a := attrStringOf(el, "automation_id"); a != "" {
			autoIDs[identityKeyOf(el, a)]++
		}
	}
	return labels, autoIDs
}

// identityDurable reports whether an element would still be findable after the UI is
// rebuilt — that is, whether anything about it survives the platform reissuing its
// runtime id.
//
// This is measured rather than assumed because re-observing a static window is a
// misleading test: every element matches on its runtime id, so identity looks
// perfect while the structural matcher never runs. Live, the intrinsic durability
// ranged from 84% (Notepad) to 25% (Discord) across windows that all reported 100%
// continuity between identical snapshots.
func identityDurable(el *directorapi.Element, labels, autoIDs map[string]int) bool {
	if a := attrStringOf(el, "automation_id"); a != "" && autoIDs[identityKeyOf(el, a)] == 1 {
		return true
	}
	if el.Label != "" && labels[identityKeyOf(el, strings.ToLower(el.Label))] == 1 {
		return true
	}
	return false
}

func identityKeyOf(el *directorapi.Element, value string) string {
	return string(el.WindowID) + "\x00" + string(el.Role) + "\x00" + value
}

func attrStringOf(el *directorapi.Element, key string) string {
	if el.Attributes == nil {
		return ""
	}
	s, _ := el.Attributes[key].(string)
	return s
}

// ── ordering helpers, shared with the belief-side query package ───────────────

// SortElements orders elements by window, then reading order, then ID. Exported
// because both the world queries and the diagnostics need the same order, and two
// implementations of "reading order" would eventually disagree about a toolbar.
func SortElements(out []*directorapi.Element) {
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].WindowID != out[b].WindowID {
			return out[a].WindowID < out[b].WindowID
		}
		ay, by := out[a].Bounds.Y, out[b].Bounds.Y
		// Elements within a row-height of each other count as the same row, so a
		// toolbar reads left-to-right rather than by a pixel of vertical jitter.
		if ay-by > RowTolerance || by-ay > RowTolerance {
			return ay < by
		}
		if out[a].Bounds.X != out[b].Bounds.X {
			return out[a].Bounds.X < out[b].Bounds.X
		}
		return out[a].ID < out[b].ID
	})
}

// RowTolerance (px) is the vertical slack within which two elements count as being
// on the same row for reading order.
const RowTolerance = 8
