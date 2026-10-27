---
type: subsystem
status: active
owners:
  - engine
depends_on:
  - demonstrations
used_by: []
updated: 2026-10-25
source_paths:
  - plugins/e2e
  - plugins/fixture
  - internal/acceptance
---

# Desktop Harness

How a change to Marco is checked against a real desktop without a person operating the computer.

There are two harnesses, they answer different questions, and both run only when asked.

`plugins/e2e` is the **automated regression gate for the scenarios below** — reviewed and closed at
`5eccbcb`, and a change to Studio's product path is expected to be run against it. It does not
replace a person at the keyboard for what it cannot reach, and the two things it cannot reach are
stated in every report and in their own sections here: the FULL physical-recording path is NOT
EXERCISED — a gesture Windows captured from somebody's own hand, through the hooks, into a draft —
and the drain barrier establishes that the fixture's queue was empty rather than that Windows had
finished handing input to it. Review, Save, Record again and the replacement refusals are not
themselves manual: parts of each are covered by the scripted-recorder layer below. It is the
physical gesture at the start of them that no automated run can supply.

| | question | entrance |
|---|---|---|
| `internal/acceptance` | can **Marco** use a computer — perceive, interpret, ground, authorise, perform and verify | a gated Go test (`MARCO_ACCEPT_DESKTOP`) |
| `plugins/e2e` | does the **product path through Studio** work — page, binding, backend, library, generated Marco, child executor, native input, window guard, run record | `plugins\e2e\run.cmd` |

Both point Marco at the same application: `plugins/fixture`, a WinForms window with a fixed title,
a fixed layout, fixed control names and no update channel, so that a failure against it is a
finding about Marco rather than about somebody else's software.

## The application keeps two accounts of itself

	state   fixture-state.json   what the window HOLDS — fields, selection, per-button press
	                             counts, and its own geometry, DPI and control rectangles
	events  fixture-events.jsonl what the window RECEIVED — every press and key, where it
	                             landed, on which control, and what the window was at that
	                             instant

The second is the stronger one and it is why a coordinate replay can be checked at all. A press
that misses Save by four pixels and lands on the panel behind it leaves the state file exactly as
a press that never happened does; in the log it is a `left_down` on the wrong control, with the
coordinate that produced it. It comes from an `IMessageFilter` that sees each message before the
controls do and returns false always — a witness that swallowed a message would be an application
behaving differently because it was being watched.

	MARCO MUST NEVER READ EITHER.

That is the whole basis of both harnesses, and it is held by a test over the tree rather than by
custom: `internal/acceptance` `TestMarcoCannotReadTheOracle` scans every `.go` and `.cs` file for
both names and permits only the fixture that writes them and the two harnesses that read them.
Anything Marco can reach that knew about these files would turn "Marco pressed Save" into "Marco
can read a file", and every scenario would still pass.

## What the desktop harness does, in order

1. **Refuses, before anything opens.** A remote session, no screen, or somebody else's fixture
   already running. Each is NOT RUN with the reason — never a failure, because none of them is a
   condition a change to Marco could satisfy. Another Studio running is NOT a refusal: every
   window the harness reads or presses is addressed by the process id of the child it started.
2. **Builds a bundle.** `marco.exe`, `director.exe` and `studio.exe` (with the Wails tags) into a
   directory of this run's own, with `fixture.exe` and `uia.exe` copied beside them, and every
   binary's digest and the tree's revision written down. Studio spawns the `marco` beside
   ITSELF, so they have to be together — that is the trap CLAUDE.md spends a paragraph on.
3. **Seeds a routine** through the production generator — `Deps.DraftOf`, `simplify`, `codegen`,
   `routes.Registry` — from events measured off the real window. Seeded, never recorded.
4. **Drives the real Studio** through UI Automation: the DOM id is the element address, `Invoke`
   runs the page's own handler, and a snapshot is the operating system's account of what is on
   screen.
5. **Waits for the fixture to say it has nothing left to process** before reading its log for a
   count or an absence. See below: a run finishing says nothing more will be SENT, which is not
   the same as the application having seen what it already has.
