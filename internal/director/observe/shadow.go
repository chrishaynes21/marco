package observe

// Carrying an experiment's findings through the session path, beside the evidence Marco
// believes and never mixed into it.
//
// # Why this rides on Sample rather than on its own channel
//
// Because the previous attempt did have its own channel, and the channel had no consumer. The
// shadow accumulator and its renderer were written, tested in isolation, and never called from
// production: a five-minute Rocket League session ran ScreenParser for 149 CPU-seconds and the
// session result discarded every one of its findings. That is the failure
// [[Wiring-Tests]] documents first — a mechanism that exists and that production never invokes.
//
// A Sample already flows from the one sampler, through the session accumulator, into the
// terminal Result, across the service protocol and out of the CLI. Putting the experiment's
// per-cycle findings ON that value means it cannot be dropped without dropping the sample, and
// every layer that already carries authoritative data carries this by construction.
//
// # What it is not
//
// Not evidence. Nothing here reaches fusion, World State, planning or policy — that boundary is
// structural and lives in the collector, which routes shadow outcomes into Cycle.Shadow, a
// field Admitted() never reads. This type exists so the experiment can be REPORTED, and
// reporting is the only thing it may do.

// ShadowSample is what an experimental provider found during one cycle.
//
// Aggregate counts and bounded role tallies, never raw detections: a five-minute session at a
// two-second cadence is 150 cycles, and retaining every box would make a session result grow
// without bound for no diagnostic gain.
type ShadowSample struct {
	// Detector names the experiment, so a report can never be read as authoritative.
	Detector string `json:"detector"`
	// Gate describes the purchase policy this slot was offered under. Carried on every
	// sample so a report can say which configuration produced its numbers; a run whose
	// policy nobody recorded is a measurement of an unknown system.
	Gate string `json:"gate,omitempty"`
	// Declined says the slot was not taken because the primary reading claimed to be
	// sufficient — which is not the cadence pacing itself, and on a surface with no
	// accessibility tree is the difference between an experiment that is sampling and one
	// that has been switched off by a claim about other evidence.
	Declined bool `json:"declined,omitempty"`
	// Ran reports whether an inference actually happened this cycle. False for a slot the
	// cadence gate skipped — which is not a failure, and must not be counted as one.
	Ran bool `json:"ran"`
	// Unrequested says the slot was taken and the detector was never ASKED, because the
	// request carried no pixel source.
	//
	// Its own fact, because for a while it was none of the others. A provider nobody asked
	// returns instantly with no observations and no provenance, and that arrived here as an
	// inference that ran and could not prove which window it looked at — so the experiment
	// reported 122 inferences and 116 provenance failures on a run that performed six
	// inferences and failed none. Three different findings, one appearance: the detector was
	// never asked, the detector ran and found nothing, the detector ran and its evidence was
	// disqualified. See [[ADR-006-unknown-is-not-false]].
	Unrequested bool `json:"unrequested,omitempty"`
	// Unavailable explains why nothing ran, when the reason is not simply "not due".
	Unavailable string `json:"unavailable,omitempty"`
	// TargetProven reports whether the experiment can show its evidence describes the same
	// window generation the authoritative cycle observed. When false the two observed
	// different worlds and no comparison between them means anything.
	TargetProven bool `json:"target_proven"`

	Detections int `json:"detections"`
	Nameable   int `json:"nameable"`
	// Roles is the MARCO role distribution. A detector's native vocabulary ends at the
	// plugin boundary; anything unmapped arrives here as its native word and is counted
	// under Unknown rather than quietly renamed into something plausible.
	Roles   map[string]int `json:"roles,omitempty"`
	Unknown int            `json:"unknown"`

	LatencyMS int64 `json:"latency_ms"`

	// Regions are this cycle.s detections in identity-free geometry, for tracking.
	// Bounded by the detector.s own output; nothing raw, no labels, no screen coordinates.
	Regions []ShadowRegion `json:"regions,omitempty"`

	// Shapes are rectangles the detector proposed and nobody could name, with how each is
	// painted. Counted and reported APART from Detections, all the way out: a region being
	// detected is not proof that it is a menu entry. See shape.go.
	Shapes []ShadowShape `json:"shapes,omitempty"`

	// Inputs is the navigation observed since the previous sample, as closed-vocabulary
	// intents — never keys and never text; see input.go.
	//
	// It rides HERE, on the experimental sample, precisely because the only thing it feeds
	// is screen-state transition correlation, which has no authority. Putting player input
	// on the authoritative Sample would make it evidence the fusion engine could reach,
	// and "the user pressed down" is not something Marco should be able to believe its way
	// into acting on.
	Inputs []InputEvent `json:"inputs,omitempty"`

	// Labels is where the scoped reading budget went this inference.
	//
	// Reported because every way of producing no terms looks identical from the outside. "The
	// detector found nothing nameable", "the regions were too small to hold a word", "two
	// controls overlapped so no word could be attributed" and "we stopped at the ceiling" are
	// four different findings with four different remedies, and an empty term list is what all
	// of them look like.
	Labels ShadowLabels `json:"labels,omitzero"`

	// Semantic is what this inference's readable interface said about itself, as
	// closed-vocabulary terms and counts — never text. See terms.go.
	//
	// Rides here for the same reason Inputs does: it is evidence for a hypothesis and has
	// no authority, and the one value that already reaches the accumulator, the Result, the
	// protocol and the CLI is this one.
	Semantic SemanticEvidence `json:"semantic,omitzero"`

	// InputStats is the navigation producer's own cumulative counters, when one is wired.
	//
	// Rides on the sample for the same reason Inputs does: a producer diagnostic delivered
	// on its own channel is a diagnostic nothing consumes. Cumulative, so a slot that
	// carried no navigation still carries the current state of the producer — which is
	// exactly the slot on which "it produced nothing because it was never running" needs
	// to be distinguishable from "the player pressed nothing".
	InputStats *InputStats `json:"input_stats,omitempty"`

	// Comparison against the authoritative evidence of THIS cycle only.
	Comparison ShadowComparison `json:"comparison"`
}

