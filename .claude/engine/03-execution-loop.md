# Engine — Execution Loop

How the runtime actually walks the graph. The cursor model.

---

## Cursor

A **cursor** is a unit of sequential execution. It owns a current frame and a program counter inside that frame's body. Walking an edge means: read the edge at PC, execute it, advance PC.

```go
type Cursor struct {
    ID       CursorID
    Frame    FrameID       // currently executing frame
    PC       int           // index into Frame.Origin.Body.Edges
    Stack    []ResumePoint // saved PCs when we descend into nested phrases
    Status   CursorStatus  // Running | Parked | Done
}

type ResumePoint struct {
    Frame FrameID
    PC    int
}
```

- `do X...` — push a ResumePoint, switch Frame to the new child, reset PC to 0.
- `!` (RESOLVE) — pop ResumePoint, switch Frame back to parent, advance PC past the `do` site.
- `start X as N...` — allocate a fresh cursor for the child; current cursor's PC advances immediately.

This is the same shape as a coroutine call stack, but reified so we can suspend and resume cursors when they park on listeners or locks.

---

## Scheduler

```go
type Scheduler struct {
    Ready  []*Cursor    // run these now
    Parked map[CursorID]*Cursor
    Done   []*Cursor
}
```

The scheduler is a simple ready-queue. Run a cursor until it parks, completes, or yields voluntarily. Then pick the next.

A cursor parks when its current frame moves to `StatusWaiting` — triggered by:

