---
status: reference
---

---
# Locking {#locking}
---

---
## Definition {#definition}
---

Locking is used to acquire exclusive mutation access to a set.

Canonical form:

```marco
lock Settings...
```

### Example

Canonical:
```marco
lock Settings...
    that's Theme is Dark.
    this ok with that!
```

---
## Semantics {#semantics}
---

`lock <Set>...` is equivalent to:

```marco
wait until <Set> unlocked...
<Set> is locked.
```

Meaning:

- waits until the set is available
- acquires exclusive mutation access
- exposes the mutable set via `that`
- releases the lock when the phrase closes

---
## Rules {#rules}
---

- `lock` creates a Frame
- `lock` blocks until the set is unlocked or the phrase resolves
- inside a successful lock:
- `that` refers to the mutable set
- mutation is allowed
- outside a lock:
- shared sets are read-only unless copied

---
## Mutation Rule {#mutation-rule}
---

Lockable sets may not be mutated unless locked.

Invalid:

```marco
that's Theme is Dark.
```

Valid:

```marco
lock Settings...
    that's Theme is Dark.
```

---
## Timeout Example {#timeout-example}
---

Canonical:
```marco
lock Settings...
    when it's been 5 seconds?
        this failed with error "lock timeout"!
```

---
## Lock Release {#lock-release}
---

Locks are released automatically when the phrase closes.

No explicit unlock is required.

---
## Relationship To `wait` {#relationship-to-wait}
---

- `wait until <Set> unlocked...` = passive check
- `lock <Set>...` = active acquisition

---
## Syntactic Sugar {#syntactic-sugar}
---

`lock` is the preferred direct form.

The expanded form defines its behavior.

One-line lock:

`lock` acquires exclusive mutation access to a set for the duration of a phrase.

---
## Constraint {#constraint}
---

Do not introduce additional locking keywords.
