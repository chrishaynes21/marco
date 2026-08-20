---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - navigation
source_paths:
  - internal/platform/navsource/navsource.go
  - internal/director/observe/input.go
  - cmd/director/surfaceowner.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/observewiring.go
---

# ADR-065 — operating Marco is not demonstrating to it

Learn is driven by a control surface with buttons on it. Pressing one is a real physical click
that the hook sees, inside a session that is watching. Until now nothing could tell that click
apart from the task the person was demonstrating.

## What was wrong, in two places

**The clicks themselves.** Start, Stop, Try It, and clicking Marco mid-demonstration to see what
it thinks are all how a person OPERATES Marco. Admitted as evidence they are worse than noise: the
demonstration acquires an action nobody performed on the application, and whichever screen change
happens next is attributed to it.

Geometry nearly hides this and does not solve it. A press outside the watched window's rectangle
is already refused as unplaceable, which covers a panel sitting *beside* the application — but
Marco's overlay is a full-screen surface lying *over* that window, so a press on it is inside the
rectangle by construction.

**The window.** Pressing Start necessarily brings Marco to the front, and a session that resolved
its target at that instant pinned Marco's own control surface: fingerprinted it as the place the
task starts from, then watched a window the person was walking away from. "Put the application in
front first" is not a fix — the button is in Marco, so Marco is always the last thing they
touched. Both halves have the same cause: *the surface is in front, and it is not the subject.*

## The decision

**Ownership is decided when input is classified, and a session started from a surface waits for a
subject.**

- `observe.IgnoreMarcoOwned` joins the closed refusal vocabulary. A press or key admitted while
  Marco's own surface held the foreground is refused under its own name — not as
  `unplaceable_pointer`, which would report operating Marco as a perception failure and send a
  reader hunting for a coordinate bug.
- `navsource.Source.SetSurfaceOwner(owned, at)` is pushed by the composition root on every valid
  inference, beside `SetWindowBounds` and `SetActionables`. Pushed rather than asked, for the
  reason the whole hook path is arranged this way: answering "whose window is in front" is a
  syscall, and a low-level hook callback that makes one is a hook Windows drops.
- `teach.WaitingForDemonstration` is where a session begins. It is a PRECONDITION of the first
  real step rather than a step of its own, so it costs no cycle when there was never anything to
  wait for. The wait itself is `Passes.AwaitSubject`, so the coordinator still cannot look at a
  window or tell one program from another.
- Only a request that declares `Surface: true` waits. `director teach` resolves the foreground
  once and pins it, exactly as it always has — the command line is a developer tool and must not
  change shape underneath the people and tests using it.

### How ownership is actually decided, and the honest limitation

Two clauses, because Marco's surfaces are not all Marco's processes:

1. the foreground window belongs to one of Marco's own programs — the overlay, and any native
   surface;
2. the foreground window is the one a Learn session was STARTED from.

The second is an inference and is stated as one. The control centre is a local web page, so the
window hosting it is the person's own browser and is indistinguishable from any other browser
window by process. The click that started the session necessarily came from the window hosting the
surface that sent it, so that window is adopted once, at that moment, and never guessed at again.
It is released when the session ends: a browser window that once showed the control centre is an
ordinary window the moment it does not.

What this deliberately does **not** do is treat "the browser" as Marco. A person may well be
demonstrating a task in a browser, and one window being the control centre must not make the rest
un-teachable.

## Which way the doubt falls

Toward keeping the person's evidence, every time. With no recent ownership answer, or one older
than its freshness bound, input is **admitted**. Refusing input because ownership could not be
confirmed would silently discard a real demonstration and tell the person they had shown Marco
nothing — the worst failure available here, and the exact opposite of
[[ADR-057-attributed-input-survives-interpretation]].

## Considered and rejected

- **Clear the captured input when Start or Stop is pressed.** It discards evidence the person did
  produce in order to compensate for evidence that was never real, and it cannot help at all with
  the presses in between — clicking Marco mid-demonstration to check on it.
- **Refuse everything outside the watched window.** Already true, already insufficient: the
  overlay is inside it.
- **Recognise the control centre by its window title.** A title is content. "Marco" in a browser
  tab would then make somebody's notes page part of Marco.
- **Have the surface tell Marco which clicks were its own.** A surface that can declare input
  uninteresting can declare any input uninteresting, and the boundary would live in the least
  trustworthy place.

## Enforced by

- `internal/platform/navsource/owned_test.go` —
  `TestAClickOnMarcosOwnSurfaceIsNotCaptured` (a press at coordinates *inside* the watched
  window), `TestTheFirstRealPressAfterMarcosOwnIsCaptured` (the half that matters: the person's
  own click survives), `TestAKeyPressedWhileMarcoIsInFrontIsNotCaptured`,
  `TestWithoutAnOwnershipAnswerInputIsStillCaptured` and `TestAStaleOwnershipAnswerStopsRefusing`
  (the direction the doubt falls).
- `cmd/director/navcontextwiring_test.go` —
  `TestMarcoOwnedInputNeverBecomesDemonstrationEvidence` holds the composition root's push.
  Deleting it fails; without it a pushed fact nobody pushes is indistinguishable from a fact that
  is always false.
- `cmd/director/learnwiring_test.go` —
  `TestStartingFromMarcosOwnSurfaceWaitsForSomethingElse` asserts the session is handed NO window,
  and `TestANamedWindowIsNotWaitedFor` is its control. The assertion is on the selector rather than
  on the phase deliberately: the teaching owner publishes a session only when a step returns, so a
  mutation that pinned Marco's window and then blocked inside a pass would still read as "waiting"
  for as long as any unit test is willing to sleep.

## Related

[[ADR-057-attributed-input-survives-interpretation]] · [[ADR-060-input-has-no-address]] ·
[[ADR-013-navigation-is-meaning-not-keys]] · [[ADR-066-stop-is-a-product-event]] ·
[[Navigation]] · [[Demonstrations]]
