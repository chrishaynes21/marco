---
type: adr
status: accepted
date: 2026-10-27
decision: "ACCEPTED — A PRESS IS A BUTTON AND A PLACE, AND IT IS ONE OPERATION. A right or middle press was refused before Save in any routine whose presses are checked, and the refusal's own words said why: *an aim and then a press. Wrapping the aim leaves the press unwrapped, and the press is the thing that acts.* That was a true statement about how the press was GENERATED — `do OS's Move with p1` and then `do OS's Click with \"right\"`, which presses wherever the cursor has got to — and it was never a statement about the press. The recorder had measured these all along: `newClick` takes the button as a parameter and reads the window the same way for all three, and `simplify` has carried `Button` on a click step since before any of this. What was missing was somewhere in the SAVED ROUTE to put the button, so `Point` gains `Button`, a text that is `\"left\"` when absent — which is every Point ever saved and every left-handed press generated since, so not one existing file changes — and a guarded press of any button is ONE `do OS's Click with p1`, wrapped by the chain exactly as a left press is. THE BUTTON IS NOT A MEASUREMENT and goes beside the COORDINATES rather than among the window terms: nothing about it is compared with the desktop before pressing, the contract reader's list of terms a guarded press must state is unchanged by it, and `Drag` has carried its button in that position since it existed. A HOST-MODULE CHANGE AND NOT A CORE ONE: no new word in the language, no new sentence shape, one field on a set `os.marco` already declares. AN UNKNOWN BUTTON NAME IS A REFUSAL, which is a defect this slice found rather than one it introduced: `mouseButtonFlags` answers LEFT for anything it does not recognise, so a route naming a button that does not exist — a hand-edited file, a typo, a term from some later recorder — sent a LEFT press at a correctly resolved, correctly guarded point. A press at the right place with the wrong button is a different act in most applications: one opens a menu and the other chooses something in it. 'Could not tell which button' is not 'left', so the name is read once, checked against the three a press can use, and a refusal names the value and goes into the account `marco do` reads — asked FIRST, before the window, because a button nothing can press is a fact about the route and a refusal naming the window would send somebody to look at their application for a defect that is in a file. AND THE PRESS NOW READS THE STOP, TWICE, WHICH IT NEVER DID AT ALL: the hold reads it, the keystroke reads it and the drag reads it twice, and a click dispatched after a run was cancelled went to the machine — `clickAt` returns the cancellation AFTER the send, so it was reported as a press that failed, having happened. It is read on entry, so a stopped run neither presses nor interrogates a desktop it was never going to press in; and AGAIN IMMEDIATELY BEFORE THE BUTTON GOES DOWN, because confirming the window reads a rectangle, a scale and a program name off a live desktop and takes time of its own — the ordering [[ADR-259-a-guarded-drag-is-two-points-in-one-window]] settled for a drag, applied to the press, since whatever is last before the press is what has to be read there. WHAT IS UNCHANGED: a press that measured nothing still emits `do OS's Click with \"right\"` and still presses where the cursor is, because that is the contract every legacy route was saved under and the recordings that produce it are not this contract's business; Director's own lowering (`internal/marcoexec`) still emits an aim and then a click, which is not a guarded press — those Points carry no window, so there is no check to leave on the wrong side of the aim — and folding it onto the new form is a change to Director's lowering with its own tests and no defect behind it; and no saved routine is regenerated. Studio's review says which button each press is and no longer tells anybody to 'record it again using ordinary left-clicks', advice that named the one thing that was not the problem. Four desktop scenarios RAN GREEN on 2026-10-27, 4 of 4 and exit 0 on the first run: two buttons arriving as themselves (`right_down` on Save then `middle_down` on Cancel), a moved app (+220,+130, still landing 45,14 into Save), a resized app refusing with nothing sent, and a Ctrl-held right press arriving as `right_down` with ctrl held by the application's own reading of the keyboard. They rest on the fixture's own log of what it RECEIVED, because no account inside Marco knows which button went down — `SendInput` with the wrong flag is a successful send — and because the application's own `save_presses` counter cannot corroborate them at all: WinForms raises Click for a left press and not for the other two, so the oracle that confirms every left-handed scenario here is silent for these by design. PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/osmod/os.marco
  - internal/codegen/codegen.go
  - internal/oshost/oshost.go
  - plugins/studio/recording.go
  - plugins/e2e/scenario_button.go
  - plugins/e2e/seed.go
---

# A press is a button and a place

## The refusal described the emission, not the press

