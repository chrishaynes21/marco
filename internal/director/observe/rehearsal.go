package observe

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The boundary between having enough evidence to ask, and having permission to try.
//
// # Three things that are not each other
//
//	CandidateAssessment  what does the demonstration evidence support?
//	RehearsalJudgement   is that evidence enough to ASK for one controlled experiment?
//	RehearsalGrant       the user said yes to one attempt.
//
// Evidence is not authority. Eligibility is not authority. A question is not authority. A
// previous yes to "learn this" is not authority — that one authorised WATCHING, and the whole
// point of a separate typed question is that the two permissions have different consequences and
// must not be reachable from one another.
//
// # And even the grant cannot act
//
// A RehearsalGrant is authority DATA. It has no method that performs anything, it lives in the
// analysis core whose transitive imports reach no keyboard, mouse, controller or runtime, and it
// is never written to disk. The engine that will one day consume it lives on the other side of
// that boundary and must ask for it.
//
// See [[ADR-023-rehearsal-is-attempt-scoped-authority]].

// ── is this worth asking about? ───────────────────────────────────────────────

// RehearsalRefusal is the CLOSED vocabulary of why Marco will not ask to rehearse.
//
// The default is refusal, so this list is the interesting half of the design. Every entry is a
// property of the ASSESSMENT rather than of the candidate: the assessment already owns "what does
// this evidence support", and a second reading of the candidate here would be a second answer to
// that question.
type RehearsalRefusal string

const (
	// RefusalNotConsistent is a verdict below `candidate_consistent`.
	RefusalNotConsistent RehearsalRefusal = "not_consistent"
	// RefusalDemonstrationsDisagree is two examples describing different routes — Marco
	// does not know which procedure it would be trying.
	RefusalDemonstrationsDisagree RehearsalRefusal = "demonstrations_disagree"
	// RefusalUnverifiableCheckpoint is a step whose result could not be checked afterwards.
	// Taking it would be acting blind.
	RefusalUnverifiableCheckpoint RehearsalRefusal = "unverifiable_checkpoint"
	// RefusalRequiresTextEntry is a route through a screen the user typed on. Nothing was
	// retained, so there is nothing to reproduce — and the fix is not to start retaining.
	RefusalRequiresTextEntry RehearsalRefusal = "requires_text_entry"
	// RefusalUnresolvedPointer is a click with no semantic control behind it. A coordinate
	// is evidence that somebody clicked somewhere; it is not a target.
	RefusalUnresolvedPointer RehearsalRefusal = "unresolved_pointer_target"
	// RefusalNearCaptureBound is a demonstration that may be missing its own ending.
	RefusalNearCaptureBound RehearsalRefusal = "near_capture_bound"
	// RefusalEndpointUnrecognised is nothing to start from, or nothing to aim at.
	RefusalEndpointUnrecognised RehearsalRefusal = "endpoint_unrecognised"
	// RefusalApplicationMismatch is a candidate belonging to another application.
	RefusalApplicationMismatch RehearsalRefusal = "application_mismatch"
	// RefusalNoSteps is a candidate with nothing to try.
	RefusalNoSteps RehearsalRefusal = "no_steps"
	// RefusalAlreadyAuthorized is an attempt already granted and not yet used.
	RefusalAlreadyAuthorized RehearsalRefusal = "already_authorized"
	// RefusalDeclined and RefusalRefused are what the user already said.
	RefusalDeclined RehearsalRefusal = "already_declined"
	RefusalRefused  RehearsalRefusal = "already_refused"
	// RefusalQuestionOpen is the shared interruption budget.
	RefusalQuestionOpen RehearsalRefusal = "another_question_open"
	// RefusalAsked is this session's own question, awaiting an answer.
	RefusalAsked RehearsalRefusal = "already_asked"
)

// StepVerifiability is how well Marco could check one step after taking it.
//
// Three values, because two would force a lie. The middle one is the design's accepted
// counterexample: `down` in a menu moves a selection the screen-state segmenter cannot resolve,
// so the step has no observation of its own — but it is still CONTAINED, because the screen must
// remain the same screen, the target must remain proven, and a later step has to settle the
// procedure. Calling that "verified" would be the collapse this vocabulary exists to prevent.
type StepVerifiability string

