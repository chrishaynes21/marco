// Package playbill is the ONE read-only account of what the Director currently
// believes and is doing, in a form a presentation is allowed to render.
//
// # Why it is called a playbill
//
// Marco is a play. A playbill is what the audience is handed: it says which play is on,
// who is on stage and how far in we are. It cannot change the performance, it names no
// backstage machinery, and it is written for somebody who has never read the script.
// Every one of those is a property this type has to have, so it may as well be the name.
//
// # One representation, three readings
//
// NORMAL, WATCH and DIAGNOSTICS are three renderings of THIS value and nothing else.
// They are produced by Normal, Watch and Deep below, so a presentation that wants to
// look different chooses colours and layout, never facts. A second path from the
// Director to a screen is how two surfaces come to disagree about what is happening,
// and the disagreement is always discovered in front of a user.
//
// # It knows only what the Director already decided
//
// Nothing here derives, scores, thresholds or interprets. The Director publishes this;
// a presentation renders it. In particular this package holds NO analysis: it has no
// hypothesis generator, no recogniser and no policy, and it imports nothing that has
// one. See guard.go for the boundary it enforces on itself.
//
// # It carries no authority
//
// There is no field on this type that anything can act on, no id that can be replayed,
// and no method that performs. A pending question carries the id needed to ANSWER it
// through the ordinary response path, and that is the whole of the interaction surface.
// Seeing "ready to rehearse" on a screen is not a rehearsal grant, and the only thing
// that issues one is a person answering the typed question.
package playbill

import "time"

// Version is the shape this build publishes.
//
// Bumped when a field's MEANING changes rather than when one is added, because a
// presentation that renders an unknown field skips it and a presentation that renders a
// changed one lies.
const Version = 1

// ── availability ──────────────────────────────────────────────────────────────

// Reach is whether there is anything to show at all.
//
// Three values and not a bool, because "the engine could not be run" and "the Director
// is not started" send a person to two different places, and a surface that merged them
// would tell somebody to start a service that is already running.
type Reach string

const (
	// Unreachable: the engine itself could not be run or did not answer.
	Unreachable Reach = "unreachable"
	// Absent: the engine answered and the Director service is not running.
	Absent Reach = "absent"
	// Present: the Director answered.
	Present Reach = "present"
)

// Live reports whether this playbill describes a Director that is actually there.
func (r Reach) Live() bool { return r == Present }

// ── uncertainty, preserved ────────────────────────────────────────────────────

// Recognition is how sure the Director is about WHICH screen this is.
//
// A closed vocabulary, and deliberately NOT a percentage. The Director's own recall
// produces four discrete verdicts precisely because two remembered screens at 0.71 and
// 0.69 are not "the first one" — they are a case where it cannot tell. Rendering that
// as "71% confident" would invent a precision no part of the system claims and would
// invite a reader to pick the higher number.
type Recognition string

const (
	// Unobservable: no usable evidence is reaching the Director at all.
	Unobservable Recognition = "unobservable"
	// Unknown: evidence is arriving and nothing remembered matches it.
	Unknown Recognition = "unknown"
	// Candidate: something remembered fits structurally, with nothing distinctive to
	// confirm it. Plenty of screens have five buttons.
	Candidate Recognition = "candidate"
	// Ambiguous: several remembered screens fit equally well, so the answer is "cannot
	// tell" rather than "the closest one".
	Ambiguous Recognition = "ambiguous"
	// Contested: what is on screen positively disagrees with what was remembered.
	Contested Recognition = "contested"
	// Recognised: established — the structure agrees AND a real discriminator agrees.
	Recognised Recognition = "recognised"
)

// Standing is where one interpretation stands.
//
// The Director's own hypothesis vocabulary, carried across unchanged rather than
// collapsed into "confident / not confident". `Recalled` is separate from `Confirmed`
// on purpose: one is what the user said, the other is what an earlier session recorded.
type Standing string

const (
	Tentative Standing = "tentative"
	Supported Standing = "supported"
	Disputed  Standing = "contested"
	Confirmed Standing = "confirmed"
	Recalled  Standing = "recalled"
	Withdrawn Standing = "withdrawn"
)

// ── the lifecycle, as far as it is observable ─────────────────────────────────

// Stage is where LEARNING has got to.
//
// Only stages the Director can actually be observed to be in. There is no "thinking"
// or "understanding" here, because nothing in the Director publishes either and a
// surface that showed them would be animating a guess.
type Stage string

