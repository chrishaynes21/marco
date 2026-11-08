---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — THE KEYS GAMES BIND ARE RECORDED AND PRESSED APART. The recorder named the numpad's digits as the top row's (a numpad 1 recorded as `1` and was played back as the top-row 1, and as typed text at that), recorded numpad Enter as Enter, and recorded F13-F24, the numpad's operators, Num Lock, Scroll Lock, Pause, Print Screen and the menu key as `?`. Each now has its own name in all three key tables (the recorder's, the host's, and `macroir`'s list that Studio checks an edit against): `num0`-`num9`, `numstar`, `numplus`, `numminus`, `numdot`, `numslash` (spelt out, because `+` joins a chord), `numenter`, `numlock`, `scrolllock`, `pause`, `printscreen`, `menu` and `f13`-`f24`. Numpad Enter is told apart by the hook's extended flag and pressed as Enter sent extended. Numpad divide, Num Lock, Print Screen and the menu key are sent as extended keys, so a game reading scan codes sees them and not the main-block key that shares the code. Left and right modifiers are still recorded as one `shift`, `ctrl` and `alt`."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/recorder/keys_windows.go
  - internal/recorder/recorder_windows.go
  - internal/oshost/keys_windows.go
  - internal/macroir/keys.go
---

# The keys games bind are recorded apart

Follows [[ADR-334-the-side-buttons-are-recorded-and-pressed]], from the same request: "Recording
should record the full range of inputs that can be done".

## Decision

- **New names** are listed in the decision above. They are words, never `num+`, because a
  key step's name is split on `+` into a chord.
- **Numpad digits are no longer folded into typed text.** `num1` is not a one-character name,
  so it becomes `press num1`, which is what a game reads.
- **A recording made before this is unchanged.** Its numpad presses were saved as `1`, and they
  still play back as the top-row key.
- **Not distinguished yet:**
  - left from right Shift, Ctrl and Alt;
  - media and browser keys.

## Enforced by

- `internal/recorder`: `TestTheNumpadsKeysAreRecordedApart`, `TestTheKeyHookTellsNumpadEnterApart`, `TestEveryKeyTheRecorderWritesIsOneMarcoCanPress`.
- `internal/oshost`: `TestTheNumpadsKeysArePressedApart`, `TestTheHostPressesExactlyTheKeysMarcoNames`.
