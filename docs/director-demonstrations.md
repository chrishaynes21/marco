---
type: milestone
status: historical
---

# Demonstration recording and semantic procedure extraction

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

The user performs a task once. The Director extracts the procedure.

    Record semantics, never mechanics.
    Extract intent, not clicks.
    Every learned procedure must still compile into legal Marco.
    Recording never bypasses verification.
    The learned result must be explainable.

## What this changes

The goal layer closed the gap between "describe every step" and "describe the outcome".
This closes the one between "describe the outcome" and "show me once".

What is kept is not coordinates, timings, window handles or keystrokes. It is the semantic
execution: which verbs, aimed at which semantic targets, under which waits, and what
verification made of each.

    Focus item → Invoke Rename → editor opens → set value → commit

becomes

    Rename(new_name):  select the object → invoke the rename command →
                       set the editor to ${new_name} → confirm

## Where it sits

```
a request → observe → plan → policy → execute → observe → verify → action graph node
                                                                          │
                                                     Recorder.Observe(outcome)
                                                                          │
                                             Demonstration (semantic steps, node refs)
                                                                          │
                                                                    Extract
                                                                          │
                                              Extraction: Candidate + Decisions
                                                                          │
                                                          the user reads it and approves
                                                                          │
                                            Learned → goal.Procedure → the same registry
```

`internal/director/demo` holds all of it. `cmd/director` owns the recorder and the store
because a demonstration spans several requests and the CLI is a fresh process each time.

## Recording adds no observation path

The recorder subscribes to `Runtime.Handle`'s own outcome — a request that has already been
observed, planned, policy-checked, executed, re-observed, verified and recorded to the
action graph. It observes nothing, decides nothing and touches nothing.

That is what makes *recording never bypasses verification* structural rather than a rule
someone has to remember: there is nothing to record until an action has been verified.

## What a step keeps, and what it cannot

| kept | not kept |
|---|---|
| the semantic verb (invoke, select, confirm) | coordinates |
| the control's semantic ROLE (`rename_command`) | window handles |
| whether the target was pointed at, derived, or named | native ids, runtime ids, element ids |
| the semantic waits the step ran under | screenshots, OCR text |
| the KINDS of verification evidence | the evidence detail (it carries the user's content) |
| the action-graph node id | a copy of the node |
| what was typed, when it was ordinary text | anything the value layer called sensitive |

Asserted over the serialised form, not by inspection: `TestTheRecorderKeepsSemanticsAndNoMechanics`
marshals a demonstration recorded through the real pipeline and fails on any handle in it.

A **click is recorded as an invoke**. That is not a reinterpretation — the capability
ladder's lowest rung for invoke *is* a click, so the two are one act described at two
levels, and a procedure is made of the higher one. Recording "click" would throw away the
control's own Invoke pattern on every machine that has one.

## Goal recovery reads the actions

    Do not rely on the spoken phrase. Recover the goal from the semantic execution.

The phrase is what the user said they were doing; the actions are what they did, and the
actions are the half a learned procedure repeats. When the two disagree the ACTIONS decide,
and the disagreement is reported.

Recovery is a table of signatures over roles, verbs and ordering — `signatures` in
`extract.go`. `copy → paste` is a **duplicate**, not a copy: it claims more, and everything
it claims was observed. Two signatures that fit equally well REFUSE rather than letting
table order decide, the same rule the procedure registry already uses.

Rename is recovered from *the rename command was invoked, and something was typed after
it*. Deliberately not *the typing went into the inline editor* — that is the truer
statement and is not recoverable, because whether a control is an inline editor is known
from its CLASS, which lives in the world and not in the verified action record.

## What becomes a parameter

    Only parameterize user-provided data. Err toward fewer parameters.

One rule: **a value becomes a parameter when the user TYPED it.** Not when they chose it,
clicked it, or when the procedure named it.

That is what keeps "move to Downloads" constant when Downloads was a folder the user
clicked, and makes it a parameter when it was a path they typed — and it is decidable from
what was recorded rather than from what the extractor imagines the user meant.

Four exceptions keep typed text constant: the application's own name, the name of a
semantic control, a procedural verb ("rename" typed into a command palette), and empty text.

Parameters are named in order: the goal's own parameter name for the first typed value
(`new_name`, `folder_name`), then the FIELD's label slugified (`Customer name` →
`customer_name`), then `value_N`. A derived editor's label is never used — it is the
object's old value, and would produce `alpha_txt` for a rename.

### The subject is generalised too

A rename demonstrated on `Alpha.txt` is not a procedure that renames `Alpha.txt`. The first
step aimed at a CONTENT element — a list item, a row, a tree item — that carries no
semantic control role becomes *the object the user points at*, which is what lets the
learned procedure answer "rename this file to Q4" at all. Buttons, menu items and tabs are
controls the procedure uses, and are never generalised this way.

## What is refused

Two different questions, checked at two different moments.

**Safety** asks whether this demonstration may EVER become a procedure, and is decided when
the session CLOSES, so the refusal is a durable fact rather than a verdict something could
be asked to re-reach later with different rules. Sensitive values · authentication flows ·
payment flows · destructive controls · bulk iteration · anything that needed a confirmation
· anything that did not verify · a cancelled program · an unanswered clarification.

