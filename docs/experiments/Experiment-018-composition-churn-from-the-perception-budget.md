---
type: experiment
status: complete
date: 2026-08-28
backend:
  - production-perception
  - semantic-memory
  - vision
fixture: acceptance-37g.ps1
result: one-unchanged-screen-is-one-durable-place-whatever-sensors-ran
supersedes: []
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/perception/observation/kind.go
  - internal/director/observe/composition_test.go
  - cmd/director/kindfrompixels_test.go
---

# Experiment 018 — the screen that changed because Marco changed its mind

## Question

[[Experiment-017-live-place-identity-convergence]] measured that recognition is invariant to
sensor richness and that establishment is not: one cold `director learn` on one Windows Settings
page, with the visual detector configured and nobody touching anything, left TWO durable Places.

> Can one learning licence create several durable Places for one unchanged semantic screen
> because the perception budget changed?

## Method

Deterministic live reproduction before any code changed: isolated `$MARCO_HOME`, cold store,
detector configured, real Learn licence, real production perception, interface held still. The
state graph is read from the session's own account while the session is running, so the
segmentation that produced the Places is visible rather than inferred from the store afterwards.

Run twice before the fix, twice after, plus the same matrix
[[Experiment-017-live-place-identity-convergence]] used, re-run with the detector on.

## The defect, reproduced

```
state_1   inferences 2    settled   … icon 22 …
state_2   inferences 9    settled   … unknown 1 …    local_from=state_1  surface=state_1
state_transitions   state_1 -> state_2   count 1   unattributed 1

subj_e8447bc75334   … icon 22 …      established first, matched never again
subj_71727a02470f   … unknown 1 …    established second, and what everything resolves to
```

Reproducible and not identical between runs: the first run of the day recorded `icon 9`, the
second `icon 22`. **The same page, the same width, a different number.** What varies is how many
samples the escalation gate stayed ignorant for and what the detector drew in them — which is
itself the proof that a detector's box count is not a property of the interface.

## What the composition is actually made of

The fused world, one reading each way, on the same window seconds apart:

```
vision=false — 115 elements          vision=true — 136 elements
  button     accessibility x17         button   accessibility x14 + accessibility+vision x3
  image      accessibility x13         image    accessibility x2  + accessibility+vision x11
  unknown    accessibility x1          icon     accessibility+vision x1
                                       icon     vision x21
```

Two different things, and the difference decides the fix:

- **21 detections nothing structural reported.** Pixels and nothing else.
- **1 element accessibility DID report** — as `unknown`, which fusion treats as no claim at all
  (correctly: `generic` roles are not claims, so the first specific claim wins), so the detector
  named it `icon`.

That single element is not a rounding error. `unknown` is a layout role and `icon` is not, so it
alone moves the identity role SET. Two single-bit rules were tried and each failed on one half:

| rule | result |
|---|---|
| keep everything a structural source reported | the 21 boxes stay in identity; defect unchanged |
| drop anything a structural source did not NAME | removed `text x29` and `unknown x1` from the page |

The second failure is the instructive one. Accessibility described those twenty-nine text nodes
and said they were text. A poorer claim than `button` is not the same as no account at all.

## Result

`directorapi.KindEvidence` — described / pixel-named / pixel-only — classified in `buildSample`
beside the chrome classifier, carried on the region exactly as `Chrome` is, and read at
`NewScreenSignature`, the one choke point both the segmenter and the durable fingerprint go
through.

One cold Learn, same page, same conditions:

| | before | after |
|---|---|---|
| screen states | 2 | 1 |
| durable Places | 2 | 1 |
| samples / elapsed | 12 / 6.3s | 12 / 6.1s |
| providers proving their target | accessibility + vision | accessibility + vision |

The surviving subject is `subj_71727a02470f`, **byte-identical to the Place a Director with no
detector establishes for the same page**. A configured machine and an unconfigured one now learn
the same Place.

Selective perception is untouched — the same samples in the same wall-clock with the detector
contributing to the same fraction of them. Nothing was pinned, delayed, frozen or reconciled: the
change is to what is COUNTED, not to what is acquired.

## What the defect was not

The suspected heart was `unmatched + licence = establish`. Measured, it is not: the licence acts
on settled screen STATES, and establishing every state a pass settled on is right — an
intermediate place that never becomes durable leaves the edges either side of it unresolvable.
The establishment layer asked the segmenter which screens it had seen and was told two. It was
told correctly, from evidence that was wrong.

So the licence semantics are unchanged, and no orphan cleaner, migration, matcher loosening or
sensor pinning was added. See
[[ADR-107-a-sensor-appearing-is-not-the-screen-changing]].

## What this does not answer

**A rich reading of a page already known to be sufficient**, still. Production correctly has no
reason to buy one, and the deterministic composition tests cover the alternation that production
cannot produce live.

**Applications other than Windows Settings**, and detectors other than `icon_detect`. The rule
names no sensor, so a second detector is covered by the same sentence — but it has been measured
against one.
