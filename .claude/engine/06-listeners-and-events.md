# Engine — Listeners and Events

Three flavors that look similar in the source but behave differently:

1. **Branch listeners** — single-shot, installed at branch-group entry, removed atomically when one fires.
2. **Wait listeners** — single-shot, install on `wait until`; wake the parked cursor.
3. **Actor listeners** — long-lived, installed by `when X hears Y?`; fire repeatedly.
4. **Feed listeners** — like actor listeners but with stream semantics (replay deferred to v2).

---

## Branch listeners

Lifecycle:

```
BRANCH_OPEN edge:
    create BranchGroup with arms=[]
BRANCH_ALT edge:
    append arm to current group
BRANCH_FALLBACK edge:
    set group.Fallback
BRANCH_CLOSE (synthetic, on dedent or end-of-block):
    if all subjects already settled:
        evaluate predicates inline; run matching arm (or fallback)
    else:
        install one Listener per arm + fallback on subject(s)
        park if needed (depends on whether parent has more edges)
```

When any arm's predicate matches:

```
1. atomically remove all arms' listeners
2. allocate child frame for the matching arm body
3. transfer cursor execution into the arm body
4. on arm body resolve, branch group is done; parent cursor advances past the group
```

"Atomically remove" matters — without it, two arms could fire on the same status transition. In single-threaded v1 this is just a flag check during dispatch; in multi-threaded later it'd need a CAS.

Special case — **no async subject in branch**: if all predicates evaluate against already-settled frames (e.g., the immediately preceding `do X...` resolved before the branch), no listener install is needed. Branch is purely sequential.

---

## Wait listeners

`wait until <pred>...` is parking. The listener body is "advance the parked cursor".

```
WAIT_UNTIL edge:
    if pred currently true: skip; advance PC
    else:
        install Listener:
            Subject  = pred.Subject
            Predicate = pred
            Body     = AdvanceCursor(currentCursor, currentPC+1)
            Once     = true
        park current cursor
```

When the listener fires, body runs. `AdvanceCursor` is a synthetic body — not user code, just a scheduler instruction. The cursor's PC moves to the next edge after the wait, and the cursor is moved Ready → run.

Time-based predicates (`wait until it's been 5 seconds?`) park on a timer instead of a status listener. Same shape from the cursor's POV.

---

## Actor listeners (`hears`)

Long-lived. Installed at script-load or actor-construction time. Persist for the lifetime of the actor.

```go
type ActorListenerKey struct {
    Target  NodeID    // the actor or set being listened on
    Message MessageName
}

type ActorListenerRegistry struct {
    table map[ActorListenerKey][]*Listener
}
```

On `say X to Y!`:

```
1. Y is the target — could be an actor (single delivery target) or a set (fan-out)
2. if Y is a set:
     for each member M in Y: dispatch to M
3. if Y is an actor:
     for each L in registry[(Y, X)]:
         allocate fresh frame with body = L.Body
         allocate fresh cursor; push to Ready
4. close owning phrase (the trailing `!`)
```

Each listener runs in its own frame on its own cursor. Order of dispatch across listeners is undefined (per spec) unless explicitly sequenced.

The listener body's scope captures the actor's `this` — so inside `when this hears X?`, `this` is the receiving actor.

---

## Feed listeners (`reads`)

Same shape as actor listeners but indexed differently and with stream semantics:

```go
type FeedListenerKey struct {
    Feed    NodeID
    Message MessageName
}
```

`write <Item> to <Feed>.`:

```
1. push Item onto Feed's buffer
2. for each listener registered on (Feed, Message):
     allocate frame, allocate cursor, push to Ready
```

Differences from actor `hears`:

- A feed has a buffer; if listeners can't keep up, items queue up. (Backpressure is open — see `04-scheduler.md`.)
- Feeds may support replay later. v1: live-only, no replay.
- Feed write does not auto-close the writer's phrase (no trailing `!`); writers continue executing.

---

## Queue mechanics (`put`, `has next`)

Queues are pull, not push:

- `put X into Q.` — append to FIFO buffer; wake any cursor parked on `Q has next?`.
- `when Q has next? do X with value.` — installs a listener that fires when the queue is non-empty; in the body, `value` binds to the dequeued item.

Implementation: a queue is a Set with `Source = Queue`, plus a wait-list of cursors blocked on `has next?`. On `put`, dequeue the head, hand it to the next waiter as `value`, wake.

**Important**: on `has next?` matching, the item is removed from the queue and bound. Each item goes to exactly one consumer (work-stealing semantics, not broadcast). If multiple `when Q has next?` listeners exist, the first to install gets the next put — or some other deterministic rule (FIFO of waiters seems right).

---

## Single-shot vs long-lived

| Listener kind | Lifetime | Removal trigger |
|---|---|---|
| Branch arm | Until any arm fires or branch closes | First fire OR group closes |
| Branch fallback | Until any arm fires or branch closes | First fire OR group closes |
| `wait until` | Until predicate fires | First fire |
| Actor `hears` | Until owning actor terminates | Owner terminates |
| Feed `reads` | Until owning actor terminates | Owner terminates |
| Queue `has next` | Until cursor unblocks | First match |

The runtime needs a "remove listener" path for each. Centralize: `Listener.Cancel()` should always work, and frame/actor cleanup walks installed listeners and cancels them.

---

## Listener install at process-mode entry vs declaration

Long-lived `when this hears X?` declarations appear inside actor bodies. They're processed at *actor construction* time (when the actor is brought into scope), not at every invocation. So the actor body has a phase distinction:

- **Construction phase**: install all `hears` / `reads` listeners declared inline.
- **Runtime phase**: subsequent `do this's <X>...` invocations follow normal phrase semantics.

Need a way to mark which top-level edges in an actor body are "construction" vs "runtime". Probably: any `when` at actor body's top level (not nested) is construction. Any `do`/`start`/`execute` at top level is also construction (runs once on actor init).

This is similar to scripts — `the App is a script. do MacroMarco's Start...` runs the body once at script load. The same model applies to actor init.

---

## Open

- **Listener fairness** — when 100 listeners hear the same message, all 100 frames are scheduled. Do they run interleaved or in install-order? Default: install-order, run-to-quiescence per cursor. Reconsider when we have data.
- **Listener removal during fire** — if a listener body removes another listener, what happens? Trivially safe in single-threaded; document the semantics.
- **Cross-actor `hears`** — `when Game hears X?` from inside another actor — does the listener live on the other actor's body or the foreign actor's registry? Foreign — registered at the target's address.
- **Group routing via sets** — sets used as routing targets. Should the set itself be live (mutations affect dispatch) or snapshotted at `say` time? Live, probably. Need to decide.

---

## Next file

`07-locks.md` — `lock <S>...`, ownership, FIFO waiters, what happens on phrase close.
