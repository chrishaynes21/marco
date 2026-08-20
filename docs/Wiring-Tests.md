---
type: guide
status: active
updated: 2026-08-07
source_paths:
  - internal/director/execute/binding_wiring_test.go
  - cmd/director/observelive_test.go
  - internal/director/perception/providers/collector_provenance_test.go
  - plugins/vision/vocabulary_test.go
  - cmd/director/benchsplit_test.go
---

# Implementation correctness and integration correctness are separate gates

> A subsystem is not complete when its implementation tests pass. At least one
> production-path test must prove the real runtime invokes it, and that its result affects
> behaviour.

> **Integration correctness requires invocation, consumption, and contract compatibility.**
> All three. Two out of three still ships a feature that does nothing.

## Why this rule exists

This codebase has shipped the same defect three times. Every time, every gate was green.

**1. The unpinned accessibility provider.** `observation.Request.Window` existed, every layer
honoured it, and it had tests. `liveSampler.request()` never set it. For three milestones a
targeted session silently observed the foreground window — usually the terminal running the
diagnostics — and the resulting baseline measurement was published before anyone noticed. See
[[director-accessibility-targeting]] and the superseded conclusion in
[[Experiment-002-dnfc-observation-baseline]].

**2. The target provenance layer.** `ProviderOutcome`, `TargetProvenance`, `TargetProven` and
`ObserveTargeted` were complete and thoroughly unit-tested. The collector still called
`Observe`, cycles carried no outcomes, and fusion read the flat observation list. The
mechanism was a well-tested no-op. See [[ADR-011-provenance-is-proven-not-assumed]].

**3. ScreenParser's two incompatible contracts.** The third instance is different in kind, and
it is the reason this note now names three requirements rather than one. The detector was
invoked, and its result *was* consumed — producer and consumer were genuinely connected. They
simply disagreed about what the values meant, in two independent ways at once:

| | Producer said | Consumer expected | Result |
|---|---|---|---|
| **Vocabulary** | `Button`, `Heading` | `button`, `text` | 14 detections, 14 refused as unknown classes |
| **Confidence** | 0.22–0.47, calibrated at 0.15 | ≥ 0.35 / ≥ 0.50 floors | every survivor discarded as low-confidence |

Both times the benchmark reported a working model as a failing one, and both times the number
was initially read as model performance. Grounding DINO had already produced the identical
symptom — a correct `menu` at 0.40 against a 0.50 floor, twelve of thirteen detections
discarded, and the report blamed the model ([[Experiment-001-vision-backend-comparison]]).

A reachability check passes here. So does a "did the consumer receive something" check. What
fails is quieter: the consumer received data it could not interpret and silently dropped it,
which is indistinguishable from the producer finding nothing.

None of the three was a bug in a mechanism. Two were a missing *call*; the third was a missing
*agreement*. A unit test cannot see either: it supplies the inputs production was supposed to
supply, in the representation the code under test already expects, then verifies the code did
the right thing with them.

### The contract-compatibility corollary

Where two components meet, one side owns the translation and that ownership is written down.
For vision, the rule is: **a model's own vocabulary ends at the plugin boundary**
(`plugins/vision/vocabulary.go`), and everything above speaks Marco's closed set. For
confidence, a backend declares its own acceptance floors (`visionbench.Calibrated`), because
confidence scales are not comparable between models and judging one model at another's floors
measures calibration rather than usefulness.

The test that catches it asserts on **semantics, not delivery**: not "the consumer got N
items", but "the consumer got items it could act on". `TestSequenceModeChangesTheResult` is the
same shape applied to a benchmark — it proves the declaration changes the number, rather than
that the declaration was read.

## What the failure looks like

It is not a dead package. A package-level reachability check over this repository finds every
`internal/director/*` package in the production binary's dependency graph — including, at the
time it was suspected of being dead, `binding`. Linking is not calling.

The real shape is narrower, and always one of:

- a new field the production caller never populates (`Request.Window`, `Request.Target`);
- a new method beside an old one, with production still on the old path
  (`Observe` / `ObserveTargeted`);
- a guard whose caller reads pre-guard data;
- a result computed and then discarded;
- **a result delivered but not understood** — producer and consumer connected, disagreeing on
  vocabulary, units, or scale, with the consumer dropping what it cannot parse;
