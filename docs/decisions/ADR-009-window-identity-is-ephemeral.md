---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - windows
  - game-packs
  - action-graph
  - replay
---

# Application identity may survive a restart; window identity may not

## Context

"Close Rocket League and reopen it, then resume where we were." Something has to persist
across that restart, and something must not.

The application is recognisably the same thing: same executable, same capability pack, same
semantics for "the pause menu". The window is not. It is a new window with a new handle, new
bounds, and no relationship to the old one beyond belonging to the same program — and the
operating system is free to hand out the previous handle again to something else entirely.

Treating a window handle as a durable identity is what produces a recycled-handle bug, and
those are silent: the handle resolves, the capture succeeds, the pixels are wrong.

## Decision

**Application identity may survive a restart. Window identity may not.**

A window's identity is scoped to a *generation*. A handle recycled within the same
application gets a new generation and is treated as a different window, because it is one.
Anything that needs to outlive a restart is keyed on the application, never on a handle.

## Consequences

- Game packs, capability declarations and semantic knowledge attach to the application and
  survive a relaunch.
- Nothing positional survives. Recorded coordinates, cached bounds and window-scoped
  observations are discarded at the boundary, which is why [[ADR-008-no-stale-window-geometry]]
  can hold.
- Replay re-acquires the window rather than reusing one, and re-lowers rather than replaying
  a mechanism — see [[Action-Graph]].
- `TestResourceIdentityCarriesNoCoordinates` exists because a resource identity that quietly
  carried a rectangle would reintroduce the problem through the back door.

## Enforced by

- **implementation** — `internal/director/perception/windowref` (generation, selector),
  `internal/platform/winprovider`
- **regression test** — `TestAHandleRecycledWithinTheSameApplicationIsCaught`, mutation-verified:
  deleting the process check makes it fail
- **boundary test** — `TestResourceIdentityCarriesNoCoordinates`
  (`internal/director/boundary_test.go`)
- **milestone record** — *The lifecycle* in [[director-windows]]

## Related

- [[ADR-008-no-stale-window-geometry]]
- [[Windows]], [[Game-Packs]], [[Action-Graph]]
