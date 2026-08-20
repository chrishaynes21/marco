---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - visibility
source_paths:
  - internal/director/learn/say.go
  - cmd/director/learngrounding.go
  - cmd/director/learncmd.go
  - plugins/overlay/cmd/marco-show/main.go
---

# ADR-059 — a presentation belongs to its claim

Grounding highlights were fire-and-forget processes whose only exit was their own hold
timer. There was no dismissal path anywhere: a settled teach session went on offering its
endpoints to every new reader, a bare `director teach` status read relaunched old
highlights at wherever the window had moved to, and two overlapping surfaces adopted each
other's window through a shared title. The 15-odd-second self-expiry was masquerading as a
lifecycle.

## The decision

A VisualReferent presentation is **ephemeral presentation of one decision**, owned by the
claim that raised it:

- The coordinator says which claims are CURRENT: each grounded endpoint carries `Current`,
  true only during the phase window its decision belongs to (`startLive`,
  `destinationLive`), and NOTHING is current once the session settles — a failed,
  cancelled or completed Learn owns no presentation.
- A surface draws only current claims, keeps the handle of what it drew, dismisses it the
  moment the claim stops being current, and closes everything it owns when it exits
  (`groundingShown`). The sentence is always said; the picture has an owner.
- Every `marco-show` instance names its window uniquely (per-pid title), so concurrent
  surfaces can no longer adopt each other.
- The surface's own hold timer remains as a backstop against an orphaned process — never
  as the lifecycle.

## Enforced by

- `TestEndedGroundingCannotLeaveHighlights`, `TestAGroundedEndpointIsCurrentOnlyDuringItsClaim`
  — the coordinator's account
- `TestFailedLearnDismissesOwnedPresentation`, `TestAStalePresentationIsDismissedWhenItsClaimEnds`,
  `TestAStatusReadAfterTheMomentDrawsNothing` — the surface policy
- `TestTheViewCarriesWhetherAnEndpointIsCurrent` — the one line carrying `Current` across,
  run as a mutation and killed
  (all `cmd/director/groundinglifecycle_test.go`)

## Related

[[ADR-033-one-account-many-presentations]] · [[ADR-046-grounding-a-screen-points-at-its-structure]] ·
[[Visibility]]
