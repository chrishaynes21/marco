---
type: adr
status: accepted
date: 2026-10-20
decision: "ACCEPTED — a person authorises an Act ONCE, by pressing Play on it, and the run performs its named interactions without asking again. The authorisation is an explicit GRANT: the Act, the demonstration revision somebody was shown, the digest of the play its interactions were bound to, WHAT EACH NAME MEANS as a demonstrated position, the application it is pinned to, the interactions by the names the person gave them, who gave it and when. The mapping is the term the others cannot carry: the same Act recomposed over the same demonstration with the same names in the same order on different interactions leaves the revision, the names, the order and the play's own digest all unchanged, and changes every control that would be pressed. It is checked against the preflighted plan before anything is observed, it permits exactly those interactions and refuses any other press mid-run, and it is recorded as the authority in the run record — nothing simulates a keypress and no provenance rule is relaxed. The keypress ceremony it replaces is gone: no start countdown, no screen-identification question, no agree-then-authorise pair per interaction. Everything that made a supervised press safe stays exactly where it was, in the runner, because none of it ever asked anybody anything: the occlusion refusal, the world re-check at the instant of the press, the pointer-arrival measurement, the outcome gate between interactions, the stop signal, the deadline and the action cap. The screen question is replaced by EVIDENCE — the shape as a gate at `observe.VisualScreenMatch`, the words as the discriminator at `observe.TextKept`, and no second saved screen allowed to fit — and when that evidence does not identify the screen the run PAUSES with a sentence a person can act on, rather than refusing or guessing. Bringing the application forward belongs to the surface, because Windows refuses it to anybody else."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/actgrant.go
  - cmd/director/performworkflow.go
  - internal/director/workflowstore/standing.go
  - internal/director/workflowstore/anchor.go
  - internal/director/runstate/runstate.go
  - plugins/studio/play.go
  - plugins/studio/said.go
  - plugins/studio/raise_windows.go
  - plugins/studio/acts.go
---

# ADR-244 — pressing Play is the authorisation

## What was handed over, and the verdict on it

A person who wanted to run something they had demonstrated had to: tick a box that said what live
meant, type the executable name, tick a second box about reading the screen, press Run, find their
game, bring it forward without clicking anything in it, race a five-second countdown, press F7 to
say they were ready, answer a question about which saved screen they were looking at by agreeing
with an arrangement score, and then answer two more questions per interaction — one to agree the
step belonged in the run, one to permit the press.

The owner's verdict, after being handed the checklist for it:

> That is an engineering acceptance checklist handed to you as a product flow. You're being asked
> to manage windows, race a countdown, understand artifact positions, inspect JSON, and kill
> processes before finding out whether Marco is useful.

And the Architect's disposition, having first approved the handover:

> I treated the CLI's supervision mechanism as a product requirement. That was the wrong call.
> Clicking Play should authorize the selected Act. […] The safety checks stay in the runner. The
> repeated keypress ceremony goes. This changes the authorization contract, so engineering needs
> to implement it explicitly — not just auto-answer the existing prompts.

The last sentence is the whole of why this is an ADR and not a UI change.

## 1. The ceremony was never the safety

This is the claim everything else rests on, so it is worth being exact about. Every guard that
makes a supervised press safe is in the runner, and not one of them asks a person anything:

| Guard | What it refuses | Where |
|---|---|---|
| `Sighting.Usable` | a look that may be pixels of whatever covers the window | the loop, per look |
| the world re-check | a press whose look disagrees with the screen the point was approved on | `stillThere`, at the instant |
| the pointer check | a move that reported completed and did not happen (ADR-198) | after the dwell |
| the outcome gate | the second interaction, until the first has a verdict (ADR-242) | the coordinator |
| the stop signal | everything, at the last moment it could matter (ADR-240) | the run's context |
| the deadline and cap | a run that outlives its budget, an Act that presses more than it says | the command |

What the three questions added was a person's agreement — and they had already given it. They
composed the Act out of interactions they picked and named, they chose which Act to run, and they
pressed the button. Asking again per press is not a second safety term; it is the same term,
collected four more times, from somebody who has gone to look at their game.

