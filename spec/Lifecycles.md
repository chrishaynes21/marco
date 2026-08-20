---
status: reference
---

---
# Lifecycles {#lifecycles}
---

This page defines status behavior for [[Frames|Frames]].

---
## Canonical Status Set {#canonical-status-set}
---

The eight built-in lifecycle states:

| Status | Phase | Meaning |
|--------|-------|---------|
| `created` | initial | Frame allocated, not yet running |
| `running` | active | Frame is executing edges |
| `waiting` | active | Frame is parked on a listener / wait / lock |
| `ok` | terminal | Successful resolution |
| `failed` | terminal | Failure resolution; carries `error` |
| `canceled` | terminal | Resolved via the `cancel` capability |
| `exited` | terminal | Graceful early exit |
| `died` | terminal | Abnormal termination (panic-equivalent) |

User-introduced statuses (e.g. `Saved`, `Saving`, `Ready`) layer on top via [[Contracts|contract]] declarations. The eight above are always available without declaration.

Resolution moves a Frame to one of the five terminal statuses (`ok`, `failed`, `canceled`, `exited`, `died`). Once terminal, the Frame is frozen.

---
## State Updates {#state-updates}
---

`.` updates a Frame status without returning or resolving.

A state update changes the current Frame's status but does not close the owning phrase.

### Example
```marco
it is Saving.
it is ok.
```

These lines update the status. They do not return or resolve the phrase.

---
## Returns {#returns}
---

`!` closes the owning [[Phrases|phrase]] and returns from it.

### Example

Canonical:
```marco
this is ok with that!
this is failed with its error!
```

Optional shorthand (secondary):
```marco
this ok with that!
```

Rule: **Returns should read like English.** Keep `is` in the canonical form. The no-`is` shorthand is permitted but secondary.

`it is <Status>!` and `it's <Status>!` are no longer canonical for returns.

### Return Validity

A return must include a status.

`this is that!` is **invalid** — `that` is a Frame, not a status. A return must say what status the Frame becomes.

Valid returns:

```marco
this is ok with that!
this is failed with its error!
```

Invalid:

```marco
this is that!
```

### Branch-Return Shorthand

In a branch group, return sentences may elide `this is` when the meaning is unambiguous.

Canonical:
```marco
when failed?
    this is failed with its error!
or?
    this is ok with that!
```

Compact:
```marco
if failed, failed with its error!
or, ok with that!
```

Both forms are equivalent. The compact form elides `this is` and uses `if`/`or` as branch starters; the canonical `when`/`or?` remains the source of truth.

---
## Ownership {#ownership}
---

Only [[Phrases|phrases]] own returns. Branches (e.g. `when`) do **not** own returns.

`!` always returns from the owning phrase, not from the branch in which it appears.

### Example
```marco
do Save...
    when ok?
        this is ok with that!
```

The `this is ok with that!` returns from `Save`, not from the `when` branch.

---
## Frame Status Predicates {#frame-status-predicates}
---

A status check reads `when <frame> is <status>?`. Either both the frame ref and `is` appear (canonical), or both are dropped (shorthand).

| Form | Meaning |
|------|---------|
| `when that is <status>?` | Canonical — previous / watched Frame |
| `when this is <status>?` | Canonical — current Frame |
| `when <NamedFrame> is <status>?` | Canonical — named Frame |
| `when <status>?` | Shorthand — implies `that is <status>?` when unambiguous |

### Example

Canonical:
```marco
when that is failed?
when that is ok?
when SaveFrame is failed?
when SaveFrame is canceled?
```

Shorthand (when `that` is unambiguous):
```marco
when failed?
when ok?
```

Rule: **Checks read like English at the canonical layer; shorthand drops both `that` and `is` for terseness.**

The half-shorthand `when <frame> <status>?` (no `is`) is **not** canonical. Use either the full canonical or the bare-status shorthand.

Data comparison `when User's Age is 30?` is not a status predicate and is unaffected.

---
## `when` {#when}
---

A `when` sentence with `?` inspects a condition.

If the subject is settled, `when` acts as an immediate check.

If the subject is active, `when` acts as a listener.

`when` does not modify the inspected Frame or result.

### Example
```marco
when this is ok?
when this is failed?
when SaveFlow is ok?
```

See [[Phrases]] for how branches resolve.

---
## Named Frames {#named-frames}
---

