package observe

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Segmenting a session into the screen states an element can actually exist in.
//
// # The semantic defect this closes
//
// Track presence was Seen over EVERY valid inference in the session. A pause-menu button
// observed in all eight inferences during which the menu was open, in a session that also ran
// eight inferences of gameplay, reported presence 0.50 and shape `bursty`. That reads as "this
// control is unreliable". It is not: it appeared every single time the screen it lives on was
// on screen. The denominator was wrong, not the button.
//
// Experiment 006 and the live Rocket League diagnostic together ruled out every mechanical
// cause — the tracker does not fragment, geometry is stable to IoU 0.99, production and replay
// assign identically. What remained was arithmetic: absence during a DIFFERENT screen state is
// correct evidence, and counting it as a miss is the error.
//
// # The governing rule
//
// An element is judged against the opportunities where its state was active, not against every
// frame in the session. A pause-menu button being absent during gameplay is not a miss.
//
// # Why the states are generic and numbered
//
// `state_1`, not `pause menu`. This layer discovers that the screen's structural composition
// changed and came back; it has no business deciding what a human would call it. Naming is
// interpretation and interpretation belongs to a capability pack, which may later attach that
// hypothesis to a recurring state ID. Building the name in here would make the tracker
// game-specific and would put an unfalsifiable guess underneath a measurement.

// Screen-state model bounds and thresholds.
const (
	// StateGridCols and StateGridRows are the coarse spatial quantisation.
	//
	// Deliberately coarse. The signature must survive a panel shifting a few pixels while
	// still separating a centre-stacked menu from a corner HUD, and a fine grid would make
	// every small movement a new state — the same over-sensitivity that produced the
	// fragmentation scare this work came out of.
	StateGridCols = 3
	StateGridRows = 3

	// StateMatchSimilarity is the overlap at which an inference JOINS a known state.
	//
	// Measured against the live Rocket League trace: menu inferences score 0.71–0.79
	// against the menu state and 0.00–0.33 against everything else, so the band between
	// them is wide and this sits in the middle of it.
	StateMatchSimilarity = 0.55

	// StateContainment is how much of an unmatched inference must be EXPLAINED by a known
	// state before it is read as a transition into or out of that state rather than as a
	// screen in its own right.
	//
	// Similarity alone cannot make this call, and the first version of this file got it
	// wrong: a half-drawn menu and a structurally different dialog both score around 0.3
	// against the menu, so any single threshold either turns every transition frame into a
	// junk one-frame state or swallows genuinely new screens into state_unknown.
	//
	// Containment separates them, because the two differ in KIND rather than in degree. A
	// partial rendering is a subset of the screen it is becoming: everything in it is
	// already accounted for, there is just less of it. A different screen introduces
	// structure — a checkbox, a panel — that the known state has never had. So the question
	// is not "how alike are these" but "does this frame contain anything new".
	StateContainment = 0.75

	// MinLocalCellStructures is how much of the surface must be made of something it was not
	// before that counts as a part of it having been REPLACED.
	//
	// Eight, and it is the ONLY number this comparison has. There was a similarity threshold
	// beside it — how much of a region had to survive — and it was removed once the whole
	// suite proved indifferent to its value: once "replaced" is measured as composition rather
	// than as amount, a ratio adds nothing but a knob somebody would later tune.
	//
	// Not calibrated against any application. Derived from what the sentence has to mean:
	// "a part of this surface now holds something it did not" is a claim about a region with
	// parts, and below about eight of them it is arithmetic about two or three boxes. A window
	// exposing a dozen structures in total has no part that can carry it, and such a surface
	// falls back to the whole-surface comparison alone — which is the "insufficient evidence"
	// answer, made structural rather than left implicit.
	MinLocalCellStructures = 8.0

	// StatePromotionCount is how many times an ambiguous composition must recur before it
	// becomes a state of its own rather than a transition.
	//
	// Two. One sighting is a transition frame; a second is a screen. Higher would be safer
	// and would also throw away the early evidence of a screen that only appears twice in a
	// short session, which is most of them.
	//
	// The value is load-bearing at every setting, which took a change to `promote` to make
	// true: the count used to be applied in a way that made a first sighting unable to
	// promote whatever the number said, so 1 and 2 were the same policy and neither could be
	// told from the other by any test.
	StatePromotionCount = 2

	// MaxScreenStates bounds one session's state model. Beyond it nothing new is minted
	// and the overflow is reported rather than silently absorbed.
	MaxScreenStates = 32
	// MaxStateTransitions bounds the transition history.
	MaxStateTransitions = 256
	// MaxCrossings bounds the ORDERED walk — see ShadowTotals.Crossings.
	//
	// Generous relative to what reads it. A demonstration may pass through
	// DefaultCaptureBounds().MaxCheckpoints screens and crosses at most two changes per
	// screen, so 128 holds every walk any consumer is allowed to read, several times over.
	// The cost is three words per entry and there is no per-crossing evidence, deliberately.
	MaxCrossings = 128
	// MaxTrackStates bounds how many states one track carries evidence for. A control that
	// genuinely appears in more than a handful of distinct screens is not a control.
	MaxTrackStates = 6
	// MaxSignatureKeys bounds one signature, so a pathological frame cannot make the state
	// model grow with the detector's output.
	MaxSignatureKeys = 64
)

// ScreenStateID is a session-local state identity. Meaningless outside the session, by design.
type ScreenStateID string

// ScreenStateUnknown is the state of an inference that could not be confidently placed.
//
// Load-bearing. It is not a state elements can belong to and it grants no eligibility: an
// inference Marco cannot place is not evidence that anything was missing from it.
const ScreenStateUnknown ScreenStateID = "state_unknown"

// ScreenSignature is a bounded, generic structural summary of ONE inference.
//
// Role composition and coarse normalised arrangement. Nothing else: no labels, no OCR text, no
// absolute coordinates, no sequence names, and — critically — no track identities. See
// ScreenSegmenter.Observe for why that last exclusion is what keeps the model non-circular.
type ScreenSignature struct {
	Roles map[string]int `json:"roles,omitempty"`
	Cells map[string]int `json:"cells,omitempty"`
	Total int            `json:"total"`
}

// NewScreenSignature summarises one inference's raw detections.
//
// THE choke point for what a screen may be identified BY, and therefore for what reaches
// durable memory: a signature becomes a StructureSignature, and a StructureSignature is stored.
// Every structural source in the system passes through here.
func NewScreenSignature(regions []ShadowRegion) ScreenSignature {
	sig := ScreenSignature{Roles: map[string]int{}, Cells: map[string]int{}}
	for _, r := range regions {
		if r.Chrome {
			// A WINDOW IS NOT A PLACE.
			//
			// The title bar and the scroll bars belong to the frame the page is
			// shown in, not to the page. Counted here they made the same Settings
			// screen look like several: three families of twins in one live store
			// each differed from their named original by exactly three buttons,
			// which were the frame.s Minimize, Restore and Close.
			//
			// Classified by HIERARCHY in observation.ChromeIn, where the parent
			// links still exist, and carried here on the region. Not by geometry:
			// an earlier attempt excluded zero-area elements and stripped real page
			// content — Settings reports a combo box, links and nineteen pieces of
			// text with no rectangle at all.
			//
			// Chrome is still OBSERVED. This is the only place it is left out, and
			// what it is left out of is what makes a screen that screen.
			//
			// Deleting this must fail TestAWindowsOwnMachineryIsNotPartOfThePlace.
			continue
		}
		if !roleShaped(r.Role) {
			// A role is a KIND, from a provider's closed class vocabulary. A string that
			// is not shaped like one is not a kind — it is text that arrived in the wrong
			// field, and admitting it would put a label into a screen's identity and from
			// there into the durable store, where it would outlive every session that
			// could have contradicted it.
			//
			// Refused on SHAPE rather than against an allowlist. The role vocabularies are
			// the providers' own and they grow — the live sessions reported `link`,
			// `combo_box`, `tab_list`, `progress_bar` and `scroll_bar`, none of which any
			// list in this repository had — so an allowlist here would silently discard
			// most of a real accessibility tree.
			continue
		}
		sig.Total++
		sig.Roles[r.Role]++
		// A CELL is a statement about WHERE something sits, and a control with no
		// rectangle sits nowhere. It is still part of what the page is made of — see the
		// note in fusedStructure on why extent is not a membership test — but placing it
		// at the origin would pile every off-viewport item into one cell and distort the
		// arrangement the segmenter compares states by.
		//
		// So: roles always, cells only where there is a rectangle to speak of.
		if r.Region.Width > 0 && r.Region.Height > 0 {
			sig.Cells[cellKey(r.Role, r.Region)]++
		}
	}
	boundKeys(sig.Cells, MaxSignatureKeys)
	boundKeys(sig.Roles, MaxSignatureKeys)
	return sig
}