const (
	// NotLearning: no passive observation session is running.
	NotLearning Stage = "idle"
	// Observing: watching, with nothing yet worth saying.
	Observing Stage = "observing"
	// AwaitingEvidence: it has something, and the policy has not earned a question.
	AwaitingEvidence Stage = "waiting_for_evidence"
	// Asking: a question is open and unanswered.
	Asking Stage = "asking"
	// Capturing: watching a demonstration the user agreed to give.
	Capturing Stage = "capturing"
	// Comparing: a demonstration finished and is being judged against what is
	// remembered — including against an earlier example of the same route.
	Comparing Stage = "comparing"
	// RehearsalOffered: a route has earned the right to be TRIED, and Marco is waiting
	// for a person to say yes. Displaying this grants nothing.
	RehearsalOffered Stage = "ready_to_rehearse"
	// Rehearsing: an authorized attempt is in flight.
	Rehearsing Stage = "rehearsing"
	// Rehearsed: an attempt completed and its outcome is known.
	Rehearsed Stage = "rehearsed"
	// PlayAvailable: a completed rehearsal still supports writing this down as Marco.
	PlayAvailable Stage = "play_available"

	// The stages an EXPLICIT learn session can be in that ordinary passive
	// observation never reaches. A person who asked Marco to learn something is owed a
	// different account from somebody being watched: they are waiting on a cue.

	// Establishing: learning where the demonstration starts, before saying "go ahead".
	Establishing Stage = "establishing"
	// ShowMe: the bounded demonstration window is OPEN and Marco is waiting to be
	// shown. The one stage a person must never have to guess at.
	ShowMe Stage = "show_me"
	// Naming: a play is blocked on the user's own word for a screen.
	Naming Stage = "naming"
	// Saved: a durable play exists on disk. Reached only when one does.
	Saved Stage = "saved"
)

// StepState is where one step of a learn flow has got to.
//
// Four values because a checklist that cannot say "skipped" forces every session through
// a wizard it may legitimately not need, and a UI that hard-coded the sequence would
// disagree with the coordinator the first time one did.
type StepState string

const (
	StepPending StepState = "pending"
	StepCurrent StepState = "current"
	StepDone    StepState = "done"
	StepSkipped StepState = "skipped"
)

// Step is one stage of the learn checklist, named in ordinary words.
type Step struct {
	Name  string    `json:"name"`
	State StepState `json:"state"`
}

// MaxLearnSteps bounds the checklist.
const MaxLearnSteps = 8

// MaxDidIntents bounds how many attributed actions travel at once.
//
// Eight. The question a person is asking of this field is "did Marco see what I just
// did", and the answer is the last handful — not a transcript. A longer list would be a
// record of somebody's keyboard by another route.
const MaxDidIntents = 8

// LearnSession is an EXPLICIT learn session: what the person asked for, and how far it got.
//
// # Why this is not folded into Learning
//
// Because they have different owners, and an account whose sections disagree is worse
// than one that is missing a section. `Learning` is derived from the observation session
// — what a passive watcher can tell. This is derived from the Learn coordinator, which
// is the only thing that knows a person asked for something, what they called it, which
// cue they are waiting for, and whether a file was written.
//
// # What is NOT here
//
// No subject id, no fingerprint, no similarity, no proposal id, no capture internals.
// `Did` is the closed navigation vocabulary — meanings, never keys — which is the same
// boundary a durable relationship's evidence already lives inside.
type LearnSession struct {
	// Active says a learn session is running right now.
	Active bool `json:"active,omitempty"`
	// Asked is what the person called the behaviour. Their words, held and not
	// interpreted — Marco still has to discover what actually happened.
	Asked string `json:"asked,omitempty"`
	// Progress is the ordered checklist. Derived from the coordinator's own phase, so a
	// session that legitimately skips a step shows it skipped rather than pending.
	Progress []Step `json:"progress,omitempty"`
	// Waiting says Marco has asked something and can do nothing until a person answers.
	//
	// Read from the coordinator, which owns the distinction. Without it every phase that
	// is not the cue reads identically, and "I am thinking" and "I am waiting for you"
	// are the two states a person most needs told apart — one of them is their turn.
	Waiting bool `json:"waiting,omitempty"`
	// Armed says the bounded demonstration window is OPEN.
	//
	// The single most important boolean on this surface. A person being watched
	// normally and a person mid-demonstration must never look the same.
	Armed bool `json:"armed,omitempty"`
	// Did is what Marco believes the person just did, in order, in the closed
	// navigation vocabulary.
	//
	// Empty is AMBIGUOUS on its own — nothing happened, or nothing could be
	// attributed — so `Unattributed` says which. That distinction is the whole reason
	// this field exists: "I saw the screen change and could not tell what you did" is
	// the most common honest failure in mouse-driven software, and it must never
	// render as a blank space.
	Did []string `json:"did,omitempty"`
	// Unattributed says a change was seen with nothing attributable before it.
	Unattributed bool `json:"unattributed,omitempty"`
	// Examples is how many demonstrations this attempt has captured.
	Examples int `json:"examples,omitempty"`
	// Because is one ordinary sentence for where it has got to, or why it stopped.
	Because string `json:"because,omitempty"`
	// Learned is the play that was written, empty until a file exists.
	//
	// Read from the ARTIFACT, never from a phase. A surface that inferred completion
	// from "the flow reached the end" would tell somebody Marco had learned a play it
	// had not written down.
	Learned string `json:"learned,omitempty"`
	// Registered says a later request can find it. Two facts, kept apart.
	Registered bool `json:"registered,omitempty"`
	// Stopped says the attempt ended without learning anything.
	Stopped bool `json:"stopped,omitempty"`
}

