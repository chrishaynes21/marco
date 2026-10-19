---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: msedge
fixture: "A disposable HTML menu in an Edge app window, 1280x860 at +80+60, captured twice by `director capture-game-session` (20 frames each, 0 inputs) and derived into a one-step `msedge`-scoped artifact by `perceptionreplay -frames -fixture GRAPHICS -fixture-asserted ...`. Five live cases run on the owner's own desktop, each with an independent witness in the window's own title."
result: "The first guarded, verified pixel action this project has sent. An actual input was emitted and the expected result was independently observed: the window's title went from `FXM 0 0` to `FXM 1 1` — one press received, the expected panel standing — while Marco separately verified 18 of 18 expected words over a full settled window. Five of five cases behaved as required. SEVEN defects were found live, five of them in the verification path, and every one of them would have reported a false verdict about a correct action."
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - internal/director/workflowrun/verify.go
  - internal/director/workflowrun/workflowrun.go
  - cmd/director/performworkflow.go
  - cmd/director/capturegamesession.go
  - docs/experiments/Experiment-089-four-terms-passed-and-nobody-pressed-the-button.md
---

# Experiment 090 — the first guarded pixel action, and the seven things in its way

## The question

Nothing in this project had ever sent a verified pixel action. The owner set the standard:

> the test must confirm an actual input was emitted and the expected result was independently
> observed. A successful plan, matched control, or command exit code isn't enough.

So Marco's own report cannot be the evidence — Marco is the thing being tested, and a run that
says `pressed` and `verified` is a run agreeing with itself.

## The witness

The test menu counts the presses it actually receives and publishes them, with the standing
panel, in its own window TITLE: `FXM <panel-index> <presses>`. The operating system reports that
title to something that is not Marco and has no idea what Marco intended. `pointerdown` on the
document rather than on the anchors, so a press that lands in the gap between tabs still counts
— a test that only counted presses on controls could not tell "Marco clicked nothing" from
"Marco clicked the wrong thing". Digits rather than words, because `observe.TextWords` discards
tokens under three characters and the witness must not contaminate the vocabulary it witnesses.

## The five cases

| case | asked for | witness | Marco | verdict |
|---|---|---|---|---|
| 1 correct menu | execute + verify | `FXM 0 0` → `FXM 1 1` | 18 of 18 words, 5 readings, 2.495s | VERIFIED |
| 2 wrong application | refuse | untouched | refused before anything was built | REFUSED |
| 3 reset and repeat | execute + verify | `FXM 0 0` → `FXM 1 1` | 18 of 18 words, 5 readings, 2.495s | VERIFIED |
| 4 foreground switch | refuse | `FXM 0 0`, **zero presses** | refused at the final boundary | REFUSED |
| 5 verification timeout | unconfirmed | `FXM 0 0` → `FXM 1 1` | 0 of 5 readings in 293ms | UNCONFIRMED |

Case 4 is the strongest. The pointer had already been moved to the control and was resting on it
when the foreground was taken away; the press was refused four seconds after a check that had
passed:

	the pointer was moved to the control and the world changed before the press, so nothing was
	clicked: msedge does not lead the desktop (other), and a click lands in whatever is in front

The witness agrees — zero presses — so a valid check at invocation did not become permission for
a press later. That is the time-of-check/time-of-use race the owner asked to see tested.

Case 5 is the one the witness makes interesting. The press WORKED — `FXM 1 1` — and Marco said
UNCONFIRMED, which is the right answer: it had 293ms and needed five readings. "Whether it
worked is unknown" is not "it failed", and here it is demonstrably not a failure.

## Case 2 is not the case that was asked for, and this is the honest account

The brief asked for *wrong menu → refuse*. That cannot be produced on this interface, for two
reasons that are both the design working:

- **The tab bar is shared.** Pressing GRAPHICS from the AUDIO panel really does open graphics,
  so a refusal would be wrong.
- **The first screen's identity is a person's claim, not a pixel match.** `--from`, or an
  answer, establishes it; [[ADR-186-a-person-can-say-where-they-are]] is why. Run without
  `--from`, BOTH the right menu and the wrong one refuse with *nobody could say which saved
  screen visual_1 is* — measured, as a control, in the same batch. An unconditional refusal
  proves nothing about the wrong menu.

So the refusal tested is the one that is release-blocking and that does discriminate: an
`msedge`-scoped artifact aimed at Notepad, refused by observed provenance, naming both
applications, with *Nothing was pressed.* Notepad's text was unchanged. It also proves the two
new window selectors do not launder authorisation: `--pid` and `--foreground` choose what to
look at, and the scope is still asked of the window acquired.

## The seven defects, in the order they were found

Each was found by running it, and five of the seven would have reported a FALSE verdict about an
action that worked.

**1. `--application` is ambiguous and the advice was unfollowable.** A controlled page in an Edge
app window could not be captured OR acted on, because the person's own Edge was showing a
floating `Search bar`. The refusal said to run `director windows --window-id`: a command needing
a daemon that was not running, naming a flag neither command had. Fixed with `--foreground`,
`--pid` and `--window-id` on both, resolved in order of exactness.

