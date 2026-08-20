---
type: milestone
status: historical
---

# Conditions and the Wait Engine

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> **Time is never evidence. Observations are evidence.**
> The Director waits because it cannot yet prove the required condition — not because
> it guesses something will eventually happen.

## What was wrong with a sleep

The pipeline used to pause 350 ms after acting, chosen because a menu usually renders in
less than that. It is wrong in **both** directions and invisible in both:

- too short on a loaded machine, and verification runs against a half-drawn screen and
  concludes the action failed;
- too long, and every command is slower than it needs to be, forever, with nothing to
  point at.

A condition is **falsifiable** where a duration is not. "The region around the target
has stopped changing" is answered by looking, can be **Unknown**, and finishes the
instant it becomes true.

Measured live, the same click on the same control:

```
settle  waited region_stable: satisfied after 2 observation(s) in 416ms
settle  waited region_stable: satisfied after 3 observation(s) in 800ms
```

Different because the screen was different. A constant cannot do that.

## Unsatisfied is not Unknown

| state | meaning |
|---|---|
| `satisfied` | the world provides sufficient evidence |
| `unsatisfied` | the Director **looked**, and it is not so — keep waiting |
| `unknown` | the Director **could not look** — it does not know |
| `cancelled` | the user stopped it |
| `timed_out` | the bound elapsed, carrying **what it kept seeing** |

Collapsing the middle two is what makes waiting dangerous. A wait that treated blindness
as a negative would poll a window it cannot read until it timed out, then report that
the condition never came true — a confident claim about something it never observed.

A timeout therefore says *which*: "timed out while unknown" sends someone to the
perception layer, "timed out while unsatisfied" does not.

## Conditions

Element: `exists`, `missing`, `enabled`, `disabled`, `focused`.
Window: `exists`, `closed`, `foreground`, `title_contains`.
Text: `appears`, `disappears`.
Region: `stable`, `changed`, `still_changing`.
Plus `verification_satisfied`.

All **semantic** — none names a platform API, a handle, or a poll interval. Elements are
named by **query**, not by id: an id is meaningful only within one identity history, so
a condition naming one could not survive a replay.

Notable behaviours:

- **`ElementEnabled` uses structural state first.** `StateEvidence` records which source
  set the flag; a visual inference is used only where nothing structural spoke.
- **`TextAppears` does not require OCR.** A label the accessibility tree reports *is*
  visible text. Demanding a capture would be slower, less reliable, and unusable where
  OCR is absent.
- **`WindowClosed` means gone, not unfocused** — and says so when it refuses, because
  those are the two most easily confused.
- **An element that does not exist is `Unknown` for enabled/disabled**, not unsatisfied.
  A button that isn't there is not a disabled button, and reporting it as one would let
  a wait for "disabled" finish the moment the dialog closed.

## The loop

```
Observe → Evaluate → Satisfied? → no → wait → Observe …
```

The single duration in the engine is the **poll interval**: how often to look, never how
long anything takes. Nothing concludes a condition holds because it elapsed.

**Stability is a run, not an instant.** `StableObservations` (default 2) requires
consecutive satisfied evaluations, because an animation is momentarily identical between
frames and a page part-way through loading is quiet between paints. An unsatisfied
evaluation resets the run; an **Unknown** neither advances nor resets it.

Cancellation is checked **before observing, after observing, before the pause and after
it** — a wait is long, and the pause is where a stop most often arrives. It reports
`Cancelled`, never `TimedOut`.

## Verification defers instead of retrying

When the target region is still changing, verification now **waits and verifies again**.
The action is never repeated — only the looking is. That is the correct response to
"still changing", and it is what the fixed delay could never express.

## Replay

Replay has **no per-iteration delay constant** (the old `ReplaySettleDelay` was dead code
and is gone). Every iteration runs the full cycle, and its settle is the same semantic
wait, re-evaluated from scratch. A replay of an action that once took two looks does not
wait for two looks, and does not wait for however long it took last time.

## Remaining sleeps

Two, both the absence of a condition rather than a preference for time — and both
announced in the trace as `fixed delay (<reason>)`:

1. no region watcher or wait engine is wired, so the condition cannot be evaluated;
2. the action has no target region to watch.

## Diagnostics

```
director wait            what the Director is waiting FOR
director wait --follow
director wait --json
```

A wait that keeps returning Unknown is called out explicitly: *"Every look has been
UNKNOWN: that is blindness, not slowness — waiting longer will not answer it."*

## Why the wait engine is not compiled into Marco

The milestone asked for waits to compile into Marco. They do not, and the reason is
structural rather than incidental.

Marco's wait vocabulary is `OS's Sleep` (raw milliseconds) and `Find` with a `Timeout`
(poll the screen for an image or OCR anchor). **Neither can evaluate a World State
condition.** "The Save button is enabled" is answered from the accessibility tree,
fusion, and element identity — none of which Marco has or should have. Delegating the
wait would mean giving the Marco runtime access to the Director's perception layer,
which inverts the dependency the whole architecture rests on: the Director is built *on*
Marco, not beside it.

So the engine lives in the Director, where the evidence is. What Marco executes is
unchanged — input, window operations — and the Director no longer asks it to sleep.

Adding a Marco-side primitive would be justified the day a condition is expressible in
Marco's own vocabulary and useful inside a taught route. `Find`-with-`Timeout` already is
that primitive for screen anchors, and the routes path already uses it.
