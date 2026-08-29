---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - observation
  - semantic-memory
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/perception/observation/kind.go
  - pkg/directorapi/observation.go
  - cmd/director/observesnapshot.go
  - internal/director/observe/composition_test.go
  - cmd/director/kindfrompixels_test.go
---

# ADR-107 — a sensor appearing is not the screen changing

## Context

[[ADR-106-a-place-is-not-how-long-you-looked-at-it]] closed 35D and left one defect diagnosed
and unfixed: on a machine with `$MARCO_VISION_MODEL` configured, one cold `director learn` on
one Windows Settings page that nobody touched left TWO durable Places.

Reproduced deterministically, twice, with the full state graph:

```
state_1   inferences 2    settled   … icon 22 …                established first
state_2   inferences 9    settled   … unknown 1 …   local_from=state_1, surface=state_1
state_transitions   state_1 -> state_2   count 1   unattributed 1

subj_e8447bc75334   … icon 22 …      matched never again
subj_71727a02470f   … unknown 1 …    what everything resolves to
```

The causal chain, every link of it deliberate and correct on its own:

1. The pass begins with nothing settled, so `EscalationOf` does not know whether the reading
   suffices and keeps the visual pass — ADR-104's rule that ignorance is not a decline.
2. Twenty-one detections nothing structural had reported join the composition as `icon`, and one
   element accessibility DID report — as `unknown`, which fusion correctly treats as no claim —
   is named `icon` by the detector.
3. That composition settles.
4. The reading is now placed and sufficient, so the gate stops buying — ADR-105.
5. The composition changes back.
6. The segmenter sees a coherent part of the surface replaced and calls it a different state of
   the same surface, which is exactly what that branch is for.
7. The licence is still open, so `PlacesToEstablish` — which walks every settled state, because a
   demonstration walks THROUGH places — makes both durable.

**Marco concluded that the world had changed when only its evidence had.**

## What the defect was NOT

The roadmap's hypothesis was that `unmatched + licence = establish` is the real defect: a licence
answers *may* this durable knowledge be created, not *is* this a new semantic state.

**Measured, that is not where this went wrong.** The licence acts on SETTLED SCREEN STATES, not
on unmatched samples, and establishing every state a pass settled on is right — an intermediate
place that never becomes durable leaves the edges either side of it unresolvable, which is what
`PlacesToEstablish` was written for. The establishment layer asked the segmenter which screens it
had seen and was told two. It was told correctly, from evidence that was wrong.

So the licence semantics are unchanged, and nothing here freezes a Place for a session, pins a
sensor, delays establishment, or reconciles two Places after the fact.

## Decision

**The composition a screen is identified by is what a source that can describe structure said
the screen is made of. A detector's own word is corroboration, not composition — unless pixels
are all Marco has.**

This is the precedence `StructureOf` already applies between the fused world and the detector,
asked one level down: that chose between two whole readings, and an authoritative reading can now
contain both kinds of evidence at once.

### Three answers, because two of them are not the same

`directorapi.KindEvidence` says who accounted for what a thing IS — which is a different question
from `Provenance.OnlyDescribesPixels`, which asks whether anything but a camera reported that the
thing is THERE.

| | what it means | what identity does with it |
|---|---|---|
| `described` | a structural source said what this is — or nothing recorded where it came from | counted, as it is |
| `pixel_named` | something structural reported the object and could not name it; a detector did | counted, as `unknown` — the object is real, its kind is a detector's word |
| `pixel_only` | pixels are the whole account | not counted, unless pixels are the whole reading |

Two bits were tried first and both single-bit rules were wrong, measured:

- **Keep everything a structural source reported** leaves the detector's twenty-one boxes in the
  identity, and the defect stands.
- **Drop anything a structural source did not NAME** removed `text x29` and `unknown x1` from a
  real Settings page. Accessibility described those elements and said they were text; a poorer
  claim than `button` is not the same as no account at all.

