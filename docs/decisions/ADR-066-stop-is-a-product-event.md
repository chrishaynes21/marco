---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - demonstrations
  - learned-plays
source_paths:
  - internal/director/learn/learn.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/learnview.go
  - internal/director/service/protocol.go
---

# ADR-066 — stop is a product event

A demonstration used to end when a clock ran out — `Bounds.Watch`, forty-five seconds — or when
the person happened to hold still long enough to look finished.

## What was wrong

Both are guesses about something the person knows for certain, and both fail in the two directions
that matter. A careful demonstration is cut off mid-route. A quick one leaves the person sitting
in front of a window doing nothing, wondering whether Marco noticed.

Measured, twice, on the same afternoon: one live attempt spent its whole forty-five seconds
waiting for somebody who had not been told the window had opened, and refused `nothing_changed`
having measured nothing about Marco at all. A second was rescued only by an audible beep bolted
onto a test harness. Every one of those failures was a failure to agree on WHEN the demonstration
was over — a question the person can answer and the system cannot.

## The decision

**"I am done" is evidence, and the person is the only reliable source of it.**

`Coordinator.Finish()` ends admission of new task input and keeps every single thing already
captured. The in-flight pass is FINISHED rather than abandoned: `Passes.Finish()` cancels the
running observation the ordinary way, which the observation layer already defines as "stop early
and keep the evidence" — the same thing `cancel-observation` does, so `RunPass` returns the
finished record and the pipeline continues from a shorter pass rather than from nothing.

Nothing downstream is skipped or shortened. The destination is established, the demonstration is
built, the candidate is assessed. A second, shorter path for "finished early" would be a second
implementation of everything after capture.

### Not Cancel, and the difference is everything

`Cancel` throws the attempt away and keeps nothing. `Finish` is the reason the attempt exists.
They are separate fields on the request, separate methods on the coordinator and separate flags in
its state, because routing one to the other silently destroys a demonstration a person has just
given — and it would look like it worked.

### What it does not require

The person does not have to end where they started, walk a circular route, or sit still for an
arbitrary time before pressing Stop. The clock survives only as a backstop.

## Enforced by

- `cmd/director/learnwiring_test.go` — `TestStopFinishesTheDemonstrationAndCancelDiscardsIt`
  holds both halves in one test, so neither can be satisfied by collapsing them. Mutation-gated:
  routing Stop to `teach.stop()` fails it.
  `TestStopReachesThePassThatIsRunning` holds the seam — a `Finish` that set its flag and did not
  tell the pass would leave the person waiting out the very timer they pressed Stop to avoid.

## Related

[[ADR-065-operating-marco-is-not-demonstrating-to-it]] ·
[[ADR-052-the-pass-that-watched-it-is-the-demonstration]] ·
[[ADR-057-attributed-input-survives-interpretation]] · [[Demonstrations]]
