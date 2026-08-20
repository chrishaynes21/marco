package observe

import (
	"fmt"
	"sort"
)

// Watching ONE bounded demonstration, because the user said "watch me".
//
// # What that permission is, exactly
//
// A person answered yes to "do you want me to learn how you do that?". That grants Marco leave
// to observe one clean example of ONE approved transition. It does not grant leave to record
// what they type, to watch indefinitely, to watch a different transition that happened to occur,
// or — most importantly — to act.
//
// So every boundary here is structural rather than careful:
//
//   - Capture cannot start without a LearningPending request naming a durable edge, and the
//     controller has no other way to be armed.
//   - What is recorded is the same closed NavIntent vocabulary passive observation already uses.
//     A yes does not widen the privacy boundary by one key.
//   - Every bound is a number, and exceeding one ends the capture with a NAMED reason rather
//     than quietly truncating a partial demonstration into a valid-looking one.
//   - The output has no Execute, no Run, no Replay and no registration. It lives in this
//     package, whose boundary test already proves nothing here can reach anything that acts.
//
// # What the output claims
//
//	"During the demonstration you approved, I observed this sequence of navigation while the
//	 screen moved through these states."
//
// Not "these are the steps". Not "I know how". A candidate is evidence that a procedure might
// exist, and deciding whether it does is a later milestone with its own confirmation.

// CaptureState is where one demonstration is in its life.
//
// Five values, and each names a situation a report has to be able to distinguish. `Armed` and
// `Capturing` are different — one is waiting for the user to get to the right screen, the other
// is watching — and collapsing them would make "nothing is happening" indistinguishable from
// "you are not where I need you to be".
type CaptureState string

const (
	// CaptureArmed is approved and waiting for the demonstration to begin at the start
	// subject.
	CaptureArmed CaptureState = "armed"
	// CaptureCapturing is watching: the start subject was recognised and navigation is
	// being recorded.
	CaptureCapturing CaptureState = "capturing"
	// CaptureComplete produced a candidate.
	CaptureComplete CaptureState = "complete"
	// CaptureIncomplete stopped without one, for a named reason.
	CaptureIncomplete CaptureState = "incomplete"
	// CaptureCancelled was stopped by the user.
	CaptureCancelled CaptureState = "cancelled"
)

// Settled reports whether this capture is over.
func (s CaptureState) Settled() bool {
	return s == CaptureComplete || s == CaptureIncomplete || s == CaptureCancelled
}

// CaptureReason is the CLOSED vocabulary of why a capture ended as it did.
type CaptureReason string

const (
	ReasonArrived              CaptureReason = "arrived"
	ReasonDestinationMismatch  CaptureReason = "destination_mismatch"
	ReasonAmbiguousDestination CaptureReason = "ambiguous_destination"
	ReasonWrongStart           CaptureReason = "wrong_start"
	ReasonTooManyEvents        CaptureReason = "bound_semantic_events"
	ReasonTooManyCheckpoints   CaptureReason = "bound_checkpoints"
	ReasonTooLong              CaptureReason = "bound_duration"
	ReasonTooManyRestarts      CaptureReason = "bound_restarts"
	ReasonCancelled            CaptureReason = "cancelled"
	ReasonSessionEnded         CaptureReason = "session_ended"
)

// CaptureBounds structurally limit what one demonstration may watch.
//
// A recorder that can watch indefinitely is a recorder nobody can reason about, and "the user
// said yes" is not a licence to follow them around. Every one of these ends the capture with its
// own reason rather than truncating: a demonstration cut short at an arbitrary point and then
// presented as complete is a candidate that would teach the wrong thing.
type CaptureBounds struct {
	// MaxEvents is how many semantic navigation events one demonstration may contain.
	//
	// Sixty. A menu route is a handful of presses; sixty is generous enough to cover a
	// fumbled attempt and far short of "the user went off to play for a while".
	MaxEvents int
	// MaxCheckpoints is how many screen states the demonstration may pass through.
	//
	// Eight, counting the start. A route through more than eight screens is not one
	// demonstration, it is a tour.
	MaxCheckpoints int
	// MaxRunLength is how many events one navigation run between checkpoints may hold.
	MaxRunLength int
	// MaxInferences is how many valid observations the capture may span, which is the
	// duration bound expressed in the only clock this layer has.
	//
	// A sampler runs at a known cadence, so this IS wall-clock — and unlike wall-clock it
	// cannot be distorted by a machine that stalled, which would otherwise end a
	// demonstration the user was in the middle of.
	MaxInferences int
	// MaxRestarts is how many times the user may return to the start without reaching the
	// destination before the capture gives up.
	MaxRestarts int
}

