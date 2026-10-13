---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
source_paths:
  - internal/director/fastobserve/fastobserve.go
  - internal/director/observe/arrangement.go
  - cmd/director/observescreen.go
  - cmd/perceptionreplay/observer.go
---

# ADR-178 — the fast observer

## Context

Five decisions led here. [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] measured
that the session-local screen model needs about ten looks a second and that the AI discovery
path's ~3.8 seconds cannot support it with any threshold — not a different count, not a
duration. [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] then measured that a
full-resolution look is affordable: acquisition 16.7ms, conversion 2.1ms, proposer ~32ms.

The owner authorised the implementation:

> Implement the fast passive visual observer using the existing full-resolution Windows capture
> path. […] Target approximately 10 observations per second, independently of the slower
> AI/semantic observation cadence. **Measure actual sustained end-to-end throughput, latency,
> dropped observations, and CPU cost rather than assuming** the estimated ~50.7ms stage total is
> guaranteed.

## Decision

`internal/director/fastobserve` is the fast half of Marco's perception: it notices that the
screen **changed**, and what was clicked. The slow semantic path keeps its own cadence and says
what the screen **is**.

### Its cadence is its own, and it refuses to inherit a slow one

`observe.VisualSettleLooks = 3` was never really a count — it was a duration, about 200ms from
the first look of a run to its third at 10fps. So `SettleWindow` is 200ms, the observer takes its
interval and derives the count, and a replay at 10fps and a live observer at 10fps damp
identically because they are counting the same 200ms.

**And it refuses to start at a rate too coarse to damp.** At 3.8 seconds the arithmetic gives one
look, and one look is no damping at all: ADR-175 measured settle=1 at 113 promotions over 900
looks with 72 of the causes unknown. A refusal rather than a clamp, because a clamp would observe
at a rate whose results nobody should trust and say nothing about it — and silently inheriting
the AI cadence is the exact accident this package exists to prevent.

### Three ways a look is not evidence, counted apart

| | |
|---|---|
| **missing** | no pixels — a failed capture, a minimised window |
| **covered** | another window was in front. A capture by window returns the covering window's pixels under the target's own name, and Experiment-025 captured the Windows taskbar on one look of 107 with provenance clean |
| **interrupted** | the window's generation changed under the observer |

The first two are offered to the stabiliser as unobserved looks, which is what they are: one
should not drop a standing claim and a run of them should. The third is different in kind and
takes effect at once — `VisualScreens.Interrupt` gives up the claim **without recording a
transition**, so no edge can span a window being replaced. The screens already settled are kept,
so coming back to one is still a return.

### It never blocks

Publication is a non-blocking send to a bounded channel and a refused update is counted. An
observer that waited for a slow consumer would run at the slow consumer's rate, which is the
defect this package exists to fix arriving by a different door.

### It cannot act and cannot remember

Held by the build rather than by discipline. `TestTheFastObserverCannotActOrRemember` walks the
package's transitive imports and fails if it can reach the durable store, the execution pipeline,
the OS host, the recorder, or anything else that can touch a desktop.
`TestTheFastObserverNamesNothingDurable` reads the source and fails on `EstablishPlace`,
`StructureSignature` and the rest.

The session-local tier's allowlist grew to include this package — the second time, deliberately,
and with the test failing first. A widening of the TIER is not a widening of what may become
durable, which is why the list is worth keeping.

## Measured outcome

The production observer, folded over the 900-frame recording every offline conclusion came from:

```
  900 look(s), settle rule 3 at 100ms
  proposer     37.38ms a look
  observer      0.05ms a look   <-- the fold itself
  per look     median 92.25ms  p95 104.63ms  max 123.00ms
               (EXCLUDING the 16.7ms acquisition; INCLUDING a 54ms PNG decode
                a live observer never pays)
  so a live look is about 54.7ms — 18.3 a second
  9 screen(s), 21 transition(s), 13 click / 5 automatic / 3 unknown
```

**The fold itself costs 0.05ms.** The observer adds essentially nothing to the proposer, and the
~50ms estimate holds: **comfortably more than the ten looks a second the model needs.**

### And 18.3 was the optimistic end of one run

