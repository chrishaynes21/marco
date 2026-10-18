---
type: experiment
status: complete
date: 2026-10-17
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1 — the same 900-frame recording Experiment 029 used, with its cached OCR readings (300 readings, stride 3). Three independent observers, no counter or identity shared. Nothing executed, nothing stored."
result: "all-four-discriminators-the-artifact-carries-fail-to-separate-a-correct-binding-from-an-incorrect-one-and-the-words-point-the-wrong-way"
supersedes: []
affects:
  - perception
source_paths:
  - docs/decisions/ADR-229-proposed-a-single-qualifying-candidate-is-not-permission.md
  - cmd/perceptionreplay/binding.go
---

# Experiment 076 — nothing the artifact carries says which screen this is

> Investigate the two confidently wrong screen bindings in Experiment 029. Establish a
> conservative, evidence-based abstention boundary. Do not claim that a numerical threshold alone
> solves the problem without testing unseen and confusingly similar screens.

[[Experiment-029-binding-a-screen-across-two-independent-folds]] established that a score floor
cannot separate right from wrong, and
[[Experiment-036-recognising-a-screen-you-were-shown]] then tried four more measures -- two
palette histograms, a per-cell one and the detector's classes -- and found that all five nominate
the two unseen screens **unopposed**, at 0.886 and 0.559 for arrangement and 1.000 and 0.996 for
palette and detector.

So five measures had already failed. This asks the remaining quantities a saved artifact carries,
and it is worth saying plainly that the expectation going in was that the WORDS would be
different in kind from the five: they are the only one that is not a function of the pixels'
layout.

## First: the baseline reproduces

`perceptionreplay -frames -bind`, pixel-only (the OCR cache absent), on the same recording:

```
7 bound: 5 CORRECT, 2 WRONG   (both to a screen fold B never saw at all)
1 ambiguous
correct bindings scored 0.556 0.611 0.688 0.833 0.901
wrong   bindings scored 0.559 0.886
```

Unchanged, fourteen months of subsystem work later. The measurement is stable.

## Then: the words, which were the obvious candidate

`Screen.Text` exists because *"the closest two screens are separated by 0.010 while the words on
them agree at 0.231"*. The fold already collected readings and `bind` never consulted them, so
this run folds them in.

**Folding text in changes which screens exist.** Fold A saw 11 instead of 8, fold B 10 instead of
8 — text separates screens the arrangement merged, which is
[[Experiment-048-the-destination-a-workflow-can-see-for-itself]]'s finding arriving from the other
side. So this is a different segmentation and not a like-for-like rerun.

On that set:

```
11 screen(s) of fold A:
    4 bound: 2 CORRECT, 2 WRONG   (both, again, to a screen fold B never saw)
    6 ambiguous
    1 correctly unbound

A          oracle      result   arr     text    words
visual_9   visual_9    WRONG    0.556  0.818   both read
visual_2   visual_2    CORRECT  0.901  0.500   both read
visual_3   visual_3    WRONG    0.886  0.692   both read
visual_10  visual_10   CORRECT  0.560  0.250   both read

of the bindings where both sides were read:
  wrong, and the words reject it    0
  wrong, and the words allow it     2
  correct, and the words reject it  0
  correct, and the words allow it   2
```

**The words point the wrong way.** Both wrong bindings have HIGHER text similarity than both
correct ones, and `observe.TextsDisagree` — a positive statement that two readings are of
different screens — rejects none of the four. The evidence that was expected to be the
discriminator is, on this data, anti-correlated with correctness.

The honest reading of why: a screen fold B never saw is being matched against a screen that is
adjacent to it in the same menu, and adjacent menu pages share most of their chrome. The words
agree because the pages really do say similar things.

## Then the margin, and the qualifier count

```
and would the MARGIN to the runner-up have separated them?
    correct bindings had gaps 0.110 0.450
    wrong   bindings had gaps 0.051 0.343
    NO. The widest WRONG margin was 0.343, wider than 1 of the 2 correct ones.
    Being unopposed is not evidence of being right.

how many candidates qualified, for each binding that was made:
    visual_9   WRONG    1 qualified of 10 offered
    visual_2   CORRECT  1 qualified of 10 offered
    visual_3   WRONG    1 qualified of 10 offered
    visual_10  CORRECT  1 qualified of 10 offered
```

The qualifier count is the sharpest of the four, because it is the rule `BindScreens` already
acts on. **All four bindings — right and wrong alike — were the only candidate the evidence
supported, out of ten offered.** "The only candidate" is not a distinguishing fact.

## The result, in the terms that were asked for

> We should also avoid automatically turning every saved workflow into a series of per-step
> confirmation prompts. […] And we should measure wrong actions, not just how often Marco finds
> a candidate.

Under the existing abstention — `ambiguous` refuses, `no-counterpart` refuses — a pixel replay
would have **acted on 4 of 11 screens and been wrong on 2**. Two wrong actions out of four
attempts. Not two out of eleven: the refusals are not successes, and counting them as such is
the hit-rate framing the owner's sentence rules out.

| discriminator | separates right from wrong? |
|---|---|
| a higher score floor | no |
| what the screens say | **no, and inverted** |
| the margin to the runner-up | no |
| how many candidates qualified | no — all four had exactly one |

The one thing that does separate them is not available at run time: every wrong binding, in both
runs, was to a screen the other fold had never seen.

With Experiment 036's five, that makes **seven measures** that fail on the same two bindings --
four of them appearance, one structural, one lexical and one about the shape of the candidate set.
That is no longer a gap in the measures. It is a statement about what the artifact contains.

## What follows

Not a fifth term. [[ADR-229-proposed-a-single-qualifying-candidate-is-not-permission]] states it:
the artifact does not contain the answer, so the next piece of work is to stop asking the
artifact. The desktop replay in
[[Experiment-075-a-demonstration-saved-named-and-replayed]] is honest for a reason that has
nothing to do with screen identity — it reacquires each control by name against a fresh
observation and says *nothing matching "Gaming" is present* when it cannot. A pixel replay has no
such sentence available, because its controls are regions and not names.

`Control.Text` is the field that would give it one. It is now written for a desktop
demonstration and still empty for a frame recording, and filling it is measurable in the same
harness.

## Reproducing

`go run ./cmd/perceptionreplay -frames -bind .tmp/game-capture-review/rl-freeplay-1`. The
readings come from `words.jsonl` beside the recording; deleting that file reproduces the
pixel-only baseline, and the report says `words: none` when it does.

## Enforced by

`cmd/perceptionreplay` `TestTheBindingFoldsShareNothing` — the independence the whole measurement
rests on — and `TestEveryBindingDiscriminatorIsReported`, which holds that the report names all
four discriminators, because a negative result that quietly stopped printing one of them would
read as a result about the other three.
