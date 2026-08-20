---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - demonstrations
  - semantic-memory
  - marco-boundary
  - programs
---

# Rehearsal is attempt-scoped authority, and what it earns is readable Marco

**Design only.** Nothing here is implemented. No executor exists after this ADR, and the one
mechanical change it made was a test proving that nothing Marco has learned can act.

---
## Context

Marco can watch somebody, recognise the screens they moved between across sessions, ask whether
that is worth learning, watch one bounded demonstration, ask for a second when — and only when —
another would resolve a named gap, compare the two, and say whether the evidence hangs together.

The strongest state it can reach is `candidate_consistent` corroborated by an agreeing second
demonstration. That is a claim about **what happened**. It is not a claim that repeating the
sequence would reproduce it, and the gap between those two sentences is the whole of this
milestone.

A rehearsal is the first controlled experiment. It is also the first time Marco would generate
input, which means it is the first place the architecture can go badly wrong.

---
## What already exists, and what it is safe to reuse

| Machinery | Where | Reusable for rehearsal? |
|---|---|---|
| `ProcedureCandidate`, `CandidateAssessment`, `CompareCandidates` | `observe` | **Yes** — the eligibility input. Do not build a second scoring system. |
| `Memory.Recall` / `CompareStructure` | `observe` | **Yes** — the only way to say "this screen is that subject". |
| Four-value `ResolutionStatus` (`resolved`/`ambiguous`/`unobservable`/`absent`) | `directorapi` | **Yes, as the pattern** for step verification. It already encodes "could not look" ≠ "not there". |
| `foregroundGuard` | `cmd/director` | **Yes** — refuses input when the intended window is not in front, checked at the instant of execution. |
| Target provenance / window generation (ADR-011) | `perception` | **Yes** — the abort signal for "the window was replaced". |
| `SceneBecameStable` / `SceneBecameUnstable` | `observe/analysis.go` | **Yes** — settling, so a step is verified against a screen that has stopped moving. |
| `marcoexec.Preview` — lowers an operation **without** executing | `marcoexec` | **Yes**, for the lowering boundary later. |
| `internal/director/execute`, `plan`, `target`, `verify` | — | **Available, and deliberately not reached from the learning core.** |

**The finding that matters most:** every learned type — remembered subjects, relationships,
learning requests, candidates, assessments, captures, proposals — is defined in
`internal/director/observe`, and that package's transitive imports contain nothing that can touch
a desktop. The seven "cannot execute" invariants are therefore already true **structurally**, not
by discipline. `TestEverythingMarcoLearnsLivesWhereItCannotAct` now names those types so that
moving one somewhere more convenient fails loudly.

---
## Decision

### The five states, and the two that are not states

```
candidate_consistent   an observation. Two demonstrations agree.
rehearsal_eligible     a JUDGEMENT: every step Marco would take has an expectation it could check.
rehearsal_authorized   a PERMISSION, scoped to one attempt.
verified_procedure     an EXPERIMENT succeeded, once, under those constraints.
generated_marco        the durable thing: readable Marco.
```

`candidate_consistent` already exists as a `CandidateVerdict`. `rehearsal_eligible` is **not** a
new verdict — it is a judgement over the assessment, in exactly the shape `FollowUpFrom` already
uses, because a second scoring system is how two answers to one question appear.
`rehearsal_authorized` is not a state of the candidate at all; it is a separate, expiring grant.

**No confidence float anywhere.** Discrete evidence and named refusals, as everywhere else.

### Rehearsal eligibility: the rule, stated once

> A candidate is rehearsal-eligible only when **every action Marco proposes to take has a
> corresponding observable expectation it can check afterwards.**

Default is refusal. Derived from `CandidateAssessment`, never re-derived from the candidate.

| Refusal | Why |
|---|---|
| `not_consistent` | verdict is not `candidate_consistent` |
| `single_demonstration` | one example describes what happened once; a rehearsal is a claim about repeatability |
| `demonstrations_disagree` | Marco does not know which procedure to try |
| `unverifiable_checkpoint` | a step whose result cannot be checked is a step taken blind |
| `requires_text_entry` | see below |
| `unresolved_pointer_target` | see below |
| `near_capture_bound` | the demonstration may be missing its own ending |
| `endpoint_unrecognised` | nothing to aim at, or nothing to start from |
| `application_mismatch` | the candidate belongs to another application's namespace |

