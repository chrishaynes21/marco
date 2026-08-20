---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - visibility
  - semantic-memory
  - hypotheses
source_paths:
  - pkg/playbill/playbill.go
  - pkg/playbill/narrate.go
  - pkg/playbill/guard.go
---

# ADR-035 — uncertainty survives the screen

Every discrete verdict in the Director exists because a number would have been a lie.
`MatchVerdict` has four values and no similarity score precisely so that two remembered screens
at 0.71 and 0.69 read as *"I cannot tell which"* rather than as *"the first one"*
([[ADR-016-cross-session-identity-is-structural-and-conservative]]).

A rendering layer is where that care gets thrown away, because a percentage is easier to draw
than a hedge and looks more impressive.

## Decision 1 — no Director confidence reaches a person as a number

`Recognition` is `unobservable | unknown | candidate | ambiguous | contested | recognised`.
`Standing` is the hypothesis vocabulary carried across unchanged. There is no field on the
account that holds a Director confidence, so there is nothing for a surface to render as 87%.

## Decision 2 — a provider's own metric may appear, labelled as the provider's own

A detector's 0..1 output is a fact about that detector. It may appear in DIAGNOSTICS, it must
carry the metric's name, and the guard refuses a score with no name — because a bare number
beside a provider is exactly how a detector's threshold becomes "confidence" in a reader's head.
It never appears in WATCH and it is never converted into a Director confidence.

## Decision 3 — every hedge survives into the sentence

`candidate` renders as *"I've seen a screen like this before, but I'm not certain it's the same
one."* It never renders as *"I recognise this."* A renderer that upgraded a hedge would be the
most damaging bug this milestone could ship, because it would look like progress.

`Recalled` is separate from `Confirmed` for the same reason: *"you told me this"* and *"you told
me this before"* are different claims, and *"I worked it out"* is a third.

## Decision 4 — a claim ships with its contradictions or it does not ship

`Reading.But` carries every contradiction the hypothesis carried. A reading shipped with only its
supporting evidence is advocacy, which is the same rule
[[ADR-014-hypotheses-are-evidence-not-identity]] applies one layer down.

## Decision 5 — silence gets an explanation

"Marco did not offer to learn anything" has a dozen causes — too few sessions, no navigation
evidence, all of it context-admitted, already declined, another question open, memory
unreadable. `Learning.Silence` carries the closed reasons, because without them a person cannot
tell which, nor whether the policy is working at all.

## Consequences

- Watch is less reassuring than a progress bar, and that is the point: if Watch reveals that
  Marco is performing badly, the milestone has succeeded.
- An unmapped hypothesis concept renders as *"this screen"* rather than as its own identifier.
  Adding a concept without adding its sentence loses detail; it cannot leak vocabulary.

## Enforced by

- `pkg/playbill/playbill_test.go` — `TestUncertainVerdictsNeverReadAsCertainty`,
  `TestNoConfidencePercentageReachesWatch`, `TestNoInternalIdentifierReachesWatch`,
  `TestAProviderScoreMustSayWhoseItIs`
- `cmd/director/playbillwiring_test.go` — `TestARecognisedScreenIsCalledWhatTheUserCallsIt`,
  `TestAnUnnamedScreenIsDescribedRatherThanIdentified`
- `cmd/director/playbillmoment_test.go` — `TestTheSyntheticMarcoMoment`
