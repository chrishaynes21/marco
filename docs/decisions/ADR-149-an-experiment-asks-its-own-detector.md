---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
  - perception
source_paths:
  - internal/director/perception/shadow/provider.go
  - internal/director/observe/shadow.go
  - internal/director/observe/structure.go
  - cmd/director/shadowsample.go
  - cmd/director/shadowtrace.go
  - cmd/director/observecmd.go
---

# ADR-149 — an experiment asks its own detector

## Context

[[ADR-148-a-claim-of-sufficiency-is-not-a-claim-about-a-game]] added `$MARCO_SHADOW_UNGATED` so
the detector could be run on a surface whose primary reading claims to be sufficient, and
recorded the result as **130 inferences in 322 samples**, against 1 in 78 with the gate on.

That number was wrong, and the switch was not doing what its own report said. The first live run
through the fixed belief reported this:

```
inferences   122 of 297 slots   skipped 175   failures 0
provenance   116 inference(s) could not prove they observed the session's window generation
detections   28 total
```

116 of those 122 had **latency 0ms and zero detections**. Nothing ran. The vision provider is
opt-in on its own source — it looks at nothing unless the request NAMES vision — and the request
the shadow provider hands it is the authoritative cycle's, whose `Include` list is built by the
same purchase policy:

```go
// cmd/director/observewiring.go
out := observation.Request{}
if buy := s.rt.moreEvidenceIsWorthBuying(); buy.Buy {
	out = observation.WithVision(nil)
}
```

So there were **two gates in series**, and `$MARCO_SHADOW_UNGATED` removed only the first. The
provider claimed its cadence slot, called its detector, and the detector returned instantly
having been asked for nothing. The experiment reported that it ran 122 times. It ran six.

The report said `UNGATED` throughout. A configuration switch that states the opposite of what
the system is doing is worse than one that does nothing, and this one produced a published
measurement.

## Decision

### The experiment asks for what the experiment needs

`shadow.Provider` widens the request it hands its inner provider to include its inner provider's
own `Sources()`. Its activation is its own: the cadence it was built with, and the purchase
policy attached to it. The authoritative cycle's source set is a decision about the
AUTHORITATIVE sensors and was never meant to be a third gate on this one.

It widens a **copy**. That is not a detail — the authoritative vision provider answers to the
same source name and its detections do reach fusion, so a version of this that mutated the
caller's request would turn a diagnostic switch into a change in what Marco believes. The test
for it checks the caller's backing array past its own length, because `append` to a full slice
allocates and the mistake would otherwise hide.

Measured on the same game, same machine, immediately after:

| | before | after |
| --- | --- | --- |
| real inferences | 6 of 122 claimed | 55 of 55 |
| provenance failures | 116 | 0 |
| uncomparable comparisons | 116 | 0 |
| detections | 28 in 180s | 316 in 240s, 268 nameable |

54 of 62 detections in the first verification run matched nothing accessibility reported, 44 of
them nameable. That is the premise of the whole experiment and it had never been measured.

### A provider nobody asked did not run

`StateNotRequested` carries no observations, no provenance and no timing, and every one of those
absences was being read further down as a finding about the SCREEN. It reached the sample as
`Ran=true, TargetProven=false` — an inference that happened and could not say which window it
described — which is how a session with six inferences reported 116 provenance failures.

So it is its own fact on the sample, counted apart in the totals, named in the report, named in
the structural view, and named in the per-slot trace. Three findings that looked identical now
read differently: the detector was never asked, the detector ran and found nothing, the detector
ran and its evidence was disqualified.

### A look that was disqualified is not an empty screen

Same rule as [[ADR-006-unknown-is-not-false]], one layer up from where the previous milestone
applied it. The interface belief folded both an unasked slot and an unproven inference in as
evidence, and so reported *"no shaded regions were proposed this look"* — a statement about
PIXELS — for three minutes in front of a menu, on slots that had no pixels.

Both now leave the belief stale with the reason, and unproven regions can no longer move the
selection. The second half matters more than the first: the regions on an unproven look are real
detections of a real window, just possibly not this one, and adopting them would move the
selection to another window's mark and report it as this interface's with a sample number to
cite.

### What is NOT decided

**The shadow inference blocks the observation cycle.** The type's own note says it "runs on its
own cadence and never blocks". It never appeared to because it was never running: with the
detector actually asked, the authoritative loop fell from 297 samples in 180s to 55 in 240s, and
`samples_late` rose accordingly. Making it asynchronous would break the one-cycle comparison
invariant in `shadowSampleFor` — *"a slow inference whose result arrived late is not a
disagreement about the interface, it is a disagreement about time"* — so the cost is reported
and the design is left alone.

**The navigation producer cannot see a gamepad.** `navsource` installs `WH_KEYBOARD_LL` and
`WH_MOUSE_LL`. Over four minutes with a game in front of a player it reported `received: 0` and
`unavailable: ""` — "navigation observation is working, nobody pressed anything" — which is a
sentence it cannot support about a controller. 40 screen-state transitions were observed and all
40 were unattributed. That is the floor under any milestone about associating input with
transitions, and adding a gamepad source is a new platform surface rather than a repair.

## Enforced by

- `TestTheExperimentAsksItsOwnDetectorForPixels`
  (`internal/director/perception/shadow`) — the gate that was invisible.
- `TestAskingForPixelsDoesNotWidenTheAuthoritativeRequest` — and that it cannot reach fusion,
  checked against the caller's backing array rather than its length.
- `TestAskingForPixelsDoesNotDefeatTheGate` — the provider's own purchase policy still decides.
- `TestADetectorNobodyAskedIsNotAnInference` (`cmd/director`) — `StateNotRequested` is not a run,
  not a provenance failure, and counted apart.
- `TestADetectorNobodyAskedIsNotAnEmptyScreen`, `TestAnUnprovenLookIsNotAnEmptyScreen`,
  `TestUnprovenPixelsDoNotMoveTheSelection`, `TestANeverAskedSlotIsNotACadenceSkip`
  (`internal/director/observe`) — the belief, both refusals, and the direction that matters.
- `TestTheStructuralViewSaysWhichKindOfAbsence`,
  `TestTheTraceSaysASkippedSlotWasNeverAsked`,
  `TestTheReportSaysHowManySlotsWereNeverAsked`,
  `TestASessionNobodyAskedSaysSoRatherThanSayingCadence` — the four diagnostic surfaces, so the
  cause stays findable rather than being found once.