// Phase is where DOING has got to.
//
// Execution only. A phase is never entered by a presentation and never left by one.
type Phase string

const (
	// NotDoing: nothing is executing.
	NotDoing Phase = "idle"
	// AwaitingPermission: something is blocked on a person agreeing.
	AwaitingPermission Phase = "awaiting_permission"
	// CheckingStart: confirming the starting screen before acting.
	CheckingStart Phase = "checking_start"
	// Performing: input is being emitted.
	Performing Phase = "performing"
	// CheckingResult: looking to see where the action ended.
	CheckingResult Phase = "checking_result"
	Succeeded      Phase = "succeeded"
	// Unverified: it ran, and where it ended could not be established. Distinct from
	// failure, because the action may well have worked.
	Unverified Phase = "unverified"
	Failed     Phase = "failed"
	// Refused: Marco declined BEFORE acting. The most important phase on this list.
	Refused   Phase = "refused"
	Cancelled Phase = "cancelled"
)

// ── the sections ──────────────────────────────────────────────────────────────

// Current is what the Director thinks it is looking at.
type Current struct {
	// Watching says a passive observation session is running now.
	Watching bool `json:"watching"`
	// Application is the normalised executable key the Director already publishes for
	// provider status and world state. NOT a window title: a title is arbitrary
	// observed content and carries document names, chat and account names with it.
	Application string `json:"application,omitempty"`
	// Screen is what the USER called this screen, and is empty until they name one.
	//
	// The single free-text field in this whole record, and it is here because it is
	// user-supplied by construction: it reaches memory only by somebody typing it in
	// answer to a naming question, and it is validated at that boundary.
	Screen string `json:"screen,omitempty"`
	// Recognition is the discrete verdict. Never a score.
	Recognition Recognition `json:"recognition"`
	// Samples is how many observations the current session has taken, and FreshnessMS
	// how old the newest evidence is. Together they answer "is Marco still looking?",
	// which a still picture cannot.
	Samples     int   `json:"samples,omitempty"`
	FreshnessMS int64 `json:"freshness_ms,omitempty"`
	// Interrupted says the watched window went away mid-session, so the evidence either
	// side of it must not be read as one continuous view.
	Interrupted bool `json:"interrupted,omitempty"`
}

// Seeing is what useful evidence is actually reaching the Director.
//
// Counts and closed vocabulary. There is no field here that can hold what a control
// SAID: a region's label went through the Director's privacy classifier long before this
// type existed, and only the fact that it had one survives.
//
// Every field maps one-to-one onto something the observation session already counts. That
// is deliberate and it was not free — an earlier draft had a "named" count that quietly
// mixed a per-screen number with a whole-session one and rendered "204 of 5 things have a
// name", which is the shape of every observability bug worth having a rule about.
type Seeing struct {
	// Structure is how many recurring structures Marco trusts on THIS screen.
	Structure int `json:"structure"`
	// Looks is how many times it has looked at this screen, and Readable how many of
	// those looks had interface text it could classify. Readable of Looks is the honest
	// answer to "can Marco read this application", and it is usually low: scoped reading
	// is expensive and runs on roughly one look in six.
	Looks    int `json:"looks,omitempty"`
	Readable int `json:"readable,omitempty"`
	// Unrecognised is how many things it detected across the session whose KIND it could
	// not name. The number that says a detector is struggling.
	Unrecognised int `json:"unrecognised,omitempty"`
	// Terms are the CLOSED interface vocabulary classified on this screen — "settings",
	// "audio", "back". A fixed list Marco knows the words of, not text read off a screen:
	// a word that is not in the vocabulary never becomes one.
	Terms []string `json:"terms,omitempty"`
	// Sources are the structural kinds present, in the detector's own closed role
	// vocabulary — "button", "icon". Never a label.
	Sources []string `json:"sources,omitempty"`
	// Quiet says evidence is arriving and nothing in it is usable yet.
	Quiet bool `json:"quiet,omitempty"`
}

