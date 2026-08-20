---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - passive-observation
  - semantic-memory
  - demonstrations
source_paths:
  - internal/director/observe/hypothesis.go
  - internal/director/observe/recall.go
  - internal/director/observesession/checkpointidentity_test.go
---

# ADR-041 — a screen is not its dominant group

A demonstration between two places inside one application could never begin. It reported
`start_unverifiable` — memory did not recognise the place the user was standing on — while those
same two places were two durable subjects that resolved by name and were told apart by a learned
play's start guard.

## It was never two matchers

Checkpoint capture, `Memory.Recall`, `Screen's Showing` and relationship endpoint resolution all
go through `SignatureOfState` → `Memory.Recall`. One derivation, one lookup, one tolerance. What
differed was **when** the question was asked.

## The defect

`stateFingerprint` borrowed `Members` from the state's dominant structural group. A group is made
of tracks persistent in **exactly one state**, so while a session has only ever seen one place,
the chrome that place shares with its neighbours is still persistent-in-one-state and counts
toward it; the moment a neighbour appears that structure becomes ambient and belongs to neither.

Measured on one surface — same roles, same terms, same application:

```
observed alone                 24 members
observed beside its neighbour  12 members        MemberTolerance = 1
```

So a stored 12 met a current 24 and memory answered `different` about a screen it held. Waiting
did not converge it: 1, 2, 4, 6 and 10 frames all reported 24. Only going somewhere **else**
changes the count — and going somewhere else is precisely what the demonstration was waiting to
record.

`Members` was not a property of the place. It was a property of how much of the session had
happened, which is the one thing durable identity may not depend on.

## The decision

**A state's fingerprint carries no member count.** Groups keep theirs, where it is intrinsic: a
group *is* its members. A screen is not its dominant group, and borrowing a sub-part's size as
the whole's identity was the error.

This is the smallest repair of the three considered. Replacing it with an intrinsic count was
unnecessary — role composition and terms already discriminate — and redesigning `dominantGroup`
would have changed the presence model and hypothesis machinery to preserve a field that should
not have existed.

### The original reason for `Members`, and why removing it is safe

`Members` became identity-bearing because four fingerprint constructors populated it
**inconsistently**: one screen described by `possible_menu_like_state` reported 4 and by
`possible_reversible_place` reported 0, they compared as different, and memory stored one screen
twice. The repair for that was **one constructor**, not the field. With no member count on a
state at all, two hypotheses about one screen cannot disagree about it —
`TestOneScreenIsOneRecordHoweverManyHypothesesDescribeIt` holds that directly.

## Serialised shape

Unchanged. `Members` remains on `StructureSignature` for group subjects, with `omitempty`, so a
state simply stops emitting it and old files still parse. Records written for a **state** before
this change carry a member count that current evidence will not match; there is no deployed user
memory, so no migration is written. Anything stale is re-learned.

## Enforced by

`internal/director/observesession/checkpointidentity_test.go`:

- `TestAPlaceIsRecognisedWhileItIsAllTheSessionHasSeen` — the regression, from the first frame.
- `TestObservationOrderDoesNotChangeDurableIdentity` — A-alone, A-then-B and B-then-A produce the
  same identity.
- `TestAStoredPlaceIsRecognisedColdFromDisk` — reopened memory, a session that visits one place.
- `TestOneScreenIsOneRecordHoweverManyHypothesesDescribeIt` — the original defect stays closed.
- `TestMemoryRecognisesBothPlacesOfOneSurface` — the control.

## Related

[[ADR-039-a-surface-and-a-place-inside-it]] · [[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[Semantic-Memory]] · [[Passive-Observation]]
