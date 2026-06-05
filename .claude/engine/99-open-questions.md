# Engine — Open Questions

Running list of engine-specific gaps that the spec doesn't fully resolve and that affect implementation.

Categorized loosely. Some bleed into language-level questions — those should bubble up to the spec audit.

---

## Frame lifecycle

- **Orphaned async frames** — when a parent resolves before its async child completes, what happens to the child? Lean: child continues independently, root reaps on script exit. Need to lock.
- **Cancelation propagation** — if a parent is canceled, are children canceled? Spec doesn't say. Lean: yes, parent cancel cascades to children unless they declare `noCancel` or similar. Open.
- **Frame reaping** — completed frames stay around for `that` access. When can we actually free them? Tied to scope lifetime; need clear rule. Lean: free when the parent that owns the binding completes.
- **Re-entry into the same frame** — can the same Frame be resumed twice? Probably not — frame transition Active → Waiting → Active is fine, but after Completed it's terminal. Document.

---

## Statuses

- **Status alias / hierarchy** — does `Saved` automatically satisfy `ok`? Lean: no, distinct names are distinct. Could revisit if `ok-ish` patterns become common.
- **Built-in status set** — what's the canonical list? `ok`, `failed`, `canceled` clearly. `Saved` and `Saving` are user-introduced (not built-in). What about `ready`, `active`, `waiting`, `finished`, `unlocked`, `locked`? They appear in built-in contracts but aren't enumerated. Lock the full list.
- **Empty-slot read** — `that's error` when no error has been set: return what? Spec says errors live on the slot but doesn't define empty-slot read. Lean: return `none` (a built-in absent value), and `has error` is the way to gate it.

---

## Capabilities

- **User-defined capability dispatch** — Built-ins (`retry`, `cancel`) have engine semantics. Can users define their own? Spec says yes via `this can X.` + `this's X does...`. Implementation: registered handler in the capability table. Confirm the dispatch semantics: does `do that retry.` re-run the action with same input? If user-defined, what's the input?
- **`commit` and `rollback` semantics** — listed as capabilities but not designed. Default to no-op + trace event for v1; flesh out later.
- **Capability arguments** — can capabilities take arguments? `do that retry with NewBackoff.`? Implies invocation form `do <ref> <cap> with <input>.` Spec doesn't show this; lean no for v1.

---

## Listeners

- **Listener fairness** — many listeners on one message. Install-order? Hash-order? Random? Default install-order; document. Multi-threaded later may force a change.
- **Listener removal during fire** — listener body cancels another listener. Single-threaded: safe. Document.
- **Live vs snapshot for set-based routing** — `say X to Components.` where Components is a set: if Components changes mid-fan-out, should the new members receive too? Default: snapshot at `say` time. Reconsider.
- **Listener install latency** — between actor construction and listener install, can events be missed? In v1, actor construction happens synchronously when the actor is declared/instantiated, so no events can fire before listeners install. Not a problem unless we add lazy actor instantiation later.

---

## Locks

- **Reader/writer locks** — v2 only?
- **Plain set lockable by default?** Lean no. Lockable is opt-in. `the X is a set.` is not lockable; `the X is a lockable set.` is. Need to confirm syntax.
- **Deadlock** — no detection in v1; rely on `what happened?` to expose. v2: cycle detection.
- **Lock holder transfer** — if `start` inherits from a frame holding a lock, does the started child get the lock? Lean no — independent frame, must re-acquire.
- **Read access while locked** — can other frames *read* a locked set? Lean yes. Lock is for write exclusion only.

---

## Imports & resolution

- **Module loading order** — when modules import each other, what's the order? Topological if no cycles; cycles error. Standard.
- **Module reload / hot-reload** — for the REPL or live scripting. Out of scope for engine MVP but informs the design — modules should be replaceable without restarting the runtime.
- **Lazy resolution caching** — once a name resolves to module M, do we cache? Lean yes; invalidate on reload.
- **Self-referential `this's <Action>` invocation** — when actor body invokes its own action, qualify or not? Lean: `this's Action` is canonical; bare `Action` is shorthand iff unambiguous.