// ShadowLabels is where one inference's scoped-reading budget went.
//
// Cumulative on the totals, per-inference on the sample. Every field is a REASON a region did
// not become a term, except Read, which is the one that did — and the ratio between them is the
// answer to "why does this application produce no semantic evidence".
type ShadowLabels struct {
	// Unsayable counts structural regions whose role may not carry plaintext. The number
	// that says where a detector's vocabulary stops being useful: a pass reporting forty
	// detections of which thirty-eight are unsayable has found plenty of structure and
	// nothing anybody can name.
	Unsayable int `json:"unsayable,omitempty"`
	// Ambiguous counts readings that could not be attributed to one control.
	Ambiguous int `json:"ambiguous,omitempty"`
	// Skipped counts regions too small to hold a word, or past the ceiling on readings.
	Skipped int `json:"skipped,omitempty"`
	// Read and Unreadable are the readings that happened: named, and refused by the shape or
	// confidence filters.
	Read       int `json:"read,omitempty"`
	Unreadable int `json:"unreadable,omitempty"`
	// ScreenTexts counts TEXT regions read for screen-level meaning. Held apart from Read
	// because they are different evidence: one names a control, the other names nothing.
	ScreenTexts int `json:"screen_texts,omitempty"`
}

// Add folds one inference's budget into the totals.
func (l *ShadowLabels) Add(o ShadowLabels) {
	l.Unsayable += o.Unsayable
	l.Ambiguous += o.Ambiguous
	l.Skipped += o.Skipped
	l.Read += o.Read
	l.Unreadable += o.Unreadable
	l.ScreenTexts += o.ScreenTexts
}

// Attempted is how many readings were actually paid for.
func (l ShadowLabels) Attempted() int { return l.Read + l.Unreadable + l.ScreenTexts }