const (
	// DirectlyVerifiable arrives at a remembered subject. Marco could tell.
	DirectlyVerifiable StepVerifiability = "directly_verifiable"
	// ProgressUnobservable moves something Marco cannot see, inside a screen it can.
	//
	// WEAKER EVIDENCE BY DEFINITION, and never success. What it permits is continuing —
	// nothing more — and only while the screen, the target and the application all still
	// hold. See [[ADR-023-rehearsal-is-attempt-scoped-authority]].
	ProgressUnobservable StepVerifiability = "progress_unobservable"
	// Unverifiable is a step Marco could not check at all. It blocks rehearsal.
	Unverifiable StepVerifiability = "unverifiable"
)

// Progressed reports whether a step of this kind may be followed by another.
//
// Deliberately NOT named `Succeeded`. A progress-unobservable step permits continuing and
// establishes nothing, and a method called `Succeeded` returning true for it is exactly how the
// distinction would be lost in six months.
func (v StepVerifiability) Progressed() bool {
	return v == DirectlyVerifiable || v == ProgressUnobservable
}

// StepPlan is one step of the candidate, judged for whether it could be checked.
type StepPlan struct {
	// Position is 1..n over the candidate's steps.
	Position int `json:"position"`
	// Intents is the ordered navigation this step would issue.
	Intents []NavIntent `json:"intents,omitempty"`
	// Targets is aligned with Intents: what each demonstrated event was aimed at, carried
	// so an attempt can aim at the same thing. A `point` with a named target is emitted as
	// an invocation of that control, resolved live at emission time; one without a name is
	// unexpressable and never reaches a plan (the assessment refuses it first).
	Targets []SemanticTarget `json:"targets,omitempty"`
	// Verifiability is how well the result could be checked.
	Verifiability StepVerifiability `json:"verifiability"`
	// Expect is the remembered subject the screen must show once the step is taken.
	//
	// For a directly-verifiable step that is where it ARRIVES; for a progress-unobservable one
	// it is the screen it must REMAIN on, which is the whole of what containment means. The two
	// readings are told apart by Verifiability, never by this field being empty.
	Expect string `json:"expect,omitempty"`
}

// RehearsalJudgement is whether Marco may ASK to rehearse, and why not.
//
// Derived and recomputed, never durable truth — the same discipline
// [[ADR-021-a-judgement-is-recomputed-not-recorded]] established for the assessment it reads.
type RehearsalJudgement struct {
	Relationship RelationshipRef `json:"relationship"`
	Application  string          `json:"application"`
	Eligible     bool            `json:"eligible"`
	// Refusals is why not, in the closed vocabulary.
	Refusals []RehearsalRefusal `json:"refusals,omitempty"`
	// Plan is the steps Marco would take, each with how well it could be checked.
	Plan []StepPlan `json:"plan,omitempty"`
	// Source and Destination are where the rehearsal would begin and end.
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	// Inputs is how many semantic inputs the whole rehearsal would issue.
	Inputs int `json:"inputs"`
	// Unobservable counts the steps whose progress could not be checked.
	Unobservable int `json:"unobservable,omitempty"`
	// LongestUnobservable is the longest CONSECUTIVE run of them.
	//
	// The bound an attempt is actually held to, and tighter than the constant: a plan
	// whose every step lands on a different remembered screen carries 0, so a step that
	// could only be contained cannot be smuggled into an attempt authorised for a plan
	// that never had one.
	LongestUnobservable int `json:"longest_unobservable,omitempty"`
	// Sequence is which demonstration of the route this judgement is about.
	//
	// Carried so a stored rehearsal result can say WHICH example was rehearsed — a route with
	// two demonstrations that disagree must not have one of them vouched for by the other.
	Sequence int `json:"sequence,omitempty"`
	// Digest binds a question to this exact evidence. See RehearsalDigest.
	Digest string `json:"digest,omitempty"`
}

// MaxUnobservableRun bounds how many progress-unobservable steps may run consecutively.
//
// Two. `down, down, confirm` is the shape every menu route has, and it is two unseeable moves
// followed by one that settles everything. A third would mean Marco is walking further into a
// screen it cannot read than any demonstration in this repository has ever needed.
const MaxUnobservableRun = 2

