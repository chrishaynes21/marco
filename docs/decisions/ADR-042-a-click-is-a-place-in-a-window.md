---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - navigation
  - passive-observation
source_paths:
  - internal/platform/navsource/navsource.go
  - internal/platform/navsource/navsource_windows.go
  - internal/director/observe/input.go
  - cmd/director/observewiring.go
---

# ADR-042 — a click is a place in a window, or it is nothing

`NavPoint` — "a pointer press" — had been in the navigation vocabulary since navigation was
designed. It was consumed by the demonstration assessment and by the play generator. It was
**produced by nothing**: `navsource` installed a `WH_KEYBOARD_LL` hook and no other.

[[Experiment-011-two-level-identity-against-real-software]] measured the consequence. Watching
File Explorer, the screen model saw every change correctly and **1 of 7** of them had any
navigation observed before it. The policy that decides whether Marco may offer to learn a habit
allows at most **half** to be unattributed. So a mouse-driven application could produce a perfect
record of screens changing with nothing whatever to say about how the person did it, and Marco
would correctly never offer to learn it.

Most desktop software is mouse-driven.

## The decision

A `WH_MOUSE_LL` hook is installed beside the keyboard one, on the same thread and the same pump,
and a button press becomes `NavPoint` **only when it can be placed inside the watched window**.

### What a press must survive

- **A frame.** The composition root pushes the pinned window's bounds on every valid inference,
  from the same validated reference every other piece of that cycle's evidence is scoped to.
- **Freshness, in both directions.** Bounds older than `ScreenContextTTL` may describe a window
  that has moved; bounds confirmed *after* the press cannot decide where it landed. Both are the
  rules the screen assessment already follows.
- **Containment.** A press outside the window is somebody's other application, and attributing it
  would put their other windows into this session.

A press that fails any of these is counted as `unplaceable_pointer` — **not** admitted at the
origin. A press at the top-left corner and a press nobody could place are different things, and
only one of them happened at the corner.

## What may be observed, and what may not

Two messages: `WM_LBUTTONDOWN` and `WM_RBUTTONDOWN`.

**`WM_MOUSEMOVE` is absent, and its absence is the guarantee.** A movement stream is a pointer
trail — a continuous record of where somebody's hand went — and it is on the list of things no
durable record may contain. The way that is enforced is that the callback has no branch for it:
no flag, no configuration and no later edit can produce one without somebody writing a new case on
purpose. Button-up is absent for the same reason key-up is; emitting on both edges would double
every press.

## Privacy

An absolute desktop coordinate exists on `rawEvent`, which is unexported, has no `String` method
and never leaves the package. It is normalised against the window on the worker and discarded.
What crosses the boundary is `PointerAt` — window-relative, in 0..1 — which describes a place in
an application rather than a place on somebody's desk.

`PRIVACY_WIDENED: NO`. `PointerAt` was already on `InputEvent` and already zeroed for every
non-pointer intent by `admissibleInputs`; what changed is that it is now ever non-zero.

## Latency

The callback does what the keyboard one does: bounds-check, read the struct, offer to a bounded
channel, chain. No lock, no allocation, no window lookup, no normalisation. Windows silently drops
a low-level hook that overruns its timeout and takes the overlay's F12 stop key with it, so
placement — which needs the window — happens on the worker.

## What this does NOT do

**It does not make mouse-driven plays learnable.** Both consumers of `NavPoint` still refuse it,
deliberately: a demonstration containing one is `unresolved_pointer_target`, and lowering refuses
with `cannot_say_pointer`. *"A pointer press needs a position, and a position is not a meaning."*

So the change converts a **silent** failure into a **specific** one. Marco can now say "I saw you
do something before that change" and then "I cannot write down a click, because there is nothing
nameable to aim at". Resolving a press to the control underneath it is a separate piece of work
with its own evidence to gather; this is its prerequisite, not its substitute.

## Enforced by

- `internal/platform/navsource/pointer_test.go` — placement and its corners, refusal on stale,
  future, missing and zero-area frames, refusal outside the window, no session retains nothing,
  no absolute coordinate survives, and the structural guarantee that movement has no branch.
- `cmd/director/navcontextwiring_test.go` — `pushNavContext` through the production method: a
  valid inference gives the pointer a frame, and a cycle Marco could not observe leaves the
  previous one standing. Both pushes survived deliberate deletion before this existed.

## Related

[[ADR-013-navigation-is-meaning-not-keys]] · [[Navigation]] ·
[[Experiment-011-two-level-identity-against-real-software]]