// ShadowComparison counts how one cycle's experimental evidence related to what was believed.
//
// No category names a winner. `ShadowOnly` says the experiment reported structure belief did
// not have; whether it was right is not a question this layer is entitled to answer.
type ShadowComparison struct {
	Agreed               int `json:"agreed"`
	ShadowOnly           int `json:"shadow_only"`
	AuthoritativeOnly    int `json:"authoritative_only"`
	RoleDisagreement     int `json:"role_disagreement"`
	GeometryDisagreement int `json:"geometry_disagreement"`
	Uncomparable         int `json:"uncomparable"`
	// ShadowOnlyNameable is the product question in one number: safe, role-bearing
	// structure the experiment offered and authoritative perception never supplied.
	ShadowOnlyNameable int `json:"shadow_only_nameable"`
}

// ShadowTotals is a session's accumulated experimental findings.
type ShadowTotals struct {
	Detector    string `json:"detector"`
	Unavailable string `json:"unavailable,omitempty"`

	// Interface is what the session currently believes it is looking at, and Changed is
	// what moved at the last look. Both are rendered from the live state, which is why
	// they are fields rather than something a caller recomputes.
	Interface InterfaceBelief `json:"interface"`
	Changed   InterfaceChange `json:"interface_changed"`
	// InterfaceMoments is the bounded journal of looks at which that answer was different,
	// so a report can say what the session emitted and when rather than what a client
	// happened to catch while polling.
	InterfaceMoments []InterfaceMoment `json:"interface_moments,omitempty"`
	// watch carries the belief between looks. Unexported: a reader of a session gets the
	// answer, never the machinery that produced it.
	watch *InterfaceWatch

	Opportunities int `json:"opportunities"`
	Inferences    int `json:"inferences"`
	Skipped       int `json:"skipped"`
	// Declined is the subset of Skipped the purchase policy refused rather than the cadence
	// paced. Held apart because the two have opposite remedies: one is the gate working,
	// the other is the experiment being starved by a claim about different evidence.
	Declined int `json:"declined,omitempty"`
	// Unrequested is the subset of Skipped where the slot was taken and the detector was
	// never asked, because the request carried no pixel source. Its own number because it
	// is the one skip a reader would otherwise read as the experiment working: it looks
	// like an inference, it is counted like a cadence skip, and it is neither.
	Unrequested int `json:"unrequested,omitempty"`
	// Changing is the screen change being confirmed right now, empty when settled, and
	// ChangingLooks how many looks agree with it so far.
	//
	// The uncertainty, kept visible. A gate that hid this would be a smoother that silently
	// lagged, and a reader asking "what am I looking at" deserves "something else, once"
	// rather than the old answer repeated with confidence.
	Changing      ScreenStateID `json:"changing,omitempty"`
	ChangingLooks int           `json:"changing_looks,omitempty"`
	// ChangingUnreadable is how many looks during the pending change could not be placed
	// at all — the explicit uncertainty between the before and the after, reported rather
	// than smoothed away.
	ChangingUnreadable int `json:"changing_unreadable,omitempty"`
	// Gate describes the policy the experiment ran behind, so a report can never leave a
	// reader guessing which configuration produced its numbers.
	Gate string `json:"gate,omitempty"`
	// Flicker is the screen-change confirmation gate's own account: how many divergences
	// were proposed, how many became transitions, how many went away first, and how many of
	// those a deliberate press preceded.
	//
	// Reported beside the transitions rather than instead of them, because a transition
	// count on its own cannot be read. Forty transitions is either a busy player or an
	// unstable detector, and only this says which.
	Flicker FlickerStats `json:"flicker,omitzero"`
	// ProvenanceFailures counts inferences that ran but could not be shown to describe the
	// authoritative cycle's window. Held apart from failures: the experiment worked, it just
	// observed a world that had already moved on.
	ProvenanceFailures int `json:"provenance_failures"`
	Failures           int `json:"failures"`

	Detections int            `json:"detections"`
	Nameable   int            `json:"nameable"`
	Roles      map[string]int `json:"roles,omitempty"`
	Unknown    int            `json:"unknown"`

	// Shapes is how much unnamed geometry the session preserved, and ShapesShaded how much
	// of it could actually be measured.
	//
	// Separate from Detections on purpose and forever. The repair that produced this
	// channel was authorised with the warning attached: preserving seventy unreadable
	// rectangles could increase ambiguity rather than resolve it. A single number mixing
	// named structure with unnamed geometry is how that would happen without anybody
	// noticing, so the report prints both and the belief says which it was built from.
	Shapes       int `json:"shapes,omitempty"`
	ShapesShaded int `json:"shapes_shaded,omitempty"`

	// Labels is where the session's whole scoped-reading budget went. Read it before
	// reading an empty term set as a fact about the application.
	Labels ShadowLabels `json:"labels,omitzero"`

	MedianMS int64 `json:"median_ms"`
	P95MS    int64 `json:"p95_ms"`
	MaxMS    int64 `json:"max_ms"`

	Comparison ShadowComparison `json:"comparison"`

	// Tracks are the recurring structures this session discovered. Session-local and
	// bounded: a track is an observation that something recurred, never a World State
	// entity, an Action Graph node or an executable target.
	Tracks  []ShadowTrack `json:"tracks,omitempty"`
	Evicted int           `json:"evicted_tracks,omitempty"`

	// Structure is which kind of evidence the screen model was segmented from, and
	// StructureWhy explains an unobserved composition.
	//
	// Reported because "no screens" has four causes that send a reader to four different
	// places — nothing looked, the detector sat out, the window could not be proved, or the
	// application genuinely presents no structure — and a session that could not tell them
	// apart is what made the live blocker take a milestone to find.
	Structure    StructuralSource `json:"structure_source,omitempty"`
	StructureWhy string           `json:"structure_why,omitempty"`

	// Match is what the screen-identity comparison actually scored this session.
	//
	// The number behind every verdict. "1 screen, 0 transitions" is consistent with an
	// application that never changed and with a threshold that cannot see the change, and
	// only this tells them apart.
	Match MatchProfile `json:"match,omitzero"`

	// CurrentState is where the most recent valid inference placed the screen. Session-local
	// and ephemeral by construction; it exists so a running demonstration can ask where the
	// user is, and it is never durable identity.
	CurrentState ScreenStateID `json:"current_state,omitempty"`
	// InputQuiet is how many consecutive valid inferences have carried no navigation. It is
	// what lets a licensed pass tell "the person paused between legs" from "the person has
	// stopped": a multi-leg demonstration settles on screens along the way, and ending the
	// pass at the first of them would cut the demonstration off mid-route.
	InputQuiet int `json:"input_quiet,omitempty"`

	// States are the recurring screen compositions this session was segmented into, and
	// Transitions the observed changes between them. Session-local, generic and numbered:
	// discovering that the screen changed and came back is measurement; deciding that
	// state_2 is "the pause menu" is interpretation, and belongs to a capability pack.
	States      []ScreenState      `json:"states,omitempty"`
	Transitions []ScreenTransition `json:"state_transitions,omitempty"`
	// Crossings is the session's WALK: the same changes, in the order they were seen.
	//
	// Transitions is keyed by the pair, so it says which changes happened and how often and
	// cannot say what followed what. That is sufficient for every question about a recurring
	// habit and insufficient for one about a route somebody has just walked — which is the
	// question Learn asks. See unsettledIntervals for the failure that produced this.
	Crossings []Crossing `json:"crossings,omitempty"`
	// EvictedCrossings counts changes the walk bound dropped. A truncated walk is refused
	// rather than read short.
	EvictedCrossings int `json:"evicted_crossings,omitempty"`
	// Groups are tracks that behave as one structure within a state. Derived from Tracks
	// and States rather than retained, so they cost nothing to keep.
	Groups []StructuralGroup `json:"structural_groups,omitempty"`
	// EvictedStates and EvictedAssociations report what the bounds dropped, rather than
	// letting a capped model read as a complete one.
	EvictedStates       int `json:"evicted_states,omitempty"`
	EvictedAssociations int `json:"evicted_track_states,omitempty"`

	// Input is the navigation producer's own account of the session: how much physical
	// input it saw, how much became an intent, what it refused and why, and what the
	// bounded hook queue dropped.
	//
	// Read this BEFORE reading an empty correlation as a finding.
	Input InputStats `json:"input_producer,omitzero"`

	// InputLog is every admitted input event this session, in order, with the context known
	// when each was banked — kept whether or not any later layer could interpret it.
	// Attribution buffers expire and captures gate; this does neither. See inputlog.go.
	InputLog InputLog `json:"input_log,omitzero"`

	// latencies is unexported so the totals stay a plain, copyable value.
	latencies []int64
	tracker   *ShadowTracker
}

