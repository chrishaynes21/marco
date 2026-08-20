package observe

import (
	"fmt"
	"sort"
	"strings"
)

// Deciding what Marco actually KNOWS from one watched demonstration.
//
// # Candidate and assessment are different things, deliberately
//
// A [ProcedureCandidate] is what was observed: it happened, it is fixed, and it does not change.
// An assessment is a JUDGEMENT over that observation made against what Marco currently
// remembers — and what Marco remembers improves. A transient checkpoint nobody could recognise
// today becomes a remembered subject the moment the user confirms it, and the same demonstration
// becomes more verifiable without a single new observation.
//
// So the verdict is never written into the candidate. It is recomputed, every time, from the
// candidate plus the current topology. Baking "unverifiable" into the record would freeze a
// judgement made when Marco knew less, and nothing downstream would ever question it.
//
// # What this layer may and may not conclude
//
//	may:      "the example is internally consistent"
//	          "I could verify these checkpoints if I met them again"
//	          "I cannot yet verify this intermediate screen"
//	          "a second example would tell me whether these steps are reproducible"
//
//	may not:  "I know how to do this"
//	          "I can do this"
//	          "I learned the procedure"
//
// Nothing here sets `Verified`, registers anything, or produces something that can run. The
// strongest verdict available is that one observation hangs together.
//
// # No confidence number
//
// The useful output is not 0.73. It is the list of things Marco cannot check — because that list
// is actionable, and a number is not. The same argument the hypothesis layer makes about
// discrete status, one level up.

// CandidateVerdict is the discrete outcome of judging one demonstration.
//
// Four values, and the naming is careful. `candidate_consistent` is the STRONGEST available and
// says only that the observation hangs together — not `plausible`, which reads as "probably
// works", and certainly not `verified`. Nothing in this milestone can produce a verdict that
// means a procedure has been reproduced, because nothing has reproduced one.
type CandidateVerdict string

const (
	// CandidateConsistent is the strongest verdict here: every checkpoint is verifiable,
	// the navigation has a clear shape, and nothing blocks a future attempt from being
	// CHECKED. It does not mean the procedure works.
	CandidateConsistent CandidateVerdict = "candidate_consistent"
	// CandidateInsufficient is coherent evidence with a gap Marco cannot currently close —
	// an unrecognisable checkpoint, text it may not retain, a pointer with no semantic
	// target.
	CandidateInsufficient CandidateVerdict = "insufficient_evidence"
	// CandidateAmbiguous is a demonstration whose navigation has no clear shape: long runs,
	// backtracking, or evidence that the user was hunting rather than doing.
	CandidateAmbiguous CandidateVerdict = "ambiguous"
	// CandidateInvalid is a demonstration that describes nothing Marco can recognise —
	// typically an endpoint that is no longer a remembered subject.
	CandidateInvalid CandidateVerdict = "invalid"
)

// AssessmentReason is the CLOSED vocabulary of why a verdict came out as it did.
//
// Closed and enumerated for the same reason the learning refusals are: "Marco is not sure" has a
// dozen meanings and a reader cannot distinguish them. Each one also records whether ANOTHER
// demonstration could resolve it, which is what the next milestone will need and what stops this
// one from having to ask for one.
type AssessmentReason string

const (
	// ReasonSingleDemonstration is always present after one example. It is not a fault; it
	// is the honest ceiling on what one observation can establish.
	ReasonSingleDemonstration AssessmentReason = "single_demonstration_only"
	// ReasonIncompleteDemonstration is a capture that never reached its destination.
	ReasonIncompleteDemonstration AssessmentReason = "incomplete_demonstration"
	// ReasonStartUnverifiable and ReasonEndUnverifiable are endpoints memory can no longer
	// resolve.
	ReasonStartUnverifiable AssessmentReason = "start_unverifiable"
	ReasonEndUnverifiable   AssessmentReason = "end_unverifiable"
	// ReasonTransientCheckpoint is an intermediate screen with no durable identity — Marco
	// could perform the step and would not be able to tell whether it worked.
	ReasonTransientCheckpoint AssessmentReason = "transient_checkpoint_unverifiable"
	// ReasonRequiresTextEntry is a demonstration that crossed a screen the user typed on.
	ReasonRequiresTextEntry AssessmentReason = "requires_text_entry"
	// ReasonUnresolvedPointer is a pointer activation with no semantic target behind it.
	ReasonUnresolvedPointer AssessmentReason = "unresolved_pointer_target"
	// ReasonAmbiguousRun is a navigation run too long to read as deliberate.
	ReasonAmbiguousRun AssessmentReason = "ambiguous_navigation_run"
	// ReasonBacktracking is a run that reverses itself — evidence of hunting.
	ReasonBacktracking AssessmentReason = "backtracking_run"
	// ReasonNearCaptureBound is a demonstration close enough to a capture limit that what
	// was recorded may not be all of what happened.
	ReasonNearCaptureBound AssessmentReason = "near_capture_bound"
	// ReasonNoSteps is a demonstration with no navigation between its endpoints at all.
	ReasonNoSteps AssessmentReason = "no_steps"
	// ReasonDemonstrationsDisagree is two approved examples of one route that describe
	// materially different navigation.
	//
	// NOT resolvable by another demonstration — not because a third could not exist, but
	// because this milestone deliberately stops at two and a reason that invited a third
	// would be the nagging loop in another costume. Deciding whether a third is worth asking
	// for needs the evidence these two produced.
	ReasonDemonstrationsDisagree AssessmentReason = "demonstrations_disagree"
)

