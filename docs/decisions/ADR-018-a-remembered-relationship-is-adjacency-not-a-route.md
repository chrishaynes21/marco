---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - semantic-memory
  - hypotheses
  - navigation
  - passive-observation
---

# A remembered relationship is adjacency, not a route

## Context

Marco can now recognise a subject across sessions, say what it is about in generic terms, and
observe privacy-bounded navigation intents around the moment a screen changes. What it could not
do was remember that two recognised subjects are **connected**.

Screen transitions already existed and already carried everything needed —
`Preceded`, `Unattributed`, `ConditionalOnly`, ordered `Sequences` — keyed on `state_3 → state_8`.
Those references are counters minted by one session's segmenter. The next run renumbers
everything, so no session-local edge can corroborate what a previous session saw.

The Action Graph was considered and rejected as the home for this. Its edges serve executable
semantics; an observation that two screens are adjacent is a different claim, and putting it in
a structure whose other edges can be executed is the shortest available path to executing it.

## Decision

**A durable relationship records that subject A was observed becoming subject B, and what
navigation was seen around it. It records nothing about cause and nothing about how.**

The durable object is an `observed_transition`. It is deliberately not called a procedure, a
route, an action or a binding: each of those implies something executable, and a type named for
what it is not yet is a type somebody will eventually execute.

### Identity is the endpoints, and direction is part of it

`From` and `To` are `RememberedSubject` ids, resolved through `Memory.Recall` — which is
`CompareStructure`, the identity layer that already exists. The relationship layer performs **no
matching of its own**; a second matcher with its own tolerances would be a second answer to "is
this the same screen", and the two would diverge.

A→B and B→A are different records. A menu opened by `confirm` and closed by `back` would
otherwise collapse into one edge that appears to answer to both.

### Both ends, or nothing

An edge is written only when **both** endpoints reach `same`. `candidate`, `insufficient` and
`different` are all refusals, and the transition stays session-local evidence — reported, so
"nothing transitioned" is never confused with "nothing was recognised".

The bar is the one [[ADR-016-cross-session-identity-is-structural-and-conservative]] sets: a
wrong durable edge is a claim about a screen Marco cannot actually recognise, and it would
survive every session that could have contradicted it. A missed edge costs one session.

Two session-local states that resolve to ONE subject do **not** produce a self-loop. What
happened there is that Marco cannot tell them apart, which is not an observation about the
screen.

### Every category of evidence survives, unflattened

| kept | why it cannot be dropped |
|---|---|
| `Preceded` per intent | competing intents stay visible; `confirm 3, down 2` never becomes "confirm" |
| `Unattributed` | the control evidence. 10 observations with `confirm` before 3 is mostly a change that happens on its own |
| `ConditionalOnly` | [[ADR-013-navigation-is-meaning-not-keys]] does not stop at the durable boundary. Context-admitted keys stay weaker, and an edge resting entirely on them says so where a person reads it |
| `Sequences` | order cannot be reconstructed later. `down, down, confirm` and `confirm, down, down` have identical `Preceded` maps and describe two different interactions |

An ordered run is an **observed navigation sequence**, never "the steps to reach B". The field is
plural and counted on purpose: three different runs before the same change is evidence that
there is no one way.

`Sessions` is held apart from `Observations`, discretely. Twenty observations in one sitting and
the same edge on five separate days are different kinds of evidence, and only the second has
survived a restart.

### Branching is topology, not contradiction

A subject may lead to several others. Nothing collapses them, ranks them, or treats the second
as evidence against the first — a settings screen that leads to audio and to controls is the
ordinary case. Contradiction means conflict with a *claim*, not more than one destination.

### Current evidence first, memory second

```
screen transition (this session)
  → current semantic subjects
    → semantic-memory resolution
      → navigation attribution
        → relationship observation
          → memory update
```

The arrow points one way. A remembered edge does not put a transition into a session that did
not see one, and `TestMemoryDoesNotManufactureATransition` holds it.

### One write, at session end