- WAIT_UNTIL with a predicate that doesn't immediately match
- LOCK on a held set
- LISTEN install (the *body* of the listener doesn't run until fired)
- INVOKE_SYNC on a child that itself parks

Wakers (events that move parked cursors back to Ready):

- listener fires → wake the listener's owning cursor (or spawn a fresh one for actor `hears`)
- lock release → wake the next FIFO waiter
- async cursor completes → no waker needed; observers fire via their own listeners
- `wait until` predicate becomes true → fire the same way as a status listener

Open: do we run all Ready cursors to a quiescent point in one round, or interleave? V1: run-to-quiescence per cursor (cooperative), one cursor at a time. Multi-thread later if needed.

---

## Tick

A single scheduler tick:

```
loop:
  if no ready cursors and no parked cursors:
      shutdown
  if no ready cursors but parked exist:
      block on next external event (UI input, IO, timer)
      drain wakers into Ready
  pop cursor from Ready
  run cursor until it parks, completes, or yields:
      step()
  if cursor.completed:
      add to Done
      fire wakers for observers of this cursor's frame
```

`step()`:

```
edge = cursor.Frame.Origin.Body.Edges[cursor.PC]
cursor.PC++
dispatch(edge)
```

Dispatch is a switch on `Edge.Kind`. Each case is small.

---

## Edge dispatch — the meat

A walkthrough of the important kinds.

### INVOKE_SYNC

```
1. resolve target node (action)
2. allocate child Frame:
     parent = cursor.Frame
     origin = action node
     input  = build payload from edge.Payload
3. push ResumePoint(cursor.Frame, cursor.PC) onto cursor.Stack
4. cursor.Frame = child
5. cursor.PC = 0
6. continue stepping in the child's body
```

When the child RESOLVEs (or its body completes via fall-through... wait, that's a compile error — phrases must resolve). On RESOLVE:

```
1. set child.Result, child.Error, child.UserStatus from RESOLVE edge
2. set child.Status = Completed
3. pop ResumePoint
4. cursor.Frame = parent
5. cursor.PC = parent's saved PC + 1
6. parent.scope.that = child   // update `that` binding
7. fire any listeners on `child` whose predicates now match
```

### INVOKE_ASYNC

```
1. resolve target, allocate child same as INVOKE_SYNC
2. allocate fresh Cursor with:
     Frame = child
     PC    = 0
3. push fresh cursor onto Scheduler.Ready
4. parent's cursor.PC advances immediately (no Stack push)
5. bind name (`as N`) so observers can reference the child frame
```

### CAPABILITY_INVOKE

```
1. resolve target frame (`that` or `its` ref)
2. look up capability on target's contract; reject if not declared
3. behavior depends on capability:
     - retry  -> spawn a sibling INVOKE_SYNC of target.Origin with same input
     - cancel -> set target.UserStatus = canceled, fire its observers
     - rollback -> run rollback handler if defined; otherwise generic state revert (TBD)
     - commit  -> finalize pending state (TBD; semantics need design)
```

The runtime needs a capability dispatch table per capability name. Built-ins (`retry`, `cancel`) have engine-defined semantics. Open question whether user-defined capabilities are possible in v1.

### BRANCH_OPEN / BRANCH_ALT / BRANCH_FALLBACK

Compile a branch group into an internal "select" form:

```go
type BranchGroup struct {
    Arms     []BranchArm
    Fallback *Block         // nil if no `or?`
    State    BranchState    // Pending | Resolved
}

type BranchArm struct {
    Predicate *Predicate
    Body      *Block
}
```

Dispatch:

```
1. install each arm's predicate as a listener on its subject
   (most predicates are status checks on `that` — listener gates on UserStatus)
2. install fallback as a "match-anything-still-unmatched" listener
3. when a listener fires:
     - all other arms are removed
     - the firing arm's body runs in a fresh child frame whose parent is the
       branch's owning phrase
4. when no arm has fired and all subjects are settled with no match:
     - run fallback (if any)
     - else compile error (this should have been caught at compile time)
```

For an immediate-check (subject is already settled at branch entry), skip the listener install — evaluate predicates inline and run the matching body directly.

### LISTEN (passive — `when X hears Y?`)

```
1. install Listener on target X
2. continue stepping (does NOT block — listener is passive)
3. when X emits Y:
     - allocate fresh Frame whose Origin is the listener's body block
     - allocate fresh Cursor for the listener frame
     - push to Ready
```

Multiple listeners on the same message → multiple frames, each on their own cursor. Order undefined.

### EMIT (`say X to Y!`)

```
1. resolve target Y (must be an actor or a set being used as a routing target)
2. allocate a "message frame" (kind: NodeMessage origin) with payload from invocation site
3. for each listener installed on Y for X:
     - allocate a body frame, schedule on a fresh cursor
4. close the owning phrase (the `!` carries the return)
```

Worth noting: the "message frame" itself doesn't run any body of its own. It's a record. The bodies that run are the listener bodies, each in their own frame.

### WAIT_UNTIL

```
1. compile the predicate
2. if predicate already true: skip; advance PC
3. else:
     - install Listener whose Body is "advance PC past this edge"
     - park cursor (Status = Parked)
     - listener fires when predicate becomes true; wakes cursor
```

### LOCK

```
1. resolve set S
2. if S is unlocked OR held by current frame:
     - mark Holder, install handle on current frame
     - continue stepping inside the lock body
3. else:
     - enqueue current cursor on S.Lock.Waiters
     - park cursor
4. on phrase close (RESOLVE or fall-through):
     - release lock
     - wake the next waiter
```

### FOR_EACH

For collections that satisfy `iterable`:

```
1. allocate iteration frame; bind key/value/previous/next/first/last
2. for each entry:
     - update bindings
     - run body in a fresh sub-frame
     - on RESOLVE of sub-frame, advance to next entry
     - SKIP / STOP edges short-circuit
3. resolve the iteration frame (typically with `ok`)
```

Streams (`feed`) make this potentially infinite — the iteration frame is parked between entries, woken by feed writes.

---

## Frame status transitions

```
                +---------+
   spawned -->  | Pending |  rare; allocated but not yet on a cursor
                +---------+
                     |
                     v
                +---------+
        run --> | Active  | <----+
                +---------+      |
                  |   ^   |      |
   park           |   |   |   resume
   (wait, lock,   v   |   |
    listener)  +---------+|
               | Waiting ||
               +---------+|
                     |    |
                resolve   |
                     v    |
                +-----------+
                | Completed |
                +-----------+
```

Once Completed, a frame's parent can read `that.UserStatus`, `that's <field>`, `that's error` freely. The frame itself doesn't do anything else.

---

## Where it gets interesting

- **Branch listener install order** matters when multiple subjects could fire simultaneously. Spec says listener order is undefined unless explicitly sequenced — but within a branch group, only one arm wins. Need a guard: first-to-match wins, others are no-ops, all listeners removed atomically.

- **Lock priority** is FIFO per spec. But if `wait until` and `lock` are both waiting on the same set... open question. Tend to think `lock` always waits for the lock release; `wait until <S> unlocked` does the same thing more explicitly.

- **Async result observation** — when a `start` child completes, its observers fire. If no observer is installed, the child's completion is silent. That's fine — but tracing should still record it.

- **Cursor death on parent resolve** — if parent A spawns async child B, then A resolves before B completes, what happens to B? Two options: (a) B continues independently, becomes orphaned; (b) B is canceled. Lean (a) for v1; orphaned async frames are reaped when their root script ends. Open question. See `99-open-questions.md`.

---

## Next file

`04-scheduler.md` — drilling further into cursor scheduling, especially across actor boundaries.
