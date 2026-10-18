---
type: experiment
status: complete
date: 2026-10-17
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1 — 900 frames with real human presses, folded once, scored two ways in the same pass. Plus a live session on Windows Settings through the production command. Nothing executed, nothing injected."
result: "dropping-the-screen-gained-no-match-rate-and-cost-3323-collisions-and-2470-matches-on-screens-the-control-was-never-demonstrated-on"
supersedes: []
affects:
  - perception
source_paths:
  - docs/decisions/ADR-230-a-demonstration-from-pixels-and-what-the-screen-was-doing.md
  - cmd/perceptionreplay/reacquire.go
  - cmd/director/demonstratevisual.go
---

# Experiment 077 — the screen was not supporting context

> Make individual learned controls and transitions the basis for game-menu replay. Use
> whole-screen matching as supporting context, not proof of identity.
> Do not assume local matching is reliable merely because global screen matching failed.

The second sentence is the one that earned its place. The hypothesis was reasonable and the
measurement says it is the wrong way round.

## Method

One fold of `rl-freeplay-1` through the production observer, scored twice in the same pass, so a
difference between the two tallies is the screen and nothing else:

- **scoped** — `obs.Reacquire`, which asks "of the controls on the screen I believe I am standing
  on, which can I find". The ADR-180 measurement, unchanged.
- **free** — `observe.ReacquireStored` over EVERY demonstrated target on every look, with the
  screen dropped. What a replay grounded per control would see.

Both exclude the look a target was pressed on, both compute reacquisition before the look is
folded, and both take their ground truth from second presses.

## Result

| | scoped by screen | no screen |
|---|---|---|
| target-looks | 1287 | 6594 |
| matched | 674 — 52.4% | 3144 — 47.7% |
| ambiguous | 163 — 12.7% | 634 — 9.6% |
| unobservable | 450 — 35.0% | 2816 — 42.7% |
| indistinct from a container | 27 | 30 |
| ground truth, second presses | 3 verified, 0 contradicted, 2 declined | 3 verified, 0 contradicted, 2 declined |
| **colliding matches** | **0 over 0 looks** | **3323 over 616 looks** |
| **matched on a screen it was never demonstrated on** | — | **2470 — 78.6% of all matches** |

Dropping the screen bought **nothing**: the match rate went slightly down. It cost 3323 pairs of
controls claiming one rectangle on the same look, and 2470 matches — nearly four in five — made
on a screen the control had never been seen on. Every one of those is a look on which a replay
looking for that control alone would have been prepared to press something that is not there, and
unlike a collision it takes only one control to be wrong.

The per-target column shows it is not one outlier. `target_3` matched 632 times and 593 of those
were off its own screen; `target_12` matched 133 times with 127 off-screen; `target_10`, 268 with
263.

## Why, and it should have been predictable

A game menu is a column of near-identical rectangles. `sameRect` is mutual centre containment —
threshold-free and robust to a proposer reading the same row slightly larger, and deliberately
not an overlap fraction, because this subsystem has a measured history of invented ratios being
wrong. It cannot distinguish the third row of one menu from the third row of another, because
geometrically they ARE the same rectangle.

So the screen was never supporting context. **It was the discriminator, and the control match was
the weaker half of the pair.** [[Experiment-035-what-a-control-looks-like]] already said the
adjacent thing at control scale — *the descriptor reads a state, not an identity* — and this is
the same finding one level up.

## What this does and does not license

It does **not** say screen identity is sound. [[Experiment-029-binding-a-screen-across-two-independent-folds]]
and [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] stand: seven
measures fail to separate a correct global screen binding from an incorrect one, and under the
existing abstention a pixel replay would have acted on 4 of 11 screens and been wrong on 2.

Two things fail alone, then. Similarity alone fails (076). The control alone fails worse (this).
And the rule the design already states has never been measured —
[[ADR-186-a-person-can-say-where-they-are]]:

> Expectation supplies presence; similarity supplies agreement. Neither alone is admitted:
> without the transition this is Experiment-029's two wrong bindings; without the agreement it is
> arriving somewhere and calling it arriving where you meant.

A replay at step three expects the screen step two led to. That expectation comes from the SAVED
ORDER, not from a global classifier, and it narrows the candidate set to the controls of one step
before any pixel is compared. **That conjunction is the next measurement**, and the recording has
the second fold to make it on. It is a measurement rather than a rule, and it is the piece of
work this experiment hands over.

## The live half

The production command, on Windows Settings:

```
director demonstrate-visual "settings check" --window-title Settings --seconds 15 --dry-run
  Watching applicationframehost window, generation 1, at 10/second for 15s
  settle rule 3 consecutive looks, derived from the 100ms interval
  1 screen(s) seen, 1 visited in order, 0 demonstrated control(s)
  presses: 0 of 0 classified
  nothing was demonstrated — no press landed on a settled screen
```

The loop runs live, the screen settles in three looks, and a session that learned nothing says so
and writes nothing. Two other things the live runs established:

- **A second-monitor window produced 0 screens and 9 unplaceable presses.** The Notepad window
  sat at `+-1790+2` and every press of the session was outside it, refused as
  `unplaceable_pointer` — which is correct, not a defect: a press somewhere else is about
  something else. Worth recording because `0 screens, 9 unplaceable` looks exactly like a broken
  capture and is not.
- **`--application` alone is ambiguous on a real desktop.** Both `notepad` and
  `applicationframehost` own several windows, so both commands take `--window-title` too —
  exactly one of the two, because `Selector.Validate` requires exactly one.

And the one thing the live half cannot do: **Marco cannot demonstrate to itself.** `navsource`
discards injected presses at the callback as not coming from a hand, so a session driven by
Marco's own clicks has zero attributed presses by construction. A pixel demonstration needs the
person's hands.

## Reproducing

```
go run ./cmd/perceptionreplay -frames -reacquire .tmp/game-capture-review/rl-freeplay-1
```

Both tallies print, scoped first and free second, from the one pass.

## The mutation run

Fourteen mutations across the two limits, the screen-independent view, the demonstrating command
and the report. All fourteen killed, after one round — and the two survivors were the same gap,
worth recording because it is the class this repository keeps finding.

`Runs` travels three hops: the artifact's requested count to the procedure, the procedure to the
expansion, the expansion to the program. The first test built a `program.Program` directly and
asserted the two limits on it, so deleting either of the first two hops changed nothing — a test
that asserts a LIMIT and not the WIRING that reaches it. `TestABoundedRepeatDeclaresItsRunsToThe
Program` now goes through `ExpandRequest` and checks the program's own `Runs` and `Body()`,
which is the eight-runs case that failed live.
