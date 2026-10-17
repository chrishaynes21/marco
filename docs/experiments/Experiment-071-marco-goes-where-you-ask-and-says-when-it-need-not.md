---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the owner's live desktop, with standing authorization for ordinary reversible operations. Windows Settings navigated by request, and File Explorer pointed at a disposable temp folder. An isolated MARCO_HOME throughout, so nothing touched the real store. No setting was changed; one folder was created in the disposable directory and removed afterwards."
result: five-natural-phrasings-of-go-somewhere-now-parse-resolve-and-navigate-for-real-and-a-request-for-a-page-already-open-performs-nothing-and-says-so-with-four-defects-found-live-two-fixed-two-recorded
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-221-a-question-the-person-already-answered.md
  - docs/decisions/ADR-222-going-somewhere-is-one-request-however-you-ask.md
---

# Experiment 071 — Marco goes where you ask, and says when it need not

The first live-desktop session under standing authorization for ordinary reversible
operations. Four of the owner's own example requests, run against real applications through the
ordinary request path.

## What Marco now does without being taught

**"Take me to Bluetooth settings"**, from the Personalization page, with real input:

```
intent    semantic "bluetooth settings"
observe   117 elements, coverage 0.70, actionable 0.88
resolve   list_item "Bluetooth & devices" (0.66)
plan      open "Bluetooth & devices" (medium risk)
policy    allowed
execute   open via double_click · after 1 stronger option(s) were unavailable
verify    "Bluetooth & devices" became selected
COMPLETED: "Bluetooth & devices" became selected  (1 action(s) performed)
```

Every layer earning its place at once: the destination phrasing parsed
([[ADR-222-going-somewhere-is-one-request-however-you-ask]]), "settings" was set aside and the
destination roles tried first ([[ADR-218-a-request-reworded-in-the-screens-own-words]]), the
on/off switch was rejected as the wrong kind of control
([[ADR-219-a-match-is-not-an-answer]]), the capability ladder fell from Invoke to a double
click, and verification read the same selection evidence the satisfaction rule reads.

**The same request again, having arrived:**

```
resolve   list_item "Bluetooth & devices" (0.56)
state     "Bluetooth & devices" is already the one you are on, so nothing needs pressing
          — accessibility reported it
COMPLETED  (no action performed)
```

**And the measured Experiment-068 failure, from the other direction.** "Show me bluetooth
devices" on that page resolves to FOUR controls about equally well — and the answer is not a
question:

```
! resolve   ambiguous: 4 controls match "bluetooth devices" about equally well
  state     "Bluetooth & devices" is already the one you are on, so nothing needs pressing
COMPLETED
```

**Five phrasings, one destination.** From Personalization, every one of these parsed and
resolved to the right navigation item: `show me my installed apps` → Apps, `go to installed
apps` → Apps, `take me to bluetooth settings` → Bluetooth & devices, `navigate to accounts` →
Accounts, `switch to gaming` → Gaming. Before this session, only the ones beginning with the
word "open" reached resolution at all.

**And in Explorer**, pointed at a disposable folder: `open new` → `button "New"` (0.97), `show
me the view options` → `button "View"`, `go to notes.txt` → the file's list item.

## Four defects, found live

Two were in the work of this session and are fixed. Two are older, are characterised here, and
are not fixed.

### Fixed: the account said it was planning something it had decided not to do

The satisfaction check was placed after the plan. The live account read:

```
resolve   list_item "Bluetooth & devices" (0.56)
plan      open "Bluetooth & devices" (medium risk)
policy    allowed
state     already the one you are on, so nothing needs pressing
```

Three lines about acting, under a conclusion that acting was not called for. Nothing was
performed either way, so the defect was entirely in the account — and the account is what a
person reads. Moved to immediately after resolution: no plan, no policy verdict, no record.

### Fixed: a no-op success was reported as a failure

`ResultNothingToDo` fell through `classify`'s default and came back as `FAILED`, with a
non-zero exit status, under a sentence that was correct:

```
FAILED: "Bluetooth & devices" is already the one you are on, so nothing needs pressing
```

The headline said the opposite of the sentence beneath it. Mapped to `COMPLETED` — the command
did complete, and what distinguishes it is that it performed nothing, which the action count
already reports as zero.

### Recorded, not fixed: a menu item that disappears because you clicked it

`"make a new folder called Reports"` expands into the `create_folder` goal and ran:

```
[1/4] invoke the New menu    — verified   (12 menu items appeared)
[2/4] choose the Folder entry — unverified
    ! verify inconclusive: "Folder" is no longer in the world, so its selection
      could not be read
UNVERIFIED: step 2 of 4 could not be verified
```

**The folder was created.** Both steps did exactly the right thing; the menu closed, as menus
do when an item is chosen, and the verifier went looking for the item's selection state. Its
absence IS the evidence that the click landed, and reading it as inconclusive stopped the
program two steps from finishing — so the name "Reports" was never typed and Explorer was left
with a folder called "New folder".

This is the single clearest "feels unintelligent" moment in the session: the operation worked
and Marco said it could not tell, then stopped.

### Recorded, not fixed: a goal that needs a name refuses in the wrong words

`"make a new folder"`, with no name, answers *"I only understand click, focus, move, repeat and
text editing so far"*. Marco has a `create_folder` goal and an `explorer create folder`
procedure; what it lacks is the name the procedure requires. The honest sentence is "what
should it be called?" — a clarification, which is a status Marco already has.

## What still requires teaching

Paraphrase, exactly as [[Experiment-070-the-interpreter-knows-some-meaning-and-does-not-know-what-it-does-not-know]]
measured it. `themes` for Personalization and `wifi` for Network & internet are still ABSENT:
the words are not on the screen, and no phrase table fixes that. The learning loop is the
fallback that works, and this session did not need it once for a destination request.

## One thing worth saying about privacy

Navigating to the Accounts page put the owner's name and email address on screen as control
LABELS, and the verifier quoted one in an ordinary status line. Nothing was stored and nothing
left the machine. It is noted because the interpreter question in
[[ADR-220-asking-what-a-request-meant-and-believing-none-of-it]] sends labels to an external
program: that page is a concrete example of why that question has its own switch, is off by
default, and should stay local.

## Scoreboard, against the owner's four examples

| request | before | now |
|---|---|---|
| "Open Bluetooth settings" | the on/off switch, confidently | the page — and "already there" when it is |
| "Show me my installed apps" | not understood at all | the Apps destination |
| "Open Apps" while already there | a question with two wrong answers | "already the one you are on", nothing pressed |
| "Make a new folder" | — | the goal runs and creates it; verification stops it two steps early |

## Enforced by

The behaviour is held by the deterministic suites named in
[[ADR-221-a-question-the-person-already-answered]] and
[[ADR-222-going-somewhere-is-one-request-however-you-ask]]; this record is the live
confirmation, not the gate. Nothing here needs a desktop to re-run except the live session
itself.

## What the mutation gate said about the new guards

Eleven mutants: six killed, two refused by the compiler, three survived — and all three
survivors are the same thing, which is worth naming because it changed three comments.

Each is a guard whose property is enforced AGAIN somewhere else, so deleting it changes no
outcome:

- `settle`'s "the kind must fit exactly one contender" — the accept predicate already refuses a
  re-asking that comes back AMBIGUOUS, which is what two buttons both called Close produce.
- `settle`'s "and it must be one of the contenders" — nothing in either corpus produces a
  re-asking that resolves elsewhere, because the reduced query is an exact match on the
  contenders' own label and nothing outscores that.
- `Already`'s verb test — `AlreadyThere` asks it again of every control.

None was removed. Each comment now says that it is an early-out or defence in depth rather than
the guarantee, and names where the guarantee lives — because a comment claiming to be
load-bearing when it is not is how the real guard gets deleted later by somebody tidying up.
