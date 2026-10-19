---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "Ark: Survival Ascended (94.34.0) at 1920x1080, on the owner's own desktop, with their hand on the mouse for the demonstration and on F7 for every authorisation. One demonstration, four live runs."
result: "The full acceptance sequence, on a real game. A person clicked one control; Marco recorded it with provenance, scoped it to the window it watched, saved it under their name with a 61-word outcome expectation, and later re-grounded that control from fresh pixels, took per-step and per-press authorisation, pressed it through legal Marco, and VERIFIED the result on 25 independent words that stood across a full settled window. Repeated on the unchanged artifact: verified again on 24. Attempted from the pause menu: refused with nothing pressed and no authorisation even requested. Two defects were fixed to get there and both were found by the owner's own failed attempts."
supersedes: []
affects:
  - learning
  - execution
  - perception
source_paths:
  - internal/director/workflowrun/workflowrun.go
  - cmd/director/performworkflow.go
  - docs/experiments/Experiment-091-four-demonstrations-and-no-press.md
---

# Experiment 092 — the first human-taught pixel macro

## What was asked for

> A person clicks a menu control in a real game. Marco records it, saves it, and later performs
> that same action while verifying the outcome. That's the first genuinely human-taught pixel
> macro.

[[Experiment-091-four-demonstrations-and-no-press]] got as far as a saved demonstration and no
press. This is the rest of it.

## The sequence, and what each step rested on

| step | evidence |
|---|---|
| a person demonstrates | `look 107  44.8%,26.8% on screen_c  control_2` — the press, with its coordinate, as the record of what they did |
| Marco captures and saves | scoped `arkascended` from the window WATCHED, named `ark graphics live`, with a **61-word outcome expectation** |
| the menu is reset | by hand, to a tab other than the one demonstrated |
| invoked by name | `perform-workflow --name "ark graphics live" --step 2`, no edit to the artifact |
| re-grounded | `1816x54 px at 52,248, geometrically re-acquired, reading "AUDIO GRAPHICS Ul oD"` — the same tab strip `ark-menu-1` measured offline |
| authorised | `steps agreed to by key 1 of 1`, `presses permitted by key 1 of 1`, F7 each time |
| executed | `real input sent YES`, pressed at (960,275), pointer confirmed there |
| verified | **25 of the 61 expected words appeared where the control is not and stayed for 5 readings** |

The words: `back bloom brightness cinematics decals enable enhancement foliage footstep high
interaction light low medium off post processing quality settings shadows shafts view`. The
GRAPHICS panel, and the same vocabulary the offline capture measured for that control.

**And the screen never changed.** `the screen changed after the action — NO`,
`destinations established 0 of 1`. The step finished on its own outcome, which is the entire
reason the local verifier exists ([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]).

## The other two cases

**From the pause menu — refused, and not by failing.** `real input sent NO`,
`actions lowered, compiled and run 0`, `steps agreed to by key 0 of 0`. It never asked for
authorisation: *no proposal on this look agrees with any reading of this control*. The screen
read `exit · main · menu · mod · photo · settings · toggle`, which is the pause menu, and the
control is on the settings screen.

**Reset and repeated, unchanged artifact — verified again**, on 24 words rather than 25. The
difference is OCR: `follege`, `medum`, `partidas`. Two runs of a real measurement rather than
one replayed twice.

Two VERIFIED, one REFUSED, zero CONTRADICTED, zero UNCONFIRMED, zero incorrect actions.

## Two defects, both found by the attempts that failed

**`--from` was spent on a fold nobody was looking at.** The claim is about where the person is
standing, and it was spent on the FIRST observation whatever that was:

	visual_1    4 look(s)   nobody read it
	visual_2  136 look(s)   ...

spent on `visual_1`, and the screen they were actually on could never be named for the remaining
200 observations. It now waits for the same screen twice. Still one answer about one screen,
spent once and never re-offered — only no longer spent on a screen nobody was looking at.

**And an acted step could not be answered unless its screen could be named.** The verdict lived
in section 4a; the grounder returns at section 2, two hundred lines earlier, on the first look
whose screen nobody can name. So the run reported

	1 action(s) were performed and no post-action evidence was collected about any of them

while holding 25 readings taken after the press. The question was never asked. This is the
ordinary case rather than a corner: a press that changes a panel changes what the screen LOOKS
like, which is exactly when the fold stops recognising it — the verifier was unreachable in the
case it was built for. An acted step is now answered from readings of the window, which do not
require a nameable screen.

## And two labels that contradicted the verdict

A summary is evidence too. `step 1: complete` on a one-step workflow named a step that does not
exist, because the cursor had moved past the last one. Worse,

	the screen changed after the action    NO — the application did not visibly respond

printed directly above a verified outcome listing 25 words of the panel that had just opened.
The two lines answer different questions — global screen identity, and what appeared on it — and
read together the summary denied its own verdict. Somebody skimming would have concluded nothing
happened.

## What this does and does not establish

It establishes the chain end to end on a real game: demonstration, provenance, scope, naming,
re-grounding from fresh evidence, authorisation, legal execution, and verification — twice, with
a correct refusal between them.

It does not establish independence. `actions independently verified — NOT ESTABLISHED` is still
printed and is still right: the confirmation that the panel opened is Marco's reading plus **the
owner watching it happen**. There was no witness outside Marco of the kind the controlled menu
provided ([[Experiment-090-the-first-guarded-pixel-action]]). Human observation is what stands
behind this result, and it should be recorded as that rather than as instrumentation.

Nor does it generalise to Ark. The settings screen held perfectly still — 46 looks, one screen,
43 settled — while the pause menu over live 3D folded `visual_1 -> visual_2` nine times in 267
looks. The qualification answer is per-surface and this is one surface.

## Enforced by

`internal/director/workflowrun` `TestAnActedStepIsAnsweredEvenWhenNobodyCanNameTheScreen`, whose
grounder stops naming the screen the moment a press goes out, and `cmd/director`
`TestDiscoveryWaitsForThePersonToSwitchAndNamingDoesNot`.

## See also

- [[Experiment-091-four-demonstrations-and-no-press]]
- [[Experiment-090-the-first-guarded-pixel-action]]
- [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