**The counterexample, stated rather than silently weakened.** The rule as written excludes a step
whose only visible effect is a *selection moving inside one screen* — `down` in a menu changes
which row is highlighted, and Marco's screen-state segmentation does not resolve rows. Three
options were considered:

1. Weaken the rule to "the run as a whole has an expectation". **Rejected** — it is the "fire
   everything and inspect afterwards" model this design exists to avoid.
2. Treat a within-screen step as verified by *the screen still being the same screen*. That is
   honest and weak: it proves nothing moved unexpectedly, and does not prove the selection moved.
   **Accepted, as `progress_unobservable`** — the step is taken, the screen is re-verified, and
   the result is recorded as an unobservable intermediate rather than as a success.
3. Require row-level perception first. **Deferred** — it is a perception milestone, not a
   rehearsal one.

So a `down, down, confirm` run rehearses as: verify source → `down` → screen unchanged
(`progress_unobservable`) → `down` → screen unchanged → `confirm` → **destination verified**.
Every step is still bounded, aborted on mismatch, and reported honestly; only the middle two
carry weaker evidence, and the record says which.

### Authorization: a new question, not a reused yes

**A previous yes to "watch me do it" is not authorization to rehearse.** They are different
permissions with different consequences, and the repository already has the machinery to keep
them apart: `Proposal.Ask` is a typed kind, and adding `AskRehearse` costs one constant.

What the request says, in the language a person actually reads:

> "I think I know how you get from the settings screen to the controller screen. I'd like to try
> it once: I'll start from the settings screen, press down twice and confirm, and I expect to end
> up on the controller screen. I'll stop the moment the screen isn't what I expected. Shall I try?"

No state ids, no confidence, no bounding boxes, no track numbers. What Marco thinks it learned,
where it starts, what it will try, what it expects, that this is one attempt, and that it stops
on surprise.

### Attempt-scoped authority

A grant names **one** candidate, **one** application, **one** starting subject, **one** attempt,
with a bounded input count, a bounded duration and the expected checkpoints and destination.

It expires when the attempt completes, fails, aborts, the target changes, or the user cancels.
It is **held in memory only** — it must not survive a Director restart, and the simplest way to
guarantee that is to give it no durable representation at all. It never becomes a capability and
never enters the Action Graph.

### One step at a time

```
precondition:  the current screen is the subject this step starts from
input:         one closed-vocabulary intent
settle:        wait for the scene to become stable
observe:       one perception cycle, target provenance proven
verify:        resolved / progress_unobservable / wrong_state / unobservable / ambiguous
next:          only on resolved or progress_unobservable
```

Verification vocabulary mirrors `ResolutionStatus` deliberately — it already encodes the
distinction that matters, that "I could not look" is not "it is not there".

Geometry alone never becomes semantic verification. That was settled by ADR-004 and nothing here
reopens it.

### Abort, fail-closed

Foreground changed · target generation changed · expected checkpoint unverifiable · a *different*
remembered subject appeared · perception unavailable · ambiguity beyond what the candidate
describes · input bound · duration bound · user cancelled · **Marco lost the ability to observe
the result of its own action.**

No best-effort continuation. No guessing the next step after a mismatch. The last one is the
subtle one and it is the most important: if Marco cannot see what its own input did, it stops,
because everything after that point would be blind.

### Text entry

`RequiresTextEntry` makes a candidate **rehearsal-ineligible**, and stays that way.

The learning system deliberately never retained what was typed, so there is nothing to replay,
and the fix is not to start retaining it. The path forward is a **typed parameter supplied at
invocation**: the procedure knows there is a text step and what it is for, the value arrives from
the person at the moment of use, and no captured string ever exists. That is a later milestone
with its own consent, and it is precisely the shape Marco already uses for secrets — the value is
resolved at run time and never lives in the route.

### Pointer actions

A normalised coordinate is evidence that somebody clicked somewhere. It is not a target.

