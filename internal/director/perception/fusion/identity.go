package fusion

// Stable element identity across snapshots.
//
// This lives inside fusion because fusion OWNS semantic identity. Providers emit
// evidence and never name an element; deciding that the button seen now is the button
// seen a moment ago is the same decision as deciding that two sources are describing
// one object, made across time instead of across sources. Splitting them into separate
// packages let them disagree about what a label is, which is why they are one now.
//
// This is the quiet load-bearing piece of the whole Director. Every one of these
// works only because the Save button that exists now is recognisably the same object
// as the Save button that existed a moment ago:
//
//	Click that again.
//	Move the window back.
//	Use the same field as before.
//	Do this to all the other rows.
//
// Minting a fresh ID on every observation would make all of them impossible, and —
// worse — would make them impossible SILENTLY, since a re-resolved target still
// looks like a successful lookup.
//
// # How matching works
//
// Three tiers, strongest first, each conclusive on its own:
//
//  1. NATIVE ID. UI Automation's RuntimeId and a DOM node id are the platform's own
//     identity for a live element. When it is unchanged, continuity is settled and
//     nothing needs to be inferred.
//
//  2. AUTOMATION ID. An application-authored identifier, unique within its window.
//     Weaker moment-to-moment than a native id but stronger across RECREATION: close
//     a dialog and reopen it and every RuntimeId changes, while the AutomationId of
//     its Save button does not.
//
//  3. STRUCTURE. Window, role, label, parent and position, scored together. This is
//     the fallback for the large fraction of real applications that expose neither
//     stable id — most WinForms and Win32 dialogs among them.
//
// Matching is one-to-one and greedy over the best scores: an element can inherit at
// most one previous identity, and an identity can be inherited by at most one
// element. Without that, a row of identical buttons would all inherit the first
// one's ID and "click that again" would click the wrong one.