6. **Compares two accounts** and reports the FIRST place they disagree, from the act outwards.
   Everything after the first disagreement is consequence, and listing consequences as findings is
   how a report becomes something nobody reads.

## The drain barrier, and why an absence needs one

The fixture's log is written when its message loop DISPATCHES a message. So a press sent to a
fixture that is behind exists while the log does not mention it — and every scenario whose claim is
*nothing arrived* reads that log right after the sender stopped. Reading it there reports the guard
as having refused when a press is sitting in a queue, which is the one direction that fails
quietly. A longer wait is the same sentence with a bigger number in it.

So the fixture answers: the harness writes a token, and the fixture writes it back once its pump
has gone **idle** after reading the request — an idle meaning the queue held nothing, so whatever
was in it has been dispatched and written down. An idle and not an acknowledged message, because a
posted message is retrieved *ahead* of queued input and a sentinel would come back with the presses
still behind it.

It establishes that the fixture's queue was empty. It does **not** establish that input the system
had accepted but not yet handed to that queue had arrived — a system thread does that handoff and
nothing in user mode orders against it. The barrier closes the gap a slow fixture opens, which is
the one a harness can be slower than, and the code says so where it is defined.

`checkTheBarrier` proves it before the first scenario, against a fixture holding a press its own
loop has not seen: invisible before, present after — and in between, the fixture **decides what to
do about the request while the press is still pending, and decides not to acknowledge it**, which
is the step that tells a barrier apart from an echo. Mutating the idle requirement out of the
fixture ends the run at exit 3.

That middle step took three goes to aim, and both misses are worth knowing about because both
looked right. The first read the request somewhere answering it would have been harmless — from
the fixture's timer, which cannot run while the loop is held, so the press had already been
dispatched. The second read the silence one pass before the answer was decided: `Drain` consumes
the request on one pass and compares idles on a later one, so waiting for the request file to
disappear released the held loop before an acknowledge-on-receipt barrier had reached the line
where it answers. Both passed the mutation in their usual words — the second one measurably, as
soon as the fixture's reply evaluation was slowed past the harness's poll interval. The fixture now
publishes its DECISION, naming the token, and the harness waits for that.
`TestEveryCountOfWhatArrivedIsTakenAfterTheBarrier` and
`TestTheSelfCheckLooksAfterTheFixtureHasDecided` read both rules off the syntax, because the
mistake in each case is an omission or an order and no reviewer catches those by reading.

## A partial run is not a pass

	0  every scenario selected ran, and passed
	1  a scenario failed
	2  something selected did not run — none, some, or a selection that matched nothing
	3  the harness itself could not get started

This used to be 0 whenever something passed and nothing failed, so a run missing only the
resized-window refusal — the scenario most able to decline, and the one that establishes the guard
refuses at all — reported success. `run.cmd` builds and runs the binary rather than using `go run`,
which would flatten 2 and 3 into 1.

## What it establishes today

Ten scenarios, about 45 seconds, one command — and three more that are written and gated out of a
default run until they have run green once (see below).

| | |
|---|---|
| a seeded routine presses the application | all three channels: what the fixture received, what the record says, and what the window itself says — and it presses Report a problem, which is how the child's transcript is kept |
| a moved window still gets the press | a guarded press re-bases onto the window where it is now |
| a resized window is refused | and the application receives NOTHING, which only the event log can establish |
| a chord arrives as a chord | `ctrl+shift+a`, on three accounts: the order of the key messages, the modifier state the application read from the system when the key arrived, and what this desktop is still holding down afterwards |
| two runs are two records | distinct references, both accounted for |
| Stop ends a run as cancelled | and the press after the stop never arrives |
| a killed runner leaves an unfinished record | `started` with no `ended`, and only the runner this run owns is killed |
| readiness notices a missing sibling | the one readiness case that never fails loudly |
| a routine saved while the window is open appears | the library is read rather than remembered |
| the recording path | the named scripted-recorder tests, labelled apart and never totalled with the rest |

## The limit it prints every time

Marco's recorder discards injected input in both hooks, on purpose. So nothing here can establish
that Windows captured a physical gesture, and the report says so in the same words on a green run
as on a red one:

> physical recording capture: NOT EXERCISED

