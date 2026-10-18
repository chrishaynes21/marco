---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "`ark-menu-1` — the owner's own 300-frame Ark capture, the same recording [[Experiment-085-the-graphics-tab-is-a-button]] used. Asked three ways, none of which presses anything: `perceptionreplay -frames -readiness`, `-frames -workflow`, `-frames -semantic`."
result: "The demonstrated GRAPHICS click produces NO STEP, because Ark's VIDEO panel and GRAPHICS panel are one screen under both the pixel fold and the text fold — 2 screens over 300 looks, one visit each. So three of the owner's four replay criteria are met on this press and the fourth is not: there is no destination screen, no transition and nothing to verify after acting. The verdict is DO NOT CLICK, and the missing evidence is a second visit rather than a better detector."
supersedes: []
affects:
  - perception
  - learning
source_paths:
  - cmd/perceptionreplay/readiness.go
  - cmd/perceptionreplay/workflow.go
  - cmd/perceptionreplay/semantic.go
  - docs/experiments/Experiment-085-the-graphics-tab-is-a-button.md
---

# Experiment 086 — the tab changed the panel and not the screen

## The question, as the owner set it

> Inspect whether the captured action has adequate pre-state evidence, a specific supported
> action site, fresh text/visual corroboration, and a verifiable expected outcome.
>
> If the expected transition cannot be established or the target cannot be uniquely grounded, do
> not click. State the exact missing evidence rather than opening another broad matching
> investigation.

Four criteria, asked of **one** press: the GRAPHICS click at 0.414,0.261 on look 248 that
[[Experiment-085-the-graphics-tab-is-a-button]] resolved to a 138 × 30 boundary holding exactly
one label.

## 1. Three of the four are met, from the recording alone

**Pre-state evidence — YES.** The VIDEO panel is stable across looks 240 to 249. The OCR phrase
list is identical on all of them:

```
GENERAL ADVANCED KEYBOARD GAMEPAD AUDIO VIDEO GRAPHICS Ul CAMERA COSMETICS
Resolution 1920x1080 / Window Mode WindowedFullscreen / Upscaler & Anti Aliasing DLSS / ...
```

**A specific supported action site — YES.** 138 × 30 px, identical on all eleven looks around
the press, holding the single word `GRAPHICS`. Experiment 085.

**Fresh text corroboration — YES.** `GRAPHICS` is read on the clicked look itself and on every
look either side of it, which is the case [[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]]
found was NOT true of Rocket League's PLAY button.

**A verifiable expected outcome — NO.** And this is the result.

## 2. The panel changed completely and the screen did not

The frames say the click worked. Look 249 is the VIDEO panel, look 252 onwards is the GRAPHICS
panel, and the two share no body text at all:

```
look 249  Resolution 1920x1080 | Window Mode | Upscaler & Anti Aliasing | Frame Generation
look 252  Graphics Presets Medium | Motion Blur OFF | View Distance Medium | Textures High
look 290  Graphics Presets Medium | Motion Blur OFF | View Distance Medium | Textures High
```

Marco's screen identity does not move. `-frames -semantic` folds the same looks twice and gets
the same answer both times:

```
                             screens settled offered admitted   stale refused
  pixel-only                       2     293       -       -       -       -
  with text, same look             2     293     300     300       0       0

  how each pixel-only screen's looks were distributed by the richer fold
  pixel-only    looks  with text
  visual_1        130  visual_1 x130
  visual_2        168  visual_2 x168

  visits per screen -- a screen entered, left and returned to must rejoin
  pixel-only screens entered more than once       0 of 2
  with-text screens entered more than once        0 of 2
```

**Two screens over three hundred looks, one visit each.** `visual_1` is the world and the pause
menu; `visual_2` is *the settings screen*, and both panels are inside it. 293 of 300 looks
settled, so the panel swap never even held as a candidate for the settle window.

