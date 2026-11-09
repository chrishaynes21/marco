---
type: adr
status: accepted
date: 2026-11-09
decision: "ACCEPTED — A SHORTCUT MAY BE PRESSED WITHOUT THE LEADER. A key or a combo (`f9`, `ctrl+1`, `alt+q`, `num5`) bound to a play runs it on its own: the widget's keyboard hook catches the press before the application in front sees it and asks for the play through the same door as a key after the leader, `marco hotkey`, carrying the application it was caught for. It is stored as `direct:<keys>` in Binding.Key, so it is a different shortcut from the same key after the leader everywhere app and key are compared, with every rule of ADR-297 unchanged. The combo is stored in the recorder's spelling (macroir.CanonicalChord: modifiers ctrl, alt, shift, win first, every key by the recorder's name), so what somebody typed and what the hook sees pressed are one text. Refused: a key Marco does not know, modifiers alone, the leader, Esc, and a key that types (a letter, digit, symbol, Space, Enter, Tab, Backspace, Delete, a numpad key) listened for everywhere unless Ctrl, Alt or Win is held with it. While somebody is recording, or that cannot be read, nothing is caught. Set from Studio's Shortcut tab with `no leader` ticked (the box then takes the combo pressed in it), or `marco bind --direct`."
supersedes: []
affects:
  - interface
  - execution
source_paths:
  - internal/routes/bindings.go
  - internal/orchestrator/shortcuts.go
  - internal/macroir/keys.go
  - internal/recorder/keys_windows.go
  - cmd/marco/bind.go
  - plugins/overlay/direct.go
  - plugins/overlay/direct_windows.go
  - plugins/overlay/controller_windows.go
  - plugins/overlay/arm.go
  - plugins/studio/shortcuts.go
  - plugins/studio/replace.go
  - plugins/studio/forgetone.go
  - plugins/studio/frontend/index.html
---

# A hotkey without the leader

Follows [[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]] and
[[ADR-298-studio-sets-shortcuts-through-the-one-owner]].

## Why

The owner asked for "literal hotkeys that don't require a leader". In a game, the leader then a key
is two presses where one is wanted, and a key held with Ctrl or a spare F-key is the usual way a
macro is started.

## How it is caught

- **Off the hook thread, a watcher** (every 100ms) reads whether anybody is recording, the shortcuts
  (only when the file changes), the window in front and its application, and publishes a table: the
  combos bound without the leader for that application and everywhere, and the window it was read
  for.
- **On the hook thread**, the modifiers held are followed from the hook's own events. A press that is
  not a modifier builds its combo and is caught only if the window in front is the table's window
  and the combo is in the table. Its key-repeats and its release are caught with it, so the
  application never sees half a key. Injected input is ignored, as it always was.
- **The press** goes through the leader's door: `carryPress` with the application, then the
  `Hotkeys` event, `pressShortcut` and `marco hotkey direct:<keys> --armed-in=<app>`, which refuses
  if something else is in front by then. The list shown after the leader leaves these out.

## Not established

- Not tried on a desktop or in a game. The hook was entered through `handleKey` with a stated
  table and window.
- A press in the first 100ms after switching to a game is not caught: the table still names the
  window that was in front before.
- The numpad's Enter is caught as `enter`: the hook does not read the extended flag the recorder uses
  to tell them apart.
- Mouse buttons cannot be hotkeys yet.
- The control centre lists these but cannot set one.
- A hotkey shared with something the game binds is the person's choice and is not warned about.

## Mutation run

24 mutants across the hook, the watcher, the engine's rules, the command line, Studio's backend and
its page; all killed once `TestTheListAfterTheLeaderLeavesOutAHotkeyWithoutIt` and
`TestAHotkeyWithoutTheLeaderIsSaidWithoutIt` were added and the in-use test checked how the key is
shown. One line was dead (a modifier check the key naming already made) and was removed. The first
two runs were void: the runner split the test pattern on its own `|` and ran one test name only.

## Enforced by

- `plugins/overlay`: `TestAHotkeyWithoutTheLeaderIsCaughtAndAsksForItsPlayOnce`, `TestAHotkeyIsNotCaughtInAnotherWindow`, `TestAnUnboundKeyAndAMissingTableCatchNothing`, `TestAfterTheLeaderTheKeyIsTheLeaders`, `TestTheActionLoopCarriesTheApplicationItWasCaughtFor`, `TestTheHookSpellsAComboAsItIsStored`, `TestATableHoldsWhatAPressHereCouldRun`, `TestThereIsNoTableWhileSomebodyIsRecording`.
- `internal/orchestrator`: `TestAHotkeyWithoutTheLeaderIsNotTheLeadersKey`, `TestAHotkeyIsStoredInTheSpellingItIsPressedIn`, `TestAHotkeyThatWouldTakeAKeyAwayIsRefused`, `TestAHotkeyThatTakesNothingIsAllowed`, `TestAHotkeyIsSaidWithoutTheLeader`.
- `internal/macroir`: `TestACanonicalComboIsOneSpelling`.
- `cmd/marco`: `TestAHotkeyWithoutTheLeaderRunsItsPlayAtTheDoor`, `TestDirectIsAFlagOfBindAndUnbind`, `TestTheListAfterTheLeaderLeavesOutAHotkeyWithoutIt`, `TestAHotkeyWithoutTheLeaderIsSaidWithoutIt`.
- `plugins/studio`: `TestAHotkeyWithoutTheLeaderSetInStudioIsWhatAPressRuns`, `TestAHotkeyThatWouldStopTypingIsRefusedInStudio`, `TestTheKeyBoxTakesAComboPressedWithoutTheLeader`.
