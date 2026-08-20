---
type: experiment
status: superseded
date: 2026-08-05
superseded_date: 2026-08-06
backend:
  - icon-detect
game: dnfc
result: not-suitable-as-default
supersedes: []
superseded_by:
  - director-accessibility-targeting
source_paths:
  - internal/director/observe/metrics.go
  - internal/director/observesession
---

# Experiment 002 — DNFC passive observation baseline

> ## ⚠ Superseded finding
>
> **Original conclusion:** a targeted observation session is vision-only *by
> construction*, because the accessibility provider is not pinned to the session's
> window.
>
> **Later finding (2026-08-06):** the cause was a single missing assignment —
> `liveSampler.request()` never set `observation.Request.Window`, so accessibility fell
> back to the foreground window. Every layer of the targeting mechanism already existed
> and worked. See [[director-accessibility-targeting]].
>
> **Corrected interpretation:** accessibility-rich applications provide substantial
> targeted structural evidence once the existing plumbing is used. A re-run against VS
> Code, with the terminal foregrounded throughout, produced **355 accessibility
> observations → 353 fused elements → 279 stable entities**, including readable
> `button` and `tab` labels. The prior calibration that produced "46 entities, all
> `role=icon`" was measuring the terminal, not VS Code.
>
> **What still stands:** the DNFC numbers below are honest measurements of
> `icon_detect` alone on a game that exposes little accessibility. What does not stand
> is the generalisation from them to *all* targeted sessions.
>
> This record is kept rather than rewritten. The wrong conclusion was reached, acted on
> for three milestones, and is part of the evidence.

A real three-minute passive observation session, 52 samples, scored by
`director benchmark-vision <session>` with no game running.

## Result

```
stable entities            1        unstable  139
anonymous ratio           70%       (want under 60%)
structural roles           1%       (want 25%+)
safe-label opportunity     0%       (want 15%+)
flicker                  0.30       (want under 0.15)
transition utility        36%
vocabulary          icon=129 grid=10 pane=1
→ NOT suitable as a default: fails 5 of 7 thresholds
```

## The defect that limits what this measures — *and the later correction*

Calibrating against VS Code — which has a rich accessibility tree — produced 46 entities,
**all of them `role=icon`**. Not one accessibility element reached the sample.

The diagnosis at the time: the sampler pins the validated window for vision and OCR, which
read `Runtime.activeWindow`, while the accessibility provider "has its own notion of the
active window and is not pinned". From that, this record concluded that a targeted session
is **vision-only by construction**.

**That diagnosis was half right and the conclusion was wrong.** The symptom was real —
accessibility genuinely observed the foreground terminal — but the cause was not
architectural. `liveSampler.request()` returned `observation.WithVision(nil)` and never set
`Request.Window`; the provider, the client and the C# bridge all honoured that field
correctly the whole time. One line fixed it. See [[director-accessibility-targeting]] for
the full audit.

So the DNFC numbers above remain honest about `icon_detect` alone on a game with little
accessibility. What must not be carried forward is the generalisation: combined perception
was never measured here, and it was measurable all along.

## A crash this session caused

`fatal error: too many callback functions`. `syscall.NewCallback` allocates from a fixed
process-wide table Go never frees. `Monitors()` had leaked one per call harmlessly for a long
time, because it was rarely called — then `onScreen()` began calling it **per window** inside
`LiveWindows()`, and a session sampling every two seconds exhausted the table and killed the
service in under three minutes.

Fixed with `sync.Once` for both callbacks, and the monitor layout read once per enumeration.
`internal/winctx/callback_windows_test.go` runs 4,500 enumerations — a regression crashes the
test binary, which is the right severity for "the process dies". See [[Windows]].

## What this implies

The 70% anonymous ratio and 1% structural-role coverage are the same finding as
[[Experiment-001-vision-backend-comparison]] arrived at from the other direction: one class,
and that class is not nameable.

## Blocked by

- The accessibility provider is not pinned to the session's window — see [[Perception]] and
  the second Known gap in [[Windows]].

## Related

- [[Passive-Observation]], [[Vision]], [[Windows]]
- [[ADR-010-passive-observation-cannot-execute]]
- [[Experiment-001-vision-backend-comparison]]