import (
	"sort"
	"strconv"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Tracker assigns stable element IDs across successive snapshots. The zero value is
// ready to use; it is not safe for concurrent use.
type Tracker struct {
	prev   []tracked
	nextID int
}

// tracked is what the Tracker remembers about one element from the previous
// snapshot.
type tracked struct {
	id             directorapi.ElementID
	nativeID       string
	automationID   string
	windowID       directorapi.WindowID
	role           directorapi.ElementRole
	label          string
	parentNativeID string
	bounds         directorapi.Rect
	siblingIndex   int
	firstSeen      time.Time
}

// New returns an empty Tracker.
func New() *Tracker { return &Tracker{} }

// Assign gives every element a stable ID, carrying identities forward from the
// previous snapshot where an element can be matched, and minting fresh ones
// otherwise. It also resolves ParentID from the native parent links.
//
// It MUTATES the elements in fused, and replaces the Tracker's memory with this
// snapshot. Call it exactly once per snapshot, in order.
func (t *Tracker) Assign(fused []Fused, now time.Time) []explain.IdentityExplanation {
	current := make([]tracked, len(fused))
	for i, f := range fused {
		current[i] = describe(f, i)
	}

	why := map[int]matchReason{}
	matched := t.match(current, why)

	out := make([]explain.IdentityExplanation, len(fused))
	for i := range fused {
		el := fused[i].Element
		// Durability is a property of the element, not of this match: whether it would
		// still be findable after the UI is rebuilt and every runtime id changes. An
		// element can match perfectly this cycle and be undurable, which is exactly the
		// case that makes "click that again" fail silently a minute later.
		out[i].Stable = current[i].automationID != "" || current[i].label != ""

		if prev, ok := matched[i]; ok {
			el.ID = prev.id
			el.FirstSeen = prev.firstSeen
			current[i].id = prev.id
			current[i].firstSeen = prev.firstSeen

			r := why[i]
			previous := prev.id
			out[i].MatchedPrevious = true
			out[i].Rule = r.rule
			out[i].Reason = r.reason
			out[i].PreviousElement = &previous
			out[i].Score = r.score
			continue
		}
		t.nextID++
		el.ID = directorapi.ElementID("e" + strconv.Itoa(t.nextID))
		el.FirstSeen = now
		current[i].id = el.ID
		current[i].firstSeen = now

		out[i].Rule = "new"
		out[i].Reason = "no compatible prior identity — nothing in the previous cycle " +
			"matched on a runtime id, an authored id, or structure closely enough to claim it"
		if len(t.prev) == 0 {
			out[i].Reason = "the first cycle of this session, so there was no prior identity to inherit"
		}
	}

	linkParents(fused)
	t.prev = current
	return out
}

// matchReason is which tier claimed a pair, and how convincingly.
type matchReason struct {
	rule   string
	reason string
	score  float64
}

// note records a match reason if anyone is collecting them.
func note(why map[int]matchReason, ci int, r matchReason) {
	if why == nil {
		return
	}
	why[ci] = r
}

// match pairs current elements to previous identities, one-to-one.
//
// Returns a map from current index to the previous identity it inherits. Elements
// with no entry are new.
// why records which tier claimed each pair, for the explanation layer. Recorded HERE,
// on the code path that decides, because the decision depends on the previous cycle
// and cannot be reconstructed afterwards from the current one alone.
func (t *Tracker) match(current []tracked, why map[int]matchReason) map[int]tracked {
	out := map[int]tracked{}
	if len(t.prev) == 0 {
		return out
	}

	usedPrev := make([]bool, len(t.prev))

	// Tier 1 and 2 are conclusive, so they run first and claim their pairs before
	// any scoring happens. A structural near-match must never outbid a native-id
	// match for the same previous identity.
	for ci := range current {
		if current[ci].nativeID == "" {
			continue
		}
		for pi := range t.prev {
			if usedPrev[pi] || t.prev[pi].nativeID != current[ci].nativeID {
				continue
			}
			out[ci], usedPrev[pi] = t.prev[pi], true
			note(why, ci, matchReason{rule: "native_id",
				reason: "the platform reissued the same runtime id, which settles continuity outright"})
			break
		}
	}

	uniqueNow := uniqueAutomationIDs(current)
	uniquePrev := uniqueAutomationIDs(t.prev)
	for ci := range current {
		if _, done := out[ci]; done {
			continue
		}
		key := automationKey(current[ci])
		// Only when the AutomationId identifies exactly one element on BOTH sides.
		// Toolkits reuse ids across list rows, and an ambiguous id is worse than no
		// id: it would confidently assign the wrong identity.
		if key == "" || !uniqueNow[key] || !uniquePrev[key] {
			continue
		}
		for pi := range t.prev {
			if usedPrev[pi] || automationKey(t.prev[pi]) != key {
				continue
			}
			out[ci], usedPrev[pi] = t.prev[pi], true
			note(why, ci, matchReason{rule: "automation_id",
				reason: "an application-authored id, unique on both sides — it survives the " +
					"dialog being rebuilt, where a runtime id does not"})
			break
		}
	}

	// Tier 3: score every remaining pair, then take them best-first. Sorting the
	// candidate pairs rather than greedily walking elements in order is what stops a
	// mediocre early match from stealing an identity that a later element matches
	// far better.
	type pair struct {
		ci, pi int
		score  float64
	}
	var pairs []pair
	for ci := range current {
		if _, done := out[ci]; done {
			continue
		}
		for pi := range t.prev {
			if usedPrev[pi] {
				continue
			}
			if s := structuralScore(current[ci], t.prev[pi]); s >= matchThreshold {
				pairs = append(pairs, pair{ci, pi, s})
			}
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool {
		if pairs[a].score != pairs[b].score {
			return pairs[a].score > pairs[b].score
		}
		if pairs[a].ci != pairs[b].ci {
			return pairs[a].ci < pairs[b].ci
		}
		return pairs[a].pi < pairs[b].pi
	})
	for _, p := range pairs {
		if _, done := out[p.ci]; done || usedPrev[p.pi] {
			continue
		}
		out[p.ci], usedPrev[p.pi] = t.prev[p.pi], true
		note(why, p.ci, matchReason{rule: "structural", reason: "no stable id on either side, matched on window, role, label and position", score: p.score})
	}
	return out
}

// matchThreshold is the structural score required to claim two elements are the
// same object across snapshots.
//
// Set high deliberately. A wrong carry-forward is the most dangerous error this
// package can make: "do that again" would then repeat the action on a DIFFERENT
// control while reporting complete confidence. Minting a new ID when we should have
// carried one forward merely costs the user a re-reference, and is recoverable.
const matchThreshold = 0.75

// structuralScore rates how likely two elements are the same object across
// snapshots, 0..1.
func structuralScore(now, prev tracked) float64 {
	// Hard gates. An object does not change window or change what kind of thing it
	// is; if either differs, no amount of positional similarity matters.
	if now.windowID != prev.windowID {
		return 0
	}
	if now.role != prev.role {
		return 0
	}

	score, weight := 0.0, 0.0
	add := func(w, s float64) { score += w * s; weight += w }

	// Label is the strongest structural signal: it is what the user says out loud,
	// and it is what they mean by "the same button".
	switch {
	case now.label != "" && prev.label != "":
		if now.label == prev.label {
			add(0.45, 1)
		} else {
			// A changed label usually means a changed object ("Play" → "Pause" is
			// arguably the same control, but treating it as new is the safe error).
			add(0.45, 0)
		}
	case now.label == "" && prev.label == "":
		add(0.15, 1) // both unlabelled: weakly consistent, not evidence
	default:
		add(0.45, 0) // one gained or lost its label
	}

	// Parent linkage separates the two identical "Apply" buttons that live in
	// different groups — the case label and role cannot touch.
	if now.parentNativeID != "" && prev.parentNativeID != "" {
		add(0.2, boolScore(now.parentNativeID == prev.parentNativeID))
	}

	// Position. Elements move — windows are dragged, layouts reflow — so this is a
	// supporting signal, never a gate.
	add(0.25, positionScore(now.bounds, prev.bounds))

	// Order among siblings survives a window move intact, which is exactly when
	// position is least useful.
	add(0.1, boolScore(now.siblingIndex == prev.siblingIndex))

	if weight == 0 {
		return 0
	}
	return score / weight
}

// positionScore rates positional similarity: 1 for the same place, falling off with
// distance, plus agreement on size.
func positionScore(a, b directorapi.Rect) float64 {
	if a.Empty() || b.Empty() {
		return 0
	}
	if iou := a.IoU(b); iou > 0 {
		return iou
	}
	// No overlap: the element moved. Same size in the same window is still weak
	// evidence of the same control (a dragged window, a reflowed toolbar).
	if a.Width == b.Width && a.Height == b.Height {
		return 0.3
	}
	return 0
}

func boolScore(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// describe extracts what the Tracker needs from a fused element.
func describe(f Fused, index int) tracked {
	el := f.Element
	return tracked{
		nativeID:       f.NativeID,
		automationID:   attrString(el.Attributes, "automation_id"),
		windowID:       el.WindowID,
		role:           el.Role,
		label:          normalise(el.Label),
		parentNativeID: f.ParentNativeID,
		bounds:         el.Bounds,
		siblingIndex:   index,
	}
}

// automationKey scopes an AutomationId to its window and role. A bare id is not
// unique across an application; "submit" can appear in three dialogs at once.
func automationKey(t tracked) string {
	if t.automationID == "" {
		return ""
	}
	return string(t.windowID) + "\x00" + string(t.role) + "\x00" + t.automationID
}

// uniqueAutomationIDs reports which automation keys identify exactly one element.
func uniqueAutomationIDs(all []tracked) map[string]bool {
	counts := map[string]int{}
	for _, t := range all {
		if k := automationKey(t); k != "" {
			counts[k]++
		}
	}
	unique := make(map[string]bool, len(counts))
	for k, n := range counts {
		unique[k] = n == 1
	}
	return unique
}

// linkParents resolves each element's ParentID from the native parent links, now
// that every element has an ID.
func linkParents(fused []Fused) {
	byNative := make(map[string]directorapi.ElementID, len(fused))
	for _, f := range fused {
		if f.NativeID != "" {
			byNative[f.NativeID] = f.Element.ID
		}
	}
	for _, f := range fused {
		if f.ParentNativeID == "" {
			continue
		}
		if pid, ok := byNative[f.ParentNativeID]; ok && pid != f.Element.ID {
			parent := pid
			f.Element.ParentID = &parent
		}
	}
}

func attrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if s, ok := attrs[key].(string); ok {
		return s
	}
	return ""
}

// Label normalisation is shared with clustering (see normalise in cluster.go). It has
// to be: an accelerator or ellipsis that appears in one snapshot and not the next must
// not read as a changed label, and clustering and identity disagreeing about that
// would make an element that merged one moment fail to match the next.
