---
type: decision
status: accepted
date: 2026-08-29
affects:
  - passive-observation
  - semantic-memory
  - service
source_paths:
  - internal/director/semanticmemory/learning.go
  - cmd/director/learningfeed.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/observeambient.go
  - cmd/marco/observefeed.go
---

# ADR-111 — a demonstration takes the slot from watching, and the feed only says what committed

## Context

The 37-series closed as `READY_FOR_DOGFOOD`. 38A is the first phase whose question is not "does
this subsystem hold" but "what happens when somebody uses this". Two things came out of turning
Marco on: a product surface that did not exist, and a defect that stopped the headline loop dead.

## The feed: tell me when you learn something

A person watching Marco watch is not learning anything. The moment worth surfacing is the one
where **durable knowledge changes**, because that is the moment they can turn round and ask Marco
to use it.

**The events come from `semanticmemory.Store` itself, after its own write has committed.**
Nothing upstream announces, and nothing predicts. A Place refused at a bound, a signature that
turned out to match a record already held, a file that could not be written — all ordinary — reach
a person as silence, which is the truth.

Four words, because they are four facts:

| | |
|---|---|
| `learned` | it did not exist and now does |
| `strengthened` | it existed and gained independent evidence |
| `named` | something Marco could recognise can now be spoken about |
| `rebound` | a word that meant one place now means another |

A feed that said "learned" every time somebody walked a familiar route would train them to stop
reading it.

**Names are resolved when somebody looks, not when the write happened.** A Place is established on
one pass and named on a later one; a value carrying the name it had at the instant of the write
would say `[unnamed]` forever about a Place that is now perfectly well named. An unnamed Place says
so and shows its subject — [[ADR-110-a-navigation-rail-is-a-list-of-places-you-could-go]] leaves
that a real outcome, and hiding it would hide the thing a dogfood session exists to find.

`marco observe --follow` prints it. When learning is off it says so rather than sitting blank,
because a silent feed reads as a broken one.

## The defect: you could not teach Marco while Marco was watching

Measured, from a cold isolated home, doing exactly what the product invites:

```
marco observe learn            → Marco is watching, learning from what it sees
marco learn "…"                → phase: refused
                                 refused: no_observation
                                 "I couldn't watch — I lost sight of that window."
```

Every time. With watching **off**, the identical command reached `ready_for_demo` and established
a Place — which is what makes it a slot conflict rather than anything about the window.

One observation session runs at a time, by design: two would contend for the screen and neither
could attribute what it saw. Ambient watching held it, and Learn was refused. So the loop the whole
product is built around — *watch me, then teach me* — could not be walked from a command line at
all, and the message sent the person to inspect their screen for a fault that was not there.

## Decision

**Background attention yields to a demonstration.** Light Mode already had this rule; ambient
watching now follows it, for the reason that rule gives: watching is an instrument, a
demonstration is the work, and somebody who typed `learn` has said which of the two they want.

It yields **only the session ambient itself started** — a passive `observe-game` somebody set up
deliberately is not Marco's to cancel, and the refusal is right for that one. Watching is not
switched off: the supervisor sees the slot go, stops early, and asks for it back. Measured after
the fix, Marco is still watching and still recognises where it is.

### And it is taken where a session is actually started

The first version put the yield in `Runtime.Learn`, which only the control surface reaches. The
command line goes through `LearnSession`, so `marco learn` went on being refused exactly as
before — **the rule was right and nothing on the path a person types called it**. That is the
fourth time in this series a correct mechanism has been wired to the wrong caller, and it is why
the gate for it enters at the arm that starts the session rather than at the helper.

## Consequences

- `marco observe --follow` is a new surface. It reports committed knowledge and nothing else: no
  samples, no walks, no candidates, no confidences.
- The feed is per-process and bounded at 256 events. Dropping older events is reported rather than
  hidden — a reader who looked away must not be shown silence and conclude nothing happened.
- No authority anywhere near it. Learning events grant nothing, and the read cannot create the
  knowledge it reports.

## Enforced by

- `internal/director/semanticmemory` `TestRecognisingAPlaceAgainIsNotLearningIt`,
  `TestAWriteThatFailedAnnouncesNothing`,
  `TestAKnownWayIsStrengthenedRatherThanLearnedAgain`,
  `TestBindingAWordToAPlaceIsAnnouncedAndRebindingSaysSo`
- `cmd/director` `TestTheFeedIsFedByTheStoresOwnCommits`,
  `TestAPlaceNamedLaterRendersWithItsName`,
  `TestTheDirectorWiresItsFeedWhereItOpensItsMemory`
- `cmd/director` `TestLearningTakesTheSlotBackFromWatching`

## Related

- [[Experiment-022-the-first-dogfood]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
