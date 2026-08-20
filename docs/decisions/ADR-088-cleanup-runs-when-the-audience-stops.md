---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - runtime
  - language
  - cancellation
source_paths:
  - internal/runtime/runtime.go
  - internal/runtime/frame.go
  - internal/runtime/cleanup_test.go
---

# ADR-088 — cleanup runs when the Audience stops

## `finally` had never once run on a cancellation

`spec/Core.md` is **normative**, and on `finally` it says:

> Runs however the surrounding work ended, **including cancellation**.

Its worked example is `do Keyboard's KeyUp with "shift"` — releasing a key the program is holding
down. That is precisely the work that must still happen when somebody presses stop, and it is the
one case in which it never happened.

The mechanism was one `if`. `runBlock` bailed out of the body when the frame read `StatusCanceled`
and called `runFinallies`, which re-entered `runBlock` **on the same still-cancelled frame** — so
the cleanup body's first ordinary edge hit the same guard and returned immediately. Both entrances
to cancellation, the external context watchdog and the language-level `cancel`, go through the same
`cancelTree`, so this was universal.

Underneath it, a second fault that would have made a naive fix cosmetic. `Frame.cancelIfRunning`
also cancels the frame's `goctx`; child frames chain from `parent.ctx()` and host calls receive it
as `HostCall.Ctx`. So even with the guard fixed, a `finally` that called a host would have handed it
an **already-cancelled context** and the release would have refused. Both halves had to move or
neither was worth moving.

Nothing covered it. No shipped Play in `routes/` uses `finally`, so it was latent rather than
reported — but the canonical use of the feature is releasing held input, and the product had just
been given one stop word that reaches every running Play
([[ADR-087-one-stop-and-it-crosses-a-process-boundary]]). A stop that arrives as a cancellation and
then skips the cleanup releases nothing; it merely stops in a different way than a kill did.

## The decision

**A frame whose cleanup would otherwise be stranded is rescued: the cancellation bail-out is
suspended for the duration of its `finally` bodies, and those bodies run under a fresh, bounded
context.**

This is implementing the spec, not amending it. No syntax, no new word, no change to `spec/`.
Language work stays closed; what changed is that the implementation now does what Core already said.

Four properties, and each is load-bearing:

- **The status does not move.** The frame stays `StatusCanceled` the whole way through, so
  `when this was canceled?` inside a cleanup still matches. `runFinallies`' own doc comment already
  promised this; it is now true under cancellation as well.
- **The cleanup gets a LIVE context.** `Frame.ctx()` returns the cleanup context while in cleanup,
  so child frames chain from it and host calls receive it. This is the half that actually releases
  the key; the guard alone would have run a cleanup that could do nothing.
- **It is bounded.** A person pressed stop, and stop must not silently become hang. One named
  constant, with its derivation beside it: far longer than any honest cleanup — a key, a mouse
  button, a handle — and short enough that a wedged one reads as a hiccup rather than a freeze. A
  cleanup that overruns is **abandoned where it stands**, not retried.
- **Nesting shares one budget.** A `finally` inside a `finally` shares the outer context rather than
  minting a fresh one, because a new budget per level would let an expired cleanup buy more time,
  and buying more time is a retry the person did not ask for.

**The rescue keys off the CONTEXT, not only the status, and that is not redundant.** `cancelTree`
flips a frame's status and cancels its context in the same breath, but context cancellation
propagates down the whole chain instantly while the status walk is still descending. A deep child
can therefore finish its body and reach its `finally` reading `ok` under a context its cancelled
ancestor has already killed. Keying off the status alone misses that child. This was found by
mutation — two mutations survived the first set of tests, which is why the frame-level unit tests
exist.

**The ordinary path enters nothing.** A frame that ended normally with a live context keeps exactly
the behaviour it had, with no cleanup context and no deadline. That is the case that runs every time
anybody uses Marco, and it is asserted directly rather than assumed.

## Considered and rejected

- **Clear the cancelled status before running `finally`.** One line, and it destroys the feature: a
  cleanup could no longer tell how the work ended, and `spec/Core.md` promises it "cannot turn a
  failure into a success by accident".
- **Delay cancelling `goctx` until after the cleanup.** The whole purpose of cancelling it is to
  interrupt the in-flight host call the person is trying to stop. Keeping it alive would make stop
  wait for the thing it is stopping.
- **Give the cleanup an unbounded fresh context.** Honest-looking and wrong: a `finally` containing
  a long wait would turn one stop into an indefinite hang, with nothing on screen to explain it.
- **Fix only the guard.** The visible symptom disappears and the actual work still fails, because the
  host call is handed a dead context. It would have looked fixed and released nothing — the worst of
  the available outcomes.
- **Handle it in `cmd/marco` instead, by releasing held input after a cancelled run.** That is a
  second cleanup mechanism living beside the language's own, applying only to the hosts `cmd/marco`
  knows about, and silently not applying to a Play's own `finally`.

## Consequences, including the costs

- **Stop is now slower by however long a cleanup takes**, bounded by the budget. That is the correct
  trade — a key left held is worse than a stop that takes a moment — but it is a real change in how
  stop feels, and the budget is a guess that will need revisiting against a real Play.
- **A cleanup that overruns is abandoned mid-way**, which can leave a Play half-cleaned-up. There is
  currently no report of this to the Audience; the run simply stops where it stands.
- **`Frame.ctx()` now takes the frame lock.** It is called once per host call. The cost is
  negligible and the alternative — an unguarded field read across goroutines — is the data race this
  package has already had once.
- **A `finally` that blocks on a Marco-level `wait`, or on a host that ignores its context, is still
  not interruptible before the budget expires.** The budget bounds every case that respects the
  context, which is every real host in this tree; a context-ignoring host is unbounded by
  construction and no runtime change can fix it.
- **A second stop cannot interrupt a cleanup.** Only the budget can. That is the intended reading —
  the cleanup *is* what the cancellation asked for — but it means the stop word does not compose with
  itself during that window.

## Enforced by

- `internal/runtime` — `TestFinallyRunsAfterExternalStop` is the load-bearing one: it is the path the
  Audience's stop word actually takes. `TestFinallyHostCallGetsLiveContextAfterStop` holds the half
  that releases the key. `TestCanceledStatusVisibleInsideFinally` holds the status.
  `TestBlockedFinallyDoesNotHangTheRun` holds the bound.
  `TestFinallyOnSuccessPathIsUnchanged` / `TestFinallyOnFailurePathIsUnchanged` hold the ordinary
  path, which is the one that must not have moved.
- Proven independently of the package, through the shipped binary: a Play running a long loop with a
  `finally`, stopped from a **separate process** by `marco stop`, cut off mid-loop with its cleanup
  line printed — and, with the guard reverted, the cleanup line absent.

## Related

- [[ADR-087-one-stop-and-it-crosses-a-process-boundary]] — the reason this became urgent
- [[Wiring-Tests]] — a complete, correct, untested mechanism that nothing reached; the fourth
  recorded instance
