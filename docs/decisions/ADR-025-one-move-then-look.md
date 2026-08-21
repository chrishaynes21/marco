---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - demonstrations
  - perception
  - marco-boundary
---

# One move, then look at the board again

The first milestone in which something Marco learned by watching may move a real computer.

---
## Context

[[ADR-024-a-dry-step-is-not-evidence]] left one authorized step lowered through legal Marco into a
recording host. Everything above the host was already the real path. Roadmap 22 changes the host —
and nothing else above it — and adds the whole of what must be true *around* one real input.

The governing rule is the one this file is named for. Marco gets one action. Then it looks. Then
it classifies. Then it stops. It does not get a second action because the first one looked
promising.

---
## Decision 1 — the starting screen comes from perception, never from an argument

Roadmap 21 accepted the source subject as scope input, which was correct for a milestone that
could not act. It is not correct now.

`Live.establish` watches the window for a bounded run of observations, derives the screen state,
and resolves it through `observe.SignatureOfState` → `Memory.Recall` — the same path a
demonstration capture uses. If the resolved subject is not the grant's source, or memory does not
recognise the screen, or more than one remembered subject fits, **nothing is sent**.

The grant says the user demonstrated A → B, which is a claim about history. Whether the interface
is showing A *at this moment* is a question only looking can answer. Assuming it because the grant
said so is the stale-ordinal mistake this repository has made before.

---
## Decision 2 — the order, and where the permission is spent

```
look → compare → CLAIM → re-check the window → emit → settle → look again → classify → stop
```

**The claim is after the comparison and before the emission.** Before it, Marco has not yet been
able to act and a mismatch costs the user nothing. After it, an input is possible, so the
permission is gone whatever happens next — a claim-fail-retry loop would turn "one attempt" into
"as many attempts as it takes to get past the setup".

**The final guard is as late as it can be.** Between establishing the subject and emitting, the
user may have alt-tabbed. The window is re-acquired and compared on identity, process and
generation — never the title, which changes while a window lives. If anything moved, zero input,
and the attempt is cancelled with the grant spent.

That race — verify the screen, the user switches, the keystroke lands in their email — is the one
failure this milestone most needs to make impossible.

---
## Decision 3 — a refusal is not a result

`RehearsalResult` exists only when input was emitted. Everything before that returns an error
carrying a closed refusal.

"Marco declined to try" and "Marco tried and it went wrong" are different facts about the world,
and a reader who cannot tell them apart cannot audit anything. The line is drawn at
`StepEmission.Reached` — the moment the program is handed to the runner. Before it, nothing was
sent. After it, part of the run may have landed and claiming otherwise would be a claim that is
not available.

That is also why **host failure is an outcome (`input_failed`) rather than a refusal.**

---
## Decision 4 — the two unobservables are not the same word

| | |
|---|---|
| `progress_unobservable` | a property of the STEP, known before Marco tried. Containment held: same window, same application, same screen. It does NOT mean the selection moved correctly. |
| `unobservable` | a RUNTIME failure to look. Input went out and Marco cannot say what came of it. |

Collapsing them would let every broken observation read as a contained success — the same
collapse the three-valued `StepVerifiability` exists to prevent, one layer further down.

And `directly_verified` requires the SPECIFIC expected subject. A screen that became a different
remembered screen is `wrong_state`. Treating change as success is how a procedure gets promoted
for going somewhere nobody asked for.

---
## Decision 5 — settling is watching, not sleeping

The post-input wait watches the SCREEN STATE — the same evidence the classification reads — and
stops the moment it has held for three consecutive observations, bounded at eight. Outcomes are
`stable`, `still_changing`, `target_lost`, `interrupted`.

Not `execute`'s `RegionStable` waiter: that lives behind the execution pipeline, which the
rehearsal path must not be able to reach, and it answers a question about pixels where this layer's
evidence is screen states. Not a fixed sleep either — a condition is falsifiable where a duration
is not.

---
## Decision 6 — being able to act is a decision somebody made

`rehearse.Live` is constructed with perception and memory, neither of which can affect anything,
and is **incapable of emitting** until `WithActuator` is called. That call happens in
`cmd/director/rehearserun.go` and nowhere else.

The trigger is equally explicit: a typed `ObserveRehearse` request, reached by `director rehearse`.
`--live` chooses the real host; without it the whole path runs into a recorder. No session spends a
grant, no session-end review spends one, and nothing spends one on a timer — a later session
**withdraws** an unspent authorization rather than leaving it lying around.

`rehearse.Live` also takes a `Recogniser`, not `observe.Memory`: one method, and it is a read.
A rehearsal has no write to reach.

## Decision 7 — a dry run concludes nothing about the world

With a recording host the application does not move, so classifying the post-input screen would
report *"the screen became A, which is not what that step was for"* — which reads as a failed step
when in fact nothing was sent.

So `Live` is told whether its actuator reaches a computer. When it does not, the emission is the
whole of what happened: `Outcome` stays empty and the record says *"Marco did not try it"*. It
changes nothing about WHAT is sent — the same program goes to the same interface either way — and
everything about what Marco is entitled to conclude afterwards.

---
## Consequences

- A `RehearsalResult` is the first record in this system entitled to say Marco tried something. It
  is **not** verification of the procedure: `ProcedureCandidate.Verified` stays false, no Marco is
  generated, and nothing is promoted.
- Results are carried in the response and are **not durable**. Folding experimental evidence into
  the learning loop changes what an assessment means and deserves its own ADR.
- [[ADR-005-legal-marco-only]] holds unchanged: the live input is the same `KindNavigate`
  operation, lowered to the same program, compiled by the same compiler.

## Enforced by

- `TestOneAuthorizedStepIsLoweredToARecordingHost` — the whole chain through the real protocol
  request, ending in exactly one recorded call.
- `TestAnAttemptThatReachesTheExpectedScreenIsDirectlyVerified`,
  `TestAnAttemptThatReachesAnotherScreenIsWrongState`,
  `TestAContainedStepReportsProgressUnobservable`,
  `TestPerceptionFailingAfterInputIsUnobservableNotContained`,
  `TestPerceptionThatCannotSeparateSubjectsIsAmbiguous`,
  `TestTheWindowChangingAfterInputIsTargetMoved`,
  `TestTheWindowDisappearingAfterInputIsTargetUnavailable`,
  `TestAHostFailureIsAResultAndNeverRetried` — the classification matrix.
- `TestAWindowThatChangesBetweenCheckingAndActingSendsNothing` — the final guard, load-bearing.
- `TestTheResultComesFromAFreshObservation` — the classification cannot come from a pre-action
  sample.
- `TestAMultiStepRouteRunsOneStepAtATime` — one move, even when it verified and two more were
  authorized.
- `TestAnAttemptOnTheWrongScreenSendsNothing`, `TestAnUnrecognisedScreenSendsNothing`,
  `TestAnAmbiguousStartingScreenSendsNothing` — perception decides where Marco is.
- `TestARunnerWithoutAnActuatorCannotSendAnything`, `TestALiveRehearsalRefusesWithoutARealHost`,
  `TestNoSessionEverSpendsAGrantByItself`, `TestANewSessionWithdrawsAnUnspentAuthorization` —
  acting is an explicit decision, twice over.
- `TestARehearsalCannotWriteToMemory` — the dependency is one read.
- `TestADryRehearsalDrawsNoConclusion` — a dry run reports what it would send and concludes
  nothing about a screen it never touched.
- `TestCancellingBeforeInputSendsNothing`, `TestCancellingAfterInputKeepsTheFactThatItHappened`.