// ResolvableByDemonstration reports whether another example could settle this.
//
// The whole preparation for the next milestone, and the reason it needs nothing from the user
// now: a gap that a second demonstration would close is a different problem from one it would
// not, and asking for one before knowing which is asking somebody to do work for nothing.
func (r AssessmentReason) ResolvableByDemonstration() bool {
	switch r {
	case ReasonSingleDemonstration, ReasonIncompleteDemonstration,
		ReasonAmbiguousRun, ReasonBacktracking, ReasonNearCaptureBound, ReasonNoSteps,
		ReasonTransientCheckpoint:
		return true
	}
	// The rest are about what Marco is ALLOWED to keep or what it can never resolve by
	// looking again: text entry needs its own consent, a pointer with no semantic target is
	// the same pointer next time, an endpoint memory has lost stays lost, and two
	// demonstrations that disagree are not settled by a third this milestone may not ask for.
	//
	// # Correction, 2026-08-10: transient checkpoints ARE resolvable
	//
	// [[ADR-021-a-judgement-is-recomputed-not-recorded]] put this on the other side, arguing
	// that watching the same unrecognised screen again does not make it recognisable. True,
	// and it answers the wrong question. A second demonstration cannot give the screen a
	// durable identity, but it CAN corroborate that the same unrecognised screen appears at
	// the same point of the same route — which is real evidence about the route even though
	// it is not recognition. "Would another example reduce my uncertainty" and "would it fix
	// this gap" are different questions, and this method answers the first.
	return false
}

// CheckpointVerification is whether Marco could tell, later, that it had arrived here.
//
// The central question of this milestone, asked per checkpoint. A procedure Marco could perform
// and could not check is blind replay, and the whole perception stack exists so that execution
// never has to be.
type CheckpointVerification struct {
	// Position is 0 for the start and 1..n for each step's destination.
	Position int `json:"position"`
	// Subject is the remembered subject, empty for a transient checkpoint.
	Subject string `json:"subject,omitempty"`
	// Verifiable says Marco could recognise this screen again.
	Verifiable bool `json:"verifiable"`
	// Why names the gap, empty when there is none.
	Why AssessmentReason `json:"why,omitempty"`
}

// CandidateAssessment is what Marco currently concludes about one demonstration.
//
// Derived, never stored on the candidate — see the package note. Recomputing it is cheap and is
// the only way an improving memory can improve an old observation.
type CandidateAssessment struct {
	Relationship RelationshipRef  `json:"relationship"`
	Verdict      CandidateVerdict `json:"verdict"`
	// Reasons are why, in the closed vocabulary, deduplicated and in a stable order.
	Reasons []AssessmentReason `json:"reasons,omitempty"`
	// Checkpoints is the verification coverage, in order: which parts of this demonstration
	// Marco could check and which it could not.
	//
	// A list rather than a percentage. "70% verifiable" tells a reader nothing they can act
	// on; "the second screen cannot be recognised" tells them exactly what to fix.
	Checkpoints []CheckpointVerification `json:"checkpoints,omitempty"`
	// Steps is how many legs the demonstration had, and Events how much navigation.
	Steps  int `json:"steps"`
	Events int `json:"events"`
	// Verified is always false, and is here to be read rather than as a placeholder.
	//
	// A consumer asking "may I use this" must get an answer, and after one watched example
	// the answer is no. Nothing in this package can set it true.
	Verified bool `json:"verified"`
}

// NeedsAnotherDemonstration reports whether a second example could close any current gap.
func (a CandidateAssessment) NeedsAnotherDemonstration() bool {
	for _, r := range a.Reasons {
		if r.ResolvableByDemonstration() {
			return true
		}
	}
	return false
}

// ConfirmableByRehearsal reports whether Marco TRYING the thing itself would settle this.
//
// # Why this is a different question from ResolvableByDemonstration
//
// One reason, and only one, is answered better by an attempt than by a repetition.
// `single_demonstration_only` is not a fault in the demonstration; it is the observation that
// there has been exactly one of them. Asking a person to perform the same route a second time
// buys one more observation of the same kind — and Marco doing it and arriving where it expected
// is evidence of a different and stronger kind: that what was understood is sufficient to act on.
//
// Refusing to rehearse because there has only been one demonstration is also circular. The
// mechanism that would raise confidence is the mechanism being withheld for lack of confidence.
//
// Every other resolvable reason stays resolvable-by-demonstration, because each names something
// Marco could not READ — a run it could not tell from hunting, a screen with no identity, a
// capture that may be missing its own end. An attempt does not clarify a reading; it acts on one.
func (r AssessmentReason) ConfirmableByRehearsal() bool {
	return r == ReasonSingleDemonstration
}

// Blocking is the reasons that must be closed by more evidence BEFORE Marco may offer to try.
//
// Returned as a list rather than a bool so a person can be told what is actually unresolved.
// "Show me the whole thing again" is the answer to every uncertainty at once, which means it is
// the answer to none of them in particular.
func (a CandidateAssessment) Blocking() []AssessmentReason {
	var out []AssessmentReason
	for _, r := range a.Reasons {
		if r.ResolvableByDemonstration() && !r.ConfirmableByRehearsal() {
			out = append(out, r)
		}
	}
	return out
}