// DefaultCaptureBounds are the conservative defaults.
func DefaultCaptureBounds() CaptureBounds {
	return CaptureBounds{
		MaxEvents: 60, MaxCheckpoints: 8, MaxRunLength: MaxSequenceLength,
		MaxInferences: 90, MaxRestarts: 2,
	}
}

// Checkpoint is one screen the demonstration passed through.
//
// Either it resolved to a remembered subject — and then Subject names it — or it did not, and
// then it is a TRANSIENT checkpoint carrying only this session's safe structural signature.
// The distinction is load-bearing: a transient checkpoint must never read as a durable subject,
// and observing a screen during a demonstration must not quietly promote it into cross-session
// memory. Promotion is what the semantic proposal loop is for, and it involves a person.
type Checkpoint struct {
	// Subject is the remembered subject id, empty for a transient checkpoint.
	Subject string `json:"subject,omitempty"`
	// Transient says this screen was not recognised, and Structure is what was seen.
	Transient bool `json:"transient,omitempty"`
	// Structure is the safe signature — roles, members, closed terms. No coordinates, no
	// text, and no session reference.
	Structure StructureSignature `json:"structure,omitzero"`
	// Verdict is how the resolution came out, so the candidate can explain why it believes
	// this checkpoint is what it says it is.
	Verdict MatchVerdict `json:"verdict,omitempty"`
	// EditableFields is why a step may be marked as needing typed input. A count, never
	// contents.
	EditableFields int `json:"editable_fields,omitempty"`
}

// DemonstrationStep is one leg: a run of navigation, and the screen it arrived at.
//
// Deliberately NOT flattened into one navigation list. A future validation has to be able to
// check PROGRESS — did the screen actually become the next thing — rather than replay input and
// hope, and a flat list has thrown that away.
type DemonstrationStep struct {
	// Intents is the ordered navigation observed between the previous checkpoint and this
	// one. Order is the whole point; `down, down, confirm` and `confirm, down, down` are
	// different demonstrations.
	Intents []NavIntent `json:"intents,omitempty"`
	// Targets is aligned with Intents: what each event was aimed at, when the evidence at
	// the time identified it. Zero entries are the ordinary case, and the field is omitted
	// when nothing in the leg resolved.
	//
	// THE one durable home a semantic target has. A candidate exists only because a person
	// explicitly asked Marco to learn and then acted — the demonstration licence — so the
	// label of the one control each of their own events landed on may persist with the
	// evidence it explains. The durable topology never carries these; see TargetedSequence.
	Targets []SemanticTarget `json:"targets,omitempty"`
	// Conditional says at least one of those intents came from a key that is only
	// navigation in context — ADR-013's doubt, carried here too.
	Conditional bool `json:"conditional,omitempty"`
	// Arrived is the checkpoint this leg ended at.
	Arrived Checkpoint `json:"arrived"`
	// SkippedInferences counts observation slots the detector sat out during this leg.
	//
	// Reported because a leg that spanned four skipped slots is a leg whose intermediate
	// screens were never seen — the navigation is still ordered and complete, and the
	// checkpoints between are genuinely unknown rather than absent.
	SkippedInferences int `json:"skipped_inferences,omitempty"`
	// RequiresTextEntry marks a leg that crossed a screen offering somewhere to type.
	//
	// A BOUNDARY, not a recording. What the user typed is not observed, not reconstructed
	// and not stored; the candidate says only that this demonstration cannot be understood
	// without text that Marco deliberately did not watch. Handling that needs its own
	// privileged mode and its own consent.
	RequiresTextEntry bool `json:"requires_text_entry,omitempty"`
}

