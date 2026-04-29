---
# Graph {#graph}
---

Marco is a graph execution language.

Marco programs execute over a graph. Sentences are the surface; the graph is the substrate.

---
## Nodes And Edges {#nodes-and-edges}
---

Marco lines create or connect graph nodes. Edges are created by sentence structure.

Nodes in the graph represent:

- [[Actors|actors]]
- [[Actions|actions]]
- [[Modules#Scenes|scenes]]
- [[Modules#Acts|acts]]
- [[Data Model#Sets|sets]]
- [[Frames|frames]]
- [[Contracts|contracts]]
- [[Contracts#Templates|templates]]
- [[Contracts#Traits|traits]]
- [[Lifecycles#Canonical Status Set|statuses]]
- [[Lifecycles#Errors|errors]]
- [[Iteration#Feeds|feeds]]
- [[Actors#Messaging Syntax — Channels|channels]]
- [[Iteration#Queues|queues]]

Edges represent:

- execution flow
- data flow
- event propagation
- observation and branching
- resolution
- synchronization
- ownership and context

---
## Core Model {#core-model}
---

Every line creates a [[Frames|Frame]] that advances execution through the graph.

Execution is graph traversal.

### Example

Canonical:
```marco
do Save with input.
```

Meaning:

- resolve the `Save` node
- pass the `input` set
- create a Frame
- move execution to the next node

---
## Chaining {#chaining}
---

Chaining is graph traversal within a single line.

### Example

Canonical:
```marco
do X, then do Y, then do Z with that!
```

Meaning:

- traverse `X` → `Y` → `Z`
- each step produces a Frame
- `that` always refers to the previous Frame's result

Rule:

- chaining composes graph edges sequentially

See [[Execution Model#Execution Order]].

---
## Messaging {#messaging}
---

Actors communicate through graph edges.

### Example

Canonical:
```marco
says SaveRequested to SaveSystem!
```

Meaning:

- speaker emits event `SaveRequested` on a channel to `SaveSystem`
- the channel routes the message to `SaveSystem`'s registered listeners
- each listener creates its own Frame

Listener:

Canonical:
```marco
when SaveSystem hears SaveRequested? do Save.
```

Rules:

- events are edges that connect producers to listeners
- multiple listeners may exist
- execution order across listeners is not guaranteed
- each listener creates its own Frame

---
## Event Graph {#event-graph}
---

Events form a dynamic subgraph.

Nodes:

- event emitters
- event listeners

Edges:

- event propagation

Events may:

- fan out to multiple listeners
- be observed asynchronously
- create independent Frames

---
## Data Flow {#data-flow}
---

Sets flow through the graph.

### Example

Canonical:
```marco
do Validate with FormData, then do Save with that!
```

Meaning:

- `FormData` flows into `Validate`
- the result flows into `Save`

Rule:

- `with` defines incoming edges
- `that` defines outgoing edges

---
## Frame Graph {#frame-graph}
---

Frames form a runtime graph:

- parent → child (phrase nesting)
- sibling (chaining)
- observer (`when`)
- async (`start`)

Frames are the runtime representation of graph traversal. A Frame is itself a built-in actor — Frame nodes in the graph are actor nodes specialized for execution lifecycle. See [[Frames#Frame As Actor]] and [[Phrases]].

---
## Ownership {#ownership}
---

`this` defines the current graph node.

`this's X` defines edges owned by the node.

### Example

Canonical:
```marco
the Game is an actor.
this's Save does...
```

Meaning: the `Game` node owns the `Save` action node.

See [[Actions]] and [[Actors]].

---
## Contracts In Graph Terms {#contracts-in-graph-terms}
---

Contracts define the allowed exit edges of a node.

### Example

`Saveable` defines:

- `Saved` edge
- `failed` edge
- `canceled` edge

Rule:

- contracts constrain graph shape

See [[Contracts]] and [[Contracts#Unresolved Obligations]] for how unresolved exit edges propagate as obligations.

---
## Sentences As Graph Operations {#sentences-as-graph-operations}
---

Edges are created by sentence structure. Each canonical form maps to a graph operation:

| Sentence | Graph Effect |
|----------|--------------|
| `the Game is an actor.` | Creates a `Game` node typed actor. |
| `it can Save.` | Creates a capability edge from `Game` to `Save`. |
| `that does...` | Creates an implementation edge from the `Save` action to its phrase / Frame. |
| `do Save with File.` | Creates an execution edge from the current Frame to `Save`, and a data edge from `File` into `Save`'s input. |
| `when that is failed?` | Creates an observation / branch edge from the current Frame to the watched Frame's `failed` status. |
| `or when that is canceled?` | Creates an alternate branch edge in the same branch group. |
| `this is failed with that's error!` | Creates a resolution edge from the current Frame to `failed`, carrying the watched Frame's error. |
| `says SaveRequested to Game.` | Creates a message edge from the current Frame / actor to `Game`. |
| `when Game hears SaveRequested?` | Creates a listener edge from `Game` to `SaveRequested`. |
| `write Event to Feed.` | Creates a stream-write edge. |
| `when Feed reads Event?` | Creates a stream-read edge. |
| `put Work into Queue.` | Creates an enqueue edge. |
| `when Queue has next?` | Creates a dequeue / listener edge. |
| `lock Settings...` | Creates a synchronization edge from the current Frame to `Settings`. |
| `use Ahk.` | Creates an import edge into the graph. |

See [[Actions]], [[Phrases]], [[Lifecycles#Frame Status Predicates]], [[Actors#Messaging Syntax — Channels]], [[Iteration#Feeds]], [[Iteration#Queues]], [[Locking]], and [[Modules#Imports]].

---
## Sentence Effect {#sentence-effect}
---

Every meaningful Marco sentence does one of the following:

- creates a node
- creates an edge
- moves data across an edge
- observes an edge
- resolves a Frame through an edge

This is the **core rule** of the graph model: there are no sentences that only "compute" — every sentence is a graph operation. Even pure assignment (`User's Name is "Chris".`) creates or updates a data edge on a set node.

---
## One-Line Lock {#one-line-lock}
---

**Marco is a sentence-driven graph language: syntax defines nodes and edges, frames are runtime traversals of that graph.**

---
## Open Questions {#open-questions}
---

- Formal grammar for `says ... to ...!` event emission.
- Whether listener ordering may be made deterministic by declaration.
- How module boundaries appear in the graph (subgraphs, namespaces, or both).
- Visualization conventions for the graph (debug view, runtime introspection).