// MaxRoleLength bounds a structural kind's name.
//
// Generous for a role — `progress_bar` is twelve — and far too short for anything anybody
// would want to smuggle through.
const MaxRoleLength = 32

// roleShaped reports whether a string is shaped like a structural kind.
//
// Lowercase letters, digits and underscores, bounded. Every role any provider in this system
// has ever reported satisfies it; a label, a window title, a line of read text and a file path
// all fail on their spaces, their capitals or their length.
func roleShaped(s string) bool {
	if s == "" || len(s) > MaxRoleLength {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

// localChange reports the WORST-preserved part of the surface, and where.
//
// # Why the global comparison is not enough
//
// `signatureSimilarity` sums over the whole surface. A change confined to one part therefore
// contributes only its own fraction of the total, and an application that redraws a large
// fraction without going anywhere contributes more. Those two are not merely close — they are
// INVERTED. Measured on generic surfaces: a whole new kind of content behind unchanged chrome
// scored 0.915 globally, and a scroll that churned eight structures scored 0.947. The thing that
// mattered looked less different than the thing that did not.
//
// That is an information-loss defect, not a threshold that needs moving. No value separates two
// populations that overlap, and lowering the global bar to catch the first would turn ordinary
// use of every application into a storm of false states.
//
// # What this asks instead
//
// The signature already decomposes the surface spatially — every key is `role@col,row`. So the
// question "was a coherent PART of this surface replaced" needs no new evidence, no hierarchy
// and no second model: it is the same comparison, asked per cell instead of summed.
//
// On the same generic surfaces this orders correctly, where the global comparison does not:
//
//	replaced content   0.000     |  scroll            1.000
//	a sidebar appears  0.400     |  churn +3          0.974
//	a modal appears    0.840     |  churn -8          0.917
//	                             |  a small dropdown  0.913
//
// # What it deliberately cannot see
//
// A block of the SAME KIND of structure laid over the same kind of content is a count change in
// one cell and nothing else. With role and a coarse cell as the whole vocabulary, that is
// genuinely indistinguishable from more of the same thing arriving. It is an information limit
// of the signature rather than a choice made here, and it is why the result is HELD and promoted
// on persistence rather than acted on immediately.
func localChange(a, b map[string]float64) (worst float64, at string) {
	byCell := func(m map[string]float64) map[string]map[string]float64 {
		out := map[string]map[string]float64{}
		for k, v := range m {
			cell, ok := cellOf(k)
			if !ok {
				continue
			}
			if out[cell] == nil {
				out[cell] = map[string]float64{}
			}
			out[cell][k] = v
		}
		return out
	}
	ca, cb := byCell(a), byCell(b)

	// Sorted, because float addition is not associative and a verdict that depended on map
	// iteration order would be irreproducible — the same rule signatureSimilarity follows.
	cells := make([]string, 0, len(ca)+len(cb))
	seen := map[string]bool{}
	for k := range ca {
		if !seen[k] {
			seen[k], cells = true, append(cells, k)
		}
	}
	for k := range cb {
		if !seen[k] {
			seen[k], cells = true, append(cells, k)
		}
	}
	sort.Strings(cells)

	if len(cells) == 0 {
		return 1, ""
	}

	// A cell only gets a vote if it holds enough to HAVE a composition.
	//
	// A cell holding three structures has no resolution: one of them going changes it by a
	// third, and "a third of this region was replaced" is a sentence about arithmetic rather
	// than about an interface. A surface made entirely of such cells — a window exposing a
	// dozen structures — cannot support a local question at all, and asking one anyway turns
	// every flicker into a new place.
	//
	// The bar is ABSOLUTE rather than a fraction of the surface, and that is a deliberate
	// reversal. It began as the mean cell mass, on the reasoning that "substantial" ought to
	// scale with what is being looked at. Mutation found nothing depending on it, and working
	// out what it actually suppressed showed why nothing should: it only ever bit on a QUIET
	// cell of a LARGE surface, which is precisely a dialog opening over the empty corner of a
	// big window. Scaling the bar with the surface makes a window's own size an argument
	// against noticing things inside it, and what it hides is the case worth noticing.
	// Resolution is a property of a region, not of its neighbours.
	//
	// Below the bar the comparison abstains and the whole-surface one decides alone — the
	// behaviour these surfaces had before this question existed, which was never the problem.
	floor := MinLocalCellStructures

	// Every part that is substantially not what it was, and how much of it went.
	//
	// Summed ACROSS those parts rather than required of any one, because the grid is coarse
	// and a panel does not respect it: a modal over the centre of a surface lands half in one
	// cell and half in another, and a rule that made each cell carry the whole amount would
	// miss exactly the change this milestone is about. What matters is how much of the
	// surface was replaced, not whether it happened to fall inside one square.
	worst, at = 1, ""
	var replaced float64
	for _, c := range cells {
		before, after := ca[c], cb[c]
		if mass(before)+mass(after) < floor {
			continue
		}
		if m := replacedMass(before, after); m > 0 {
			replaced += m
			// The worst-preserved part, for the record. It explains a verdict; it does
			// not make one.
			if s := signatureSimilarity(before, after); s < worst {
				worst, at = s, c
			}
		}
	}

	// Below the bar the comparison ABSTAINS — it does not vote "unchanged", it declines to
	// vote at all, and 1 is how a similarity says nothing to add. The caller then decides on
	// the whole-surface comparison alone, which is what it did before this question existed.
	//
	// This is what keeps a surface with no persistent chrome honest. Where every part is the
	// content, replacing the content replaces the surface, the global comparison already owns
	// it, and a local claim on top of that would be the same evidence counted twice.
	if replaced < MinLocalCellStructures {
		return 1, ""
	}
	return worst, at
}

func mass(m map[string]float64) float64 {
	var n float64
	for _, v := range m {
		n += v
	}
	return n
}

// replacedMass is the structure whose KIND arrived or left, in either direction.
//
// # Why kinds and not amounts
//
// Because amounts rank these the wrong way round, and they do it at every scale. A list loading
// another page changes forty structures and remains entirely itself; a panel of a different kind
// changes fifteen and means somewhere else. A guard sized on the amount lets the first through
// and stops the second — which is the same inversion, one level down from the one this whole
// milestone is about.
//
// A key here is `role@col,row`, so a key ARRIVING means a part of the surface now holds a kind
// of thing it did not, and a key LEAVING means a kind that was there is gone. That is what
// "replaced" means. More of what was already there is not replacement however much of it there
// is, and this counts none of it.
//
// The cost is stated plainly: a block of the SAME kind laid over the same kind is invisible
// here, because at this vocabulary it is genuinely the same observation as more of that kind
// arriving. See TestWhatTheLocalComparisonStillCannotSee.
func replacedMass(a, b map[string]float64) float64 {
	var n float64
	for k, v := range a {
		if b[k] == 0 {
			n += v
		}
	}
	for k, v := range b {
		if a[k] == 0 {
			n += v
		}
	}
	return n
}

// cellOf extracts the `col,row` part of a feature key.
//
// Only `cell:` keys have one. A `role:` key is a whole-surface tally with no place, and folding
// it into a cell would make one part of the surface answer for the composition of all of it.
func cellOf(key string) (string, bool) {
	const prefix = "cell:"
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return "", false
	}
	at := strings.LastIndex(key, "@")
	if at < 0 || at+1 >= len(key) {
		return "", false
	}
	return key[at+1:], true
}

// cellKey places a region by its CENTRE in the coarse grid.
//
// Centre rather than origin so a box that grows slightly does not cross a cell boundary
// because its left edge moved.
func cellKey(role string, r Region) string {
	return fmt.Sprintf("%s@%d,%d", role,
		clampCell(r.X+r.Width/2, StateGridCols), clampCell(r.Y+r.Height/2, StateGridRows))
}

func clampCell(v float64, n int) int {
	i := int(v * float64(n))
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// boundKeys keeps the highest-count keys, deterministically.
func boundKeys(m map[string]int, max int) {
	if len(m) <= max {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if m[keys[a]] != m[keys[b]] {
			return m[keys[a]] > m[keys[b]]
		}
		return keys[a] < keys[b]
	})
	for _, k := range keys[max:] {
		delete(m, k)
	}
}

// SignatureSimilarity compares two signatures the way the segmenter compares a frame against
// a state.
//
// Exported for the characterisation tests that measure what the identity threshold can see at
// different provider scales. Deliberately the SAME arithmetic the segmenter uses rather than a
// copy for tests to read: a characterisation whose numbers drifted from the thing it
// characterises would be worse than none.
func SignatureSimilarity(a, b ScreenSignature) float64 {
	return signatureSimilarity(a.features(), b.features())
}

// features flattens a signature into one comparable vector.
func (s ScreenSignature) features() map[string]float64 {
	f := make(map[string]float64, len(s.Roles)+len(s.Cells))
	for k, v := range s.Roles {
		f["role:"+k] = float64(v)
	}
	for k, v := range s.Cells {
		f["cell:"+k] = float64(v)
	}
	return f
}

// signatureSimilarity is weighted Jaccard over two feature vectors.
//
// Weighted rather than set-based because composition is the discriminator: four stacked buttons
// and two buttons beside a checkbox share the role `button` and must not merge on that alone.
// Sums are accumulated over SORTED keys — float addition is not associative, and a state model
// whose identities depended on map iteration order would be irreproducible.
func signatureSimilarity(a, b map[string]float64) float64 {
	if len(a) == 0 && len(b) == 0 {
		// Two screens with no structural evidence are the same kind of screen. This is the
		// sparse gameplay case, and it is a real state rather than a failure to observe.
		return 1
	}
	seen := make(map[string]bool, len(a)+len(b))
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		if !seen[k] {
			seen[k], keys = true, append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k], keys = true, append(keys, k)
		}
	}
	sort.Strings(keys)

	var minSum, maxSum float64
	for _, k := range keys {
		minSum += math.Min(a[k], b[k])
		maxSum += math.Max(a[k], b[k])
	}
	if maxSum <= 0 {
		return 0
	}
	return minSum / maxSum
}

