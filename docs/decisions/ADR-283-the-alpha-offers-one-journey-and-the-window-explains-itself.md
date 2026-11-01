---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — THE ALPHA OFFERS ONE JOURNEY, AND THE WINDOW EXPLAINS ITSELF IN ONE PLACE. Studio's rail offered Routines, Acts, Scenes and Run history as four equal choices, and the alpha supports one of them. Nothing is REMOVED: the views, the bindings and the capabilities behind Acts and Scenes stay exactly as they are, and what is withheld is the way IN — because a rail that offers a surface the alpha cannot carry pays for that at the far end of it, where somebody has already spent a demonstration. Two environment flags, `$MARCO_STUDIO_ACTS` and `$MARCO_STUDIO_SCENES`, read ONCE when the window's backend is made and defaulting to off; no settings control and no runtime toggle, because a rail that can change between two polls is a rail that moves under a cursor. The BUTTONS AND SECTIONS STAY IN THE DOCUMENT and only the rail LIST narrows: `paintViews` walked a hardcoded `['routines','acts','scenes','history']` that was both the rail and the element lookup, so deleting an entry threw mid-paint and stopped the window rendering altogether — silently, because a `-H windowsgui` binary has no console. `show` refuses a view that is not offered, because a hidden button is not a closed door and the bootstrap and several handlers call it by name. The reads a withheld view would want are guarded in the two functions themselves rather than at their six call sites; the recorder's state keeps being read, SLOWLY, because it also paints the rail's build stamp, which is the one thing in this window that makes a stale director visible and whose absence CLAUDE.md records as having cost the owner a day of recordings. AND HELP IS ONE SECTION, COMPOSED BY THE BACKEND. Not because a page cannot hold sentences, but because two of them name a CONFIGURED key — the stop gesture and the argument key — and this repository has already shipped the defect of a surface that answered from the raw variable, so an unset override read as `there is no shortcut` in the default installation, which is every installation. `Acts.Help` composes from the same `everywhereSentence`, `stopLabel` and `argKeyLabel` the record card uses, so the day a key moves the help moves with it. Five sections, each an ability the audit found complete and invisible; each carries a stable key, and a small link beside Record, beside the argument sentence and beside `Edit its steps` opens THAT section rather than dropping somebody at the top of a page of everything. The recording section says what the recorder does: Marco records ONE window and THE WINDOW IS CHOSEN, so the first draft's `the window you go to` was corrected before it shipped — it is the kind of false that teaches somebody to click into something else and then wonder where their demonstration went."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/help.go
  - plugins/studio/acts.go
  - plugins/studio/frontend/index.html
---

# The alpha offers one journey, and the window explains itself

## What happened

The owner, on the window as it stands:

> The UI still feels clunky. The idea is to remove some of the text by adding a help section, acts
> and scenes get hidden since we don't support them right now, and the edit screen gets a facelift
> + usabilty

Three things, and this ADR is the first two. The third is
[[Studio-alpha-UI-slice-2026-10-31]]'s slice 2.

## Acts and Scenes

The rail offered four entries and the alpha supports one of them. That is not a layout problem.
Somebody who follows `Acts` into the composer spends a demonstration before they find out, and the
cost of an unsupported surface is always paid at the far end of it.

So the way in is withheld and **nothing underneath it is touched**. `view-acts` and `view-scenes`
are still in the document, their bindings are still exported, `loadActs` and `loadDemos` are still
written, and turning either flag on gives back exactly the window that shipped — plus Help.

### The flags

`$MARCO_STUDIO_ACTS` and `$MARCO_STUDIO_SCENES`, read **once**, when `newActs` runs. `1`, `on`,
`true`, `yes` and `y` all mean on, because this is set by hand in a shell and not by a form, and a
flag that quietly ignores `true` is a flag somebody will believe they turned on. Anything else,
including absent, means off — a flag whose ABSENCE turned a surface on would be a surface nobody
chose.

Once, and not per call, is the Architect's ruling and it has a reason: a rail that can change
between two polls is a rail that moves under a cursor. There is no settings control and no runtime
toggle, which is a decision and not an omission — it belongs to whoever ships the alpha rather than
to whoever is using it.

### The one that would have broken the window

`paintViews` walked a hardcoded list and did `$('rail-' + v).classList.toggle(…)`. The obvious
implementation of this change — delete the two buttons — throws a `TypeError` mid-paint, and the
page stops rendering **altogether**. A `-H windowsgui` binary has no console to print it in, so
what a person would see is an empty window.

So the elements stay and the LIST narrows. `Features.Views` comes from the backend and is what the
rail is painted from; `EVERY_VIEW` is what the page looks elements up by. They were one array doing
both jobs.

`show` refuses a view that is not offered. A hidden button is not a closed door: `show` is called
from the bootstrap and from handlers that send somebody somewhere after a press, and a door left
open onto a surface the alpha cannot carry is worse than a visible one.

### The reads

Guarded in `loadActs` and `loadDemos` themselves rather than at their six call sites, because a
seventh added later would not know to ask.