// JudgeRehearsal decides whether one candidate has earned a rehearsal question.
//
// # Everything about WHETHER comes from the assessment
//
// This function reads the verdict and the reasons, and maps them. It does not re-examine the
// demonstration's evidence, because CandidateAssessment already did and two readings would
// eventually disagree. What it adds is the rehearsal-specific question the assessment never
// asks: for each step Marco would take, could it check the result?
func JudgeRehearsal(c ProcedureCandidate, a CandidateAssessment, top Topology,
	application string) RehearsalJudgement {

	out := RehearsalJudgement{
		Relationship: c.Relationship,
		Application:  application,
		Source:       c.Start.Subject,
		Destination:  c.Relationship.To,
		Sequence:     c.Sequence,
		Inputs:       c.Events,
	}
	seen := map[RehearsalRefusal]bool{}
	add := func(r RehearsalRefusal) {
		if !seen[r] {
			seen[r] = true
			out.Refusals = append(out.Refusals, r)
		}
	}

	// Scope, and the unscoped case with it. Two empty strings compare equal, so a candidate
	// belonging to no application would otherwise pass a check meant to confine it to one.
	if application == "" || c.Application == "" || !strings.EqualFold(c.Application, application) {
		add(RefusalApplicationMismatch)
	}
	if a.Verdict != CandidateConsistent {
		add(RefusalNotConsistent)
	}
	// The assessment's reasons map one-to-one onto rehearsal refusals. Mapped rather than
	// re-derived, so a change to what the assessment concludes reaches here automatically.
	for _, r := range a.Reasons {
		switch r {
		// `single_demonstration_only` is deliberately absent. It used to refuse here, and
		// the refusal was circular: a rehearsal is how Marco finds out whether one
		// demonstration was enough, and withholding it for lack of confidence withheld the
		// only thing that could produce confidence. What it cost was a person being asked to
		// perform the same route twice and being refused when the second attempt to watch
		// them failed — with the correct route already in the store.
		//
		// The safety property is unchanged and lives elsewhere: a rehearsal still happens
		// only when the user says yes to it. See
		// [[ADR-051-one-demonstration-and-an-attempt]].
		case ReasonDemonstrationsDisagree:
			add(RefusalDemonstrationsDisagree)
		case ReasonTransientCheckpoint:
			add(RefusalUnverifiableCheckpoint)
		case ReasonRequiresTextEntry:
			add(RefusalRequiresTextEntry)
		case ReasonUnresolvedPointer:
			add(RefusalUnresolvedPointer)
		case ReasonNearCaptureBound:
			add(RefusalNearCaptureBound)
		case ReasonStartUnverifiable, ReasonEndUnverifiable:
			add(RefusalEndpointUnrecognised)
		case ReasonNoSteps:
			add(RefusalNoSteps)
		}
	}
	if len(c.Steps) == 0 {
		add(RefusalNoSteps)
	}
	if c.Start.Subject == "" || !subjectKnown(top, c.Start.Subject) {
		add(RefusalEndpointUnrecognised)
	}

	// THE rehearsal-specific question, asked per step: could Marco check this afterwards?
	//
	// The discriminator is where the step LANDED relative to where it started, not whether the
	// checkpoint was transient. A step that leaves for another remembered screen is checkable.
	// A step that leaves the screen behind for something Marco does not recognise is not. And a
	// step that stays on the screen it started on is the middle case — `down` moved a selection
	// nothing in the observation stack can see, and all Marco can confirm is that the screen it
	// recognises is still the screen it recognises.
	run, prev := 0, c.Start.Subject
	for i, s := range c.Steps {
		p := StepPlan{Position: i + 1, Intents: append([]NavIntent{}, s.Intents...)}
		if len(s.Targets) > 0 {
			p.Targets = append([]SemanticTarget{}, s.Targets...)
		}
		at, known := arrivedAt(top, s.Arrived)
		switch {
		case known && at != prev:
			p.Verifiability, p.Expect = DirectlyVerifiable, at
			run, prev = 0, at
		case known:
			// CONTAINED, not successful. What this permits is carrying on — nothing more
			// — and only while the screen it must remain on is still there.
			p.Verifiability, p.Expect = ProgressUnobservable, at
			out.Unobservable++
			run++
			if run > out.LongestUnobservable {
				out.LongestUnobservable = run
			}
			if run > MaxUnobservableRun {
				// Further into a screen Marco cannot read than any demonstration has
				// ever needed. Past here it is guessing how far it has got.
				add(RefusalUnverifiableCheckpoint)
			}
		default:
			p.Verifiability = Unverifiable
			add(RefusalUnverifiableCheckpoint)
			run, prev = 0, ""
		}
		out.Plan = append(out.Plan, p)
	}
	// The last step has to settle the procedure. A rehearsal that ended on something Marco
	// could not check would be a rehearsal with no result — and one that ended on a step it
	// could only contain would be an attempt with no answer either way.
	if n := len(out.Plan); n > 0 && out.Plan[n-1].Verifiability != DirectlyVerifiable {
		add(RefusalUnverifiableCheckpoint)
	}

	sort.Slice(out.Refusals, func(i, j int) bool { return out.Refusals[i] < out.Refusals[j] })
	out.Eligible = len(out.Refusals) == 0
	out.Digest = RehearsalDigest(c, a, out)
	return out
}