// BlocksRehearsal reports whether anything stands between this assessment and an offer to try.
func (a CandidateAssessment) BlocksRehearsal() bool { return len(a.Blocking()) > 0 }

// Thresholds for reading a navigation run.
const (
	// MaxDeliberateRun is the longest run this layer reads as one deliberate move.
	//
	// Six. A menu step is a few directional presses and a confirm; a run of twelve is
	// somebody looking for something, and treating it as a procedure step would teach the
	// hunting rather than the destination.
	MaxDeliberateRun = 6
	// NearBoundFraction is how close to a capture bound counts as "what was recorded may
	// not be all of what happened", expressed as numerator over NearBoundOf.
	NearBoundFraction = 3
	NearBoundOf       = 4
)

// AssessCandidate judges one demonstration against what Marco currently remembers.
//
// # Why the topology is a parameter
//
// Because the answer depends on it, and pretending otherwise would make the assessment look
// self-contained when it is not. The same candidate assessed against a richer memory produces a
// better verdict, and that dependency is modelled rather than hidden — which is also what makes
// replay parity a statable property: the same candidate and the same topology give the same
// assessment.
// Corroboration is what a SECOND approved demonstration of the same route said.
//
// A third input to the judgement, modelled explicitly for the same reason the topology is: the
// verdict depends on it, and a function that hid the dependency would look self-contained while
// silently changing its answer.
type Corroboration struct {
	// Compared says a second demonstration exists and was compared with this one.
	Compared bool
	// Agreement is what the comparison found.
	Agreement CandidateAgreement
}

func AssessCandidate(c ProcedureCandidate, top Topology, b CaptureBounds,
	corr Corroboration) CandidateAssessment {
	if b.MaxEvents == 0 {
		b = DefaultCaptureBounds()
	}
	out := CandidateAssessment{
		Relationship: c.Relationship,
		Steps:        len(c.Steps),
		Events:       c.Events,
	}
	seen := map[AssessmentReason]bool{}
	add := func(r AssessmentReason) {
		if !seen[r] {
			seen[r] = true
			out.Reasons = append(out.Reasons, r)
		}
	}

	// One example is the ceiling on what any of this can establish, and it is stated first
	// so no reader has to infer it from the absence of anything else.
	//
	// A SECOND approved demonstration lifts it — but only when the two agree. Two examples
	// that describe different routes are not two pieces of evidence for one procedure; they
	// are evidence that Marco does not know what the procedure is.
	switch {
	case !corr.Compared:
		add(ReasonSingleDemonstration)
	case corr.Agreement == AgreementDifferent:
		add(ReasonDemonstrationsDisagree)
	case corr.Agreement == AgreementSame || corr.Agreement == AgreementCompatible:
		// Corroborated. Nothing is added, and nothing is PROMOTED either: two agreeing
		// observations are still observations, and `candidate_consistent` remains the
		// ceiling.
	default:
		// Incomparable — a second demonstration exists and says nothing about this one.
		add(ReasonSingleDemonstration)
	}
	if !c.Complete {
		add(ReasonIncompleteDemonstration)
	}

	// ENDPOINTS. Verified through the semantic-memory machinery, not by trusting the ids the
	// candidate carries: a subject can be removed, and an edge to a screen nothing remembers
	// describes nothing.
	start := CheckpointVerification{Position: 0, Subject: c.Start.Subject}
	switch {
	case c.Start.Subject == "" || c.Start.Transient:
		start.Why = ReasonStartUnverifiable
		add(ReasonStartUnverifiable)
	case !subjectKnown(top, c.Start.Subject):
		start.Why = ReasonStartUnverifiable
		add(ReasonStartUnverifiable)
	default:
		start.Verifiable = true
	}
	out.Checkpoints = append(out.Checkpoints, start)

	if len(c.Steps) == 0 {
		add(ReasonNoSteps)
	}

	for i, s := range c.Steps {
		cp := CheckpointVerification{Position: i + 1, Subject: s.Arrived.Subject}
		last := i == len(c.Steps)-1
		switch {
		case s.Arrived.Transient || s.Arrived.Subject == "":
			// A screen that had no durable identity WHEN IT WAS SEEN. It may have one
			// now: the user may have named it since, and the candidate cannot know that
			// because a candidate is immutable. So the structure is resolved against
			// memory HERE, which is the whole point of judging rather than recording.
			if id, ok := resolveTransient(top, s.Arrived.Structure); ok {
				cp.Subject, cp.Verifiable = id, true
				break
			}
			// Still unrecognisable. Kept as evidence, and not something a future attempt
			// could check itself against — the gap this layer exists to name rather than
			// to flatten away.
			cp.Why = ReasonTransientCheckpoint
			add(ReasonTransientCheckpoint)
		case !subjectKnown(top, s.Arrived.Subject):
			cp.Why = ReasonEndUnverifiable
			if !last {
				cp.Why = ReasonTransientCheckpoint
			}
			add(cp.Why)
		default:
			cp.Verifiable = true
		}
		if last && cp.Why == ReasonTransientCheckpoint {
			// The DESTINATION specifically. A route whose end cannot be recognised cannot
			// be told to have succeeded at all.
			add(ReasonEndUnverifiable)
		}
		out.Checkpoints = append(out.Checkpoints, cp)

		if s.RequiresTextEntry {
			add(ReasonRequiresTextEntry)
		}
		for k, intent := range s.Intents {
			// A pointer activation with no semantic control behind it. Geometry is
			// evidence, never identity — inventing a button from a coordinate is the
			// failure ADR-004 exists for, one layer up.
			//
			// A press that RESOLVED — the evidence at the time identified the control, and
			// its name was admitted — is a different fact: `point` aimed at "Mouse" is as
			// reproducible a claim as `confirm`, and refusing it was the route-centric
			// world's way of making every mouse-driven demonstration unlearnable.
			if intent == NavPoint && !s.TargetAt(k).Named() {
				add(ReasonUnresolvedPointer)
			}
		}
		if len(s.Intents) > MaxDeliberateRun {
			add(ReasonAmbiguousRun)
		}
		if backtracks(s.Intents) {
			add(ReasonBacktracking)
		}
	}

	// VOLUME IS NOT STRENGTH. A demonstration that nearly hit a bound may be missing the end
	// of itself, and more recorded navigation is a reason for less confidence rather than
	// more.
	if c.Events*NearBoundOf >= b.MaxEvents*NearBoundFraction ||
		c.Checkpoints*NearBoundOf >= b.MaxCheckpoints*NearBoundOf-NearBoundOf {
		add(ReasonNearCaptureBound)
	}

	sort.Slice(out.Reasons, func(i, j int) bool { return out.Reasons[i] < out.Reasons[j] })
	out.Verdict = verdictFrom(seen)
	return out
}

