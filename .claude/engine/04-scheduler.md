# Engine — Scheduler

How cursors are picked, parked, and woken. The thing that makes the engine feel responsive vs. dead.

---

## V1 model: cooperative single-threaded

One OS thread runs all cursors. Cursors yield voluntarily when they:

- park (wait/lock/listener install with no immediate fire)
- complete
- explicitly yield (rare; not a v1 feature)

This is fine for desktop scripting and most automation use cases. Throughput-bound server work would push us toward multi-threaded later — but the cursor model is already designed to scale: cursors are independent except where they touch shared sets (which are governed by `lock`).

Pros for v1:
- no data races by construction
- no need for atomic ref counts on frames
- listener firing is deterministic-by-discovery-order
- easy to reason about

Cons:
- one slow action blocks the whole runtime; `start` doesn't help if the started action is CPU-bound
- IO has to be non-blocking or run on a side thread feeding events back

Mitigation: external IO (HotKey reads, file IO, network) lives behind acts (FFI-shaped nodes). Acts run their work on a side goroutine and post a completion event back to the main scheduler. The main scheduler stays single-threaded.

---

## Ready queue

```go
type Scheduler struct {
    ready   *ringBuffer[*Cursor]
    parked  map[CursorID]*Cursor
    timers  *minHeap[Timer]      // for wait until time conditions
    events  chan ExternalEvent   // from acts / IO
}
```

Each tick:

1. Drain `events` non-blockingly into wakers.
2. Drain expired `timers` into wakers.
3. If `ready` is non-empty, pop and run.
4. Else if `parked` is non-empty: block on `events` (or until next timer).
5. Else: shutdown — no work and no observers.

Running a cursor means: run `step()` until the cursor parks, completes, or hits a budget cap. Budget cap is for fairness across cursors — without it, a `while true` loop would starve everyone else.

Budget cap proposal: ~1024 edges before voluntary yield. Tune empirically.

---

## Wakers

A waker is "this thing happened, that cursor is now Ready again". Sources:

| Trigger | Wakes |
|---|---|
| Listener fires | A *fresh* cursor running the listener body |
| `wait until` predicate becomes true | Owning cursor (advance PC past the wait edge) |
| Lock release | Next FIFO waiter on that lock |
| Async cursor (`start`) completes | Any observers (themselves listeners) |
| External event from act | Any matching `hears` listeners |
| Timer expires | Whoever was waiting on that time predicate |

The waker logic is centralized — listener fires generate cursor wakes through a uniform pathway. Don't scatter wake-up logic across edge dispatch sites.

---

## Listener registry

Two kinds of listeners need different homes:

- **Per-frame listeners** — branch arms, `wait until`, status observers on `that` or named frames. Lifetime ≤ owning frame. Stored on the frame: `frame.Listeners`.
- **Per-actor listeners** — `when X hears Y?`, `when Feed reads Y?`. Lifetime ≤ owning actor / module. Stored on the actor's node or a global registry indexed by (target, message).

Lookup paths:

- `that` resolves → check `that.Listeners` for status predicates that match new UserStatus
- `say X to Y!` → look up `(Y.NodeID, X)` in the actor registry
- `wait until <pred>` → install on the relevant subject; on subject change, re-evaluate

The actor registry is keyed by (target, message-name), so emit is O(listeners-on-this-message), not O(all-listeners).

---

## Status change as the universal trigger

Most wakers reduce to "frame X just transitioned to UserStatus Y". A unified path:

```
on frame.transition(newStatus):
    for L in frame.Listeners:
        if L.Predicate.matches(frame, newStatus):
            wake(L)
    // also notify listeners on named-frame bindings
    if frame is bound to a name N:
        for L in actor.Listeners(N):
            if L.Predicate.matches(frame, newStatus):
                wake(L)
```

This handles:
- branch group resolution (predicates installed at branch entry)
- `wait until <Frame> ok?` (single-shot listener on Frame's status)
- `start X as N...` followed by `when N ok?` (named-frame listener)

`hears` is structurally similar but indexed by (target, message-name) instead of (frame, status).

---

## Multi-threaded later

When/if we go multi-threaded:

- Cursors become work units for a worker pool.
- Frames stay owned by one cursor at a time; transferring ownership requires a barrier.
- Sets need locking even for reads if mutators are running concurrently. `lock` already gives us write exclusion; reads-during-mutation become the tricky case.
- Listener firing needs atomic dispatch — first-match-wins on a branch group must be a CAS or a single-writer.

Most of this is achievable without changing the language semantics. The single-threaded v1 is a load-bearing simplification that buys us correctness, but the design isn't single-thread-bound.

---

## Real-time / determinism

Marco's spec says ordered flows execute deterministically; listener ordering is not guaranteed unless explicitly controlled. The single-threaded scheduler trivially satisfies the first part. For the second, we need to decide:

- Should listener firing be deterministic-by-installation-order? Probably yes for testability — same input + same code = same output.
- Should it be configurable? Maybe — for performance-sensitive workloads where order doesn't matter.

V1 default: deterministic-by-installation-order within a single tick. Document and lock.

---

## Open

- **Cooperative yield** — should we expose explicit `yield.` for hand-tuning fairness? Probably not for v1.
- **Timer resolution** — `wait until it's been 5 seconds?` — what granularity? OS timer ticks are ~ms; finer needs polling. V1: ms granularity is plenty.
- **Backpressure on feeds** — when listeners can't keep up, what happens to `write`? Drop, block writer, or grow unbounded? Spec flags this as open. Lean: bounded buffer with `block writer` default; `write` returns `failed with error "feed full"!` if buffer full and the writer wants to fail-fast. Configurable per feed.

---

## Next file

`05-contracts-runtime.md` — runtime enforcement of contracts, obligations, capabilities.