// Offers is what Marco could currently ACT ON, as it understands them.
//
// # Why this exists, when Seeing is right there
//
// Seeing answers "is Marco's eyesight working" in counts. It cannot answer the question a
// person actually asks of a perception surface — *what does it think is in front of me?* —
// because it has no field that can hold what a control is called. Watching a Learn attempt
// fail with "204 structures, 41 actionable" tells nobody whether Marco is looking at the
// right screen. `System · Bluetooth & devices · Network & internet` tells them instantly.
//
// # The names, and the licence they arrive under
//
// This is the one section of the playbill that carries observed interface text, and it
// introduces NO new permission. Every name here has already passed
// `observe.AdmittedTargetLabel` — the same single policy function
// [[ADR-058-a-demonstrated-target-may-keep-its-name]] introduced for semantic targets:
// the canonical plaintext role allowlist, widened to activatable controls only while an
// explicit Learn licence is in force, and the standing shape filter either way. A control
// whose name is withheld appears by ROLE with no name rather than vanishing, because "there
// are four things here I may not name" is itself worth seeing.
//
// EPHEMERAL. It describes the live screen and is recomputed per read; nothing here is
// written anywhere, and the durable store still has nowhere to put a label.
type Offers struct {
	// Actionable is how many controls Marco could currently aim at.
	Actionable int `json:"actionable,omitempty"`
	// Named is the admitted names, bounded. Fewer than Actionable whenever the label
	// gate withheld some, which is the ordinary case outside a Learn licence.
	Named []Offer `json:"named,omitempty"`
	// Focused is the control holding the keyboard's attention, empty when none does or
	// when its name is withheld.
	Focused Offer `json:"focused,omitzero"`
	// Withheld is how many actionable controls Marco can see and may not name. Stated
	// rather than omitted: a short list beside a big count is a privacy decision showing
	// its work, not perception failing.
	Withheld int `json:"withheld,omitempty"`
}

// Offer is one control Marco could act on.
type Offer struct {
	// Role is the control's kind, from the closed structural vocabulary.
	Role string `json:"role"`
	// Name is what it is called, empty when the label gate withheld it.
	Name string `json:"name,omitempty"`
}

// MaxOffers bounds the named controls one reading carries.
//
// Twenty-five. Enough that a navigation pane reads as itself, and far short of the hundreds
// a content-heavy page reports — a surface that printed every one would bury the answer it
// exists to give.
const MaxOffers = 25

// Reading is one active interpretation, with its uncertainty attached.
//
// Says, Because and But are sentences the DIRECTOR authored from counts and ratios.
// Nothing here is prose from a model and nothing here was read off a screen.
type Reading struct {
	// Says is the claim, hedged the way the Director hedges it.
	Says string `json:"says"`
	// Standing is where it stands. Always present: a reading without one is an
	// assertion.
	Standing Standing `json:"standing"`
	// Because is the plain evidence sentence, kept apart from the guess.
	Because string `json:"because,omitempty"`
	// But is every contradiction. Always carried when any exist — a reading shipped
	// with only its supporting evidence is advocacy.
	But []string `json:"but,omitempty"`
	// Settles is what a person could do to resolve it, when the Director knows.
	Settles string `json:"settles,omitempty"`
	// Seen is how many samples support it.
	Seen int `json:"seen,omitempty"`
}

