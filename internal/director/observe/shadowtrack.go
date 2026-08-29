package observe

import (
	"fmt"
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Turning repeated detections into structures that persist.
//
// # The gap this closes
//
// A live session could say "ScreenParser produced 179 detections". It could not say "this same
// button-like thing was there for fourteen inferences, went away during play, and came back
// with the menu". The first is a count; the second is an entity, and only the second can carry
// a capability. A capability built on a box that happened once is built on nothing.
//
// # What a track is NOT
//
// Not a World State entity, not an Action Graph node, not an executable target. A track is a
// session-local observation that some region kept recurring. Identities are minted per session
// and are meaningless outside it — deliberately, because a durable id would invite exactly the
// promotion this milestone forbids.
//
// # The rule everything here depends on
//
// A SKIPPED INFERENCE IS NOT AN ABSENCE. The five-minute session skipped 48 of 150 slots to
// keep its cadence honest; if those counted against presence, every element would look like it
// flickered and nothing would ever be stable. Presence is measured against inferences that
// actually ran AND proved their target. Everything else is unknown, and unknown stays unknown.

// admissibleRegions keeps the regions that are structural evidence.
//
// Shape only, and deliberately not an allowlist: role vocabularies belong to the providers and
// they grow — the live sessions reported `link`, `combo_box`, `tab_list`, `progress_bar` and
// `scroll_bar`, none of which any list in this repository had — so an allowlist here would
// silently discard most of a real accessibility tree while looking careful.
//
// Returns the input unchanged when nothing is refused, which is the ordinary case: this must
// not allocate a copy of a 128-region frame twice a second for nothing.
func admissibleRegions(in []ShadowRegion) []ShadowRegion {
	ok := true
	for _, r := range in {
		if !roleShaped(r.Role) {
			ok = false
			break
		}
	}
	if ok {
		return in
	}
	out := make([]ShadowRegion, 0, len(in))
	for _, r := range in {
		if roleShaped(r.Role) {
			out = append(out, r)
		}
	}
	return out
}

// ShadowRegion is one detection, in identity-free geometry.
//
// Window-relative and normalised: absolute screen coordinates are matching evidence at most,
// never identity, and a durable key built from a pixel position would break the moment the
// window moved.
type ShadowRegion struct {
	Role       string  `json:"role"`
	Region     Region  `json:"region"`
	Confidence float64 `json:"confidence,omitempty"`
	Nameable   bool    `json:"nameable"`
	// Chrome says this belongs to the WINDOW rather than to the page — a title bar, a
	// scroll bar, or anything inside one. Classified where the accessibility hierarchy is
	// still available and carried here, because by the time a region reaches identity the
	// parent links are long gone.
	//
	// It is a label, not a removal: chrome is still observed, still addressable, and still
	// shown. Exactly one consumer reads it — the durable place signature.
	Chrome bool `json:"chrome,omitempty"`
	// Kind says who accounted for what this structure IS. See KindEvidence.
	//
	// The same shape as Chrome and for the same reason: classified where the evidence and
	// its provenance are both still in scope, carried here because by the time a region
	// reaches identity there is nothing left to ask. It is a label, not a removal — the
	// region is still tracked, still counted toward whether the window was read, and still
	// reported. One consumer reads it, and it is the durable place signature.
	Kind directorapi.KindEvidence `json:"kind_evidence,omitempty"`
}

const (
	// TrackMatchIoU is the overlap at which a detection continues an existing track.
	//
	// 0.30, below the benchmark's 0.35, because a live game jitters more than a still
	// corpus: a HUD element under camera motion shifts a few pixels between frames and must
	// stay one identity. Fragmenting one control into six is the failure this guards.
	TrackMatchIoU = 0.30

	// TrackAbsenceTolerance is how many VALID inferences may miss a track before its
	// episode ends.
	//
	// One. A confidence dip for a single frame should not split an episode in two; two
	// consecutive misses is the detector agreeing with itself that the thing has gone.
	// Skipped slots are not counted here at all — only inferences that really ran.
	TrackAbsenceTolerance = 1

	// TrackReacquireGap is how many valid inferences a track may be absent and still resume
	// rather than be replaced.
	//
	// Generous, because the question a track answers is whether an interface element
	// RECURS, not whether it is literally the same native control. A pause menu returning
	// after a minute of play is the same recurring structure.
	TrackReacquireGap = 120

	// MaxActiveTracks and MaxRetiredTracks bound one session's memory.
	//
	// # Why these numbers, and why they changed
	//
	// 128 was sized when a session's evidence came from a detector reporting a handful of
	// boxes per frame. A fused accessibility tree reports 52 (Chrome), 128 (VS Code) and 128
	// (File Explorer) stable structures for ONE screen — so the old bound was exactly one
	// realistic screen, and the SECOND screen a session visited could begin no tracks at all.
	//
	// The consequence was total and silent. No tracks means no structural group, no group
	// means no hypothesis, no hypothesis means no durable subject, and a relationship needs
	// BOTH endpoints to be durable subjects. So on real software every session could observe
	// a transition and none could ever remember one. A deterministic two-screen session
	// evicted 1,460 tracks and produced structure for one screen out of two.
	//
	// The new bound admits roughly eight realistic screens. A ShadowTrack is on the order of
	// 600 bytes once its per-state evidence is counted, so 1024 active and 2048 retired is
	// under two megabytes — against an observation history that already retains five cycles
	// of a 685-element accessibility tree with labels, sources and geometry. The bound is
	// still a bound; it is now above the size of the thing it is bounding rather than below it.
	//
	// The cost is in `assign`, which is O(regions x tracks) per inference. See
	// BenchmarkTrackAssignmentAtAccessibilityScale for the measurement.
	MaxActiveTracks  = 1024
	MaxRetiredTracks = 2048
)

// TemporalShape is the closed, GAME-AGNOSTIC vocabulary for a track's behaviour.
//
// Deliberately not "HUD", "menu" or "dialog". Those are interpretations, and interpretation
// belongs to a capability pack. This layer discovers structure; it does not name games.
type TemporalShape string

const (
	ShapePersistent TemporalShape = "persistent"
	ShapeBursty     TemporalShape = "bursty"
	ShapeTransient  TemporalShape = "transient"
	ShapeRare       TemporalShape = "rare"
	ShapeUnstable   TemporalShape = "unstable"
)

// ShadowTrack is one recurring structure, as a session remembers it.
type ShadowTrack struct {
	ID   string `json:"id"`
	Role string `json:"role"`

	FirstInference int `json:"first_inference"`
	LastInference  int `json:"last_inference"`
	// Seen is how many VALID inferences found this track; Eligible is how many it could
	// have been found in. The pair is the whole point — a ratio against total samples would
	// count the cadence's skips as disappearances.
	Seen     int `json:"seen"`
	Eligible int `json:"eligible"`
	Episodes int `json:"episodes"`

	MeanConfidence float64 `json:"mean_confidence"`
	MinConfidence  float64 `json:"min_confidence"`
	MaxConfidence  float64 `json:"max_confidence"`

	// MeanIoU is the average overlap of each observation with the track's reference
	// geometry — the clearest single statement of whether a region sits still.
	MeanIoU      float64 `json:"mean_iou"`
	CentreDrift  float64 `json:"centre_drift"`
	SizeVariance float64 `json:"size_variance"`
	Reference    Region  `json:"reference"`

	Nameable bool          `json:"nameable"`
	Shape    TemporalShape `json:"shape"`
	Present  bool          `json:"present"`

	// RoleChanges counts compatible regions that arrived under a different role. Surfaced
	// rather than smoothed: a region that is a button in one frame and an image in the next
	// is unstable evidence, and hiding that would manufacture confidence.
	RoleChanges int `json:"role_changes"`

	// States is this track's evidence PER SCREEN STATE, which is the only denominator that
	// makes presence mean anything for a state-dependent control.
	//
	// Seen/Eligible above are the SESSION-global figures and are deliberately unchanged: a
	// menu button really is absent for half a session that is half gameplay, and reporting
	// otherwise would hide a genuine detector failure behind a favourable denominator.
	// Both are true, they answer different questions, and the reader gets both.
	States []TrackState `json:"states,omitempty"`

	confSum   float64
	iouSum    float64
	centreSum float64
	areaSum   float64
	areaSqSum float64
	missRun   int
	lastSeen  int
}

// TrackState is one track's evidence within one screen state.
//
// The unit that fixes the meaning of presence: Seen over the inferences in which THIS state was
// active, so a pause-menu button is measured against pause-menu frames and a gameplay HUD icon
// against gameplay frames. Neither is penalised for the screen it does not live on.
type TrackState struct {
	State ScreenStateID `json:"state"`
	// Seen and Eligible are both state-local. Eligible advances only on valid inferences
	// confidently placed in this state — never on a skipped slot, a failed inference, an
	// unproven target, an unplaceable transition, or an inference in another state.
	Seen     int `json:"seen"`
	Eligible int `json:"eligible"`
	Episodes int `json:"episodes"`
	// Shape is the temporal shape WITHIN this state. A menu control is routinely bursty
	// globally and persistent here, and both statements are correct.
	Shape TemporalShape `json:"shape"`

	present  bool
	missRun  int
	lastSeen int
}

// PresenceRatio is state-local Seen over state-local Eligible.
func (s TrackState) PresenceRatio() float64 {
	if s.Eligible == 0 {
		return 0
	}
	return float64(s.Seen) / float64(s.Eligible)
}

// PresenceRatio is Seen over Eligible, zero when nothing was eligible.
//
// SESSION-GLOBAL. For a state-dependent control this is not the number that says whether it is
// reliable — see StateIn and TrackState.PresenceRatio.
func (t ShadowTrack) PresenceRatio() float64 {
	if t.Eligible == 0 {
		return 0
	}
	return float64(t.Seen) / float64(t.Eligible)
}

// StateIn returns this track's evidence within one state.
func (t ShadowTrack) StateIn(id ScreenStateID) (TrackState, bool) {
	for _, s := range t.States {
		if s.State == id {
			return s, true
		}
	}
	return TrackState{}, false
}

// PrimaryState is the state this track has the most evidence in, if any.
//
// The state a capability would reason about: "this control belongs to that screen".
func (t ShadowTrack) PrimaryState() (TrackState, bool) {
	best, ok := TrackState{}, false
	for _, s := range t.States {
		if !ok || s.Seen > best.Seen || (s.Seen == best.Seen && s.State < best.State) {
			best, ok = s, true
		}
	}
	return best, ok
}

// StateStable reports whether this track is a reliable structure WITHIN some screen state.
//
// The capability-relevant question, and the one global Stable() answers wrongly for anything
// state-dependent. Geometry still has to hold up — a state-local denominator excuses absence in
// the wrong state, never a control that wobbles in its own.
func (t ShadowTrack) StateStable() bool {
	s, ok := t.PrimaryState()
	if !ok {
		return false
	}
	switch s.Shape {
	case ShapePersistent, ShapeBursty:
		return t.GeometryStable()
	}
	return false
}

// GeometryStable reports whether this track held still enough to be worth acting on later.
func (t ShadowTrack) GeometryStable() bool {
	return t.Seen >= 2 && t.MeanIoU >= 0.60 && t.RoleChanges == 0
}

// Stable reports whether a track is a real recurring structure.
func (t ShadowTrack) Stable() bool {
	switch t.Shape {
	case ShapePersistent, ShapeBursty:
		return t.GeometryStable()
	}
	return false
}

// ShadowTracker follows recurring shadow structures across one session.
type ShadowTracker struct {
	tracks  []*ShadowTrack
	retired []*ShadowTrack
	// inferences counts VALID inferences only — the denominator every GLOBAL presence ratio
	// uses. State-local presence uses ScreenState.Inferences instead.
	inferences int
	nextID     int
	Evicted    int

	// states segments the session by screen composition. Its input is raw detections only,
	// so state is decided independently of anything tracking has concluded.
	states ScreenSegmenter
	// current is the state the most recent valid inference placed the screen in, exposed so a
	// demonstration capture can ask "where is the user now" without re-segmenting.
	current ScreenStateID
	// pendingInputs is navigation banked since the last valid inference, bounded.
	pendingInputs []InputEvent
	// log is the session's whole admitted-input record, kept apart from pendingInputs on
	// purpose: the buffer is attribution evidence and expires by design, the log is capture
	// evidence and must not. See inputlog.go.
	log InputLog
	// inputSinceInference says navigation arrived since the last inference, including
	// during slots the detector sat out.
	inputSinceInference bool
	// quietInferences counts consecutive inferences that carried no navigation.
	quietInferences int
	// EvictedAssociations counts track-state associations dropped at MaxTrackStates.
	EvictedAssociations int
}

// States returns the screen states this session discovered.
func (k *ShadowTracker) States() []ScreenState {
	out := k.states.States()
	// Tracks is reported per state so a reader can tell a rich screen from a bare one
	// without cross-referencing the track table by hand.
	for i := range out {
		for _, t := range k.tracks {
			if _, ok := (ShadowTrack{States: t.States}).StateIn(out[i].ID); ok {
				out[i].Tracks++
			}
		}
	}
	return out
}

// Transitions returns the observed screen-state changes.
func (k *ShadowTracker) Transitions() []ScreenTransition { return k.states.Transitions() }

// Crossings returns those same changes in the order they were seen.
func (k *ShadowTracker) Crossings() []Crossing { return k.states.Crossings() }

// Observe folds one cycle into the screen model and the tracks.
//
// TWO inputs, because they fail independently and always did. `s` is the structural
// detector's own record — its cadence, its latency, its navigation and its semantic
// evidence — and `structure` is the composition this frame actually presented, from
// whichever admissible source described it. Before this milestone they were the same value,
// which is why a Director with no detector had no screens at all.
//
// Returns immediately for anything that is not a valid observation of the session's window.
// A slot nothing looked at, a failed inference and an unproven target are all UNKNOWN, and
// unknown must never be recorded as "the element was not there".
func (k *ShadowTracker) Observe(s *ShadowSample, structure StructuralView) {
	if s == nil {
		s = &ShadowSample{}
	}
	// Navigation accumulates across slots that carry no screen evidence, and so is banked
	// BEFORE the early return. A skipped slot means the detector sat out; the player kept
	// playing through it, and discarding that input would lose exactly the keypress that
	// preceded the change the next inference is about to see. Screen evidence and input
	// evidence fail independently, so they must be gated independently.
	fresh := admissibleInputs(s.Inputs)
	// THE capture-first line. The log takes every admitted event before any gate below —
	// the structural return, the quiet expiry, the state change that consumes the buffer —
	// so no failure of interpretation can erase the fact that the person acted. Deleting it
	// must fail TestOneClickSurvivesFailedSemanticResolution.
	k.log.bank(fresh, k.inferences, k.current)
	k.pendingInputs = append(k.pendingInputs, fresh...)
	if len(fresh) > 0 {
		// Since the last INFERENCE, not since the last call. A slot the detector sat out
		// does not advance an inference, and the keypress a player made during one still
		// belongs to the change the next inference is about to see.
		k.inputSinceInference = true
	}
	if len(k.pendingInputs) > MaxBufferedInputs {
		k.pendingInputs = k.pendingInputs[len(k.pendingInputs)-MaxBufferedInputs:]
	}
	if !structure.Observed() {
		// Nothing looked at this frame's composition, so nothing is known about it. NOT an
		// empty screen: minting a state from silence would invent a screen, and every track
		// would then be judged absent from it.
		//
		// This is the gate that used to read `!s.Ran || !s.TargetProven || s.Unavailable`.
		// It is the same rule; it now lives where provenance is decided, so a second
		// structural source cannot arrive without one. See observe/structure.go.
		return
	}
	k.inferences++
	n := k.inferences

	// Navigation is held until the screen CHANGES, not until the next inference.
	//
	// # What draining per inference threw away
	//
	// A person pressing `down down enter` takes a second or so. The sampler runs about twice
	// a second, so that one interaction spans three inferences — and the first two produce no
	// screen change, which means `note` discarded their intents (`from == to` returns early).
	// Only the window containing the final press survived, and every multi-key interaction in
	// this system was recorded as its last key alone.
	//
	// Live, that turned `down down enter` into a learned route of `confirm`: press Enter on
	// whatever happens to be focused. It replayed onto a different screen and the rehearsal
	// correctly refused to believe it. Nothing about the person's demonstration was wrong; the
	// model could not hold an interaction longer than one sampling window.
	//
	// # Why holding is the honest reading
	//
	// The navigation that produced a change is the navigation SINCE THE LAST CHANGE. That is
	// what a leg between two checkpoints already means everywhere else in this subsystem, and
	// `Sequences` exists to record its order — "reconstructing a procedure needs the order",
	// which is unachievable if the order is truncated to whatever landed in the final slot.
	//
	// Bounded by `MaxBufferedInputs`, which was already the bound, so nothing new is invented
	// here and a long idle cannot grow the buffer without limit. The residual cost is that a
	// change following a quiet spell may be credited with keys pressed a while before it —
	// visible as an order that does not recur, which is exactly what the sequence tally and
	// `Unattributed` exist to expose.
	// # And why holding is still not reaching backwards
	//
	// A change is credited with the run only when THIS inference carried input of its own. An
	// interaction is a contiguous thing that ends at the change it caused: the last press and
	// the new screen arrive together, which is exactly how single-key attribution has always
	// worked here. A change that arrives during silence is attributed to nothing, however much
	// is sitting in the buffer — that is nearest-neighbour attribution, and
	// TestAnOldKeypressIsNotForcedOntoALaterEdge forbids it for good reason.
	//
	// So `down down enter` keeps all three, and a confirm pressed during gameplay followed by
	// a scene loading on its own keeps none.
	inputs := k.pendingInputs
	hadInput := k.inputSinceInference
	k.inputSinceInference = false
	if hadInput {
		k.quietInferences = 0
	} else {
		// A PAUSE is not a stop, and the difference is one inference.
		//
		// What bends is how long a run survives a gap, and the answer is the one this file
		// already gives about a track blinking out: "one dip should not split an episode in
		// two; two consecutive misses is the detector agreeing with itself that the thing
		// has gone." A person pressing down, down, enter leaves a quiet inference between
		// presses at two samples a second; somebody who has stopped leaves several.
		//
		// Strict silence was tried first and it was too strict: live, three classified
		// intents still produced a one-key route, because every human pause broke the run.
		//
		// # And an EFFECT may arrive one inference after its cause
		//
		// This used to drop the run on the first quiet inference, so a change was credited
		// only when the input landed in the very same sample. That fits a keyboard menu,
		// which flips the instant the key goes down. It does not fit click-driven desktop
		// software: a Settings page takes a beat to render, so the click is drained on one
		// sample and the composition change is only visible on the next.
		//
		// Measured live against Windows Settings on 2026-08-17: a person clicked through to
		// the Mouse page, both transitions were recorded, and both came out
		// `unattributed 2/2`. The route was discovered and could not be learned —
		// `no_attributed_navigation` — so Learn fell back to asking for a second
		// demonstration, which is exactly the choreography this milestone removes.
		//
		// So the run survives ONE quiet inference and is still offered to the change on it.
		// The bound is `TrackAbsenceTolerance`, which already means precisely "one dip is
		// not a disappearance" — not a number invented for the occasion, and the same
		// discipline bridge.go follows in taking its bound from StatePromotionCount.
		//
		// The nearest-neighbour rule is NOT weakened past that: beyond one quiet inference
		// the run is dropped and a later change is attributed to nothing, however much is
		// sitting in the buffer. A confirm pressed during gameplay followed several
		// inferences later by a scene loading on its own still keeps none, which is what
		// TestAnOldKeypressIsNotForcedOntoALaterEdge holds.
		k.quietInferences++
		if k.quietInferences > TrackAbsenceTolerance {
			inputs = nil
			k.pendingInputs = nil
		}
	}
	before := k.states.current

	// The screen state is decided FIRST, from this inference's raw structure alone, before
	// any track is read or written. That ordering is what makes the model non-circular:
	// evidence determines the state, then tracks accumulate within it. Nothing the tracker
	// has concluded can influence which state it is accumulating into.
	// THE structural admission point. Every region that becomes a signature, a track, a
	// group or a durable subject passes through here exactly once.
	//
	// A region whose kind is not shaped like a kind is not structural evidence: it is text
	// that arrived in the wrong field. Admitting it would put a label into a screen's
	// identity and into a track's role, and from there into the durable store — where it
	// would outlive every session that could have contradicted it. See roleShaped.
	regions := admissibleRegions(structure.Regions)
	state := k.states.Observe(n, regions, inputs, s.Semantic)
	if state != before {
		// The change consumed them. Held otherwise, so the next change is credited with the
		// whole run rather than with its final keystroke.
		k.pendingInputs = nil
	}
	k.current = state

	// Every live track becomes eligible for this inference, found or not. This is the
	// GLOBAL denominator, and it advances only here — once per valid inference.
	for _, t := range k.tracks {
		t.Eligible++
		// The state-local denominator advances only for tracks that already have evidence
		// in the state that is actually active. A track with no association to this state
		// is not being observed at all right now, so this inference is not an opportunity
		// it can fail. That single condition is the whole semantic correction.
		if state != ScreenStateUnknown {
			if ts := t.stateIn(state); ts != nil {
				ts.Eligible++
			}
		}
	}

	matched := k.assign(regions)
	for i, r := range regions {
		if ti, ok := matched[i]; ok {
			k.tracks[ti].record(n, r, state, k)
			continue
		}
		k.begin(n, r, state)
	}

	// Only now does anything count as missing. These are inferences that really ran, so
	// not finding a track here is genuine absence evidence.
	found := map[int]bool{}
	for _, ti := range matched {
		found[ti] = true
	}
	for i, t := range k.tracks {
		if found[i] || t.lastSeen == n {
			continue
		}
		t.missRun++
		if t.Present && t.missRun > TrackAbsenceTolerance {
			t.Present = false
		}
		// State-local absence, under the same four-way rule the global path already
		// obeys plus one more: absence counts only when the track's OWN state is the
		// active one. A different state active, or an unplaceable transition, is not
		// evidence about this track. Skipped slots, failures and unproven targets never
		// reach here at all.
		if state != ScreenStateUnknown {
			if ts := t.stateIn(state); ts != nil {
				ts.missRun++
				if ts.present && ts.missRun > TrackAbsenceTolerance {
					ts.present = false
				}
			}
		}
	}
	k.evict()
}

// assign maps detection index to track index, deterministically.
//
// Greedy by descending overlap with every tie broken by a stable key. Map iteration order must
// never decide identity: two runs over the same evidence producing different tracks would make
// every downstream number irreproducible.
func (k *ShadowTracker) assign(regions []ShadowRegion) map[int]int {
	type pair struct {
		det, track int
		score      float64
	}
	var cands []pair
	for di, r := range regions {
		for ti, t := range k.tracks {
			if !roleCompatible(t.Role, r.Role) {
				continue
			}
			if k.inferences-t.lastSeen > TrackReacquireGap {
				continue
			}
			if o := regionIoU(t.Reference, r.Region); o >= TrackMatchIoU {
				cands = append(cands, pair{di, ti, o})
			}
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].score != cands[b].score {
			return cands[a].score > cands[b].score
		}
		if cands[a].track != cands[b].track {
			return cands[a].track < cands[b].track
		}
		return cands[a].det < cands[b].det
	})

	out := map[int]int{}
	usedD, usedT := map[int]bool{}, map[int]bool{}
	for _, c := range cands {
		if usedD[c.det] || usedT[c.track] {
			continue
		}
		usedD[c.det], usedT[c.track] = true, true
		out[c.det] = c.track
	}
	return out
}