A person demonstrating something in Studio remains the only evidence of that, and `E2E.md` remains
where it is written down. See
[[ADR-257-a-desktop-harness-that-says-what-it-did-not-prove]] for the three options that were
considered and why none was taken.

## Neither harness may perform the action under test

	THE HARNESS MAY PUT THE SAVE BUTTON THERE. ONLY MARCO MAY PRESS SAVE.

Held by a test in each: no path through either module names `SendInput`, `keybd_event`,
`SetCursorPos`, Marco's input backend, the engine, or the recorder's constructor. A harness that
could perform the action under test could make every scenario pass by doing the work itself, and
the failure would look exactly like a success.

## Validated by

- `plugins/e2e` — the scenarios themselves, the barrier's own self-check, and the nine boundary
  tests beside them
- `plugins/e2e/hold_test.go` — the hold scenario's ORACLE, without a desktop. Every assertion a
  scenario makes is a function over a log, and a log is a value, so the half that decides whether
  a green run means anything is exercised before the scenario has ever run: each case is fed a
  log the fixture could really have written, in both directions — the right one must produce no
  finding, the wrong one must produce one that names what was wrong. Eight mutations, eight killed
- `internal/acceptance` `boundary_test.go` — the oracle rule and the input rule, over the tree
- `plugins/fixture/Fixture.cs` — the witness, and the rule written at the top of it

## A chord is the one thing here asserted against the desktop itself

Every other scenario asks the fixture and the run record. A chord needs a third account, because
the thing that can go wrong with one is invisible to both: a modifier nobody released stays down
on the machine, and no message arrives to tell an application about a key it never saw pressed.
So `keys/a chord arrives as a chord` reads `GetAsyncKeyState` — a read, like every other thing
this harness asks Windows directly — before the run and after it. Before, because a modifier
already held is a fact about this desktop rather than about Marco, and the scenario declines
rather than fails. After, because that is the promise a chord makes.

The fixture supplies the middle account: every key record now carries the modifiers the window
read **from the system** at the instant the key arrived. The order of four messages is evidence
that a chord happened; this is the fact itself, and it is what catches a sender whose modifier
events the system never applied — same four messages, same order, a plain keystroke delivered.
See [[ADR-258-a-chord-is-a-promise-to-let-go]].

## And three hold scenarios, written before a desktop was available and run on one since

| | |
|---|---|
| `hold/a hold is held across the press` | a held key is DOWN, by the application's own reading of the keyboard, when the press inside the hold arrives — and up afterwards, by the desktop's |
| `hold/a stopped run lets go of the key` | Stop during a hold ends the run as cancelled, sends nothing more, and leaves no key down |
| `hold/a ctrl-held click arrives with ctrl held` | a demonstrated Ctrl+click reaches the application as one, by its own reading of the keyboard, and lets go after |

**All three ran green on a desktop on 2026-10-26** (`plugins\e2e\run.cmd -only hold`, 3 of 3,
exit 0), so they are ordinary members of a default run from then on and `onlyWhenAsked` is off
them. Until that run they carried it, because adding an unexercised scenario to a regression gate
makes the next ordinary run a debugging session: the first failure anybody sees is as likely to be
the scenario's own as Marco's, and a gate that cries wolf once gets ignored twice. They were not
merely absent in the meantime, because a harness holding back a scenario it owns and saying
nothing makes its own green summary mean more than it should — so every report named them, beside
the physical-capture limit and in the same spirit:

> implemented and NOT EXERCISED: hold/… Written, gated out of this selection, and waiting on one
> green desktop run before it joins the gate — run with `-only hold`.

**The machinery stays, and the drags are using it now** — see below. `TestAGatedScenarioIsNamedInTheSummary`
keeps the line honest against scenarios of its own rather than whichever ones happen to be gated
today.

They are filed under `hold/` and not `keys/` on purpose, so `-only keys` — the chord's gate — does
not sweep them into every chord change. That is a separate decision and outlives the gate.
`TestTheHoldScenariosRunLikeAnyOther` holds the prefix, the layer, the count, and the inverse of
what it used to: re-gating one would take it out of default runs, and the only trace would be a
line in a report nobody reads twice.