// TargetAt is the target aligned with intent i, zero when the leg carries none.
func (s DemonstrationStep) TargetAt(i int) SemanticTarget {
	if i < 0 || i >= len(s.Targets) {
		return SemanticTarget{}
	}
	return s.Targets[i]
}

// ProcedureCandidate is what one approved demonstration produced.
//
// # Why the name says candidate
//
// Because `Procedure`, `Macro` and `Route` all name things this repository can execute, and a
// type named for what it might become is a type somebody will eventually run. This one says
// what was observed, once, under permission, and nothing about whether repeating it would work.
//
// It has no Execute, no Run, no Replay and no Compile. It is not registered with the planner,
// the resolver, the action graph or any capability registry — and it lives in `observe`, whose
// boundary test proves that nothing reachable from here can affect the machine.
type ProcedureCandidate struct {
	// Request and Relationship trace the authorisation. A candidate with no approved
	// request behind it is not a candidate, and there is no path that produces one.
	Relationship RelationshipRef `json:"relationship"`
	Application  string          `json:"application"`
	// Start is where the demonstration began, established from CURRENT evidence at the time
	// rather than assumed from the request.
	Start Checkpoint `json:"start"`
	// Steps are the legs, in order. The last one arrives at the destination.
	Steps []DemonstrationStep `json:"steps,omitempty"`
	// Complete says the demonstration reached the approved destination.
	Complete bool `json:"complete"`
	// Reason is why it ended, from the closed vocabulary.
	Reason CaptureReason `json:"reason"`
	// Events, Checkpoints, Inferences and Skipped are what it cost to watch.
	Events      int `json:"events"`
	Checkpoints int `json:"checkpoints"`
	Inferences  int `json:"inferences"`
	Skipped     int `json:"skipped_inferences,omitempty"`
	// Restarts counts returns to the start without arriving.
	Restarts int `json:"restarts,omitempty"`
	// Waited is what was seen before the demonstration began, which is the only evidence
	// available at all when it never did.
	Waited ArmedWait `json:"waited,omitzero"`
	// Sequence is 1 for the first approved demonstration of a route and 2 for the follow-up.
	//
	// LINEAGE, not a version: candidate 1 is never mutated into candidate 2. Observed
	// evidence is immutable, and two examples of one route are two observations rather than
	// one that got better.
	Sequence int `json:"sequence,omitempty"`
	// Verified is always false and is here to be read.
	//
	// Not a placeholder: a consumer that asks "can I use this" must get an answer, and the
	// honest answer after one watched example is no. Nothing in this milestone can set it.
	Verified bool `json:"verified"`
}

// Describe renders a candidate for a person, including what it does NOT claim.
func (c ProcedureCandidate) Describe() []string {
	out := []string{
		fmt.Sprintf("demonstration of %s → %s (%s)",
			c.Relationship.From, c.Relationship.To, c.Reason),
		"start: " + describeCheckpoint(c.Start),
	}
	for i, s := range c.Steps {
		out = append(out, fmt.Sprintf("  step %d: %s → %s",
			i+1, describeIntents(s.Intents), describeCheckpoint(s.Arrived)))
		if s.RequiresTextEntry {
			out = append(out, "    this screen offers somewhere to type; what was typed was "+
				"not observed and this demonstration cannot be understood without it")
		}
		if s.Conditional {
			out = append(out, "    some of that navigation came from keys that only mean "+
				"navigation in a menu, which is weaker evidence")
		}
		if s.SkippedInferences > 0 {
			out = append(out, fmt.Sprintf("    %d observation slot(s) were skipped during "+
				"this leg, so any screen in between was never seen", s.SkippedInferences))
		}
	}
	out = append(out, fmt.Sprintf("%d semantic event(s), %d checkpoint(s), %d observation(s)",
		c.Events, c.Checkpoints, c.Inferences))
	out = append(out, "verified: no. This is one watched example. Marco has not established "+
		"that repeating it would work, and cannot perform it")
	return out
}

