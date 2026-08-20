---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - teaching
  - proposals
  - semantic-memory
source_paths:
  - internal/director/teach/teach.go
  - internal/director/teach/tail.go
  - internal/director/observe/walk.go
  - cmd/director/teachtail.go
---

# ADR-074 — one demonstration, every leg reviewed

Settings Home → Bluetooth & devices → Mouse. Marco captured **both** edges — the decomposition
into separately reusable route knowledge is the design working — and then rehearsed exactly one of
them. Teaching gets a single exempt rehearsal question, the terminal leg claimed it, and when that
rehearsal finished the lifecycle advanced to `Naming`. The first leg was never offered, the episode
ended, and the goal was unreachable from where the person had actually started.

Teaching a two-hop task must not require demonstrating it twice.

## Decision

**`ReviewingEdges` is a real, re-entrant phase.** The episode advances only when every required
edge is terminal. One edge is under review at a time: the phase points `Session.Route` at it and
the machinery underneath does exactly what it already did for a single-edge demonstration — raise
the question, take the answer, spend the grant, run the attempt. Nothing below the phase knows
there is a sequence.

### The order is the walk, not the store

Required edges come from `observe.DemonstratedWalk`, which reads `ShadowTotals.Crossings` — the
session's own ordered walk — and intersects it with the edges that actually became durable. Not
store recency, not subject id, not map order.

Order is load-bearing rather than cosmetic: after Marco rehearses A → B it is **standing at B**,
which is exactly the source B → C needs. The other order asks a person to walk back and forth
between screens to satisfy an ordering nobody chose.

The walk bridges `ScreenStateUnknown`. Real navigation goes through frames nobody can place, and
reading crossings pairwise produced an **empty** walk on live evidence — which silently restored
the single-edge behaviour this ADR exists to replace.

### Status is a fold, never a flag

`Session.Status()` derives `verified` / `partial` / `unverified` from the edges, and
`Session.Verified()` reports `n/m`. A route with one unreviewed leg must never read as a learned
one, and the legs that did verify are durable knowledge whatever happened to the others.

### The exemption is room to ask, not permission to act

`RehearsalOfferer` lets the coordinator name **which** edge is next. It does not raise a question:
the proposal machinery still judges the evidence. The allowance is one edge, of this demonstration,
in this episode, while that edge is the one under review. Each leg still needs its own explicit
yes.

## An absent question is not an answer

The first live two-edge run reviewed both legs and lost the first one anyway:

```
step 1 of 2  …  — unresolved (no answer created permission to try this step)
no rehearsal question: another_question_open
```

A **screen-naming** question held the interruption slot while step 1 was under review. The offer
produced no question, and the review read "nothing open, no grant" as an answer — writing the step
off before the Audience had been asked anything at all, then moving on to step 2.

Two silences look identical through `Question`, which reports only what is open right now:

- nobody has been asked yet, because something else holds the slot — **temporary**;
- somebody was asked and the answer was not a yes — **terminal**.

**A leg ends only on a positive signal that an answer happened, and only when that answer was not
a yes.** `RehearsalAnswer.AnswerToRehearsal` reports what the Audience actually said, from the
proposal record the ledger already holds, and `GrantDiagnoser.GrantRefusal` supplies the reason
when a yes created no authority. Absent both, the leg stays under review and is offered again.

The answer has to travel, not merely the fact of one. A bool was tried first and produced a second
live failure of the same shape: somebody pressed Yes and read back
`unresolved (the answer to this step was not a yes)` about that step. Saying yes writes two facts —
the proposal gains a response, and a grant is created — and there is an ordinary moment where the
first exists and the second does not. Absence of authority is not a denial; it is the middle of a
yes. So `confirmed` never ends a leg, and `no` and `not now` end it in the words the person used.

Retrying is bounded by two conditions, which is what keeps it from being a busy loop: a question
already open has nothing to ask for, and an answered or granted edge is past asking. One proposal
per route is enforced by identity, so a repeat either raises the question that could not be raised
before or changes nothing.

The fail-closed direction is deliberate: waiting costs a cycle, writing off a step costs the step.

## Enforced by

- `internal/director/teach/multiedge_test.go` — `TestBothDemonstratedEdgesAreOffered` (deleting the
  re-entry fails it); `TestEdgesAreReviewedInDemonstratedOrder`;
  `TestAThreeEdgeDemonstrationReviewsAllThree` (nothing counts to two);
  `TestASingleEdgeDemonstrationStillWorks`; `TestOneUnverifiedEdgeLeavesTheRoutePartial`;
  `TestTheSecondEdgeGetsItsOwnQuestion`; `TestTheSecondQuestionDidNotExistBeforeTheFirstWasDone`;
  `TestApprovingOneEdgeDoesNotApproveTheNext`;
  `TestABusyQuestionSlotDoesNotWriteOffTheStep`; `TestAnAnsweredEdgeIsNotOfferedAgain`;
  `TestAYesIsNotReadAsARefusalBeforeTheGrantExists` (the fake withholds the grant across the
  first review pass, so the test stands inside the gap rather than beside it);
  `TestARefusedStepSaysWhichRefusalItWas`.
- `internal/director/observe/walk_test.go` — the walk is the demonstrated order, and it bridges
  `ScreenStateUnknown` on the live crossings that produced the empty result.
- `cmd/director/learnwiring_test.go` — `TestTheTailReportsWhatWasAnsweredAboutARehearsal`.

## Related

[[ADR-073-a-place-is-a-composition-marco-saw]] — the walk can only be as good as the places it
runs between; three durable Settings pages with no twin is what made a two-edge route derivable at
all. · [[ADR-029-resolution-is-not-permission]] · [[Demonstrations]] · [[Learned-Plays]]