**2. The capture recorded the application it was ASKED for.** `--foreground` left the field
empty, and everything downstream treats it as provenance — `-fixture` makes it the derived
workflow's SCOPE. A scope taken from a flag is a scope taken from whoever typed it. Now
`v.Ref.Application`, the executable of the window actually acquired. Found by capturing 20
frames of the owner's Trackmania window labelled `trackmania` while the fixture window was the
target; the capture was deleted.

**3. A selector carrying two fields resolves nothing.** `windowref` refuses more than one
selector, so `{ProcessID, Application}` — carried so that a disagreeing name could not
resolve — resolved nothing at all. The check it reached for already exists and is stronger: the
scope gate, against observed provenance.

**4. Five readings after the action were one reading counted five times.** OCR answers about
four times a second against a loop that looks ten, so the last reading stands for the looks that
follow it, and the run counted it once per look. A press whose panel had not arrived yet was
CONTRADICTED 572ms later on "5 fresh reading(s)" that were five copies of the pre-press reading.
Fixed with `Sighting.WordsSeq`, an identity for which reading the words are, and the rule that
only a reading taken after the action is after-evidence.

**5. A contradiction was declared on evidence that had one side.** `enough` counted the after
readings only, so five after and NONE before read as "enough evidence, and it disagrees" — and
the first guarded press in this project's history was reported contradicted while the witness
said `FXM 1 1`. That is blaming the application for a reading Marco never took. A contradiction
now needs both sides; one side is UNCONFIRMED.

**6. And the run acted before it had ever read the screen.** Which made 5 unanswerable by
construction: `--from` established the screen on observation 1 and the press went out on
observation 2. A step with an outcome expectation now WAITS for one reading of the screen it
stands on, within the same patience every other not-ready case gets. Position is the whole fix:
placed after the authorisation section it waited correctly and the run still sent nothing,
because the exploration grant had been taken and `--max-actions 1` was spent on the observation
that waited — *the session authorised 1 action(s) and has used them all*, for an action that
never happened. A wait must cost no permission.

**7. A screen change big enough to be NAMED turned the verifier off.** The after-readings were
scoped to the same screen id. The press worked, the run itself observed `visual_1 -> visual_2`,
visual_2's words were the eighteen the step expected, and not one of them reached the verifier.
The check exists for the case where the screen does not change
([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]); a change large enough to mint
a screen must not be the one thing that disables it. Now scoped to the window generation, which
is the fact the rule needed, with the saved expectation's overlap doing the discriminating.

**And one correction to the fix for 7.** The restarting window verified on ONE reading, where
persistence says nothing at all — the right answer with the term switched off. A verdict either
way now needs a full settled window of `OutcomeSettleLooks` readings, which is how case 1 went
from "verified after 774ms on 1 reading" to "verified after 2.495s on 5".

## The transition inside the window

`OutcomeSkipReadings = 2` was measured offline on a recording read at every look. Live, OCR
answers about four times a second and a page took longer than two readings to redraw, so the
window straddled the change and the persistence term correctly called every word TRANSIENT. A
fixed skip cannot know when the response happened, so a reading bringing `OutcomeWords` words
the previous one did not have is taken as the transition itself and the window starts again from
there — through `OutcomeBetween`, over a window of one reading each side, so there is still one
statement of what "appeared" means. Bounded, because a screen that keeps changing substantially
would restart forever; past the bound the readings are taken as they come, and on a screen like
that the verdict is a refusal.

## What this does not establish

Nothing about learning. No person demonstrated this workflow; its step says ASSEMBLED and names
the asserted link between its two captures. The run's own report says it plainly, and keeps
saying it after a verified outcome: *arriving where a step expected is not by itself proof that
the step's action caused it.*

Nor anything about a game. The surface was a page this project controls, chosen so that a
failure would be attributable to wiring rather than to perception — and seven of them were.
Generalising to a real game needs a real game's recording with a press in it, which is
[[Experiment-089-four-terms-passed-and-nobody-pressed-the-button]]'s open end.

## Enforced by

`internal/director/workflowrun`, in `verifylive_test.go`, which exists because the first
mutation round over this work killed only 2 of 9 — every test in `verify_test.go` leaves
`WordsSeq` zero and so takes the path that existed before it did:
`TestOneReadingStandingForManyLooksIsOnePieceOfEvidence`,
`TestALateReadingOfAnOlderFrameIsNotAfterEvidence`,
`TestATransitionLandingInsideTheWindowRestartsIt`,
`TestAScreenThatKeepsChangingDoesNotRestartForever`,
`TestOCRNoiseDoesNotRestartTheWindowForever`,
`TestAStepWithAnOutcomeIsNotPressedBeforeItsScreenHasBeenRead` and
`TestTheWaitForAReadingIsBoundedByPatience`; plus
`TestAScreenChangeAfterTheActionIsStillTheActionsEvidence` and
`TestAnotherWindowsReadingsDoNotVerifyTheAction` in `verify_test.go`, and `cmd/director`
`TestPerformWorkflowInstallsItsGates`, whose installed-call list names both halves of the
reading identity. Ten mutations, all killed.

## See also

- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]]
- [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]