- **a declaration read but not acted on** — data loaded into a field nothing branches on, which
  reads as configured behaviour while being none;
- **a test that performs production's step itself.** Every binding test began
  `ctx := ensureBindings(context.Background())` — the fixture installing the store the
  pipeline is responsible for installing. The tests would have passed had production stopped.
- **a call made with the wrong key, on a field only the fixture fills.** The demonstration
  arming read its application name from `cfg.Selector.Application`. A selector names a *window* —
  by ephemeral id, by title, by process — and only the `--application` form carries an application
  at all, so the lookup was `Topology("")` for every session that chose its window any other way,
  including the foreground one a person actually uses. Every fixture in the package built
  `Selector{Application: "testgame"}`, so the call site was exercised on every run, with an
  argument production could never supply. Found live, by a user demonstrating a route four times
  and being told "I wasn't watching for your example". See
  [[ADR-050-a-session-is-keyed-by-the-window-it-resolved]].

The last two are the most dangerous, because the test looks like an integration test. Note the
difference from the earlier shapes: this one is not uncalled code. The line ran, every time. What
was untested was the *value* it ran with — a distinction a coverage tool cannot draw, and the
reason the rule below is about mutating the connection rather than reaching the line.

## What a wiring test must do

Enter through the **production entry point** — the function the service actually calls — and
supply only what a real caller supplies. Then assert the mechanism's result changed the
outcome.

```go
// Not this: the fixture did production's job.
ctx := ensureBindings(context.Background())
out := pipeline.handleParsed(ctx, ...)

// This: a bare context, the production entry, and an assertion about the RESULT.
out := pipeline.HandleProgram(context.Background(), "rename this file to Budget")
if mentionsMissingStore(out.Message) { t.Fatal(...) }
```

Asserting that a constructor ran, or that a method was called, is not enough. Assert that the
mechanism **materially affected** what the product did — the host acted on the bound object,
the stale evidence never became belief, the request carried the generation.

## The gate: mutate the connection, not the mechanism

A wiring test is only real if a compiling mutation that disconnects production from the
mechanism — while leaving the mechanism untouched — fails the suite.

Worked examples from this repository:

| Mutation | Mechanism touched | Caught by |
|---|---|---|
| `cycle.Admitted()` → raw observation list | no | `TestEvidenceFromAReplacedWindowNeverBecomesBelief` |
| `Tracker.Confirm` → `Tracker.Current` | no | `TestExpectedAndObservedComeFromDifferentPaths` |
| drop `out.Target = expectedTarget(...)` | no | `TestTheSamplerPinsTheGenerationAndNotOnlyTheWindow` |
| drop **all** `ensureBindings(ctx)` | no | `TestProductionInstallsItsOwnBindingStore` |
| `labelFor` returns the model's native class | no | `TestVocabularyIsNormalisedAtThePluginBoundary` |
| `EvaluateTruthModes` ignores the modes map | no | `TestSequenceModeChangesTheResult` |
| every `SequenceTruth.Boundary` → 0 | no | `TestTransitionBoundaryIsLoadBearing` |
| `SequenceMode.Transitional()` → `false` | no | `TestCorpusMirrorSequencesScoreAlike` |

Note the `ensureBindings` row says **all**. Disconnecting one of the five call sites left the suite green,
because the other four still installed a store — redundancy, not protection. A mutation that
survives is telling you the connection is not load-bearing *at that site*; decide deliberately
whether that is fine or a hole.

## When to apply it

Whenever a subsystem is declared done, and specifically before writing "implemented and
tested" in a roadmap entry. The [[Roadmap]] carried "safe bindings are implemented and
unit-tested but **not wired into the execution path**" for several milestones after binding
had in fact been wired — the claim was never re-checked, because nothing failed when it went
stale.

Both directions of that error are cheap to prevent and expensive to leave: a wired mechanism
recorded as dead wastes a milestone re-doing it; a dead mechanism recorded as wired ships a
feature that does nothing.

## Related

- [[Architecture]] — the layering these tests protect
- [[Decisions]], [[ADR-011-provenance-is-proven-not-assumed]]
- [[AI-CONTEXT]] — how to retrieve this while working
- [[Roadmap]]