// arrivedAt resolves the screen a step landed on, by the same rule the assessment uses.
//
// A checkpoint that named a remembered subject names it. One that did not is matched on its
// structure, because a screen glimpsed mid-transition is still that screen — this is the
// resolution [[ADR-021-a-judgement-is-recomputed-not-recorded]] already established, and a
// different rule here would be a second opinion about what Marco is looking at.
func arrivedAt(top Topology, cp Checkpoint) (string, bool) {
	if cp.Subject != "" && subjectKnown(top, cp.Subject) {
		return cp.Subject, true
	}
	return resolveTransient(top, cp.Structure)
}

// RehearsalDigest binds a question to the exact evidence it was asked about.
//
// # Why a digest and not a pointer
//
// Because the candidate can change between the question and the answer, and a pointer would
// silently authorise the newest version. The digest covers the verdict, the reason set, the
// endpoints, and the plan's shape — everything a person was implicitly agreeing to when they said
// yes. If any of it moves, the grant is refused and Marco asks again rather than acting on
// evidence the user never saw.
func RehearsalDigest(c ProcedureCandidate, a CandidateAssessment, j RehearsalJudgement) string {
	reasons := make([]string, 0, len(a.Reasons))
	for _, r := range a.Reasons {
		reasons = append(reasons, string(r))
	}
	sort.Strings(reasons)

	var plan strings.Builder
	for _, p := range j.Plan {
		plan.WriteString(string(p.Verifiability))
		plan.WriteString(":")
		for k, i := range p.Intents {
			plan.WriteString(string(i))
			// The target is part of what the person agreed to. A plan that clicks
			// "Mouse" and one that clicks "Bluetooth & devices" must not share a digest,
			// or a yes to one could authorise the other.
			if k < len(p.Targets) && p.Targets[k].Named() {
				plan.WriteString("@" + p.Targets[k].Role + "=" + p.Targets[k].Label)
			}
			plan.WriteString(",")
		}
		plan.WriteString(">" + p.Expect + "|")
	}
	return digest("", "rehearsal",
		"app="+strings.ToLower(j.Application),
		"edge="+c.Relationship.From+">"+c.Relationship.To,
		"start="+c.Start.Subject,
		"verdict="+string(a.Verdict),
		"reasons="+strings.Join(reasons, ","),
		"plan="+plan.String(),
		fmt.Sprintf("inputs=%d", c.Events))
}

// RehearsalProposalIdentity derives the question's identity from the durable edge.
func RehearsalProposalIdentity(from, to string) ProposalID {
	return ProposalID("r_" + digest("", "rehearse", from+">"+to))
}

