package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Converting a World State into something a session may keep.
//
// This is the privacy boundary and the identity boundary at once, and it is a NARROWING:
// everything the Director knows arrives, and only a bounded safe subset leaves.
//
// Dropped deliberately, every time:
//
//	ElementID, RuntimeId, native provider ids   unstable between cycles, and meaningless
//	                                            to anyone reading a report
//	window handles                              ephemeral platform references
//	desktop coordinates                         change when the window moves, and describe
//	                                            somebody's monitor layout
//	raw OCR text                                whatever happened to be on screen, which in
//	                                            a game includes account names and chat
//
// What survives is role, a safely-classified label, confidence, provenance, and geometry
// expressed relative to the window — which is the only form that still compares after the
// player drags the game to the other monitor.

// buildSample converts one fused world into one safe sample.
func buildSample(world directorapi.WorldState, cycle observation.Cycle,
	window directorapi.Window, req observesession.SampleRequest) observe.Sample {

	policy := observe.DefaultLabelPolicy()
	frame := window.Bounds

	sample := observe.Sample{
		Sequence:         req.Sequence,
		Timestamp:        time.Now(),
		WindowGeneration: req.Window.Generation,
		Frame: observe.FrameSummary{
			Application:      req.Window.Application,
			WindowGeneration: req.Window.Generation,
			Width:            frame.Width,
			Height:           frame.Height,
		},
		Providers: providerSummaries(cycle),
	}

	// Deterministic order, so two identical scenes produce identical samples and a
	// fixture replays byte-for-byte.
	ids := make([]directorapi.ElementID, 0, len(world.Elements))
	for id := range world.Elements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// WHICH OF THESE BELONG TO THE WINDOW RATHER THAN TO THE PAGE.
	//
	// Asked here because this is the last place the hierarchy exists: an EntitySnapshot
	// carries a role and a rectangle, and the durable signature built from it could never
	// work out that a button is one of a scroll bar.s arrows. The same classifier
	// `director inspect -chrome` reports with.
	//
	// Deleting this must fail TestAScrollBarsArrowsAreNotPartOfThePlace.
	nodes := make([]observation.ChromeNode, 0, len(ids))
	for _, id := range ids {
		el := world.Elements[id]
		parent := ""
		if el.ParentID != nil {
			parent = string(*el.ParentID)
		}
		nodes = append(nodes, observation.ChromeNode{
			ID: string(id), Parent: parent, Role: el.Role,
		})
	}
	chrome := observation.Chrome(nodes)

	for _, id := range ids {
		el := world.Elements[id]
		if el.WindowID != "" && window.ID != "" && el.WindowID != window.ID {
			// Belongs to another window. A session describes ONE window; folding in a
			// neighbour's elements would make the timeline unreadable.
			continue
		}
		snapshot, ok := safeEntity(el, frame, policy)
		snapshot.Chrome = chrome[string(id)]
		if !ok {
			continue
		}
		if grid, isGrid := safeGrid(el, frame); isGrid {
			sample.Grids = append(sample.Grids, grid)
			continue
		}
		sample.Entities = append(sample.Entities, snapshot)
	}
	// THE authoritative composition. The same elements, reduced to what a screen is made
	// of — a role and a window-relative region — and nothing else.
	//
	// Deleting this line returns the Director to having no screen model unless the
	// experimental detector is switched on, which is the live blocker this milestone was
	// about. See TestFusedStructureReachesTheScreenModel.
	sample.Structure = fusedStructure(sample.Entities, cycle, frame, world.Confidence)
	return sample
}

