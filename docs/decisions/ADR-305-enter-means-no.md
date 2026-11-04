---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED (THE OWNER'S RULING) — ENTER MEANS NO FOR EVERY TEXTUAL YES/NO CONFIRMATION, AND EVERY ONE SHOWS IT AS [y]es / [N]o. Only an explicit y or yes runs an offered routine, begins an offered recording, deletes, replaces, saves or grants permission; a bare Enter, and an EOF, decline. ENTER STILL SUBMITS A COMMAND, and runs a read-back that is on screen: that is the command line, not a yes/no question. Choice prompts (which window, which scope) keep their stated defaults, and Studio's buttons are unchanged."
supersedes: []
affects:
  - interface
source_paths:
  - cmd/marco/assistant.go
  - cmd/marco/dispatch.go
  - internal/orchestrator/orchestrator.go
  - internal/orchestrator/authority.go
  - plugins/overlay/acts.go
  - plugins/overlay/controller_windows.go
---

# Enter means no

F6 of [[Surfaces-alignment-proposal-2026-11-02]], part of S8. The owner ruled on what Enter
means.

## What it was

The finding recorded one disagreement: the widget's offers took Enter as yes, and the access
check before a run (`AskFirst`) took it as no. The inventory found more. These meant **yes**:

- the widget's *Run …?* offer and its *Learn …?* offer;
- `askYes` in the CLI: `marco forget` (one, and all), *Did you mean …?* before a simplify, and
  `marco dispatch`'s Run and Learn offers. `askYes` also took an EOF as yes, so a script piping
  nothing deleted;
- narration's *Save this as …?* (`chooseSave`);
- Simplify's *Save the simplified version?* (the shared `confirm`), which replaces a routine.

These already meant **no**: `AskFirst`, the CLI's bind and replace questions, `marco rename`, the
save questions of `marco learn` (ADR-266), and Director's two `[y/N]` questions.

## What was decided

- **Every textual yes/no confirmation takes only `y`/`yes` as yes.**
  - `askYes` and `chooseSave` now say no to Enter, and an EOF is no answer.
  - The shared `confirm` was removed. Its one caller, Simplify's question, uses
    `confirmDefaultNo`. That changes the meaning of Simplify's question and nothing about the
    Simplify operation, which stays the reviewed path.
  - The widget's offers are answered by `answerPrompt`, moved out of the Windows-only loop so it
    can be tested.
- **Every one shows it.** `[y]es / [n]o` became `[y]es / [N]o` throughout. Prompts that already
  meant no but displayed `[n]o`, such as `AskFirst`, now display the default too.
- **Enter still submits.** Typing a command and pressing Enter runs it, and Enter on a read-back
  that is on screen runs that read-back (ADR-301). Those are the command line's own action, not
  answers to a question.
- **Not changed.** These keep their stated defaults:
  - choice prompts, such as *Which window* (the first, the foreground) and the scope question (the
    ruled default, R2);
  - Studio's buttons and their keyboard activation.

## Compatibility

- A script that piped nothing into `marco forget` deleted. It now keeps the routine and needs
  `echo y |`.
- In the widget, a routine Director found and an offered recording each need `y`. Enter declines
  both.
- Narration's save question discards on Enter.

## Enforced by

- `plugins/overlay`: `TestEnterDeclinesAnOfferAndStillSubmitsACommand`. It covers both sides in
  one test: Enter declines the Run offer and the Learn offer, `y` runs the routine, and Enter on a
  typed line with its read-back on screen still runs it.
- `cmd/marco`: `TestEnterDeclinesEveryCLIConfirmation`, covering Enter, EOF, n, y and Yes.
- `internal/orchestrator`: `TestEnterAtTheSaveQuestionDiscards`, `TestEnterAtTheSimplifyQuestionLeavesTheRoutine`.

Mutations: 4 runs, all killed: Enter as yes again in the widget, in `askYes` and in `chooseSave`,
and Simplify's question bypassed. The last needed a second form to compile.

## Not established

- No desktop run. Windows only.
- A spoken answer to a widget prompt is not part of this. The prompts are answered by keys.
