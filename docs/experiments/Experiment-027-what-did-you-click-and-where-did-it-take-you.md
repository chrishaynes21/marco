---
type: experiment
status: complete
date: 2026-10-12
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: every-press-landed-on-something-marco-found-and-no-screen-was-distinguished
supersedes: []
source_paths:
  - cmd/director/capturegamesession.go
  - cmd/perceptionreplay/frames.go
  - internal/platform/navsource/navsource.go
---

# Experiment 027 — what did you click, and where did it take you

**Status: RUN once, 2026-10-12, with the owner's authorization and at their instruction.**
900 frames, 90 seconds, Rocket League, the route demonstrated twice with the mouse. Nothing was
injected and nothing was replayed.

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
    --session rl-freeplay-1 --seconds 90 --fps 10 --delay 8s
```

`--delay` is a DURATION. `--delay 8` is a parse error that prints usage and records nothing;
this cost one false start.

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

### The recording itself is clean

| | |
|---|---|
| frames | 900, none dropped, no capture failures |
| rate | **10.0 fps measured** against 10 requested |
| frames taken while another window was in front | 0 |
| inputs | 19 — 17 mouse presses, 2 keyboard `pause` |
| presses refused as unplaceable | 0 |

The 17 presses carry positions, which is the thing that was impossible a day earlier
([[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]]).

### The route is visible in the positions, and it repeats

Eight presses per demonstration, same order, same places, with a keyboard `pause` at the same
point in each:

| run 1 | run 2 |
|---|---|
| 7.2s (0.065, 0.444) | 53.8s (0.083, 0.462) |
| 10.8s (0.251, 0.557) | 56.7s (0.250, 0.606) |
| 13.8s (0.241, 0.430) | 59.6s (0.277, 0.418) |
| 16.8s (0.249, 0.512) | 61.9s (0.272, 0.494) |
| 19.6s (0.421, 0.644) | 63.9s (0.455, 0.647) |
| 24.0s `pause` keyboard | 69.1s `pause` keyboard |
| 30.2s (0.492, 0.586) | 73.3s (0.496, 0.577) |
| 33.4s (0.467, 0.551) | 75.7s (0.474, 0.569) |
| 39.4s (0.107, 0.581) | 83.2s (0.148, 0.594) |

The pairing is at the level of coordinates, which is **not** evidence that Marco can repeat the
route. It is evidence that the demonstration was repeated, which is the input the method needs.

### Every press landed on something Marco found in the pixels

```
pointer presses     17, 17 landed on a proposed region (100%), 5 inside more than one
proposals           217403 total, 241.6 per frame
grouping            745 of 900 frames produced a stack of >=3 (83%)
```

**17 of 17.** This is the result the experiment existed for. A press inside a region the
model-free proposer found can be re-grounded later by finding that region again; a press on bare
background can only ever be replayed as a coordinate. On an unmodded game menu, with no
accessibility tree and no learned detector, every click the person made was on something the
pixels alone offered.

Five of the seventeen were inside more than one region, so the choice the live pointer path
makes — smallest containing control wins — is a choice that will actually have to be made here,
on about a third of presses.

### And no screen was distinguished from any other

```
screen model        1 states, 0 transitions
moves retained      0 distinct, 0 observations
durable edges       0 offered, 0 session-local
belief              unknown, selected -1, reading insufficient_evidence
because             no list has been seen handing a mark between its members
                    (115 candidates watched over 898 frames)
```

Predicted, and predicted for the right reason: segmentation builds its signature from NAMED
roles and the structure proposer names nothing, so four screens are four readings of `state_1`.

### The finding that was NOT predicted

> no list has been seen handing a mark between its members

The entire list-identification mechanism is keyed to **a mark moving between entries under
directional input** — [[ADR-158-the-active-list-is-the-one-that-responds]]. Mouse navigation has
no such mark. The cursor moves, the highlight follows it or there is none, and nothing ever
hands a mark along a list.

So on a mouse demonstration this machinery can never qualify a list, and that is not a defect to
fix. It means the owner's product decision also changes which half of perception the first
usable version depends on. The dependency chain for click-target navigation is:

| | state |
|---|---|
| the press lands on a region the proposer found | **works, 17/17** |
| that region is identifiable again on a later frame | partly — candidate tracking exists and is list-oriented |
| the screen after the click is distinguishable from the screen before | **no** |

The middle rung is where the next work is, and the bottom rung is what blocks a route.

## The owner's four acceptance questions, answered

1. **"Did Marco distinguish the main menu, Play menu, Training menu, and Free Play
   destination?" NO.** One state, zero transitions, for the structural reason above. Not a
   Rocket League problem and not a recording problem.
2. **"Did it associate your observed inputs with visually confirmed selection changes and screen
   activations?" PARTLY — the press half only.** Every press is placed in the same coordinate
   space as the geometry of its own frame and every one landed on a proposed region. No selection
   change was confirmed, because no mark moves under a mouse, and no activation was confirmed,
   because no screen change was detected.
3. **"Did it retain a stable route through the graph across both demonstrations?" NO.** A route
   is edges between recognised screens, and no screen was recognised. The repetition is present
   in the evidence and unused.
4. **"Could it produce an inspectable, legal-Marco execution plan without inventing missing
   transitions?" It produced NOTHING, which is the correct failure.** `durable edges 0 offered`
   and the replay says why in its own words: *a durable edge needs both endpoints recognised, and
   nothing here is.* No transition was manufactured from the input events, which were right there
   and would have made a much nicer-looking report.

## What this cost, and what it bought

It also exposed a defect in the recorder's honesty that the owner caught before the analysis did:
**17 presses was fewer than they had clicked, and the recording could not explain the
difference.** Two of the four causes were counted nowhere at all. Fixed in
[[ADR-169-a-recording-that-could-not-explain-its-own-total]], along with `controller: observed`
appearing in a session with no controller in it, and a press made during the final frame being
thrown away with the subscription that held it.

How many presses this session actually lost is **not known and cannot now be established** — the
counters did not exist when it was made. The next recording will say.

