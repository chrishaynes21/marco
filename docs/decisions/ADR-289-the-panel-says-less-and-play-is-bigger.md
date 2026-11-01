---
type: adr
status: accepted
date: 2026-11-01
decision: "ACCEPTED — THE WAY TO RENAME A ROUTINE GOES WHERE THE NAME IS, PLAY IS BIGGER THAN THE CONTROLS AROUND IT, AND THE MACHINE DETAIL COMES OFF THE CARD. The owner, looking at the window with [[ADR-285-a-routine-can-be-called-something-else]] in it: *add the ability to rename routines* — and then *there's aliasing, but no straight rename*. It was there. The flow was complete, reviewed, mutation-gated and landed the night before; it was a deed row FOUR ROWS DOWN, below the step editor, below the box that adds another phrase a routine answers to, and below the scope row. Somebody looking for a way to change a routine's name finds the alias box first, because that box is about what a routine is called, and concludes the window cannot do it. A CAPABILITY NOBODY CAN FIND IS THE SAME AS ONE NOBODY BUILT — the third time in this window, after Simplify inside a collapsed detail and the whole of [[Studio-journey-discoverability-2026-10-31]] — and the lesson is not the one the earlier two taught. Those were about collapsing; this one was visible, on the page, in plain sight, and still unfindable, because it was BELOW A CONTROL THAT ANSWERS A NEIGHBOURING QUESTION. So the control moves onto the identity row, beside the name it changes; nothing else about the rename changed, and the review panel is scrolled into view because the button that opens it is now the length of a card away from it. PLAY IS BIGGER, AND BY SIZE RATHER THAN COLOUR. `--ctl` is one height for every button, field and select on this page, which is what stops a row of controls looking assembled and is the wrong answer for the one thing most visits to this window end in: Play was exactly as big as `Refresh` beside it. `button.go.big` is a 46px target with 15.5px lettering; the three tiers still mean what they meant, because the fill was already the loudest thing on the panel and a louder one would say something about urgency rather than about size. AND THE CARD STOPS ENUMERATING WHAT A GUARDED ROUTINE CHECKS. *We don't need window size and scale checked before each press, that type of stuff belongs in the help section.* It read `Checks the program, window size and scale before each press. Stops if a check fails.` — true, and a few lines above the press-in row that says the same thing about the window the run will actually raise, so one fact was said twice and the half that enumerates three measurements sat in front of somebody deciding whether to press a button. The guarded contract — which is every routine recorded since Marco measured windows — now says NOTHING there. Silence for the ordinary case, words for the two that are a warning: a legacy routine presses where it was recorded having checked nothing, and an incomplete one refuses with the missing measurement named. What is checked is a new Help section, linked from that row whatever the contract, because somebody asking it is asking about Marco and not about this routine. THE SAME CUT, ONE SURFACE IN: the step editor printed `left · of 1280×800, 96 dpi, notepad.exe` beside every movable press, to promise that moving a point keeps what it is checked against. The promise was right and the place was wrong — it is the same promise on every row — so it is made once in Help's `measured` section and the row keeps the BUTTON, which is what the press does rather than a measurement of the window. Nothing about what is preserved changed; `TestAMeasuredPressCanBeMovedAndKeepsWhatItIsCheckedAgainst` still reads the saved file."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/routines.go
  - plugins/studio/help.go
  - internal/orchestrator/pointedit.go
---

# The panel says less and Play is bigger

## What happened

The owner, on the window as it stood after the four-slice alpha pass:

> add the ability to rename routines. The play button needs to be bigger too, there's still too
> much text, like we don't need window size and scale checked before each press, that type of
> stuff belongs in the help section

and then, a moment later:

> there's aliasing, but no straight rename

## The one worth writing down

**Rename was already built.** `c85b31c` landed it the night before — the box that edits only the
name, the chip carrying the arguments it keeps, the reviewed move, collisions refused with no
overwrite, the credential check, eight tests and a rendered page test. The binary the owner was
looking at had it: `studio.exe` was written at 08:56:06 and the window opened at 08:56:10.

It was a deed row **four rows down**:

| | |
| --- | --- |
| `routineStepsRow` | Edit its steps |
| `routineAnswersBox` | **Also answers to** — add another phrase |
| `routineWhereRow` | Change where it works |
| `routineRenameRow` | Rename it |

Somebody looking for a way to change a routine's name reaches the second row first, and that row
is about what a routine is called. It answers the neighbouring question convincingly enough to
stop the search.

So this is the third time this window has hidden a finished capability, and it is **not the same
mistake as the first two**. Simplify was inside a collapsed `<details>`; the learn-offer journey
was unreachable. This one was expanded, on screen, two scrolls down, with its own explanatory
sentence — and still unfindable, because what decides whether somebody finds a control is not
whether it is visible but **what they meet before it**.

The fix is placement and nothing else. The button sits on the identity row beside the name it
changes; the box drops in under the two provenance lines; `Review the rename` opens the same
panel, and now scrolls it into view, because the thing it opens is a card's length below the
thing that opened it.

## Play

`--ctl` is one height for every button, field and select on the page. That rule is why a row of
three controls here does not look assembled, and it had made Play exactly as big as the
`Refresh` beside it and the `Rename` above it.

`button.go.big`: 46px tall, 15.5px lettering, more horizontal air. **Size and not colour** — the
primary fill was already the loudest thing on the panel, and making it louder would say something
about urgency instead of something about importance.

Asserted by measuring it in a browser, not by reading the markup: one later rule setting
`min-height` or `font-size` on `button` takes the class back silently, and a markup test would go
on passing.

## The text

Two cuts, both of the same kind: a true sentence in the wrong place.

