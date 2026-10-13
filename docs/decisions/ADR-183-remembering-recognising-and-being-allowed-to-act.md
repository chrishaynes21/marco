---
type: decision
status: partly-accepted
date: 2026-10-12
affects:
  - passive-observation
  - navigation
  - execution
  - semantic-memory
---

# ADR-183 — remembering a workflow, recognising where you are, and being allowed to act

## Status: the Workflow is built; the Binding is not, and the measurement says why

The owner authorised the first of the three objects below, and only the first:

> I now think there's enough evidence to begin persisting workflow intent and demonstration
> evidence, while keeping screen bindings temporary. […] When loaded in a new session, it should
> have no automatically trusted screen bindings. Cross-session recognition remains an unresolved
> capability, not something persistence should silently imply.

So `internal/director/workflowstore` exists and is the **Workflow** half of this note. The
**Binding** half is still unbuilt, because
[[Experiment-029-binding-a-screen-across-two-independent-folds]] measured the relation it would
need and found it binds the wrong screen at 0.886 while binding right ones at 0.556, with no
floor between them. The store therefore stores an arrangement as evidence and offers nothing
that compares one. The **Authorisation** half is unchanged and unbuilt.

## Context

ADR-181 built the middle of the product — a demonstration becomes an ordered draft, the draft
plans against the screen in front of it, a planned action lowers into legal Marco — and left one
thing between it and "tomorrow, Marco, enter Free Play": **a draft is session-local.** It names
screens identified by a count of rectangles in nine boxes, and those identities are allocated in
the order a session happens to meet them.

The owner declined to make cross-session identity the next engineering task, and was explicit
about why:

> I would not promote geometry-only screens into durable Places. But I also wouldn't make
> cross-session identity the next engineering task. […] If Marco cannot find a demonstrated
> control on a screen it already recognizes, persisting that screen's identity won't solve the
> problem.

And named the separation this note exists to hold:

> I favor persisting the learned workflow separately from its session-local visual bindings. […]
> This avoids conflating three different claims: we remember what you taught, we recognize where
> we are, and we can safely perform the next action.
>
> Persistence shouldn't imply recognition, and recognition shouldn't imply authority.

With the requirement that makes it more than a slogan:

> if a saved workflow contains only `visual_1`, `visual_2`, and `target_2`, it hasn't preserved
> enough information to re-establish those bindings later. The saved artifact needs its evidence
> or references to retained evidence — not merely session-generated IDs.

## Proposal: three objects, three lifetimes, three authorities

| | lives | says | can it license a press |
|---|---|---|---|
| **Workflow** | durably | this is what you showed me | no |
| **Binding** | one session | this is where I think I am | no |
| **Authorisation** | one action, one look | this press, now, here | yes, with a person |

### 1. The Workflow — durable, inert, evidence-bearing

Name, and the steps in the order they were demonstrated. Each step carries:

- its **position** in the walk, because a workflow is a sequence and the same control can appear
  at two positions with different expectations attached — `visual_1 --target_1--> visual_2` is at
  step 0 and step 11 of the Rocket League demonstration;
- its **cause**, so a load completing stays a thing to wait for and never becomes a thing to press;
- its **expected destination**, because an action nobody can verify is one nobody should take;
- the **evidence of the control**: every reading, and the containers that are not it;
- the **evidence of the screens** it leaves and arrives at: the arrangement itself, not a name;
- what is **unresolved**, in words a person can answer;
- an **audit record** of who confirmed which step and when.

What it must **not** contain, each for a reason already settled:

- **a press coordinate.** ADR-181 keeps the correlator out of the planner's reach so the
  coordinates are out of scope rather than merely unused; a store that wrote them back would undo
  that in one field.
- **a durable Place, an entry identity, or any node in the durable graph.** Saving a workflow is
  not a claim about the world. [[ADR-158-the-active-list-is-the-one-that-responds]] and the
  durable entry identities are untouched by this.
- **a session-generated screen ID used as though it meant something.** Inside the artifact a
  screen is a label — `screen_a` — scoped to the artifact, and the label's meaning is the
  arrangement stored beside it. A label that escaped into the durable graph would be exactly the
  geometry-only Place promotion that is not authorised.
- **a semantic claim.** `Support` stays two-valued. There is still no evidence that two readings
  of a rectangle are the same control rather than the same place.

A Workflow is **inert**. Saving one creates nothing, recognises nothing and licenses nothing. A
workflow that can never be bound again is still worth keeping: it is what the person taught, and
the honest failure is "I remember this and I cannot find it", not "I never learned it".

### 2. The Binding — one session, revocable, a proposal

For each screen the artifact names, which screen standing in THIS session corresponds to it, or
`unbound` with the reason.

- computed from the stored **arrangement** against the session's own, by the relation the screen
  model already uses — never by name, and never by the order screens were met;