func describeCheckpoint(c Checkpoint) string {
	if c.Transient || c.Subject == "" {
		return "a screen Marco does not recognise"
	}
	return c.Subject
}

// CaptureInput is what one observation cycle tells a running capture.
//
// Derived by the caller from the same sample everything else reads. Nothing here is a handle, a
// window, a coordinate or a piece of text.
type CaptureInput struct {
	// Ran says a valid inference happened. False for a slot the detector sat out, which is
	// counted as a skip and must not erase the navigation that arrived with it.
	Ran bool
	// Subject is the remembered subject the current screen resolved to, empty when it
	// resolved to none.
	Subject string
	// Verdict is how that resolution came out.
	Verdict MatchVerdict
	// Structure is the current screen's safe signature, for a transient checkpoint.
	Structure StructureSignature
	// Placed says the current screen was placeable at all. A transition frame is not a
	// checkpoint and must not become one.
	Placed bool
	// EditableFields is how many text-editable controls the current screen reported.
	EditableFields int
	// Inputs is the ordered navigation observed since the previous cycle.
	Inputs []InputEvent
}

// Capture is one armed or running demonstration.
//
// Session-scoped plain data, like the proposal ledger: no clock, no I/O, no goroutine. The
// runner owns it and feeds it; it decides nothing about when to observe.
type Capture struct {
	Relationship RelationshipRef `json:"relationship"`
	Application  string          `json:"application"`
	State        CaptureState    `json:"state"`
	Reason       CaptureReason   `json:"reason,omitempty"`
	Bounds       CaptureBounds   `json:"-"`

	Start       Checkpoint          `json:"start,omitzero"`
	Steps       []DemonstrationStep `json:"steps,omitempty"`
	Events      int                 `json:"events"`
	Inferences  int                 `json:"inferences"`
	Skipped     int                 `json:"skipped_inferences,omitempty"`
	Restarts    int                 `json:"restarts,omitempty"`
	Checkpoints int                 `json:"checkpoints"`

	// Waited is what the capture saw while it was still waiting to begin.
	Waited ArmedWait `json:"waited,omitzero"`

	// pending is the navigation seen since the last checkpoint, and the doubt attached to it.
	pending []NavIntent
	// pendingTargets is aligned with pending: what each event was aimed at, when known.
	pendingTargets     []SemanticTarget
	pendingConditional bool
	pendingSkips       int
	pendingText        bool
	// at is the checkpoint the capture is currently standing on.
	at Checkpoint
}

// ArmedWait is what a capture saw while it was armed and had not yet begun.
//
// # Why this exists
//
// A capture that never begins produces `0 event(s), 0 checkpoint(s)` and an assessment of
// `start_unverifiable`, and those say the same thing three times without saying which of three
// very different failures happened: nothing was ever recognised, the user was recognised somewhere
// that is not the start, or the start was recognised only as a candidate. The first is a
// recognition problem, the second is the person standing in the wrong place, and the third is a
// threshold. Telling somebody "I wasn't watching" covers all three.
//
// Counts only, and no labels: a subject id is opaque and a count is a count. Transient — it lives
// on the capture and travels into the candidate, and nothing here is written to the store.
type ArmedWait struct {
	// Placed is how many inferences resolved to a screen at all while armed.
	Placed int `json:"placed,omitempty"`
	// Unplaced is how many ran but could not say where the user was.
	Unplaced int `json:"unplaced,omitempty"`
	// OnStart is how many placed inferences named the requested starting subject.
	OnStart int `json:"on_start,omitempty"`
	// Elsewhere is how many named a DIFFERENT remembered subject — the user standing
	// somewhere real that is not where the demonstration begins.
	Elsewhere int `json:"elsewhere,omitempty"`
	// Unestablished is how many named the start on a verdict too weak to begin on.
	Unestablished int `json:"unestablished,omitempty"`
}