// verdictFrom maps the reasons onto the discrete verdict.
//
// Ordered by severity, and `single_demonstration_only` alone is deliberately NOT a downgrade:
// it is the honest ceiling on every assessment this milestone can produce, and if its presence
// blocked the best verdict then the best verdict would be unreachable and therefore meaningless.
func verdictFrom(seen map[AssessmentReason]bool) CandidateVerdict {
	switch {
	case seen[ReasonStartUnverifiable] || seen[ReasonEndUnverifiable] ||
		seen[ReasonIncompleteDemonstration]:
		// Neither end can be recognised, or the demonstration never finished. There is
		// nothing here a future attempt could even be aimed at.
		return CandidateInvalid
	case seen[ReasonAmbiguousRun] || seen[ReasonBacktracking] ||
		seen[ReasonDemonstrationsDisagree]:
		return CandidateAmbiguous
	case seen[ReasonTransientCheckpoint] || seen[ReasonRequiresTextEntry] ||
		seen[ReasonUnresolvedPointer] || seen[ReasonNearCaptureBound] || seen[ReasonNoSteps]:
		return CandidateInsufficient
	}
	return CandidateConsistent
}

// subjectKnown reports whether memory currently holds this subject.
func subjectKnown(top Topology, id string) bool {
	if id == "" {
		return false
	}
	_, ok := top.Subjects[id]
	return ok
}

// backtracks reports a run that reverses itself.
//
// `down, up` or `left, right` adjacent is somebody who overshot and corrected — evidence of
// hunting for something rather than knowing where it is. Harmless in a person and misleading in
// a procedure, because the correction is not part of how the thing is done.
func backtracks(run []NavIntent) bool {
	opposite := map[NavIntent]NavIntent{
		NavUp: NavDown, NavDown: NavUp, NavLeft: NavRight, NavRight: NavLeft,
	}
	for i := 1; i < len(run); i++ {
		if opposite[run[i-1]] == run[i] && run[i] != "" {
			return true
		}
	}
	return false
}

// DescribeAssessment renders one judgement for a person.
func DescribeAssessment(a CandidateAssessment) []string {
	out := []string{
		fmt.Sprintf("candidate: %s → %s", a.Relationship.From, a.Relationship.To),
		"assessment: " + string(a.Verdict),
	}
	for _, r := range a.Reasons {
		line := "  reason: " + string(r)
		if r.ResolvableByDemonstration() {
			line += " (another example could settle this)"
		}
		out = append(out, line)
	}
	var verifiable, gaps []string
	for _, cp := range a.Checkpoints {
		name := "checkpoint " + fmt.Sprint(cp.Position)
		if cp.Position == 0 {
			name = "start"
		} else if cp.Position == len(a.Checkpoints)-1 {
			name = "end"
		}
		if cp.Verifiable {
			verifiable = append(verifiable, name)
			continue
		}
		gaps = append(gaps, fmt.Sprintf("%s (%s)", name, cp.Why))
	}
	if len(verifiable) > 0 {
		out = append(out, "verifiable: "+joinWords(verifiable))
	}
	if len(gaps) > 0 {
		out = append(out, "not verifiable: "+joinWords(gaps))
	}
	out = append(out, "verified: no. This is one watched example, judged against what Marco "+
		"remembers now. Nothing here says the steps would work, and nothing here can run them")
	return out
}