// Observed reports whether the experiment produced anything at all this session.
//
// The distinction a report depends on: a session where the detector never ran must read
// differently from one where it ran and saw nothing. Collapsing them is how "you did not
// enable the experiment" comes to look like "the experiment found nothing".
func (t ShadowTotals) Observed() bool { return t.Inferences > 0 }

// Add folds one cycle's findings into the totals.
// Observe folds ONE WHOLE SAMPLE: the structural composition it presented, and the
// detector's own accounting of its attempt to describe it.
//
// THE fold call site, and it takes the sample rather than the shadow record because the two
// halves come from different places. Screen segmentation reads whichever admissible source
// described the composition — see StructureOf — and the experiment's counters read only the
// experiment. Before this milestone they were one value, and a Director with the experiment
// switched off therefore had no screen model at all, for any application.
//
// Deleting the StructureOf call must fail TestAnAccessibleApplicationProducesScreens.
func (t *ShadowTotals) Observe(sample Sample) {
	structure := StructureOf(sample)
	t.Structure = structure.Source
	t.StructureWhy = structure.Why
	// What interface is in front of the session, updated every look.
	//
	// Folded HERE, with the rest of the shadow evidence, because that is exactly what it
	// is made of: regions a detector proposed and two numbers describing how each is
	// painted. Putting it on the authoritative sample would make it something fusion
	// could reach, and "a menu is open and Settings is selected" is not something Marco
	// should be able to believe its way into acting on — the same argument this file
	// already makes for player input.
	//
	// Deleting this must fail TestASessionAnswersWhatItIsLookingAt.
	if t.watch == nil {
		t.watch = NewInterfaceWatch()
	}
	// A look the detector did not take is not a look at an empty screen. The live cadence
	// lets an inference start every two seconds against a session sampling twice a second,
	// so most looks bring nothing — and folding those in as absence made the belief flap
	// to unknown three looks in four and kept the tracker from ever accumulating.
	//
	// Deleting this must fail TestASessionAnswersWhatItIsLookingAt.
	t.watch.Observe(interfaceLookFrom(sample))
	t.Interface, t.Changed = t.watch.Belief(), t.watch.Change()
	t.InterfaceMoments = t.watch.Moments()
	if sample.Shadow != nil {
		t.add(*sample.Shadow, structure)
		return
	}
	// No experiment at all — the ordinary Director. The screen model is still built,
	// because a screen is a fact about the interface; the experiment's counters stay
	// untouched, because a Director that never ran one must not look like a Director whose
	// experiment got fifty slots and did nothing with them.
	if t.tracker == nil {
		t.tracker = &ShadowTracker{}
	}
	t.tracker.Observe(nil, structure)
	t.publishScreenModel()
	t.EvictedStates = t.tracker.states.EvictedStates
	t.EvictedAssociations = t.tracker.EvictedAssociations
}