The zero value is `described`, deliberately. Elements are constructed as well as observed — a
fixture, a capability pack's enrichment, a hand-built query — and treating "nobody wrote down
where this came from" as "only a camera saw it" would quietly empty every composition built that
way. The same rule, and the same reason, as `OnlyDescribesPixels`.

### Where it lands

`NewScreenSignature` — which its own comment already calls *"THE choke point for what a screen
may be identified BY"*, and which both the segmenter and `stateFingerprint` read. One place, so
the state boundary and the durable signature cannot disagree.

It is classified in `buildSample`, beside the chrome classifier, for the reason that one gives:
this is the last point where the evidence and its provenance are both in scope. And it is carried
on the region exactly as `Chrome` is — **a label, not a removal**. Every structure is still
tracked, still counted toward whether the window was read, still addressable and still reported.
One consumer reads it.

### What it does not touch

No change to `EscalationOf`, to when a sensor runs, to fusion's role resolution, to the matcher,
to the tolerances, to the licence, or to the store. No sensor is named anywhere in the rule: a
future detector, a future OCR reader and a future model are all covered by the same sentence,
because the question is what kind of evidence accounted for a thing rather than which product
produced it.

## Measured

One cold Learn, Windows Settings → Mouse, detector configured, interface untouched throughout:

| | before | after |
|---|---|---|
| screen states | 2 | 1 |
| durable Places | 2 | 1 |
| samples / elapsed | 12 / 6.3s | 12 / 6.1s |
| providers that proved their target | accessibility + vision | accessibility + vision |

The surviving subject is `subj_71727a02470f` — **byte-identical to the Place a Director with no
detector at all establishes for the same page**. A configured machine and an unconfigured one now
learn the same Place, which also closes the compatibility question
[[ADR-105-repair-buys-knowledge-not-permission]] recorded as a known cost.

Selective perception is untouched: the same number of samples in the same wall-clock, with the
detector contributing to the same fraction of them. The fix costs nothing because it changes what
is counted, not what is acquired.

## Consequences

- A screen state can no longer be minted by Marco changing its own instrument. The evidence
  change is still visible — the regions carry it, the tracks keep it, diagnostics report it.
- `ElementRole.Generic` now has one definition, in `directorapi`, shared by fusion's role
  resolution and by this classifier. They must agree: an element given its kind under one rule and
  counted under another is how the two would drift.
- Retrospective Learn needs no second rule. It reaches `observe.PlacesToEstablish` over the same
  `ShadowTotals`, whose states come from the same segmenter and therefore the same filtered
  signature.
- A game read only by a detector is unaffected: nothing structural described anything, so pixels
  are the composition, which is the case the detector exists for.

## Enforced by

- `internal/director/observe` `TestASensorAppearingIsNotAScreenChanging` — one screen, seven
  readings alternating rich and primary, through the real `ScreenSegmenter`: one state
- `internal/director/observe` `TestReplacingWhatIsOnThePageIsStillAScreenChanging` — the other
  half, and the one that matters more: Settings navigates by the same local-replacement shape the
  defect wore, so the discrimination this must not lose is the one the defect looks like
- `internal/director/observe` `TestAScreenOnlyADetectorCouldDescribeIsStillOneScreen`
- `cmd/director` `TestTheSampleProductionBuildsSaysWhoNamedEachThing` — entered at `buildSample`,
  because 37E, 37F and 37G each found a correct mechanism wired to nothing
- `cmd/director` `TestAScreenNothingButPixelsDescribedIsStillAComposition`

## Related

- [[Experiment-018-composition-churn-from-the-perception-budget]] — the reproduction and the
  measurements, in full
- [[ADR-106-a-place-is-not-how-long-you-looked-at-it]] — where this defect was diagnosed
- [[ADR-104-perception-is-a-budget-not-a-habit]] — the gate whose own answer changed the evidence
- [[ADR-105-repair-buys-knowledge-not-permission]] — repair may improve what Marco knows; it may
  not improve what Marco may do, and now: nor what Marco thinks the screen IS
- [[ADR-101-visual-presence-is-not-legal-actionability]]
