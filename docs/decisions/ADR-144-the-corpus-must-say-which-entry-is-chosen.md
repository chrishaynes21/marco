---
type: decision
status: accepted
date: 2026-10-08
affects:
  - vision
  - control-flow
  - game-packs
source_paths:
  - internal/director/visionbench/temporaltruth.go
  - cmd/director/benchv2.go
  - fixtures/vision/v2/rocketleague
---

# ADR-144 — the corpus must say which entry is chosen

## Context

[[ADR-143-a-search-is-a-wait-with-a-move-in-it]] built a bounded search and named its next
pattern outright: a game's menu, stepped through until the thing being looked for is
highlighted. That is the proving ground this project exists for.

The search's hardest bound is that **an unchanged view is not a place that was searched**
([[ADR-007-no-progress-no-repetition]]). Scrolling satisfies it by fingerprinting the view: a
region at its bottom accepts a page-down forever, and without the check the search spends its
whole budget re-reading one screen and then claims to have looked everywhere.

A menu breaks that check completely. The panel and every entry are present in every frame and
at the same coordinates; **the only thing that moves is which entry is chosen**. A view
fingerprint over a menu is constant, so a menu search with no notion of selection would spend
one move, see nothing change, and report `exhausted` — "it is not anywhere I could get to" —
while sitting on the first entry. That is the confident lie ADR-143 was written to prevent,
arriving through a door it did not cover.

So selection is not an annotation nicety. It is the progress signal, and without it a menu
search cannot be bounded honestly.

### What the corpus could say, measured

The v2 ground-truth vocabulary has sixteen region kinds — `button`, `menu_item`, `menu`,
`panel`, `tab`, `checkbox` and the rest. Every one of them is a **shape**. Not one expresses
**state**, and nothing anywhere in `fixtures/vision/v2` records which control is chosen.

The frames themselves are better than the annotation. Rocket League's pause menu renders the
chosen entry as a pale filled button with a gamepad glyph against three dark blue ones — a
strong signal, and the one the OCR work already found needs PSM 3 rather than sparse PSM 11.

Measured over the annotated entry rects, as mean brightness:

| sequence | frames | resume | change_mode | settings | exit |
|---|---|---|---|---|---|
| `pause-stable` | all 9 | **215** | 100 | 93 | 99 |
| `pause-open` | 029–030 | **215** | 100 | 93 | 99 |
| `pause-close` | 039–041 | 95 | 100 | 93 | 99 |

Two things fall out, and the second was a surprise.

**The highlight never moves.** Resume Game is chosen in all nine `pause-stable` frames, at a
2.1× separation that is trivially detectable and completely static. No sequence in the corpus
contains a highlight passing from one entry to another, which is the one thing a menu search's
progress check is judged on.

**But `pause-close` is a second selection state.** Its frames 039–041 show the menu at full
opacity — the sequence note says so and the pixels agree — with **all four entries identical
and no glyph**: the state Rocket League leaves a menu in when the pointer takes over from the
controller. Nothing is chosen, and that is an observation, not a gap.

So the corpus is poorer than it needs to be and richer than it first looked. It cannot measure
a *moving* highlight — the failure [[Experiment-003-classical-cv-tuning]] established, in a new
place. It can already falsify "always answer the first entry", because on `pause-close` the
right answer is *none*.

## Decision

**Selection is declared as intervals, beside presence, at sequence level.**

```json
{ "identity": "menu_settings", "present": [[0, 8]], "selected": [[3, 5]] }
```

The same vocabulary `present` already uses, because it is the same kind of statement about the
same timeline, and a second interval dialect would be a second thing to get wrong.

Four parts, each load-bearing:

**The word is `selected`.** `observe.Selected` already exists in this tree and already means
"the Actor reports this as the chosen one of its siblings" — which is exactly a highlighted
menu entry. A highlighted entry and a list's chosen row are one claim seen through two
providers. A corpus that said "highlighted" would need a translation nobody would maintain.

**Sequence level, not per frame.** `FrameTruth` could carry a flag per region, and then nine
frames would each restate a fact `sequence.json` already holds. `Shape`'s own comment refuses
exactly this — "a label that could change a number would be a second source of truth about the
same fact" — and the scorer already reads intervals for timing.

**Unannotated is not unselected.** [[ADR-006-unknown-is-not-false]], for the sixth time, and
the subtlest instance yet: a sequence nobody annotated and a sequence where nothing is ever
chosen produce *identical* tracks, and they are opposite findings — one is a hole in the
corpus, the other a property of the scene. `pause-close` is a live instance of the second, so
this is not a hypothetical the design had to guard against; it is a frame in the repository.

