---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — SHIFT ALONE ON A KEY THAT TYPES NOTHING IS RECORDED AS THE CHORD IT IS. `simplify` treated Shift on its own as the key that makes capitals, so on a key with no character (Tab, the arrows, F-keys, the numpad, Enter) it was dropped: shift+tab saved as tab, shift+f5 as f5, shift+num1 as num1, and shift+left (a text selection) as left. Such a key with Shift held is now a chord step, `shift+tab`, exactly as one with Ctrl, Alt or Win always was. And a keystroke that typed a capital or a symbol and was left a keystroke (one press, or a repeat, that was not folded into typed text) is now pressed with Shift, `shift+e`: it was saved as `Key \"e\"` and played back as a small e. A run folded into typed text types its own capitals and is unchanged."
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

- When a run of capitals is folded into typed text, a game reading scan codes may not take it as
  keypresses. Typed text has always been sent that way.

## Enforced by

- `internal/simplify`: `TestShiftAloneOnAKeyThatTypesNothingIsAChord`, `TestShiftOnALetterLeftAsAKeyIsPressedWithShift`, `TestShiftForACapitalLetterIsStillTyping` (now one `shift+a` step).