// Thinking is the current interpretations and relationships.
type Thinking struct {
	Readings []Reading `json:"readings,omitempty"`
	// Total is how many existed before the bound, so a short list is never mistaken
	// for the whole picture.
	Total int `json:"total,omitempty"`
	// Retracted is how many were published and later withdrawn. Carried because a list
	// of live ones alone reads as though nothing was ever wrong.
	Retracted int `json:"retracted,omitempty"`
	// Links are the screen-to-screen relationships being observed, in ordinary words.
	Links []Link `json:"links,omitempty"`

	// SameSurface says the changes counted below all happened INSIDE one application
	// surface, rather than between unrelated worlds.
	//
	// Two statements instead of one identity carrying both. A settings page opening behind
	// an unchanged rail and tab strip is not a different application, and describing it as
	// one would be as wrong as not noticing it at all.
	SameSurface bool `json:"same_surface,omitempty"`

	// Changes is how many times the screen changed this session, and Caused how many of
	// those had something the person did observed before them.
	//
	// Both, always, and the pair is the point. A count of changes alone cannot distinguish
	// an application Marco is watching somebody drive from one that redraws itself, and
	// "three changes, none of them after anything you did" is the honest reading of the
	// second. Correlation, never cause: the number says what was SEEN before a change.
	Changes int `json:"changes,omitempty"`
	Caused  int `json:"caused,omitempty"`
}

// Link is one observed relationship between two screens.
type Link struct {
	// From and To are the screens' USER-GIVEN names where they have them, and an
	// ordinary phrase like "a screen you haven't named" where they do not.
	From string `json:"from"`
	To   string `json:"to"`
	// Times is how often it was observed and Sessions across how many sittings.
	Times    int `json:"times,omitempty"`
	Sessions int `json:"sessions,omitempty"`
	// Attributed is how many of those observations could be tied to something the user
	// deliberately did, as opposed to the screen simply having changed.
	Attributed int `json:"attributed,omitempty"`
	// Established says memory holds this as a durable relationship rather than as
	// evidence from this sitting alone.
	Established bool `json:"established,omitempty"`
}

// Learning is where the learning lifecycle has got to.
type Learning struct {
	Stage Stage `json:"stage"`
	// About is the route this stage concerns, in the user's words where they exist.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Examples is how many demonstrations of it exist.
	Examples int `json:"examples,omitempty"`
	// Captured and Checkpoints describe a demonstration in progress: how many things
	// the user did, and how many times Marco could tell where it had got to.
	Captured    int `json:"captured,omitempty"`
	Checkpoints int `json:"checkpoints,omitempty"`
	// Because explains the stage in one ordinary sentence, especially a waiting one.
	Because string `json:"because,omitempty"`
	// Silence is why Marco offered to learn NOTHING, in its own closed reasons.
	//
	// Carried because silence is the hard case. "Marco did not ask" has a dozen
	// explanations and a person watching cannot otherwise tell which — nor whether
	// the policy is working at all.
	Silence []string `json:"silence,omitempty"`
	// Remembered is how many screens this application has durable names for.
	Remembered int `json:"remembered,omitempty"`
}

// Doing is where execution has got to.
type Doing struct {
	Phase Phase `json:"phase"`
	// What is the request being performed, as the person said it.
	What string `json:"what,omitempty"`
	// Step and Steps are the position in a plan; both zero when there is no plan.
	Step  int `json:"step,omitempty"`
	Steps int `json:"steps,omitempty"`
	// Expected and Reached are screen names where they are known.
	Expected string `json:"expected,omitempty"`
	Reached  string `json:"reached,omitempty"`
	// Because is the Director's own sentence for a refusal, failure or non-result.
	Because string `json:"because,omitempty"`
	// RunningMS is how long it has been going, so a considered pause is
	// distinguishable from a hang.
	RunningMS int64 `json:"running_ms,omitempty"`
	// Live says real input is being emitted, as opposed to a dry attempt.
	Live bool `json:"live,omitempty"`
}

// Wants says what kind of answer a question is waiting for.
type Wants string

const (
	// WantsChoice takes one of Answers.
	WantsChoice Wants = "choice"
	// WantsName takes a word the user chooses. The one place free text is invited.
	WantsName Wants = "name"
)

// Via names the EXISTING response path an answer to this question travels.
//
// Carried because there is more than one, and they are not interchangeable: a
// confirmation unblocks a command that is still holding its bindings, a clarification
// re-runs a request with a refinement, and a proposal answer is evidence about a
// hypothesis that may be read minutes later. A presentation reads this to know which
// ordinary production request to make — it is NOT permission to invent a fourth.
type Via string

const (
	// ViaConfirm is the confirmation broker: the CONFIRM request.
	ViaConfirm Via = "confirm"
	// ViaClarify is the clarification path: the CLARIFY request.
	ViaClarify Via = "clarify"
	// ViaProposal is the observation ledger: an OBSERVATION request carrying an answer.
	ViaProposal Via = "proposal"
)