Telling them apart needs one bit the tracks cannot carry, because the distinction is precisely
that both have no spans. So the sequence carries `selection_read`: somebody looked, and this is
what they saw. Coverage is then four-valued, and must be asked before any selection number is
believed:

```
unannotated   nobody wrote it down here; withhold the metric, do not report zero
none-chosen   read, and nothing is highlighted; a real answer, and the one that makes
              "always say the first entry" falsifiable
still         read, and it stays on one identity; enough to ask "which one is chosen"
moves         read, and it passes between identities; the only coverage a menu search can be scored on
```

`Measurable()` is the one question a scorer asks: everything but `unannotated` has a truth to
compare against. And writing down a chosen entry **is** reading selection, so a file that
declares spans without the flag is refused rather than quietly reporting `unannotated` over an
annotation it holds.

**One chosen sibling, and only while present.** Two identities selected in one frame is an
editing slip, not a scene anybody photographed, and letting the first win would silently decide
which. An identity chosen in a frame it is declared absent from is the same contradiction
`CheckAnnotations` already refuses between the sequence file and the frames. Both are reported,
never repaired.

### No schema bump

`SequenceSchema` stays at 2. The bump rule exists to stop a file being **misread**, and a
schema-2 file written before this field is read correctly as one that does not annotate
selection — which is precisely what it is. Mode-versus-intervals genuinely could be misread,
which is why *that* change bumped. This one cannot be, and bumping would invalidate a corpus
for gaining a capability it does not use.

## What this deliberately does not do

**It does not annotate the existing frames**, and this is the part most worth revisiting.
Annotating `pause-stable` as `still` and `pause-close` as `none-chosen` is now cheap and
defensible — the measurement above is the evidence — and it would give a real, falsifiable
"can the detector see which entry is chosen" score today.

It is left undone because it is not what blocks the menu search, and doing it here would mean
annotating the corpus from a brightness heuristic written to answer a different question.
Annotation belongs in a capture session with the frames in front of a person, next to the
sequence it is actually missing. The corpus reports `unannotated` meanwhile, and the bench
prints that word in a column so nobody mistakes the silence for a measurement.

**It does not build a selection metric, and it does not build the menu search.** There is
nothing to score yet. Building the measuring stick first is the order this area already learned
the hard way; building the metric first would mean calibrating it against the one scene the
corpus happens to contain.

## What unblocks the rest

One capture: the pause menu open, the highlight walked down every entry and back, a few frames
per entry, nothing activated. [[Vision-Corpus-Workflow]] carries it as its own row with the
reason attached. It needs the game and a person playing it, so it is not something a test can
produce — and it is worth more than any amount of additional held-open-menu footage.

It is also the cheap half of the environment question. **Capturing is read-only** — it watches
and screenshots, injects nothing, and is safe to do on a machine somebody is using. Stepping a
menu for real is the half that needs the isolated environment, and on Windows 11 Home no
software-only isolation exists: Sandbox and Hyper-V are Pro features and Home cannot host an
RDP session.

## Enforced by

- `TestASequenceThatNeverMentionsSelectionIsNotASequenceWithNothingSelected` — the three-valued
  coverage, and that the per-frame answer is identical in both states, which is why the
  coverage has to be asked first.
- `TestAMenuWithNothingChosenIsAnAnswerNotAGap` — `none-chosen` against `unannotated`, the
  distinction the real corpus forced, carrying the measured brightnesses that found it.
- `TestAChosenEntryWithoutSelectionReadIsRefused` — the two halves of one statement, so a file
  cannot hold an annotation and report that it holds none.
- `TestSelectionThatStaysPutCannotMeasureAMenuSearch`,
  `TestSelectionPassingBetweenEntriesIsTheCoverageAMenuSearchNeeds` — `still` against `moves`,
  the distinction the whole decision turns on.
- `TestTwoEntriesChosenAtOnceIsRefused`, `TestAnEntryCannotBeChosenWhileItIsDeclaredAbsent` —
  the two contradictions, reported rather than resolved.
- `TestSelectionSpansObeyTheSameRulesAsPresenceSpans`, `TestPresenceSpansStillObeyTheirRules` —
  the shared span rules, from both sides, since they were extracted from the presence path and
  the older half must not lose them silently.
- `TestTheShippedCorpusCannotYetMeasureAMovingHighlight` — over the real corpus, asserting the
  gap. **This test failing is the good outcome**: it means a capture landed, and whoever added
  it must read what the corpus can now support before writing a metric over it.
- `TestTheBenchReportsSelectionCoverageBesideEverySequence` (`cmd/director`) — the column,
  because a coverage nothing prints is a coverage nobody consults.
