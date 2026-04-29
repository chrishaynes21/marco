---
# Frames {#frames}
---

---
## Definition {#definition}
---

Every line creates a [[Frames|Frame]].

A Frame is a built-in [[Actors|actor]] that represents execution. Frames are the runtime representation of graph traversal — see [[Graph#Frame Graph]].

---
## Frame As Actor {#frame-as-actor}
---

A Frame is not a separate runtime object — it is an actor with execution-lifecycle state.

Conceptually:

```marco
the Frame is an actor.
it's input is a set.
it's status is a status.
it's result is a set.
it's error is an error.
```

`input`, `status`, `result`, and `error` are the four core Frame slots. See [[#Frame Slots]] for access semantics.

Rules:

- every line creates a Frame
- phrase Frames remain active until resolved
- Frames may have [[#Capabilities|capabilities]] such as `rollback`, `retry`, `cancel`
- [[Contracts]] define valid Frame statuses and capabilities
- results are sets
- Frame behavior uses the same actor model as the rest of Marco

One-line lock: **Frames are not a separate magic runtime object; Frames are built-in actors that represent execution.**

A Frame:

- evolves over time
- has a status
- produces a result
- may contain an error
- may be observed
- may be waited on
- may have listeners
- may have child Frames
- stores structured data as sets

Every executable line creates a Frame.

### Frame Structure

A Frame contains four **core slots**:

- `input` — the received set for the Frame
- `status` — the current state of the Frame
- `result` — the returned set produced by the Frame
- `error` — any error that occurred during evaluation

And auxiliary state:

- `timing` — start and end times
- `listeners` — registered callbacks or watchers
- `child frames` — Frames created by nested phrases

Frame model:

```text
frame
|- input
|- status
|- result
|- error
|- timing
|- listeners
`- child frames
```

---
## Set {#set}
---

A set is the universal structured value in Marco.

Sets are used for:

- inputs (arguments)
- results
- actor state
- configuration
- errors
- frame data
- grouped values

Marco does not use "properties" as a concept. Everything structured is a set.

See [[Data Model]] for primitives, containers (lists, maps), assignment, construction, and access rules.

### Example

Canonical:
```marco
do Save.
```

This creates a completed Frame.

Canonical:
```marco
do Save...
```

This creates an open Frame (a phrase).

---
## Result {#result}
---

The result is the set produced by a Frame.

`that` refers to the most recent completed [[Frames|Frame]] — a child or observed Frame.

`that's <field>` accesses data on `that`'s result set.

When used in data-flow contexts (e.g., `with that`), Marco unwraps `that` to its result.

### Example

Canonical:
```marco
do Save...
    when ok?
        do Process with that.
```

Equivalent:
```marco
do Process with its result.
```

After return:

Canonical:
```marco
that
that's File
its result
```

See [[#Frame vs Result]] for the separation of execution and data.

---
## Reference Model {#reference-model}
---

Mappings:

- `this` = current focused subject (the Frame being resolved in process mode)
- `it` = current Frame
- `its` = current Frame set
- `input` = current Frame input set
- `that` = previous / watched Frame (most recent completed child or observed)
- `that's` = access to data on `that`'s result set (with `that's error` special-cased to the error slot)

For status checks use the bare-status predicate form: `when this <status>?`, `when that <status>?`, or shorthand `when <status>?`. See [[Lifecycles#Frame Status Predicates]].

### Example

Canonical:
```marco
when this is ok?
when this is failed?

its error
its result

input's File
input's User

that's File
that's User
```

---
## Frame Access {#frame-access}
---

`it` refers to the current Frame.

`its` refers to the current Frame set.

`input` refers to the current Frame input set.

### Example

Canonical:
```marco
its status
its result
its error
input's File
```

---
## Status {#status}
---

Status is part of the Frame.

Canonical state update:
```marco
it is Saving.
```

Canonical return:
```marco
this is ok with that!
this is failed with its error!
```

Status predicate:
```marco
when this is ok?
when this is failed?
```

Rule:

- status is expressed with `is`, not `has`
- returns use `this is <status>!`
- predicates use `<frame> <status>?` (no `is`)

`it's` is no longer canonical for status. See [[Lifecycles#Frame Status Predicates]] and [[Lifecycles#Returns]].

---
## Lifecycle Behavior {#lifecycle-behavior}
---

The Frame behaves like a lifecycle object with built-in actions.

Lifecycle behavior is expressed using normal execution:

Canonical:
```marco
do its rollback.
do its commit.
do its cancel.
```

Meaning:

- these are actions on the current Frame
- they may update status
- they may modify the Frame set
- they may emit new results

`rollback`, `commit`, and `cancel` live in the Frame set, not on the status.

### Example

Canonical:
```marco
when this is failed?
    do its rollback.
```

---
## Frame vs Result {#frame-vs-result}
---

Marco separates execution from data.

- A Frame represents execution and lifecycle.
- A result is a [[#Set|set]] produced by a Frame.

Core rule: **Frames act. Results do not.**

`that` refers to a Frame in process mode (the most recent child or observed Frame).

`that's <field>` accesses a field on that Frame's result set, except `that's error`, which accesses the Frame's `error` slot directly. See [[#Frame Slots]].

### Example

Canonical:
```marco
when that is failed?
    this is failed with that's error!
```

Meaning:

- `that failed?` inspects the previous Frame's status
- `that's error` reads the previous Frame's error slot
- `this is failed!` resolves the current Frame

---
## Capabilities {#capabilities}
---

Capabilities such as `cancel`, `retry`, and `rollback` are invoked on Frames, not on result sets.

### Example

Canonical:
```marco
do that cancel.
do that retry.
do that rollback.
```

Invalid:

```marco
do that's rollback.
```

Reason: `rollback` is a Frame capability, not data. Result sets are pure sets and contain no behavior.

Capabilities on the current Frame use `its`:

```marco
do its rollback.
do its commit.
do its cancel.
```

In graph terms, capabilities are operations available at a Frame node. See [[Graph#Contracts In Graph Terms]] and [[Contracts#Capabilities]] for how contracts declare which capabilities are available.

---
## Process Mode References {#process-mode-references}
---

Process mode is one of Marco's two reference modes — see [[Reference Modes]] for the declaration-mode counterpart.

Inside a phrase:

- `this` is the current Frame being resolved
- `that` is a child or observed Frame

Rules:

- `this` is the Frame you will return from
- `that` is the Frame you are inspecting or reacting to
- `this` may be mutated (status changes, return)
- `that` may not be mutated unless explicitly allowed by a capability

### Example

Canonical:
```marco
do save...
    when failed?
        this is failed with that's error!
    or?
        this is ok with that!
```

`this` resolves the current `save` Frame; `that` (implied by the bare-status shorthand) is the child Frame being inspected.

---
## Frame Slots {#frame-slots}
---

`input`, `status`, `result`, and `error` are the four core Frame slots.

Access semantics:

- `that's <field>` defaults to `that.result.<field>` — a field on the result set
- `that's error` is special-cased to `that.error` — the Frame's error slot, not a subfield of result
- `its error` accesses the current Frame's error slot directly

### Examples

Canonical:
```marco
that's User
that's File
```

Both resolve through the result set: `that.result.User`, `that.result.File`.

Canonical:
```marco
that's error
its error
```

Both resolve to the Frame's `error` slot directly.

### Why It Matters

Failure handling stays clean:

```marco
when that is failed?
    this is failed with that's error!
```

Not:

```marco
this is failed with that's result's error!
```

Errors live on the Frame, not buried inside the result.

Rule: results should not be used as the normal place for errors. Use the Frame's `error` slot.

See [[Lifecycles#Errors]] for the error model — string shorthand, canonical expansion, and custom error types.

To act on a Frame (rather than read its data), use `that` directly — see [[#Capabilities]].

---
## One-Line Lock {#one-line-lock}
---

Results hold returned data; Frames hold errors. Capabilities are invoked on Frames and constrained by [[Contracts]].

---
## Input Model {#input-model}
---

Actions, scenes, actors, and verbs receive input through `with`.

### Example

Canonical:
```marco
do Save with SaveArgs.
do Render with ScreenSet.
do Validate with UserForm.
```

`input` is the received set for the current Frame.

Input sets are implicitly accessible inside the phrase when unambiguous.

If ambiguous, use explicit access:

Canonical:
```marco
SaveArgs's File
SaveArgs's User
```

Example:

Canonical:
```marco
do Save with SaveArgs...

when failed?
    do its rollback.
    this is failed with its error!
```

Inside `Save`, args can be accessed either way:

Canonical:
```marco
input's File
input's User
```

Or implicitly when unambiguous:

Canonical:
```marco
File
User
```

---
## Ownership {#ownership}
---

Frames form a parent-child hierarchy.

Rules:

- phrases create parent Frames
- actions and nested phrases create child Frames
- child Frames belong to their parent (this is the `Children` link in the [[Graph#Frame Graph|Frame graph]])
- parent Frames observe and resolve child Frames
- a parent's resolution may propagate child state (e.g., child error → parent error via `this is failed with that's error!`)
- on parent cancellation, children are also canceled — see [[Lifecycles#Cancellation Propagation]]

This is **the** ownership lock for the runtime: every Frame except the root has exactly one parent, and the parent owns the child's lifetime.

---
## Frame Creation And Completion {#frame-creation-and-completion}
---

Every line creates a Frame.

- `.` completes the Frame immediately
- `...` opens a phrase and keeps the Frame active
- `!` closes the owning phrase and returns the Frame

---
## Return Propagation {#return-propagation}
---

When a Frame returns:

- the Frame is completed
- the parent Frame receives it as the most recent result
- `that` in the parent refers to the returned Frame's result set
- `it` remains the parent Frame

Return does not skip levels.

---
## Frame vs Snapshot {#frame-vs-snapshot}
---

A Frame is not a snapshot.

A Frame:

- is live
- evolves over time
- may emit multiple statuses

A snapshot is not part of the core language model.

---
## Updated Mental Model {#updated-mental-model}
---

Line -> Frame

Frame -> status + input + result (set)

Mappings:

- `it` = current Frame
- `it's` = current Frame status
- `its` = current Frame set
- `input` = current Frame input set
- `that` = most recent completed child or observed Frame
- `that's` = access to data on `that`'s result

Lock:

- Frames are live execution objects
- all structured data is represented as sets
- `input` is the Frame's received set
- `that` is the most recent completed child/observed Frame
- `that's <field>` accesses data on `that`'s result; in data-flow contexts (`with that`) Marco unwraps to the result
- `it's` refers to Frame status
- lifecycle behavior and capabilities are invoked on Frames, not on result sets

---
## Naming Without Focus: `as` {#naming-without-focus}
---

`as` names a Frame without changing focus.

After naming with `as`:

- The named Frame is accessible by its name
- `this` remains unchanged

### Example

Canonical:
```marco
do Save with GameState as SaveFrame.
```

After this line:

- `SaveFrame` refers to the produced Save Frame
- `this` is unchanged

---
## Naming With Focus: `the` {#naming-with-focus}
---

`the` names a Frame and focuses it, making it `this`.

After naming with `the`:

- The named Frame is accessible by its name
- `this` is now the named Frame

### Example

Canonical:
```marco
the SaveFrame is do Save with GameState.
```

After this line:

- `SaveFrame` refers to the produced Save Frame
- `this` is now `SaveFrame`

---
## `this` — Current Frame {#this-current-frame}
---

`this` refers to the current focused Frame.

When a Frame is named with `the`, it becomes `this`.

When a Frame is named with `as`, `this` is unchanged.

### Example

Canonical:
```marco
do Save with GameState as SaveFrame.
do Show Result.
```

After the first line:

- `this` refers to the result of `do Save`
- `SaveFrame` is accessible by name

After the second line:

- `this` refers to the result of `do Show Result`
- `SaveFrame` is still accessible by name

---
## Comparison {#comparison}
---

| Form | Behavior | Use Case |
|------|----------|----------|
| `as` | Names only; `this` unchanged | Observing async or later frame status |
| `the` | Names and focuses; `this` becomes the name | Current subject becomes captured Frame |

### Example

Canonical:
```marco
do Save with GameState as SaveFrame.
when SaveFrame is ok?
    the Result is do Show SaveFrame.
```

In this example:

- `SaveFrame` is named but not focused
- `this` is still the original frame until `the Result` is evaluated
- After `the Result`, `this` becomes `Result`

---
## Open Questions {#open-questions}
---

- Whether a Frame may be named more than once during execution.
- Whether renaming an already-named Frame rebinds or creates an error.
- Scope and lifetime of named Frames after the owning [[Phrases|phrase]] resolves.
