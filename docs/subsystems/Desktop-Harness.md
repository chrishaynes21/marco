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

Ten scenarios, about 45 seconds, one command.

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
