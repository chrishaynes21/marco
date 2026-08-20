---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - programs
  - marco-boundary
  - demonstrations
---

# A play says where it begins — and Core v1 does not change

Roadmap 26 found that a learned play could be invoked on the wrong screen inside the right
application. This is the fix, and the headline is that **the language already said it**.

---
## The investigation, in the mandatory order

**1. An existing Core construct.** Yes. This compiles today:

```marco
do Screen's Showing with "the pause menu"...
    when ok?
        log "starting".
    or?
        this is failed with error "this play starts on the pause menu"!
do OS's Navigate with "down".
```

`when?` / `or?` over a capability that answers ok-or-failed is exactly the shape `OS's Focus` has
always used to mean *"ok if the active window matches"*. The `or?` arm returns, so nothing after
the block is reached unless the answer was ok.

**So `when?` succeeds and the investigation stops there.** Contracts and anchors were not needed
and were not reached for: a familiar word is not automatically the right abstraction, and reaching
past a construct that already works would have been the wrong kind of tidy.

**What was actually missing was a CAPABILITY, not a construct.** Nothing could answer *"is the
screen the user named the one in front?"*. A capability is declared in an act, which is the route
[[Marco-Boundary]] already prescribes and the same one `Navigate` took in Roadmap 21.

---
## Decision 1 — Core v1 is unchanged. No syntax was added.

Not one sentence of Marco is new. The language freeze holds, and the governance clause reserved
for "a concrete Director requirement Core cannot express" was **not** invoked, because Core could
express it.

---
## Decision 2 — one read-only act

`internal/screenmod/screen.marco`:

```marco
the Screen is an act.
this exports Showing.   // ok if the screen the user named is the one in front
```

Every capability in it READS. There is no sentence in this act that moves a pointer, presses a
key, focuses a window or launches anything — deciding must not be doing.

An act rather than an undeclared actor on purpose: the compiler checks capabilities of *declared*
acts, so a typo fails at compile time. An undeclared `Screen` would have compiled and then done
nothing, which is the failure ADR-005 exists to prevent.

---
## Decision 3 — the shape was found by the RUNTIME, not by the compiler

**Corrected in Roadmap 28, and the correction matters more than the original.**

The shape this ADR first recorded asked the question, let the `or?` arm return, and put the steps
after the block. It compiled. It did not guard: **a return inside an arm ends the arm, not the
capability**, so execution walked straight on to the steps and pressed the keys anyway.

A test that only read the source and asked the compiler would have shipped it — and did, for one
milestone. What caught it was running the play against a Screen host that said no and watching the
keys come out anyway.

The shape that actually guards nests the steps INSIDE the `when ok?` arm, with the `or?` arm ending
in `this is failed with error "…"!`. Nesting is structural: there is no line after the block for
control to fall through to.

**Compiling is not behaving.**

## Decision 3a — the earlier reasoning, kept for the record

The first attempt wrapped the steps inside the `when ok?` arm and ended the `or?` arm with a
period. The compiler refused: *"falls off the end without an explicit return"*. The four endings
in Core are exclamations, and a body whose only returns are inside a two-armed block is not
recognised as returning.

The shape that compiles is the early return — *ask, and stop if the answer is no* — which is also
the one a person would say out loud. That is the language holding its own line, and it is worth
recording that the readable shape and the legal shape turned out to be the same shape.

---
## Decision 4 — fail closed, by construction

`Screen's Showing` returns ok or failed. Mismatch, ambiguity, an unobservable screen, an
unavailable target, a host that cannot answer at all — every one of them is *not ok*, and the
`or?` arm returns before any effect.

Under `cmd/marco` the fallback host is `oshost`, whose unknown-action branch returns **failed**,
so a Marco with no screen recogniser wired refuses a guarded play rather than running it.
**Silence is never yes.**

---
## What this does NOT yet do, and it matters

The capability is **declared but not fulfilled**. No host answers `Screen's Showing` yet, so today
every guarded play refuses. That is the safe direction and it is deliberate — but it means a
learned play is currently unrunnable until Director exposes a read-only recogniser.

And that exposes the answer to the Director-availability question plainly: **a learned play cannot
validate its own starting screen without Director.** Application context is ordinary Marco; screen
recognition is Director's. The separation is "Director figures out the play, Marco performs it" —
not "Marco performs it alone".

The naming question is also open: `"the pause menu"` is what a USER would call the screen, and
nobody has been asked yet. Director must not guess it from OCR.

## Enforced by

- `TestAGuardedPlayCompilesAndIsStillCoreMarco` — compiles against the real act surfaces, stays
  inside Core's vocabulary, names no backstage concept, and no effect is reachable without the
  guard having refused first.
- `TestAPlayWithoutAnEntryConditionSaysNothingAboutScreens` — the mechanism is a property of the
  play, not of learned plays.