// roleCompatible reports whether a detection may continue a track.
//
// Exact match only. A button must not silently become an image because the geometry overlaps —
// that would let one wandering false positive absorb every role in turn and then report itself
// as a stable structure.
func roleCompatible(trackRole, detRole string) bool { return trackRole == detRole }

func (k *ShadowTracker) begin(n int, r ShadowRegion, state ScreenStateID) {
	if len(k.tracks) >= MaxActiveTracks {
		k.Evicted++
		return
	}
	k.nextID++
	t := &ShadowTrack{
		ID: fmt.Sprintf("shadow_%d", k.nextID), Role: r.Role,
		FirstInference: n, Reference: r.Region, Nameable: r.Nameable,
		Eligible: 1, Present: true,
		MinConfidence: r.Confidence, MaxConfidence: r.Confidence,
	}
	k.tracks = append(k.tracks, t)
	t.record(n, r, state, k)
	t.Episodes = 1
}

// stateIn returns the mutable association for a state, or nil when the track has none.
func (t *ShadowTrack) stateIn(id ScreenStateID) *TrackState {
	for i := range t.States {
		if t.States[i].State == id {
			return &t.States[i]
		}
	}
	return nil
}

// associate returns the association for a state, creating it on first sight there.
//
// A new association starts with Eligible 1, mirroring how a new track starts globally: the
// inference that discovered it is an opportunity it took.
func (t *ShadowTrack) associate(id ScreenStateID, k *ShadowTracker) *TrackState {
	if ts := t.stateIn(id); ts != nil {
		return ts
	}
	if len(t.States) >= MaxTrackStates {
		if k != nil {
			k.EvictedAssociations++
		}
		return nil
	}
	t.States = append(t.States, TrackState{State: id, Eligible: 1, present: true})
	return &t.States[len(t.States)-1]
}