`TestAGrantedRunStillWaitsForEachOutcome` is the test that says this plainly: the same
two-interaction Act, through the production coordinator, verifier and runtime, authorised by a
grant and asked about by nobody — and the second press still does not happen until the first
interaction's verdict has arrived.

## 2. The grant, and its four narrowings

An `actGrant` is one person's decision, at one instant, over named things:

```
the Act, by the name they gave it
the revision of the demonstration its interactions came from
the application it is pinned to
the interactions it performs, by the names the person gave THEM
who gave it, and when
```

ADR-183 refuses a session flag. This is narrower than one in five ways at once — **one Act, one
revision, one play, one meaning per name, one run** — and each of them is a check with a test:

- the Act, so a grant cannot be spent on a different one (`TestAGrantForAnotherActStopsTheCommand`);
- the revision, so an Act composed again between the press and the run is refused rather than
  performed — the person authorised the thing they were shown, and re-composing changes which
  interactions those names mean;
- the application, so an Act pinned to one executable cannot be performed against another;
- the order, because a grant is given over a sequence somebody read. The same two interactions the
  other way round are a different thing to have agreed to: the second press is made on the screen
  the first one produced.

### And what each name MEANS, which none of the above can carry

The first version of this ADR shipped with the four terms above and a hole the Architect found by
reading them: an Act can be composed again over the **same demonstration** with the **same names**
in the **same order**, bound to **different interactions**. Then the revision is the
demonstration's and the demonstration did not move; the names and their order are what the person
typed; the application is the demonstration's; and the generated play is *byte-identical*, because
its sentences are those names. Everything a person can see is unchanged and every control that
would be pressed is different.

That premise is not argued, it is measured:
`TestRecomposingTheSameNamesChangesNothingTheOldTermsCouldSee` composes one demonstration both
ways round through the production composer and asserts the two plays are identical, the bindings
record the same digest, and the names and revisions match — with only the positions differing. If
that test ever fails, the hole has closed itself.

So the grant carries the mapping — `name → demonstrated interaction` — and the play's own digest,
and `agrees` checks both against the preflighted plan. A grant given for "the graphics tab"
meaning interaction 0 is refused when that name now means interaction 1, before anything is
observed.

**Required, not optional.** `readActGrant` refuses a grant that omits the revision, the digest or
the mapping, and refuses one whose mapping does not name the same interactions as its order. An
optional identity check is one that can be skipped by leaving a field out, and two lists of one
fact is the shape this repository already keeps a note about: they agree when they are written and
disagree the first time one changes.

### And the window between selecting an Act and performing it

The same replacement can happen between a person reading the list and pressing the button, so the
page hands back the identity it was **displaying** and Studio refuses when the Act on disk is no
longer that one — sending them to read it again, which is the only honest instruction. Empty when
they typed a name instead of clicking a listed Act: the page was not showing them anything about
it, and says so rather than pretending.

**The whole identity, and not the play digest.** The first version of this handed back the digest,
which is the one term the dangerous recompose leaves untouched — the same argument as above, and
the same measurement. So that check agreed with itself in exactly the case it was written for.
What travels now is the digest, the revision, the application and what every interaction name
means, in run order (`identityOf`), and anything that moves one of those is a different string.

**One read, carried through.** Studio used to read the Act twice: once in `Play`, to check it and
refuse it, and again at the launch, to write the grant from whatever it found then. The
authorisation came out of the second read, so an Act replaced between them was caught by neither
check — it became the authorised version, and the grant and the plan the runner preflights would
have agreed with each other about an Act nobody had read. The snapshot `Play` checked is now what
travels into the grant and what aims the run at an application; `Performance.Grant` is the Act
itself rather than a flag beside its name, because a flag left the reading to the other end.

The interval itself cannot be closed from here — it is a filesystem, and two commands can replace
an Act in it. What is closed is which version a replacement can authorise: none. `demonstrated.Save`
refuses to write over a play that exists, which narrows it further — an Act cannot be recomposed
in place — but not to nothing: the pair can be removed and composed again, and the bindings are a
file somebody can edit while the play's digest stays what it was. Past that point the runner is
the one that refuses: `TestAnActReplacedBetweenPlayAndStartupIsRefused` does it the way a person
would, through the command, and asserts the refusal lands before any observation and leaves no
record.

### What it is not

