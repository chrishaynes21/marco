---
type: decision
status: accepted
date: 2026-08-29
affects:
  - passive-observation
  - semantic-memory
  - invocation
  - editing
source_paths:
  - cmd/marco/watchui.go
  - cmd/marco/runaccount.go
  - cmd/marco/edit.go
---

# ADR-112 — the loop belongs where a person is already looking

## Context

[[ADR-111-a-demonstration-takes-the-slot-from-watching]] built the feed that says what Marco
committed, and [[Experiment-022-the-first-dogfood]] proved it reports the truth and nothing else.
Then the session it was built for was run out of **two PowerShell consoles** — one holding the
Director, one printing `marco observe --follow` — with a person alt-tabbing between them to find
out whether their assistant had learned anything.

That is a test of the plumbing. The feed was correct and the loop it served was not a product.

## Decision

**The dogfood loop lands on the Here panel, which is where somebody already goes to ask what Marco
can see.** Not a new view, not a new application, not a second window. Three things were missing
from it and they are all this change is:

| | |
|---|---|
| whether Marco may REMEMBER | Here already had Light Mode's Watch — attention. It could not say whether what was seen may become durable, which is the other half somebody agreed to separately and is entitled to see the state of. |
| what it just COMMITTED | The feed, polled from the same page, newest first. |
| a way to USE it | The words, run through the door a typed `marco do` uses. |

### It may ask, and may not decide

Four verbs, and every one is about attention or permission — `watch`, `stop`, `learn`, `unlearn`.
There is no fifth, and **an unrecognised verb is a read rather than the nearest match**: two of
these change what Marco is doing, and routing a typo to one of them would turn a mistyped word into
a decision about somebody's memory.

Nothing on this surface writes to semantic memory, and nothing on it announces learning. The events
are the store's own, after its own write committed, and they arrive **already worded** by the only
process holding the store — so the page cannot invent a Place name even by accident. The feed's
request sets one field, a cursor, and leaves every other nil; a poll on a timer changes nothing
about what Marco does however often it fires.

### Try it is not a new executor

It spawns `marco do`, exactly as a clicked Run has since Phase 0. The only difference between the
two is which of the two things the person did — chose a play from a list, or said what they wanted
— which is why it is a second argument vector and not a second mechanism. Intake, planning,
compilation to legal Marco, the authority check, the actuation lease and verification all happen
where they already happen. The answer is a run id and `/api/run` says what became of it, because
the engine's own word is the only honest one.

### And the mux became a function

`serveMux` was built inline inside `runEdit`, which binds a port, prints, opens a browser and then
blocks in `http.Serve` — so **nothing could ask what this surface serves without starting it**.
That is the exact shape of the failure this branch keeps finding: a handler written, a page written
to call it, and no test able to enter where production enters. The wiring gate now reads every
`/api/…` the page fetches and asks the real mux whether it answers, naming none of them itself.

Falling through to the `/` catch-all is a failure, not an answer: an unregistered `/api` path
serves the page's own HTML with a 200, so a missing handler would reach a person as a JSON parse
error rather than as a missing handler.

## Consequences

- `marco ui` is the whole dogfood surface. No second console, and the page starts the Director
  itself when somebody presses a button that turns something on — a press is not a read.
- Closing the control centre stops nothing. The only request that ends watching is the one
  somebody pressed Stop for.
- The page keeps twelve events on screen; the Director's ring of 256 is the real bound and
  `missed` is rendered rather than swallowed.

## Enforced by

- `cmd/marco` `TestEveryDoorThePageKnocksOnIsAnswered`
- `cmd/marco` `TestTheDogfoodStripOnlyAsksAboutAttentionAndPermission`
- `cmd/marco` `TestTheFeedAsksFromWhereItGotTo`, `TestTheFeedCarriesNoScreenContent`
- `cmd/marco` `TestTryItRunsThePhraseThroughMarcoDo`, `TestTryItWithNoWordsStartsNothing`
- `cmd/marco` `TestTheDogfoodLoopIsOnOneSurface`, `TestTheDogfoodStripRendersWithNoDirector`

## Related

- [[ADR-111-a-demonstration-takes-the-slot-from-watching]]
- [[Experiment-022-the-first-dogfood]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[ADR-113-learning-is-inside-watching]]
