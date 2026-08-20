---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - visibility
  - service
  - passive-observation
source_paths:
  - pkg/playbill/playbill.go
  - pkg/playbill/narrate.go
  - internal/director/service/playbill.go
  - cmd/director/playbill.go
---

# ADR-033 — one account, many presentations

Marco had ten diagnostic surfaces and no way to say what it was doing.

`status`, `world`, `events`, `perception`, `explain`, `observation`, `live-analysis`,
`observation-events`, `learned`, `game` — each answers a specialist's question precisely, and a
front-end that wanted to say *"I recognise this as the pause menu"* had to poll four of them and
join the results itself. The overlay's Director panel did exactly that, and it is the reason the
panel showed provider counts and fusion totals: those were the questions it could answer without
joining anything.

Putting the join in the client puts the DISAGREEMENT in the client. Two front-ends deriving
"is Marco waiting on me?" from four payloads will eventually derive it differently, and the
divergence is discovered by a person staring at two windows that contradict each other.

## Decision 1 — there is exactly one account, and the Director composes it

`pkg/playbill.View` is the whole of what a presentation is allowed to know. It is composed by
the Director and rendered by everybody: the overlay's Watch panel, the overlay's Diagnostics
mode, the overlay's always-visible status line, and `marco director watch | diagnose | normal`.

The composition is split across exactly two places, and the split is not arbitrary:

- `Runtime.Playbill` (cmd/director) owns CURRENT, SEEING, THINKING, LEARNING and the passive
  observation question. Only the Director knows those.
- `Server.playbillFor` (internal/director/service) owns DOING and the confirmation and
  clarification questions. Only the service knows those.

Merging them would mean handing the layer that observes the desktop a reference to the layer
that drives it — the coupling [[ADR-010-passive-observation-cannot-execute]] exists to prevent —
for the sake of one status field.

## Decision 2 — the WORDS live with the account, not with each surface

`View.Watch()`, `View.Deep()` and `View.Normal()` are in the shared package. A presentation
chooses colour, order and layout; it does not choose what a sentence says.

The alternative was tempting: let each surface phrase things to suit itself. Then "I couldn't
tell where that ended" and "destination unverified" become two surfaces describing one state,
and the person testing Marco has to know which to believe.

`Deep()` returns `Watch()` plus evidence, by construction — asserted, not intended. The question
a person asks of diagnostics is almost always "why does it say that", and answering it beside
the claim is what stops the two being compared from memory.

## Decision 3 — a translation, never a second derivation

Everything in `Runtime.Playbill` reads state that already exists, through
`observationRegistry.Snapshot` — the same production gather `director status` uses. It computes
no stability, forms no hypothesis, scores nothing and ranks nothing.

The one place this was expensive to honour is worth naming: asking "could this be written down
as a play yet?" goes through `observe.JudgeLowering` directly rather than through
`Runtime.LearnedPlay`, because `LearnedPlay` is a read everywhere except that it may raise a
naming question ([[ADR-031-the-user-names-the-stage]]). A visibility surface polling that would
interrogate somebody about screen names twice a second.

## Decision 4 — the presentation cannot reach the analysis

`pkg/playbill` imports no `internal/` package. The overlay imports `pkg/playbill` and nothing
else of the engine, and therefore has no path to the hypothesis generator, the recogniser or the
policy — so "just recompute it here" does not compile.

## Consequences

- Protocol version 6 adds `PLAYBILL`. It is non-mutating, so it stays answerable while a command
  is in flight — which is when a person most wants it.
- Adding a fact to a surface now means adding it to the account, where the privacy guard sees it.
- A surface that wants to say something the account cannot say is a signal that the Director does
  not know it, which is the correct place for that conversation to start.

## Enforced by

- `internal/director/service/playbill_test.go` — `TestRemovingTheRuntimeCallEmptiesTheWatchSurface`
  (the mutation), `TestTheServerContributesTheCommandHalf`
- `cmd/director/playbillwiring_test.go` — `TestAWatchedSessionsBeliefReachesTheAccount`,
  `TestTheAccountReportsTheSessionsOwnInterpretations`,
  `TestTheVisibilityRepresentationImportsNoAnalysis`,
  `TestReadingTheAccountDoesNotAskTheUserAnything`
- `pkg/playbill/playbill_test.go` — `TestDeepIsWatchPlusEvidence`,
  `TestEveryStateReducesToAHeadline`
- `plugins/overlay/watch_test.go` — `TestTheOverlayRendersTheAccountAndAddsNothing`
