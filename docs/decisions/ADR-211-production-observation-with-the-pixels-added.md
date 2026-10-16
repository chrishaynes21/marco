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
  - cmd/director/surfacelearnphases.go
  - cmd/director/tryitout.go
---

# ADR-211 — production observation with the pixels added

## Context

Two configurations were tried on Windows Settings and neither could close the demonstration loop,
for opposite reasons — see
[[Experiment-063-the-configuration-that-cannot-close-its-own-loop]].

```
  production   a place establishes; every tie is a control beside its own near-identical
               wrapper, which no press can separate (see ADR-210)
  pixel-only   the ties are genuinely nested and separable; no place can be established at
               all, because a screen discriminates on interface terms and nothing credits
               one without accessibility
```

The owner's reading, and their decision:

> Augmented: Accessibility can establish identity while OCR and structure provide additional
> interaction-surface evidence. This is the combination Director was designed to use. … Approved:
> augmented observation for the bounded Try It Out Alpha experiment.

And the scope correction that goes with it:

> stop treating Windows Settings as the acceptance test itself. It's just a safe application for
> exercising Marco.

## Decision

**A third configuration: production observation with the shape proposer and OCR added.**

```
  Sources   (empty)              nothing is restricted
  Include   [structure ocr]      two opt-in providers are asked for by name
  providers every production provider, accessibility among them
```

**Additive, and that is the whole of why it is safe.** `Include` adds opt-in sources and removes
nothing, so accessibility is collected exactly as it always is, every downstream rule is the
production one, and production itself is unchanged because production does not ask — an empty
request includes nothing.

`Sources` stays **empty**, deliberately. Naming anything there would switch accessibility off by
implication, because `Request.Wants` answers false for every source a non-empty list omits. That
is how this would quietly become a second pixel-only mode wearing a friendlier name, and it is the
one mistake this ADR exists to prevent.

### The same door

`enableTestObservation` refuses outside a sandbox, through the same `resetGuard` the reset command
uses, for both test configurations. The augmented mode removes nothing and could defend itself as
harmless — and that argument is precisely how a second door ends up beside the first. It
establishes places and keeps demonstrations, so it belongs behind the one that can be emptied
afterwards.

### One value, not a flag

The mode is an `observationMode` threaded through every phase, after the first version used a
boolean that had grown a second meaning within a day of being written. Three states, one value,
and `pixelOnlyNote()` prints which one every phase ran under — because a result obtained under an
unusual configuration and reported without it is the kind of finding that gets quoted later as
though it were about production.

### Restricted to the harness

The owner's limit, carried as written:

> Keep the augmented configuration restricted to the disposable Try It Out harness until the
> results justify broader use.

## Consequences

**Unverified.** The only probe of it so far read a game window, because the foreground had moved.
Nothing about this configuration has been measured on a live Settings screen, and the owner's
instruction is explicit that it must not be assumed:

> Do not assume the augmented configuration succeeds until measured on the live Settings screen.

**And it is not the long-term design.** The owner was clear that this is alpha scaffolding:

> Ultimately, Try It Out should observe the real desktop using the best available evidence, not
> require the user to choose between accessibility, pixels, and augmented modes. … I would make
> the next milestone a unified observation strategy with bounded costs and clear evidence
> provenance — not another Settings-specific workaround.

## Enforced by

`cmd/director` — `TestTheAugmentedConfigurationRemovesNothing`,
`TestEveryTestConfigurationIsReachableOnlyFromASandbox`,
`TestAskingForProductionClearsTheTestConfiguration`,
`TestProductionObservationDefaultsAreUnchanged`. 8 of 8 mutants killed.

## Related

- [[ADR-208-one-guarded-test-configuration-and-no-second-one]] — the configuration this replaces
  for the experiment, and whose "no second one" rule this consciously extends by one.
- [[ADR-210-a-question-nobody-could-answer-is-not-asked]] — why production observation alone
  cannot exercise the feature.
- [[Experiment-063-the-configuration-that-cannot-close-its-own-loop]] — the measurements.
