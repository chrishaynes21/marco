---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - fusion
  - vision
source_paths:
  - cmd/director/benchscreenparser.go
  - pkg/directorapi/actionability.go
  - pkg/directorapi/actionability_test.go
  - pkg/directorapi/observation.go
  - internal/director/policy/policy.go
  - internal/director/perception/shadow/provider.go
---

# ADR-101 — Visual presence is not legal actionability

## The decision

**`SCREENPARSER_DECISION = REMAIN_SHADOW_ONLY`** — on measured cost and an unmeasured workload,
not for want of a runtime. And, separately, because the safety property admission would have
depended on **did not exist**. It does now.

## What was actually found

### One: it runs here, and it is good at menus

An early pass concluded the measurement could not be taken because no ONNX Runtime was
installed. **That was wrong** — the runtime is in the repository
(`tools/onnxruntime/onnxruntime-win-x64-1.28.0/`, and an older 1.26 copy beside the plugin), the
model is at `tools/vision-export/weights/screenparser-1280.onnx`, and the only missing pieces
were `$MARCO_ONNXRUNTIME` and a plugin built with `-tags onnxvision`. The first search looked
outside the repository and missed both.

Measured over `fixtures/vision/v2/rocketleague` — 39 frames, 5 sequences, 120 annotated regions:

| | ScreenParser | classical CV | Grounding DINO |
|---|---|---|---|
| structural precision / recall | **90% / 63%** | 0% / 0% | 77% / 17% |
| nameable precision / recall | **81% / 88%** | 0% / 0% | — |
| detections | 145 (80 TP, 9 FP, 44 FN) | 3915 (0 TP, 1858 FP) | 43 (20 TP, 6 FP) |
| latency median / p95 | **895–932ms / 992ms–1.04s** | 1ms / 2ms | — |
| ScoreV2 | **66.8** | 5.0 | — |

Per sequence, the shape of it is clear:

| sequence | precision | recall |
|---|---|---|
| pause-stable (a static menu) | 100% | 83% |
| pause-open | 100% | 56% |
| pause-close | 75% | 67% |
| freeplay-camera-motion (in-game) | 50% | 10% |
| freeplay-static (in-game) | — | 0% |

**It reads menus and panels well and gameplay HUD barely at all**, which is exactly what a
UI-trained detector should do. Nameable precision and recall had been 0% for four milestones;
81%/88% is the number the [[director-vision-ui-detector-decision|detector survey]] set out to
move, and it moved.

### Two: the benchmark was comparing one model against itself

The `current` and `screenparser` rows came back **byte-identical** — 145 detections, 80 TP, 9 FP,
ScoreV2 66.8. The challenger's backend configured the plugin with process-wide `os.Setenv`, and a
bridge host launches its child on FIRST USE, which is during the run and after every backend has
been constructed. So the baseline's plugin spawned inheriting ScreenParser's model.

This is the same defect `newShadowVision` records having been caught in the first live start —
fixed there, and not here. With per-child environment, `current` correctly reports UNAVAILABLE
and only the challenger produces numbers. Any benchmark run that constructed both backends was
affected.

### Three: the firewall admission would have needed did not exist

`Element.Actions()` derives capability from **role**:

```go
Invokable: roleInvokable(r)
```

and `Targetable()` asked only `Enabled && ClickableByBounds && Any()`.

So an element whose entire evidence was a learned detector classifying a rectangle as `button`
read as **legally targetable** — with nothing anywhere having claimed a mechanism to press it.
Unreported state defaults to usable, and any visible box is clickable-by-bounds, so a detection
needed nothing else.

That was safe only because ScreenParser is shadow-only. **A safety property that depends on an
experiment's configuration is not a safety property**, and it would have evaporated silently the
moment anyone did what this roadmap was asking about — with no existing test able to see it.

## The firewall

Affordance and capability are now different questions.

```
AFFORDANCE   the interface presents something operable        Actionability.Affords()
CAPABILITY   Marco has a mechanism able to operate it         Actionability.Actuable
TARGETABLE   both, plus enabled and somewhere to aim          Actionability.Targetable()
```

`Actuable` is decided from **provenance**, at the one place an element's evidence is in scope:

```go
func ActuatingSource(s ObservationSource) bool {
	switch s {
	case SourceNative, SourceDOM, SourceAccessibility, SourcePlugin:
		return true
	}
	return false
}
```

Accessibility, a native integration and a DOM bridge expose invoke/toggle/select patterns; a
plugin speaks for an application that can act on its own behalf. A vision detector and an OCR
reader describe pixels. The list is **explicit and short rather than a rank comparison**, because
a threshold would silently admit whatever source somebody adds next.

### It denies on evidence, never on absence

`Provenance.OnlyDescribesPixels()` is false for an element with **no** provenance at all. That is
deliberate and it was measured: the first attempt required a positive actuating source and broke
five tests across three packages — hand-built queries, capability-pack enrichment, fixtures.
"Nobody recorded where this came from" and "only a camera saw it" are different claims, and
conflating them would refuse every caller that constructs an element rather than observing one.

So the rule denies on **positive** evidence: every source is a pixel source, therefore nothing
here has claimed a mechanism. That is the exact shape of the risk, and nothing wider.

### And the refusal says which

Two very different windows answered `Blind()` identically — one with nothing in it, and one full
of controls that only a camera saw. Told "nothing in this window can be operated", a person
looking at a screen full of buttons would reasonably conclude their application was broken. The
policy gate now separates them:

> I can see controls here but nothing told me how to operate them — this window was described by
> the screen alone, with no accessibility information.

That is the diagnostic that would matter most on the day a detector *is* admitted.

## Why the answer is still no

Not because it cannot be measured, but because of what the measurement says.

**Cost.** 895–932ms median, ~1s p95, on this machine, over 39 frames. Ambient Observe samples at
1 second when active. Admitting this into the ordinary session cycle spends the entire budget on
one sensor — which is precisely why the provider already runs on its own cadence with
skip-never-queue, and why its own documentation records ~0.9s per frame and 1.25 GB resident.
That figure is now confirmed rather than quoted.

**The wrong workload.** The corpus is a game. The loop that matters — Observe, Learn, Perform,
recovery — runs against desktop applications, and the measurement says the detector is excellent
on menus and near-blind on in-game HUD. Windows Settings is a menu-shaped interface, so the
result is *encouraging*, but encouraging is not measured. Admission on evidence from a different
workload would be admission on a hunch wearing a table.

**A third of the structure is missed.** 63% structural recall, 44 false negatives and 56
detections matching nothing in the truth set. Useful, and not yet a reading to plan from.

**And no degraded case.** The scenario that would justify admission — accessibility returning
shell-only while the detector recovers real content — did not occur, and manufacturing it was
forbidden.

## What would be required to reconsider

1. **The same measurement on a desktop corpus** — Settings, Explorer, a browser — captured with
   coherent accessibility evidence beside each frame. This is the single most valuable next step
   and the reason the recommendation below is what it is.
2. **A bounded corpus of coherent readings** — the same window, same moment — carrying
   accessibility evidence, OCR evidence, the production fused world, and ScreenParser's
   detections.
3. **Match and conflict rates against healthy accessibility**: how many detections correspond to
   an element production already has, how many are unmatched, how many disagree on role.
4. **Stability**: detection count, class and geometry across repeated inference on an unchanged
   screen. Semantic churn on a static screen is disqualifying; geometry jitter is not.
5. **Evidence from a naturally degraded reading** — not a manufactured one — showing that
   detections in the opaque region correspond to real structure.
6. **Per-class calibration**, since a detector excellent at buttons and poor at containers should
   be admitted for buttons only.
7. **A cost that fits somewhere.** 895–932ms is the whole ambient budget. Either a cheaper model,
   a smaller input, an escalation policy that runs it rarely, or admission confined to a
   synchronous path where a second is affordable — Learn and verification can spend it; ambient
   watching cannot.

Even with all seven, admission should be **constrained**: geometry and presence before role, role
before structure, and **never actionability**. The firewall above now makes that last one
structural rather than a matter of care.

## What did NOT change

ScreenParser is still `ShadowOnly`; its evidence still reaches `Cycle.Shadow` and never fusion.
No second fusion engine, no visual graph, no visual store, no coordinate-actuation path, no
application-specific rule. The existing boundary tests still hold it —
`TestShadowEvidenceNeverReachesAdmittedObservations`,
`TestShadowOutputCannotChangeWhatIsBelieved`, `TestShadowCannotReachAnythingThatActs`,
`TestShadowProvenanceIsNotWaived` — and removing the `ShadowOnly` marker now fails to compile,
because the collector routes by interface satisfaction.

## KNOWN FOLLOW-ONS

1. **The firewall widens what "targetable" refuses, and that surface is not fully explored.** It
   was measured against the whole suite and against the policy and target packages specifically,
   and the only behaviour that changed was a refusal becoming more precise. A source that
   genuinely can operate a control and is missing from `ActuatingSource` would be refused, so the
   list is the thing to check when a provider is added.
2. **Performance remains unmeasured for the ordinary pipeline.** ScreenParser's own cost is now
   measured; UIA, capture, OCR and fusion timings exist on every sample as `Phases` and were not
   sampled. That debt is now two roadmaps old.
3. **Degraded-UIA repair remains UNMEASURED**, and no degraded reading occurred to analyse.
4. **The benchmark's other backends are unverified against the isolation fix.** Grounding DINO
   runs out of process and is unaffected; classical CV takes no model. Nothing else was checked.

## Enforced by

- `pkg/directorapi` — `TestVisualPresenceIsNotLegalActionability` (the firewall, both directions,
  including that a corroborating camera does not remove capability and that an element with no
  provenance is unchanged); `TestOnlySourcesThatCanOperateAControlSaySo`;
  `TestAWindowWithAccessibilityIsNotSeenOnlyByPixels`.
- `internal/director/policy` — `TestTheGateSaysWhenOnlyACameraSawIt`;
  `TestTheGateNamesWhatWasInsufficient`.
- `cmd/director` — `TestTheBenchmarkBaselineIsNotTheChallenger` (no process-wide detector
  configuration, so a benchmark cannot compare one model against itself).
- And, holding shadow-only, `internal/director/perception/providers` and
  `internal/director/perception/shadow` — the five existing boundary tests named above.

## Related

[[ADR-100-marco-sees-through-evidence]] ·
[[ADR-017-structure-earns-a-name-text-never-earns-structure]] ·
[[ADR-010-passive-observation-cannot-execute]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-005-legal-marco-only]] ·
[[Experiment-010-vision-structure-as-a-semantic-path]] ·
[[Experiment-015-screenparser-admission-measured]] ·
[[Vision]] · [[Fusion]]
