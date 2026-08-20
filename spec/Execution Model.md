---
status: reference
---

---
## Execution Model {#execution-model}
---

Marco defines three execution forms for invoking a phrase.

---
## Blocking {#blocking}
---

`do X...` is blocking.

The caller waits for the phrase to resolve before continuing.

### Example

Canonical:
```marco
do Save...
```

---
## Asynchronous And Observable {#async-observable}
---

`start X...` is asynchronous and observable.

The caller does not block. The started phrase's status may be observed.

### Example

Canonical:
```marco
start Save as SaveFlow...

when SaveFlow is ok?
```

---
## Fire-And-Forget {#fire-and-forget}
---

`execute X.` is fire-and-forget.

The caller does not wait for the phrase and does not observe its status.

### Example

Canonical:
```marco
execute Save.
```

---
## Relationship To Phrases {#relationship-to-phrases}
---

Execution changes how a phrase is invoked. It does not change phrase ownership.

Only the invoked [[Phrases|phrase]] owns its return.

### Example

Canonical:
```marco
start Save as SaveFlow...

when SaveFlow is failed?
	this is failed with that's error!
```

In this example, `SaveFlow` is observed from outside. The `!` still returns from the current owning phrase.

---
## Execution Order {#execution-order}
---

Execution is left-to-right within a line.

Each step produces a [[Frames|Frame]], updating `it` and `that`.

### Run-On Lines

Canonical:
```marco
do X, then do Y, then do Z.
```

Execution order:

1. `X`
2. `Y`
3. `Z`

After each step, `it` and `that` reflect the most recently completed Frame.

---
## Control Flow Interruption {#control-flow-interruption}
---

Execution stops when:

- a return (`!`) occurs
- a blocking wait occurs
- a branch captures control

Otherwise, execution continues.

---
## Branch Grouping {#branch-grouping}
---

`when`, `or when`, and `or?` form a branch group. See [[Core Concepts#Branching: `when` / `or when` / `or?`]] for the full grammar.

Rules:

- only one branch in a group executes
- `or when` adds additional conditions to the same group
- `or?` captures all remaining unmatched cases
- after a branch resolves, execution continues after the group

### Example

Canonical:
```marco
when Ready? do X. or? do Y.
do Z.
```

Meaning:

- if `Ready`, `X` runs; otherwise `Y` runs
- `Z` runs after the group, regardless of which branch matched

See [[Contracts#`or?` Exclusive Fallback]] for fallback semantics and [[Contracts#Unresolved Obligations]] for how branch groups discharge child statuses.

---
## Concurrency {#concurrency}
---

Concurrency in Marco is Frame-based.

- `start` creates concurrent [[Frames|Frames]]
- `when` observes them
- `wait` synchronizes (see [[Iteration#Waiting]])

There is no separate thread model at the language level.

For loop constructs (`for each`, `while`) and stream consumption (queues, feeds), see [[Iteration]].

### Example

Canonical:
```marco
start Save as SaveFlow...
start Render as RenderFlow...

when SaveFlow is ok? do Continue.
```

See [[#Asynchronous And Observable]].

---
## Runtime Guarantees {#runtime-guarantees}
---

Marco guarantees:

- ordered flows execute deterministically
- event listener ordering is not guaranteed unless explicitly controlled
- [[Frames|Frames]] are isolated unless shared
- Frame results freeze after return
- shared mutation requires [[Locking|locking]]
- unresolved statuses must be handled, transformed, or returned (see [[Contracts#Unresolved Obligations]])

---
## One-Line Lock {#one-line-lock}
---

Execution is left-to-right; control flow stops only at `!`, blocking wait, or a captured branch; concurrency is expressed through Frames, not threads.

---
## Open Questions {#open-questions}
---

- Whether `start X...` requires `as Name` to make the started Frame observable.
- Whether `execute X.` may target only named phrases or also inline phrases.
- Delivery guarantees and ordering rules for observed status changes from `start X...`.
