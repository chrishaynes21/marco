---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "Four live human demonstrations against Ark: Survival Ascended (94.34.0), 1920x1080, on the owner's own desktop with their hands on the mouse. Three saved artifacts, three live execution attempts with `perform-workflow --name`."
result: "The demonstration half works on a real game: a person's click was recorded with provenance, scoped to the watched window, saved under their own name, and offered to the execution path. No press was ever emitted. Two defects were found and fixed — a session that ended before it began without a terminal, and a correlator that discarded the outcome of any press that CHANGED the screen. The remaining blocker is neither: Ark's fold mints four screens over 43 looks of one static settings menu, and three of the four are read as nothing, so a saved screen cannot be re-established at replay."
supersedes: []
affects:
  - perception
  - learning
  - execution
source_paths:
  - internal/director/observe/outcome.go
  - cmd/director/demonstratevisual.go
  - docs/decisions/ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it.md
---

# Experiment 091 — four demonstrations, and no press

## The question

[[Experiment-090-the-first-guarded-pixel-action]] proved execution on a surface this project
controls. [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]] joined the two
halves. The owner set the next milestone:

> A person clicks a menu control in a real game. Marco records it, saves it, and later performs
> that same action while verifying the outcome. That's the first genuinely human-taught pixel
> macro.

Ark was the game that happened to be open.

## What worked

The demonstration half, on a real game, from a real hand:

	look 103  48.4%,35.9% on screen_a     control_1, one of 2 regions containing it

A click recorded with its coordinate as evidence of what the person did, a control at 8%x3%
matching the GRAPHICS tab the offline capture measured at 7.2%x2.8%, an artifact scoped to
`arkascended` from the window that was WATCHED, saved under the name the person gave it, and
offered by `director sequences` to `perform-workflow --name`. Everything ADR-239 built carried
a real demonstration end to end.

## Two defects, found because a person was sitting there

**A session with no terminal ended before it began.** `demonstrate-visual` offers *Press ENTER
when you are done*, and the reader for that ENTER ended the session on any error from
`ReadString`. EOF is an error, and EOF is what a shell with no console returns immediately. The
report:

	0 screen(s) seen, 0 visited in order, 0 demonstrated control(s)
	words: offered 0, read 0, dropped while busy 0, 0 word(s)

which reads as a perception failure and is a closed stdin. Two of the owner's clicks into a
live game were spent before anybody looked at the reader. A session nobody can end by hand now
runs to its `--seconds` ceiling, which is what that ceiling is for.

**And the correlator discarded the outcome of any press that changed the screen.**

	if w.screen != screen || len(w.after) >= OutcomeSettleLooks {
		continue
	}

Every reading of the panel a press PRODUCED was thrown away for being a reading of what the
press produced. Invisible offline for a measured reason: on `ark-menu-1` the GRAPHICS tab
changes the panel and not the global screen
([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]), so the watch's screen still
matched and 36 words were recorded. Live, on the same game and the same control, the panel
changed enough to mint a new screen — `visual_1 -> visual_2` — and the same press recorded
nothing at all. Three demonstrations produced three artifacts with no expectation.

This is the mirror of the run-side defect fixed the same day in
[[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]. The rule now is *where the press
landed, or the one place it led* — a third screen closes the watch, which is the part of the old
scoping that was right.

## Three refusals, every one correct

| state | what Marco did |
|---|---|
| an inventory screen was open | no proposal agreed with the saved control — refused, nothing clicked |
| the GRAPHICS tab was already selected | the destination was already standing, so there was nothing to do |
| the screen could not be named | stopped rather than guessing |

`real input sent: NO` in all three. The owner's requirement — *if a target cannot be grounded,
refuse without clicking* — held on a real game in three different states, which is worth more
than it sounds: two of those three are states nobody designed a test for.

## The blocker, measured

	the screens this run saw   4 screen(s) over 43 look(s)
	  visual_1   1 look(s)   nobody read it
	  visual_2   2 look(s)   nobody read it
	  visual_3  20 look(s)   read, and no word was there twice
	  visual_4   3 look(s)   nobody read it

That is ONE static settings menu. The fold mints a new screen every few looks and OCR reads
almost nothing usable — offline, the same screen yields 38 stable words. A screen established at
demonstration time therefore cannot be recognised at replay, and everything downstream of
establishment is unreachable: grounding, pressing, verifying.

`--from` cannot bridge it, and the reason is written into the code it would have to change:

	What keeps it honest is that it is spent ONCE, on the first observation it can be spent in
	[…] A claim about one screen, not a standing answer.

Turning a person's single answer into a standing one would trade a documented property of
[[ADR-186-a-person-can-say-where-they-are]] for a workaround to a perception instability. It was
not done.

## What this says about where the product is

The execution path is proven on a surface whose fold holds still. The demonstration path is
proven on Ark. What is not proven is the two of them meeting on a game whose screen identity
moves under them — and that is a perception-stability question about one surface, not an
integration gap. Ark's settings panel is opaque and still measured four screens in four seconds;
the live-background pause menu measured worse, changing "by unknown" ten times in thirty.

## Cost to the person

Four demonstrations, three of them re-demonstrations, for zero presses. Two were spent on the
stdin defect and one on a demonstration whose clicks landed in another window while they read
instructions. The owner's standing instruction is the right measure of that:

> Do not require the user to repeat a demonstration simply because the system discarded evidence
> it should have retained.

Two of the four were exactly that.

## Enforced by

`cmd/director` `TestASessionNobodyCanEndRunsToItsCeiling` and `TestSomebodySayingSoStillEndsIt`,
over a seam extracted for them: the reader was inline, which is why nothing could test it.

`internal/director/observe` `TestAPressThatChangedTheScreenStillHasAnOutcome`,
`TestAThirdScreenIsNotThisPressesOutcome`,
`TestATransitionInsideTheWindowRestartsItWhileDemonstrating` and
`TestTheReadingsTakenMidResponseAreSkippedWhileDemonstrating` — the last two added after a
mutation run found the restart and the skip surviving on the demonstration side, having been
copied from the run side without tests behind them, which is the gap that let the original
defect live.

## See also

- [[Experiment-090-the-first-guarded-pixel-action]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