- **at most one** current screen per artifact screen. Two candidates of comparable similarity is
  `ambiguous`, which is a refusal and not a tie to break: the whole subsystem's rule is that a
  hypothesis does not become an identity by being the best one available;
- **nothing is written back.** A binding is a reading of now. It does not update the artifact, it
  does not create a Place, and it does not survive the session;
- **targets are not bound.** Not at save time and not at bind time. A control is re-acquired from
  the artifact's stored readings against the proposals of the look in front of Marco, at the
  moment of acting — `observe.Reacquire`, unchanged. That is why the artifact stores readings and
  not points: a bound coordinate would be a stale answer that still looks like a fresh one.

Recognition is a claim about now, and it expires with the session that made it.

### 3. The Authorisation — one action, one look

A step may be lowered only when all of:

1. its bound source screen is the screen standing on **this** look;
2. reacquisition on **this** look is `matched` and not uncertain;
3. the step is confirmed in the artifact **and** the session carries live execution
   authorisation;
4. a destination exists to verify against.

Then `marcoexec.Lower`, then `Verify`, and stop on anything that is not `arrived`.

The stored confirmation is **audit metadata and never reusable authorisation**.
`internal/director/execute/replaypolicy.go` already states the discipline in its own words —

> Stored confirmation history is audit metadata only. It is never reusable authorization. […]
> Replay re-establishes that object from the snapshot and refuses when it cannot.

— and reacquisition **is** that re-establishment. A workflow is a deictic action with a snapshot:
"press this" means nothing until "this" is found again.

## What has to be measured before any of it is built

The design rests on one unmeasured assumption, and it is named here rather than discovered during
implementation: **does an arrangement bind the same screen across two independent folds?**

It is a count of rectangles in nine boxes. The proposer's yield varies enormously with the screen
— 90 to 153 proposals per look on `visual_2`, 400 on `visual_6` — and nothing has ever compared
arrangements across sessions. `ArrangementShapeSimilarity` exists because the owner asked that
question about yields and is deliberately not the rule; it was never put to this use.

An offline test answers it with the recordings already on disk and no new demonstration: fold
`rl-freeplay-1` as two independent sessions — two halves, or two strides so the frames differ —
and ask, for each screen of the first, whether it binds to the right screen of the second, how
close the best wrong answer came, and whether the right answer is ever beaten. Both relations can
be reported side by side, which is the comparison that has never been made.

If arrangements do not bind across folds, persistence is premature and the answer to "what
evidence does cross-session recognition need" is that it needs something the proposer does not yet
produce. Either way the question is cheap, offline, and answerable this week.

## The prerequisite was measured, and it does not hold

[[Experiment-029-binding-a-screen-across-two-independent-folds]] ran the experiment this section
asked for, the day after it was written. Three separate observers over the same recording — two
folds that share no counter, registry or signature, and an oracle whose only job is to say which
screens are really the same.

```
8 screens of fold A:  5 bound CORRECTLY, 2 bound WRONG, 1 ambiguous, 0 missed
correct bindings scored   0.556  0.611  0.688  0.833  0.901
wrong   bindings scored   0.559  0.886
```

The best **wrong** binding scored above four of the five correct ones, so **no floor separates
them** and raising the threshold cannot make one. Both wrong bindings were to screens the other
fold had never seen at all, where the only correct answer was to refuse — and
`VisualScreens.promote` has no refusal: it takes the best match above the floor and never reports
that it was choosing. Fold B also split one of the oracle's screens into three identities.

So the design above holds, and one layer of it is not ready:

- **The Workflow is unaffected.** Remembering what somebody taught, with its evidence, costs
  nothing and claims nothing.
- **The Binding needs an abstention the screen model does not have.** "No counterpart" has to be
  an available answer, decided on something other than being the best of what is lying around.
  The experiment's `ambiguous` is a sketch of it and is not sufficient on its own: the two wrong
  bindings were not ambiguous, they were unopposed.
- **An arrangement is not enough evidence for a durable claim.** It is a session-local stabiliser
  cue — its own file calls it "a far weaker claim" — and this is what that weakness costs when it
  is asked to carry identity across sessions.

This is the separation earning its keep rather than an argument against it. *Persistence does not
imply recognition* is exactly the right boundary, because recognition is the half that the
measurement says is not ready.

## What was built, and the two refusals it rests on

`workflowstore.Artifact` is this note's Workflow: a name, the steps in the order they were
demonstrated with their positions, each screen's **arrangement** under a label scoped to the
artifact, each control's **readings and containers** under a label of its own, what is
unresolved, and an audit record of who confirmed what. Saved from `rl-freeplay-1` it is 11,917
bytes: 21 steps, 9 screens, 9 controls, 9 readings, 5 containers, 9 arrangements, 13 audit
entries.

A reloaded workflow cannot run, for **two independent reasons**, and the independence is the
design:

