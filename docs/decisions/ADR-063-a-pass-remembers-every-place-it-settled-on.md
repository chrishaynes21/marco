---
type: decision
status: accepted
date: 2026-08-17
supersedes:
  - ADR-047-a-place-is-remembered-a-meaning-is-answered
affects:
  - semantic-memory
  - passive-observation
  - demonstrations
  - learned-plays
source_paths:
  - internal/director/observe/establish.go
  - internal/director/observesession/runner.go
---

# ADR-063 — a pass remembers every place it settled on

Amends **one bound** of [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]: its
*at most one place per licensed pass*. Everything else in ADR-047 — the licence, the
observational/semantic split, the empty interpretation list, the refusal vocabulary, the
prohibition on touching a judgement — stands unchanged and is not reopened here.

## What was wrong

Establishment considered exactly one state: `ShadowTotals.CurrentState`, the place the user was
standing on when the pass ended. That is right for a pass that watches somebody *arrive*
somewhere. It is wrong for what Learn actually watches — a person walking **through** places to
reach one.

Measured live on 2026-08-17, on a cold store, with a person demonstrating
`Settings Home → Bluetooth & devices → Mouse`:

```
state_1  Home       7 inferences   settled   terms[back settings]
state_2  Bluetooth  3 inferences   settled   terms[back settings]
state_3  Mouse      3 inferences   settled   terms[back controls settings]
```

All three passed **every** quality gate. Only Mouse was established, because only Mouse was
current. So neither edge had two durable endpoints:

```
Home      → Bluetooth    destination_unresolved
Bluetooth → Mouse        source_unresolved
```

and the pass refused with `destination_not_recognised` — a demonstration that had been captured,
attributed and semantically resolved end to end, thrown away at the last step for want of a
subject Marco had already decided was worth remembering.

It also made [[ADR-056-a-goal-is-a-destination-not-a-route]] unreachable in practice. A goal is a
destination and a demonstration decomposes into per-edge knowledge; `A → B → C` yields two
reusable edges **only if B is durable**. Under the cap, every multi-step demonstration collapsed
to at most one edge, and usually to none.

## The decision

**A licensed pass may establish every place it settled on, each one passing the same gates
independently.** The place it ended on keeps its existing meaning and its existing refusal
reason; the others are reported alongside it.

- `observe.PlacesToEstablish(t, application, m, th) ([]PlaceCandidate, PlaceRefusal)` walks
  `t.States` in first-sighting order. The current state's answer comes from the unchanged
  `PlaceToEstablish`, so every existing reader and every existing refusal string means exactly
  what it did.
- `placeToEstablishAt` applies the **same gates in the same order** to a non-current state —
  describable → settled → discriminating → not already known. Deliberately not a relaxed variant:
  a place somebody merely passed through clears exactly what the place they stopped on clears.
- `PlaceEstablishment.Also []string` carries the others. `Subject` and `Reason` are untouched, so
  `Established()` still asks the question it always asked.

## Why widening the count is safe

ADR-047 capped this at one to avoid *"the unbounded persistence this mechanism is careful not to
be"* — concretely, to avoid minting a subject for every transition frame somebody walked through.
That concern is real. It is answered by the **gates**, not by the count, and the count was
standing in for them:

- a transition frame is not `Settled` — `settledComposition` needs `StatePromotionCount`
  observations with a stable modal count per role;
- a screen with nothing to recognise it by is not `Discriminating()`, and the store refuses it
  anyway;
- a place already known is already known.

What still bounds it, and all of it predates this ADR except the last:

| bound | what it stops |
|---|---|
| the episode licence | passive observation persists nothing, still |
| the four per-place gates | frames, blanks, and screens nothing could match again |
| `MaxSubjects` | the store as a whole |
| `DefaultCaptureBounds().MaxCheckpoints` | a pass that settled on more places than a demonstration may have checkpoints |

That last one is the new bound, and it is the existing one restated. A pass that settled on more
distinct places than a demonstration is allowed to have checkpoints is not a route — it is a
tour — and the number that says so for the capture says so here.

Order is first sighting, so a walk is established in the order it was walked and two identical
sessions write identical files.

## What does not change

- **No judgement, anywhere.** Every place established here carries an empty interpretation list,
  exactly as the single one did. Widening *which* places are remembered does not widen what is
  claimed about any of them, and there is a test that says so per subject rather than for the
  first one.
- **No new licence.** `Episode.EstablishPlaces` is still set by `teachPasses.episode()` alone.
- **A store failure is still not fatal.** It is fatal to the *report* only for the current place,
  which is the one a refusal message is about.

## Considered and rejected

- **Establish the intermediate places lazily, when an edge needs an endpoint.** Puts a memory
  write inside the relationship fold, where the code has no licence in hand and no refusal
  vocabulary to explain itself with. It also writes places for edges and not for evidence, so two
  passes over the same walk would store different things.
- **Relax the gates for intermediate places** on the grounds that they are seen briefly. That is
  precisely backwards: brief observation is *less* evidence, not licence for less. Bluetooth
  settled on three inferences on its own merits and needed no help.
- **Keep the cap and make Learn demonstrate one edge at a time.** Optimising the choreography
  rather than removing the need for it, which R34 exists to stop doing.

## Enforced by

- `internal/director/observesession/establishwiring_test.go` —
  `TestEveryPlaceOnTheRouteBecomesDurable`: a pass that walks through one settled place and stops
  on another leaves **both** durable, reports the intermediate one in `Also`, and the report
  agrees with the reopened file. Mutation-gated twice — narrowing `PlacesToEstablish` back to the
  current state fails it, and making the runner's loop skip non-current candidates fails it.
  `TestAnUnlicensedPassEstablishesNoPlaceHowManyItSees` is the control: the licence did not widen.
  `TestTeachingEstablishesTheStartThroughTheProductionPath` and
  `TestAnEstablishedPlaceCarriesNoUserJudgement` now assert the property — every settled place
  durable, none carrying a judgement — rather than ADR-047's count.

## Related

[[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-062-a-scroll-bar-is-not-a-screen]] ·
[[Experiment-014-identity-variance-across-real-applications]] ·
[[Semantic-Memory]] · [[Demonstrations]] · [[Learned-Plays]]
