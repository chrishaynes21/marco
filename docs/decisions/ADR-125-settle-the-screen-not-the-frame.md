---
type: decision
status: accepted
date: 2026-09-03
affects:
  - passive-observation
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/establish.go
---

# ADR-125 — settle the screen, not the frame

## Context

[[ADR-124-a-screen-may-say-which-screen-it-is]] fixed segmentation: four Settings pages walked at
normal speed became four transient states instead of one, and three Chrome Places collapsed back
into one. Mouse and System still did not become Places — but the failure had moved downstream, and
the trace said where:

```
applicationframehost state_4   agreeing 1 of 1 (2 distinct shapes), settled false
    2 shapes, same kinds, worst count drift 6 [group list_item text]
```

**The same kinds throughout, differing only in how many, saying the same word on every reading.**
Nothing contradicted anything.

`settledWhole` requires one exact whole role histogram to recur twice. The unit is
`compositionKey` — `role=count` for every role — so `text=14` and `text=15` are different
compositions. A page visited at normal speed gets two or three readings, and a live interface
almost never presents an identical histogram twice in that window. Reproduced through the
production segmenter:

```
wobble   readings=3 agreeing=1 distinct=3 settled=false  names=[Mouse:3]
steady   readings=5 agreeing=5 distinct=1 settled=true   names=[Mouse:5]
```

Both are ONE transient state. The wobble refuses because a status line moved.

## Decision

A state may settle when its readings **agreed about where they were while their counts moved**:

- enough sightings — never one frame
- the destination word tallied against it is **unanimous** and settled by recurrence
- every distinct composition is made of the **same kinds**

The composition it settles on is still the most-seen one, by the existing tie rule. It is a shape
Marco actually saw, never an average — the invariant [[ADR-073-a-place-is-a-composition-marco-saw]] exists for is untouched.

### This is not "settle faster"

Nothing shortens a wait or lowers a count. What changes is **what has to recur**. ADR-124
established that transient continuity is a different question from structural equality; this is the
same distinction one layer later. Settlement was still asking the frame question after segmentation
had already answered the semantic one.

### Same kinds, because that is where the measurement divides

The same live run showed the other case, and it must keep refusing:

```
applicationframehost state_1   2 shapes, DIFFERENT kinds, drift 9 [button group image progress_bar text]
chrome state_1                 7 shapes, DIFFERENT kinds, drift 34 [button group link tab text text_field]
```

The first folded in a loading frame — `progress_bar` is the indicator `stillLoading` already knows.
The second is a page whose content genuinely churns. A composition that gained or lost a KIND is a
page mid-arrival or a different page, and neither may settle on a word. Counts moving is drift;
kinds moving is not.

### Unanimous, not merely settled

`settledPlaceName` tolerates a minority word — three `Mouse` against one `System` names the screen
Mouse, which is right for NAMING. It is wrong here: the minority word is evidence that segmentation
put two screens together, and settlement is the layer that must not turn a segmentation mistake
into durable knowledge because it grew permissive.

### The negative result: structure alone cannot say a screen persisted

A later attempt tried to remove the recurring-word requirement from coherence, on the reasoning
that it made coherence mean *structurally coherent AND named twice* and so defeated the separation
between settlement and naming. That reasoning is sound and the change is unsafe, and the reason is
worth keeping so nobody rediscovers it:

**The coherent path only ever fires when no composition repeated** — one seen twice settles through
the ordinary rule and never reaches here. So the condition it answers is *same kinds, counts
moving, nothing repeated*, and that is exactly the condition of a screen still rendering in:

```
arriving([]int{4, 9, 14, 18}, 21, 0)     same kinds throughout, every stage seen once
```

`TestAScreenStillArrivingIsNotIdentityBearing` already held that case, and the defect behind it was
measured across four independent cold stores: a learn's START, fingerprinted mid-render, minted a
new subject nearly every time while its DESTINATION, read once the page had finished, reproduced
the same one. Partial and full renderings of one page becoming two durable Places.

Monotonicity does not divide the two either — the live wobble climbed, `14, 15, 16`.

> **Same structural kinds across changing compositions are insufficient evidence of semantic
> persistence, because a partially arriving interface has the same observable shape.**

So the recurring word is not a naming requirement wearing a settlement hat. Recurrence supports two
different conclusions from one piece of evidence: for NAMING, it earns the right to persist the
word; for SETTLEMENT, it is the only available evidence that the state survived across moments
rather than being one partially-rendered frame. What remains genuinely open is not the rule but the
evidence density — see 38D.4.

## Consequences

- Interfaces that make no destination claim settle exactly as before: one whole composition, twice.
  This is a widening for applications with an accessibility tree rich enough to say what page they
  are on, and a no-op for everything else.
- The sightings floor is **redundant** and is written down as redundant rather than left claiming a
  test: a settled name needs two tallies, two tallies need two readings, two readings are two
  sightings. It stays because "one frame never settles a screen" is a rule this function has to
  state rather than leave implied.
- Durable Place identity, segmentation, polling and naming are all untouched.

## Enforced by

- `internal/director/observe` `TestAScreenThatKeptSayingWhatItWasSettlesThroughItsWobble`
- `internal/director/observe` `TestAWobblingScreenIsRememberedAsAShapeItActuallyHad`
- `internal/director/observe` `TestAScreenWhoseKindsChangeDoesNotSettleThroughCoherence`
- `internal/director/observe` `TestAMinorityWordStopsAWobblingStateSettling`
- `internal/director/observe` `TestAWordSeenOnceCannotSettleAWobblingScreen`
- `internal/director/observe` `TestOneReadingNeverSettlesHoweverWellNamed`
- `internal/director/observe` `TestWithNoDestinationSettlementIsUnchanged`

## Related

- [[ADR-124-a-screen-may-say-which-screen-it-is]]
- [[ADR-123-a-control-you-can-see-is-worth-knowing-about]]
- [[Experiment-022-the-first-dogfood]]
