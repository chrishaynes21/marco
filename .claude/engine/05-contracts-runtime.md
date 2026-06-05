# Engine — Contracts at Runtime

The compiler does most of the work. The runtime exists for cases the compiler can't see (dynamic dispatch, capability lookup, error propagation).

---

## What the compiler enforces (statically)

- Exhaustiveness: every contract status produced by a child action has a matching `when` arm or is captured by `or?` or returned upward by `!`.
- Capability legality: `do that <cap>.` is rejected if `<cap>` isn't declared on `that`'s contract.
- Required-field presence: `that's <field>` for a contract-required field is always safe; for an optional field the compiler insists on a `has` proof in scope.
- Status name validity: `when that <status>?` checks that `<status>` is in the contract.
- Ambiguity errors per Imports / Scoping rules.

When the compiler can prove all this, the runtime doesn't need to check. Runtime checks exist for the residue.

---

## What the runtime enforces

### Capability dispatch

`do that <cap>.` at runtime:

```
1. resolve `that` → frame F
2. let C = F.Origin.Contract
3. if cap not in C.Capabilities: panic("invalid capability: cap not declared on contract C")
4. dispatch built-in or user-defined handler
```

In well-typed code (statically checked), step 3 never fires. It's there for FFI / dynamically constructed sentences (REPL, debug). On a panic, we return a structured runtime error rather than crash — but it's a programming error in normal use.

### Obligation propagation

The compiler statically proves every child status is handled in the parent's branch group. The runtime's job is just to *execute* the handlers. If a child completes with a status the parent didn't compile a handler for, the compiler will have caught it — the runtime should never see "unhandled status" at execution time.

There's one exception: dynamically-constructed actions where the compiler can't see the call site. For v1, we don't support that — all action invocations are statically visible. So obligation enforcement is purely a compile-time concern.

### Status validity on emit

`this is <Status>!` requires `<Status>` to be in the current frame's contract. Checked statically when the contract is known. If the contract is *inferred* (not explicitly declared), the runtime accumulates emitted statuses to populate the inferred contract for tooling purposes, but doesn't reject anything.

### Frame slot access

`that's error` always works — the slot exists on every frame, populated or not. Reading an empty slot returns `none` (or whatever we settle on for absent values; see Open Questions below).

`that's <field>` for a non-`error` field reads from `that.Result`. If the field is contract-required, it must be present (compiler-checked). If optional, the compiler required a `has` proof — runtime trusts the proof.

### Required fields on input

When invoking an action, the compiler matches the input set against the action's input contract. Missing required fields → compile error. Extra fields → either ignored (lenient) or rejected (strict). Strict by default; lenient via an opt-in pragma later.

---

## Built-in contracts at runtime

Each built-in contract has a runtime hook.

| Contract | Runtime support |
|---|---|
| `failable` | Default `failed` UserStatus; `error` slot populated by `RESOLVE`. |
| `failable with error` | Same as `failable`, plus `error` is required to be non-empty on `failed` resolution. |
| `waitable` | Provides `active`/`waiting`/`finished` UserStatus values; default state machine. |
| `cancelable` | Adds `canceled` status; capability `cancel` invokes a runtime hook that sets status, fires observers, and prevents further state changes. |
| `lockable` | Adds `locked`/`unlocked` states; integrates with the lock manager (see `07-locks.md`). |
| `retryable` | Capability `retry` re-invokes the action with the same input as a sibling frame; adds `retried` status to the current frame for trace. |
| `iterable` | Gates `for each` legality. Iteration delegates to `iterator` member, which produces values. |

These are not user-extensible in v1. Custom contracts are user-defined but built-ins are runtime-special.

---

## Inferred contracts

If an action doesn't declare a contract, the compiler walks its body and collects every status it might emit. That set becomes the inferred contract.

The runtime stores inferred contracts so `what can that do?` can show them to the developer. For tooling — not for enforcement (the compiler already handled enforcement).

---

## Errors as values

Errors are not exceptions. There's no unwinding. A `failed` resolution is just a frame transition with a populated `error` slot. The parent frame sees it via the same channels (`when failed?`, `that's error`).

That means: no try/catch, no error propagation overhead, no special control flow. Errors flow on the same edges as success.

This is a load-bearing simplification — it means the runtime has *one* control flow story, not two.

---

## Capability dispatch table (sketch)

Built-in capability handlers, keyed by capability name:

```go
var BuiltinCaps = map[CapName]CapHandler{
    "retry":    handleRetry,
    "cancel":   handleCancel,
    "rollback": handleRollback,    // semantics TBD
    "commit":   handleCommit,      // semantics TBD
}

type CapHandler func(target *Frame, args *Set) error
```

User-defined capabilities (declared with `this can <X>.` and implemented with `this's <X> does...`) are dispatched through the same table — registered at graph-build time when the action is defined.

For built-ins not yet designed (`rollback`, `commit`), the v1 plan: implement as no-ops that emit a structured trace event. Lock semantics later.

---

## Validation lifecycle

```
1. Parse: source → sentence trees
2. Build graph: declaration sentences → nodes/edges in the graph
3. Compile bodies: process-mode sentences → Block of Edges
4. Static validation pass:
     - resolve all names
     - check contract exhaustiveness
     - check capability declarations
     - check presence proofs for optional fields
     - check ambiguity in imports
5. Runtime: execute as designed
```

If any step in 4 fails, the runtime never starts — we surface diagnostics per the Compiler spec.

---

## Open

- **Custom error types** at runtime — when a contract says `failable with SaveError`, do we runtime-check that the error slot is a `SaveError` instance, or trust the compiler? Lean: trust, but tag with type info for `what's that's error?` to display correctly.
- **Status alias / hierarchy** — should `Saved` automatically satisfy `ok` (i.e., is `Saved` a sub-status)? Spec doesn't say. Lean: no, distinct names are distinct. User can declare both: `this has status Saved.` and `this has status ok.`, but they don't share semantics by default.
- **Empty-slot read** — `that's error` when no error has been set. Return `none`? Return empty set? Make it a compile error if the frame can't have an error? Open.

---

## Next file

`06-listeners-and-events.md` — listeners in detail (passive vs branch vs wait), feed/queue mechanics.
