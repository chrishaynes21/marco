---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - perception
  - fusion
  - passive-observation
  - semantic-memory
source_paths:
  - cmd/director/observewiring.go
  - cmd/director/observesnapshot.go
  - cmd/director/substrate_test.go
  - internal/director/perception/fusion/engine.go
  - internal/platform/navsource/navsource.go
---

# ADR-100 — Marco sees through evidence

## The claim

Marco does not see through UIA, or OCR, or a vision model. It sees through **evidence**, and one
component turns evidence into belief.

```
sensors observe            UIA, vision, OCR, window context, temporal continuity
FUSION interprets          one coherent semantic reading of what is present NOW
semantic memory remembers  what Marco has learned about the computer
the graph models           transitions between Places
goals describe             the outcome somebody wants
planning chooses           the route, now
policy grants              permission
execution acts             through legal Marco
verification proves        what actually happened
```

Fusion answers exactly one of those questions and must not own any other.

## The audit — what the pipeline actually is

This roadmap's first deliverable was to establish the production pipeline rather than infer it from
package names. It is:

```
window (validated by the runner, pinned for the cycle)
    ↓
providers.Collector.Collect      accessibility · window system · OCR · vision
    ↓                            (+ shadow providers, routed away from belief)
observation.Cycle
    ↓
fusion.Engine.Fuse               THE evidence/belief boundary
    ↓
directorapi.WorldState           elements with roles, labels, geometry, provenance
    ↓
buildSample                      the narrowing into the semantic layer
    ↓
observe.Sample                   entities, chrome, actionables, place-name evidence
    ↓
observation session              shadow tracks, screen states, transitions
    ↓
PlaceNow / StructureSignature / target resolution / ReachOfState
```

**There is one door.** `Engine.Fuse` is called from exactly four places, and each serves a
different surface rather than a different interpretation:

| | |
|---|---|
| `observewiring.go` | the observation SESSION sampler |
| `runtime.go` | the foreground pipeline — waits, commands, diagnostics |
| `main.go` | a one-shot reading for the inspect commands |
| `ocrwiring.go` | the text-reading path |

**And every semantic consumer is a session.** Ambient Observe runs a long unlicensed one, Learn a
licensed one, execution takes a short look, and 36F's recovery takes another after a failure. All
four are observation sessions, so all four go through `liveSampler.Sample`, so all four consume the
same fused reading. `Observe`, `Learn`, `Perform` and recovery do not have four perceptions; they
have four sensor budgets on one.

**Sensors reach belief; belief does not reach sensors.** `internal/director/observe`,
`semanticmemory` and `ambient` cannot import a provider, a capture surface or the fusion engine.
That is structural rather than conventional: no amount of editing inside them can grow a second
reading of the screen.

### What runs, and what does not

- **Always**: accessibility, window system.
- **Requested on every session cycle**: vision (`observation.WithVision`), and OCR as well when a
  text engine is available (`WithPixels`). A provider that cannot run reports unavailability rather
  than emptiness — `TestAFailedSourceReachesTheWorldAsDegradationNotAsAbsence` holds that, and it
  is the difference between "I did not look" and "there is nothing there".
- **Never**: `shadow.Provider` — the ScreenParser ONNX experiment. It implements `ShadowOnly`, so
  the collector routes its evidence to `Cycle.Shadow` and **fusion never sees it**. It exists so a
  diagnostic can report what it saw and what it cost, and its evidence has no authority at all.

That last point is the honest answer to "does visual parsing repair a degraded accessibility
reading today": **the authoritative visual provider can, and the ScreenParser experiment cannot,
because its evidence is not admissible.** Whether to promote it is a decision with its own
evidence to gather, not a wiring change.

## What was wrong

One thing, at the seam between the fused world and the consumer that resolves human clicks.

`pushActionables` iterated `world.Elements` — a **map** — and truncated at `MaxActionables`. So on
a screen offering more clickable controls than the bound, *which* of them reached the navigation
producer depended on Go's map iteration, which Go deliberately randomises per range.

Two readings of one unchanged screen offered different sets. That set is what a human click is
attributed against (36B), so the same press could resolve to a Target on one reading and to nothing
on the next — and from outside, "Marco didn't see what you clicked" is indistinguishable from a
perception failure.