A pointer step is rehearsable **only** when the click resolves to a semantic control — a nameable
role at that position, through the machinery ADR-017 built. Coordinates then travel as
corroboration, never as identity. `click at x,y` with nothing behind it stays
`unresolved_pointer_target` and rehearsal-ineligible.

This is not solved with application-specific selectors, ever.

### Start state

Before any input, the current screen must resolve to the candidate's source subject with
`MatchSame`. `candidate` and `insufficient` are refusals.

If the source is not recognised, **Marco does not act, and does not navigate there.** Navigating
back to the source would require already knowing another procedure — which is exactly the thing
being tested.

### Success, and what it produces

Correct source established · every step within authorization · every *verifiable* checkpoint
verified · destination recognised · no provenance violation · no unexplained transition.

A successful rehearsal produces a **separate `RehearsalResult` record**. It does not mutate the
demonstration into "verified". The observation is what happened; the experiment is a different
kind of evidence and lives beside it, exactly as `CandidateAssessment` lives beside the candidate
rather than inside it (ADR-021).

### Failure means something specific

Failure does **not** mean the demonstration was wrong or the user taught Marco badly.

| Reason | Consequence |
|---|---|
| `wrong_start` | assessment unchanged. Try again from the right screen. |
| `target_lost`, `perception_unavailable` | assessment unchanged. Nothing was learned about the procedure. |
| `settle_timeout` | assessment unchanged; rehearsal temporarily unavailable. |
| `wrong_state` — a *different* remembered subject appeared | **contradiction evidence** on the candidate. This is the one that means the inferred procedure may be wrong. |
| `ambiguous_destination` | another demonstration becomes useful. |

Observation and experiment stay distinguishable in the record. A rehearsal that failed because
the window closed says nothing about the procedure, and the record must not let a reader think
otherwise.

### Verified means one narrow thing

> Marco performed an authorized rehearsal of this candidate and observed the expected semantic
> progression to the expected destination.

It does **not** mean guaranteed, safe in every state, universally applicable, or executable
without future guards.

**One successful rehearsal is enough for the first verified state.** The smallest defensible
requirement: the corroboration bar was already paid at `candidate_consistent` (two agreeing
demonstrations), and requiring N rehearsals would be inventing a statistical threshold with no
evidence behind it. Repeated rehearsal is a later refinement if drift is measured.

### What verification earns: readable Marco

```
verified procedure → legal Core v1 Marco → ordinary compiler → ordinary runtime
```

**Director does not grow a private second automation language.** The durable thing Marco owns
after learning is a `.marco` file a person can read, edit, and delete.

**Is the information sufficient?** Checked against Core v1 rather than assumed:

| Needed | Carried today? |
|---|---|
| an actor to own the verb | no — but it is a *name*, and `codegen` already invents one |
| the verb name | no — the relationship has no name a person gave it |
| the ordered intents | **yes** — `DemonstrationStep.Intents` |
| an act to perform them through | **yes** — `os.marco` exports `Key`, `Click`, `Type` |
| a success ending | **yes** — `this is ok!` |
| a failure ending | **yes** — `this is failed with error "…"!` |
| the entry point | **yes** — `the App is a script.` |

**Two gaps, both naming, neither a language gap.** Core v1 can express everything a verified
procedure does; what is missing is what to *call* it. The natural answer is to ask the person —
they already answered "is this a settings screen?", and "what would you call this?" is the same
kind of question. **No language change is required, and none is proposed.**

The one thing Core v1 cannot express is a text-entry parameter, and that is why text entry is
rehearsal-ineligible rather than a language request.

---
## Consequences

- Rehearsal becomes buildable as a series of small milestones, each of which can be tested
  without generating input until the last one.
- The authority chain is closed by construction: seven "cannot execute" invariants are properties
  of the import graph, now named in a test.
- Marco's learned skills end up as readable Marco, not as a hidden procedure graph — which is the
  product direction the language reconciliation was for.


---
## Built (2026-08-10) — eligibility and the grant

`internal/director/observe/rehearsal.go`. Design followed, with two corrections the code forced.

