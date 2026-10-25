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
5. **Compares two accounts** and reports the FIRST place they disagree, from the act outwards.
   Everything after the first disagreement is consequence, and listing consequences as findings is
   how a report becomes something nobody reads.

## What it establishes today

Nine scenarios, about 36 seconds, one command.

| | |
|---|---|
| a seeded routine presses the application | all three channels: what the fixture received, what the record says, and what the window itself says |
| a moved window still gets the press | a guarded press re-bases onto the window where it is now |
| a resized window is refused | and the application receives NOTHING, which only the event log can establish |
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

- `plugins/e2e` — the scenarios themselves, and the four boundary tests beside them
- `internal/acceptance` `boundary_test.go` — the oracle rule and the input rule, over the tree
- `plugins/fixture/Fixture.cs` — the witness, and the rule written at the top of it
