---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — THE WIDGET'S COMMAND LINE HAS A CARET THAT MOVES. ← and → move it a character, Ctrl+← and Ctrl+→ a word, Home to the start of the text and End to the end; typing inserts at it, Backspace and Delete remove the character before and after it, and with Ctrl the word. The caret is kept as a distance from the end, so everything that rewrites the line (↑/↓, Tab completion, a read-back reopening the line) leaves it at the end as before. Characters, not bytes. A letter typed with Ctrl held types nothing. No selection yet."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/lineedit.go
  - plugins/overlay/controller_windows.go
  - plugins/overlay/model.go
  - plugins/overlay/view.go
---

# The command line edits like an editor

Builds on [[ADR-326-the-command-line-recalls-commands]].

## Why

The owner asked to navigate the widget's command line "similarly to editors, such as using the
arrow keys to go to specific chars". It had no caret: typing appended and Backspace removed the
last byte, so fixing one letter of a recalled command meant deleting back to it.

## Decision

- **Keys** (while the line is open, swallowed from the application; the hook thread only queues):
  ← →, Ctrl+← Ctrl+→, Home, End, Delete, Ctrl+Delete, Ctrl+Backspace. ↑ and ↓ stay the recall.
- **The caret** is `cursorBack`, characters from the end of the line. Setting the line resets it,
  so no caller that writes the line had to learn about it. Home stops after the space the badge
  leaves, and ← does not go into it.
- **Characters, not bytes**: Backspace on "café" removes "é" (it removed half of it before).
- **Ctrl**: tracked like Shift and passed through to the application; a letter with Ctrl held is
  not typed, so Ctrl+C is not a "c".
- **Drawn** where it is: the `_` sits under the character the caret is before, measured by laying
  out the text up to it the way the line is laid out.

## Not established

- Not tried live, and the drawing is not tested (it needs Ebiten's screen).
- No selection (Shift+arrow moves like the arrow), no clipboard, no mouse.

## Enforced by

- `plugins/overlay` `TestTheCommandLineEditsAtTheCaret`, `TestTheEditorKeysMoveTheCaret`.
