---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — WORDS NO ROUTINE MATCHES EXACTLY ARE RESOLVED AGAINST THE LIBRARY BEFORE THEY ARE READ AGAINST THE SCREEN. The intake's Director arm asks Director's resolver (RESOLVE_ROUTE, the read `marco resolve` makes, which asks the local model) which saved routine was meant. A routine it names is PROPOSED on the `[proposed] ` wire line, the intake ends `clarify`, and nothing runs; the widget opens that routine for review with the proposed values. Anything else (learn, clarification, chat, mismatch, no Director) goes on to Director's screen reading as before. An answer to a question Director is holding is not resolved."
supersedes: []
affects:
  - interface
source_paths:
  - cmd/marco/intake.go
  - cmd/marco/propose.go
  - internal/outcome/outcome.go
  - plugins/overlay/acts.go
  - plugins/overlay/intake.go
  - plugins/overlay/resolve.go
---

# Resolve before reading the screen

Builds on [[ADR-321-a-misheard-name-goes-to-the-model]] and [[ADR-267-director-owns-route-resolution]].

## What happened

Second live try after ADR-321, all binaries at `b53c658`: "cloud paul stone" spoken into the widget.
The owner: *"it doesn't try the llm, it tries to run whatever i say then shows it failed"*. The
latest-attempt record: spoken, 3 words, Director, `failed`. Asked directly afterwards, the same
Director answered `cloud pull`, `resource: stone`, source `advisor`, in 3.5s.

The order was the defect. `invoke.Decide` found no exact match, so the intake handed the words to
Director's screen reading, which tried them as a desktop task and failed; the routine was looked for
only after that, from the widget, and what the person saw was the failure.

## Decision

- **The Director arm asks the resolver first** (`proposeRoutine`, `cmd/marco/propose.go`). It
  connects without starting Director; no Director means no resolver, and the words go on as before.
- **A named routine is proposed, never performed.** The whole `ResolveRouteResponse` is printed on
  `[proposed] ` (`outcome.ProposedPrefix`), a sentence follows for a terminal, and the outcome is
  `clarify`. Running is still the person's confirmation through `--play`.
- **The widget reads the line** in `streamChild`, before the console echo (it carries values somebody
  said), and `dispatchIntake` renders it with the same `offerResolved` an ask uses, without asking
  Director a second time.
- **Not resolved:** an answer to a pending question (the same `intent.ParseClarification` reading
  `Decide` uses); a learn, clarification or chat answer; a library mismatch; a run with no identity.
- Exact names and demonstrations are untouched: they are decided before this arm.
- **What it became, not what was heard.** The review line writes each value with a space after the
  colon (`resource: stone`; `routes.ParseInvocation` reads either), and a proposal's history row,
  status and HUD line name the routine rather than the transcript. The "you asked:" line is gone.

## Verified (Windows only)

- Tests below; mutants each killed: the engine's proposal step disabled, the pending-answer guard
  removed, the widget's read of the line dropped, `childRun.proposed` ignored.
- Not yet tried live: the stack must be rebuilt with `plugins\studio\build.cmd`.

## Not established

- Every phrase that no routine matches exactly now waits for the model before the screen is read
  (about 0.4s warm, ~9s cold). A screen task whose words resemble a routine is proposed that routine
  and not attempted; the review shows it, and nothing runs.
- A model's answer is not deterministic (ADR-321).

## Enforced by

- `cmd/marco` `TestAMisheardNameIsProposedBeforeTheScreenIsRead`, `TestNoProposalLeavesTheScreenReadingAsItWas`, `TestAnAnswerIsNotResolvedAsARoutine`.
- `plugins/overlay` `TestAProposedRoutineIsReviewedAndItsValueIsNotLogged`.
- `internal/outcome` `TestTheWireLineIsOneSharedLiteral`.
