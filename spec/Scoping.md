---
status: reference
---

---
# Scoping {#scoping}
---

---
## Core Rule {#core-rule}
---

Marco resolves names using context-first inference.

Resolution order:

1. local names in the current phrase
2. `this` context
3. named [[Frames|Frames]] (declared with `as` or `the`)
4. accessible sets (`input`, `that`, `its`)
5. outer scopes
6. global scope

If a name is ambiguous, the compiler must require explicit qualification.

---
## Ambiguity {#ambiguity}
---

If multiple sources expose a name, qualification is required.

### Example

`User` may refer to:

- `input's User`
- `that's User`
- `this's User`

If more than one exists, the author must qualify:

Canonical:
```marco
input's User
that's User
this's User
```

See [[Inference#Ambiguity Rule]].

---
## Shadowing {#shadowing}
---

Inner scopes shadow outer scopes.

### Example

Canonical:
```marco
the User is a set.

do Something...
    the User is do LoadUser.
```

Inside the phrase, `User` refers to the loaded user, not the global one.

---
## Explicit Qualification {#explicit-qualification}
---

When ambiguity exists, qualify by source:

Canonical:
```marco
input's File
that's File
this's File
```

Kind-based disambiguation may exist but is not preferred.

---
## One-Line Lock {#one-line-lock}
---

Names are resolved by context first; explicit qualification is required when ambiguous.

---
## Open Questions {#open-questions}
---

- Whether shadowing emits a compiler warning by default.
- Formal grammar for kind-based disambiguation when allowed.
