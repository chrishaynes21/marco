# Engine — Steering Edges

The central abstraction. Sentence forms emit edges. The runtime walks them.

---

## Edge model

An edge is a typed transition:

```
Edge {
    kind:       EdgeKind        // see taxonomy below
    source:     NodeRef         // what we're walking from (current frame, named frame, set)
    target:     NodeRef         // what we're walking to (action node, set node, listener slot)
    payload:    SetRef?         // input data flowing along the edge
    binding:    Binding?        // optional naming (`as X`, `the X is ...`)
    contract:   ContractRef?    // expected exit shape on the other side
    listener:   ListenerSpec?   // for question/listener edges
    span:       Span            // source location for diagnostics & trace
}
```

Edges are emitted by the phrase compiler. The runtime consumes them in order — each edge either runs to completion (sync) or installs an artifact (listener, lock, async cursor) and moves on.

---

## Edge kinds

Grouped by what they do.

### Invocation edges

| Kind | Surface | Behavior |
|---|---|---|
| `INVOKE_SYNC` | `do X.` / `do X with S.` | Allocate child frame, run, await completion, copy frame to parent's `that`. Status: completed before next edge. |
| `INVOKE_PHRASE` | `do X...` ... `!` | Like INVOKE_SYNC, but body opens; child frame stays referenced for nested edges. |
| `INVOKE_ASYNC` | `start X as N...` | Allocate child frame on a parallel cursor. Parent doesn't wait. Bind to `N` for observation. |
| `INVOKE_FORGET` | `execute X.` | Allocate, schedule, drop handle. No observation possible. |
| `CAPABILITY_INVOKE` | `do that <cap>.` / `do its <cap>.` | Look up capability on target frame's contract; reject if not declared; run it. |

### Status / lifecycle edges