// interfaceLookFrom says what this sample gave the interface watch a chance to see.
//
// Six outcomes, kept apart: no detector was configured, the detector could not run, the
// cadence gate held it back, nobody asked it, it ran and cannot say which window it
// described, or it ran and described THIS screen. Only the last is evidence, and only the
// last may move a belief.
//
// The two added here are the two a live run produced and a replay never can. A replay hands
// every look a frame from a recording whose provenance was settled when it was recorded; live,
// most looks were a provider nobody asked, and a belief that folded those in reported "no
// shaded regions were proposed this look" — an empty SCREEN — for three minutes in front of a
// menu. An unproven look is the same mistake with a different cause: the pixels were real and
// cannot be shown to be pixels of the window in question, which is not evidence that the
// window was empty. See [[ADR-006-unknown-is-not-false]].
func interfaceLookFrom(sample Sample) InterfaceLook {
	look := InterfaceLook{Sequence: sample.Sequence, At: sample.Timestamp}
	switch {
	case sample.Shadow == nil:
		look.Why = "no detector is configured"
	case sample.Shadow.Unavailable != "":
		look.Why = sample.Shadow.Unavailable
	case sample.Shadow.Unrequested:
		look.Why = "the detector was never asked: this slot's request carried no pixel source"
	case !sample.Shadow.Ran:
		look.Why = "the detector sat out this slot"
	case !sample.Shadow.TargetProven:
		look.Why = "the detector cannot show which window it looked at, so what it found " +
			"is not evidence about this one"
	default:
		look.Observed = true
		look.Regions = sample.Shadow.Regions
		// AND THE GEOMETRY NOBODY COULD NAME, on the same look rather than a second one.
		//
		// One look is one moment. Handing the named structure and the unnamed geometry to
		// the belief as two looks would make a single frame read as two observations of
		// the screen, which is exactly the arithmetic the flicker gate was built to stop.
		//
		// Deleting this must fail TestAListOfUnnamedRowsCanStillBeWatched.
		look.Shapes = sample.Shadow.Shapes
	}
	return look
}