// Empty reports whether nothing was recorded, so a reader is not shown a row of zeroes.
func (w ArmedWait) Empty() bool {
	return w.Placed == 0 && w.Unplaced == 0
}

// noteWhileArmed records one inference the capture watched without beginning.
//
// Called only in CaptureArmed, so a running demonstration adds nothing: once it has begun, the
// question this answers no longer exists.
func (c *Capture) noteWhileArmed(in CaptureInput) {
	if !in.Ran {
		return
	}
	if !in.Placed {
		c.Waited.Unplaced++
		return
	}
	c.Waited.Placed++
	switch {
	case in.Subject == c.Relationship.From && !in.Verdict.Established():
		c.Waited.Unestablished++
	case in.Subject == c.Relationship.From:
		c.Waited.OnStart++
	case in.Subject != "":
		c.Waited.Elsewhere++
	}
}

// NewCapture arms a demonstration for one approved relationship.
//
// The ONLY constructor. There is no path that produces a Capture without a relationship
// reference, which is what makes "no orphan observation becomes a candidate" a property of the
// type rather than of a check somebody could forget.
func NewCapture(application string, ref RelationshipRef, b CaptureBounds) *Capture {
	if b.MaxEvents == 0 {
		b = DefaultCaptureBounds()
	}
	return &Capture{
		Relationship: ref, Application: application,
		State: CaptureArmed, Bounds: b,
	}
}

// Cancel stops the capture at the user's request.
func (c *Capture) Cancel() {
	if c == nil || c.State.Settled() {
		return
	}
	c.State, c.Reason = CaptureCancelled, ReasonCancelled
}

// EndSession stops an unfinished capture because the session did.
//
// An incomplete capture is never carried forward. A demonstration is a bounded thing somebody
// agreed to give, and resuming one across a restart would be watching without being asked again.
func (c *Capture) EndSession() {
	if c == nil || c.State.Settled() {
		return
	}
	c.State, c.Reason = CaptureIncomplete, ReasonSessionEnded
}

// Observe folds one observation cycle into the capture.
//
// # The order of the checks, which is the whole design
//
//  1. Navigation is banked FIRST, always, even on a slot the detector sat out. Screen evidence
//     and input evidence fail independently, and a keypress that arrived during a skipped slot
//     is still a keypress the user made.
//  2. Bounds are checked next, so an overrun ends the capture rather than being recorded.
//  3. Only then is the screen consulted, and only a PLACED screen may move a checkpoint.
func (c *Capture) Observe(in CaptureInput) {
	if c == nil || c.State.Settled() {
		return
	}
	if in.Ran {
		c.Inferences++
	} else {
		c.Skipped++
		c.pendingSkips++
	}

	// THE ordering guarantee. Intents are appended in the order they arrived, and a slot the
	// detector skipped adds to the skip count without disturbing the run.
	for _, e := range in.Inputs {
		if !e.Intent.Known() {
			continue
		}
		if c.State == CaptureCapturing {
			c.pending = append(c.pending, e.Intent)
			if e.Target != nil {
				c.pendingTargets = append(c.pendingTargets, *e.Target)
			} else {
				c.pendingTargets = append(c.pendingTargets, SemanticTarget{})
			}
			c.Events++
			if e.Conditional {
				c.pendingConditional = true
			}
		}
	}

	if c.State == CaptureCapturing && !c.withinBounds() {
		return
	}

	// Above the return below, deliberately: the unplaced and not-yet-recognised inferences are
	// exactly the ones this is here to count, and they are the ones that return early.
	if c.State == CaptureArmed {
		c.noteWhileArmed(in)
	}

	if !in.Ran || !in.Placed {
		// Unknown is not false. A transition frame or a skipped slot says nothing about
		// where the user is, and treating it as a checkpoint would invent one.
		return
	}

	switch c.State {
	case CaptureArmed:
		c.beginAt(in)
	case CaptureCapturing:
		c.arriveAt(in)
	}
}

