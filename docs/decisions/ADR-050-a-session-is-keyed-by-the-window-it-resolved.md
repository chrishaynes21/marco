---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - windows
source_paths:
  - internal/director/observesession/runner.go
  - internal/director/observesession/armwiring_test.go
---

# ADR-050 — a session is keyed by the window it resolved, not the one it was asked for

The second half of teaching never watched anything. A person demonstrated a route on Windows
Settings four times, every traversal landed in the store with its navigation intact, and Marco
said:

> Something went wrong on my side — I wasn't watching for your example.

It was right that something had gone wrong, and wrong about everything else.

## What was actually wrong

A demonstration capture is armed only when durable memory holds a relationship whose learning
request is pending. That lookup is keyed by **application**:

```go
r.armCapture(cfg.Selector.Application)   // before the loop
```

A `windowref.Selector` names a **window** — by ephemeral id, by title, by process id — and only the
`--application` form happens to carry an application name. `currentContext`, which resolves the
foreground and pins it, returns `Selector{EphemeralID: id}` and nothing else. So the arming asked
for `Topology("")`, which holds no relationships, found nothing pending, and created no capture.

The resolved name was one line further down the whole time, as `ref.Application`, and the session
already used it: `r.session.Application = ref.Application`.

The pre-loop placement was not arbitrary — the capture must exist before the first sample so that a
user *already standing* where the request begins is seen. But an application cannot be known before
a window reference resolves, and a window reference resolves inside the loop. The two requirements
were in tension and the tension was resolved silently in favour of the wrong one.

## The decision

**Identity of what a session is watching comes from resolution, never from the request.** The
selector says which window to find; only the resolved reference says what was found.

The arming moves onto the first sample, above the point where anything reaches the capture:

```go
r.armForApplication(ref.Application)   // then the locked fold, then observeCapture
```

- **Once per session.** The lookup reads the durable topology, which takes the store's lock and may
  touch a file; per-sample it would put both on the sampling path twice a second. `armAttempted`
  makes it one attempt, and the answer cannot change — the selector is pinned to one window of one
  process for the run.
- **An empty name does not consume the attempt.** A reference that resolves without an application
  — a process still starting, a shell surface that resolves late — must not spend the only try.
- **Outside the session lock**, for the reason `reviewLearning` already gives.

## Why no test could see it

Every fixture in `observesession` builds `Selector{Application: "testgame"}`. The call site was
executed on every run of the suite, with an argument production could never produce.

This is a wiring failure of a shape [[Wiring-Tests]] had not recorded: not code that is never
called, but code called with a value only the fixture can supply. Reachability is satisfied,
coverage is satisfied, and the mechanism is wrong for every real caller. The new tests differ from
the existing ones in exactly one field.

`--window-id` has the same hole, so it was never only the foreground path — but the foreground path
is the one a person uses, and no live teach had ever reached its second pass before, which is why
this surfaced on the first run that got that far.

## Considered and rejected

- **Resolve the application before the loop.** Needs a platform call in the runner purely to learn
  something the first sample is about to say, and adds a second resolution that can disagree with
  the one the session uses.
- **Fill `Selector.Application` in when resolving the foreground.** Makes the selector carry a
  derived fact, and a selector that names both a window and an application invites the two to
  disagree after a process is recycled — the identity problem `windowref` exists to prevent.
- **Fall back to the selector's application when the reference has none.** Preserves the broken
  path as a special case and would have kept the defect alive for `--application` sessions whose
  window resolves to something else.

## Enforced by

`internal/director/observesession/armwiring_test.go`:

- `TestAWindowChosenWithoutNamingItsApplicationStillArms` — the defect itself, with a selector that
  carries only an ephemeral id.
- `TestNamingTheApplicationStillArms` — the control, so the fix did not move the hole.
- `TestTheCaptureSeesTheVeryFirstSample` — arming even one sample late loses a user already
  standing on the start.
- `TestTheTopologyIsConsultedOncePerSession` — the store is not read on the sampling path.
- `TestAnUnnamedFirstSampleDoesNotBurnTheArmingAttempt` — a late-resolving name still arms.

Four mutations, four caught. The empty-name guard survived the first run and its test was written
before the second.

## Related

[[ADR-042-a-click-is-a-place-in-a-window]] ·
[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[Wiring-Tests]] · [[Passive-Observation]] · [[Windows]]
