---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: rocketleague
fixture: "`rl-freeplay-1` — 900 frames at 10 fps, 17 pointer presses with positions. The same click asked of every look in a +/-5 look (+/-500ms) window through the production proposer, with `perceptionreplay -frames -temporal`. Still NO Ark recording with frames and click positions."
result: "reaching back to an earlier frame rescued one press of seventeen while the click's own look held the smallest boundary of its whole window on four, and on both presses with an independently known answer the boundary at the press was already correct — so temporal evidence is not what Ark's panel problem needs, and two limits of the identity relation were measured on the way"
supersedes: []
affects:
  - perception
source_paths:
  - cmd/perceptionreplay/temporal.go
  - cmd/perceptionreplay/temporalrun.go
  - cmd/perceptionreplay/temporal_test.go
  - internal/director/observe/onecontrol_test.go
---

# Experiment 084 — the highlight that hides the label draws the box

The owner's reading of [[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]]:

> The important discovery is that OCR can lose the very label being clicked because the button's
> visual state changes. That means a strategy based entirely on the clicked frame may be
> fundamentally weaker than one using the frames before the click. We should treat the
> demonstration as a short sequence of visual evidence, not one screenshot.

That is a testable claim about the region proposer, not about OCR, and it is testable on evidence
already on disk. Nothing had ever asked one click of more than one frame.

## 0. What this still cannot ask

There is **no Ark recording with frames and click positions**. `demonstrate-visual` keeps neither
on purpose and `director capture-game-session` has never been run on Ark, exactly as Experiment
083 reported. So this is Rocket League again — the regression case the owner named — and the Ark
answer is still one command away.

## 1. The measurement

For each press, the smallest proposal containing the click, on every look from five before to
five after. The **same rule the correlator uses**: `smallestHolding` is a second implementation of
`today` and `TestTheTemporalRuleAtTheClickLookIsExactlyTodaysAnswer` pins them together, because
a second implementation that drifted would make every number below a measurement of something
else.

Five verdicts, and the split between the last two is the one that matters:

```
STABLE          no other look offers a smaller boundary that is a DIFFERENT control
TIGHTER BEFORE  an earlier look does — certainly the same screen
TIGHTER AFTER   a later look does — which may already be the screen the press caused
APPEARS         nothing held the click on its own look and something did nearby
NOTHING         no look in the window held the click at all
```

"Smaller" alone is not tighter. Around press 1 the proposer offered the same button as 13.0%,
10.9%, 11.2%, 8.6%, 3.8% and 9.2% of the window wide — a factor of three — and the relation calls
every one of them one control, as does the oracle. A rule that counted any smaller rectangle as a
rescue would have called eleven of seventeen presses rescued and learned nothing.

## 2. The result

```
  17 press(es), each asked of +/-5 look(s)
    11 STABLE   no neighbouring look offers a smaller, different boundary
     1 TIGHTER BEFORE the press — the same screen, so usable evidence
     5 TIGHTER AFTER the press — may be the screen the press caused
     0 APPEARS  nothing held the click on its own look and something did nearby
     0 NOTHING  no look in the window held the click at all
```

**One press of seventeen.** Press 16: the click look offered 100.0%×46.1% — half the window — and
look 754, four looks earlier, offered 20.0%×3.1%. That is the whole of the case for reaching back.

The five TIGHTER AFTER cases are not usable as they stand, and press 11 shows why: eight looks of
100.0%×65.6%, then looks 572 and 573 offer 3.4%×2.6%. The press navigated. A smaller rectangle
found on the screen the press *caused* is a different control, not a better boundary for the one
that was pressed.

## 3. And the hypothesis is inverted on four presses

```
    4 of 17 press(es) had the SMALLEST boundary of their whole window on
    the click's own look.
```

Press 7 is the clean case. Looks 330–334 all offer an 83% panel; look 335 — the press itself —
offers 20.0%×3.1%. The button exists on **no other look in the window**. Presses 6, 8, 9 and 17
look the same.

> The highlight that stops OCR reading the label is the same highlight that gives the proposer an
> edge to segment.

Experiment 083 found that engaging a control destroys its text evidence. This finds that engaging
it *creates* its visual evidence. Same cause — the entry renders differently when engaged — and
opposite consequences for the two readers of those pixels. So "use the frames before the click"
would have cost four presses to buy one.

## 4. Against the one known button

```
    2 press(es) land inside it; the proposer offered it on 12 of 22
    readable look(s) around them, and on EVERY look of 0 of them.

    AND ON THE CLICK'S OWN LOOK: 2 of 2.
```

Two things worth keeping apart.

**The click look was already right, both times.** So no rule reaching for another frame can be
shown better here: there is nothing for it to improve. That is a limit of the oracle, not a
result, and it is stated as one.

**But the right boundary was offered on only 12 of 22 looks, and on neither press was it offered
on every look.** Away from the press the proposer's answer for a known button is right about half
the time. That is a number about reacquisition, not about targeting, and it is worse than the
earlier framing implied.

## 5. Two limits of the identity relation, found by fixtures that would not fail

Both were discovered because a test fixture refused to behave, and both are now recorded in
`internal/director/observe/onecontrol_test.go` beside the relation itself.

**A panel is one control with whichever button sits on its centre.** `sameRect` is mutual centre
containment, so a 49%×64% panel and a 16%×5% button on its centre point each contain the other's
centre. A menu with an odd number of entries has such a button by construction. A button away
from the centre is correctly separated — so this is a limit, not a blanket failure — but it means
**no comparison of rectangles alone can establish that [[Experiment-082-a-control-that-is-half-the-window]]
learned the wrong thing.**

**And the relation is not transitive.** Measured on press 10: the click look offered 353×44 and
the look after it offered 90×33. Each is one control with the button a person established. They
are not one control with each other. So a *different control* verdict drawn from this relation is
not evidence of a different control, and a chain of reacquisitions can drift.

Neither is a bug. The relation is threshold-free and symmetric on purpose, and the alternative
was an invented overlap fraction this subsystem has a measured history of getting wrong. They are
limits, and they are the whole of the answer to the owner's second question —
*can Marco distinguish that button from similar but incorrect controls?*

## 6. What this does and does not license

**Not** a pre-click targeting rule. One rescue against four presses where the click look is
already the best look is not a case, and it is measured rather than argued.

**Not** a consensus-across-looks rule either, and the reason is the oracle rather than the idea:
the click look was already correct on both presses that can be checked, so a rule agreeing across
frames cannot be shown better on this evidence. It is untested, not refuted.

**It does** leave the blocker where Experiment 083 put it. The click → target rule works where a
button-sized proposal exists. On Ark no proposal was button-sized. Nothing measured here moves
that, and nothing measured here suggests a different frame would.

## 7. The missing observation, unchanged and now narrower

```sh
director capture-game-session --application arkascended --session ark-menu-1 --seconds 45
```

With the harness in place, that recording answers three questions in one run: whether Ark's menus
ever offer a button-sized proposal for a demonstrated click (`-button`), whether any look around
the press offers one when the press's own look does not (`-temporal`), and whether the panel that
was learned is one control with the button on its centre, which is now a specific thing to check
rather than a worry.

The second observation Experiment 083 named — **OCR of the candidate rectangle rather than the
frame** — is untouched and still unbuilt.