// Question is the ONE question currently waiting on a person.
//
// The Director asks at most one at a time on purpose: the cost of asking is not the
// cost of a dialog, it is the cost of breaking somebody's attention.
type Question struct {
	// ID binds an answer to this question and nothing else. Carried because the
	// ordinary response path REQUIRES it — an answer routed by "whatever is on screen
	// now" would land on the wrong thing, since by the time somebody answers the
	// screen has usually changed. It is never the primary thing shown.
	ID string `json:"id"`
	// Asks is the sentence a person reads.
	Asks string `json:"asks"`
	// Wants is what kind of answer, and Answers the closed vocabulary when it is a
	// choice. A presentation offers exactly these and nothing else.
	Wants   Wants    `json:"wants"`
	Answers []string `json:"answers,omitempty"`
	// Via is which existing response path the answer travels.
	Via Via `json:"via"`
	// About is the screen or route the question concerns, in ordinary words.
	About string `json:"about,omitempty"`
	// Waiting is how long it has been open.
	WaitingMS int64 `json:"waiting_ms,omitempty"`
}

// ── the timeline ──────────────────────────────────────────────────────────────

// Tone is how a line should read, so a presentation colours by MEANING rather than by
// matching on the text — which is what the first insight panel had to do, and which
// silently stops working the moment a sentence is reworded.
type Tone string

const (
	Plain  Tone = "plain"
	Good   Tone = "good"
	Doubt  Tone = "doubt"
	Alarm  Tone = "alarm"
	Muted  Tone = "muted"
	Accent Tone = "accent"
)

// Moment is one meaningful change, already in ordinary language.
//
// Derived from the Director's OWN bounded event log rather than from a second recorder
// this package would have to keep in agreement with it. Meaningful changes, not one
// entry per sample: the Director already decides what is material, and a timeline that
// restated an unchanged belief thirty times a minute would say nothing.
type Moment struct {
	// Seq is the Director's sequence number, so a presentation can resume without a
	// gap and without replaying.
	Seq uint64    `json:"seq"`
	At  time.Time `json:"at"`
	// Says is the line. No keys, no typed text, no screenshots, no arbitrary labels —
	// there is no field here that could hold one.
	Says string `json:"says"`
	Tone Tone   `json:"tone,omitempty"`
}

// MaxMoments bounds the timeline a playbill carries.
//
// Session-local and bounded by construction. The Director's own log is bounded above
// this, so the bound here is the one a presentation sees and never the only one.
const MaxMoments = 64

// ── diagnostics ───────────────────────────────────────────────────────────────

// Provider is one perception source's health, for DIAGNOSTICS only.
type Provider struct {
	Name string `json:"name"`
	// Available says it is present and usable; Why explains it when it is not.
	Available bool   `json:"available"`
	Why       string `json:"why,omitempty"`
	// Observations is how many pieces of evidence it contributed to the last cycle.
	// Zero from a healthy provider is the exact shape of the unpinned-accessibility
	// defect, and it is invisible everywhere else.
	Observations int `json:"observations"`
	// Quarantined is evidence refused because the provider could not prove it
	// described the window the Director had pinned.
	Quarantined int `json:"quarantined,omitempty"`
	// LatencyMS is the provider's own measured cost, when it reports one.
	LatencyMS int64 `json:"latency_ms,omitempty"`
	// Score is a PROVIDER-SPECIFIC numeric metric, and Metric names which one.
	//
	// Never converted into a Director confidence and never shown outside DIAGNOSTICS.
	// A detector's own 0..1 output is a fact about that detector; treating it as
	// belief about the world is the exact mistake the discrete vocabularies above
	// exist to prevent.
	Score  float64 `json:"score,omitempty"`
	Metric string  `json:"metric,omitempty"`
}

// Fusion is what the fusion engine made of one cycle.
type Fusion struct {
	Observations int      `json:"observations"`
	Elements     int      `json:"elements"`
	Merged       int      `json:"merged"`
	Rejected     int      `json:"rejected"`
	Degraded     []string `json:"degraded,omitempty"`
	// ProvenanceOK says every contributing source proved it described the same live
	// window. False is the single most important line in diagnostics.
	ProvenanceOK bool  `json:"provenance_ok"`
	CycleMS      int64 `json:"cycle_ms,omitempty"`
}

