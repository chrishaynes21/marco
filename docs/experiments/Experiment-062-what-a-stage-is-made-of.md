---
type: experiment
status: complete
date: 2026-10-16
backend:
  - structure
  - ocr
  - window_system
fixture: "the owner's live desktop, observed through the pixel-only test configuration of ADR-208 — the shape proposer and OCR, with accessibility neither requested nor registered"
result: the-first-live-run-produced-a-stage-with-no-window-and-no-props-at-all-because-the-accessibility-walk-is-the-only-thing-that-enumerates-windows-and-every-pixel-provider-is-scoped-to-one-so-removing-accessibility-removed-the-stage-rather-than-thinning-it-fixed-by-letting-the-window-system-enumerate-when-asked-and-measured-again-at-46-props
supersedes: []
affects:
  - perception
---

# Experiment 062 — what a Stage is made of

**Run 2026-10-16, on the owner's machine, read-only.** The first live invocation of the harness
from [[ADR-208-one-guarded-test-configuration-and-no-second-one]].

## The question

> Does the pixel-only test configuration produce a Stage?

Asked only because the configuration had been verified the other way round. `-check` asserted the
PROVIDER LIST the runtime actually built — `[window_system ocr vision structure]`, accessibility
absent — and that assertion was true and remains true. It says nothing about what that list can
see.

## What happened

```
  no   the instruction against the live Stage
        foreground ; 0 Props; "Bluetooth & devices" → insufficient evidence
        nothing was observed in this window, so there is nothing to press
```

An empty foreground and zero Props, with Windows Settings in front and 159 elements visible to an
ordinary `director inspect` at the same moment.

## Why

**The accessibility walk is the only thing that enumerates windows.** `providers.Accessibility`
emits `observation.Window` for each window in the bridge's snapshot and `observation.Application`
for the one in front; the window system provider is a REFINER, which adds bounds, state and
monitor placement to window evidence somebody else produced and reports nothing of its own. Its
own comment said so, and said what the fix was:

> the platform's window enumeration happens to live inside the accessibility bridge's walk, so
> this provider has nothing of its own to enumerate … When it does enumerate — the platform APIs
> are right there, it is a question of where the walk lives — it becomes an ordinary Provider.

And **every pixel provider is scoped to a window.** OCR and the shape proposer both begin with
`p.window(ctx, req)`, which is `Runtime.activeWindow`, which takes its candidate from the last
observed world. No window evidence means no candidate, which means `no window to look at`, which
means no observations, which means no window evidence next time either.

So removing accessibility did not thin the Stage. It removed the Stage. Not a defect in the
configuration — a fact about what a Stage is made of, which nothing had had occasion to discover
because no configuration had ever left accessibility out.

## What changed

**The window system enumerates when it is asked**, through `WindowEnumerator` — the optional half
of a window source, implemented by the platform backend whose `AllCandidates` already listed the
desktop for `director windows`, already carried bounds, title, image name and the foreground flag,
and already excluded Marco's own surfaces. One window observation per candidate, one application
observation for the one in front.

The gate is `Includes` rather than `Wants`, which is the gate
[[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] put on the shape proposer and
for the same reason: being permitted to run is not being asked to. So `observation.Request{}` —
every production cycle — gets exactly what it got before, which is nothing, and production
observation is unchanged.

**And the harness collects two cycles.** The first establishes the window; the second is the first
that can see inside it. That is what "the last observed world" has always meant, and it is
invisible whenever accessibility is collected because the walk reports a window and its controls
in the same breath.

## Measured again

```
  foreground chrome ("Marco Director - Google Chrome"); 46 Props
  46 of 46 Props were described by the shape proposer
```

A window, a foreground application, and forty-six proposed rectangles on a live screen with no
accessibility evidence anywhere in the cycle. The precondition that remains false is the ordinary
one: Settings was no longer in front, so the instruction had nothing to match.

## What this does not establish

No demonstration, no press, no click. The loop this Stage exists for has still never run live.

## Related

- [[ADR-208-one-guarded-test-configuration-and-no-second-one]] — the configuration whose first run
  this was.
- [[Experiment-061-the-live-focused-demonstration]] — the run this unblocks.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — the opt-in idiom the
  enumeration borrows.