// coverage is how much of `f` a known state already accounts for.
//
// Asymmetric on purpose, and that asymmetry is the whole point: it asks only whether `f`
// introduces structure `known` does not have, and says nothing about what `known` has that `f`
// lacks. A frame showing one row of a four-row menu is fully covered; a dialog with a checkbox
// is not, however much of it happens to overlap.
func coverage(f, known map[string]float64) float64 {
	var total, explained float64
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		total += f[k]
		explained += math.Min(f[k], known[k])
	}
	if total <= 0 {
		// A frame with no structure is not a partial rendering of anything. It is its own
		// kind of screen — the sparse one gameplay actually produces.
		return 0
	}
	return explained / total
}

// ScreenState is one recurring screen composition, as a session remembers it.
// SurfaceID identifies the enclosing application surface a state belongs to.
//
// Two states with the same surface are two PLACES INSIDE ONE APPLICATION — a content region
// changed, a panel opened — rather than two unrelated worlds. Session-local like a state id, and
// for the same reason: it is a counter, it means something different in the next run, and
// nothing durable may be keyed on it.
type SurfaceID string

type ScreenState struct {
	ID ScreenStateID `json:"id"`
	// Surface is the enclosing surface this state is a state OF. Empty means the state is
	// its own surface — the first composition of its kind the session saw.
	//
	// This is what lets both truths be told at once: "the same application surface" and "not
	// the place it was". A single identity could carry only one of them, and the one it
	// carried was the one that could not see a local change.
	Surface SurfaceID `json:"surface,omitempty"`
	// LocalFrom is the state this one was distinguished FROM, and LocalCell where. Session-
	// local diagnostics: they explain a verdict and nothing reads them to make one.
	LocalFrom ScreenStateID `json:"local_from,omitempty"`
	LocalCell string        `json:"local_cell,omitempty"`
	// Inferences is how many valid inferences were confidently placed in this state — the
	// state-local denominator every presence ratio below is measured against.
	Inferences int `json:"inferences"`
	// Episodes is how many separate times this state was entered.
	Episodes       int            `json:"episodes"`
	FirstInference int            `json:"first_inference"`
	LastInference  int            `json:"last_inference"`
	Roles          map[string]int `json:"roles,omitempty"`
	// Compositions is how often each WHOLE role composition was observed in this state.
	//
	// Reported so a reader can tell a durable identity that WAS observed from one assembled
	// out of per-role modes taken at different moments. See settledWhole.
	Compositions map[string]int `json:"compositions,omitempty"`
	// PlaceNames counts what this screen appeared to be CALLED, per word.
	//
	// Tallied rather than taken from the latest inference, for the reason ADR-073 tallies
	// whole compositions: a transition frame can carry the name of the page being left. The
	// word that recurs is the screen.s; a word seen once is a frame.
	PlaceNames map[string]int `json:"place_names,omitempty"`
	// Settled says this screen has stopped materially changing shape, and is therefore
	// worth remembering. See settledWhole: visible is not settled, and settled is
	// what identity is allowed to rest on.
	Settled bool `json:"settled,omitempty"`
	Tracks  int  `json:"tracks,omitempty"`
	// Terms counts the inferences of THIS state in which each generic interface term was
	// read, so a term always arrives with the denominator that makes it meaningful.
	//
	// The denominator is what separates evidence from coincidence: `settings` seen in 4 of
	// this state's 4 inferences is a property of the screen, and `settings` seen in 1 of 40
	// is a toast notification that happened to be up, or a misread. Storing a bare count
	// would render those identically, which is how a hypothesis layer comes to believe
	// everything it has ever seen once.
	Terms map[InterfaceTerm]int `json:"terms,omitempty"`
	// TermEpisodes counts the separate visits to this state in which each term appeared.
	//
	// Held apart from Terms because eight inferences of one long visit are one opportunity
	// to be wrong, and one inference in each of four separate visits is four.
	TermEpisodes map[InterfaceTerm]int `json:"term_episodes,omitempty"`
	// EditableFields is the most text-editable controls seen in any one inference here.
	EditableFields int `json:"editable_fields,omitempty"`
	// TermObservations is how many of this state's inferences actually had interface text
	// to read — the DENOMINATOR every term ratio is measured against.
	//
	// Not the inference count. Scoped OCR runs on roughly one inference in six, so a term
	// present on every reading would score about 0.17 against the inference count and never
	// clear the ratio threshold. Measured against the readings that happened, the same term
	// scores 1.00, which is what it deserves.
	//
	// Zero means the terms for this state are UNKNOWN rather than absent — nothing ever
	// gave Marco anything to classify.
	TermObservations int `json:"term_observations,omitempty"`

	sum map[string]float64
	n   int
	// compositions is how often each WHOLE role composition was observed, and what it was.
	//
	// THE durable identity substrate. A composition assembled from per-role modes can be one
	// no sample ever showed; this keeps whole observations so the one that is promoted is one
	// that actually happened. See settledWhole.
	compositions map[string]*seenComposition
	// termEpisode remembers which episode last credited a term, so a long visit cannot
	// inflate the episode count.
	termEpisode map[InterfaceTerm]int
}

