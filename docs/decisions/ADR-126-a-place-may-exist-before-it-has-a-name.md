---
type: decision
status: accepted
date: 2026-09-03
affects:
  - passive-observation
source_paths:
  - cmd/director/observeledger.go
  - cmd/director/observelook.go
---

# ADR-126 — a place may exist before it has a name

## Context

[[ADR-123-a-control-you-can-see-is-worth-knowing-about]] made dwelling establish Places, then made
it wait for the screen's word to settle — a reaction to a real defect, where transit screens became
durable before their name recurred and a walk rendered as `Home --> Unnamed place --> Mouse`.

Three runs later, with segmentation ([[ADR-124-a-screen-may-say-which-screen-it-is]]) and
settlement ([[ADR-125-settle-the-screen-not-the-frame]]) both corrected, the instrument showed what
that gate was now costing:

```
afh state_1   7 read, word read 7x, settled       ->  ESTABLISHED
afh state_2   1 read, word read 1x, settled       ->  no Place
afh state_3   2 read, 3 shapes, DIFFERENT kinds   ->  not settled, no Place
```

`state_2` is a real page somebody walked through: settled, one distinct shape, no contradiction,
refused **solely because its name had been admitted once rather than twice**. `state_3` is the
control — it should be refused, and still is, because it never settles.

## Decision

Two jobs, two gates:

```
Place establishment  is gated by coherent settled state evidence.
Place naming         is gated by name recurrence.
```

`settledPlaceName` is untouched and still wants its two sightings. What changed is that it no
longer decides whether the SCREEN may exist.

An unnamed Place is a legitimate thing, and this introduces no new category: crossing promotion has
always established nameless endpoints, because an edge whose destination cannot be written down is
lost. What this removes is the inconsistency between the two establishment paths.

The name attaches later, to the same subject, through the naming sweep that already runs on every
reading — establishment is idempotent by signature, so a second visit that can name the screen does
not mint a second record.

### What still refuses

`look.Shape` is non-nil only for a state `PlacesToEstablish` accepted: settled, not loading,
discriminating, describable. That gate is only worth resting on because ADR-124 and ADR-125 made a
settled state mean something — before them one state could absorb four pages and never settle.

And a state carrying **two words** is refused. One word tallied against a screen is Marco
recognising it; two is Marco not knowing which screen it was looking at, which is evidence
segmentation put two together. Establishment is the last place that can decline to make a
segmentation mistake permanent.

## Consequences

- Unnamed Places will be more common in the map. `PlaceWordsAsking` already describes one by what
  it is made of, and that presentation question is deliberately left to a later dogfood: get the
  knowledge model right first, then find out whether these nodes read acceptably.
- The feed announces such a Place with whatever `PlaceWords` gives it, which is `Unnamed place`
  until the word settles. That is more truthful than suppressing the screen entirely.
- Naming still reaches nothing in durable identity. A word is not an identity, and the same Place
  keeps its id when the word arrives.

## Enforced by

- `cmd/director` `TestACoherentScreenBecomesAPlaceBeforeItsNameSettles` — and that the name
  attaches later to the same subject
- `cmd/director` `TestAScreenThatSaysNothingStillBecomesAPlace`
- `cmd/director` `TestAStateCarryingTwoWordsIsNotEstablishedByDwelling`
- `cmd/director` `TestAScreenThatCouldNotBeEstablishedIsStillRefused`
- `cmd/director` `TestACrossingStillEstablishesAnEndpointItCannotName` — unchanged
- `cmd/director` `TestWatchingAloneRemembersNoPlaceItMerelyLookedAt`

## Related

- [[ADR-123-a-control-you-can-see-is-worth-knowing-about]]
- [[ADR-125-settle-the-screen-not-the-frame]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