// Diagnostics is the developer-level evidence UNDER what Watch says.
//
// The same privacy model applies. Nothing becomes showable because a reader called
// themselves a developer, and there is no field here that carries a screenshot, a
// window title, a raw label or a provider payload.
type Diagnostics struct {
	// Notes is why a section of this report is empty, when it is empty for a reason.
	//
	// The deepest view is the one somebody opens when things are already wrong, so it
	// must be able to say "this half was never wired" rather than render as a Director
	// that has nothing to report — or, worse, fail.
	Notes []string `json:"notes,omitempty"`

	Providers []Provider `json:"providers,omitempty"`
	Fusion    Fusion     `json:"fusion,omitzero"`

	// SampleIntervalMS and LabelPasses are where the session's perception budget went.
	SampleIntervalMS int `json:"sample_interval_ms,omitempty"`
	LabelPasses      int `json:"label_passes,omitempty"`
	SamplesSkipped   int `json:"samples_skipped,omitempty"`
	SamplesLate      int `json:"samples_late,omitempty"`

	// Screens and Transitions are how many distinct compositions the session
	// segmented into and how many changes it saw between them.
	Screens     int `json:"screens,omitempty"`
	Transitions int `json:"transitions,omitempty"`
	// Attributed, Unattributed and ContextAdmitted split the navigation evidence:
	// tied to something the user did, not tied to anything, and admitted only because
	// the surrounding context made it plausible.
	Attributed      int `json:"attributed,omitempty"`
	Unattributed    int `json:"unattributed,omitempty"`
	ContextAdmitted int `json:"context_admitted,omitempty"`

	// Match is what the screen-identity comparison scored this session, as a bounded
	// summary: how many frames were read as the same screen and how weakly, how many became
	// another screen and how strongly.
	//
	// The number behind every screen verdict. "One screen, no transitions" is consistent
	// with an application that never changed AND with a comparison that cannot see the
	// change, and this is the only thing that tells them apart. `MatchOverlap` says the two
	// populations met, which means no threshold could separate them on this application.
	MatchJoined       int     `json:"match_joined,omitempty"`
	MatchJoinedMin    float64 `json:"match_joined_min,omitempty"`
	MatchJoinedMean   float64 `json:"match_joined_mean,omitempty"`
	MatchSeparated    int     `json:"match_separated,omitempty"`
	MatchSeparatedMax float64 `json:"match_separated_max,omitempty"`
	MatchThreshold    float64 `json:"match_threshold,omitempty"`
	MatchOverlap      bool    `json:"match_overlap,omitempty"`

	// Local is the SECOND comparison, and the two are read together or not at all.
	//
	// The whole-surface figures above answer "is this the same application"; these answer
	// "is this the same place inside it". A session showing many frames on one surface and
	// no local replacements is one where the second question was asked and always answered
	// no — which is what a person debugging "why is this all one screen" needs to see, and
	// is a different diagnosis from the question never having been asked at all.
	LocalSeen     int     `json:"local_seen,omitempty"`
	LocalMin      float64 `json:"local_min,omitempty"`
	LocalMean     float64 `json:"local_mean,omitempty"`
	LocalReplaced int     `json:"local_replaced,omitempty"`

	// StructureSource is which kind of evidence the screen model was built from — the
	// authoritative fused world, or the structural detector alone.
	//
	// A closed vocabulary and the first thing to read when there are no screens: the two
	// send somebody to two completely different places, and until the screen model could be
	// built from either there was nothing to report.
	StructureSource string `json:"structure_source,omitempty"`

	// Structure explains why screen recognition produced nothing, when it did.
	//
	// The single most confusing thing this surface can show: Watch saying "I can't tell
	// one screen from another" while the perception list reports six hundred
	// accessibility observations a cycle. Both are true — recognition runs on the
	// STRUCTURAL detector and accessibility is not it — and without this line a person
	// goes and checks the wrong provider.
	Structure string `json:"structure,omitempty"`

	// Candidates is how many remembered screens fit the current one — the number
	// behind an "ambiguous" verdict.
	Candidates int `json:"candidates,omitempty"`
	// Verdict is the recall engine's own word for the current match.
	Verdict string `json:"verdict,omitempty"`

	// Proposal is the open question's identity and status, which WATCH deliberately
	// does not show.
	Proposal       string `json:"proposal,omitempty"`
	ProposalStatus string `json:"proposal_status,omitempty"`
	Suppressed     int    `json:"suppressed,omitempty"`

	// Rehearsal is the last attempt's shape: how far it got and how it ended.
	RehearsalStep    int    `json:"rehearsal_step,omitempty"`
	RehearsalPlanned int    `json:"rehearsal_planned,omitempty"`
	RehearsalOutcome string `json:"rehearsal_outcome,omitempty"`
	// Authority is the outstanding grant's STATE, never the grant. A grant that could
	// be marshalled is a grant that could be replayed.
	Authority string `json:"authority,omitempty"`

	// Memory explains why cross-session recognition could not run, when it could not.
	// Reported so an absence of recognition is never mistaken for novelty.
	Memory string `json:"memory,omitempty"`

	// ComposeMS is how long assembling this playbill took, so the cost of watching is
	// itself watchable.
	ComposeMS int64 `json:"compose_ms,omitempty"`
}

