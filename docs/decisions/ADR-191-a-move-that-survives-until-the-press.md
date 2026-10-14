---
type: decision
status: accepted
date: 2026-10-14
affects:
  - execution
---

# ADR-191 — a move that survives until the press

## Context

[[ADR-190-the-pointer-arrives-before-the-button-goes-down]] made an action a sequence: move the
pointer to the control, let the interface notice, press it. Against a live Rocket League menu the
sequence ran, reported success at every stage, and changed nothing — and the hover probe said the
control's pixels had differed by exactly **0.0/255** over four thousand of them.

Exactly zero, between two captures 400ms apart of a rendered 3D scene, is not a menu declining to
light up. Ruling out what it could be took five steps, and the order matters because four of them
were wrong:

1. The frames were saved and cropped. The capture really was the menu, PLAY unlit in both.
2. No cursor appeared in either frame — and that is not evidence. GDI `BitBlt` does not composite
   the cursor, so the capture cannot answer where the pointer is. `winctx.CursorPos` can.
3. It answered: the pointer was at `448,315`, nowhere near the `85,485` it had been sent to.
4. Elevation and `ClipCursor` were both ruled out: same integrity level, clip rect the whole
   virtual desktop.
5. The lowered `OS's Move` was run on its own through `marco run --host windows`, with Rocket
   League **not** in the foreground. The cursor did not move. So: not the game, not the Director,
   not the coordinates.

The cause is in the driver, and it is written down above the function that does it:

```go
// Restore the cursor to where the user left it after the run
defer cs.CursorSnapshot()()
```

`marcorunner` registers the OS host as `hosts["*"]` precisely so the driver's deferred guarantees
cover Director programs — a change made because they previously did not. Each Director operation
is its own program, so the restore fires after the move, before the press that depended on it.
The click still landed, because `clickAt` sends move-down-up in one `SendInput` batch; it landed
on an item the cursor had never hovered.

## Decision

The Director's runner wraps the OS host in `actionHost`, which keeps one of the driver's two
deferred guarantees and withholds the other.

**Releasing a held key is safety.** A `KeyDown` with no `KeyUp` leaves a key stuck down in
somebody's application after Marco has gone, and nothing later can undo it. It fires.

**Restoring the cursor is courtesy**, and it is courtesy to a person whose hand is on the mouse: a
menu macro should put the pointer back rather than park it on whatever it clicked last. That is
right for a ROUTE, which is one complete errand with a person waiting at the end of it.

A Director action is not one complete errand. It is one operation in a deliberate sequence, each
operation its own program, and the next one relies on where the last one left the pointer.
Restoring between them does not merely lose a courtesy — it makes the sequence impossible to
express at all.

So a Director program leaves the pointer where it put it.

## Consequences

- **A move survives until the press.** Rocket League's PLAY button lit up — 136.7/255 across the
  control's own rectangle — and the click opened the menu.
  [[Experiment-040-the-first-menu-Marco-opened]]
- **The pointer is left on the last control pressed.** Correct for an unattended sequence, and the
  thing to revisit on the day a Director action runs while somebody is using the mouse.
- **The two guarantees are no longer one decision.** They were bundled because they are both
  deferred cleanups on the same host; they answer to different things and now say so.
- **A capture is not a cursor locator**, and the hover line carries `winctx.CursorPos` so nobody
  has to infer a pointer's position from pixels that never contained it.

## Enforced by

`internal/platform/marcorunner` — `TestADirectorActionLeavesThePointerWhereItPutIt`,
`TestADirectorActionStillReleasesAHeldKey`.