**What the first green run found was in the harness, not the product.** The Ctrl scenario passed
while its own artifact trail said the press inside the hold arrived holding nothing: the line for
a mouse event printed `Watched` and left out `Mods`, which is the account that verdict rests on.
The verdict was right and the evidence beside it was unreadable — and in the failing direction a
reader would have had no way to tell a product defect from a harness that forgot to ask. Both
kinds of line go through one `heldAt` now, and an empty reading says `nothing held`, because a
missing phrase and a key nobody was holding have to look different.

**What the fixture had to learn.** `Control.ModifierKeys` answers for Ctrl, Shift and Alt, which is
the whole question a chord asks and half of the one a hold asks: a held Q is not a modifier, so a
window asked about one could only ever say "no modifiers". The fixture now reports the state of
keys the harness NAMES — `--watch q` — read with `GetKeyState` at the instant the message is
handled, and on PRESSES as well as on keys, because the claim is about what was held when the
click arrived. Named rather than enumerated: a window reporting every key's state would be a
keylogger, and the scenario knows which key it is about.

**And a third, for the gesture that used to be dropped.** `hold/a ctrl-held click arrives with
ctrl held` — a demonstrated Ctrl+click, asserted on the application's own reading of the keyboard
at the instant the press arrived. The fixture reports the modifier state on PRESSES as well as on
keys for it, which is the only account that can settle the claim: the four messages of a chord and
the two of a modified press look the same from outside. Gated with the other two.

**And what they deliberately do not do.** A refused delivery and a refused release are covered
headlessly, in `internal/oshost`. A desktop scenario that needed a key to be STUCK in order to
assert anything would be arranging to leave one down on somebody's computer — and when one of
these does find a key held, the harness reports it and says to press and release the key, because
it sends no input and will not put it up itself.

## And four drag scenarios, written before a desktop was available and run green on one since

| | |
|---|---|
| `drag/a drag arrives as a drag` | the application receives a press, movement with the button held, and a release — not two clicks |
| `drag/it follows an app that moved` | both ends of a guarded drag re-base onto the window as it is now, and the whole gesture stays inside it |
| `drag/a resized app is refused with nothing sent` | the routine refuses before the button goes down, so the application receives nothing at all |
| `drag/a stopped drag lets go where it got to` | Stop with the button demonstrably down releases on the travel, short of the destination, after the stop was asked for |

**All four ran green on a desktop on 2026-10-27** (`plugins\e2e\run.cmd -only drag`, 4 of 4,
exit 0), so they are ordinary members of a default run from then on and `onlyWhenAsked` is off
them. They carried it until that run for the holds' reason — a scenario nobody has exercised must
not turn the next ordinary run into a debugging session — and the gate came off in the commit
where they passed, with `TestTheDragScenariosRunLikeAnyOther` asserting the inverse by name, the
shape `TestTheHoldScenariosRunLikeAnyOther` set.

**It took four runs to get a first result, and not one of the three findings was in the drag
path.** No run found a defect in the drag path: the scenarios' claims and their mutation tests came
through all four unchanged. That is narrower than saying the drag path is correct, which these runs
did not establish. What stopped them was everything around the claims.

- **A guarded drag read back as `legacy`** and all four failed on their own precondition, before
  a gesture was sent. `orchestrator.ContractOf` knew one guarded shape, `is a Point with`, so the
  routine that carries both ends' offsets, the window's size, its scale and the program was
  described as one whose presses are not checked at all — on the Studio card somebody reads before
  pressing Play. A product defect, and the reason the seeded routine's precondition exists. See
  [[ADR-259-a-guarded-drag-is-two-points-in-one-window]].
- **Noticing the press cost a 100ms poll tick** against a gesture that lasts about 190ms — 24
  steps of 8ms, fixed however far the drag goes. Half the gesture was spent before the stop was
  requested. `waitForMouse` asks every 5ms now, which is affordable because the fixture's log is a
  file it reads: asking more often cannot perturb what is being measured. A wait on a window or a
  process keeps the 100ms cadence.
