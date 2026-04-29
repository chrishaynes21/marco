---
# Iteration {#iteration}
---

Marco unifies looping, waiting, and streaming under a single rule: each is a phrase that produces [[Frames|Frames]].

- Loops and waits are Frame-producing phrases.
- Queues store work for pull-style consumption.
- Feeds stream events for push-style consumption.
- Iteration exposes `key`/`item` traversal state on each step.

---
## Typed and Untyped Collections {#typed-and-untyped-collections}
---

Collections may be typed or `any`.

### Typed

```marco
the Users is a list of User.
the SaveQueue is a queue of SaveRequest.
```

When iterating a typed collection, `item` has the declared type and field access is type-safe.

### Untyped (`any`)

```marco
the WorkQueue is a queue of any.
```

When `item` is `any`, the user must prove its type before accessing fields:

```marco
when WorkQueue has next?
    when item is SaveRequest?
        do Save with item.
    or when item is RenderRequest?
        do Render with item.
    or?
        log "unknown item type".
```

### Type Proof Predicate

`when <ref> is <Type>?` is a runtime type check that gates safe access in the branch body. Inside the body, `<ref>` is proven to be of `<Type>` and its fields are accessible.

This is symmetric with [[Data Model#Presence and Safe Access|`has`]]: `has` proves a field is present; `is <Type>` proves a value's type.

Rule: **Marco doesn't have casts at runtime — it has proofs.**

---
## For Each {#for-each}
---

`for each` iterates over sets, lists, maps, queues, and feeds when they expose iterable behavior. See [[Data Model]] and [[#Iterator Contracts]].

Canonical:

```marco
for each <Name> in <Collection>...
```

Anonymous:

```marco
for each in <Collection>...
```

### Loop Frame

A `for each` phrase creates a Frame. Each iteration exposes:

- `item` — the current value
- `key` — the current key or index (when applicable: maps, lists, indexed collections)
- `previous` — the previous iteration entry
- `next` — the next iteration entry
- `first` — true on the first item
- `last` — true on the last item

`first` and `last` may both be true for a one-item collection.

`item` is the canonical name for the current value across **all** iterable kinds — lists, maps, queues, feeds, channels, and user-defined iterables. Use it consistently. `value` is **not** part of canonical Marco.

### Examples

Canonical:
```marco
for each item in Users...
    do Render with item.
```

(A custom name may also be used — e.g., `for each User in Users...` — but `item` is the canonical default.)

Canonical:
```marco
for each item in Files...
    when first? do StartBatch.

    when previous exists?
        do Compare with previous's item and item.

    when last?
        do FinishBatch.
```

---
## Loop Control {#loop-control}
---

```marco
skip.
stop.
```

Rules:

- `skip` continues to the next iteration
- `stop` exits the loop phrase

---
## While Loops {#while-loops}
---

`while <condition>...` opens a phrase that repeats while the condition holds.

### Example

Canonical:
```marco
while Game Running...
    do Render.
    do PollInput.
```

Rules:

- `while` opens a phrase
- the condition is checked before each iteration
- the phrase must eventually resolve or explicitly continue looping
- the compiler may warn on obviously infinite loops without `wait`, `stop`, or an external condition

---
## Waiting {#waiting}
---

`wait until <condition>...` opens a Frame that observes until the condition becomes true.

### Examples

Canonical:
```marco
wait until that unlocked...
wait until SaveFrame ok...
wait until it's been 5 seconds...
```

Rules:

- `wait` opens a Frame
- `wait` observes until the condition becomes true
- `wait` may be interrupted by status or event checks
- `wait` resolves through normal Frame rules

See [[Lifecycles#Time]] for time-based conditions and [[Locking]] for `unlocked` waits.

---
## Queues {#queues}
---

A queue is an ordered container for work or values. Queues are pull/backlog containers.

### Example

Canonical:
```marco
the SaveQueue is a queue.

put SaveRequest into SaveQueue.

when SaveQueue has next? do Save with item.
```

Rules:

- queues preserve order
- `has next?` checks whether work is available
- inside the listener, `item` refers to the next queued value

---
## Feeds {#feeds}
---

A feed is a live stream of values or messages. Feeds are push/event streams.

### Example

Canonical:
```marco
the GameEvents is a feed.

write SaveRequested to GameEvents.

when GameEvents reads SaveRequested? do Save.
```

### Payloads

Feeds support the same payload pattern as channels — see [[Actors#Message Payloads]].

```marco
write SaveRequested with User to GameEvents.

when GameEvents reads SaveRequested?
    when that has User?
        do Save with that's User.
```

Rules:

- feeds are push/event streams
- feeds may be replayable later, but replay is not required for v1
- payload becomes part of the feed item's result set; access via `that` / `that's <field>`

### Queues vs Feeds

- a queue stores work
- a feed streams events

Use a queue when consumers pull on demand; use a feed when listeners react as events arrive. See [[Actors#Messaging Syntax]] for the actor-level messaging model that feeds compose with.

---
## Iterable Contract {#iterator-contracts}
---

`iterable` is a built-in [[Contracts|contract]].

A Frame, actor, set, list, map, queue, or feed may be `iterable` if it defines traversal behavior.

`iterable` defines:

- `item`
- `key`
- `previous`
- `next`
- `first`
- `last`

Collections such as `list`, `map`, `queue`, and `feed` provide `iterable` automatically.

Actors may become iterable by defining what traversal means.

### Example

Canonical:
```marco
the Party is an actor.
this is iterable.
this's Members is a list.

this's iterator does...
    for each Member in Members...
        this is ok with Member!
```

Meaning:

- `Party` can be used in `for each`
- `Party` defines its own traversal behavior by delegating to `Members`

Rule: `for each` requires the target to satisfy `iterable`.

One-line lock: **`iterable` is the contract that allows `for each`.**

See [[Contracts#Built-In Default Contracts]].

---
## One-Line Lock {#one-line-lock}
---

Loops and waits are Frame-producing phrases; queues store work, feeds stream events, and iteration exposes `item`/`key` traversal state.

---
## Open Questions {#open-questions}
---

- Canonical access for `previous`/`next` entries (`previous's item` is the locked form).
- Whether `for each` over a feed is bounded by feed lifetime or runs indefinitely until `stop`.
- Backpressure semantics for feeds with slow listeners.
- Whether multiple consumers may share a queue (work-stealing) or each gets a private cursor.
- Replay semantics for feeds beyond v1.