**Validation** asks whether this demonstration DESCRIBES a procedure, and runs at
extraction. Contiguous ordering · every step verified · no clarification · no replay-only
target · no consumed program-local value · at least two steps.

Both refuse the WHOLE demonstration. A validator that dropped the offending step would
produce a procedure the user never demonstrated, from the parts of one they did — a login
with the credential entry removed is a procedure that logs in without logging in.

Destructiveness is read from the CONTROL ROLE, not from the verb's reversibility. The
semantic vocabulary calls invoke, paste and submit irreversible because an ordinary undo
cannot be relied on to reverse them — a correct thing to say about a click on an unknown
button, and a useless test for "did this destroy something". Using it refused every
demonstration; `ControlRole.Destructive` is the vocabulary's own declaration of the controls
that lose work.

## Approval is a step nothing skips

    The extractor proposes. The user approves.

`Extract` returns an `Extraction`. The registry accepts a `goal.Procedure`. The only thing
that turns one into the other is `Approve`, and the type is the gate.

The service re-runs the extraction rather than accepting a candidate from the client. A
client that could hand the service a candidate could hand it an edited one, which would
make approval a way to AUTHOR procedures — and user-written procedure code is a non-goal.

## A learned procedure is an ordinary procedure

`Learned.AsProcedure()` is an adapter, and that is all it is. Its `Steps` function reads
stored data and emits typed directives; it evaluates nothing. From the registry's point of
view a learned procedure and a built-in are the same kind of thing, which is why expansion,
validation, binding, confirmation, lowering into Marco and verification are the built-in
path — they ARE the built-in path.

**The demonstrated value is never typed again.** A learned procedure declares the
requirement its parameters imply, so a rename with no new name is refused BEFORE expansion,
as a typed question. There is no path by which the example reaches a control.

### Precedence

A learned procedure and the built-in it was demonstrated against serve the same goal in the
same application, so specificity cannot separate them. **Provenance** does: between two
procedures that constrain the same amount, the one the user demonstrated here wins.

Deliberately not folded into the specificity score — a learned procedure does not constrain
MORE, it constrains the same and comes from somewhere better, and pretending otherwise would
make one number mean two things. Two LEARNED procedures for one task stay ambiguous: the
rule prefers what the user showed the Director over what it shipped with, and is not a way
for demonstrations to compete.

Learned procedures are disclosed — in the name, in `director procedures`, and in the plan of
every request they serve.

## Explainability

Every rule records a `Decision` at the moment it fires, by the code that fires it. Nothing
re-derives the reasoning later; a renderer that recomputed it would be a second
implementation of the extractor, and the two would disagree exactly when it mattered.

The decisions are stored WITH the approved procedure, so `director explain procedure`
answers months later with the reasoning the user approved rather than what a newer
extractor would produce today.

`Explanation` groups them by the question they answer:

```
Why this outcome?              Why is this a parameter?
Why wasn't this parameterised? Why was this refused?
```

## Commands

```
director demonstrate start           record the next task you perform
director demonstrate stop            end it and report what was kept
director demonstrate abandon         discard it
director demonstrations              what has been demonstrated
director demonstration <id>          one session, step by step
director extract <id>                the procedure it suggests (installs nothing)
director extract <id> --why          ... with the reason behind every decision
director extract <id> --approve      accept it into the procedure registry
director procedures                  built-in and learned, learned marked *
director explain procedure <name>    why it has the shape it has
```

## Replay and the action graph are unchanged

Learning is additive. A demonstration REFERENCES action-graph nodes and never duplicates
them; replay still replays actions, and knows nothing about procedures. Nothing in the
recorder, the extractor or the registry adapter is reachable from the replay path.

## Known gaps

- **None of this is live-tested.** `TestADemonstratedRenameBecomesAProcedureThatRunsOnAnotherFile`
  drives the real pipeline — observe, resolve, plan, policy, execute, verify, record — over
  scripted worlds, records through the real recorder, extracts, approves, registers and
  expands the result for a different file. That is the closest thing to the live validation
  that runs in CI, and it is not the live validation: no desktop has been demonstrated to.
- **The inline editor is not recognised at record time.** A verified action record carries a
  label and a role, not a control class, so a demonstration cannot tell the rename box from
  any other text field. Goal recovery works around it with ordering (see above). The fix is
  for the recorder to see the resolved ELEMENT rather than the resolved target, which is a
  change to what the pipeline reports.
- **Sensitive-field detection is a word list.** There is no password-field role in the
  Director's element model; a provider that reported one would be the better signal.
- **One application per demonstration.** A session that crossed two applications has no
  Application and is refused — a procedure is registered for an application, and one that
  spanned two is either two procedures or a workflow.
- **Thirteen of the fifteen outcomes have signatures.** `delete` and
  `close_without_saving` are deliberately absent: every demonstration of either trips the
  destructive-control refusal before recovery is reached, so a signature for them would be
  unreachable code. Asserted by `TestEveryRecoverableOutcomeIsEitherSignedOrRefused`.
- **A learned procedure is never generic.** It was demonstrated once, in one application,
  and claiming it works anywhere is a claim nobody made.
- **Forgetting takes effect on the next start.** The registry is append-only; a procedure
  that vanished mid-session would make one request behave differently from the next for
  reasons nobody could see.