[[ADR-251-a-press-checks-the-window-it-was-recorded-in]] made a guarded routine a **chain**: each
action that can fail to happen gets its own capability, calling the next only when its own action
went through. That works by wrapping ONE `do` sentence in arms that both end, so the list of
things that cannot be saved in a guarded routine is the list of things emitted as something else.

A right-click was on that list, and the entry read:

> a NON-LEFT CLICK is an aim and then a press. Wrapping the aim leaves the press unwrapped, and
> the press is the thing that acts. One guarded operation would need a button beside the point —
> a permanent word in the language for a case the owner's recordings have not needed yet, so it
> waits for a recording that needs it.

The first sentence was true of `emit`:

	do OS's Move with p1.
	do OS's Click with "right".

Two sentences. Wrap the first and the second still runs; wrap neither and the press lands wherever
the pointer has got to by then, which on a desktop with a hover-sensitive menu is not where the
aim pointed. The second sentence was the problem and the first sentence was the diagnosis of it.

What none of it was, was a statement about a right-click. **A drag left the same list for the same
kind of reason** ([[ADR-259-a-guarded-drag-is-two-points-in-one-window]]): it was refused because
"it resolves no offset, so it is checked against nothing", which described what the recorder had
measured rather than the gesture. Both entries read as facts about a kind of press and were facts
about this version's emission of one.

## What was already there

Everything except somewhere to write it down.

- The **recorder** measures a right press exactly as it measures a left one. `newClick` takes the
  button as a parameter, and the window — the basis, the offset, the size, the scale, the program
  — is read the same way for all three. It has been doing that since the window facts existed.
- **`simplify`** carries `Button` on a click step, from the press edge, and has since long before
  any of this.
- The **host backend** takes a button: `clickAt(ctx, button, x, y)` positions and presses in one
  atomic `SendInput` batch, with `mouseButtonFlags` translating the name into the two flags.
- The **guard** is about a Point and knows nothing about buttons. `resolve` → `guarded` →
  `confirmWindow` reads a window and compares five terms; which button is about to go down was
  never part of that question.

The gap was the saved file. `codegen` had the button and nowhere to put it, so it threw the place
away and emitted the button alone.

## The button goes on the Point, beside the coordinates

	the p1 is a Point with X 842, Y 274, Button "right", RelX 722, RelY 210,
	    RelTo "window", WinW 1920, WinH 1080, Dpi 144, Process "arkascended.exe".

	do OS's Click with p1...
	    when ok? ...

One sentence. One guard before it. One failure for the arm to stop at.

**Silence means left.** `Button` is written only when it is not `"left"`, which is not tidiness:
every Point ever saved is a left-click and says nothing about a button, and writing `Button "left"`
onto them would change the text of every guarded routine in existence for no change in meaning.
Each of those files is something a person reads, diffs and keeps. The host reads an absent button
as left, `os.marco` documents it as the default, and the generator leaves it out.

**It is not a measurement.** It sits with the coordinates rather than among the window terms
because nothing about it is compared with the desktop before pressing. That placement has a
consequence worth stating: `orchestrator.ContractOf`'s list of terms a guarded press must state is
unchanged, so a right press reads back as `guarded` with no new shape for any reader of the
contract to learn. The lesson of ADR-259's last section is that a new guarded SHAPE is a new shape
to every reader; a new field that is not a term is not one.

**It is a host-module change.** No new word in Marco, no new sentence form, one field on a set
`os.marco` already declares — `Drag` has carried `Button` in exactly that position since it
existed. Core is untouched.

## An unknown button is a refusal, and that is a defect this found

`mouseButtonFlags` has always answered LEFT for a name it does not recognise:

	switch strings.ToLower(button) {
	case "right":  …
	case "middle": …
	default:       return _MOUSEEVENTF_LEFTDOWN, _MOUSEEVENTF_LEFTUP
	}

So a route asking for a button that does not exist pressed the LEFT one, at a point the guard had
just resolved and checked. Every safeguard did its whole job and the act was the wrong act. A
press at the right place with the wrong button is a different act in most applications — one opens
a context menu and the other chooses something in it — and "could not tell which button" is not
"left", which is the same rule this boundary applies to a DPI nobody can read.

A route cannot be demonstrated into that state, because the recorder names three buttons. The
shapes that reach it are a hand-edited file, a typo, and a term from some later recorder, and none
of those arrives with a test — which is exactly why the check is here rather than left to the
shape of the inputs.

