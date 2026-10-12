---
type: experiment
status: planned
date: 2026-10-12
backend:
  - structure
game: rocketleague
fixture: ""
result: not-yet-run
supersedes: []
source_paths:
  - cmd/director/capturegamesession.go
  - cmd/perceptionreplay/frames.go
  - internal/platform/navsource/navsource.go
---

# Experiment 027 — what did you click, and where did it take you

**Status: PLANNED. Nothing has been recorded. No session has been started.**

Written before the data on purpose. The acceptance questions below are the owner's, in their
words, and they were set before anybody knew what the recording would show — which is the only
time a measurement can be agreed on honestly.

## Why Rocket League and not Trackmania

The owner's reason, and it is a better reason than "a second game":

> If your Trackmania mods overlay graphics on the UI, we're introducing extra visual ambiguity
> that may not be representative of the interface Marco is supposed to learn. We want to test
> whether Marco can learn navigation — not whether it can overcome your particular Trackmania
> overlay configuration.

So the 864-frame Trackmania corpus every number in
[[Experiment-026-the-same-menu-seen-from-pixels]] rests on is a recording of a **modded** UI.
That does not invalidate it — the menu rows are the game's — but it does mean the hardest
measurement in this repository was taken on the harder of two surfaces, and nobody said so.
Trackmania stays as the secondary, harder regression environment.

Rocket League is not a special case and must not become one: same perception, same graph, same
input correlation, no game-specific heuristics. There are none in the tree — every mention of
Trackmania in Go source is a comment recording where a number came from.

## Why the mouse and not the controller

The owner's product decision, which changes how Marco acts and not how Marco understands:

> Marco's first intelligent behavior should be "I saw what you clicked, I learned where it took
> me, and I can find and click that target again."

It removes three inference steps that controller navigation needs — which direction selects an
entry, how many presses move a mark, which device produced them — and replaces them with one
observation: the person pressed *there*, and *that* happened. The press is far stronger evidence
of intent than a direction is.

It does not remove the perception problem. Marco still has to identify the thing that was
clicked, tell it from its neighbours, and recognise the screen that follows.

## The route

Main menu → **Play** → **Training** → **Free Play**, and back to the main menu. Then the same
route again, with the same device.

60–90 seconds at 10 frames per second. At 90s that is about 900 frames and roughly 1.8GB, well
inside the recorder's 9000-frame / 6GB bound.

## What must be true before it is worth recording

| | state |
|---|---|
| the recorder can place a mouse press | **fixed 2026-10-12**, [[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]] — it could not, and dropped every press whole |
| the press's position reaches disk | fixed, same decision |
| the position reaches perception on replay | fixed, same decision |
| the replay says what a press landed on | built: presses seen / on a proposed region / on more than one |
| a press can resolve to a NAMED target in a game | **no.** `SemanticTarget` resolves against an accessibility index a game does not have |
| two different screens segment as two states | **no.** Segmentation reads named roles and the proposer names nothing |

The last two are the ones that decide what this recording can answer, and both are honest
limits rather than surprises — see the known gaps in [[Passive-Observation]].

## The owner's acceptance questions, and what can be answered

> - Did Marco distinguish the main menu, Play menu, Training menu, and Free Play destination?
> - Did it associate your observed inputs with visually confirmed selection changes and screen
>   activations?
> - Did it retain a stable route through the graph across both demonstrations?
> - Could it produce an inspectable, legal-Marco execution plan without inventing missing
>   transitions?

Against the table above, before the recording exists:

1. **Distinguishing the four screens: expected NO**, and for a reason that is not about Rocket
   League and not about the recording. On the pixels discovery path every screen is one vacuous
   state, so a four-screen route is four readings of `state_1`. This is the gap to close next
   and it is a decision of its own, not a widening: giving segmentation an unnamed-geometry cue
   reaches cross-session identity and durable memory.
2. **Associating presses with what was on screen: partially.** The press's position and the
   geometry of the frame it happened on are both available and comparable. Whether the press
   landed on something perception found is measured. Whether the thing it landed on is the entry
   the person meant is not yet asked.
3. **A stable route across both demonstrations: NO, while (1) holds.** A route is edges between
   recognised screens.
4. **An inspectable legal-Marco plan: only as far as (1) and (3) allow**, and it must refuse
   rather than invent. Nothing in the tree manufactures a transition, and that is the one thing
   this experiment must not relax to produce a nicer answer.

Recording it anyway is still worth doing: it is the first L0 recording with a mouse in it, the
first evidence about whether a game menu's rows are proposed where a person clicks them, and the
first unmodded game surface in the corpus. What it will not produce is a route.

## Protocol

Passive. Nothing is injected, nothing is approved, nothing is replayed.

```sh
go build -o director.exe ./cmd/director
./director.exe capture-game-session --application rocketleague \
    --session rl-freeplay-1 --seconds 90 --fps 10 --delay 8
```

- `--application` matches the **executable name, lowercased, without its path or extension**, so
  `rocketleague` and not `RocketLeague.exe`.
- Run the game **windowed or borderless**, not exclusive fullscreen, and keep every other window
  off it: a window capture is not occlusion-safe and will return the covering window's pixels
  under the target's own name. The recording counts frames taken while something else was in
  front; it cannot refuse them.
- The owner starts it. It waits `--delay` for the switch to the game.

Then, offline:

```sh
go run ./cmd/perceptionreplay -frames -stride 1 .tmp/game-capture-review/rl-freeplay-1
```

## Results

None. Not run.