// RehearsalQuestion renders the request, in the words a person reads.
//
// What it has to communicate: what Marco thinks it learned, where it starts, what it will try,
// what it expects, that this is one attempt, and that it stops the moment the screen is not what
// it expected. No ids, no digests, no confidence, no reason enums.
func RehearsalQuestion(j RehearsalJudgement, top Topology) string {
	// NAMED, always. Two questions that both said "this move" were two different routes
	// and the Audience could not tell them apart. See PlaceWords.
	from := PlaceWords(top.Subjects[j.Source])
	to := PlaceWords(top.Subjects[j.Destination])
	if from != "" && from == to {
		to = ""
	}
	route := "this move"
	switch {
	case from != "" && to != "":
		route = fmt.Sprintf("getting from %s to %s", from, to)
	case from != "":
		route = fmt.Sprintf("getting from %s to the other screen", from)
	case to != "":
		route = fmt.Sprintf("getting to %s", to)
	}
	return fmt.Sprintf("I've watched %s twice and both times went the same way. "+
		"I'd like to try it once myself, one step at a time, and stop the moment the screen "+
		"isn't what I expected. Shall I have a go?", route)
}

// DescribeRehearsal renders one judgement for a person.
func DescribeRehearsal(j RehearsalJudgement, top Topology) []string {
	out := []string{"rehearsal: " + eligibleWord(j.Eligible)}
	for _, r := range j.Refusals {
		out = append(out, "  reason: "+string(r))
	}
	if j.Eligible {
		out = append(out,
			fmt.Sprintf("  %d step(s), %d semantic input(s)", len(j.Plan), j.Inputs))
		if j.Unobservable > 0 {
			out = append(out, fmt.Sprintf("  %d step(s) move something Marco cannot see; "+
				"it would check the screen has not changed and carry on", j.Unobservable))
		}
		out = append(out, "  question: "+RehearsalQuestion(j, top))
	}
	out = append(out, "authority: none yet. Asking is not permission, and nothing here can act")
	return out
}

func eligibleWord(b bool) string {
	if b {
		return "eligible"
	}
	return "refused"
}

// ── the grant ─────────────────────────────────────────────────────────────────

// GrantState is where one authorization is in its very short life.
type GrantState string

const (
	// GrantIssued is authorised and unused.
	GrantIssued GrantState = "issued"
	// GrantConsumed has been claimed by an attempt. It can never be claimed again — and
	// the claim happens when the attempt BEGINS, not when it succeeds, so a crash midway
	// cannot leave a reusable authorization behind.
	GrantConsumed GrantState = "consumed"
	// GrantRevoked was cancelled, or its evidence moved.
	GrantRevoked GrantState = "revoked"
)

// grantSeq numbers attempts within this process.
//
// An atomic counter beside the runtime epoch rather than a timestamp. `time.Now().UnixNano()` is
// nanosecond-typed and not nanosecond-resolved — the Windows clock advances in ~15ms steps — so
// two grants issued back to back read the same instant. This repository has already paid for that
// lesson once, in runtimeEpochCounter.
var grantSeq atomic.Uint64

// RehearsalGrant is permission for ONE future controlled experiment.
//
// # What it is
//
// Authority data. It names exactly one candidate's evidence, one application, one starting
// screen, one destination and one attempt, with bounds. It is created only by answering a typed
// rehearsal question with yes.
//
// # What it is not
//
// It is not a procedure — the evidence stays in the ProcedureCandidate and the grant carries a
// digest of it rather than a copy, so authority and behaviour cannot drift apart. It is not a
// capability. It is not durable: there is no marshalling of it anywhere, and a new Director has
// no path that could restore one.
//
// And it cannot act. There is no Execute, Run, Replay, Perform or Invoke on this type, it lives
// in a package whose transitive imports reach no keyboard, mouse, controller or runtime, and the
// engine that will one day consume it has to reach across that boundary and ask.
type RehearsalGrant struct {
	// ID is unique within this process. Opaque; equality is the only meaningful operation.
	ID string
	// Application, Source and Destination are the scope. A grant for one application may not
	// be claimed by another, and a grant to go from A to B may not become a grant to go from
	// A to whatever appears.
	Application  string
	Source       string
	Destination  string
	Relationship RelationshipRef
	// Evidence is the rehearsal digest the user said yes to. If the candidate or the
	// assessment behind it moves, this no longer matches and the grant is refused.
	Evidence string
	// MaxInputs and MaxDuration bound the attempt. Non-zero by construction — see
	// NewRehearsalGrant, where a malformed bound refuses the grant rather than meaning
	// "unlimited".
	MaxInputs       int
	MaxUnobservable int
	MaxDuration     time.Duration
	// IssuedAt is for diagnostics and for the optional wall-clock guard. It is NOT the
	// primary expiry: restart, cancellation, evidence drift and consumption all come first.
	IssuedAt time.Time

	mu    sync.Mutex
	state GrantState
}

