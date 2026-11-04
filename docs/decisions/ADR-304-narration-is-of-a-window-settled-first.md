---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — A NARRATED RECORDING'S WINDOW IS SETTLED BEFORE NARRATION STARTS, AND THE SESSION IS OF THAT WINDOW. The widget starts `marco learn --narrate` with `--window <the window in front>`, as its interactive learn already did, and starts nothing when it has none to give. A terminal that must answer `Which window` reads that one line and not a byte more, so the narration typed after it reaches the session. And the engine no longer throws the choice away: the narration session places each click relative to the chosen window and files the routine under that window's application, instead of whatever is in front as each phrase lands."
supersedes: []
affects:
  - learning
source_paths:
  - cmd/marco/assistant.go
  - plugins/overlay/acts.go
---

# Narration is of a window settled first

F7 of [[Surfaces-alignment-proposal-2026-11-02]], part of S8 in the owner's numbering. It was
traced, not executed, when the proposal recorded it. These are now the three defects found in the
source.

## What was wrong

1. **The widget gave no window.** It ran `learn --narrate <name>` with no `--window`, while its
   interactive `learn` passed the window in front. With several windows open the engine asked
   *Which window should Marco record?* on stdin, and that stdin is where the widget writes every
   narrated phrase. So the first phrase (*click this*) was taken as the answer.
2. **The answer was read through a buffered reader.** Even at a terminal, `bufio.NewReader`
   reading the answer could take the narration typed right after it, and the session's own
   scanner would then never see it.
3. **The choice was then thrown away.** `runNarrateLearn` built a fresh `newDeps()`. The session
   placed clicks relative to whatever window was in front at each phrase and filed the routine
   under the foreground application, whichever window had been chosen.

## What was decided

- **The widget gives the window or starts nothing** (`runNarrateLearn` in `plugins/overlay`).
  - It passes `--window <window in front>`, as `runLearnInteractive` does: the HUD reads the
    keyboard through the global hook and does not take the foreground, so the window in front is
    the one the person typed the command over.
  - With no window to give, it starts no child and says *Marco could not tell which window you
    are in, so narration was not started — bring the window forward and say it again*. It also
    clears the session flag, which the caller sets and only a started session would clear;
    otherwise every later phrase would be forwarded to nothing.
- **The answer is one line** (`readWindowAnswer`, through the existing unbuffered
  `readStdinLine`).
- **The session is of the chosen window** (`narrationIn`). The deps keep the chosen window, the
  application is that window's process image, and `WindowOrigin` reads that window's rectangle
  (`chosenWindowEnv`), reporting none once it is gone. With `--whole-desktop` nothing is chosen,
  and the session is the foreground's, as before.

## Enforced by

- `cmd/marco`: `TestTheWindowAnswerLeavesTheNarrationAlone`, and on Windows
  `TestANarrationIsOfTheChosenWindow` and `TestTheNarratedRoutineIsFiledUnderTheChosenWindow`. The
  last one enters `runNarrateLearn` with narration on a piped stdin and checks where the routine
  is saved.
- `plugins/overlay`: `TestNarrationStartsInAWindowItWasGiven`.

Mutations: 6 runs, all killed:

- in the widget: no `--window`, the flag left set, and starting with no window;
- in the engine: a buffered answer (this needed a second form to compile), dropping the chosen
  application, and fresh deps in the session. Fresh deps survived until the end-to-end test was
  added.

## Not established

- No desktop run. Narration's clicks against a real window, and the origin read from a real one,
  were not exercised.
- Windows only.
- The window question's own Enter (*the first one, which is the foreground*) is a choice and not a
  yes/no confirmation, so F6 leaves it as it is.
