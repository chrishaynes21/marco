---
type: experiment
status: partial
date: 2026-08-29
backend:
  - production-perception
  - semantic-memory
fixture: cmd/marco/observefeed.go
result: the-loop-was-blocked-by-a-slot-conflict-and-the-observation-half-needs-human-hands
supersedes: []
source_paths:
  - cmd/marco/observefeed.go
  - cmd/director/learningfeed.go
  - cmd/director/learnsessionwiring.go
---

# Experiment 022 — the first dogfood

## Question

> Can Marco watch normal computer use, learn useful topology, say what it learned, survive a
> restart, and then use that knowledge?

## Method

Isolated `$MARCO_HOME`, cold store, real Director, the real product commands: `marco observe
learn`, `marco observe --follow`, `marco learn`. The real user's home was hashed before and after
and is unchanged.

**One honest limit on the method, stated up front.** Navigation was driven by `ms-settings:` URI
activation and window management, not by a hand on a mouse. That was the only way to drive it from
here, and it turns out to matter — see below.

## What the feed does

Built for this session, because nothing surfaced durable change. It reports what
`semanticmemory.Store` committed, after the write succeeded, and nothing else:

```
Watching. I'll tell you when I learn something you can use.

  + learned place       Mouse
  + named               Mouse
```

With learning off it says so rather than sitting blank. Through the whole session it produced
**zero false events** — including through everything below, where Marco genuinely learned nothing
and correctly said nothing.

## Finding 1 — you could not teach Marco while Marco was watching

The headline loop, from a cold home, doing what the product invites:

```
marco observe learn      → Marco is watching, learning from what it sees
marco learn "…"          → phase: refused
                           refused: no_observation
                           "I couldn't watch — I lost sight of that window."
```

Every time. With watching **off**, the identical command reached `ready_for_demo` and established
a Place. So it is a slot conflict, not the window — and the message points at the window, which
sends somebody to look for a fault that is not there.

One observation runs at a time by design. Ambient watching held it; Learn was refused. **Watch me,
then teach me could not be walked from a command line at all.**

Fixed as a `DOGFOOD_BLOCKER_FIX`, because everything after it was unreachable. Background
attention now yields to a demonstration, which is the rule Light Mode already followed. See
[[ADR-111-a-demonstration-takes-the-slot-from-watching]].

**And the first attempt at the fix did not work**, which is worth as much as the fix. It went into
`Runtime.Learn` — reached only by the control surface — so `marco learn` went on being refused
exactly as before. The rule was right and nothing on the path a person types called it. Fourth
occurrence in this series.

After the fix, measured: `ready_for_demo`, one Place committed, the feed said
`+ learned place Mouse`, and `marco observe status` still reported *"on a screen it knows"* — the
slot came back.

## Finding 2 — the observation half needs a hand on the mouse

Seven Settings pages visited, 180 samples, 10 ambient sessions, 10 transitions recorded — and
**zero candidate edges**. The cause is in `noticed`:

```go
if len(s.Did) != 1 {
    return
}
```

A transition becomes candidate evidence only when exactly one human ACTION is attributed to it.
Actions come from real input events, stamped with the screen state that was current when they were
banked. Driving navigation by URI presses nothing, so from Marco's point of view the screen changed
on its own — and it correctly learned nothing.

**That is the mechanism working, and this method cannot exercise it.** Learning "this control leads
there" requires seeing the control pressed. The remaining phases — edges, goals, execution,
composition, different starting places, midway entry, recovery — all sit behind that, so they were
not reached.

## Finding 3 — two status surfaces disagree

At the same moment:

```
marco observe status              →  noticed 0 screens and 8 moves so far
marco observe status --evidence   →  Marco hasn't watched you go anywhere yet
```

Both true under their own meanings — the first counts the transient buffer, the second the
candidate ledger — and together they read as a contradiction. A person cannot tell from either
that what is missing is an attributed action.

## What was verified

| | |
|---|---|
| feed reports committed knowledge | yes — `learned place`, `named` |
| false learning events | **0** |
| Place established from a cold store | yes, `subj_71727a02470f` |
| survives a Director restart | yes |
| recognised after restart | yes — `recognised subj_71727a02470f` |
| watching resumes after a demonstration | yes |
| edges / goals / execution / composition | **not reached** |

## What a human session needs to do next

Turn on `marco observe learn --follow`, then navigate Windows Settings **by clicking**: Home →
Bluetooth & devices → Mouse, and Home → System → Display. Watch for `+ learned way`. Then
`marco learn` a destination, watch for `+ learned destination`, and immediately try it from
somewhere else.

Everything up to that point is now verified. Everything after it is unmeasured.