It is **not a simulated keypress.** Nothing in this path touches `navsource`, nothing synthesises
F7, and the human-input provenance rules are unchanged. This matters for the audit trail as much
as for the rule: `workflowrun.Authorisation.By` reads "the person who pressed Play in Studio at
14:03:22, authorising "the audio tab"" and never "the person at the keyboard (F7)". Those are two
different claims about how a press came to be allowed, and the record keeps them apart —
`RunRecord.Authority` is where a granted run's answer lives.

It is also **not longer-lived than a keyed authorisation.** The authorisation returned per press
is stamped with the observation and window the intent was about and expires in ten seconds,
exactly as the desk's did. A grant given before the run does not reach past that: it cannot
authorise a press in an observation it was not made in. The runner cannot tell the two granters
apart, which is the point — every check it makes on an authorisation is unchanged.

## 3. The screen is identified, or the run pauses

The screen question was the most dishonest of the three. A widget has no terminal to type a label
in, so the ranking was read out and one function key accepted its top candidate: a person agreeing
with an arrangement score they have no way to check.

So the keypress is gone and the evidence that replaces it is in
`workflowstore.SessionBinding.Standing`, with three terms:

	1. the wanted screen agrees with this look at `observe.VisualScreenMatch`   the existing threshold
	2. and a fresh admissible reading POSITIVELY identifies it                  `kept >= observe.TextKept`
	3. and no OTHER saved screen clears both of those                           the margin

**The arrangement is a gate and never the discriminator**, and the measurement is the argument.
Over all 36 pairs of one artifact's nine screens, three pairs sit within 0.05 of joining by
arrangement, five are already over the threshold, and the closest pair is separated by 0.010 —
while the measure's own variance between a recording pass and a live pass of the SAME two screens
is 0.181. The words of that pair agree at 0.231 against 0.721 by shape: a margin of +0.319.

Term 2 is deliberately stronger than `Reached`'s third term, where a reading that merely fails to
contradict a destination is admissible. There, a press has already happened and a demonstrated
step led there. Here nothing has happened yet and a press is about to be made without anybody
being asked again, so "nothing says otherwise" is not enough.

Term 3 is what the keypress was really being asked for. Two sibling panels under one tab bar are
the case the owner's own game presents; if both clear the gates then nothing in the evidence
tells them apart, and an Act's press must not be made on "one of these two".

### The ground is its own word

`GroundIdentified` — "nobody was asked, and the screen said which one it is" — beside `confirmed`,
`inferred` and `reached`. It would have been one line to call this a confirmation and reuse
`Confirm`, and it would have put "a person said yes while looking at it" in the account of a
question this contract deliberately does not ask.

### And a run that cannot identify it PAUSES

A new phase, `runstate.Paused`, with the reason written to be read by the person who pressed Play.
Not an ending and not a refusal: the evidence is re-read on every look, so the pause resolves by
itself the moment the person is on the screen their Act starts on — nobody presses anything to
resume. Stop stays available throughout, and the surface must not render it as a failure.

This is also the honest reply to the one thing that got worse. Under the old contract a person
could force a screen by answering the question; now, if the evidence cannot identify it, the run
waits instead. An Act over a demonstration whose screens carry **no words at all** — every
demonstration recorded before `Control.Text` and `Screen.Said` were written — can therefore never
be performed under a grant. It pauses, saying so, and the demonstration is what needs redoing.
That is a deliberate refusal to press on weak evidence, and `TestAWordlessScreenPausesInsteadOfPressing`
holds it.

## 4. Bringing the application forward belongs to the surface

Windows refuses `SetForegroundWindow` to a process that does not already own the foreground. It
was measured on this machine from a harness trying to raise a window from outside: the call
returned false and the foreground did not move. The supervised runner is a child process nobody
is looking at, so it can never raise anything.

Studio can, at exactly one instant — the person just clicked a button in it — and that is when it
does. The same split `plugins/fixture` already draws in its own words: the surface may put the
window in front, and Marco's own foreground guard is what CHECKS that it is there at the instant
of execution.

Three things this deliberately does not do:

