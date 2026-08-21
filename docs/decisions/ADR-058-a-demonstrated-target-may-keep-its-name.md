---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - navigation
  - passive-observation
  - demonstrations
  - learned-plays
source_paths:
  - internal/director/observe/input.go
  - internal/director/observe/sample.go
  - internal/platform/navsource/navsource.go
  - cmd/director/observewiring.go
  - internal/director/rehearse/rehearse.go
---

# ADR-058 — a demonstrated target may keep its name

`NavPoint` made a click observable and left it meaningless: a position is not a meaning, so
every mouse-driven demonstration ended at `unresolved_pointer_target`
([[ADR-042-a-click-is-a-place-in-a-window]] left that as the named prerequisite). Most
desktop software is mouse-driven.

## The decision

A placed input event may carry a **semantic target** — the role of the control it landed
on, and that control's name when one of two licences admits it:

1. **The canonical plaintext role allowlist** (`directorapi.ElementRole.NameablePlaintext`)
   — a button's name is a fact about the interface, exactly as the label classifier already
   holds.
2. **The demonstration licence** — during an explicit Learn pass, a role a person can
   ACTIVATE (a list item, a tree item, a link) may carry its name too, for the one control
   their own input landed on. The same shape as
   [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]: an explicit human semantic
   event licenses persisting the one identity it is about, and reaches nothing else on the
   screen.

Both gates and the standing shape filter live in ONE function beside the classifier —
`observe.AdmittedTargetLabel` — and the licence travels from the episode the caller
declared (`Episode.EstablishPlaces`), so passive observation structurally cannot widen what
a label rides on.

**Resolution is at event time, from admitted evidence.** The composition root pushes each
valid inference's actionable controls (absolute rectangles, roles, admitted labels, the
focused one) to the navigation producer beside the window bounds it already pushes; the
worker resolves a placed press to the smallest containing control, and a confirm to the
focused one. The index obeys the same two-sided freshness rule as the pointer frame.
Resolution failing costs the enrichment, never the event
([[ADR-057-attributed-input-survives-interpretation]]).

## Where a name may persist, and where it may not

- The **durable topology** carries no labels, structurally: session transitions hold
  `TargetedSequence`, the store holds `NavSequence`, and the fold strips at the type
  boundary.
- The **candidate** of a licensed demonstration keeps its steps' targets — the meaning of
  the demonstration, named as a visible exception in the candidate privacy sweep.
- The **rehearsal** aims by the name: a `point` step with a named target lowers to
  `Accessibility's Invoke` on the control resolved against the LIVE tree at emission time
  (`rehearse.ControlResolver`), uniqueness demanded, and refuses before emitting when the
  control is not on offer. No coordinate is ever aimed.
- A **saved play** still cannot say a click (`cannot_say_pointer`): writing one down needs
  a run-time name-resolving capability like `Screen's Showing`'s, which is follow-on work.

`PRIVACY_WIDENED: YES` — deliberately, reviewed as such, and recorded in the two
type-allowlist tests it required widening.

## Enforced by

- `TestRawKeyIdentityCannotCrossTheBoundary`,
  `TestASemanticTargetCannotCarryPhysicalIdentityOrGeometry`,
  `TestAPlacedPressResolvesToTheSmallestControlUnderIt`,
  `TestAPressSurvivesEveryWayResolutionCanFail`,
  `TestAConfirmResolvesToTheFocusedControl` (`internal/platform/navsource`)
- `TestAValidInferenceOffersActionablesToTheProducer`
  (`cmd/director/navcontextwiring_test.go`) — the passive/licensed label gate, through the
  production push
- `TestNoRawInputCanReachADemonstration` (`internal/director/observesession`) — the sweep,
  with the one named exception
- `TestAClickDemonstrationBecomesAnAimableCandidate`
  (`internal/director/observesession/goalwiring_test.go`)
- `TestADemonstratedClickRehearsesAsAProduction`, `TestAnUnproducibleClickEmitsNothing`
  (`internal/director/rehearse/route_test.go`)

## Related

[[ADR-042-a-click-is-a-place-in-a-window]] · [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[ADR-017-structure-earns-a-name-text-never-earns-structure]] · [[Navigation]]