The choke point is `Runner.Run`, after the hypotheses that name the endpoints, once per session.
Three reasons and each would be enough on its own: the transition tally **grows** while a session
runs, so a per-sample write would write an incomplete count and then rewrite it forever;
`Sessions` counts independent corroborations and only a finished session is one; and the store
writes its whole file atomically, so a batch of edges is one write where n edges would be n.

### Storage sits beside the subjects

`relationships` in the same file as `subjects`, under the same lifecycle: atomic temp+rename,
`0o600`, versioned, corruption reported rather than swallowed. Two files could disagree about
whether an endpoint exists.

**Referential integrity is enforced twice.** At the write, an edge to a subject the store does
not hold — or across an application namespace — is refused and counted. At the load, an edge
whose endpoint is gone is dropped and counted. That second one also defines what happens when a
subject is removed: its edges go with it, deterministically, with no migration.

### Bounds

`MaxRelationships` 2048; per edge, `MaxDurableSequences` 6 and `MaxDurableIntents` matching the
session-local caps, with `MaxSequenceLength` 8. Counts may grow forever; structure plateaus.
Dropped run variants are **counted**, because a capped set that says nothing reads as a complete
one.

### Privacy

A relationship may contain subject ids, closed `NavIntent` values, counts, and bounded ordered
runs of closed intents. `down → confirm` is representable; `S, S, Enter` is not, and not by
care at the call sites — `Fold` checks `NavIntent.Known()` on admission, so a raw key has
nowhere to go.

## Consequences

- Marco can now say: *I recognise this subject, I recognise that one, and I have seen the first
  become the second before — this navigation was around it, this often, across this many
  sessions.*
- It still cannot say how to perform the transition, and nothing here can be executed. That
  distinction is the entire point of the milestone.
- An edge needs both endpoints to be subjects a person has settled, so early sessions produce
  session-local evidence and no topology. That is reported rather than silent.
- A defect was found and fixed on the way: **two hypotheses about one screen produced two
  different fingerprints** — `possible_menu_like_state` set `Members`, `possible_reversible_place`
  and `possible_text_entry_state` did not — so `Remember` stored one screen as two subjects.
  Identity is now `stateFingerprint`, one function. See the note on it.

## Enforced by

- `TestARelationshipIsCorroboratedByALaterSession` — THE production test: two runners over one
  store file, session-local ids deliberately renumbered, then a third session that must
  corroborate rather than duplicate. Deleting the write call, building identity from ephemeral
  state ids, or duplicating per session each fail it.
- `TestDirectionIsPartOfRelationshipIdentity` — and that the two directions carry different
  navigation.
- `TestBranchingTopologyIsPreservedNotCollapsed`.
- `TestOrderedNavigationRunsAreRememberedAsObservations`.
- `TestUnattributedObservationsSurviveIntoTheDurableRecord`,
  `TestConditionalNavigationStaysWeakerWhenItBecomesDurable`,
  `TestEveryEvidenceCategoryCrossesTheDurableBoundary`.
- `TestAnUnestablishedEndpointDoesNotMakeADurableEdge`,
  `TestTwoStatesResolvingToOneSubjectDoNotBecomeASelfLoop`,
  `TestSimilarEndpointsAreSeparatedByTheIdentityLayerNotByThisOne`,
  `TestAStateWithNoHypothesisIsNotAnEndpoint`,
  `TestATransitionWithAnUnrecognisedEndpointStaysSessionLocal`.
- `TestMemoryDoesNotManufactureATransition`.
- `TestARelationshipDoesNotOutliveItsEndpoints`,
  `TestARelationshipWithAnUnknownEndpointIsRefused`,
  `TestARelationshipDoesNotCrossTheApplicationBoundary`.
- `TestNothingCapturedCanReachDurableMemory` (extended to the relationship type graph),
  `TestTheStoredTopologyContainsNothingCaptured`,
  `TestARawKeyIdentityIsRefusedByAdmissionNotByCapacity`.
- `TestTheDurableTopologyPlateaus`, `TestFoldingARelationshipIsBoundedAndSaysWhatItDropped`.
- `TestATracedSessionReplaysToTheSameRelationships` — replay parity from the safe representation.
- `TestARelationshipExplainsItselfWithoutClaimingCause` — the explanation names each endpoint's
  own provenance and contains no instruction.
