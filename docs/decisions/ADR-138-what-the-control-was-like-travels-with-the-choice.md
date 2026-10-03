---
type: decision
status: accepted
date: 2026-10-02
affects:
  - semantic-actions
  - safety
  - execution
source_paths:
  - pkg/directorapi/record.go
  - internal/director/target/target.go
  - internal/director/uiact/uiact.go
  - cmd/director/graph.go
---

# ADR-138 — what the control was like travels with the choice

## Context

Found by asking Marco for ordinary things in ordinary words, with the fixture's oracle printed
beside every verdict. Three requests in a row against one checkbox:

```
untick remember me   remember false -> TRUE    unverified
uncheck Remember me  remember true  -> false   performed
untick remember me   remember false -> false   performed ("already unchecked")
```

The first and third are the same sentence against the same state and they did opposite things.
Asked to turn something off, Marco turned it on.

The proximate cause is that a checkbox has only one mechanism and it is a FLIP — the C# host calls
`tp.Toggle()`, which takes no argument and no direction — while `alreadySatisfied` could not tell
the box was already unchecked, because `Checked` was nil.

The real cause is one line that was never written. The capability ladder reads three things off
its target, and all three arrived through a single optional field:

```go
// Elements looks up the element a semantic action targets, for the state and
// patterns the ladder reads. Optional — see the Elements doc for what is lost.
Elements Elements
```

**`execute.Executor.Elements` was never assigned in the production Director.** The only
`Elements:` in `cmd/director` is an unrelated count. So on every desktop, every semantic action
reached `uiact.TargetFrom` with `el == nil`, and:

| what the ladder reads | what it got | what that did |
|---|---|---|
| `PatternsKnown` | false | every pattern rung offered on ROLE evidence, never on reported capability |
| `Checked` / `Expanded` | nil | the idempotent verbs could never short-circuit, so they flipped blindly |
| `Enabled` | defaulted true | the ladder's own `"%q is disabled"` refusal was unreachable |

Each degrades into something that looks like a decision, which is why a green suite never saw it.
`Elements`' own doc described the cost accurately and nothing checked that it was paid.

The host had been right all along. Queried directly:

```
"Label":"Remember me","Patterns":"toggle","Checked":"false"
```

The fact was dropped two layers after it arrived.

## Decision

**A resolution carries what the chosen control was LIKE, not only which control it was.**

`ResolvedTarget` gains `Control *ControlFacts`: enabled, offscreen, the reported patterns with
their known/unknown distinction, and the three-valued expanded/checked/selected states. It is
filled by `directorapi.FactsOf(el)` at both places that resolve a target — the Director's resolver
and the executor's re-resolution — from the element each already holds. `uiact.TargetFrom` reads
it.

This belongs on `ResolvedTarget` rather than behind a second lookup for two reasons. The type
already exists to carry "what was chosen" forward — `Role` and `Label` are there "so history stays
readable after the element itself is long gone", and a control's capabilities and state are facts
of exactly that kind. And the resolver is holding the element at the moment it chooses, so
recording them costs nothing, where a later lookup would mean **a second observation of a world
that had already moved** — against a `collect → fuse → enrich → record` cycle measured at ~250ms
with side effects of its own.

`Elements` stays on the Executor for a caller that has a world in hand, but production no longer
depends on it, and nothing silently loses a capability when it is absent.

## And a destination is not a direction

The wiring restores the state. A second decision keeps the system honest when the state is
genuinely missing, which it still can be — see
[[ADR-139-a-destination-is-not-a-direction]].

## Consequences

- Measured after, same protocol: `untick` on an unticked box leaves it unticked and says "already
  unchecked"; `tick` on a ticked box leaves it ticked; both directions work and both are
  idempotent. `click Delete` reaches the disabled refusal.
- Pattern selection is now made on reported capability rather than on role, everywhere. That is a
  behaviour change on every desktop and the reason the acceptance suite was re-run: 12 pass, 0
  fail, 0 skip.
- `ControlFacts` is nil when a resolver did not record them, and every reader must treat that as
  unknown rather than as "the control has none". The same distinction `PatternsKnown` draws one
  level down, for the same reason.

## Enforced by

- `TestAResolutionCarriesTheControlsState` — the resolver records patterns, state and enabled.
- `TestAnUnreportedStateStaysUnknown` — nil stays nil; rounding it to false is the whole defect.
- `TestAProviderThatReportsNoPatternsStaysUnknown` — "supports none" is not "did not say".
- `TestTargetFromWorksWithNoElement` — **the wiring test this needed and never had.** It asserts
  the shape production actually uses: a ResolvedTarget with ControlFacts and no element at all.
  Deleting the `rt.Control` read kills it.
- `TestADisabledControlIsRefusedThroughTheProductionShape` — the third casualty, now reachable.
- Each half was mutated away separately and the matching tests died.
