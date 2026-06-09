# overlay — the native gamer-HUD UI layer

A **native, cross-platform gamer overlay** for the headless `marco` engine: a
transparent, click-through, always-on-top HUD with a leader-key command line and
spam hotkeys. It replaces the AutoHotkey overlay (`ahk-overlay`) with a real
windowed app, and — unlike that one — its **behaviour lives in Marco**
(`programs/overlay.marco`); this process only renders the window and captures
global input.

It's the **UI layer** in a three-layer split:

```
overlay (this — ebiten HUD)  ──Overlay act resp / Keys+Commands events──┐
                                                                        │ bridge stdio
marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay overlay.marco
                                                                        │ bridge stdio
marco-macros (the macros layer)  ──OS act resp (Key/Click/Spam/Find/…)──┘
```

- **Engine** (`marco`) — the brain. Choreography + the `marco do/routes/active` seam.
- **Macros layer** (`cmd/marco-macros`) — fulfils the `OS` act (real keystrokes,
  clicks, spam, find …) as a separable process.
- **UI layer** (this) — fulfils the `Overlay` act and pushes the events that move
  the HUD.

## Architecture — Marco is the brain; the Go side is Model/View/Controller

The overlay is split so behaviour lives in Marco and the Go process is a thin,
well-separated front-end:

- **Marco (the brain)** — `programs/overlay.marco`. Dispatches typed commands
  (runs the route), and pushes display state out through the `Overlay` act.
- **Model** — `model.go`. The single source of truth for what the HUD shows.
  Driven by Marco (via the act handlers in `acts.go`) and by the controller for
  the input it must own synchronously (command buffer, leader echo, capture mode).
- **View** — `view.go`. An ebiten game that only renders a snapshot of the model
  (window, fonts, theme, layout, fade, clock). It decides nothing.
- **Controller** — `controller_windows.go` (+ `controller_other.go` stub). Captures
  global input, updates the model's live input, and emits intents (the `Commands`
  feed) to Marco. Key suppression is synchronous here because it can't round-trip.
- **`acts.go`** — the boundary: the `Overlay` act handlers (which write the model)
  and the `marco do/routes/active` CLI seam.

## How it connects — the bidirectional bridge

The overlay is a **bidirectional bridge host** (see `../../spec/Hosts.md`). The
engine calls *out* to the `Overlay` act; the overlay pushes events *in* on the
same stdout, demuxed by shape:

| Engine → overlay (Overlay act) | Overlay → engine (events) |
|---|---|
| `Show` / `Hide` | `Commands` `Run` — a typed command (data = text) |
| `Status` (text) / `Log` (text) / `Clear` | `Hotkeys` `E` / `C` / `Stop` — spam keys |
| `SetInput` (text) | |
| `ListRoutes` → names · `Run` (name) · `Active` → app | |

`Run`/`ListRoutes`/`Active` shell the same CLI seam `web-ui` uses
(`marco do "<name>"`, `marco routes --json`, `marco active`); `$MARCO_BIN` points
at the engine (default `marco` on PATH).

## The HUD