`pollLearning` was **not** stopped here, and this was the one place the ruling *"must not keep
unnecessary polling active"* was answered with something other than a stop. That poll paints two
things, and only one of them is on the Scenes view. The other is the rail's **build stamp** — which
build this window is, and which build is doing the recording — and CLAUDE.md records what the
invisible version of that cost: *"It cost the owner a day of recordings."* So it kept running and
the CADENCE went from two and a half times a second to once per ten seconds.

**That was a disclosed interim compromise, and the Architect ruled it was not completion.** The
split is made in [[ADR-288-the-rail-keeps-its-build-stamp-when-scenes-is-withheld]]: the rail's
question is `Acts.BuildStatus`, asked on its own slow timer whatever is offered, and
`pollLearning` is back to four hundred milliseconds, started only when Scenes is offered, and
stopped dead — no reschedule — when it is not.

## Help

There was none. 82 `muted` blocks in the page, 22 of them in the Routines view, and no single place
that explains anything.

The sentences are composed **by the backend**, which is not a style choice. `everywhereSentence`
and `argSentence` name F12 and F9, and both are read from the environment. This repository has
already shipped the version of that defect where a surface answered from the raw variable:
`Recording` reported `$MARCO_STOP_KEY` unparsed on one path and the label on the other, so the
default installation — which is every installation — was told during a recording that the key was
F12 and told before one that there was no key at all. `Acts.Help` reuses `stopLabel`,
`argKeyLabel` and `everywhereSentence`, so there is one resolution of each and the help moves when
the key does.

### The five sections

Each one is an ability the audit found **complete and invisible**, not a tour of the window:

| | why it is here |
| --- | --- |
| Recording a routine | the stop shortcut existed only in the band that sits *behind* the application being demonstrated in |
| Asking for a value each time | three ways in, and the window named none of them |
| Editing what a routine does | a review step nobody knew was a review step |
| Where a press is measured | a press in a window is written down twice and nothing said which number you are looking at |
| Where a routine works | the three scope answers, which are chosen once and never explained |

Each carries a stable `Key`, and a small link beside Record, beside the argument sentence and
beside `Edit its steps` opens **that** section. A link that drops somebody at the top of a page of
everything has moved the explanation rather than kept it.

### The claim that was wrong in the first draft

The proposal's Help said Marco *"watches only the window you go to."* The Architect corrected it:
it records **the window explicitly chosen beforehand**. `Deps.Begin` REFUSES a confined recording
that was not told which window, and the first version of the recorder — which latched the first
foreground window that was not Marco's — is precisely the defect
[[ADR-268-record-only-the-chosen-window]] exists to have fixed. "The window you go to" is the kind
of false that teaches somebody to click into something else and then wonder where their
demonstration went.

## What this does not do

- It does not remove Acts or Scenes, or any of their code.
- It does not move any prose out of the step editor. That is slice 2.
- It does not give the flags a control in the window.
- It does not separate the rail's build stamp from the recorder's state.
  [[ADR-288-the-rail-keeps-its-build-stamp-when-scenes-is-withheld]] does.

## Enforced by

- `plugins/studio` `TestTheAlphaRailOffersOneJourney` — with **nothing set**, which is the state
  that matters: the rail is Routines, Run history and Help, and neither flag is on.
- `plugins/studio` `TestAFlagPutsAWithheldViewBackInTheRail` — five spellings of on and six of off,
  and the restored rail is in the order it shipped in.
- `plugins/studio` `TestTheFlagsAreReadWhenTheWindowOpensAndNotAfter` — the variable changes under
  an open window and the window does not; the next window has it.
- `plugins/studio` `TestHelpNamesTheConfiguredShortcuts` — with `MARCO_STOP_KEY=esc` and
  `MARCO_ARG_KEY=f8`, Help says **Esc** and **F8** and says neither F12 nor F9. The overrides are
  set to something other than the defaults because that is the only arrangement that can tell a
  composed sentence from a written-down one.
- `plugins/studio` `TestHelpSaysTheWindowIsChosenBeforeRecordingStarts` — the recording section
  says the window is chosen and that Marco asks which one before it starts, and does not say *the
  window you go to*.
- `plugins/studio` `TestEverySectionTheWindowLinksToExists` — every key the page links to is a
  section, and every section the page does not link to is one somebody reaches from the rail on
  purpose.
- `plugins/studio` `TestTheAlphaWindowOffersOneJourney` — **rendered in headless Edge**: the
  Routines view is on screen at all (which is what deleting a rail button takes away), Acts and
  Scenes are not in the rail, `show('acts')` arrives at Routines, Help is drawn and a link opens
  the section it names, the two withheld reads are never made, the recorder's state is read at
  most twice in five seconds, and Help is asked for once.
- `plugins/studio` `TestTurningBothFlagsOnGivesBackTheWholeRail` — rendered: both entries come
  back, both views can be reached, and both fill.

## See also

- [[Studio-alpha-UI-slice-2026-10-31]] — the whole slice, the two rendered layouts, and the
  Architect's four decisions and five amendments
- [[ADR-268-record-only-the-chosen-window]] — why the recording claim had to be corrected
- [[Studio-UI-audit-2026-10-29]] and [[Studio-journey-discoverability-2026-10-31]] — where the
  five Help sections come from
