---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - windows
  - perception
  - capture
  - safety
---

# Stale window geometry is never reused

## Context

A window was validated. Its bounds were recorded. Some time later a capture is taken using
those bounds.

Between those two moments the window may have moved, resized, closed, or closed and been
replaced by a different window that the operating system gave the same handle. The recorded
rectangle is still a perfectly valid rectangle; it now describes a region of the screen
belonging to something else.

The result is **wrong-region capture**: the Director perceives one application while
believing it is looking at another, and every downstream conclusion is confidently wrong.
This is the incident that motivated the whole milestone.

## Decision

**Geometry is never reused across a validity boundary.** A capture re-establishes that the
window is alive, is the same window, and is where it is believed to be — or it refuses.

Refusal is a normal outcome, not an error condition. "No window belonging to mspaint is
currently available to capture" is the correct answer when that is true.

## Consequences

- Capture costs a validation round trip that a cached rectangle would not.
- Diagnostics lead with refusals, because a refusal is the common and healthy case when a
  target is not present.
- A live observation session that loses its window stops producing samples rather than
  producing samples of the wrong thing.

## Enforced by

- **implementation** — `internal/director/perception/windowref` (selector, liveness,
  generation), `internal/platform/wincapture`
- **unit tests** — `windowref` (25 tests) against a fake desktop, including the exact
  incident: handle A at bounds X → closed → reopened as B at bounds Y → A rejected, X never
  captured, B selected, Y used, generation changed
- **mutation-verified** — deleting the liveness check makes
  `TestADestroyedWindowWhoseProcessSurvivesIsCaughtByLivenessAlone` fail with "the old
  rectangle was returned — this is wrong-region capture"
- **capture boundary** — `wincapture` (7 tests), every one asserting a refusal
- **live test** — `MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/ -run Live`
- **milestone record** — [[director-windows]], *What is proven, and how*

## Related

- [[ADR-009-window-identity-is-ephemeral]] — why a handle is not an identity
- [[Windows]], [[Perception]]
