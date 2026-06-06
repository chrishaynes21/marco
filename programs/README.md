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
| `windows` | real keystrokes/mouse via Win32 `SendInput` |
| `bridge:<exe>` | a subprocess in any language performs the effect (e.g. `../bridges/ahk-bridge.ahk`) |

```sh
# See the ARK transfer steps without touching the game:
marco run --host dryrun  programs/ark.marco

# Drive the real game UI:
marco run --host windows programs/ark.marco

# Type into a focused Notepad window:
marco run --host windows programs/notepad.marco
```

## Programs

- **`ark.marco`** — the ARK: Survival Evolved cloud-pull transfer: a click/wait/
  type sequence performed once via `do Ark's Transfer`.
- **`notepad.marco`** — types two lines into the focused window.
- **`globals.marco`** — global commands routed by **hotkey events**. Run it as a
  persistent server and feed it events on stdin:

  ```sh
  marco serve --host windows programs/globals.marco
  # then, from a hotkey daemon / the AHK UI, pipe lines like:
  # {"feed":"Hotkeys","event":"E"}
  # {"feed":"Hotkeys","event":"Greet"}
  ```

  Each `when Hotkeys reads <X>?` listener runs one command. A host that captures
  the backtick leader key writes those event lines — letting the existing AHK UI
  act as the front-end while Marco orchestrates.