// mean is the state's running representative composition.
//
// A running mean, not the first signature ever seen. Freezing the first observation is exactly
// the reference policy whose failure mode Experiment 006 pinned with a test; repeating it one
// layer up would make a state drift away from its own definition as its screen animated in.
func (s *ScreenState) mean() map[string]float64 {
	if s.n == 0 {
		return nil
	}
	out := make(map[string]float64, len(s.sum))
	for k, v := range s.sum {
		out[k] = v / float64(s.n)
	}
	return out
}

func (s *ScreenState) fold(f map[string]float64) {
	if s.sum == nil {
		s.sum = map[string]float64{}
	}
	for k, v := range f {
		s.sum[k] += v
	}
	s.n++
	s.tallyComposition(f)
}

// tallyComposition records the WHOLE role composition this observation saw.
//
// Bounded: a state that has genuinely presented MaxSignatureKeys
// different whole compositions is not describing one screen, and the bound stops a long session
// growing this without limit.
func (s *ScreenState) tallyComposition(f map[string]float64) {
	if s.compositions == nil {
		s.compositions = map[string]*seenComposition{}
	}
	k := compositionKey(f)
	if _, seen := s.compositions[k]; !seen && len(s.compositions) >= MaxSignatureKeys {
		return
	}
	if c := s.compositions[k]; c != nil {
		c.n++
		return
	}
	s.compositions[k] = &seenComposition{roles: rolesOf(f), n: 1}
}

// seenComposition is one whole composition this state actually presented, and how often.
type seenComposition struct {
	roles map[string]int
	n     int
}

// rolesOf reads the role composition out of one observation's feature vector.
func rolesOf(f map[string]float64) map[string]int {
	out := map[string]int{}
	for k, v := range f {
		if !strings.HasPrefix(k, "role:") || v <= 0 {
			continue
		}
		out[strings.TrimPrefix(k, "role:")] = int(v)
	}
	return out
}

