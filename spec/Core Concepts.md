---
status: reference
---

---
# Core Concepts {#core-concepts}
---

---
## Sentences {#sentences}
---

Every line in Marco is a sentence. The terminating punctuation determines the sentence's effect.

| Symbol | Meaning |
|--------|---------|
| `.`    | State update |
| `!`    | Return from the owning phrase |
| `?`    | Question / listener |

### Example
```marco
it is ok.
this is ok with that!
when this is ok?
```

---
## `when` Overview {#when-overview}
---

`when` is Marco's question mode.

Canonical `when` must end with `?`.

---
## `it` — Frame {#it-frame}
---

`it` refers to the current [[Frames|Frame]].

Status is set with `is`.

### Example
```marco
this is ok!
```

See [[Frames]].

---
## `that` — Most Recent Frame {#that-result}
---

`that` refers to the most recent completed [[Frames|Frame]] — a child or observed Frame.

`that's <field>` accesses data on `that`'s result set.

In data-flow contexts (e.g., `with that`), Marco unwraps `that` to its result set.

### Example

Canonical:
```marco
do Process with that.
```

Equivalent:
```marco
do Process with its result.
```

### Example

Canonical:
```marco
that's File
that's User
```

`that` may also be acted on directly when a [[Contracts#Capabilities|capability]] permits:

```marco
do that cancel.
do that retry.
```

See [[Frames#Frame vs Result]].

---
## `input` — Received Set {#input-received-set}
---

`input` is the current Frame's received set.

### Example

Canonical:
```marco
input's File
input's User
```

See [[Frames]].

---
## `its` {#its-and-its-status}
---

`its` refers to the current Frame set.

### Example

Canonical:
```marco
its error
its result
```

For status checks, use the bare-status predicate form:

```marco
when this is ok?
when this is failed?
when that is ok?
when failed?
```

`it's` is no longer canonical for status. See [[Lifecycles#Frame Status Predicates]].

See [[Frames]].

---
## `this` — Current Frame {#this-frame}
---

`this` refers to the current focused [[Frames|frame]].

A frame is created by every line. Frames may be named with `the` to become `this`, or with `as` to remain accessible without changing focus.

### Example

Canonical:
```marco
the Result is do Save with GameState.
```

After this line, `this` is the Result frame.

See [[Frames]].

---
## Phrases {#phrases}
---

`...` opens a [[Phrases|phrase]]. A phrase owns a Frame context and may return with `!`.

Only a phrase owns a return. Branches such as `when` do not.

### Example

Canonical:
```marco
do Save...
	when ok?
		this is ok with that!
```

In this example, `this is ok with that!` returns from the owning phrase `Save`.

---
## `when` — Question Mode {#when-question-mode}
---

`when` is a single-line question in canonical form.

`when`:

- must end with `?` in canonical form
- does not own a return
- may open a branch body via indentation
- may inspect `it`, `that`, named Frames, or graph state
- does not modify `it` or `that`
- may act as an immediate check if the subject is settled
- may act as a listener if the subject is active

### Example

Canonical:
```marco
when this is ok?
when SaveFrame is failed?
when User's Session is active?
```

(The third example is data comparison on a set field, not a Frame status check — `is` stays.)

---
## Condition Chaining {#condition-chaining}
---

`and` and `or` chain multiple conditions inside a single `when`.

### Canonical Form

```marco
when that has Name and has Email?
when that is failed or that is canceled?
```

Shorthand may omit the repeated `that`:

```marco
when has Name and has Email?
```

### Rules

- `and` requires **all** conditions to be true
- `or` requires **any** condition to be true
- conditions must be unambiguous — mixing `and`/`or` in the same `when` requires explicit grouping (open question whether parentheses or order-of-precedence applies; lean: same-operator chains only, and split into `or when` for mixed logic)
- shorthand may omit the repeated subject when context permits

`and`/`or` apply **inside conditions only**. They are not general boolean operators in expressions.

### Predicate Kinds That Can Be Chained

- presence: `has <Field>` (see [[Data Model#Presence and Safe Access]])
- existence: `<Path> exists`
- status: `is <Status>` (see [[Lifecycles#Frame Status Predicates]])
- type: `is <Type>` (see [[#Type Checks]])
- equality: `is <Value>` (see [[Data Model#Comparison]])

---
## Type Checks {#type-checks}
---

`is <Type>` is a runtime type predicate. Inside the body, the subject is proven to be of that type and its fields are accessible.

### Examples

```marco
when item is SaveRequest?
when that is User?
```

### Rules

- `is <Type>` checks type identity
- may be used in conditions and branching
- the type must be known (declared) or inferable
- in the branch body, the subject is proven to be of `<Type>` — fields are safely accessible
- gates safe access on `any`-typed values (see [[Data Model#`any` Type]])

Symmetric with [[Data Model#Presence and Safe Access|`has`]]: `has` proves a field; `is <Type>` proves a type.

---
## Branching: `when` / `or when` / `or?` {#branching}
---

Branching is a grouped conditional chain.

### Canonical Form

```marco
when <Condition>?
    ...
or when <Condition>?
    ...
or?
    ...
```

### Rules

- `when` starts a branch group
- `or when` adds additional conditions to the same group
- `or?` captures all remaining unmatched cases
- only one branch in the group executes
- indentation defines the body of each branch
- after a branch resolves, execution continues after the group

### Example

Canonical:
```marco
when that is failed?
    this is failed with that's error!
or when that is canceled?
    this is canceled!
or?
    this is ok with that!
```

### Shorthand Condition

A bare status check uses [[Lifecycles#Frame Status Predicates|the predicate shorthand]] when `that` is unambiguous:

```marco
when failed?
    this is failed with that's error!
```

Means `when that is failed?`.

### Constraints

- `when` conditions must end with `?`
- `or when` must follow a `when` or another `or when`
- `or?` must be the final branch in a group
- `or?` must explicitly resolve, transform, or return
- ordinary lines after a branch group are continuation flow, not fallback

Do not allow phrase-style branching with `when ...` (no `?`) in v1.

One-line lock: **Branching is a `when` / `or when` / `or?` chain where only one branch executes and `or?` handles the remaining cases.**

See [[Contracts#`or?` Exclusive Fallback]] for the contract-level interpretation and [[Execution Model#Branch Grouping]] for execution semantics.

---
## Run-On `when` {#run-on-when}
---

`when` may appear inside a run-on sentence using commas.

In run-on form, the `?` may be omitted for readability, but is implied.

Run-on `when` is shorthand for inline branching.

The condition is evaluated at that point in execution.

If the condition is true, execution continues to the next clause.

If the condition is false, the clause is skipped.

`it` and `that` reflect the most recent completed Frame at the time of evaluation.

### Example

Shorthand:
```marco
do X, when ok, do Y!
```

Equivalent canonical form:
```marco
do X... when ok? do Y!
```

### Example

Shorthand:
```marco
do X, then do Y, when ok, do Z with that!
```

Interpretation:

- `X` runs
- `Y` runs
- `when` checks `Y`'s status
- if `Y` is `ok`, `Z` runs with `Y`'s result

---
## Access {#access}
---

Sets and maps use the possessive form:
```marco
User's Name
```

Lists use `at`:
```marco
Items at 45
```

See [[Data Model]] for full container semantics, existence checks, and comparison.

---
## Naming Convention {#naming-convention}
---

Identifier case is meaningful in Marco.

| Case | Meaning |
|------|---------|
| `lowercase` | Built-in actions, keywords, and statuses (e.g., `log`, `show`, `do`, `start`, `when`, `or?`, `is`, `has`, `ok`, `failed`, `canceled`) |
| `Capitalized` / `PascalCase` | User-defined names (actors, actions, sets, contracts, errors, fields, capability declarations) |

### Override

System built-ins can be overridden by user-defined identifiers using their capitalized variants.

Canonical:
```marco
log "starting".         // built-in log
do Log with "starting". // user-defined Log overrides
```

Rule: a `Capitalized` user-defined name shadows the lowercase built-in within its scope. Caller intent is explicit because the case is different.

Caveat: overriding system built-ins is permitted, but accept the responsibility — only do it if your version is genuinely better than the built-in.

### Status Values

Status names follow the same convention. Built-in statuses (`ok`, `failed`, `canceled`, `saved`) are lowercase; user-introduced statuses (`Saved`, `Saving`, `Ready`) follow user-defined casing — typically capitalized.

This resolves the prior Open Question on identifier case sensitivity.

---
## Comments {#comments}
---

`//` introduces a line comment.

### Example

```marco
// this is a comment
```

Rules:

- comments do not affect execution
- comments are ignored by the compiler
- spec examples should avoid comments unless documenting comments

---
## Formatting And Punctuation {#formatting}
---

Marco's surface grammar uses indentation and punctuation:

- indentation defines phrase structure
- commas define run-on chaining
- periods terminate lines
- `...` opens [[Phrases|phrases]]
- `?` marks question mode
- `!` closes the owning phrase

Rules:

- question mode is one line and requires `?`
- run-on clauses may omit `?` when the question is inline and unambiguous (see [[#Run-On `when`]])
- canonical spec examples should prefer explicit `?`

---
## Canonical vs Shorthand {#canonical-vs-shorthand}
---

Marco's canonical form is verbose and unambiguous. Shorthand contracts the canonical form using familiar English contractions where it doesn't sacrifice readability.

### Example

Canonical:
```marco
this is ok with that!
```

Optional shorthand:
```marco
this ok with that!
```

---
## Open Questions {#open-questions}
---

- Full set of recognized contractions for shorthand (currently observed: `that's`).
- Formal clause grammar for run-on sentences beyond the locked `when` shorthand examples.
- Whether the `Capitalized` override of a built-in must be declared (`the Log is an action.`) before use, or can be inferred from context.