// Add folds one experimental sample, segmenting from the detector's own regions.
//
// Retained for the paths that genuinely have only a detector record — replay of a stored
// shadow trace, and the shadow diagnostics. Production goes through Observe.
func (t *ShadowTotals) Add(s ShadowSample) { t.add(s, s.Structure()) }

func (t *ShadowTotals) add(s ShadowSample, structure StructuralView) {
	if s.Detector != "" {
		t.Detector = s.Detector
	}
	if s.Unavailable != "" {
		t.Unavailable = s.Unavailable
	}
	if s.Gate != "" {
		t.Gate = s.Gate
	}
	t.Opportunities++
	if t.tracker == nil {
		t.tracker = &ShadowTracker{}
	}
	// THE tracking call site. One place, on the path every slot already takes.
	//
	// EVERY slot, not only the ones that ran. Screen evidence and input evidence fail
	// independently: a slot the cadence gate declined carries no detections and still
	// carries the navigation the player made during it — very often the keypress that
	// opened the menu the next inference is about to see. The tracker gates screen
	// evidence itself, so handing it a skipped slot adds nothing to presence.
	//
	// Screen-state segmentation happens INSIDE Observe, ahead of any track update, so it
	// cannot be wired up separately, forgotten, or left without a consumer — the failure
	// [[Wiring-Tests]] documents first. Deleting it fails
	// TestTheProductionSessionPathSegmentsScreenStates at the runner level.
	t.tracker.Observe(&s, structure)
	// The producer's own account, folded BEFORE the skipped-slot return.
	//
	// Deliberately: drops happen when the machine is busy, which is when slots get skipped,
	// so folding this after the early return would lose the diagnostic in precisely the
	// conditions it exists to report. The counters are cumulative, so the newest snapshot
	// supersedes rather than sums.
	if s.InputStats != nil {
		t.Input = admissibleStats(*s.InputStats)
	}
	// The SCREEN MODEL is published before the experiment's own early return.
	//
	// It has to be. The tracker has already observed, and everything below this line is the
	// DETECTOR's accounting — its cadence, its latency, its role histogram. Publishing the
	// screen model after `!s.Ran` returned is what made a Director with no detector report
	// zero screens while its tracker held several: the model existed and nothing copied it
	// out. Deleting this call must fail TestAnAccessibleApplicationProducesScreens.
	t.publishScreenModel()
	// Unnamed geometry, counted on EVERY slot that carried any — including ones the
	// detector sat out, which carry none, and therefore add nothing. Above the `!s.Ran`
	// return for the same reason the input account is: what a slot carried is a fact about
	// the slot, and a count that stopped at the cadence gate would describe the gate.
	for _, sh := range s.Shapes {
		t.Shapes++
		if sh.Shaded() {
			t.ShapesShaded++
		}
	}
	if !s.Ran {
		t.Skipped++
		if s.Declined {
			t.Declined++
		}
		if s.Unrequested {
			t.Unrequested++
		}
		return
	}
	t.Inferences++
	if !s.TargetProven {
		t.ProvenanceFailures++
	}
	t.Detections += s.Detections
	t.Nameable += s.Nameable
	t.Unknown += s.Unknown
	t.Labels.Add(s.Labels)
	if len(s.Roles) > 0 {
		if t.Roles == nil {
			t.Roles = map[string]int{}
		}
		for k, v := range s.Roles {
			t.Roles[k] += v
		}
	}
	if s.LatencyMS > 0 {
		t.latencies = append(t.latencies, s.LatencyMS)
		if s.LatencyMS > t.MaxMS {
			t.MaxMS = s.LatencyMS
		}
	}
	c := s.Comparison
	t.Comparison.Agreed += c.Agreed
	t.Comparison.ShadowOnly += c.ShadowOnly
	t.Comparison.AuthoritativeOnly += c.AuthoritativeOnly
	t.Comparison.RoleDisagreement += c.RoleDisagreement
	t.Comparison.GeometryDisagreement += c.GeometryDisagreement
	t.Comparison.Uncomparable += c.Uncomparable
	t.Comparison.ShadowOnlyNameable += c.ShadowOnlyNameable
	t.EvictedStates = t.tracker.states.EvictedStates
	t.EvictedAssociations = t.tracker.EvictedAssociations
	t.finalise()
}

