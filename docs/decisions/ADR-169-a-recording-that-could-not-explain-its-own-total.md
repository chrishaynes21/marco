---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - navigation
source_paths:
  - internal/platform/navsource/navsource.go
  - internal/platform/navsource/navsource_windows.go
  - internal/director/observe/input.go
  - cmd/director/capturegamesession.go
---

# ADR-169 — a recording that could not explain its own total

## Context

[[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]] made it possible to record a
mouse click. The first real session — 90 seconds of Rocket League, 900 frames, the route
Play → Training → Free Play demonstrated twice — held **17 pointer presses, every one of them
placed**. The owner, who had made the demonstration with their own hand, said:

> I clicked way more than 8 times

And the recording could not settle it. It carried one derived sentence about one refusal
reason, and `navsource`'s own account — how many events the hook received, how many it
classified, how many it refused and why, how many the bounded queue dropped — existed in memory
for the whole session and went away with the process. Worse, two of the four things that cause
the difference were counted **nowhere at all**.

This is the same failure this subsystem has been bitten by four times and written down twice: a
derived summary where the counts belonged. [[ADR-156-a-refusal-is-a-measurement]] is the
decision that says a refusal is evidence; [[ADR-150-silence-is-not-evidence-about-a-device-nobody-watched]]
is the one that says silence needs provenance. The recorder obeyed neither.

## Decision

### Four causes, four numbers, four different responses

A pointer total lower than the person remembers has four causes, and they are not the same
problem:

| cause | where it was counted before | what it means |
|---|---|---|
| a middle click, a side button, a wheel notch | **nowhere** | not navigation here; nothing is wrong |
| a press Windows marks as injected | **nowhere** | not a hand; correctly refused |
| a press outside the watched window | `ignored[unplaceable_pointer]` | correctly refused |
| an event the bounded queue dropped | `Stats().Dropped`, never written down | **a defect in Marco** |

Only the third was visible in the recording, and it was zero — which is exactly why the number
could not be explained. The two uncounted causes happen on the OS callback, before the queue, so
`Received` never saw them and no refusal on this side could have described them.

### The callback counts the presses it declines

`mouseProc` now names `WM_MBUTTONDOWN`, `WM_XBUTTONDOWN` and `WM_MOUSEWHEEL` — **declined, not
admitted**, with no branch that offers them — purely so a press of one is *counted* rather than
invisible. An injected press is counted separately, because "you used the wheel" and "something
else was driving the mouse" are different facts.

Both are package atomics incremented on the hook thread, for the reason `Source.drops` is one: a
lock on that thread is the single thing this design is arranged to avoid, because Windows
unhooks a slow low-level hook and does it silently. They are reset when the hook is installed,
so a second source in one process cannot inherit the first one's tally.

**`WM_MOUSEMOVE` is still absent and still deliberate.** A count of moves is a record of a hand
moving, there are tens of thousands of them a minute, and the callback has no branch that sees
one. Enforced by `TestTheMouseHookHasNoBranchForMovement`, which this change had to keep green —
adding a constant to that switch is the one edit that could widen what a demonstration observes,
which is why the new case is tested to contain nothing but an increment.

### The recording carries the counters, not a sentence about them

`session.json` now holds `observe.InputStats` whole, and the recorder prints
`received / classified / refused` with the reasons when a session ends — on the terminal,
because the person who knows how many times they clicked is standing there when it finishes.

The `pointer` note still exists and now accounts for all four causes in the same sentence. A
clean recording reads as clean rather than listing four zeroes.

## Two defects found in passing, both in the same reading

- **`controller: observed` appeared in a session with no controller in it.** `controllerNote`
  took a count of *all* inputs, so 17 mouse presses and two keyboard presses made it report the
  controller as observed. Asked per device now, from the provenance every event already carries.
  This is precisely the distinction [[ADR-150-silence-is-not-evidence-about-a-device-nobody-watched]]
  exists to keep, failed by the command that decision was written for.
- **A press made during the final frame was thrown away.** The loop drains the subscription once
  an interval and then leaves; nothing drained what the last interval held. The last press of a
  demonstration is the one that opened the thing being demonstrated.

## Measured outcome

The Trackmania corpus is unaffected — it has no pointer presses — and the stride 1/3/10/20
numbers are unchanged from [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]].

What this cannot yet say is how many presses the Rocket League session actually lost, because
the recording that raised the question was made before the counters existed and the process is
gone. **The honest answer is that it is not known, and that the next recording will say.** The
only thing the existing session establishes is that of the pointer events the hook handed over,
all 17 were admitted and none were refused for placement.

## Consequences

- A recording can now explain its own total, and a person's count of their own clicks can be
  argued with evidence instead of theory.
- The two new counters are the only ones in this subsystem that are **not** reset per session by
  the session itself; they are reset by hook installation. A long-lived process that opened
  several sessions on one hook would see them accumulate. Recorded rather than fixed, because
  `capture-game-session` installs a hook per run and nothing else reads them yet.
- Nothing about what is OBSERVED changed. Three more message kinds are seen by the callback and
  none of them is offered, admitted, or written anywhere but a count.

## Mutation gate

15 mutants, all 15 killed.

## Enforced by

- `internal/platform/navsource` — `TestWhatTheCallbackRefusedReachesTheStats`,
  `TestThePressesTheHookDeclinesAreCountedAndNotOffered`,
  `TestTheMouseHookHasNoBranchForMovement` (unchanged, and it had to stay green).
- `cmd/director` — `TestARecordingCanExplainItsOwnTotal`,
  `TestTheCounterSaidTheControllerWasObservedWhenNobodyTouchedIt`,
  `TestTheSessionSaysWhatTheMouseHalfIsWorth`, `TestAGameRecordingGivesThePointerAFrame`.
