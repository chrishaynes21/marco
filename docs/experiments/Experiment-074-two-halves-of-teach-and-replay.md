---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the owner's live desktop under standing authorization for ordinary reversible operations. Windows Settings for the demonstration and the repeat, Notepad on a disposable temp file for the typing case. An isolated MARCO_HOME throughout. No setting was changed: every navigation moved a pane and nothing else."
result: "recording-extraction-bounded-repeat-and-cancel-all-work-live-while-naming-and-reachability-do-not-and-the-two-learning-subsystems-each-hold-the-half-the-other-lacks"
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - docs/decisions/ADR-227-a-step-may-name-its-own-control.md
  - internal/director/demo/extract.go
  - internal/director/workflowstore/workflowstore.go
---

# Experiment 074 — two halves of teach and replay

The asked-for workflow, step by step: start a teaching session, demonstrate three to five
actions, have Marco associate them with observed controls and state transitions, **name and
save** it, have Marco reacquire the UI and **replay** it, and ask for a bounded repeat count that
can be cancelled immediately.

Measured against what is built, live, today.

## What happened

| asked for | result |
|---|---|
| start a teaching session | `director demonstrate start` → `Recording demo-20261018-000538.` |
| demonstrate 3–5 actions | three navigations, each `COMPLETED` and each verified |
| associate them with controls and transitions | `1 open the object the user points at` / `2 open "Accessibility"` / `3 open "Personalization"`, each *waits until: open changes something* |
| **name and save it** | **no.** The name is derived: `learned applicationframehost open_file` |
| **reacquire and replay** | **no.** `director goal "open gaming"` → *Not a goal* |
| bounded repeat count | **yes.** `do that again 3 times` → `COMPLETED: repeated 3 time(s)`, 3 of 3 verified |
| cancel immediately | **yes.** `do that again 20 times`, then `director stop` → `CANCELLED: cancelled after 4 of 20` |

## The three findings

### 1. A demonstrated step was keeping nothing about what it opened

The extraction above is what it says **after** [[ADR-227-a-step-may-name-its-own-control]].
Before it, steps 2 and 3 read `open the focused control` twice: a constant step kept only its
`Role`, and a settings navigation item has none. Three demonstrated destinations became a
procedure with one — and the mutation run showed that wording was flattering it. A step with
nothing to aim at does not reach a focused control; `SemanticOpen` needs a target, so the
procedure could not expand at all and would have refused at step 2 with *open needs a control to
act on*, one step in. The proposal read plausibly and was unrunnable.

The fix's own test then caught a second defect the live run would not have shown for a while: the
label routed through `Directive.Target` attached the procedure's **object binding** to all three
steps, so every one of them came out aimed at the bound file. A control a procedure passes
through is not the object it is about, and the two now have separate fields.

### 2. Nothing a person can type reaches a learned procedure

`director extract … --approve` reports *"learned applicationframehost open_file" is installed and
will serve open from the next request*, and `director goals` lists it beside the built-in. Both
are unreachable:

```
director goal "open gaming"            → Not a goal.
director goal "open the gaming settings" → Not a goal.
director goal "open settings"          → open_settings   (a different goal entirely)
```

`goal.OpenFile` has **no entry in the pattern table**. So `genericOpenFile` has never been
reachable either, and approving a procedure to override it changes nothing a person can observe.

This is a routing decision rather than a missing line. `Pipeline.IsGoal` is consulted **before**
anything observes and wins over the semantic-action reading, so adding a bare `open` prefix would
reroute every *"open the file menu"* in the system away from `uiact` and into goal expansion. The
narrowest safe form is a prefix that cannot collide — one that names a destination rather than a
control — and it is proposed, not taken, in
[[ADR-223-proposed-an-interpreter-asked-only-where-nothing-matched]]'s spirit: measure the collision
first.

### 3. The unit of learning is a goal, not a sequence

The deeper reason naming has nowhere to go. `demo` reads a demonstration as *a better way to
achieve something Director already has a word for*: it recovers a `goal.Kind` from the semantic
shape of what was done, generalizes the first step into the procedure's parameter, and freezes
the rest. Three demonstrated destinations therefore became **"open ⟨X⟩, then Accessibility, then
Personalization"** — a correct reading of one intention and a wrong reading of three.

*"A thing I am calling my settings tour"* has no representation in it. The name is derived
because there is nothing for a person's name to name.

## And the repeat, which does work

`do that again 3 times` ran three full observe → resolve → plan → policy → execute → verify
cycles, re-resolving `"Back"` from its query each time, and reported each outcome separately:

```
✓ 1/3  READY  "Personalization" became unselected and "Ac…
✓ 2/3  READY  "Gaming" became selected and "Accessibility…
✓ 3/3  READY  "Bluetooth & devices" became selected and "…
completed 3 of 3 — completed
```

No per-step confirmation, which is [[ADR-225-a-request-authorizes-the-task-it-asked-for]] doing
its job. Cancellation mid-loop reported the honest count, 4 of 20, with the stopping point named:
*"it will stop at the next safe point"*.

Two limits worth recording. It repeats **the last action**, not a saved sequence — nothing can
ask for "my settings tour, three times". And it refuses to rebuild an `edit`:

```
do that again 3 times  →  BLOCKED: no way to rebuild a "edit" action
```

Click, focus, semantic and window-move rebuild; typing does not. That refusal is defensible
rather than a gap — repeating a typed value has to choose between replacing and appending, and
nothing in the record says which — but it means the repeat is unavailable on exactly the actions
that change a document.

## Two subsystems, each holding the half the other lacks

| | `demo` | `workflow` + `workflowstore` + `workflowrun` |
|---|---|---|
| what it watches | Marco's own verified outcomes | the person's own clicks, over frames |
| substrate | the accessibility tree | pixels, and what OCR read |
| a person's name for it | no — derived | **yes**, `Draft.Name`, and saving refuses without one |
| reachable by request | no | by artifact path, `director perform-workflow` |
| step targeting | role, then label | screen + control, scoped to the artifact |
| authorization | request-scoped, no per-step prompt | **per step, every time**, by construction |
| live capture | `director demonstrate start` | none — the artifact is written offline by a diagnostic |

`workflowstore`'s own doc states the refusal that makes the right-hand column safe and also makes
it unfinished: a saved workflow's identities are `screen_a` and `control_1`, scoped to the
artifact, so *"a loaded workflow handed to `workflow.Plan` is refused because the screen standing
now is not the screen the step names"*, and a step read back *"comes out UNCONFIRMED regardless"*.
Binding those labels to the session is the caller's decision, recorded as the caller's — and
[[Experiment-029-binding-a-screen-across-two-independent-folds]] measured that binding as not
ready: five screens of eight bound correctly, two bound confidently to screens the other fold had
never seen, and no threshold separates them.

## What prevents this transferring to a pixel-only game menu

Stated precisely, because the answer differs by subsystem.

**The saved format is already clean.** `demo`'s serialisation test forbids handles, element ids,
runtime ids and coordinates, and `workflowstore` forbids press coordinates, session ids and
durable places. Neither format requires an accessibility element id, which was the constraint
asked for, and ADR-227 adds the one targeting form a pixel surface can actually fill: a label is
what OCR reads.

**The live capture is not.** Three things block it, in order of difficulty:

1. **No live pixel capture path.** `director capture-game-session` records frames and observed
   input on one clock; `perceptionreplay -frames -workflow -save` turns a recording into an
   artifact. `director` itself never writes one, and the file it writes is inert by design.
2. **Per-step confirmation.** Every step of a loaded workflow is UNCONFIRMED, so a replay asks
   once per step — the opposite of what ADR-225 just established for requests, and the two
   authorization models have no connection between them. A request-scoped licence for a replayed
   workflow would have to say what a demonstration authorizes, which nobody has yet said.
3. **Screen identity.** Experiment 029's measurement, above. Recognising the menu you are standing
   in is the prerequisite for replaying anything into it, and it is the open problem rather than
   the plumbing.

**What does not block it:** the demonstration itself, the explainability, the bounded repeat and
the cancellation. All four were measured working today.

## The mutation run

Seven mutations of ADR-227's four lines, each run against
`internal/director/{demo,goal,execute}`:

| deleted or inverted | result |
|---|---|
| the recorded label never reaches the candidate step | killed |
| the candidate step's label never becomes a directive | killed |
| the label goes through `Directive.Target`, so the step inherits the procedure's object | killed |
| `reference`'s label branch deleted outright | killed — *open needs a control to act on* |
| the proposal does not say which control a step named | killed |
| a label outranks a role in the directive | killed |
| a label outranks a role in the description | killed |

Two further attempts were **non-mutations** and are recorded as such rather than as survivors:
assigning `d.Target = d.TargetLabel` inside `reference` mutates a local copy *after* the
expansion loop has already set that step's binding to nil, and one regex failed to match its own
target so nothing was changed at all. The second cost a round of believing a line was untested;
the by-line deletion of the same branch killed immediately. A mutation that does not apply looks
exactly like a mutation that survives.
