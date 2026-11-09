---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — SHIFT ALONE ON A KEY THAT TYPES NOTHING IS RECORDED AS THE CHORD IT IS. `simplify` treated Shift on its own as the key that makes capitals, so on a key with no character (Tab, the arrows, F-keys, the numpad, Enter) it was dropped: shift+tab saved as tab, shift+f5 as f5, shift+num1 as num1, and shift+left (a text selection) as left. Such a key with Shift held is now a chord step, `shift+tab`, exactly as one with Ctrl, Alt or Win always was. Shift on a key that types a character is unchanged: the capital or the symbol it types."
supersedes: []
affects:
  - execution
source_paths:
  - internal/simplify/simplify.go
---

# Shift alone is kept on a key that types nothing

Follows [[ADR-335-the-keys-games-bind-are-recorded-apart]].

## Why

Found while answering the owner's question about chords ("we can still do chords with them
right?"). Ctrl, Alt and Win chords were kept, and Shift on its own was lost on every key that
types nothing.

## Not established

- **Shift on a letter or a digit is still recorded as the character it types.** A game that binds
  Shift+E receives a typed `E`, and a game reading scan codes may not take typed text as a
  keypress. That needs its own decision: when a capital is typing and when it is a key.

## Enforced by

- `internal/simplify`: `TestShiftAloneOnAKeyThatTypesNothingIsAChord`.
