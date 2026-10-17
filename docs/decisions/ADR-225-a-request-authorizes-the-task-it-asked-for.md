---
type: adr
status: accepted
date: 2026-10-17
decision: a person's request stands as authorization for the declared steps of the procedure it expanded into, read by the confirmation gate and never by policy — and it cannot waive a confirmation the planner asked for about the control itself, cannot cover a procedure that declared itself destructive, irreversible or external until its one goal-level question is answered, and cannot travel to another application, another step or a retry
supersedes: []
affects:
  - execution
source_paths:
  - pkg/directorapi/tasklicence.go
  - internal/director/execute/confirm.go
  - internal/director/plan/semantic.go
  - internal/director/program/program.go
---

# ADR-225 — a request authorizes the task it asked for

## What was measured

Three workflows completed on a live desktop in
[[Experiment-073-marco-finishes-what-it-starts]], and every one stopped partway through:

```
make a new folder called Reports
  [1/4] invoke the New menu     — verified
  [2/4] choose the Folder entry — verified
  [3/4] set the name to "Reports" — verified
  [4/4] plan    confirm "Reports" (high risk)
        confirm the confirmation could not be put to you (no answer within 1m30s)
  BLOCKED: step 4 of 4 did not complete
```

The person asked for a folder. Marco opened the menu, chose Folder and typed the name without
asking, and then stopped to ask whether it might press Enter.

`confirm` is high-risk by its verb, and the comment that says so is right about why: *"committing
is the archetypal irreversible act: it sends, buys, posts or deletes, and which one is not
knowable from the verb."*

Not knowable from the verb. **Knowable from the task.** `goal.Safety` already says so in as many
words: *"only the procedure's author knows what pressing it MEANS. 'Invoke the Delete menu item'
is three low-risk semantic actions and one irreversible outcome."*

## The decision

A `directorapi.TaskLicence` is the person's request carried to the declared steps of the
procedure it expanded into. It reaches exactly one decision: whether a step's generic high-risk
prompt is already answered by the authorization the request itself gave.

### Where it is read, and the invariant that decided that

**In the confirmation gate. Never in policy**, which a source guard already held before this
existed — `TestPolicyNeverConsultsALicence` — for a reason that applies here exactly: *"a
permission layer that could see [a licence] would be deciding risk from what the person said
rather than from what the action does."*

So policy classifies the action precisely as it always did. `confirm` stays high-risk, and
`decision.RequiresConfirmation` is still set. What changes is one tier up, in the gate that was
already asking "has the person agreed to this?" — which now also asks "did the person ask for
this?".

The first draft of this change put the licence in `EvaluatePlan` and that guard caught it. The
guard was right and the architecture already had the answer: `coveredByGoalConfirmation` lives
in `execute` for the same reason.

### Two coverages, deliberately apart

`coveredByGoalConfirmation` asks whether an ANSWER somebody gave covers this action. Its clauses
are about an agreement: the risk level they agreed to, the object the question named, whether a
binding refresh changed the description.

`coveredByTheRequest` asks whether the REQUEST covers it. Its clauses are about a task: what the
procedure declared in advance, which of its steps this is, which application.

Collapsing them would mean one clause set applying where it does not belong. *A yes for medium
risk must not cover a high-risk step* is true of an answer. *A low-risk procedure's own commit
keystroke must not need a second question* is true of a task. Neither clause set can state the
other.

### Four things a licence cannot do

1. **It cannot waive a confirmation the PLANNER asked for.** Risk has two sources, and the level
   cannot tell them apart: "confirm" on a field holding a folder name and "confirm" on a button
   labelled "Delete account" are the same verb. The planner now records which — setting
   `Plan.RequiresConfirmation` when the TARGET raised the risk — and that field's doc has always
   said what it means: *"policy may add one; it may never remove one."* This is the first thing
   to rely on it.
2. **It cannot cover a consequential procedure unanswered.** A procedure declaring itself
   destructive, irreversible or externally visible is covered only once its goal question has
   been answered. One question about the whole task, which is what a person can meaningfully
   agree to.
3. **It cannot travel.** It names the request, the procedure, the STEP and the application, and
   it is minted per step by the code that knows this is a declared step. A step reached by any
   other path has no phrase on its licence, and the absence of a phrase IS the absence of
   authorization. A retry passes `nil` — a retry re-aims, and a task authorizes the step it
   declared rather than wherever a second attempt lands. A replay passes `nil` and keeps its own
   gate.
4. **It cannot be inferred.** Nothing derives one from a world or a plan.

### What is deliberately NOT consulted

`actionConfirmation.Destructive`, which is `!SemanticActionKind.Reversible()` — and that doc says
what it means: an action is reversible only when the Director can NAME the inverse. `confirm` has
no nameable inverse, so it is "destructive" by that measure, as is every commit keystroke there
is. Keying on it would refuse the entire case this exists for.

The consequential check is therefore the planner's, on the CONTROL. That is how deletion actually
arises in a procedure, and it is caught. What remains is trust in the procedure's declaration —
which is the trust `goal.Safety` was built on rather than something this adds, and the reason
procedures are hand-written and never generated.

## What it does, live

```
make a new folder called Reports
  4/4 policy allowed: "make a new folder called Reports" authorized this: confirm the name
  4/4 execute confirm via keyboard · enter
  COMPLETED: list_item "Reports" is there        (4 action(s) performed)
```

No prompt, and the account quotes the person's own words as the authorization — because "we did
not ask" needs a reason a person can recognise as theirs.

And the boundary, in the same session:

```
delete scratch.txt
  declined — delete scratch.txt — removes or overwrites something, cannot be undone?
  CANCELLED: cancelled — nothing was done
```

One question, naming the concrete file and the effect, at the goal level. Declining left the
file alone.

## The three categories, as implemented

| category | what decides it | behaviour |
|---|---|---|
| routine | the procedure declared itself non-destructive, non-irreversible, non-external, not high risk | runs within the request |
| context-dependent | the procedure's own declaration, made in advance by hand | asks once, at the goal, when the declaration says so |
| consequential | the procedure declares it, or the planner sees it in the control | asks, and no authorization waives the second kind |

## Consequences

- A routine multi-step task completes from one request. Measured: four steps, one request, no
  prompts.
- Grounding, freshness, foreground, cancellation and the capability ladder are untouched. A
  licence is read after all of them and decides only whether to ask.
- `Plan.RequiresConfirmation` is set for the first time since it was written, by the planner,
  when the control raised the risk.

## Enforced by

- `internal/director/execute` `TestARequestAuthorizesTheStepsOfTheTaskItAskedFor`,
  `TestAStepNobodyDeclaredIsNotAuthorized`,
  `TestAConfirmationAboutTheControlItselfIsNeverWaived`,
  `TestAConsequentialProcedureNeedsItsGoalQuestionAnswered`,
  `TestAnAuthorizationDoesNotTravel`.
- `internal/director/plan` `TestTheControlRaisingTheRiskIsAConfirmationThePlannerAsksFor` — the
  half that could stop being true without anything else noticing.
- `internal/director/policy` `TestPolicyNeverConsultsALicence`, unchanged and still passing,
  which is what keeps this in the gate rather than in the permission layer.
