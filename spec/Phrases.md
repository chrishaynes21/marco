---
status: reference
---

---
# Phrases {#phrases}
---

---
## Definition {#definition}
---

A phrase is an open [[Frames|frame]] created by `...`.

A phrase owns:

- its own status and result context
- its own return with `!`
- a stack of child [[Frames|frames]]

A phrase must resolve before returning to its caller.

### Completed vs Open Frames

Canonical:
```marco
do Save.
```

This creates a **completed Frame**. The sentence `.` ends and the frame is done.

Canonical:
```marco
do Save...
```

This creates an **open Frame** (a phrase). The `...` opens a block that must later be closed with `!`.

### Example

Canonical:
```marco
do Save...
    this is ok with that!
```

---
## Ownership {#ownership}
---

Only phrases own returns.

Branches such as `when` do not own returns. A `!` inside a branch returns from the branch's owning phrase.

### Example

Canonical:
```marco
do Save...
    when failed?
        this is failed with that's error!
```

The return belongs to `Save`, not to `when`.

---
## Return Semantics {#return-semantics}
---

A return with `!` closes the owning [[Phrases|phrase]] and produces a completed Frame.

Canonical:
```marco
this is ok with that!
this is failed with its error!
```

When a phrase returns:

- The owning phrase closes
- The current Frame becomes completed
- The completed Frame is passed to the parent as the most recent result
- `that` in the parent now refers to the returned Frame's result
- `it` in the parent continues as the parent Frame

### Example

Canonical:
```marco
do X...
    do Y...
        this is ok with Result!
```

After `Y` returns:

- `Y`'s Frame is completed
- `X` receives `Y`'s Frame as the most recent result
- Inside `X`, `that` now refers to `Result`
- Inside `X`, `it` still refers to `X`'s Frame

---
## Return Propagation {#return-propagation}
---

A returned Frame is propagated **exactly one level up**.

Return does **NOT** automatically bubble to the root.

Higher-level propagation must be explicit.

### Example

Canonical:
```marco
do X...
    do Y...
        when ok?
            this is ok with Result!
```

After `Y` returns:

- `Y` closes and returns to `X`
- `X` continues execution
- `X` may decide to return or continue
- If `X` returns, its Frame goes to `X`'s parent
- The return from `Y` does **not** skip `X`

---
## Branch Rule {#branch-rule}
---

Branches such as `when` do not own returns.

When a `!` appears inside a branch, it closes the phrase that owns the branch.

Canonical:
```marco
do Save...
    when failed?
        this is failed with its error!
```

This `!` closes `Save`, not `when`.

---
## Final Lock {#final-lock}
---

A return:

- Closes the current owning [[Phrases|phrase]]
- Produces a completed [[Frames|Frame]]
- Passes that Frame to the parent as the most recent result
- Updates `that` in the parent to the returned result
- Does **not** skip levels in the stack
- Does **not** bubble automatically to root

This makes return semantics precise and removes ambiguity.

---
## Resolution {#resolution}
---

A phrase opened with `...` must resolve explicitly.

Resolution requires:

- `this is <status>!`
- or an equivalent explicit return

Branches such as `when` do not resolve phrases on their own.

**Falling off the end of a phrase is a compile error. There is no implicit `ok` return.** Every code path through a phrase must end in an explicit `this is <status>!`.

Status updates with `it is` (using `.`) do not resolve a phrase.

For child Frames with contracts, branch resolution is required.

The compiler must prove every possible child status has a path to explicit resolution.

### Example

Canonical:
```marco
do Save...
    when ok?
        this is ok with that!

    when failed?
        this is failed with its error!
```

### Invalid Example

```marco
do Save...
    show Banner.
```

Reason: The phrase never resolves.

---
## Child Status Obligations {#child-status-obligations}
---

If a child Frame has unresolved statuses, those statuses propagate to the parent as obligations.

Propagation is one level at a time. Obligations remain active and **haunt** the parent until they are:

- handled with `when`
- captured with `or?`
- transformed into another status
- returned upward with `!`

Continuing execution does not resolve obligations. A phrase may not close while unresolved child statuses remain.

See [[Contracts#Unresolved Obligations]] for the full lifetime model and [[Contracts#`or?` Exclusive Fallback]] for fallback semantics.

### Example

Canonical:
```marco
do save...
    when saved? this is ok with that!
    or? this is failed with its error!
```

This resolves child statuses in the parent phrase.

---
## Invocation {#invocation}
---

Phrases may be invoked in different execution modes. See [[Execution Model]].

### Example

Canonical:
```marco
do Save...
start Save...
execute Save.
```

---
## Open Questions {#open-questions}
---

- Whether a phrase always begins with a default status before its first sentence executes.