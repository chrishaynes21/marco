---
# Data Model {#data-model}
---

All structured data in Marco is represented as sets. Primitives and containers define the shape of those sets and how they are accessed.

---
## Sets {#sets}
---

A set is the universal structured data container.

Sets represent:

- inputs
- results
- actor state
- configuration
- errors
- grouped values

Sets are key-value collections.

### Example

Canonical:
```marco
the User is a set.
its Name is a text.
its Email is a text.
```

Rules:

- sets contain named fields
- fields use possessive access
- sets may nest other sets
- sets may be typed by declaration

Access:

```marco
User's Name
User's Email
```

See [[Frames#Set]] and [[Declarations#Field Declarations]].

---
## Primitives {#primitives}
---

Marco supports basic primitive types.

Core primitives:

- `text`
- `number`
- `boolean`
- `time`
- `id`

### Examples

Canonical:
```marco
this's Name is a text.
this's Age is a number.
this's Active is a boolean.
```

Rules:

- primitives are immutable values
- primitives may be assigned directly
- primitives may be compared in conditions

---
## Literals {#literals}
---

Marco supports primitive literals.

### Examples

Canonical:
```marco
"Chris"
'Chris'
42
true
false
```

Strings may use single or double quotes.

### Escape Character

Backslash is the escape character.

Examples:

```marco
"Chris said \"hello\"."
'Chris said \'hello\'.'
"C:\\Users\\Chris"
```

Rules:

- backslash escapes quotes and special characters
- supported escapes include `\"`, `\'`, `\\`, `\n`, and `\t`

---
## Containers {#containers}
---

Containers are structured data nodes that hold or organize sets.

Core containers:

- `set` — universal structured data container
- `list` — ordered collection
- `map` — keyed collection
- `queue` — ordered work / backlog
- `feed` — stream of values / events
- `channel` — conversational message surface

All containers are sets. Lists, maps, queues, feeds, and channels are specialized sets with their own access rules.

**Channels vs feeds**: channels coordinate actors (conversational, ephemeral); feeds describe streams (data flow, optionally replayable).

---
## List {#list}
---

A list is an ordered collection.

### Example

Canonical:
```marco
the Users is a list of User.
the Things is a list of any.
```

Rules:

- lists preserve order
- list values share one declared item type
- `any` allows mixed item types
- positional access uses `at`

Access:

```marco
Users at 0
Users at 45
```

Presence:

```marco
when Users at 45 exists?
```

Iteration:

```marco
for each item in Users...
    do Process with item.
```

`key` is the index. `item` is the current value. See [[Iteration#For Each]].

---
## Map {#map}
---

A map is a keyed collection.

### Example

Canonical:
```marco
the UsersById is a map of id to User.
the UsersByEmail is a map of text to User.
the Cache is a map of text to any.
```

Rules:

- maps require key type and value type
- keys are unique
- values share one declared value type unless `any`
- access uses possessive syntax

Assignment:

```marco
UsersById's UserId is User.
UsersByEmail's "chris@example.com" is User.
```

Access:

```marco
UsersById's UserId
UsersByEmail's "chris@example.com"
```

Presence:

```marco
when UsersById has UserId?
```

Iteration:

```marco
for each item in UsersById...
    log key.
    do Process with item.
```

`key` is the current map key. `item` is the current map value. See [[Iteration#For Each]].

---
## Queue {#queue}
---

A queue is an ordered work / backlog container.

### Example

Canonical:
```marco
the SaveQueue is a queue of SaveRequest.
the WorkQueue is a queue of any.
```

Rules:

- queues preserve insertion order
- queues are used for pending work / items
- queues may be typed or `any`

Add item:

```marco
put SaveRequest into SaveQueue.
```

Consume:

```marco
when SaveQueue has next?
    do Save with item.
```

`item` is the next queued value. See [[Iteration#Queues]].

---
## Feed {#feed}
---

A feed is a stream of values / events.

### Example

Canonical:
```marco
the GameEvents is a feed of GameEvent.
the Telemetry is a replayable feed of any.
```

Rules:

- feeds are stream-oriented
- feeds are read / write
- feeds may be replayable
- feeds describe flowing data over time

Write:

```marco
write SaveRequested to GameEvents.
write SaveRequested with User to GameEvents.
```

Read:

```marco
when GameEvents reads SaveRequested?
    do Save.
```

See [[Iteration#Feeds]].

---
## Channel {#channel}
---

A channel is a conversational message surface.

### Example

Canonical:
```marco
the GameChannel is a channel.
```

Rules:

- channels coordinate actors
- channels use `says` and `hears`
- channels are ephemeral by default
- speaker / listener terminology applies

Speak:

```marco
says SaveRequested to GameChannel.
```

Listen:

```marco
when GameChannel hears SaveRequested?
    do Save.
```

See [[Actors#Messaging Syntax — Channels]].

---
## Container Traits {#container-traits}
---

Containers may use [[Contracts#Traits|traits]].

Examples:

```marco
the Settings is a lockable set.
the Events is a replayable feed.
the CustomList is an iterable set.
```

Rules:

- traits add capability wrappers
- traits do not define statuses directly
- contracts define Frame behavior

See [[Contracts#Traits]] and [[Locking]].

---
## Set vs Containers {#set-vs-containers}
---

All containers are sets. Lists, maps, queues, feeds, and channels are specialized sets with their own access rules.

See [[Core Concepts#Access]] for the canonical access forms (`'s` for sets and maps, `at` for lists).

### Container Summary

**Sets hold fields, lists order items, maps key items, queues backlog work, feeds stream data, and channels carry conversation.**

---
## Assignment {#assignment}
---

Values are assigned using `'s ... is ...`.

### Example

Canonical:
```marco
User's Name is "Chris".
User's Age is 30.
```

---
## Construction {#construction}
---

Sets may be constructed implicitly during assignment.

### Example

Canonical:
```marco
the User is a set.
User's Name is "Chris".
User's Age is 30.
```

---
## Inline Construction {#inline-construction}
---

`with` builds a set inline.

### Examples

Canonical:
```marco
User with Name "Chris", Age 30.
Order with Id 123, User with Name "Chris".
```

Rules:

- `with` builds a set
- named values become entries in the set
- nested sets are allowed
- types may be inferred when unambiguous

The same `with` is used to pass input to actions (`do Save with SaveArgs.`); passing input is constructing a set inline. See [[Frames#Input Model]] and [[Inference#Input Inference]].

---
## Inference {#inference}
---

Types may be inferred where unambiguous.

### Example

Canonical:
```marco
User's Name is "Chris".
```

Infers `Name` is `text`.

See [[Inference]] for the broader inference policy.

---
## Existence {#existence}
---

Fields may be checked for existence.

### Example

Canonical:
```marco
when User's Name exists?
```

`exists?` is a path-style predicate. For presence proof on optional fields that gates safe access, use [[#Presence and Safe Access|`has`]].

---
## Presence and Safe Access {#presence-and-safe-access}
---

Marco enforces safe access through proof of presence. Use `has` to prove that a value exists before accessing it.

### Canonical Form

```marco
when <Container> has <Field>?
```

### Examples

Canonical:
```marco
when that has User?
when that has Message?
when that has error?
```

### Rules

- `has` checks whether a field exists on a [[Frames|Frame]] or set
- if the target is a Frame:
  - check Frame slots first (`error`, `input`, `result`, `status` when applicable — see [[Frames#Frame Slots]])
  - then check the result set
- accessing a value without proving presence is invalid if optional
- the compiler may enforce presence checks at compile time
- the runtime must error if accessed without proof
- required-by-contract fields skip the proof requirement

### Safe Access

```marco
when that has User?
    do that's User's Save!
```

The body executes only when `User` is present on `that`. Inside the body, `that's User` is proven safe.

### Binding After Proof

```marco
when that has User?
    the User is that's User.
    do User's Save!
```

`the User is that's User.` binds the proven-present value to a local name. See [[#Destructuring]].

### Unsafe Access

```marco
do that's User's Save!
```

**Invalid** if `User` is optional — presence was not proven. The compiler must reject this; the runtime must error.

### Status vs Presence

Marco distinguishes between status checks and structure checks.

Status (lifecycle):
```marco
when that is failed?
when that is ok?
```

Presence (available data):
```marco
when that has error?
when that has Message?
```

Rules:

- status checks describe lifecycle
- presence checks describe available data
- failure handling should use status checks

Canonical failure handling:
```marco
when that is failed?
    this is failed with that's error!
```

`when that has error?` is structurally valid but semantically wrong for failure handling — `that's error` is a Frame slot that may exist on any Frame, while `that failed?` answers the lifecycle question.

### Optional vs Required

If a field is required by [[Contracts|contract]], no presence check is needed.

Example:
```marco
this allows Saved with User.
```

Then `that's User` is always safe after `when that is Saved?` — the `with User` clause makes `User` part of the result shape required by the contract.

If a field is not declared as required, it is optional and access requires `has`.

### Principles

**Proof over null.** Marco does not use null checks. Marco uses proof. You do not ask "is this null?" — you prove "this has X."

**Frames act, sets hold.** Frames represent execution and lifecycle. Sets represent data. Frames own status and capabilities; sets hold values. `that` refers to a Frame; `that's` accesses its result set. See [[Frames#Frame vs Result]].

**Ownership is explicit.** `this` is the Frame you resolve; `that` is the Frame you observe. You may only return or mutate `this`. You may only inspect `that` unless explicitly allowed by a [[Contracts#Capabilities|capability]]. See [[Frames#Process Mode References]].

### One-Line Lock

**`has` proves existence; status describes lifecycle; access requires proof.**

Marco doesn't have null checks — it has proof checks.

---
## Operators {#operators}
---

Marco's v1 operator set is intentionally minimal.

| Operator | Meaning |
|----------|---------|
| `+` | Text concatenation **or** numeric addition |

### Examples

```marco
the Greeting is "Hello, " + User's Name.
the Total is User's Score + 10.
```

Rules:

- `+` operates on compatible types only (text+text or number+number)
- mixed-type `+` is a compile error (use explicit cast: `text + number as a text`)
- additional operators (`-`, `*`, `/`, etc.) may be added later
- v1 keeps the operator set minimal

There are no boolean operators in expression position. `and`/`or` exist only inside `when` conditions — see [[Core Concepts#Condition Chaining]].

---
## Expression Rules {#expression-rules}
---

Marco evaluates expressions left to right.

Nested expressions are allowed when they remain readable and unambiguous.

### Rules

- `+` supports text concatenation and numeric addition (same-type only)
- parentheses may be used to clarify nested expressions
- if nesting becomes ambiguous, the compiler requires clearer structure
- Marco prefers readable intermediate lines over dense expressions

### Example

Preferred:
```marco
the FullName is User's First + " " + User's Last.
the Greeting is "Hello, " + FullName + "!".
```

Over a single dense line. The compiler should suggest splitting when an expression's nesting depth crosses a readability threshold.

---
## Value vs Frame Boundary {#value-vs-frame-boundary}
---

Actions create [[Frames|Frames]]. Frame results are sets / values. Expressions use values, not Frames — unless the Frame result is safely inferable from context.

### Inferred Form

```marco
User's Name is GetName.
```

Valid if `GetName` is an action whose Frame returns a `text` result. The compiler infers a Frame invocation and uses its result.

### Equivalent Expanded Form

```marco
do GetName.
User's Name is that's result.
```

### Rule

The shorthand `<binding> is <Action>.` is allowed when:

- `<Action>` is unambiguously an action node
- the action's result type matches the binding's expected type
- inference is unambiguous (per [[Inference#Ambiguity Rule]])

Otherwise, write the explicit two-line form.

---
## `that` Scoping {#that-scoping}
---

`that` always refers to the previous Frame in the current execution context — whatever that Frame is.

`that's <field>` accesses that Frame's result, except for [[Frames#Frame Slots|Frame slots]] like `error` which are special-cased to the slot directly.

This rule is unconditional: `that` is never a value; it's always a Frame reference. Data flow through `that` is via the result-set unwrap rule in [[Frames#Result]].

---
## `any` Type {#any-type}
---

`any` represents an unknown or dynamic type.

### Example

```marco
the WorkQueue is a queue of any.
```

### Rules

- `any` disables compile-time type guarantees
- the user must prove type using `is <Type>` before accessing fields
- type proof gates safe access in the branch body

### Example with proof

```marco
when WorkQueue has next?
    when item is SaveRequest?
        do Save with item.
    or when item is RenderRequest?
        do Render with item.
    or?
        log "unknown item type".
```

See [[Core Concepts#Type Checks]] and [[Iteration#Typed and Untyped Collections]].

---
## No Null {#no-null}
---

**Marco has no null.**

Absence is represented by lack of presence, not by a null value.

Rules:

- there is no `null` literal
- there is no "nullable" type modifier
- absence is proven via [[#Presence and Safe Access|`has`]]
- access without proof is invalid

Marco doesn't have null checks — it has proof checks.

---
## Comparison {#comparison}
---

Values may be compared.

### Equality

Canonical:
```marco
when User's Age is 30?
when User's Active is true?
when Name is "Chris"?
```

### Inequality

Canonical:
```marco
when User's Age is not 30?
```

Rules:

- `is` checks equality/state
- `is not` checks inequality
- `exists` checks presence (see [[#Existence]])

---
## Destructuring {#destructuring}
---

Destructuring binds named values from a result set into local names.

### Example

Canonical:
```marco
do LoadFile...
    when that is Saved?
        the File is that's File.
        do Process with File.
```

Meaning:

- `that` is the LoadFile Frame
- `that's File` accesses the `File` entry on that Frame's result
- `the File is that's File.` creates a local name for that entry

### Canonical Form

Marco v1 prefers explicit binding via `the <Name> is <Source>.`.

```marco
the File is that's File.
```

Do not use `File = that's File` — assignment is via `is`, not `=`. Compact destructuring shorthand may be considered later; v1 prefers explicit binding.

---
## Defaults {#defaults}
---

Default values are declared with `by default`.

### Examples

Canonical:
```marco
User's Age is 0 by default.
Settings's Theme is "dark" by default.
```

Rules:

- defaults apply when a value is not set
- defaults are resolved at access time
- explicit values override defaults

---
## Immutability {#immutability}
---

Primitives are immutable.

Sets are mutable only when:

- owned by the current phrase
- copied
- unlocked through a [[Locking|lockable]] shared set

[[Frames|Frame]] results are immutable after return unless copied or explicitly shared.

---
## One-Line Lock {#one-line-lock}
---

All structured data in Marco is represented as sets, with primitives and containers defining their shape and access.

---
## Open Questions {#open-questions}
---

- Full primitive set beyond `text`, `number`, `boolean`, `time`, `id` (e.g., binary, decimal, currency).
- Whether lists support negative indices or slice access.
- Whether map keys are restricted to primitive types or may be arbitrary sets.
- Mutation semantics for nested sets and containers.
- Whether `time` is absolute (instant), relative (duration), or both.
