---
type: experiment
status: complete
date: 2026-10-16
backend:
  - accessibility
  - structure
  - ocr
fixture: "the owner's live desktop — two pages of Windows Settings, plus incidental readings of Chrome, Discord and Visual Studio Code — observed through the augmented configuration of ADR-211, read-only, nothing pressed"
result: the-augmented-configuration-does-what-it-was-approved-to-do-a-durable-place-on-every-settings-page-tried-and-a-press-separable-surface-question-on-one-of-them-but-the-only-answerable-question-found-on-settings-changes-a-real-windows-preference-while-chrome-discord-and-vs-code-offer-them-routinely-so-settings-is-the-wrong-instrument
supersedes: []
affects:
  - perception
  - execution
---

# Experiment 064 — the augmented configuration works, and Settings is the wrong instrument

**Run 2026-10-16 on the owner's machine. Read-only throughout: nothing was pressed, nothing was
demonstrated, and no Windows preference was changed.**

## The question

> Does production observation with the shape proposer and OCR added produce both halves a
> demonstration needs — a durable Place, and a surface question a press could actually answer?

Asked because neither of the first two configurations could, and for opposite reasons. See
[[Experiment-063-the-configuration-that-cannot-close-its-own-loop]].

## It does

Windows Settings, Home page:

```
  configuration   AUGMENTED — production defaults with [structure ocr] added
  foreground      applicationframehost ("Settings"), 180 Props
  PLACE           yes — subj_cd63af216350
  question        "Dark" — 2 options, depth 0 {1468 972 80 19}
  CHECKPOINT      READY
```

Both halves, on the first page measured. Accessibility establishes the Place that pixel-only
could never establish, and the question survives the answerability gate of
[[ADR-210-a-question-nobody-could-answer-is-not-asked]] — which the same page's ties under
production observation alone do not.

## And a result that was the opposite of the prediction

**The richer evidence produced FEWER askable words, not more.** The same Home page offered
`"installed apps"` and `"Recommended"` under pixel-only, and offers one word under augmented.

Not a defect. Accessibility names more things, and a surface containing several named things is
refused because its middle stands for several at once. So adding evidence TIGHTENED the question
rather than loosening it. Worth recording because the intuition runs the other way, and because
it means the configurations are not ordered by quality: they see different screens.

## But the target is not usable

`click Dark` is the theme picker. Pressing it changes the desktop theme, which the owner excluded:

> No games, no automatic retries, and no changes to real Windows preferences.

The second page measured, Apps, establishes a Place and offers nothing at all:

```
  PLACE           yes — subj_2fd76a62aeb8
  103 Props       NOT ONE WORD WOULD RAISE A SURFACE QUESTION
   20             only the words' own region is defensible
   18             no surface: insufficient evidence
    8             several Props are named alike
```

Searching further was ruled out by the owner, and rightly:

> Do not search endlessly across Settings pages. If this configuration fails, report the evidence
> and stop.

## Which is the finding that matters

Settings is a poor instrument for this feature, and ordinary software is not. Measured in passing,
under the same configuration, on windows that happened to be in front:

```
  Visual Studio Code   19 askable   icon buttons where the tree gives the button and a
                                    container around it
  Discord               4 askable   "1080p", "60FPS", "ENABLED", "Future Gaming"
                       20 refused   as unanswerable, by ADR-210
  Chrome                2 askable   "Bookmark this tab", "Open Gemini in Chrome"
                        6 refused   as unanswerable
  Settings (Home)       1 askable   and it changes a preference
  Settings (Apps)       0 askable
```

The caption-inside-a-container tie is an ordinary property of real software seen through an
accessibility tree. Settings, chosen because it was safe, turns out to be among the worst examples
of the thing being tested — which is the owner's own conclusion, reached independently:

> stop treating Windows Settings as the acceptance test itself. It's just a safe application for
> exercising Marco.

Twenty-six unanswerable pairs were refused across Chrome and Discord alone. ADR-210 is carrying
more weight than the case that motivated it.

## What has still never happened

No demonstration, no press, no execution. The loop from a human demonstration to a reused
instruction has never run live, under any configuration. What is now established is that a
configuration exists in which it COULD — and that the authorised application does not offer a
target it may run on.

## Related

- [[ADR-211-production-observation-with-the-pixels-added]] — the configuration this verifies.
- [[ADR-210-a-question-nobody-could-answer-is-not-asked]] — the gate that refuses the Settings
  ties, and twenty-six others.
- [[Experiment-063-the-configuration-that-cannot-close-its-own-loop]] — why the first two
  configurations could not.
- [[ADR-209-the-assistant-operates-the-harness]] — the orchestration that made six live
  measurements cost one session.
