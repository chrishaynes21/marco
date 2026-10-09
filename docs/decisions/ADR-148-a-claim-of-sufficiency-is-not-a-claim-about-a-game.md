---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
  - perception
source_paths:
  - cmd/director/shadowwiring.go
  - cmd/director/shadowsample.go
  - internal/director/perception/shadow/provider.go
  - internal/director/observe/shadow.go
---

# ADR-148 — a claim of sufficiency is not a claim about a game

## Context

[[ADR-147-the-interface-in-front-of-a-session-is-a-shadow-belief]] put the interface belief on
the live observation path. The first live run against a real game — Trackmania, through
`director observe-game` and the production sampler — produced **one inference in seventy-eight
looks**, and the session report called the other seventy-seven cadence skips.

They were not cadence skips. The experiment is gated on
`Runtime.moreEvidenceIsWorthBuying`, which declines an inference on a positive statement that
the current reading suffices. That gate is right, and measured: over six desktop moments
ScreenParser added no actionable item to a healthy accessibility reading, at 645–1379ms a
frame.

On a game it is the wrong answer to the right question. Accessibility exposes a window and a
pane, `SufficiencyOf` reports that as content reached, and the one sensor that can see the menu
is declined for the rest of the epoch. The claim being made is *"accessibility described this
window"*. The question being asked is *"can anything see which entry is highlighted"*. They are
not the same question, and
[[ADR-102-a-detector-earns-its-place-where-perception-fails]] is about the ABSENCE of better
evidence — which a claim of structural sufficiency does not establish on a surface that has no
accessibility tree to be sufficient about.

## Decision

**Three things, and deliberately not a fourth.**

### A declined slot says it was declined

There was one skip reason. Both the cadence gate and the purchase policy emitted *"shadow slot
skipped: cadence 2s"*, so every slot the policy declined was counted, reported and read as the
gate pacing itself. The two have opposite remedies — one is a gate working, the other is an
experiment switched off by a claim about different evidence — and nothing downstream could tell
them apart. `shadow.DeclinedReason` is now a constant, the sample carries `Declined`, the totals
count it, and the report prints it. *"All skipped by cadence"* was a sentence that was always
wrong about the interesting case.

### Every sample says which purchase policy produced it

The same detector on the same screen differs by two orders of magnitude between the two
policies. A run whose policy nobody recorded is a measurement of an unknown system, so the
description travels on every sample and is printed with the numbers.

It is taken from the **provider** — `shadow.Provider.Gated()` — and not from the environment.
Asking the environment describes the configuration somebody intended; asking the provider
describes the system that produced the numbers, and one line missing at the composition root
makes those different systems. This way a Director that never attached its policy reports
UNGATED, loudly, on every ordinary desktop session, instead of quietly attributing an ungated
run to the default.

That matters because that one line is the one thing here a test cannot reach: it lives in
`NewRuntime`, which cannot build a shadow provider without a model and a plugin on disk. The
mutation that deletes it survives the gate, knowingly. What has changed is that its effect is
now self-reporting rather than silent.

### `$MARCO_SHADOW_UNGATED` runs the experiment anyway

Off by default, in the same family as `$MARCO_SHADOW_VISION` and `$MARCO_SHADOW_CADENCE`, and
for the reason given in `shadowwiring.go`: the activation surface for an experiment is one a
person sets when they start the service they are experimenting with. With it set, the live run
went from 1 inference in 78 looks to **130 in 322**, and the interface belief received 130 real
looks instead of one.

### What is NOT decided

**The policy is not changed.** Buying an inference every cadence slot for as long as a game is
in front of somebody is a real cost — ~2.2s an inference, measured live on this machine — and
the right rule for a surface with no accessibility tree is a product decision that should be
made on evidence rather than on this argument. This ADR makes the evidence collectable. The
candidate rule, when it is written, is likely to be narrower than "always": a standing question
— *is something handing a mark around* — cannot be answered by one purchase, and that is a
different shape from the two needs the policy already models.

## Enforced by

- `TestADeclinedSlotSaysItWasDeclinedAndNotPaced`, `TestACadenceSkipStillSaysCadence`
  (`internal/director/perception/shadow`) — both sides of the reason.
- `TestTheSampleRecordsWhyTheSlotWasNotTaken` (`cmd/director`) — the join from the provider's
  free-text reason onto the sample, which is the kind that rots silently.
- `TestADeclinedSlotIsNotReportedAsCadencePacing`, `TestACadenceSkipIsNotReportedAsADecline` —
  the report, both ways round, so the fix cannot be "say declined for everything".
- `TestTheExperimentIsGatedUnlessTheSwitchIsSet`, `TestTheSwitchRemovesTheGateAndSaysSo` — the
  default, and that an off word means off.
- `TestTheSwitchDecidesWhetherThePolicyIsAttached` — the switch reaching the provider, probed
  with a sentinel in the slot the policy would take. The sentence in the report and the
  behaviour of the detector are two different claims and only the second is why it exists.
- `TestTheGateNoteTravelsWithTheSample`, `TestTheReportSaysWhichPurchasePolicyItRanBehind` —
  the policy arriving wherever its numbers do, including that a provider nobody gated says so
  rather than describing the default.