- **Pressing a button costs longer than the gesture.** `uia.press` takes a fresh four-thousand-node
  snapshot of Studio's window before invoking anything, because a UIA runtime id goes stale on a
  repaint and that page repaints on a poll. Measured at 277ms against a 190ms travel, so the stop
  arrived while the drag was already over and the scenario reported — correctly — that nothing
  ordered the two.

**So the stopped drag RAISES the stop rather than pressing it**, which is this harness's one
departure from "the presses are the presses a person makes", and it is recorded where it happens
(`studioRun.stopNow`). It is the same stop: `Acts.Cancel` is `stopsignal.Raise(stopsignal.Home())`
and nothing else, F9 and `director stop` raise the same generation file, and `marco do` watches it.
What it therefore does NOT exercise is Studio's Stop **button** — on the surface, enabled at the
right moment, wired to that call — which `hold/a stopped run lets go of the key` presses, because
a hold lasts long enough to afford the snapshot. The coverage moved to the scenario that can pay
for it. And the raise checks that the generation advanced, because a raise into the wrong home
would leave the run finishing normally and be reported as Marco ignoring a stop — a finding about
the product manufactured by a mistake in the arrangement.

**What was NOT changed to get them green:** the bracket, the ordering rule, and every claim. An
unorderable result is still NOT RUN. Making the harness quick enough to land a stop inside a
gesture is not the same act as widening what counts as having landed one.

They are filed under `drag/` and not `window/`, although two of them are about a window that moved
or was resized and that is how they want to be named. `-only window` is the guarded press's gate,
and the test above failed on the first names for exactly that reason. It asks about every
registered family now rather than the one that was obviously a risk.

**What the fixture had to learn for these.** It logged no pointer movement at all, on purpose: a
replay aims before every click and a person moves the pointer continuously, so logging all of it
would bury the presses in thousands of records. But a drag is not its two ends — those are two
clicks — and the travel between them is the one part of the gesture no account outside the
application can see. So movement is logged **only while a button is down**, which is exactly drag
motion and is bounded by the release. Whether a button is down is counted from the edges the window
RECEIVED rather than read from the mouse's state now, because that is the state at a later instant
than the one being logged.

**Four things these deliberately do not assert**, and each is a way a scenario can be green about
nothing:

- **Not every injected point, and no count.** Windows coalesces mouse moves, so a gesture that
  sent twenty-four can arrive as six depending on how busy the machine is. A count would be an
  assertion about the scheduler. Required instead: **a received point that is neither end**, and
  the ends in order around it. Not merely "a move arrived" — a replay that jumped to the far end
  and let go there sends one of its own, at the destination.
- **A stop that arrived after the release is NOT RUN**, not a pass — and the fixture's own counter
  is what says so, read **on both sides of the Stop request**. An outcome word cannot: it is what
  Marco concluded, not when the application was told, and reading it as a timestamp excuses a stop
  that was IGNORED (`performed` looks like "too late") and convicts a drag nothing interrupted
  (`cancelled` is still recorded when the run was stopped between steps). The stop goes in once
  the application has LOGGED the press, the ordering is asked before any verdict, and a release
  that lands inside the bracket is reported as **ambiguous** — neither a pass nor a failure,
  because a verdict either way would be a guess about which of two things happened first.
- **No exemption is applied before a held button is reported.** Both of the NOT RUN answers above
  are ways for the stopped-drag scenario to report nothing, so the missing release is asked ABOVE
  both of them: a log with a press and nothing else has no movement in it, and the scope exemption
  used to answer first and file a button left down on somebody's desktop as "nothing happened".
  The whole decision is one function rather than a list of claims, because the order of it is the
  part that was wrong.
- **No stuck button is manufactured here.** A release the machine refuses is arranged headlessly,
  where a machine can be told to refuse. On a desktop it would mean leaving a real button held down
  on somebody's computer to see whether the report says so.

See [[ADR-259-a-guarded-drag-is-two-points-in-one-window]].

## And four button scenarios, run green on a desktop the day they were written