// beginAt decides whether the demonstration may start here.
//
// CURRENT evidence wins. The request says the demonstration is about A → B, and that is a claim
// about history; whether the user is standing on A right now is a question only this cycle can
// answer, and assuming it because the request said so is exactly the stale-ordinal mistake this
// repository has made before.
//
// A screen that is not A is not a refusal — the user has to walk there, and armed means waiting.
// Only an EXPLICIT wrong start is reported, and this layer does not have one: it waits.
func (c *Capture) beginAt(in CaptureInput) {
	if in.Subject != c.Relationship.From || !in.Verdict.Established() {
		return
	}
	c.Start = Checkpoint{
		Subject: in.Subject, Structure: in.Structure, Verdict: in.Verdict,
		EditableFields: in.EditableFields,
	}
	c.at = c.Start
	c.Checkpoints = 1
	c.State = CaptureCapturing
	c.pending, c.pendingTargets = nil, nil
	c.pendingConditional, c.pendingSkips, c.pendingText = false, 0, false
}

// arriveAt records a screen change during the demonstration.
func (c *Capture) arriveAt(in CaptureInput) {
	next := Checkpoint{
		Subject: in.Subject, Structure: in.Structure, Verdict: in.Verdict,
		EditableFields: in.EditableFields,
	}
	if in.Subject == "" || !in.Verdict.Established() {
		// Not recognised. Retained as a TRANSIENT checkpoint carrying this session's safe
		// signature — losing it would flatten the demonstration into one navigation run and
		// throw away the progress a future validation has to check against. It is not, and
		// must not become, a remembered subject.
		next = Checkpoint{Transient: true, Structure: in.Structure, Verdict: in.Verdict,
			EditableFields: in.EditableFields}
	}
	// A transient checkpoint that has since been RECOGNISED is the same screen, not a new one.
	//
	// This happens on every visit and is not an edge case: a screen's structural group takes a
	// few observations to form, so the first sighting resolves to nothing and a later one
	// resolves to the remembered subject. Recording that as a second leg would insert an empty
	// navigation run between a screen and itself — and, worse, would leave the destination
	// looking like it was reached without the user doing anything.
	if c.at.Transient && next.Subject != "" && sameStructure(c.at.Structure, next.Structure) {
		c.at = next
		if n := len(c.Steps); n > 0 {
			c.Steps[n-1].Arrived = next
		} else {
			c.Start = next
		}
		c.settleAt(next)
		return
	}
	if sameCheckpoint(c.at, next) {
		return
	}
	if textEntryBlocks(next, c.Relationship.To) {
		c.pendingText = true
	}

	step := DemonstrationStep{
		Intents: append([]NavIntent{}, c.pending...), Conditional: c.pendingConditional,
		Arrived: next, SkippedInferences: c.pendingSkips, RequiresTextEntry: c.pendingText,
	}
	if anyTarget(c.pendingTargets) {
		step.Targets = append([]SemanticTarget{}, c.pendingTargets...)
	}
	if len(step.Intents) > c.Bounds.MaxRunLength {
		c.stop(CaptureIncomplete, ReasonTooManyEvents)
		return
	}
	c.Steps = append(c.Steps, step)
	c.Checkpoints++
	c.at = next
	c.pending, c.pendingTargets = nil, nil
	c.pendingConditional, c.pendingSkips, c.pendingText = false, 0, false

	c.settleAt(next)
	if c.State == CaptureCapturing {
		_ = c.withinBounds()
	}
}

