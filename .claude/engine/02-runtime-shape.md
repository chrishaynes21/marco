# Engine — Runtime Shape

Concrete data structures. Sketches in Go-flavored pseudocode. Names will likely shift; the shape is what matters.

---

## The big four

There are four runtime entity kinds. Everything else is data carried by these.

1. **Node** — static graph entry: actor, action, set, contract, error, scene, act, script, message, queue, feed.
2. **Frame** — running execution unit. The built-in actor.
3. **Set** — structured value (universal data container).
4. **Edge** — typed transition (transient at runtime, recorded for trace).

---

## Node

```go
type NodeKind uint8

const (
    NodeActor NodeKind = iota
    NodeAction
    NodeSet         // "the X is a set" — type, not instance
    NodeContract
    NodeError
    NodeScene
    NodeAct
    NodeScript
    NodeMessage     // event symbol (SaveRequested etc.)
    NodeQueue
    NodeFeed
    NodeStatus      // declared status name
    NodeCapability  // declared capability
)

type Node struct {
    ID         NodeID
    Kind       NodeKind
    Name       string         // e.g., "Game", "Save"
    Module     ModuleID
    Parent     NodeID         // owner (e.g., this's Save's parent is Game)

    Fields     []FieldDecl    // declared members (this's X is a Y.)
    Statuses   []StatusName   // for contracts
    Capabilities []CapName    // declared capabilities (this can X.)
    Contract   NodeID         // contract attached to this node (action -> Saveable)
    Body       *Block         // for actions: compiled phrase body
    Defaults   map[string]Value

    Span       Span           // source loc, declaration site
}
```

Nodes are immutable once the graph is built. Mutation of *state* happens on Frames and Sets, not Nodes.

A `Block` is the compiled body of an action — a sequence of edges and nested blocks. (Cf. `01-steering-edges.md`.)

---

## Frame

```go
type FrameStatus uint8

const (
    StatusPending FrameStatus = iota
    StatusActive
    StatusWaiting    // parked on listener / wait until / lock
    StatusCompleted
    // Custom statuses (ok, failed, canceled, Saved, Saving, ...) layer on top —
    // see "Custom statuses" below.
)

type Frame struct {
    ID        FrameID
    Parent    FrameID                 // tree edge upward
    Children  []FrameID
    Origin    NodeID                  // which Node spawned this frame (action node)

    Input     *Set                    // received input
    Result    *Set                    // produced result (populated on RESOLVE)
    Error     *ErrorValue             // populated on RESOLVE-failed

    Status    FrameStatus             // built-in lifecycle state
    UserStatus StatusName             // ok / failed / canceled / Saved / etc.

    Listeners []*Listener             // installed observers on this frame
    Locks     []LockHandle

    StartTime time.Time
    EndTime   time.Time

    PC        ProgramCounter          // cursor inside Origin.Body when Active
                                      // not used for Completed frames

    Cursor    CursorID                // which scheduler cursor owns this frame

    // Tracing
    TraceID   TraceID
    Spans     []Span                  // edges executed (for `what happened?`)
}
```

Custom statuses (`ok`, `failed`, `Saved`, ...) are *not* an enum extension. They're string-named values on `UserStatus`. The built-in `StatusCompleted` says "the frame finished"; the `UserStatus` says *how* it finished.

Why split? Because the runtime cares about pending/active/waiting/completed for scheduling, but contract validation and `when X failed?` predicates work on user status names. Mixing them costs us — separating them makes the scheduler agnostic to user vocabulary.

---

## Set

```go
type Set struct {
    Type   NodeID            // type node (e.g., the User node)
    Items  map[string]Value  // for sets and maps
    Order  []string          // for ordered iteration
    List   []Value           // for list type — separate field, mutually exclusive with Items
    Source SetKind           // SetKind: Plain | List | Map | Queue | Feed

    Frozen bool              // post-return immutability flag
    Owner  FrameID           // current mutator (for lock checks)
}

type Value interface {
    isValue()
}

type Primitive struct {
    Kind  PrimKind  // Text | Number | Boolean | Time | Id
    Bytes []byte    // representation
}

type SetVal   struct { *Set }       // nested set
type FrameRef struct { ID FrameID } // values may reference frames (e.g., `that`)
type NodeRef  struct { ID NodeID }  // for declaration sites
```

A queue is a Set with `Source = Queue` and FIFO semantics on `List`. A feed is a Set with `Source = Feed` plus a listener registry. They share enough surface to live in one type with a discriminator.

---

## Edge

Already sketched in `01-steering-edges.md`. Concrete form:

```go
type Edge struct {
    Kind     EdgeKind
    Span     Span

    // Optional fields — populated based on Kind.
    Target   *Ref            // node or frame the edge points at
    Payload  *PayloadSpec    // input set construction
    Bind     *BindSpec       // `as X` or `the X is ...`
    Body     *Block          // for BRANCH_OPEN / FOR_EACH / WAIT_UNTIL etc.
    Predicate *Predicate     // for branch / wait / when
    Group    *BranchGroup    // for BRANCH_ALT / BRANCH_FALLBACK
}
```