// ── the playbill ──────────────────────────────────────────────────────────────

// View is the whole read-only account. There is exactly one of these.
type View struct {
	Version int   `json:"version"`
	Reach   Reach `json:"reach"`
	// Why is a short ordinary sentence for the most recent meaningful refusal, failure
	// or absence. It is what a person reads when something is not happening.
	Why string `json:"why,omitempty"`

	// Epoch identifies the Director instance. A change means it restarted and every
	// cursor is void — which is not the same as a quiet moment, and a surface that
	// confused them would show stale certainty across a restart.
	Epoch   string    `json:"epoch,omitempty"`
	TakenAt time.Time `json:"taken_at"`
	// UptimeMS is how long the Director has been up.
	UptimeMS int64 `json:"uptime_ms,omitempty"`

	Current      Current      `json:"current,omitzero"`
	Seeing       Seeing       `json:"seeing,omitzero"`
	Offers       Offers       `json:"offers,omitzero"`
	Thinking     Thinking     `json:"thinking,omitzero"`
	Learning     Learning     `json:"learning,omitzero"`
	LearnSession LearnSession `json:"learnSession,omitzero"`
	Doing        Doing        `json:"doing,omitzero"`
	Question     *Question    `json:"question,omitempty"`

	// Recent is the bounded timeline, oldest first.
	Recent []Moment `json:"recent,omitempty"`
	// Cursor is the newest sequence issued and Oldest the lowest still retained, so a
	// presentation can tell "nothing happened" from "I missed some" without doing the
	// arithmetic itself — two front-ends doing it differently is exactly the drift this
	// representation exists to prevent.
	Cursor uint64 `json:"cursor,omitempty"`
	Oldest uint64 `json:"oldest,omitempty"`

	// Digest is a stable fingerprint of everything a person would notice a change in.
	//
	// It deliberately excludes clocks, freshness and sample counts, so a presentation
	// can hold still while the Director samples away behind it. This is what stops an
	// unchanging screen from producing an event every poll.
	Digest string `json:"digest,omitempty"`

	// Diagnostics is present only when it was asked for. Watch never needs it, and
	// computing it on every poll would make the act of watching expensive.
	Diagnostics *Diagnostics `json:"diagnostics,omitempty"`
}

// Unavailable builds the playbill for "there is nothing to show, and here is why".
//
// A constructor rather than a zero value, because an empty View would render as a
// Director that is present and believes nothing — which is a real and very different
// state, and the one a person is most likely to misread.
func Unavailable(r Reach, why string) View {
	return View{
		Version: Version, Reach: r, Why: why, TakenAt: time.Now(),
		Current:  Current{Recognition: Unobservable},
		Learning: Learning{Stage: NotLearning},
		Doing:    Doing{Phase: NotDoing},
	}
}

// Normalise fills the inert member of each closed vocabulary.
//
// An unset verdict IS "unobservable", an unset stage IS "not learning", and an unset
// phase IS "not doing" — so the zero value of each section describes a Director that is
// simply not doing that thing, which is by far the commonest truth about it.
//
// Without this the admission guard would refuse a perfectly honest partial account and
// replace it with a refusal notice, which is failing closed in the one case where there
// was nothing to fail about. Reach is deliberately NOT normalised: an unset reach means
// nobody said whether the Director is there, and guessing "present" is exactly the stale
// certainty this representation exists to prevent.
func (v View) Normalise() View {
	if v.Version == 0 {
		v.Version = Version
	}
	if v.Current.Recognition == "" {
		v.Current.Recognition = Unobservable
	}
	if v.Learning.Stage == "" {
		v.Learning.Stage = NotLearning
	}
	if v.Doing.Phase == "" {
		v.Doing.Phase = NotDoing
	}
	if v.Question != nil && v.Question.Wants == "" {
		v.Question.Wants = WantsChoice
	}
	return v
}

// Bound trims the timeline to what a presentation may carry.
//
// Newest kept: a bounded log that dropped the newest would show a person the beginning
// of the thing they are trying to understand and not the end of it.
func (v View) Bound() View {
	if len(v.Recent) > MaxMoments {
		v.Recent = v.Recent[len(v.Recent)-MaxMoments:]
	}
	return v
}