// textEntryBlocks reports whether arriving at this checkpoint makes the leg unreproducible.
//
// # The distinction, and why the destination is exempt
//
// Marco does not observe what anybody types, so a procedure that DEPENDS on typed text cannot be
// replayed. The conservative reading of that was "a leg that arrived anywhere offering somewhere to
// type", and it is too broad by one case.
//
// At an INTERMEDIATE checkpoint the concern is real: unobserved typing there may be how the user
// reached the next screen, and nothing in the record could show it. The leg after it would replay
// navigation into a state that was never going to arrive.
//
// At the DESTINATION nothing comes after. Whether the final screen offers a search box says nothing
// about whether text entry was required to GET there, and the question lowering is asking is only
// "can Marco reproduce the demonstrated route to this endpoint". A live Settings run refused on
// exactly this: a person pressed down, down, enter, arrived, and was told the play could not be
// learned because the page they landed on has a search box. Windows Settings, Explorer, Chrome and
// VS Code all put one on nearly every page, so the broad reading makes mainstream software
// unlearnable in exchange for a concern that does not apply.
//
// It is NOT a claim that text entry is irrelevant in general, and it is not the stronger rule that
// "all observed intents were navigation, so nobody typed" — typing is invisible, so that inference
// is unavailable until it is not. See [[ADR-053-a-search-box-on-the-last-page-is-not-a-step]].
func textEntryBlocks(cp Checkpoint, destination string) bool {
	if cp.EditableFields == 0 {
		return false
	}
	return cp.Subject == "" || cp.Subject != destination
}

// settleAt decides what arriving at this checkpoint means for the demonstration.
//
// Called from both paths that can change where the capture is standing: a genuine screen change,
// and a transient checkpoint being recognised in place. One function, because a mismatch reached
// by the second route is exactly as much a mismatch as one reached by the first — and having the
// terminal rules in two places is how one of them silently stops applying.
func (c *Capture) settleAt(next Checkpoint) {
	switch {
	case next.Subject == c.Relationship.To:
		// ARRIVED. The end condition is the destination being established from current
		// evidence — never a timeout, never a keypress, never a count of inputs.
		c.stop(CaptureComplete, ReasonArrived)
	case next.Subject == c.Relationship.From:
		// Back where we started without arriving. Bounded, then given up on: a capture that
		// restarted forever would be a recorder watching indefinitely by another route.
		c.Restarts++
		if c.Restarts > c.Bounds.MaxRestarts {
			c.stop(CaptureIncomplete, ReasonTooManyRestarts)
			return
		}
		c.Start = next
		c.Steps, c.Checkpoints = nil, 1
	case next.Subject != "":
		// A DIFFERENT remembered subject. The approved request was A → B, and quietly
		// reinterpreting it as A → C would produce a candidate nobody authorised. A future
		// proposal can offer to learn A → C on its own evidence.
		c.stop(CaptureIncomplete, ReasonDestinationMismatch)
	case next.Verdict == MatchInsufficient:
		// Several remembered subjects fit equally. Completing here would attach a
		// demonstration to a coin toss; asking for another one costs a minute.
		c.stop(CaptureIncomplete, ReasonAmbiguousDestination)
	}
}

// withinBounds stops the capture if it has outgrown one, reporting WHICH.
func (c *Capture) withinBounds() bool {
	switch {
	case c.Events > c.Bounds.MaxEvents:
		c.stop(CaptureIncomplete, ReasonTooManyEvents)
	case c.Checkpoints > c.Bounds.MaxCheckpoints:
		c.stop(CaptureIncomplete, ReasonTooManyCheckpoints)
	case c.Inferences > c.Bounds.MaxInferences:
		c.stop(CaptureIncomplete, ReasonTooLong)
	default:
		return true
	}
	return false
}

func (c *Capture) stop(state CaptureState, reason CaptureReason) {
	c.State, c.Reason = state, reason
}

func sameCheckpoint(a, b Checkpoint) bool {
	if a.Subject != "" || b.Subject != "" {
		return a.Subject == b.Subject
	}
	return a.Transient && b.Transient && sameStructure(a.Structure, b.Structure)
}

// sameStructure compares two transient signatures for "this is still the same screen".
//
// The tolerant structural comparison the identity layer already uses — so a transient checkpoint
// does not split in two because one detection was missed — with Members deliberately excluded.
//
// Members is the size of the screen's dominant structural GROUP, and a group materialises part
// way through a visit: the first few observations of a screen report 0 and later ones report 4.
// Across sessions that is stable and identity-bearing; WITHIN one visit it grows, and comparing
// on it would split a single screen into a new checkpoint every time the group came into focus.
// That is the same maturing-quantity mistake `Recurrence` was excluded from durable identity for.
//
// None of this is durable identity and none of it produces a record — see Checkpoint.
func sameStructure(a, b StructureSignature) bool {
	a.Members, b.Members = 0, 0
	return CompareStructure(a, b) != MatchDifferent
}