// fusedStructure is the composition the authoritative world presented this cycle.
//
// # Why it is not simply "the entities"
//
// Because a screen is a composition, and a composition is role and place. Everything else an
// entity carries — its label, its identity, its capabilities — belongs to other questions,
// and handing it to segmentation would let a screen's identity depend on what a control was
// CALLED. `ScreenSignature` reads role and region; this hands it exactly that.
//
// # Why an empty result is still an observation
//
// If a provider proved its target and reported nothing structural, the screen is genuinely
// empty — the sparse case an application can legitimately present — and that is a state
// which recurs. If NOTHING proved its target, nothing looked, and minting a state from that
// would invent a screen out of silence ([[ADR-006-unknown-is-not-false]]). The two are
// distinguished here, where the provider outcomes are in scope, and never downstream.
func fusedStructure(entities []observe.EntitySnapshot, cycle observation.Cycle,
	frame directorapi.Rect, confidence directorapi.WorldConfidence) observe.StructuralView {

	if frame.Width <= 0 || frame.Height <= 0 {
		return observe.StructuralView{
			Why: "the window had no usable bounds, so nothing could be placed on it"}
	}

	// A COLLAPSED SHELL is not a composition, and this is the third case — the one the two
	// below did not cover.
	//
	// A backgrounded application can stop exposing its interior entirely. Measured live:
	// Discord active reports hundreds of structures; the same window, backgrounded, reports
	// `pane=7 window=1` with coverage 0.00 and nothing operable. That is not a sparse screen,
	// it is a window whose contents nobody can see, and Marco was minting durable places from
	// it — a live learn established a "screen" of five controls in a 44-pixel strip and
	// remembered it forever.
	//
	// The signal is the world's OWN, not a second opinion: `Shallow` already says "well
	// observed but never seen into", and `Blind` says nothing here can be operated. BOTH are
	// required, which is what keeps a genuinely simple application admissible — a small app
	// with real controls is not blind, however little of the window it covers.
	//
	// Refused as an OBSERVATION rather than filtered as evidence: the honest reading of a
	// shell is "Marco could not see into this window", and the model already has a shape for
	// that. The frame becomes unplaceable, no state is minted, and nothing durable can follow.
	// See [[ADR-056-a-window-nobody-can-see-into-is-not-a-screen]].
	if confidence.Shallow() && confidence.Blind() {
		return observe.StructuralView{
			Why: "the window is open but not exposing its interior, so nothing is known " +
				"about what is on it"}
	}

	var regions []observe.ShadowRegion
	for _, e := range entities {
		if e.Role == "" {
			// A thing with no kind is not part of a composition.
			//
			// EXTENT IS NOT A MEMBERSHIP TEST, and it used to be one here. The comment
			// that stood in its place said counting invisible nodes would make the
			// signature depend on how many an accessibility tree happens to expose. The
			// measurement says the opposite, and it is the whole of Roadmap 34E's last
			// blocker:
			//
			//	Settings Home, one window, three heights — 155 elements every time.
			//	  h=700   zero-area: button 9  group 16 list_item 7 combo_box 1
			//	  h=900   zero-area: button 7  group 8  list_item 1 combo_box 1
			//	  h=1040  zero-area: button 6  group 7  list_item 0 combo_box 0
			//
			// The tree is CONSTANT. What moves is which items report a rectangle, because
			// an item scrolled out of the viewport reports 0x0. Excluding by extent
			// therefore wrote the WINDOW'S HEIGHT into the page's identity: the same Home
			// established as three different places at three different sizes, and every
			// twin family in the live store was one page seen at two window sizes.
			//
			// Counting them instead is viewport-invariant and still separates the pages:
			//
			//	              h=700              h=900              h=1040
			//	Home          button 18 …        identical          identical
			//	Bluetooth     button 10 …        identical          identical
			//	Mouse         button 14 slider 2 identical          identical
			//
			// What a person can SEE is a different question and still has its own answer:
			// the world model keeps every rectangle, Sight shows what is realised now, and
			// target resolution acts on what is actually on screen. This decides only what
			// the page IS. See [[ADR-072-a-place-is-not-its-viewport]].
			continue
		}
		regions = append(regions, observe.ShadowRegion{
			Role: string(e.Role), Region: e.Region,
			Nameable: e.Label.Text != "", Confidence: e.Confidence,
			Chrome: e.Chrome,
		})
	}

	// Structure existing is ITSELF the proof that something looked.
	//
	// These regions came from the FUSED world, and fusion admits only evidence whose
	// provider proved it described the pinned window — see Cycle.Admitted. So an element
	// reaching here has already been through the provenance guard, and asking the outcomes
	// again would be a second copy of a rule that has exactly one implementation.
	if len(regions) > 0 {
		return observe.StructuralView{Source: observe.StructureFused, Regions: regions}
	}

	// Nothing structural. The two reasons for that are completely different, so they are
	// separated here rather than downstream: an application can genuinely present no
	// structure — a real, recurring, sparse screen — and a cycle in which nothing proved
	// its target has observed NOTHING, which must not become a screen.
	if !cycleLooked(cycle) {
		return observe.StructuralView{
			Why: "no provider proved it observed this window, so nothing is known about " +
				"what was on it"}
	}
	return observe.StructuralView{Source: observe.StructureFused}
}

// cycleLooked reports whether anything in this cycle can be shown to have observed the
// target.
//
// The same admission rule fusion applies, asked of the outcomes rather than of the evidence:
// a provider that proved its target, or one that makes no window claim at all and did not
// fail. An untargeted cycle — an ordinary command's perception — passes on the second arm,
// which is correct: nothing was pinned, so nothing had a target to prove.
//
// A provider that was not asked to run says nothing either way, and must not be mistaken
// for one that looked.
func cycleLooked(cycle observation.Cycle) bool {
	for _, o := range cycle.Outcomes {
		switch o.State {
		case observation.StateFailed, observation.StateUnavailable,
			observation.StateNotRequested:
			// Did not deliver, or was never asked to.
			continue
		case observation.StateProvenanceMismatch, observation.StateTargetChanged:
			// Delivered something it could not attribute to this window. The state says so
			// on its own, and is checked here rather than left to TargetProven alone —
			// which answers `true` for an outcome that never recorded what it expected.
			continue
		}
		if o.TargetProven() || o.Global() {
			return true
		}
	}
	return false
}

