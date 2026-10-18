---
type: adr
status: proposed
date: 2026-10-17
decision: PROPOSED — a saved screen matched by exactly one qualifying candidate must not be treated as recognised, because no quantity the artifact carries separates a correct binding from an incorrect one; the abstention boundary cannot be a threshold, a margin, a word overlap or a qualifier count, and must come from outside the artifact — a person stating that the screen is there, or a replay that grounds per control instead of per screen
supersedes: []
affects:
  - perception
source_paths:
  - cmd/perceptionreplay/binding.go
  - internal/director/workflowstore/workflowstore.go
  - internal/director/observe/screentext.go
---

# ADR-229 — proposed: a single qualifying candidate is not permission

## What was asked

> Experiment 029 found two confidently incorrect screen bindings out of ten. That's more
> concerning than missing macro naming. […] I'd require the workflow replay system to
> distinguish: a screen it recognizes with sufficient evidence; a screen that may match but needs
> more evidence; a screen it cannot identify. […] Do not claim that a numerical threshold alone
> solves the problem without testing unseen and confusingly similar screens.

The three-way distinction already exists. `workflowstore.BindScreens` answers `bound`,
`ambiguous` or `no-counterpart`, refuses to break a tie, and says in its own doc that *agreement
is necessary and not sufficient*. So the open question was the one the owner put in the last
sentence: what evidence moves a screen from the middle state to the first one.

## What was measured

[[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] re-ran Experiment 029's
fold split and asked four more candidate discriminators, all computed from what a saved artifact
already carries. [[Experiment-036-recognising-a-screen-you-were-shown]] had already ruled out
four appearance measures beside the arrangement, all five of which nominate the two unseen
screens unopposed. Every one of the new four fails too:

| discriminator | result |
|---|---|
| a higher score floor | **no** — the best wrong binding scored 0.886, above 4 of 5 correct ones |
| what the screens SAY (`TextSimilarity`, `TextsDisagree`) | **no, and inverted** — the wrong bindings' words agreed MORE (0.818, 0.692) than the correct ones' (0.500, 0.250), and `TextsDisagree` rejected none of the four |
| the margin to the runner-up | **no** — the widest wrong margin was 0.343, wider than a correct one at 0.110 |
| how many candidates qualified | **no** — all four bindings had exactly one qualifier of ten offered, correct and incorrect alike |

The one thing that did separate them is not available at run time: both wrong bindings, in both
runs, were to a screen the other fold **had never seen**. The only right answer was to refuse,
and nothing in the evidence says so.

## The proposal

**A single qualifying candidate is validation, not recognition, and must not be read as
permission to act.** Concretely:

1. `BindBound` keeps its current meaning — *the only candidate the stored arrangement supports* —
   and that meaning is already correct. What must change is nowhere in `workflowstore`: it is
   that no caller may treat it as "this is the screen".
2. The move from "may match" to "recognised" requires evidence **independent of the artifact**.
   Two kinds exist, and the measurement says to prefer the second:
   - **a person stating the screen is there.** `workflowstore/anchor.go` already implements it,
     valid for one observation generation, and
     [[Experiment-036-recognising-a-screen-you-were-shown]] already said why: *what a person adds
     is not a better ranking — it is the one thing no measure supplied, a statement that the
     screen is THERE*.
   - **grounding per CONTROL instead of per screen.** Which is what the desktop replay does, and
     what made the live negative tests in
     [[Experiment-075-a-demonstration-saved-named-and-replayed]] honest: a saved sequence
     reacquires each control by name against a fresh observation and fails at the step that
     cannot be found, with the reason. It never asks which screen it is standing on.
3. **Measure wrong ACTIONS, not candidates found.** Under the existing abstention the pixel path
   would have acted on 4 of 11 screens and been wrong on 2 — *two wrong actions out of four
   attempts*. The report now prints that rather than a hit rate.

## Why nothing was implemented

Because the measurement says a rule would be the wrong kind of answer, and the owner said so
first. Adding a term, a margin floor or a qualifier rule would each produce a system that is
usually in the right place and occasionally certain it is somewhere it has never been — with a
new number to tune and the same error. The finding that matters is that **the artifact does not
contain the answer**, and the next piece of work is to stop asking the artifact: write
`Control.Text` from a demonstration, so a pixel replay can refuse per control the way the desktop
replay already does.

`Control.Text` is now written for a desktop demonstration
([[ADR-228-a-named-sequence-is-a-demonstration-somebody-kept]]). It is still empty for a frame
recording, and filling it is the measurable next step rather than another scoring rule.

## Enforced by

`cmd/perceptionreplay` `TestTheBindingFoldsShareNothing` (unchanged — the independence the whole
measurement rests on) and `TestEveryBindingDiscriminatorIsReported`, which holds that the report
names all four and does not silently drop one.
