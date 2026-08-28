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
  - pkg/directorapi/actionability.go
  - pkg/directorapi/actionability_test.go
  - pkg/directorapi/observation.go
  - internal/director/policy/policy.go
  - internal/director/perception/shadow/provider.go
---

# ADR-101 — Visual presence is not legal actionability

## The decision

**`SCREENPARSER_DECISION = REMAIN_SHADOW_ONLY`**, because the evidence required to admit it
cannot be gathered on this machine — and, separately, because the safety property that admission
would have depended on **did not exist**. It does now.

## What was actually found

37B set out to measure whether ScreenParser's evidence has earned a seat at fusion. The audit
found two things, and the second changed the roadmap.

### One: the measurement cannot be taken here

ScreenParser is opt-in behind three environment variables and is `nil` on an ordinary Director —
"nothing loaded, nothing allocated, no reason to report". It needs:

| | |
|---|---|
| `$MARCO_SHADOW_VISION=screenparser` | unset |
| `$MARCO_SCREENPARSER_MODEL` | unset, but the model **is** on disk (`tools/vision-export/weights/screenparser-1280.onnx`, 97.4 MB) |
| `$MARCO_ONNXRUNTIME` | unset, and **no ONNX Runtime shared library is installed** |

Without the runtime the detector refuses before it starts, so the corpus comparison, the
match/conflict rates, the stability runs and the per-class calibration this roadmap asks for
have no way to produce a number. [[Experiment-010-vision-structure-as-a-semantic-path]] recorded
the same blocker on 2026-08-09 with the model *also* missing; half of it has since been resolved.

Fabricating those measurements was the one thing the roadmap forbade above all, so they are
reported UNMEASURED and the decision follows from that rather than from a guess.

### Two: the firewall admission would have needed did not exist

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

## What would be required to reconsider

Stated precisely, as the roadmap demands of a shadow-only outcome:

1. **An ONNX Runtime shared library on the measuring machine**, with `$MARCO_ONNXRUNTIME` set.
   Without it nothing below is possible.
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
7. **Cost**, against the ambient cadence. The provider's own documentation records **~0.9s per
   frame and 1.25 GB resident**, which is why it runs on a separate cadence with skip-never-queue
   rather than in the observation loop. Against Observe's 1s active cadence that is the whole
   budget, and it is the single most likely reason the answer stays no.

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
2. **Performance remains unmeasured for the ordinary pipeline.** The 0.9s/1.25 GB figure is
   ScreenParser's own, recorded in its provider. UIA, capture, OCR and fusion timings exist on
   every sample as `Phases` and were not sampled for this roadmap.
3. **Degraded-UIA repair remains UNMEASURED**, and no degraded reading occurred to analyse.

## Enforced by

- `pkg/directorapi` — `TestVisualPresenceIsNotLegalActionability` (the firewall, both directions,
  including that a corroborating camera does not remove capability and that an element with no
  provenance is unchanged); `TestOnlySourcesThatCanOperateAControlSaySo`;
  `TestAWindowWithAccessibilityIsNotSeenOnlyByPixels`.
- `internal/director/policy` — `TestTheGateSaysWhenOnlyACameraSawIt`;
  `TestTheGateNamesWhatWasInsufficient`.
- And, holding shadow-only, `internal/director/perception/providers` and
  `internal/director/perception/shadow` — the five existing boundary tests named above.

## Related

[[ADR-100-marco-sees-through-evidence]] ·
[[ADR-017-structure-earns-a-name-text-never-earns-structure]] ·
[[ADR-010-passive-observation-cannot-execute]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-005-legal-marco-only]] ·
[[Experiment-010-vision-structure-as-a-semantic-path]] ·
[[Vision]] · [[Fusion]]
