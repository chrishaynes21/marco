---
type: milestone
status: historical
---

# Window identity, liveness and stale-capture prevention

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

    A window handle is an ephemeral execution reference, never durable application identity.
    Valid pixels from the wrong window are invalid evidence.
    Unknown capture ownership is a failure, not a reason to reuse cached geometry.
    A frame is evidence only when its ownership is current and proven.

## The incident

Rocket League was closed and relaunched during a live bring-up. What happened next is worth
stating precisely, because nothing about it looked like a failure:

```
Rocket League closed
→ HWND 661516 destroyed
→ relaunched as HWND 662844 on the other monitor
→ the Director went on holding 661516
→ the live-bounds lookup for a dead window failed
→ capture fell back to the CACHED bounds (-1920,0 1920x1080)
→ the detector found 169 real UI elements there
→ every diagnostic attributed them to Rocket League
```

No error. No empty result. Real pixels, correctly detected, from a window that had not
existed for minutes — and the boxes were VS Code's, on the left-hand monitor.

That is the failure mode this gate exists to make impossible.

## Where it came from

Two independent contributors, both of which had to be fixed:

1. **`wincapture.CaptureWindow` fell back to remembered bounds.** The live lookup was
   optional and, on failure, the caller's `Window.Bounds` were used. Those bounds are a
   memory of where the window was when somebody last looked, and a memory is not a place to
   point a camera.
2. **`activeWindow` returned a snapshot, unvalidated.** It read the last observed world,
   which is correct for choosing a *candidate* — text has to fuse against elements from the
   same moment — but nothing then asked the platform whether that window still existed.
   `ReadVision` additionally never refreshed the world first, so the snapshot could be
   arbitrarily old.

## The shape of the fix

```
internal/director/perception/windowref   what a window reference IS, and its lifecycle
internal/winctx/liveness.go              the platform questions (IsWindow, PID, image name)
internal/platform/winprovider/live.go    windowref.Platform over winctx
internal/platform/wincapture             the capture-time guard
cmd/director/ocrwiring.go                activeWindow: candidate → validate → capture
```

`windowref` contains no Windows code. `Platform` is the seam, which is what makes the
interesting states testable at all — a destroyed window, a recycled handle, two equally
plausible candidates are none of them things you can stage on a real desktop on demand.

## The lifecycle

```
candidate (from the last observed world)
   → Propose        an unvalidated suggestion, kept SEPARATE from what is held
   → Acquire        validate what is held; on failure invalidate, then reacquire
   → capture        the platform boundary checks ownership again, before and after
```

**Validation asks the platform, every time.** Does a window still have this handle; does it
belong to the expected process; is that process alive; is the application the same; are the
bounds usable; do they intersect a monitor. Cached bounds are never evidence of any of it.

**Invalidation is atomic.** After it returns, no caller can obtain the old handle or the old
bounds from the tracker — there is nothing left to fall back *to*, which is what makes the
original failure structurally impossible rather than merely guarded against.

**Reacquisition searches by identity**, never by the old geometry and never assuming the
handle is unchanged. The application is matched on executable name, which is the thing that
survives a restart; titles are not used, because they change while a window lives.

Ranking is explicit and deterministic: the foreground window if it belongs to the
application, else the largest visible one, else **nothing**. Two equally plausible windows
produce `ambiguous` rather than a choice — answering by enumeration order would be a coin
toss presented as a fact.

### States

`valid` is the only one that authorises a capture. It is an allow-list, so a state added
later is refused until somebody decides otherwise:

| state | meaning |
|---|---|
| `valid` | the handle still names the expected window of the expected process |
| `destroyed` | no window has this handle any more |
| `ownership_changed` | the handle belongs to a different process now — recycling, caught |
| `process_exited` | the window's process is gone |
| `bounds_unavailable` | it exists but reports no usable rectangle |
| `off_screen` | the bounds intersect no monitor (Windows parks minimized windows at −32000) |
| `ambiguous` | several equally plausible windows; refusing to choose |
| `unavailable` | the application has no capturable window |
| `unknown` | the platform could not answer |

### Epochs

Each distinct window gets a generation. It increments on a different **handle** or a
different **process**, and *not* on a window that merely moved — a generation that changed
every time a window was dragged would tell nobody anything, and one that survived a restart
would be a lie.

The generation is what diagnostics show. **The raw handle is not**, deliberately: a handle in
a diagnostic invites a reader to compare it with one from five minutes ago and conclude the
window is "the same", which is exactly the mistake behind the incident.

## The capture-time guard

Defence in depth. The tracker validates upstream; `wincapture` checks again, because the
window can close between the two and because a future caller may simply forget:

- no live-bounds lookup wired → refuse (it used to be optional; that optionality was the bug)
- lookup says the window is gone → refuse, and never fall back
- owner is not the expected application → refuse
- window disappears or changes hands *during* the capture → `window_changed_during_capture`,
  no frame

## What is proven, and how

**Unit** — `windowref` (25 tests) against a fake desktop, including the exact incident:
handle A at bounds X → closed → reopened as B at bounds Y → assert A is rejected, X is never
captured, B is selected, Y is used, the generation changes.

**Mutation-verified.** Deleting the liveness check makes
`TestADestroyedWindowWhoseProcessSurvivesIsCaughtByLivenessAlone` fail with "the old
rectangle was returned — this is wrong-region capture". Deleting the process check makes
`TestAHandleRecycledWithinTheSameApplicationIsCaught` fail. Both were confirmed by actually
applying the mutation and watching the suite go red — the first attempt at each was *not*
caught, because another check masked it, and the tests were sharpened until they were.

**Capture boundary** — `wincapture` (7 tests), every one asserting a refusal.

**Live, against the real operating system** — `winprovider` has an opt-in test that launches
a real program, validates its window, kills it, and asserts the tracker refuses:

```sh
MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/ -run Live -v
```

Its output on the machine where the incident happened:

```
live window: pid=14060 bounds={191 107 1536 864} title="Untitled - Paint"
validated: mspaint window, generation 1, at 1536x864+191+107
refused, correctly: unavailable — no window belonging to mspaint is currently
available to capture
```

## Known gaps

1. **The Rocket League close/relaunch has not been driven end to end through `director`
   itself.** The mechanism is proven live against mspaint and the exact scenario is a unit
   test, but the full game restart with a person in Free Play remains unrun.
2. **`activeWindow` still depends on which window is FOCUSED.** Running a diagnostic from a
   terminal takes focus, so live game work is awkward and the captured window is often the
   terminal. A `--window <id|title>` target would remove a whole class of confusion. This is
   the same gap noted in `docs/director-vision.md`.
3. **`ReadVision` still does not refresh the world before looking**, unlike `ReadText`. The
   validation now catches the consequence, but the candidate can be older than it needs to
   be.
4. **Ambiguity is refused, not resolved.** An application with two same-sized windows and no
   foreground yields no frame at all. That is the honest answer and may prove annoying.
5. **Non-Windows platforms refuse everything.** The stubs answer "no" to liveness, which is
   the safe direction, but it means no capture on those platforms rather than an unvalidated
   one.