A bound is fine. **Which half of a screen it keeps must not be a coin toss.** The offering is now
sorted into reading order — top to bottom, then left to right, then role, label and id — before the
bound is applied. That order is transient: it decides what to KEEP when there is too much, and
nothing durable is derived from it.

The neighbouring path, `buildSample`, already sorted its element ids. `AdmittedPlaceName` is
order-independent by construction and says so at its own definition — it returns nothing when two
entries disagree, so there is no first to pick. Those were checked and left alone.

## The rules that did not change, and why they are worth restating

**Geometry is matching evidence, never identity.** A Place's durable signature is semantic
structure — roles, terms, tolerated counts, normalised envelope — per
[[ADR-091-a-place-is-not-its-presentation]]. Moving every control on a screen does not make it a
different screen, and a resolution change does not either.

**Temporal identity is transient.** The fusion tracker carries element identity between cycles so
"click that again" can mean something. It is perception state, bounded by
`observation.DefaultHistory`, and a tracker id never reaches durable memory: a restart loses it and
loses nothing semantic.

**Expectation is not observation.** Semantic memory may help *match* what a sensor reports. It may
not manufacture an element nobody saw, and a graph edge expecting a Target does not make that
Target actionable. Fusion outputs what evidence supports.

**A missing sensor is not a negative observation.** OCR not being installed is not evidence that
there is no text, and healthy accessibility is not degraded by an optional provider failing.

**Unknown and unreadable stay separate.** A healthy reading of a screen Marco has never learned is
*unknown*. A reading whose evidence is insufficient for the question being asked is *unreadable*.
`ReachOfState` decides the second from arrangement — a window whose every large space came back
empty — and never from a control count, which is knowledge that layer does not have.

**Fusion has no authority.** It emits no input, holds no desktop lease, mints no grant and grants
no Learn permission. It cannot write the graph; the semantic layers it feeds cannot reach it.

## KNOWN FOLLOW-ONS

1. **ScreenParser remains shadow-only.** The most likely next perception step is deciding whether
   its evidence is admissible, which needs measurement against real degraded readings rather than a
   wiring change. Promoting it casually would let an experimental detector's boxes become Places.
2. **Actionability in `observe.Sample` is `Enabled && Visible`**, which is a fact about the element
   rather than a claim that a legal actuator can reach it. Today that is safe because the
   authoritative visual provider does not assert actionability it cannot support, and because
   target resolution has to find a real control to press. It would stop being safe the moment a
   visual-only element could lower to a coordinate click without passing that resolution.
3. **No performance measurement was taken.** The phase timings exist on every sample —
   `Detect`, `Fuse`, `Snapshot`, `Total` — and are reported per reading, but no baseline was
   recorded for this roadmap, so no boundedness target is claimed.
4. **The degraded-repair case is unmeasured.** Whether a real Settings window whose accessibility
   came back as shell-only is repaired by the authoritative visual provider is a live question, and
   the roadmap forbade breaking accessibility to manufacture one.

## Enforced by

- `cmd/director` — `TestWhichControlsAreOfferedDoesNotDependOnMapOrder` (the defect, and reading
  order); `TestFusionIsTheOnlyDoorFromSensorsToBelief`;
  `TestOnlyTheSessionSamplerBuildsAReading`; `TestTheSemanticLayersCannotReachASensor`.
- `internal/platform/navsource` — `TestReadingTheActionablesCannotRaceTheClassifier`.
- And, holding fusion itself, `internal/director/perception/fusion` —
  `TestThreeSourcesFuseIntoOneElement`; `TestConflictingLabelsBlockMerge`;
  `TestIncompatibleRolesBlockMerge`; `TestDifferentWindowsNeverMerge`; `TestFusionIsDeterministic`;
  `TestAFailedSourceReachesTheWorldAsDegradationNotAsAbsence`;
  `TestEveryElementRecordsTheEvidenceItCameFrom`; `TestElementIdentityIsStableAcrossCyclesAndOwnedByFusion`;
  `TestIdentitySurvivesAWindowMove`.

## Related

[[ADR-099-a-failed-attempt-is-not-a-false-edge]] ·
[[ADR-091-a-place-is-not-its-presentation]] ·
[[ADR-010-passive-observation-cannot-execute]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-005-legal-marco-only]] ·
[[Perception]] · [[Fusion]]
