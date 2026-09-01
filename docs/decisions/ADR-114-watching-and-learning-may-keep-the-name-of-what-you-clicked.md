---
type: decision
status: accepted
date: 2026-08-30
affects:
  - passive-observation
  - semantic-memory
  - editing
source_paths:
  - cmd/director/observewiring.go
  - cmd/director/observeambient.go
  - cmd/marco/watchui.go
  - cmd/marco/edit.go
---

# ADR-114 — watching and learning may keep the name of what you clicked

## Context

A person turned **Watch & Learn** on and walked `Home → Bluetooth & devices → Mouse` in Windows
Settings, twice, with real clicks. The control centre said nothing. No learned, no goal, no Try it,
no reason.

The Director's own diagnostic had the answer the whole time, in a terminal nobody was reading:

```
Marco has watched you take 3 ways between screens.
  0 it knows, 0 waiting on something, 3 it can't learn as they stand

 x activate in applicationframehost
     traversed 4 times
     I couldn't read what the control was called, so I can't say what to press
```

The dogfood store held the whole transaction: sixteen moves noticed, three candidate edges with
correct structure on both ends, correct settled place names (`Home`, `Bluetooth & devices`,
`Mouse`), traversed four times and twice. **Attribution worked perfectly.** `"role": "list_item"`,
and `target` absent — every candidate had an empty control name.

### The chain, and where it broke

```
AdmittedTargetLabel(role, demonstration, …)   list_item is not on the plaintext allowlist,
                                             and `demonstration` was false
      ↓
Actionable.Label = ""                         the label is withheld at the push
      ↓
SemanticTarget{Role: "list_item", Label: ""}  a resolved press with no name
      ↓
Act.Representable() == false
      ↓
ambient.Judge → Never / control_not_named     structurally unpromotable, forever
```

`NameablePlaintext` is `{button, menu_item, menu, tab, checkbox, radio}`. Windows Settings
navigates entirely by `list_item`. Ambient sessions are started with the zero episode, so
`liveSampler.nameActivatedTargets` was false. **Ambient learning could not promote anything in
Settings, ever, however long anybody left it on** — and the same is true of every interface that
navigates by list items, tree items, links or rows.

This is not a naming-rule question and 37K/37L are not involved. It is the sixth wiring defect on
this branch of the identical shape.

## Decision

### Ambient learning is the second door to a permission the system already declared

`ambientPromotionLicence()` has returned `observesession.LearnLicence()` since the day it was
written, and that licence holds `NameActivatedTargets: true`. The permission was declared;
perception was never told, because the licence travels to the sampler as a copy of the EPISODE and
ambient sessions declare none.

So `liveSampler.mayNameTargets()` reads both doors: the licence the caller declared, or ambient
learning being on. The second is the same human semantic event as the first — an explicit act, off
by default, a separate switch from watching — arriving by a different route.

**Read live rather than copied at session start**, because somebody presses Watch & Learn while a
session is already running. Ambient sessions last twenty seconds; a copy would leave the mode inert
for the rest of one, with nothing on screen to say why.

### What does not change

Watching alone still keeps no control name — attention is not agreement to retain the text of
whatever somebody clicks, and a list item's text is very often a fact about them. The shape filter
is unconditional and unchanged: a friend tag, a filename, a token, anything over the length and
word bounds is refused whatever the licence says. The gate still admits only what one input
event's own resolution touched, never a sweep. Nothing here widens what is PERCEIVED.

### Learning must produce something observable

The second half of the same report: even with the blocker, the page should never have been silent.

```
WHAT MARCO MADE OF IT        every way between screens you have taken
  ✓ activate "Bluetooth & devices" — 4 times
      Marco remembers this way between two screens.
  ✗ activate — twice
      I couldn't read what the control was called, so I can't say what to press
```

`/api/made` is the evidence read the command line already makes, unchanged, rendered on Here. Every
line is the Director's own verdict and the Director's own sentence: the page decides nothing about
whether something was learned, and cannot. An empty list says so rather than rendering blank —
"nothing yet" and "I learned nothing from that" are different answers and a blank panel is neither.

One rendering rule is not obvious and is enforced: a candidate the policy has already admitted
comes back as `never / already_known`, so a list reading the verdict alone would draw a refusal
beside the one relationship Marco actually learned.

### Topology is learned; a goal is meant

Ambient learning admits places and the ways between them and creates **no goal** — `admitWatched`
says so in as many words, and that stands. A name for an outcome is a thing a person means, not a
thing repetition implies. So after a perfectly successful traversal there is still nothing to Try,
and the page now says so and offers the canonical way to close the gap:

```
Marco learns places and the ways between them by watching. What to ask for is still
yours to name.
[ open mouse settings                    ] [ Name what I just did ]
```

That posts to `/api/learn/recent` — the same request `marco learn --recent` makes, which promotes
the walk, keeps the demonstration and records the goal. **The endpoint already existed and the page
had never called it.** The goal's commit announces `learned goal`, which lands in JUST LEARNED, and
Try it is offered from there through `marco do` as it always was.

The progression is now visible end to end:

```
known Place  →  known way between screens  →  a name you gave it  →  Try it
```

## Consequences

- Under Watch & Learn, the name of the one control a person's own click resolved to may be
  retained — including list items, tree items, links and rows. This is a real widening of what is
  kept while that mode is on, held by the same unconditional shape filter as every other admitted
  label, and it is the permission the promotion path has always declared it held.
- Candidates recorded before this fix keep their empty control name permanently: the name is part
  of the candidate's handle, and folding never fills one in. A new traversal makes a new candidate
  beside the old dead one. `dogfood.cmd reset` before the next run avoids the noise.
- The evidence read names controls, so the Here panel now shows a control's name where it
  previously showed only counts. That widening is the Director's, already reviewed, and the page
  carries it no further.

## Enforced by

- `cmd/director` `TestWatchAndLearnCanKeepTheNameOfWhatYouClicked` — entered at
  `EnableAmbientLearning`, read off the real navigation subscription
- `cmd/director` `TestWatchingAloneKeepsNoControlName`
- `cmd/director` `TestTheLabelLicenceFollowsTheSwitchWithinOneSession`
- `cmd/director` `TestOnlyAGrantedSessionMayNameWhatWasActivated` — the episode copy, unchanged
- `cmd/director` `TestACleanTraversalUnderWatchAndLearnReachesTheGraphAndTheFeed`
- `cmd/director` `TestACandidateThatCannotBeLearnedSaysWhy`
- `cmd/marco` `TestTheHereViewAsksWhatMarcoMadeOfIt`
- `cmd/marco` `TestTheOutcomeListReadsTheVerdictFromTheDirector`
- `cmd/marco` `TestThePageOffersTheCanonicalWayToNameWhatYouJustDid`
- `cmd/marco` `TestEveryDoorThePageKnocksOnIsAnswered`

## Related

- [[ADR-113-learning-is-inside-watching]]
- [[ADR-112-the-loop-belongs-where-a-person-is-already-looking]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[Experiment-022-the-first-dogfood]]
- [[ADR-115-one-experiment-and-the-desktop-given-back]]