Re-folded twice more on 2026-10-12 while the press account below was added, the same 900 frames
gave a proposer of 43.3 to 46.1ms a look and a live look of **60.5 to 64.6ms — 15.5 to 16.5 a
second**, against the 37.4ms and 18.3/s recorded above. Same code, same recording, same machine.

The spread is this machine under varying load, and the honest reading is a range rather than a
figure: **a live look is 55–65ms here, so 15 to 18 a second.** Either end clears the ten the
model needs with room, which is why this changes no decision — but a single measurement quoted
to one decimal place reads as a precision nobody measured, and the next person to compare a live
session against it would be comparing against the best of three runs.

### It reproduces the offline conclusions exactly

Nine screens with the same look counts (416/57/56/46/8/150/39/15/2), the same settle points
(3/80/114/168/192/220/311/341/779), and the same 13/5/3 cause split as every decision from
ADR-171 onward. The live observer and the offline measurement are the same code on the same
arithmetic, and now that is a recorded fact rather than an intention — which is what makes "did
the live session see what the recording saw" a question with an answer.

### Backpressure, measured

The same 300 looks, once with a consumer that keeps up and once with one 300ms behind:

| | published | dropped | per look | screens | transitions |
|---|---|---|---|---|---|
| keeping up | 300 | 0 | 89.8ms → 18.6/s | 6 | 7 |
| 300ms behind | 123 | **177** | 91.3ms → **18.3/s** | 6 | 7 |

**The cadence does not move and the conclusions are identical.** A slow consumer costs a
consumer's view, never the observer's knowledge. That is the property the bounded queue exists
for, and it is measured rather than asserted.

The first attempt at this measured nothing: the "slow" consumer slept 2ms against a 92ms look and
dropped zero, which read as *the consumer kept up* when it meant *the consumer was never tested*.
Recorded because a green measurement of nothing is the most expensive kind.

Repeated over the whole recording rather than its first 300 looks, the result is stronger: **541
and 552 of 900 updates dropped, and the conclusions character for character identical** — nine
screens, the same look counts, the same settle points, 13/5/3, and all seventeen presses folded.
A consumer that saw two looks in five arrived at the same session.

## The mouse, accounted for end to end

The owner asked the right question about the live command before running it:

> it isn't entirely clear whether `observe-screen` subscribes to live mouse events. If it
> doesn't, we should expect automatic or unknown classifications rather than treating a lack of
> click-attributed transitions as a regression.

It does subscribe — `navsource.New`, a subscription opened after the delay, drained once a look,
`NavPoint` events folded as presses. But the question could not be answered **from the output**,
and that is the defect worth fixing before a bounded live run rather than after it.

A report of `0 click` has at least four causes, and they live in four different places:

| | |
|---|---|
| no hook was installed | `navsource` said why and nobody printed it |
| no press was placed inside the window | `unplaceable_pointer`, counted by the producer and not read |
| the press arrived on a look that was not evidence | discarded by the observer, counted **nowhere** |
| the press landed on nothing the proposer found | `Blind`, already counted |

Three of the four were invisible, and the third was a silent drop — a press real enough to have
happened and gone without a number, which is
[[ADR-169-a-recording-that-could-not-explain-its-own-total]] in a different wall. So:

- `fastobserve.Stats` gains **`Presses`**, the denominator, and **`PressesUnusable`**, the ones
  offered on a look with no geometry. A press on a missing or covered look has no candidate set
  and must not become evidence; discarding it is right, discarding it silently is not.
- `observe.ClickEvidence` gains **`Presses`**, what the correlator was actually given. The two
  accounts have to close: offered, less unusable, is what the correlator holds.
- The command reports the producer's own `InputStats` — received, placed, and the ignore reasons
  by name — and **says `NOT OBSERVED` with the reason** when no mouse could be watched, adding
  that the causes are then automatic or unknown *by construction*. An absence that is forced
  must not read as an absence that was observed.
- Keyboard events are counted as **seen and not used**. This path correlates presses only, and a
  keyboard-navigated session reporting "no input" is the controller mistake of ADR-169 exactly.

### And the frame is pushed before anything is watching

