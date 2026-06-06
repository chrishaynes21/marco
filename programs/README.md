# MacroMarco, ported to Marco

These are the original MacroMarco automation macros rewritten as Marco programs.
Marco owns the choreography (sequences, branching, hotkey routing); a **host**
performs the real OS effects through the `OS` act (see `../spec/Hosts.md`).

`os.marco` is the shared act surface — the foreign primitives every macro uses.
Each program imports it with `use os.`.

## Running

Pick a host with `--host`:

| Host | Effect |
|------|--------|
| `dryrun` (default) | prints each OS call; does nothing real — good for a dry read |
| `windows` | real keystrokes/mouse via Win32 `SendInput`, plus randomness |
| `bridge:<exe>` | a subprocess in any language performs the effect (e.g. `../bridges/ahk-bridge.ahk`) |

```sh
marco run   --host dryrun  programs/ark.marco      # see the ARK transfer steps
marco run   --host windows programs/ark.marco      # drive the real game (ARK focused)
marco run   --host windows programs/notepad.marco  # type into a focused Notepad
```

## Programs

- **`ark.marco`** — ARK cloud-pull **transfer** for multiple items (`for each`),
  window-scoped via a `Focus` check, plus a context-action command. Runs the
  "rock me mama" preset (stone + flint from the argent).
- **`notepad.marco`** — types two lines into the focused window.
- **`globals.marco`** — global commands, hotkey-routed: **spam E / spam click
  until stop**, hotbar keys 1–4 → 5–8. The spam runs in the host (one loop at a
  time, exactly like the original engine) and persists until `Stop`.
- **`fun.marco`** — dice **roller**, Magic **8-Ball**, fantasy **name** generator.
  These need randomness, supplied by the host (`Roll` / `EightBall` / `Name`), so
  run them with `--host windows`.

## Hotkey-driven programs (the leader-key engine)

`globals.marco` and `fun.marco` are **servers**: they stay alive and react to
inbound `Hotkeys` events. Drive them with the original backtick leader key via
the AHK front-end, which captures the chords and pipes JSON events:

```sh
AutoHotkey.exe ../bridges/ahk-hotkeys.ahk | marco serve --host windows programs/globals.marco
```

`ahk-hotkeys.ahk` emits, e.g., `` `e `` → `{"feed":"Hotkeys","event":"E"}`,
`` `c `` → `C`, `Esc` → `Stop`, `` `1 ``..`` `4 `` → `K1`..`K4`. So the existing
AutoHotkey UI stays the front-end while Marco runs the macro logic.

You can also drive them by hand for a quick test:

```sh
printf '{"feed":"Hotkeys","event":"E"}\n{"feed":"Hotkeys","event":"Stop"}\n' \
  | marco serve --host dryrun programs/globals.marco
```

## Not ported

The `marco.ahk` meta module (config panel, themes, layouts, log viewer, visual
builder) and the stamp/coords tool are about the **AHK GUI itself**, not macros —
they stay in the AutoHotkey front-end. This port covers the automation: the
commands that actually drive the keyboard, mouse, and game.