1. **Nobody has agreed to it.** The file records who confirmed which step, and that record is
   read back as audit and never as agreement — `execute/replaypolicy.go`'s rule, honoured by
   there being no setter at all. `Confirm` remains the only thing that can confirm a step.
2. **Its screens are the artifact's own labels.** `screen_a` is not an identity any session
   mints, so `workflow.Plan` refuses because the screen standing now is not the screen the step
   names. This one survives somebody deleting the first.

```
1. step 0 has not been confirmed by anybody. Its evidence is geometric […]
2. this step was demonstrated on screen_a and the screen standing now is visual_1
```

Binding is a caller's decision, stated in full or not at all: a partial binding is refused and
names what is missing, because a workflow that bound what it could would act before discovering
it was lost.

### The leak that only the real recording found

Every structural field was clean and the first artifact saved from `rl-freeplay-1` still carried
two session identities — inside **prose**. `Unresolved` is a sentence for a person and it quotes
what it is about: *"this session could not tell target_2 from [target_10] on visual_2"*. Those
names mean nothing tomorrow and look like they mean something, which is the exact failure this
note forbids.

They are now rewritten with the artifact's own labels; an identity the artifact does not
describe becomes a phrase, because the ambiguity is the point of the sentence and the dead name
is not. No synthetic fixture produced this — one of the package's fixtures now writes what
`workflow.Learn` actually writes.

## The Binding half, conservatively

Built after the owner asked for it in the one shape the measurement allows:

> it may require externally supplied screen correspondence and use the stored evidence to
> validate or reject that correspondence.

`BindScreens` takes a caller's proposed correspondences and answers **bound**, **ambiguous** or
**no counterpart** — the three outcomes this note's Binding section asked for. It never selects a
best match. Two candidates clearing the floor is `ambiguous` however far apart they score,
because the ordering of those scores is precisely what Experiment-029 measured and found
unreliable.

**Agreement is necessary and not sufficient.** This can reject a correspondence the stored
evidence contradicts; it cannot confirm one. That is validation, and calling it recognition would
be the error this note exists to prevent.

[[Experiment-033-a-workflow-asked-for-in-another-session]] ran it across two independent folds:
**5 bound, 1 ambiguous, 2 with no counterpart** — and the ambiguous one had three candidates
above the floor with the best at 0.787, exactly the case a best-match binder would have got
wrong. The workflow as a whole was therefore refused, which is the correct answer.

Targets are still not bound, per this note's own rule. `Artifact.Targets` hands the stored
readings to `observe.ReacquireStored` — the registry's own matcher, now reachable without a
registry — and a control keeps the artifact's label in every session. **2 of 4 controls on a
bound screen were found again by a session that never saw the demonstration.**

## What is still required before anything can be performed

Named here so that "it saves and loads" is not mistaken for "it runs":

- **A binding that can be ESTABLISHED rather than validated.** The abstention exists now,
  and it fires; what does not exist is anything that can propose a correspondence
  without a person. Experiment-029's wrong bindings were unopposed rather than
  ambiguous, and an unopposed wrong candidate still binds.
- **A complete binding.** On the fold split measured, three of eight screens could not be
  established, so the workflow was refused as a whole.
- **The controls that cannot be found again.** Two of four on a bound screen matched on
  none of their looks. [[Experiment-033-a-workflow-asked-for-in-another-session]]
- **Fresh confirmation, per session.** Built: a loaded workflow has none.
- **Live execution authorisation and a destination to verify against**, both unchanged from the
  Authorisation section above and both unbuilt.

## Consequences

- **Three refusals become possible to state separately**, where today there is one silence:
  *I do not have this workflow*, *I have it and cannot tell where I am*, and *I know where I am
  and may not act*.
- **A saved workflow is not a durable Place and must not become one by accident.** A boundary test
  on the store would hold that, the way ADR-181's import walk holds the workflow package's.
- **The artifact is bigger than a list of IDs**, because it carries the evidence that makes it
  re-bindable. That is the owner's requirement and the whole reason this note exists.
- **Nothing is built.** [[ADR-182-two-controls-that-could-never-be-found-again]] is the work that
  came first, deliberately: a control Marco cannot find on a screen it already recognises is not
  made findable by remembering the screen for longer.

## Enforced by

- `internal/director/workflowstore` — round-trip, provenance (no session identity and no press
  coordinate reaches the file, asserted against the encoded form), authority (a loaded workflow
  is not executable and cannot plan), unsupported binding (partial bindings refused in both
  directions), versioning (refused in both directions, and this build will not write a version
  it does not own), corruption (seven damaged artifacts refused, never half-read), and prose
  provenance. **Mutation: 20 of 20 killed.**
- `internal/director/workflowstore/boundary_test.go` — an import walk: a saved workflow cannot
  reach `marcoexec`, `execute`, the action graph, semantic memory, the OS host or `os/exec`. It
  could not press anything or write itself into the durable graph.
- The Binding and Authorisation halves are still enforced by nothing, because they do not exist.
