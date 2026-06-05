---
# Contracts {#contracts}
---

---
## Definition {#definition}
---

A contract defines a required interface.

Contracts impose obligations: every member declared on a contract must be implemented by adopting actors, actions, scenes, or [[Frames|Frames]]. Contracts apply to all four entity kinds.

A contract may define:

- allowed statuses (with shape)
- declared members (`it can X with Y, gives Z, and is W.`)
- allowed Frame capabilities (see [[#Capabilities]])
- caller obligations

Fields declared as required by contract are always safe to access; optional fields require a presence proof — see [[Data Model#Presence and Safe Access]].

### Member Declaration

Members are declared with the compound `it can` form:

```marco
it can <Action> [with <Input>] [, gives <Output>] [, and is <Contract>].
```

Each clause is optional except the action name. Multiple clauses combine with `, and`.

### Example

Canonical:
```marco
the Game is a contract.
it can Save with an optional File, and is Saveable.
it can Load, and gives a File.
it can Render with a File, gives a Stage, and is Quick.
it can Run with Engine, and is waitable.
it can Play with a feed UserInput, and is waitable.
```

Reading: the `Game` contract requires implementations to provide `Save`, `Load`, `Render`, `Run`, `Play`. Each member's input, output, and adopted contract are inline.

`optional` marks an input as not required — callers may omit it; the implementation must use [[Data Model#Presence and Safe Access|`has`]] to check for it.

### Adoption

Actors adopt contracts via `is`:

```marco
the RocketLeague is an actor.
it is a Game.
```

Reading: `RocketLeague` adopts the `Game` contract — it must implement every declared member.

### Allowed Statuses

Contracts declare allowed Frame outcomes with `this allows`:

```marco
the Saveable is a contract.
this allows Saved with File.
this allows failed with an optional error.
this allows canceled.
```

Rules:

- each status may be declared once per contract
- each status has exactly one shape within a contract
- statuses belong to Frames, not sets
- `with <Type>` declares the result shape; `with an optional <Type>` makes that shape optional

---
## Action Contract Declaration {#action-contract-declaration}
---

An action adopts a contract via `is`:

```marco
that is <Contract>.
```

### Example

Canonical:
```marco
it can Save...
that is Saveable.
that does...
    ...
```

Meaning:

- a Frame produced by `Save` adopts `Saveable`
- the Frame may become `Saved`, `failed`, or `canceled`
- `Saved` returns a `File` as result
- `failed` may carry an optional `error`
- callers must handle or float all legal statuses

---
## Contract Implementation {#contract-implementation}
---

When an actor or action adopts a contract, it must satisfy every required member.

Rules:

- action implementations must provide every member declared on the contract
- implementations may emit a subset of the allowed statuses (concrete emitted statuses are inferred from the implementation body)
- implementations may be more specific than the contract requires, but never less compatible
- the compiler enforces declared contracts at the implementation site
- contracts do not transform or erase result types — concrete result shapes are preserved from the implementation
- contracts provide guarantees, not restrictions beyond their declared surface

---
## Built-In Default Contracts {#built-in-default-contracts}
---

Marco provides built-in default contracts.

Built-in contracts include:

- `failable`
- `waitable`
- `cancelable`
- `lockable`
- `retryable`
- `iterable`
- `replayable`

Canonical forms:

```marco
this is failable.
this is failable with error.
this is waitable.
this is cancelable.
this is lockable.
this is iterable.
this is replayable.
```

### Example

Canonical:
```marco
the Save is an action.
this is failable with error.

that does...
    when SaveFile exists?
        this is ok with SaveFile!

    or?
        this is failed with error "Save file missing"!
```

Canonical:
```marco
the Download is an action.
this is waitable.
this is cancelable.
```

Meaning:

- `failable` allows `failed` status
- `failable with error` means `failed` carries `its error`
- `waitable` allows active/waiting/finished-style behavior
- `cancelable` allows `canceled` status
- `lockable` allows locked/unlocked access states (see [[Locking]])
- `retryable` allows retry/retried-style behavior
- `iterable` allows `for each` traversal exposing `item`/`key`/`previous`/`next`/`first`/`last` (see [[Iteration#Iterable Contract]])
- `replayable` marks a feed (or other value-producing node) as supporting replay — listeners may receive past events when they attach. Replay support is optional in v1.

Contracts can be explicit or inferred.

Explicit:
```marco
this is failable with error.
```

Inferred:
```marco
this is failed with error "save failed"!
```

Rule:

- built-in contracts are shortcuts for common Frame status patterns
- user-defined contracts may extend or compose them

---
## Capabilities {#capabilities}
---

Contracts define which Frame capabilities callers may invoke.

A capability is declared with `this can <Capability>.`

### Example

Canonical:
```marco
the Saveable is a contract.
this allows Saved with File.
this allows failed with an optional error.
this can retry.
```

Meaning:

- Frames adopting `Saveable` may be retried
- `do that retry.` is valid for any such Frame

If a capability is not declared by the contract, it is invalid to invoke.

See [[Frames#Capabilities]] for invocation syntax and [[Graph#Contracts In Graph Terms]] for the graph-level interpretation.

---
## Implicit Contracts {#implicit-contracts}
---

Contracts are optional by default.

If no contract is declared, Marco infers a contract from emitted statuses.

### Example

Canonical:
```marco
do save...
    when File exists?
        this is Saved with File!
    or?
        this is failed with error "missing file"!
```

Inferred contract:

- `Saved` with `File`
- `failed` with `error`

Rules:

- if a contract is declared, it must be enforced
- if no contract is declared, it is inferred
- explicit contracts are preferred for public or reusable actions
- explicit contracts are required for module, API, or cross-boundary surfaces
- compiler or debug views should expose inferred contracts

---
## Exhaustiveness {#exhaustiveness}
---

If a Frame may produce multiple statuses, all must be handled or floated.

A status remains unresolved until it is explicitly handled.

Handled means:

- matched by `when`
- captured by `or?`
- transformed into another status
- returned upward with `!`

Valid:

Canonical:
```marco
do save...
    when Saved?
        this is ok with that!

    or when failed?
        this is failed with its error!

    or when canceled?
        this is canceled!
```

---
## Unresolved Obligations {#unresolved-obligations}
---

When a child Frame returns, any statuses from its contract that were not handled become obligations on the parent Frame.

Obligations propagate as data, not behavior. The compiler must prove that every possible child status has a path to resolution.

These obligations remain active until they are:

- handled with `when`
- captured with `or?`
- transformed into another status
- returned upward with `!`

Continuing execution does not resolve obligations.

Unresolved statuses **haunt** the parent Frame until they are explicitly handled, transformed, or returned.

A phrase may not close while unresolved child statuses remain.

---
## Unreachable Arms {#unreachable-arms}
---

The compiler also rejects the inverse of an unhandled obligation: an arm that matches a status the callee provably never emits.

```marco
do User's Save...
    when Saved?
        this is ok!
    or when bogus?      // compile error: User's Save never emits "bogus"
        log "never".
    or?
```

Rules:

- only literal `<ref> is <Status>` arms (and the `<Status>` shorthand) are checked; predicate arms with `exists`, `is <value>`, `has`, or compound expressions are not analyzed
- `exited` and `died` are runtime-panic canonicals — they may arise from any callee and are never flagged
- the diagnostic includes the callee's inferred contract so the author can see what statuses are actually reachable

---
## Unreachable Code {#unreachable-code}
---

The compiler also rejects code that follows a terminal return in the same straight-line path.

```marco
this's Save does...
    this is ok!
    log "after".        // compile error: unreachable code after terminal return
```

Rules:

- terminal returns are `this is <Status>!`, `this is <Status> with <expr>!`, `this is failed with error "<msg>"!`, and `this is that!`
- a branch group is itself terminal when every arm terminates AND a bare `or?` fallback is present (so some arm always fires); code after such a group is unreachable
- branch arms otherwise reset the analysis — each arm is a separate path
- `finally...` blocks are a deferred cleanup hook and are exempt; they may follow a terminal return

---
## Frame Reference Validation {#frame-reference-validation}
---

Inside an arm of `do <Subject>'s <Action>...` matching `when that is <Status>?`, accesses to `that's <Field>` are checked against the callee's contract:

- if the callee's contract allows `<Status>` with no value, any `that's <Field>` is rejected (there is no result set)
- if the callee's contract allows `<Status> with <Shape>`, the field must exist on `<Shape>`
- `that's error` is always allowed — every frame has an error slot
- accesses outside a status-matched arm are not analyzed (the result shape isn't known)

```marco
do User's Save...
    when Saved?
        log that's Id.          // valid if Saveable allows Saved with a User { Id }
        log that's BogusField.  // compile error: User has no field "BogusField"
    or?
```

---
## Type Predicates {#type-predicates}
---

`when <ref> is <Type>?` is a runtime type assertion. The compiler validates against the ref's *statically known* type when one is available — currently only `input` (immutable for the frame). State slots (`this's <Field>`) are intentionally polymorphic and are not analyzed.

```marco
it can Process with a User.
this's Process does...
    when input is SaveData?    // compile error: input has type User, not SaveData
        ...
```

### Invalid Example

```marco
do save...
    when saved? this is ok with that!
    do cleanup.
```

If `save` can also return `failed` or `canceled`, those statuses remain unresolved after `cleanup`. The phrase cannot close, because `do cleanup.` is continuation flow, not resolution.

### Correct Example

Canonical:
```marco
do save...
    when saved? this is ok with that!
    or? this is failed with its error!
```

`or?` captures the remaining statuses (`failed`, `canceled`) and resolves them by returning.

### Auto-Handler Example

Canonical:
```marco
handle failable with DefaultErrorHandler.

when any failed?
    this is failed with its error!
```

Auto-handlers may exist, but v1 remains strict: unresolved statuses must still be provably resolved.

---
## `or?` Exclusive Fallback {#or-exclusive-fallback}
---

`or?` is exclusive fallback flow.

It captures the remaining unresolved statuses or unmatched branch conditions from the nearest active branch group. A branch group is `when` / `or when` / `or?` — see [[Core Concepts#Branching: `when` / `or when` / `or?`]].

`or?` is not the same as continuation. Ordinary following lines are continuation flow; `or?` is fallback flow.

### Example

Canonical:
```marco
when this is Ready? do X. or? do Y.
```

This is **not** equivalent to:

```marco
when this is Ready? do X.
do Y.
```

In the first example:

- if `Ready`, `X` runs
- otherwise, `Y` runs

In the second example:

- if `Ready`, `X` runs
- `Y` runs regardless afterward

### Rules

- `or?` only runs when prior branches in the group did not match
- `or?` captures unresolved alternatives
- `or?` must explicitly resolve, transform, or return what it captures
- `or?` does not imply success or failure
- ordinary following lines are continuation flow, not fallback flow

See [[#Unresolved Obligations]] for how `or?` participates in resolving child statuses.

---
## Templates {#templates}
---

A template defines an **optional** interface — a whitelist of allowed behavior, not an obligation.

### Canonical Form

```marco
the GameExtras is a template.
it can Pause.
it can Resume.
it can Screenshot.
```

### Adoption

Actors and other entities adopt templates via `uses`:

```marco
the RocketLeague is an actor.
it uses GameExtras.
```

### Rules

- declared members **may** be implemented but are not required
- if a member is implemented, it must follow the declared shape
- templates act as a whitelist of allowed behavior — only members declared on the template (or on a contract / inherent capability) may be implemented under that template's name
- a single entity may adopt multiple templates and contracts simultaneously

---
## Traits {#traits}
---

A trait is a capability wrapper applied to **data and resources** rather than execution entities.

### Canonical Form

Traits compose with data type declarations:

```marco
its Lck is a lockable set.
the Events is a replayable feed.
the CustomList is an iterable set.
```

The pattern is `<Trait> <BaseType>` — the trait modifies the base type with additional capability or behavior.

### Rules

- traits add capability or behavior to data
- traits do **not** impose required implementation
- traits do **not** define Frame statuses directly
- traits may expose actions, data, or access semantics
- the same built-in name (`lockable`, `iterable`, `replayable`) is a contract when applied to an action/actor and a trait when applied to a data type — the mechanism is the same; the role differs by application site

### Composition

Multiple traits may compose on a single data declaration when the combination is well-defined (e.g., `lockable iterable set`). v1 supports the common pairs; complex stacks may be deferred.

---
## Contracts vs Templates vs Traits {#contracts-vs-templates-vs-traits}
---

| Kind | Imposes? | Applies to | Adoption verb |
|------|----------|------------|---------------|
| Contract | Yes — required interface | actors, actions, scenes, Frames | `is` |
| Template | No — optional interface | actors, scenes | `uses` |
| Trait | No — capability wrapper | sets, feeds, queues, channels, lists, maps | inline modifier |

Summary:

- **Contracts impose behavior.**
- **Templates allow behavior.**
- **Traits compose behavior.**

Contracts and templates govern *execution and lifecycle*; traits govern *data and resource access*.

---
## One-Line Lock {#one-line-lock}
---

Contracts require, templates allow, traits compose.

Unresolved statuses haunt the parent Frame until explicitly handled, transformed, or returned.

`or?` is exclusive fallback flow; ordinary following lines are continuation flow.

---
## Open Questions {#open-questions}
---

- Formal syntax for representing floated statuses in source when not handled locally.