Edges are produced by the phrase compiler and stored in `Block`. At runtime the cursor walks them sequentially.

---

## Predicates

Predicates are compiled, not strings. The compiler resolves what kind of predicate you have:

```go
type Predicate struct {
    Kind PredKind
    // Kind: PredStatus | PredHas | PredExists | PredDataEq | PredHears
    Subject Ref
    Status  StatusName    // for PredStatus
    Field   string        // for PredHas / PredExists / PredHears
    Value   Value         // for PredDataEq
}
```

Branch groups install all predicates up-front; firing the matching one runs its body, others are skipped.

---

## Listeners

```go
type Listener struct {
    ID         ListenerID
    Owner      FrameID         // frame that installed it (closes when owner closes)
    Kind       ListenerKind    // ListenStatus | ListenHears | ListenFeed | ListenWaitUntil
    Subject    Ref             // who we're listening on
    Predicate  *Predicate      // optional gate
    Body       *Block          // what to run when fired
    Once       bool            // typically true for branches; false for actor `hears`
    InScope    *Scope          // captured lexical scope (named frames, locals)
}
```

A `when X hears Y?` listener is installed for the lifetime of its owning actor. A `when X ok?` branch listener is installed at branch-group entry and removed when the group closes.

Firing a listener allocates a fresh `Frame` whose `Origin` is a synthetic node (the listener body block). The listener's captured scope becomes the new frame's scope.

---

## Locks

```go
type Lock struct {
    SetID    SetID
    Holder   FrameID
    Waiters  []FrameID    // FIFO; woken in order on release
}

type LockHandle struct {
    LockID LockID
    Held   bool
}
```

A `lock <S>...` edge:

1. If `S` is unlocked: mark the holder, install handle on current frame, run body, release on phrase close.
2. If `S` is locked by another: park current frame in `Waiting`, queue on `Lock.Waiters`.

Release on phrase close (whether by RESOLVE or by falling out of scope after compile-time check). Re-entry by the same holder is allowed (same frame can re-lock without blocking — open question whether transitive holds count).

---

## Scopes

Name resolution at runtime needs a scope chain that mirrors what was visible at compile time.

```go
type Scope struct {
    Parent  *Scope
    Bindings map[string]Binding
    Frame   FrameID  // the frame this scope belongs to
}

type Binding struct {
    Kind  BindKind  // BindLocal | BindNamedFrame | BindNode | BindImport
    Ref   Ref
}
```

Resolution order (per Scoping spec):

1. Local bindings (this scope)
2. `this`
3. Named frames (`as` / `the` bindings)
4. Accessible sets (`input`, `that`, `its`)
5. Outer scopes (via `Parent`)
6. Imported nodes (lazy — only matched on reference)
7. Global scope

Scope is built statically by the compiler; runtime only updates `that` and `it` references as it walks.

---

## Module / import lazy resolution

```go
type Module struct {
    ID      ModuleID
    Name    string
    Nodes   map[string]NodeID    // exported symbols
    Imports []ModuleID           // `use X.` declarations
}
```

When a name is referenced inside a module, resolution searches:

- the local module's nodes
- each `Imports[i].Nodes`
- on collision, error with both candidates (per the lazy/selective rule)

The `use` doesn't pre-populate any local table. Resolution walks the import list at lookup time. Caching is fine but not eager.

---

## Trace storage

The trace tree is just the frame tree. Trace data is read off `Frame.Spans` and the parent/child links. No separate event log needed for v1 — the frame tree IS the log.

Optionally: a sidecar ring buffer for high-volume traces if memory becomes an issue. Out of scope for v1.

---

## Memory ownership rough cut

- Nodes: long-lived, owned by the module / graph root. Don't get GC'd while their module is loaded.
- Frames: owned by their parent's Children list. Completed frames stay around until the parent completes (so `that` is still readable). Once the root completes, the entire tree can be reaped.
- Sets: usually owned by frames. Shared sets (mutated under `lock`) live longer; need a refcount or owner reassignment rule. Open: see `99-open-questions.md`.
- Listeners: owned by the frame that installed them. Removed automatically when owner closes.

---

## Sizing

Quick sanity check on a Frame in Go:

- ~10 pointers + a few ints + 2 timestamps ≈ 100 bytes
- Plus Input/Result Sets: variable, but small for typical actions
- Plus Spans: maybe 40 bytes per edge, capped at ~64 edges per frame in practice

Call it ~1 KB per frame in the typical case. Spawning a million frames is ~1 GB. Fine for desktop/server, tight for embedded. Don't optimize until we measure.

---

## Next file

`03-execution-loop.md` — what does the runtime *do* tick by tick.