// publishScreenModel copies the tracker's conclusions out for a reader.
//
// Called on EVERY fold, including the ones the structural detector sat out. The screen
// model is a fact about the interface; whether an experiment also looked is a fact about
// the experiment, and gating the first on the second is the defect this milestone fixed.
func (t *ShadowTotals) publishScreenModel() {
	t.CurrentState = t.tracker.current
	t.InputLog = t.tracker.log
	t.InputQuiet = t.tracker.quietInferences
	t.Tracks = t.tracker.Tracks()
	t.States = t.tracker.States()
	t.Transitions = t.tracker.Transitions()
	t.Crossings = t.tracker.Crossings()
	// The gate's account travels with the transitions it produced. Deleting this must fail
	// TestTheSessionReportsWhatTheFlickerGateRefused.
	t.Flicker = t.tracker.states.Flicker
	t.Changing, t.ChangingLooks, t.ChangingUnreadable = t.tracker.states.Pending()
	t.EvictedCrossings = t.tracker.states.EvictedCrossings
	t.Groups = Groups(t.Tracks, t.States)
	t.Evicted = t.tracker.Evicted
	t.Match = t.tracker.states.Match
}

// finalise recomputes the latency percentiles.
func (t *ShadowTotals) finalise() {
	if len(t.latencies) == 0 {
		return
	}
	sorted := append([]int64(nil), t.latencies...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	at := func(p int) int64 {
		i := (len(sorted) * p) / 100
		if i >= len(sorted) {
			i = len(sorted) - 1
		}
		return sorted[i]
	}
	t.MedianMS, t.P95MS = at(50), at(95)
}
