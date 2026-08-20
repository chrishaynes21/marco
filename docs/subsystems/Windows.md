---
type: subsystem
status: active
owners:
  - director
depends_on: []
used_by:
  - perception
  - vision
  - passive-observation
  - game-packs
updated: 2026-08-06
source_paths:
  - internal/director/perception/windowref
  - internal/platform/wincapture
  - internal/platform/winprovider
  - internal/winctx
---

# Windows

Window identity, liveness, and stale-capture prevention.

## The incident

A window was validated and its bounds recorded. It closed. The operating system handed the
same handle to something else. A capture taken with the recorded rectangle returned pixels
belonging to a different application — and everything downstream was confidently wrong about
what it was looking at.

## The fix

A window reference carries a **generation**. Validity is re-established at capture time —
alive, same process, same window, same place — or the capture **refuses**. Refusal is a
normal outcome.

Governed by [[ADR-008-no-stale-window-geometry]] and
[[ADR-009-window-identity-is-ephemeral]].

## Responsibilities

- Select and validate a window.
- Detect liveness, process identity and handle recycling.
- Guard capture at the boundary.
- Answer, without changing anything, whether a held reference is still itself
  (`Tracker.Confirm`) — the read-only check the provenance guard is built on.
- Refuse clearly, with a reason.

## Confirm versus Acquire

`Acquire` is what you call before **acting**: it validates, and where validation fails it
reacquires and adopts, because a caller about to capture needs a window.

`Confirm` is what you call to ask a question about evidence already in hand. It reads the
platform and changes nothing — no adopt, no reacquire, no epoch change. Using `Acquire` there
would repair the window as a side effect of checking somebody's homework, advancing the
generation and making the very staleness being tested for disappear at the moment of testing.
See [[ADR-011-provenance-is-proven-not-assumed]].

## Callback exhaustion, and why it is tested at 4,500 iterations

`syscall.NewCallback` allocates from a fixed process-wide table that Go never frees.
`Monitors()` leaked one per call for a long time, harmlessly, until `onScreen()` began
calling it **per window** inside `LiveWindows()`. A session sampling every two seconds
exhausted the table and killed the service in under three minutes with `fatal error: too many
callback functions`.

Both callbacks are now created once via `sync.Once`, and the monitor layout is read once per
enumeration. `internal/winctx/callback_windows_test.go` runs 4,500 enumerations — a
regression crashes the test binary, which is the right severity for "the process dies".

## Related systems

- [[Perception]] — cannot sample what cannot be captured
- [[Passive-Observation]] — the workload that exposed the callback leak
- [[Game-Packs]] — application identity survives what window identity does not

## Decisions

- [[ADR-008-no-stale-window-geometry]]
- [[ADR-009-window-identity-is-ephemeral]]
- [[ADR-011-provenance-is-proven-not-assumed]]

## Validated by

- `windowref` — 25 unit tests against a fake desktop, **mutation-verified** twice
- `windowref/confirm_test.go` — `Confirm` refuses a destroyed window and a recycled handle,
  and `TestConfirmDoesNotReacquireOrAdvanceTheGeneration` holds the read-only property
- `wincapture` — 7 tests, every one asserting a refusal
- `internal/winctx/callback_windows_test.go` — 4,500 enumerations
- live: `MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/ -run Live -v`

## Known gaps

1. The Rocket League close/relaunch has not been driven end to end through `director` itself.
2. **`activeWindow` still depends on which window is FOCUSED.** Running a diagnostic from a
   terminal takes focus, so the captured window is often the terminal. A `--window <id|title>`
   target would remove a whole class of confusion — and this is the same root cause as the
   unpinned accessibility provider noted in [[Perception]].

## Milestone record

[[director-windows]] — the incident, the lifecycle, the capture-time guard, what is proven.
