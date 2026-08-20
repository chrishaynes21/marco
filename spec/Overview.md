---
status: reference
---

---
# Overview {#overview}
---

Marco is a graph execution language with a sentence syntax.

Programs run as graph traversal: every line is a sentence, every sentence creates a [[Frames|Frame]], and Frames move execution through the graph. Terminating punctuation determines the sentence's effect. See [[Graph]].

---
## Sentence Punctuation {#sentence-punctuation}
---

| Symbol | Meaning |
|--------|---------|
| `.`    | State update |
| `!`    | Return from the owning phrase |
| `?`    | Question / listener |

---
## Core Pronouns {#core-pronouns}
---

- `it` — the current [[Frames|frame]]
- `it's` — the current frame status
- `its` — the current frame set
- `input` — the current frame input set
- `that` — the most recent completed child or observed frame (see [[Core Concepts]], [[Frames#Frame vs Result]])
- `that's` — access to data on `that`'s result set
- `this` — the current focused [[Frames|frame]] — the frame being resolved in process mode (see [[Frames]])

---
## Graph {#graph}
---

Marco's substrate is a graph. Nodes include actors, actions, scenes, acts, sets, frames, contracts, templates, traits, statuses, errors, feeds, channels, and queues. Edges are created by sentence structure — every meaningful sentence creates a node, creates an edge, moves data across an edge, observes an edge, or resolves a Frame through an edge. Frames are the runtime traversal of that graph. See [[Graph]].

---
## Reference Modes {#reference-modes}
---

Marco has two reference modes. **Declaration mode** anchors `it` to the current declared subject and shapes the graph. **Process mode** anchors `this` to the Frame being resolved and `that` to a child or observed Frame, driving execution. See [[Reference Modes]].

---
## Phrases {#phrases}
---

A phrase is opened with `...` and runs with its own frame context (`it`) and result context (`that`). A phrase must resolve before control returns. See [[Phrases]].

---
## Frames {#frames}
---

Every line creates a [[Frames|Frame]]. A Frame is a built-in [[Actors|actor]] that represents execution: it carries `status`, `input`, `result`, and `error`, and may expose capabilities like `rollback` or `retry`. Frames may be named with `as` (without focus change) or `the` (with focus change). See [[Frames]].

---
## Data Model {#data-model}
---

All structured data is a set. Primitives (`text`, `number`, `boolean`, `time`, `id`) and containers (`set`, `list`, `map`) define shape and access. See [[Data Model]].

---
## Declarations {#declarations}
---

Declarations define named concepts and members using canonical `the ... is a ...` and `this's ... is a ...` forms. See [[Declarations]].

---
## Actions {#actions}
---

Actions and functions are defined with `does...` as the universal behavior opener. See [[Actions]].

---
## Contracts {#contracts}
---

Contracts define required interfaces. Templates allow optional interfaces. Traits compose capability into data types. Actions adopt contracts with `that is <Contract>.`; actors with `it is a <Contract>.`; templates are adopted with `it uses <Template>.`. See [[Contracts]].

---
## Locking {#locking}
---

Locking provides exclusive mutation access to lockable sets using canonical `lock ...` phrases. See [[Locking]].

---
## Inference {#inference}
---

Marco infers names, input, and result mappings only when exactly one valid interpretation exists; otherwise explicit syntax is required. See [[Inference]].

---
## Translators {#translators}
---

Translators bridge types via declared inputs and outputs. Partial types are bridges-in-progress that promote into their target. Auto-mapping handles same-named fields automatically. See [[Translators]].

---
## Scoping {#scoping}
---

Names are resolved by context first — local, `this`, named Frames, accessible sets, outer scopes, then global. Ambiguity requires explicit qualification. See [[Scoping]].

---
## Execution {#execution}
---

A phrase may be invoked as blocking, async, or fire-and-forget. Execution is left-to-right; concurrency is Frame-based. See [[Execution Model]].

---
## Iteration {#iteration}
---

`for each` and `while` open loop phrases; `wait until ...` opens an observing Frame; queues store work for pull-style consumption and feeds stream events for push-style consumption. See [[Iteration]].

---
## Modules {#modules}
---

Programs are organized into scripts and modules containing actors, acts, scenes, sets, and contracts. See [[Modules]].

---
## Actors {#actors}
---

Actors are miniature state machines that own state, hear and say messages, and coordinate execution. Frames themselves are built-in actors. See [[Actors]].

---
## Testing {#testing}
---

Tests describe Frame inputs, expected statuses, and expected result shapes. Mocks replace graph nodes directly. See [[Testing]].

---
## Observability {#observability}
---

`log` observes execution; tracing exposes the Frame graph; observation patterns combine status listeners with logging. `show` is for UI rendering, not debugging. `what` is the conversational debug-introspection keyword. See [[Observability]].

---
## Compiler {#compiler}
---

Marco is strict at compile time and generous in guidance. Diagnostics include suggested fixes; autocomplete is graph-driven. See [[Compiler]].

---
## Canonical vs Shorthand {#canonical-vs-shorthand}
---

Marco has a canonical form and a shorthand form. Both are valid; the canonical form is the source of truth.

### Example

Canonical:
```marco
this is ok with that!
```

Optional shorthand:
```marco
this ok with that!
```

---
## Language Principles {#language-principles}
---

- **Frames act, sets hold.** Frames represent execution and lifecycle; sets represent data. See [[Frames#Frame vs Result]].
- **Proof over null.** Marco has no null. Absence is proven by lack of presence; access requires proof. See [[Data Model#No Null]] and [[Data Model#Presence and Safe Access]].
- **Actors participate.** Actors are conversational entities, not data producers/consumers.
- **Channels coordinate, feeds describe.** Channels are conversational and ephemeral; feeds are streams. See [[Actors#Messaging Syntax — Channels]] and [[Iteration#Feeds]].
- **Syntax reads like English.** Returns keep `is`; canonical forms favor readability over brevity.
- **Correctness is enforced at compile time.** Illegal Marco does not compile. See [[Compiler]].

---
## Final Lock {#final-lock}
---

**Marco is a strict, readable language where execution is explicit, data is proven, and behavior flows through a graph of frames and actors.**

---
## Reading Order {#reading-order}
---

1. [[Core Concepts]]
2. [[Graph]]
3. [[Reference Modes]]
4. [[Lifecycles]]
5. [[Frames]]
6. [[Data Model]]
7. [[Declarations]]
8. [[Actions]]
9. [[Contracts]]
10. [[Locking]]
11. [[Inference]]
12. [[Scoping]]
13. [[Phrases]]
14. [[Execution Model]]
15. [[Iteration]]
16. [[Modules]]
17. [[Actors]]
18. [[Translators]]
19. [[Testing]]
20. [[Observability]]
21. [[Compiler]]
