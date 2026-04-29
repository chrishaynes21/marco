---
# Compiler {#compiler}
---

The Marco compiler is strict at compile time and generous in guidance. It rejects illegal code but explains how to fix it.

---
## Diagnostics {#diagnostics}
---

Compiler diagnostics must be developer-friendly. Every diagnostic should include:

- exact line
- failing phrase
- unresolved name or type
- expected valid forms
- suggested fixes
- nearby valid graph nodes
- contract / status hints

### Example

Invalid:
```marco
do Save with User.
```

If `Save` expects `SaveArgs`, the compiler suggests:

```
Save expects SaveArgs. User is a User. Possible fixes:
  - do Save with User as a SaveArgs.
  - define a translator from User to SaveArgs.
  - pass SaveArgs instead.
```

See [[Inference#Result Inference]] for the `as a` cast and translator/scene fallback.

---
## Autocomplete {#autocomplete}
---

Marco tooling should use the [[Graph|graph]] to predict likely next tokens.

| Position | Suggest |
|----------|---------|
| after `when that` | valid statuses (`ok`, `failed`, `canceled`, `Saved`, …) |
| after `when this` | valid statuses for the current Frame |
| after `that's` | result fields on the previous Frame |
| after `this's` | members on the current declared subject |
| after `do` | actions in scope |
| after `this can` | action declaration shape (capability name) |
| after `this is` (return position) | statuses allowed by current contract |
| after `it is` (state update position) | statuses allowed by current contract |
| after `when ... has` | optional fields on the target |
| after `say` | hearable messages in scope |
| after `to` (in `say ... to`) | actors and sets in scope |

Suggestions should respect:

- the current [[Reference Modes|reference mode]] (declaration vs process)
- the current [[Contracts|contract]] (only legal statuses / capabilities)
- [[Scoping|name resolution order]] (local first, then `this`, named Frames, accessible sets, outer scopes, global)

---
## Principles {#principles}
---

- Reject illegal code, but explain how to fix it.
- Prefer concrete suggestions over generic error messages.
- Use the graph: contracts, scopes, and named Frames are all introspectable at compile time.
- Surface the contract surface — if a `failable` action is missing a `failed` handler, name the contract.

---
## One-Line Lock {#one-line-lock}
---

**Marco is strict at compile time and generous in guidance.**

---
## Open Questions {#open-questions}
---

- Severity model — errors vs warnings vs notes; whether warnings are configurable per project.
- Whether the compiler emits machine-readable diagnostics (LSP / JSON) alongside human-readable text by default.
- Quick-fix protocol — whether suggested fixes are auto-applicable or advisory only.
- How autocomplete handles partial sentences mid-line (e.g., between two run-on `when` clauses).
- Whether the compiler exposes the inferred contract for an action (see [[Contracts#Implicit Contracts]]) directly in tooling.
