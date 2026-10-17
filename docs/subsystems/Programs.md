---
type: subsystem
status: active
owners:
  - director
depends_on:
  - fusion
  - control-flow
  - marco-boundary
used_by:
  - goals
  - collections
  - demonstrations
updated: 2026-10-08
source_paths:
  - internal/director/program
  - internal/director/verify/outcome.go
  - internal/director/execute
  - internal/director/plan
  - internal/director/verify
---

# Programs

A program is an ordered sequence of semantic steps, validated as a whole and executed one at
a time with verification between them.

Not a batch. A batch commits to a plan made against a world that has since changed; a program
re-observes between steps, which is what makes the difference between "click Save then click
Yes" working and it clicking whatever now occupies the coordinates Yes used to.

## Splitting is a claim about somebody's words

"And" is not always a separator. *"Type hello and goodbye"* is one instruction whose TEXT
contains a conjunction, so a clause that does not begin like an instruction is rejoined to the
one before it — and a cut inside a quoted run is refused outright.

Two facts the splitter must not hold a private copy of, both found by one task:

- **Which words begin an instruction.** It kept forty of them by hand, and the list did not
  learn "change" when the editor did; the clause became the tail of an **append** and went into
  the document as text. The editing vocabulary is now derived from
  [[Editing]]'s own literals.
- **What was cut.** Fragments were rejoined with a hard-coded `" and "`, so "add milk, eggs and
  bread" came back as "add milk and eggs and bread". Nothing failed and the document held a
  list nobody wrote. The separator travels with the fragment and is restored verbatim.

See [[ADR-142-the-splitter-keeps-no-vocabulary-of-its-own]].

## A step can wait for its target to exist, and now go and find it

A step is resolved against the reading taken for THAT step, which is what makes "open File, then
click Save" possible — Save does not exist until File has been clicked. A target that is not in
that reading used to end the program.

It no longer has to. A resolution of `absent` earns a bounded search
([[ADR-143-a-search-is-a-wait-with-a-move-in-it]]), and a step that finds its target carries on
from the reading it was found in. That is the whole of the continuation: one request, and nobody
asked to insert "scroll down" between two clauses they did not think of as separate.

## Responsibilities

- Decompose a request into steps — by **splitting**, never by inferring an unstated step.
- Validate the whole request before performing any of it.
- Execute sequentially, verifying each step before the next.
- Maintain program context across steps.
- Refuse cleanly when busy.

## What it may not do

- Infer a step nobody asked for.
- Proceed past a step it could not verify — see [[ADR-007-no-progress-no-repetition]].
- Reach the desktop by any route but lowering — see [[ADR-005-legal-marco-only]].

## Related systems

- [[Goals]] — the layer above, which produces programs
- [[Semantic-Actions]] — what an individual step means
- [[Control-Flow]] — waits as step preconditions, and cancellation
- [[Action-Graph]] — what execution records
- [[Marco-Boundary]] — how a step reaches the desktop

## A procedure is finished when the person has what they asked for

Not when its steps ran. Measured live: "make a new folder called Reports" invoked the New menu,
chose the Folder entry, CREATED THE FOLDER — and stopped, because the verifier asked whether the
control it had pressed still reported itself selected. A menu closes when you choose from it.

Three changes, all in the existing mechanisms:

- **An intermediate action is judged by its transition.** A menu closing, taking all of its
  items, is evidence the press landed — paired with separate evidence that something appeared
  OUTSIDE the menu, because Escape closes a menu too. Neither alone concludes.
- **A step that needed nothing is counted and the procedure continues.** "Rename this file" with
  the file already selected abandoned a four-step rename at step one. The step was not
  unverifiable, it was unnecessary. No record is written, because nothing was performed.
- **Completion is the goal's declared OUTCOME**, stated in the `SuccessConditions` vocabulary
  that had never been set or read, and observed on a control Marco did NOT itself write to. The
  first version of that check reported `COMPLETED: text_field "Reports" is there` about an edit
  box holding Marco's own keystrokes while the folder on disk was still "New folder" — the same
  trap as the menu, one tier up.

Five bounded workflows then completed or failed honestly on a real desktop. See
[[ADR-224-a-procedure-is-finished-when-the-person-has-what-they-asked-for]] and
[[Experiment-073-marco-finishes-what-it-starts]].

## A request authorizes the task it asked for

Every one of the three workflows that completed in Experiment-073 stopped partway through to ask
whether it might press Enter. The person had asked for a folder; Marco opened the menu, chose
Folder and typed the name without asking, and then stopped at the keystroke that commits it —
because `confirm` is high-risk by its verb, and *which* irreversible thing a confirm commits is
not knowable from the verb. It is knowable from the TASK, and `goal.Safety` already said so:
"only the procedure's author knows what pressing it MEANS."

So a person's request now stands as authorization for the DECLARED steps of the procedure it
expanded into. Read by the confirmation gate and never by policy — a source guard held that
before this existed and caught the first draft, for the right reason: a permission layer that
could read a licence would decide risk from what the person SAID rather than from what the action
DOES. Policy still classifies `confirm` as high-risk; the gate one tier up, which already asked
"have they agreed to this?", now also asks "did they ask for this?".

It cannot waive a confirmation the PLANNER asked for about the control itself — "confirm" on a
button labelled "Delete account" is the same verb as on a folder name, and the planner now
records which raised the risk. It cannot cover a procedure that declared itself destructive,
irreversible or external until that procedure's one goal-level question is answered. And it
cannot travel: it names the request, the procedure, the step and the application, and a retry or
a replay carries none.

Measured live: four steps, one request, no prompts — and `delete scratch.txt` asked once, named
the file and the effect, and left it alone when declined. See
[[ADR-225-a-request-authorizes-the-task-it-asked-for]].

## Decisions

- [[ADR-225-a-request-authorizes-the-task-it-asked-for]]
- [[ADR-224-a-procedure-is-finished-when-the-person-has-what-they-asked-for]]
- [[ADR-143-a-search-is-a-wait-with-a-move-in-it]]
- [[ADR-142-the-splitter-keeps-no-vocabulary-of-its-own]]
- [[ADR-005-legal-marco-only]]
- [[ADR-006-unknown-is-not-false]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/program`, `internal/director/execute` and `internal/director/verify`
  unit tests
- `cmd/director/dryrun_test.go`, `cmd/director/runtime_test.go`

## Known gaps

See the *Known gaps* section of [[director-programs]].

## Milestone record

[[director-programs]] — the model, decomposition, whole-request validation, verification,
cancellation, program context.
