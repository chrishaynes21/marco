---
status: reference
---

---
# Testing {#testing}
---

Testing is built from [[Frames]] and [[Contracts]].

---
## Test Definition {#test-definition}
---

Tests describe:

- input sets
- expected statuses
- expected result shapes

A test runs its body in an isolated Frame. Pass / fail is determined by:

- the test Frame's terminal status (`failed` / `died` are failures)
- any `expect` assertion in the body that evaluates false
- an unhandled child failure left in `that` at end of the test, *unless* an `expect that is <Status>.` assertion has explicitly checked the child's status (the assertion counts as handling)

### Example

Canonical:
```marco
test SaveUser...
    do User's Save with input.
    expect that is Saved.
    expect that's Id exists.
```

---
## `expect` Assertions {#expect-assertions}
---

Assertions use the `expect` keyword followed by any predicate accepted by `when ...?`. The predicate is evaluated at the point of the assertion; if false, the test fails with a diagnostic naming the source position.

### Canonical Form

```marco
expect <pred>.
```

### Predicate forms

The same predicates as branch headers — see [[Core Concepts#Branching: `when` / `or when` / `or?`]] — including:

- `expect that is <Status>.` — frame status check
- `expect <ref> exists.` — field presence
- `expect <ref> is <value>.` — equality
- `expect <ref> has <Field>.` — optional-field presence
- `expect <pred-a> and <pred-b>.` — compound
- `expect not <pred>.` — negation; inverts any predicate

### Example

Canonical:
```marco
test SaveUser...
    do User's Save with input.
    expect that is Saved.
    expect that's Id exists.
    expect that's Id is "42".
```

Rules:

- `expect` is valid in test bodies and script bodies; in scripts a failed expectation terminates the run with the same diagnostic
- multiple `expect` lines all run; the first failure stops the test
- predicates have no side effects — they read state, never mutate it

---
## Mocks {#mocks}
---

Mocks replace graph nodes. Two forms exist:

### Actor-replacement form

Swaps one actor for another wherever the original is invoked.

```marco
mock EmailService with FakeEmailService.
```

The replacement actor must declare the capabilities used at the call sites.

### Inline form

Replaces a single action's body with a contract-conformant emission. The compiler already knows the capability's contract; the mock supplies the status (and result, if the contract requires one).

```marco
mock User's Save is Saved with a User with Id "42".
mock Email's Send is failed.
mock Pay's Process is failed with error "card declined".
```

Rules:

- mocking replaces behavior at the graph level — no dependency injection required
- the inline form mirrors the action-return grammar (`is <Status> [with <expr>]`) but ends in `.` instead of `!`
- the status must be allowed by the action's contract; the optional value must match the declared shape — both are checked at compile time
- inline mocks discard the original body (state updates, side effects, sub-invokes are skipped)
- declared at the top level, mocks are global; declared inside a `test ...` body, mocks are scoped to that test only and are unwound when the test finishes

---
## One-Line Lock {#one-line-lock}
---

Tests describe Frame inputs, expected statuses, and expected result shapes; assertions use `expect <pred>.`; mocks either replace whole actors (`mock X with Y.`) or inline a contract-conformant return (`mock X's Y is <Status> [with <expr>].`).

---
## Open Questions {#open-questions}
---

- Per-module mock scope (per-test and global are both supported; per-module is still open).
- Whether tests may observe side effects beyond Frame status and result.
