---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - passive-observation
  - passive-visual-screen-integration
source_paths:
  - internal/director/observe/screentext.go
  - internal/director/observe/screentextfold.go
  - internal/director/observe/arrangement.go
  - internal/director/fastobserve/fastobserve.go
  - internal/director/workflowstore/workflowstore.go
  - cmd/perceptionreplay/semantic.go
---

# ADR-194 — a screen is also what it says

## Context

A session-local screen was a count of rectangles in nine boxes, and a saved screen was
`{label, arrangement}` and nothing else.

[[Experiment-041-the-control-Marco-found-for-itself]] measured what that costs on a real game.
Marco navigated Rocket League's Main Menu → Play → Training → **Free Play** with three real
clicks and could not establish that it had arrived. The diagnosis ruled out the two obvious
answers:

- **The saved evidence was good.** `screen_d` agreed with the live destination at **0.750** and
  ranked first by a wide margin — higher than the 0.590 that was ACCEPTED one step earlier.
- **The threshold was not the problem.** Over all 36 pairs of that artifact's nine screens, three
  sit within 0.05 of joining and the closest two are separated by **0.010**. A margin of 0.010 is
  not a margin. And the same two screens agreed at **0.540** as saved and **0.721** looked at
  live: the variance of the measure between one pass and another is larger than the distance it
  has to resolve.

The failure was one layer earlier. The observer had already merged the Training page and the Free
Play submenu into one live identity, so the destination matcher was never offered a distinct
screen to recognise. **Premature identity fusion.**

The words on those two screens agree at 0.111. OCR reads both at about 95% confidence today.

## Decision

A screen carries what it was read to say, beside what it looked like, and that evidence
participates where identity is decided.

There are three places an arrangement decides who a look belongs to. Two of them could merge two
screens, and those two consult text:

| | |
|---|---|
| the standing screen — a look alike enough settles onto it | **vetoed** |
| the pending run — a look alike enough continues it | not vetoed |
| promotion — a settled run alike enough to a known screen reuses it | **vetoed** |

The pending run is deliberately left alone: it is a claim about one arrangement holding still, and
splitting it on text would make a run fail to settle because OCR read a hover highlight on its
second look.

### The one thing text may do

**Refuse a merge.** It cannot propose one, cannot mint a screen on its own, and is never part of a
stored arrangement — `visualScreen.sig` stays pixel-only, so a screen's signature does not depend
on whether anybody read it, and a run with no text source folds exactly as it did before any of
this existed.

The asymmetry is the whole safety argument. The worst a bad reading can do is leave Marco with
two identities for one screen, which costs a recognition. The failure in the other direction —
two screens silently becoming one — is the one that cost a navigation.

### What counts as a contradiction

A conjunction of two asymmetric fractions, because **one symmetric similarity cannot do this
job**: two readings of the same screen at different OCR yields agree at 0.300 and the two screens
that must separate agree at 0.231, and no threshold sits between those.

- `kept` — the share of the screen's vocabulary this look still says. Low means what the screen
  said has largely gone.
- `novel` — the share of this look that the screen never said. High means what is here instead is
  largely new.

Both must fail. Either one alone is a screen read differently: a menu whose labels finished
loading keeps everything and adds; a frame where OCR resolved fewer labels keeps less and adds
nothing.

### Four rules that keep transient text out

Every one of them was a measured defect first, and [[Experiment-042-a-screen-remembers-what-it-said]]
records what each cost.

1. **A refusal is carried by the screen, not the look.** Text is sparse; an unread look settles
   onto the standing screen and would clear the pending run on its way past. At one reading in
   three the refusal fired on every reading and no run ever reached the three looks it needed.
2. **Both sides are agreements.** A screen's vocabulary is the words its readings agreed on; a
   look's side is the words it and the reading before it both had. A single reading as the look's
   side split one main menu into eight identities over 449 looks, because each reading brought
   thirty fresh misreadings and every one counted as a word the screen never said.
3. **A vocabulary is what is usually there.** A word belongs to a screen when it appears in at
   least half its readings. Keeping everything ever folded in meant a screen seen sixty times
   accumulated a hundred words of which forty were real, `kept` fell towards nothing for a screen
   that never changed, and the main menu flapped between three identities 55 times.
4. **Nothing under three characters is a word.** Two readings of one menu disagreed about forty
   tokens like `ae an bs ee fe om on oo pe ro si we ww` while agreeing about every label on the
   screen.

