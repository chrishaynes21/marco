---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — SIMPLIFY SHORTENS A ROUTINE, WHICH MEANS IT CLAMPS EVERY REMAINING WAIT TO A QUARTER OF A SECOND AS WELL AS FOLDING TYPED RUNS. It folded typed runs and nothing else, so on a recording of CLICKS it did nothing at all: the owner pressed it on a real Ark routine and got two identical panes. Measured, that routine carried 17 waits totalling 19.6 seconds — every one of them the person's own reaction time between presses — and five characters of typing. The waits ARE the length of a demonstration, so a simplification that could not touch them was a no-op on exactly the recordings somebody most wants shortened, and the window said so only by showing the same source twice. The same recording now renders at 19.4s under the ordinary rebuild and 2.9s under Simplify. THIS AMENDS ADR-254 AND DOES NOT REVERSE IT: that decision disabled the wait cap for a routine stating `coordinates`, and it stands for an ordinary save and for `RebuildCurrent`. Its defect was the ENVIRONMENT ($MARCO_CV) deciding to cap, at save time, invisibly, with nothing on screen to say so; this is a person pressing a control named Simplify, being shown the before and the after, and choosing Replace. 250ms rather than `MaxWaitMs`'s default 50ms, which belongs to the anchored world where a wait-gate does the real waiting: a quarter second is several frames at any refresh rate a game runs at, the same reasoning `workflowrun.DefaultDwell` was chosen under. IT CAN BE TOO FAST AND THAT IS THE PERSON'S TO JUDGE: nothing in Marco knows when a menu will appear, which is precisely what an anchor would have known and anchors are off by default, so the caution says in those words that the routine may press before a screen has caught up and that the answer is to keep the version you have. The owner's call, made explicitly: menu timing is the user's problem while vision is not working, and reducing the length of a routine is what Simplify is for. The control keeps the word Simplify rather than acquiring a second name."
supersedes: []
affects:
  - execution
source_paths:
  - internal/simplify/simplify.go
  - internal/orchestrator/draft.go
  - plugins/studio/frontend/index.html
---

# Simplify takes the waiting out

## What happened

The owner pressed **Simplify…** on a real routine — `cloud pull stone`, an Ark macro — and reported
that the source and the rebuilt version were identical. They were. Reading the routine and running
the production pipeline over its stored demonstration reproduced it exactly: **35 steps in, 35
steps out, byte for byte.**

The routine is 17 waits totalling **19.6 seconds**, and all of it is the person's own reaction time
while demonstrating: 2610ms, 2180ms, 2180ms, 1160ms, 1170ms, 1250ms, 2410ms, 970ms, 2660ms, 2410ms
between presses. The only typing in it is the word `stone`.

Simplify folded typed runs and touched nothing else. So on a recording of clicks — which is most
of what anybody demonstrates in a game — it had nothing it was allowed to do.

## Why it was allowed to do nothing

`Options.MaxWaitMs` caps every wait, and it defaults to 50ms. But
[[ADR-254-what-a-recording-captures-is-stated]] disables that cap entirely for a routine whose file
states `coordinates`, which every Studio recording does. The reasoning is sound and still is: with
anchors off there is nothing to wait **on**, so the recorded gap is the only waiting the routine
has, and capping it presses a menu before it has drawn. That ADR exists because a Studio launched
with `$MARCO_CV=on` once capped every gap and put nothing in its place.

So the cap was off, and folding typing was all that was left.

## The decision

**Simplify shortens the routine.** `RebuildSimplifyTyping` now sets
`simplify.SimplifiedMaxWaitMs` (250ms) as well as `AggressiveTypingRhythmMs`, overriding
`learnOptionsFor` deliberately and only on that path.

Measured on the owner's own recording:

| | steps | waits | total |
| --- | --- | --- | --- |
| `current` (ordinary rebuild) | 35 | 16 | **19.4s** |
| `simplify-typing` | 35 | 16 | **2.9s** |

### This amends ADR-254 rather than reversing it

