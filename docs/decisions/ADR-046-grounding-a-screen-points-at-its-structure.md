---
type: decision
status: accepted
date: 2026-08-12
supersedes: []
affects:
  - visibility
  - passive-observation
  - demonstrations
source_paths:
  - internal/director/observe/placereferent.go
  - internal/director/observe/referent.go
  - internal/director/learn/learn.go
  - cmd/director/learngrounding.go
---

# ADR-046 — grounding a screen points at its structure, not at the screen

Teaching establishes a START and, after the discovery pass, a DESTINATION. Both are the most
consequential decisions a teach session makes before anybody is asked to demonstrate anything, and
both were invisible: Marco said "okay, go ahead and show me" and a person had no way to find out
that it had settled on the wrong screen. A wrong start surfaces after a demonstration; a wrong
destination after two demonstrations and a rehearsal.

## The tension

`ReferentForSubject` refuses a screen-shaped subject with `ReferentNotAPart`, and that rule is
right — outlining the window says nothing a person did not already know and implies a precision
that is not there. But START and DESTINATION *are* screens. Taken literally, grounding a teach
session was impossible by construction.

## The decision

**A place is grounded by the structure it is recognised by, and never by its window.** The largest
co-occurring group belonging to that screen state — the same `dominantGroup` notion the hypothesis
layer already uses to say what a screen is dominated by — is resolved through `ReferentForSubject`
and drawn one region per member.

Three things fall out of that and are load-bearing:

- **The state is pinned by the caller.** `Session.StartState` and `Session.DestinationState` are
  recorded at the moment of the decision and never recomputed. Grounding that re-read the current
  state would confirm whatever the user happened to be looking at — a highlight that agrees with
  you is worse than no highlight, because it looks like understanding.
- **The wording carries the smaller claim.** "8 controls I recognise this screen by", never "the
  start is these controls". The start is the screen; the highlight is the evidence.
- **Every existing rule applies unchanged.** Absent members are omitted, whole-window members are
  not parts, members outside their own window cannot be placed, enclosures are dropped. This adds
  no geometry path and no second conversion — it selects a subject and delegates.

## This does not reopen ADR-041

[[ADR-041-a-screen-is-not-its-dominant-group]] removed the dominant group's member count from a
state's **durable identity**, because membership depends on how much of the session has happened —
24 members alone, 12 beside a neighbour — and identity may not depend on that.

That property is still true here, and it is still a reason not to store this. A grounded referent
is transient, valid for the inference that produced it, compared against nothing and written
nowhere. Two sessions may ground the same screen on differently-sized groups and both highlights
are correct about the moment they were drawn. What ADR-041 forbids is treating that number as
identity, which nothing on this path does.

## Grounding cannot change a teach session

`Coordinator.showing` returns a value and has no path to the phase, the refusal, the route or the
question. Every caller assigns its result to one field and reads it no further, and both call sites
run *after* the phase has already moved. So a grounding that fails cannot establish an identity,
change an assessment, advance Teach or grant authority — not because those are checked afterwards,
but because there is nothing there that could reach them. A Director with no grounding wired
teaches exactly the same lesson and simply cannot show you.

## The frame travels with the measurement

Regions are normalised against the window rectangle they were measured against, and `pkg/referent`
refuses to convert them when the window has moved since. That check is only worth anything if the
frame is recorded at grounding time: reading the newest sampled frame at display time would compare
the window with itself and satisfy the freshness rule by construction. `VisualReferent` carries no
desktop rectangle by design, so the frame is kept beside it in live process state, in
`cmd/director`, and is lost on restart — as a teach session is.

## Enforced by

`internal/director/observe/placereferent_test.go`:

- `TestGroundingAPlaceUsesThePinnedScreenAndNotWhateverIsCurrent` — the pinning rule.
- `TestStructureParkedUnderTheUnknownScreenIsNeverGrounded` — an unsettled screen is not a screen.
- `TestGroundingAPlaceStillRefusesTheWholeWindow` — the container rule survives delegation.
- `TestAGroundedScreenSaysItIsWhatTheScreenIsRecognisedBy` — the smaller claim.

`internal/director/learn/grounding_test.go`:

- `TestGroundingFailureDoesNotChangeAnythingAboutTheLearnSession` — every unavailable reason, and
  the successful case, produce an identical session.

`internal/director/observe/referentdiagnosis_test.go` (added 2026-08-12):

- `TestAnUnreliableFrameIsDistinguishableFromUnplaceableMembers` and
  `TestAGroupWhoseMembersAreOutsideTheWindowSaysSo` — `coordinate_mapping_unreliable` is returned
  from two places that mean opposite things, and `ReferentDiagnosis` tells them apart. One is a
  wiring defect and the other is a provider limitation; a live Explorer refusal could not be
  attributed without it. See [[Experiment-012-why-explorer-cannot-be-pointed-at]].
- `TestTheMemberFunnelAccountsForEveryMember` — the counts reconcile at every step, so the step
  that lost the regions is the one where they stop matching.

`internal/director/learn/grounddiagnosis_test.go`:

- `TestTheWatchPanelDistinguishesTheTwoUnreliableCauses` — the Watch panel is the surface that has
  to separate them; deleting the line makes both causes print identically.
- `TestTheGroundingDiagnosisNeverReachesNormalMode` — the product sentence stays coarse.
- `TestAGroundingRefusalDoesNotDisturbTheEstablishedPlace` — grounding is presentation. It cannot
  revoke a place, refuse a session or change a phase.
- `TestEachEndpointIsGroundedOnTheScreenItWasEstablishedOn` — START and DESTINATION are pinned to
  different screens and grounded on their own.

`cmd/director/learngroundingwiring_test.go`:

- `TestLearningInstallsGroundingThroughTheProductionStart` — the coordinator tests all inject their
  own grounding, so this is the only place that notices when production stops installing one.
- `TestAGroundedLearnViewCarriesDesktopRectangles` — the conversion actually happens.
- `TestAWindowThatMovedSinceGroundingIsNotDrawnAtTheOldPlace` — the freshness refusal.

## Related

[[ADR-041-a-screen-is-not-its-dominant-group]] · [[ADR-008-no-stale-window-geometry]] ·
[[ADR-043-teaching-is-two-passes-not-a-new-capture]] · [[Visibility]] · [[Demonstrations]]
