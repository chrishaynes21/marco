# Engine — Overview

Working notes for the Marco runtime. Not polished. Will be revised as I go.

---

## Central insight

> Sentence edges steer Marco to nodes to perform actions.

The graph is the substrate (already locked in spec). Every actor / action / set / contract / event is a node. Every traversal — invoking, observing, returning, locking, listening — is an edge. **Sentence punctuation and connectors are not syntactic decoration; they are the edge types of the runtime graph.**

This means the engine is, at its core, a graph walker driven by parsed sentences. There is no separate "instruction set" — the sentence *is* the instruction, and its shape determines which edge type to follow.

---

## Steering verbs and punctuation as edge types

A first-pass taxonomy of edge types implied by the spec:

| Surface form | Edge type | Effect |
|---|---|---|
| `.` | complete | terminate sentence; frame becomes completed |
| `!` | return | close owning phrase, propagate frame to parent |
| `?` | question | read-only inspection; may register a listener |
| `...` | open | phrase opens; child frames may be created |
| `,` | chain | sequential: next clause sees previous frame as `that` |
| `then` | sequence-marker | explicit chain (alias for `,` semantics in run-on) |
| `do X` | invoke | traverse to action node X, run synchronously inline |
| `start X` | invoke-async | traverse to action node X, run on parallel cursor |
| `execute X` | fire-and-forget | traverse, do not retain handle |
| `wait until <C>` | observe-until | suspend cursor until predicate true |
| `lock <S>` | exclusive | acquire mutation rights on set node |
| `say X to Y` | emit | message edge from current actor to Y, fans out |
| `when Y hears X?` | listen | passive edge installed at Y for message X |
| `for each <V> in <C>` | iterate | edge per collection entry |
| `when <pred>?` / `or when <pred>?` / `or?` | branch group | one-of selection |
| `with <S>` | data-in | input set carried along the invocation edge |
| `that's <F>` | data-read | read field on previous frame's result |
| `that's error` | data-read (slot) | read frame's error slot directly |
| `do that <cap>.` | capability invoke | act on previous frame via declared capability |
| `it is <S>.` | state update | mutate current frame status (no edge crossed) |
| `this is <S>!` | resolve+return | mutate current frame status and traverse return edge |
| `the X is <expr>.` | bind+focus | declare and focus subject |
| `<obj> as <name>` | bind | name a frame without focusing |
| `<set>'s <field>` | set access | field access on set/map |
| `<list> at <i>` | list access | indexed access |
| `use <Module>.` | import-edge (lazy) | exposes module's nodes for resolution; no eager pull |

**This table is the contract between parser and runtime.** Whatever the parser emits, the runtime knows how to walk.

---

## Top-level architecture

Pipeline:

```
source text
  └─> Lexer
       └─> Parser → sentence tree
            └─> Graph Builder (declarations build nodes/edges, statically)
            └─> Phrase Compiler (process-mode sentences become traversal programs)
                 └─> Runtime
                      ├─ Frame allocator
                      ├─ Scheduler (sync / start / execute)
                      ├─ Resolver (scoping, imports, lazy)
                      ├─ Contract checker (exhaustiveness + obligations)
                      ├─ Capability dispatcher
                      ├─ Listener registry (when/hears/reads)
                      ├─ Lock manager
                      └─ Tracer (frame tree, callstack, what-queries)
```

The graph itself is the program. The runtime is the cursor walking it.

---

## Frame as the only runtime entity

Big simplification (already locked in spec): `Frame` is a built-in actor with execution-lifecycle state. The engine doesn't need a separate "task" or "fiber" type. Everything that runs is a frame.

That makes the runtime data model very small:

- `Frame` — input, status, result, error + parent / children + listeners
- `Actor` — long-lived; owns a set of frames over time
- `Set` — structured value; the universal data container
- `Edge` — typed transition; transient (consumed during traversal) but recorded for tracing

Open: do listeners need their own type, or are they just frames in a `waiting` status? Lean toward the latter — every listener is a frame parked in `waiting` until the predicate fires, then it transitions to `active` and traverses its body.

---

## What I'm going to plan next

Files I plan to write in this folder, in order:

1. `01-steering-edges.md` — full edge taxonomy with semantics + traversal rules
2. `02-runtime-shape.md` — concrete data structures for Frame / Actor / Set / Edge
3. `03-execution-loop.md` — the main loop; how a sentence becomes traversal
4. `04-scheduler.md` — sync vs async vs fire-and-forget; cursor model
5. `05-contracts-runtime.md` — how the runtime enforces obligations & capabilities
6. `06-listeners-and-events.md` — say/hears/reads + waitable + branch listeners
7. `07-locks.md` — exclusive access; what blocks, what waits
8. `08-debug-and-what.md` — `what` introspection + tracing surface
9. `09-mvp-slice.md` — narrowest possible runnable subset
10. `99-open-questions.md` — running list of engine-specific gaps

Will not all be polished. Some will be sketches. Saving early and often.

---

## Implementation language — open

Candidates the user mentioned earlier: Go or Rust.

Trade-offs at a glance:

- **Go** — ergonomic for graph + scheduler work, channels match the actor mailbox model, GC removes the lifetime puzzles around frame parents/children, built-in concurrency primitives. Less ceremony for a v1.
- **Rust** — ownership model maps well to lock + capability semantics, zero-cost abstractions, much harder to prototype quickly, lifetime annotations get gnarly when frames hold references to parent state.

Lean: **Go for v1**, port to Rust later if perf or embedding demands it. The runtime is a graph walker with a scheduler — Go does that with minimal friction. Rust's strengths come into play once we know the static model is right.

Will revisit after sketching the data structures.