- **No focus-stealing tricks.** Attaching to another thread's input queue, or synthesising an Alt
  keypress so the process looks like it received the last input, would get around the lock. The
  second is simulated human input, which is the one thing this whole product's provenance rule is
  about, and a window appearing is not a reason to break it. If Windows says no, the run pauses
  saying it cannot see the application.
- **No moving or resizing.** The demonstration records no window geometry — control regions are
  fractions of whatever the window was — so there is no size to restore, and resizing a game makes
  it re-lay-out its own interface until those fractions no longer land on the control. "Arrange
  its window for reliable capture" is, honestly, bring it forward and require it uncovered.
- **No raising from the runner**, even as a fallback, because it cannot work and a call that
  silently fails would read in the log as something having been tried.

A refusal to FIND the application is a different thing and is answered before anything starts:
nothing can be performed against an application that is not running, and saying so in the window
somebody is already looking at costs them nothing.

### A minimized window is restored, not complained about

The first version chose between windows before raising one and skipped minimized candidates, then
told the person their application "does not seem to be running, or its window is minimised — bring
it up and press Play again". Two things wrong with it: that is the surface asking for something it
can do itself, and `raiseWindow` restores before it activates — so the restore branch was
unreachable for exactly the windows it was written for.

Minimized windows are candidates now. What the choice prefers is a window already on screen, and
then the biggest: an executable owns a launcher, a splash and the main window, and the one
somebody is looking at is the one with the pixels. A minimized window's reported bounds are not
evidence of a size on Windows — an iconic window reports an off-screen position and whatever it
will restore to — so it loses to anything that is up, and minimized windows are compared with each
other on the same arithmetic because there is nothing better to compare them on.

The restore itself is a syscall and no Go test can watch it happen. What is held is that the
decision reaches it, with the handle of the window that is down.

## 5. What a person does now

	choose an Act          its interactions shown by the words they read on the screen,
	                       and whether each can be checked afterwards
	press Play             or "Try it without clicking"
	watch                  progress per interaction, in the Act's own numbering and names
	Stop                   at any point, including while it is paused

The engineering contract is kept, unchanged, behind a disclosure in the same window and driven by
E2E.md's acceptance checklist: every press asked for by key, the screen named by key, the
application typed by hand. Two contracts, one runner, and the surface says which is which.

## 6. What is still not done

- **The per-press scope check is defence in depth and says so.** `grantedGranter.Authorise`
  refuses a press whose aimed interaction the grant does not name, and while preflight holds, the
  name case cannot fire in production — `agrees` has already compared the grant with the plan's
  own order. The index-zero case can fire: an aim that outlived its interaction names nothing. It
  is kept, tested directly, and labelled rather than presented as load-bearing, because an
  unreachable discriminator that looks reachable is its own kind of defect.
- **No live trial.** Nothing in this ADR has been performed on a real screen. The deterministic
  tests say the contract holds through the production coordinator with scripted looks; what they
  cannot say is whether the pause reasons read as actionable to somebody standing in their game,
  or whether `Standing` identifies the owner's settings panels at all — the one demonstration on
  this machine that carries words carries them on its CONTROL, and whether its screens do is a
  question only a run will answer.
- **The surface's own pause does not say how to fix a wordless demonstration.** It says what the
  run said. Turning "screen_c was saved without any words" into "re-demonstrate this, pausing
  after each click" is a product sentence nobody has written yet.

## What came of it

The first live trial of this contract worked and pressed nothing. The grant was read, the run
said `starting` at once, nothing was asked of anybody, and the record names the authority as the
person who pressed Play — and then it paused on every look for thirty seconds, correctly, because
the Act's first interaction starts on a screen the demonstration had read no words on, and
refused with nothing sent. Two things were wrong on the surface's side of the line, both of them
now closed: Studio raised the window and ignored the answer, and the Act was listed as `ready`.
[[ADR-245-one-task-at-a-time]] is what replaced the window that said that.

## Enforced by

