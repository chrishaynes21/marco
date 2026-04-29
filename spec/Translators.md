---
# Translators {#translators}
---

A translator is a declared bridge that converts one type into another. Translators are first-class — they're the canonical way to handle ambiguous mappings flagged by [[Inference#Result Inference]].

---
## Declaration {#declaration}
---

Canonical:
```marco
the ApiUserToUser is a translator.
it takes an ApiUser and gives a User.
```

A translator declares:

- its **input type** via `it takes <Type>`
- its **output type** via `it gives <Type>`

The body resolves with `this is ok!` carrying the produced value.

---
## Body {#body}
---

Canonical:
```marco
the ApiUserToUser is a translator.
it takes an ApiUser and gives a User.

User is ApiUser as a partial User.
User's Name is ApiUser's Fullname.
this is ok!
```

Inside the body:

- the input type binding (`ApiUser` here) is read-only
- the output type binding (`User` here) is owned and mutable
- assignments produce the result
- `this is ok!` resolves the translator with the populated `User` as the result

---
## Rules {#rules}
---

- `takes` creates a read-only input binding
- `gives` creates an owned, mutable result binding
- the translator returns with `this is ok!` (or `this is failed!` on error)
- only declared input/output bindings are visible at the body's top level
- translators participate in [[Inference#Result Inference|cast inference]] — the compiler picks them up automatically when an `as a <Type>` cast cannot be inferred

---
## Auto-Mapping {#auto-mapping}
---

A translator may auto-map matching fields:

```marco
the ApiUserToUser is a translator.
it takes an ApiUser and gives a User.

it maps what it can.
this is ok!
```

`it maps what it can.` is a directive that:

- pairs each field on the output type with a same-named field on the input
- copies values where types match
- leaves unmatched fields unset (must be either optional or filled manually below this line)

Manual assignments below `it maps what it can.` override the auto-mapped values.

### Mixed Form

```marco
the ApiUserToUser is a translator.
it takes an ApiUser and gives a User.

it maps what it can.
User's Name is ApiUser's Fullname.   // override: source field has a different name
this is ok!
```

Rules:

- matching field names map automatically
- manual assignments override
- missing required fields must be handled or the compiler rejects the translator

---
## Partial Types {#partial-types}
---

`as a partial <Type>` creates a **bridge type** — a value of `<Type>` whose required fields may not all be populated yet.

Canonical:
```marco
User is ApiUser as a partial User.
```

The result is a value of bridge type `ApiUserUser` (Marco names the bridge by joining the source and target types) that:

- is structurally a `User` (same fields)
- inherits any fields that match by name from `ApiUser`
- requires [[Data Model#Presence and Safe Access|`has` proof]] for fields not yet populated

### Promotion

A bridge type may be promoted to its target once all required fields are present:

```marco
User is ApiUserUser as a User.
```

Promotion is allowed when:

- all contract-required fields on `User` are populated
- the compiler can prove this (or runtime check at promotion site)

### Rules

- bridge types are first-class — they have their own type identity
- bridge types require `has` proof for any field not provably populated
- promotion via `as a <Target>` succeeds when required fields are complete; otherwise the cast is invalid

One-line: **Partial types are bridge types that evolve into their target.**

---
## Translator vs Cast {#translator-vs-cast}
---

A bare `as a <Type>` cast asks the compiler to coerce a value structurally. If there's exactly one valid mapping, the cast succeeds.

A translator is invoked when:

- the cast is ambiguous (multiple mapping interpretations)
- structural coercion isn't sufficient (fields need transformation)
- the source type doesn't have all the required target fields

If a translator named `<A>To<B>` exists, the compiler may use it implicitly to satisfy `<value> as a <B>` from a value of type `<A>`. Explicit invocation is also possible — `do ApiUserToUser with apiUser` runs the translator like any other action.

---
## One-Line Lock {#one-line-lock}
---

Translators bridge types via declared inputs and outputs. Partial types are bridges-in-progress. Auto-mapping handles the easy cases; manual assignments handle the rest.

---
## Open Questions {#open-questions}
---

- Whether translators may be variadic (multiple `takes` clauses).
- Whether a translator can `gives` multiple outputs (tuple-like).
- Bridge type naming conventions when source and target names collide.
- Whether `it maps what it can.` may appear more than once in a body.
- Translator chaining — `<A>` to `<C>` via `<B>` when both `<A>To<B>` and `<B>To<C>` exist.
- Failure handling: should `it maps what it can.` emit `failed` if any required mapping is missing, or always succeed and defer the error to the use site?