func joinWords(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// ── comparing two demonstrations ──────────────────────────────────────────────

// CandidateAgreement is how two demonstrations of the same relationship relate.
//
// Discrete, and deliberately not an edit distance over raw input. Two people — or one person
// twice — do not press identical keys, and a metric over presses would call every honest repeat
// "different". What must agree is the SEMANTICS: the same screens in the same order, reached by
// navigation of the same shape.
type CandidateAgreement string

const (
	// AgreementSame is the same checkpoints in the same order with the same runs.
	AgreementSame CandidateAgreement = "same"
	// AgreementCompatible is the same route, differing only in ways that changed nothing:
	// extra directional presses that moved no checkpoint, a skipped observation, a screen
	// that has since become recognisable.
	AgreementCompatible CandidateAgreement = "compatible"
	// AgreementDifferent is a materially different route.
	AgreementDifferent CandidateAgreement = "different"
	// AgreementIncomparable is two demonstrations of different relationships, or one that
	// never completed. Held apart from `different` for the usual reason: one means Marco
	// knows they disagree, the other means the question does not apply.
	AgreementIncomparable CandidateAgreement = "incomparable"
)

// CompareCandidates decides whether two demonstrations describe the same procedure evidence.
//
// # The tolerance, stated
//
// Identical is not the bar and could not be: a demonstration is a person moving through a menu,
// and the same person doing the same thing twice will press a direction once more or less. What
// this compares is
//
//   - the endpoints, by durable subject;
//   - the CHECKPOINT SEQUENCE, by durable subject where there is one and by safe structure
//     where there is not — so a transient screen that has since been recognised still matches
//     the transient one it used to be;
//   - the DECISIVE navigation of each run: the non-directional intents, in order. `confirm` is
//     the part of `down, down, confirm` that did something, and `down, down, down, confirm`
//     is the same move made from one row further away.
//
// Directional padding is therefore tolerated and everything else is not. `left, back, down,
// confirm` does not reduce to `confirm`, because `back` is decisive.
func CompareCandidates(a, b ProcedureCandidate) CandidateAgreement {
	if a.Relationship != b.Relationship || !a.Complete || !b.Complete {
		return AgreementIncomparable
	}
	if a.Start.Subject != b.Start.Subject {
		return AgreementDifferent
	}
	if len(a.Steps) != len(b.Steps) {
		return AgreementDifferent
	}
	exact := true
	for i := range a.Steps {
		x, y := a.Steps[i], b.Steps[i]
		if !sameCheckpoint(x.Arrived, y.Arrived) {
			return AgreementDifferent
		}
		if x.RequiresTextEntry != y.RequiresTextEntry {
			// One of them crossed a screen the user typed on and the other did not. That
			// is not padding; it is a different route through the interface.
			return AgreementDifferent
		}
		if !sameIntentRun(x.Intents, y.Intents) {
			exact = false
			if !sameDecisiveRun(x.Intents, y.Intents) {
				return AgreementDifferent
			}
		}
		// Two clicks are the same step only when they aimed at the same thing. Intents
		// alone cannot see this — every click is `point` — so a demonstration that clicked
		// "Mouse" and one that clicked "Bluetooth & devices" would otherwise corroborate
		// each other.
		if !samePointTargets(x, y) {
			return AgreementDifferent
		}
	}
	if exact {
		return AgreementSame
	}
	return AgreementCompatible
}

func sameIntentRun(a, b []NavIntent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameDecisiveRun compares two runs ignoring directional padding.
func sameDecisiveRun(a, b []NavIntent) bool {
	return sameIntentRun(decisive(a), decisive(b))
}

// samePointTargets compares what the two legs' pointer presses were aimed at, in order.
//
// Only the NAMED targets take part: a press that resolved to nothing constrains nothing,
// because "unknown" is not a claim about which control it was.
func samePointTargets(a, b DemonstrationStep) bool {
	an, bn := namedPointLabels(a), namedPointLabels(b)
	if len(an) != len(bn) {
		return len(an) == 0 || len(bn) == 0
	}
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}

func namedPointLabels(s DemonstrationStep) []string {
	var out []string
	for i, intent := range s.Intents {
		if intent != NavPoint {
			continue
		}
		if t := s.TargetAt(i); t.Named() {
			out = append(out, t.Label)
		}
	}
	return out
}

// decisive strips the intents that only move a selection.
//
// Up, down, left and right change WHERE a selection is; they do not commit to anything. Two
// demonstrations that differ only in how many rows the user moved are the same move made from
// different starting positions, and a comparison that called them different would never find two
// demonstrations compatible.
//
// `back`, `confirm`, `pause`, `menu` and `point` are all decisive and are all kept.
func decisive(in []NavIntent) []NavIntent {
	out := make([]NavIntent, 0, len(in))
	for _, i := range in {
		switch i {
		case NavUp, NavDown, NavLeft, NavRight:
			continue
		}
		out = append(out, i)
	}
	return out
}

// ── asking for a second demonstration ─────────────────────────────────────────

// FollowUpRefusal is the CLOSED vocabulary of why Marco did NOT ask for another example.
//
// Silence is the hard case here more than anywhere else in this system: a user who agreed to
// show Marco something and then heard nothing has no way to tell whether Marco is satisfied,
// stuck, or broken.
type FollowUpRefusal string

const (
	// RefusalNoCompletedCandidate is nothing to ask about.
	RefusalNoCompletedCandidate FollowUpRefusal = "no_completed_candidate"
	// RefusalNothingResolvable is a judgement no further example could improve.
	RefusalNothingResolvable FollowUpRefusal = "follow_up_not_resolvable"
	// RefusalNonResolvableBlocker is the important one: something else about this candidate
	// makes another example pointless, whatever it would resolve.
	RefusalNonResolvableBlocker FollowUpRefusal = "non_resolvable_blocker"
	// RefusalCandidateInvalid is a demonstration that describes nothing recognisable.
	RefusalCandidateInvalid FollowUpRefusal = "candidate_invalid"
	// RefusalFollowUpPending is already asked and agreed to; Marco is waiting to be shown.
	RefusalFollowUpPending FollowUpRefusal = "follow_up_pending"
	// RefusalFollowUpDeclined and RefusalFollowUpRefused are what the user already said.
	RefusalFollowUpDeclined FollowUpRefusal = "already_declined"
	RefusalFollowUpRefused  FollowUpRefusal = "already_refused"
	// RefusalSecondAlreadyCaptured is the bound: one follow-up, then stop.
	RefusalSecondAlreadyCaptured FollowUpRefusal = "second_demo_already_captured"
	// RefusalAnotherQuestionOpen is the shared interruption budget.
	RefusalFollowUpQuestionOpen FollowUpRefusal = "another_question_open"
	// RefusalAlreadyAsked is this session's own question, awaiting an answer.
	RefusalFollowUpAlreadyAsked FollowUpRefusal = "already_asked"
)

// FollowUpJudgement is whether another example is worth asking for, and why not.
type FollowUpJudgement struct {
	Eligible bool `json:"eligible"`
	// Resolvable is what another demonstration COULD improve.
	Resolvable []AssessmentReason `json:"resolvable,omitempty"`
	// Blocking is what it could not, and what therefore makes asking pointless.
	Blocking []AssessmentReason `json:"blocking,omitempty"`
	Refusals []FollowUpRefusal  `json:"refusals,omitempty"`
}

// FollowUpFrom decides whether another demonstration is worth asking for.
//
// # The rule is the assessment's, not this function's
//
// Nothing here inspects the candidate. It reads the verdict and partitions the reasons by
// `ResolvableByDemonstration`, which is where the knowledge lives. Duplicating the assessment's
// rules in proposal code would be a second answer to "is this evidence any good", and the two
// would drift the first time either changed.
//
// # Why one non-resolvable blocker is enough to stop
//
// Conservative on purpose. A candidate with `single_demonstration_only` and
// `requires_text_entry` has a gap another example WOULD close and a gap it would not — and the
// second one means the exercise stays unusable however well the first goes. Asking anyway would
// be asking somebody to do work whose value Marco already knows is capped, which is exactly the
// nagging this whole layer is arranged to avoid.
func FollowUpFrom(a CandidateAssessment) FollowUpJudgement {
	out := FollowUpJudgement{}
	for _, r := range a.Reasons {
		switch {
		case r.ConfirmableByRehearsal():
			// NOT a reason to ask for another example. `single_demonstration_only` is
			// answered better by Marco trying the thing than by the person performing it
			// twice — see [[ADR-051-one-demonstration-and-an-attempt]] — and asking anyway
			// spends the one interruption slot the rehearsal question needs.
			//
			// Live, that was the whole failure: a clean one-shot candidate produced
			// "want to show me again?", the rehearsal review then refused with
			// `another_question_open`, and Learn waited for a grant that could not
			// exist because the question granting it was never asked.
			continue
		case r.ResolvableByDemonstration():
			out.Resolvable = append(out.Resolvable, r)
		default:
			out.Blocking = append(out.Blocking, r)
		}
	}
	if a.Verdict == CandidateInvalid {
		out.Refusals = append(out.Refusals, RefusalCandidateInvalid)
	}
	if len(out.Blocking) > 0 {
		out.Refusals = append(out.Refusals, RefusalNonResolvableBlocker)
	}
	if len(out.Resolvable) == 0 {
		out.Refusals = append(out.Refusals, RefusalNothingResolvable)
	}
	out.Eligible = len(out.Refusals) == 0
	return out
}

// FollowUpProposalIdentity derives the question's identity from the durable edge.
//
// Distinct from the learning question's identity — a different question about the same
// relationship — and derived from nothing session-local, so two sessions that both notice the
// gap ask once.
func FollowUpProposalIdentity(from, to string) ProposalID {
	return ProposalID("f_" + digest("", "followup", from+">"+to))
}

// FollowUpDigest is the SHAPE of the judgement a follow-up question was asked on.
//
// The material-change key. Built from the verdict, the reason SET and the verification coverage
// pattern — never from counts, never from timestamps, never from the rendered text. A declined
// question comes back when Marco's judgement has genuinely changed, and not because a session
// ran again.
//
// `single_demonstration_only` becoming `transient_checkpoint_unverifiable` is material.
// The same reasons with a new timestamp is not.
func FollowUpDigest(a CandidateAssessment) string {
	reasons := make([]string, 0, len(a.Reasons))
	for _, r := range a.Reasons {
		reasons = append(reasons, string(r))
	}
	sort.Strings(reasons)
	coverage := ""
	for _, cp := range a.Checkpoints {
		if cp.Verifiable {
			coverage += "v"
			continue
		}
		coverage += "-"
	}
	return digest("", "followup",
		"edge="+a.Relationship.From+">"+a.Relationship.To,
		"verdict="+string(a.Verdict),
		"reasons="+strings.Join(reasons, ","),
		"coverage="+coverage)
}

// FollowUpQuestion renders the request, saying WHY Marco is asking.
//
// The wording is driven by the named gap, which is the whole product principle of this
// milestone. "Show me again" is a demand; "I couldn't verify the middle screen, and another
// example would tell me whether the same transition happens reliably" is a reason somebody can
// weigh — and if Marco cannot produce that sentence it has no business asking.
func FollowUpQuestion(a CandidateAssessment, top Topology) string {
	from := subjectName(top.Subjects[a.Relationship.From])
	to := subjectName(top.Subjects[a.Relationship.To])
	route := "that move"
	switch {
	case from != "" && to != "" && from != to:
		route = fmt.Sprintf("going from %s to %s", from, to)
	case from != "":
		route = fmt.Sprintf("going from %s to the other screen", from)
	}

	why := "I saw one example, and I'd like to check whether the same navigation repeats."
	for _, r := range a.Reasons {
		switch r {
		case ReasonTransientCheckpoint:
			why = "I saw one example, but I couldn't recognise the screen in the middle " +
				"well enough to be sure I'd know it again."
		case ReasonAmbiguousRun, ReasonBacktracking:
			why = "I saw one example, but there was enough moving about that I couldn't " +
				"tell which part was deliberate."
		case ReasonIncompleteDemonstration:
			why = "I started watching and it didn't finish, so I don't have a whole example."
		}
	}
	return fmt.Sprintf("%s Another example of %s would help me tell whether it happens the "+
		"same way each time. Want to show me again?", why, route)
}

// DescribeFollowUp renders one follow-up judgement for a person.
func DescribeFollowUp(a CandidateAssessment, j FollowUpJudgement) []string {
	out := []string{
		fmt.Sprintf("candidate: %s → %s", a.Relationship.From, a.Relationship.To),
		"assessment: " + string(a.Verdict),
	}
	for _, r := range a.Reasons {
		out = append(out, "  reason: "+string(r))
	}
	if len(j.Resolvable) > 0 {
		names := make([]string, 0, len(j.Resolvable))
		for _, r := range j.Resolvable {
			names = append(names, string(r))
		}
		out = append(out, "another demonstration could help with: "+strings.Join(names, ", "))
	}
	if len(j.Blocking) > 0 {
		names := make([]string, 0, len(j.Blocking))
		for _, r := range j.Blocking {
			names = append(names, string(r))
		}
		out = append(out, "another demonstration would not help with: "+
			strings.Join(names, ", "))
	}
	if !j.Eligible {
		names := make([]string, 0, len(j.Refusals))
		for _, r := range j.Refusals {
			names = append(names, string(r))
		}
		out = append(out, "not asked: "+strings.Join(names, ", "))
	}
	out = append(out, "authority: none. Whatever the examples show, Marco has not learned to "+
		"do this and cannot perform it")
	return out
}

// ── asking for a second demonstration ─────────────────────────────────────────

// FollowUpReport is one route's follow-up judgement, for the session result.
type FollowUpReport struct {
	Relationship RelationshipRef     `json:"relationship"`
	Assessment   CandidateAssessment `json:"assessment"`
	Judgement    FollowUpJudgement   `json:"judgement"`
	// Asked says a question was put this session.
	Asked bool `json:"asked,omitempty"`
	// Question is what was, or would be, asked.
	Question string `json:"question,omitempty"`
}

// ReviewFollowUp asks for a second demonstration of one route, where the judgement says one
// would help.
//
// # Everything about WHETHER to ask comes from the assessment
//
// This function contributes the bookkeeping and nothing else: has it been asked, has it been
// answered, has a second example already been given, is the interruption budget free. Whether
// another demonstration is worth anything is `FollowUpFrom`, over the assessment's own reasons —
// and duplicating that judgement here would be a second answer to the same question.
//
// # One follow-up, then stop
//
// Bounded at two demonstrations by construction: a route with a second candidate reports
// `second_demo_already_captured` and is never asked again. "Show me again, show me again" is a
// collection loop, and the point at which a third example is worth asking for needs the evidence
// the first two produced.
func (l *ProposalLedger) ReviewFollowUp(ref RelationshipRef, candidates []ProcedureCandidate,
	top Topology, bounds CaptureBounds, th ProposalThresholds) FollowUpReport {

	if th.MaxOpen == 0 && th.MaxProposals == 0 {
		th = DefaultProposalThresholds()
	}
	out := FollowUpReport{Relationship: ref}

	// The FIRST demonstration is the one a follow-up question is about, and a second one
	// existing is the bound rather than a new subject.
	var first *ProcedureCandidate
	var second bool
	for i := range candidates {
		switch {
		case candidates[i].Sequence >= 2:
			second = true
		case first == nil || candidates[i].Sequence < first.Sequence:
			first = &candidates[i]
		}
	}
	if first == nil || !first.Complete {
		out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalNoCompletedCandidate)
		return out
	}

	// The judgement, recomputed — including whatever the second demonstration said, so a
	// route that has already been corroborated reads as corroborated rather than as one
	// example.
	corr := Corroboration{}
	if second {
		for i := range candidates {
			if candidates[i].Sequence >= 2 {
				corr = Corroboration{
					Compared:  true,
					Agreement: CompareCandidates(*first, candidates[i]),
				}
			}
		}
	}
	out.Assessment = AssessCandidate(*first, top, bounds, corr)
	out.Judgement = FollowUpFrom(out.Assessment)
	out.Question = FollowUpQuestion(out.Assessment, top)

	if second {
		out.Judgement.Eligible = false
		out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalSecondAlreadyCaptured)
		return out
	}

	// What the user has already said about THIS request.
	digest := FollowUpDigest(out.Assessment)
	for _, rel := range top.Relationships {
		if rel.From != ref.From || rel.To != ref.To || rel.FollowUp == nil {
			continue
		}
		switch rel.FollowUp.Status {
		case LearningPending:
			out.Judgement.Eligible = false
			out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalFollowUpPending)
			return out
		case LearningFulfilled:
			out.Judgement.Eligible = false
			out.Judgement.Refusals = append(out.Judgement.Refusals,
				RefusalSecondAlreadyCaptured)
			return out
		case LearningRefused:
			// A preference, and durable. More evidence does not overturn it.
			out.Judgement.Eligible = false
			out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalFollowUpRefused)
			return out
		case LearningDeclined:
			// Suppressed until Marco's JUDGEMENT changes shape — not until a session runs
			// again, and not on a clock.
			if rel.FollowUp.Evidence == digest {
				out.Judgement.Eligible = false
				out.Judgement.Refusals = append(out.Judgement.Refusals,
					RefusalFollowUpDeclined)
				return out
			}
		}
	}

	id := FollowUpProposalIdentity(ref.From, ref.To)
	if existing := l.find(id); existing != nil {
		if existing.Status == ProposalDeclined && existing.Evidence != digest {
			existing.Evidence = digest
			existing.Status = ProposalOpen
			existing.Response = ResponseNone
			existing.Asked++
			out.Asked = true
			return out
		}
		out.Judgement.Eligible = false
		out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalFollowUpAlreadyAsked)
		return out
	}
	if !out.Judgement.Eligible {
		return out
	}
	if l.openCount() >= th.MaxOpen || len(l.Proposals) >= th.MaxProposals {
		out.Judgement.Eligible = false
		out.Judgement.Refusals = append(out.Judgement.Refusals, RefusalFollowUpQuestionOpen)
		return out
	}
	l.Proposals = append(l.Proposals, Proposal{
		ID: id, Ask: AskSecondDemonstration, Question: out.Question,
		Relationship: &RelationshipRef{From: ref.From, To: ref.To},
		Evidence:     digest, Status: ProposalOpen, Asked: 1,
		Support: []EvidenceSource{FromNavigation, FromStructure},
	})
	out.Asked = true
	return out
}