A phrase may be given a name with `as` so its status can be observed from outside the phrase.

### Example
```marco
do Save as SaveFlow...

when SaveFlow is ok?
```

---
## Cancellation Propagation {#cancellation-propagation}
---

`canceled` is a terminal status reachable through:

- the `cancel` capability (`do that cancel.` on a Frame whose contract permits cancel — see [[Frames#Capabilities]])
- `stop.` inside a loop or phrase body (transitions the loop's owning Frame to `canceled`)
- propagation from a canceled parent

Rules:

- when a Frame is canceled, its still-active children are also canceled
- propagation is depth-first across the [[Frames#Ownership|Frame ownership tree]]
- children that have already resolved (terminal status) are unaffected — their resolution stands
- canceled Frames trigger any registered `finally...` blocks before being marked terminal

Cancel does **not** flow upward automatically — a child's cancellation does not cancel its parent. The parent must observe the cancellation via `when child is canceled?` and decide how to respond.

`stop.` vs cancel: `stop.` is loop-local control flow that resolves the loop with `canceled`. `cancel` capability is an external action that targets a specific Frame.

---
## Cleanup — `finally` {#cleanup-finally}
---

`finally...` opens a cleanup phrase that runs when the owning phrase closes.

### Canonical Form

```marco
do Save...
    when this is ok?
        this is ok with that!
    or?
        this is failed with that's error!
finally...
    do Cleanup.
```

### Rules

- runs on `ok`, `failed`, `canceled`, `exited`, or `died` — every terminal state
- runs even when a `return` (`!`) occurs in the main phrase
- must **not** silently override the phrase result
- may only override result/status if it explicitly returns with `this is <status>!`
- multiple `finally...` blocks chain in declaration order; each runs regardless of the previous's outcome

If `finally...` itself fails or dies, its status is propagated only if it explicitly returned. Otherwise the owning phrase's original resolution stands and the failure is logged.

### Example — Inspecting Status In Cleanup

```marco
do Save...
    when this is ok?
        this is ok with that!
    or?
        this is failed with that's error!
finally...
    when this was failed?
        log "save failed; rolling back".
        do its rollback.
```

`when this was failed?` is the past-tense form for inspecting a **resolved** Frame from cleanup. The Frame is terminal at this point, so `is` would be misleading.

---
## Errors {#errors}
---

Errors are first-class. They live on a dedicated Frame slot — not inside the result — and have their own type system.

### Shorthand

The string-error shorthand is the easy surface form:

Canonical:
```marco
this is failed with error "save failed"!
```

This sets status to `failed`, populates the Frame's `error` slot with a default error whose `Message` is `"save failed"`, and returns.

### Canonical Expanded

The shorthand expands to three explicit state updates:

```marco
it is failed.
its error is an error.
its error's Message is "save failed".
```

The closing `!` on the shorthand returns; the expanded form sets state without returning.

### Custom Error Types

Errors may be declared as named types:

Canonical:
```marco
the error SaveError is an error.
it's Code is a text.
```

`SaveError` extends the built-in `error` and adds a `Code` field.

### Usage

Canonical:
```marco
this is failed with error "missing save file"!
this is failed with SaveError!
```

The first form uses the string-error shorthand. The second returns `failed` with a `SaveError` instance.

### Access

`its error` accesses the current Frame's error slot directly.

`that's error` accesses the child Frame's error slot directly (special-cased — not `that.result.error`).

See [[Frames#Frame Slots]] for the full access rule.

### Rule

Results hold returned data; Frames hold errors. Do not bury errors inside result sets, and do not shape error payloads as if they were result fields — errors have their own slot and their own type system.

---
## Time {#time}
---

Time is part of the [[Frames|Frame]] lifecycle.

### Example

Canonical:
```marco
when it's been 5 seconds?
```

Meaning: the Frame has existed for 5 seconds.

There is no global timer; time is observed relative to a Frame's own lifetime.

---
## One-Line Lock {#one-line-lock}
---

Marco unifies control flow, async, events, state, errors, and time through Frames, sets, and statuses.

---
## Open Questions {#open-questions}
---

- The canonical set of status values beyond `ok` and `failed`.
- Whether `.` (state update) may carry a `with that` result, or whether results are exclusive to `!` (return).
- Resolution order when multiple `when` branches match the same status event.
- Scope and lifetime of a named Frame (e.g. `SaveFlow`) after its owning phrase resolves.