// record folds one observation into a track.
func (t *ShadowTrack) record(n int, r ShadowRegion, state ScreenStateID, k *ShadowTracker) {
	if t.Seen > 0 && !t.Present {
		// Returning after real absence begins a new episode. This is what separates a
		// menu-like structure from a permanent one.
		t.Episodes++
	}
	// An unplaceable inference contributes geometry and global presence but no state
	// evidence. Guessing which state a transition frame belonged to is precisely how a
	// state model acquires memberships it cannot defend.
	if state != ScreenStateUnknown {
		if ts := t.associate(state, k); ts != nil {
			if ts.Seen > 0 && !ts.present {
				ts.Episodes++
			}
			ts.Seen++
			ts.present, ts.missRun, ts.lastSeen = true, 0, n
			if ts.Episodes == 0 {
				ts.Episodes = 1
			}
		}
	}
	if t.Seen > 0 && t.Role != r.Role {
		t.RoleChanges++
	}
	t.Seen++
	t.LastInference, t.lastSeen = n, n
	t.Present, t.missRun = true, 0

	t.confSum += r.Confidence
	t.MeanConfidence = t.confSum / float64(t.Seen)
	if t.Seen == 1 || r.Confidence < t.MinConfidence {
		t.MinConfidence = r.Confidence
	}
	if r.Confidence > t.MaxConfidence {
		t.MaxConfidence = r.Confidence
	}

	t.iouSum += regionIoU(t.Reference, r.Region)
	t.MeanIoU = t.iouSum / float64(t.Seen)
	t.centreSum += centreDistance(t.Reference, r.Region)
	t.CentreDrift = t.centreSum / float64(t.Seen)
	area := r.Region.Width * r.Region.Height
	t.areaSum += area
	t.areaSqSum += area * area
	mean := t.areaSum / float64(t.Seen)
	t.SizeVariance = math.Max(0, t.areaSqSum/float64(t.Seen)-mean*mean)
	if r.Nameable {
		t.Nameable = true
	}
}

