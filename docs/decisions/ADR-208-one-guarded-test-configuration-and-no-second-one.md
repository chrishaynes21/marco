---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - perception
  - execution
source_paths:
  - cmd/director/pixelonly.go
  - cmd/director/surfacelive.go
  - cmd/director/surfacelearnphases.go
  - cmd/director/runtime.go
---

# ADR-208 — one guarded test configuration, and no second one

## Context

[[Experiment-060-the-preconditions-for-one-live-demonstration]] found the live focused
demonstration unreachable on Windows Settings, for a reason no amount of hands fixes. A surface
question exists only where identity and interaction surface have come apart, which is where
nothing but pixels describes the control — and Settings is XAML with a full accessibility tree, so
`click Bluetooth & devices` resolves to a real control and never asks. The experiment needs a
surface where the two come apart; the only application the authorisation permitted is one where
they do not.

The owner chose the configuration rather than the redesign:

> Approve option 3: Windows Settings in a pixel-only test configuration. … This is a test
> configuration decision, not a reason to redesign Director. Keep pixel-only mode isolated to the
> test harness. Do not disable accessibility globally or change normal production observation
> defaults.

## Decision

**One configuration exists, it is reachable only from a sandbox, and it changes what is OBSERVED
rather than what is ALLOWED.**

```
  Sources   [window_system structure ocr]        accessibility is never asked for
  providers accessibility is not registered      so it cannot be fallen back to
  rules     resolver, ADR-202 licence, policy, ladder, Marco runtime, foreground
            guards, cancellation — all unchanged, all the ones production uses
```

Two mechanisms for one exclusion, deliberately. `Request.Wants` answers false for anything the
`Sources` list omits, which is the system's own way of saying "not asked for"; leaving the
provider unregistered as well makes a silent fall back structurally impossible rather than merely
unrequested. Measured on the real machine: `[window_system ocr vision structure]`.

### The door, and why a package variable is safe behind it

`pixelOnly` is a package variable in `cmd/director`, because the provider list and the observation
request are both built inside `NewRuntime`, which fifteen callers share — a parameter would widen
every one of them for a flag one command may set.

What makes it safe is not the comment but the door. `enablePixelOnly` is the only way to set it
and it **refuses unless `MARCO_HOME` names a throwaway directory**, through the same `resetGuard`
the reset command uses. A Director started normally has no `MARCO_HOME`, so the configuration is
unreachable; pointing `MARCO_HOME` at the real store is not a declaration of anything.

That ties two things together on purpose: the configuration that produces an unusual Stage and the
store that can be emptied afterwards are the same decision rather than two.

**Nil is the production default**, so forgetting to consult the variable is a Director that
behaves normally rather than one that behaves oddly.

### The phases are separate commands because that is a safety property

`-check` and `-reground` and `-learn` build a DRY RUN runtime — the only host that could reach the
desktop is `runtime.DryRunHost{}`, which is a property of the object graph rather than a promise —
and `-execute` is the one phase that builds a live one, needs its own `-authorise-one-click`, and
performs exactly one click. Nothing retries: every refusal returns, says why, and stops.

### And the live screen is asked before anything is demonstrated

> Verify that the live Stage actually contains the nested caption/row candidates before asking for
> a demonstration. Don't assume the recorded fixture predicts the live screen.

So `-learn` evaluates the live Stage first and **stops if there is no surface question**, reporting
that as the result:

> this screen does not produce a surface question, so there is nothing to demonstrate. That is the
> result, not a failure — and the answer is to find a screen where identity and interaction
> surface really do come apart, never to loosen what counts as one.

## Consequences

**Six of eight preconditions now hold on the real machine**, verified by `-check`: the disposable
learning record, the configuration, the bridge, the registered provider list, the installed input
hook, durable memory, and both ADR-207 seams. The two that remain need a hand and a screen, and
are what `-watch` and `-instruction` measure.

**Nothing about production observation changed.** `pixelOnlyRequest()` returns
`observation.Request{}` when the variable is nil, which is what every production cycle has always
sent.

**This is the only test configuration, and there should not be a second.** A second one would
make "which configuration was that result obtained under" a question a reader has to ask, which is
why every phase of the harness prints `pixelOnlyNote()` before it does anything.

## Enforced by

`cmd/director` — `TestPixelOnlyCannotBeEnabledOutsideASandbox`,
`TestProductionObservationDefaultsAreUnchanged`,
`TestThePixelOnlyConfigurationLeavesAccessibilityOut`,
`TestThePixelOnlyRuntimeRegistersNoAccessibility`,
`TestADemonstrationIsNeverLearnedIntoTheRealStore`,
`TestAFalsePreconditionStopsTheExperiment`,
`TestAnUnmeasuredPreconditionIsNeitherPassNorFail`, `TestTheInstructionsTargetPhrase`.

## Related

- [[Experiment-060-the-preconditions-for-one-live-demonstration]] — the blocker this answers.
- [[Experiment-061-the-live-focused-demonstration]] — the run this configuration exists for.
- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the wiring the
  harness drives.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — why the shape proposer is
  opt-in in the first place.
