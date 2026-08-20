---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - semantic-memory
  - demonstrations
  - navigation
  - passive-observation
---

# "Watch me" is permission to observe, not to act

## Context

[[ADR-019-an-invitation-to-learn-is-not-a-correction]] let Marco ask *"do you want me to learn
how you do that?"* and record a `LearningPending` request. Nothing then watched.

The repository already has a demonstration recorder — `internal/recorder`, feeding
`internal/director/demo` — and reusing it was the obvious move and the wrong one. That recorder
captures raw keys and produces `demo.Procedure`, a type the executor runs. Both properties are
disqualifying here: a person answering yes to one question about one transition has not agreed to
have their keystrokes recorded, and the output of this milestone must be evidence rather than
something anything can run.

## Decision

**A demonstration is a bounded, authorised observation whose output cannot be executed.**

### What was reused, and what was refused

| reused | refused |
|---|---|
| the closed `NavIntent` vocabulary the navigation producer already emits | `internal/recorder`'s raw key capture |
| the per-sample observation path — same evidence everything else reads | `demo.Procedure` and its store |
| `Memory.Recall` for resolving where the user is | any second identity matcher |
| `EditableFields` as the text-entry boundary | reading, reconstructing or storing what was typed |

A yes widens the privacy boundary by **nothing**. What is recorded is exactly what passive
observation already records.

### Authorisation is structural

`NewCapture` is the only constructor and requires a `RelationshipRef`; `armCapture` is the only
caller and reads `PendingLearning` from durable memory. There is no path from a passive
observation to a candidate, and the property is a shape rather than a check.

One at a time. Two demonstrations watched at once would share one stream of navigation, and
there is no honest way to say which press belonged to which.

### Start and end are decided by current evidence

The request says the transition is A → B. That is a claim about *history*. Whether the user is
standing on A right now is a question only the current cycle can answer, and assuming it because
the request said so is the stale-ordinal mistake in another costume.

The end condition is the destination being **established from current evidence** — never a
timeout, never a keypress, never a count of inputs.

| situation | outcome |
|---|---|
| arrives at B | `complete`, reason `arrived` |
| ends at a different remembered subject C | `incomplete`, `destination_mismatch`. The request was A → B, and reinterpreting it would produce a candidate nobody authorised |
| several subjects fit equally | `incomplete`, `ambiguous_destination` |
| never reaches A | no start is recorded, and no candidate |
| returns to A without arriving | bounded restarts, then `bound_restarts` |
| cancelled | no partial candidate |
| session ends first | `incomplete`, `session_ended` |

### Bounds are numbers, and exceeding one is named

60 semantic events, 8 checkpoints, 8 intents per run, 90 observations, 2 restarts. Exceeding any
of them **stops** the capture with its own reason rather than truncating: a demonstration cut
short at an arbitrary point and presented as complete is a candidate that would teach the wrong
thing.

### Steps, not a flat list

`A —run→ X —run→ B`, with the intermediate screens preserved. A future validation has to be able
to check *progress* — did the screen actually become the next thing — rather than replay input
and hope, and a flat list has thrown that away.

An unrecognised intermediate screen is a **transient checkpoint**: it keeps this session's safe
signature and is explicitly not a remembered subject. Observing a screen while teaching does not
promote it into cross-session memory; promotion is what the semantic proposal loop is for, and it
involves a person.

Two subtleties the implementation had to get right, both instances of the same class of mistake:

- `Members` is excluded from checkpoint comparison. It is the size of the dominant structural
  group, which *matures* over the first few observations of a screen — comparing on it split one
  screen into a new checkpoint every time the group came into focus. Same reason `Recurrence` is
  excluded from durable identity.
- A transient checkpoint that is later **recognised** is upgraded in place, not appended. It is
  the same screen; Marco just learned what it was. Recording it as a second leg would insert an
  empty navigation run and make the destination look like it was reached without the user doing
  anything.

### Text entry is a boundary, not a recording

A step that crossed a screen with editable fields is marked `RequiresTextEntry`. What was typed
is not observed, not reconstructed and not stored. The candidate says only that it cannot be
understood without text Marco deliberately did not watch — handling that needs its own privileged
mode and its own consent.

### Persistence

A **completed** candidate survives, one per relationship, beside the topology, under the same
lifecycle (atomic, `0o600`, versioned, corruption visible, referential integrity at load). It is
meaningful evidence and later validation will need it.

An **active** capture is never persisted and so can never be resumed. A demonstration is a
bounded thing somebody agreed to give; carrying an unfinished one across a restart would be
watching without being asked again.

### No authority, structurally

`ProcedureCandidate` lives in `internal/director/observe`, whose boundary test already proves
nothing reachable from there can affect the machine. It has no `Execute`, `Run`, `Replay` or
`Compile`, is registered with no planner, resolver, action graph or capability registry, and
carries a `Verified` field that is always false and exists to be read.

## Consequences

- Marco can say: *I watched one example of how you moved from this screen to that one. I observed
  these navigation events and these intermediate screens. I have not verified that repeating them
  would work.*
- It cannot say it knows how, and cannot do it.
- A demonstration that goes wrong is reported with a closed reason rather than silently producing
  nothing, so a user who said yes and saw no result can find out why.

## Enforced by

- `TestAnApprovedDemonstrationIsCapturedEndToEnd` — THE production test: a persisted corroborated
  edge, a real learning question, a real yes, the runner arming itself, and a complete candidate
  with two ordered legs and the intermediate screen preserved.
- `TestWithoutAnApprovedRequestNothingIsCaptured`, `TestARefusedRequestArmsNothing`.
- `TestADemonstrationThatNeverStartsAtTheApprovedSubject`,
  `TestADemonstrationEndingElsewhereIsAMismatch`.
- `TestNavigationSurvivesASkippedInferenceSlot` — a press made during a skipped inference is
  still a press, and no checkpoint is fabricated.
- `TestAnUnknownIntermediateScreenIsTransientAndNotRemembered` — including that memory did not
  grow.
- `TestACancelledDemonstrationProducesNoCandidate`,
  `TestADemonstrationStopsAtItsBoundAndSaysWhich`.
- `TestAScreenThatTakesTypingIsMarkedNotRecorded`.
- `TestAProcedureCandidateHasNoWayToBeRun`, `TestNoRawInputCanReachADemonstration`,
  `TestTheStoredDemonstrationContainsNothingCaptured`.
- `TestACompletedCandidateSurvivesAndAnUnfinishedOneDoesNot`,
  `TestARunningCaptureCanBeDescribed`, `TestOnlyOneDemonstrationIsWatchedAtATime`.