ADR-254 stands everywhere it stood: an ordinary save keeps every recorded gap exactly, and so does
`RebuildCurrent`, including under `$MARCO_CV=on`. The test that holds it still holds it.

What changed is which actor is allowed to decide. ADR-254's defect was the **environment** capping,
at save time, invisibly — a routine quietly acquiring different timing because of a variable the
process inherited, with nothing on screen to say so. This is a **person** pressing a control named
Simplify, reading a caution that says what it costs, being shown the old and the new source side by
side, and choosing Replace. Nothing is written until they do.

### Why 250ms

`MaxWaitMs`'s 50ms default belongs to the anchored world, where the wait-gate does the real waiting
and the recorded gap is scaffolding. With anchors off there is no gate, so the number has to stand
on its own: a quarter of a second is several frames at any refresh rate a game runs at, and still a
quarter of a second rather than a pause. It is the same reasoning
[[ADR-190-the-pointer-arrives-before-the-button-goes-down]] chose `DefaultDwell` under.

### And it can be too fast

Nothing in Marco knows when a menu will appear. That is exactly what an anchor would have known,
and anchors are off by default — the open question recorded in `cvOff`'s own doc comment, which
this does not answer. So a routine whose screen takes longer than a quarter second to catch up
**will** press before it is there.

That is the owner's call to make, and it was made explicitly: while vision is not working, menu
timing is the user's problem, and reducing the length of a routine is what Simplify is for. What
the product owes them is the sentence before the press rather than the discovery afterwards, so
the caution says it in those words, and the way back is *keep the version you have*.

### The control keeps its name

`Rebuild.Choice()` read **"Simplify typing pauses"**, which was narrower than what this does and
narrower than why anybody presses it. It is now **"Simplify — shorten the waiting"**. It was
briefly "Make it faster" while this was being written, and that was wrong: the button says
Simplify, the owner says simplify, and a second name for one thing is the shape this repository
spends its time undoing.

## What this does not change

- **An ordinary save.** Every recorded gap, exactly as recorded.
- **`RebuildCurrent`.** The rebuild a caller gets by saying nothing still changes the least.
- **The demonstration.** Stored exactly as recorded; simplification remains a re-derivation from it.
- **Which button each press used, which keys were held across what**, and the routine stopping at
  the first action that did not happen.

## Still broken, and not this

The word `stone` in that routine **still does not fold**, and this decision does not fix it. It was
recorded as four `KeyDown`/`KeyUp` holds plus one tap, because `markHolds` classifies a key as held
when another non-modifier key goes down before it is released — which is what typing rollover is,
and the owner's own event log shows it: `s` down at 07.850, `t` down at 07.943, `s` up at 07.999. A
hold carries no `Text`, and both typing-fold passes require one. The hold rule is right for a game
(`hold Q, click, release Q`) and wrong for typing, and the discriminator is not obvious, because Q
and E are printable too. Left open deliberately.

## Enforced by

- `internal/orchestrator` `TestAStatedCoordinateModeSurvivesSimplifyingWithVisionOn` — now a
  three-way reading rather than a two-way one: a stated `coordinates` routine keeps its 3-second
  pause under `RebuildCurrent` with `$MARCO_CV=on` (ADR-254, unchanged), the same routine is
  clamped to `Sleep with 250` under `RebuildSimplifyTyping`, and the control — a routine stating
  nothing, same environment, same ordinary rebuild — is capped, which is what makes the first case
  a reading of the file rather than a build that never caps anything. The mode statement survives
  into both.
- `internal/orchestrator` `TestTheTwoRebuildsProduceDifferentRoutines` — the caution names the
  quarter second, the risk (*before a menu has appeared*) and what is unchanged.
- `internal/orchestrator` `TestSimplifyingAndSayingNoLeavesTheRoutineAlone` — the caution is on
  screen **before** the question, in the terminal.
- One mutation, killed: dropping `opts.MaxWaitMs = simplify.SimplifiedMaxWaitMs` fails the first
  test with *simplifying kept the 3 second pause*.