## One question, however many sessions raised it

A rehearsal proposal's identity is its route, and `ProposalLedger.ReviewRehearsal` dedupes on it —
so one session asks once. But **the ledger belongs to a session and the question belongs to a
route**, which outlives it, while the panel a person reads aggregates every session's open
questions. A multi-pass episode mints a copy per pass:

```
Questions open: 3
I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
```

No session did anything wrong. Three sessions each asked once, and the interruption budget — which
exists to protect a person from a queue — cannot see across them. `ReviewingEdges` made this
visible by keeping an episode alive for many passes; the mechanism predates it.

### The copies must exist

The obvious fix is to suppress the question in later sessions, and it is wrong. A yes creates
authority through `Runner.Respond`, which looks the proposal up in **its own runner's ledger**;
`observationRegistry.Answer` applies the semantic half via `g.last` — the newest runner. A question
only an older session holds is therefore visible and **unanswerable**. Suppression was implemented,
and `TestAVerifiedRouteBecomesOrdinaryMarco` failed by refusing to authorise a route the fixture
had said yes to — which is the same failure a person would hit.

The same constraint is already recorded for naming questions on `ProposeScreenName`: *a question in
only the first ledger is unanswerable because nobody can see it; a question in only the second is
visible and cannot be answered.*

### So it is read as one, and answered as one

- `Runtime.asking` and `Runtime.openQuestions` fold by proposal identity, and the **newest** copy
  is the one offered, because that is the copy an answer can reach.
- `observationRegistry.Answer` settles **every** copy. Settling only the one it found leaves the
  others open, and the panel re-offers a question already answered.

## Enforced by (continued)

- `cmd/director/proposalwiring_test.go` — `TestOneQuestionIsShownOnceHoweverManySessionsRaisedIt`
  (three sessions, one identity: shown once, newest wins, count agrees with the list);
  `TestAnAnswerSettlesEveryCopyOfTheQuestion`.

## Authority is per-route, and so is the silence around it

The review sat on `step 1 of 2: Home → Bluetooth — trying` forever. Measured, not guessed: both
legs judged **eligible with no refusals**, and `director rehearse` reported an active grant with
`source_mismatch` — an authority for **Bluetooth → Mouse** that nobody could spend, because Marco
was standing at Mouse.

How it got there is ordinary. The per-pass machinery asks about the route a session demonstrated,
which is the LAST leg; that was the only question on screen; the person said yes to it. The review
was still on the first leg.

Then two guards, both reading "some authority exists" where they meant "this route is authorised":

- `ProposalLedger.ReviewRehearsal` refused to ASK about leg 1 — `already_authorized`;
- `Runner.authorizeRehearsal` would have refused to authorise leg 1 — `already_granted`.

So the leg that had to go first could neither be asked about nor authorised, and nothing in the
lifecycle releases a grant for a leg it is not on. A `RevokeRehearsal` happens when a new session
starts, and `ReviewingEdges` starts none while it waits.

### Decision

**A grant names its route, and both guards must read it.**

- `granted` passed to `ReviewRehearsal` means *this route* is already authorised. Asking is not
  acting, so an authority for another route has no business silencing the question.
- A yes about a **different** route **supersedes** an outstanding grant. Still exactly one active
  authority, and it still came from an explicit yes about the thing it authorises — which is the
  whole of what that slot protects. A second yes about the **same** route is still refused: it
  queues no second attempt, which is what the rule was always for.

## Enforced by (continued)

- `internal/director/observesession/questionorder_test.go` —
  `TestAGrantForOneRouteDoesNotSilenceTheQuestionAboutAnother` (the deadlock: an unspendable grant
  for leg 2 must not refuse the question about leg 1, and the authorised leg is still not re-asked);
  `TestAYesAboutAnotherRouteSupersedesAnUnspendableGrant`.

## An answer has to reach a runner that can act on it

`step 1 of 2: Home → Bluetooth — trying`, again, with the grant fix in and no grant outstanding.
Measured rather than guessed:

```
the proposal:  answered, confirmed, evidence 9f4a56779b4f2389
the judgement: eligible, digest 9f4a56779b4f2389, inputs 1
the authority: none
```

The yes was not refused — every reason it could have been refused was checked and none applied.
It was **dropped**, and no refusal was written anywhere, because the code that writes refusals
never ran.

Answering is two acts: recording what was said, which the ledger does, and acting on it — the
durable write, the armed capture, the grant — which only a `Runner` can do, because the runner owns
the store, the bounds and the one ephemeral grant. `observationRegistry.Answer` performs the second
through `g.last`, the newest runner, and `Runner.Respond` could only act on a proposal in **its
own** ledger.

The comment justifying that said every question is asked at the END of a session, so by the time
anybody can answer, no session is running and the newest runner is the one that asked. **Teaching
breaks that assumption completely**: a teach episode runs bounded passes back to back, so the
newest runner is routinely a session that started after the question was raised and whose ledger
has never held it.

### Decision

`Runner.ApplyAnswer(proposal, response)` gives an answer its meaning for a proposal the runner did
not raise, and `Answer` falls back to it when `Respond` reports the question unknown. The proposal
is in hand at that point; there is no reason to require the runner to have asked.

This is the same shape as every other defect in this ADR: **a question outlives the session that
raised it, and the machinery around it was bound to the newest session.**

## Enforced by (continued)

- `cmd/director/proposalwiring_test.go` —
  `TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted` (a newer runner with an empty ledger
  becomes `last`; the answer must still reach memory). It asserts the RUNNER's half specifically —
  an earlier version asserted the ledger's and passed under the mutation.