`navsource` can only place a press against bounds pushed **before** the press happened. The
observation loop pushed them, but not until after its first capture — so every press in the
first interval would have come back `unplaceable_pointer`: ADR-168's defect at one look's scale,
and the shape that once reported six live clicks as a perception failure. The rectangle is now
acquired and pushed before the subscription opens, re-acquired rather than reused from before
the delay, because an eight-second-old rectangle stamped `now` is a small lie the pointer path
would then trust.

Over the recording the account closes: **17 presses offered, 0 unusable, 17 folded, 0 blind, 5
ambiguous** — against a recording known to hold seventeen.

## One statement of the normalisation

The observer and the replay both turn the proposer's pixel rectangles into normalised regions.
Four divisions written twice is this repository's worst defect class — two statements of one rule,
drifting, with the measurement and the production path no longer the same arithmetic — so it is
`observe.NormaliseRects`, used by both, and `shapesFrom` was rewritten to call it.

It subtracts the image's own origin. A sub-image's bounds do not start at zero, and a
normalisation that assumed they did would place every region by however far the crop was offset:
plausible, and wrong by a whole window.

## Consequences

- **The fast observer exists and nothing in Director's live sampler calls it.** It is reached by
  `director observe-screen`, which the owner starts.
- **The three-cause model travels through it.** A published step carries the cause the correlator
  actually recorded, taken from `Arrived`'s return rather than looked up by endpoints — because
  two steps can share a source and destination and differ only in cause, and the cause is the
  field that decides whether anything may ever be pressed.
- **Nothing is durable and nothing is executable.** The actuation boundary is untouched.
- **18 looks a second is this machine, this window, this one recording.** It is not a claim about
  other applications, other resolutions, or shorter dialogs, and the owner's warning on that
  stands.
- **The nine-screens-for-five question is untouched**, deliberately. The observation architecture
  came first.

## The command, which has not been run

```sh
director observe-screen --application rocketleague --seconds 60 --fps 10 --delay 8s
```

Bounded by `--seconds` (≤300), refuses to start without `--application`, writes nothing, injects
nothing, remembers nothing. It prints the settle rule it derived so a reader can see what cadence
it actually damped at.

## Mutation gate

19 mutants, all 19 killed; the press account added 14 more, all killed. Three survivors needed a test that did not exist: at 10fps the derived
settle rule equals the default, so an observer that computed the rule and never applied it looked
correct at the one rate every offline measurement was taken at. The test now checks a 5fps
observer, whose rule is two, and asserts that two looks settle a screen there.

The press account's run found one real survivor and one bug in the harness. The survivor: a
mutant that folded a keyboard event in as a press passed, because the split lived inside the
observation loop where only a real window could reach it — so it is `splitNav` now, tested
directly, and a key carrying its zero position into a click at the window's origin is what the
test names. The harness bug was funnier and worth recording: it scored six kills as compile
errors, because it looked for the words "not used" in the output and the report it was testing
prints "seen and not used".

## Enforced by

- `internal/director/fastobserve` — `TestTheSettleRuleComesFromTheObserversOwnInterval`,
  `TestAnObserverTooSlowToDampRefusesToRun`, `TestASlowConsumerCostsUpdatesAndNotCadence`,
  `TestAWindowChangeDoesNotManufactureATransition`,
  `TestALookWithNoPixelsOrOfAnotherWindowIsNotAScreen`,
  `TestAnUnobservedLookSaysSoRatherThanRepeatingTheLastScreen`,
  `TestASettledChangePublishesItsCause`,
  `TestAPressOnThePromotingLookBelongsToTheScreenItLeft`,
  `TestTheFastObserverCannotActOrRemember`, `TestTheFastObserverNamesNothingDurable`,
  `TestThisPackageIsPartOfTheSessionLocalTier`,
  `TestAPressOnALookThatIsNotEvidenceIsCountedAndNotCorrelated`.
- `internal/director/observe` — `TestRectanglesAreNormalisedToTheImageTheyCameFrom`,
  `TestAVisualScreenCannotReachDurableMemory`.
- `cmd/director` — `TestTheScreenObserverIsReachableAndBounded`,
  `TestTheObserverReportAccountsForEveryPress`,
  `TestAKeyboardEventIsCountedAndNotTakenForAPress`.