| | |
|---|---|
| `button/a right and a middle press arrive as themselves` | two presses, two buttons, two controls, each arriving as the button it was demonstrated with |
| `button/a moved app still gets the right press` | a guarded right press re-bases onto the window where it is now, and is still a right press when it lands |
| `button/a resized app refuses the right press, sending nothing` | the routine refuses before the button goes down, so the application receives nothing at all |
| `button/a ctrl-held right press arrives with ctrl held` | a demonstrated Ctrl+right-click reaches the application as one, by its own reading of the keyboard, and lets go after |

**All four ran green on 2026-10-27** (`plugins\e2e\run.cmd -only button`, 4 of 4, exit 0), so they
are ordinary members of a default run from then on and `onlyWhenAsked` is off them. They carried
it until that run for the holds' and the drags' reason — a scenario nobody has exercised must not
turn the next ordinary run into a debugging session — and the gate came off in the commit where
they passed, with `TestTheButtonScenariosRunLikeAnyOther` asserting the inverse by name.

**They passed 4 of 4 on the first run**, which the two families before them did not: the holds
took two runs and the drags four. That is a fact about the slice rather than about the harness.
Everything the arrangement needed — the press, the guard, the window check, the stop, the drain
barrier, the fixture's log — was already exercised here; what the contract added was a button on
a Point.

They are filed under `button/` and never `window/`, although two of them are about a window that
moved or was resized — `-only window` is the guarded press's own regression gate, and a right
press filed there would be swept into every press change by a flag somebody typed for a different
reason. The drags' reasoning, which is why their names say `app` where the geometry scenarios say
`window`.

**Why the fixture's log is the whole evidence here.** No account inside Marco knows which button
went down. The run record says a routine performed and the host's own count says how many input
actions it asked the machine for; `SendInput` with the wrong flag is a successful send. So a left
press delivered where a right one was demonstrated is, from every account Marco has, a run that
worked — and the application's log of what it RECEIVED is the only thing that can tell them apart.

**And the application's own state cannot corroborate them.** `save_presses` is the fixture's count
of its Save button being CLICKED, which WinForms raises for a left press and not for the other
two. The oracle that confirms every left-handed scenario in this harness is therefore silent for
these BY DESIGN: asserting it would be asserting something that cannot change, and asserting its
absence would be asserting that WinForms is WinForms. Each scenario says so where it uses the log
instead.

See [[ADR-260-a-press-is-a-button-and-a-place]].

## The gate for a change to chords

`plugins\e2e\run.cmd -only keys` is the regression gate for an executable change to the chord
path, and it needs nobody at the keyboard. Run it for a change to the sequence, to the key
resolution, or to what a stopped run reports; a documentation change needs no run at all.

**A foreground interruption is a FAILURE or a NOT RUN, and never a retry.** It fails safe — the
window Marco was going to press is not in front, so it refuses rather than pressing into whatever
is — and a harness that re-ran until green would be turning the one honest answer into noise. The
cases that are genuinely about this desktop rather than about Marco are declined by name: a
modifier already held before the run, a fixture that could not be measured, a remote session.

## Three flakes it had, and the two that are fixed

A gate that fails for its own reasons is a gate people learn to ignore, so these are written down
rather than re-run until green.

- **Done was pressed before the window offered it.** `waitForRecord` returns when the CHILD has
  written its account; Done becomes pressable when the WINDOW has noticed the child exit. The
  two-runs scenario pressed in the gap and reported *the window would not go back to the
  routines*. It waits for the button now, in the same shape `play` already waited for Play.
- **The fixture's own snapshot could not be read while it was being replaced.** The fixture wrote
  a temp file, deleted the real one and moved the temp over it — so a reader arriving in the gap
  was told *Access is denied*, which is Windows's answer for a file whose deletion is pending and
  not a sentence anybody would connect to a file being rewritten. The swap is `File.Replace` now,
  which is atomic, and the harness retries a transient read anyway: a reader of another process's
  file does not get to assume how that process writes it.
- **Not fixed, and recorded as observed:** once in about ten full runs, the first scenario failed
  with Marco's own refusal — *the fixture window is no longer in front, so a press would land in
  whatever is*. Marco was right to refuse and the harness was right to report it; what took the
  foreground between the activation and the press is not established. It fails SAFE, which is why
  it is recorded rather than worked around.
