# Engine — Locks

`lock <S>...` is the only mutation primitive for shared sets. Everything else is owned-by-one-frame.

---

## When you need a lock

The spec says sets are mutable when:

- owned by the current phrase (single-owner; no lock needed), or
- copied (immutable snapshot; no lock needed), or
- unlocked through a `lockable` shared set

So locks come into play only when the same set is reachable from multiple frames concurrently. In single-threaded v1 that means: across `start` boundaries, across actor `hears` listeners, across queue/feed consumers.

---

## Lock model

```go
type Lock struct {
    SetID   SetID
    Holder  FrameID       // FrameID(0) when unheld
    Reentry int           // counter for re-entry from same holder
    Waiters []*Cursor     // FIFO
}
```

Single writer. No readers/writers split in v1 — too much complexity for the marginal gain. If reads-during-writes become a hot path, we'll add a reader/writer split with a separate `lock for read` form later.

---

## Acquisition

`lock <S>...` edge:

```
1. resolve S → SetID
2. lookup or create Lock for SetID
3. if Lock.Holder == 0:
       Lock.Holder = currentFrame.ID
       Lock.Reentry = 1
       enter body
4. else if Lock.Holder == currentFrame.ID:
       Lock.Reentry++
       enter body
5. else:
       Lock.Waiters.push(currentCursor)
       park cursor (StatusWaiting)
       cursor resumes when wake fires
```

Re-entry support means a frame can `lock` the same set inside an inner phrase without deadlocking itself. Lean: yes, support re-entry in v1 — this matches user intuition for "I already locked this, doing more work shouldn't deadlock."

---

## Release

Lock release happens when the lock-phrase closes. That's:

- The lock body's RESOLVE
- The lock body's parent phrase RESOLVE (cleans up unreleased locks)
- Any orphan cleanup path

```
1. Lock.Reentry--
2. if Lock.Reentry > 0: keep held (we re-entered; only the outermost release counts)
3. else:
       Lock.Holder = 0
       if Lock.Waiters non-empty:
           next = Waiters.pop()
           Lock.Holder = next.Frame.ID
           Lock.Reentry = 1
           wake(next)
```

Worth being explicit: lock release is tied to **phrase close**, not edge advancement. If the body emits a `!` or falls out due to a return, the lock releases. If the body just completes normally (last sentence in the lock phrase ends with `.`), the lock releases when the phrase resolves.

---

## What a phrase close means here

A `lock <S>...` phrase has its own frame. That frame's resolution is the release trigger. In the trace tree:

```
parent
└── lock-frame (origin = synthetic LockNode for S)
    └── body... (body contents become children of lock-frame)
```

Or maybe simpler: the lock body is just sentences in the parent's phrase, and we attach lock metadata directly to the parent frame:

```
parent.Locks.append(LockHandle{S})
```

Going with the second model. Rationale: the spec treats `lock <S>...` as opening a phrase, which it does syntactically — but the body is part of the parent's flow, not a separately-resolvable frame. Treating it as a phrase-with-lock-handle is simpler than allocating a separate frame just for the lock scope.

Open to revisiting if this complicates trace presentation.

---

## Deadlock detection

V1: no automatic detection. Locks are FIFO, re-entrant on same frame, released on phrase close. If you write code that takes lock A then waits on lock B while another holds B and waits on A, you deadlock. We document this and rely on `what happened?` to expose the wait chain.

Possible v2: cycle detection on the wait graph at park time. Cheap because each cursor is on at most one lock waitlist.

---

## Interaction with `wait until`

Two locking-adjacent shapes:

- `lock <S>...` — acquire-or-block.
- `wait until <S> unlocked?` — observe-without-acquire.

The second is non-acquiring. It just parks until the predicate fires. Useful for "let me know when this set is free" without taking the lock.

If you both `wait until <S> unlocked?` and `lock <S>...` in sequence, there's a TOCTOU window — the set could become locked again between your wait and your lock attempt. Standard concurrency hazard; document but don't try to fix in v1.

---

## Lockable contract

The `lockable` built-in contract gives a node `locked`/`unlocked` UserStatus values. A `lock <S>...` operation moves S to `locked` and back to `unlocked`. Status listeners can observe that:

```marco
when MySet locked?
    log "MySet was just locked".
```

Useful for diagnostics and reactive UIs.

Note: `lockable` is on the *contract* of nodes that can be locked. Not every set is `lockable` by default — it's an opt-in declaration. Open question: do plain `the X is a set.` sets get `lockable` automatically? Spec doesn't say; lean **no, opt-in**, since locking adds runtime overhead and most sets don't need it.

---

## Open

- Plain `set` lockable by default? Lean no.
- Reader/writer locks for v2.
- Deadlock detection — at runtime via wait-graph traversal.
- `lock <S> for read` form, deferred.
- What if you `start` a child while holding a lock — does the child inherit the lock holder, or is it a separate frame that has to re-acquire? Default: re-acquire (the started frame is independent). Document.

---

## Next file

`08-debug-and-what.md` — `what` queries, trace tree access, breakpoint sketch.