// Candidate renders what the capture produced.
//
// Returns false while the capture is still armed or running, and for a cancelled one: a
// cancelled demonstration produces no partial candidate, because a partial demonstration
// presented as evidence is evidence for the wrong thing.
func (c *Capture) Candidate() (ProcedureCandidate, bool) {
	if c == nil || !c.State.Settled() || c.State == CaptureCancelled {
		return ProcedureCandidate{}, false
	}
	out := ProcedureCandidate{
		Relationship: c.Relationship, Application: c.Application,
		Start: c.Start, Steps: append([]DemonstrationStep{}, c.Steps...),
		Complete: c.State == CaptureComplete, Reason: c.Reason,
		Events: c.Events, Checkpoints: c.Checkpoints,
		Inferences: c.Inferences, Skipped: c.Skipped, Restarts: c.Restarts,
		Waited: c.Waited,
	}
	return out, true
}

// SignatureOfState builds the durable signature for one screen state.
//
// The SAME derivation hypotheses use — `stateFingerprint` — so a checkpoint is resolved against
// memory by exactly the value the subject would have been stored under. A second derivation here
// would be a second answer to "what is this screen", and the two would drift.
func SignatureOfState(t ShadowTotals, id ScreenStateID,
	th HypothesisThresholds) (StructureSignature, bool) {

	if id == "" || id == ScreenStateUnknown {
		return StructureSignature{}, false
	}
	for _, st := range t.States {
		if st.ID != id {
			continue
		}
		return SignatureOf(Hypothesis{
			Subject: Subject{Kind: SubjectState, Ref: string(id),
				Fingerprint: stateFingerprint(t, st, th)},
		}), true
	}
	return StructureSignature{}, false
}

// CaptureView is a bounded, safe description of a running capture, for `director status`.
//
// No raw keys, because there are none to have. Counts and closed vocabulary only.
type CaptureView struct {
	Relationship RelationshipRef `json:"relationship"`
	State        CaptureState    `json:"state"`
	Reason       CaptureReason   `json:"reason,omitempty"`
	Events       int             `json:"events"`
	Checkpoints  int             `json:"checkpoints"`
	Inferences   int             `json:"inferences"`
	Skipped      int             `json:"skipped_inferences,omitempty"`
	Restarts     int             `json:"restarts,omitempty"`
	Waited       ArmedWait       `json:"waited,omitzero"`
}

// View summarises the capture for a status read.
func (c *Capture) View() *CaptureView {
	if c == nil {
		return nil
	}
	return &CaptureView{
		Relationship: c.Relationship, State: c.State, Reason: c.Reason,
		Events: c.Events, Checkpoints: c.Checkpoints, Inferences: c.Inferences,
		Skipped: c.Skipped, Restarts: c.Restarts,
		Waited: c.Waited,
	}
}

// PendingFollowUp returns the relationships awaiting a SECOND demonstration.
//
// Separate from PendingLearning so the two requests cannot be confused, and so a capture armed
// for a follow-up knows it is producing candidate 2.
func PendingFollowUp(top Topology) []RelationshipRef {
	var out []RelationshipRef
	for _, rel := range top.Relationships {
		if rel.FollowUp.Pending() {
			out = append(out, RelationshipRef{From: rel.From, To: rel.To})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// PendingLearning returns the relationships whose learning request is awaiting a demonstration.
//
// Deterministic order, so a session with two approved requests always arms the same one first —
// only one demonstration may be captured at a time, and which one it is must not depend on map
// iteration.
func PendingLearning(top Topology) []RelationshipRef {
	var out []RelationshipRef
	for _, rel := range top.Relationships {
		if rel.Learning.Pending() {
			out = append(out, RelationshipRef{From: rel.From, To: rel.To})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}