// DefaultRehearsalDuration is the wall-clock guard on a grant.
//
// A secondary bound. The primary expiries are structural — a new Director has no grant at all,
// cancellation revokes, evidence drift refuses, and consumption is single-use — and a time limit
// is here so authority cannot be held open indefinitely.
//
// # Why it is ten minutes and not thirty seconds
//
// It is only ever read by `BeginAttempt`, so what it actually bounds is how long after saying yes
// an attempt may BEGIN. Thirty seconds was right when a grant was spent the instant it was given.
// It stopped being right when the rehearsal learned to wait for its start: what the number governs
// now is how long Marco may wait for a PERSON to walk to the screen the route begins on, and
// nobody does that in half a minute. Live, a granted rehearsal waited correctly through four
// `source_mismatch` checks and then expired with the person still on their way.
//
// Ten minutes is the answer this system already gives to that exact question — `Bounds.Answer`,
// which bounds how long a teach session waits for somebody to answer, and is justified there as
// "a session left open overnight waiting for somebody to type a screen name is a session nobody
// meant to leave running". Waiting for a person to arrive somewhere is the same kind of wait.
//
// A stalled ATTEMPT is bounded by different things and always was: the input count, the
// unobservable-step count, and the per-step settle bound. None of them are this.
const DefaultRehearsalDuration = 10 * time.Minute

// NewRehearsalGrant issues authority for one attempt.
//
// The ONLY constructor, and it refuses rather than defaulting: a zero bound does not silently
// mean unlimited, an unscoped grant is not issued, and a judgement that was not eligible cannot
// produce one. `epoch` is the runtime's own identity, so a grant from a previous Director could
// never be mistaken for one from this.
func NewRehearsalGrant(epoch string, j RehearsalJudgement, now time.Time) (*RehearsalGrant, error) {
	switch {
	case !j.Eligible:
		return nil, fmt.Errorf("rehearsal: not eligible: %v", j.Refusals)
	case j.Application == "":
		return nil, fmt.Errorf("rehearsal: a grant must name an application")
	case j.Source == "":
		return nil, fmt.Errorf("rehearsal: a grant must name the screen it starts from")
	case j.Destination == "":
		return nil, fmt.Errorf("rehearsal: a grant must name the screen it expects to reach")
	case j.Inputs <= 0:
		return nil, fmt.Errorf("rehearsal: a grant must bound how many inputs it permits")
	case j.Digest == "":
		return nil, fmt.Errorf("rehearsal: a grant must bind to the evidence it was given for")
	}
	return &RehearsalGrant{
		ID:              fmt.Sprintf("attempt_%s_%d", epoch, grantSeq.Add(1)),
		Application:     j.Application,
		Source:          j.Source,
		Destination:     j.Destination,
		Relationship:    j.Relationship,
		Evidence:        j.Digest,
		MaxInputs:       j.Inputs,
		MaxUnobservable: j.LongestUnobservable,
		MaxDuration:     DefaultRehearsalDuration,
		IssuedAt:        now,
		state:           GrantIssued,
	}, nil
}