## 3. Which is why the whole recording learns one step, and it is the wrong press

```
  #    from       target     to         cause      support    repeatable
  0    visual_1   target_1   visual_2   click      geometric  true

  STEP 0, LOWERED INTO LEGAL MARCO. Not run.
  |     the at is a Point with X 960, Y 458.
```

960/1920, 458/1080 is **0.500, 0.424** — press 1, the pause-menu click that Experiment 085
reported as an honest ABSTAIN: a 73 × 12 rectangle with no word in it. One step was learned from
three presses, and it is the one press whose target cannot be named.

Presses 2 and 3 were both made while `visual_2` was standing and were followed by no screen
change, so they are **inert**: real presses with a real outcome, and no transition to build a
step on. The one click Marco could identify perfectly is the one it learned nothing from.

## 4. And the replay chain stops before the first step

`-frames -readiness` splits the recording in half and asks how far a reloaded artifact gets
without borrowing evidence or authority from the session that taught it:

```
  1. artifact reload and validation               READY
  2. fresh screen establishment                   READY  [SIMULATED decision]
  3. stored-control reacquisition                 READY
     0 step(s) could not find their control, 0 could
  6. legal Marco lowering and compile validation  not reached
     missing: no step became plannable

  how far it got: stopped before the first step: screen_a is not established in
  this observation; it has to be settled here, and an earlier answer about it is
  not evidence now
```

Stage 3's `0 could not / 0 could` is the shape of a stage that never ran. The cause is the line
above it: **no screen of this recording is entered more than once**, so the second fold never
sees the screen the step starts from, and a step whose source screen is not standing cannot be
planned. The recording is a one-way walk — world, pause, settings — and a replay needs the
screen to come back.

## 5. The verdict

**Do not click.** Not because the target is weak — it is the best-corroborated target in this
project — but because there is no observed transition to verify afterwards, and acting without
one would be pressing something and calling the absence of evidence a success. That is the
refusal [[ADR-183-remembering-recognising-and-being-allowed-to-act]] exists for, and
the owner's third question about a learned control — can Marco verify the
intended transition after acting — stays untested for one more round.

## 6. The exact missing evidence, which is not a detector

Two things, both of them demonstrations rather than algorithms:

1. **A second visit to the screen the click was made on.** 0 of 2 screens in this recording were
   entered more than once. Clicking GRAPHICS, then VIDEO, then GRAPHICS again gives the screen
   two visits and the control two presses — which also settles the one clarification the
   artifact raises (`clicked once, so nothing independent confirms that this control can be
   found again`).
2. **The two panels being two screens.** The words that separate them exist in the recording and
   are not in the fold, and the mechanism is specific: stability is measured ACROSS A VISIT, and
   this visit contains both panels, so `visual_2`'s stable words are exactly the words the two
   panels share — the tab bar:

   ```
   visual_2  161 reading(s): advanced all audio back camera cinematics cosmetic cosmetics
             gamepad general graphics items keyboard mods multiplier quality reset restore
             save settings unblock video
   ```

   Every one of those is a tab label or a button that is on both panels. `Resolution`,
   `Upscaler`, `Graphics Presets` and `Motion Blur` are in the readings and in neither screen's
   identity.

Stated and deliberately not fixed. The owner's instruction was to state the missing evidence
rather than open another matching investigation, and the first of the two needs nobody to build
anything.

## 7. What this does not say

It does not say Ark's settings panels cannot be told apart — nothing here measured a distance
between them, because they never became two screens to measure. It does not say the stabiliser
is wrong: two panels under one persistent tab bar, differing in their body, is genuinely the
hardest case for a segmenter that counts rectangles, and the honest reading is that this
recording cannot decide it either way.

And it does not move [[ADR-234-the-person-does-not-have-to-prove-the-click]]. The press is not in
doubt and never was. What is missing is the OUTCOME, which is the one of the owner's three
questions that has never been answered for any control.