**It is asked first, before the window.** A button nothing can press is a fact about the ROUTE and
not about the desktop: whatever is on screen, this press cannot be performed as written, and a
refusal naming the window would send somebody to look at their application for a defect that is in
a file. The refusal names the value, and it goes into `Unperformed` — without that the step fails,
the chain's own arm ends the routine, and `marco do` reports success about a routine that pressed
nothing.

## And the press reads the stop, which it never did at all

Found while tracing what "validate before button-down, honour cancellation" means for a press: the
hold reads the stop, the keystroke reads it, the drag reads it twice, and **a click read it
nowhere**. `clickAt` ends with `return ctx.Err()` — after the send — so a press dispatched into a
cancelled run went to the machine and was then reported as a press that had failed, having
happened.

It is read twice now, and each reading has its own reason.

- **On entry**, which is `doKeyDown`'s reason — it holds for every backend, including the ones a
  test can drive, so "a stopped run presses nothing" is a claim about the host rather than about
  Windows — and one of its own: everything after it reads a live desktop, so a stopped run that
  got past it would be told its window had changed, which is a finding about somebody's
  application manufactured by a stop they raised themselves.
- **Immediately before the button goes down**, because the check in between TAKES TIME: it reads a
  rectangle, a scale and a program name off a live desktop. A stop raised while that ran found
  nothing between its own answer and the press. That is the ordering ADR-259 settled for a drag,
  and the press is the same shape one instant earlier: *whatever is last before the press is what
  has to be read there.*

## What is unchanged

- **A press that measured nothing.** Still `do OS's Click with "right"`, still pressed where the
  cursor is. That is the contract every legacy route was saved under — a click is attempted and
  the run carries on — and a recording that measured nothing still produces it. Changing it would
  change what a kind of route that has always worked does.
- **Director's own lowering.** `internal/marcoexec` still emits an aim and then a click for a
  non-left press. Those Points carry no window, so nothing there is guarded and there is no check
  to leave on the wrong side of the aim; folding it onto the new form is a change to Director's
  lowering with its own tests and no defect behind it. The comments that said the language could
  not do better were corrected.
- **No regeneration.** Every saved routine is the routine it was.

## What the desktop scenarios can and cannot see

Four ran, under `button/`, **green on 2026-10-27, 4 of 4 and exit 0 on the first run**
(`plugins\e2e\run.cmd -only button`). They carried `onlyWhenAsked` until that run — the shape the
holds set and the drags followed — and the gate came off in the commit where they passed.

Passing first time is worth a sentence, because the two families before them did not: the holds
took two runs and the drags four. What that says is about this slice rather than about the
harness. The press, the guard, the window check, the stop and the fixture's own log were all
exercised here already; what this contract added was a button on a Point. The arrangement was not
new — only the fact being asserted was.

They rest on the fixture's own log of what it RECEIVED, and they have to: **no account inside
Marco knows which button went down.** The run record says a routine performed; the host's own
count says how many input actions it asked for. `SendInput` with the wrong flag is a successful
send, so a left press delivered where a right one was demonstrated is, from every account Marco
has, a run that worked.

And the application's own state cannot corroborate them either. `save_presses` is the fixture's
count of its Save button being CLICKED, which WinForms raises for a left press and not for the
other two — so the oracle that confirms every left-handed scenario in that harness is silent for
these **by design**. Asserting it would be asserting something that cannot change; asserting its
absence would be asserting that WinForms is WinForms. The event log is the whole evidence, and
each scenario says so where it uses it.

**Physical recording capture remains NOT EXERCISED.** Every one of these plays a seeded
demonstration; what they establish is that a right press ARRIVED as one.

## Enforced by

- `internal/codegen` `TestAMeasuredNonLeftPressCarriesItsButtonAndItsWindow` — the saved line,
  whole, for right and middle: the button beside the place with every window term intact.
  `generated` compiles AND runs it, which is the only way a new field on a set is shown to be
  assignable rather than merely parseable.
- `internal/codegen` `TestALeftPressStillSaysNothingAboutAButton` — the compatibility promise, and
  the reason `buttonTerm` returns "" for left.
- `internal/codegen` `TestAMeasuredNonLeftPressIsAGuardedLink` — the switch, asserted on the shape
  of a LINK (`do … ...` followed by `when ok?`) rather than on the press being in the route.
- `internal/codegen` `TestAGuardedNonLeftPressIsNotAnAimAndThenACursorClick` — the case every
  other assertion here would pass without: the two-sentence shape is GONE rather than merely
  unused.
- `internal/codegen` `TestAnUnmeasuredNonLeftPressIsUnchanged` and
  `TestARightClickAndADragAreGeneratedWhenNothingAsksToBeChecked` — what a legacy route still is.