- `cmd/director/actgrant_test.go` — `TestAGrantThatIsNotADecisionIsRefused` (six ways a surface
  can hand over something that is not a decision, plus a missing file);
  `TestAGrantIsForTheActSomebodyWasShown` (the narrowings, one case each);
  `TestTheSameNamesBoundToDifferentInteractionsAreRefused` (the mapping term, and that the other
  four all pass for the same case — so it is the term doing the work and not a second copy of one
  that already held);
  `TestRecomposingTheSameNamesChangesNothingTheOldTermsCouldSee` (the premise, through the
  production composer);
  `TestAnActReplacedBetweenPlayAndStartupIsRefused` (the whole window, end to end through the
  command: composed, authorised, recomposed the other way round, refused before any observation
  and with no record kept);
  `TestAGrantAgreesWithTheApplicationTheActIsPinnedTo`;
  `TestAPressTheGrantDoesNotNameIsRefused` (the edge at the press: permitted, stamped, expiring —
  then refused twice, with nothing sent);
  `TestAGrantForAnotherActStopsTheCommand` (the call site, exit 1, no stream opened);
  `TestAGrantedRunStillWaitsForEachOutcome` (two interactions through the real coordinator under a
  real grant: both permitted, none refused, the second not pressed until the first had a verdict,
  no question in the stream);
  `TestAGrantedRunAsksNobodyAnything` (the production command: no question and no asking phase in
  the stream, `starting` first, no keypress invited on the terminal, the record naming the
  authority, and a pause with a reason).
- `internal/director/workflowstore/standing_test.go` —
  `TestTheScreenAnActStartsOnIsIdentifiedWithoutAsking` (both terms, the ground, and the authority
  recorded); `TestTwoScreensThatBothFitEstablishNeither` (the case the keypress was covering);
  `TestAScreenOnlyUncontradictedDoesNotCarryAnActsPress` (the deliberate difference from
  `Reached`); `TestAContradictedScreenIsNotWhereAnActStarts`; `TestALookThatIsNotTheScreenSaysSo`;
  `TestAWordlessScreenPausesInsteadOfPressing`; `TestAPersonsOwnAnswerStands`.
- `plugins/studio/play_test.go` — `TestPlayHandsTheRunnerAnAuthorisationForTheChosenAct` (every
  field of the grant the child was really given); `TestTryingItWithoutClickingDoesNotPassLive`
  (and `--reading` always on); `TestPlayRefusesWhatItCanAnswerBeforeStartingAnything`;
  `TestAnActWhoseDemonstrationMovedIsRefusedWithTheReason`;
  `TestAnApplicationThatIsNotRunningIsSaidSoBeforeAnythingStarts` (the production
  `bringForward`); `TestAnActComposedAgainSinceItWasListedIsRefused` (the selection window, both
  ways: a stale identity refused, the listed one performed);
  `TestTheSameNamesMovedToOtherInteractionsIsRefused` (the recompose a digest cannot see, with the
  premise asserted in the same test: same digest, same revision, same names and order, different
  meanings);
  `TestAnActReplacedBetweenPlayAndTheAuthorisationIsNotAuthorised` (the second read, through the
  seam that stands inside the interval: the grant carries what was chosen, not what replaced it);
  `TestAMinimizedApplicationIsRestoredRatherThanRefused` (the decision reaches the restore with
  the right handle); `TestTheWindowRaisedIsTheOneSomebodyIsLookingAt` (six cases of the choice);
  `TestEveryOutcomeIsSaidInWords` (all eight);
  `TestWhatTheWindowShowsSaysWhatHappened`; `TestThePageIsTheFlowThatWasAskedFor` (seven source
  guards over the page).
- `cmd/director/performworkflowwiring_test.go` — `grant = byGrant` and
  `byGrant.agreed(a.Step, session)`, both held as source over the production assembly because the
  tests that drive the contract supply the granter themselves and cannot see whether the command
  would have used it. Both failure modes are quiet: a run waiting on a pipe nobody is at, and a
  run that performs the presses a person authorised while reporting that nobody agreed to them.
- `internal/director/observe/arrangement_test.go` — `standing.go` added to the boundary
  allowlist with its reason, which is the guard demanding that promoting a session-local screen
  anywhere near the durable store be a decision somebody takes on purpose and says why.

## Links

- [[ADR-243-a-door-of-its-own-and-the-order-in-the-file]]
- [[ADR-242-the-second-action-waits-for-the-first-ones-answer]]
- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]]
- [[ADR-186-a-person-can-say-where-they-are]]
- [[Experiment-094-four-failures-no-mechanism-and-a-window-that-has-to-hold-still]]