// resolveTransient asks whether a screen that was unrecognisable when it was seen has since
// become a remembered subject.
//
// # Why this happens at assessment time and not in the candidate
//
// Because the candidate is the OBSERVATION and must not change: a comparison between two
// examples is worthless if one of them has been edited since. What can change is what Marco
// knows, and a screen the user named yesterday is recognisable today — so the resolution belongs
// to the judgement, recomputed, exactly like everything else here.
//
// Ambiguity is a refusal. If more than one remembered subject matches the structure, Marco cannot
// tell which one this checkpoint was, and calling it verifiable would be calling a coin toss
// verifiable. That is the same rule `Recall` applies, for the same reason.
func resolveTransient(top Topology, sig StructureSignature) (string, bool) {
	// Members is excluded, for the reason it is excluded from checkpoint comparison: it is
	// the dominant structural group's size, and a screen glimpsed early in a visit reports 0
	// where the settled subject reports 4. Comparing on it would mean a transient checkpoint
	// could never resolve to the subject it actually is.
	found := ""
	sig.Members = 0
	for id, s := range top.Subjects {
		candidateSig := s.Structure
		candidateSig.Members = 0
		if CompareStructure(sig, candidateSig) != MatchSame {
			continue
		}
		if found != "" {
			return "", false
		}
		found = id
	}
	return found, found != ""
}

