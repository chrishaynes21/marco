---
# Observability {#observability}
---

How Marco programs are inspected, traced, and logged. Errors live on [[Frames|Frame]] slots (see [[Lifecycles#Errors]]); this page covers the surface for observing execution without altering it.

---
## Three Verbs — `log`, `inspect`, `show` {#three-verbs}
---

Marco distinguishes three runtime side-effect verbs by audience:

| Verb | Audience | Purpose |
|------|----------|---------|
| `log` | machines / records | Structured record for traces, monitoring, post-mortem |
| `inspect` | developers | Human-friendly developer view in dev tools |
| `show` | end users | UI rendering — see [[Modules#Show — UI Rendering]] |

All three are side-effect-only: they do not affect control flow.

---
## Logging — `log` {#logging}
---

`log` is a built-in action. Records structured data to the trace.

### Canonical Form

```marco
log <Value>.
```

### Examples

Canonical:
```marco
log "starting save".
log that's error.
log User.
```

### Rules

- logging does not affect control flow
- logging is side-effect only
- `log` writes to the trace tree / configured log sink
- `log` is a system built-in (see [[Core Concepts#Naming Convention]] — lowercase identifies built-ins)

---
## Inspect — `inspect` {#inspect}
---

`inspect` is a developer-facing introspection verb. Unlike `log` (structured record) or `show` (UI render), `inspect` produces a developer-readable description suitable for dev tools, REPL output, or attached debuggers.

### Canonical Form

```marco
inspect <Value>.
```

### Examples

Canonical:
```marco
inspect that.
inspect that's error.
inspect User.
```

### Rules

- `inspect` is for developer-facing introspection
- output goes to the active dev tool / REPL / IDE channel, not the user-facing UI
- `inspect` does not affect control flow
- `inspect` is a system built-in

`inspect` differs from `what` ([[#Debugging — `what`]]) in that `inspect` is a sentence in normal execution flow, while `what` is a debug-only query at a breakpoint or REPL.

---
## Observation {#observation}
---

Marco allows observing execution without modifying it. Status listeners are the primary observation pattern.

### Example

Canonical:
```marco
when that is failed?
    log that's error.
```

Rules:

- observation does not consume the Frame
- multiple observers may exist
- observers do not own returns (see [[Phrases#Branch Rule]])

Listeners attached to actor messages are also observers — see [[Actors#Messaging Syntax]].

---
## Tracing {#tracing}
---

Marco tracks Frame execution. Each Frame may carry trace data:

- `id`
- `parent`
- `children`
- `start time`
- `end time`

[[Frames|Frames]] form a trace tree, mirroring the runtime [[Graph#Frame Graph|Frame graph]].

### Trace Access (Conceptual)

```marco
callstack
previous
root
```

### Examples

Canonical:
```marco
log callstack.
log root.
```

Rules:

- trace data is read-only
- used for debugging and introspection
- trace data does not affect execution

---
## Debugging — `what` {#debugging}
---

`what` is the debug / introspection keyword. It is **not** part of execution flow.

`show` is for UI rendering, not debugging — see [[Modules#Show — UI Rendering]].

### Canonical Forms

```marco
what is that?
what's that?
what happened?
what was previous that?
what can that do?
```

### Meaning

| Form | Meaning |
|------|---------|
| `what is that?` | Inspect the [[Frames|Frame]] (`that`) |
| `what's that?` | Inspect `that`'s result |
| `what happened?` | Show recent Frame activity / callstack |
| `what was previous that?` | Inspect parent Frame |
| `what can that do?` | List capabilities from the [[Contracts|contract]] |

### Contractions

`what's` is valid shorthand for `what is`. (The `'s` then composes with Marco's standard possessive — `what's that?` reads as "what is *that's* [result]", inspecting the result set rather than the Frame itself.)

### Rules

- `what` is only valid in debug mode, REPL, or inspector
- `what` does not create [[Frames|Frames]]
- `what` does not affect execution
- `what` cannot be used inside phrases (`do`, `when`, etc.)
- `what` returns human-readable output

### Examples

```marco
what is that?
what's that?
what can that do?
what happened?
```

### Principle — Conversational Debugging

Marco supports conversational introspection.

Execution is structured. Debugging is natural language.

### One-Line Lock

`what` is a debug-only introspection command that queries Frames, results, and capabilities.

---
## Errors and Observability {#errors-and-observability}
---

Errors are first-class Frame slots. Observation patterns combine with the [[Lifecycles#Errors|error model]]:

```marco
when that is failed?
    log that's error.
    this is failed with that's error!
```

This logs the failure without consuming it, then propagates the failure upward.

See [[Lifecycles#Errors]] for the full error model.

---
## One-Line Lock {#one-line-lock}
---

Errors live on Frames, logs observe execution, and tracing exposes the Frame graph.

---
## Open Questions {#open-questions}
---

- Full breakpoint semantics — how a debug session pauses execution and binds `that` to the frame at the breakpoint. Initial direction (preliminary): something like `now break.` to suspend at the current point. Form and integration TBD.
- Whether `what` accepts free-form natural language beyond the canonical forms above.
- Whether `log` writes to a structured trace, a stream sink, or both by default.
- Log levels (`info`/`warn`/`error`) — built-in or expressed through the contract system.
- Whether `callstack`, `previous`, `root` are bare-name references or accessed through `its`.
- Whether tracing can be selectively disabled per Frame for performance.
- Exact format/structure of the `id` slot (UUID, monotonic counter, scoped path).