// safeEntity converts one element, or refuses it.
func safeEntity(el *directorapi.Element, frame directorapi.Rect,
	policy observe.LabelPolicy) (observe.EntitySnapshot, bool) {

	if frame.Width <= 0 || frame.Height <= 0 {
		return observe.EntitySnapshot{}, false
	}
	region := observe.RelativeTo(el.Bounds, frame)

	// The label goes through the shared classifier, never a second policy. One place
	// decides what may be kept in the clear, so there is one place to audit.
	label := observe.Classify(el.Label, el.Confidence,
		observe.LabelContext{Role: el.Role, Sources: sourceNames(el)}, policy)

	gridPosition := ""
	if row, ok := attrInt(el.Attributes, "grid_row"); ok {
		if col, ok2 := attrInt(el.Attributes, "grid_column"); ok2 {
			gridPosition = fmt.Sprintf("r%dc%d", row, col)
		}
	}

	return observe.EntitySnapshot{
		Identity:     observe.IdentityOf(el.Role, label, gridPosition, region),
		Role:         el.Role,
		Label:        label,
		Confidence:   el.Confidence,
		Sources:      sourceNames(el),
		Region:       region,
		Enabled:      el.Enabled,
		Focused:      el.Focused,
		Actionable:   el.Enabled && el.Visible,
		GridPosition: gridPosition,
	}, true
}

// safeGrid recognises an element that IS a grid rather than a cell of one.
func safeGrid(el *directorapi.Element, frame directorapi.Rect) (observe.GridSnapshot, bool) {
	rows, ok := attrInt(el.Attributes, "grid_rows")
	if !ok {
		return observe.GridSnapshot{}, false
	}
	columns, ok := attrInt(el.Attributes, "grid_columns")
	if !ok {
		return observe.GridSnapshot{}, false
	}
	cells, _ := attrInt(el.Attributes, "grid_cells")
	if cells == 0 {
		// A cell carries rows and columns too; only the arrangement itself reports a
		// cell COUNT, which is how the two are told apart.
		return observe.GridSnapshot{}, false
	}
	region := observe.RelativeTo(el.Bounds, frame)
	fill := 0.0
	if rows*columns > 0 {
		fill = float64(cells) / float64(rows*columns)
	}
	return observe.GridSnapshot{
		Identity: observe.IdentityOf("grid", observe.SafeLabel{},
			fmt.Sprintf("%dx%d", rows, columns), region),
		Rows: rows, Columns: columns, Cells: cells, Region: region, Fill: fill,
	}, true
}

// sourceNames lists which providers contributed, without their native identifiers.
func sourceNames(el *directorapi.Element) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range el.Sources {
		name := string(s)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// providerSummaries records what each source did this cycle.
//
// "Missing", "failed" and "returned nothing" are kept apart. They send a reader to three
// different places, and a summary that merged them would be worse than none.
func providerSummaries(cycle observation.Cycle) []observe.ProviderSummary {
	// ADMITTED observations, not collected ones. A refused provider's evidence is
	// quarantined in its outcome and still present in cycle.Observations; counting that
	// here would report a stale provider as having contributed, which is the precise
	// impression the guard exists to prevent.
	admitted, _ := cycle.Admitted()
	counts := map[string]int{}
	for _, o := range admitted {
		counts[string(o.Source())]++
	}
	failed := map[string]string{}
	for _, f := range cycle.Failures {
		failed[string(f.Source)] = f.Reason
	}

	names := make([]string, 0, len(counts)+len(failed))
	for n := range counts {
		names = append(names, n)
	}
	for n := range failed {
		if _, ok := counts[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	// The guard's verdict, per provider. Taken from the OUTCOMES rather than recomputed:
	// the comparison has exactly one implementation and a diagnostic that made its own
	// would eventually disagree with the engine, which is worse than showing nothing.
	outcomes := map[string]observation.ProviderOutcome{}
	for _, o := range cycle.Outcomes {
		outcomes[string(o.Source)] = o
		if _, ok := counts[string(o.Source)]; !ok {
			if _, ok := failed[string(o.Source)]; !ok {
				names = append(names, string(o.Source))
			}
		}
	}
	sort.Strings(names)

	out := make([]observe.ProviderSummary, 0, len(names))
	for _, n := range names {
		s := observe.ProviderSummary{Name: n, Observations: counts[n]}
		if reason, ok := failed[n]; ok {
			s.Failed, s.Reason = true, reason
		}
		if o, ok := outcomes[n]; ok {
			s.State = string(o.State)
			s.Expected = o.ExpectedTarget.WindowGeneration
			s.Observed = o.ObservedTarget.WindowGeneration
			s.Proven = o.TargetProven()
			s.Global = o.Global()
			// A refusal that reached no other field would be invisible here, because a
			// refused provider's observations are quarantined rather than counted.
			if !s.Proven {
				s.Quarantined = len(o.Observations)
				if s.Reason == "" {
					s.Reason = o.Reason
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// attrInt reads an integer attribute, whatever numeric shape JSON left it in.
func attrInt(attrs map[string]any, key string) (int, bool) {
	v, ok := attrs[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
