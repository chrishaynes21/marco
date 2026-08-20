---
type: milestone
status: complete
date: 2026-08-09
affects:
  - navigation
  - passive-observation
source_paths:
  - internal/platform/navsource
  - internal/director/observe/input.go
  - internal/director/observe/screenstate.go
  - cmd/director/observewiring.go
  - cmd/director/shadowtrace.go
---

# The live navigation producer, proven without a live session

Canonical notes: [[Navigation]], [[ADR-013-navigation-is-meaning-not-keys]]. This is the record
of what was built and what it cost, including the two things that were wrong.

## What this milestone was

The correlation layer already existed: closed intent vocabulary, session-local input evidence,
per-transition support, dominant intent, unattributed counts. What did not exist was proof that
any of it was **reachable from a running Director**, and the deliberate constraint was to
establish that with deterministic tests, replay and fake platform sources rather than with
another Rocket League session.

The governing rule, stated at the outset and worth keeping: *use live play to confirm a finished
mechanism; do not use stressful live play as the primary debugger.*

## The existing input infrastructure, audited before writing anything

Three low-level keyboard hooks already exist in this repository, and none of them was reusable:

| hook | process | purpose |
|---|---|---|
| `internal/recorder/recorder_windows.go` | `marco.exe` | keyboard + mouse, during a demonstration |
| `plugins/overlay/controller_windows.go` | `overlay.exe` | the leader key and F12 |
| `internal/platform/navsource/navsource_windows.go` | `director.exe` | this one |

They live in **different processes**, so this is not duplication: Windows supports multiple
low-level hooks from different processes, and the constraint that matters is per-callback
latency rather than global uniqueness. There is no XInput, GameInput, RawInput or gamepad polling
anywhere in the repository, which is what settled the controller question below.

Everything shares one requirement, documented in `CLAUDE.md` and honoured structurally here:
the callback does a non-blocking enqueue and returns. Windows silently drops a hook that
overruns its timeout, and the failure is invisible — no error, no log, F12 simply stops working.

## Two defects, both of the same kind

**The composition root never subscribed, and nothing noticed.** Deleting
`navSource.Open(...)` from `Runtime.newObservationSampler` left every navigation test in the
repository green. The earlier wiring test built its own sampler holding its own subscription, so
it proved the producer, the observe path and the correlation — and said nothing about whether
the Director ever subscribed. A shipped build would have installed the hook, classified the
player's intents, discarded every one of them, and reported a graph with no attributed edges,
which reads exactly like a player who pressed nothing.

This is [[Wiring-Tests]]'s failure for the third recorded time. Closed by
`TestTheCompositionRootSubscribesTheSessionToNavigation`, which enters through the production
constructor and calls the real `Sample`.

**The drop counter was wired to nothing.** The Windows backend counted refused offers into a
package-level atomic; `Stats().Dropped` read a different field that only dead code ever wrote;
and no report called `Stats()` at all. Backpressure was therefore unobservable, in a design whose
whole justification for dropping events is that dropping is safer than blocking. The counter now
lives on the `Source` (an atomic, incremented inside `offer`, because that call is on the hook
thread), and the counters ride the sample into the session totals and out through both reports.

Both were found by asking "what happens if I delete this", which is the only question that
distinguishes a wired mechanism from a complete one.

## What changed

- **Producer diagnostics reach a human.** `observe.InputStats` — received, classified, refused by
  reason, dropped, unavailable — rides the shadow sample and is rendered under `navigation` in
  the live report and in `director shadow-trace`. It is printed even when nothing was classified,
  because that is the case where it matters: an empty correlation has two explanations that call
  for opposite responses.
- **The ignore-reason vocabulary moved to the consumer** (`observe.IgnoreReason`). It crosses the
  privacy boundary, so the Director defines what may arrive; a producer can no longer invent a
  diagnostic string, which is the obvious route by which a key name would eventually travel.
- **Edge-local order is retained.** `ScreenTransition.Sequences` keeps the ordered runs that
  preceded a change, bounded and deduplicated by shape. `down, down, confirm` and
  `confirm, down, down` were previously identical evidence; the difference is not recoverable
  after the fact, so the decision had to be made here or not at all.
- **Traces carry navigation.** `tracedSlot` and `shadowreplay.Slot` gained `Inputs` and
  `InputStats`, recorded on **every** slot including skipped ones. Production/replay agreement
  had been established for geometry ([[Experiment-007-state-relative-tracking]]) and silently did
  not hold for attribution.
- **One timebase.** The subscription is stamped from the session's own clock rather than
  `time.Now()`. The drift is invisible in production, where both are the wall clock — which is
  precisely why it needed fixing before something depended on it.

## The mutation gate

Every new mechanism was deleted in turn, and the test that should have died, died:

| mutation | test that fails |
|---|---|
| remove `navSource.Open` from the composition root | `TestTheCompositionRootSubscribesTheSessionToNavigation` |
| stop recording `Inputs` into the trace | `TestACapturedTraceReplaysToTheSameAttributedGraph`, `TestATraceRecordsNavigationOnSkippedSlotsToo` |
| drop the ordered sequence, keep the intent set | `TestTheOrderOfNavigationSurvivesOnTheEdge`, `TestARecurringOrderIsCountedOnce`, `TestAScriptedInteractionProducesAnAttributedDiscoveryGraph` |
| fold producer counters after the skipped-slot return | `TestProducerCountersSurviveASkippedSlot` |
| stop counting dropped events on the hook path | `TestAStalledConsumerCannotBlockTheHook` |

## Scope decisions

- **Controller: not built.** No XInput/GameInput infrastructure exists to normalise, so this
  would be new platform code rather than a small reuse. Keyboard alone proves the loop. The
  contract if it is added is recorded in [[Navigation]]: edges not held state, masks stay inside
  the adapter, and downstream correlation must not be able to tell which device produced an
  intent.
- **Pointer: not built.** `director.exe` installs no mouse hook, and a click would additionally
  need normalisation against the validated target so that a click outside it cannot become
  target-relative evidence. `NavPoint` remains in the vocabulary with its admission rules
  enforced and no producer behind it.
- **Text entry: documented only**, as a separate privilege on a separate channel.

## Known gaps

- **No live confirmation.** Deterministic proof is complete; a 60-second Rocket League check
  would confirm that a real device produces intents through a real hook. It is confirmation of a
  finished mechanism, and the milestone is not blocked on it.
- **Sub-sample ordering is unavailable.** Two intents inside one ~3.5s observation interval can
  be ordered against the screen change but not usefully against each other.
- **Ambiguous keys stay refused.** A game whose menus are driven by WASD contributes no
  navigation evidence until admission can be conditioned on screen state.
- **The `Preceded` map and `Sequences` can disagree under their bounds.** Both are bounded
  independently (8 intents, 6 orders), so a pathological edge could retain an intent in one and
  not the other. Not reachable at the observed cadence; recorded because a reader comparing the
  two would otherwise assume they cannot differ.