---

## Data model

- **Custom error types with type tagging** — `failable with SaveError` vs generic error. Runtime tag for inspection? Lean yes — store type ID on the error value.
- **List access bounds** — `Items at 99` when list has 5 items: error or none? Spec says "indices must be valid" — runtime error.
- **Map keys typed** — spec is open on whether map keys are restricted. Engine-side: arbitrary primitive values OK; arbitrary sets gets weird (set equality).
- **Nested set mutation** — `User's Settings's Theme is "dark".` mutates a nested set. Is the nested set its own set or part of User? Lean part of User; mutations ripple up.
- **Default values resolution** — `<X>'s Y is Z by default.` evaluated at access time. Where are defaults stored? On the type (Node) or on the instance (Set)? Lean Node — defaults are part of the type's contract.

---

## Iteration

- **Feed iteration bounds** — `for each in <Feed>` is potentially infinite. Spec flags this. Default: runs until feed is closed or `stop.` issued.
- **Queue iteration semantics** — `for each in <Queue>` consumes? Or peeks? Lean consumes (it's a queue).
- **Iterator frame for actors** — `the Party is iterable.` + `this's iterator does... for each Member in Members... this is ok with Member!`. Each iteration emits a value via `this is ok with V!`. Engine: the iterator action is invoked once per call to `for each`, and each `this is ok with V!` yields a value. The action *resumes* between yields — coroutine-shaped. v1: support this via cursor parking (the iterator's cursor parks after each yield, resumes on next `for each` step).

---

## Tracing & debug

- **`what` query language** — full grammar. The five canonical forms are good; what about `what is User?` where User is a binding? Lean yes, generalize.
- **Trace persistence** — serialize trace tree? Useful for post-mortem and CI. Out of scope for MVP.
- **Per-frame tracing toggle** — disable tracing for hot paths. Run config flag, not language.
- **Breakpoint surface** — `now break.` is preliminary. Lock once we've used it in practice.

---

## Tooling

- **LSP integration** — Marco needs an LSP for the autocomplete table to actually be useful in editors. Plan: separate process that consumes the parser + graph builder, exposes diagnostics + completions.
- **Error recovery** — parser must continue after errors so the LSP shows multiple errors at once. Standard parser tech (synchronization tokens at sentence boundaries — `.`, `!`, `?` make it easy).

---

## Implementation

- **Go vs Rust** — Lean Go for v1. Decision should be made before writing more than ~500 LOC.
- **No-GC concerns** — frames are tree-shaped; reference counting is fine if we go Rust. Go's GC handles it transparently.
- **Embedding** — can Marco be embedded in another process (e.g., a game engine)? Should design for it: runtime as a library, not an executable. CLI is a thin wrapper.
- **Cross-platform** — Windows / macOS / Linux. Go: easy. Rust: also easy. No concern.

---

## Things that deserve to bubble back to the spec

These are language-level questions the engine raised:

- Status name list (built-in vs user-defined)
- Empty-slot semantics for `that's error` when no error
- Whether `lockable` is implicit on plain sets
- Whether canceled parents propagate to children
- `what` query grammar generalization
- `now break.` syntax lock

I'll surface these to the spec audit as Open Questions, batched.

---

## State of these notes

This is the seventh and final planning file for the first sketch. Everything in this folder is preliminary — I'd expect to revise heavily once we have a real lexer in front of us. The shape should hold; the details will move.

Files in this folder:

```
00-overview.md              done
01-steering-edges.md         done
02-runtime-shape.md          done
03-execution-loop.md         done
04-scheduler.md              done
05-contracts-runtime.md      done
06-listeners-and-events.md   done
07-locks.md                  done
08-debug-and-what.md         done
09-mvp-slice.md              done
99-open-questions.md         done (this file)
```