And: unread is not silent. A screen nobody read can never disagree with anything, which is what
keeps every artifact and every caller already in the tree behaving exactly as it did.

### Look-local by construction

`fastobserve.Look.Text` is what a source read of THAT look's pixels. The type cannot express
anything else, and that is a hard requirement rather than a convenience: the comment beside the
fold already refused to feed it `cands`, because a channel arriving ten looks late and
intermittently would make the signature of one screen depend on whether inference had landed yet.
Text may be there because it is a function of the look alone, and because the most it can do is
decline a merge.

OCR costs hundreds of milliseconds against a 100ms cadence, so **no live source satisfies this
yet.** That is the next question and is not this decision.

### At learn time

`workflowstore.Screen.Text` holds what each screen was read to say, with its provenance, saved
through `SaveWith`. A pointer, so an artifact nobody read carries no `text` key at all and is
byte-identical to one saved before the field existed — and `Screen.Said()` answers unread text
for nil, because a reader that could not tell "nobody read this" from "this screen said nothing"
would treat every old route as a chain of silent screens.

## Consequences

Measured offline on the existing 900-frame recording, no live session:

```
                                     pixel-only  with text
  screens settled                             9         13
  looks that settled onto a screen          767        729
  merges text refused                       n/a         18
```

- **The Training page and the Free Play submenu are two screens.** Arrangement 0.559 — over the
  0.55 that merges them — words refuse at kept 0.20, novel 0.80.
- Three other real screens the arrangement had absorbed also separated: the map select, the
  garage, and the custom training panel.
- **The main menu is one identity** across 259 looks and eleven separate visits, and no screen
  that recurred before stopped recurring.

**The named limit.** Rocket League's main menu has a rotating featured panel: 0.946 against
itself by arrangement, with different words. It is tolerated because the four real separations
keep at most 0.21 of the screen's vocabulary and this pair keeps 0.43 — but text over the whole
window cannot tell "the labels changed" from "a panel's content changed", and doing that needs
position. At `TextKept = 0.50` that menu split in two and flapped between the halves. The number
sits inside the measured gap rather than on its edge, and the experiment prints the gap so a later
reader can see whether it is still there.

## Enforced by

`internal/director/observe` — `TestTheTwoMenusAgreeByArrangementAndDisagreeByWords`,
`TestAScreenThatSaysSomethingElseIsNotMergedIntoTheStandingOne`,
`TestSeparatingTwoMenusProducesATransitionBetweenThem`,
`TestWithNothingReadTheFoldIsTheOneItAlwaysWas`,
`TestOneReadingIsNotAVocabulary`,
`TestTransientWordsDoNotSplitAScreen`,
`TestAScreenReturnedToIsStillTheSameScreen`,
`TestAScreenThatSaysMoreThanItDidIsNotANewScreen`,
`TestAScreenReadLessFullyIsNotANewScreen`,
`TestARefusalSurvivesTheUnreadLooksBetweenReadings`,
`TestOneOddReadingCostsAScreenNothingItKeeps`,
`TestAReadingThatAgreesWithdrawsAnEarlierRefusal`,
`TestNobodyLookedIsNotTheSameAsNothingWasThere`,
`TestTooFewWordsIsNotADisagreement`,
`TestWordsAreSplitOneWay`.

`internal/director/workflowstore` — `TestASavedScreenCarriesWhatItSaid`,
`TestAnArtifactWithNoTextIsUnreadRatherThanSilent`,
`TestSaveWithoutTextWritesNoTextAtAll`.

And the measurement itself: `go run ./cmd/perceptionreplay -frames -semantic <recording>`, which
folds the recording both ways and prints the distributions the two numbers sit between.

## Since

The look's side of the rule moved. A single uncorroborated reading may now refuse a merge, on a
stricter bar and with the screen's side exactly as it is above, because a page a workflow passes
THROUGH is read once and the reading that would corroborate it is a reading of the next page. See
[[ADR-199-one-reading-may-refuse-a-merge-and-may-not-mint-a-screen]] and
[[Experiment-047-what-one-reading-is-allowed-to-do]]; the measurement that set the SCREEN's side
stands and is not reopened there.

## Related

- [[Experiment-042-a-screen-remembers-what-it-said]] — the measurement, and the four attempts it
  took.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the live failure this answers.
- [[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] — what an arrangement can and
  cannot see, and the transition storm the settle rule damps.
- [[ADR-184-two-speeds-of-looking]] — why late evidence may not reach screen identity.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the same artifact's other half:
  the recording kept the geometry and dropped the meaning, for controls as well as screens.