### What a guarded routine checks

> Checks the program, window size and scale before each press. Stops if a check fails.

A few lines below it, the press-in row says — about the window the run will actually bring
forward — *"Its presses are checked against that window and stop if something else comes
forward."* One fact, twice, and the version on the card names three measurements to somebody who
is deciding whether to press a button.

The guarded contract is every routine recorded since Marco measured windows. It now says nothing
there.

| contract | what the card says |
| --- | --- |
| guarded | nothing — it is the ordinary case |
| legacy | *its presses are not checked: each one happens at the point it was recorded at, whatever is there now* |
| incomplete | nothing here; the missing measurement is in `routineWhy`, which is also why it will not run |

**Silence for the ordinary case and words for the exceptions.** `Contract` still carries the
state for anything that needs to branch on it, and the fact that a guarded routine's presses are
checked is still said beside Play, where it names the window and bears on the press about to
happen.

What is checked is a new Help section, **linked from that row whatever the contract** — because
somebody asking that question is asking it about Marco, not about the routine in front of them.

### What a change to a press keeps

The editor printed `left · of 1280×800, 96 dpi, notepad.exe` beside every movable press.
[[ADR-286-a-press-can-be-moved]] put it there for a good reason: a row of two editable numbers
and nothing else leaves somebody to find out afterwards whether moving a press also changed what
it was checked against.

The reason stands and the answer was in the wrong place. It is **the same promise on every row**,
so it is made once, in Help's `measured` section. The row keeps the button, which is not a
measurement — it is what the press *does*, it differs from row to row, and a list of presses that
did not say which button each one uses would be missing something about the steps.

**And the button was already on the row.** [[ADR-291-the-press-row-says-what-it-does]] took
`Terms` away a day later: a step is written as one sentence and the row splits it into an action
column and a value column, so `Left-click` was the first thing on the row while `left` sat after
the numbers in a muted machine face. The argument above is right about what belongs on a press
row and wrong about where it was missing from. It was not missing.

Nothing about what is preserved changed. The press still carries its button, its modifiers and
all five of ADR-250's terms through a move, and the test that reads the saved file still says so.

## And one defect found on the way

`StepEdit` declared `X, Y int` under a single `json:"x,omitempty"`, which names **both** fields
`x` — so anything marshalling one would write the horizontal offset twice and lose the vertical
one. Nothing marshals it today (Studio builds these in Go and the orchestrator reads them in Go),
which is why it went unnoticed: `go test` runs a subset of vet that leaves `structtag` out, and
only a deliberate `go vet ./...` says so. Fixed here because it was found here.

## What this does not do

- It does not change what a rename does, what it preserves, or what it refuses.
- It does not change what a press keeps when it is moved.
- It does not change any contract, guard or check. Three sentences moved and one was deleted as
  a duplicate.
- It does not touch the Acts `Play`, which is behind a withheld view.

## Enforced by

- `plugins/studio` `TestPlayIsTheBiggestThingOnARowAndTheRowReadsLeftToRight` — **rendered in
  headless Edge**, because both claims are about what is computed and where things land: Play is
  at least 44px, taller than the control beside it, and set in bigger lettering, with Rename to
  the right of the name and Play to the right of both.

  It was TestPlayIsTheBiggestThingOnThePanelAndRenameIsAtTheTop (unbackticked: it is the
  record of what the claim was, not a test that exists), which also held that the
  rename control was drawn ABOVE Play and above the box that adds another phrase. That ordering
  is gone, not weakened: [[ADR-290-a-routine-opens-where-it-is]] put Rename and Play on the same
  line of the routine's own row, which is a stronger answer to the finding above — the way in is
  beside the name, and the alias box is now inside the routine rather than under it.
- `plugins/studio` `TestARoutineOpensWhereItIsAndPlayStaysOnItsRow` — that the checks row is
  silent for the ordinary case and the link to Help is still on it.
- `plugins/studio` `TestASavedRoutineIsListedAndReadyToPlay` — the guarded contract writes **no**
  sentence on the card, and Help says `program`, `size`, `scale` and `stops`. Both halves in one
  test, because a sentence removed from one surface and not added to another is a sentence nobody
  can read.
- `plugins/studio` `TestAMeasuredPressCanBeMovedAndKeepsWhatItIsCheckedAgainst` — Help's
  `measured` section carries the promise, and **the saved file still carries every guard term**,
  which is the assertion that was always the point of that test. It also asserted the row's terms
  were exactly `left`; [[ADR-291-the-press-row-says-what-it-does]] replaced that with the sentence
  OPENING with `left-click at `, which is what puts the button in the action column.
- `plugins/studio` `TestEverySectionTheWindowLinksToExists` — `checks` is a section and something
  in the window opens it.
- `plugins/studio` `TestTheWindowRenamesARoutineAndKeepsWhatItAsksFor` and
  `TestARefusedRenameSaysSoWhereTheAnswerIs` — unchanged and still passing, which is the evidence
  that only the placement moved.

Ten mutations, ten killed: Play's class removed, `big` given `--ctl`'s own metrics, the rename
button deleted, the rename button moved back below the alias box, the Help link removed, the
enumeration restored to the card, the `checks` section removed from Help, the window measurements
put back on every row, the keeps-promise removed from Help, and the button taken off the row too.

## See also

- [[ADR-285-a-routine-can-be-called-something-else]] — the rename this makes findable
- [[ADR-286-a-press-can-be-moved]] — where the row's terms came from
- [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]] — Help, and why its
  sentences are composed by the backend
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — the contract the card stopped
  enumerating
- [[Studio-journey-discoverability-2026-10-31]] — the two earlier hidden capabilities