| Kind | Surface | Behavior |
|---|---|---|
| `STATE_UPDATE` | `it is <S>.` | Mutate current frame's status. No frame transition. |
| `RESOLVE` | `this is <S>!` / `this is <S> with <D>!` | Mutate status, populate result, close owning phrase, traverse return edge to parent. |
| `STATE_FAIL_SHORT` | `this is failed with error "..."!` | RESOLVE + populate error slot from string shorthand (see Lifecycles#Errors). |

### Branch edges

| Kind | Surface | Behavior |
|---|---|---|
| `BRANCH_OPEN` | `when <pred>?` | Begin branch group, install predicate. |
| `BRANCH_ALT` | `or when <pred>?` | Add alternative to active group. |
| `BRANCH_FALLBACK` | `or?` | Capture remaining unmatched cases. Final in group. |
| `BRANCH_CLOSE` | (implicit on dedent or new top-level) | Group closes; if obligations remain, compile error. |

Predicates inside `when` come in two shapes:

- **Status predicate**: `<frame> <status>?` (or shorthand `<status>?` implying `that`).
- **Data predicate**: `<path> is <value>?`, `<path> exists?`, `<container> has <field>?`, `<frame> hears <message>?`.

### Data flow edges

| Kind | Surface | Behavior |
|---|---|---|
| `INPUT_BIND` | `with <S>` (in invocation) | Pass `S` as the target frame's `input`. |
| `RESULT_READ` | `that's <field>` / `<frame>'s <field>` | Read the field from the named frame's result set. |
| `SLOT_READ` | `that's error` / `its error` | Read the frame slot directly (special case). |
| `BIND_LOCAL` | `the X is <expr>.` | Bind name to value; subsequent reads resolve `X` locally. |
| `BIND_NAME` | `... as N.` | Like BIND_LOCAL but does not change `this`. |

### Communication edges

| Kind | Surface | Behavior |
|---|---|---|
| `EMIT` | `say <Msg> to <Target>!` | Create message frame, deliver to target's listener registry. The trailing `!` also closes the owning phrase. |
| `LISTEN` | `when <Target> hears <Msg>?` | Install passive listener at `Target`. Each fire creates a fresh listener-frame. |
| `FEED_WRITE` | `write <Item> to <Feed>.` | Push to feed; each feed listener runs in its own frame. |
| `FEED_READ` | `when <Feed> reads <Msg>?` | Install feed listener. Same shape as `hears` but with stream semantics. |
| `QUEUE_PUT` | `put <Item> into <Queue>.` | Enqueue. Wakes a consumer if one is waiting on `has next?`. |

### Iteration edges

| Kind | Surface | Behavior |
|---|---|---|
| `FOR_EACH` | `for each <V> in <C>...` | Open phrase per entry; set `key`/`value`/`previous`/`next`/`first`/`last` in scope. |
| `WHILE` | `while <pred>...` | Re-evaluate predicate, run body, repeat until false. |
| `WAIT_UNTIL` | `wait until <pred>...` | Park current frame in `waiting` until predicate fires. |
| `LOOP_SKIP` | `skip.` | Re-traverse to next iteration head. |
| `LOOP_STOP` | `stop.` | Resolve loop frame and exit. |

### Resource edges

| Kind | Surface | Behavior |
|---|---|---|
| `LOCK` | `lock <S>...` | Acquire exclusive write rights on set node. Body runs holding the lock; release on phrase close. |

### Declaration edges (declaration mode)

These don't run at "process time" — they execute at graph-build time, populating the graph.

| Kind | Surface | Behavior |
|---|---|---|
| `DECL_SUBJECT` | `the <N> is a <T>.` | Add or look up node; set `it`/`this` to it. |
| `DECL_FIELD` | `it's <F> is a <T>.` / `this's <F> is a <T>.` | Add field declaration to current subject node. |
| `DECL_CAPABILITY` | `it can <C>.` / `this can <C>.` | Add capability declaration (gates dispatch in CAPABILITY_INVOKE). |
| `DECL_STATUS` | `this has status <S>.` | Add allowed status to a contract node. |
| `DECL_DEFAULT` | `<X>'s <F> is <V> by default.` | Bind default value, evaluated lazily on first read. |
| `DECL_CONTRACT_ATTACH` | `that gives a <Contract>.` (inside action) | Attach contract reference to action node. |
| `DECL_CONTRACT_BUILTIN` | `this is failable.` etc. | Apply built-in contract to current node. |
| `IMPORT` | `use <Module>.` | Lazy import — exposes module's namespace for resolution. |

### Debug edges

`what` queries are not real edges in the execution graph — they're a side-channel into the trace tree. See `08-debug-and-what.md`.

---

## How sentences map to edge sequences

Two examples to make this concrete.

### Example 1 — simple invocation with branching

```marco
do this's WriteToFile with this's State...
    when ok?
        this is Saved with that!
    or?
        this is failed with that's error!
```

Edge stream:

```
INVOKE_PHRASE  target=this.WriteToFile  payload=this.State
  BRANCH_OPEN  predicate=STATUS(that, ok)
    RESOLVE    status=Saved  data=that
  BRANCH_FALLBACK
    SLOT_READ  source=that  slot=error  -> tmp
    RESOLVE    status=failed  data=tmp
  BRANCH_CLOSE
```

After `INVOKE_PHRASE`, the cursor is inside the WriteToFile frame's body. The two RESOLVE edges target the *owning* phrase — which is the parent `Save` frame, not `WriteToFile`. The branch group resolves the parent based on the child's outcome.

### Example 2 — async + listener

```marco
when this hears SaveRequested?
    start this's Save as SaveFlow...

    when SaveFlow Saved?
        say SaveCompleted to this!
```

Edge stream:

```
LISTEN        target=this  message=SaveRequested  -> body:
  INVOKE_ASYNC  target=this.Save  bind=SaveFlow
  LISTEN        target=SaveFlow  predicate=STATUS(SaveFlow, Saved)  -> body:
    EMIT        message=SaveCompleted  target=this
```

Listeners install passive edges. Firing them creates fresh frames for the body. The `EMIT` carries `!` so its frame is closed on emission.

---

## Edge ordering and concurrency

Within a single phrase body, edges are emitted in source order and executed in source order — that's the locked rule from Execution Model. Concurrency only enters via:

- `INVOKE_ASYNC` — child runs on its own cursor; parent moves to next edge immediately
- `EMIT` to multiple listeners — each listener runs in its own frame (order undefined unless explicitly sequenced)
- `FEED_WRITE` to multiple feed listeners — same as EMIT
- branches in a `BRANCH_OPEN`/`OR_WHEN`/`OR_FALLBACK` group — exactly one body runs

Everything else is sequential per cursor.

---

## What this gives the runtime

The runtime has to understand, at minimum, this list of edge kinds — plus how to:

1. Allocate a new frame and wire it into the trace tree.
2. Run an action node's body (which produces more edges).
3. Pause a frame on `wait`, listener install, or `lock`.
4. Resume a frame when the wake condition fires.
5. Close a frame on resolve and propagate it as `that` to the parent.
6. Reject contract violations early (statically where possible, at dispatch time otherwise).

Everything else — error semantics, presence proofs, status predicates — is built on top of these edges. They don't add new runtime primitives; they're patterns over the existing ones.

---

## Next file

`02-runtime-shape.md` — concrete data structures for Frame, Actor, Set, plus how edges are represented and stored.