// WithRehearsal folds durable rehearsal evidence into an assessment.
//
// # Why this is a fold rather than a stored flag
//
// `Verified` is the one word in this system that means "Marco tried this and it worked", and it is
// tempting to write it down the moment a rehearsal completes. That would be wrong for the reason
// [[ADR-021-a-judgement-is-recomputed-not-recorded]] gives about every other judgement here: the
// claim depends on things that can stop being true without anybody editing the record.
//
// A completed rehearsal proves the route ran on the screens Marco knew THEN. If the demonstration
// has since been revised, the evidence is about a different procedure. If a subject has since been
// contradicted, the success is a fact about a screen nobody can find. A stored boolean would go on
// saying yes through both; this asks again every time.
//
// Nothing is set unless a stored attempt COMPLETED — a prefix, a contained step and a dry run all
// leave it false, which is `RehearsalEvidence.VerifiedBy`'s business rather than this function's.
func (a CandidateAssessment) WithRehearsal(c ProcedureCandidate, digest string, top Topology,
	evidence []RehearsalEvidence) CandidateAssessment {

	for _, e := range evidence {
		if e.Sequence != c.Sequence {
			continue
		}
		if e.VerifiedBy(c, digest, top) {
			a.Verified = true
			return a
		}
	}
	return a
}
