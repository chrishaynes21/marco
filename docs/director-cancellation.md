---
type: milestone
status: historical
---

# Runtime cancellation contract

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> A deadline limits how long Director waits. It does not prove the computer stopped acting.
>
> Cancellation is immediate at the control plane and safe at the execution boundary.
>
> An operation is cancellable only when the code proves it can stop without leaving the
> machine in a damaged intermediate state.

## The two modes

| mode | meaning |
|---|---|
| `cancellable` | the operation checks the context *while running* and can stop part-way, leaving no held input |
| `atomic_non_interruptible` | once started, the operation runs to completion; cancellation is recorded and prevents the *next* one |

A context reaching a host is **not** evidence of cancellability. Most host actions take
a `ctx` and return `ctx.Err()` **after** the input has already been sent — the check
reports that the command was cancelled, not that the action was prevented.

## Audit

Classified from implementation, not intuition. File and line references are to the
Windows backend, which is the only one that performs real input.

| capability | mode | evidence |
|---|---|---|
| `OS's Click` | **atomic** | `backend_windows.go` `clickAt` builds move+down+up as **one** `SendInput` batch, then returns `ctx.Err()`. The batch is submitted before the context is ever consulted. |
| right/middle click | **atomic** | lowered as `OS's Move` then `OS's Click`; each is individually atomic |
| `OS's Move` | **atomic** | `move` sends one event then returns `ctx.Err()` |
| `OS's Drag` | **cancellable, with guaranteed release** | `drag` presses, then glides in 24 steps with `if ctx.Err() != nil { break }`. **On break it still sends the final move and the button-up.** The button cannot be left held. |
| `OS's Key` | **atomic** | `keyDown`/`keyUp` each send then return `ctx.Err()` |
| `OS's KeyDown` | **atomic, and leaves a key held by design** | the hold is the point; release is `KeyUp` or the driver's `ReleaseHeld` |
| `OS's KeyUp` | **atomic** | as above |
| `OS's Type` | **cancellable** | `typeText` checks `ctx.Err()` per character and on its inter-character wait. Stops part-way; text already typed stays typed. |
| `OS's Secret` | **assume atomic** | types a credential through the same path; not separately audited |
| `OS's Activate` / `Launch` | **atomic** | no context parameter reaches the implementation |
| `OS's MoveWindow` / `WindowState` | **atomic** | `winctx` calls take no context |
| `OS's ClipboardGet` / `ClipboardSet` | **atomic** | Win32 clipboard calls take no context |
| `Accessibility's Focus` / `SetValue` / `GetValue` | **unknown** | crosses the bridge subprocess; the context bounds the *Director's wait*, not the C# work |
| `OS's Sleep` | **cancellable** | `doSleep` selects on `c.Ctx.Done()` |

### What "atomic" means in practice

Cancelling during an atomic click does **not** prevent the click. The user's cancel is
recorded immediately at the control plane, the click completes, and no *later* step
runs. That is the honest behaviour, and it is why a cancelled command must never be
reported as "nothing happened".

### Drag is the interesting case

`drag` is the only input operation that genuinely honours cancellation mid-flight, and
its cleanup is real: the release is outside the loop, so breaking out still releases the
button. But note what cancellation does **not** do — the release still happens **at the
destination**, so a cancelled drag still drops its payload where it was headed. It
prevents the glide, not the drop.

## Held-input and clipboard safety

`driver.RunSourceWithHostsCtx` defers two cleanups:

```go
if cs, ok := hosts["*"].(cursorSnapshotter); ok { defer cs.CursorSnapshot()() }
if hr, ok := hosts["*"].(heldKeyReleaser); ok { defer hr.ReleaseHeld() }
```

Both fire on success, error and cancellation.

**They were not covering the Director.** `marcorunner` passed only `{"OS": …,
"Accessibility": …}` with no `"*"` entry, so neither ran for a Director-generated
program. Nothing reaches it today — `marcoexec` emits no `KeyDown` — but the guarantee
existing *in the codebase* is not the same as it covering *this path*. `marcorunner`
now registers the unwrapped OS host under `"*"`. Unwrapped deliberately: both are
optional-capability type assertions, and the recording tee does not implement them, so
wrapping would silently disable them again.

**Clipboard** restoration is independent of cancellation: `clipboard.Loan.Restore` is
called on every exit path from `deliver`, including a failed paste, and whether it
succeeded is recorded rather than assumed. See `docs/director-editing.md`.

## Timeout is not interruption

A `runtime_execute` deadline expiring means the Director stopped waiting. For an atomic
operation the host may still be acting. So:

- the phase closes `TIMED_OUT`, naming itself;
- the command is **never** retried;
- the outcome is reported as unverified rather than failed — *failure to receive
  confirmation is not proof the action did not happen*.

## Cancellation is not timeout

`trace.Do` classifies the outer context's cancellation **before** the phase's own
deadline, so a phase that was cancelled while also past its deadline is reported as
`CANCELLED`. The user asking to stop is the more informative fact, and reporting it as
a timeout would invent a fault that did not occur.

## Where sequences stop

Cancellation is checked before every program step. A Marco program that has started
runs to its end — abandoning it halfway is what would leave a key down or half a string
typed — so cancellation prevents the **next** operation, not the current one.

Live: `open File then click Save then click Cancel`, cancelled mid-run, produced
`CANCELLED: stopped before step 2 of 3 (click Save)` with step 1 left verified.

## Known gaps

These are **not** proven and are stated rather than assumed:

- **The accessibility bridge's cancellability is unknown.** The context bounds how long
  the Director waits for the subprocess; whether the C# side stops is untested. It is
  classified `unknown` and should be treated as atomic until proven otherwise.
- **There is no `CancellationMode` registry in code.** This document is the audit; the
  classification is not yet machine-checked, so a new capability cannot currently
  "fail closed" automatically.
- **`OS's Secret` was not separately audited**, only assumed to share `Type`'s path.