- `internal/codegen` `TestANonLeftPressThatCouldNotBeMeasuredDeclaresItsWindowAndItsButton` — the
  third contract with a button on it: the basis without the offset, which replay refuses.
- `internal/codegen` `TestARefusedNonLeftPressStopsWhatFollowsIt` — behavioural, through the real
  generator and runtime: the press after one that was refused is never asked for.
- `internal/codegen` `TestAStepThatCannotStopTheRoutineIsRefused` and
  `TestARecordingOfOnlyAnUnwrappablePressIsRefused` — both repointed at a press that is still
  unwrappable, because both were posed with a right-click.
- `internal/oshost` `TestADemonstratedNonLeftPressArrivesAsThatButtonWhereTheWindowIsNow` — the
  recording → simplification → generation → saved file → runtime → host chain, asserting the
  button the machine was asked for AND the pixel, since either alone is satisfied by something
  wrong.
- `internal/oshost` `TestTheButtonIsNotCrossedWithTheTarget` — two presses, two buttons, two
  places, in order. A host that read the button once, or a generator that reused one Point, passes
  every single-press assertion.
- `internal/oshost` `TestANonLeftPressIntoAWindowOfAnotherSizeSendsNothing` — the guard applies
  unchanged, with nothing sent and the account naming the term.
- `internal/oshost` `TestANonLeftPressTheMachineWouldNotDeliverStopsTheRoutine` — counted as
  SENDS rather than as clicks, because a send the machine rejected delivers nothing and an
  assertion on the clicks would pass for a run that tried both presses.
- `internal/oshost` `TestAPressAskingForAButtonNothingCanPressSendsNothing` — both input shapes,
  nothing sent, the value named, and the refusal in the account the command reads.
- `internal/oshost` `TestAPressThatSaysNothingAboutAButtonIsALeftPress` — beside it on purpose: a
  check that refused what it did not recognise would refuse the empty string and stop every
  routine in the library.
- `internal/oshost` `TestAStoppedRunDoesNotPressAndDoesNotAskTheDesktop` — the gate on entry, and
  that no desktop question is asked for a press that will not happen.
- `internal/oshost` `TestAStopRaisedWhileTheWindowWasBeingCheckedPressesNothing` — the moment, not
  the mechanism: the stop arrives DURING the confirmation, against a HEALTHY window, so the
  refusal can only be the stop.
- `plugins/studio` `TestARightOrMiddlePressIsSavedAndTheReviewSaysWhichButton` — it saves, the
  review names the button in that press's own line, the file carries the button with the window
  terms, and `ContractOf` reads it back as guarded. This test asserted the opposite before.
- `plugins/e2e` `TestTheButtonScenariosRunLikeAnyOther` — they are in every default run, and the
  name claim that outlived the gate: not under `window/`, asked of every registered family. It
  asserted the gate until the run below, which is the holds' shape.
- `plugins/e2e` `TestAPressThatArrivedWithAnotherButtonIsAFinding` — the claim this family rests
  on, exercised headlessly, including the press that would have passed before any of this existed:
  everything right except the act.
- `plugins/e2e` `TestEveryButtonScenarioAsksWhichButtonArrived` — the wiring, by AST, because
  these four run only on a desktop: a scenario that stopped asking would go on passing while
  reading the log for everything except the fact it exists for. Either question counts — the
  button that arrived, or whether anything did — because there is no button on a press that never
  happened, and that pair is a rule rather than an exemption.
- `plugins/e2e` `TestAShortOrLongLogIsAFindingOfItsOwn`, `TestAPressOnAnotherControlIsAFinding`,
  `TestTheEvidenceLineNamesTheButton`, `TestASeededPressThatLostItsButtonIsRefused` — the count
  asked before anything is read by index, the place per press, the report that names the button,
  and the seeding precondition that keeps all four scenarios from being vacuous.
- `plugins/e2e` `button/a right and a middle press arrive as themselves`,
  `button/a moved app still gets the right press`,
  `button/a resized app refuses the right press, sending nothing`,
  `button/a ctrl-held right press arrives with ctrl held` — **RAN GREEN ON A DESKTOP ON
  2026-10-27**, 4 of 4, exit 0 (`plugins\e2e\run.cmd -only button`). The application received
  `right_down` on Save at 258,372 and then `middle_down` on Cancel at 358,372; the moved window
  went +220,+130 and the right press still landed 45,14 into Save; the resized one was refused
  with nothing sent at all; and the Ctrl-held press arrived as `right_down` with `ctrl` held by
  the application's own reading of the keyboard.
