---
type: experiment
status: complete
date: 2026-08-11
backend:
  - accessibility
game: none — ordinary desktop software
fixture: live capture, 100s, one File Explorer window, 40 samples
result: two-level-identity-confirmed
supersedes: []
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/hypothesis.go
  - cmd/director/playbill.go
  - pkg/playbill/narrate.go
---

# Experiment 011 — the two-level identity, in front of real software

## Question

Five milestones of deterministic work rested on one live measurement and one deterministic
prediction. The measurement was a failure: 141 samples of VS Code produced **one screen and zero
transitions** while the person driving it went somewhere and came back, twice. The prediction was
that a two-level identity — *is this the same application surface* asked separately from *is this
the same place inside it* — would fix that without turning ordinary use into a transition storm.

Both halves had to be true at once, and no fixture can settle it. Fixtures were written by the
same person who wrote the model.

> Does an ordinary, unfamiliar application produce structural evidence the repaired model reads
> correctly — meaningful local changes becoming places, and ordinary activity not?

## Method

One accessible application, deliberately **not** VS Code and not a game: a single File Explorer
window, pinned by window id. 100 seconds, 1 s requested interval, accessibility only, no
structural detector. Three qualitatively different changes, each held still for about ten
seconds afterwards:

1. navigate to a different folder — substantial primary-content replacement;
2. toggle the Details/Preview pane — a side-panel change;
3. open a right-click context menu and leave it up — a modal/menu-like overlay.

No code was changed during the run. No threshold was touched. Nothing was tuned afterwards. The
session emitted no input; `observe-game` cannot.

## Result

```
VS Code, 141 samples — the failure that opened the investigation
  1 screen    0 transitions
  identity: 141 same (weakest 0.759, mean 0.928)   0 other

File Explorer, 40 samples — this experiment
  4 screens   7 transitions
  identity:          33 same (weakest 0.951, mean 0.989)   6 other (strongest 0.833)
  within a screen:   40 compared (weakest 0.000, mean 0.880)   6 part(s) replaced
```

Watch: *"Part of the screen changed, in the same application 7 times. I saw you do something
before 1 of them — I can't say it caused them."*

### The load-bearing number

The six frames read as a **different place** scored **0.833** at their strongest on the
whole-surface comparison — far above its 0.55 threshold. Every one of them would have been "the
same screen, nothing happened" under the single-level model, which is precisely the VS Code
failure. They were separated by the local comparison, and `6 part(s) replaced` matches the six
exactly.

This is the clearest possible demonstration that the split is **load-bearing rather than
diagnostic**: the second question, and only the second question, produced every transition in the
session.

### Stability was not traded away

33 frames read as the same place at weakest **0.951**, mean 0.989 — considerably tighter than VS
Code's 0.759 weakest. Ordinary activity inside a folder barely moved the whole-surface figure. No
transition storm, and no `OVERLAP` warning: no frame read as the same place scored worse than one
read as different, so the two populations separated cleanly. That is the property no single
global threshold could deliver — see [[ADR-039-a-surface-and-a-place-inside-it]].

### What it found

Four places, three substantial enough to describe: screens dominated by **7**, **9** and **24**
grouped controls. The 7-control screen also read the concept `settings` in 9 of its 9
observations. Weakest local similarity **0.000** — one region was replaced outright, which is the
folder navigation.

Every hypothesis is `[contested]` with *"seen in only one visit"*. Correct and expected for a
single pass; a second visit is what settles them.

## What this does NOT establish

- **Attribution of each transition to each action.** Four places for a starting state plus three
  changes is consistent, and the 24-control screen is plausibly the context menu, but the output
  does not attribute them and this record does not guess.
- **The same-kind overlay limit.** It never arose. Explorer's panels and menus are built from
  structurally distinct roles, so the blind spot recorded in
  [[ADR-040-a-few-scales-were-not-better-than-one]] was not exercised and remains unmeasured
  against real software.
- **Causation.** Only 1 of 7 changes had navigation observed before it. Honest rather than
  broken: Explorer is driven by mouse and menus, and the navigation vocabulary is
  keyboard-meaning-shaped. Watch says "I can't say it caused them" and means it.

## The finding that decides what happens next

**Nothing in this session could ever have earned an invitation to learn, and the reason is not
the screen model.**

`DefaultLearningThresholds` requires that at most **half** the observations of a habit have no
navigation before them. This session attributed **1 of 7**. That is not a near miss; it is not
close.

The cause is upstream of everything this milestone touched: `navsource` installs a
`WH_KEYBOARD_LL` hook and nothing else. `NavPoint` — "a pointer press" — is defined in the
vocabulary, consumed by lowering and by assessment, and **produced by no live source**. Mouse
clicks are invisible to observation.

File Explorer is driven by mouse. So is most desktop software. The one attributed change was
almost certainly a keyboard action.

This does not weaken the result above: the screen model saw every change correctly, which is what
the experiment asked. It does mean the *learning* path, as wired today, can only be reached by
driving an application from the keyboard. Anything else produces a perfect record of screens
changing with nothing to say about how the person did it — and Marco correctly declines to offer
to learn "how you do that" when it did not see how.

## Two findings worth acting on

**Sampling cadence is not met.** 41 late samples against 40 taken, at a 1000 ms requested
interval. A 269-element accessibility tree costs more than a second per observation cycle. Not a
correctness problem — every sample was used and every provider proved its target — but nothing
should be built that depends on the requested interval being honoured.

**`observe-game --application` fails slowly on an ambiguous target.** The first attempt selected
`explorer`, which had twenty windows; it ran for 21 seconds and ended `target_unavailable` with
zero samples, reporting the ambiguity only in the failure text. The ambiguity is detectable at
start. Refusing immediately would have cost nothing and saved a whole run.

## Conclusion

The repaired model reads real software the way the deterministic corpus predicted. Meaningful
local change becomes a place; ordinary use does not; the enclosing surface stays stable
throughout. Sight needs no further work on this evidence.

`LOCAL_STATE_MIGRATION: COMPLETE` is confirmed outside its own fixtures.

## Related

[[ADR-039-a-surface-and-a-place-inside-it]] ·
[[ADR-040-a-few-scales-were-not-better-than-one]] ·
[[ADR-041-a-screen-is-not-its-dominant-group]] ·
[[Passive-Observation]] · [[Visibility]]
