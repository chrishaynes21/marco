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

### Example

Canonical:
```marco
test SaveUser...
    with UserInput.
    expect: status Saved that's Id exists
```

---
## Mocks {#mocks}
---

Mocks replace graph nodes.

### Example

Canonical:
```marco
mock EmailService with FakeEmailService.
```

Rules:

- mocking replaces behavior at the graph level
- no dependency injection required

---
## One-Line Lock {#one-line-lock}
---

Tests describe Frame inputs, expected statuses, and expected result shapes; mocks replace graph nodes directly.

---
## Open Questions {#open-questions}
---

- Canonical grammar for `expect:` clauses.
- Scope of a mock — per-test, per-module, or global.
- Whether tests may observe side effects beyond Frame status and result.