Backtick (`` ` ``) is the **leader**; the next key chooses what happens:

- **`` `m ``** — open the marco command line. Type a route name, **Enter** runs it
  (output streams into the HUD log). Type **`teach <name>`** to record a new route
  by demonstration (opens a guided console: demonstrate, press F12, save). Type
  **`help`** (or **`` `h ``**) shows the help menu — the leader keys plus your
  known routes. Type **`config`** for the
  live **config editor**: **↑/↓** pick a setting, **←/→** change it (applies
  instantly), **S** saves it to disk, **Esc** closes. Editable: theme (default,
  dracula, solarized-dark, monokai, nord, tokyo-night, catppuccin-mocha,
  gruvbox-dark, rose-pine, light), idle opacity, monitor, corner, width, max log
  lines, **border** (accent outline on/off), **mini** (just the command line —
  hides the clock/state/logs), **metrics** (CPU/RAM widget in the crown, full
  mode only), leader key. (Height isn't a setting — the panel
  **auto-fits its content**,
  growing for menus/logs and shrinking when idle; the command echo and log tail
  also clear after a minute idle.) Saved to
  `%AppData%\marco\overlay.json` (override with `MARCO_OVERLAY_CONFIG`); that file
  takes precedence over the `MARCO_OVERLAY_*` env vars once written.
- **`` `<key> ``** — runs the route **bound** to that key in the **current app**
  (and nothing if unbound — harmless over a game). Bind one from the command line:
  focus the app, then `` `m `` → `bind s say hello` (binds `` `s `` → "say hello"
  for that app); `unbind s` removes it.
- **Command history** — each command you run lists in the panel with its outcome
  + time, right-justified and colour-coded: **✓** success (green), **■** canceled
  (grey), **✗** failed (red). It fades with the text and sticks ~1 minute, then
  clears. Capped to the `max lines` config.
- **Voice (optional)** — run the [`voice`](../voice) layer piped into `serve`;
  speaking shows a live preview and a finished phrase runs as a command.
- **Esc** — cancel the command while typing; otherwise stop a running route
  (→ canceled) and unfocus the panel. Esc is *not* swallowed when idle, so games
  still get it.

A bare leader with no follow-up key disarms after 2s. Keys the overlay consumes
(the leader and the chosen command key) never reach the game.

## Build & run

```sh
go build -o marco.exe ./cmd/marco                 # engine
go build -o marco-macros.exe ./cmd/marco-macros    # macros layer
go -C plugins/overlay build -o overlay.exe .       # this UI layer

set MARCO_BIN=%CD%\marco.exe
marco serve --host OS=bridge:marco-macros --host Overlay=bridge:.\plugins\overlay\overlay.exe programs\overlay.marco
```

Closing the HUD window ends the served run (the engine reaps the macros layer).

It fades: a faint dark panel when idle (≈ the AHK's idleOpacity), ~opaque on the
leader key or any activity, then back to faint ~5s later. It never steals focus (the global
hook captures input while the panel stays click-through) — it draws attention,
not focus.

Env: `MARCO_OVERLAY_THEME` (`default` teal | `dracula`), `MARCO_OVERLAY_SIZE`
(`WxH`, default `340x270`), `MARCO_OVERLAY_POS` (`X,Y`; default top-right),
`MARCO_OVERLAY_MONITOR` (index; default `0` = primary/main),
`MARCO_OVERLAY_IDLE` (idle panel opacity 0–1; default `0.47`),
`MARCO_OVERLAY_LEADER` (leader key: `` ` `` | `capslock` | `tab` | a letter | `f8` …),
`MARCO_OVERLAY_FONT` (path to a `.ttf`; defaults to Consolas on Windows),
`MARCO_BIN`, `MARCO_ROUTES`.

The HUD reproduces the MacroMarco look: an opaque rounded, theme-bordered panel
(gomono / gomonobold, Consolas-like) with the title crown (clock · date · Marco),
the `· app  word` state line, the `` `m config | `m help`` idle hint, a `>`
command line, and a log tail — top-right, default teal theme. The foreground app
in the state line is polled via `marco active`. Not yet ported from the AHK
overlay: the live **config panel** (tabs: Presets/General/Keybinds/Appearance/
Layout/Widgets) and the history/metrics/monitor widgets.

## Portability

Rendering is **ebiten** — transparent / undecorated / floating / mouse-passthrough,
cross-platform. Global input capture is **Windows-first** (a `WH_KEYBOARD_LL`
hook, `input_windows.go`). macOS/Linux build with a stub (`input_other.go`): the
HUD still renders and runs engine-driven routes, but leader/typed-command capture
awaits a `CGEventTap` (macOS) / `XGrabKey`/evdev (Linux) backend. (ebiten needs a
C toolchain on macOS/Linux; the Windows build is cgo-free.)

A future renderer can swap in behind the same `Overlay` act — the boundary is
chosen so the HUD could one day be drawn natively from Marco itself.

## Same seam as every front-end

Like `web-ui` and `ahk-overlay`, this is one front-end over the engine's seam —
here joined to a clean `Overlay` act and bidirectional events so the HUD's logic
can live in Marco. The engine stays headless; UIs are layers.
