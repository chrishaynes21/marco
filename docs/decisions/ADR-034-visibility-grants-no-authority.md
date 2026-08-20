---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - visibility
  - demonstrations
  - passive-observation
source_paths:
  - pkg/playbill/guard.go
  - internal/director/service/playbill.go
  - cmd/director/playbill.go
---

# ADR-034 — visibility grants no authority

A surface that can see everything is one refactor away from a surface that can do everything,
and the refactor is always made by somebody who is tired of switching to a terminal.

## Decision 1 — the account carries no field anything can act on

`playbill.View` has no method that runs, approves, confirms, rehearses, cancels or marks. That is
checked structurally by `TestNothingOnThePlaybillCanAct`, because "we reviewed it and there is no
`Run`" is true right up until the convenience is added.

The rehearsal grant is the case that matters. A `RehearsalGrant` is authority data
([[ADR-023-rehearsal-is-attempt-scoped-authority]]); the account carries its **state** — the word
`issued` — and never the grant. A view is serialised to JSON and handed around, and authority
that can be marshalled is authority that can be replayed.

Rendering *"you said yes — I can try this once"* creates nothing. Rendering *"I could write this
down"* authorises nothing. Rendering *"I recognise this as the pause menu"* crosses no invocation
door.

## Decision 2 — a question carries the door it goes through, not a new one

A pending question carries the id its answer routes by and a `Via` naming which EXISTING request
the answer travels: `confirm`, `clarify` or `proposal`. Those are three different things — a
confirmation unblocks a command still holding its bindings, a clarification re-runs a request
with a refinement, and a proposal answer is evidence about a hypothesis that may be read minutes
later — and they are not interchangeable.

A question with no `Via` is refused by the guard, because a surface rendering it would have to
invent a way to answer, which is the one thing this representation must never invite.

There is no `answerFromWatch`. The overlay makes the same request a terminal client makes.

## Decision 3 — reading changes nothing, including reading a lot

`PLAYBILL` starts no observation, takes no sample, attaches no provider, runs no OCR pass and no
vision inference, and raises no question. The last one is not free — see Decision 3 of
[[ADR-033-one-account-many-presentations]] — and it is the one a future change is most likely to
break, because the natural way to ask "is there a play yet?" is the call that also asks the user
what a screen is called.

## Decision 4 — cancellation cannot be blocked by visibility

Esc closes the panel and releases the mouse without consulting the Director, so an unreachable or
wedged Director cannot leave somebody with a captured cursor. `PLAYBILL` is non-mutating, so it
never queues behind or delays `CANCEL_ACTIVE`.

## Consequences

- Interactive controls may be added to Watch later. They must call the ordinary production
  request, and the account gives them nothing else to call.
- "The panel shows it but I cannot act on it" is the intended experience, not a gap.

## Enforced by

- `pkg/playbill/playbill_test.go` — `TestNothingOnThePlaybillCanAct`,
  `TestAQuestionMustNameAnExistingResponsePath`
- `internal/director/service/playbill_test.go` — `TestTheVisibilitySurfaceCannotAuthoriseAnything`,
  `TestAPendingConfirmationAppearsAndIsAnsweredNormally`,
  `TestAWatcherDisappearingDoesNotStopTheDirector`
- `cmd/director/playbillwiring_test.go` — `TestAnOutstandingGrantIsReportedAndNeverHandedOver`,
  `TestReadingTheAccountDoesNotAskTheUserAnything`
- `plugins/overlay/watch_test.go` — `TestEscapeAlwaysReleasesTheSurface`
