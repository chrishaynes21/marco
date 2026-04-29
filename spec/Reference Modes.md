---
# Reference Modes {#reference-modes}
---

Marco has two major reference modes. Each mode anchors pronouns to a different subject.

- **Declaration mode** describes graph shape.
- **Process mode** describes Frame execution.

---
## Declaration Mode {#declaration-mode}
---

Declaration mode is used while defining actors, sets, actions, scenes, contracts, and errors.

In declaration mode:

- `it` refers to the current declared subject
- `it's` accesses the current subject
- declarations may update `this` only through `the`

### Example

Canonical:
```marco
the Game is an actor.
it can Save.
it has SaveFile.
it's Name is a text.
```

After `the Game is an actor.`, `it` is `Game`; subsequent declarations describe `Game` without changing focus until another `the` appears.

See [[Declarations]] and [[Actors]].

---
## Process Mode {#process-mode}
---

Process mode is used inside phrases opened by `does...`, `do...`, `start...`, or `wait...`.

In process mode:

- `this` refers to the current [[Frames|Frame]] being resolved
- `that` refers to the child or observed Frame
- `that's` accesses that Frame's result set

### Example

Canonical:
```marco
do Save...
    when that is failed?
        this failed with that's error!
```

See [[Frames#Process Mode References]] and [[Phrases]].

---
## Mode Rule {#mode-rule}
---

Declaration mode describes graph shape (nodes, edges, members, capabilities).

Process mode describes Frame execution (status changes, results, capability invocation).

Do not mix object anchors (declaration) and Frame anchors (process) implicitly.

If a reference is ambiguous between modes, the compiler requires explicit qualification.

See [[Scoping]] for general name resolution and [[Inference#Ambiguity Rule]] for the broader inference policy.

---
## One-Line Lock {#one-line-lock}
---

`it` shapes declarations. `this`/`that` drive execution.

---
## Open Questions {#open-questions}
---

- Whether mode is always inferable from syntactic context, or whether some constructs straddle modes.
- Formal qualification syntax when a reference is ambiguous between modes.
- Whether declarations inside a process-mode phrase (e.g., a local `the X is a set.`) switch mode for that line only.