// State reports where the grant is. Safe to call from any goroutine.
func (g *RehearsalGrant) State() GrantState {
	if g == nil {
		return GrantRevoked
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state
}

// Active reports whether this grant could still authorise an attempt.
func (g *RehearsalGrant) Active() bool { return g.State() == GrantIssued }

// BeginAttempt claims the grant for one attempt, and refuses every later claim.
//
// # Why the claim happens at the START
//
// Because the alternative loses. If a grant were only spent on success, an attempt that crashed
// halfway — after input had been issued — would leave the authorization intact and reusable, and
// the second attempt would begin from a screen nobody checked. Spending it up front means a
// crash costs the user a question, which is the cheap failure.
//
// This milestone performs no attempt. The lifecycle exists so single use is proven before
// anything can consume it.
func (g *RehearsalGrant) BeginAttempt(application, source, evidence string, now time.Time) error {
	if g == nil {
		return fmt.Errorf("rehearsal: no authorization")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	switch g.state {
	case GrantConsumed:
		return fmt.Errorf("rehearsal: this authorization has already been used; " +
			"one grant permits one attempt")
	case GrantRevoked:
		return fmt.Errorf("rehearsal: this authorization was withdrawn")
	}
	// SCOPE, checked at the moment of claiming rather than trusted from when it was issued.
	if !strings.EqualFold(application, g.Application) {
		return fmt.Errorf("rehearsal: authorized for %s, not %s", g.Application, application)
	}
	if source != g.Source {
		return fmt.Errorf("rehearsal: authorized to start from a particular screen, " +
			"and this is not it")
	}
	if evidence != g.Evidence {
		return fmt.Errorf("rehearsal: the evidence has changed since this was authorized; " +
			"Marco must ask again")
	}
	// AND the wall clock, last of the scope checks. A permission given twenty minutes ago is
	// about a screen the user was looking at twenty minutes ago.
	if !now.IsZero() && now.Sub(g.IssuedAt) > g.MaxDuration {
		return fmt.Errorf("rehearsal: this authorization has expired")
	}
	g.state = GrantConsumed
	return nil
}

// Revoke withdraws the grant. Idempotent, and never resurrects a consumed one.
func (g *RehearsalGrant) Revoke() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state == GrantIssued {
		g.state = GrantRevoked
	}
}

// Describe renders the scope for a person, without the opaque id.
func (g *RehearsalGrant) Describe() []string {
	if g == nil {
		return nil
	}
	return []string{
		"authorization: " + string(g.State()),
		"scope: one attempt, in " + g.Application,
		fmt.Sprintf("at most %d input(s), at most %s", g.MaxInputs, g.MaxDuration),
		"this permits one controlled experiment. It performs nothing by itself",
	}
}

// ReviewRehearsal puts the rehearsal question, where the judgement allows one.
//
// # Bookkeeping only
//
// Everything about WHETHER comes from JudgeRehearsal, over the assessment. What this contributes
// is the same bookkeeping every other question gets: has it been asked, has it been answered, is
// the interruption budget free — and one rule of its own, that a grant already outstanding stops
// Marco asking for a second.
//
// It runs last of the three question kinds. Semantic understanding, then learning, then another
// example, then — only when everything else has had its chance at the budget — permission to try.
func (l *ProposalLedger) ReviewRehearsal(j RehearsalJudgement, top Topology,
	granted bool, th ProposalThresholds) RehearsalJudgement {

	if th.MaxOpen == 0 && th.MaxProposals == 0 {
		th = DefaultProposalThresholds()
	}
	// 'granted' means THIS ROUTE is already authorised, which is the caller's to determine —
	// a grant names its route, and reading it as a blanket "some authority exists" let one
	// outstanding permission veto the question about every other route. See
	// Runner.reviewRehearsal.
	if granted {
		j.Eligible = false
		j.Refusals = append(j.Refusals, RefusalAlreadyAuthorized)
		return j
	}
	id := RehearsalProposalIdentity(j.Relationship.From, j.Relationship.To)
	if existing := l.find(id); existing != nil {
		// A declined question comes back only when the JUDGEMENT changes shape — by the
		// same materiality rule every other question uses, over this one's own digest.
		if existing.Status == ProposalDeclined && existing.Evidence != j.Digest {
			existing.Evidence = j.Digest
			existing.Status = ProposalOpen
			existing.Response = ResponseNone
			existing.Asked++
			return j
		}
		j.Eligible = false
		switch existing.Response {
		case ResponseContradicted:
			j.Refusals = append(j.Refusals, RefusalRefused)
		case ResponseDeclined:
			j.Refusals = append(j.Refusals, RefusalDeclined)
		default:
			j.Refusals = append(j.Refusals, RefusalAsked)
		}
		return j
	}
	if !j.Eligible {
		return j
	}
	if l.openCount() >= th.MaxOpen || len(l.Proposals) >= th.MaxProposals {
		j.Eligible = false
		j.Refusals = append(j.Refusals, RefusalQuestionOpen)
		return j
	}
	l.Proposals = append(l.Proposals, Proposal{
		ID: id, Ask: AskRehearse, Question: RehearsalQuestion(j, top),
		Relationship: &RelationshipRef{From: j.Relationship.From, To: j.Relationship.To},
		Evidence:     j.Digest, Status: ProposalOpen, Asked: 1,
		Support: []EvidenceSource{FromNavigation, FromStructure, FromUser},
	})
	return j
}

// ── what a completed rehearsal leaves behind ──────────────────────────────────

// RehearsalEvidence is the durable, bounded record that one route was rehearsed and survived.
//
// # Why anything is stored at all
//
// Because "verified" has to mean something after a restart. The milestone that lowers a verified
// procedure into readable Marco cannot ask a process that has exited whether the route once
// worked, and asking the user to rehearse again on every launch is not a design.
//
// # What is deliberately NOT stored
//
// Nothing executable. There is no step-by-step program here, no key, no coordinate, no host call —
// only WHICH candidate, WHERE it ran, WHICH meanings were sent, and how each step came out.
// Reproducing any of it means lowering the candidate again through the boundary, under a fresh
// authorization, exactly as the first time. A stored procedure graph would be a capability that
// nobody granted.
//
// # And it does not itself mean verified
//
// It is EVIDENCE. Whether a candidate counts as verified is recomputed from this plus what memory
// currently holds — see [[ADR-026-verification-is-derived-from-a-completed-rehearsal]] and the
// same discipline in [[ADR-021-a-judgement-is-recomputed-not-recorded]].
type RehearsalEvidence struct {
	Application  string          `json:"application"`
	Relationship RelationshipRef `json:"relationship"`
	// Sequence is which demonstration of the route was rehearsed.
	Sequence int `json:"sequence"`
	// Evidence is the rehearsal digest the attempt was authorized against.
	//
	// The field that stops a stored result vouching for a demonstration that has since been
	// revised: a candidate whose digest no longer matches this is simply not the thing that
	// was rehearsed.
	Evidence string `json:"evidence"`
	// Source and Destination are the endpoints the attempt actually ran between.
	Source      string `json:"source"`
	Destination string `json:"destination"`
	// Completed says the whole route ran and ended, directly verified, where it was meant to.
	Completed bool `json:"completed"`
	// Steps and Inputs are what it cost.
	Steps  int `json:"steps"`
	Inputs int `json:"inputs"`
	// Verifications is the per-step outcome, in order, from the closed vocabulary.
	Verifications []string `json:"verifications,omitempty"`
	// At is when it ran.
	At time.Time `json:"at"`
}

// RehearsalStore is durable storage for what a rehearsal proved.
//
// A separate interface from Memory for the same reason CandidateStore is: the analysis core reads
// and writes different things at different moments, and a package that needed only to ask "was
// this rehearsed" should not thereby gain the ability to write a subject.
type RehearsalStore interface {
	// RememberRehearsal keeps one attempt's bounded evidence.
	RememberRehearsal(application string, e RehearsalEvidence) error
	// Rehearsals returns what is stored for one application.
	Rehearsals(application string) []RehearsalEvidence
}

// VerifiedBy reports whether this evidence still supports calling a candidate verified.
//
// DERIVED, and re-derived on every read. Three things must hold, and each of them can stop being
// true without anybody editing the record:
//
//	the route completed when it ran;
//	it was the SAME demonstration — the digest still matches;
//	memory still recognises both endpoints, so the claim is still about screens Marco knows.
//
// The last one is why this is a function rather than a flag. A subject that has been contradicted
// since the rehearsal makes the old success a fact about a screen nobody can find, and a stored
// boolean would go on saying yes.
func (e RehearsalEvidence) VerifiedBy(c ProcedureCandidate, digest string, top Topology) bool {
	if !e.Completed || e.Evidence == "" || digest == "" || e.Evidence != digest {
		return false
	}
	if e.Relationship != c.Relationship || !strings.EqualFold(e.Application, c.Application) {
		return false
	}
	return subjectKnown(top, e.Source) && subjectKnown(top, e.Destination)
}