// compositionKey renders one observation.s role composition canonically.
//
// Roles only. Cells describe arrangement and move with the viewport; the composition is what the
// screen is MADE of, which is the thing a durable identity rests on.
func compositionKey(f map[string]float64) string {
	parts := make([]string, 0, len(f))
	for k, v := range f {
		if !strings.HasPrefix(k, "role:") {
			continue
		}
		if v <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", strings.TrimPrefix(k, "role:"), int(v)))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// ScreenTransition is one observed change of screen composition.
type ScreenTransition struct {
	From  ScreenStateID `json:"from"`
	To    ScreenStateID `json:"to"`
	Count int           `json:"count"`
	// Preceded counts how often each navigation intent was observed in the interval
	// before this change.
	//
	// CORRELATION, and the name is chosen to keep it that way. "Pressing pause opened the
	// menu" is a causal claim this layer has no way to establish: the player may have
	// pressed three keys, the change may have been a cutscene ending, and a 3.5-second
	// sampling interval cannot order two events inside it. What can be said honestly is
	// that the intent was seen before the change, and how often that has held.
	Preceded map[NavIntent]int `json:"preceded,omitempty"`
	// Unattributed counts changes with no navigation observed before them at all — a
	// timer, a network event, a scene load. Reported beside the correlations rather than
	// omitted, because a transition that usually has no input is evidence AGAINST reading
	// the correlated ones as causes.
	Unattributed int `json:"unattributed,omitempty"`
	// ConditionalOnly counts the attributed observations whose navigation came ENTIRELY
	// from keys that are only navigation in context.
	//
	// A third category beside attributed and unattributed, and it has to be its own number.
	// W preceding a change while the screen looked like a set of choices is real evidence
	// and weaker evidence: the key also means "walk forwards", and whether it was navigation
	// rests on an assessment of the screen made up to one observation earlier. Folding
	// those observations in with the unambiguous ones would let a session of somebody
	// walking around produce the same confident edge as one of somebody using a menu.
	ConditionalOnly int `json:"conditional_only,omitempty"`
	// UnsettledRun is the LONGEST contiguous run of unplaceable inferences that immediately
	// preceded this change, and is only ever non-zero when From is ScreenStateUnknown.
	//
	// It records how long Marco could not say where the user was — nothing about WHAT the
	// unplaced samples were, because that is exactly what could not be read. It exists so the
	// relationship layer can tell a change that crossed a single transition frame from one
	// that crossed an interval long enough to have hidden a screen; see bridge.go, which takes
	// its bound from StatePromotionCount rather than from a number of its own.
	//
	// The LONGEST rather than the latest, deliberately: an edge is bridged on the worst
	// interval it ever crossed, so a run that was once too long is never made acceptable by a
	// shorter one later.
	UnsettledRun int `json:"unsettled_run,omitempty"`
	// Sequences are the ORDERED intent runs seen before this change, one entry per
	// distinct order, with how often that order recurred.
	//
	// Preceded answers "which intents", and deliberately counts each once. This answers
	// "in what order", and it is a different question with a different use: `down, down,
	// confirm` and `confirm, down, down` produce identical Preceded maps and describe two
	// different interactions. Reconstructing a procedure — move the selection twice, then
	// confirm — needs the order, and an unordered bundle has thrown it away irrecoverably.
	//
	// Adjacent repeats are KEPT. A held key was already collapsed to one intent by the
	// producer, so a second `down` here is a second deliberate press, which is exactly the
	// signal a selection-changing step is made of.
	//
	// Session-local runs carry TARGETS beside the intents — what each event was aimed at,
	// when the evidence identified it. The durable topology folds these down to plain
	// NavSequence; only a licensed demonstration's candidate keeps them.
	Sequences []TargetedSequence `json:"sequences,omitempty"`
}

// NavSequence is one observed order of navigation before a change, and how often it recurred.
type NavSequence struct {
	Intents []NavIntent `json:"intents"`
	Count   int         `json:"count"`
}

// TargetedSequence is one ordered run WITH what each event was aimed at.
//
// A SESSION-LOCAL sibling of NavSequence, split by type on purpose: the durable topology
// stores NavSequence, which has nowhere to put a label, so folding a session's evidence into
// the store strips the targets structurally rather than by someone remembering to. Targets
// persist in exactly one durable place — a licensed demonstration's candidate — which is the
// deliberate, reviewed exception (see SemanticTarget).
type TargetedSequence struct {
	Intents []NavIntent `json:"intents"`
	// Targets is aligned with Intents; a zero entry means the event resolved to nothing.
	// Omitted entirely when nothing in the run resolved.
	Targets []SemanticTarget `json:"targets,omitempty"`
	Count   int              `json:"count"`
}

// Plain strips the targets, which is the ONLY way a run enters the durable topology.
func (s TargetedSequence) Plain() NavSequence {
	return NavSequence{Intents: append([]NavIntent{}, s.Intents...), Count: s.Count}
}

// TargetAt is the target aligned with intent i, zero when the run carries none.
func (s TargetedSequence) TargetAt(i int) SemanticTarget {
	if i < 0 || i >= len(s.Targets) {
		return SemanticTarget{}
	}
	return s.Targets[i]
}

// Equal reports whether this run describes the same order of intents. Order identity is the
// intents alone — a run resolved to targets and the same run unresolved are one observation
// of one order, which is exactly why a repeat may fill targets in.
func (s TargetedSequence) Equal(other []NavIntent) bool {
	if len(s.Intents) != len(other) {
		return false
	}
	for i := range other {
		if s.Intents[i] != other[i] {
			return false
		}
	}
	return true
}

// sameOrder is Equal, under the name addSequence reads best with.
func (s TargetedSequence) sameOrder(other []NavIntent) bool { return s.Equal(other) }

// PlainSequences strips a whole set of runs for the durable boundary.
func PlainSequences(in []TargetedSequence) []NavSequence {
	if len(in) == 0 {
		return nil
	}
	out := make([]NavSequence, 0, len(in))
	for _, s := range in {
		out = append(out, s.Plain())
	}
	return out
}

// Equal reports whether two sequences describe the same order.
func (s NavSequence) Equal(other []NavIntent) bool {
	if len(s.Intents) != len(other) {
		return false
	}
	for i := range other {
		if s.Intents[i] != other[i] {
			return false
		}
	}
	return true
}

// MaxSequencesPerTransition bounds how many distinct orders one edge remembers.
const MaxSequencesPerTransition = 6

// MaxSequenceLength bounds one remembered order.
//
// Long enough for the interactions this is for — a few moves and a confirm — and short enough
// that an edge cannot become a transcript of a session. Learning long macros is explicitly a
// later milestone; this retains only what a short procedure would need.
const MaxSequenceLength = 8

// Attributed reports how often this change followed some observed navigation.
func (t ScreenTransition) Attributed() int { return t.Count - t.Unattributed }

// Dominant returns the intent most often seen before this change, and how often.
//
// Reported with its count so a reader can see the strength for themselves. One observation of
// `pause` before a change is not the same claim as four out of four, and a bare "dominant
// intent" would render them identically.
func (t ScreenTransition) Dominant() (NavIntent, int) {
	best, n := NavIntent(""), 0
	keys := make([]NavIntent, 0, len(t.Preceded))
	for k := range t.Preceded {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })
	for _, k := range keys {
		if t.Preceded[k] > n {
			best, n = k, t.Preceded[k]
		}
	}
	return best, n
}

// transitionRecord accumulates one edge of the state graph.
type transitionRecord struct {
	count           int
	unattributed    int
	conditionalOnly int
	preceded        map[NavIntent]int
	sequences       []TargetedSequence
	// unsettledRun is the LONGEST run of unplaceable inferences seen immediately before
	// this change. Only ever set when the change came FROM the unknown state.
	unsettledRun int
}

// addSequence records one observed order, merging it with an identical earlier one.
//
// Bounded twice over: a long run is truncated, and a transition that keeps seeing NEW orders
// stops remembering them rather than growing. Dropping the newest is the right way round — an
// order seen once carries less than one already seen twice, and the counts of the retained ones
// stay true.
func (r *transitionRecord) addSequence(seq []NavIntent, targets []SemanticTarget) {
	if len(seq) == 0 {
		return
	}
	if len(seq) > MaxSequenceLength {
		seq = seq[:MaxSequenceLength]
		if len(targets) > MaxSequenceLength {
			targets = targets[:MaxSequenceLength]
		}
	}
	for i := range r.sequences {
		if r.sequences[i].sameOrder(seq) {
			r.sequences[i].Count++
			// A repeat of the order may know MORE about its targets than the first
			// sighting did — a later pass resolved what an earlier one could not.
			// Filling in is admitting better evidence; overwriting a resolved target
			// with an empty one would be forgetting on a technicality.
			if len(r.sequences[i].Targets) == 0 && anyTarget(targets) {
				r.sequences[i].Targets = append([]SemanticTarget{}, targets...)
			}
			return
		}
	}
	if len(r.sequences) >= MaxSequencesPerTransition {
		return
	}
	entry := TargetedSequence{Intents: append([]NavIntent{}, seq...), Count: 1}
	if anyTarget(targets) {
		entry.Targets = append([]SemanticTarget{}, targets...)
	}
	r.sequences = append(r.sequences, entry)
}

// anyTarget reports whether a run resolved anything at all, so an all-empty alignment is
// stored as nothing rather than as a row of blanks.
func anyTarget(in []SemanticTarget) bool {
	for _, t := range in {
		if t.Role != "" || t.Label != "" {
			return true
		}
	}
	return false
}

// pendingComposition is a structure seen but not yet trusted to be a screen of its own.
type pendingComposition struct {
	f map[string]float64
	n int
}

// ScreenSegmenter assigns each valid inference to a session-local screen state.
type ScreenSegmenter struct {
	states  []*ScreenState
	pending []*pendingComposition
	current ScreenStateID
	next    int

	transitions map[[2]ScreenStateID]*transitionRecord
	// unsettled is how many inferences IN A ROW have been unplaceable, right now.
	//
	// Reset the moment anything is placed, so it always describes the CURRENT run rather
	// than the session. It records only the LENGTH of the gap; nothing about what was in it,
	// because that is precisely what could not be read.
	unsettled int
	// crossings is the session's WALK: every change, in the order it was seen.
	//
	// The transition map answers "which changes happened, and how often". It cannot answer
	// "in what order", because it is keyed by the pair. That is enough for every question
	// about a recurring habit and not enough for one about a route somebody just walked:
	// `A → ? → B → ? → C` aggregates to two entries into the unplaceable state and two exits
	// out of it, and which entry belongs with which exit is not recoverable from counts. See
	// unsettledIntervals.
	crossings []Crossing
	// EvictedStates counts inferences that would have minted a state past the bound.
	EvictedStates      int
	EvictedTransitions int
	// EvictedCrossings counts changes the walk bound dropped. A truncated walk is not a
	// short one: consumers that pair intervals refuse outright rather than pair across a gap.
	EvictedCrossings int

	// Match is what the identity comparison actually scored, this session.
	//
	// The number behind every verdict, and the one thing a live session could not report.
	// `1 screen, 0 transitions` is consistent with an application that never changed AND
	// with a threshold that cannot see the change — and the two want opposite work done
	// about them.
	Match MatchProfile
}

// MatchProfile summarises the similarity scores one session's identity comparisons produced.
//
// Bounded to a handful of numbers, deliberately. A distribution would be more informative and
// would also be a per-inference record of what somebody's screen looked like; the summary
// answers the question the threshold work needs — where do the two populations actually sit —
// without becoming a trace.
type MatchProfile struct {
	// Joined describes the frames that were read as the SAME screen: how many, and the
	// weakest, strongest and mean similarity among them. The weakest is the interesting
	// one — it is the noise floor of whatever source is feeding the model.
	Joined     int     `json:"joined,omitempty"`
	JoinedMin  float64 `json:"joined_min,omitempty"`
	JoinedMax  float64 `json:"joined_max,omitempty"`
	JoinedMean float64 `json:"joined_mean,omitempty"`
	// Separated describes the frames that became another screen, or were held as an
	// ambiguous composition. The strongest is the other interesting one: together with
	// JoinedMin it says whether the two populations overlap on this application.
	Separated     int     `json:"separated,omitempty"`
	SeparatedMax  float64 `json:"separated_max,omitempty"`
	SeparatedMean float64 `json:"separated_mean,omitempty"`
	// First counts inferences with nothing to compare against yet.
	First int `json:"first,omitempty"`

	// LocalSeen, LocalMin and LocalMean describe the SECOND comparison: how well the
	// worst-preserved part of the surface survived, per inference. LocalReplaced counts the
	// inferences where the surface was recognisably the same and a coherent part of it had
	// been replaced — the case a global comparison alone cannot express.
	LocalSeen     int     `json:"local_seen,omitempty"`
	LocalMin      float64 `json:"local_min,omitempty"`
	LocalMean     float64 `json:"local_mean,omitempty"`
	LocalReplaced int     `json:"local_replaced,omitempty"`

	joinedSum, separatedSum, localSum float64
}

// surfaceOf is the surface this state belongs to, which is itself when it has no other.
//
// A state that was never distinguished from anything IS a surface — the first composition of
// its kind the session saw — so the relation is total and nothing downstream has to handle a
// state with no surface.
func (s *ScreenState) surfaceOf() SurfaceID {
	if s.Surface != "" {
		return s.Surface
	}
	return SurfaceID(s.ID)
}

// recordLocal folds one local comparison into the profile.
func (p *MatchProfile) recordLocal(worst float64, replaced bool) {
	if p.LocalSeen == 0 || worst < p.LocalMin {
		p.LocalMin = worst
	}
	p.LocalSeen++
	p.localSum += worst
	p.LocalMean = p.localSum / float64(p.LocalSeen)
	if replaced {
		p.LocalReplaced++
	}
}

// record folds one comparison's score into the profile.
func (p *MatchProfile) record(bestSim float64, joined bool) {
	if bestSim < 0 {
		p.First++
		return
	}
	if joined {
		if p.Joined == 0 || bestSim < p.JoinedMin {
			p.JoinedMin = bestSim
		}
		if bestSim > p.JoinedMax {
			p.JoinedMax = bestSim
		}
		p.Joined++
		p.joinedSum += bestSim
		p.JoinedMean = p.joinedSum / float64(p.Joined)
		return
	}
	if bestSim > p.SeparatedMax {
		p.SeparatedMax = bestSim
	}
	p.Separated++
	p.separatedSum += bestSim
	p.SeparatedMean = p.separatedSum / float64(p.Separated)
}

// Overlaps reports whether a frame read as the same screen scored no better than one read as
// a different screen.
//
// The whole question in one predicate: when this is true, no single similarity threshold can
// separate the two on this application, and the identity model needs something other than a
// bigger or smaller number.
func (p MatchProfile) Overlaps() bool {
	return p.Joined > 0 && p.Separated > 0 && p.SeparatedMax >= p.JoinedMin
}

// Observe places one valid inference and returns its state.
//
// # How circularity is prevented
//
// The ONLY input is `regions` — this inference's raw detections, straight off the detector.
// Not track identities, not track membership, not anything the tracker has concluded. State is
// therefore decided from independent screen evidence BEFORE any track is touched, and the
// tracker then accumulates within a state it had no part in choosing.
//
// The alternative — signing a state with the set of track IDs present — would be circular:
// tracks would define the state that defines their own eligibility, and a track could never be
// found absent in the state it was minting. TestStateIdentityDoesNotDependOnTracking is what
// holds this open.
func (g *ScreenSegmenter) Observe(n int, regions []ShadowRegion,
	inputs []InputEvent, sem SemanticEvidence) ScreenStateID {

	sig := NewScreenSignature(regions)
	f := sig.features()

	// Best MATCH and best EXPLANATION are different questions and are asked separately. The
	// state most similar to this frame need not be the one that accounts for it: a sparse
	// frame is fully contained in a rich screen it barely resembles.
	best, bestSim, bestCover := -1, -1.0, 0.0
	for i, st := range g.states {
		m := st.mean()
		if sim := signatureSimilarity(m, f); sim > bestSim {
			best, bestSim = i, sim
		}
		if c := coverage(f, m); c > bestCover {
			bestCover = c
		}
	}

	// THE SECOND QUESTION, and the one that was missing.
	//
	// "How alike are these overall" and "was a coherent part of this replaced" are different
	// questions with different answers, and only the first was ever asked. A surface can be
	// 92% the same and be somewhere else entirely; it can be 90% the same and be exactly where
	// it was with somebody scrolling. See localChange.
	//
	// Asked of the state the global comparison chose, because the question only means
	// something relative to a particular remembered composition.
	localWorst, localAt := 1.0, ""
	if best >= 0 {
		localWorst, localAt = localChange(g.states[best].mean(), f)
	}
	sameSurface := best >= 0 && bestSim >= StateMatchSimilarity
	// localWorst is 1 when nothing coherent was replaced — see localChange, which returns
	// exactly that when the replaced composition does not clear the resolution bar. So the
	// decision needs no threshold of its own: either a part of the surface is now made of
	// something it was not, or it is not.
	samePlace := sameSurface && localWorst == 1

	// THE measurement. What the comparisons scored, before anything is decided from them.
	//
	// Recorded here rather than at each branch so it cannot fall out of step with the
	// decision it describes: one call, on the same values the switch below reads.
	g.Match.record(bestSim, samePlace)
	g.Match.recordLocal(localWorst, sameSurface && !samePlace)

	var id ScreenStateID
	switch {
	case samePlace:
		st := g.states[best]
		st.fold(f)
		id = st.ID

	case sameSurface:
		// The surface is recognisably the same and a coherent part of it has been replaced.
		// That is a different STATE of the same surface — the case a single global identity
		// could never express, because expressing it there would have meant lowering the bar
		// until ordinary use became a storm of new screens.
		//
		// HELD rather than acted on. A frame is not a state: something that changed and
		// changed back is a dropdown, a tooltip or a redraw, and only something that STAYS
		// is a place. The existing promotion machinery already says exactly this about
		// ambiguous compositions, and it says it here for the same reason.
		st := g.promote(f)
		if st == nil {
			return g.unplaced(inputs)
		}
		// The new state belongs to the SAME surface as the one it changed away from, and
		// saying so is what lets everything downstream keep both truths: the application is
		// still the application, and the place inside it is not the place it was.
		st.Surface = g.states[best].surfaceOf()
		st.LocalFrom, st.LocalCell = g.states[best].ID, localAt
		st.fold(f)
		id = st.ID
	case bestCover >= StateContainment:
		// Everything here is already accounted for by a screen Marco knows, there is just
		// less of it. That is what a partial rendering looks like — and also what a genuinely
		// sparse screen looks like when a rich one happened to be seen first. The two are
		// indistinguishable in a single frame, and the difference is that one of them COMES
		// BACK.
		//
		// So an ambiguous composition is held rather than judged, and recurrence promotes it.
		// This costs the first sighting, which stays unknown and grants nobody eligibility;
		// that is the honest price of not guessing. It is also why gameplay does not become a
		// permanent transition just because the session opened on a menu.
		if st := g.promote(f); st != nil {
			st.fold(f)
			id = st.ID
			break
		}
		return g.unplaced(inputs)
	default:
		st := g.mint()
		if st == nil {
			g.EvictedStates++
			return g.unplaced(inputs)
		}
		st.fold(f)
		id = st.ID
	}

	st := g.state(id)
	if st.Inferences == 0 {
		st.FirstInference = n
	}
	st.Inferences++
	st.LastInference = n
	// A state re-entered after anything else — including an unplaceable transition — begins a
	// new episode. Conservative on purpose: the screen may well have stayed put behind an
	// unknown frame, but claiming it did would be inventing continuity Marco cannot see.
	if g.current != id {
		st.Episodes++
	}
	st.creditTerms(admissibleTerms(sem))
	g.note(g.current, id, inputs)
	g.current = id
	// AFTER the note, which is what reads the run this placement ends.
	g.unsettled = 0
	return id
}

// creditTerms records this inference's semantic evidence against the state it was seen in.
//
// Terms are credited to the state that was ACTIVE, which is what gives them a denominator and
// therefore a meaning. A term read during a transition frame belongs to no state and is
// dropped: the alternative is attributing a loading screen's text to whatever came next.
func (s *ScreenState) creditTerms(sem SemanticEvidence) {
	if sem.EditableFields > s.EditableFields {
		s.EditableFields = sem.EditableFields
	}
	// An OPPORTUNITY to read, counted whether or not anything matched. An inference where
	// perception had text and matched no concept is evidence that the screen carries none;
	// an inference where it had nothing to read is not evidence about the screen at all.
	if sem.Observed {
		s.TermObservations++
	}
	// The NAME this inference offered, credited to the state that was active — the same
	// denominator terms get, and for the same reason.
	if n := sem.PlaceName; n != "" {
		if s.PlaceNames == nil {
			s.PlaceNames = map[string]int{}
		}
		if _, known := s.PlaceNames[n]; known || len(s.PlaceNames) < MaxTermsPerState {
			s.PlaceNames[n]++
		}
	}
	if len(sem.Terms) == 0 {
		return
	}
	if s.Terms == nil {
		s.Terms, s.TermEpisodes = map[InterfaceTerm]int{}, map[InterfaceTerm]int{}
		s.termEpisode = map[InterfaceTerm]int{}
	}
	for _, t := range sem.Terms {
		if _, known := s.Terms[t]; !known && len(s.Terms) >= MaxTermsPerState {
			continue
		}
		s.Terms[t]++
		// One credit per episode. A five-minute stare at a settings screen is one
		// opportunity for the word SETTINGS to be a coincidence, not sixty.
		if s.termEpisode[t] != s.Episodes {
			s.termEpisode[t] = s.Episodes
			s.TermEpisodes[t]++
		}
	}
}

// MaxTermsPerState bounds one state's semantic tally.
const MaxTermsPerState = 16

func copyTerms(in map[InterfaceTerm]int) map[InterfaceTerm]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[InterfaceTerm]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// promote records an ambiguous composition and returns a state once it has recurred.
//
// Returns nil while the composition has been seen fewer than StatePromotionCount times, which
// is the caller's signal to leave the inference unplaced.
//
// # Why the count is applied once, at the end
//
// It used to be applied inside the match loop, with the first sighting appended afterwards and
// its count set to one — so a first sighting could never promote whatever the constant said, and
// setting the constant to 1 changed no behaviour at all. That is a mutation that survives by
// construction, which is the same thing as an untested constant: nothing could tell "the policy
// is two sightings" from "the policy is whatever, and the code happens to do two".
//
// Recording the sighting first and asking one question about it afterwards makes the constant
// mean what it says at every value. Production behaviour at 2 is unchanged.
func (g *ScreenSegmenter) promote(f map[string]float64) *ScreenState {
	seen := g.pendingFor(f)
	seen.n++
	if seen.n < StatePromotionCount {
		return nil
	}
	st := g.mint()
	if st == nil {
		g.EvictedStates++
		return nil
	}
	// Folded once here for the sighting that promoted it; the caller folds the current frame.
	// The earlier unplaced sightings are deliberately not back-dated — they were reported as
	// unknown and rewriting history would make the counts unreproducible.
	st.fold(seen.f)
	g.drop(seen)
	return st
}

// pendingFor is the unpromoted composition this frame belongs to, recording a new one if none.
func (g *ScreenSegmenter) pendingFor(f map[string]float64) *pendingComposition {
	for _, p := range g.pending {
		if signatureSimilarity(p.f, f) >= StateMatchSimilarity {
			return p
		}
	}
	if len(g.pending) >= MaxScreenStates {
		// Bounded like everything else. The oldest unpromoted composition goes first: one
		// that has not recurred by now is a transition, which is what it was recorded as.
		g.pending = g.pending[1:]
	}
	p := &pendingComposition{f: f}
	g.pending = append(g.pending, p)
	return p
}

func (g *ScreenSegmenter) drop(target *pendingComposition) {
	for i, p := range g.pending {
		if p == target {
			g.pending = append(g.pending[:i], g.pending[i+1:]...)
			return
		}
	}
}

func (g *ScreenSegmenter) mint() *ScreenState {
	if len(g.states) >= MaxScreenStates {
		return nil
	}
	g.next++
	st := &ScreenState{ID: ScreenStateID(fmt.Sprintf("state_%d", g.next))}
	g.states = append(g.states, st)
	return st
}

func (g *ScreenSegmenter) state(id ScreenStateID) *ScreenState {
	for _, st := range g.states {
		if st.ID == id {
			return st
		}
	}
	return &ScreenState{}
}

// note records a transition and the navigation observed before it, bounded.
// unplaced records an inference nobody could place, and lengthens the current unsettled run.
//
// ONE exit for every branch that gives up, so the run length cannot drift out of step with the
// transitions those branches record. It assigns no identity: the sample stays unknown, which is
// the honest description of a frame that could not be read.
func (g *ScreenSegmenter) unplaced(inputs []InputEvent) ScreenStateID {
	g.note(g.current, ScreenStateUnknown, inputs)
	g.current = ScreenStateUnknown
	g.unsettled++
	return ScreenStateUnknown
}

func (g *ScreenSegmenter) note(from, to ScreenStateID, inputs []InputEvent) {
	if from == "" || from == to {
		return
	}
	// THE walk, before the aggregate. Order is the one thing folding into a map destroys,
	// and it is what tells `A → ? → B → ? → C` from `A → ? → C` with a stray excursion.
	//
	// No evidence rides here: what preceded a change stays on the transition record, which
	// already insists on an unambiguous order before anybody may read it as a procedure
	// (legOf). This says only that the change happened, and when.
	if len(g.crossings) < MaxCrossings {
		c := Crossing{From: from, To: to}
		if from == ScreenStateUnknown {
			// How long Marco could not say where the user was, for THIS interval —
			// not the longest such run the edge ever crossed, which is what the
			// aggregate keeps and is the wrong number to bound one crossing by.
			c.Run = g.unsettled
		}
		g.crossings = append(g.crossings, c)
	} else {
		g.EvictedCrossings++
	}
	if g.transitions == nil {
		g.transitions = map[[2]ScreenStateID]*transitionRecord{}
	}
	k := [2]ScreenStateID{from, to}
	rec, ok := g.transitions[k]
	if !ok {
		if len(g.transitions) >= MaxStateTransitions {
			g.EvictedTransitions++
			return
		}
		rec = &transitionRecord{preceded: map[NavIntent]int{}}
		g.transitions[k] = rec
	}
	rec.count++
	// How long Marco could not say where the user was, immediately before this change.
	// The LONGEST such run this edge has ever crossed — see ScreenTransition.UnsettledRun.
	if from == ScreenStateUnknown && g.unsettled > rec.unsettledRun {
		rec.unsettledRun = g.unsettled
	}

	// Each DISTINCT intent counts once per transition. A held direction key delivering
	// forty events must not outvote a single deliberate confirm — the question is which
	// intents were present before the change, not how many samples the hook produced.
	seen := map[NavIntent]bool{}
	// The ORDER, kept beside the set rather than instead of it. Inputs arrive in time
	// order — the producer stamps them from the session's own clock and Drain sorts on it —
	// so this is a real sequence and not an arbitrary map iteration.
	seq := make([]NavIntent, 0, len(inputs))
	// The targets ride beside the order, aligned by position: what each event was aimed
	// at, when the evidence at the time identified it. Zero entries are the ordinary case.
	targets := make([]SemanticTarget, 0, len(inputs))
	// Whether EVERY intent before this change was one that is only navigation in context.
	// One unambiguous key among them is enough to carry the observation on its own.
	allConditional := true
	for _, e := range inputs {
		seq = append(seq, e.Intent)
		if e.Target != nil {
			targets = append(targets, *e.Target)
		} else {
			targets = append(targets, SemanticTarget{})
		}
		if !e.Conditional {
			allConditional = false
		}
		if seen[e.Intent] {
			continue
		}
		seen[e.Intent] = true
		if _, known := rec.preceded[e.Intent]; !known &&
			len(rec.preceded) >= MaxIntentsPerTransition {
			continue
		}
		rec.preceded[e.Intent]++
	}
	rec.addSequence(seq, targets)
	if len(seen) == 0 {
		rec.unattributed++
	} else if allConditional {
		rec.conditionalOnly++
	}
}

// Crossing is ONE observed change between screens, in the order it was seen.
//
// Deliberately thin. It carries no navigation, no counts and no interpretation — those live on
// the aggregated ScreenTransition, which is still where anything is READ from. A crossing exists
// to answer one question the aggregate cannot: what followed what.
type Crossing struct {
	From ScreenStateID `json:"from"`
	To   ScreenStateID `json:"to"`
	// Run is the length of the run of unplaceable samples that ENDED at this crossing, and
	// is non-zero only when From is ScreenStateUnknown.
	//
	// Per-interval, unlike ScreenTransition.UnsettledRun, which is the longest run that edge
	// ever crossed. Bridging asks about THIS gap.
	Run int `json:"run,omitempty"`
}

// Crossings returns the session's walk, in order.
func (g *ScreenSegmenter) Crossings() []Crossing {
	return append([]Crossing{}, g.crossings...)
}

// States returns the discovered states, most-observed first.
func (g *ScreenSegmenter) States() []ScreenState {
	out := make([]ScreenState, 0, len(g.states))
	for _, st := range g.states {
		c := *st
		// ONE WHOLE COMPOSITION THIS SCREEN ACTUALLY PRESENTED.
		//
		// Not the average of how it rendered while somebody watched, and — since
		// 2026-08-18 — not an assembly of per-role modes either. See settledWhole for
		// the blend it replaced and for the tie rule.
		c.Roles, c.Settled = st.settledWhole()
		// The tallies are copied rather than shared. A returned state is a snapshot, and
		// aliasing the live maps would let a caller's read change while it reads.
		c.Compositions = map[string]int{}
		for k, seen := range st.compositions {
			c.Compositions[k] = seen.n
		}
		c.Terms, c.TermEpisodes = copyTerms(st.Terms), copyTerms(st.TermEpisodes)
		c.sum, c.n, c.termEpisode = nil, 0, nil
		c.compositions = nil
		out = append(out, c)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Inferences != out[b].Inferences {
			return out[a].Inferences > out[b].Inferences
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// Transitions returns the observed state changes, most frequent first.
//
// Counts only. No action is inferred from a transition here — "state_2 followed state_1 three
// times" is evidence a later layer may use to discover an action, and naming it `pause` at this
// level would be the same interpretation leak the numbered state IDs exist to prevent.
func (g *ScreenSegmenter) Transitions() []ScreenTransition {
	out := make([]ScreenTransition, 0, len(g.transitions))
	for k, rec := range g.transitions {
		t := ScreenTransition{
			From: k[0], To: k[1], Count: rec.count, Unattributed: rec.unattributed,
			ConditionalOnly: rec.conditionalOnly, UnsettledRun: rec.unsettledRun,
		}
		if len(rec.preceded) > 0 {
			t.Preceded = make(map[NavIntent]int, len(rec.preceded))
			for intent, c := range rec.preceded {
				t.Preceded[intent] = c
			}
		}
		for _, s := range rec.sequences {
			c := TargetedSequence{
				Intents: append([]NavIntent{}, s.Intents...), Count: s.Count,
			}
			if len(s.Targets) > 0 {
				c.Targets = append([]SemanticTarget{}, s.Targets...)
			}
			t.Sequences = append(t.Sequences, c)
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		if out[a].From != out[b].From {
			return out[a].From < out[b].From
		}
		return out[a].To < out[b].To
	})
	return out
}

// LocalChangeForTest exposes the local comparison for the characterisation tests that measure
// what it can and cannot see. Production reads it through Observe.
func LocalChangeForTest(a, b map[string]float64) (float64, string) { return localChange(a, b) }

// FeaturesForTest exposes the feature vector a signature is compared as, so a test measuring
// the comparison feeds it exactly what production does rather than a hand-built map.
func FeaturesForTest(s ScreenSignature) map[string]float64 { return s.features() }

// SurfaceOf is the surface this state belongs to, which is itself when it has no other.
//
// Exported so a reader — a report, a diagnostic, a test — can ask which application surface a
// state is a state OF without knowing that a state with no surface is its own.
func (s ScreenState) SurfaceOf() SurfaceID {
	if s.Surface != "" {
		return s.Surface
	}
	return SurfaceID(s.ID)
}

// settledWhole is the composition this state is remembered by, and whether it may be.
//
// # A place may not be remembered as something it never was
//
// The producer this replaced asked `settledCount(role)` for each role separately and assembled
// the answers. Roles were moded independently, so a state whose samples disagreed could emit a
// composition equal to none of them. Proven in isolation:
//
//	observed  {18,26,49} {18,26,49} {17,27,49} {17,27,49} {17,26,45}
//	emitted   {17,26,49}   ← no sample ever showed this
//
// That is how the last surviving twin got into a live store. So promotion now names one WHOLE
// composition that actually occurred, and the durable identity is a thing Marco saw rather than
// an average of things it saw.
//
// # The threshold is the existing one
//
// `StatePromotionCount`: one sighting is a transition frame, a second is a screen. It is the same
// rule the per-role tally used, applied to the composition instead of to each role. Measured
// across eleven real states — three Settings pages, VS Code, Discord, and sessions spanning page
// transitions and resizes — the winning composition recurred 14 to 53 times, and no state that
// established under the old producer failed to establish under this one.
//
// # Ties, deliberately
//
// Most recurrences wins. On a tie, the LARGER composition wins — the same preference the old
// per-role tiebreak had, and for the same reason: a screen caught part-way through rendering is
// smaller than the screen, not a different one.
//
// If two compositions are equally frequent AND the same size, nothing here can say which the
// screen is, and lexicographic order is not a reason. The state is left UNRESOLVED: `ok` is
// false, so it cannot be promoted. Failing closed costs a place that would have been a coin toss.
func (s *ScreenState) settledWhole() (map[string]int, bool) {
	best, bestTied := (*seenComposition)(nil), false
	for _, c := range s.compositions {
		switch {
		case best == nil:
			best = c
		case c.n > best.n:
			best, bestTied = c, false
		case c.n == best.n:
			switch {
			case total(c.roles) > total(best.roles):
				best, bestTied = c, false
			case total(c.roles) == total(best.roles):
				bestTied = true
			}
		}
	}
	if best == nil || bestTied {
		return nil, false
	}
	return copyRoles(best.roles), best.n >= StatePromotionCount
}

// total is how many structures a composition has in it.
func total(roles map[string]int) int {
	n := 0
	for _, v := range roles {
		n += v
	}
	return n
}