// evict bounds the tracker, reporting rather than silently discarding.
func (k *ShadowTracker) evict() {
	if len(k.tracks) <= MaxActiveTracks {
		return
	}
	sort.SliceStable(k.tracks, func(a, b int) bool { return k.tracks[a].Seen > k.tracks[b].Seen })
	for _, t := range k.tracks[MaxActiveTracks:] {
		if len(k.retired) < MaxRetiredTracks {
			k.retired = append(k.retired, t)
			continue
		}
		k.Evicted++
	}
	k.tracks = k.tracks[:MaxActiveTracks]
}

// Tracks returns every track with its temporal shape resolved, best evidence first.
func (k *ShadowTracker) Tracks() []ShadowTrack {
	all := append(append([]*ShadowTrack(nil), k.tracks...), k.retired...)
	out := make([]ShadowTrack, 0, len(all))
	for _, t := range all {
		c := *t
		c.Shape = c.classify()
		// Deep-copied: a shallow copy would share the association backing array with the
		// live track, and resolving shapes into it would mutate tracker state from a
		// reporting call.
		c.States = append([]TrackState(nil), t.States...)
		for i := range c.States {
			c.States[i].Shape = c.classifyState(c.States[i])
		}
		sort.SliceStable(c.States, func(a, b int) bool {
			if c.States[a].Seen != c.States[b].Seen {
				return c.States[a].Seen > c.States[b].Seen
			}
			return c.States[a].State < c.States[b].State
		})
		out = append(out, c)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Seen != out[b].Seen {
			return out[a].Seen > out[b].Seen
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// Inferences is the valid-inference denominator.
func (k *ShadowTracker) Inferences() int { return k.inferences }

// classifyState derives the temporal shape WITHIN one screen state.
//
// Same thresholds as the global classifier, deliberately — the change under test is the
// denominator, not the bar. Geometry gates stay global because a track's shape stability and
// role consistency are properties of the track, not of the screen it was seen on.
func (t ShadowTrack) classifyState(s TrackState) TemporalShape {
	switch {
	case s.Seen < 3:
		return ShapeRare
	case t.RoleChanges > 0 || t.MeanIoU < 0.40:
		return ShapeUnstable
	case s.PresenceRatio() >= 0.80 && s.Episodes == 1:
		return ShapePersistent
	case s.Episodes >= 2:
		return ShapeBursty
	default:
		return ShapeTransient
	}
}

// classify derives a generic temporal shape from measured behaviour.
//
// SESSION-GLOBAL, and for a state-dependent control it under-reports on purpose: a menu button
// really is bursty across a session that is half gameplay. classifyState answers the other,
// usually more useful question.
func (t ShadowTrack) classify() TemporalShape {
	switch {
	case t.Seen < 3:
		return ShapeRare
	case t.RoleChanges > 0 || t.MeanIoU < 0.40:
		return ShapeUnstable
	case t.PresenceRatio() >= 0.80 && t.Episodes == 1:
		return ShapePersistent
	case t.Episodes >= 2:
		return ShapeBursty
	default:
		return ShapeTransient
	}
}

func regionIoU(a, b Region) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2 := math.Min(a.X+a.Width, b.X+b.Width)
	y2 := math.Min(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	union := a.Width*a.Height + b.Width*b.Height - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func centreDistance(a, b Region) float64 {
	dx := (a.X + a.Width/2) - (b.X + b.Width/2)
	dy := (a.Y + a.Height/2) - (b.Y + b.Height/2)
	return math.Hypot(dx, dy)
}