**The eligibility rule was expressed per-step, and the discriminator changed.** The design
described `progress_unobservable` as "a transient checkpoint that stayed inside a screen". That is
not distinguishable in the data: a transient checkpoint Marco could not resolve is *precisely* the
case `CandidateAssessment` already refuses, so on that reading the middle verdict was
structurally unreachable — the same failure mode as the term-ratio denominator two milestones
back. The real discriminator is **where the step landed relative to where it started**: another
remembered screen is `directly_verifiable`, the screen it started on is `progress_unobservable`,
anything Marco cannot resolve is `unverifiable`. `down` in a menu lands back on the menu, which is
the case the design was reaching for.

Consequently `StepPlan.Expect` is populated for an unobservable step too — with the screen it must
*remain* on. That IS the containment check, and the two readings are told apart by
`Verifiability`, never by the field being empty.

**Two empty application names compare equal.** `strings.EqualFold(c.Application, application)`
passed a candidate belonging to no application through a check whose only job was confining it to
one. Now refused outright as `application_mismatch`.

**Authority outlives the session, never the process.** A rehearsal happens *after* observing —
Marco cannot try something while passively watching somebody else — so `observationRegistry` keeps
the last runner past `finish()` purely to hold the grant, and answers to a retired session's
rehearsal question are forwarded to it. Nothing is written; the next session replaces it.

### What is NOT killable by mutation, and why

Honest gaps, both defence-in-depth rather than sole gates:

- **The plan's `unverifiable` branch.** `CandidateAssessment` already emits
  `transient_checkpoint_unverifiable` for *any* step whose checkpoint does not resolve — including
  a non-transient one naming a subject memory no longer holds. Forcing the plan to treat unknown
  screens as checkable leaves every refusal in place, so no test fails. The branch stays: it is
  the reading that would survive the assessment changing its mind.
- **`Runner.authorizeRehearsal`'s one-active-grant guard.** `ProposalThresholds.MaxOpen` is 1 and
  `ReviewRehearsal` short-circuits on `granted`, so production cannot present a second rehearsal
  question while one grant stands. The reachable half of the invariant *is* tested
  (`TestOnlyOneAuthorizationIsActiveAtATime` kills the `granted` short-circuit); the guard itself
  is a fail-safe for a future second yes→grant path.

## Enforced by

- `TestARehearsalIsProposedOnlyWhenTheEvidenceAllows` — the whole chain through the production
  path: two agreeing demonstrations earn a question, a yes earns one inert grant, and the store
  holds exactly what it held.
- `TestOnlyARehearsalYesCreatesAuthority` — a yes to *learn this* or to *watch again* creates
  nothing.
- `TestTheRehearsalRefusalMatrix` — every designed blocker refuses; a contained unobservable run
  does not.
- `TestProgressUnobservableIsNeverSuccess` — the middle verdict is reachable, reported separately,
  and never counted as arrival.
- `TestAGrantAuthorizesExactlyOneAttempt`, `TestAGrantRefusesOutOfScopeClaims`,
  `TestAGrantMustBeScopedAndBounded` — single use, scope re-checked at claim time, malformed
  grants refused rather than defaulted.
- `TestAStaleQuestionCannotAuthorize` — the judgement is recomputed at yes time and fails closed
  both when the evidence got worse and when it merely changed.
- `TestADelayedRehearsalAnswerBindsToItsOwnCandidate` — an answer reaches for its own question's
  route, proven against a route that sorts ahead of it.
- `TestCancellingWithdrawsTheAuthorization`, `TestNoAuthoritySurvivesARestart`,
  `TestAttemptIdentityIsDistinctBackToBack`, `TestTheAuthorizationIsRaceSafe`.
- `TestAGrantIsInertAndHoldsNothingCaptured` — no executor-shaped method, no field that could hold
  a key, a label, a title or a pixel.
- `TestEverythingMarcoLearnsLivesWhereItCannotAct` — now naming `RehearsalJudgement` and
  `RehearsalGrant` too.
- `TestNoLearnedTypeOffersAWayToRunItself` — no learned type has a method named like an executor.
- `TestNothingLearnedClaimsToBeVerified` — the strongest state the loop can reach leaves
  `Verified` false; judging a candidate does not change it, and a grant has no `Verified` field.
- `TestTheObservationPackageCannotAct` (pre-existing) — the transitive import guarantee.
