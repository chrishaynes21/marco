---
status: reference
---

---
# Example {#example}
---

A complete canonical example exercising every required v1 element using only defined syntax.

Required elements: actor, action, contract, set, error, phrase, `do`, `start`, `when`, `or?`, `it is ...!`, `that`, `that's`, `lock`, `say`, `hears`.

Open Question: `copy` was requested for this example but has no defining section in the spec. Treated as undefined — see [[Audit#Undefined Syntax]]. Not used here.

---
## SaveApp {#saveapp}
---

```marco
// SaveApp — canonical example.

the error SaveError is an error.
this's Code is a text.

the Saveable is a contract.
this allows Saved with SaveData.
this allows failed with an optional error.
this can retry.

the SaveData is a set.
this's Path is a text.

the Game is an actor.
this's State is a SaveData.

this can WriteToFile.
this's WriteToFile does...
    this is ok with input!

this can Save.
this's Save does...
    that is Saveable.

    lock this's State...
        do this's WriteToFile with this's State...
            when ok?
                this is Saved with that!
            or?
                this is failed with that's error!

when this hears SaveRequested?
    start this's Save as SaveFlow...

    when SaveFlow is Saved?
        says SaveCompleted to this!

    when SaveFlow is failed?
        says SaveFailed to this!
```

---
## Element Trace {#element-trace}
---

| Required Element | Where it appears |
|------------------|------------------|
| actor | `the Game is an actor.` |
| action | `this can Save.` + `this's Save does...` |
| contract | `the Saveable is a contract.` |
| set | `the SaveData is a set.` |
| error | `the error SaveError is an error.` |
| phrase | `does...`, `do...`, `start...`, `lock...` (multiple) |
| `do` | `do this's WriteToFile with this's State...` |
| `start` | `start this's Save as SaveFlow...` |
| `when` | `when this hears SaveRequested?`, `when SaveFlow is Saved?`, `when ok?` (shorthand for `when that is ok?`), etc. |
| `or?` | inside `Save` — exclusive fallback to `failed` |
| `this is <status>!` | `this is Saved with that!`, `this is failed with that's error!`, `this is ok with input!` |
| `that` | implied by `when ok?` shorthand (refers to the `WriteToFile` Frame) |
| `that's` | `that's error` (the Frame error slot, special-cased) |
| `lock` | `lock this's State...` |
| `says` | `says SaveCompleted to this!`, `says SaveFailed to this!` |
| `hears` | `when this hears SaveRequested?` |
| `copy` | **omitted — undefined in v1; see [[Audit#Undefined Syntax]]** |

---
## Walkthrough {#walkthrough}
---

1. `SaveError` extends the built-in `error` with a `Code` field. See [[Lifecycles#Custom Error Types]].
2. `Saveable` declares two statuses (`Saved`, `failed`) and a `retry` capability. See [[Contracts#Capabilities]].
3. `SaveData` is a set with a `Path` field. See [[Data Model#Sets]].
4. `Game` is an actor with `State` typed as `SaveData`. See [[Actors#Declaration]].
5. `WriteToFile` is a helper action that echoes its input (placeholder for I/O).
6. `Save` declares `that is Saveable.` to adopt the contract. Inside the body, `lock this's State...` opens an exclusive section. The blocking `do this's WriteToFile with this's State...` writes the state. The branch group resolves: `when ok?` (shorthand for `when that is ok?`) returns `Saved`; `or?` returns `failed` carrying the child Frame's error via `that's error`.
7. The `SaveRequested` listener kicks the save off async with `start this's Save as SaveFlow...`. The named `SaveFlow` Frame is then observed: `when SaveFlow is Saved?` → `says SaveCompleted to this!`; `when SaveFlow is failed?` → `says SaveFailed to this!`.

---
## Open Questions Triggered By This Example {#example-open-questions}
---

- `copy` is needed for "snapshot the state before writing" but has no defining section.
- `do this's WriteToFile with this's State...` — qualified action invocation form (`<Subject>'s <Action>`) is implied by the possessive rule but not enumerated for action invocation specifically.
- `this is ok with input!` returns the received input as the result. Semantics OK, but not shown elsewhere — confirms `input` is a valid `with` argument.
- The `failed` branch of the `or?` accesses `that's error`. This relies on the `that's error` special case (Frame slot, not result subfield). The example would silently break if read against the general "`that's <field>` accesses result" rule from Core Concepts and Inference. See [[Audit#Rule Conflicts]] B.
